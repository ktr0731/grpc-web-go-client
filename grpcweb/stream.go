package grpcweb

import (
	"context"
	"encoding/binary"
	"io"

	"github.com/ktr0731/grpc-web-go-client/grpcweb/transport"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

type ClientStream interface {
	// Header returns the response header.
	Header() (metadata.MD, error)
	// Trailer returns the response trailer.
	Trailer() metadata.MD
	Send(ctx context.Context, req interface{}) error
	CloseAndReceive(ctx context.Context, res interface{}) error
}

type clientStream struct {
	endpoint    string
	transport   transport.ClientStreamTransport
	callOptions *callOptions
}

func (s *clientStream) Header() (metadata.MD, error) {
	return nil, nil
}

func (s *clientStream) Trailer() metadata.MD {
	return nil
}

func (s *clientStream) Send(ctx context.Context, req interface{}) error {
	r, err := parseRequestBody(s.callOptions.codec, req)
	if err != nil {
		return errors.Wrap(err, "failed to build the request")
	}
	if err := s.transport.Send(ctx, r); err != nil {
		return errors.Wrap(err, "failed to send the request")
	}
	return nil
}

func (s *clientStream) CloseAndReceive(ctx context.Context, res interface{}) error {
	if err := s.transport.CloseSend(); err != nil {
		return errors.Wrap(err, "failed to close the send stream")
	}
	rawBody, err := s.transport.Receive(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to receive the response")
	}
	resBody, err := parseLengthPrefixedMessageFromHeader(rawBody)
	if err != nil {
		return errors.Wrap(err, "failed to parse the response body")
	}
	status, _, err := parseStatusAndTrailerFromHeader(rawBody)
	if err != nil && !errors.Is(err, io.EOF) {
		return errors.Wrap(err, "failed to parse trailer")
	}

	if err := s.callOptions.codec.Unmarshal(resBody, res); err != nil {
		return errors.Wrap(err, "failed to unmarshal the response body")
	}
	return status.Err()
}

type ServerStream interface {
	Send(ctx context.Context, req interface{}) error
	Receive(ctx context.Context, res interface{}) error
}

type serverStream struct {
	endpoint    string
	transport   transport.UnaryTransport
	resStream   io.ReadCloser
	callOptions *callOptions
}

func (s *serverStream) Send(ctx context.Context, req interface{}) error {
	codec := s.callOptions.codec

	r, err := parseRequestBody(codec, req)
	if err != nil {
		return errors.Wrap(err, "failed to build the request body")
	}

	contentType := "application/grpc-web+" + codec.Name()
	_, rawBody, err := s.transport.Send(ctx, s.endpoint, contentType, r)
	if err != nil {
		return errors.Wrap(err, "failed to send the request")
	}
	s.resStream = rawBody
	return nil
}

func (s *serverStream) Receive(ctx context.Context, res interface{}) (err error) {
	if s.resStream == nil {
		return errors.New("Receive must be call after calling Send")
	}
	defer func() {
		if err == io.EOF {
			if rerr := s.transport.Close(); rerr != nil {
				err = rerr
			}
			s.resStream.Close()
		}
	}()

	var h [5]byte
	n, err := s.resStream.Read(h[:])
	if err != nil {
		return err
	}
	if n != len(h) {
		return io.ErrUnexpectedEOF
	}

	flag := h[0]
	length := binary.BigEndian.Uint32(h[1:])
	if length == 0 {
		return io.EOF
	}
	if flag == 0 || flag == 1 { // Message header.
		msg, err := parseLengthPrefixedMessage(s.resStream, int(length))
		if err != nil {
			return err
		}
		if err := s.callOptions.codec.Unmarshal(msg, res); err != nil {
			return errors.Wrap(err, "failed to unmarshal response body")
		}
		return nil
	}

	status, _, err := parseStatusAndTrailer(s.resStream, int(length))
	if err != nil {
		return errors.Wrap(err, "failed to parse trailer")
	}
	if status.Code() != codes.OK {
		return status.Err()
	}
	return io.EOF
}

type BidiStream interface {
	Send(ctx context.Context, req interface{}) error
	Receive(ctx context.Context, res interface{}) error
	CloseSend() error
}

type bidiStream struct {
	*clientStream
}

var (
	canonicalGRPCStatusBytes = []byte("Grpc-Status: ")
	gRPCStatusBytes          = []byte("grpc-status: ")
)

func (s *bidiStream) Receive(ctx context.Context, res interface{}) error {
	rawBody, err := s.transport.Receive(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to receive the response")
	}

	var h [5]byte
	n, err := rawBody.Read(h[:])
	if err != nil {
		return err
	}
	if n != len(h) {
		return io.ErrUnexpectedEOF
	}

	flag := h[0]
	length := binary.BigEndian.Uint32(h[1:])
	if length == 0 {
		return io.EOF
	}
	if flag == 0 || flag == 1 { // Message header.
		msg, err := parseLengthPrefixedMessage(rawBody, int(length))
		if err != nil {
			return err
		}
		if err := s.callOptions.codec.Unmarshal(msg, res); err != nil {
			return errors.Wrap(err, "failed to unmarshal response body")
		}
		return nil
	}

	status, _, err := parseStatusAndTrailer(rawBody, int(length))
	if err != nil {
		return errors.Wrap(err, "failed to parse trailer")
	}

	if err := s.transport.Close(); err != nil {
		return errors.Wrap(err, "failed to close the gRPC transport")
	}

	if status.Code() != codes.OK {
		return status.Err()
	}
	return io.EOF
}

func (s *bidiStream) CloseSend() error {
	if err := s.transport.CloseSend(); err != nil {
		return errors.Wrap(err, "failed to close the send stream")
	}
	return nil
}
