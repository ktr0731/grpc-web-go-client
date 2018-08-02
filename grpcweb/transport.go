package grpcweb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
	"github.com/k0kubun/pp"
	"github.com/pkg/errors"
)

type TransportBuilders struct {
	Normal, Stream TransportBuilder
}

type TransportBuilder func(host string, req *Request) Transport

var DefaultTransportBuilder TransportBuilder = HTTPTransportBuilder

var DefaultTransportBuilders = &TransportBuilders{
	Normal: DefaultTransportBuilder,
	Stream: DefaultTransportBuilder,
}

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
}

func (t *WebSocketTransport) Send(body io.Reader) error {
	var b bytes.Buffer
	_, err := io.Copy(&b, body)
	if err != nil {
		return errors.Wrap(err, "failed to read request body")
	}
	return t.conn.WriteMessage(websocket.TextMessage, b.Bytes())
}

func (t *WebSocketTransport) CloseAndReceive() (io.ReadCloser, error) {
	_, b, err := t.conn.ReadMessage()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}
	return ioutil.NopCloser(bytes.NewBuffer(b)), nil
}

func WebSocketTransportBuilder(host string, req *Request) StreamTransport {
	pp.Println(host)
	u := url.URL{Scheme: "ws", Host: host}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		panic(err)
	}
	return &WebSocketTransport{
		conn: conn,
	}
}
