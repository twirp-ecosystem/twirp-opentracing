package ottwirp

import (
	"context"
	"net/http/httptest"
	"testing"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/stretchr/testify/assert"
	"github.com/twitchtv/twirp"
	"github.com/twitchtv/twirp/twirptest"
)

func TestTracingHooks(t *testing.T) {
	tracer := setupMockTracer()
	hooks := NewOpenTracingServerHook(tracer)

	server, client := serverAndClient(hooks)
	defer server.Close()

	_, err := client.MakeHat(context.Background(), &twirptest.Size{})
	if err != nil {
		t.Fatalf("twirptest Client err=%q", err)
	}

	rawSpan := tracer.FinishedSpans()[0]
	expectedTags := map[string]interface{}{
		"package":          "twirp.twirptest",
		"component":        "twirp",
		"service":          "Haberdasher",
		"span.kind":        "server",
		"http.status_code": int64(200),
	}

	assert.Equal(t, expectedTags, rawSpan.Tags(), "expected tags to match")
	assert.Equal(t, "MakeHat", rawSpan.OperationName, "expected operatino name to be MakeHat")
}

func serverAndClient(hooks *twirp.ServerHooks) (*httptest.Server, twirptest.Haberdasher) {
	return twirptest.ServerAndClient(twirptest.NoopHatmaker(), hooks)
}

func setupMockTracer() *mocktracer.MockTracer {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	return tracer
}
