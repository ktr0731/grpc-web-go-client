package grpcweb

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
)

type TransportBuilder func(host string, req *Request) Transport

var DefaultTransportBuilder TransportBuilder = HTTPTransportBuilder

// Transport creates new request.
// Transport is created only one per one request, MUST not use used transport again.
type Transport interface {
	Send(body []byte) error
}

type HTTPTransport struct {
	sent bool

	host   string
	req    *Request
	client *http.Client
}

func (t *HTTPTransport) Send(body []byte) error {
	if t.sent {
		return errors.New("Send must be called only one time per one Request")
	}
	defer func() {
		t.sent = true
	}()

	req, err := http.NewRequest(http.MethodPost, t.req.URL(t.host), bytes.NewReader(body))
	if err != nil {
		return errors.Wrap(err, "failed to build the API request")
	}
	res, err := t.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to send the API")
	}
	fmt.Println(res)
	return nil
}

func HTTPTransportBuilder(host string, req *Request) Transport {
	return &HTTPTransport{
		host:   host,
		req:    req,
		client: &http.Client{},
	}
}
