package main

import (
	"testing"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/ktr0731/grpc-web-go-client/lib/grpcweb"
	"github.com/stretchr/testify/require"
)

func parseProto(t *testing.T, fname string) []*desc.FileDescriptor {
	t.Helper()

	p := &protoparse.Parser{
		ImportPaths: []string{"testdata"},
	}
	d, err := p.ParseFiles(fname)
	require.NoError(t, err)
	return d
}

func main() {
	grpcweb.NewClient("http://localhost:50051")
}
