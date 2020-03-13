package grpcweb

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/ktr0731/grpc-test/api"
	"github.com/ktr0731/grpc-web-go-client/grpcweb/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type unaryTransport struct {
	h   http.Header
	r   io.ReadCloser
	err error
}

func (t *unaryTransport) Send(ctx context.Context, endpoint, contentType string, body io.Reader) (http.Header, io.ReadCloser, error) {
	return t.h, t.r, t.err
}

func (t *unaryTransport) Close() error {
	return nil
}

func TestInvoke(t *testing.T) {
	header := http.Header{
		"hakase": []string{"shinonome"},
		"nano":   []string{"shinonome"},
	}

	cases := map[string]struct {
		transportHeader          http.Header
		transportContentFileName string
		expectedHeader           metadata.MD
		expectedTrailer          metadata.MD
		expectedContent          api.SimpleResponse
		expectedStatus           *status.Status
		wantErr                  bool
	}{
		"normal (only response)": {
			transportHeader:          header,
			transportContentFileName: "response.in",
			expectedHeader: metadata.New(map[string]string{
				"hakase": "shinonome",
				"nano":   "shinonome",
			}),
			expectedContent: api.SimpleResponse{Message: "hello, ktr"},
			expectedStatus:  status.New(codes.OK, ""),
		},
		"normal (trailer and response)": {
			transportHeader:          header,
			transportContentFileName: "trailer_response.in",
			expectedHeader: metadata.New(map[string]string{
				"hakase": "shinonome",
				"nano":   "shinonome",
			}),
			expectedTrailer: metadata.New(map[string]string{
				"trailer_key1": "trailer_val1",
				"trailer_key2": "trailer_val2",
			}),
			expectedContent: api.SimpleResponse{Message: "response"},
			expectedStatus:  status.New(codes.OK, ""),
		},
		"error (trailer and response)": {
			transportHeader:          header,
			transportContentFileName: "trailer_response_error.in",
			expectedHeader: metadata.New(map[string]string{
				"hakase": "shinonome",
				"nano":   "shinonome",
			}),
			expectedTrailer: metadata.New(map[string]string{
				"trailer_key1": "trailer_val1",
				"trailer_key2": "trailer_val2",
			}),
			expectedStatus: status.New(codes.Internal, "internal error"),
			wantErr:        true,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r, err := os.Open(filepath.Join("testdata", c.transportContentFileName))
			if err != nil {
				t.Fatalf("Open should not return an error, but got '%s'", err)
			}

			injectUnaryTransport(t, &unaryTransport{
				h: c.transportHeader,
				r: r,
			})

			var header, trailer metadata.MD
			client, err := DialContext(":50051")
			if err != nil {
				t.Fatalf("DialContext should not return an error, but got '%s'", err)
			}

			var res api.SimpleResponse
			opts := []CallOption{Header(&header), Trailer(&trailer)}
			req := api.SimpleRequest{Name: "nano"}
			err = client.Invoke(context.Background(), "/service/Method", &req, &res, opts...)
			if c.wantErr {
				if err == nil {
					t.Fatalf("should return an error, but got nil")
				}
			} else if err != nil {
				t.Fatalf("should not return an error, but got '%s'", err)
			}

			stat := status.Convert(err)

			if diff := cmp.Diff(c.expectedHeader, header); diff != "" {
				t.Errorf("-want, +got\n%s", diff)
			}
			if diff := cmp.Diff(c.expectedTrailer, trailer); diff != "" {
				t.Errorf("-want, +got\n%s", diff)
			}
			if diff := cmp.Diff(c.expectedContent, res); diff != "" {
				t.Errorf("-want, +got\n%s", diff)
			}
			if stat.Code() != c.expectedStatus.Code() {
				t.Errorf("expected status code: %s, but got %s", c.expectedStatus.Code(), stat.Code())
			}
			if stat.Message() != c.expectedStatus.Message() {
				t.Errorf("expected status message: %s, but got %s", c.expectedStatus.Message(), stat.Message())
			}
		})
	}
}

func injectUnaryTransport(t *testing.T, tr transport.UnaryTransport) {
	old := transport.NewUnary
	t.Cleanup(func() {
		transport.NewUnary = old
	})
	transport.NewUnary = func(string, *transport.ConnectOptions) transport.UnaryTransport {
		return tr
	}
}

