package grpcweb

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"google.golang.org/grpc/encoding"
	pb "google.golang.org/grpc/encoding/proto"
)

type ClientOption func(*Client)

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

func WithCodec(codec encoding.Codec) ClientOption {
	return func(c *Client) {
		c.codec = codec
	}
}

// Client starts each API session.
type Client struct {
	host string

	tb    TransportBuilder
	stb   StreamTransportBuilder
	codec encoding.Codec
}

// NewClient instantiates new API client for a gRPC Web API server.
// Client accepts some options to configure transports, codec, and so on.
// The default codec is Protocol Buffers.
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

	if c.codec == nil {
		// use Protocol Buffers as a default codec.
		c.codec = encoding.GetCodec(pb.Name)
	}

	return c
}

// Unary sends an unary request. (also known as simple request)
func (c *Client) Unary(ctx context.Context, req *Request) (*Response, error) {
	r, err := parseRequestBody(c.codec, req.in)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build the request body")
	}

	rawBody, err := c.tb(c.host, req).Send(ctx, r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send the request")
	}
	defer rawBody.Close()

	resBody, err := parseResponseBody(rawBody)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build the response body")
	}

	if err := c.codec.Unmarshal(resBody, req.out); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal response body by codec %s", c.codec.Name())
	}

	return &Response{
		ContentType: c.codec.Name(),
		Content:     req.out,
	}, nil
}

type ServerStreamClient interface {
	Receive() (*Response, error)
}

type serverStreamClient struct {
	ctx context.Context
	t   Transport
	req *Request

	resStream io.ReadCloser

	codec encoding.Codec
}

// Receive receives multi responses through a stream.
// Receive returns io.EOF at the end.
func (c *serverStreamClient) Receive() (*Response, error) {
	resBody, err := parseResponseBody(c.resStream)
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

	contentProto := proto.Clone(c.req.out)
	if err := c.codec.Unmarshal(resBody, contentProto); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response body")
	}

	return &Response{
		ContentType: c.codec.Name(),
		Content:     contentProto,
	}, nil
}

// ServerStreamClient sends only one request and receives multi responses through a stream.
func (c *Client) ServerStreaming(ctx context.Context, req *Request) (ServerStreamClient, error) {
	t := c.tb(c.host, req)

	r, err := parseRequestBody(c.codec, req.in)
	if err != nil {
		return nil, err
	}

	resStream, err := t.Send(ctx, r)
	if err != nil {
		return nil, err
	}

	return &serverStreamClient{
		ctx:       ctx,
		t:         t,
		req:       req,
		resStream: resStream,
		codec:     c.codec,
	}, nil
}

// ClientStreamClient sends multi requests and receives only one response.
// At the end, ClientStreamClient must be call CloseAndReceive method.
type ClientStreamClient interface {
	Send(*Request) error
	CloseAndReceive() (*Response, error)
}

type clientStreamClient struct {
	ctx context.Context

	reqOnce sync.Once

	// curried StreamTransportBuilder
	stb func(req *Request) (StreamTransport, error)
	t   StreamTransport
	req *Request

	codec encoding.Codec
}

func (c *clientStreamClient) Send(req *Request) error {
	var err error
	c.reqOnce.Do(func() {
		c.t, err = c.stb(req)
		c.req = req
	})
	if err != nil {
		return err
	}

	r, err := parseRequestBody(c.codec, req.in)
	if err != nil {
		return err
	}

	return c.t.Send(r)
}

func (c *clientStreamClient) CloseAndReceive() (*Response, error) {
	res, err := c.t.Finish()
	if err != nil {
		return nil, err
	}
	defer res.Close()

	resBody, err := parseResponseBody(res)
	if err != nil {
		return nil, err
	}

	if err := c.codec.Unmarshal(resBody, c.req.out); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response body")
	}

	return &Response{
		ContentType: c.codec.Name(),
		Content:     c.req.out,
	}, nil
}

// ClientStreamClient sends multi requests and receives only one response.
func (c *Client) ClientStreaming(ctx context.Context) (ClientStreamClient, error) {
	return &clientStreamClient{
		ctx: ctx,
		stb: func(req *Request) (StreamTransport, error) {
			return c.stb(c.host, req.endpoint)
		},
		codec: c.codec,
	}, nil
}

// BidiStreamClient sends multi requests and receives multi responses.
// At the end, BidiStreamClient must be call Close method.
type BidiStreamClient interface {
	Send(*Request) error
	Receive() (*Response, error)
	Close() error
}

type bidiStreamClient struct {
	ctx context.Context

	t StreamTransport

	req *Request

	codec encoding.Codec
}

func (c *bidiStreamClient) Send(req *Request) error {
	r, err := parseRequestBody(c.codec, req.in)
	if err != nil {
		return err
	}

	return c.t.Send(r)
}

func (c *bidiStreamClient) Receive() (*Response, error) {
	res, err := c.t.Receive()
	if err != nil {
		return nil, err
	}

	resBody, err := parseResponseBody(res)
	if err != nil {
		return nil, err
	}

	contentProto := proto.Clone(c.req.out)
	if err := c.codec.Unmarshal(resBody, contentProto); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response body")
	}

	return &Response{
		ContentType: c.codec.Name(),
		Content:     contentProto,
	}, nil
}

func (c *bidiStreamClient) Close() error {
	return c.t.Close()
}

// BidiStreamClient instantiates bidirectional streaming client.
func (c *Client) BidiStreaming(ctx context.Context, req *Request) (BidiStreamClient, error) {
	t, err := c.stb(c.host, req.endpoint)
	if err != nil {
		return nil, err
	}
	return &bidiStreamClient{
		ctx:   ctx,
		t:     t,
		req:   req,
		codec: c.codec,
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
func parseRequestBody(codec encoding.Codec, in proto.Message) (io.Reader, error) {
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
