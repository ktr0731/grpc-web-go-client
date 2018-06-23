package grpcweb

type TransportBuilder func(*Client) Transport

var DefaultTransportBuilder TransportBuilder = func(_ *Client) Transport {
	return &HTTPTransport{}
}

// Transport creates new request.
// Transport is created only one per one request, MUST not use used transport again.
type Transport interface {
	Send()
}

type HTTPTransport struct {
	Transport
}
