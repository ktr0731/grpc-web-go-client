package grpcweb

import (
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"strconv"
	"sync"

	"github.com/duongcongtoaimanabie/grpc-web-go-client/grpcweb/parser"
	"github.com/duongcongtoaimanabie/grpc-web-go-client/grpcweb/transport"
	"github.com/pkg/errors"
	"go.uber.org/atomic"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type ClientStream interface {
	// Header returns the header metadata from the server, if there is any.
	// It blocks if the metadata is not ready to read.
	Header() (metadata.MD, error)
	// Trailer returns the trailer metadata from the server, if there is any.
	// It must only be called after stream.CloseAndReceive has returned, or
	// stream.Receive has returned a non-nil error (including io.EOF).
	Trailer() metadata.MD
	Send(ctx context.Context, req interface{}) error
	CloseAndReceive(ctx context.Context, res interface{}) error
}

type clientStream struct {
	endpoint    string
	transport   transport.ClientStreamTransport
	callOptions *callOptions

	trailersOnly, closed atomic.Bool
	headerMu, trailerMu  sync.RWMutex
	headerMD, trailerMD  metadata.MD
}

func (s *clientStream) Header() (metadata.MD, error) {
	if s.trailersOnly.Load() {
		return nil, nil
	}

	h := s.header()
	if h != nil {
		return h, nil
	}

	md := metadata.New(nil)
	headers, err := s.transport.Header()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get headers")
	}
	for k, v := range headers {
		md.Append(k, v...)
	}
	s.headerMu.Lock()
	s.headerMD = md
	s.headerMu.Unlock()
	return md, nil
}

func (s *clientStream) header() metadata.MD {
	s.headerMu.RLock()
	defer s.headerMu.RUnlock()
	return s.headerMD
}

func (s *clientStream) Trailer() metadata.MD {
	if !s.closed.Load() {
		panic("Trailer must be called after stream.CloseAndReceive has been called")
	}
	return s.trailer()
}

func (s *clientStream) trailer() metadata.MD {
	s.trailerMu.RLock()
	defer s.trailerMu.RUnlock()
	return s.trailerMD
}

func (s *clientStream) Send(ctx context.Context, req interface{}) error {
	r, err := encodeRequestBody(s.callOptions.codec, req)
	if err != nil {
		return errors.Wrap(err, "failed to build the request")
	}

	h := make(http.Header)
	md, ok := metadata.FromOutgoingContext(ctx)
	if ok {
		for k, v := range md {
			for _, vv := range v {
				h.Add(k, vv)
			}
		}
	}
	s.transport.SetRequestHeader(h)

	if err := s.transport.Send(ctx, r); err != nil {
		return errors.Wrap(err, "failed to send the request")
	}
	return nil
}

func (s *clientStream) CloseAndReceive(ctx context.Context, res interface{}) error {
	if err := s.transport.CloseSend(); err != nil {
		return errors.Wrap(err, "failed to close the send stream")
	}

	s.closed.Store(true)

	rawBody, err := s.transport.Receive(ctx)
	if s.isTrailerOnly(err) {
		// Parse headers as trailers.
		trailer, err := s.Header()
		if err != nil {
			return errors.Wrap(err, "failed to get header instead of trailer")
		}
		s.trailerMu.Lock()
		s.trailerMD = trailer
		s.trailersOnly.Store(true)
		s.trailerMu.Unlock()

		// Try to extract *status.Status from headers.
		return statusFromHeader(trailer).Err()
	}
	if err != nil {
		return errors.Wrap(err, "failed to receive the response")
	}

	var closeOnce sync.Once
	defer closeOnce.Do(func() { rawBody.Close() })

	resHeader, err := parser.ParseResponseHeader(rawBody)
	if err != nil {
		return errors.Wrap(err, "failed to parse response header")
	}

	if resHeader.IsMessageHeader() {
		resBody, err := parser.ParseLengthPrefixedMessage(rawBody, resHeader.ContentLength)
		if err != nil {
			return errors.Wrap(err, "failed to parse the response body")
		}
		codec := s.callOptions.codec
		if err := codec.Unmarshal(resBody, res); err != nil {
			return errors.Wrapf(err, "failed to unmarshal response body by codec %s", codec.Name())
		}

		closeOnce.Do(func() { rawBody.Close() })

		// improbable-eng/grpc-web returns the trailer in another message.
		rawBody2, err := s.transport.Receive(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to receive the response trailer")
		}
		defer rawBody2.Close()
		rawBody = rawBody2

		resHeader, err = parser.ParseResponseHeader(rawBody2)
		if err != nil {
			return errors.Wrap(err, "failed to parse response header2")
		}
	}
	if !resHeader.IsTrailerHeader() {
		return errors.New("unexpected header")
	}

	status, trailer, err := parser.ParseStatusAndTrailer(rawBody, resHeader.ContentLength)
	if err != nil {
		return errors.Wrap(err, "failed to parse status and trailer")
	}
	s.trailerMu.Lock()
	defer s.trailerMu.Unlock()
	s.trailerMD = trailer
	return status.Err()
}

func (s *clientStream) isTrailerOnly(err error) bool {
	return errors.Is(err, io.ErrUnexpectedEOF) && s.trailer().Len() == 0
}

