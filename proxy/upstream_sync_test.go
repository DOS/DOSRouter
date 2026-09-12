package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DOS/DOSRouter/logger"
	"github.com/DOS/DOSRouter/router"
	"github.com/DOS/DOSRouter/spendcontrol"
)

func syncTestServer(t *testing.T, upstream http.HandlerFunc, configure func(*Config)) (*Server, *spendcontrol.SpendControl) {
	t.Helper()
	remote := httptest.NewServer(upstream)
	t.Cleanup(remote.Close)
	sc, err := spendcontrol.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{UpstreamBase: remote.URL, SpendControl: sc, UsageLogger: func(logger.UsageEntry) {}}
	if configure != nil {
		configure(&cfg)
	}
	srv := New(cfg)
	t.Cleanup(srv.Close)
	return srv, sc
}

func syncTestJSON(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func syncTestChat(t *testing.T, srv *Server, payload string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	srv.handleChatCompletions(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(payload)))
	return w
}

func syncTestOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, `{"model":"openai/gpt-4o-mini","choices":[{"index":0,"message":{"role":"assistant","content":"Done."},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`)
}

func syncTestTools() []any {
	return []any{map[string]any{"type": "function", "function": map[string]any{"name": "read_file", "parameters": map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}}}}}
}

func syncTestToolCall() []any {
	return []any{map[string]any{"id": "call_test", "type": "function", "function": map[string]any{"name": "read_file", "arguments": `{"path":"README.md"}`}}}
}

func TestUpstreamSyncPreservesExtendedToolConversation(t *testing.T) {
	original := map[string]any{
		"model": "gpt5", "stream": false, "max_completion_tokens": 8192,
		"reasoning_effort": "high", "response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "answer", "strict": true, "schema": map[string]any{"type": "object"}}},
		"parallel_tool_calls": false, "metadata": map[string]any{"trace": "test-only"}, "tools": syncTestTools(),
		"messages": []any{
			map[string]any{"role": "user", "name": "operator", "content": strings.Repeat("The exact input must remain unchanged. ", 500)},
			map[string]any{"role": "assistant", "content": nil, "tool_calls": syncTestToolCall(), "reasoning_content": "existing reasoning", "provider_extension": map[string]any{"retain": true}},
			map[string]any{"role": "tool", "tool_call_id": "call_test", "name": "read_file", "content": strings.Repeat("Repeated but semantically meaningful tool output. ", 300)},
		},
	}
	received := make(chan map[string]any, 1)
	srv, _ := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		received <- body
		syncTestOK(w)
	}, nil)
	result := syncTestChat(t, srv, syncTestJSON(t, original))
	if result.Code != 200 {
		t.Fatalf("status = %d: %s", result.Code, result.Body.String())
	}
	got := <-received
	// Compare normalized JSON values, including numeric and null fields.
	var want map[string]any
	json.Unmarshal([]byte(syncTestJSON(t, original)), &want)
	want["model"] = "openai/gpt-5.6-terra"
	if !reflect.DeepEqual(got, want) {
		t.Errorf("forwarded request lost or mutated tool/extension fields\ngot: %#v\nwant: %#v", got, want)
	}
}

func TestUpstreamSyncNonstreamToolProse(t *testing.T) {
	for _, kind := range []string{"native", "recovered"} {
		for _, optout := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/off=%t", kind, optout), func(t *testing.T) {
				setting := ""
				if optout {
					setting = "off"
				}
				t.Setenv("DOSROUTER_TOOL_CALL_PROSE", setting)
				content := "<think>private deliberation</think>I will read the file."
				message := map[string]any{"role": "assistant", "content": content}
				finish := "tool_calls"
				if kind == "native" {
					message["tool_calls"] = syncTestToolCall()
				} else {
					message["content"] = content + "\n```json\n{\"name\":\"read_file\",\"arguments\":{\"path\":\"README.md\"}}\n```"
					finish = "stop"
				}
				response := syncTestJSON(t, map[string]any{"choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": finish}}})
				srv, _ := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, response) }, nil)
				result := syncTestChat(t, srv, syncTestJSON(t, map[string]any{"model": "openai/gpt-4o-mini", "messages": []any{map[string]any{"role": "user", "content": "Read the file"}}, "tools": syncTestTools()}))
				if result.Code != 200 {
					t.Fatalf("status = %d: %s", result.Code, result.Body.String())
				}
				var got struct {
					Choices []struct {
						Finish  string `json:"finish_reason"`
						Message struct {
							Content   string `json:"content"`
							ToolCalls []any  `json:"tool_calls"`
						} `json:"message"`
					} `json:"choices"`
				}
				if err := json.Unmarshal(result.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if len(got.Choices) != 1 {
					t.Fatalf("choices = %d", len(got.Choices))
				}
				choice := got.Choices[0]
				want := "I will read the file."
				if optout {
					want = ""
				}
				if choice.Message.Content != want || choice.Finish != "tool_calls" || len(choice.Message.ToolCalls) != 1 {
					t.Errorf("tool result = %+v, want prose %q with one structured call", choice, want)
				}
			})
		}
	}
}

