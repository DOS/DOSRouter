package proxy

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DOS/DOSRouter/logger"
	"github.com/DOS/DOSRouter/router"
)

type requestSpend struct {
	usageKnown     bool
	server         *Server
	id             uint64
	model          string
	estimate, cost float64
	input, output  int
	completions    int
	header         http.Header
	finished       bool
	source         string
}

func (s *Server) reserveChat(w http.ResponseWriter, req chatRequest, body []byte, model string) (*requestSpend, bool) {
	if s.spendError != nil || s.spendControl == nil {
		http.Error(w, "Spending state unavailable", http.StatusServiceUnavailable)
		return nil, false
	}
	price, known := s.modelPricing[model]
	limited := len(s.spendControl.GetLimits()) > 0
	if !known && limited {
		http.Error(w, "Unknown model cost under configured spend limits", http.StatusTooManyRequests)
		return nil, false
	}
	if raw, ok := req.Extra["max_tokens"]; ok {
		var value int
		if json.Unmarshal(raw, &value) != nil || value <= 0 {
			http.Error(w, "Invalid max_tokens", http.StatusBadRequest)
			return nil, false
		}
	}
	if raw, ok := req.Extra["max_completion_tokens"]; ok {
		var value int
		if json.Unmarshal(raw, &value) != nil || value <= 0 {
			http.Error(w, "Invalid max_completion_tokens", http.StatusBadRequest)
			return nil, false
		}
	}
	output := req.MaxTokens
	if raw, ok := req.Extra["max_completion_tokens"]; ok {
		if json.Unmarshal(raw, &output) != nil {
			output = -1
		}
	}
	if output <= 0 {
		output = 4096
	}
	n := 1
	if raw, ok := req.Extra["n"]; ok {
		if json.Unmarshal(raw, &n) != nil || n < 1 {
			http.Error(w, "Invalid completion count", http.StatusBadRequest)
			return nil, false
		}
	}
	// Reserve a conservative text-token estimate including history and tools.
	estimate := (float64(len(body))*price.InputPrice + float64(output)*float64(n)*price.OutputPrice) / 1_000_000
	if price.FlatPrice != nil {
		estimate = *price.FlatPrice * float64(n)
	}
	id, check := s.spendControl.Reserve(estimate)
	if !check.Allowed {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{"error": check.Reason, "blockedBy": check.BlockedBy, "remaining": check.Remaining})
		return nil, false
	}
	return &requestSpend{server: s, id: id, model: model, estimate: estimate, cost: estimate, source: "estimate", completions: n}, true
}

func (sp *requestSpend) readUsage(body []byte) {
	var parsed struct {
		Usage *struct {
			Input  *int `json:"prompt_tokens"`
			Output *int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &parsed) == nil && parsed.Usage != nil && parsed.Usage.Input != nil && parsed.Usage.Output != nil {
		sp.input, sp.output = *parsed.Usage.Input, *parsed.Usage.Output
		sp.usageKnown = true
	}
}

func (sp *requestSpend) finish(body []byte) {
	if sp == nil || sp.finished {
		return
	}
	sp.finished = true
	if len(body) > 0 {
		sp.readUsage(body)
	}
	if cost, ok := settledCost(sp.header); ok {
		sp.cost, sp.source = cost, "gateway"
	} else if (sp.usageKnown || sp.input > 0 || sp.output > 0) && sp.input >= 0 && sp.output >= 0 {
		if p, ok := sp.server.modelPricing[sp.model]; ok {
			sp.cost = (float64(sp.input)*p.InputPrice + float64(sp.output)*p.OutputPrice) / 1_000_000
			if p.FlatPrice != nil {
				sp.cost = *p.FlatPrice * float64(sp.completions)
			}
			sp.source = "tokens"
		}
	}
	// A persistence error keeps the controller fail-closed; never retry a paid call.
	if err := sp.server.spendControl.Commit(sp.id, sp.cost, sp.model, "chat"); err != nil {
		// Backend errors may contain sensitive paths or data; keep the event generic.
		log.Print("DOSRouter: spend settlement failed; accounting state requires attention")
	}
}

func (sp *requestSpend) release() {
	if sp == nil || sp.finished {
		return
	}
	sp.finished = true
	sp.server.spendControl.Release(sp.id)
}

func settledCost(h http.Header) (float64, bool) {
	for _, name := range []string{"X-DOS-Cost-USD", "X-Blockrun-Cost-USD"} {
		raw := strings.TrimSpace(h.Get(name))
		if raw == "" {
			continue
		}
		cost, err := strconv.ParseFloat(raw, 64)
		if err == nil && cost >= 0 && !math.IsNaN(cost) && !math.IsInf(cost, 0) {
			return cost, true
		}
	}
	return 0, false
}

func gatewayRequestID(h http.Header) string {
	for _, name := range []string{"X-DOS-Request-Id", "X-Blockrun-Request-Id", "X-Request-Id", "Request-Id"} {
		if id := strings.TrimSpace(h.Get(name)); id != "" {
			return sanitizeHeaderValue(id)
		}
	}
	return ""
}

func mediaCost(h http.Header, body []byte) float64 {
	if cost, ok := settledCost(h); ok {
		return cost
	}
	var data struct {
		Price struct {
			Amount json.RawMessage `json:"amount"`
		} `json:"price"`
	}
	if json.Unmarshal(body, &data) == nil {
		raw := strings.Trim(string(data.Price.Amount), `"`)
		cost, err := strconv.ParseFloat(raw, 64)
		if err == nil && cost >= 0 && !math.IsNaN(cost) && !math.IsInf(cost, 0) {
			return cost
		}
	}
	return 0
}

func (s *Server) logSettledRequest(model string, decision *router.RoutingDecision, start time.Time, spend *requestSpend, status string) {
	entry := logger.UsageEntry{Timestamp: time.Now().UTC().Format(time.RFC3339), Model: model, Tier: "DIRECT", Cost: spend.cost, CostSource: spend.source, RequestID: gatewayRequestID(spend.header), InputTokens: spend.input, OutputTokens: spend.output, Status: status, LatencyMs: time.Since(start).Milliseconds()}
	if decision != nil {
		entry.Tier = string(decision.Tier)
		entry.BaselineCost = decision.BaselineCost
		entry.Savings = decision.Savings
	}
	s.writeUsage(entry)
}
