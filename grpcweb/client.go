package grpcweb

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/jhump/protoreflect/desc"
	"github.com/pkg/errors"
)

type ClientOption func(*Client)

type Client struct {
	host string

	tb  TransportBuilder
	stb StreamTransportBuilder
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

	if c.stb == nil {
		c.stb = DefaultStreamTransportBuilder
	}

	return c
}

func (c *Client) Unary(ctx context.Context, req *Request) error {
	return c.unary(ctx, req)
}

func (c *Client) unary(ctx context.Context, req *Request) error {
	b, err := proto.Marshal(req.in)
	if err != nil {
		return errors.Wrap(err, "failed to marshal the request body")
	}

	r, err := parseRequestBody(b)
	if err != nil {
		return errors.Wrap(err, "failed to build the request body")
	}

	res, err := c.tb(c.host, req).Send(ctx, r)
	if err != nil {
		return errors.Wrap(err, "failed to send the request")
	}
	defer res.Close()

	resBody, err := parseResponseBody(res, req.outDesc.GetFields())
	if err != nil {
		return errors.Wrap(err, "failed to build the response body")
	}

	if err := proto.Unmarshal(resBody, req.out); err != nil {
		return errors.Wrap(err, "failed to unmarshal response body")
	}

	return nil
}

type ServerStreamClient interface {
	Receive() (proto.Message, error)
}

type serverStreamClient struct {
	ctx context.Context
	t   Transport
	req *Request

	reqOnce   sync.Once
	resStream io.ReadCloser
}

func (c *serverStreamClient) Receive() (proto.Message, error) {
	var err error
	c.reqOnce.Do(func() {
		var b []byte
		b, err = proto.Marshal(c.req.in)
		if err != nil {
			return
		}

		var r io.Reader
		r, err = parseRequestBody(b)
		if err != nil {
			return
		}

		c.resStream, err = c.t.Send(c.ctx, r)
		if err != nil {
			return
		}
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to request server stream")
	}

	resBody, err := parseResponseBody(c.resStream, c.req.outDesc.GetFields())
	if err == io.EOF {
		return nil, err
	}

	if err != nil {
		return nil, errors.Wrap(err, "failed to build the response body")
	}

	// check compressed flag.
	// compressed flag is 0 or 1.
	if resBody[0]>>3 != 0 && resBody[0]>>3 != 1 {
		return nil, io.EOF
	}

	if err := proto.Unmarshal(resBody, c.req.out); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response body")
	}

	return c.req.out, nil
}

func (c *Client) ServerStreaming(ctx context.Context, req *Request) (ServerStreamClient, error) {
	return &serverStreamClient{
		ctx: ctx,
		t:   c.tb(c.host, req),
		req: req,
	}, nil
}

type ClientStreamClient interface {
	Send(proto.Message) error
	ReceiveAndClose() (proto.Message, error)
}

type clientStreamClient struct {
	ctx context.Context

	reqOnce sync.Once

	// curried StreamTransportBuilder
	stb func(req *Request) StreamTransport
	t   StreamTransport
	req *Request
}

func (c *clientStreamClient) Send(req *Request) error {
	c.reqOnce.Do(func() {
		c.t = c.stb(req)
		c.req = req
	})

	// TODO: refactoring
	b, err := proto.Marshal(req.in)
	if err != nil {
		return err
	}

	r, err := parseRequestBody(b)
	if err != nil {
		return err
	}

	return c.t.Send(r)
}

func (c *clientStreamClient) CloseAndReceive() (proto.Message, error) {
	res, err := c.t.CloseAndReceive()
	if err != nil {
		return nil, err
	}
	defer res.Close()

	resBody, err := parseResponseBody(res, c.req.outDesc.GetFields())
	if err != nil {
		return nil, err
	}

	if err := proto.Unmarshal(resBody, c.req.out); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response body")
	}

	return c.req.out, nil
}

func (c *Client) ClientStreaming(ctx context.Context) (ClientStreamClient, error) {
	return &clientStreamClient{
		ctx: ctx,
		stb: func(req *Request) StreamTransport {
			return c.stb(c.host, req.endpoint)
		},
	}, nil
}

type BidiStreamClient interface {
	Send(proto.Message) error
	Receive() (proto.Message, error)
	Close() error
}

type bidiStreamClient struct {
	ctx context.Context

	reqOnce sync.Once

	t   StreamTransport
	req *Request
}

func (c *bidiStreamClient) Send(req *Request) error {
	b, err := proto.Marshal(req.in)
	if err != nil {
		return err
	}

	r, err := parseRequestBody(b)
	if err != nil {
		return err
	}

	return c.t.Send(r)
}

func (c *bidiStreamClient) Recv() (proto.Message, error) {
	res, err := c.t.Receive()
	if err != nil {
		return nil, err
	}

	resBody, err := parseResponseBody(res, c.req.outDesc.GetFields())
	if err != nil {
		return nil, err
	}

	if err := proto.Unmarshal(resBody, c.req.out); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response body")
	}

	return nil, nil
}

func (c *bidiStreamClient) Close() error {
	res, err := c.t.CloseAndReceive()
	if err != nil {
		return err
	}
	io.Copy(os.Stdout, res)
	return nil
}

func (c *Client) BidiStreaming(ctx context.Context, endpoint string) (BidiStreamClient, error) {
	return &bidiStreamClient{
		ctx: ctx,
		t:   c.stb(c.host, endpoint),
	}, nil
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
func parseRequestBody(body []byte) (io.Reader, error) {
	buf := bytes.NewBuffer(make([]byte, 0, headerLen+len(body)))
	buf.Write(header(body))
	buf.Write(body)
	return buf, nil
}

// TODO: compressed message
// copied from rpc_util#parser.recvMsg
func parseResponseBody(resBody io.Reader, fields []*desc.FieldDescriptor) ([]byte, error) {
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

func WithTransportBuilder(b TransportBuilder) ClientOption {
	return func(c *Client) {
		c.tb = b
	}
}

func WithStreamTransportBuilder(b StreamTransportBuilder) ClientOption {
	return func(c *Client) {
		c.stb = b
	}
}
