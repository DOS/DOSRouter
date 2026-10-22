package proxy

import (
	"encoding/json"
	"testing"

	"github.com/DOS/DOSRouter/router"
)

func TestVerifiedToolsAreRecoveryOnly(t *testing.T) {
	srv, _ := syncTestServer(t, nil, nil)
	decision := &router.RoutingDecision{Tier: router.TierSimple, Profile: "auto", TierConfigs: map[router.Tier]router.TierConfig{router.TierSimple: {Primary: "openai/gpt-4o-mini", Fallback: []string{"free/nemotron-3.5-lightning"}}}}
	req := chatRequest{Tools: json.RawMessage(`[{"type":"function","function":{"name":"test"}}]`)}
	chain := smartFallbackChain(req, decision, srv.catalogView())
	if chain[0] != "openai/gpt-4o-mini" {
		t.Fatal("verified fallback displaced configured primary")
	}
	found := false
	for _, id := range chain {
		if id == "free/nemotron-3.5-lightning" {
			t.Fatal("tool-ineligible free model retained")
		}
		if id == "openai/gpt-6.1-sol" {
			found = true
		}
	}
	if !found {
		t.Fatal("new verified tool recovery model omitted")
	}
	req.Extra = map[string]json.RawMessage{"tool_choice": json.RawMessage(`"none"`)}
	if len(smartFallbackChain(req, decision, srv.catalogView())) != 2 {
		t.Fatal("disabled tools still changed chain")
	}
}
