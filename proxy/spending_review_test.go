package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/DOS/DOSRouter/logger"
	"github.com/DOS/DOSRouter/router"
	"github.com/DOS/DOSRouter/session"
	"github.com/DOS/DOSRouter/spendcontrol"
)

func TestFlatPriceSettlementPreservesCompletionCount(t *testing.T) {
	const flatPrice = 0.125
	for _, tc := range []struct {
		name, n, usage, gateway string
		reserved, settled       float64
	}{
		{"default completion", "", `{"usage":{"prompt_tokens":10,"completion_tokens":5}}`, "", flatPrice, flatPrice},
		{"multiple completions", `,"n":3`, `{"usage":{"prompt_tokens":10,"completion_tokens":15}}`, "", 3 * flatPrice, 3 * flatPrice},
		{"zero tokens", `,"n":3`, `{"usage":{"prompt_tokens":0,"completion_tokens":0}}`, "", 3 * flatPrice, 3 * flatPrice},
		{"missing usage", `,"n":3`, `{}`, "", 3 * flatPrice, 3 * flatPrice},
		{"gateway total is authoritative", `,"n":3`, `{"usage":{"prompt_tokens":10,"completion_tokens":15}}`, "0.25", 3 * flatPrice, 0.25},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, sc := syncTestServer(t, func(http.ResponseWriter, *http.Request) {
				t.Error("unit settlement test unexpectedly dispatched upstream")
			}, nil)
			flat := flatPrice
			const model = "openai/gpt-4o-mini"
			srv.modelPricing[model] = router.ModelPricing{FlatPrice: &flat}
			if err := sc.SetLimit(spendcontrol.WindowSession, 1); err != nil {
				t.Fatal(err)
			}
			payload := []byte(`{"model":"` + model + `","messages":[{"role":"user","content":"test"}]` + tc.n + `}`)
			var req chatRequest
			if err := json.Unmarshal(payload, &req); err != nil {
				t.Fatal(err)
			}
			spend, allowed := srv.reserveChat(httptest.NewRecorder(), req, payload, model)
			if !allowed || spend == nil {
				t.Fatal("flat-price reservation rejected")
			}
			if got := sc.GetSpending()[spendcontrol.WindowSession]; got != tc.reserved {
				t.Fatalf("reserved=%v, want %v", got, tc.reserved)
			}
			spend.header = make(http.Header)
			spend.header.Set("X-DOS-Cost-USD", tc.gateway)
			spend.finish([]byte(tc.usage))
			spend.finish(nil)
			history := sc.GetHistory()
			if len(history) != 1 || history[0].Amount != tc.settled {
				t.Fatalf("settlement=%+v, want one charge of %v", history, tc.settled)
			}
			if got := sc.GetSpending()[spendcontrol.WindowSession]; got != tc.settled {
				t.Fatalf("settlement left a pending reservation: total=%v, want %v", got, tc.settled)
			}
		})
	}
}