func TestUpstreamSyncDoesNotRecoverPrivateThinkingToolSyntax(t *testing.T) {
	content := "<think>call:read_file({\"path\":\"PRIVATE\"})</think>I can explain the file."
	srv, _ := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": content}, "finish_reason": "stop"}}})
	}, nil)
	result := syncTestChat(t, srv, syncTestJSON(t, map[string]any{"model": "openai/gpt-4o-mini", "messages": []any{map[string]any{"role": "user", "content": "Explain the file"}}, "tools": syncTestTools()}))
	if result.Code != 200 {
		t.Fatalf("status = %d", result.Code)
	}
	var body map[string]any
	json.Unmarshal(result.Body.Bytes(), &body)
	choice := body["choices"].([]any)[0].(map[string]any)
	message := choice["message"].(map[string]any)
	if tc, ok := message["tool_calls"].([]any); ok && len(tc) > 0 {
		t.Errorf("private thinking became an executable tool call: %v", tc)
	}
	if message["content"] != "I can explain the file." || choice["finish_reason"] != "stop" {
		t.Errorf("visible result = %v", choice)
	}
}

func TestUpstreamSyncStreamingToolProseAndSplitThinking(t *testing.T) {
	for _, optout := range []bool{false, true} {
		t.Run(fmt.Sprintf("off=%t", optout), func(t *testing.T) {
			setting := ""
			if optout {
				setting = "off"
			}
			t.Setenv("DOSROUTER_TOOL_CALL_PROSE", setting)
			srv, _ := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				chunks := []map[string]any{
					{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "<thi"}, "finish_reason": nil}}},
					{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "nk>private reasoning</th"}, "finish_reason": nil}}},
					{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "ink>I will read the file.", "tool_calls": syncTestToolCall()}, "finish_reason": "tool_calls"}}},
				}
				for _, chunk := range chunks {
					b, _ := json.Marshal(chunk)
					fmt.Fprintf(w, "data: %s\n\n", b)
					w.(http.Flusher).Flush()
				}
				io.WriteString(w, "data: [DONE]\n\n")
			}, nil)
			result := syncTestChat(t, srv, syncTestJSON(t, map[string]any{"model": "openai/gpt-4o-mini", "stream": true, "messages": []any{map[string]any{"role": "user", "content": "Read the file"}}, "tools": syncTestTools()}))
			if result.Code != 200 {
				t.Fatalf("status = %d", result.Code)
			}
			var content strings.Builder
			toolCount := 0
			scanner := bufio.NewScanner(strings.NewReader(result.Body.String()))
			for scanner.Scan() {
				line := scanner.Text()
				if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
					continue
				}
				var chunk struct {
					Choices []struct {
						Delta struct {
							Content string `json:"content"`
							Tools   []any  `json:"tool_calls"`
						} `json:"delta"`
					} `json:"choices"`
				}
				if err := json.Unmarshal([]byte(line[6:]), &chunk); err != nil {
					t.Fatal(err)
				}
				for _, c := range chunk.Choices {
					content.WriteString(c.Delta.Content)
					toolCount += len(c.Delta.Tools)
				}
			}
			want := "I will read the file."
			if optout {
				want = ""
			}
			if content.String() != want || toolCount != 1 {
				t.Errorf("stream prose/tools = %q/%d, want %q/1", content.String(), toolCount, want)
			}
			if strings.Contains(result.Body.String(), "private reasoning") {
				t.Error("thinking leaked through SSE")
			}
		})
	}
}

func TestUpstreamSyncCancellationReachesChatAndImage(t *testing.T) {
	for _, mode := range []string{"direct", "smart", "image"} {
		t.Run(mode, func(t *testing.T) {
			reached := make(chan struct{}, 1)
			cancelled := make(chan struct{}, 1)
			release := make(chan struct{})
			var unblock sync.Once
			defer unblock.Do(func() { close(release) })
			var attempts atomic.Int32
			srv, _ := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				attempts.Add(1)
				io.Copy(io.Discard, r.Body)
				select {
				case reached <- struct{}{}:
				default:
				}
				select {
				case <-r.Context().Done():
					select {
					case cancelled <- struct{}{}:
					default:
					}
				case <-release:
				}
			}, nil)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			model := "openai/gpt-4o-mini"
			if mode == "smart" {
				model = "auto"
			}
			payload := syncTestJSON(t, map[string]any{"model": model, "messages": []any{map[string]any{"role": "user", "content": "hello"}}})
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(payload)).WithContext(ctx)
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				w := httptest.NewRecorder()
				if mode == "image" {
					srv.handleImageGen(w, req)
				} else {
					srv.handleChatCompletions(w, req)
				}
			}()
			select {
			case <-reached:
			case <-time.After(3 * time.Second):
				cancel()
				unblock.Do(func() { close(release) })
				t.Fatal("request did not reach test upstream")
			}
			cancel()
			select {
			case <-cancelled:
			case <-time.After(3 * time.Second):
				unblock.Do(func() { close(release) })
				t.Error("upstream request did not observe cancellation")
			}
			select {
			case <-finished:
			case <-time.After(3 * time.Second):
				t.Error("proxy handler did not stop")
			}
			if attempts.Load() != 1 {
				t.Errorf("cancelled request made %d upstream attempts", attempts.Load())
			}
		})
	}
}

