package main

import (
	"context"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/k0kubun/pp"
	"github.com/ktr0731/grpc-test/api"
	"github.com/ktr0731/grpc-web-go-client/grpcweb"
)

func parseProto(fname string) []*desc.FileDescriptor {
	p := &protoparse.Parser{
		ImportPaths: []string{"."},
	}
	d, err := p.ParseFiles(fname)
	if err != nil {
		panic(err)
	}
	return d
}

func main() {
	desc := parseProto("grpcweb/testdata/api.proto")
	client := grpcweb.NewClient("http://localhost:50051")
	svc := desc[0].GetServices()[0].AsServiceDescriptorProto()
	in, out := &api.SimpleRequest{Name: "ktr0731"}, &api.SimpleResponse{}
	req, err := grpcweb.NewRequest(svc, svc.Method[0], in, out)
	if err != nil {
		panic(err)
	}
	if err := client.Send(context.Background(), req); err != nil {
		panic(err)
	}

	pp.Println(out.GetMessage())
}
