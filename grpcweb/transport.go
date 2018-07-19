package grpcweb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
)

type TransportBuilder func(host string, req *Request) Transport

var DefaultTransportBuilder TransportBuilder = HTTPTransportBuilder

// Transport creates new request.
// Transport is created only one per one request, MUST not use used transport again.
type Transport interface {
	Send(ctx context.Context, body io.Reader) (io.Reader, error)
}

type HTTPTransport struct {
	sent bool

	host   string
	req    *Request
	client *http.Client

	insecure bool
}

func (t *HTTPTransport) Send(ctx context.Context, body io.Reader) (io.Reader, error) {
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
	defer res.Body.Close()

	var buf bytes.Buffer
	_, err = buf.ReadFrom(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read the response body")
	}

	return &buf, nil
}

func HTTPTransportBuilder(host string, req *Request) Transport {
	return &HTTPTransport{
		host:   host,
		req:    req,
		client: &http.Client{},
	}
}
