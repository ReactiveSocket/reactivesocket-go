package mono_test

import (
	"context"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/jjeffcaii/reactor-go/scheduler"
	"github.com/pkg/errors"
	"github.com/rsocket/rsocket-go/payload"
	"github.com/rsocket/rsocket-go/rx"
	. "github.com/rsocket/rsocket-go/rx/mono"
	"github.com/stretchr/testify/assert"
	"go.uber.org/atomic"
)

func TestProxy_Error(t *testing.T) {
	originErr := errors.New("error testing")
	errCount := atomic.NewInt32(0)
	_, err := Error(originErr).
		DoOnError(func(e error) {
			assert.Equal(t, originErr, e, "bad error")
			errCount.Inc()
		}).
		Block(context.Background())
	assert.Error(t, err, "should got error")
	assert.Equal(t, originErr, err, "bad blocked error")
	assert.Equal(t, int32(1), errCount.Load(), "error count should be 1")
}

func TestEmpty(t *testing.T) {
	res, err := Empty().Block(context.Background())
	assert.NoError(t, err, "an error occurred")
	assert.Nil(t, res, "result should be nil")
}

func TestJustOrEmpty(t *testing.T) {
	// Give normal payload
	res, err := JustOrEmpty(payload.NewString("hello", "world")).Block(context.Background())
	assert.NoError(t, err, "an error occurred")
	assert.NotNil(t, res, "result should not be nil")
	// Give nil payload
	res, err = JustOrEmpty(nil).Block(context.Background())
	assert.NoError(t, err, "an error occurred")
	assert.Nil(t, res, "result should be nil")
}

func TestJust(t *testing.T) {
	Just(payload.NewString("hello", "world")).
		Subscribe(context.Background(), rx.OnNext(func(i payload.Payload) {
			log.Println("next:", i)
		}))
}

func TestMono_Raw(t *testing.T) {

	Just(payload.NewString("hello", "world")).Raw()

}

func TestProxy_SubscribeOn(t *testing.T) {
	v, err := Create(func(i context.Context, sink Sink) {
		time.AfterFunc(time.Second, func() {
			sink.Success(payload.NewString("foo", "bar"))
		})
	}).
		SubscribeOn(scheduler.Parallel()).
		DoOnSuccess(func(i payload.Payload) {
			log.Println("success:", i)
		}).
		Block(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "foo", v.DataUTF8(), "bad data result")
	m, _ := v.MetadataUTF8()
	assert.Equal(t, "bar", m, "bad metadata result")
}

func TestProxy_Block(t *testing.T) {
	v, err := Just(payload.NewString("hello", "world")).Block(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "hello", v.DataUTF8())
	m, _ := v.MetadataUTF8()
	assert.Equal(t, "world", m)

}

func TestProcessor(t *testing.T) {
	p := CreateProcessor()
	time.AfterFunc(3*time.Second, func() {
		p.Success(payload.NewString("hello", "world"))
	})
	v, err := p.Block(context.Background())
	assert.NoError(t, err)
	log.Println("block:", v)
}

func TestProxy_Filter(t *testing.T) {
	Just(payload.NewString("hello", "world")).
		Filter(func(i payload.Payload) bool {
			return strings.EqualFold("hello_no", i.DataUTF8())
		}).
		DoOnSuccess(func(i payload.Payload) {
			assert.Fail(t, "should never run here")
		}).
		DoFinally(func(i rx.SignalType) {
			log.Println("finally:", i)
		}).
		Subscribe(context.Background())
}

func TestCreate(t *testing.T) {
	Create(func(i context.Context, sink Sink) {
		sink.Success(payload.NewString("hello", "world"))
	}).
		DoOnSuccess(func(i payload.Payload) {
			log.Println("doOnNext:", i)
		}).
		DoFinally(func(s rx.SignalType) {
			log.Println("doFinally:", s)
		}).
		Subscribe(context.Background(), rx.OnNext(func(i payload.Payload) {
			log.Println("next:", i)
		}))

	Create(func(i context.Context, sink Sink) {
		sink.Error(errors.New("foobar"))
	}).
		DoOnError(func(e error) {
			assert.Equal(t, "foobar", e.Error(), "bad error")
		}).
		DoOnSuccess(func(i payload.Payload) {
			assert.Fail(t, "should never run here")
		}).
		Subscribe(context.Background())
}
