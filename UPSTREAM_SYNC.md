# Upstream Sync Tracker

**Upstream**: [BlockRunAI/ClawRouter](https://github.com/BlockRunAI/ClawRouter) (TypeScript)
**This repo**: [DOS/DOSRouter](https://github.com/DOS/DOSRouter) (Go port)
**Last synced**: v0.12.146 (2026-04-11)

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
| v0.12.142 | SKIP | Deferred proxy startup for plugin config | OpenClaw plugin lifecycle |
| v0.12.141 | DONE | Agentic mode 3-state semantics | `nil`=auto, `true`=force, `false`=disable |
| v0.12.140 | SKIP | Solana doctor fix | Solana-only, DOSRouter is EVM-only |
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
| v0.12.24 | SKIP | Preserve user allowlist on restart | OpenClaw plugin-specific |
| v0.12.10 | DONE | /stats clear command | Ported as `dosrouter stats clear` CLI command |

### 2026-04-11 - Full port expansion
- Added: wallet module (EVM key derivation, DOS Chain/Base/Avalanche)
- Added: payment module (x402 protocol, pre-auth cache)
- Added: image generation endpoint (`/v1/images/generations`)
- Added: CLI commands (cache, report, wallet, chain, doctor, stats clear)
- Updated: all docs rebranded from ClawRouter/BlockRun to DOSRouter
- Updated: config paths from `~/.openclaw/blockrun/` to `~/.dosrouter/`
- Updated: model prefix from `blockrun/` to `dosrouter/`
