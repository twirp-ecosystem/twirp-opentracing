package ottwirp

import (
	"io"
	"net/http"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/twitchtv/twirp"
)

// Doer as the interface used to "Do" HTTP requests.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// TraceHTTPClient wraps a provided http.Client and tracer for instrumenting
// requests.
type TraceHTTPClient struct {
	client Doer
	tracer opentracing.Tracer
}

func NewTraceHTTPClient(client Doer, tracer opentracing.Tracer) Doer {
	return &TraceHTTPClient{
		client: client,
		tracer: tracer,
	}
}

// Do injects the tracing headers into the tracer and updates the headers before
// making the actual request.
func (c *TraceHTTPClient) Do(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	methodName, ok := twirp.MethodName(ctx)
	if !ok {
		// No method name, let's use the URL path instead then.
		methodName = req.URL.Path
	}
	span, ctx := opentracing.StartSpanFromContext(ctx, methodName, ext.SpanKindRPCClient)
	ext.HTTPMethod.Set(span, req.Method)
	ext.HTTPUrl.Set(span, req.URL.String())

	err := c.tracer.Inject(span.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header),
	)
	if err != nil {
		span.LogFields(otlog.String("event", "tracer.Inject() failed"), otlog.Error(err))
	}
	req = req.WithContext(ctx)

	res, err := c.client.Do(req)
	if err != nil {
		span.SetTag("error", true)
		span.LogFields(otlog.String("event", "error"), otlog.String("message", err.Error()))
		span.Finish()
		return res, err
	}
	ext.HTTPStatusCode.Set(span, uint16(res.StatusCode))

	// We want to track when the body is closed, meaning the server is done with
	// the response.
	res.Body = closer{res.Body, span}
	return res, nil
}

type closer struct {
	io.ReadCloser
	sp opentracing.Span
}

func (c closer) Close() error {
	err := c.ReadCloser.Close()
	c.sp.Finish()
	return err
}
