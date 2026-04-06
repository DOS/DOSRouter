// Package models provides model definitions, aliases, and pricing for the
// DOSRouter smart LLM routing system.
package models

import (
	"strings"
	"time"

	"github.com/DOS/DOSRouter/router"
)

// ModelDef describes a model's capabilities and pricing.
type ModelDef struct {
	ID            string
	Name          string
	Version       string
	InputPrice    float64 // per 1M tokens
	OutputPrice   float64 // per 1M tokens
	ContextWindow int
	MaxOutput     int
	Reasoning     bool
	Vision        bool
	Agentic       bool
	ToolCalling   bool
	Deprecated    bool
	FallbackModel string
	Promo         *PromoDef
}

// PromoDef is a time-limited promotional pricing.
type PromoDef struct {
	FlatPrice float64 // per request USD
	StartDate string  // ISO "2006-01-02"
	EndDate   string  // ISO "2006-01-02"
}

// ModelAliases maps shorthand names to full model IDs.
var ModelAliases = map[string]string{
	// Claude
	"claude":   "anthropic/claude-sonnet-4.6",
	"sonnet":   "anthropic/claude-sonnet-4.6",
	"sonnet-4": "anthropic/claude-sonnet-4.6",
	"opus":     "anthropic/claude-opus-4.6",
	"opus-4":   "anthropic/claude-opus-4.6",
	"haiku":    "anthropic/claude-haiku-4.5",
	"anthropic/sonnet": "anthropic/claude-sonnet-4.6",
	"anthropic/opus":   "anthropic/claude-opus-4.6",
	"anthropic/haiku":  "anthropic/claude-haiku-4.5",
	"anthropic/claude": "anthropic/claude-sonnet-4.6",

	// OpenAI
	"gpt":    "openai/gpt-4o",
	"gpt4":   "openai/gpt-4o",
	"gpt5":   "openai/gpt-5.4",
	"mini":   "openai/gpt-4o-mini",
	"nano":   "openai/gpt-5.4-nano",
	"codex":  "openai/gpt-5.3-codex",
	"o1":     "openai/o1",
	"o3":     "openai/o3",

	// DeepSeek
	"deepseek":      "deepseek/deepseek-chat",
	"deepseek-chat": "deepseek/deepseek-chat",
	"reasoner":      "deepseek/deepseek-reasoner",

	// Kimi / Moonshot
	"kimi":     "moonshot/kimi-k2.5",
	"moonshot": "moonshot/kimi-k2.5",

	// Google
	"gemini": "google/gemini-2.5-pro",
	"flash":  "google/gemini-2.5-flash",

	// xAI
	"grok":      "xai/grok-3",
	"grok-fast": "xai/grok-4-fast-reasoning",

	// Free models
	"nvidia":       "free/gpt-oss-120b",
	"free":         "free/nemotron-ultra-253b",
	"nemotron":     "free/nemotron-ultra-253b",
	"devstral":     "free/devstral-2-123b",
	"qwen-coder":   "free/qwen3-coder-480b",

	// Z.AI
	"glm":   "zai/glm-5",
	"glm-5": "zai/glm-5",

	// Routing profiles
	"auto-router": "auto",
	"router":      "auto",
}

