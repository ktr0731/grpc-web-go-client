package grpcweb

import (
	"bytes"
	"encoding/binary"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/jhump/protoreflect/desc"
)

func getWireType(d *desc.FieldDescriptor) int32 {
	switch d.GetType() {
	case descriptor.FieldDescriptorProto_TYPE_FIXED32, descriptor.FieldDescriptorProto_TYPE_SFIXED32, descriptor.FieldDescriptorProto_TYPE_FLOAT:
		return proto.WireFixed32
	case descriptor.FieldDescriptorProto_TYPE_FIXED64, descriptor.FieldDescriptorProto_TYPE_UINT64, descriptor.FieldDescriptorProto_TYPE_DOUBLE:
		return proto.WireFixed64
	case descriptor.FieldDescriptorProto_TYPE_STRING, descriptor.FieldDescriptorProto_TYPE_BYTES, descriptor.FieldDescriptorProto_TYPE_MESSAGE:
		return proto.WireBytes
	default:
		return proto.WireVarint
	}
}

func toBytes(n int32) ([]byte, error) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, n); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// field-number + wire format
func buildFieldHeader(f *desc.FieldDescriptor) (byte, error) {
	h, err := toBytes(f.GetNumber()<<3 | getWireType(f))
	if err != nil {
		return byte(0), err
	}
	return h[len(h)-1], nil
}
