package grpcweb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/k0kubun/pp"
	"github.com/pkg/errors"
)

type (
	TransportBuilder       func(host string, req *Request) Transport
	StreamTransportBuilder func(host string, req *Request) StreamTransport
)

var (
	DefaultTransportBuilder       TransportBuilder       = HTTPTransportBuilder
	DefaultStreamTransportBuilder StreamTransportBuilder = WebSocketTransportBuilder
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
	CloseAndReceive() (io.ReadCloser, error)
}

type WebSocketTransport struct {
	conn *websocket.Conn
	once sync.Once
}

func (t *WebSocketTransport) Send(body io.Reader) error {
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

	fmt.Printf("REQ: %x\n", b.Bytes())

	return t.conn.WriteMessage(websocket.BinaryMessage, b.Bytes())
}

func (t *WebSocketTransport) CloseAndReceive() (io.ReadCloser, error) {
	defer t.conn.Close()

	t.conn.WriteMessage(websocket.BinaryMessage, []byte{0x01})

	var buf bytes.Buffer

	// skip wire type and message content
	_, _, err := t.conn.ReadMessage()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	_, _, err = t.conn.ReadMessage()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	_, b, err := t.conn.ReadMessage()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}
	buf.Write(b)

	_, b, err = t.conn.ReadMessage()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}
	buf.Write(b)

	err = t.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		return nil, err
	}

	return ioutil.NopCloser(&buf), nil
}

func WebSocketTransportBuilder(host string, req *Request) StreamTransport {
	u := url.URL{Scheme: "ws", Host: host, Path: req.endpoint}
	pp.Println(u.String())
	h := http.Header{}
	h.Set("Sec-WebSocket-Protocol", "grpc-websockets")
	conn, res, err := websocket.DefaultDialer.Dial(u.String(), h)
	if err != nil {
		panic(err)
	}
	b, err := httputil.DumpResponse(res, false)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(b))
	return &WebSocketTransport{
		conn: conn,
	}
}
