package grpcweb

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/jhump/protoreflect/desc"
	"github.com/k0kubun/pp"
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

// toBytes converts n to protobuf representation.
//
// for example,
// toBytes(8) is 0x08 (0000 1000)
// toBytes(343) is 0xd702 (1101 0010 0000 0010)
func EncodeVarint(n int32) ([]byte, error) {
	var buf bytes.Buffer
	var a interface{}
	switch {
	case n < 255:
		a = int8(n)
		buf.Write([]byte{0x00, 0x00, 0x00})
	case n < 65535:
		a = int16(n)
		buf.Write([]byte{0x00, 0x00})
	default:
		a = n
	}
	pp.Println(buf.Bytes())
	if err := binary.Write(&buf, binary.LittleEndian, a); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encode(w io.Writer, i int, n int32) {
	if n < 127 {
		binary.Write(w, binary.LittleEndian, n)
		return
	}
	binary.Write(w, binary.LittleEndian, byte(n%127))
	encode(w, i+1, n%127)
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
