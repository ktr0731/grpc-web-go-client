package grpcweb

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"

	"github.com/ktr0731/grpc-web-go-client/grpcweb/parser"
	"github.com/ktr0731/grpc-web-go-client/grpcweb/transport"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/metadata"
)

type ClientConn struct {
	host        string
	dialOptions *dialOptions
}

func DialContext(host string, opts ...DialOption) (*ClientConn, error) {
	opt := defaultDialOptions
	for _, o := range opts {
		o(&opt)
	}
	return &ClientConn{
		host:        host,
		dialOptions: &opt,
	}, nil
}

func (c *ClientConn) Invoke(ctx context.Context, method string, args, reply interface{}, opts ...CallOption) error {
	callOptions := c.applyCallOptions(opts)
	codec := callOptions.codec

	tr := transport.NewUnary(c.host, nil)
	defer tr.Close()

	r, err := encodeRequestBody(codec, args)
	if err != nil {
		return errors.Wrap(err, "failed to build the request body")
	}

	md, ok := metadata.FromOutgoingContext(ctx)
	if ok {
		for k, v := range md {
			for _, vv := range v {
				tr.Header().Add(k, vv)
			}
		}
	}

	contentType := "application/grpc-web+" + codec.Name()
	header, rawBody, err := tr.Send(ctx, method, contentType, r)
	if err != nil {
		return errors.Wrap(err, "failed to send the request")
	}
	defer rawBody.Close()

	if status := header.Get("grpc-status"); status != "" && status != "0" {
		return fmt.Errorf("GRPC status %s: %s", status, header.Get("grpc-message"))
	}

	if contentType := header.Get("content-type"); contentType != "application/grpc-web+"+codec.Name() {
		return fmt.Errorf("unexpected content type %q", contentType)
	}

	if callOptions.header != nil {
		*callOptions.header = toMetadata(header)
	}

	resHeader, err := parser.ParseResponseHeader(rawBody)
	if err != nil {
		return errors.Wrap(err, "failed to parse response header")
	}

	if resHeader.IsMessageHeader() {
		resBody, err := parser.ParseLengthPrefixedMessage(rawBody, resHeader.ContentLength)
		if err != nil {
			return errors.Wrap(err, "failed to parse the response body")
		}
		if err := codec.Unmarshal(resBody, reply); err != nil {
			return errors.Wrapf(err, "failed to unmarshal response body by codec %s", codec.Name())
		}

		resHeader, err = parser.ParseResponseHeader(rawBody)
		if err != nil {
			return errors.Wrap(err, "failed to parse response header")
		}
	}
	if !resHeader.IsTrailerHeader() {
		return errors.New("unexpected header")
	}

	status, trailer, err := parser.ParseStatusAndTrailer(rawBody, resHeader.ContentLength)
	if err != nil {
		return errors.Wrap(err, "failed to parse status and trailer")
	}
	if callOptions.trailer != nil {
		*callOptions.trailer = trailer
	}

	return status.Err()
}

func (c *ClientConn) NewClientStream(desc *grpc.StreamDesc, method string, opts ...CallOption) (ClientStream, error) {
	if !desc.ClientStreams {
		return nil, errors.New("not a client stream RPC")
	}
	tr, err := transport.NewClientStream(c.host, method)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a new transport stream")
	}
	return &clientStream{
		endpoint:    method,
		transport:   tr,
		callOptions: c.applyCallOptions(opts),
	}, nil
}

func (c *ClientConn) NewServerStream(desc *grpc.StreamDesc, method string, opts ...CallOption) (ServerStream, error) {
	if !desc.ServerStreams {
		return nil, errors.New("not a server stream RPC")
	}
	return &serverStream{
		endpoint:    method,
		transport:   transport.NewUnary(c.host, nil),
		callOptions: c.applyCallOptions(opts),
	}, nil
}

func (c *ClientConn) NewBidiStream(desc *grpc.StreamDesc, method string, opts ...CallOption) (BidiStream, error) {
	if !desc.ServerStreams || !desc.ClientStreams {
		return nil, errors.New("not a bidi stream RPC")
	}
	stream, err := c.NewClientStream(desc, method, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a new client stream")
	}
	return &bidiStream{
		clientStream: stream.(*clientStream),
	}, nil
}

func (c *ClientConn) applyCallOptions(opts []CallOption) *callOptions {
	callOpts := append(c.dialOptions.defaultCallOptions, opts...)
	callOptions := defaultCallOptions
	for _, o := range callOpts {
		o(&callOptions)
	}
	return &callOptions
}

// copied from rpc_util.go#msgHeader
const headerLen = 5

func header(body []byte) []byte {
	h := make([]byte, 5)
	h[0] = byte(0)
	binary.BigEndian.PutUint32(h[1:], uint32(len(body)))
	return h
}

// header (compressed-flag(1) + message-length(4)) + body
// TODO: compressed message
func encodeRequestBody(codec encoding.Codec, in interface{}) (io.Reader, error) {
	body, err := codec.Marshal(in)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal the request body")
	}
	buf := bytes.NewBuffer(make([]byte, 0, headerLen+len(body)))
	buf.Write(header(body))
	buf.Write(body)
	return buf, nil
}

func toMetadata(h http.Header) metadata.MD {
	if len(h) == 0 {
		return nil
	}
	md := metadata.New(nil)
	for k, v := range h {
		md.Append(k, v...)
	}
	return md
}
