package ottwirp

import (
	"context"
	"errors"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/stretchr/testify/assert"
	"github.com/twitchtv/twirp"
	"github.com/twitchtv/twirp/twirptest"
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
			desc:    "sets tags with operation name for a valid requestS",
			service: twirptest.NoopHatmaker(),
			expectedTags: map[string]interface{}{
				"package":          "twirp.twirptest",
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
				"package":          "twirp.twirptest",
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
			hooks := NewOpenTracingServerHook(tracer)

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
