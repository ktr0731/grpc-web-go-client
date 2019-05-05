package transport

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
)

type UnaryTransport interface {
	Send(ctx context.Context, endpoint, contentType string, body io.Reader) (io.ReadCloser, error)
	Close() error
}

type httpTransport struct {
	host   string
	client *http.Client
	opts   *ConnectOptions

	sent bool
}

func (t *httpTransport) Send(ctx context.Context, endpoint, contentType string, body io.Reader) (io.ReadCloser, error) {
	if t.sent {
		return nil, errors.New("Send must be called only one time per one Request")
	}
	defer func() {
		t.sent = true
	}()

	// TODO: HTTPS support.
	scheme := "http"
	u := url.URL{Scheme: scheme, Host: t.host, Path: endpoint}
	url := u.String()
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build the API request")
	}

	req.Header.Add("content-type", contentType)
	req.Header.Add("x-grpc-web", "1")

	res, err := t.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send the API")
	}

	return res.Body, nil
}

func (t *httpTransport) Close() error {
	t.client.CloseIdleConnections()
	return nil
}

func NewUnary(host string, opts *ConnectOptions) UnaryTransport {
	return &httpTransport{
		host:   host,
		client: http.DefaultClient,
		opts:   opts,
	}
}

type ClientStreamTransport interface {
}

func NewClientStream() ClientStreamTransport {
	return nil
}
