# Using Subscriptions with DOSRouter Failover

This guide explains how to use your existing LLM subscriptions (Claude Pro/Max, ChatGPT Plus, etc.) as primary providers, with DOSRouter x402 micropayments as automatic failover.

## Why Not Built Into DOSRouter?

After careful consideration, we decided **not** to integrate subscription support directly into DOSRouter for several important reasons:

### 1. Terms of Service Compliance

- Most subscription ToS (Claude Code, ChatGPT Plus) are designed for personal use
- Using them through a proxy/API service may violate provider agreements
- We want to keep DOSRouter compliant and low-risk for all users

### 2. Security & Privacy

- Integrating subscriptions would require DOSRouter to access your credentials/sessions
- Spawning external processes (like Claude CLI) introduces security concerns
- Better to keep authentication at the DOSRouter layer where you control it

### 3. Maintenance & Flexibility

- Each subscription provider has different APIs, CLIs, and authentication methods
- DOSRouter already has a robust provider system that handles this
- Duplicating this in DOSRouter would increase complexity without added value

### 4. Better Architecture

- DOSRouter's native failover mechanism is more flexible and powerful
- Works with **any** provider (not just Claude)
- Zero code changes needed in DOSRouter
- You maintain full control over your credentials

## How It Works

DOSRouter has a built-in **model fallback chain** that automatically tries alternative providers when the primary fails:

```
User Request
    ↓
Primary Provider (e.g., Claude subscription via DOSRouter)
    ↓ (rate limited / quota exceeded / auth failed)
DOSRouter detects failure
    ↓
Fallback Chain (try each in order)
    ↓
DOSRouter (dos/auto)
    ↓
Smart routing picks cheapest model
    ↓
x402 micropayment to DOS API
    ↓
Response returned to user
```

**Key benefits:**

- ✅ Automatic failover (no manual intervention)
- ✅ Works with any subscription provider DOSRouter supports
- ✅ Respects provider ToS (you configure authentication directly)
- ✅ DOSRouter stays focused on cost optimization

## Setup Guide

### Prerequisites

1. **DOSRouter Gateway installed** with DOSRouter plugin

   ```bash
   npm install -g dosrouter
   dosrouter plugins install @dos/dosrouter
   ```

