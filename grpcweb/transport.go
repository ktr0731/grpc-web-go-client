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
	StreamTransportBuilder func(host string, endpoint string) (StreamTransport, error)
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
	Close() error
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

func (t *HTTPTransport) Close() error {
	t.client.CloseIdleConnections()
	return nil
}

func HTTPTransportBuilder(host string, req *Request) Transport {
	return &HTTPTransport{
		host:   host,
		req:    req,
		client: &http.Client{},
	}
}

// StreamTransport is used to send API requests for ClientStreamClient and BidiStreamClient.
type StreamTransport interface {
	Send(body io.Reader) error
	Receive() (io.ReadCloser, error)

	// CloseSend sends a close signal to the server.
	CloseSend() error

	// Close closes the connection.
	Close() error
}

// WebSocketTransport is a stream transport implementation.
//
// Currently, gRPC Web specification does not support client streaming. (https://github.com/improbable-eng/grpc-web#client-side-streaming)
// WebSocketTransport supports improbable-eng/grpc-web's own implementation.
//
// spec: https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-WEB.md
type WebSocketTransport struct {
	conn *websocket.Conn

	once    sync.Once
	resOnce sync.Once

	closed bool

	writeMu sync.Mutex
}

func (t *WebSocketTransport) Send(body io.Reader) error {
	if t.closed {
		return ErrConnectionClosed
	}

	t.once.Do(func() {
		h := http.Header{}
		h.Set("content-type", "application/grpc-web+proto")
		h.Set("x-grpc-web", "1")
		var b bytes.Buffer
		h.Write(&b)

		t.writeMessage(websocket.BinaryMessage, b.Bytes())
	})

	var b bytes.Buffer
	b.Write([]byte{0x00})
	_, err := io.Copy(&b, body)
	if err != nil {
		return errors.Wrap(err, "failed to read request body")
	}

	return t.writeMessage(websocket.BinaryMessage, b.Bytes())
}

func (t *WebSocketTransport) Receive() (res io.ReadCloser, err error) {
	if t.closed {
		return nil, ErrConnectionClosed
	}

	defer func() {
		if err == nil {
			return
		}

		if berr, ok := errors.Cause(err).(*net.OpError); ok && !berr.Temporary() {
			err = ErrConnectionClosed
		}
	}()

	// skip response header
	t.resOnce.Do(func() {
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
	})

	var buf bytes.Buffer
	var b []byte

	_, b, err = t.conn.ReadMessage()
	if err != nil {
		if cerr, ok := err.(*websocket.CloseError); ok {
			if cerr.Code == websocket.CloseNormalClosure {
				return nil, io.EOF
			}
		}
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

func (t *WebSocketTransport) CloseSend() error {
	// 0x01 means the finish send frame.
	// ref. transports/websocket/websocket.ts
	t.writeMessage(websocket.BinaryMessage, []byte{0x01})
	return nil
}

func (t *WebSocketTransport) Close() error {
	// Send the close message.
	err := t.writeMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		return err
	}
	t.closed = true
	// Close the WebSocket connection.
	return t.conn.Close()
}

func (t *WebSocketTransport) writeMessage(msg int, b []byte) error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	return t.conn.WriteMessage(msg, b)
}

func WebSocketTransportBuilder(host string, endpoint string) (StreamTransport, error) {
	u := url.URL{Scheme: "ws", Host: host, Path: endpoint}
	h := http.Header{}
	h.Set("Sec-WebSocket-Protocol", "grpc-websockets")
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), h)
	if err != nil {
		return nil, err
	}

	return &WebSocketTransport{
		conn: conn,
	}, nil
}
