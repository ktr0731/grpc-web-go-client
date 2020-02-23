package grpcweb

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"strings"

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

	r, err := parseRequestBody(codec, args)
	if err != nil {
		return errors.Wrap(err, "failed to build the request body")
	}

	tr := transport.NewUnary(c.host, nil)
	defer tr.Close()

	contentType := "application/grpc-web+" + codec.Name()
	header, rawBody, err := tr.Send(ctx, method, contentType, r)
	if err != nil {
		return errors.Wrap(err, "failed to send the request")
	}
	defer rawBody.Close()

	if callOptions.header != nil {
		*callOptions.header = header
	}

	trailer, resBody, err := parseResponseBody(rawBody)
	if err != nil {
		return errors.Wrap(err, "failed to parse the response body")
	}
	if callOptions.trailer != nil {
		*callOptions.trailer = trailer
	}

	if err := codec.Unmarshal(resBody, reply); err != nil {
		return errors.Wrapf(err, "failed to unmarshal response body by codec %s", codec.Name())
	}
	return nil
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
func parseResponseBody(resBody io.Reader) (metadata.MD, []byte, error) {
	var h [5]byte
	if _, err := resBody.Read(h[:]); err != nil {
		return nil, nil, err
	}

	length := binary.BigEndian.Uint32(h[1:])
	if length == 0 {
		return nil, nil, nil
	}

	// TODO: check message size

	content := make([]byte, int(length))
	if n, err := resBody.Read(content); err != nil {
		if err == io.EOF && int(n) != int(length) {
			err = io.ErrUnexpectedEOF
		}
		return nil, nil, err
	}

	// Trailer.

	_, err := resBody.Read(h[:])
	if err == io.EOF {
		return nil, content, nil
	}
	if err != nil {
		return nil, nil, err
	}

	length = binary.BigEndian.Uint32(h[1:])
	if length == 0 {
		return nil, nil, nil
	}

	var readLen uint32
	trailer := metadata.New(nil)
	s := bufio.NewScanner(resBody)
	for s.Scan() {
		readLen += uint32(len(s.Bytes()))
		if readLen > length {
			return nil, nil, io.ErrUnexpectedEOF
		}

		t := s.Text()
		i := strings.Index(t, ": ")
		if i == -1 {
			return nil, nil, io.ErrUnexpectedEOF
		}
		k := strings.ToLower(t[:i])
		if k == "grpc-status" || k == "grpc-message" { // Ignore non custom metadata.
			continue
		}
		trailer.Append(k, t[i+2:])
	}

	return trailer, content, nil
}
