// Package proxy implements an OpenAI-compatible HTTP proxy server with smart
// routing. It classifies incoming requests, selects the cheapest capable model,
// and forwards to the upstream API.
package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/DOS/DOSRouter/cache"
	"github.com/DOS/DOSRouter/compression"
	"github.com/DOS/DOSRouter/dedup"
	"github.com/DOS/DOSRouter/journal"
	"github.com/DOS/DOSRouter/logger"
	"github.com/DOS/DOSRouter/models"
	"github.com/DOS/DOSRouter/retry"
	"github.com/DOS/DOSRouter/router"
	"github.com/DOS/DOSRouter/session"
	"github.com/DOS/DOSRouter/spendcontrol"
)

// Version is set at build time or defaults to "dev".
var Version = "1.0.0"

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
}

// New creates a new proxy server.
func New(cfg Config) *Server {
	rc := router.DefaultRoutingConfig()
	if cfg.RoutingConfig != nil {
		rc = *cfg.RoutingConfig
	}

	return &Server{
		config:        cfg,
		routingConfig: rc,
		modelPricing:  models.BuildPricingMap(),
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		dedup:        dedup.New(),
		cache:        cache.New(),
		sessions:     session.NewStore(session.DefaultConfig()),
		journal:      journal.New(journal.DefaultConfig()),
		spendControl: mustSpendControl(),
	}
}

// Close shuts down the proxy server and its components.
func (s *Server) Close() {
	s.sessions.Close()
}

// ListenAndServe starts the proxy server.
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("/v1/models", s.handleModels)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/debug", s.handleDebug)
	mux.HandleFunc("/cache", s.handleCacheStats)

	addr := fmt.Sprintf(":%d", s.config.Port)
	log.Printf("DOSRouter proxy listening on %s (upstream: %s)", addr, s.config.UpstreamBase)
	return http.ListenAndServe(addr, mux)
}

