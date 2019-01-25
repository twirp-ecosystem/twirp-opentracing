package ottwirp

import (
	"context"
	"strconv"

	ot "github.com/opentracing/opentracing-go"
	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/twitchtv/twirp"
)

const (
	RequestReceivedEvent = "request.received"
)

// TODO: Add functional options for things such as filtering or maybe logging
// custom fields?

// NewOpenTracingHooks provides a twirp.ServerHooks struct which records
// OpenTracing spans.
func NewOpenTracingHooks(tracer ot.Tracer) *twirp.ServerHooks {
	hooks := &twirp.ServerHooks{}

	// RequestReceived: Create the initial span that we will use for the duration
	// of the request.
	hooks.RequestReceived = func(ctx context.Context) (context.Context, error) {
		// Create the initial span, it won't have a method name just yet.
		span, ctx := ot.StartSpanFromContext(ctx, RequestReceivedEvent)
		if span != nil {
			span.SetTag("component", "twirp")
			span.SetTag("span.kind", "server")

			packageName, havePackageName := twirp.PackageName(ctx)
			if havePackageName {
				span.SetTag("package", packageName)
			}

			serviceName, haveServiceName := twirp.ServiceName(ctx)
			if haveServiceName {
				span.SetTag("service", serviceName)
			}
		}

		return ctx, nil
	}

	// RequestRouted: Set the operation name based on the MethodName extracted
	// from span.
	hooks.RequestRouted = func(ctx context.Context) (context.Context, error) {
		span := ot.SpanFromContext(ctx)
		if span != nil {
			method, ok := twirp.MethodName(ctx)
			if !ok {
				return ctx, nil
			}

			span.SetOperationName(method)
		}

		return ctx, nil
	}

	// ResponseSent: Set the status code and mark the span as finished.
	hooks.ResponseSent = func(ctx context.Context) {
		span := ot.SpanFromContext(ctx)
		if span != nil {
			status, haveStatus := twirp.StatusCode(ctx)
			code, err := strconv.ParseInt(status, 10, 64)
			if haveStatus && err == nil {
				// TODO: Check the status code, if it's a non-2xx/3xx status code, we
				// should probably mark it as an error of sorts.
				span.SetTag("http.status_code", code)
			}

			span.Finish()
		}
	}

	// Error: Set "error" as true and log the error event and the human readable
	// error message.
	hooks.Error = func(ctx context.Context, err twirp.Error) context.Context {
		span := ot.SpanFromContext(ctx)
		if span != nil {
			span.SetTag("error", true)
			span.LogFields(otlog.String("event", "error"), otlog.String("message", err.Msg()))
		}

		return ctx
	}

	return hooks
}
