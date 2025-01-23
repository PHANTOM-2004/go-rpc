package codec

import "io"

type Header struct {
	ServiceMethod string
	SeqNum        uint64
	ErrMsg        string
}

type Codec interface {
	io.Closer
	ReadHeader(*Header) error
	ReadBody(any) error
	Write(*Header, any) error
}

type (
	Type         = string
	NewCodecFunc = func(io.ReadWriteCloser) Codec
)

const (
	GobType  Type = "application/gob"
	JsonType Type = "application/json"
)

var NewCodecFuncMap map[Type]NewCodecFunc

func init() {
	NewCodecFuncMap = make(map[string]NewCodecFunc)
	NewCodecFuncMap[GobType] = NewGobCodec
}
