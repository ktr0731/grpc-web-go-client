package grpcweb

// Response contains ContentType and its Content.
// ContentType is same as the value of Name() of encoding.Codec.
// Actual type of Content is depends on ContentType.
// For example, proto.Message if ContentType is "proto".
type Response struct {
	ContentType string
	Content     interface{}
}
