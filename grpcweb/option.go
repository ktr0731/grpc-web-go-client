package grpcweb

import (
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
)

var (
	defaultDialOptions = dialOptions{}
	defaultCallOptions = callOptions{
		codec: encoding.GetCodec(proto.Name),
	}
)

type dialOptions struct {
	defaultCallOptions []CallOption
}

type DialOption func(*dialOptions)

func WithDefaultCallOptions(opts ...CallOption) DialOption {
	return func(opt *dialOptions) {
		opt.defaultCallOptions = opts
	}
}

type callOptions struct {
	codec encoding.Codec
}

type CallOption func(*callOptions)

func ForceCodec(codec encoding.Codec) CallOption {
	return func(opt *callOptions) {
		opt.codec = codec
	}
}
