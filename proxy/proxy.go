// Package proxy implements an OpenAI-compatible HTTP proxy server with smart
// routing. It classifies incoming requests, selects the cheapest capable model,
// and forwards to the upstream API.
package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/DOS/DOSRouter/cache"
	"github.com/DOS/DOSRouter/compression"
	"github.com/DOS/DOSRouter/dedup"
	"github.com/DOS/DOSRouter/journal"
	"github.com/DOS/DOSRouter/logger"
	"github.com/DOS/DOSRouter/models"
	"github.com/DOS/DOSRouter/partners"
	"github.com/DOS/DOSRouter/retry"
	"github.com/DOS/DOSRouter/router"
	"github.com/DOS/DOSRouter/session"
	"github.com/DOS/DOSRouter/spendcontrol"
)

// Version is set at build time or defaults to "dev".
var Version = "1.0.0"

// Per-model attempt timeouts (upstream v0.12.182). Reasoning models get a
// longer window because first-token cold-start can take 60-120s (DeepSeek V4
// Pro, Claude opus adaptive thinking, GPT-5 reasoning_effort=high); everything
// else falls through to the next model after 60s.
const (
	perModelTimeoutNonReasoning = 60 * time.Second
	perModelTimeoutReasoning    = 180 * time.Second
)

// perModelTimeout returns the per-attempt timeout for a resolved model ID,
// using the longer reasoning window for reasoning-capable models.
func perModelTimeout(modelID string) time.Duration {
	if models.IsReasoningModel(modelID) {
		return perModelTimeoutReasoning
	}
	return perModelTimeoutNonReasoning
}

// Config controls the proxy server behavior.
type Config struct {
	// Port to listen on
	Port int
	// Upstream API base URL (e.g. "https://api.example.com")
	UpstreamBase string
	// API key for upstream
	UpstreamAPIKey string
	// Routing config override (nil = use default)
	RoutingConfig *router.RoutingConfig
	// SpendControl overrides the file-backed controller (useful for embedded servers).
	SpendControl *spendcontrol.SpendControl
	// UsageLogger overrides usage logging.
	UsageLogger func(logger.UsageEntry)
}

// Server is the OpenAI-compatible proxy with smart routing.
type Server struct {
	config        Config
	routingConfig router.RoutingConfig
	modelPricing  map[string]router.ModelPricing
	httpClient    *http.Client

	// Middleware components
	dedup        *dedup.Deduplicator
	cache        *cache.Cache
	sessions     *session.Store
	journal      *journal.SessionJournal
	spendControl *spendcontrol.SpendControl
	spendError   error

	// Deferred startup for OpenClaw plugin config (upstream v0.12.142)
	// When OpenClaw calls Register() twice, the first call has empty pluginConfig.
	// We defer proxy startup by 250ms to allow the second call with real config.
	startMu    sync.Mutex
	deferTimer *time.Timer
	registered bool
}

// New creates a new proxy server.
func New(cfg Config) *Server {
	rc := router.DefaultRoutingConfig()
	if cfg.RoutingConfig != nil {
		rc = *cfg.RoutingConfig
	}

	sc := cfg.SpendControl
	var spendErr error
	if sc == nil {
		sc, spendErr = spendcontrol.New(spendcontrol.NewFileStorage())
	}
	return &Server{
		config:        cfg,
		routingConfig: rc,
		modelPricing:  models.BuildPricingMap(),
		httpClient: &http.Client{
			Timeout:       5 * time.Minute,
			Transport:     http.DefaultTransport.(*http.Transport).Clone(),
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		dedup:        dedup.New(),
		cache:        cache.New(),
		sessions:     session.NewStore(session.DefaultConfig()),
		journal:      journal.New(journal.DefaultConfig()),
		spendControl: sc,
		spendError:   spendErr,
	}
}

// Close shuts down the proxy server and its components.
func (s *Server) Close() {
	s.startMu.Lock()
	if s.deferTimer != nil {
		s.deferTimer.Stop()
	}
	s.startMu.Unlock()
	s.sessions.Close()
}

// Register handles OpenClaw plugin registration (upstream v0.12.142).
// OpenClaw calls Register() twice: first with empty config (pre-gateway),
// then with the user's actual pluginConfig from openclaw.json.
// If pluginConfig is empty, we defer startup by 250ms to wait for the real config.
func (s *Server) Register(pluginConfig *router.RoutingConfig) {
	s.startMu.Lock()
	defer s.startMu.Unlock()

	if pluginConfig != nil {
		// Got real config — cancel any deferred timer and apply
		if s.deferTimer != nil {
			s.deferTimer.Stop()
			s.deferTimer = nil
		}
		s.routingConfig = *pluginConfig
		s.registered = true
		log.Println("DOSRouter: registered with plugin config")
		return
	}

	if s.registered {
		return // Already registered, ignore empty re-register
	}

	// Empty pluginConfig — defer startup by 250ms
	// If a second Register() call arrives with real config, it cancels this timer
	if s.deferTimer == nil {
		s.deferTimer = time.AfterFunc(250*time.Millisecond, func() {
			s.startMu.Lock()
			defer s.startMu.Unlock()
			if !s.registered {
				s.registered = true
				log.Println("DOSRouter: registered with default config (no plugin config received)")
			}
		})
	}
}

// IsRegistered returns true if the server has been registered (with or without plugin config).
func (s *Server) IsRegistered() bool {
	s.startMu.Lock()
	defer s.startMu.Unlock()
	return s.registered
}

// ListenAndServe starts the proxy server.
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("/v1/images/generations", s.handleImageGen)
	mux.HandleFunc("/v1/models", s.handleModels)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/debug", s.handleDebug)
	mux.HandleFunc("/cache", s.handleCacheStats)

	// Self-hosted market data endpoints backed by Pyth Network.
	// Registers: /v1/stocks/*, /v1/crypto/price/*, /v1/fx/price/*, /v1/commodity/price/*.
	partners.NewMarketHandler().Routes(mux)

	addr := fmt.Sprintf(":%d", s.config.Port)
	log.Printf("DOSRouter proxy listening on %s (upstream: %s)", addr, s.config.UpstreamBase)
	return http.ListenAndServe(addr, mux)
}

