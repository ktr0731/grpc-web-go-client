package grpcweb

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/jhump/protoreflect/desc"
	"github.com/pkg/errors"
)

type Request struct {
	s *descriptor.ServiceDescriptorProto
	m *descriptor.MethodDescriptorProto

	in, out proto.Message
	outDesc *desc.MessageDescriptor
}

func NewRequest(
	service *descriptor.ServiceDescriptorProto,
	method *descriptor.MethodDescriptorProto,
	in proto.Message,
	out proto.Message,
) (*Request, error) {
	desc, err := desc.LoadMessageDescriptorForMessage(out)
	if err != nil {
		return nil, errors.Wrap(err, "invalid MessageDescriptor passed")
	}
	return &Request{
		s:       service,
		m:       method,
		in:      in,
		out:     out,
		outDesc: desc,
	}, nil
}

func (r *Request) URL(host string) string {
	// TODO: package name
	return fmt.Sprintf("%s/api.%s/%s", host, r.s.GetName(), r.m.GetName())
}