// chatRequest is the OpenAI-compatible request format.
type chatRequest struct {
	Model       string          `json:"model"`
	Messages    []chatMessage   `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Tools       json.RawMessage `json:"tools,omitempty"`
}

type chatMessage struct {
	Role             string          `json:"role"`
	Content          json.RawMessage `json:"content"`
	ReasoningContent *string         `json:"reasoning_content,omitempty"`
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

	// --- Response cache check (non-streaming only) ---
	if !req.Stream {
		if entry, ok := s.cache.Get(body, false); ok {
			w.Header().Set("X-DOSRouter-Cache", "hit")
			for k, vs := range entry.Header {
				for _, v := range vs {
					w.Header().Add(k, v)
				}
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
		// Check session pin first
		if sessionID != "" {
			if entry := s.sessions.GetSession(sessionID); entry != nil {
				resolvedModel = entry.Model
				isSmartRoute = false
				w.Header().Set("X-DOSRouter-Session", "pinned")
				// Preserve the explicit flag from the pinned session
				userExplicit = entry.UserExplicit
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

		// --- Spend control check ---
		if decision.CostEstimate > 0 {
			check := s.spendControl.Check(decision.CostEstimate)
			if !check.Allowed {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":     check.Reason,
					"blockedBy": check.BlockedBy,
					"remaining": check.Remaining,
					"resetIn":   check.ResetIn,
				})
				return
			}
		}

		// Pin to session (smart-routed, not user-explicit)
		if sessionID != "" {
			s.sessions.SetSession(sessionID, resolvedModel, d.Tier, false)
		}
	}

	// Pin explicit model selection to session
	if userExplicit && sessionID != "" && !isSmartRoute {
		s.sessions.SetSession(sessionID, resolvedModel, "", true)
	}

	// --- Context compression (if enabled) ---
	compMsgs := toNormalizedMessages(req.Messages)
	if compression.ShouldCompress(compMsgs) {
		result := compression.CompressContext(compMsgs, compression.DefaultCompressionConfig())
		if result.Stats.Ratio < 0.95 && result.Stats.Ratio > 0 {
			// Re-marshal with compressed messages
			compReq := req
			compReq.Messages = fromNormalizedMessages(result.Messages)
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

	newBody, err := json.Marshal(req)
	if err != nil {
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
		w.Header().Set("X-DOSRouter-Reasoning", decision.Reasoning)
	}

	// Forward to upstream with retry
	upstreamURL := fmt.Sprintf("%s/v1/chat/completions", s.config.UpstreamBase)
	makeReq := func() (*http.Request, error) {
		upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, bytes.NewReader(newBody))
		if err != nil {
			return nil, err
		}
		upstreamReq.Header.Set("Content-Type", "application/json")
		if s.config.UpstreamAPIKey != "" {
			upstreamReq.Header.Set("Authorization", "Bearer "+s.config.UpstreamAPIKey)
		} else {
			upstreamReq.Header.Set("Authorization", r.Header.Get("Authorization"))
		}
		return upstreamReq, nil
	}

	resp, err := retry.Do(r.Context(), makeReq, retry.WithClient(s.httpClient))
	if err != nil {
		http.Error(w, "Upstream error: "+err.Error(), http.StatusBadGateway)
		s.logRequest(resolvedModel, decision, startTime, "error")
		return
	}
	defer resp.Body.Close()

	latencyMs := time.Since(startTime).Milliseconds()

	// Stream response back
	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(resp.StatusCode)

		flusher, ok := w.(http.Flusher)
		if !ok {
			io.Copy(w, resp.Body)
			s.logRequest(resolvedModel, decision, startTime, "success")
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		var streamInputTok, streamOutputTok int
		for scanner.Scan() {
			line := scanner.Text()
			// Parse usage from streaming chunks (some providers send usage in final chunk)
			if strings.HasPrefix(line, "data: ") && line != "data: [DONE]" {
				var chunk struct {
					Usage *struct {
						PromptTokens     int `json:"prompt_tokens"`
						CompletionTokens int `json:"completion_tokens"`
					} `json:"usage"`
				}
				if json.Unmarshal([]byte(line[6:]), &chunk) == nil && chunk.Usage != nil {
					streamInputTok = chunk.Usage.PromptTokens
					streamOutputTok = chunk.Usage.CompletionTokens
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
							fmt.Fprintf(w, "data: %s\n\n", b)
							flusher.Flush()
						}
					}
				}
			}
			fmt.Fprintf(w, "%s\n", line)
			flusher.Flush()
		}
	} else {
		respBody, _ := io.ReadAll(resp.Body)

		// --- Empty turn fallback detection ---
		// If the response has empty content, no tool_calls, and finish_reason "stop",
		// treat as degraded and try the next model in the fallback chain.
		if resp.StatusCode == http.StatusOK && decision != nil && isEmptyTurn(respBody) {
			fallbackChain := router.GetFallbackChain(decision.Tier, decision.TierConfigs)
			nextModel := ""
			for i, m := range fallbackChain {
				if m == resolvedModel && i+1 < len(fallbackChain) {
					nextModel = fallbackChain[i+1]
					break
				}
			}
			if nextModel != "" {
				log.Printf("degraded response: empty turn from %s, falling back", resolvedModel)
				resolvedModel = nextModel
				req.Model = nextModel
				newBody, _ = json.Marshal(req)

				// Re-pin session to fallback model
				if sessionID != "" {
					s.sessions.SetSession(sessionID, nextModel, decision.Tier, userExplicit)
				}

				// Retry with fallback model
				makeReq = func() (*http.Request, error) {
					upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, bytes.NewReader(newBody))
					if err != nil {
						return nil, err
					}
					upstreamReq.Header.Set("Content-Type", "application/json")
					if s.config.UpstreamAPIKey != "" {
						upstreamReq.Header.Set("Authorization", "Bearer "+s.config.UpstreamAPIKey)
					} else {
						upstreamReq.Header.Set("Authorization", r.Header.Get("Authorization"))
					}
					return upstreamReq, nil
				}
				fbResp, fbErr := retry.Do(r.Context(), makeReq, retry.WithClient(s.httpClient))
				if fbErr == nil {
					defer fbResp.Body.Close()
					respBody, _ = io.ReadAll(fbResp.Body)
					resp = fbResp
					w.Header().Set("X-DOSRouter-Fallback", nextModel)
					w.Header().Set("X-DOSRouter-Model", nextModel)
				}
			}
		}

		// Cache the response
		s.cache.Set(body, cache.Entry{
			Body:       respBody,
			StatusCode: resp.StatusCode,
			Header:     resp.Header,
		})

		// Inject usage.cost into non-streaming response (upstream v0.12.146)
		if resp.StatusCode == http.StatusOK && decision != nil {
			var parsed map[string]interface{}
			if json.Unmarshal(respBody, &parsed) == nil {
				// Overwrite model with actual resolved model
				parsed["model"] = resolvedModel
				// Inject cost breakdown if usage tokens available
				if usage, ok := parsed["usage"].(map[string]interface{}); ok {
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

		// Copy headers
		for k, v := range resp.Header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
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

	// Record spend
	if decision != nil && decision.CostEstimate > 0 {
		_ = s.spendControl.Record(decision.CostEstimate, resolvedModel, "chat")
		if sessionID != "" {
			s.sessions.AddSessionCost(sessionID, int64(decision.CostEstimate*1_000_000))
		}
	}

	// Log usage
	_ = latencyMs
	s.logRequest(resolvedModel, decision, startTime, "success")
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
	logger.LogUsage(logger.UsageEntry{
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

func mustSpendControl() *spendcontrol.SpendControl {
	sc, err := spendcontrol.New(spendcontrol.NewFileStorage())
	if err != nil {
		// Non-fatal: start with empty state
		sc, _ = spendcontrol.New(nil)
	}
	return sc
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
	}
	// Full health includes session/journal stats
	if r.URL.Query().Get("full") == "true" {
		sessStats := s.sessions.GetStats()
		jStats := s.journal.GetStats()
		resp["sessions"] = sessStats.Count
		resp["journalSessions"] = jStats.Sessions
		resp["journalEntries"] = jStats.TotalEntries
		resp["spendControl"] = s.spendControl.GetStatus()
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
