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
	// Upstream chat aliases plus DOS-compatible shorthand targets.
	// Bare Claude/Sonnet, o1, Gemini, Flash and paid DeepSeek Pro retain DOS behavior.
	"anthropic/claude":              "anthropic/claude-sonnet-5",
	"anthropic/claude-fable-5.0":    "anthropic/claude-fable-5",
	"anthropic/claude-haiku-4":      "anthropic/claude-haiku-4.5",
	"anthropic/claude-haiku-4-5":    "anthropic/claude-haiku-4.5",
	"anthropic/claude-opus-4":       "anthropic/claude-opus-4.8",
	"anthropic/claude-opus-4-5":     "anthropic/claude-opus-4.5",
	"anthropic/claude-opus-4-6":     "anthropic/claude-opus-4.6",
	"anthropic/claude-opus-4-7":     "anthropic/claude-opus-4.7",
	"anthropic/claude-opus-4-8":     "anthropic/claude-opus-4.8",
	"anthropic/claude-opus-5":       "anthropic/claude-opus-5",
	"anthropic/claude-opus-5-0":     "anthropic/claude-opus-5",
	"anthropic/claude-opus-5.0":     "anthropic/claude-opus-5",
	"anthropic/claude-sonnet-4":     "anthropic/claude-sonnet-4.6",
	"anthropic/claude-sonnet-4-5":   "anthropic/claude-sonnet-4.5",
	"anthropic/claude-sonnet-4-6":   "anthropic/claude-sonnet-4.6",
	"anthropic/fable":               "anthropic/claude-fable-5",
	"anthropic/haiku":               "anthropic/claude-haiku-4.5",
	"anthropic/opus":                "anthropic/claude-opus-5",
	"anthropic/sonnet":              "anthropic/claude-sonnet-5",
	"auto-router":                   "auto",
	"br-sonnet":                     "anthropic/claude-sonnet-4.6",
	"chat-latest":                   "openai/chat-latest",
	"chatgpt":                       "openai/chat-latest",
	"chatgpt-instant":               "openai/chat-latest",
	"claude":                        "anthropic/claude-sonnet-5",
	"claude-fable":                  "anthropic/claude-fable-5",
	"coder-free":                    "free/north-mini-code",
	"codex":                         "openai/gpt-5.3-codex",
	"cohere/north-mini-code":        "free/north-mini-code",
	"deepseek":                      "deepseek/deepseek-chat",
	"deepseek-chat":                 "deepseek/deepseek-chat",
	"deepseek-free":                 "free/nemotron-3.5-lightning",
	"deepseek-v4-flash":             "free/nemotron-3.5-lightning",
	"deepseek-v4-pro":               "deepseek/deepseek-v4-pro",
	"deepseek-vision":               "deepseek/deepseek-v4-flash-vision-exp",
	"devstral":                      "free/nemotron-3.5-lightning",
	"devstral-2":                    "free/nemotron-3.5-lightning",
	"fable":                         "anthropic/claude-fable-5",
	"fable-5":                       "anthropic/claude-fable-5",
	"fable-5.0":                     "anthropic/claude-fable-5",
	"flash":                         "google/gemini-2.5-flash",
	"free":                          "free/nemotron-3.5-lightning",
	"free/deepseek-v3.2":            "free/nemotron-3.5-lightning",
	"free/deepseek-v4-flash":        "free/nemotron-3.5-lightning",
	"free/deepseek-v4-pro":          "free/nemotron-3.5-lightning",
	"free/devstral-2-123b":          "free/nemotron-3.5-lightning",
	"free/mistral-large-3-675b":     "free/nemotron-3.5-lightning",
	"free/mistral-small-4-119b":     "free/nemotron-3.5-lightning",
	"free/nemotron-3-super-120b":    "free/nemotron-3.5-lightning",
	"free/nemotron-super-49b":       "free/nemotron-3.5-lightning",
	"free/nemotron-ultra-253b":      "free/nemotron-3.5-lightning",
	"free/qwen3-coder-480b":         "free/nemotron-3.5-lightning",
	"free/seed-oss-36b":             "free/nemotron-3.5-lightning",
	"gemini":                        "google/gemini-2.5-pro",
	"gemini-2.5-flash-lite":         "google/gemini-2.5-flash-lite",
	"gemini-3-pro":                  "google/gemini-3.1-pro",
	"gemini-3-pro-preview":          "google/gemini-3.1-pro",
	"gemini-3.1-flash-lite":         "google/gemini-3.1-flash-lite",
	"gemini-3.1-pro":                "google/gemini-3.1-pro",
	"gemini-3.1-pro-preview":        "google/gemini-3.1-pro",
	"gemini-3.5-flash":              "google/gemini-3.5-flash",
	"gemini-3.5-flash-lite":         "google/gemini-3.5-flash-lite",
	"gemini-3.6":                    "google/gemini-3.6-flash",
	"gemini-3.6-flash":              "google/gemini-3.6-flash",
	"gemini-pro":                    "google/gemini-3.1-pro",
	"glm":                           "zai/glm-5.3",
	"glm-5":                         "zai/glm-5",
	"glm-5-3":                       "zai/glm-5.3",
	"glm-5-3-flash":                 "zai/glm-5.3-flash",
	"glm-5-turbo":                   "zai/glm-5-turbo",
	"glm-5.1":                       "zai/glm-5.1",
	"glm-5.2":                       "zai/glm-5.2",
	"glm-5.3":                       "zai/glm-5.3",
	"glm-5.3-flash":                 "zai/glm-5.3-flash",
	"glm-flash":                     "zai/glm-5.3-flash",
	"glm-free":                      "free/nemotron-3.5-lightning",
	"google/gemini-3-pro-preview":   "google/gemini-3.1-pro",
	"google/gemini-3.1-pro-preview": "google/gemini-3.1-pro",
	"gpt":                           "openai/gpt-4o",
	"gpt-120b":                      "free/gpt-oss-120b",
	"gpt-20b":                       "free/gpt-oss-20b",
	"gpt-5-nano":                    "openai/gpt-5.4-nano",
	"gpt-5.4":                       "openai/gpt-5.4",
	"gpt-5.4-mini":                  "openai/gpt-5.4-mini",
	"gpt-5.4-nano":                  "openai/gpt-5.4-nano",
	"gpt-5.4-pro":                   "openai/gpt-5.4-pro",
	"gpt-5.5":                       "openai/gpt-5.5",
	"gpt-5.5-pro":                   "openai/gpt-5.5-pro",
	"gpt-5.6":                       "openai/gpt-5.6-terra",
	"gpt-5.6-luna":                  "openai/gpt-5.6-luna",
	"gpt-5.6-luna-pro":              "openai/gpt-5.6-luna-pro",
	"gpt-5.6-sol":                   "openai/gpt-5.6-sol",
	"gpt-5.6-sol-pro":               "openai/gpt-5.6-sol-pro",
	"gpt-5.6-terra":                 "openai/gpt-5.6-terra",
	"gpt-5.6-terra-pro":             "openai/gpt-5.6-terra-pro",
	"gpt4":                          "openai/gpt-4o",
	"gpt5":                          "openai/gpt-5.6-terra",
	"grok":                          "xai/grok-4.5",
	"grok-4-20":                     "xai/grok-4.20-reasoning",
	"grok-4-5":                      "xai/grok-4.5",
	"grok-4.20":                     "xai/grok-4.20-reasoning",
	"grok-4.3":                      "xai/grok-4.3",
	"grok-4.5":                      "xai/grok-4.5",
	"grok-build":                    "xai/grok-build-0.1",
	"grok-code":                     "xai/grok-build-0.1",
	"grok-code-fast-1":              "deepseek/deepseek-chat",
	"grok-fast":                     "xai/grok-4-fast-reasoning",
	"haiku":                         "anthropic/claude-haiku-4.5",
	"hunyuan":                       "tencent/hy3",
	"hy3":                           "tencent/hy3",
	"kimi":                          "moonshot/kimi-k2.7",
	"kimi-k2":                       "moonshot/kimi-k2.6",
	"kimi-k2.5":                     "moonshot/kimi-k2.5",
	"kimi-k2.6":                     "moonshot/kimi-k2.6",
	"kimi-k2.7":                     "moonshot/kimi-k2.7",
	"kimi-k3":                       "moonshot/kimi-k3",
	"laguna":                        "free/laguna-xs-2.1",
	"laguna-xs":                     "free/laguna-xs-2.1",
	"lightning":                     "free/nemotron-3.5-lightning",
	"llama-3.2-vision":              "free/llama-3.2-11b-vision",
	"llama-free":                    "free/llama-3.2-11b-vision",
	"llama-vision":                  "free/llama-3.2-11b-vision",
	"luna-pro":                      "openai/gpt-5.6-luna-pro",
	"maverick":                      "free/llama-4-maverick",
	"mimo":                          "xiaomi/mimo-v2.5-pro",
	"mimo-v2.5":                     "xiaomi/mimo-v2.5",
	"mimo-v2.5-pro":                 "xiaomi/mimo-v2.5-pro",
	"mimo-vision":                   "xiaomi/mimo-v2.5",
	"mini":                          "openai/gpt-4o-mini",
	"minimax":                       "minimax/minimax-m3",
	"minimax-m2.5":                  "minimax/minimax-m2.5",
	"minimax-m2.7":                  "minimax/minimax-m2.7",
	"minimax-m3":                    "minimax/minimax-m3",
	"mistral-free":                  "free/nemotron-3.5-lightning",
	"mistral-large":                 "free/mistral-large-3-675b",
	"mistral-large-3-675b":          "free/mistral-large-3-675b",
	"mistral-nemotron":              "free/mistral-nemotron",
	"mistral-small":                 "free/nemotron-3.5-lightning",
	"moonshot":                      "moonshot/kimi-k2.7",
	"nano":                          "openai/gpt-5.4-nano",
	"nano-30b":                      "free/nemotron-3-nano-30b",
	"nano-omni":                     "free/nemotron-3-nano-omni-30b-a3b-reasoning",
	"nano-vl":                       "free/nemotron-3-nano-omni-30b-a3b-reasoning",
	"nemotron":                      "free/nemotron-3.5-lightning",
	"nemotron-120b":                 "free/nemotron-3.5-lightning",
	"nemotron-253b":                 "free/nemotron-3.5-lightning",
	"nemotron-3.5-lightning":        "free/nemotron-3.5-lightning",
	"nemotron-49b":                  "free/nemotron-3.5-lightning",
	"nemotron-lightning":            "free/nemotron-3.5-lightning",
	"nemotron-nano":                 "free/nemotron-3-nano-30b",
	"nemotron-nano-30b":             "free/nemotron-3-nano-30b",
	"nemotron-nano-9b":              "free/nemotron-3-nano-30b",
	"nemotron-nano-vl":              "free/nemotron-3-nano-omni-30b-a3b-reasoning",
	"nemotron-omni":                 "free/nemotron-3-nano-omni-30b-a3b-reasoning",
	"nemotron-super":                "free/nemotron-3.5-lightning",
	"nemotron-ultra":                "free/nemotron-3.5-lightning",
	"nemotron-ultra-550b":           "free/nemotron-3-ultra-550b",
	"north-mini":                    "free/north-mini-code",
	"north-mini-code":               "free/north-mini-code",
	"nvidia":                        "free/nemotron-3.5-lightning",
	"nvidia/deepseek-v3.2":          "free/nemotron-3.5-lightning",
	"nvidia/deepseek-v4-flash":      "free/nemotron-3.5-lightning",
	"nvidia/deepseek-v4-pro":        "free/nemotron-3.5-lightning",
	"nvidia/devstral-2-123b":        "free/nemotron-3.5-lightning",
	"nvidia/glm-4.7":                "free/glm-4.7",
	"nvidia/gpt-oss-120b":           "free/gpt-oss-120b",
	"nvidia/gpt-oss-20b":            "free/gpt-oss-20b",
	"nvidia/kimi-k2.5":              "moonshot/kimi-k2.5",
	"nvidia/llama-3.2-11b-vision":   "free/llama-3.2-11b-vision",
	"nvidia/llama-4-maverick":       "free/llama-4-maverick",
	"nvidia/mistral-large-3-675b":   "free/nemotron-3.5-lightning",
	"nvidia/mistral-nemotron":       "free/mistral-nemotron",
	"nvidia/nemotron-3-nano-30b":    "free/nemotron-3-nano-30b",
	"nvidia/nemotron-3-nano-omni-30b-a3b-reasoning": "free/nemotron-3-nano-omni-30b-a3b-reasoning",
	"nvidia/nemotron-3-super-120b":                  "free/nemotron-3.5-lightning",
	"nvidia/nemotron-3-ultra-550b":                  "free/nemotron-3-ultra-550b",
	"nvidia/nemotron-3.5-lightning":                 "free/nemotron-3.5-lightning",
	"nvidia/nemotron-nano-12b-v2-vl":                "free/nemotron-nano-12b-v2-vl",
	"nvidia/nemotron-nano-9b-v2":                    "free/nemotron-nano-9b-v2",
	"nvidia/nemotron-super-49b":                     "free/nemotron-3.5-lightning",
	"nvidia/nemotron-ultra-253b":                    "free/nemotron-3.5-lightning",
	"nvidia/qwen3-coder-480b":                       "free/qwen3-coder-480b",
	"nvidia/qwen3-next-80b-a3b-instruct":            "free/qwen3-next-80b-a3b-instruct",
	"nvidia/qwen3-next-80b-a3b-thinking":            "free/qwen3-next-80b-a3b-instruct",
	"nvidia/qwen3.5-122b-a10b":                      "free/qwen3.5-122b-a10b",
	"nvidia/seed-oss-36b":                           "free/seed-oss-36b",
	"nvidia/step-3.7-flash":                         "free/step-3.7-flash",
	"o1":                                            "openai/o3",
	"o1-mini":                                       "openai/o4-mini",
	"o3":                                            "openai/o3",
	"openai-codex/gpt-5.4-mini":                     "openai/gpt-5.4-mini",
	"openai/chatgpt-instant":                        "openai/chat-latest",
	"openai/gpt-5.6":                                "openai/gpt-5.6-terra",
	"opus":                                          "anthropic/claude-opus-5",
	"opus-4":                                        "anthropic/claude-opus-4.8",
	"opus-4-6":                                      "anthropic/claude-opus-4.6",
	"opus-4-7":                                      "anthropic/claude-opus-4.7",
	"opus-4-8":                                      "anthropic/claude-opus-4.8",
	"opus-4.6":                                      "anthropic/claude-opus-4.6",
	"opus-4.7":                                      "anthropic/claude-opus-4.7",
	"opus-4.8":                                      "anthropic/claude-opus-4.8",
	"opus-5":                                        "anthropic/claude-opus-5",
	"opus-5-0":                                      "anthropic/claude-opus-5",
	"opus-5.0":                                      "anthropic/claude-opus-5",
	"poolside/laguna-xs-2.1":                        "free/laguna-xs-2.1",
	"qwen-3.7-flash":                                "qwen/qwen3.7-flash",
	"qwen-3.7-max":                                  "qwen/qwen3.7-max",
	"qwen-3.7-plus":                                 "qwen/qwen3.7-plus",
	"qwen-coder":                                    "free/nemotron-3.5-lightning",
	"qwen-coder-free":                               "free/nemotron-3.5-lightning",
	"qwen-max":                                      "qwen/qwen3.7-max",
	"qwen-thinking":                                 "free/nemotron-3.5-lightning",
	"qwen-vision":                                   "qwen/qwen3.8-flash",
	"qwen/qwen3-coder-480b-a35b-instruct":           "free/qwen3-coder-480b",
	"qwen3-122b":                                    "free/qwen3.5-122b-a10b",
	"qwen3-7-max":                                   "qwen/qwen3.7-max",
	"qwen3-8-flash":                                 "qwen/qwen3.8-flash",
	"qwen3-next":                                    "free/nemotron-3.5-lightning",
	"qwen3-next-80b":                                "free/qwen3-next-80b-a3b-instruct",
	"qwen3.5-122b":                                  "free/qwen3.5-122b-a10b",
	"qwen3.7-flash":                                 "qwen/qwen3.7-flash",
	"qwen3.7-max":                                   "qwen/qwen3.7-max",
	"qwen3.7-plus":                                  "qwen/qwen3.7-plus",
	"qwen3.8-flash":                                 "qwen/qwen3.8-flash",
	"reasoner":                                      "deepseek/deepseek-reasoner",
	"router":                                        "auto",
	"seed-oss":                                      "free/seed-oss-36b",
	"seed-oss-36b":                                  "free/seed-oss-36b",
	"sol-pro":                                       "openai/gpt-5.6-sol-pro",
	"sonnet":                                        "anthropic/claude-sonnet-5",
	"sonnet-4":                                      "anthropic/claude-sonnet-4.6",
	"sonnet-4-5":                                    "anthropic/claude-sonnet-4.5",
	"sonnet-4-6":                                    "anthropic/claude-sonnet-4.6",
	"sonnet-4.5":                                    "anthropic/claude-sonnet-4.5",
	"sonnet-4.6":                                    "anthropic/claude-sonnet-4.6",
	"sonnet-5":                                      "anthropic/claude-sonnet-5",
	"sonnet-5-0":                                    "anthropic/claude-sonnet-5",
	"sonnet-5.0":                                    "anthropic/claude-sonnet-5",
	"step-3.7-flash":                                "free/step-3.7-flash",
	"step-flash":                                    "free/step-3.7-flash",
	"tencent":                                       "tencent/hy3",
	"terra-pro":                                     "openai/gpt-5.6-terra-pro",
	"ultra-550b":                                    "free/nemotron-3-ultra-550b",
	"v4-flash":                                      "free/nemotron-3.5-lightning",
	"v4-flash-vision":                               "deepseek/deepseek-v4-flash-vision-exp",
	"v4-pro":                                        "free/nemotron-3.5-lightning",
	"vision-free":                                   "free/nemotron-3-nano-omni-30b-a3b-reasoning",
	"xai/grok-3-fast":                               "xai/grok-4-fast-reasoning",
	"xai/grok-code-fast-1":                          "deepseek/deepseek-chat",
	"xiaomi":                                        "xiaomi/mimo-v2.5-pro",
}

