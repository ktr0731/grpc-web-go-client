package grpcweb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

type (
	TransportBuilder       func(host string, req *Request) Transport
	StreamTransportBuilder func(host string, endpoint string) StreamTransport
)

var (
	DefaultTransportBuilder       TransportBuilder       = HTTPTransportBuilder
	DefaultStreamTransportBuilder StreamTransportBuilder = WebSocketTransportBuilder
)

var (
	ErrConnectionClosed = errors.New("connection closed")
)

// Transport creates new request.
// Transport is created only one per one request, MUST not use used transport again.
type Transport interface {
	Send(ctx context.Context, body io.Reader) (io.ReadCloser, error)
}

type HTTPTransport struct {
	sent bool

	host   string
	req    *Request
	client *http.Client

	insecure bool
}

func (t *HTTPTransport) Send(ctx context.Context, body io.Reader) (io.ReadCloser, error) {
	if t.sent {
		return nil, errors.New("Send must be called only one time per one Request")
	}
	defer func() {
		t.sent = true
	}()

	// TODO: insecure option
	protocol := "http"

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s://%s%s", protocol, t.host, t.req.endpoint), body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build the API request")
	}

	req.Header.Add("content-type", "application/grpc-web+proto")
	req.Header.Add("x-grpc-web", "1")

	res, err := t.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send the API")
	}

	return res.Body, nil
}

func HTTPTransportBuilder(host string, req *Request) Transport {
	return &HTTPTransport{
		host:   host,
		req:    req,
		client: &http.Client{},
	}
}

type StreamTransport interface {
	Send(body io.Reader) error
	Receive() (io.ReadCloser, error)

	// Finish sends EOF request to the server.
	Finish() (io.ReadCloser, error)

	// Close closes the connection.
	Close() error
}

type WebSocketTransport struct {
	conn *websocket.Conn

	once sync.Once

	m      sync.Mutex
	closed bool
}

func (t *WebSocketTransport) Send(body io.Reader) error {
	t.m.Lock()
	if t.closed {
		return ErrConnectionClosed
	}
	t.m.Unlock()

	t.once.Do(func() {
		h := http.Header{}
		h.Set("content-type", "application/grpc-web+proto")
		h.Set("x-grpc-web", "1")
		var b bytes.Buffer
		h.Write(&b)

		t.conn.WriteMessage(websocket.BinaryMessage, b.Bytes())
	})

	var b bytes.Buffer
	b.Write([]byte{0x00})
	_, err := io.Copy(&b, body)
	if err != nil {
		return errors.Wrap(err, "failed to read request body")
	}

	return t.conn.WriteMessage(websocket.BinaryMessage, b.Bytes())
}

func (t *WebSocketTransport) Receive() (res io.ReadCloser, err error) {
	t.m.Lock()
	if t.closed {
		return nil, ErrConnectionClosed
	}
	t.m.Unlock()

	defer func() {
		if err == nil {
			return
		}

		if berr, ok := errors.Cause(err).(*net.OpError); ok && !berr.Temporary() {
			err = ErrConnectionClosed
		}
	}()

	var buf bytes.Buffer

	// skip wire type and message content
	_, _, err = t.conn.ReadMessage()
	if err != nil {
		err = errors.Wrap(err, "failed to read response header")
		return
	}

	_, _, err = t.conn.ReadMessage()
	if err != nil {
		err = errors.Wrap(err, "failed to read response header")
		return
	}

	var b []byte
	_, b, err = t.conn.ReadMessage()
	if err != nil {
		err = errors.Wrap(err, "failed to read response body")
		return
	}
	buf.Write(b)

	var r io.Reader
	_, r, err = t.conn.NextReader()
	if err != nil {
		return
	}

	res = ioutil.NopCloser(io.MultiReader(&buf, r))

	return
}

func (t *WebSocketTransport) Finish() (io.ReadCloser, error) {
	defer t.conn.Close()

	t.conn.WriteMessage(websocket.BinaryMessage, []byte{0x01})

	res, err := t.Receive()
	if err != nil {
		return nil, err
	}

	err = t.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		return nil, err
	}

	return ioutil.NopCloser(res), nil
}

func (t *WebSocketTransport) Close() error {
	t.m.Lock()
	defer t.m.Unlock()
	t.closed = true
	return t.conn.Close()
}

func WebSocketTransportBuilder(host string, endpoint string) StreamTransport {
	u := url.URL{Scheme: "ws", Host: host, Path: endpoint}
	h := http.Header{}
	h.Set("Sec-WebSocket-Protocol", "grpc-websockets")
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), h)
	if err != nil {
		panic(err)
	}
	return &WebSocketTransport{
		conn: conn,
	}
}
