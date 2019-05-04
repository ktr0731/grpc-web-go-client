package grpcweb

import (
	"context"

	"google.golang.org/grpc"
)

type ClientConn struct{}

func DialContext(target string, opts ...DialOption) (*ClientConn, error) {
	return nil, nil
}

func (c *ClientConn) Invoke(ctx context.Context, method string, args, reply interface{}, opts ...CallOption) error {
	return nil
}

func (c *ClientConn) NewClientStram(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...CallOption) (ClientStram, error) {
	return nil, nil
}

func (c *ClientConn) NewServerStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...CallOption) (ServerStream, error) {
	return nil, nil
}
