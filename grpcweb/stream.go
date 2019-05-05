package grpcweb

import (
	"context"

	"github.com/golang/protobuf/proto"
)

type ClientStream interface{}

type ServerStream interface{}

// TODO: remove it

type Request struct {
	endpoint string
	in, out  proto.Message
}
type Response struct {
	ContentType string
	Content     interface{}
}
type BidiStreamClient interface {
	Send(*Request) error
	Receive() (*Response, error)
	CloseSend() error
}

func (c *ClientConn) BidiStreaming(context.Context, *Request) (BidiStreamClient, error) {
	return nil, nil
}

func NewRequest(
	endpoint string,
	in interface{},
	out interface{},
) *Request {
	panic("remove")
	return nil
}
