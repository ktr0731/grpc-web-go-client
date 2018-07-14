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

func (c *Client) Send(ctx context.Context, req *Request) error {
	if req.m.GetClientStreaming() && req.m.GetServerStreaming() {
		// TODO
		panic("TODO")
		// return c.bidi()
	}
	if req.m.GetClientStreaming() {
		// TODO
		panic("TODO")
		// return c.client()
	}
	if req.m.GetServerStreaming() {
		// TODO
		panic("TODO")
		// return c.server()
	}
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

	res, err := c.tb(c.host, req).Send(r)
	if err != nil {
		return errors.Wrap(err, "failed to send the request")
	}

	resBody, err := c.parseResponseBody(res, req.outDesc.GetFields())
	if err != nil {
		return errors.Wrap(err, "failed to build the response body")
	}

	return proto.Unmarshal(resBody, req.out)
}

// header (compressed-flag(1) + message-length(4)) + body

// "req header:" []uint8{
//   0x00, 0x00, 0x00, 0x00, 0x05,
// }
func (c *Client) parseRequestBody(body []byte) (io.Reader, error) {
	bodyLen := int32(len(body))

	buf := bytes.NewBuffer(make([]byte, 0, 5+bodyLen))

	// write compressed flag (1 byte), body length (4 bytes, big endian), body

	buf.WriteByte(0x00)

	length := proto.EncodeVarint(uint64(bodyLen))
	tmp := bytes.NewBuffer(make([]byte, 4))
	binary.Write(tmp, binary.BigEndian, length)
	buf.Write(tmp.Bytes()[tmp.Len()-4:])

	buf.Write(body)

	return buf, nil
}

func (c *Client) parseResponseBody(resBody io.Reader, fields []*desc.FieldDescriptor) ([]byte, error) {
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

		// if f is string, then
		length := proto.EncodeVarint(uint64(len(content[2:])))
		resHeader = append(resHeader, length[len(length)-1])

		content = append(resHeader, content[2:]...)
	}

	return content, nil
}

func WithTransportBuilder(b TransportBuilder) ClientOption {
	return func(c *Client) {
		c.tb = b
	}
}
