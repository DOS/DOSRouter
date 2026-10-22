# Upstream Sync Tracker

**Upstream**: [BlockRunAI/ClawRouter](https://github.com/BlockRunAI/ClawRouter) (TypeScript)
**This repo**: [DOS/DOSRouter](https://github.com/DOS/DOSRouter) (Go port)
**Last synced**: 0.12.297 source (`78e489a`, checked 2026-10-22; latest published release/tag is v0.12.294, source-only changes through 0.12.297 included; adaptations and exclusions below)

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

### 2026-10-22 - Post-merge local runtime acceptance

- Owner: Codex, branch `codex/smoke-upstream-20261022`.
- Revision under test: merged main `422774fe3ba3d94c3e9be2e7082945ed54222770`.
- Outcome: verify the actual standalone CLI process over HTTP and a browser.
- Scope: DOSRouter test harness and evidence; isolated localhost gateway and
  synthetic state only. No provider charge, credentials or remote rollout.
- Acceptance: binary revision/hash, startup catalog refresh, explicit model
  and free aliases, fallback/outcomes, streaming, spend admission, local trust,
  clean shutdown and browser readback with machine-readable evidence.
- Environment discovery: GitHub environments list is empty; the current
  process has no DOSROUTER_UPSTREAM, DOSROUTER_PORT or DOSROUTER_API_KEY.
  Historical npm/Docker plans do not describe this standalone Go runtime.
- Completed: clean-source Go 1.26.10 binary, SHA-256
  `5bd95979b1176002133def84734e3b98cd6d70cb107d58627ffb00cede9f7a27`;
  all 18 process scenarios passed. The real refresh intervals were 300011
  and 300012 ms; invalid startup recovered and invalid updates retained the
  last good catalog. MCP Playwright verified same-origin health, model list
  and chat, plus cross-site HTTP 403. All three CLI processes exited and
  their ports closed. Report rendering passed desktop and 390px checks.
- Driver correction: Node fetch ignored a forged Host override. The original
  failed observation remains recorded; literal raw HTTP against the same
  binary returned four HTTP 403 responses with zero gateway sends. The fixed
  harness then passed a separate full 18-scenario run without corrections.
- Independent review found two harness gaps: late SIGINT registration and a
  vacuously true reasoning assertion. Both were fixed. A focused final-source
  probe passed 16 immediate scenarios, preserved the exact assistant message
  and model, and confirmed owned-process shutdown on Ctrl+C during the timer
  wait. It makes no claim for the interrupted refresh scenario.
- CodeRabbit's subsequent review added browser-proof correlation and cleanup
  on a failed concurrency assertion. The finish endpoint now rejects missing
  gateway-chat evidence or mismatched browser fields; the renderer checks the
  correlation too. An injected unexpected concurrency response still releases
  and settles the held request. A final-source process probe passed all 16
  immediate scenarios and interrupt shutdown. External MCP browser observations
  remain operator attestations, with gateway chat independently correlated.
- Evidence: [Vietnamese report](docs/validation/2026-10-22/upstream-smoke.html),
  [initial run and correction](docs/validation/2026-10-22/upstream-smoke.json),
  [full rerun and final-source probes](docs/validation/2026-10-22/harness-recheck.json)
  [failure-path and final delta probes](docs/validation/2026-10-22/harness-failure-probes.json)
  and [report browser readback](docs/validation/2026-10-22/report-browser.json).
- Remaining boundary: remote staging, paid-provider behavior and deployment
  remain unverified because no target environment is configured or authorized.
  No product source change was needed for the process acceptance.

### 2026-10-22 - Sync through 0.12.297 source

Owner: Codex, branch `codex/sync-clawrouter-20261022`. Compared the previous
release commit `becb2968711bb203a1498958de27a2b1be3780c3` with upstream main
`78e489a` (package version 0.12.297). GitHub's latest published release and
tag at capture time is v0.12.294; 0.12.295-297 are source versions, not newly
published GitHub releases. The sync includes the subsequent retired-Surf cleanup.

Pinned dependency sources: router-core
`8773a9954b091bd0090acdfec67c75a006c0e156` and model-catalog
`b95efe2f025c9968742e6479bc9ec71b6a40c0f4` (catalog 2026.10.20.4).

| Upstream change | Go disposition |
| --- | --- |
| v0.12.280-281 / 294-297 chat catalog | Add 17 chat records and 34 aliases, correct Sonnet 5 and DeepSeek Vision prices, carry Mistral Large 4's list price, remove Nano 30B from the free fallback lineup, and mark Hy3/retired GPT/Grok IDs. Preserve DOS-specific aliases, exact Nano 30B pins and retired free-to-free redirects. Bare `deepseek` follows V4.1 Flash; free V4 Flash aliases remain free. |
| v0.12.287 gateway catalog | Each serving proxy refreshes only its configured gateway's `/v1/models?format=json` at startup and every five minutes. Validate complete updates; preserve the last good snapshot after failures; refuse redirects and free-product billing changes. New chat models are explicit picks, with no inferred tool eligibility or automatic tier promotion. Each request retains its admission-price snapshot through settlement. No remote ordering policy is fetched. |
| v0.12.281 outcome memory | Per-server, mutex-protected last 10 outcomes for each model/chat-or-tools pair, 30-minute expiry, 512 keys. Three failures at a 50% rate move a model behind reliable candidates before the five-attempt cap. Preserve explicit pins and all-unreliable ordering. Count server failures, incomplete bodies, empty turns, attempt timeouts and observed SSE outcomes; ignore caller cancellation and payment errors. Empty non-streaming turns use the full bounded chain. |
| v0.12.291-297 verified tools | Add the 13 probe-verified recovery candidates behind configured eligible models, ordered by estimated token price outside Premium. Preserve proven capability metadata; catalog categories cannot grant tool support. All 16 configured tier chains retain the previous primaries. |
| v0.12.292 scoring | Size CJK characters at roughly one token each, bound intent scans to both ends of 8,000 characters, match word boundaries/inflections instead of unrelated substrings, use explicit `response_format` for structured output, and stop claiming savings for unknown prices. |
| v0.12.282 local trust | Bind the CLI server to `127.0.0.1`; reject non-local Host, cross-site/opaque Origin and cross-site browser requests on every route. `Server.Handler()` applies the same HTTP guards for embedders. |
| v0.12.283 strict spend reservations | Existing Go reservations already account atomically for in-flight attempts and conservatively settle lost/5xx responses. Keep that implementation. Gateway claims of zero pricing outside local `free/*` products cannot bypass configured limits. Quote-before-sign accounting is inapplicable while x402 signing remains unimplemented. |
| v0.12.284 / 286, media, host integration | Skip account-ledger reconciliation, Solana batch settlement, local img2img filesystem/URL readers, paid OpenClaw sender commands, frozen OpenClaw configuration and Desktop lifecycle: these surfaces do not exist in this Go runtime. Existing image passthrough and Pyth-backed partners remain unchanged. |
| npm/Node, brand, picker, Surf | Skip TS dependencies/build/publishing, Desktop UI, brand counters and picker-only changes. DOSRouter has no Surf integration to remove. Retain the existing Go CI/security workflows and independent automated review. |

Intentional limitations: the existing Go rules strategy remains in place; the
optional TypeScript portfolio strategy, empirical speed-probe decay and its
task-affinity model are not introduced by this sync. Go does not yet identify
repetitive-output loops as a separate failure signal. No x402 signer, ledger
reconciliation or new paid media/provider contract is enabled.

Prices and availability in the bundled snapshot are upstream source metadata,
not live DOS provider verification. Gemini 4 Argon's bundled $2/$10 promotion
ends after 2026-11-30 (then $4/$20); offline date-based repricing is not
implemented. Successful catalog refreshes use the configured gateway's rates.
The existing Gemini Flash promotion caveat below still applies offline.

Validation: regression tests, `go test ./...`, `go vet ./...` and
`go build ./...` passed locally. Native Windows has no cgo compiler, so the
race run is covered by the existing Linux CI job. CI race tests, vulnerability
scan, CodeQL, final-head review and merge acceptance are recorded on the sync
PR. Scope is DOSRouter source and documentation only; no deployment, paid
provider request, credential change or cross-repository write is included.

CI unblock: the first scanner run on `35a4b8c` found 10 reachable standard
library advisories in Go 1.26.6, including GO-2026-6809 (fixed in 1.26.10)
and the Go 1.26.9 HTTP/TLS fixes. No open PR owns this module change. The
smallest unblock raises only the preferred toolchain to Go 1.26.10; the Go
language minimum and module dependencies stay unchanged. Re-run the existing
security, build/race and CodeQL jobs on the resulting head before merge.
The same govulncheck command then passed locally with zero reachable
vulnerabilities (18 module-level advisories remain in code not called here).
Review hardening: bound non-streaming chat bodies to 64 MiB and error bodies
to 1 MiB. Incomplete/oversized bodies stop fallback and preserve accounting;
boundary tests and the existing truncated-response settlement regression pass.
The completed baseline CodeRabbit review identified three Go adaptation edges:
retarget bare Tencent/Hy3/Hunyuan aliases directly to the declared Qwen
successor (keep exact `tencent/hy3` pins), reject remote zero flat-price claims
under spend limits, and use Go's 500-character scan default for omitted or
negative limits. Each issue has a failing-then-passing regression test.

### 2026-09-22 - Sync to the v0.12.279 release

Verified the upstream release list, fetched tags, and compared the previous
source baseline `05de1e0` with the peeled `v0.12.279` commit
`becb2968711bb203a1498958de27a2b1be3780c3`. This is the latest non-preview
release observed on this date, published on 2026-09-16. The subsequently
published `v0.12.278` tag resolves to
`107c2d1d8b4847671ec0aad4b00902b9bcf4170c`; the previous sync had already
covered the later `05de1e0` source snapshot, so its changes are not new work.

The complete incremental diff contains 12 files. There are **no changes**
under upstream `src/`, `packages/`, or `runtime/`. The Go router, catalog,
proxy, caching, spend controls and payment implementation therefore require
no new port in this release. Existing exclusions and limitations from the
2026-09-12 sync remain in force.

| Incremental upstream change | Disposition |
| --- | --- |
| Desktop control-plane layout, themes, wallet/funding UI, API view models | Skip: Electron/React application, absent from this standalone Go runtime. |
| Desktop model-count deduplication and chart/card calendar-window alignment (`usage-stats.ts` and tests) | Skip: presentation calculations over Desktop data; no equivalent chart/card in DOSRouter and no upstream server stats change. |
| Desktop managed runtime package version | Skip: npm child-runtime packaging; DOSRouter ships its own Go binary. |
| Brand snapshot and README counts | Skip: BlockRun marketing/catalog counts are not verified DOS provider availability. No catalog rows or prices changed in the incremental source diff. |
| Hardened `sync-brand-numbers.mjs` rendering/attribute escaping | Skip: DOSRouter does not run this upstream brand-sync script. |
| Root package/lock version and release changelog | Record the release here; no Go module dependency change. |

The v0.12.279 release notes also summarize earlier restart-free Desktop
chain switching, Hermes reload behavior, axios/hono patches and the Solana
single-copy signer build guard. Those changes are already present in the
previous upstream snapshot and are outside the standalone Go scope; they
must not be counted again as newly ported fixes.

Reproducible scope checks:

```sh
git rev-parse 'v0.12.279^{}'
git diff --name-only 05de1e0 v0.12.279
git diff --exit-code 05de1e0 v0.12.279 -- src packages runtime
```

This is a release-parity documentation update. It does not deploy DOS-AI,
change live routing or catalog records, or activate deferred payment features.
Validation and automated review are retained on the sync PR.

### 2026-09-12 - Sync v0.12.245 to v0.12.278 source snapshot

Compared `v0.12.245...05de1e0` from BlockRunAI/ClawRouter. The newest
published Git tag at sync time is `v0.12.277`; upstream `package.json` and
CHANGELOG identify the main snapshot as `0.12.278`. This is a source parity
update, not a claim that every BlockRun product feature is implemented.

**Ported or adapted:**

| Upstream area | Go adaptation |
| --- | --- |
| v0.12.248 assistant/tool prose | Preserve assistant prose with native calls and text-recovered calls; strip tagged thinking, including split SSE tags. `DOSROUTER_TOOL_CALL_PROSE=off` restores legacy suppression. Recover syntax only when tools are supplied. |
| v0.12.252 tool-pair safety | Preserve `tool_calls`, `tool_call_id`, names and all provider extension fields during request rewriting. Avoid compressing protocol-bearing or multimodal messages. DOSRouter has no upstream-style message truncation path. |
| v0.12.254-256 cancellation/cache | Keep Go request contexts through chat/image requests, stop fallback after disconnect, reject incomplete bodies, preserve caller-controlled timestamp content in cache keys; normalize JSON object key order without conflating arrays. |
| v0.12.257-278 models/routing | Align chat catalog metadata and all four profile chains with router-core `5ee7c23c993013a8052588191569db5cf7fb793c`; retain DOS aliases and exact explicit pins. Retire dead free defaults, fix capability claims and prices. |
| v0.12.263/269/274 spend safety | Atomic in-flight reservations for direct and routed chat, including fallback attempts; pending spend counts in rolling/session caps. Persist snapshots serially with atomic file replacement. Invalid state/cost fails closed. Each Server uses one controller; embedded callers may inject a shared controller explicitly. |
| v0.12.267 ambiguous sends | Do not repeat a chat send or switch models after an ambiguous transport failure. Each reservation authorizes one HTTP send. Status retries are disabled; model fallback obtains a separate reservation and ambiguous server errors retain their estimate. DOSRouter does not yet sign x402 payments. |
| v0.12.271-275 accounting/health | Prefer settled gateway cost headers, otherwise actual token usage, then explicitly labelled estimates. Capture gateway request IDs in usage logs; report the configured gateway origin in health. Image cost reads headers/body. Reject unknown-priced images when amount limits are configured. |
| v0.12.272 credential transport | Refuse upstream redirects, avoid shared internal caching across caller-supplied bearer credentials (including a configured upstream key), and mark authenticated responses `no-store`. |
| Stats day windows | Go already defaulted nonpositive windows safely; cap aggregate reporting to 30 days and test using isolated log directories. |
| Validation adaptation | Replace stale TypeScript/npm CI and missing Docker scanner targets with Go build, vet, race tests and govulncheck. Preserve job/workflow names and automatic CodeQL; prefer patched Go 1.26.6 via the toolchain directive. Dependabot follows Go modules and Actions. |

**Catalog notes:** 114 upstream chat rows plus nine compatibility records, with
250 chat aliases. The free default is `free/nemotron-3.5-lightning`. Model
metadata reflects upstream source, not independently probed DOS providers.
Gemini 3.6/3.8 Flash's $0.75/$3.75 promotional rates end on 2027-01-01,
when upstream documents $1.50/$7.50; automated repricing is not implemented.

**Intentional divergence:** The upstream timestamp-stripping optimization is not enabled: a standalone server cannot distinguish injected prefixes from client-authored content. Cache/dedup keys preserve both string and first text-block timestamps until trusted injection provenance exists.

**Already satisfied:** `/v1/models` lists active catalog entries; chat and image
requests derive their context from the client; full health performs no balance
RPC; nonpositive stats/log windows have safe defaults.

**Excluded or deferred:**

- OpenClaw plugin identity/migrations, desktop releases, npm dependencies,
  Solana defaults/RPC/signing, and the added-then-removed TWZRD integration are
  outside this standalone Go runtime.
- BlockRun login/account-credit/status/reconcile APIs, account service proxying,
  vendor-specific paid endpoints, image/video aliases and async polling require
  separate product/provider contracts. The existing image endpoint remains a
  passthrough. Missing media cost must not be interpreted as proof of a free call.
- x402 counterparty policy and signing hooks remain deferred: `payment.submitPayment`
  still returns `Success:false`; no signing, live charge or facilitator rollout
  occurred. Earlier tracker wording about a full payment port overstated support.
- Spend limits use estimates before dispatch, then gateway/token evidence when
  available. They are not a provider-enforced USD guarantee. Unconfirmed sends
  conservatively consume their estimate. Reservations and session counters are
  process-local and reset at restart; file storage is not a multi-process ledger.
  Cross-server sharing requires an explicitly shared controller.
- Streaming textual tool-call synthesis is not implemented; native streaming
  tool calls and prose are preserved. No live provider request was used to
  verify catalog availability or pricing.

Validation is recorded in the sync PR: Go unit/integration tests, race tests,
build, vet, vulnerability scan and configured automated reviews. No deployment
workflow exists in this repository, and this sync does not deploy DOS-AI.

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
