package grpcweb

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
)

type Request struct {
	endpoint string
	in, out  interface{}
}

// NewRequest instantiates new API request from passed endpoint and I/O types.
// endpoint must be formed like:
//
//   "/{package name}.{service name}/{method name}"
//
func NewRequest(
	endpoint string,
	in proto.Message,
	out proto.Message,
) *Request {
	return &Request{
		endpoint: endpoint,
		in:       in,
		out:      out,
	}
}

// ToEndpoint generates an endpoint from a service descriptor and a method descriptor.
func ToEndpoint(pkg string, s *descriptor.ServiceDescriptorProto, m *descriptor.MethodDescriptorProto) string {
	return fmt.Sprintf("/%s.%s/%s", pkg, s.GetName(), m.GetName())
}
