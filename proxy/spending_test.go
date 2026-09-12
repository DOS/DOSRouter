package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/DOS/DOSRouter/logger"
)

func TestZeroTokenUsageIsNotAnEstimate(t *testing.T) {
	var entry logger.UsageEntry
	srv, sc := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"choices":[{"message":{"content":"No charge"},"finish_reason":"stop"}],"usage":{"prompt_tokens":0,"completion_tokens":0}}`)
	}, func(cfg *Config) { cfg.UsageLogger = func(e logger.UsageEntry) { entry = e } })
	got := syncTestChat(t, srv, `{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"zero"}]}`)
	if got.Code != 200 || entry.Cost != 0 || entry.CostSource != "tokens" {
		t.Fatalf("status=%d log=%+v", got.Code, entry)
	}
	if h := sc.GetHistory(); len(h) != 1 || h[0].Amount != 0 {
		t.Fatalf("history=%+v", h)
	}
}

func TestCallerCredentialsDoNotShareResponseCache(t *testing.T) {
	var calls atomic.Int32
	srv, _ := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); syncTestOK(w) }, nil)
	for _, credential := range []string{"Bearer test-a", "Bearer test-b"} {
		r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"same"}]}`))
		r.Header.Set("Authorization", credential)
		w := httptest.NewRecorder()
		srv.handleChatCompletions(w, r)
		if w.Code != 200 || w.Header().Get("X-DOSRouter-Cache") == "hit" || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("status=%d headers=%v", w.Code, w.Header())
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("upstream calls=%d", calls.Load())
	}
}
