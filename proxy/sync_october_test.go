package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DOS/DOSRouter/router"
)

func TestSyncRecurringFailuresAreTriedLast(t *testing.T) {
	calls := []string{}
	srv, _ := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		json.NewDecoder(r.Body).Decode(&req)
		calls = append(calls, req.Model)
		if req.Model == "openai/gpt-4o-mini" {
			http.Error(w, "model failure", 502)
			return
		}
		syncTestOK(w)
	}, func(c *Config) {
		cfg := router.DefaultRoutingConfig()
		for tier := range cfg.Tiers {
			cfg.Tiers[tier] = router.TierConfig{Primary: "openai/gpt-4o-mini", Fallback: []string{"deepseek/deepseek-chat"}}
		}
		c.RoutingConfig = &cfg
	})
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(fmt.Sprintf(`{"model":"auto","messages":[{"role":"user","content":"hello %d"}]}`, i)))
		req.Header.Set("Authorization", "Bearer test-only")
		w := httptest.NewRecorder()
		srv.handleChatCompletions(w, req)
		if w.Code != 200 {
			t.Fatalf("request %d: %d %s", i, w.Code, w.Body.String())
		}
	}
	if len(calls) != 7 || calls[len(calls)-1] != "deepseek/deepseek-chat" {
		t.Fatalf("recurring failure was not demoted: %v", calls)
	}
	w := syncTestChat(t, srv, `{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"pinned"}]}`)
	if w.Code != 502 || calls[len(calls)-1] != "openai/gpt-4o-mini" {
		t.Fatal("explicit pin displaced")
	}
}

func TestSyncEmptyTurnsUseWholeFallbackChain(t *testing.T) {
	calls := 0
	srv, _ := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			io.WriteString(w, `{"choices":[{"message":{"content":""},"finish_reason":"stop"}]}`)
			return
		}
		syncTestOK(w)
	}, func(c *Config) {
		cfg := router.DefaultRoutingConfig()
		for tier := range cfg.Tiers {
			cfg.Tiers[tier] = router.TierConfig{Primary: "openai/gpt-4o-mini", Fallback: []string{"deepseek/deepseek-chat", "google/gemini-3.8-flash"}}
		}
		c.RoutingConfig = &cfg
	})
	w := syncTestChat(t, srv, `{"model":"auto","messages":[{"role":"user","content":"hello"}]}`)
	if w.Code != 200 || calls != 3 || !strings.Contains(w.Body.String(), "Done.") {
		t.Fatalf("empty turn recovery: status=%d calls=%d body=%s", w.Code, calls, w.Body.String())
	}
}
