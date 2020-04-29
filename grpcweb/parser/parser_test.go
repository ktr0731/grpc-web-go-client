package parser_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/ktr0731/grpc-web-go-client/grpcweb/parser"
	"github.com/pkg/errors"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestParseResponseHeader(t *testing.T) {
	type headerType int
	const (
		message headerType = iota
		trailer
	)
	cases := map[string]struct {
		in                    []byte
		expectedHeaderType    headerType
		expectedContentLength uint32
		wantErr               bool
		expectedErr           error
	}{
		"message header": {
			in:                    []byte{0x00, 0x00, 0x00, 0x00, 0x0c},
			expectedContentLength: 12,
			expectedHeaderType:    message,
		},
		"trailer header": {
			in:                    []byte{0x80, 0x00, 0x00, 0x00, 0x48},
			expectedContentLength: 72,
			expectedHeaderType:    trailer,
		},
		"unexpected error": {
			in:          []byte{0x80},
			wantErr:     true,
			expectedErr: io.ErrUnexpectedEOF,
		},
		"the length is zero": {
			in:          []byte{0x00, 0x00, 0x00, 0x00, 0x00},
			wantErr:     true,
			expectedErr: io.EOF,
		},
	}

	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			h, err := parser.ParseResponseHeader(bytes.NewReader(c.in))
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected a error, but got nil")
				}
				if c.expectedErr != nil && !errors.Is(err, c.expectedErr) {
					t.Errorf("expected error is '%v', but got '%v'", c.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("should not return an error, but got '%s'", err)
			}

			m := map[headerType]bool{
				message: h.IsMessageHeader(),
				trailer: h.IsTrailerHeader(),
			}
			if v, ok := m[c.expectedHeaderType]; !v || !ok {
				t.Errorf("header type is not %d", c.expectedHeaderType)
			}
		})
	}
}

func TestParseLengthPrefixedMessage(t *testing.T) {
	cases := map[string]struct {
		bytes       []byte
		length      uint32
		wantErr     bool
		expectedErr error
	}{
		"ok": {
			bytes:  []byte{0x01, 0x02, 0x03},
			length: 3,
		},
		"unexpected EOF": {
			bytes:       []byte{0x01, 0x02},
			length:      3,
			wantErr:     true,
			expectedErr: io.ErrUnexpectedEOF,
		},
		"EOF": {
			bytes:       []byte{},
			length:      0,
			wantErr:     true,
			expectedErr: io.EOF,
		},
	}

	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			_, err := parser.ParseLengthPrefixedMessage(bytes.NewReader(c.bytes), c.length)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected a error, but got nil")
				}
				if c.expectedErr != nil && !errors.Is(err, c.expectedErr) {
					t.Errorf("expected error is '%v', but got '%v'", c.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("should not return an error, but got '%s'", err)
			}
		})
	}
}

func TestParseStatusAndTrailer(t *testing.T) {
	cases := map[string]struct {
		fname           string
		length          uint32
		expectedStatus  *status.Status
		expectedTrailer metadata.MD
		expectedErr     error
	}{
		"ok": {
			fname:          "status_trailer.in",
			expectedStatus: status.New(codes.OK, ""),
			expectedTrailer: metadata.New(map[string]string{
				"trailer_key1": "trailer_val1",
				"trailer_key2": "trailer_val2",
			}),
		},
		"ok with message": {
			fname:          "status_trailer_error.in",
			expectedStatus: status.New(codes.Internal, "internal error"),
			expectedTrailer: metadata.New(map[string]string{
				"trailer_key1": "trailer_val1",
				"trailer_key2": "trailer_val2",
			}),
		},
		"ok with grpc-status-details-bin": {
			fname: "status_grpc_status_details_bin.in",
			expectedStatus: func() *status.Status {
				s, err := status.New(codes.Internal, "internal error").WithDetails(
					&errdetails.BadRequest{
						FieldViolations: []*errdetails.BadRequest_FieldViolation{
							&errdetails.BadRequest_FieldViolation{
								Field:       "field",
								Description: "description",
							},
						},
					},
					&errdetails.PreconditionFailure{
						Violations: []*errdetails.PreconditionFailure_Violation{
							&errdetails.PreconditionFailure_Violation{
								Type:        "type",
								Subject:     "subject",
								Description: "description",
							},
						},
					},
				)
				if err != nil {
					t.Fatalf("WithDetails should not return an error, but got '%s'", err)
				}
				return s
			}(),
			expectedTrailer: metadata.New(map[string]string{
				"trailer_key1": "trailer_val1",
				"trailer_key2": "trailer_val2",
			}),
		},
		"bytes exceeds length": {
			fname:       "status_trailer.in",
			length:      3,
			expectedErr: io.ErrUnexpectedEOF,
		},
		"invalid metadata": {
			fname:       "status_trailer_invalid_metadata.in",
			expectedErr: io.ErrUnexpectedEOF,
		},
		"invalid status": {
			fname:          "status_trailer_invalid_status.in",
			expectedStatus: status.New(codes.Unknown, ""),
			expectedTrailer: metadata.New(map[string]string{
				"trailer_key1": "trailer_val1",
				"trailer_key2": "trailer_val2",
			}),
		},
	}

	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			fpath := filepath.Join("testdata", c.fname)
			b, err := ioutil.ReadFile(fpath)
			if err != nil {
				t.Fatalf("Open should not return an error, but got '%s'", err)
			}

			in := bytes.NewReader(b)
			if c.length == 0 {
				c.length = uint32(in.Len())
			}
			status, trailer, err := parser.ParseStatusAndTrailer(in, c.length)
			if err != c.expectedErr {
				t.Errorf("expected error: '%s', but got '%s'", c.expectedErr, err)
				if err != nil {
					return
				}
			}
			if diff := cmp.Diff(c.expectedStatus.Proto(), status.Proto(), protocmp.Transform()); diff != "" {
				t.Errorf("-want, +got\n%s", diff)
			}
			if diff := cmp.Diff(c.expectedTrailer, trailer); diff != "" {
				t.Errorf("-want, +got\n%s", diff)
			}
		})
	}
}
