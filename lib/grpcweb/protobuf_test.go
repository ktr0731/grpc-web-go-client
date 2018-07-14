package grpcweb

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeVarint(t *testing.T) {
	cases := []struct {
		in       int32
		expected []byte
	}{
		{in: 8, expected: []byte{0x00, 0x00, 0x00, 0x08}},
		{in: 343, expected: []byte{0x00, 0x00, 0xd7, 0x02}},
	}

	for _, c := range cases {
		actual, err := EncodeVarint(c.in)
		require.NoError(t, err)
		assert.Equal(t, c.expected, actual)
	}
}

func Test_encode(t *testing.T) {
	cases := []struct {
		in       int32
		expected []byte
	}{
		{in: 8, expected: []byte{0x08}},
		{in: 343, expected: []byte{0x00, 0x00, 0xd7, 0x02}},
	}

	for _, c := range cases {
		var buf bytes.Buffer
		encode(&buf, 0, c.in)
		assert.Equal(t, c.expected, buf.Bytes())
	}
}
