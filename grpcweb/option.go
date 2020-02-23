package grpcweb

import (
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/metadata"
)

var (
	defaultDialOptions = dialOptions{}
	defaultCallOptions = callOptions{
		codec: encoding.GetCodec(proto.Name),
	}
)

type dialOptions struct {
	defaultCallOptions   []CallOption
	insecure             bool
	transportCredentials credentials.TransportCredentials
}

type DialOption func(*dialOptions)

func WithDefaultCallOptions(opts ...CallOption) DialOption {
	return func(opt *dialOptions) {
		opt.defaultCallOptions = opts
	}
}

func WithInsecure() DialOption {
	return func(opt *dialOptions) {
		opt.insecure = true
	}
}

func WithTransportCredentials(creds credentials.TransportCredentials) DialOption {
	return func(opt *dialOptions) {
		opt.transportCredentials = creds
	}
}

type callOptions struct {
	codec           encoding.Codec
	header, trailer *metadata.MD
}

type CallOption func(*callOptions)

func ForceCodec(codec encoding.Codec) CallOption {
	return func(opt *callOptions) {
		opt.codec = codec
	}
}

func Header(h *metadata.MD) CallOption {
	return func(opt *callOptions) {
		opt.header = h
	}
}

func Trailer(t *metadata.MD) CallOption {
	return func(opt *callOptions) {
		opt.trailer = t
	}
}