// chatRequest is the OpenAI-compatible request format.
type chatRequest struct {
	Model       string                     `json:"model"`
	Messages    []chatMessage              `json:"messages"`
	MaxTokens   int                        `json:"max_tokens,omitempty"`
	Temperature *float64                   `json:"temperature,omitempty"`
	Stream      bool                       `json:"stream,omitempty"`
	Tools       json.RawMessage            `json:"tools,omitempty"`
	Extra       map[string]json.RawMessage `json:"-"`
}

type chatMessage struct {
	Role             string                     `json:"role"`
	Content          json.RawMessage            `json:"content"`
	ReasoningContent *string                    `json:"reasoning_content,omitempty"`
	Extra            map[string]json.RawMessage `json:"-"`
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	startTime := time.Now()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	cacheBody := append([]byte(nil), body...)
	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// --- Session tracking ---
	sessionID := session.GetSessionID(r.Header, "")
	if sessionID == "" {
		// Derive from first user message content
		for _, m := range req.Messages {
			if m.Role == "user" {
				var content string
				if len(m.Content) > 0 && m.Content[0] == '"' {
					json.Unmarshal(m.Content, &content)
				} else {
					content = string(m.Content)
				}
				if content != "" {
					sessionID = session.DeriveSessionID(content)
					break
				}
			}
		}
	}

	// Caller-specific credentials must never share an internal response cache.
	cacheAllowed := s.config.UpstreamAPIKey != "" || r.Header.Get("Authorization") == ""
	// --- Response cache check (non-streaming only) ---
	if !req.Stream && cacheAllowed {
		if entry, ok := s.cache.Get(body, false); ok {
			w.Header().Set("X-DOSRouter-Cache", "hit")
			for k, vs := range entry.Header {
				for _, v := range vs {
					w.Header().Add(k, v)
				}
			}
			if s.config.UpstreamAPIKey != "" || r.Header.Get("Authorization") != "" {
				w.Header().Set("Cache-Control", "no-store")
			}
			w.WriteHeader(entry.StatusCode)
			w.Write(entry.Body)
			return
		}
	}

	// Resolve model alias
	resolvedModel := models.ResolveModelAlias(req.Model)
	isSmartRoute := resolvedModel == "auto" || resolvedModel == "eco" || resolvedModel == "premium"
	userExplicit := !isSmartRoute // User explicitly selected a model (not a routing profile)

	var decision *router.RoutingDecision
	if isSmartRoute {
		// Check session pin first (user-explicit or cache-sticky)
		if sessionID != "" {
			if entry := s.sessions.GetSession(sessionID); entry != nil {
				// Honor user-explicit pin unconditionally
				if entry.UserExplicit {
					resolvedModel = entry.Model
					isSmartRoute = false
					w.Header().Set("X-DOSRouter-Session", "pinned")
					userExplicit = true
				} else if s.sessions.IsCacheSticky(sessionID) {
					// Cache-sticky: keep model to maximize prefix cache hits
					resolvedModel = entry.Model
					isSmartRoute = false
					w.Header().Set("X-DOSRouter-Session", "cache-sticky")
					userExplicit = false
				}
			}
		}
	}

	if isSmartRoute {
		// Extract prompt and system prompt from messages
		prompt, systemPrompt := extractPrompts(req.Messages)
		maxOutputTokens := req.MaxTokens
		if maxOutputTokens == 0 {
			maxOutputTokens = 4096
		}

		// Determine routing profile
		routingProfile := "auto"
		if resolvedModel == "eco" {
			routingProfile = "eco"
		} else if resolvedModel == "premium" {
			routingProfile = "premium"
		}

		d, err := router.Route(prompt, systemPrompt, maxOutputTokens, router.RouterOptions{
			Config:         s.routingConfig,
			ModelPricing:   s.modelPricing,
			RoutingProfile: routingProfile,
			HasTools:       len(req.Tools) > 0,
		})
		if err != nil {
			http.Error(w, "Routing error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		decision = &d
		resolvedModel = d.Model

		// Pin to session (smart-routed, not user-explicit)
		if sessionID != "" {
			s.sessions.SetSession(sessionID, resolvedModel, d.Tier, false)

			// Cache-aware sticky: if context is large, pin model to maximize
			// provider-side prefix cache hits (reduces latency + cost).
			// TTL is per-provider (e.g. vLLM self-host gets 10min, Anthropic gets 5min).
			msgs := extractMessageInfos(req.Messages)
			if session.ShouldCacheSticky(msgs) {
				provider := session.ProviderFromModel(resolvedModel)
				s.sessions.SetCacheSticky(sessionID, provider)
				w.Header().Set("X-DOSRouter-Cache-Sticky", fmt.Sprintf("activated;provider=%s;ttl=%ds",
					provider, session.CacheTTLForProvider(provider)/1000))
			}
		}
	}

	// Pin explicit model selection to session
	if userExplicit && sessionID != "" && !isSmartRoute {
		s.sessions.SetSession(sessionID, resolvedModel, "", true)
	}

	// --- Context compression (if enabled) ---
	compMsgs := toNormalizedMessages(req.Messages)
	if canCompressMessages(req.Messages) && compression.ShouldCompress(compMsgs) {
		result := compression.CompressContext(compMsgs, compression.DefaultCompressionConfig())
		if result.Stats.Ratio < 0.95 && result.Stats.Ratio > 0 {
			// Re-marshal with compressed messages
			compReq := req
			compReq.Messages = fromNormalizedMessages(result.Messages)
			req.Messages = compReq.Messages
			if b, err := json.Marshal(compReq); err == nil {
				body = b // Use compressed body for upstream
			}
		}
	}

	// --- Journal: inject context if needed ---
	if sessionID != "" {
		prompt, _ := extractPrompts(req.Messages)
		if s.journal.NeedsContext(prompt) {
			if ctx := s.journal.Format(sessionID); ctx != "" {
				w.Header().Set("X-DOSRouter-Journal", "injected")
			}
		}
	}

	// Rewrite model in request body
	req.Model = resolvedModel

	// Normalize assistant messages for reasoning models (upstream v0.12.92)
	if models.IsReasoningModel(resolvedModel) {
		req.Messages = normalizeMessagesForThinking(req.Messages)
	}

	// Validate request can be marshaled (actual marshaling happens per-model in fallback loop)
	if _, err := json.Marshal(req); err != nil {
		http.Error(w, "Failed to marshal request", http.StatusInternalServerError)
		return
	}

	// Set routing headers
	if decision != nil {
		w.Header().Set("X-DOSRouter-Model", decision.Model)
		w.Header().Set("X-DOSRouter-Tier", string(decision.Tier))
		w.Header().Set("X-DOSRouter-Confidence", fmt.Sprintf("%.2f", decision.Confidence))
		w.Header().Set("X-DOSRouter-Savings", fmt.Sprintf("%.0f%%", decision.Savings*100))
		w.Header().Set("X-DOSRouter-Profile", decision.Profile)
		w.Header().Set("X-DOSRouter-Reasoning", sanitizeHeaderValue(decision.Reasoning))
		if decision.CostEstimate > 0 {
			w.Header().Set("X-DOSRouter-Cost", fmt.Sprintf("%.6f", decision.CostEstimate))
		}
	}

	// Forward to upstream with retry and structured fallback (upstream v0.12.64)
	upstreamURL := fmt.Sprintf("%s/v1/chat/completions", s.config.UpstreamBase)
	authHeader := r.Header.Get("Authorization")
	if s.config.UpstreamAPIKey != "" {
		authHeader = "Bearer " + s.config.UpstreamAPIKey
	}

	makeReqFor := func(bodyBytes []byte) func() (*http.Request, error) {
		return func() (*http.Request, error) {
			upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, bytes.NewReader(bodyBytes))
			if err != nil {
				return nil, err
			}
			upstreamReq.Header.Set("Content-Type", "application/json")
			upstreamReq.Header.Set("Authorization", authHeader)
			return upstreamReq, nil
		}
	}

	// Build fallback chain: if smart-routed, try all models in the tier
	var fallbackChain []string
	if decision != nil {
		fallbackChain = router.GetFallbackChain(decision.Tier, decision.TierConfigs)
	}
	if len(fallbackChain) == 0 {
		fallbackChain = []string{resolvedModel}
	}

	type attemptResult struct {
		model  string
		reason string
	}
	var attempts []attemptResult
	var resp *http.Response
	var spend *requestSpend
	defer func() {
		if spend != nil {
			spend.finish(nil)
		}
	}()
	// cancelResp cancels the context of the SUCCESSFUL attempt; it is deferred
	// after the loop so the chosen response body stays streamable until the
	// handler returns, then its resources are released.
	var cancelResp context.CancelFunc

	for _, tryModel := range fallbackChain {
		if r.Context().Err() != nil {
			return
		}
		if req.MaxTokens <= 0 && req.Extra["max_completion_tokens"] == nil && s.spendControl != nil && len(s.spendControl.GetLimits()) > 0 {
			req.MaxTokens = 4096
		}
		req.Model = tryModel
		tryBody, _ := json.Marshal(req)
		currentSpend, ok := s.reserveChat(w, req, tryBody, tryModel)
		if !ok {
			return
		}
		// Per-model timeout (upstream v0.12.182): reasoning models get 3min for
		// cold-start first-token (DeepSeek V4 Pro / Claude opus thinking / GPT-5
		// reasoning_effort=high can take 60-120s); non-reasoning get 60s. On
		// timeout the loop falls through to the next model rather than failing.
		//
		// Implemented as cancel-context + AfterFunc (the Go equivalent of
		// setTimeout/clearTimeout): the timer fires cancel() only if the attempt
		// has not produced a response yet. On success we Stop() the timer so the
		// long-lived stream is NOT cut at the per-model bound — it then runs under
		// the parent request context / client timeout, matching upstream's
		// clearTimeout-on-success behavior. Derived from r.Context() so a client
		// disconnect still cancels the in-flight attempt.
		attemptCtx, cancelAttempt := context.WithCancel(r.Context())
		timer := time.AfterFunc(perModelTimeout(tryModel), cancelAttempt)
		tryResp, tryErr := retry.Do(attemptCtx, makeReqFor(tryBody), retry.WithClient(s.httpClient), retry.WithNetworkRetries(false))
		if tryResp != nil && tryResp.StatusCode >= 300 {
			tryErr = nil
		}
		if tryErr != nil {
			currentSpend.finish(nil) // A lost response may already have incurred a charge.
			timer.Stop()
			cancelAttempt()
			if tryResp != nil {
				tryResp.Body.Close()
			}
			if r.Context().Err() != nil {
				return
			}
			attempts = append(attempts, attemptResult{model: tryModel, reason: tryErr.Error()})
			break
		}
		// Provider returned an error status (4xx/5xx except 429 which retry handles)
		if tryResp.StatusCode >= 300 {
			currentSpend.release()
			errBody, _ := io.ReadAll(tryResp.Body)
			tryResp.Body.Close()
			timer.Stop()
			cancelAttempt()
			reason := fmt.Sprintf("HTTP %d", tryResp.StatusCode)
			if len(errBody) > 0 {
				var errObj struct {
					Error struct {
						Message string `json:"message"`
					} `json:"error"`
				}
				if json.Unmarshal(errBody, &errObj) == nil && errObj.Error.Message != "" {
					reason = errObj.Error.Message
				}
			}
			attempts = append(attempts, attemptResult{model: tryModel, reason: reason})
			continue
		}
		// Success: stop the per-model timer so streaming is not cut at the bound,
		// and keep the context alive (cancel deferred after the loop).
		timer.Stop()
		cancelResp = cancelAttempt
		resp = tryResp
		spend = currentSpend
		spend.header = resp.Header
		resolvedModel = tryModel
		if tryModel != fallbackChain[0] {
			w.Header().Set("X-DOSRouter-Fallback", tryModel)
			w.Header().Set("X-DOSRouter-Model", tryModel)
		}
		break
	}
	if cancelResp != nil {
		defer cancelResp()
	}

	if resp == nil {
		// All models failed - structured error
		parts := make([]string, len(attempts))
		for i, a := range attempts {
			parts[i] = fmt.Sprintf("%s (%s)", a.model, a.reason)
		}
		errMsg := fmt.Sprintf("All %d models failed. Tried: %s", len(attempts), strings.Join(parts, ", "))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": errMsg,
				"type":    "all_models_failed",
				"models":  len(attempts),
			},
		})
		s.logRequest(resolvedModel, decision, startTime, "error")
		return
	}
	defer resp.Body.Close()

	if id := gatewayRequestID(resp.Header); id != "" {
		w.Header().Set("X-DOSRouter-Request-Id", id)
	}

	// Stream response back
	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(resp.StatusCode)

		flusher, ok := w.(http.Flusher)
		if !ok {
			_, copyErr := io.Copy(w, resp.Body)
			status := "success"
			if copyErr != nil {
				status = "interrupted"
			}
			s.logRequest(resolvedModel, decision, startTime, status)
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		var streamInputTok, streamOutputTok int
		proseFilters := make(map[int]*proseFilter)
		for scanner.Scan() {
			line := scanner.Text()
			// Parse and rewrite streaming chunks: inject model name + track usage
			if strings.HasPrefix(line, "data: ") && line != "data: [DONE]" {
				var chunk map[string]interface{}
				if json.Unmarshal([]byte(line[6:]), &chunk) == nil {
					// Track usage tokens
					if u, ok := chunk["usage"].(map[string]interface{}); ok {
						_, hasInput := u["prompt_tokens"]
						_, hasOutput := u["completion_tokens"]
						spend.usageKnown = hasInput && hasOutput
						if pt, ok := u["prompt_tokens"].(float64); ok {
							streamInputTok = int(pt)
						}
						if ct, ok := u["completion_tokens"].(float64); ok {
							streamOutputTok = int(ct)
						}
					}
					// Preserve tool-call prose by default (upstream v0.12.248).
					// The operator can opt back into suppression.
					mutated := false
					if choices, ok := chunk["choices"].([]interface{}); ok {
						for _, c := range choices {
							choice, ok := c.(map[string]interface{})
							if !ok {
								continue
							}
							if delta, ok := choice["delta"].(map[string]interface{}); ok {
								index, _ := choice["index"].(float64)
								f := proseFilters[int(index)]
								if f == nil {
									f = &proseFilter{}
									proseFilters[int(index)] = f
								}
								content, _ := delta["content"].(string)
								finish, _ := choice["finish_reason"].(string)
								cleaned := f.filter(content, finish != "")
								if cleaned != content {
									delta["content"] = cleaned
									mutated = true
								}
							}
							if choiceEndsWithToolCalls(choice) && !forwardToolCallProse() {
								if delta, ok := choice["delta"].(map[string]interface{}); ok {
									if s, _ := delta["content"].(string); s != "" {
										delta["content"] = ""
										mutated = true
									}
								}
							}
						}
					}
					// Inject actual routed model into every chunk (upstream v0.12.64)
					if decision != nil {
						chunk["model"] = resolvedModel
						mutated = true
					}
					// Only re-marshal when we actually changed the chunk. Re-encoding
					// every pass-through chunk would silently rewrite provider-specific
					// extension fields / key order on non-routed requests.
					if mutated {
						if b, err := json.Marshal(chunk); err == nil {
							line = "data: " + string(b)
						}
					}
				}
			}
			// Intercept [DONE] to inject cost usage chunk
			if line == "data: [DONE]" {
				if decision != nil && (streamInputTok > 0 || streamOutputTok > 0) {
					cb := buildCostBreakdown(resolvedModel, string(decision.Tier), decision.Profile, s.modelPricing, streamInputTok, streamOutputTok)
					if cb != nil {
						usageChunk := map[string]interface{}{
							"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixMilli()),
							"object":  "chat.completion.chunk",
							"created": time.Now().Unix(),
							"model":   resolvedModel,
							"choices": []interface{}{},
							"usage": map[string]interface{}{
								"prompt_tokens":     streamInputTok,
								"completion_tokens": streamOutputTok,
								"total_tokens":      streamInputTok + streamOutputTok,
								"cost":              cb,
							},
						}
						if b, err := json.Marshal(usageChunk); err == nil {
							if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
								return
							}
							flusher.Flush()
						}
					}
				}
			}
			if _, err := fmt.Fprintf(w, "%s\n", line); err != nil {
				return
			}
			flusher.Flush()
		}
		if scanner.Err() != nil || r.Context().Err() != nil {
			return
		}
		spend.input, spend.output = streamInputTok, streamOutputTok
	} else {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			if r.Context().Err() == nil {
				http.Error(w, "Incomplete upstream response", http.StatusBadGateway)
			}
			return
		}

		// --- Empty turn fallback detection ---
		// If the response has empty content, no tool_calls, and finish_reason "stop",
		// treat as degraded and try the next model in the fallback chain.
		if resp.StatusCode == http.StatusOK && decision != nil && isEmptyTurn(respBody) {
			nextModel := ""
			for i, m := range fallbackChain {
				if m == resolvedModel && i+1 < len(fallbackChain) {
					nextModel = fallbackChain[i+1]
					break
				}
			}
			if nextModel != "" {
				log.Printf("degraded response: empty turn from %s, falling back to %s", resolvedModel, nextModel)
				req.Model = nextModel
				fbBody, _ := json.Marshal(req)

				// Re-pin session to fallback model
				if sessionID != "" {
					s.sessions.SetSession(sessionID, nextModel, decision.Tier, userExplicit)
				}

				spend.finish(respBody)
				spend = nil
				nextSpend, allowed := s.reserveChat(w, req, fbBody, nextModel)
				if !allowed {
					return
				}
				fbResp, fbErr := retry.Do(r.Context(), makeReqFor(fbBody), retry.WithClient(s.httpClient), retry.WithNetworkRetries(false))
				if fbResp != nil && fbResp.StatusCode >= 300 {
					fbErr = nil
				}
				if fbErr != nil {
					nextSpend.finish(nil)
					if fbResp != nil {
						fbResp.Body.Close()
					}
					if r.Context().Err() == nil {
						http.Error(w, "Fallback request failed", http.StatusBadGateway)
					}
					return
				}
				defer fbResp.Body.Close()
				if fbResp.StatusCode >= 300 {
					nextSpend.release()
					http.Error(w, "Fallback request rejected", http.StatusBadGateway)
					return
				}
				spend = nextSpend
				spend.header = fbResp.Header
				respBody, readErr = io.ReadAll(fbResp.Body)
				if readErr != nil {
					http.Error(w, "Incomplete fallback response", http.StatusBadGateway)
					return
				}
				resp = fbResp
				resolvedModel = nextModel
				w.Header().Set("X-DOSRouter-Fallback", nextModel)
				w.Header().Set("X-DOSRouter-Model", nextModel)

			}
		}

		spend.readUsage(respBody)
		// Inject usage.cost into non-streaming response (upstream v0.12.146)
		if resp.StatusCode == http.StatusOK {
			var parsed map[string]interface{}
			if json.Unmarshal(respBody, &parsed) == nil {
				// Overwrite model with actual resolved model
				if decision != nil {
					parsed["model"] = resolvedModel
				}
				// Preserve assistant prose alongside native tool calls (v0.12.248).
				if choices, ok := parsed["choices"].([]interface{}); ok {
					for _, c := range choices {
						choice, ok := c.(map[string]interface{})
						if !ok {
							continue
						}
						// Tool-call recovery (upstream v0.12.214, v0.12.215, v0.12.230):
						// If model output formatted tool calls in plain text content, recover them.
						if msg, ok := choice["message"].(map[string]interface{}); ok {
							tc, _ := msg["tool_calls"].([]interface{})
							if len(tc) == 0 && strings.TrimSpace(string(req.Extra["tool_choice"])) != `"none"` {
								contentStr, _ := msg["content"].(string)
								contentStr = stripThinking(contentStr)
								if recovered, cleaned := recoverToolCallsWithProse(contentStr, req.Tools); len(recovered) > 0 && len(req.Tools) > 0 {
									msg["content"] = cleaned
									recList := make([]interface{}, len(recovered))
									for idx, r := range recovered {
										recList[idx] = r
									}
									msg["tool_calls"] = recList
									choice["finish_reason"] = "tool_calls"
								}
							}
						}

						if msg, ok := choice["message"].(map[string]interface{}); ok {
							if content, ok := msg["content"].(string); ok {
								msg["content"] = stripThinking(content)
							}
						}
						if choiceEndsWithToolCalls(choice) && !forwardToolCallProse() {
							if msg, ok := choice["message"].(map[string]interface{}); ok {
								if s, _ := msg["content"].(string); s != "" {
									msg["content"] = ""
								}
							}
						}
					}
				}
				// Inject cost breakdown if usage tokens available
				if usage, ok := parsed["usage"].(map[string]interface{}); ok && decision != nil {
					inputTok, _ := usage["prompt_tokens"].(float64)
					outputTok, _ := usage["completion_tokens"].(float64)
					cb := buildCostBreakdown(resolvedModel, string(decision.Tier), decision.Profile, s.modelPricing, int(inputTok), int(outputTok))
					if cb != nil {
						usage["cost"] = cb
					}
				}
				if b, err := json.Marshal(parsed); err == nil {
					respBody = b
				}
			}
		}

		if cacheAllowed && resp.StatusCode == http.StatusOK && !isEmptyTurn(respBody) {
			s.cache.Set(cacheBody, cache.Entry{Body: respBody, StatusCode: resp.StatusCode, Header: resp.Header.Clone()})
		}
		// Copy headers with sanitization (upstream v0.12.208)
		for k, v := range resp.Header {
			for _, vv := range v {
				w.Header().Add(k, sanitizeHeaderValue(vv))
			}
		}
		if authHeader != "" {
			w.Header().Set("Cache-Control", "no-store")
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(respBody)))
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)

		// Journal: extract events from response
		if sessionID != "" {
			var respData struct {
				Choices []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			}
			if json.Unmarshal(respBody, &respData) == nil && len(respData.Choices) > 0 {
				events := s.journal.ExtractEvents(respData.Choices[0].Message.Content)
				if len(events) > 0 {
					s.journal.Record(sessionID, events, resolvedModel)
				}
			}
		}
	}

	spend.finish(nil)
	if sessionID != "" {
		s.sessions.AddSessionCost(sessionID, int64(spend.cost*1_000_000))
	}
	s.logSettledRequest(resolvedModel, decision, startTime, spend)

}

