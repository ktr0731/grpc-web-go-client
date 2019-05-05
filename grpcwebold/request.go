package grpcweb

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
)

type Request struct {
	endpoint string
	in, out  proto.Message
}

// NewRequest instantiates new API request from passed endpoint and I/O types.
// endpoint must be formed like:
//
//   "/{package name}.{service name}/{method name}"
//
// in and out must be proto.Message.
func NewRequest(
	endpoint string,
	in interface{},
	out interface{},
) *Request {
	defer func() {
		if err := recover(); err != nil {
			panic("currently, ktr0731/grpc-web-go-client only supports Protocol Buffers as a codec")
		}
	}()
	return &Request{
		endpoint: endpoint,
		in:       in.(proto.Message),
		out:      out.(proto.Message),
	}
}

// ToEndpoint generates an endpoint from a service descriptor and a method descriptor.
func ToEndpoint(pkg string, s *descriptor.ServiceDescriptorProto, m *descriptor.MethodDescriptorProto) string {
	return fmt.Sprintf("/%s.%s/%s", pkg, s.GetName(), m.GetName())
}
