package ottwirp

import (
	"context"
	"errors"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/stretchr/testify/assert"
	"github.com/twirp-ecosystem/twirptest"
	"github.com/twitchtv/twirp"
)

func TestTracingHooks(t *testing.T) {
	serverType := ext.SpanKindEnum("server")
	tests := []struct {
		desc         string
		service      twirptest.Haberdasher
		traceOpts    []TraceOption
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
				"span.kind":        serverType,
				"http.status_code": int64(200),
			},
			expectedLogs: []mocktracer.MockLogRecord{},
		},
		{
			desc:    "set tags and logs with additional tags",
			service: twirptest.NoopHatmaker(),
			traceOpts: []TraceOption{
				WithTags(
					TraceTag{"foo", "bar"},
					TraceTag{"city", "tokyo"},
				),
			},
			expectedTags: map[string]interface{}{
				"package":          "twirptest",
				"component":        "twirp",
				"service":          "Haberdasher",
				"span.kind":        serverType,
				"http.status_code": int64(200),
				"foo":              "bar",
				"city":             "tokyo",
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
				"span.kind":        serverType,
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
		{
			desc:      "user error should be not reported as an erroneous span when correct option is set",
			service:   twirptest.ErroringHatmaker(twirp.NotFoundError("not found")),
			traceOpts: []TraceOption{IncludeClientErrors(false)},
			expectedTags: map[string]interface{}{
				"package":          "twirptest",
				"component":        "twirp",
				"service":          "Haberdasher",
				"span.kind":        serverType,
				"http.status_code": int64(404),
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
							ValueString: "not found",
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
			hooks := NewOpenTracingHooks(tracer, tt.traceOpts...)

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

func serverAndClient(h twirptest.Haberdasher, hooks *twirp.ServerHooks) (*httptest.Server, twirptest.Haberdasher) {
	return twirptest.ServerAndClient(h, hooks)
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
