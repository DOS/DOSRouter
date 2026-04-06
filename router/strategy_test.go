package router

import (
	"testing"
	"time"
)

func TestRulesStrategy_Route(t *testing.T) {
	config := DefaultRoutingConfig()
	pricing := map[string]ModelPricing{
		"google/gemini-2.5-flash":        {InputPrice: 0.15, OutputPrice: 0.60},
		"moonshot/kimi-k2.5":             {InputPrice: 0.60, OutputPrice: 3.0},
		"google/gemini-3.1-pro":          {InputPrice: 1.25, OutputPrice: 10.0},
		"xai/grok-4-1-fast-reasoning":    {InputPrice: 0.20, OutputPrice: 0.50},
		"anthropic/claude-opus-4.6":       {InputPrice: 5.0, OutputPrice: 25.0},
	}

	tests := []struct {
		name    string
		prompt  string
		profile string
		wantMin Tier // minimum expected tier
	}{
		{"simple auto", "hello", "auto", TierSimple},
		{"medium+ auto", "Write a distributed system with kubernetes microservices, step 1: design the architecture, step 2: implement the algorithm", "auto", TierMedium},
		{"reasoning auto", "prove this theorem step by step using formal mathematical proof", "auto", TierReasoning},
	}

	strategy := &RulesStrategy{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := strategy.Route(tt.prompt, "", 4096, RouterOptions{
				Config:         config,
				ModelPricing:   pricing,
				RoutingProfile: tt.profile,
			})

			t.Logf("tier=%s model=%s confidence=%.2f savings=%.0f%% reasoning=%s",
				decision.Tier, decision.Model, decision.Confidence, decision.Savings*100, decision.Reasoning)

			if TierRank(decision.Tier) < TierRank(tt.wantMin) {
				t.Errorf("tier %s below minimum %s", decision.Tier, tt.wantMin)
			}
		})
	}
}

func TestApplyPromotions(t *testing.T) {
	baseTiers := map[Tier]TierConfig{
		TierSimple: {Primary: "google/gemini-2.5-flash", Fallback: []string{"deepseek/deepseek-chat"}},
		TierMedium: {Primary: "moonshot/kimi-k2.5", Fallback: []string{}},
	}

	promos := []Promotion{
		{
			Name:      "Test Promo",
			StartDate: "2026-04-01",
			EndDate:   "2026-04-15",
			TierOverrides: map[Tier]PartialTierConfig{
				TierSimple: {Primary: "zai/glm-5"},
			},
			Profiles: []string{"auto"},
		},
	}

	// During promo window
	during := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	result := applyPromotions(baseTiers, promos, "auto", during)
	if result[TierSimple].Primary != "zai/glm-5" {
		t.Errorf("expected promo model, got %s", result[TierSimple].Primary)
	}
	// Medium should be unchanged
	if result[TierMedium].Primary != "moonshot/kimi-k2.5" {
		t.Errorf("MEDIUM should be unchanged, got %s", result[TierMedium].Primary)
	}

	// After promo window
	after := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	result = applyPromotions(baseTiers, promos, "auto", after)
	if result[TierSimple].Primary != "google/gemini-2.5-flash" {
		t.Errorf("expected original model after promo, got %s", result[TierSimple].Primary)
	}

	// Wrong profile
	result = applyPromotions(baseTiers, promos, "eco", during)
	if result[TierSimple].Primary != "google/gemini-2.5-flash" {
		t.Errorf("eco profile should not get promo, got %s", result[TierSimple].Primary)
	}
}

func TestRoutingProfiles(t *testing.T) {
	config := DefaultRoutingConfig()
	pricing := map[string]ModelPricing{
		"google/gemini-2.5-flash":     {InputPrice: 0.15, OutputPrice: 0.60},
		"free/gpt-oss-120b":           {InputPrice: 0, OutputPrice: 0},
		"anthropic/claude-opus-4.6":    {InputPrice: 5.0, OutputPrice: 25.0},
		"moonshot/kimi-k2.5":          {InputPrice: 0.60, OutputPrice: 3.0},
	}

	strategy := &RulesStrategy{}
	prompt := "hello world"

	// Eco profile should pick cheapest
	eco := strategy.Route(prompt, "", 4096, RouterOptions{
		Config:         config,
		ModelPricing:   pricing,
		RoutingProfile: "eco",
	})
	t.Logf("eco: model=%s tier=%s profile=%s", eco.Model, eco.Tier, eco.Profile)
	if eco.Profile != "eco" {
		t.Errorf("expected eco profile, got %s", eco.Profile)
	}

	// Premium profile should pick quality
	premium := strategy.Route(prompt, "", 4096, RouterOptions{
		Config:         config,
		ModelPricing:   pricing,
		RoutingProfile: "premium",
	})
	t.Logf("premium: model=%s tier=%s profile=%s", premium.Model, premium.Tier, premium.Profile)
	if premium.Profile != "premium" {
		t.Errorf("expected premium profile, got %s", premium.Profile)
	}
}

func TestForceComplexLargeContext(t *testing.T) {
	config := DefaultRoutingConfig()
	pricing := map[string]ModelPricing{
		"google/gemini-3.1-pro": {InputPrice: 1.25, OutputPrice: 10.0},
	}

	strategy := &RulesStrategy{}

	// Generate a long prompt that exceeds maxTokensForceComplex (100k tokens)
	longPrompt := repeatStr("word ", 500_000) // ~500k chars = ~125k tokens

	decision := strategy.Route(longPrompt, "", 4096, RouterOptions{
		Config:         config,
		ModelPricing:   pricing,
		RoutingProfile: "auto",
	})

	if decision.Tier != TierComplex {
		t.Errorf("expected COMPLEX for large context, got %s", decision.Tier)
	}
}

func repeatStr(s string, n int) string {
	b := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		b = append(b, s...)
	}
	return string(b)
}
