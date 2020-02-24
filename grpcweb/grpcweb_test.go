package grpcweb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/ktr0731/grpc-test/api"
	"google.golang.org/grpc/metadata"
)

func Test_parseResponseBody(t *testing.T) {
	cases := map[string]struct {
		fname           string
		expectedTrailer metadata.MD
		expectedContent api.SimpleResponse
		wantErr         bool
	}{
		"normal (only response)": {
			fname:           "response",
			expectedContent: api.SimpleResponse{Message: "hello, ktr"},
		},
		"normal (trailer and response)": {
			fname: "trailer_response",
			expectedTrailer: metadata.New(map[string]string{
				"trailer_key1": "trailer_val1",
				"trailer_key2": "trailer_val2",
			}),
			expectedContent: api.SimpleResponse{Message: "response"},
		},
		"error (trailer and response)": {
			fname: "trailer_response_error",
			expectedTrailer: metadata.New(map[string]string{
				"trailer_key1": "trailer_val1",
				"trailer_key2": "trailer_val2",
			}),
		},
	}

	codec := defaultCallOptions.codec

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r, err := os.Open(filepath.Join("testdata", c.fname))
			if err != nil {
				t.Fatalf("Open should not return an error, but got '%s'", err)
			}
			actualTrailer, actualContent, err := parseResponseBody(r)
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
		})
	}
}
