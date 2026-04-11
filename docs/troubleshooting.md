# Troubleshooting

Quick solutions for common DOSRouter issues.

> Need help? [Open a Discussion](https://github.com/DOSAI/DOSRouter/discussions) or check [existing issues](https://github.com/DOSAI/DOSRouter/issues).

## Table of Contents

- [Quick Checklist](#quick-checklist)
- [Common Errors](#common-errors)
- [Security Scanner Warnings](#security-scanner-warnings)
- [Port Conflicts](#port-conflicts)
- [How to Update](#how-to-update)
- [Verify Routing](#verify-routing)

---

## Quick Checklist

```bash
# 1. Check your version (should be 0.12+)
cat ~/.openclaw/DOS/extensions/dosrouter/package.json | grep version

# 2. Check proxy is running
curl http://localhost:8402/health

# 3. Check wallet (both EVM + Solana addresses and balance)
/wallet

# 4. Watch routing in action
dosrouter logs --follow
# Should see: kimi-k2.5 $0.0012 (saved 99%)

# 5. View cost savings
/stats
```

---

## Common Errors

### "Unknown model: dos/auto" or "Unknown model: auto"

Plugin isn't loaded or outdated. **Don't change the model name** — `dos/auto` is correct.

**Fix:** Update to v0.3.21+ which handles both `dos/auto` and `auto` (DOSRouter strips provider prefix). See [How to Update](#how-to-update).

### "No API key found for provider dos"

Auth profile is missing or wasn't created properly.

**Fix:** See [How to Update](#how-to-update) — the reinstall script automatically injects the auth profile.

### "Config validation failed: plugin not found: dosrouter"

Plugin directory was removed but config still references it. This blocks all DOSRouter commands until fixed.

**Fix:** See [How to Update](#how-to-update) for complete cleanup steps.

### "No USDC balance" / "Insufficient funds"

Wallet needs funding. DOSRouter accepts **USDC** (not SOL or ETH) on either chain.

**Fix:**

1. Find your wallet address: run `/wallet` in any DOSRouter conversation
2. Choose your preferred chain and send **USDC** to that address:
   - **Base (EVM):** Send USDC on Base network to your EVM address (`0x...`)
   - **Solana:** Send USDC on Solana network to your Solana address (base58)
3. $1–5 is enough for hundreds of requests
4. Restart DOSRouter (or wait up to 60s for balance cache to refresh)

---

## Security Scanner Warnings

### "WARNING: dangerous code patterns — possible credential harvesting"

This is a **false positive**. DOSRouter legitimately:

1. Reads `DOSROUTER_WALLET_KEY` from environment (for authentication)
2. Sends authenticated requests to DOS API (for x402 micropayments)

This pattern triggers DOSRouter's security scanner, but it's the intended behavior — the wallet key is required to sign payment transactions. The code is fully open source and auditable.

### "env-harvesting" Warning

DOSRouter's security scanner may flag DOSRouter with:

```
[env-harvesting] Environment variable access combined with network send
```

**This is a false positive.** The scanner's heuristic (`env variable + network request = suspicious`) flags all payment plugins, but this pattern is inherently required for non-custodial payments.

DOSRouter reads `DOSROUTER_WALLET_KEY` to sign x402 payment transactions — this is required and intentional:

- The wallet key is used **locally** for cryptographic signing (EIP-712)
- The **signature** is transmitted, not the private key itself
- The key **never leaves the machine** — only cryptographic proofs are sent
- This is standard [x402 payment protocol](https://x402.org) behavior
- Source code is [MIT licensed and fully auditable](https://github.com/DOSAI/DOSRouter)

See [`dosrouter.security.json`](../dosrouter.security.json) for detailed security documentation and [this discussion](https://x.com/bc1beat/status/2020158972561428686) for more context.

---

## Port Conflicts

### Port 8402 already in use

As of v0.4.1, DOSRouter automatically detects and reuses an existing proxy on the configured port instead of failing with `EADDRINUSE`. You should no longer see this error.

If you need to use a different port:

```bash
# Set custom port via environment variable
export DOSROUTER_PROXY_PORT=8403
dosrouter gateway restart
```

To manually check/kill the process:

```bash
lsof -i :8402
# Kill the process or restart DOSRouter
```

---

## How to Update

```bash
npx @DOS/DOSRouter@latest
dosrouter gateway restart
```

This installs the latest version and restarts the gateway. Alternatively:

```bash
curl -fsSL https://raw.githubusercontent.com/DOSAI/DOSRouter/main/scripts/reinstall.sh | bash
dosrouter gateway restart
```

---

## Verify Routing

```bash
dosrouter logs --follow
```

You should see model selection for each request:

```
[plugins] [SIMPLE] google/gemini-2.5-flash $0.0012 (saved 99%)
[plugins] [MEDIUM] deepseek/deepseek-chat $0.0003 (saved 99%)
[plugins] [REASONING] deepseek/deepseek-reasoner $0.0005 (saved 99%)
```