func TestTruncatedEmptyTurnFallbackSettlesReservation(t *testing.T) {
	var attempts atomic.Int32
	var logs []logger.UsageEntry
	const firstModel = "openai/gpt-4o-mini"
	const nextModel = "openai/gpt-4o"
	srv, sc := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		switch attempts.Add(1) {
		case 1:
			if req.Model != firstModel {
				t.Errorf("first model=%q, want %q", req.Model, firstModel)
			}
			w.Header().Set("X-DOS-Cost-USD", "0.125")
			w.Header().Set("X-DOS-Request-Id", "initial-attempt")
			io.WriteString(w, `{"choices":[{"message":{"content":""},"finish_reason":"stop"}]}`)
		case 2:
			if req.Model != nextModel {
				t.Errorf("fallback model=%q, want %q", req.Model, nextModel)
			}
			w.Header().Set("X-DOS-Cost-USD", "0.25")
			w.Header().Set("X-DOS-Request-Id", "fallback-attempt")
			w.Header().Set("Content-Length", "4096")
			io.WriteString(w, `{"choices":[`)
		default:
			t.Error("truncated fallback was retried")
			http.Error(w, "unexpected retry", http.StatusBadRequest)
		}
	}, func(cfg *Config) {
		routing := router.DefaultRoutingConfig()
		routing.Promotions = nil
		for _, tiers := range []map[router.Tier]router.TierConfig{routing.Tiers, routing.AgenticTiers, routing.EcoTiers, routing.PremiumTiers} {
			for _, tier := range []router.Tier{router.TierSimple, router.TierMedium, router.TierComplex, router.TierReasoning} {
				if tiers != nil {
					tiers[tier] = router.TierConfig{Primary: firstModel, Fallback: []string{nextModel}}
				}
			}
		}
		cfg.RoutingConfig = &routing
		cfg.UsageLogger = func(e logger.UsageEntry) { logs = append(logs, e) }
	})
	if err := sc.SetLimit(spendcontrol.WindowSession, 1); err != nil {
		t.Fatal(err)
	}
	result := syncTestChat(t, srv, `{"model":"auto","messages":[{"role":"user","content":"hello"}]}`)
	if result.Code != http.StatusBadGateway || !strings.Contains(result.Body.String(), "Incomplete fallback response") {
		t.Fatalf("status=%d, body=%q", result.Code, result.Body.String())
	}
	if result.Header().Get("X-DOSRouter-Request-Id") != "fallback-attempt" {
		t.Fatalf("fallback request ID=%q", result.Header().Get("X-DOSRouter-Request-Id"))
	}
	if len(logs) != 2 || logs[0].Cost != 0.125 || logs[1].Cost != 0.25 || logs[0].RequestID != "initial-attempt" || logs[1].RequestID != "fallback-attempt" {
		t.Fatalf("fallback usage logs=%+v", logs)
	}
	if attempts.Load() != 2 {
		t.Fatalf("upstream attempts=%d, want 2", attempts.Load())
	}
	history := sc.GetHistory()
	if len(history) != 2 || history[0].Model != firstModel || history[0].Amount != 0.125 || history[1].Model != nextModel || history[1].Amount != 0.25 {
		t.Fatalf("incomplete fallback was not settled exactly once: %+v", history)
	}
	status := sc.GetStatus()[spendcontrol.WindowSession]
	if status.Spent != 0.375 || status.Remaining != 0.625 {
		t.Fatalf("fallback left a pending reservation: %+v", status)
	}
}

type closedStreamWriter struct{ *httptest.ResponseRecorder }

func (w *closedStreamWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestInterruptedStreamRetainsObservedUsage(t *testing.T) {
	for _, failure := range []string{"client write", "upstream read"} {
		t.Run(failure, func(t *testing.T) {
			var logs []logger.UsageEntry
			srv, sc := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				if failure == "upstream read" {
					w.Header().Set("Content-Length", "4096")
				}
				io.WriteString(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":1000,\"completion_tokens\":500}}\n\n")
			}, func(cfg *Config) { cfg.UsageLogger = func(e logger.UsageEntry) { logs = append(logs, e) } })
			srv.sessions.Close()
			srv.sessions = session.NewStore(session.Config{Enabled: true, TimeoutMs: 60000})
			if err := sc.SetLimit(spendcontrol.WindowSession, 1); err != nil {
				t.Fatal(err)
			}
			const model = "openai/gpt-4o-mini"
			srv.modelPricing[model] = router.ModelPricing{InputPrice: 1, OutputPrice: 2}
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"`+model+`","stream":true,"messages":[{"role":"user","content":"stream"}]}`))
			request.Header.Set("x-session-id", "interrupted-session")
			var writer http.ResponseWriter = httptest.NewRecorder()
			if failure == "client write" {
				writer = &closedStreamWriter{httptest.NewRecorder()}
			}
			srv.handleChatCompletions(writer, request)
			history := sc.GetHistory()
			const expectedCost = 0.002
			if len(history) != 1 || history[0].Amount != expectedCost {
				t.Fatalf("observed usage lost on interruption: %+v, want one charge %v", history, expectedCost)
			}
			if len(logs) != 1 || logs[0].Cost != expectedCost || logs[0].Status != "interrupted" {
				t.Fatalf("interrupted usage log=%+v", logs)
			}
			if cost := srv.sessions.GetSessionCostUSD("interrupted-session"); cost != expectedCost {
				t.Fatalf("session cost=%v, want %v", cost, expectedCost)
			}
			if spent := sc.GetSpending()[spendcontrol.WindowSession]; spent != expectedCost {
				t.Fatalf("pending reservation remained: %v", spent)
			}
		})
	}
}
