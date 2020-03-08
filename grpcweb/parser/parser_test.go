package parser_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/ktr0731/grpc-web-go-client/grpcweb/parser"
	"github.com/pkg/errors"
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
			expectedContentLength: uint32(12),
			expectedHeaderType:    message,
		},
		"trailer header": {
			in:                    []byte{0x80, 0x00, 0x00, 0x00, 0x48},
			expectedContentLength: uint32(72),
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
