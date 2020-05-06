# OpenTracing Hooks for Twirp

![CI](https://github.com/twirp-ecosystem/twirp-opentracing/workflows/CI/badge.svg)

The `ottwirp` package creates an OpenTracing Twirp hook to use in your server. Derived from [grpc-opentracing](https://github.com/grpc-ecosystem/grpc-opentracing).

## Installation

`go get -u github.com/twirp-ecosystem/twirp-opentracing`

## Server-side usage example

Where you are instantiating your Twirp server:

```go
var tracer opentracing.Tracer = ...

...

hooks := NewOpenTracingHooks(tracer)
service := haberdasherserver.New()
server := WithTraceContext(haberdasher.NewHaberdasherServer(service, hooks), tracer)
log.Fatal(http.ListenAndServe(":8080", server))
```

## Client-side usage example

When instantiating your Twirp client:

```go
var tracer opentracing.Tracer = ...

...

client := haberdasher.NewHaberdasherProtobufClient(url, NewTraceHTTPClient(http.DefaultClient, tracer))
```
