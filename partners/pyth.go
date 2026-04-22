package partners

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	hermesBaseURL     = "https://hermes.pyth.network"
	benchmarksBaseURL = "https://benchmarks.pyth.network"
	pythRequestTimeout = 8 * time.Second
)

// PythAssetType is the set of asset categories supported by Pyth Hermes.
type PythAssetType string

const (
	PythEquity   PythAssetType = "equity"
	PythCrypto   PythAssetType = "crypto"
	PythFX       PythAssetType = "fx"
	PythMetal    PythAssetType = "metal"
)

// PythFeed is a subset of the Hermes /v2/price_feeds response entry.
type PythFeed struct {
	ID         string            `json:"id"`
	Attributes map[string]string `json:"attributes"`
}

// PythLatestPrice is the parsed payload for a single feed from
// /v2/updates/price/latest?parsed=true. Price is price * 10^Expo.
type PythLatestPrice struct {
	ID          string
	Price       *big.Int
	Confidence  *big.Int
	Expo        int
	PublishTime int64
}

// FormattedPrice returns a float64 suitable for display. Precision is
// limited by float64; callers needing full precision should use the raw
// Price + Expo fields.
func (p PythLatestPrice) FormattedPrice() float64 {
	if p.Price == nil {
		return 0
	}
	f, _ := new(big.Float).SetInt(p.Price).Float64()
	return f * pow10(p.Expo)
}

// FormattedConfidence returns the confidence interval as a float64.
func (p PythLatestPrice) FormattedConfidence() float64 {
	if p.Confidence == nil {
		return 0
	}
	f, _ := new(big.Float).SetInt(p.Confidence).Float64()
	return f * pow10(p.Expo)
}

func pow10(e int) float64 {
	if e == 0 {
		return 1
	}
	r := 1.0
	if e > 0 {
		for i := 0; i < e; i++ {
			r *= 10
		}
	} else {
		for i := 0; i < -e; i++ {
			r /= 10
		}
	}
	return r
}

// PythClient queries the public Hermes and Benchmarks endpoints.
type PythClient struct {
	httpClient *http.Client
}

// NewPythClient constructs a client with sensible defaults.
func NewPythClient() *PythClient {
	return &PythClient{httpClient: &http.Client{Timeout: pythRequestTimeout}}
}

// SearchFeeds calls /v2/price_feeds with the given asset type and query.
// Returns the full list from Hermes (Hermes does not paginate this endpoint).
func (c *PythClient) SearchFeeds(assetType PythAssetType, query string) ([]PythFeed, error) {
	u, _ := url.Parse(hermesBaseURL + "/v2/price_feeds")
	q := u.Query()
	q.Set("asset_type", string(assetType))
	if query != "" {
		q.Set("query", query)
	}
	u.RawQuery = q.Encode()

	resp, err := c.httpClient.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("pyth: search feeds: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("pyth: read search response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("pyth: search returned %d: %s", resp.StatusCode, string(body))
	}

	type wire struct {
		ID         string            `json:"id"`
		Attributes map[string]string `json:"attributes"`
	}
	var raw []wire
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("pyth: decode search response: %w", err)
	}
	out := make([]PythFeed, 0, len(raw))
	for _, r := range raw {
		out = append(out, PythFeed{ID: r.ID, Attributes: r.Attributes})
	}
	return out, nil
}

