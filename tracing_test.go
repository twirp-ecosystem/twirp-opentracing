package ottwirp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/iheanyi/twirptest"
	opentracing "github.com/opentracing/opentracing-go"
	ot "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/stretchr/testify/assert"
	"github.com/twitchtv/twirp"
)

func TestTracingHooks(t *testing.T) {
	tests := []struct {
		desc         string
		service      twirptest.Haberdasher
		expectedTags map[string]interface{}
		expectedLogs []mocktracer.MockLogRecord
		errExpected  bool
	}{
		{
			desc:    "sets tags with operation name for a valid requests",
			service: twirptest.NoopHatmaker(),
			expectedTags: map[string]interface{}{
				"package":          "twirptest",
				"component":        "twirp",
				"service":          "Haberdasher",
				"span.kind":        "server",
				"http.status_code": int64(200),
			},
			expectedLogs: []mocktracer.MockLogRecord{},
		},
		{
			desc:    "set tags and logs with operation name for an errored request",
			service: twirptest.ErroringHatmaker(errors.New("test")),
			expectedTags: map[string]interface{}{
				"package":          "twirptest",
				"component":        "twirp",
				"service":          "Haberdasher",
				"span.kind":        "server",
				"http.status_code": int64(500),
				"error":            true,
			},
			expectedLogs: []mocktracer.MockLogRecord{
				{
					Fields: []mocktracer.MockKeyValue{
						{
							Key:         "event",
							ValueKind:   reflect.String,
							ValueString: "error",
						},
						{
							Key:         "message",
							ValueKind:   reflect.String,
							ValueString: "test",
						},
					},
				},
			},
			errExpected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			tracer := setupMockTracer()
			hooks := NewOpenTracingHooks(tracer)

			server, client := serverAndClient(tt.service, hooks)
			defer server.Close()

			_, err := client.MakeHat(context.Background(), &twirptest.Size{})
			if err != nil && !tt.errExpected {
				t.Fatalf("twirptest Client err=%q", err)
			}

			rawSpan := tracer.FinishedSpans()[0]
			assert.Equal(t, tt.expectedTags, rawSpan.Tags(), "expected tags to match")

			actualLogs := rawSpan.Logs()
			zeroOutTimestamps(actualLogs)
			assert.Equal(t, tt.expectedLogs, actualLogs)

			assert.Equal(t, "MakeHat", rawSpan.OperationName, "expected operation name to be MakeHat")
		})
	}
}

func TestClientSpanInjection(t *testing.T) {
	tracer := setupMockTracer()
	hooks := NewOpenTracingHooks(tracer)

	server, client := TraceServerAndClient(twirptest.NoopHatmaker(), hooks, tracer)
	defer server.Close()

	span, ctx := ot.StartSpanFromContext(context.Background(), "TestClientSpanInjection", ext.SpanKindRPCClient)
	ctx, err := InjectClientSpan(ctx, span)
	if err != nil {
		t.Fatalf("error injecting span, err=%q", err)
	}

	// Check Trace Headers
	header, _ := twirp.HTTPRequestHeaders(ctx)
	assert.Regexp(t, "\\d+", header["Mockpfx-Ids-Spanid"], "Mockpfx-Ids-Spanid")
	assert.Regexp(t, "\\d+", header["Mockpfx-Ids-Traceid"], "Mockpfx-Ids-Traceid")
	assert.Equal(t, []string{"true"}, header["Mockpfx-Ids-Sampled"], "Mockpfx-Ids-Sampled")

	_, err = client.MakeHat(ctx, &twirptest.Size{})
	if err != nil {
		t.Fatalf("twirptest client err=%q", err)
	}
	span.Finish()
	clientSpan := tracer.FinishedSpans()[1]
	serverSpan := tracer.FinishedSpans()[0]
	expectedTags := map[string]interface{}{
		"span.kind": ext.SpanKindEnum("client"),
	}
	assert.Equal(t, expectedTags, clientSpan.Tags(), "expected tags to match")
	assert.Equal(t, serverSpan.SpanContext.TraceID, clientSpan.SpanContext.TraceID, "expected span to propagate properly")
}

func serverAndClient(h twirptest.Haberdasher, hooks *twirp.ServerHooks) (*httptest.Server, twirptest.Haberdasher) {
	return twirptest.ServerAndClient(h, hooks)
}

func TraceServerAndClient(h twirptest.Haberdasher, hooks *twirp.ServerHooks, tracer opentracing.Tracer) (*httptest.Server, twirptest.Haberdasher) {
	s := httptest.NewServer(WithTraceContext(twirptest.NewHaberdasherServer(h, hooks), tracer))
	c := twirptest.NewHaberdasherProtobufClient(s.URL, http.DefaultClient)
	return s, c
}

func setupMockTracer() *mocktracer.MockTracer {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	return tracer
}

func zeroOutTimestamps(recs []mocktracer.MockLogRecord) {
	for i := range recs {
		recs[i].Timestamp = time.Time{}
	}
}
