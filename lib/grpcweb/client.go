package grpcweb

import (
	"context"

	"github.com/golang/protobuf/proto"
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
		return errors.Wrap(err, "failed to marshal request body")
	}
	return c.tb(c.host, req).Send(b)
}

func WithTransportBuilder(b TransportBuilder) ClientOption {
	return func(c *Client) {
		c.tb = b
	}
}