// LatestPrice fetches the most recent price for the given feed IDs.
func (c *PythClient) LatestPrice(feedIDs []string) ([]PythLatestPrice, error) {
	if len(feedIDs) == 0 {
		return nil, nil
	}
	u, _ := url.Parse(hermesBaseURL + "/v2/updates/price/latest")
	q := u.Query()
	for _, id := range feedIDs {
		q.Add("ids[]", id)
	}
	q.Set("parsed", "true")
	q.Set("encoding", "hex")
	u.RawQuery = q.Encode()

	resp, err := c.httpClient.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("pyth: latest price: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("pyth: read latest price: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("pyth: latest price returned %d: %s", resp.StatusCode, string(body))
	}

	type priceWire struct {
		Price       string `json:"price"`
		Conf        string `json:"conf"`
		Expo        int    `json:"expo"`
		PublishTime int64  `json:"publish_time"`
	}
	type parsedEntry struct {
		ID    string    `json:"id"`
		Price priceWire `json:"price"`
	}
	type responseWire struct {
		Parsed []parsedEntry `json:"parsed"`
	}
	var r responseWire
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("pyth: decode latest price: %w", err)
	}
	out := make([]PythLatestPrice, 0, len(r.Parsed))
	for _, p := range r.Parsed {
		price, _ := new(big.Int).SetString(p.Price.Price, 10)
		conf, _ := new(big.Int).SetString(p.Price.Conf, 10)
		out = append(out, PythLatestPrice{
			ID:          p.ID,
			Price:       price,
			Confidence:  conf,
			Expo:        p.Price.Expo,
			PublishTime: p.Price.PublishTime,
		})
	}
	return out, nil
}

// Bars is a TradingView UDF-style response from the Benchmarks shim.
type Bars struct {
	Status     string    `json:"s"`
	ErrMessage string    `json:"errmsg,omitempty"`
	Times      []int64   `json:"t,omitempty"`
	Opens      []float64 `json:"o,omitempty"`
	Highs      []float64 `json:"h,omitempty"`
	Lows       []float64 `json:"l,omitempty"`
	Closes     []float64 `json:"c,omitempty"`
	Volumes    []float64 `json:"v,omitempty"`
}

// HistoryBars queries the Benchmarks TradingView shim for historical OHLC.
// pythSymbol must be the full Pyth symbol (e.g. "Crypto.BTC/USD",
// "Equity.US.AAPL/USD"). Resolution uses TradingView conventions:
// "1","5","15","60","240","D","W","M".
func (c *PythClient) HistoryBars(pythSymbol, resolution string, from, to int64) (*Bars, error) {
	u, _ := url.Parse(benchmarksBaseURL + "/v1/shims/tradingview/history")
	q := u.Query()
	q.Set("symbol", pythSymbol)
	q.Set("resolution", resolution)
	q.Set("from", strconv.FormatInt(from, 10))
	q.Set("to", strconv.FormatInt(to, 10))
	u.RawQuery = q.Encode()

	resp, err := c.httpClient.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("pyth: history bars: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("pyth: read history bars: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("pyth: history bars returned %d: %s", resp.StatusCode, string(body))
	}
	var b Bars
	if err := json.Unmarshal(body, &b); err != nil {
		return nil, fmt.Errorf("pyth: decode history bars: %w", err)
	}
	if b.Status != "" && b.Status != "ok" && b.Status != "no_data" {
		return &b, fmt.Errorf("pyth: history bars status %q: %s", b.Status, b.ErrMessage)
	}
	return &b, nil
}

// ResolveSymbol picks the canonical (regular-hours) feed from a search result
// set, filtering by market country if provided. Equity feeds have four
// variants per symbol (regular, PRE, POST, ON); the canonical feed has no
// session suffix in its Pyth symbol.
func ResolveSymbol(feeds []PythFeed, base, market string) *PythFeed {
	countryCode := strings.ToUpper(market)
	var best *PythFeed
	for i := range feeds {
		f := &feeds[i]
		if strings.ToUpper(f.Attributes["base"]) != strings.ToUpper(base) {
			continue
		}
		if countryCode != "" {
			if strings.ToUpper(f.Attributes["country"]) != countryCode {
				continue
			}
		}
		sym := f.Attributes["symbol"]
		// Prefer symbols without session suffix (.PRE/.POST/.ON).
		if !strings.HasSuffix(sym, ".PRE") && !strings.HasSuffix(sym, ".POST") && !strings.HasSuffix(sym, ".ON") {
			return f
		}
		if best == nil {
			best = f
		}
	}
	return best
}
