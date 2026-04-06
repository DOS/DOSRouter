# DOSRouter (Go)

Smart LLM router ported from [ClawRouter](https://github.com/BlockRunAI/ClawRouter) (TypeScript) to Go.

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

## Usage

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

### As a library

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
cd go/
go build ./...
go test ./router/ -v
go test ./router/ -bench=. -benchmem
```

## Performance

- Classification: ~0.04ms per request (40us)
- Zero external dependencies for routing
- LLM fallback only for ambiguous cases (~20-30%)