type ServerStream interface {
	// Header returns the header metadata from the server, if there is any.
	// It blocks if the metadata is not ready to read.
	Header() (metadata.MD, error)
	// Trailer returns the trailer metadata from the server, if there is any.
	// It must only be called after stream.CloseAndReceive has returned, or
	// stream.Receive has returned a non-nil error (including io.EOF).
	Trailer() metadata.MD
	Send(ctx context.Context, req interface{}) error
	Receive(ctx context.Context, res interface{}) error
}

type serverStream struct {
	endpoint    string
	transport   transport.UnaryTransport
	resStream   io.ReadCloser
	callOptions *callOptions

	closed          bool
	header, trailer metadata.MD
}

func (s *serverStream) Header() (metadata.MD, error) {
	return s.header, nil
}

func (s *serverStream) Trailer() metadata.MD {
	if !s.closed {
		panic("Trailer must be called after stream.CloseAndReceive has been called")
	}
	return s.trailer
}

func (s *serverStream) Send(ctx context.Context, req interface{}) error {
	codec := s.callOptions.codec

	r, err := encodeRequestBody(codec, req)
	if err != nil {
		return errors.Wrap(err, "failed to build the request body")
	}

	md, ok := metadata.FromOutgoingContext(ctx)
	if ok {
		for k, v := range md {
			for _, vv := range v {
				s.transport.Header().Add(k, vv)
			}
		}
	}

	contentType := "application/grpc-web+" + codec.Name()
	header, rawBody, err := s.transport.Send(ctx, s.endpoint, contentType, r)
	if err != nil {
		return errors.Wrap(err, "failed to send the request")
	}
	s.header = toMetadata(header)
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
		msg, err := parser.ParseLengthPrefixedMessage(s.resStream, length)
		if err != nil {
			return err
		}
		if err := s.callOptions.codec.Unmarshal(msg, res); err != nil {
			return errors.Wrap(err, "failed to unmarshal response body")
		}
		return nil
	}

	status, trailer, err := parser.ParseStatusAndTrailer(s.resStream, length)
	if err != nil {
		return errors.Wrap(err, "failed to parse trailer")
	}
	s.closed = true
	s.trailer = trailer
	if status.Code() != codes.OK {
		return status.Err()
	}
	return io.EOF
}

type BidiStream interface {
	// Header returns the header metadata from the server, if there is any.
	// It blocks if the metadata is not ready to read.
	Header() (metadata.MD, error)
	// Trailer returns the trailer metadata from the server, if there is any.
	// It must only be called after stream.CloseAndReceive has returned, or
	// stream.Receive has returned a non-nil error (including io.EOF).
	Trailer() metadata.MD
	Send(ctx context.Context, req interface{}) error
	Receive(ctx context.Context, res interface{}) error
	CloseSend() error
}

type bidiStream struct {
	*clientStream

	sentCloseSend atomic.Bool
}

var (
	canonicalGRPCStatusBytes = []byte("Grpc-Status: ")
	gRPCStatusBytes          = []byte("grpc-status: ")
)

func (s *bidiStream) Receive(ctx context.Context, res interface{}) error {
	if s.closed.Load() {
		return io.EOF
	}

	rawBody, err := s.transport.Receive(ctx)
	if s.isTrailerOnly(err) {
		// Trailers-only responses, no message.

		s.closed.Store(true)

		// Parse headers as trailers.
		trailer, err := s.Header()
		if err != nil {
			return errors.Wrap(err, "failed to get header instead of trailer")
		}

		s.trailerMu.Lock()
		s.trailerMD = trailer
		s.trailersOnly.Store(true)
		s.trailerMu.Unlock()

		// Try to extract *status.Status from headers.
		return statusFromHeader(trailer).Err()
	}
	if err != nil {
		return errors.Wrap(err, "failed to receive the response")
	}

	resHeader, err := parser.ParseResponseHeader(rawBody)
	if err != nil {
		return errors.Wrap(err, "failed to parse response header")
	}

	switch {
	case resHeader.IsMessageHeader():
		msg, err := parser.ParseLengthPrefixedMessage(rawBody, resHeader.ContentLength)
		if err != nil {
			return err
		}
		if err := s.callOptions.codec.Unmarshal(msg, res); err != nil {
			return errors.Wrap(err, "failed to unmarshal response body")
		}
		return nil
	case resHeader.IsTrailerHeader():
		s.closed.Store(true)

		status, trailer, err := parser.ParseStatusAndTrailer(rawBody, resHeader.ContentLength)
		if err != nil {
			return errors.Wrap(err, "failed to parse trailer")
		}
		s.trailerMu.Lock()
		s.trailerMD = trailer
		s.trailerMu.Unlock()

		if status.Code() != codes.OK {
			return status.Err()
		}
		return io.EOF
	default:
		return errors.New("unexpected header")
	}
}

func (s *bidiStream) CloseSend() error {
	if err := s.transport.CloseSend(); err != nil {
		return errors.Wrap(err, "failed to close the send stream")
	}
	s.sentCloseSend.Store(true)
	return nil
}

func (s *bidiStream) isTrailerOnly(err error) bool {
	return s.sentCloseSend.Load() && s.clientStream.isTrailerOnly(err)
}

func statusFromHeader(h metadata.MD) *status.Status {
	msgs, codeStr := h.Get("grpc-message"), h.Get("grpc-status")
	if len(codeStr) == 0 {
		return status.New(codes.Unknown, "response closed without grpc-status (headers only)")
	}
	i, err := strconv.Atoi(codeStr[0])
	if err != nil {
		return status.New(codes.Unknown, err.Error())
	}
	return status.New(codes.Code(i), msgs[0])
}
