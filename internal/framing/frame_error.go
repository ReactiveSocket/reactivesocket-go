package framing

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/rsocket/rsocket-go/internal/common"
)

const (
	errCodeLen       = 4
	errDataOff       = errCodeLen
	minErrorFrameLen = errCodeLen
)

// FrameError is error frame.
type FrameError struct {
	*BaseFrame
}

func (p *FrameError) String() string {
	return fmt.Sprintf("FrameError{%s,code=%s,data=%s}", p.header, p.ErrorCode(), p.ErrorData())
}

// Validate returns error if frame is invalid.
func (p *FrameError) Validate() (err error) {
	if p.body.Len() < minErrorFrameLen {
		err = errIncompleteFrame
	}
	return
}

func (p *FrameError) Error() string {
	bu := strings.Builder{}
	bu.WriteString(p.ErrorCode().String())
	bu.WriteByte(':')
	bu.WriteByte(' ')
	bu.Write(p.ErrorData())
	return bu.String()
}

// ErrorCode returns error code.
func (p *FrameError) ErrorCode() common.ErrorCode {
	v := binary.BigEndian.Uint32(p.body.Bytes())
	return common.ErrorCode(v)
}

// ErrorData returns error data bytes.
func (p *FrameError) ErrorData() []byte {
	return p.body.Bytes()[errDataOff:]
}

// NewFrameError returns a new error frame.
func NewFrameError(streamID uint32, code common.ErrorCode, data []byte) *FrameError {
	bf := common.NewByteBuff()
	var b4 [4]byte
	binary.BigEndian.PutUint32(b4[:], uint32(code))
	if _, err := bf.Write(b4[:]); err != nil {

		panic(err)
	}
	if _, err := bf.Write(data); err != nil {

		panic(err)
	}
	return &FrameError{
		NewBaseFrame(NewFrameHeader(streamID, FrameTypeError), bf),
	}
}