func TestUpstreamSyncBudgetBlocksBeforeForwarding(t *testing.T) {
	for _, model := range []string{"openai/gpt-4o-mini", "auto", "vendor/unknown"} {
		t.Run(model, func(t *testing.T) {
			var attempts atomic.Int32
			srv, sc := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) { attempts.Add(1); syncTestOK(w) }, nil)
			if err := sc.SetLimit(spendcontrol.WindowPerRequest, 0.000001); err != nil {
				t.Fatal(err)
			}
			result := syncTestChat(t, srv, syncTestJSON(t, map[string]any{"model": model, "max_completion_tokens": 10000, "messages": []any{map[string]any{"role": "user", "content": "hello"}}}))
			if result.Code != 429 || attempts.Load() != 0 {
				t.Errorf("status/attempts = %d/%d, want 429/0", result.Code, attempts.Load())
			}
			if len(sc.GetHistory()) != 0 {
				t.Error("blocked request created a charge")
			}
		})
	}
}

func TestUpstreamSyncConcurrentReservationAndGatewaySettlement(t *testing.T) {
	for _, model := range []string{"openai/gpt-4o-mini", "auto"} {
		t.Run(model, func(t *testing.T) {
			reached := make(chan struct{}, 1)
			release := make(chan struct{})
			var unblock sync.Once
			defer unblock.Do(func() { close(release) })
			var attempts atomic.Int32
			logged := make(chan logger.UsageEntry, 3)
			// Use identical bounded model chains so this test isolates admission
			// across direct and smart routing instead of scorer calibration.
			rc := router.DefaultRoutingConfig()
			for _, tiers := range []map[router.Tier]router.TierConfig{rc.Tiers, rc.EcoTiers, rc.PremiumTiers, rc.AgenticTiers} {
				for tier := range tiers {
					tiers[tier] = router.TierConfig{Primary: "openai/gpt-4o-mini"}
				}
			}
			srv, sc := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if attempts.Add(1) == 1 {
					reached <- struct{}{}
					<-release
				}
				w.Header().Set("X-Blockrun-Cost-USD", "0.002")
				w.Header().Set("X-Blockrun-Request-Id", "request-test-only")
				syncTestOK(w)
			}, func(cfg *Config) {
				cfg.RoutingConfig = &rc
				cfg.UsageLogger = func(e logger.UsageEntry) { logged <- e }
			})
			if err := sc.SetLimit(spendcontrol.WindowSession, 0.01); err != nil {
				t.Fatal(err)
			}
			payload := func(prompt string) string {
				return syncTestJSON(t, map[string]any{"model": model, "max_completion_tokens": 10000, "messages": []any{map[string]any{"role": "user", "content": prompt}}})
			}
			firstDone := make(chan *httptest.ResponseRecorder, 1)
			firstPayload := payload("hello first")
			go func() { firstDone <- syncTestChat(t, srv, firstPayload) }()
			select {
			case <-reached:
			case <-time.After(3 * time.Second):
				unblock.Do(func() { close(release) })
				t.Fatal("first request did not reach upstream")
			}
			second := syncTestChat(t, srv, payload("hello second"))
			if second.Code != 429 || attempts.Load() != 1 {
				t.Errorf("concurrent status/attempts = %d/%d, want 429/1", second.Code, attempts.Load())
			}
			unblock.Do(func() { close(release) })
			var first *httptest.ResponseRecorder
			select {
			case first = <-firstDone:
			case <-time.After(3 * time.Second):
				t.Fatal("first request did not settle")
			}
			if first.Code != 200 {
				t.Fatalf("first status = %d: %s", first.Code, first.Body.String())
			}
			if got := first.Header().Get("X-DOSRouter-Request-Id"); got != "request-test-only" {
				t.Errorf("gateway request ID = %q", got)
			}
			history := sc.GetHistory()
			if len(history) != 1 || math.Abs(history[0].Amount-0.002) > 1e-12 {
				t.Fatalf("settlement history = %+v", history)
			}
			entry := <-logged
			if entry.Cost != 0.002 || entry.CostSource != "gateway" || entry.RequestID != "request-test-only" || entry.InputTokens != 10 || entry.OutputTokens != 5 {
				t.Errorf("usage log = %+v", entry)
			}
			third := syncTestChat(t, srv, payload("hello third"))
			if third.Code != 200 || attempts.Load() != 2 {
				t.Errorf("after-settlement status/attempts = %d/%d, want 200/2", third.Code, attempts.Load())
			}
		})
	}
}

