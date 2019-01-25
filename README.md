# OpenTracing Hooks for Twirp

The `ottwirp` package creates an OpenTracing Twirp hook to use in your server.

## Installation

`go get -u github.com/iheanyi/twirp-opentracing`

## Server-side usage example

Where you are instantiating your Twirp server:

```go
var tracer opentracing.Tracer = ...

...

hooks := NewOpenTracingHooks(tracer)
service := haberdasherserver.New()
server := haberdasher.NewHaberdasherServer(service, hooks)
log.Fatal(http.ListenAndServe(":8080", server))
```