func (s *Server) logRequest(model string, decision *router.RoutingDecision, startTime time.Time, status string) {
	tier := "DIRECT"
	cost := 0.0
	baselineCost := 0.0
	savings := 0.0
	if decision != nil {
		tier = string(decision.Tier)
		cost = decision.CostEstimate
		baselineCost = decision.BaselineCost
		savings = decision.Savings
	}
	s.writeUsage(logger.UsageEntry{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		Model:        model,
		Tier:         tier,
		Cost:         cost,
		BaselineCost: baselineCost,
		Savings:      savings,
		LatencyMs:    time.Since(startTime).Milliseconds(),
		Status:       status,
	})
}

func flattenHeaders(h http.Header) map[string]string {
	flat := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) > 0 {
			flat[strings.ToLower(k)] = v[0]
		}
	}
	return flat
}

func toNormalizedMessages(msgs []chatMessage) []compression.NormalizedMessage {
	result := make([]compression.NormalizedMessage, len(msgs))
	for i, m := range msgs {
		var content string
		if len(m.Content) > 0 && m.Content[0] == '"' {
			json.Unmarshal(m.Content, &content)
		} else {
			content = string(m.Content)
		}
		result[i] = compression.NormalizedMessage{Role: m.Role, Content: content}
	}
	return result
}

