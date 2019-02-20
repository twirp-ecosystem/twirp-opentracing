package ottwirp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iheanyi/twirptest"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/stretchr/testify/assert"
	"github.com/twitchtv/twirp"
)

func TestTraceHTTPClient(t *testing.T) {
	tests := []struct {
		desc         string
		errExpected  bool
		service      twirptest.Haberdasher
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
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			tracer := setupMockTracer()
			hooks := NewOpenTracingHooks(tracer)
			server, client := TraceServerAndTraceClient(tt.service, hooks, tracer)
			defer server.Close()

			_, err := client.MakeHat(context.Background(), &twirptest.Size{})
			if err != nil {
				if !tt.errExpected {
					t.Fatalf("twirptest client err=%q", err)
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

func TraceServerAndTraceClient(h twirptest.Haberdasher, hooks *twirp.ServerHooks, tracer opentracing.Tracer) (*httptest.Server, twirptest.Haberdasher) {
	s := httptest.NewServer(WithTraceContext(twirptest.NewHaberdasherServer(h, hooks), tracer))
	c := twirptest.NewHaberdasherProtobufClient(s.URL, NewTraceHTTPClient(http.DefaultClient, tracer))
	return s, c
}
