package transport

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rsocket/rsocket-go/internal/common"
	"github.com/rsocket/rsocket-go/internal/framing"
	"github.com/rsocket/rsocket-go/logger"
)

type (
	// FrameHandler is an alias of frame handler.
	FrameHandler = func(frame framing.Frame) (err error)
	// ServerTransportAcceptor is an alias of server transport handler.
	ServerTransportAcceptor = func(ctx context.Context, tp *Transport)
)

var errTransportClosed = errors.New("transport closed")

// ServerTransport is server-side RSocket transport.
type ServerTransport interface {
	io.Closer
	// Accept register incoming connection handler.
	Accept(acceptor ServerTransportAcceptor)
	// Listen listens on the network address addr and handles requests on incoming connections.
	// You can specify onReady handler, it'll be invoked when server begin listening.
	// It always returns a non-nil error.
	Listen(ctx context.Context, notifier chan<- struct{}) error
}

// Transport is RSocket transport which is used to carry RSocket frames.
type Transport struct {
	conn        Conn
	maxLifetime time.Duration
	lastRcvPos  uint64
	once        sync.Once

	hSetup           FrameHandler
	hResume          FrameHandler
	hLease           FrameHandler
	hResumeOK        FrameHandler
	hFireAndForget   FrameHandler
	hMetadataPush    FrameHandler
	hRequestResponse FrameHandler
	hRequestStream   FrameHandler
	hRequestChannel  FrameHandler
	hPayload         FrameHandler
	hRequestN        FrameHandler
	hError           FrameHandler
	hError0          FrameHandler
	hCancel          FrameHandler
	hKeepalive       FrameHandler
}

// HandleDisaster registers handler when receiving frame of DISASTER Error with zero StreamID.
func (p *Transport) HandleDisaster(handler FrameHandler) {
	p.hError0 = handler
}

// Connection returns current connection.
func (p *Transport) Connection() Conn {
	return p.conn
}

// SetLifetime set max lifetime for current transport.
func (p *Transport) SetLifetime(lifetime time.Duration) {
	if lifetime < 1 {
		return
	}
	p.maxLifetime = lifetime
}

// Send send a frame.
func (p *Transport) Send(frame framing.FrameSupport, flush bool) (err error) {
	defer func() {
		// ensure frame done when send success.
		if err == nil {
			frame.Done()
		}
	}()
	if p == nil || p.conn == nil {
		err = errTransportClosed
		return
	}
	err = p.conn.Write(frame)
	if err != nil {
		return
	}
	if !flush {
		return
	}
	err = p.conn.Flush()
	return
}

// Flush flush all bytes in current connection.
func (p *Transport) Flush() (err error) {
	if p == nil || p.conn == nil {
		err = errTransportClosed
		return
	}
	err = p.conn.Flush()
	return
}

// Close close current transport.
func (p *Transport) Close() (err error) {
	p.once.Do(func() {
		err = p.conn.Close()
	})
	return
}

// ReadFirst reads first frame.
func (p *Transport) ReadFirst(ctx context.Context) (frame framing.Frame, err error) {
	select {
	case <-ctx.Done():
		err = ctx.Err()
	default:
		frame, err = p.conn.Read()
		if err != nil {
			err = errors.Wrap(err, "read first frame failed")
		}
	}
	if err != nil {
		_ = p.Close()
	}
	return
}

// Start start transport.
func (p *Transport) Start(ctx context.Context) (err error) {
	defer func() {
		_ = p.Close()
	}()
L:
	for {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			return
		default:
			f, err := p.conn.Read()
			if err != nil {
				break L
			}
			err = p.DispatchFrame(ctx, f)
			if err != nil {
				break L
			}
		}
	}
	if err == io.EOF {
		err = nil
		return
	}
	if err != nil {
		err = errors.Wrap(err, "read and delivery frame failed")
	}
	return
}

// HandleSetup registers handler when receiving a frame of Setup.
func (p *Transport) HandleSetup(handler FrameHandler) {
	p.hSetup = handler
}

// HandleResume registers handler when receiving a frame of Resume.
func (p *Transport) HandleResume(handler FrameHandler) {
	p.hResume = handler
}

func (p *Transport) HandleLease(handler FrameHandler) {
	p.hLease = handler
}

// HandleResumeOK registers handler when receiving a frame of ResumeOK.
func (p *Transport) HandleResumeOK(handler FrameHandler) {
	p.hResumeOK = handler
}

