package grpcweb

import (
	"context"
	"testing"

	"github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	"github.com/jhump/protoreflect/desc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func p(s string) *string {
	return &s
}

type protoHelper struct {
	*desc.FileDescriptor

	m map[string]*desc.MessageDescriptor
}

func (h *protoHelper) getMessageTypeByName(t *testing.T, n string) *desc.MessageDescriptor {
	if h.m == nil {
		h.m = map[string]*desc.MessageDescriptor{}
		for _, msg := range h.GetMessageTypes() {
			h.m[msg.GetName()] = msg
		}
	}
	msg, ok := h.m[n]
	if !ok {
		require.FailNowf(t, "MessageDescriptor not found", "no such *desc.MessageDescriptor: %s", n)
	}
	return msg
}

func getAPIProto(t *testing.T) *protoHelper {
	t.Helper()

	pkgs := parseProto(t, "api.proto")
	require.Len(t, pkgs, 1)

	return &protoHelper{FileDescriptor: pkgs[0]}
}

func TestClient(t *testing.T) {
	method := &descriptor.MethodDescriptorProto{
		Name:       p("ExampleMethod"),
		InputType:  p("AnInputType"),
		OutputType: p("AnOutputType"),
	}

	t.Run("NewClient returns new API client", func(t *testing.T) {
		client := NewClient(method)
		assert.NotNil(t, client)
	})

	t.Run("Send an unary API", func(t *testing.T) {
		client := NewClient(method)

		pkg := getAPIProto(t)

		req, res := pkg.getMessageTypeByName(t, "SimpleRequest"), pkg.getMessageTypeByName(t, "SimpleResponse")
		err := client.Send(context.Background(), req, res)
		assert.NoError(t, err)

		t.Run("Send returns an error when call Send again", func(t *testing.T) {
			err = client.Send(context.Background(), req, res)
			assert.Error(t, err)
		})
	})
}
