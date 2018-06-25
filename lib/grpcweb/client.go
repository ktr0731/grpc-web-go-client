package grpcweb

import (
	"bytes"
	"context"
	"io"

	"github.com/golang/protobuf/proto"
	"github.com/jhump/protoreflect/desc"
	"github.com/k0kubun/pp"
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

func (c *Client) Send(ctx context.Context, req *Request) error {
	if req.m.GetClientStreaming() && req.m.GetServerStreaming() {
		// TODO
		// return c.bidi()
	}
	if req.m.GetClientStreaming() {
		// TODO
		// return c.client()
	}
	if req.m.GetServerStreaming() {
		// TODO
		// return c.server()
	}
	return c.unary(ctx, req)
}

func (c *Client) unary(ctx context.Context, req *Request) error {
	b, err := proto.Marshal(req.in)
	if err != nil {
		return errors.Wrap(err, "failed to marshal the request body")
	}

	r, err := c.buildRequestBody(b)
	if err != nil {
		return errors.Wrap(err, "failed to build the request body")
	}

	res, err := c.tb(c.host, req).Send(r)
	if err != nil {
		return errors.Wrap(err, "failed to send the request")
	}

	resBody, err := c.buildResponseBody(res, req.outDesc.GetFields())
	if err != nil {
		return errors.Wrap(err, "failed to build the response body")
	}

	return proto.Unmarshal(resBody, req.out)
}

// header (compressed-flag(1) + message-length(4)) + body
func (c *Client) buildRequestBody(body []byte) (io.Reader, error) {
	bodyLen := len(body)

	h := make([]byte, 0, 5+bodyLen)
	len, err := toBytes(int32(bodyLen))
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert body length to []byte")
	}

	h = append([]byte{0x00}, len...)

	return bytes.NewReader(append(h, body...)), nil
}

func (c *Client) buildResponseBody(resBody io.Reader, fields []*desc.FieldDescriptor) ([]byte, error) {
	var header [5]byte
	if _, err := resBody.Read(header[:]); err != nil {
		return nil, err
	}
	content := make([]byte, int(header[4]))
	if _, err := resBody.Read(content); err != nil {
		return nil, err
	}

	for _, f := range fields {
		h, err := buildFieldHeader(f)
		if err != nil {
			return nil, err
		}
		resHeader := []byte{h}
		pp.Println("header:", resHeader)

		// if f is string, then
		length, err := toBytes(int32(len(content[2:])))
		if err != nil {
			return nil, err
		}
		resHeader = append(resHeader, length[len(length)-1])

		content = append(resHeader, content[2:]...)
	}
	pp.Println(content)

	return content, nil
}

func WithTransportBuilder(b TransportBuilder) ClientOption {
	return func(c *Client) {
		c.tb = b
	}
}
