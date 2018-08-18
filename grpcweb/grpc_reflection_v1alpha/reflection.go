package grpc_reflection_v1alpha

import (
	"errors"

	"github.com/ktr0731/grpc-web-go-client/grpcweb"
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
	pb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

type serverReflectionClient struct {
	cc *grpcweb.Client
}

func NewServerReflectionClient(cc *grpcweb.Client) pb.ServerReflectionClient {
	return &serverReflectionClient{cc}
}

func (c *serverReflectionClient) ServerReflectionInfo(ctx context.Context, opts ...grpc.CallOption) (pb.ServerReflection_ServerReflectionInfoClient, error) {
	if len(opts) != 0 {
		return nil, errors.New("currently, ktr0731/grpc-web-go-client does not support grpc.CallOption")
	}

	req := newRequest(nil)

	stream := c.cc.BidiStreaming(ctx, req)

	return &serverReflectionServerReflectionInfoClient{cc: stream}, nil
}

type serverReflectionServerReflectionInfoClient struct {
	cc grpcweb.BidiStreamClient

	// To satisfy pb.ServerReflection_ServerReflectionInfoClient
	grpc.ClientStream
}

func (x *serverReflectionServerReflectionInfoClient) Send(m *pb.ServerReflectionRequest) error {
	req := newRequest(m)
	return x.cc.Send(req)
}

func (x *serverReflectionServerReflectionInfoClient) Recv() (*pb.ServerReflectionResponse, error) {
	res, err := x.cc.Receive()
	if err != nil {
		return nil, err
	}
	return res.Content.(*pb.ServerReflectionResponse), nil
}

func (x *serverReflectionServerReflectionInfoClient) CloseSend() error {
	return x.cc.Close()
}

func newRequest(in *pb.ServerReflectionRequest) *grpcweb.Request {
	out := &pb.ServerReflectionResponse{}
	return grpcweb.NewRequest("/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo", in, out)
}
