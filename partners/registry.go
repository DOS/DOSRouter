// Package partners provides partner service definitions and tool builders
// for integrating external data APIs into the DOSRouter system.
package partners

// ParamType enumerates the allowed types for a service parameter.
type ParamType string

const (
	ParamTypeString  ParamType = "string"
	ParamTypeNumber  ParamType = "number"
	ParamTypeBoolean ParamType = "boolean"
)

// PartnerServiceParam describes a single parameter for a partner service.
type PartnerServiceParam struct {
	Name        string
	Type        ParamType
	Required    bool
	Description string
	Default     any // nil means no default
}

// HTTPMethod is the HTTP method used to call the partner service.
type HTTPMethod string

const (
	MethodGET  HTTPMethod = "GET"
	MethodPOST HTTPMethod = "POST"
)

// PartnerServiceDefinition describes a partner API endpoint that can be
// exposed as an agent tool.
//
// A service is routed in one of two modes:
//   - Absolute mode: BaseURL is a full external URL (e.g. api.predexon.com).
//     The tool calls the upstream directly with an API key.
//   - Local-proxy mode: ProxyPath is a path relative to the DOSRouter proxy
//     (e.g. "/v1/crypto/price/:symbol"). The tool calls the local proxy,
//     which serves the endpoint in-process (self-hosted data feeds).
//     If ProxyPath is set, BaseURL is ignored.
type PartnerServiceDefinition struct {
	ID          string
	Name        string
	Description string
	BaseURL     string
	ProxyPath   string // If set, route via DOSRouter local proxy (takes precedence over BaseURL).
	Method      HTTPMethod
	Params      []PartnerServiceParam
	Headers     map[string]string
	ResultPath  string // JSONPath-like dot-separated path to extract from response
	FormatHint  string // hint for how an LLM should present the result
}

