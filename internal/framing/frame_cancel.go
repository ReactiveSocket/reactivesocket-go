package framing

import (
	"fmt"

	"github.com/rsocket/rsocket-go/internal/common"
)

// FrameCancel is frame of cancel.
type FrameCancel struct {
	*BaseFrame
}

// Validate returns error if frame is invalid.
func (p *FrameCancel) Validate() (err error) {
	// Cancel frame doesn't need any binary body.
	if p.body != nil && p.body.Len() > 0 {
		err = errIncompleteFrame
	}
	return
}

func (p *FrameCancel) String() string {
	return fmt.Sprintf("FrameCancel{%s}", p.header)
}

// NewFrameCancel returns a new cancel frame.
func NewFrameCancel(sid uint32) *FrameCancel {
	return &FrameCancel{
		NewBaseFrame(NewFrameHeader(sid, FrameTypeCancel), common.NewByteBuff()),
	}
}
