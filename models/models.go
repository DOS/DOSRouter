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
	"claude":   "anthropic/claude-sonnet-5",
	"sonnet":   "anthropic/claude-sonnet-5",
	"sonnet-5": "anthropic/claude-sonnet-5",
	"sonnet-4": "anthropic/claude-sonnet-4.6",
	"fable":    "anthropic/claude-fable-5",
	"claude-fable": "anthropic/claude-fable-5",
	// Opus: Opus 5 is current flagship (upstream v0.12.233). Bare and forward aliases resolve to Opus 5;
	// explicit version pins stay on their version.
	"opus":                      "anthropic/claude-opus-5",
	"opus-5":                    "anthropic/claude-opus-5",
	"opus-5.0":                  "anthropic/claude-opus-5",
	"opus-4":                    "anthropic/claude-opus-4.8",
	"opus-4.8":                  "anthropic/claude-opus-4.8",
	"opus-4-8":                  "anthropic/claude-opus-4.8",
	"opus-4.7":                  "anthropic/claude-opus-4.7",
	"opus-4-7":                  "anthropic/claude-opus-4.7",
	"opus-4.6":                  "anthropic/claude-opus-4.6",
	"opus-4-6":                  "anthropic/claude-opus-4.6",
	"haiku":                     "anthropic/claude-haiku-4.5",
	"anthropic/sonnet":          "anthropic/claude-sonnet-5",
	"anthropic/opus":            "anthropic/claude-opus-5",
	"anthropic/claude-opus-5":   "anthropic/claude-opus-5",
	"anthropic/haiku":           "anthropic/claude-haiku-4.5",
	"anthropic/claude":          "anthropic/claude-sonnet-5",
	"anthropic/claude-opus-4":   "anthropic/claude-opus-4.8",
	"anthropic/claude-opus-4-8": "anthropic/claude-opus-4.8",
	"anthropic/claude-opus-4-7": "anthropic/claude-opus-4.7",
	"anthropic/claude-opus-4-6": "anthropic/claude-opus-4.6",
	"anthropic/claude-opus-4.5": "anthropic/claude-opus-4.8",

	// OpenAI — GPT-5.6 Terra is the current default flagship (upstream v0.12.219).
	"gpt":                       "openai/gpt-4o",
	"gpt4":                      "openai/gpt-4o",
	"gpt5":                      "openai/gpt-5.6-terra",
	"gpt-5.6":                   "openai/gpt-5.6-terra",
	"gpt-5.6-terra":             "openai/gpt-5.6-terra",
	"gpt-5.6-sol":               "openai/gpt-5.6-sol",
	"gpt-5.6-luna":              "openai/gpt-5.6-luna",
	"gpt-5.5":                   "openai/gpt-5.5",
	"gpt-5.5-pro":               "openai/gpt-5.5-pro",
	"chat-latest":               "openai/chatgpt-instant",
	"chatgpt-instant":           "openai/chatgpt-instant",
	"mini":                      "openai/gpt-4o-mini",
	"nano":                      "openai/gpt-5.4-nano",
	"gpt-5.4-mini":              "openai/gpt-5.4-mini",
	"openai-codex/gpt-5.4-mini": "openai/gpt-5.4-mini",
	"codex":                     "openai/gpt-5.3-codex",
	"o1":                        "openai/o3",
	"o3":                        "openai/o3",

	// DeepSeek
	"deepseek":        "deepseek/deepseek-chat",
	"deepseek-chat":   "deepseek/deepseek-chat",
	"reasoner":        "deepseek/deepseek-reasoner",
	"deepseek-v4-pro": "deepseek/deepseek-v4-pro",

	// Kimi / Moonshot — K2.7 bare flagship, K3 available (upstream v0.12.229/230)
	"kimi":             "moonshot/kimi-k2.7",
	"moonshot":         "moonshot/kimi-k2.7",
	"kimi-k3":          "moonshot/kimi-k3",
	"kimi-k2.7":        "moonshot/kimi-k2.7",
	"kimi-k2":          "moonshot/kimi-k2.6",
	"kimi-k2.6":        "moonshot/kimi-k2.6",
	"kimi-k2.5":        "moonshot/kimi-k2.5",
	"nvidia/kimi-k2.5": "moonshot/kimi-k2.5",

	// Google
	"gemini":           "google/gemini-2.5-pro",
	"flash":            "google/gemini-2.5-flash",
	"gemini-3.5-flash": "google/gemini-3.5-flash",

	// xAI — Grok 4.5 flagship (upstream v0.12.225)
	"grok":      "xai/grok-4.5",
	"grok-4.5":  "xai/grok-4.5",
	"grok-4.3":  "xai/grok-4.3",
	"grok-fast": "xai/grok-4-fast-reasoning",
	"grok-4.20": "xai/grok-4.20-reasoning",
	"grok-4-20": "xai/grok-4.20-reasoning",

	// MiniMax — M3 flagship (upstream v0.12.200)
	"minimax":    "minimax/minimax-m3",
	"minimax-m3": "minimax/minimax-m3",

	// Qwen — Qwen 3.7 Max (upstream v0.12.231)
	"qwen3.7-max": "qwen/qwen3.7-max",
	"qwen-max":    "qwen/qwen3.7-max",

	// Free models — realigned with BlockRun server up to v0.12.245
	"nvidia":          "free/gpt-oss-120b",
	"free":            "free/gpt-oss-120b",
	"qwen-coder":      "free/llama-4-maverick",
	"qwen-coder-free": "free/llama-4-maverick",
	"qwen-thinking":   "free/qwen3-next-80b-a3b-thinking",
	"qwen3-next":      "free/qwen3-next-80b-a3b-thinking",
	"mistral-small":   "free/llama-4-maverick",
	"mistral-free":    "free/llama-4-maverick",
	// DeepSeek free redirects (V4 Flash EOL -> llama-4-maverick upstream v0.12.245)
	"deepseek-free":            "free/llama-4-maverick",
	"deepseek-v4-flash":        "free/llama-4-maverick",
	"v4-flash":                 "free/llama-4-maverick",
	"free/deepseek-v3.2":       "free/llama-4-maverick",
	"free/deepseek-v4-pro":     "free/llama-4-maverick",
	"free/deepseek-v4-flash":   "free/llama-4-maverick",
	"nvidia/deepseek-v3.2":     "free/llama-4-maverick",
	"nvidia/deepseek-v4-pro":   "free/llama-4-maverick",
	"nvidia/deepseek-v4-flash": "free/llama-4-maverick",
	"glm-free":                 "free/glm-4.7",
	"llama-free":               "free/llama-4-maverick",
	"maverick":                 "free/llama-4-maverick",
	// Retired free IDs -> successors
	"nemotron":                     "free/qwen3-next-80b-a3b-thinking",
	"nemotron-ultra":               "free/qwen3-next-80b-a3b-thinking",
	"nemotron-253b":                "free/qwen3-next-80b-a3b-thinking",
	"nemotron-super":               "free/qwen3-next-80b-a3b-thinking",
	"nemotron-49b":                 "free/qwen3-next-80b-a3b-thinking",
	"nemotron-120b":                "free/qwen3-next-80b-a3b-thinking",
	"devstral":                     "free/llama-4-maverick",
	"devstral-2":                   "free/llama-4-maverick",
	"free/nemotron-ultra-253b":     "free/qwen3-next-80b-a3b-thinking",
	"free/nemotron-3-super-120b":   "free/qwen3-next-80b-a3b-thinking",
	"free/nemotron-super-49b":      "free/qwen3-next-80b-a3b-thinking",
	"free/mistral-large-3-675b":    "free/llama-4-maverick",
	"free/mistral-small-4-119b":    "free/llama-4-maverick",
	"free/devstral-2-123b":         "free/llama-4-maverick",
	"free/qwen3-coder-480b":        "free/llama-4-maverick",
	"free/seed-oss-36b":            "free/gpt-oss-120b",
	"nvidia/nemotron-ultra-253b":   "free/qwen3-next-80b-a3b-thinking",
	"nvidia/nemotron-3-super-120b": "free/qwen3-next-80b-a3b-thinking",
	"nvidia/nemotron-super-49b":    "free/qwen3-next-80b-a3b-thinking",
	"nvidia/mistral-large-3-675b":  "free/llama-4-maverick",
	"nvidia/devstral-2-123b":       "free/llama-4-maverick",

	// Z.AI — GLM-5.2 is the flagship (upstream v0.12.211)
	"glm":     "zai/glm-5.2",
	"glm-5.2": "zai/glm-5.2",
	"glm-5.1": "zai/glm-5.1",
	"glm-5":   "zai/glm-5",

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

	// OpenAI — GPT-5.6 Terra is current flagship (upstream v0.12.219).
	{ID: "openai/gpt-5.6-terra", Name: "GPT-5.6 Terra", Version: "5.6", InputPrice: 5.0, OutputPrice: 30.0, ContextWindow: 1_050_000, MaxOutput: 128_000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-5.6-sol", Name: "GPT-5.6 Sol", Version: "5.6", InputPrice: 5.0, OutputPrice: 30.0, ContextWindow: 1_050_000, MaxOutput: 128_000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-5.6-luna", Name: "GPT-5.6 Luna", Version: "5.6", InputPrice: 5.0, OutputPrice: 30.0, ContextWindow: 1_050_000, MaxOutput: 128_000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-5.5-pro", Name: "GPT-5.5 Pro", Version: "5.5", InputPrice: 21.0, OutputPrice: 168.0, ContextWindow: 1_050_000, MaxOutput: 128_000, Reasoning: true, ToolCalling: true},
	{ID: "openai/gpt-5.5", Name: "GPT-5.5", Version: "5.5", InputPrice: 5.0, OutputPrice: 30.0, ContextWindow: 1_050_000, MaxOutput: 128_000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-5.4", Name: "GPT-5.4", Version: "5.4", InputPrice: 2.5, OutputPrice: 10.0, ContextWindow: 1_050_000, MaxOutput: 128_000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-5.4-mini", Name: "GPT-5.4 Mini", Version: "5.4", InputPrice: 0.75, OutputPrice: 4.5, ContextWindow: 400_000, MaxOutput: 128_000, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-5.4-nano", Name: "GPT-5.4 Nano", Version: "5.4", InputPrice: 0.20, OutputPrice: 1.25, ContextWindow: 1_050_000, MaxOutput: 128_000, ToolCalling: true},
	{ID: "openai/gpt-5.4-pro", Name: "GPT-5.4 Pro", Version: "5.4", InputPrice: 21.0, OutputPrice: 168.0, ContextWindow: 1_050_000, MaxOutput: 128_000, Reasoning: true, ToolCalling: true},
	{ID: "openai/gpt-5.3-codex", Name: "GPT-5.3 Codex", Version: "5.3", InputPrice: 1.75, OutputPrice: 14.0, ContextWindow: 400_000, MaxOutput: 128_000, Reasoning: true, Agentic: true, ToolCalling: true},
	{ID: "openai/chatgpt-instant", Name: "ChatGPT Instant", InputPrice: 0.50, OutputPrice: 2.00, ContextWindow: 128_000, MaxOutput: 16_384, ToolCalling: true},
	{ID: "openai/gpt-4o", Name: "GPT-4o", InputPrice: 2.5, OutputPrice: 10.0, ContextWindow: 128_000, MaxOutput: 16_384, Vision: true, ToolCalling: true},
	{ID: "openai/gpt-4o-mini", Name: "GPT-4o Mini", InputPrice: 0.15, OutputPrice: 0.6, ContextWindow: 128_000, MaxOutput: 16_384, ToolCalling: true},
	{ID: "openai/o3", Name: "o3", InputPrice: 2.0, OutputPrice: 8.0, ContextWindow: 200_000, MaxOutput: 100_000, Reasoning: true, ToolCalling: true},
	{ID: "openai/o4-mini", Name: "o4-mini", InputPrice: 1.10, OutputPrice: 4.40, ContextWindow: 200_000, MaxOutput: 100_000, Reasoning: true, ToolCalling: true},

	// Anthropic — Opus 5 is the current flagship (upstream v0.12.233).
	{ID: "anthropic/claude-opus-5", Name: "Claude Opus 5", Version: "5.0", InputPrice: 5.0, OutputPrice: 25.0, ContextWindow: 1_000_000, MaxOutput: 128_000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "anthropic/claude-opus-4.8", Name: "Claude Opus 4.8", Version: "4.8", InputPrice: 5.0, OutputPrice: 25.0, ContextWindow: 1_000_000, MaxOutput: 128_000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "anthropic/claude-opus-4.7", Name: "Claude Opus 4.7", Version: "4.7", InputPrice: 5.0, OutputPrice: 25.0, ContextWindow: 1_000_000, MaxOutput: 128_000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "anthropic/claude-opus-4.6", Name: "Claude Opus 4.6", Version: "4.6", InputPrice: 5.0, OutputPrice: 25.0, ContextWindow: 1_000_000, MaxOutput: 128_000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "anthropic/claude-sonnet-5", Name: "Claude Sonnet 5", Version: "5.0", InputPrice: 3.0, OutputPrice: 15.0, ContextWindow: 200_000, MaxOutput: 64_000, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "anthropic/claude-fable-5", Name: "Claude Fable 5", Version: "5.0", InputPrice: 3.0, OutputPrice: 15.0, ContextWindow: 200_000, MaxOutput: 64_000, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "anthropic/claude-sonnet-4.6", Name: "Claude Sonnet 4.6", Version: "4.6", InputPrice: 3.0, OutputPrice: 15.0, ContextWindow: 200_000, MaxOutput: 64_000, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "anthropic/claude-haiku-4.5", Name: "Claude Haiku 4.5", Version: "4.5", InputPrice: 0.80, OutputPrice: 4.0, ContextWindow: 200_000, MaxOutput: 8192, ToolCalling: true},

	// Google
	{ID: "google/gemini-3.5-flash", Name: "Gemini 3.5 Flash", Version: "3.5", InputPrice: 0.15, OutputPrice: 0.60, ContextWindow: 1_050_000, MaxOutput: 65_536, Vision: true, ToolCalling: true},
	{ID: "google/gemini-3.1-pro", Name: "Gemini 3.1 Pro", Version: "3.1", InputPrice: 1.25, OutputPrice: 10.0, ContextWindow: 2_000_000, MaxOutput: 65_536, Vision: true, ToolCalling: true},
	{ID: "google/gemini-3.1-flash-lite", Name: "Gemini 3.1 Flash Lite", Version: "3.1", InputPrice: 0.25, OutputPrice: 1.50, ContextWindow: 1_050_000, MaxOutput: 65_536, ToolCalling: true},
	{ID: "google/gemini-3-pro-preview", Name: "Gemini 3 Pro Preview", Version: "3.0", InputPrice: 1.25, OutputPrice: 10.0, ContextWindow: 1_050_000, MaxOutput: 65_536, Vision: true, ToolCalling: true, Deprecated: true, FallbackModel: "google/gemini-3.1-pro"},
	{ID: "google/gemini-3-flash-preview", Name: "Gemini 3 Flash Preview", Version: "3.0", InputPrice: 0.15, OutputPrice: 0.60, ContextWindow: 1_050_000, MaxOutput: 65_536, ToolCalling: true},
	{ID: "google/gemini-2.5-pro", Name: "Gemini 2.5 Pro", Version: "2.5", InputPrice: 1.25, OutputPrice: 10.0, ContextWindow: 1_050_000, MaxOutput: 65_536, Reasoning: true, Vision: true, ToolCalling: true},
	{ID: "google/gemini-2.5-flash", Name: "Gemini 2.5 Flash", Version: "2.5", InputPrice: 0.15, OutputPrice: 0.60, ContextWindow: 1_050_000, MaxOutput: 65_536, Vision: true, ToolCalling: true},
	{ID: "google/gemini-2.5-flash-lite", Name: "Gemini 2.5 Flash Lite", Version: "2.5", InputPrice: 0.10, OutputPrice: 0.40, ContextWindow: 1_050_000, MaxOutput: 65_536, ToolCalling: true},

	// DeepSeek
	{ID: "deepseek/deepseek-v4-pro", Name: "DeepSeek V4 Pro", Version: "v4-pro", InputPrice: 0.55, OutputPrice: 2.19, ContextWindow: 1_000_000, MaxOutput: 65_536, Reasoning: true, ToolCalling: true},
	{ID: "deepseek/deepseek-chat", Name: "DeepSeek V3", InputPrice: 0.27, OutputPrice: 1.10, ContextWindow: 128_000, MaxOutput: 16_384, ToolCalling: true},
	{ID: "deepseek/deepseek-reasoner", Name: "DeepSeek R1", InputPrice: 0.55, OutputPrice: 2.19, ContextWindow: 128_000, MaxOutput: 16_384, Reasoning: true},

	// Kimi K3 & K2.7 - Moonshot flagship (upstream v0.12.229/230)
	{ID: "moonshot/kimi-k3", Name: "Kimi K3", Version: "k3", InputPrice: 1.20, OutputPrice: 5.0, ContextWindow: 262_144, MaxOutput: 65_536, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "moonshot/kimi-k2.7", Name: "Kimi K2.7", Version: "k2.7", InputPrice: 0.95, OutputPrice: 4.0, ContextWindow: 262_144, MaxOutput: 65_536, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "moonshot/kimi-k2.6", Name: "Kimi K2.6", Version: "k2.6", InputPrice: 0.95, OutputPrice: 4.0, ContextWindow: 262_144, MaxOutput: 65_536, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "moonshot/kimi-k2.5", Name: "Kimi K2.5", Version: "k2.5", InputPrice: 0.60, OutputPrice: 3.0, ContextWindow: 262_144, MaxOutput: 16_384, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "nvidia/kimi-k2.5", Name: "Kimi K2.5 (NVIDIA, retired)", Version: "k2.5", InputPrice: 0.60, OutputPrice: 3.0, ContextWindow: 262_144, MaxOutput: 8192, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true, Deprecated: true, FallbackModel: "moonshot/kimi-k2.5"},

	// Qwen
	{ID: "qwen/qwen3.7-max", Name: "Qwen 3.7 Max", Version: "3.7", InputPrice: 1.50, OutputPrice: 6.0, ContextWindow: 1_000_000, MaxOutput: 65_536, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},

	// xAI — Grok 4.5 & 4.3 (upstream v0.12.201, v0.12.225)
	{ID: "xai/grok-4.5", Name: "Grok 4.5", Version: "4.5", InputPrice: 2.0, OutputPrice: 6.0, ContextWindow: 2_000_000, MaxOutput: 16_384, Reasoning: true, ToolCalling: true},
	{ID: "xai/grok-4.3", Name: "Grok 4.3", Version: "4.3", InputPrice: 2.0, OutputPrice: 6.0, ContextWindow: 2_000_000, MaxOutput: 16_384, Reasoning: true, ToolCalling: true},
	{ID: "xai/grok-4-0709", Name: "Grok 4", InputPrice: 3.0, OutputPrice: 15.0, ContextWindow: 200_000, MaxOutput: 100_000, Vision: true, ToolCalling: true},
	{ID: "xai/grok-4-fast-reasoning", Name: "Grok 4 Fast Reasoning", InputPrice: 0.20, OutputPrice: 0.50, ContextWindow: 131_072, MaxOutput: 32_768, Reasoning: true, ToolCalling: true},
	{ID: "xai/grok-4-1-fast-reasoning", Name: "Grok 4.1 Fast Reasoning", InputPrice: 0.20, OutputPrice: 0.50, ContextWindow: 131_072, MaxOutput: 32_768, Reasoning: true, ToolCalling: true},
	{ID: "xai/grok-4-fast-non-reasoning", Name: "Grok 4 Fast", InputPrice: 0.20, OutputPrice: 0.50, ContextWindow: 131_072, MaxOutput: 32_768, ToolCalling: true},
	{ID: "xai/grok-4-1-fast-non-reasoning", Name: "Grok 4.1 Fast", InputPrice: 0.20, OutputPrice: 0.50, ContextWindow: 131_072, MaxOutput: 32_768, ToolCalling: true},
	{ID: "xai/grok-3", Name: "Grok 3", InputPrice: 3.0, OutputPrice: 15.0, ContextWindow: 131_072, MaxOutput: 32_768, ToolCalling: true},
	{ID: "xai/grok-3-mini", Name: "Grok 3 Mini", InputPrice: 0.30, OutputPrice: 0.50, ContextWindow: 131_072, MaxOutput: 32_768, Reasoning: true, ToolCalling: true},

	// Grok 4.20 family (upstream v0.12.155): 2M context, multi-agent variant.
	{ID: "xai/grok-4.20-reasoning", Name: "Grok 4.20 Reasoning", Version: "4.20", InputPrice: 2.0, OutputPrice: 6.0, ContextWindow: 2_000_000, MaxOutput: 16_384, Reasoning: true, ToolCalling: true},
	{ID: "xai/grok-4.20-non-reasoning", Name: "Grok 4.20", Version: "4.20", InputPrice: 2.0, OutputPrice: 6.0, ContextWindow: 2_000_000, MaxOutput: 16_384, ToolCalling: true},
	{ID: "xai/grok-4.20-multi-agent", Name: "Grok 4.20 Multi-Agent", Version: "4.20", InputPrice: 2.0, OutputPrice: 6.0, ContextWindow: 2_000_000, MaxOutput: 16_384, Reasoning: true, ToolCalling: true},

	// Free (NVIDIA)
	{ID: "free/gpt-oss-120b", Name: "GPT-OSS 120B (Free)", ContextWindow: 131_072, MaxOutput: 16_384},
	{ID: "free/gpt-oss-20b", Name: "GPT-OSS 20B (Free)", ContextWindow: 131_072, MaxOutput: 16_384},
	{ID: "free/qwen3-next-80b-a3b-thinking", Name: "Qwen3 Next 80B A3B Thinking (Free)", ContextWindow: 131_072, MaxOutput: 16_384, Reasoning: true},
	{ID: "free/mistral-small-4-119b", Name: "Mistral Small 4 119B (retired)", ContextWindow: 131_072, MaxOutput: 16_384, Deprecated: true, FallbackModel: "free/llama-4-maverick"},
	{ID: "free/deepseek-v4-flash", Name: "DeepSeek V4 Flash (retired)", Version: "v4-flash", ContextWindow: 1_000_000, MaxOutput: 16_384, Reasoning: true, Deprecated: true, FallbackModel: "free/llama-4-maverick"},
	{ID: "free/deepseek-v3.2", Name: "DeepSeek V3.2 (retired)", ContextWindow: 131_072, MaxOutput: 16_384, Deprecated: true, FallbackModel: "free/llama-4-maverick"},
	{ID: "free/deepseek-v4-pro", Name: "DeepSeek V4 Pro (retired)", ContextWindow: 1_000_000, MaxOutput: 16_384, Reasoning: true, Deprecated: true, FallbackModel: "free/llama-4-maverick"},
	{ID: "free/qwen3-coder-480b", Name: "Qwen3 Coder 480B (retired)", ContextWindow: 131_072, MaxOutput: 16_384, Deprecated: true, FallbackModel: "free/llama-4-maverick"},
	{ID: "free/glm-4.7", Name: "GLM 4.7 (Free)", ContextWindow: 131_072, MaxOutput: 16_384},
	{ID: "free/llama-4-maverick", Name: "Llama 4 Maverick (Free)", ContextWindow: 131_072, MaxOutput: 16_384},
	{ID: "free/seed-oss-36b", Name: "Seed OSS 36B (retired)", ContextWindow: 131_072, MaxOutput: 16_384, Deprecated: true, FallbackModel: "free/gpt-oss-120b"},
	// Retired free models (upstream v0.12.160/v0.12.245): kept for catalog back-compat, routed via FallbackModel.
	{ID: "free/nemotron-ultra-253b", Name: "Nemotron Ultra 253B (retired)", ContextWindow: 131_072, MaxOutput: 16_384, Reasoning: true, Deprecated: true, FallbackModel: "free/qwen3-next-80b-a3b-thinking"},
	{ID: "free/nemotron-super-49b", Name: "Nemotron Super 49B (retired)", ContextWindow: 131_072, MaxOutput: 16_384, Deprecated: true, FallbackModel: "free/qwen3-next-80b-a3b-thinking"},
	{ID: "free/nemotron-3-super-120b", Name: "Nemotron 3 Super 120B (retired)", ContextWindow: 131_072, MaxOutput: 16_384, Deprecated: true, FallbackModel: "free/qwen3-next-80b-a3b-thinking"},
	{ID: "free/mistral-large-3-675b", Name: "Mistral Large 3 675B (retired)", ContextWindow: 131_072, MaxOutput: 16_384, Deprecated: true, FallbackModel: "free/llama-4-maverick"},
	{ID: "free/devstral-2-123b", Name: "Devstral 2 123B (retired)", ContextWindow: 131_072, MaxOutput: 16_384, Deprecated: true, FallbackModel: "free/llama-4-maverick"},

	// Z.AI — GLM-5.2 is current flagship (upstream v0.12.211)
	{ID: "zai/glm-5.2", Name: "GLM-5.2", Version: "5.2", InputPrice: 1.6, OutputPrice: 5.0, ContextWindow: 200_000, MaxOutput: 128_000, ToolCalling: true},
	{ID: "zai/glm-5.1", Name: "GLM-5.1", Version: "5.1", InputPrice: 1.4, OutputPrice: 4.4, ContextWindow: 200_000, MaxOutput: 128_000, ToolCalling: true},
	{ID: "zai/glm-5", Name: "GLM-5", InputPrice: 0.60, OutputPrice: 1.92, ContextWindow: 131_072, MaxOutput: 16_384, ToolCalling: true},
	{ID: "zai/glm-5-turbo", Name: "GLM-5 Turbo", InputPrice: 1.20, OutputPrice: 4.00, ContextWindow: 131_072, MaxOutput: 16_384, ToolCalling: true},

	// MiniMax — M3 is current flagship with Vision (upstream v0.12.200, v0.12.236)
	{ID: "minimax/minimax-m3", Name: "MiniMax M3", Version: "m3", InputPrice: 0.50, OutputPrice: 2.00, ContextWindow: 1_050_000, MaxOutput: 128_000, Vision: true, ToolCalling: true},
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

	// Strip "dosrouter/" prefix (legacy "blockrun/" also supported)
	if strings.HasPrefix(normalized, "dosrouter/") || strings.HasPrefix(normalized, "blockrun/") {
		prefix := "dosrouter/"
		if strings.HasPrefix(normalized, "blockrun/") {
			prefix = "blockrun/"
		}
		withoutPrefix := normalized[len(prefix):]
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
