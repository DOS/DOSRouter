# Market Data Partner Tools

DOSRouter self-hosts six realtime market data endpoints backed by
[Pyth Network's public Hermes feed](https://hermes.pyth.network). They are
available to agents as partner tools and as direct HTTP endpoints on the
local proxy. No API key or x402 payment is required — data is served
in-process from Pyth's public feed.

## Endpoints

All paths are relative to the proxy base URL (default
`http://127.0.0.1:8402`). All requests use `GET` and return JSON.

| Path | Purpose |
|------|---------|
| `/v1/stocks/{market}/price/{symbol}` | Realtime equity price |
| `/v1/stocks/{market}/history/{symbol}` | Historical OHLC bars |
| `/v1/stocks/{market}/list` | Search/list supported tickers |
| `/v1/crypto/price/{symbol}` | Realtime crypto spot price |
| `/v1/fx/price/{symbol}` | Realtime FX rate |
| `/v1/commodity/price/{symbol}` | Realtime commodity spot price |

### Symbol format

- **Stocks** — ticker only (`AAPL`, `TSLA`). Non-US markets may use the
  BlockRun-style suffix (`0700-HK`); DOSRouter strips the 2-letter
  suffix before calling Pyth.
- **Crypto / FX / Metal** — `BASE-QUOTE` (e.g. `BTC-USD`, `EUR-USD`,
  `XAU-USD`). Slashes are also accepted (`BTC/USD`).

### Market codes

`us`, `hk`, `jp`, `kr`, `gb`, `de`, `fr`, `nl`, `ie`, `lu`, `cn`, `ca` —
matched against the Pyth feed's `country` attribute.

### Stock history

Query params:

| Param | Required | Notes |
|-------|----------|-------|
| `resolution` | no | `1`, `5`, `15`, `60`, `240` (minutes) or `D`, `W`, `M`. Default `D`. |
| `from` | yes | Unix epoch seconds. |
| `to` | no | Unix epoch seconds. Defaults to now. |

Bars follow TradingView's UDF shape: `{s, t[], o[], h[], l[], c[], v[]}`.

## Coverage

Pyth Network provides strong coverage for:

- US equities (large/mid caps, ~500 tickers)
- Major crypto pairs (BTC, ETH, SOL, DOGE, and hundreds more)
- Major FX pairs
- Metals (XAU, XAG, XPT, XPD)

**Limited coverage:** Non-US stock markets (HK, JP, KR, EU exchanges)
have partial Pyth feeds — mostly ETFs and large caps. A request to a
ticker that Pyth does not index returns `HTTP 404`.

## Pricing

All six tools are free to DOSRouter users in this release. Pyth Hermes
itself is free to query and imposes no formal rate limit at the time of
writing, though clients should be courteous (cache where possible).

If DOS Chain deploys USDC and a facilitator, future releases may add an
x402 fee layer in front of these endpoints — configurable per DOS
deployment, independent of Pyth's upstream.

## Roadmap — deferred tools

The following upstream partners from ClawRouter v0.12.159 are **not**
ported in this release and require additional work:

| Tool | Blocker |
|------|---------|
| `predexon_smart_activity` | Needs on-chain Polymarket indexer or Predexon vendor contract |
| `predexon_wallet_pnl` | Same — Predexon aggregates on-chain P&L |
| `predexon_matching_markets` | Needs Polymarket + Kalshi cross-market matching engine |
| `x_users_lookup` | Needs Twitter/X API v2 paid tier |

These can be added later as separate services; none of them block the
market-data tools above.
