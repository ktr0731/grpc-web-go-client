package grpcweb

import (
	"context"
	"io"
	"testing"

	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/jhump/protoreflect/desc"
	"github.com/k0kubun/pp"
	"github.com/ktr0731/grpc-test/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func p(s string) *string {
	return &s
}

type protoHelper struct {
	*desc.FileDescriptor

	s map[string]*descriptor.ServiceDescriptorProto
	m map[string]*descriptor.DescriptorProto
}

func (h *protoHelper) getServiceByName(t *testing.T, n string) *descriptor.ServiceDescriptorProto {
	if h.s == nil {
		h.s = map[string]*descriptor.ServiceDescriptorProto{}
		for _, svc := range h.GetServices() {
			h.s[svc.GetName()] = svc.AsServiceDescriptorProto()
		}
	}
	svc, ok := h.s[n]
	if !ok {
		require.FailNowf(t, "ServiceDescriptor not found", "no such *desc.ServiceDescriptor: %s", n)
	}
	return svc
}

func (h *protoHelper) getMessageTypeByName(t *testing.T, n string) *descriptor.DescriptorProto {
	if h.m == nil {
		h.m = map[string]*descriptor.DescriptorProto{}
		for _, msg := range h.GetMessageTypes() {
			h.m[msg.GetName()] = msg.AsDescriptorProto()
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

type stubTransport struct {
	host string
	req  *Request

	body []byte
}

func (t *stubTransport) Send(body io.Reader) (io.Reader, error) {
	return nil, nil
}

func stubTransportBuilder(host string, req *Request) Transport {
	return &stubTransport{
		host: host,
		req:  req,
	}
}

func TestClient(t *testing.T) {
	pkg := getAPIProto(t)

	service := pkg.getServiceByName(t, "Example")

	t.Run("NewClient returns new API client", func(t *testing.T) {
		client := NewClient("http://localhost:50051", WithTransportBuilder(stubTransportBuilder))
		assert.NotNil(t, client)
	})

	t.Run("Send an unary API", func(t *testing.T) {
		client := NewClient("http://localhost:50051", WithTransportBuilder(stubTransportBuilder))

		in, out := pkg.getMessageTypeByName(t, "SimpleRequest"), pkg.getMessageTypeByName(t, "SimpleResponse")
		req, err := NewRequest(service, service.GetMethod()[0], in, out)
		assert.NoError(t, err)
		err = client.Send(context.Background(), req)
		assert.NoError(t, err)
	})

	t.Run("real", func(t *testing.T) {
		client := NewClient("http://localhost:50051")

		// in, out := pkg.getMessageTypeByName(t, "SimpleRequest"), pkg.getMessageTypeByName(t, "SimpleResponse")
		in := &api.SimpleRequest{Name: "ktr"}
		out := &api.SimpleResponse{}
		req, err := NewRequest(service, service.GetMethod()[0], in, out)
		assert.NoError(t, err)
		err = client.Send(context.Background(), req)
		assert.NoError(t, err)

		// b, err := proto.Marshal(out)
		// require.NoError(t, err)
		// pp.Println(string(b))
		pp.Println(out)
	})

	// t.Run("real", func(t *testing.T) {
	// 	client := NewClient("http://localhost:50051")
	//
	// 	// in, out := pkg.getMessageTypeByName(t, "SimpleRequest"), pkg.getMessageTypeByName(t, "SimpleResponse")
	// 	in := &api.UnaryMessageRequest{Name: &api.Name{FirstName: "foo", LastName: "bar"}}
	// 	out := &api.SimpleResponse{}
	// 	req, err := NewRequest(service, service.GetMethod()[1], in, out)
	// 	assert.NoError(t, err)
	// 	err = client.Send(context.Background(), req)
	// 	assert.NoError(t, err)
	//
	// 	// b, err := proto.Marshal(out)
	// 	// require.NoError(t, err)
	// 	// pp.Println(string(b))
	// 	pp.Println(out)
	// })
}