// HandleFNF registers handler when receiving a frame of FireAndForget.
func (p *Transport) HandleFNF(handler FrameHandler) {
	p.hFireAndForget = handler
}

// HandleMetadataPush registers handler when receiving a frame of MetadataPush.
func (p *Transport) HandleMetadataPush(handler FrameHandler) {
	p.hMetadataPush = handler
}

// HandleRequestResponse registers handler when receiving a frame of RequestResponse.
func (p *Transport) HandleRequestResponse(handler FrameHandler) {
	p.hRequestResponse = handler
}

// HandleRequestStream registers handler when receiving a frame of RequestStream.
func (p *Transport) HandleRequestStream(handler FrameHandler) {
	p.hRequestStream = handler
}

// HandleRequestChannel registers handler when receiving a frame of RequestChannel.
func (p *Transport) HandleRequestChannel(handler FrameHandler) {
	p.hRequestChannel = handler
}

// HandlePayload registers handler when receiving a frame of Payload.
func (p *Transport) HandlePayload(handler FrameHandler) {
	p.hPayload = handler
}

// HandleRequestN registers handler when receiving a frame of RequestN.
func (p *Transport) HandleRequestN(handler FrameHandler) {
	p.hRequestN = handler
}

// HandleError registers handler when receiving a frame of Error.
func (p *Transport) HandleError(handler FrameHandler) {
	p.hError = handler
}

// HandleCancel registers handler when receiving a frame of Cancel.
func (p *Transport) HandleCancel(handler FrameHandler) {
	p.hCancel = handler
}

// HandleKeepalive registers handler when receiving a frame of Keepalive.
func (p *Transport) HandleKeepalive(handler FrameHandler) {
	p.hKeepalive = handler
}

// DispatchFrame delivery incoming frames.
func (p *Transport) DispatchFrame(_ context.Context, frame framing.Frame) (err error) {
	header := frame.Header()
	t := header.Type()
	sid := header.StreamID()

	var handler FrameHandler

	switch t {
	case framing.FrameTypeSetup:
		p.maxLifetime = frame.(*framing.SetupFrame).MaxLifetime()
		handler = p.hSetup
	case framing.FrameTypeResume:
		handler = p.hResume
	case framing.FrameTypeResumeOK:
		p.lastRcvPos = frame.(*framing.ResumeOKFrame).LastReceivedClientPosition()
		handler = p.hResumeOK
	case framing.FrameTypeRequestFNF:
		handler = p.hFireAndForget
	case framing.FrameTypeMetadataPush:
		if sid != 0 {
			// skip invalid metadata push
			logger.Warnf("rsocket.Transport: omit MetadataPush with non-zero stream id %d\n", sid)
			return
		}
		handler = p.hMetadataPush
	case framing.FrameTypeRequestResponse:
		handler = p.hRequestResponse
	case framing.FrameTypeRequestStream:
		handler = p.hRequestStream
	case framing.FrameTypeRequestChannel:
		handler = p.hRequestChannel
	case framing.FrameTypePayload:
		handler = p.hPayload
	case framing.FrameTypeRequestN:
		handler = p.hRequestN
	case framing.FrameTypeError:
		if sid == 0 {
			err = errors.New(frame.(*framing.ErrorFrame).Error())
			if p.hError0 != nil {
				_ = p.hError0(frame)
			}
			return
		}
		handler = p.hError
	case framing.FrameTypeCancel:
		handler = p.hCancel
	case framing.FrameTypeKeepalive:
		ka := frame.(*framing.KeepaliveFrame)
		p.lastRcvPos = ka.LastReceivedPosition()
		handler = p.hKeepalive
	case framing.FrameTypeLease:
		handler = p.hLease
	}

	// Set deadline.
	deadline := time.Now().Add(p.maxLifetime)
	err = p.conn.SetDeadline(deadline)
	if err != nil {
		return
	}

	// missing handler
	if handler == nil {
		err = errors.Errorf("missing frame handler: type=%s", t)
		return
	}

	// trigger handler
	err = handler(frame)
	if err != nil {
		err = errors.Wrap(err, "exec frame handler failed")
	}
	return
}

func newTransportClient(c Conn) *Transport {
	return &Transport{
		conn:        c,
		maxLifetime: common.DefaultKeepaliveMaxLifetime,
	}
}
