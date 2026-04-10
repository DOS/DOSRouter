# DOSRouter

**High-performance LLM router written in Go.** Routes requests to the optimal model based on prompt complexity, cost targets, and capability requirements.

Go port of [ClawRouter](https://github.com/BlockRunAI/ClawRouter) (TypeScript) - rewritten for lower latency and easier deployment.

> **Production-proven at [DOS.AI](https://dos.ai)** - powering the DOS.AI inference API that serves thousands of LLM requests daily with automatic model selection, cost optimization, and multi-provider failover.

## Features

- **15-dimension weighted scorer** - classifies prompts in ~0.04ms (40us) without any LLM call
- **Tier-based routing** - SIMPLE / MEDIUM / COMPLEX / REASONING with configurable model pools
- **Cost optimization** - automatic savings calculation vs baseline (opus-class) pricing
- **Multi-provider failover** - primary + fallback providers per model
- **Agentic detection** - identifies tool-calling patterns, routes to capable models
- **Session pinning** - maintains model consistency within conversations
- **Streaming proxy** - OpenAI-compatible SSE proxy with cost breakdown injection
- **55+ models** - built-in catalog with pricing, capabilities, and aliases
- **Zero dependencies** for core routing - only stdlib

## Architecture

```
router/          Core routing logic
  types.go       Type definitions (Tier, ScoringResult, RoutingDecision, etc.)
  config.go      Default config with multilingual keywords (9 languages)
  rules.go       15-dimension weighted scorer (<0.04ms per classification)
  selector.go    Tier -> Model selection with cost/savings calculation
  strategy.go    Strategy pattern + promotions + agentic detection
  llm_classifier.go  LLM fallback for ambiguous requests
  router.go      Entry point (Route function)

models/          Model catalog
  models.go      55+ model definitions, aliases, pricing, capabilities

proxy/           OpenAI-compatible proxy
  proxy.go       HTTP server with smart routing, SSE streaming, /debug endpoint

session/         Session management
  session.go     Conversation pinning with TTL and explicit user overrides

cmd/dosrouter/   CLI
  main.go        serve, classify, models commands
```

## 15-Dimension Weighted Scorer

| # | Dimension | Weight | Detection |
|---|-----------|--------|-----------|
| 1 | reasoningMarkers | 0.18 | "prove", "theorem", "step by step" |
| 2 | codePresence | 0.15 | "function", "class", "import", code blocks |
| 3 | multiStepPatterns | 0.12 | regex: first.*then, step \d, \d\.\s |
| 4 | technicalTerms | 0.10 | "algorithm", "kubernetes", "database" |
| 5 | tokenCount | 0.08 | <50 tokens=-1.0, >500=+1.0 |
| 6 | creativeMarkers | 0.05 | "story", "poem", "brainstorm" |
| 7 | questionComplexity | 0.05 | >3 question marks |
| 8 | agenticTask | 0.04 | "edit", "deploy", "debug", "step 1" |
| 9 | constraintCount | 0.04 | "at most", "O(n)", "limit" |
| 10 | imperativeVerbs | 0.03 | "build", "create", "implement" |
| 11 | outputFormat | 0.03 | "json", "yaml", "table" |
| 12 | simpleIndicators | 0.02 | "what is", "hello" (negative!) |
| 13 | referenceComplexity | 0.02 | "the code above", "the docs" |
| 14 | domainSpecificity | 0.02 | "quantum", "FPGA", "genomics" |
| 15 | negationComplexity | 0.01 | "don't", "avoid", "never" |

**Tier boundaries**: SIMPLE < 0.0 < MEDIUM < 0.3 < COMPLEX < 0.5 < REASONING

**Confidence**: Sigmoid calibration `1/(1+exp(-12*distance))`, threshold 0.7

## Quick Start

### CLI

```bash
# Classify a prompt
go run ./cmd/dosrouter classify "hello world"
go run ./cmd/dosrouter classify "Prove the Riemann hypothesis step by step"

# Start proxy server
go run ./cmd/dosrouter serve --port 8080 --upstream https://api.example.com --api-key sk-xxx

# List models
go run ./cmd/dosrouter models
```

### As a Library

```go
import (
    "github.com/DOS/DOSRouter/router"
    "github.com/DOS/DOSRouter/models"
)

config := router.DefaultRoutingConfig()
pricing := models.BuildPricingMap()

decision, err := router.Route(
    "Write a distributed system with kubernetes",
    "",    // system prompt
    4096,  // max output tokens
    router.RouterOptions{
        Config:         config,
        ModelPricing:   pricing,
        RoutingProfile: "auto", // "auto", "eco", "premium"
    },
)

fmt.Printf("Model: %s, Tier: %s, Savings: %.0f%%\n",
    decision.Model, decision.Tier, decision.Savings*100)
```

## Build & Test

```bash
go build ./...
go test ./router/ -v
go test ./router/ -bench=. -benchmem
```

## Performance

- Classification: ~0.04ms per request (40us)
- Zero external dependencies for routing
- LLM fallback only for ambiguous cases (~20-30%)

## Upstream Sync

This is a Go port of [BlockRunAI/ClawRouter](https://github.com/BlockRunAI/ClawRouter). Routing logic is synced periodically from upstream releases. Payment, plugin lifecycle, and CLI-specific features are excluded. See [UPSTREAM_SYNC.md](UPSTREAM_SYNC.md) for details.

**Current sync**: v0.12.146

## License

MIT
