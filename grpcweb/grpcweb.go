package grpcweb

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net/url"

	"github.com/ktr0731/grpc-web-go-client/grpcweb/transport"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
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
	callOpts := append(c.dialOptions.defaultCallOptions, opts...)
	callOptions := defaultCallOptions
	for _, o := range callOpts {
		o(&callOptions)
	}

	codec := callOptions.codec

	r, err := parseRequestBody(codec, args)
	if err != nil {
		return errors.Wrap(err, "failed to build the request body")
	}

	tr := transport.NewUnary()
	defer tr.Close()

	// TODO: HTTPS support.
	u := url.URL{Scheme: "http", Host: c.host, Path: method}

	contentType := "application/grpc-web+" + codec.Name()
	rawBody, err := tr.Send(ctx, u.String(), contentType, r)
	if err != nil {
		return errors.Wrap(err, "failed to send the request")
	}
	defer rawBody.Close()

	resBody, err := parseResponseBody(rawBody)
	if err != nil {
		return errors.Wrap(err, "failed to parse the response body")
	}

	if err := codec.Unmarshal(resBody, reply); err != nil {
		return errors.Wrapf(err, "failed to unmarshal response body by codec %s", codec.Name())
	}
	return nil
}

func (c *ClientConn) NewClientStram(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...CallOption) (ClientStream, error) {
	return nil, nil
}

func (c *ClientConn) NewServerStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...CallOption) (ServerStream, error) {
	return nil, nil
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
func parseRequestBody(codec encoding.Codec, in interface{}) (io.Reader, error) {
	body, err := codec.Marshal(in)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal the request body")
	}
	buf := bytes.NewBuffer(make([]byte, 0, headerLen+len(body)))
	buf.Write(header(body))
	buf.Write(body)
	return buf, nil
}

// copied from rpc_util#parser.recvMsg
// TODO: compressed message
func parseResponseBody(resBody io.Reader) ([]byte, error) {
	var h [5]byte
	if _, err := resBody.Read(h[:]); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(h[1:])
	if length == 0 {
		return nil, nil
	}

	// TODO: check message size

	content := make([]byte, int(length))
	if n, err := resBody.Read(content); err != nil {
		if err == io.EOF && int(n) != int(length) {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}

	return content, nil
}
