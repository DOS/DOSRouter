# Upstream Sync Tracker

**Upstream**: [BlockRunAI/ClawRouter](https://github.com/BlockRunAI/ClawRouter) (TypeScript)
**This repo**: [DOS/DOSRouter](https://github.com/DOS/DOSRouter) (Go port)
**Last synced**: v0.12.161 (2026-04-22)

## Sync Workflow

1. Check new releases: `gh api repos/BlockRunAI/ClawRouter/releases --jq '.[].tag_name' | head -20`
2. Review changelog for each release since last synced version
3. Classify changes: **port** or **skip** (TS-specific)
4. Port in batches, commit with: `port: <summary> (upstream vX.Y.Z)`
5. Update "Last synced" above after each sync session

## Scope

DOSRouter is a **full Go port** of ClawRouter. The following upstream areas are adapted:

- **Routing**: Full 15-dimension scorer, tier-based model selection, fallback chains
- **Payment**: x402 protocol ported for EVM chains (DOS Chain, Base, Avalanche)
- **Wallet**: BIP-39 mnemonic, EVM key derivation, balance queries
- **CLI**: serve, classify, models, stats, logs, cache, report, wallet, chain, doctor
- **Image gen**: `/v1/images/generations` passthrough endpoint
- **Docs**: All documentation updated for DOSRouter standalone

These upstream areas are excluded (TS/npm-specific):
- OpenClaw plugin lifecycle (register, reload, baseUrl)
- Solana wallet/payment (EVM-only in DOSRouter)
- Node.js/npm-specific (prettier, package.json, CI)

## Sync Log

### 2026-04-07 - Initial port (v0.12.106)
- Ported: Full 1:1 port of all routing modules
- Scope: scorer, strategy, session, proxy, models, tiers, config

### 2026-04-11 - Sync to v0.12.146

| Release | Status | Summary | Notes |
|---------|--------|---------|-------|
| v0.12.146 | DONE | usage.cost breakdown in responses | Actual cost, baseline, savings injected into streaming + non-streaming |
| v0.12.145 | DONE | eco/premium null tier fallback | Fall back to default tiers when eco/premiumTiers is nil |
| v0.12.144 | DONE | Session pinning `userExplicit` flag | User's /model choice wins over profile routing |
| v0.12.143 | SKIP | Prettier formatting | TS-only |
| v0.12.142 | DONE | Deferred proxy startup for plugin config | Register() method with 250ms defer timer |
| v0.12.141 | DONE | Agentic mode 3-state semantics | `nil`=auto, `true`=force, `false`=disable |
| v0.12.140 | DONE | Solana doctor fix | Added Solana chain + SPL Token balance query |
| v0.12.139 | DONE | Model roster: GLM-5.1 allowlist, nvidia/kimi | Ported model + alias changes |
| v0.12.92 | DONE | `normalizeMessagesForThinking` | reasoning_content on all assistant msgs |
| v0.12.90 | DONE | Empty turn fallback detection | Detect empty + no tool_calls as degraded |
| v0.12.69 | DONE | GPT-5.4 Mini + model roster updates | New model + alias + tier config updates |
| v0.12.66 | DONE | Payment settlement fallback | Adapted: structured fallback error for all models |
| v0.12.65 | DONE | Pre-auth cache key fix | Adapted: cache key includes model in payment module |
| v0.12.64 | DONE | Cost headers, model injection, structured fallback | Cost header, model in SSE chunks, all-models-failed error |
| v0.12.56 | DONE | GLM-5 model picker | Included in model roster updates |
| v0.12.30 | SKIP | Empty release | No changes |
| v0.12.25 | DONE | Docs refresh | Architecture, configuration, troubleshooting updated |
| v0.12.24 | DONE | Preserve user allowlist on restart | InjectModelsConfig() merges user entries |
| v0.12.10 | DONE | /stats clear command | Ported as `dosrouter stats clear` CLI command |

### 2026-04-22 - Sync to v0.12.161

| Release | Status | Summary | Notes |
|---------|--------|---------|-------|
| v0.12.161 | DONE | De-Gemini Anthropic-primary fallbacks | Correlated 503s. Removed google/gemini-* from PremiumTiers[Complex] + AgenticTiers[Complex] chains |
| v0.12.160 | DONE | Free-tier 13→8 realign + Kimi Moonshot-primary | Retired nemotron×3 + mistral-large-3 + devstral-2; added qwen3-next-80b-a3b-thinking + mistral-small-4-119b; flipped kimi-k2.5 primary to moonshot, marked nvidia/kimi-k2.5 Deprecated; added K2.6 ($0.95/$4) Moonshot-only |
| v0.12.159 | SKIP | Market data partner tools + x402 pricing | Needs paid-proxy subsystem (relative proxyPath), not present in DOSRouter partners module |
| v0.12.158 | SKIP | TS plugin lifecycle refactor | TS-only |
| v0.12.157 | SKIP | Prettier formatting | TS-only |
| v0.12.156 | DONE | Opus 4.7 flagship aliases | Added opus/opus-4/opus-4.7 → anthropic/claude-opus-4.7 redirect table |
| v0.12.155 | DONE | Grok 4.20 family (2M ctx) | Added reasoning + non-reasoning + multi-agent variants, $2/$6, 2M context |
| v0.12.153 | DONE | Claude Opus 4.7 | Added model def, kept 4.6 as fallback, promoted 4.7 as PremiumTiers[Complex].Primary |
| v0.12.149 | DONE | Explicit-pin no free fallback | Already DOSRouter default behavior (proxy.go uses [resolvedModel] when decision==nil) |
| v0.12.148 | SKIP | TS plugin config scaffolding | TS-only |

**Config diff summary:**
- `router/config.go`: 6 × `nvidia/kimi-k2.5` → `moonshot/kimi-k2.5` across Tiers/PremiumTiers/AgenticTiers
- `PremiumTiers[Complex].Primary`: `claude-opus-4.6` → `claude-opus-4.7`
- `PremiumTiers[Simple].Primary`: `nvidia/kimi-k2.5` → `moonshot/kimi-k2.6`
- `PremiumTiers[Complex].Fallback` + `AgenticTiers[Complex].Fallback`: stripped `google/gemini-*`, added moonshot K2.6/K2.5, `free/qwen3-coder-480b` backstop

### 2026-04-11 - Full port expansion
- Added: wallet module (EVM key derivation, DOS Chain/Base/Avalanche)
- Added: payment module (x402 protocol, pre-auth cache)
- Added: image generation endpoint (`/v1/images/generations`)
- Added: CLI commands (cache, report, wallet, chain, doctor, stats clear)
- Updated: all docs rebranded from ClawRouter/BlockRun to DOSRouter
- Updated: config paths from `~/.openclaw/blockrun/` to `~/.openclaw/DOS/`
- Updated: model prefix from `blockrun/` to `dosrouter/`
