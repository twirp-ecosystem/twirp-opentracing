package ottwirp

import (
	"context"
	"net/http"
	"strconv"

	ot "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/twitchtv/twirp"
)

const (
	RequestReceivedEvent = "request.received"
)

const (
	TracingInfoKey = "tracing-info"
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
		// Taken from: https://github.com/grpc-ecosystem/grpc-opentracing/blob/master/go/otgrpc/server.go#L93
		spanContext, err := extractSpanContext(ctx, tracer)
		if err != nil && err != ot.ErrSpanContextNotFound {
			// TODO: establish some sort of error reporting mechanism here. We
			// don't know where to put such an error and must rely on Tracer
			// implementations to do something appropriate for the time being.
		}
		// Create the initial span, it won't have a method name just yet.
		span, ctx := ot.StartSpanFromContext(ctx, RequestReceivedEvent, ext.RPCServerOption(spanContext))
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

// InjectSpan can be called by the client-side
func InjectClientSpan(ctx context.Context, span ot.Span) (context.Context, error) {
	header, ok := twirp.HTTPRequestHeaders(ctx)
	if !ok {
		header = http.Header{}
	}

	tracer := ot.GlobalTracer()
	tracer.Inject(span.Context(),
		ot.HTTPHeaders,
		ot.HTTPHeadersCarrier(header),
	)

	return twirp.WithHTTPRequestHeaders(ctx, header)
}

// WithTraceContext wraps the handler and extracts the span context from request
// headers to attach to the context for connecting client and server calls
// together.
func WithTraceContext(base http.Handler, tracer ot.Tracer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		carrier := ot.HTTPHeadersCarrier(r.Header)
		ctx = context.WithValue(ctx, TracingInfoKey, carrier)
		r = r.WithContext(ctx)

		base.ServeHTTP(w, r)
	})
}

func extractSpanContext(ctx context.Context, tracer ot.Tracer) (ot.SpanContext, error) {
	carrier := ctx.Value(TracingInfoKey)
	return tracer.Extract(ot.HTTPHeaders, carrier)
}
