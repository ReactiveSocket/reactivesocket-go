package mono

import (
	"context"

	"github.com/jjeffcaii/reactor-go"
	"github.com/jjeffcaii/reactor-go/mono"
	"github.com/jjeffcaii/reactor-go/scheduler"
	"github.com/pkg/errors"
	"github.com/rsocket/rsocket-go/payload"
	"github.com/rsocket/rsocket-go/rx"
)

type proxy struct {
	mono.Mono
}

func (p proxy) Raw() mono.Mono {
	return p.Mono
}

func (p proxy) mustProcessor() mono.Processor {
	m, ok := p.Mono.(mono.Processor)
	if !ok {
		panic(errors.Errorf("require processor but %v", p.Mono))
	}
	return m
}

func (p proxy) Success(v payload.Payload) {
	p.mustProcessor().Success(v)
}

func (p proxy) Error(e error) {
	p.mustProcessor().Error(e)
}

func (p proxy) ToChan(ctx context.Context) (c <-chan payload.Payload, e <-chan error) {
	errorChannel := make(chan error, 1)
	payloadChannel := make(chan payload.Payload, 1)
	p.
		DoOnSuccess(func(input payload.Payload) error {
			payloadChannel <- input
			return nil
		}).
		DoOnError(func(e error) {
			errorChannel <- e
		}).
		DoFinally(func(s rx.SignalType) {
			close(payloadChannel)
			close(errorChannel)
		}).
		Subscribe(ctx)
	return payloadChannel, errorChannel
}

func (p proxy) SubscribeOn(sc scheduler.Scheduler) Mono {
	return newProxy(p.Mono.SubscribeOn(sc))
}

func (p proxy) Block(ctx context.Context) (pa payload.Payload, err error) {
	v, err := p.Mono.Block(ctx)
	if err != nil {
		return
	}
	if v != nil {
		pa = v.(payload.Payload)
	}
	return
}

func (p proxy) Filter(fn rx.FnPredicate) Mono {
	return newProxy(p.Mono.Filter(func(i interface{}) bool {
		return fn(i.(payload.Payload))
	}))
}

func (p proxy) DoFinally(fn rx.FnFinally) Mono {
	return newProxy(p.Mono.DoFinally(func(signal reactor.SignalType) {
		fn(rx.SignalType(signal))
	}))
}

func (p proxy) DoOnError(fn rx.FnOnError) Mono {
	return newProxy(p.Mono.DoOnError(func(e error) {
		fn(e)
	}))
}
func (p proxy) DoOnSuccess(next rx.FnOnNext) Mono {
	return newProxy(p.Mono.DoOnNext(func(v reactor.Any) error {
		return next(v.(payload.Payload))
	}))
}

func (p proxy) DoOnSubscribe(fn rx.FnOnSubscribe) Mono {
	return newProxy(p.Mono.DoOnSubscribe(func(su reactor.Subscription) {
		fn(su)
	}))
}

func (p proxy) DoOnCancel(fn rx.FnOnCancel) Mono {
	return newProxy(p.Mono.DoOnCancel(fn))
}

func (p proxy) SwitchIfEmpty(alternative Mono) Mono {
	return newProxy(p.Mono.SwitchIfEmpty(alternative.Raw()))
}

func (p proxy) Subscribe(ctx context.Context, options ...rx.SubscriberOption) {
	p.SubscribeWith(ctx, rx.NewSubscriber(options...))
}

func (p proxy) SubscribeWith(ctx context.Context, actual rx.Subscriber) {
	var sub reactor.Subscriber
	if actual == rx.EmptySubscriber {
		sub = rx.EmptyRawSubscriber
	} else {
		sub = reactor.NewSubscriber(
			reactor.OnNext(func(v reactor.Any) error {
				return actual.OnNext(v.(payload.Payload))
			}),
			reactor.OnComplete(func() {
				actual.OnComplete()
			}),
			reactor.OnSubscribe(func(su reactor.Subscription) {
				actual.OnSubscribe(su)
			}),
			reactor.OnError(func(e error) {
				actual.OnError(e)
			}),
		)
	}
	p.Mono.SubscribeWith(ctx, sub)
}

func newProxy(source mono.Mono) proxy {
	return proxy{source}
}
