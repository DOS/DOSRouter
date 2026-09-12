package models

import (
	"strings"
	"testing"

	"github.com/DOS/DOSRouter/router"
)

// Catalog prices and limits feed budget admission before a request is sent.
// Cover new targets and material corrections so omitted or stale paid entries
// cannot bypass cost checks or advertise invalid context/output limits.
func TestSyncedCatalogEconomicsAndLimits(t *testing.T) {
	cases := []struct {
		id            string
		input, output float64
		context, max  int
	}{
		{"openai/gpt-5.6-sol", 4, 20, 1050000, 128000},
		{"openai/gpt-5.6-terra", 2, 12, 1050000, 128000},
		{"openai/gpt-5.6-luna", 0.2, 1.2, 1050000, 128000},
		{"openai/chat-latest", 5, 30, 128000, 128000},
		{"anthropic/claude-fable-5", 10, 50, 1000000, 128000},
		{"anthropic/claude-sonnet-4.6", 3, 15, 1000000, 128000},
		{"anthropic/claude-sonnet-5", 3, 15, 1000000, 128000},
		{"moonshot/kimi-k3", 3, 15, 1048576, 65536},
		{"google/gemini-3.8-flash", 0.75, 3.75, 1048576, 65536},
		{"google/gemini-3.6-flash", 0.75, 3.75, 1048576, 65536},
		{"deepseek/deepseek-v4-pro", 1.32, 3.96, 1048576, 65536},
		{"deepseek/deepseek-v4-flash-vision-exp", 0.44, 1.32, 1048576, 65536},
		{"qwen/qwen3.8-flash", 0.15, 0.47, 1000000, 131072},
		{"xiaomi/mimo-v2.5", 0.14, 0.28, 1048576, 131072},
		{"zai/glm-5.3", 1.4, 4.4, 1000000, 131072},
		{"zai/glm-5.3-flash", 0.15, 0.5, 1000000, 131072},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			m := GetModel(tc.id)
			if m == nil {
				t.Fatal("model missing from budget metadata")
			}
			if m.InputPrice != tc.input || m.OutputPrice != tc.output {
				t.Errorf("prices = %g/%g, want %g/%g", m.InputPrice, m.OutputPrice, tc.input, tc.output)
			}
			if m.ContextWindow != tc.context || m.MaxOutput != tc.max {
				t.Errorf("context/output = %d/%d, want %d/%d", m.ContextWindow, m.MaxOutput, tc.context, tc.max)
			}
		})
	}
}

func TestCatalogIdentityAndRetirementInvariants(t *testing.T) {
	seen := make(map[string]bool)
	for _, m := range Models {
		if seen[m.ID] {
			t.Errorf("duplicate model ID %q", m.ID)
		}
		seen[m.ID] = true
		if m.ContextWindow <= 0 || m.MaxOutput <= 0 {
			t.Errorf("model %q lacks positive limits", m.ID)
		}
		if !m.Deprecated && strings.Contains(m.ID, "/") && ResolveModelAlias(m.ID) != m.ID {
			t.Errorf("active model %q is shadowed by an alias", m.ID)
		}
		if m.Deprecated {
			fallback := GetModel(m.FallbackModel)
			if fallback == nil || fallback.Deprecated {
				t.Errorf("retired model %q has missing/retired fallback %q", m.ID, m.FallbackModel)
			}
			if strings.HasPrefix(m.ID, "free/") && !strings.HasPrefix(m.FallbackModel, "free/") {
				t.Errorf("retired free model %q redirects to a paid model", m.ID)
			}
		}
		if strings.HasPrefix(m.ID, "free/") {
			if m.InputPrice != 0 || m.OutputPrice != 0 || GetActivePromoPrice(m.ID) != nil {
				t.Errorf("free model %q has a charge", m.ID)
			}
			if m.Vision || m.ToolCalling {
				t.Errorf("free model %q advertises unsupported image/tool eligibility", m.ID)
			}
		}
	}
	// ResolveModelAlias is single-pass: every target needs immediate metadata.
	for alias, target := range ModelAliases {
		if GetModel(target) == nil {
			t.Errorf("alias %q points at uncatalogued target %q", alias, target)
		}
	}
}

func TestDefaultRoutingTargetsHaveActiveBudgetMetadata(t *testing.T) {
	cfg := router.DefaultRoutingConfig()
	profiles := map[string]map[router.Tier]router.TierConfig{
		"auto": cfg.Tiers, "eco": cfg.EcoTiers,
		"premium": cfg.PremiumTiers, "agentic": cfg.AgenticTiers,
	}
	for profile, tiers := range profiles {
		for tier, chain := range tiers {
			for _, id := range append([]string{chain.Primary}, chain.Fallback...) {
				m := GetModel(id)
				if m == nil || m.Deprecated {
					t.Errorf("%s/%s routes to missing/retired model %q", profile, tier, id)
					continue
				}
				if !strings.HasPrefix(id, "free/") && (m.InputPrice <= 0 || m.OutputPrice <= 0) {
					t.Errorf("%s/%s paid target %q lacks budget pricing", profile, tier, id)
				}
				if strings.Contains(id, "gpt-oss-") {
					t.Errorf("%s/%s selects withheld GPT-OSS model %q", profile, tier, id)
				}
			}
		}
	}
	if free := ResolveModelAlias("free"); free != cfg.EcoTiers[router.TierSimple].Primary {
		t.Errorf("free alias %q differs from eco SIMPLE primary %q", free, cfg.EcoTiers[router.TierSimple].Primary)
	}
}

