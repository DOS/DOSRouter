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

	"github.com/DOS/DOSRouter/models"
	"github.com/DOS/DOSRouter/router"
)

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
	}
}

// ListenAndServe starts the proxy server.
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("/v1/models", s.handleModels)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/debug", s.handleDebug)

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
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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

	// Resolve model alias
	resolvedModel := models.ResolveModelAlias(req.Model)
	isSmartRoute := resolvedModel == "auto" || resolvedModel == "eco" || resolvedModel == "premium"

	var decision *router.RoutingDecision
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
	}

	// Rewrite model in request body
	req.Model = resolvedModel
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

	// Forward to upstream
	upstreamURL := fmt.Sprintf("%s/v1/chat/completions", s.config.UpstreamBase)
	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, bytes.NewReader(newBody))
	if err != nil {
		http.Error(w, "Failed to create upstream request", http.StatusInternalServerError)
		return
	}

	upstreamReq.Header.Set("Content-Type", "application/json")
	if s.config.UpstreamAPIKey != "" {
		upstreamReq.Header.Set("Authorization", "Bearer "+s.config.UpstreamAPIKey)
	}
	// Forward original auth header if no upstream key configured
	if s.config.UpstreamAPIKey == "" {
		upstreamReq.Header.Set("Authorization", r.Header.Get("Authorization"))
	}

	resp, err := s.httpClient.Do(upstreamReq)
	if err != nil {
		http.Error(w, "Upstream error: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Stream response back
	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(resp.StatusCode)

		flusher, ok := w.(http.Flusher)
		if !ok {
			io.Copy(w, resp.Body)
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintf(w, "%s\n", line)
			flusher.Flush()
		}
	} else {
		// Copy headers
		for k, v := range resp.Header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"version": "1.0.0",
	})
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