func fromNormalizedMessages(msgs []compression.NormalizedMessage) []chatMessage {
	result := make([]chatMessage, len(msgs))
	for i, m := range msgs {
		content, _ := json.Marshal(m.GetTextContent())
		result[i] = chatMessage{Role: m.Role, Content: content}
	}
	return result
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	type modelEntry struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	}

	var data []modelEntry
	for _, m := range models.Models {
		if m.Deprecated {
			continue
		}
		data = append(data, modelEntry{
			ID:      m.ID,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: strings.Split(m.ID, "/")[0],
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"object": "list",
		"data":   data,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]interface{}{
		"status":  "ok",
		"version": Version,
		"gateway": gatewayOrigin(s.config.UpstreamBase),
	}
	// Full health includes session/journal stats
	if r.URL.Query().Get("full") == "true" {
		sessStats := s.sessions.GetStats()
		jStats := s.journal.GetStats()
		resp["sessions"] = sessStats.Count
		resp["journalSessions"] = jStats.Sessions
		resp["journalEntries"] = jStats.TotalEntries
		if s.spendControl != nil {
			resp["spendControl"] = s.spendControl.GetStatus()
		}
		if s.spendError != nil {
			resp["status"] = "degraded"
			resp["spendControlError"] = "spending state unavailable"
		}
	}
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	stats := s.cache.Stats()
	json.NewEncoder(w).Encode(stats)
}

// debugRequest is used for the /debug endpoint to test classification.
type debugRequest struct {
	Prompt       string `json:"prompt"`
	SystemPrompt string `json:"system_prompt,omitempty"`
	MaxTokens    int    `json:"max_tokens,omitempty"`
	Profile      string `json:"profile,omitempty"` // "auto", "eco", "premium"
	HasTools     bool   `json:"has_tools,omitempty"`
}

func (s *Server) handleDebug(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req debugRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	profile := req.Profile
	if profile == "" {
		profile = "auto"
	}

	// Run classification
	fullText := req.SystemPrompt + " " + req.Prompt
	estimatedTokens := int(math.Ceil(float64(len(fullText)) / 4))

	scoring := router.ClassifyByRules(req.Prompt, req.SystemPrompt, estimatedTokens, s.routingConfig.Scoring)

	decision, _ := router.Route(req.Prompt, req.SystemPrompt, maxTokens, router.RouterOptions{
		Config:         s.routingConfig,
		ModelPricing:   s.modelPricing,
		RoutingProfile: profile,
		HasTools:       req.HasTools,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"scoring":  scoring,
		"decision": decision,
	})
}

// handleImageGen forwards image generation requests to the upstream API.
// No smart routing - direct passthrough with logging.
func (s *Server) handleImageGen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.spendError != nil || s.spendControl == nil {
		http.Error(w, "Spending state unavailable", http.StatusServiceUnavailable)
		return
	}
	if len(s.spendControl.GetLimits()) > 0 {
		http.Error(w, "Image cost cannot be reserved under configured spend limits", http.StatusTooManyRequests)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse model from request
	var req struct {
		Model string `json:"model"`
	}
	json.Unmarshal(body, &req)
	if req.Model == "" {
		req.Model = "dall-e-3"
	}

	upstreamURL := fmt.Sprintf("%s/v1/images/generations", s.config.UpstreamBase)
	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	if s.config.UpstreamAPIKey != "" {
		upstreamReq.Header.Set("Authorization", "Bearer "+s.config.UpstreamAPIKey)
	} else {
		upstreamReq.Header.Set("Authorization", r.Header.Get("Authorization"))
	}

	resp, err := s.httpClient.Do(upstreamReq)
	if err != nil {
		http.Error(w, "Upstream error: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		if r.Context().Err() == nil {
			http.Error(w, "Incomplete upstream response", http.StatusBadGateway)
		}
		return
	}

	for k, v := range resp.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}
	w.Header().Set("X-DOSRouter-Model", req.Model)
	if upstreamReq.Header.Get("Authorization") != "" {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)

	s.writeUsage(logger.UsageEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Model:     req.Model,
		Tier:      "IMAGE",
		Cost:      mediaCost(resp.Header, respBody),
		RequestID: gatewayRequestID(resp.Header),
		Status:    fmt.Sprintf("%d", resp.StatusCode),
	})
}

