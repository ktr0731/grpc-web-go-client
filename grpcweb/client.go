package grpcweb

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"

	"github.com/golang/protobuf/proto"
	"github.com/jhump/protoreflect/desc"
	"github.com/pkg/errors"
)

type ClientOption func(*Client)

type Client struct {
	host string

	tb TransportBuilder
}

func NewClient(host string, opts ...ClientOption) *Client {
	c := &Client{
		host: host,
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.tb == nil {
		c.tb = DefaultTransportBuilder
	}

	return c
}

func (c *Client) Unary(ctx context.Context, req *Request) error {
	// if req.m.GetClientStreaming() && req.m.GetServerStreaming() {
	// 	// TODO
	// 	panic("TODO")
	// 	// return c.bidi()
	// }
	// if req.m.GetClientStreaming() {
	// 	// TODO
	// 	panic("TODO")
	// 	// return c.client()
	// }
	// if req.m.GetServerStreaming() {
	// 	// TODO
	// 	panic("TODO")
	// 	// return c.server()
	// }
	return c.unary(ctx, req)
}

func (c *Client) unary(ctx context.Context, req *Request) error {
	b, err := proto.Marshal(req.in)
	if err != nil {
		return errors.Wrap(err, "failed to marshal the request body")
	}

	r, err := c.parseRequestBody(b)
	if err != nil {
		return errors.Wrap(err, "failed to build the request body")
	}

	res, err := c.tb(c.host, req).Send(ctx, r)
	if err != nil {
		return errors.Wrap(err, "failed to send the request")
	}

	resBody, err := c.parseResponseBody(res, req.outDesc.GetFields())
	if err != nil {
		return errors.Wrap(err, "failed to build the response body")
	}

	if err := proto.Unmarshal(resBody, req.out); err != nil {
		return errors.Wrap(err, "failed to unmarshal response body")
	}

	return nil
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
func (c *Client) parseRequestBody(body []byte) (io.Reader, error) {
	buf := bytes.NewBuffer(make([]byte, 0, headerLen+len(body)))
	buf.Write(header(body))
	buf.Write(body)
	return buf, nil
}

// TODO: compressed message
// copied from rpc_util#parser.recvMsg
func (c *Client) parseResponseBody(resBody io.Reader, fields []*desc.FieldDescriptor) ([]byte, error) {
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
	if _, err := resBody.Read(content); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}

	return content, nil
}

func WithTransportBuilder(b TransportBuilder) ClientOption {
	return func(c *Client) {
		c.tb = b
	}
}
