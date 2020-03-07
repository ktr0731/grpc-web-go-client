package grpcweb

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/ktr0731/grpc-test/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestInvoke(t *testing.T) {
	cases := map[string]struct {
		fname           string
		expectedTrailer metadata.MD
		expectedContent api.SimpleResponse
		expectedStatus  *status.Status
		wantErr         bool
	}{
		"normal (only response)": {
			fname:           "response",
			expectedContent: api.SimpleResponse{Message: "hello, ktr"},
			expectedStatus:  status.New(codes.OK, ""),
		},
		"normal (trailer and response)": {
			fname: "trailer_response",
			expectedTrailer: metadata.New(map[string]string{
				"trailer_key1": "trailer_val1",
				"trailer_key2": "trailer_val2",
			}),
			expectedContent: api.SimpleResponse{Message: "response"},
			expectedStatus:  status.New(codes.OK, ""),
		},
		"error (trailer and response)": {
			fname: "trailer_response_error",
			expectedTrailer: metadata.New(map[string]string{
				"trailer_key1": "trailer_val1",
				"trailer_key2": "trailer_val2",
			}),
			expectedStatus: status.New(codes.Internal, "internal error"),
		},
	}

	codec := defaultCallOptions.codec

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r, err := os.Open(filepath.Join("testdata", c.fname))
			if err != nil {
				t.Fatalf("Open should not return an error, but got '%s'", err)
			}

			var trailer metadata.MD
			client, err := DialContext(":50051")
			if err != nil {
				t.Fatalf("DialContext should not return an error, but got '%s'", err)
			}

			var res api.SimpleResponse
			client.Invoke(context.Background(), "/service/Method", nil, &res)

			actualStatus, actualTrailer, actualContent, err := parseResponseBody(r)
			if c.wantErr {
				if err == nil {
					t.Errorf("expected an error, but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("should not return an error, but got '%s'", err)
			}
			if diff := cmp.Diff(c.expectedTrailer, actualTrailer); diff != "" {
				t.Errorf("-want, +got\n%s", diff)
			}
			var res api.SimpleResponse
			err = codec.Unmarshal(actualContent, &res)
			if err != nil {
				t.Fatalf("content should be a proto message, but marshaling failed: %s", err)
			}
			if diff := cmp.Diff(c.expectedContent, res); diff != "" {
				t.Errorf("-want, +got\n%s", diff)
			}
			if c.expectedStatus.Code() != actualStatus.Code() {
				t.Errorf("expected code: %s, but got %s", c.expectedStatus.Code(), actualStatus.Code())
			}
			if c.expectedStatus.Message() != actualStatus.Message() {
				t.Errorf("expected message: %s, but got %s", c.expectedStatus.Message(), actualStatus.Message())
			}
		})
	}
}
