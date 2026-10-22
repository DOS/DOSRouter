package proxy

import (
	"encoding/json"
	"slices"

	"github.com/DOS/DOSRouter/router"
)

// Function-calling probe results from router-core 8773a99. These are recovery
// candidates only; they never replace a configured, eligible primary.
var verifiedToolFallbacks = []string{
	"anthropic/claude-fable-5.1", "anthropic/claude-opus-5.5", "anthropic/claude-sonnet-5.5",
	"deepseek/deepseek-v4.1-flash", "google/gemini-3.8-flash", "openai/gpt-5.1",
	"openai/gpt-6-astra", "openai/gpt-6-luna", "openai/gpt-6-sol", "openai/gpt-6.1-sol",
	"xai/grok-4.6", "xai/grok-4.7", "xiaomi/mimo-v2.5",
}

func requiresTools(req chatRequest) bool {
	if !requestHasTools(req.Tools) {
		return false
	}
	var choice string
	if json.Unmarshal(req.Extra["tool_choice"], &choice) == nil && choice == "none" {
		return false
	}
	return true
}

func smartFallbackChain(req chatRequest, decision *router.RoutingDecision, catalog catalogSnapshot) []string {
	chain := router.GetFallbackChain(decision.Tier, decision.TierConfigs)
	tools := requiresTools(req)
	if tools {
		additional := append([]string(nil), verifiedToolFallbacks...)
		if decision.Profile != "premium" {
			slices.SortStableFunc(additional, func(a, b string) int {
				pa, pb := catalog.pricing[a], catalog.pricing[b]
				ca, cb := pa.InputPrice+pa.OutputPrice, pb.InputPrice+pb.OutputPrice
				if ca < cb {
					return -1
				}
				if ca > cb {
					return 1
				}
				return 0
			})
		}
		chain = append(chain, additional...)
	}
	// Retain full-chain ordering before outcome demotion and the attempt cap.
	seen := map[string]bool{}
	result := []string{}
	for _, id := range chain {
		if seen[id] {
			continue
		}
		seen[id] = true
		m := catalog.model(id)
		if m == nil || m.Deprecated || tools && !m.ToolCalling {
			continue
		}
		result = append(result, id)
	}
	return result
}
