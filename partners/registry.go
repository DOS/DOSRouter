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
type PartnerServiceDefinition struct {
	ID          string
	Name        string
	Description string
	BaseURL     string
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
