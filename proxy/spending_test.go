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

func TestRecoveredCallsHaveUniqueIDsAndDeclaredNames(t *testing.T) {
	tools := []byte(`[{"type":"function","function":{"name":"read_file"}}]`)
	input := "First. call:read_file({}) Then. call:read_file({}) call:undeclared({})"
	calls, prose := recoverToolCallsWithProse(input, tools)
	if len(calls) != 2 || calls[0]["id"] == calls[1]["id"] {
		t.Fatalf("calls=%v", calls)
	}
	if !strings.Contains(prose, "call:undeclared({})") || strings.Contains(prose, "call:read_file") {
		t.Fatalf("prose=%q", prose)
	}
}

func TestToolAndGatewayEdgeCases(t *testing.T) {
	for _, raw := range []string{"null", " null ", "[]", "", `{"name":"not-an-array"}`} {
		if requestHasTools([]byte(raw)) {
			t.Errorf("has tools for %q", raw)
		}
		calls, _ := recoverToolCallsWithProse("call:read_file({})", []byte(raw))
		if len(calls) != 0 {
			t.Errorf("recovered undeclared calls for %q", raw)
		}
	}
	for _, input := range []string{"//localhost:8080", "localhost", ""} {
		if origin := gatewayOrigin(input); origin != "" {
			t.Errorf("origin(%q)=%q", input, origin)
		}
	}
	if got := gatewayOrigin("https://example.com/v1?not=public"); got != "https://example.com" {
		t.Fatalf("origin=%q", got)
	}
}

type testWrappedTransport struct{ http.RoundTripper }

func TestNewAcceptsWrappedDefaultTransport(t *testing.T) {
	original := http.DefaultTransport
	wrapper := &testWrappedTransport{original}
	http.DefaultTransport = wrapper
	t.Cleanup(func() { http.DefaultTransport = original })
	srv, _ := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) { syncTestOK(w) }, nil)
	if srv.httpClient.Transport != wrapper {
		t.Fatal("wrapped transport was not preserved")
	}
}

func TestConfiguredGatewayDoesNotShareCallerCache(t *testing.T) {
	var calls atomic.Int32
	srv, _ := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); syncTestOK(w) }, func(cfg *Config) { cfg.UpstreamAPIKey = "test-gateway-key" })
	for _, credential := range []string{"Bearer test-a", "Bearer test-b"} {
		r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"same gateway"}]}`))
		r.Header.Set("Authorization", credential)
		w := httptest.NewRecorder()
		srv.handleChatCompletions(w, r)
		if w.Code != 200 || w.Header().Get("X-DOSRouter-Cache") == "hit" {
			t.Fatalf("status=%d headers=%v", w.Code, w.Header())
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("calls=%d", calls.Load())
	}
}

func TestNonThinkingControlTokensKeepVisibleProse(t *testing.T) {
	for _, text := range []string{"<|begin_of_text|>Visible answer", "<|end_of_turn|>Still visible"} {
		if got := stripThinking(text); got != text {
			t.Errorf("stripThinking(%q)=%q", text, got)
		}
	}
	if got := stripThinking("<｜begin▁of▁thinking｜>private<｜end▁of▁thinking｜>Visible"); got != "Visible" {
		t.Errorf("thinking=%q", got)
	}
}

func TestMalformedStreamingUsageRetainsEstimate(t *testing.T) {
	for _, usage := range []string{`{"prompt_tokens":null,"completion_tokens":null}`, `{"prompt_tokens":"10","completion_tokens":5}`, `{"prompt_tokens":1.5,"completion_tokens":0}`, `{"prompt_tokens":-1,"completion_tokens":0}`} {
		t.Run(usage, func(t *testing.T) {
			var entry logger.UsageEntry
			srv, _ := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, "data: {\"choices\":[],\"usage\":"+usage+"}\n\ndata: [DONE]\n\n")
			}, func(cfg *Config) { cfg.UsageLogger = func(e logger.UsageEntry) { entry = e } })
			w := syncTestChat(t, srv, `{"model":"openai/gpt-4o-mini","stream":true,"messages":[{"role":"user","content":"stream"}]}`)
			if w.Code != 200 || entry.CostSource != "estimate" || entry.Cost <= 0 {
				t.Fatalf("status=%d entry=%+v", w.Code, entry)
			}
		})
	}
}

func TestPaidStatusResponseDoesNotRepeatSameReservation(t *testing.T) {
	for _, status := range []int{429, 502, 503, 504} {
		var calls atomic.Int32
		srv, sc := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.Header().Set("X-Blockrun-Cost-USD", "0.002")
			http.Error(w, "failed after processing", status)
		}, nil)
		w := syncTestChat(t, srv, `{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"no replay"}]}`)
		if w.Code != 502 || calls.Load() != 1 {
			t.Fatalf("upstream=%d status=%d calls=%d", status, w.Code, calls.Load())
		}
		history := sc.GetHistory()
		if len(history) != 1 || history[0].Amount != 0.002 {
			t.Fatalf("settled error history=%v", history)
		}
	}
}
