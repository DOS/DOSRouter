# Upstream Sync Tracker

**Upstream**: [BlockRunAI/ClawRouter](https://github.com/BlockRunAI/ClawRouter) (TypeScript)
**This repo**: [DOS/DOSRouter](https://github.com/DOS/DOSRouter) (Go port)
**Last synced**: v0.12.106 (initial port, 2026-04-07)

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

### 2026-04-10 - Pending sync (v0.12.106 -> v0.12.145)

| Release | Status | Summary | Notes |
|---------|--------|---------|-------|
| v0.12.145 | TODO | eco/premium null tier fallback | Fall back to default tiers when eco/premiumTiers is nil |
| v0.12.144 | TODO | Session pinning `userExplicit` flag | User's /model choice wins over profile routing |
| v0.12.143 | SKIP | Prettier formatting | TS-only |
| v0.12.142 | SKIP | Deferred proxy startup for plugin config | OpenClaw plugin lifecycle |
| v0.12.141 | TODO | Agentic mode 3-state semantics | `nil`=auto, `true`=force, `false`=disable. Merge all tier sets |
| v0.12.140 | SKIP | Solana doctor fix | Payment module |
| v0.12.139 | SKIP | baseUrl overwrite fix + GLM allowlist | OpenClaw plugin lifecycle |
| v0.12.92 | TODO | `normalizeMessagesForThinking` fix | Add reasoning_content to all assistant msgs for reasoning models |
| v0.12.90 | TODO | Empty turn fallback detection | Detect empty content + no tool_calls as degraded response |
| v0.12.69 | TODO | GPT-5.4 Nano + Gemini 3.1 Flash Lite | Model roster update |
| v0.12.66 | TODO | Payment fail -> free model fallback | May adapt: treat as provider error for fallback |
| v0.12.65 | SKIP | Pre-auth cache key fix | Payment module |
| v0.12.64 | TODO | Review needed | Check changelog |
| v0.12.56 | TODO | Review needed | Check changelog |
| v0.12.30 | TODO | Review needed | Check changelog |
| v0.12.25 | TODO | Review needed | Check changelog |
| v0.12.24 | TODO | Review needed | Check changelog |
| v0.12.10 | TODO | Review needed | Check changelog |
