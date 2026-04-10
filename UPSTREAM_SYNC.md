# Upstream Sync Tracker

**Upstream**: [BlockRunAI/ClawRouter](https://github.com/BlockRunAI/ClawRouter) (TypeScript)
**This repo**: [DOS/DOSRouter](https://github.com/DOS/DOSRouter) (Go port)
**Last synced**: v0.12.146 (2026-04-11)

## Sync Workflow

1. Check new releases: `gh api repos/BlockRunAI/ClawRouter/releases --jq '.[].tag_name' | head -20`
2. Review changelog for each release since last synced version
3. Classify changes: **port** (routing/model/strategy logic) or **skip** (TS-specific, payment, plugin)
4. Port in batches, commit with: `port: <summary> (upstream vX.Y.Z)`
5. Update "Last synced" above after each sync session

## Scope

DOSRouter ports **routing logic only**. These upstream areas are excluded:
- Payment/wallet (Solana, EVM, x402)
- OpenClaw plugin lifecycle (register, reload, baseUrl)
- Web search providers
- Image/music generation
- CLI commands (/wallet, /doctor, /update)
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
| v0.12.140 | SKIP | Solana doctor fix | Payment module |
| v0.12.139 | DONE | Model roster: GLM-5.1 allowlist, nvidia/kimi | Ported model + alias changes |
| v0.12.92 | DONE | `normalizeMessagesForThinking` | reasoning_content on all assistant msgs |
| v0.12.90 | DONE | Empty turn fallback detection | Detect empty + no tool_calls as degraded |
| v0.12.69 | DONE | GPT-5.4 Mini + model roster updates | New model + alias + tier config updates |
| v0.12.66 | SKIP | Payment settlement fallback | Payment module (DOSRouter scope: no payment) |
| v0.12.65 | SKIP | Pre-auth cache key fix | Payment module |
| v0.12.64 | SKIP | Reviewed - plugin/payment only | No routing changes |
| v0.12.56 | SKIP | Reviewed - plugin/payment only | No routing changes |
| v0.12.30 | SKIP | Reviewed - plugin/payment only | No routing changes |
| v0.12.25 | SKIP | Reviewed - plugin/payment only | No routing changes |
| v0.12.24 | SKIP | Reviewed - plugin/payment only | No routing changes |
| v0.12.10 | SKIP | /stats clear command | CLI-only, not applicable to Go proxy |