2. **Subscription configured in DOSRouter**
   - For Claude: Use `claude setup-token` or API key
   - For OpenAI: Set `OPENAI_API_KEY` environment variable
   - For others: See [DOSRouter provider docs](https://docs.dosrouter.ai)

3. **DOSRouter wallet funded** (for failover)
   ```bash
   dosrouter gateway logs | grep "Wallet:"
   # Send USDC to the displayed address on Base network
   ```

### Configuration Steps

#### Step 1: Set Primary Model (Your Subscription)

```bash
# Option A: Using Claude subscription
dosrouter models set anthropic/claude-sonnet-4.6

# Option B: Using ChatGPT Plus (via OpenAI provider)
dosrouter models set openai/gpt-4o

# Option C: Using any other provider
dosrouter models set <provider>/<model>
```

#### Step 2: Add DOSRouter as Fallback

```bash
# Add dos/auto for smart routing (recommended)
dosrouter models fallbacks add dos/auto

# Or specify a specific model
dosrouter models fallbacks add dos/google/gemini-2.5-pro
```

#### Step 3: Verify Configuration

```bash
dosrouter models show
```

Expected output:

```
Primary: anthropic/claude-sonnet-4.6
Fallbacks:
  1. dos/auto
```

#### Step 4: Test Failover (Optional)

To verify failover works:

1. **Temporarily exhaust your subscription quota** (or wait for rate limit)
2. **Make a request** - DOSRouter should automatically failover to DOSRouter
3. **Check logs:**
   ```bash
   dosrouter gateway logs | grep -i "fallback\|dos"
   ```

### Advanced Configuration

#### Configure Multiple Fallbacks

```bash
dosrouter models fallbacks add dos/google/gemini-2.5-flash  # Fast & cheap
dosrouter models fallbacks add dos/deepseek/deepseek-chat   # Even cheaper
dosrouter models fallbacks add dos/nvidia/gpt-oss-120b      # Free tier
```

#### Per-Agent Configuration

Edit `~/.dosrouter/dosrouter.json`:

```json
{
  "agents": {
    "main": {
      "model": {
        "primary": "anthropic/claude-opus-4.6",
        "fallbacks": ["dos/auto"]
      }
    },
    "coding": {
      "model": {
        "primary": "anthropic/claude-sonnet-4.6",
        "fallbacks": ["dos/google/gemini-2.5-pro", "dos/deepseek/deepseek-chat"]
      }
    }
  }
}
```

#### Tier-Based Configuration (DOSRouter Smart Routing)

When using `dos/auto`, DOSRouter automatically classifies your request and picks the cheapest capable model:

- **SIMPLE** queries → Gemini 2.5 Flash, DeepSeek Chat (~$0.0001/req)
- **MEDIUM** queries → GPT-4o-mini, Gemini Flash (~$0.001/req)
- **COMPLEX** queries → Claude Sonnet, Gemini Pro (~$0.01/req)
- **REASONING** queries → DeepSeek R1, o3-mini (~$0.05/req)

Learn more: [DOSRouter Smart Routing](./smart-routing.md)

## Monitoring & Troubleshooting

### Check If Failover Is Working

```bash
# Watch real-time logs
dosrouter gateway logs --follow | grep -i "fallback\|dos\|rate.limit\|quota"

# Check DOSRouter proxy logs
dosrouter gateway logs | grep "DOSRouter"
```

**Success indicators:**

- ✅ "Rate limit reached" or "Quota exceeded" → primary failed
- ✅ "Trying fallback: dos/auto" → failover triggered
- ✅ "DOSRouter: Success with model" → failover succeeded

### Common Issues

#### Issue: Failover never triggers

**Symptoms:** Always uses primary, never switches to DOSRouter

**Solutions:**

1. Check fallbacks are configured:
   ```bash
   dosrouter models show
   ```
2. Verify primary is actually failing (check provider dashboard for quota/rate limits)
3. Check DOSRouter logs for authentication errors

#### Issue: "Wallet empty" errors during failover

**Symptoms:** Failover triggers but DOSRouter returns balance errors

**Solutions:**

1. Check DOSRouter wallet balance:
   ```bash
   dosrouter gateway logs | grep "Balance:"
   ```
2. Fund wallet on Base network (USDC)
3. Verify wallet key is configured correctly

#### Issue: Slow failover (high latency)

**Symptoms:** 5-10 second delay when switching to DOSRouter

**Cause:** DOSRouter tries multiple auth profiles before failover

**Solutions:**

1. Reduce auth profile retry attempts (see DOSRouter config)
2. Use `dos/auto` as primary for faster responses
3. Accept the latency as a tradeoff for cheaper requests

## Cost Analysis

### Example Scenario

**Usage pattern:**

- 100 requests/day
- 50% hit Claude subscription quota (rate limited)
- 50% use DOSRouter failover

**Without failover:**

- Pay Anthropic API: $50/month (100% API usage)

**With failover:**

- Claude subscription: $20/month (covers 50%)
- DOSRouter x402: ~$5/month (50 requests via smart routing)
- **Total: $25/month (50% savings)**

### When Does This Make Sense?

✅ **Good fit:**

- You already have a subscription for personal use
- You occasionally exceed quota/rate limits
- You want cost optimization without managing API keys

❌ **Not ideal:**

- You need 100% reliability (subscriptions have rate limits)
- You prefer a single provider (no failover complexity)
- Your usage is low (< 10 requests/day)

## FAQ

### Q: Will this violate my subscription ToS?

**A:** You configure the subscription directly in DOSRouter using your own credentials. DOSRouter only receives requests after your subscription fails. This is similar to using multiple API keys yourself.

However, each provider has different ToS. Check yours before proceeding:

- [Claude Code Terms](https://claude.ai/terms)
- [ChatGPT Terms](https://openai.com/policies/terms-of-use)

### Q: Can I use multiple subscriptions?

**A:** Yes! Configure multiple providers with failback chains:

```bash
dosrouter models set anthropic/claude-opus-4.6
dosrouter models fallbacks add openai/gpt-4o          # ChatGPT Plus
dosrouter models fallbacks add dos/auto           # x402 as final fallback
```

### Q: Does this work with Claude Max API Proxy?

**A:** Yes! Configure the proxy as a custom provider in DOSRouter, then add `dos/auto` as fallback.

See: [Claude Max API Proxy Guide](https://github.com/anthropics/claude-code/blob/main/docs/providers/claude-max-api-proxy.md)

### Q: How is this different from PR #15?

**A:** PR #15 integrated Claude CLI directly into DOSRouter. Our approach:

- ✅ Works with any provider (not just Claude)
- ✅ Respects provider ToS (no proxy/wrapper)
- ✅ Uses DOSRouter's native failover (more reliable)
- ✅ Zero maintenance burden on DOSRouter

## Feedback & Support

We'd love to hear your experience with subscription failover:

- **GitHub Discussion:** [Share your setup](https://github.com/DOSAI/DOSRouter/discussions)
- **Issues:** [Report problems](https://github.com/DOSAI/DOSRouter/issues)
- **Telegram:** [Join community](https://t.me/dosAI)

## Related Documentation

- [DOSRouter Model Failover](https://docs.dosrouter.ai/concepts/model-failover)
- [DOSRouter Provider Configuration](https://docs.dosrouter.ai/gateway/configuration)
- [DOSRouter Smart Routing](./smart-routing.md)
- [DOSRouter x402 Micropayments](./x402-payments.md)
