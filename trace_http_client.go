package ottwirp

import (
	"io"
	"net/http"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/twitchtv/twirp"
)

type TraceHTTPClient struct {
	client *http.Client
	tracer opentracing.Tracer
}

func NewTraceHTTPClient(client *http.Client, tracer opentracing.Tracer) *TraceHTTPClient {
	// Perhaps get the global tracer here?
	return &TraceHTTPClient{
		client: client,
		tracer: tracer,
	}
}

func (c *TraceHTTPClient) Do(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	methodName, ok := twirp.MethodName(ctx)
	if !ok {
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
	// Set the HTTP status code from the service.
	ext.HTTPStatusCode.Set(span, uint16(res.StatusCode))

	// We want to finish recording metrics once the body is read.
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
