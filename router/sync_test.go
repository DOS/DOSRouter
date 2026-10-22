package router

import (
	"strings"
	"testing"
)

func TestSyncKeywordBoundaries(t *testing.T) {
	for _, pair := range [][2]string{{"classic", "class"}, {"history", "story"}, {"credit", "edit"}, {"prefix", "fix"}, {"step 10", "step 1"}} {
		d := scoreKeywordMatch(pair[0], []string{pair[1]}, "test", "test", 1, 2, 0, 0.5, 1)
		if d.Score != 0 {
			t.Errorf("%q matched %q", pair[0], pair[1])
		}
	}
	for _, pair := range [][2]string{{"classes", "class"}, {"debugging", "debug"}, {"implementing", "implement"}, {"请step by step证明", "step by step"}} {
		d := scoreKeywordMatch(pair[0], []string{pair[1]}, "test", "test", 1, 2, 0, 0.5, 1)
		if d.Score == 0 {
			t.Errorf("%q did not match %q", pair[0], pair[1])
		}
	}
}

func TestSyncHostSchemasDoNotUpgradeUserTurn(t *testing.T) {
	cfg := DefaultRoutingConfig()
	cfg.Scoring.ConfidenceThreshold = 0
	cfg.Overrides.StructuredOutputMinTier = TierReasoning
	opt := RouterOptions{Config: cfg, ModelPricing: map[string]ModelPricing{}}
	a := (&RulesStrategy{}).Route("hello", "", 16, opt)
	b := (&RulesStrategy{}).Route("hello", "Tools use JSON structured schema. Be quick.", 16, opt)
	if a.Tier != b.Tier {
		t.Fatalf("host schema changed tier: %s -> %s", a.Tier, b.Tier)
	}
}

func TestSyncCJKCapacityEstimate(t *testing.T) {
	cfg := DefaultRoutingConfig()
	cfg.Overrides.MaxTokensForceComplex = 900
	d := (&RulesStrategy{}).Route(strings.Repeat("中", 1000), "", 16, RouterOptions{Config: cfg})
	if !strings.Contains(d.Reasoning, "Input exceeds") {
		t.Fatalf("CJK request undercounted: %s", d.Reasoning)
	}
}

func TestSyncUnknownPriceDoesNotClaimSavings(t *testing.T) {
	d := SelectModel(TierSimple, 1, "rules", "", map[Tier]TierConfig{TierSimple: {Primary: "unknown/model"}}, nil, 100, 100, "auto", 0)
	if d.Savings != 0 {
		t.Fatalf("unknown price claims savings: %g", d.Savings)
	}
}
