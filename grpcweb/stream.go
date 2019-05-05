package grpcweb

import (
	"context"
	"io"

	"github.com/golang/protobuf/proto"
	"github.com/ktr0731/grpc-web-go-client/grpcweb/transport"
	"github.com/pkg/errors"
)

type ClientStream interface{}

type ServerStream interface {
	Send(ctx context.Context, req interface{}) error
	Receive(ctx context.Context, res interface{}) error
}

type serverStream struct {
	endpoint    string
	transport   transport.UnaryTransport
	resStream   io.ReadCloser
	callOptions *callOptions
}

func (s *serverStream) Send(ctx context.Context, req interface{}) error {
	codec := s.callOptions.codec

	r, err := parseRequestBody(codec, req)
	if err != nil {
		return errors.Wrap(err, "failed to build the request body")
	}

	contentType := "application/grpc-web+" + codec.Name()
	rawBody, err := s.transport.Send(ctx, s.endpoint, contentType, r)
	if err != nil {
		return errors.Wrap(err, "failed to send the request")
	}
	s.resStream = rawBody
	return nil
}

func (s *serverStream) Receive(ctx context.Context, res interface{}) (err error) {
	if s.resStream == nil {
		return errors.New("Receive must be call after calling Send")
	}
	defer func() {
		if err == io.EOF {
			if rerr := s.transport.Close(); rerr != nil {
				err = rerr
			}
			s.resStream.Close()
		}
	}()

	resBody, err := parseResponseBody(s.resStream)
	if err == io.EOF {
		return io.EOF
	}
	if err != nil {
		return errors.Wrap(err, "failed to parse the response body")
	}

	// check compressed flag.
	// compressed flag is 0 or 1.
	if resBody[0]>>3 != 0 && resBody[0]>>3 != 1 {
		return io.EOF
	}

	if err := s.callOptions.codec.Unmarshal(resBody, res); err != nil {
		return errors.Wrap(err, "failed to unmarshal response body")
	}
	return nil
}

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
