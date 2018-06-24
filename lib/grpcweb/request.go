package grpcweb

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
)

type Request struct {
	s *descriptor.ServiceDescriptorProto
	m *descriptor.MethodDescriptorProto

	in, out proto.Message
}

func NewRequest(
	service *descriptor.ServiceDescriptorProto,
	method *descriptor.MethodDescriptorProto,
	in proto.Message,
	out proto.Message,
) *Request {
	return &Request{
		s:   service,
		m:   method,
		in:  in,
		out: out,
	}
}

func (r *Request) URL(host string) string {
	return fmt.Sprintf("%s/%s/%s", host, r.s.GetName(), r.m.GetName())
}
