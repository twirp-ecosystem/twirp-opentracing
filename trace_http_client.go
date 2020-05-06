package ottwirp

import (
	"io"
	"net/http"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/twitchtv/twirp"
)

// HTTPClient as an interface that models *http.Client.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// TraceHTTPClient wraps a provided http.Client and tracer for instrumenting
// requests.
type TraceHTTPClient struct {
	client HTTPClient
	tracer opentracing.Tracer
	opts *ClientOptions
}

// ClientOptions is a struct containing tracing client configuration options. Options are exposed as functional options
// which can be set in the NewTraceHTTPClient call
type ClientOptions struct {
	includeUserErrors bool
}

// ClientOption is a functional option used to configure a TraceHTTPClient
type ClientOption func(clientOpts *ClientOptions)

// IncludeUserErrors, if set, will report client errors (4xx) as errors in the span.
// If not set, only 5xx status will be reported as erroneous to the tracer.
func IncludeUserErrors(includeUserErrors bool) ClientOption {
	return func(clientOpts *ClientOptions) {
		clientOpts.includeUserErrors = includeUserErrors
	}
}

var _ HTTPClient = (*TraceHTTPClient)(nil)

func NewTraceHTTPClient(client HTTPClient, tracer opentracing.Tracer, opts ...ClientOption) *TraceHTTPClient {
	if client == nil {
		client = http.DefaultClient
	}

	clientOpts := &ClientOptions{
		includeUserErrors: true,
	}

	for _, opt := range opts {
		opt(clientOpts)
	}

	return &TraceHTTPClient{
		client: client,
		tracer: tracer,
		opts: clientOpts,
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
		setErrorSpan(span, err.Error())
		span.Finish()
		return res, err
	}
	ext.HTTPStatusCode.Set(span, uint16(res.StatusCode))

	// Check for error codes greater than 400 if withUserErr is set and codes greater than 500 if not,
	// and mark the span as an error if appropriate.
	if res.StatusCode >= 400 && c.opts.includeUserErrors || res.StatusCode >= 500 {
		span.SetTag("error", true)
	}

	// We want to track when the body is closed, meaning the server is done with
	// the response.
	res.Body = closer{
		ReadCloser: res.Body,
		span:       span,
	}
	return res, nil
}

type closer struct {
	io.ReadCloser
	span opentracing.Span
}

func (c closer) Close() error {
	err := c.ReadCloser.Close()
	c.span.Finish()
	return err
}

func setErrorSpan(span opentracing.Span, errorMessage string) {
	span.SetTag("error", true)
	span.LogFields(otlog.String("event", "error"), otlog.String("message", errorMessage))
}
