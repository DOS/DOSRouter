# Architecture

Technical deep-dive into DOSRouter's internals. DOSRouter is a Go port of [ClawRouter](https://github.com/BlockRunAI/ClawRouter) (TypeScript).

## Table of Contents

- [System Overview](#system-overview)
- [Request Flow](#request-flow)
- [Routing Engine](#routing-engine)
- [Payment System](#payment-system)
- [Optimizations](#optimizations)
- [Source Structure](#source-structure)

---

## System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      Your Application                       │
│                   (OpenAI-compatible client)                │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                DOSRouter Proxy (localhost)                  │
│  ┌─────────────┐  ┌─────────────┐  ┌───────────────────┐   │
│  │   Dedup     │→ │   Router    │→ │   x402 Payment    │   │
│  │   Cache     │  │  (15-dim)   │  │  (EVM chains)     │   │
│  └─────────────┘  └─────────────┘  └───────────────────┘   │
│  ┌─────────────┐  ┌─────────────┐  ┌───────────────────┐   │
│  │  Fallback   │  │   Balance   │  │   SSE Streaming   │   │
│  │   Chain     │  │  Monitor    │  │   + Cost Inject   │   │
│  └─────────────┘  └─────────────┘  └───────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    Upstream LLM API
              (OpenAI / Anthropic / Google)
```

**Key Principles:**

- **100% local routing** - No API calls for model selection (~0.04ms)
- **Client-side only** - Your wallet key never leaves your machine
- **Non-custodial** - Funds stay in your wallet until spent
- **EVM-compatible** - Supports DOS Chain, Base, Avalanche, and any EVM chain

---

## Request Flow

### 1. Request Received

```
POST /v1/chat/completions
{
  "model": "auto",
  "messages": [{ "role": "user", "content": "What is 2+2?" }],
  "stream": true
}
```

### 2. Deduplication Check

```go
// SHA-256 hash of canonical request body
dedupKey := dedup.Hash(body)

// Check completed cache (30s TTL)
if cached, ok := dedup.GetCached(dedupKey); ok {
    return cached // Replay cached response
}
```

### 3. Smart Routing (if model is `auto`, `eco`, or `premium`)

```go
prompt, systemPrompt := extractPrompts(req.Messages)
decision, _ := router.Route(prompt, systemPrompt, maxTokens, router.RouterOptions{
    Config:         config,
    ModelPricing:   pricing,
    RoutingProfile: "auto", // "auto", "eco", "premium"
    HasTools:       len(req.Tools) > 0,
})
// decision = {
//   Model: "google/gemini-2.5-flash",
//   Tier: "SIMPLE",
//   Confidence: 0.92,
//   Savings: 0.99,
//   CostEstimate: 0.0012,
// }
```

### 4. Session Pinning

```go
// Pin model to session for consistency
if sessionID != "" {
    sessions.SetSession(sessionID, model, tier, userExplicit)
}

// User's explicit /model choice survives profile routing
if entry.UserExplicit {
    // Skip auto-routing, keep user's choice
}
```

### 5. Fallback Chain (on provider errors)

```go
// Try all models in the tier's fallback chain
for _, tryModel := range fallbackChain {
    resp, err := retry.Do(ctx, makeReqFor(tryModel))
    if err != nil || resp.StatusCode >= 400 {
        attempts = append(attempts, attemptResult{model: tryModel, reason: err.Error()})
        continue
    }
    return resp // Success
}

// All failed → structured error
// "All 3 models failed. Tried: model-a (timeout), model-b (HTTP 503), ..."
```

### 6. Empty Turn Detection

```go
// Detect degraded responses (empty content, no tool_calls, finish_reason=stop)
if isEmptyTurn(respBody) {
    // Silently retry with next model in fallback chain
}
```

### 7. Response Enhancement

```go
// Inject actual routed model into every SSE chunk
chunk["model"] = resolvedModel

// Inject cost breakdown into usage
usage["cost"] = buildCostBreakdown(model, tier, profile, pricing, inputTok, outputTok)
// { total: 0.0012, input: 0.0004, output: 0.0008, baseline: 0.12, savings_pct: 99 }
```

---

## Routing Engine

### Weighted Scorer

The routing engine uses a 15-dimension weighted scorer that runs entirely locally:

| # | Dimension | Weight | Detection |
|---|-----------|--------|-----------|
| 1 | reasoningMarkers | 0.18 | "prove", "theorem", "step by step" |
| 2 | codePresence | 0.15 | "function", "class", "import", code blocks |
| 3 | multiStepPatterns | 0.12 | regex: first.*then, step \d |
| 4 | technicalTerms | 0.10 | "algorithm", "kubernetes", "database" |
| 5 | tokenCount | 0.08 | <50 tokens=-1.0, >500=+1.0 |
| 6 | creativeMarkers | 0.05 | "story", "poem", "brainstorm" |
| 7 | questionComplexity | 0.05 | >3 question marks |
| 8 | agenticTask | 0.04 | "edit", "deploy", "debug" |
| 9 | constraintCount | 0.04 | "at most", "O(n)", "limit" |
| 10 | imperativeVerbs | 0.03 | "build", "create", "implement" |
| 11 | outputFormat | 0.03 | "json", "yaml", "table" |
| 12 | simpleIndicators | 0.02 | "what is", "hello" (negative!) |
| 13 | referenceComplexity | 0.02 | "the code above" |
| 14 | domainSpecificity | 0.02 | "quantum", "FPGA", "genomics" |
| 15 | negationComplexity | 0.01 | "don't", "avoid", "never" |

**Tier boundaries**: SIMPLE < 0.0 < MEDIUM < 0.3 < COMPLEX < 0.5 < REASONING

**Confidence**: Sigmoid calibration `1/(1+exp(-12*distance))`, threshold 0.7

### Agentic Mode (3-state)

```go
type AgenticMode *bool // nil=auto, true=force, false=disable

// nil (auto): detect from tools + keywords
// true: force agentic routing regardless of prompt
// false: disable agentic detection entirely
```

---

## Payment System

### x402 Protocol

DOSRouter supports the [x402 protocol](https://x402.org) for pay-per-request micropayments. No API key or account required.

```
Client                    Upstream API
  │                           │
  │ 1. Request                │
  │──────────────────────────▶│
  │                           │
  │ 2. 402 Payment Required   │
  │◀──────────────────────────│
  │                           │
  │ 3. Sign EVM payment       │
  │   (USDC on configured     │
  │    chain)                  │
  │                           │
  │ 4. Retry with X-PAYMENT   │
  │──────────────────────────▶│
  │                           │
  │ 5. 200 OK + response      │
  │◀──────────────────────────│
```

### Supported Chains

| Chain | Chain ID | Token | Notes |
|-------|----------|-------|-------|
| DOS Chain | 7979 | DOS | Default chain |
| Base | 8453 | USDC | EVM L2 |
| Avalanche C-Chain | 43114 | USDC | EVM L1 |

### Wallet Management

```bash
# Auto-generated on first run, saved to ~/.dosrouter/wallet.json
dosrouter wallet

# Recover from mnemonic
dosrouter wallet recover

# Switch payment chain
dosrouter chain doschain
dosrouter chain base
dosrouter chain avalanche
```

### Pre-Authorization Cache

Cached payment requirements per endpoint+model key, with 5-minute TTL. Skips the 402 round trip for subsequent requests to the same endpoint.

---

## Optimizations

### 1. Response Cache

LRU cache with TTL, SHA-256 keyed on canonical request body. Configurable max size and TTL.

### 2. Request Deduplication

Prevents duplicate upstream calls when clients retry after timeout. Inflight requests are coalesced.

### 3. Context Compression

Automatically compresses long conversation contexts when they exceed thresholds, reducing token usage.

### 4. Balance Caching

RPC balance queries cached with 30-second TTL. Zero balances never cached (enables immediate detection of newly funded wallets).

---

## Source Structure

```
router/              Core routing logic
  types.go           Type definitions (Tier, ScoringResult, RoutingDecision)
  config.go          Default config with multilingual keywords (9 languages)
  rules.go           15-dimension weighted scorer (<0.04ms)
  selector.go        Tier -> Model selection with cost/savings calculation
  strategy.go        Strategy pattern + promotions + agentic detection
  llm_classifier.go  LLM fallback for ambiguous requests
  router.go          Entry point (Route function)

models/              Model catalog
  models.go          55+ model definitions, aliases, pricing, capabilities

proxy/               OpenAI-compatible proxy
  proxy.go           HTTP server, smart routing, SSE streaming, image gen

session/             Session management
  session.go         Conversation pinning with TTL and explicit user overrides

wallet/              Wallet management
  wallet.go          BIP-39 mnemonic, EVM key derivation, balance queries

payment/             Payment module
  payment.go         x402 protocol, pre-auth cache

cache/               Response cache
  cache.go           TTL + LRU response cache with SHA-256 keys

dedup/               Request deduplication
  dedup.go           Inflight coalescing, completed response cache

compression/         Context compression
  compression.go     Automatic conversation compaction

retry/               Retry logic
  retry.go           Exponential backoff, retryable status codes

spendcontrol/        Spending limits
  spendcontrol.go    Per-request, hourly, daily, session limits

journal/             Session journal
  journal.go         Event extraction and context injection

stats/               Usage statistics
  stats.go           Aggregation, ASCII formatting, log parsing

logger/              Logging
  logger.go          JSONL usage logging to disk

errors/              Error types
  errors.go          Wallet, balance, RPC error types

cmd/dosrouter/       CLI
  main.go            serve, classify, models, stats, wallet, doctor commands
```

### Key Files

| File | Purpose |
|------|---------|
| `proxy/proxy.go` | Core request handling, SSE streaming, fallback chain, cost injection |
| `router/rules.go` | 15-dimension weighted scorer, 9-language keyword sets |
| `router/strategy.go` | Routing strategy, promotions, agentic detection |
| `wallet/wallet.go` | EVM wallet generation, balance monitoring |
| `payment/payment.go` | x402 payment protocol, pre-auth cache |
| `session/session.go` | Session pinning with userExplicit flag |
| `models/models.go` | 55+ model definitions with pricing and capabilities |