// Models is the full catalog of supported models.
var Models = []ModelDef{
	// Smart routing meta-models
	{ID: "auto", Name: "Auto (Smart Router - Balanced)", ContextWindow: 1_050_000, MaxOutput: 128_000},
	{ID: "eco", Name: "Eco (Smart Router - Cost Optimized)", ContextWindow: 1_050_000, MaxOutput: 128_000},
	{ID: "premium", Name: "Premium (Smart Router - Best Quality)", ContextWindow: 2_000_000, MaxOutput: 200_000},
	{ID: "free", Name: "Free - Nemotron Ultra 253B", ContextWindow: 131_072, MaxOutput: 16_384, Reasoning: true},

	// OpenAI
	{ID: "openai/gpt-5.4", Name: "GPT-5.4", Version: "5.4", InputPrice: 2.5, OutputPrice: 10.0, ContextWindow: 1_050_000, MaxOutput: 128_000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-5.4-nano", Name: "GPT-5.4 Nano", Version: "5.4", InputPrice: 0.20, OutputPrice: 1.25, ContextWindow: 1_050_000, MaxOutput: 128_000, ToolCalling: true},
	{ID: "openai/gpt-5.4-pro", Name: "GPT-5.4 Pro", Version: "5.4", InputPrice: 21.0, OutputPrice: 168.0, ContextWindow: 1_050_000, MaxOutput: 128_000, Reasoning: true, ToolCalling: true},
	{ID: "openai/gpt-5.3-codex", Name: "GPT-5.3 Codex", Version: "5.3", InputPrice: 1.75, OutputPrice: 14.0, ContextWindow: 400_000, MaxOutput: 128_000, Reasoning: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-4o", Name: "GPT-4o", InputPrice: 2.5, OutputPrice: 10.0, ContextWindow: 128_000, MaxOutput: 16_384, Vision: true, ToolCalling: true},
	{ID: "openai/gpt-4o-mini", Name: "GPT-4o Mini", InputPrice: 0.15, OutputPrice: 0.6, ContextWindow: 128_000, MaxOutput: 16_384, ToolCalling: true},
	{ID: "openai/o3", Name: "o3", InputPrice: 2.0, OutputPrice: 8.0, ContextWindow: 200_000, MaxOutput: 100_000, Reasoning: true, ToolCalling: true},
	{ID: "openai/o4-mini", Name: "o4-mini", InputPrice: 1.10, OutputPrice: 4.40, ContextWindow: 200_000, MaxOutput: 100_000, Reasoning: true, ToolCalling: true},

	// Anthropic
	{ID: "anthropic/claude-opus-4.6", Name: "Claude Opus 4.6", Version: "4.6", InputPrice: 5.0, OutputPrice: 25.0, ContextWindow: 200_000, MaxOutput: 32_000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "anthropic/claude-sonnet-4.6", Name: "Claude Sonnet 4.6", Version: "4.6", InputPrice: 3.0, OutputPrice: 15.0, ContextWindow: 200_000, MaxOutput: 64_000, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "anthropic/claude-haiku-4.5", Name: "Claude Haiku 4.5", Version: "4.5", InputPrice: 0.80, OutputPrice: 4.0, ContextWindow: 200_000, MaxOutput: 8192, ToolCalling: true},

	// Google
	{ID: "google/gemini-3.1-pro", Name: "Gemini 3.1 Pro", Version: "3.1", InputPrice: 1.25, OutputPrice: 10.0, ContextWindow: 2_000_000, MaxOutput: 65_536, Vision: true, ToolCalling: true},
	{ID: "google/gemini-3.1-flash-lite", Name: "Gemini 3.1 Flash Lite", Version: "3.1", InputPrice: 0.25, OutputPrice: 1.50, ContextWindow: 1_050_000, MaxOutput: 65_536, ToolCalling: true},
	{ID: "google/gemini-3-pro-preview", Name: "Gemini 3 Pro Preview", Version: "3.0", InputPrice: 1.25, OutputPrice: 10.0, ContextWindow: 1_050_000, MaxOutput: 65_536, Vision: true, ToolCalling: true},
	{ID: "google/gemini-3-flash-preview", Name: "Gemini 3 Flash Preview", Version: "3.0", InputPrice: 0.15, OutputPrice: 0.60, ContextWindow: 1_050_000, MaxOutput: 65_536, ToolCalling: true},
	{ID: "google/gemini-2.5-pro", Name: "Gemini 2.5 Pro", Version: "2.5", InputPrice: 1.25, OutputPrice: 10.0, ContextWindow: 1_050_000, MaxOutput: 65_536, Reasoning: true, Vision: true, ToolCalling: true},
	{ID: "google/gemini-2.5-flash", Name: "Gemini 2.5 Flash", Version: "2.5", InputPrice: 0.15, OutputPrice: 0.60, ContextWindow: 1_050_000, MaxOutput: 65_536, Vision: true, ToolCalling: true},
	{ID: "google/gemini-2.5-flash-lite", Name: "Gemini 2.5 Flash Lite", Version: "2.5", InputPrice: 0.10, OutputPrice: 0.40, ContextWindow: 1_050_000, MaxOutput: 65_536, ToolCalling: true},

	// DeepSeek
	{ID: "deepseek/deepseek-chat", Name: "DeepSeek V3", InputPrice: 0.27, OutputPrice: 1.10, ContextWindow: 128_000, MaxOutput: 16_384, ToolCalling: true},
	{ID: "deepseek/deepseek-reasoner", Name: "DeepSeek R1", InputPrice: 0.55, OutputPrice: 2.19, ContextWindow: 128_000, MaxOutput: 16_384, Reasoning: true},

	// Moonshot / Kimi
	{ID: "moonshot/kimi-k2.5", Name: "Kimi K2.5", Version: "2.5", InputPrice: 0.60, OutputPrice: 3.0, ContextWindow: 131_072, MaxOutput: 16_384, Agentic: true, ToolCalling: true},

	// xAI
	{ID: "xai/grok-4-0709", Name: "Grok 4", InputPrice: 3.0, OutputPrice: 15.0, ContextWindow: 200_000, MaxOutput: 100_000, Vision: true, ToolCalling: true},
	{ID: "xai/grok-4-fast-reasoning", Name: "Grok 4 Fast Reasoning", InputPrice: 0.20, OutputPrice: 0.50, ContextWindow: 131_072, MaxOutput: 32_768, Reasoning: true, ToolCalling: true},
	{ID: "xai/grok-4-1-fast-reasoning", Name: "Grok 4.1 Fast Reasoning", InputPrice: 0.20, OutputPrice: 0.50, ContextWindow: 131_072, MaxOutput: 32_768, Reasoning: true, ToolCalling: true},
	{ID: "xai/grok-4-fast-non-reasoning", Name: "Grok 4 Fast", InputPrice: 0.20, OutputPrice: 0.50, ContextWindow: 131_072, MaxOutput: 32_768, ToolCalling: true},
	{ID: "xai/grok-4-1-fast-non-reasoning", Name: "Grok 4.1 Fast", InputPrice: 0.20, OutputPrice: 0.50, ContextWindow: 131_072, MaxOutput: 32_768, ToolCalling: true},
	{ID: "xai/grok-3", Name: "Grok 3", InputPrice: 3.0, OutputPrice: 15.0, ContextWindow: 131_072, MaxOutput: 32_768, ToolCalling: true},
	{ID: "xai/grok-3-mini", Name: "Grok 3 Mini", InputPrice: 0.30, OutputPrice: 0.50, ContextWindow: 131_072, MaxOutput: 32_768, Reasoning: true, ToolCalling: true},

	// Free (NVIDIA)
	{ID: "free/gpt-oss-120b", Name: "GPT-OSS 120B (Free)", ContextWindow: 131_072, MaxOutput: 16_384},
	{ID: "free/gpt-oss-20b", Name: "GPT-OSS 20B (Free)", ContextWindow: 131_072, MaxOutput: 16_384},
	{ID: "free/nemotron-ultra-253b", Name: "Nemotron Ultra 253B (Free)", ContextWindow: 131_072, MaxOutput: 16_384, Reasoning: true},
	{ID: "free/nemotron-super-49b", Name: "Nemotron Super 49B (Free)", ContextWindow: 131_072, MaxOutput: 16_384},
	{ID: "free/nemotron-3-super-120b", Name: "Nemotron 3 Super 120B (Free)", ContextWindow: 131_072, MaxOutput: 16_384},
	{ID: "free/deepseek-v3.2", Name: "DeepSeek V3.2 (Free)", ContextWindow: 131_072, MaxOutput: 16_384},
	{ID: "free/mistral-large-3-675b", Name: "Mistral Large 3 675B (Free)", ContextWindow: 131_072, MaxOutput: 16_384},
	{ID: "free/qwen3-coder-480b", Name: "Qwen3 Coder 480B (Free)", ContextWindow: 131_072, MaxOutput: 16_384},
	{ID: "free/devstral-2-123b", Name: "Devstral 2 123B (Free)", ContextWindow: 131_072, MaxOutput: 16_384},
	{ID: "free/glm-4.7", Name: "GLM 4.7 (Free)", ContextWindow: 131_072, MaxOutput: 16_384},
	{ID: "free/llama-4-maverick", Name: "Llama 4 Maverick (Free)", ContextWindow: 131_072, MaxOutput: 16_384},

	// Z.AI
	{ID: "zai/glm-5", Name: "GLM-5", InputPrice: 0.50, OutputPrice: 2.0, ContextWindow: 131_072, MaxOutput: 16_384, ToolCalling: true,
		Promo: &PromoDef{FlatPrice: 0.001, StartDate: "2026-04-01", EndDate: "2026-04-15"}},
	{ID: "zai/glm-5-turbo", Name: "GLM-5 Turbo", InputPrice: 0.20, OutputPrice: 0.80, ContextWindow: 131_072, MaxOutput: 16_384, ToolCalling: true},

	// MiniMax
	{ID: "minimax/minimax-m2.7", Name: "MiniMax M2.7", InputPrice: 1.0, OutputPrice: 5.0, ContextWindow: 1_050_000, MaxOutput: 128_000, ToolCalling: true},
}

// modelIndex is built on init for fast lookups.
var modelIndex map[string]*ModelDef

func init() {
	modelIndex = make(map[string]*ModelDef, len(Models))
	for i := range Models {
		modelIndex[Models[i].ID] = &Models[i]
	}
}

// ResolveModelAlias resolves a model alias to its full model ID.
func ResolveModelAlias(model string) string {
	normalized := strings.TrimSpace(strings.ToLower(model))
	if resolved, ok := ModelAliases[normalized]; ok {
		return resolved
	}

	// Strip "blockrun/" prefix
	if strings.HasPrefix(normalized, "blockrun/") {
		withoutPrefix := normalized[len("blockrun/"):]
		if resolved, ok := ModelAliases[withoutPrefix]; ok {
			return resolved
		}
		return withoutPrefix
	}

	// Strip "openai/" prefix for virtual profiles
	if strings.HasPrefix(normalized, "openai/") {
		withoutPrefix := normalized[len("openai/"):]
		if resolved, ok := ModelAliases[withoutPrefix]; ok {
			return resolved
		}
		if _, ok := modelIndex[withoutPrefix]; ok {
			return withoutPrefix
		}
	}

	return model
}

// GetModel returns a model definition by ID, or nil if not found.
func GetModel(id string) *ModelDef {
	return modelIndex[id]
}

// GetModelContextWindow returns the context window for a model, or 0 if unknown.
func GetModelContextWindow(modelID string) (int, bool) {
	m := modelIndex[modelID]
	if m == nil {
		return 0, false
	}
	return m.ContextWindow, true
}

// IsReasoningModel returns true if the model supports reasoning.
func IsReasoningModel(modelID string) bool {
	m := modelIndex[modelID]
	return m != nil && m.Reasoning
}

// SupportsToolCalling returns true if the model supports structured tool calling.
func SupportsToolCalling(modelID string) bool {
	m := modelIndex[modelID]
	return m != nil && m.ToolCalling
}

// SupportsVision returns true if the model supports image inputs.
func SupportsVision(modelID string) bool {
	m := modelIndex[modelID]
	return m != nil && m.Vision
}

// GetActivePromoPrice returns the promo flat price if a model has an active promo.
func GetActivePromoPrice(modelID string) *float64 {
	m := modelIndex[modelID]
	if m == nil || m.Promo == nil {
		return nil
	}
	now := time.Now()
	start, err1 := time.Parse("2006-01-02", m.Promo.StartDate)
	end, err2 := time.Parse("2006-01-02", m.Promo.EndDate)
	if err1 != nil || err2 != nil {
		return nil
	}
	if now.Before(start) || !now.Before(end) {
		return nil
	}
	return &m.Promo.FlatPrice
}

// BuildPricingMap builds a router.ModelPricing map from the model catalog.
func BuildPricingMap() map[string]router.ModelPricing {
	pm := make(map[string]router.ModelPricing, len(Models))
	for _, m := range Models {
		mp := router.ModelPricing{
			InputPrice:  m.InputPrice,
			OutputPrice: m.OutputPrice,
		}
		if promo := GetActivePromoPrice(m.ID); promo != nil {
			mp.FlatPrice = promo
		}
		pm[m.ID] = mp
	}
	return pm
}
