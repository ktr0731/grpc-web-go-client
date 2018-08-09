# gRPC Web Go client

gRPC Web client written in Go

## Usage
The server is [here](github.com/ktr0731/grpc-test).  

Send an unary request.

``` go
client := grpcweb.NewClient("localhost:50051")

in, out := new(api.SimpleRequest), new(api.SimpleResponse)
in.Name = "ktr"

// You can get the endpoint from grpcweb.ToEndpoint function with descriptors.
// However, I write directly in this example.
req := grpcweb.NewRequest("/api.Example/Unary", in, out)

res, err := client.Unary(context.Background(), req)
if err != nil {
  log.Fatal(err)
}

// hello, ktr
fmt.Println(res.Content.(*api.SimpleResponse).GetMessage())
```

Send a server-side streaming request.
``` go
req := grpcweb.NewRequest("/api.Example/ServerStreaming", in, out)

stream, err := client.ServerStreaming(context.Background(), req)
if err != nil {
  log.Fatal(err)
}

for {
  res, err := stream.Receive()
  if err == io.EOF {
    break
  }
  if err != nil {
    log.Fatal(err)
  }
  fmt.Println(res.Content.(*api.SimpleResponse).GetMessage())
}
```

Send an client-side streaming request.
``` go
stream, err := client.ClientStreaming(context.Background())
if err != nil {
  log.Fatal(err)
}

in, out := new(api.SimpleRequest), new(api.SimpleResponse)
in.Name = "ktr"
req := grpcweb.NewRequest("/api.Example/ClientStreaming", in, out)

for i := 0; i < 10; i++ {
  err := stream.Send(req)
  if err == io.EOF {
    break
  }
  if err != nil {
    log.Fatal(err)
  }
}

res, err := stream.CloseAndReceive()
if err != nil {
  log.Fatal(err)
}

// ktr, you greet 10 times.
fmt.Println(res.Content.(*api.SimpleResponse).GetMessage())
```

Send a bidirectional streaming request.
``` go
in, out := new(api.SimpleRequest), new(api.SimpleResponse)
req := grpcweb.NewRequest("/api.Example/BidiStreaming", in, out)

stream := client.BidiStreaming(context.Background(), req)

go func() {
  for {
    res, err := stream.Receive()
    if err == grpcweb.ErrConnectionClosed {
      return
    }
    if err != nil {
      log.Fatal(err)
    }
    fmt.Println(res.Content.(*api.SimpleResponse).GetMessage())
  }
}()

for i := 0; i < 2; i++ {
  in.Name = fmt.Sprintf("ktr%d", i+1)
  req := grpcweb.NewRequest("/api.Example/BidiStreaming", in, out)

  err := stream.Send(req)
  if err == io.EOF {
    break
  }
  if err != nil {
    log.Fatal(err)
  }
}

// wait a moment to get responses.
time.Sleep(10 * time.Second)

if err := stream.Close(); err != nil {
  log.Fatal(err)
}
```
