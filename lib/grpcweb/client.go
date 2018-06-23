package grpcweb

import (
	"context"

	"github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
)

type ClientOption func(*Client)

type Client struct {
	m         *descriptor.MethodDescriptorProto
	transport Transport
}

func NewClient(method *descriptor.MethodDescriptorProto, opts ...ClientOption) *Client {
	c := &Client{
		m: method,
	}

	return c
}

func (c *Client) Send(ctx context.Context, req, res interface{}) error {
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

func (c *Client) unary(ctx context.Context, req, res interface{}) error {
	return nil
}

func WithTransportBuilder(b TransportBuilder) ClientOption {
	return func(c *Client) {
		c.transport = b(c)
	}
}
