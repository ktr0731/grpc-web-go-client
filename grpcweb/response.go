package grpcweb

// Response
type Response struct {
	ContentType string
	Content     interface{}

	Request *Request
}