func TestUpstreamSyncRejectedRequestReleasesReservation(t *testing.T) {
	var attempts atomic.Int32
	srv, sc := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			http.Error(w, "invalid request", 400)
			return
		}
		syncTestOK(w)
	}, nil)
	if err := sc.SetLimit(spendcontrol.WindowSession, 0.007); err != nil {
		t.Fatal(err)
	}
	payload := `{"model":"openai/gpt-4o-mini","max_completion_tokens":10000,"messages":[{"role":"user","content":"hello"}]}`
	rejected := syncTestChat(t, srv, payload)
	if rejected.Code != 502 {
		t.Errorf("rejected proxy status = %d", rejected.Code)
	}
	if len(sc.GetHistory()) != 0 {
		t.Error("explicit rejection charged the session")
	}
	accepted := syncTestChat(t, srv, payload)
	if accepted.Code != 200 || attempts.Load() != 2 {
		t.Errorf("released reservation status/attempts = %d/%d", accepted.Code, attempts.Load())
	}
}

func TestUpstreamSyncTokenSettlementAndCacheDoNotDoubleCharge(t *testing.T) {
	var attempts atomic.Int32
	var logs []logger.UsageEntry
	srv, sc := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		// Invalid cost headers must not replace reliable token accounting.
		w.Header().Set("X-DOS-Cost-USD", "NaN")
		syncTestOK(w)
	}, func(cfg *Config) { cfg.UsageLogger = func(e logger.UsageEntry) { logs = append(logs, e) } })
	if err := sc.SetLimit(spendcontrol.WindowSession, 1); err != nil {
		t.Fatal(err)
	}
	const payload = `{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"hello cached"}]}`
	first := syncTestChat(t, srv, payload)
	cached := syncTestChat(t, srv, payload)
	if first.Code != 200 || cached.Code != 200 || cached.Header().Get("X-DOSRouter-Cache") != "hit" || attempts.Load() != 1 {
		t.Errorf("first/cache/attempts = %d/%d/%d, hit = %q", first.Code, cached.Code, attempts.Load(), cached.Header().Get("X-DOSRouter-Cache"))
	}
	history := sc.GetHistory()
	want := (10*0.15 + 5*0.6) / 1_000_000
	if len(history) != 1 || math.Abs(history[0].Amount-want) > 1e-12 {
		t.Errorf("token settlement = %+v, want one charge %g", history, want)
	}
	if len(logs) != 1 || logs[0].CostSource != "tokens" || math.Abs(logs[0].Cost-want) > 1e-12 {
		t.Errorf("usage log = %+v", logs)
	}
}

func TestUpstreamSyncSettledCostValidation(t *testing.T) {
	cases := []struct {
		name, dos, blockrun string
		want                float64
		valid               bool
	}{
		{"DOS priority", "0.25", "0.9", 0.25, true},
		{"zero is authoritative", "0", "0.9", 0, true},
		{"BlockRun fallback", "", "0.4", 0.4, true},
		{"invalid primary valid fallback", "NaN", "0.4", 0.4, true},
		{"negative rejected", "-0.1", "", 0, false},
		{"infinity rejected", "+Inf", "", 0, false},
		{"non-numeric rejected", "invalid", "", 0, false},
		{"absent", "", "", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := make(http.Header)
			h.Set("X-DOS-Cost-USD", tc.dos)
			h.Set("X-Blockrun-Cost-USD", tc.blockrun)
			cost, valid := settledCost(h)
			if cost != tc.want || valid != tc.valid {
				t.Errorf("settledCost = %g/%t, want %g/%t", cost, valid, tc.want, tc.valid)
			}
		})
	}
}

func TestUpstreamSyncUnknownModelWithoutLimitsRemainsForwardable(t *testing.T) {
	forwarded := make(chan string, 1)
	srv, _ := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		forwarded <- body.Model
		syncTestOK(w)
	}, nil)
	result := syncTestChat(t, srv, `{"model":"vendor/new-model","messages":[{"role":"user","content":"hello new model"}]}`)
	if result.Code != 200 {
		t.Fatalf("status = %d", result.Code)
	}
	if got := <-forwarded; got != "vendor/new-model" {
		t.Errorf("forwarded model = %q", got)
	}
}
