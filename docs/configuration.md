# Configuration Reference

Complete reference for DOSRouter configuration options.

## Table of Contents

- [Environment Variables](#environment-variables)
- [Wallet Configuration](#wallet-configuration)
- [Proxy Settings](#proxy-settings)
- [Routing Configuration](#routing-configuration)
- [Tier Overrides](#tier-overrides)
- [Scoring Weights](#scoring-weights)
- [Programmatic Usage](#programmatic-usage)

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DOSROUTER_UPSTREAM` | - | Upstream API base URL (required for `serve`) |
| `DOSROUTER_API_KEY` | - | API key for upstream provider |
| `DOSROUTER_PORT` | `8080` | Port for the proxy server |
| `DOSROUTER_WALLET_KEY` | - | EVM private key (hex). Used if no saved wallet exists. |
| `DOSROUTER_CHAIN` | `doschain` | Payment chain: `doschain`, `base`, or `avalanche` |
| `DOSROUTER_RPC_URL` | per-chain default | RPC endpoint for balance queries |
| `DOSROUTER_DISABLED` | `false` | Set to `true` to disable smart routing (passthrough) |

### DOSROUTER_WALLET_KEY

The wallet private key for signing x402 micropayments.

```bash
export DOSROUTER_WALLET_KEY=0x...your_private_key...
```

**Resolution order:**

1. Saved file (`~/.openclaw/DOS/wallet.json`) - checked first
2. `DOSROUTER_WALLET_KEY` environment variable
3. Auto-generate - creates new wallet and saves to file

> **Security Note:** The saved file takes priority to prevent accidentally switching wallets.

---

## Wallet Configuration

DOSRouter supports EVM-compatible payment chains with x402 micropayments.

### Check Wallet

```bash
# View wallet address + balance
dosrouter wallet

# Via HTTP (when proxy is running)
curl http://localhost:8080/health?full=true | jq
```

### Switch Payment Chain

```bash
dosrouter chain doschain    # DOS Chain (default)
dosrouter chain base        # Base (EVM L2)
dosrouter chain avalanche   # Avalanche C-Chain
```

The selected chain is persisted in `~/.openclaw/DOS/wallet.json`.

### Backup & Recovery

```bash
# View mnemonic (shown on first wallet creation)
cat ~/.openclaw/DOS/wallet.json

# Recover from mnemonic
dosrouter wallet recover

# Switch wallet via env var
rm ~/.openclaw/DOS/wallet.json
export DOSROUTER_WALLET_KEY=0x...
dosrouter wallet
```

**Important:** If you lose your wallet key, there is no way to recover it. The wallet is self-custodial.

---

## Proxy Settings

### Starting the Proxy

```bash
# Using CLI flags
dosrouter serve --port 8080 --upstream https://api.example.com --api-key sk-xxx

# Using environment variables
export DOSROUTER_UPSTREAM=https://api.example.com
export DOSROUTER_API_KEY=sk-xxx
dosrouter serve
```

### Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/chat/completions` | POST | Chat completions with smart routing |
| `/v1/images/generations` | POST | Image generation (passthrough) |
| `/v1/models` | GET | List available models |
| `/health` | GET | Health check (`?full=true` for details) |
| `/debug` | POST | Test classification without routing |
| `/cache` | GET | Cache statistics |

---

## Routing Configuration

### Model Routing Profiles

| Profile | Model | Description |
|---------|-------|-------------|
| `auto` | Automatic tier-based selection | Default. Routes to cheapest capable model. |
| `eco` | Economy tier models | Prioritizes low cost |
| `premium` | Premium tier models | Prioritizes quality |

Set the profile in the request `model` field:

```json
{
  "model": "auto",
  "messages": [{ "role": "user", "content": "..." }]
}
```

### Spend Control

DOSRouter includes built-in spending limits:

- **Per-request** - Maximum cost per single request
- **Hourly** - Rolling 1-hour spending cap
- **Daily** - Rolling 24-hour spending cap
- **Session** - Cumulative session total

Configuration is stored in `~/.openclaw/DOS/spending.json`.

---

## Tier Overrides

### Default Tier Mappings

| Tier | Primary Model | Fallback Chain |
|------|---------------|----------------|
| SIMPLE | `google/gemini-2.5-flash` | `deepseek/deepseek-chat` |
| MEDIUM | `deepseek/deepseek-chat` | `openai/gpt-4o-mini`, `google/gemini-2.5-flash` |
| COMPLEX | `anthropic/claude-sonnet-4.6` | `openai/gpt-4o`, `google/gemini-2.5-pro` |
| REASONING | `deepseek/deepseek-reasoner` | `openai/o3-mini`, `anthropic/claude-sonnet-4.6` |

### Fallback Behavior

When a model fails, DOSRouter tries the next model in the fallback chain:

```
Request → gemini-2.5-flash (rate limited)
       → deepseek-chat (timeout)
       → gpt-4o-mini (success)
```

If all models fail, returns a structured error:
```json
{
  "error": {
    "message": "All 3 models failed. Tried: gemini-2.5-flash (rate_limited), deepseek-chat (timeout), gpt-4o-mini (server_error)",
    "type": "all_models_failed",
    "models": 3
  }
}
```

---

## Scoring Weights

The 15-dimension weighted scorer determines query complexity. See [architecture.md](architecture.md) for the full table.

**Tier boundaries**: SIMPLE < 0.0 < MEDIUM < 0.3 < COMPLEX < 0.5 < REASONING

**Confidence threshold**: 0.7 (below this, falls back to LLM classifier)

---

## Programmatic Usage

### As a Go Library

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
        HasTools:       false,
    },
)

fmt.Printf("Model: %s, Tier: %s, Savings: %.0f%%\n",
    decision.Model, decision.Tier, decision.Savings*100)
```

### Testing Classification

```bash
# CLI
dosrouter classify "Prove sqrt(2) is irrational"

# HTTP endpoint (when proxy is running)
curl -X POST http://localhost:8080/debug \
  -H "Content-Type: application/json" \
  -d '{"prompt": "What is 2+2?"}'
```

### Run Tests

```bash
go build ./...
go test ./router/ -v
go test ./router/ -bench=. -benchmem
```