func TestSyncedVisionAndExplicitAliases(t *testing.T) {
	for _, id := range []string{
		"google/gemini-2.5-flash-lite", "google/gemini-3.8-flash",
		"qwen/qwen3.8-flash", "deepseek/deepseek-v4-flash-vision-exp",
		"xiaomi/mimo-v2.5", "zai/glm-5.3-flash",
	} {
		if !SupportsVision(id) {
			t.Errorf("verified vision model %q is excluded from image routing", id)
		}
	}
	for alias, want := range map[string]string{
		"anthropic/claude-opus-4.5": "anthropic/claude-opus-4.5",
		"openai/o1":                 "openai/o1",
		"opus-4.7":                  "anthropic/claude-opus-4.7",
		"gpt-5.6-sol-pro":           "openai/gpt-5.6-sol-pro",
		"kimi-k2.5":                 "moonshot/kimi-k2.5",
		"glm-5.2":                   "zai/glm-5.2",
		"glm":                       "zai/glm-5.3",
		"mimo-v2.5":                 "xiaomi/mimo-v2.5",
		"mimo":                      "xiaomi/mimo-v2.5-pro",
		"chat-latest":               "openai/chat-latest",
		"chatgpt-instant":           "openai/chat-latest",
		"gpt-120b":                  "free/gpt-oss-120b",
		"coder-free":                "free/north-mini-code",
		"deepseek-v4-pro":           "deepseek/deepseek-v4-pro",
	} {
		if got := ResolveModelAlias(alias); got != want {
			t.Errorf("ResolveModelAlias(%q) = %q, want %q", alias, got, want)
		}
	}
}

func TestRetiredFreeShorthandsResolveActiveSuccessor(t *testing.T) {
	aliases := []string{
		"maverick", "mistral-large", "mistral-large-3-675b", "mistral-nemotron",
		"qwen3-122b", "qwen3-next-80b", "qwen3.5-122b",
		"seed-oss", "seed-oss-36b", "step-flash", "step-3.7-flash",
	}
	for _, alias := range aliases {
		for _, prefix := range []string{"", "dosrouter/", "blockrun/", "openai/"} {
			t.Run(prefix+alias, func(t *testing.T) {
				resolved := ResolveModelAlias(prefix + alias)
				if resolved != "free/nemotron-3.5-lightning" {
					t.Fatalf("resolved = %q, want active free successor", resolved)
				}
				model := GetModel(resolved)
				if model == nil || model.Deprecated || model.InputPrice != 0 || model.OutputPrice != 0 {
					t.Errorf("free shorthand %q resolved to unavailable or paid metadata: %+v", alias, model)
				}
			})
		}
	}
	// The proxy resolves aliases once and does not interpret FallbackModel.
	// A future free shorthand must therefore target an active row immediately.
	for alias, target := range ModelAliases {
		if strings.Contains(alias, "/") || !strings.HasPrefix(target, "free/") {
			continue
		}
		model := GetModel(target)
		if model == nil || model.Deprecated {
			t.Errorf("free shorthand %q still targets missing/retired model %q", alias, target)
		}
	}
}

func TestRetiredAliasFixPreservesQualifiedAndPaidVersionPins(t *testing.T) {
	pins := map[string]string{
		"free/step-3.7-flash":                 "free/step-3.7-flash",
		"free/mistral-nemotron":               "free/mistral-nemotron",
		"free/qwen3.5-122b-a10b":              "free/qwen3.5-122b-a10b",
		"nvidia/glm-4.7":                      "free/glm-4.7",
		"nvidia/llama-4-maverick":             "free/llama-4-maverick",
		"nvidia/mistral-nemotron":             "free/mistral-nemotron",
		"nvidia/nemotron-nano-12b-v2-vl":      "free/nemotron-nano-12b-v2-vl",
		"nvidia/nemotron-nano-9b-v2":          "free/nemotron-nano-9b-v2",
		"nvidia/qwen3-coder-480b":             "free/qwen3-coder-480b",
		"nvidia/qwen3-next-80b-a3b-instruct":  "free/qwen3-next-80b-a3b-instruct",
		"nvidia/qwen3-next-80b-a3b-thinking":  "free/qwen3-next-80b-a3b-instruct",
		"nvidia/qwen3.5-122b-a10b":            "free/qwen3.5-122b-a10b",
		"nvidia/seed-oss-36b":                 "free/seed-oss-36b",
		"nvidia/step-3.7-flash":               "free/step-3.7-flash",
		"qwen/qwen3-coder-480b-a35b-instruct": "free/qwen3-coder-480b",
		"minimax-m2.5":                        "minimax/minimax-m2.5",
		"minimax/minimax-m2.5":                "minimax/minimax-m2.5",
	}
	for pin, want := range pins {
		for _, prefix := range []string{"", "dosrouter/", "blockrun/"} {
			if got := ResolveModelAlias(prefix + pin); got != want {
				t.Errorf("ResolveModelAlias(%q) = %q, want preserved pin %q", prefix+pin, got, want)
			}
		}
	}
}