// choiceEndsWithToolCalls reports whether a parsed choice object (streaming or
// non-streaming) represents a tool-call turn: either finish_reason is
// "tool_calls", or a non-empty tool_calls array is present on message or delta.
// Used for the optional legacy prose suppression setting.
func choiceEndsWithToolCalls(choice map[string]interface{}) bool {
	if fr, _ := choice["finish_reason"].(string); fr == "tool_calls" {
		return true
	}
	hasToolCalls := func(o map[string]interface{}) bool {
		if o == nil {
			return false
		}
		if tc, ok := o["tool_calls"].([]interface{}); ok && len(tc) > 0 {
			return true
		}
		return false
	}
	if msg, ok := choice["message"].(map[string]interface{}); ok && hasToolCalls(msg) {
		return true
	}
	if delta, ok := choice["delta"].(map[string]interface{}); ok && hasToolCalls(delta) {
		return true
	}
	return false
}

// isEmptyTurn detects a degraded "empty turn" response: content is empty,
// no tool_calls, and finish_reason is "stop".
func isEmptyTurn(body []byte) bool {
	var resp struct {
		Choices []struct {
			Message struct {
				Content   string          `json:"content"`
				ToolCalls json.RawMessage `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if json.Unmarshal(body, &resp) != nil || len(resp.Choices) == 0 {
		return false
	}
	c := resp.Choices[0]
	return c.Message.Content == "" &&
		(len(c.Message.ToolCalls) == 0 || string(c.Message.ToolCalls) == "null") &&
		c.FinishReason == "stop"
}

// CostBreakdown is the usage.cost payload injected into every routed response.
type CostBreakdown struct {
	Total      float64 `json:"total"`
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	Baseline   float64 `json:"baseline"`
	SavingsPct *int    `json:"savings_pct,omitempty"`
	Model      string  `json:"model"`
	Tier       string  `json:"tier,omitempty"`
}

// buildCostBreakdown computes actual cost from upstream token counts and model pricing.
// Returns nil if token counts are unavailable.
func buildCostBreakdown(model string, tier string, profile string, pricing map[string]router.ModelPricing, inputTokens, outputTokens int) *CostBreakdown {
	if inputTokens <= 0 && outputTokens <= 0 {
		return nil
	}
	p, ok := pricing[model]
	if !ok {
		return nil
	}
	input := float64(inputTokens) / 1_000_000 * p.InputPrice
	output := float64(outputTokens) / 1_000_000 * p.OutputPrice
	total := input + output

	// Baseline: what claude-opus-4.6 would have cost (reference for savings)
	baselinePrice, hasBaseline := pricing["anthropic/claude-opus-4.6"]
	baseline := 0.0
	if hasBaseline {
		baseline = float64(inputTokens)/1_000_000*baselinePrice.InputPrice + float64(outputTokens)/1_000_000*baselinePrice.OutputPrice
	}

	cb := &CostBreakdown{
		Total:    math.Round(total*1_000_000) / 1_000_000,
		Input:    math.Round(input*1_000_000) / 1_000_000,
		Output:   math.Round(output*1_000_000) / 1_000_000,
		Baseline: math.Round(baseline*1_000_000) / 1_000_000,
		Model:    model,
		Tier:     tier,
	}
	if profile != "premium" && baseline > 0 {
		pct := int(math.Round(math.Max(0, math.Min(100, (1-total/baseline)*100))))
		cb.SavingsPct = &pct
	}
	return cb
}

// normalizeMessagesForThinking adds reasoning_content: "" to all assistant
// messages that lack it, which reasoning models require on every turn.
// See upstream v0.12.92 fix for multi-turn chat with reasoning models.
func normalizeMessagesForThinking(messages []chatMessage) []chatMessage {
	hasChanges := false
	for _, m := range messages {
		if m.Role == "assistant" && m.ReasoningContent == nil {
			hasChanges = true
			break
		}
	}
	if !hasChanges {
		return messages
	}
	empty := ""
	out := make([]chatMessage, len(messages))
	for i, m := range messages {
		if m.Role == "assistant" && m.ReasoningContent == nil {
			m.ReasoningContent = &empty
		}
		out[i] = m
	}
	return out
}

// extractPrompts extracts the last user message as prompt and system message.
// extractMessageInfos converts chatMessages to session.MessageInfo for
// cache-sticky evaluation. Extracts text content from both string and
// array-of-parts formats.
func extractMessageInfos(messages []chatMessage) []session.MessageInfo {
	infos := make([]session.MessageInfo, 0, len(messages))
	for _, m := range messages {
		var content string
		if len(m.Content) > 0 && m.Content[0] == '"' {
			json.Unmarshal(m.Content, &content)
		} else {
			var parts []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if json.Unmarshal(m.Content, &parts) == nil {
				for _, p := range parts {
					if p.Type == "text" {
						content += p.Text
					}
				}
			}
		}
		infos = append(infos, session.MessageInfo{Role: m.Role, Content: content})
	}
	return infos
}

func extractPrompts(messages []chatMessage) (prompt, systemPrompt string) {
	for _, m := range messages {
		var content string
		// Content can be string or array of content parts
		if len(m.Content) > 0 && m.Content[0] == '"' {
			json.Unmarshal(m.Content, &content)
		} else {
			// Array of content parts - extract text parts
			var parts []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if json.Unmarshal(m.Content, &parts) == nil {
				for _, p := range parts {
					if p.Type == "text" {
						content += p.Text + " "
					}
				}
			}
		}

		switch m.Role {
		case "system":
			systemPrompt = content
		case "user":
			prompt = content
		}
	}
	return
}
