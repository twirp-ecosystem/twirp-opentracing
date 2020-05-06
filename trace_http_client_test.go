package ottwirp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/stretchr/testify/assert"
	"github.com/twirp-ecosystem/twirptest"
	"github.com/twitchtv/twirp"
)

func TestTraceHTTPClient(t *testing.T) {
	tests := []struct {
		desc         string
		errExpected  bool
		service      twirptest.Haberdasher
		clientOpts   []TraceOption
		expectedTags func(*httptest.Server) map[string]interface{}
	}{
		{
			desc:        "properly traces valid requests",
			errExpected: false,
			service:     twirptest.NoopHatmaker(),
			expectedTags: func(server *httptest.Server) map[string]interface{} {
				return map[string]interface{}{
					"span.kind":        ext.SpanKindEnum("client"),
					"http.status_code": uint16(200),
					"http.url":         fmt.Sprintf("%s/twirp/twirptest.Haberdasher/MakeHat", server.URL),
					"http.method":      "POST",
				}
			},
		},
		{
			desc:        "properly sets metadata for errors",
			errExpected: true,
			service:     twirptest.ErroringHatmaker(errors.New("test")),
			expectedTags: func(server *httptest.Server) map[string]interface{} {
				return map[string]interface{}{
					"span.kind":        ext.SpanKindEnum("client"),
					"error":            true,
					"http.status_code": uint16(500),
					"http.url":         fmt.Sprintf("%s/twirp/twirptest.Haberdasher/MakeHat", server.URL),
					"http.method":      "POST",
				}
			},
		},
		{
			desc:        "does not report client errors in span if correct option is set",
			errExpected: true,
			service:     twirptest.ErroringHatmaker(twirp.NotFoundError("not found")),
			clientOpts:  []TraceOption{IncludeClientErrors(false)},
			expectedTags: func(server *httptest.Server) map[string]interface{} {
				return map[string]interface{}{
					"span.kind":        ext.SpanKindEnum("client"),
					"http.status_code": uint16(404),
					"http.url":         fmt.Sprintf("%s/twirp/twirptest.Haberdasher/MakeHat", server.URL),
					"http.method":      "POST",
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			tracer := setupMockTracer()
			hooks := NewOpenTracingHooks(tracer)
			server, client := TraceServerAndTraceClient(tt.service, hooks, tracer, tt.clientOpts...)
			defer server.Close()

			_, err := client.MakeHat(context.Background(), &twirptest.Size{})
			if err != nil {
				if !tt.errExpected {
					t.Fatalf("twirptest client err=%q", err)
				} else {
					assert.Error(t, err, "expected an error")
				}
			}
			clientSpan := tracer.FinishedSpans()[1]
			serverSpan := tracer.FinishedSpans()[0]
			assert.Equal(t, clientSpan.OperationName, "MakeHat", "expected operation name to be MakeHat")
			assert.Equal(t, tt.expectedTags(server), clientSpan.Tags(), "expected tags to match")
			assert.Equal(t, serverSpan.SpanContext.TraceID, clientSpan.SpanContext.TraceID, "expected trace to propagate properly")
			assert.Equal(t, serverSpan.ParentID, clientSpan.SpanContext.SpanID, "expected span to propagate properly")
		})
	}
}

func TraceServerAndTraceClient(h twirptest.Haberdasher, hooks *twirp.ServerHooks, tracer opentracing.Tracer, opts ...TraceOption) (*httptest.Server, twirptest.Haberdasher) {
	s := httptest.NewServer(WithTraceContext(twirptest.NewHaberdasherServer(h, hooks), tracer))
	c := twirptest.NewHaberdasherProtobufClient(s.URL, NewTraceHTTPClient(http.DefaultClient, tracer, opts...))
	return s, c
}
