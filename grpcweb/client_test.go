package grpcweb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/ktr0731/grpc-test/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var defaultAddr = "localhost:50051"

func p(s string) *string {
	return &s
}

type protoHelper struct {
	*desc.FileDescriptor

	s map[string]*descriptor.ServiceDescriptorProto
	m map[string]*dynamic.Message
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

func (h *protoHelper) getMessageTypeByName(t *testing.T, n string) *dynamic.Message {
	if h.m == nil {
		h.m = map[string]*dynamic.Message{}
		for _, msg := range h.GetMessageTypes() {
			h.m[msg.GetName()] = dynamic.NewMessage(msg)
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

func readFile(t *testing.T, fname string) []byte {
	b, err := ioutil.ReadFile(filepath.Join("testdata", fname))
	require.NoError(t, err)
	return b
}

type stubTransport struct {
	host string
	req  *Request

	res []byte
}

func (t *stubTransport) Send(_ context.Context, body io.Reader) (io.ReadCloser, error) {
	return ioutil.NopCloser(bytes.NewReader(t.res)), nil
}

// for testing
func withStubTransport(t *stubTransport) ClientOption {
	return func(c *Client) {
		c.tb = func(host string, req *Request) Transport {
			t.host = host
			t.req = req
			return t
		}
	}
}

func TestClient(t *testing.T) {
	pkg := getAPIProto(t)
	service := pkg.getServiceByName(t, "Example")
	endpoint := ToEndpoint("api", service, service.GetMethod()[0])

	t.Run("ToEndpoint", func(t *testing.T) {
		expected := fmt.Sprintf("/api.%s/%s", service.GetName(), service.GetMethod()[0].GetName())
		assert.Equal(t, expected, endpoint)
	})

	t.Run("NewClient returns new API client", func(t *testing.T) {
		client := NewClient(defaultAddr, withStubTransport(&stubTransport{}))
		assert.NotNil(t, client)
	})

	t.Run("Send an unary API", func(t *testing.T) {
		client := NewClient(defaultAddr, withStubTransport(&stubTransport{
			res: readFile(t, "unary_ktr.out"),
		}))

		in, out := pkg.getMessageTypeByName(t, "SimpleRequest"), pkg.getMessageTypeByName(t, "SimpleResponse")
		req, err := NewRequest(endpoint, in, out)
		assert.NoError(t, err)
		err = client.Unary(context.Background(), req)
		assert.NoError(t, err)
	})

	t.Run("Send a server streaming API", func(t *testing.T) {
		client := NewClient(defaultAddr, withStubTransport(&stubTransport{
			res: readFile(t, "server_ktr.out"),
		}))

		in, out := pkg.getMessageTypeByName(t, "SimpleRequest"), pkg.getMessageTypeByName(t, "SimpleResponse")
		req, err := NewRequest(endpoint, in, out)
		assert.NoError(t, err)
		s, err := client.ServerStreaming(context.Background(), req)
		assert.NoError(t, err)

		for i := 0; ; i++ {
			res, err := s.Recv()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)

			expected := fmt.Sprintf("hello ktr, I greet %d times.", i)
			assert.Equal(t, expected, res.(*dynamic.Message).GetFieldByName("message"))
		}
	})

	// t.Run("Send a client streaming API", func(t *testing.T) {
	// 	client := NewClient(defaultAddr, withStubTransport(&stubTransport{
	// 		res: readFile(t, "client_streaming_ktr.out"),
	// 	}))
	//
	// 	// NOTE: in is a dummy input. actual input is above file.
	// 	in, out := pkg.getMessageTypeByName(t, "SimpleRequest"), pkg.getMessageTypeByName(t, "SimpleResponse")
	// 	req, err := NewRequest(endpoint, in, out)
	// 	require.NoError(t, err)
	//
	// 	cs, err = client.ClientStream(context.Background(), req)
	// 	require.NoError(t, err)
	//
	// 	names := []string{"ohana", "nako", "minko"}
	// 	for _, name := range names {
	// 		cs.Send()
	// 	}
	// 	cs.Close()
	// })
}

func TestClientE2E(t *testing.T) {
	pkg := getAPIProto(t)
	service := pkg.getServiceByName(t, "Example")

	t.Run("Unary", func(t *testing.T) {
		defer server.New(false).Serve(nil, true).Stop()

		client := NewClient(defaultAddr)
		endpoint := ToEndpoint("api", service, service.GetMethod()[0])

		in := pkg.getMessageTypeByName(t, "SimpleRequest")

		cases := []string{
			"ktr",
			"tooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooo-looooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooong-teeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeext",
		}

		for _, c := range cases {
			in.SetFieldByName("name", c)

			out := pkg.getMessageTypeByName(t, "SimpleResponse")

			req, err := NewRequest(endpoint, in, out)
			assert.NoError(t, err)
			err = client.Unary(context.Background(), req)
			assert.NoError(t, err)

			expected := fmt.Sprintf("hello, %s", c)
			assert.Equal(t, expected, out.GetFieldByName("message"))
		}
	})

	t.Run("ServerStreaming", func(t *testing.T) {
		defer server.New(false).Serve(nil, true).Stop()

		client := NewClient(defaultAddr)
		endpoint := ToEndpoint("api", service, service.GetMethod()[10])
		assert.Equal(t, endpoint, "/api.Example/ServerStreaming")

		in := pkg.getMessageTypeByName(t, "SimpleRequest")
		in.SetFieldByName("name", "ktr")
		out := pkg.getMessageTypeByName(t, "SimpleResponse")

		req, err := NewRequest(endpoint, in, out)
		assert.NoError(t, err)

		s, err := client.ServerStreaming(context.Background(), req)
		assert.NoError(t, err)

		for i := 0; ; i++ {
			res, err := s.Recv()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)

			expected := fmt.Sprintf("hello ktr, I greet %d times.", i)
			assert.Equal(t, expected, res.(*dynamic.Message).GetFieldByName("message"))
		}
	})
}