// Models is the full catalog of supported models.
var Models = []ModelDef{
	// Catalog metadata: ClawRouter 05de1e0 (v0.12.278 source).
	// Retired free IDs keep compatibility metadata and redirect to live successors.
	// Free models deliberately do not claim working vision or structured tools.

	// Routing profiles
	{ID: "auto", Name: "Auto (Smart Router - Balanced)", InputPrice: 0, OutputPrice: 0, ContextWindow: 1050000, MaxOutput: 128000},
	{ID: "free", Name: "Free - Nemotron 3.5 Lightning", InputPrice: 0, OutputPrice: 0, ContextWindow: 1000000, MaxOutput: 16384, Reasoning: true},
	{ID: "eco", Name: "Eco (Smart Router - Cost Optimized)", InputPrice: 0, OutputPrice: 0, ContextWindow: 1050000, MaxOutput: 128000},
	{ID: "premium", Name: "Premium (Smart Router - Best Quality)", InputPrice: 0, OutputPrice: 0, ContextWindow: 2000000, MaxOutput: 200000},

	// openai
	{ID: "openai/gpt-5.2", Name: "GPT-5.2", Version: "5.2", InputPrice: 1.75, OutputPrice: 14.0, ContextWindow: 400000, MaxOutput: 128000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-5-mini", Name: "GPT-5 Mini", Version: "5.0", InputPrice: 0.25, OutputPrice: 2.0, ContextWindow: 200000, MaxOutput: 128000, ToolCalling: true},
	{ID: "openai/gpt-5-nano", Name: "GPT-5 Nano", Version: "5.0", InputPrice: 0.05, OutputPrice: 0.4, ContextWindow: 128000, MaxOutput: 128000, ToolCalling: true, Deprecated: true, FallbackModel: "openai/gpt-5.4-nano"},
	{ID: "openai/gpt-5.2-pro", Name: "GPT-5.2 Pro", Version: "5.2", InputPrice: 21.0, OutputPrice: 168.0, ContextWindow: 400000, MaxOutput: 128000, Reasoning: true, Vision: true, ToolCalling: true},
	{ID: "openai/gpt-5.6-sol", Name: "GPT-5.6 Sol", Version: "5.6", InputPrice: 4.0, OutputPrice: 20.0, ContextWindow: 1050000, MaxOutput: 128000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-5.6-terra", Name: "GPT-5.6 Terra", Version: "5.6", InputPrice: 2.0, OutputPrice: 12.0, ContextWindow: 1050000, MaxOutput: 128000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-5.6-luna", Name: "GPT-5.6 Luna", Version: "5.6", InputPrice: 0.2, OutputPrice: 1.2, ContextWindow: 1050000, MaxOutput: 128000, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-5.6-sol-pro", Name: "GPT-5.6 Sol Pro", Version: "5.6", InputPrice: 4.0, OutputPrice: 20.0, ContextWindow: 1050000, MaxOutput: 128000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-5.6-terra-pro", Name: "GPT-5.6 Terra Pro", Version: "5.6", InputPrice: 2.0, OutputPrice: 12.0, ContextWindow: 1050000, MaxOutput: 128000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-5.6-luna-pro", Name: "GPT-5.6 Luna Pro", Version: "5.6", InputPrice: 0.2, OutputPrice: 1.2, ContextWindow: 1050000, MaxOutput: 128000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-5.5", Name: "GPT-5.5", Version: "5.5", InputPrice: 5.0, OutputPrice: 30.0, ContextWindow: 1050000, MaxOutput: 128000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-5.5-pro", Name: "GPT-5.5 Pro", Version: "5.5", InputPrice: 30.0, OutputPrice: 180.0, ContextWindow: 1050000, MaxOutput: 128000, Reasoning: true, Vision: true, ToolCalling: true},
	{ID: "openai/chat-latest", Name: "ChatGPT Instant (GPT-5.5)", Version: "5.5", InputPrice: 5.0, OutputPrice: 30.0, ContextWindow: 128000, MaxOutput: 128000, Vision: true, ToolCalling: true},
	{ID: "openai/gpt-5.4", Name: "GPT-5.4", Version: "5.4", InputPrice: 2.5, OutputPrice: 15.0, ContextWindow: 1050000, MaxOutput: 128000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-5.4-mini", Name: "GPT-5.4 Mini", Version: "5.4", InputPrice: 0.75, OutputPrice: 4.5, ContextWindow: 400000, MaxOutput: 128000, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-5.4-pro", Name: "GPT-5.4 Pro", Version: "5.4", InputPrice: 30.0, OutputPrice: 180.0, ContextWindow: 1050000, MaxOutput: 128000, Reasoning: true, Vision: true, ToolCalling: true},
	{ID: "openai/gpt-5.4-nano", Name: "GPT-5.4 Nano", Version: "5.4", InputPrice: 0.2, OutputPrice: 1.25, ContextWindow: 1050000, MaxOutput: 128000, ToolCalling: true},
	{ID: "openai/gpt-5.3", Name: "GPT-5.3", Version: "5.3", InputPrice: 1.75, OutputPrice: 14.0, ContextWindow: 128000, MaxOutput: 128000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-5.3-codex", Name: "GPT-5.3 Codex", Version: "5.3", InputPrice: 1.75, OutputPrice: 14.0, ContextWindow: 400000, MaxOutput: 128000, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-4.1", Name: "GPT-4.1", Version: "4.1", InputPrice: 2.0, OutputPrice: 8.0, ContextWindow: 128000, MaxOutput: 32768, Vision: true, ToolCalling: true},
	{ID: "openai/gpt-4.1-mini", Name: "GPT-4.1 Mini", Version: "4.1", InputPrice: 0.4, OutputPrice: 1.6, ContextWindow: 128000, MaxOutput: 32768, ToolCalling: true},
	{ID: "openai/gpt-4.1-nano", Name: "GPT-4.1 Nano", Version: "4.1", InputPrice: 0.1, OutputPrice: 0.4, ContextWindow: 128000, MaxOutput: 32768, ToolCalling: true},
	{ID: "openai/gpt-4o", Name: "GPT-4o", Version: "4o", InputPrice: 2.5, OutputPrice: 10.0, ContextWindow: 128000, MaxOutput: 16384, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "openai/gpt-4o-mini", Name: "GPT-4o Mini", Version: "4o-mini", InputPrice: 0.15, OutputPrice: 0.6, ContextWindow: 128000, MaxOutput: 16384, ToolCalling: true},
	{ID: "openai/o1", Name: "o1", Version: "1", InputPrice: 15.0, OutputPrice: 60.0, ContextWindow: 200000, MaxOutput: 100000, Reasoning: true, ToolCalling: true},
	{ID: "openai/o1-mini", Name: "o1-mini", Version: "1-mini", InputPrice: 1.1, OutputPrice: 4.4, ContextWindow: 128000, MaxOutput: 65536, Reasoning: true, ToolCalling: true, Deprecated: true, FallbackModel: "openai/o4-mini"},
	{ID: "openai/o3", Name: "o3", Version: "3", InputPrice: 2.0, OutputPrice: 8.0, ContextWindow: 200000, MaxOutput: 100000, Reasoning: true, ToolCalling: true},
	{ID: "openai/o3-mini", Name: "o3-mini", Version: "3-mini", InputPrice: 1.1, OutputPrice: 4.4, ContextWindow: 128000, MaxOutput: 100000, Reasoning: true, ToolCalling: true},
	{ID: "openai/o4-mini", Name: "o4-mini", Version: "4-mini", InputPrice: 1.1, OutputPrice: 4.4, ContextWindow: 128000, MaxOutput: 100000, Reasoning: true, ToolCalling: true},

	// anthropic
	{ID: "anthropic/claude-haiku-4.5", Name: "Claude Haiku 4.5", Version: "4.5", InputPrice: 1.0, OutputPrice: 5.0, ContextWindow: 200000, MaxOutput: 64000, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "anthropic/claude-sonnet-4.5", Name: "Claude Sonnet 4.5", Version: "4.5", InputPrice: 3.0, OutputPrice: 15.0, ContextWindow: 200000, MaxOutput: 64000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "anthropic/claude-sonnet-4.6", Name: "Claude Sonnet 4.6", Version: "4.6", InputPrice: 3.0, OutputPrice: 15.0, ContextWindow: 1000000, MaxOutput: 128000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "anthropic/claude-sonnet-5", Name: "Claude Sonnet 5", Version: "5", InputPrice: 3.0, OutputPrice: 15.0, ContextWindow: 1000000, MaxOutput: 128000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "anthropic/claude-opus-4.5", Name: "Claude Opus 4.5", Version: "4.5", InputPrice: 5.0, OutputPrice: 25.0, ContextWindow: 200000, MaxOutput: 64000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "anthropic/claude-opus-4.6", Name: "Claude Opus 4.6", Version: "4.6", InputPrice: 5.0, OutputPrice: 25.0, ContextWindow: 1000000, MaxOutput: 128000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "anthropic/claude-fable-5", Name: "Claude Fable 5", Version: "5", InputPrice: 10.0, OutputPrice: 50.0, ContextWindow: 1000000, MaxOutput: 128000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "anthropic/claude-opus-4.7", Name: "Claude Opus 4.7", Version: "4.7", InputPrice: 5.0, OutputPrice: 25.0, ContextWindow: 1000000, MaxOutput: 128000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "anthropic/claude-opus-4.8", Name: "Claude Opus 4.8", Version: "4.8", InputPrice: 5.0, OutputPrice: 25.0, ContextWindow: 1000000, MaxOutput: 128000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "anthropic/claude-opus-5", Name: "Claude Opus 5", Version: "5", InputPrice: 5.0, OutputPrice: 25.0, ContextWindow: 1000000, MaxOutput: 128000, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},

	// google
	{ID: "google/gemini-3.1-pro", Name: "Gemini 3.1 Pro", Version: "3.1", InputPrice: 2.0, OutputPrice: 12.0, ContextWindow: 1048576, MaxOutput: 65536, Reasoning: true, Vision: true, ToolCalling: true},
	{ID: "google/gemini-3-pro-preview", Name: "Gemini 3 Pro Preview", Version: "3.0", InputPrice: 2.0, OutputPrice: 12.0, ContextWindow: 1048576, MaxOutput: 65536, Reasoning: true, Vision: true, ToolCalling: true, Deprecated: true, FallbackModel: "google/gemini-3.1-pro"},
	// Promotional $0.75/$3.75 until 2027-01-01; upstream list rate then returns to $1.50/$7.50.
	{ID: "google/gemini-3.8-flash", Name: "Gemini 3.8 Flash", Version: "3.8", InputPrice: 0.75, OutputPrice: 3.75, ContextWindow: 1048576, MaxOutput: 65536, Reasoning: true, Vision: true, ToolCalling: true},
	// Promotional $0.75/$3.75 until 2027-01-01; upstream list rate then returns to $1.50/$7.50.
	{ID: "google/gemini-3.6-flash", Name: "Gemini 3.6 Flash", Version: "3.6", InputPrice: 0.75, OutputPrice: 3.75, ContextWindow: 1048576, MaxOutput: 65536, Reasoning: true, Vision: true, ToolCalling: true},
	{ID: "google/gemini-3.5-flash", Name: "Gemini 3.5 Flash", Version: "3.5", InputPrice: 1.5, OutputPrice: 9.0, ContextWindow: 1048576, MaxOutput: 65536, Reasoning: true, Vision: true, ToolCalling: true},
	{ID: "google/gemini-3.5-flash-lite", Name: "Gemini 3.5 Flash Lite", Version: "3.5", InputPrice: 0.3, OutputPrice: 2.5, ContextWindow: 1048576, MaxOutput: 65536, Reasoning: true, ToolCalling: true},
	{ID: "google/gemini-3-flash-preview", Name: "Gemini 3 Flash Preview", Version: "3.0", InputPrice: 0.5, OutputPrice: 3.0, ContextWindow: 1048576, MaxOutput: 65536, Reasoning: true, Vision: true},
	{ID: "google/gemini-2.5-pro", Name: "Gemini 2.5 Pro", Version: "2.5", InputPrice: 1.25, OutputPrice: 10.0, ContextWindow: 1048576, MaxOutput: 65536, Reasoning: true, Vision: true, ToolCalling: true},
	{ID: "google/gemini-2.5-flash", Name: "Gemini 2.5 Flash", Version: "2.5", InputPrice: 0.3, OutputPrice: 2.5, ContextWindow: 1048576, MaxOutput: 65536, Vision: true, ToolCalling: true},
	{ID: "google/gemini-2.5-flash-lite", Name: "Gemini 2.5 Flash Lite", Version: "2.5", InputPrice: 0.1, OutputPrice: 0.4, ContextWindow: 1048576, MaxOutput: 65536, Vision: true, ToolCalling: true},
	{ID: "google/gemini-3.1-flash-lite", Name: "Gemini 3.1 Flash Lite", Version: "3.1", InputPrice: 0.25, OutputPrice: 1.5, ContextWindow: 1048576, MaxOutput: 65536, ToolCalling: true},

	// deepseek
	{ID: "deepseek/deepseek-chat", Name: "DeepSeek V4 Flash Chat", Version: "4-flash", InputPrice: 0.14, OutputPrice: 0.28, ContextWindow: 1048576, MaxOutput: 65536, ToolCalling: true},
	{ID: "deepseek/deepseek-reasoner", Name: "DeepSeek V4 Flash Reasoner", Version: "4-flash", InputPrice: 0.14, OutputPrice: 0.28, ContextWindow: 1048576, MaxOutput: 65536, Reasoning: true, ToolCalling: true},
	{ID: "deepseek/deepseek-v4-pro", Name: "DeepSeek V4 Pro", Version: "4-pro", InputPrice: 1.32, OutputPrice: 3.96, ContextWindow: 1048576, MaxOutput: 65536, Reasoning: true, Agentic: true, ToolCalling: true},

	// moonshot
	{ID: "moonshot/kimi-k3", Name: "Kimi K3", Version: "k3", InputPrice: 3.0, OutputPrice: 15.0, ContextWindow: 1048576, MaxOutput: 65536, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "moonshot/kimi-k2.7", Name: "Kimi K2.7", Version: "k2.7", InputPrice: 0.95, OutputPrice: 4.0, ContextWindow: 262144, MaxOutput: 65536, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},

	// qwen
	{ID: "qwen/qwen3.7-max", Name: "Qwen3.7 Max", Version: "3.7-max", InputPrice: 1.475, OutputPrice: 4.425, ContextWindow: 1000000, MaxOutput: 65536, Reasoning: true, Agentic: true, ToolCalling: true},
	{ID: "qwen/qwen3.7-plus", Name: "Qwen3.7 Plus", Version: "3.7-plus", InputPrice: 0.32, OutputPrice: 1.28, ContextWindow: 1000000, MaxOutput: 131072, Reasoning: true, Agentic: true, ToolCalling: true},
	{ID: "qwen/qwen3.7-flash", Name: "Qwen3.7 Flash", Version: "3.7-flash", InputPrice: 0.03, OutputPrice: 0.13, ContextWindow: 1000000, MaxOutput: 65536, Reasoning: true, ToolCalling: true},

	// moonshot
	{ID: "moonshot/kimi-k2.6", Name: "Kimi K2.6", Version: "k2.6", InputPrice: 0.95, OutputPrice: 4.0, ContextWindow: 262144, MaxOutput: 65536, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "moonshot/kimi-k2.5", Name: "Kimi K2.5", Version: "k2.5", InputPrice: 0.6, OutputPrice: 3.0, ContextWindow: 262144, MaxOutput: 65536, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},

	// nvidia
	{ID: "nvidia/kimi-k2.5", Name: "Kimi K2.5 (NVIDIA, retired)", Version: "k2.5", InputPrice: 0.6, OutputPrice: 3.0, ContextWindow: 262144, MaxOutput: 16384, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true, Deprecated: true, FallbackModel: "moonshot/kimi-k2.5"},

	// xai
	{ID: "xai/grok-3", Name: "Grok 3", Version: "3", InputPrice: 3.0, OutputPrice: 15.0, ContextWindow: 131072, MaxOutput: 16384, Reasoning: true, ToolCalling: true},
	{ID: "xai/grok-3-mini", Name: "Grok 3 Mini", Version: "3-mini", InputPrice: 0.3, OutputPrice: 0.5, ContextWindow: 131072, MaxOutput: 16384, ToolCalling: true},
	{ID: "xai/grok-4-fast-reasoning", Name: "Grok 4 Fast Reasoning", Version: "4", InputPrice: 0.2, OutputPrice: 0.5, ContextWindow: 2000000, MaxOutput: 16384, Reasoning: true, ToolCalling: true},
	{ID: "xai/grok-4-fast-non-reasoning", Name: "Grok 4 Fast", Version: "4", InputPrice: 0.2, OutputPrice: 0.5, ContextWindow: 2000000, MaxOutput: 16384, ToolCalling: true},
	{ID: "xai/grok-4-1-fast-reasoning", Name: "Grok 4.1 Fast Reasoning", Version: "4.1", InputPrice: 0.2, OutputPrice: 0.5, ContextWindow: 2000000, MaxOutput: 16384, Reasoning: true, ToolCalling: true},
	{ID: "xai/grok-4-1-fast-non-reasoning", Name: "Grok 4.1 Fast", Version: "4.1", InputPrice: 0.2, OutputPrice: 0.5, ContextWindow: 2000000, MaxOutput: 16384, ToolCalling: true},
	{ID: "xai/grok-4-0709", Name: "Grok 4 (0709)", Version: "4-0709", InputPrice: 3.0, OutputPrice: 15.0, ContextWindow: 256000, MaxOutput: 16384, Reasoning: true, ToolCalling: true},
	{ID: "xai/grok-2-vision", Name: "Grok 2 Vision", Version: "2", InputPrice: 2.0, OutputPrice: 10.0, ContextWindow: 32768, MaxOutput: 16384, Vision: true, ToolCalling: true},
	{ID: "xai/grok-4.20-reasoning", Name: "Grok 4.20 Reasoning", Version: "4.20", InputPrice: 2.0, OutputPrice: 6.0, ContextWindow: 2000000, MaxOutput: 16384, Reasoning: true, ToolCalling: true},
	{ID: "xai/grok-4.20-non-reasoning", Name: "Grok 4.20", Version: "4.20", InputPrice: 2.0, OutputPrice: 6.0, ContextWindow: 2000000, MaxOutput: 16384, ToolCalling: true},
	{ID: "xai/grok-4.20-multi-agent", Name: "Grok 4.20 Multi-Agent", Version: "4.20", InputPrice: 2.0, OutputPrice: 6.0, ContextWindow: 2000000, MaxOutput: 16384, Reasoning: true, ToolCalling: true},
	{ID: "xai/grok-4.5", Name: "Grok 4.5", Version: "4.5", InputPrice: 2.0, OutputPrice: 6.0, ContextWindow: 500000, MaxOutput: 16384, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "xai/grok-4.3", Name: "Grok 4.3", Version: "4.3", InputPrice: 1.25, OutputPrice: 2.5, ContextWindow: 1000000, MaxOutput: 16384, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "xai/grok-build-0.1", Name: "Grok Build 0.1", Version: "0.1", InputPrice: 1.0, OutputPrice: 2.0, ContextWindow: 256000, MaxOutput: 16384, Agentic: true, ToolCalling: true},

	// tencent
	{ID: "tencent/hy3", Name: "Tencent Hy3", Version: "hy3", InputPrice: 0.132, OutputPrice: 0.528, ContextWindow: 262144, MaxOutput: 128000, Reasoning: true, ToolCalling: true},

	// xiaomi
	{ID: "xiaomi/mimo-v2.5-pro", Name: "Xiaomi MiMo-V2.5 Pro", Version: "v2.5-pro", InputPrice: 0.435, OutputPrice: 0.87, ContextWindow: 1048576, MaxOutput: 131072, Reasoning: true, ToolCalling: true},

	// minimax
	{ID: "minimax/minimax-m3", Name: "MiniMax M3", Version: "m3", InputPrice: 0.3, OutputPrice: 1.2, ContextWindow: 1048576, MaxOutput: 65536, Reasoning: true, Vision: true, Agentic: true, ToolCalling: true},
	{ID: "minimax/minimax-m2.7", Name: "MiniMax M2.7", Version: "m2.7", InputPrice: 0.3, OutputPrice: 1.2, ContextWindow: 204800, MaxOutput: 16384, Reasoning: true, Agentic: true, ToolCalling: true},
	{ID: "minimax/minimax-m2.5", Name: "MiniMax M2.5", Version: "m2.5", InputPrice: 0.3, OutputPrice: 1.2, ContextWindow: 204800, MaxOutput: 16384, Reasoning: true, Agentic: true, ToolCalling: true, Deprecated: true, FallbackModel: "minimax/minimax-m2.7"},

	// free
	{ID: "free/gpt-oss-120b", Name: "[Free] GPT-OSS 120B", Version: "120b", InputPrice: 0, OutputPrice: 0, ContextWindow: 128000, MaxOutput: 16384},
	{ID: "free/gpt-oss-20b", Name: "[Free] GPT-OSS 20B", Version: "20b", InputPrice: 0, OutputPrice: 0, ContextWindow: 128000, MaxOutput: 16384},
	{ID: "free/deepseek-v4-flash", Name: "[Free] DeepSeek V4 Flash", Version: "v4-flash", InputPrice: 0, OutputPrice: 0, ContextWindow: 1000000, MaxOutput: 16384, Reasoning: true, Deprecated: true, FallbackModel: "free/nemotron-3.5-lightning"},
	{ID: "free/qwen3-coder-480b", Name: "[Free] Qwen3 Coder 480B", Version: "480b", InputPrice: 0, OutputPrice: 0, ContextWindow: 131072, MaxOutput: 16384, Deprecated: true, FallbackModel: "free/nemotron-3.5-lightning"},
	{ID: "free/glm-4.7", Name: "[Free] GLM-4.7", Version: "4.7", InputPrice: 0, OutputPrice: 0, ContextWindow: 131072, MaxOutput: 16384, Reasoning: true, Deprecated: true, FallbackModel: "free/nemotron-3.5-lightning"},
	{ID: "free/llama-4-maverick", Name: "[Free] Llama 4 Maverick", Version: "4-maverick", InputPrice: 0, OutputPrice: 0, ContextWindow: 131072, MaxOutput: 16384, Reasoning: true, Deprecated: true, FallbackModel: "free/nemotron-3.5-lightning"},
	{ID: "free/nemotron-3-nano-omni-30b-a3b-reasoning", Name: "[Free] Nemotron 3 Nano Omni", Version: "30b-a3b-omni-reasoning", InputPrice: 0, OutputPrice: 0, ContextWindow: 256000, MaxOutput: 16384, Reasoning: true},
	{ID: "free/mistral-large-3-675b", Name: "[Free] Mistral Large 3 675B", Version: "3-675b", InputPrice: 0, OutputPrice: 0, ContextWindow: 131072, MaxOutput: 16384, Reasoning: true, Deprecated: true, FallbackModel: "free/nemotron-3.5-lightning"},
	{ID: "free/qwen3.5-122b-a10b", Name: "[Free] Qwen3.5 122B", Version: "3.5-122b", InputPrice: 0, OutputPrice: 0, ContextWindow: 131072, MaxOutput: 16384, Reasoning: true, Deprecated: true, FallbackModel: "free/nemotron-3.5-lightning"},
	{ID: "free/qwen3-next-80b-a3b-instruct", Name: "[Free] Qwen3-Next 80B Instruct", Version: "next-80b-a3b", InputPrice: 0, OutputPrice: 0, ContextWindow: 262144, MaxOutput: 16384, Reasoning: true, Deprecated: true, FallbackModel: "free/nemotron-3.5-lightning"},
	{ID: "free/seed-oss-36b", Name: "[Free] Seed-OSS 36B", Version: "oss-36b", InputPrice: 0, OutputPrice: 0, ContextWindow: 131072, MaxOutput: 16384, Deprecated: true, FallbackModel: "free/nemotron-3.5-lightning"},
	{ID: "free/mistral-nemotron", Name: "[Free] Mistral Nemotron", Version: "nemotron", InputPrice: 0, OutputPrice: 0, ContextWindow: 131072, MaxOutput: 16384, Deprecated: true, FallbackModel: "free/nemotron-3.5-lightning"},
	{ID: "free/step-3.7-flash", Name: "[Free] StepFun Step 3.7 Flash", Version: "3.7-flash", InputPrice: 0, OutputPrice: 0, ContextWindow: 131072, MaxOutput: 16384, Reasoning: true, Deprecated: true, FallbackModel: "free/nemotron-3.5-lightning"},
	{ID: "free/nemotron-nano-9b-v2", Name: "[Free] Nemotron Nano 9B v2", Version: "nano-9b-v2", InputPrice: 0, OutputPrice: 0, ContextWindow: 131072, MaxOutput: 16384, Reasoning: true, Deprecated: true, FallbackModel: "free/nemotron-3-nano-30b"},
	{ID: "free/nemotron-nano-12b-v2-vl", Name: "[Free] Nemotron Nano 12B v2 VL", Version: "nano-12b-v2-vl", InputPrice: 0, OutputPrice: 0, ContextWindow: 131072, MaxOutput: 16384, Reasoning: true, Deprecated: true, FallbackModel: "free/nemotron-3-nano-omni-30b-a3b-reasoning"},
	{ID: "free/nemotron-3.5-lightning", Name: "[Free] Nemotron 3.5 Lightning", Version: "3.5-lightning", InputPrice: 0, OutputPrice: 0, ContextWindow: 1000000, MaxOutput: 16384, Reasoning: true},
	{ID: "free/nemotron-3-nano-30b", Name: "[Free] Nemotron 3 Nano 30B", Version: "3-nano-30b", InputPrice: 0, OutputPrice: 0, ContextWindow: 131072, MaxOutput: 16384, Reasoning: true},
	{ID: "free/nemotron-3-ultra-550b", Name: "[Free] Nemotron 3 Ultra 550B", Version: "3-ultra-550b", InputPrice: 0, OutputPrice: 0, ContextWindow: 1000000, MaxOutput: 16384, Reasoning: true},
	{ID: "free/llama-3.2-11b-vision", Name: "[Free] Llama 3.2 11B Vision", Version: "3.2-11b-vision", InputPrice: 0, OutputPrice: 0, ContextWindow: 128000, MaxOutput: 16384},
	{ID: "free/north-mini-code", Name: "[Free] Cohere North Mini Code", Version: "north-mini-code", InputPrice: 0, OutputPrice: 0, ContextWindow: 256000, MaxOutput: 16384, Reasoning: true},
	{ID: "free/laguna-xs-2.1", Name: "[Free] Poolside Laguna XS 2.1", Version: "xs-2.1", InputPrice: 0, OutputPrice: 0, ContextWindow: 131072, MaxOutput: 16384},

	// qwen
	{ID: "qwen/qwen3.8-flash", Name: "Qwen3.8 Flash", Version: "3.8-flash", InputPrice: 0.15, OutputPrice: 0.47, ContextWindow: 1000000, MaxOutput: 131072, Reasoning: true, Vision: true, ToolCalling: true},

	// deepseek
	{ID: "deepseek/deepseek-v4-flash-vision-exp", Name: "DeepSeek V4 Flash Vision", Version: "v4-flash-vision-exp", InputPrice: 0.44, OutputPrice: 1.32, ContextWindow: 1048576, MaxOutput: 65536, Reasoning: true, Vision: true, ToolCalling: true},

	// xiaomi
	{ID: "xiaomi/mimo-v2.5", Name: "Xiaomi MiMo V2.5", Version: "2.5", InputPrice: 0.14, OutputPrice: 0.28, ContextWindow: 1048576, MaxOutput: 131072, Reasoning: true, Vision: true, ToolCalling: true},

	// zai
	{ID: "zai/glm-5.3", Name: "GLM-5.3", Version: "5.3", InputPrice: 1.4, OutputPrice: 4.4, ContextWindow: 1000000, MaxOutput: 131072, Reasoning: true, ToolCalling: true},
	{ID: "zai/glm-5.3-flash", Name: "GLM-5.3 Flash", Version: "5.3-flash", InputPrice: 0.15, OutputPrice: 0.5, ContextWindow: 1000000, MaxOutput: 131072, Reasoning: true, Vision: true, ToolCalling: true},
	{ID: "zai/glm-5.2", Name: "GLM-5.2", Version: "5.2", InputPrice: 1.4, OutputPrice: 4.4, ContextWindow: 1000000, MaxOutput: 131072, Reasoning: true, ToolCalling: true},
	{ID: "zai/glm-5.1", Name: "GLM-5.1", Version: "5.1", InputPrice: 1.4, OutputPrice: 4.4, ContextWindow: 200000, MaxOutput: 128000, Reasoning: true, ToolCalling: true, Promo: &PromoDef{FlatPrice: 0.001, StartDate: "2026-04-01", EndDate: "2026-06-05"}},
	{ID: "zai/glm-5", Name: "GLM-5", Version: "5", InputPrice: 1.0, OutputPrice: 3.2, ContextWindow: 200000, MaxOutput: 128000, Reasoning: true, ToolCalling: true},
	{ID: "zai/glm-5-turbo", Name: "GLM-5 Turbo", Version: "5-turbo", InputPrice: 1.2, OutputPrice: 4.0, ContextWindow: 200000, MaxOutput: 128000, Reasoning: true, ToolCalling: true},

	// openai
	{ID: "openai/chatgpt-instant", Name: "ChatGPT Instant (GPT-5.5)", Version: "5.5", InputPrice: 5.0, OutputPrice: 30.0, ContextWindow: 128000, MaxOutput: 128000, Vision: true, ToolCalling: true, Deprecated: true, FallbackModel: "openai/chat-latest"},

	// free
	{ID: "free/qwen3-next-80b-a3b-thinking", Name: "qwen3-next-80b-a3b-thinking (retired)", ContextWindow: 131072, MaxOutput: 16384, InputPrice: 0, OutputPrice: 0, Deprecated: true, FallbackModel: "free/nemotron-3.5-lightning"},
	{ID: "free/mistral-small-4-119b", Name: "mistral-small-4-119b (retired)", ContextWindow: 131072, MaxOutput: 16384, InputPrice: 0, OutputPrice: 0, Deprecated: true, FallbackModel: "free/nemotron-3.5-lightning"},
	{ID: "free/deepseek-v3.2", Name: "deepseek-v3.2 (retired)", ContextWindow: 131072, MaxOutput: 16384, InputPrice: 0, OutputPrice: 0, Deprecated: true, FallbackModel: "free/nemotron-3.5-lightning"},
	{ID: "free/deepseek-v4-pro", Name: "deepseek-v4-pro (retired)", ContextWindow: 131072, MaxOutput: 16384, InputPrice: 0, OutputPrice: 0, Deprecated: true, FallbackModel: "free/nemotron-3.5-lightning"},
	{ID: "free/nemotron-ultra-253b", Name: "nemotron-ultra-253b (retired)", ContextWindow: 131072, MaxOutput: 16384, InputPrice: 0, OutputPrice: 0, Deprecated: true, FallbackModel: "free/nemotron-3.5-lightning"},
	{ID: "free/nemotron-super-49b", Name: "nemotron-super-49b (retired)", ContextWindow: 131072, MaxOutput: 16384, InputPrice: 0, OutputPrice: 0, Deprecated: true, FallbackModel: "free/nemotron-3.5-lightning"},
	{ID: "free/nemotron-3-super-120b", Name: "nemotron-3-super-120b (retired)", ContextWindow: 131072, MaxOutput: 16384, InputPrice: 0, OutputPrice: 0, Deprecated: true, FallbackModel: "free/nemotron-3.5-lightning"},
	{ID: "free/devstral-2-123b", Name: "devstral-2-123b (retired)", ContextWindow: 131072, MaxOutput: 16384, InputPrice: 0, OutputPrice: 0, Deprecated: true, FallbackModel: "free/nemotron-3.5-lightning"},
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

	// A concrete catalog ID wins over shorthand matching after prefix removal.
	// For example, the DOS o1 shorthand targets o3, but openai/o1 is an exact pin.
	if _, ok := modelIndex[normalized]; ok {
		return normalized
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