func TestServerStream(t *testing.T) {
	header := http.Header{
		"hakase": []string{"shinonome"},
		"nano":   []string{"shinonome"},
	}

	cases := map[string]struct {
		transportHeader          http.Header
		transportContentFileName string
		expectedHeader           metadata.MD
		expectedTrailer          metadata.MD
		expectedContent          []api.SimpleResponse
		expectedStatus           *status.Status
	}{
		"normal (only response)": {
			transportHeader:          header,
			transportContentFileName: "server_stream_response.in",
			expectedHeader: metadata.New(map[string]string{
				"hakase": "shinonome",
				"nano":   "shinonome",
			}),
			expectedContent: []api.SimpleResponse{
				{Message: "hello nano, I greet 1 times."},
				{Message: "hello nano, I greet 2 times."},
				{Message: "hello nano, I greet 3 times."},
			},
			expectedStatus: status.New(codes.OK, ""),
		},
		"normal (trailer and response)": {
			transportHeader:          header,
			transportContentFileName: "server_stream_trailer_response.in",
			expectedHeader: metadata.New(map[string]string{
				"hakase": "shinonome",
				"nano":   "shinonome",
			}),
			expectedTrailer: metadata.New(map[string]string{
				"trailer_key1": "trailer_val1",
				"trailer_key2": "trailer_val2",
			}),
			expectedContent: []api.SimpleResponse{
				{Message: "hello nano, I greet 1 times."},
				{Message: "hello nano, I greet 2 times."},
				{Message: "hello nano, I greet 3 times."},
			},
			expectedStatus: status.New(codes.OK, ""),
		},
		"error (trailer and response)": {
			transportHeader:          header,
			transportContentFileName: "server_stream_trailer_response_error.in",
			expectedHeader: metadata.New(map[string]string{
				"hakase": "shinonome",
				"nano":   "shinonome",
			}),
			expectedTrailer: metadata.New(map[string]string{
				"trailer_key1": "trailer_val1",
				"trailer_key2": "trailer_val2",
			}),
			expectedContent: []api.SimpleResponse{
				{Message: "hello nano, I greet 1 times."},
			},
			expectedStatus: status.New(codes.Internal, "internal error"),
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r, err := os.Open(filepath.Join("testdata", c.transportContentFileName))
			if err != nil {
				t.Fatalf("Open should not return an error, but got '%s'", err)
			}

			injectUnaryTransport(t, &unaryTransport{
				h: c.transportHeader,
				r: r,
			})

			client, err := DialContext(":50051")
			if err != nil {
				t.Fatalf("DialContext should not return an error, but got '%s'", err)
			}

			stm, err := client.NewServerStream(&grpc.StreamDesc{ServerStreams: true}, "/service/Method")
			if err != nil {
				t.Fatalf("should not return an error, but got '%s'", err)
			}

			ctx := context.Background()
			if err := stm.Send(ctx, &api.SimpleRequest{Name: "nano"}); err != nil {
				t.Fatalf("Send should not return an error, but got '%s'", err)
			}

			var ress []api.SimpleResponse
			for {
				var res api.SimpleResponse
				err = stm.Receive(ctx, &res) // Don't create scoped error
				if errors.Is(err, io.EOF) {
					err = nil
					break
				}
				if err != nil {
					t.Logf("Receive returns an error: %s", err)
					break
				}
				ress = append(ress, res)
			}

			stat := status.Convert(err)

			header, err := stm.Header()
			if err != nil {
				t.Fatalf("Header should not return an error, but got '%s'", err)
			}
			if diff := cmp.Diff(c.expectedHeader, header); diff != "" {
				t.Errorf("-want, +got\n%s", diff)
			}
			if diff := cmp.Diff(c.expectedTrailer, stm.Trailer()); diff != "" {
				t.Errorf("-want, +got\n%s", diff)
			}
			if diff := cmp.Diff(c.expectedContent, ress); diff != "" {
				t.Errorf("-want, +got\n%s", diff)
			}
			if stat.Code() != c.expectedStatus.Code() {
				t.Errorf("expected status code: %s, but got %s", c.expectedStatus.Code(), stat.Code())
			}
			if stat.Message() != c.expectedStatus.Message() {
				t.Errorf("expected status message: %s, but got %s", c.expectedStatus.Message(), stat.Message())
			}
		})
	}
}
