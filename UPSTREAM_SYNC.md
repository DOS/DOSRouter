# Upstream Sync Tracker

**Upstream**: [BlockRunAI/ClawRouter](https://github.com/BlockRunAI/ClawRouter) (TypeScript)
**This repo**: [DOS/DOSRouter](https://github.com/DOS/DOSRouter) (Go port)
**Last synced**: v0.12.245 (2026-08-16, core ported & aligned)

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

### 2026-08-16 - Sync to v0.12.245 (flagship models, tool-call recovery, proxy hardening)

Diffed `v0.12.199...v0.12.245` (46 tags).

**Ported (Go core):**

| Upstream | Status | Summary |
|----------|--------|---------|
| v0.12.233 | DONE | Claude Opus 5 flagship — model def + bare alias `opus` -> Opus 5 ($5/$25, 1M ctx); promoted to Premium/Agentic Complex primary |
| v0.12.217 | DONE | Claude Sonnet 5 & Fable 5 — model def ($3/$15) + bare alias `claude`/`sonnet` -> Sonnet 5 |
| v0.12.219 | DONE | GPT-5.6 family (Sol, Terra, Luna) — `gpt5` bare alias -> GPT-5.6 Terra ($5/$30); promoted to Auto Complex primary |
| v0.12.234 | DONE | GPT-5.5 Pro & ChatGPT Instant (`chat-latest` alias) |
| v0.12.229/230 | DONE | Kimi K3 & K2.7 — K2.7 bare alias `kimi`, K3 available; promoted to Agentic Medium primary |
| v0.12.231 | DONE | Qwen 3.7 Max ($1.50/$6, 1M ctx) added |
| v0.12.211 | DONE | Z.AI GLM-5.2 flagship added ($1.60/$5) + pricing updates |
| v0.12.200/236 | DONE | MiniMax M3 flagship + vision capability flag ($0.50/$2) |
| v0.12.221/225 | DONE | Grok 4.5 & 4.3 flagships + alias |
| v0.12.201 | DONE | Google Gemini 3.5 Flash added; delisted gemini-3-pro-preview & o1-mini |
| v0.12.201 | DONE | DeepSeek V4 Pro paid model added; DeepSeek V4 Flash EOL -> free/llama-4-maverick fallback |
| v0.12.214/215/230 | DONE | Tool-call recovery: parse structured tool calls from Gemini markdown transcripts, GPT-5.4 plain-text, and Kimi K3 nameless blobs |
| v0.12.208 | DONE | Header sanitization: sanitize `x-clawrouter-reasoning` / response headers to strip non-ASCII / control chars |
| v0.12.207 | DONE | Egress proxy support: configure `http.ProxyFromEnvironment` for `HTTPS_PROXY` / `HTTP_PROXY` / `ALL_PROXY` |

Upstream stopped cutting GitHub *releases* after v0.12.159 but kept *tagging*
through v0.12.199. Diffed `v0.12.161...v0.12.199` (28 tags) by commit.

**Ported (Go core):**

| Upstream | Status | Summary |
|----------|--------|---------|
| v0.12.198 | DONE | Claude Opus 4.8 flagship — model def + aliases (bare/forward opus → 4.8, pins keep version); PremiumTiers[Complex].Primary 4.7 → 4.8, 4.7 + gpt-5.5 into fallback; opus-4.8 prepended to Reasoning/Agentic fallbacks |
| v0.12.168 | DONE | GPT-5.5 flagship — model def ($5/$30, 1.05M ctx) + `gpt5` alias → 5.5; added to Complex fallback chains |
| v0.12.191 | DONE | DeepSeek V4 Flash (free) — model def + V3.2/V4-Pro delisted (NVIDIA hung 2026-04-30) → deprecated FallbackModel redirects |
| v0.12.171/174 | DONE | Kimi K2.6 promoted to bare-alias flagship (K2.5 hidden in BlockRun UI 2026-04-28); K2.5 pin-only; auto+agentic MEDIUM primary K2.5 → K2.6 with K2.5 backstop |
| v0.12.182 | DONE | Reasoning-aware per-model timeout — reasoning models 180s (cold-start first-token), others 60s; cancel-context + AfterFunc, stops timer on success so streams aren't cut. `perModelTimeout()` + test |
| v0.12.165/166/169 | DONE | Suppress tool-call planning prose in content — blank message/delta content when finish_reason==tool_calls or tool_calls array present (Kimi planning prose leak); both SSE + non-streaming paths. `choiceEndsWithToolCalls()` + test |
| v0.12.154 | ALREADY | Degenerate/empty-turn retry — DOSRouter already has `isEmptyTurn` fallback |
| v0.12.167 | DONE | Model registry alignment — covered by the roster updates above |

**Deferred (need vendor / x402 facilitator) — ROADMAP:**

| Upstream | Status | Reason |
|----------|--------|--------|
| v0.12.180/181/186/187 | ROADMAP | Predexon prediction-market tools — shipped upstream as `skills/predexon/SKILL.md` (OpenClaw skill markdown). DOSRouter is a standalone Go binary, not an OpenClaw skill host; needs a Predexon vendor contract regardless. Same class as the v0.12.159 market-data partners (6/9 self-hosted via Pyth, 3 deferred). |
| v0.12.192 | ROADMAP | Phone/voice (Twilio lookup + Bland.ai outbound) — paid endpoints via `proxyPaidApiRequest`/payFetch (x402). Blocked on the same DOS Chain USDC + facilitator URL gap already tracked. |
| v0.12.193/194/195 | ROADMAP | Surf crypto-data + Seedance per-token pricing — also `proxyPaidApiRequest` (x402 paid) + skill markdown. Defer with phone/voice until the facilitator lands. |
| v0.12.179/190 | ROADMAP | gpt-image-2 polling + `/imagegen → /cr-imagegen` rename — image-gen is a passthrough stub in DOSRouter; revisit when image gen is productized. |

**Skipped (TS-only / not applicable to Go):**

| Upstream | Reason |
|----------|--------|
| v0.12.163, 183, 184, 185, 196, 197 | OpenClaw plugin lifecycle / npm install scripts / updater size-drop recovery — TS plugin runtime, no Go equivalent (DOSRouter ships a binary) |
| v0.12.188 | `BLOCKRUN_WEB_SEARCH=off` opt-out — DOSRouter has no web-search feature |
| v0.12.158, 169 (prettier), dist rebuilds, dep bumps, README/marketing, OKX wallet (reverted upstream) | TS-only / cosmetic / upstream-reverted |
| v0.12.172/175/176/177 | OpenClaw model-picker filtering (`TOP_MODELS`, allowlist prune on plugin load) — picker UX is an OpenClaw-plugin concern; DOSRouter exposes `/v1/models` directly |

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
| v0.12.159 | PARTIAL | Market data partner tools + x402 pricing | Ported 6/9 tools as self-hosted Pyth Hermes endpoints (stock price/history/list, crypto/fx/commodity price). See follow-up entry below. 3 tools deferred to roadmap. |
| v0.12.158 | SKIP | TS plugin lifecycle refactor | TS-only |
| v0.12.157 | SKIP | Prettier formatting | TS-only |
| v0.12.156 | DONE | Opus 4.7 flagship aliases | Added opus/opus-4/opus-4.7 → anthropic/claude-opus-4.7 redirect table |
| v0.12.155 | DONE | Grok 4.20 family (2M ctx) | Added reasoning + non-reasoning + multi-agent variants, $2/$6, 2M context |
| v0.12.153 | DONE | Claude Opus 4.7 | Added model def, kept 4.6 as fallback, promoted 4.7 as PremiumTiers[Complex].Primary |
| v0.12.149 | DONE | Explicit-pin no free fallback | Already DOSRouter default behavior (proxy.go uses [resolvedModel] when decision==nil) |
| v0.12.148 | SKIP | TS plugin config scaffolding | TS-only |

### 2026-04-22 - Market data partners self-hosted (v0.12.159 follow-up)

Rather than proxy v0.12.159's 9 market-data partners through BlockRun's
x402-paid backend, DOSRouter serves 6 of them directly from the public
Pyth Network Hermes feed. Zero upstream fee, zero API key, revenue stays
with the DOS operator if a fee layer is added later.

| Tool | Source | Status |
|------|--------|--------|
| `stock_price`, `stock_history`, `stock_list` | Pyth Hermes (`hermes.pyth.network/v2/price_feeds` + `/v2/updates/price/latest`) + Benchmarks (`benchmarks.pyth.network/v1/shims/tradingview/history`) | PORTED |
| `crypto_price`, `fx_price`, `commodity_price` | Pyth Hermes | PORTED |
| `predexon_smart_activity`, `predexon_wallet_pnl`, `predexon_matching_markets` | Predexon | ROADMAP — needs vendor contract or on-chain Polymarket indexer |
| `x_users_lookup` | Twitter/X API v2 | ROADMAP — needs paid Twitter API tier |

New files:
- `partners/pyth.go` — Hermes + Benchmarks HTTP client (search feeds, latest price, OHLC bars).
- `partners/market.go` — six `/v1/stocks|crypto|fx|commodity/*` handlers.
- `partners/market_test.go` — pure-logic unit tests.
- `docs/market-data-partners.md` — endpoint reference + coverage notes.

Schema changes:
- `PartnerServiceDefinition`: added `ProxyPath` field (routes tool via
  local DOSRouter proxy instead of absolute `BaseURL`).
- `BuildPartnerTools(apiKey, localProxyBase)` — signature extended so
  ProxyPath services know where to reach the local proxy.

Out of scope for this pass:
- **x402 signing on paid endpoints** — DOS Chain USDC not deployed, no
  facilitator URL available. Paid tools remain free at this layer;
  revenue model to be revisited once DOS Chain token lands.
- **Non-US stock coverage** — Pyth's equity feed is US-heavy. HK/JP/KR/EU
  requests return 404 for tickers Pyth doesn't index. Deferred to a
  future release that layers a secondary data provider.

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
