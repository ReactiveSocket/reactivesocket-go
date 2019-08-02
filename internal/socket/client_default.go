package socket

import (
	"context"
	"crypto/tls"

	"github.com/rsocket/rsocket-go/internal/transport"
	"github.com/rsocket/rsocket-go/logger"
)

type defaultClientSocket struct {
	*baseSocket
	uri *transport.URI
	tls *tls.Config
}

func (p *defaultClientSocket) Setup(ctx context.Context, setup *SetupInfo) (err error) {
	tp, err := p.uri.MakeClientTransport(p.tls)
	if err != nil {
		return
	}
	tp.Connection().SetCounter(p.socket.counter)
	tp.SetLifetime(setup.KeepaliveLifetime)

	p.socket.SetTransport(tp)

	go func(ctx context.Context, tp *transport.Transport) {
		if err := tp.Start(ctx); err != nil {
			logger.Warnf("client exit failed: %+v\n", err)
		}
		_ = p.Close()
	}(ctx, tp)

	go func(ctx context.Context) {
		_ = p.socket.loopWrite(ctx)
	}(ctx)
	setupFrame := setup.ToFrame()
	err = p.socket.tp.Send(setupFrame, true)
	return
}

// NewClient create a simple client-side socket.
func NewClient(uri *transport.URI, socket *DuplexRSocket, tc *tls.Config) ClientSocket {
	return &defaultClientSocket{
		baseSocket: newBaseSocket(socket),
		uri:        uri,
		tls:        tc,
	}
}
