package framing

import (
	"io"

	"github.com/rsocket/rsocket-go/core"
)

// CancelFrame is frame of cancel.
type CancelFrame struct {
	*RawFrame
}

type CancelFrameSupport struct {
	*tinyFrame
}

func (c CancelFrameSupport) WriteTo(w io.Writer) (n int64, err error) {
	var wrote int64
	wrote, err = c.header.WriteTo(w)
	if err != nil {
		return
	}
	n += wrote
	return
}

func (c CancelFrameSupport) Len() int {
	return core.FrameHeaderLen
}

// Validate returns error if frame is invalid.
func (f *CancelFrame) Validate() (err error) {
	// Cancel frame doesn't need any binary body.
	if f.body != nil && f.body.Len() > 0 {
		err = errIncompleteFrame
	}
	return
}

func NewCancelFrameSupport(id uint32) *CancelFrameSupport {
	h := core.NewFrameHeader(id, core.FrameTypeCancel, 0)
	return &CancelFrameSupport{
		tinyFrame: newTinyFrame(h),
	}
}

// NewCancelFrame creates cancel frame.
func NewCancelFrame(sid uint32) *CancelFrame {
	return &CancelFrame{
		NewRawFrame(core.NewFrameHeader(sid, core.FrameTypeCancel, 0), nil),
	}
}