package grpcweb

import (
	"context"
	"errors"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
)

type ClientOption func(*Client)

type Client struct {
	m         *descriptor.MethodDescriptorProto
	transport Transport

	sent bool
}

func NewClient(method *descriptor.MethodDescriptorProto, opts ...ClientOption) *Client {
	c := &Client{
		m: method,
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.transport == nil {
		c.transport = DefaultTransportBuilder(c)
	}

	return c
}

func (c *Client) Send(ctx context.Context, req, res proto.Message) error {
	if c.sent {
		return errors.New("Send must be called only one time per one API request")
	}

	defer func() {
		c.sent = true
	}()

	if c.m.GetClientStreaming() && c.m.GetServerStreaming() {
		// TODO
		// return c.bidi()
	}
	if c.m.GetClientStreaming() {
		// TODO
		// return c.client()
	}
	if c.m.GetServerStreaming() {
		// TODO
		// return c.server()
	}
	return c.unary(ctx, req, res)
}

func (c *Client) unary(ctx context.Context, req, res proto.Message) error {
	proto.Marshal(req)
	return nil
}

func WithTransportBuilder(b TransportBuilder) ClientOption {
	return func(c *Client) {
		c.transport = b(c)
	}
}