// PartnerServices is the canonical list of all registered partner services.
var PartnerServices = []PartnerServiceDefinition{
	{
		ID:          "x_users_lookup",
		Name:        "Twitter/X User Lookup",
		Description: "Look up Twitter/X user profiles by username. Returns user data including follower counts, bios, and verification status.",
		BaseURL:     "https://api.predexon.com/data/x/users",
		Method:      MethodGET,
		Params: []PartnerServiceParam{
			{
				Name:        "usernames",
				Type:        ParamTypeString,
				Required:    true,
				Description: "Comma-separated list of Twitter/X usernames to look up (without @ symbol)",
			},
		},
		ResultPath: "data",
		FormatHint: "Present each user with their display name, username, follower/following counts, bio, and verification status. Format large follower counts with commas.",
	},
	{
		ID:          "polymarket_events",
		Name:        "Polymarket Events",
		Description: "Get current prediction market events from Polymarket, including titles, descriptions, and market data.",
		BaseURL:     "https://api.predexon.com/data/polymarket/events",
		Method:      MethodGET,
		Params: []PartnerServiceParam{
			{
				Name:        "limit",
				Type:        ParamTypeNumber,
				Required:    false,
				Description: "Maximum number of events to return",
				Default:     10,
			},
			{
				Name:        "offset",
				Type:        ParamTypeNumber,
				Required:    false,
				Description: "Number of events to skip for pagination",
				Default:     0,
			},
		},
	},
	{
		ID:          "polymarket_leaderboard",
		Name:        "Polymarket Leaderboard",
		Description: "Get the Polymarket trader leaderboard rankings by profit and volume.",
		BaseURL:     "https://api.predexon.com/data/polymarket/leaderboard",
		Method:      MethodGET,
		Params: []PartnerServiceParam{
			{
				Name:        "period",
				Type:        ParamTypeString,
				Required:    false,
				Description: "Time period for the leaderboard (e.g. \"all\", \"daily\", \"weekly\", \"monthly\")",
				Default:     "all",
			},
			{
				Name:        "limit",
				Type:        ParamTypeNumber,
				Required:    false,
				Description: "Maximum number of entries to return",
				Default:     10,
			},
		},
	},
	{
		ID:          "polymarket_markets",
		Name:        "Polymarket Markets",
		Description: "Search and list prediction markets on Polymarket with current odds and volume.",
		BaseURL:     "https://api.predexon.com/data/polymarket/markets",
		Method:      MethodGET,
		Params: []PartnerServiceParam{
			{
				Name:        "limit",
				Type:        ParamTypeNumber,
				Required:    false,
				Description: "Maximum number of markets to return",
				Default:     10,
			},
			{
				Name:        "query",
				Type:        ParamTypeString,
				Required:    false,
				Description: "Search query to filter markets by keyword",
			},
		},
	},
	{
		ID:          "polymarket_smart_money",
		Name:        "Polymarket Smart Money",
		Description: "Track smart money movements and whale activity on Polymarket prediction markets.",
		BaseURL:     "https://api.predexon.com/data/polymarket/smart-money",
		Method:      MethodGET,
		Params: []PartnerServiceParam{
			{
				Name:        "limit",
				Type:        ParamTypeNumber,
				Required:    false,
				Description: "Maximum number of entries to return",
				Default:     10,
			},
		},
	},
	{
		ID:          "polymarket_wallet",
		Name:        "Polymarket Wallet",
		Description: "Get detailed portfolio and trading history for a specific Polymarket wallet address.",
		BaseURL:     "https://api.predexon.com/data/polymarket/wallet/{address}",
		Method:      MethodGET,
		Params: []PartnerServiceParam{
			{
				Name:        "address",
				Type:        ParamTypeString,
				Required:    true,
				Description: "Polymarket wallet address to look up",
			},
		},
	},

	// -------------------------------------------------------------------------
	// DOS Market Data (upstream v0.12.159, self-hosted via Pyth Network Hermes)
	// -------------------------------------------------------------------------
	// These 6 tools are served in-process by DOSRouter's proxy, backed by
	// Pyth Network's public Hermes feed (hermes.pyth.network). No external
	// API key required, no x402 payment at this layer — free to agent users.
	{
		ID:   "stock_price",
		Name: "Global Stock Realtime Price",
		Description: "Get realtime price for a listed equity. " +
			"Call this for ANY request about a specific stock price, quote, or current trading value. " +
			"Coverage: US large/mid caps via Pyth Network. Non-US markets (HK/JP/KR/EU) have limited coverage in this release. " +
			"Returns: symbol, price, confidence interval, publish time, feed ID.",
		ProxyPath: "/v1/stocks/{market}/price/{symbol}",
		Method:    MethodGET,
		Params: []PartnerServiceParam{
			{Name: "market", Type: ParamTypeString, Required: true, Description: "Market code (lowercase): us, hk, jp, kr, gb, de, fr, nl, ie, lu, cn, ca."},
			{Name: "symbol", Type: ParamTypeString, Required: true, Description: "Ticker for the given market (e.g. AAPL for us, 0700-HK for hk)."},
			{Name: "session", Type: ParamTypeString, Required: false, Description: "Optional session hint: pre, post, or on (regular hours)."},
		},
	},
	{
		ID:   "stock_history",
		Name: "Global Stock OHLC History",
		Description: "Get historical OHLC (candlestick) bars for a listed equity. " +
			"Use this for charting, backtesting, or any request about a stock's past price action. " +
			"Resolutions: 1, 5, 15, 60, 240 (minutes) and D, W, M (daily/weekly/monthly). " +
			"Returns: OHLC arrays (open, high, low, close, volume, timestamps).",
		ProxyPath: "/v1/stocks/{market}/history/{symbol}",
		Method:    MethodGET,
		Params: []PartnerServiceParam{
			{Name: "market", Type: ParamTypeString, Required: true, Description: "Market code: us, hk, jp, kr, gb, de, fr, nl, ie, lu, cn, ca."},
			{Name: "symbol", Type: ParamTypeString, Required: true, Description: "Ticker for the given market."},
			{Name: "resolution", Type: ParamTypeString, Required: false, Description: "Bar resolution: 1, 5, 15, 60, 240 (minutes) or D, W, M. Default: D.", Default: "D"},
			{Name: "from", Type: ParamTypeNumber, Required: true, Description: "Start time as Unix epoch seconds."},
			{Name: "to", Type: ParamTypeNumber, Required: false, Description: "End time as Unix epoch seconds. Default: now."},
		},
	},
	{
		ID:   "stock_list",
		Name: "Global Stock Ticker List",
		Description: "List and search supported tickers for a given stock market. " +
			"Use this to resolve a company name to a ticker before calling stock_price or stock_history.",
		ProxyPath: "/v1/stocks/{market}/list",
		Method:    MethodGET,
		Params: []PartnerServiceParam{
			{Name: "market", Type: ParamTypeString, Required: true, Description: "Market code: us, hk, jp, kr, gb, de, fr, nl, ie, lu, cn, ca."},
			{Name: "q", Type: ParamTypeString, Required: false, Description: "Optional search query to filter tickers by symbol or description."},
			{Name: "limit", Type: ParamTypeNumber, Required: false, Description: "Max results to return (default: 100, max: 2000).", Default: 100},
		},
	},
	{
		ID:   "crypto_price",
		Name: "Crypto Realtime Price",
		Description: "Get realtime crypto price from Pyth Network. Quote is always USD. " +
			"Call this for ANY request about current crypto prices (BTC, ETH, SOL, etc.).",
		ProxyPath: "/v1/crypto/price/{symbol}",
		Method:    MethodGET,
		Params: []PartnerServiceParam{
			{Name: "symbol", Type: ParamTypeString, Required: true, Description: "Crypto pair in BASE-QUOTE form. Examples: BTC-USD, ETH-USD, SOL-USD."},
		},
	},
	{
		ID:   "fx_price",
		Name: "Foreign Exchange Realtime Price",
		Description: "Get realtime FX rate from Pyth Network. Call this for ANY request about currency exchange rates.",
		ProxyPath: "/v1/fx/price/{symbol}",
		Method:    MethodGET,
		Params: []PartnerServiceParam{
			{Name: "symbol", Type: ParamTypeString, Required: true, Description: "Currency pair in BASE-QUOTE form. Examples: EUR-USD, GBP-USD, JPY-USD."},
		},
	},
	{
		ID:   "commodity_price",
		Name: "Commodity Realtime Price",
		Description: "Get realtime commodity spot price from Pyth Network (gold, silver, platinum, etc.).",
		ProxyPath: "/v1/commodity/price/{symbol}",
		Method:    MethodGET,
		Params: []PartnerServiceParam{
			{Name: "symbol", Type: ParamTypeString, Required: true, Description: "Commodity code in BASE-USD form. Examples: XAU-USD (gold), XAG-USD (silver), XPT-USD (platinum)."},
		},
	},
}

// GetPartnerService returns the service definition for the given ID, or nil
// if no service matches.
func GetPartnerService(id string) *PartnerServiceDefinition {
	for i := range PartnerServices {
		if PartnerServices[i].ID == id {
			return &PartnerServices[i]
		}
	}
	return nil
}
