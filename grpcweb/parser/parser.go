package parser

import (
	"bufio"
	"encoding/binary"
	"io"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Header struct {
	flag          byte
	ContentLength uint32
}

func (h *Header) IsMessageHeader() bool {
	return h.flag == 0 || h.flag == 1
}

func (h *Header) IsTrailerHeader() bool {
	return h.flag>>7 == 0x01
}

func ParseResponseHeader(r io.Reader) (*Header, error) {
	var h [5]byte
	n, err := r.Read(h[:])
	if err != nil {
		return nil, errors.Wrap(err, "failed to read header")
	}
	if n != len(h) {
		return nil, io.ErrUnexpectedEOF
	}

	length := binary.BigEndian.Uint32(h[1:])
	if length == 0 {
		return nil, io.EOF
	}
	return &Header{
		flag:          h[0],
		ContentLength: length,
	}, nil
}

func ParseLengthPrefixedMessage(r io.Reader, length uint32) ([]byte, error) {
	content := make([]byte, length)
	n, err := r.Read(content)
	switch {
	case uint32(n) != length:
		return nil, io.ErrUnexpectedEOF
	case err == io.EOF:
		return nil, io.EOF
	case err != nil:
		return nil, err
	}
	return content, nil
}

func ParseStatusAndTrailer(r io.Reader, length uint32) (*status.Status, metadata.MD, error) {
	var (
		readLen uint32
		code    codes.Code
		msg     string
	)
	trailer := metadata.New(nil)
	s := bufio.NewScanner(r)
	for s.Scan() {
		readLen += uint32(len(s.Bytes()))
		if readLen > length {
			return nil, nil, io.ErrUnexpectedEOF
		}

		t := s.Text()
		i := strings.Index(t, ": ")
		if i == -1 {
			return nil, nil, io.ErrUnexpectedEOF
		}
		k := strings.ToLower(t[:i])
		if k == "grpc-status" {
			n, err := strconv.ParseUint(t[i+2:], 10, 32)
			if err != nil {
				code = codes.Unknown
			} else {
				code = codes.Code(uint32(n))
			}
			continue
		}
		if k == "grpc-message" {
			msg = t[i+2:]
			continue
		}
		trailer.Append(k, t[i+2:])
	}

	stat := status.New(code, msg)

	if trailer.Len() == 0 {
		return stat, nil, nil
	}
	return stat, trailer, nil
}
