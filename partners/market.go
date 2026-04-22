// Self-hosted market data handlers backed by Pyth Network.
//
// These handlers serve the /v1/stocks/*, /v1/crypto/*, /v1/fx/*, and
// /v1/commodity/* partner routes registered by DOSRouter's proxy. They
// translate the DOS/BlockRun-style REST API into Pyth Hermes queries
// and return JSON that matches the upstream partner tool schema.

package partners

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// MarketHandler hosts the 6 self-served market data endpoints.
type MarketHandler struct {
	pyth *PythClient
}

// NewMarketHandler returns a handler with a fresh Pyth client.
func NewMarketHandler() *MarketHandler {
	return &MarketHandler{pyth: NewPythClient()}
}

// Routes registers the six market-data endpoints on the given mux.
// Callers should mount this after the LLM and images routes.
func (h *MarketHandler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/stocks/", h.handleStocks)
	mux.HandleFunc("/v1/crypto/price/", h.handleCryptoPrice)
	mux.HandleFunc("/v1/fx/price/", h.handleFxPrice)
	mux.HandleFunc("/v1/commodity/price/", h.handleCommodityPrice)
}

// --- /v1/stocks/{market}/{op}/{symbol?} dispatcher -------------------------

func (h *MarketHandler) handleStocks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// Expected path shapes:
	//   /v1/stocks/{market}/price/{symbol}
	//   /v1/stocks/{market}/history/{symbol}
	//   /v1/stocks/{market}/list
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/stocks/"), "/")
	if len(parts) < 2 {
		writeJSONError(w, http.StatusNotFound, "unknown stocks endpoint")
		return
	}
	market := strings.ToLower(parts[0])
	op := parts[1]
	switch op {
	case "price":
		if len(parts) < 3 {
			writeJSONError(w, http.StatusBadRequest, "missing symbol")
			return
		}
		h.stockPrice(w, r, market, parts[2])
	case "history":
		if len(parts) < 3 {
			writeJSONError(w, http.StatusBadRequest, "missing symbol")
			return
		}
		h.stockHistory(w, r, market, parts[2])
	case "list":
		h.stockList(w, r, market)
	default:
		writeJSONError(w, http.StatusNotFound, "unknown stocks operation: "+op)
	}
}

// --- Stock price -----------------------------------------------------------

type stockPriceResponse struct {
	Market      string  `json:"market"`
	Symbol      string  `json:"symbol"`
	FeedID      string  `json:"feed_id"`
	Price       float64 `json:"price"`
	Confidence  float64 `json:"confidence"`
	PublishTime int64   `json:"publish_time"`
	Source      string  `json:"source"`
}

func (h *MarketHandler) stockPrice(w http.ResponseWriter, r *http.Request, market, symbol string) {
	base := strings.ToUpper(stripMarketSuffix(symbol))
	feeds, err := h.pyth.SearchFeeds(PythEquity, base)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	feed := ResolveSymbol(feeds, base, market)
	if feed == nil {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("no equity feed for %s/%s", market, symbol))
		return
	}
	prices, err := h.pyth.LatestPrice([]string{feed.ID})
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(prices) == 0 {
		writeJSONError(w, http.StatusBadGateway, "no price returned for feed "+feed.ID)
		return
	}
	p := prices[0]
	writeJSON(w, http.StatusOK, stockPriceResponse{
		Market:      market,
		Symbol:      symbol,
		FeedID:      feed.ID,
		Price:       p.FormattedPrice(),
		Confidence:  p.FormattedConfidence(),
		PublishTime: p.PublishTime,
		Source:      "pyth-hermes",
	})
}

// --- Stock history ---------------------------------------------------------

func (h *MarketHandler) stockHistory(w http.ResponseWriter, r *http.Request, market, symbol string) {
	resolution := r.URL.Query().Get("resolution")
	if resolution == "" {
		resolution = "D"
	}
	fromStr := r.URL.Query().Get("from")
	if fromStr == "" {
		writeJSONError(w, http.StatusBadRequest, "from is required")
		return
	}
	from, err := strconv.ParseInt(fromStr, 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid from: "+err.Error())
		return
	}
	to := time.Now().Unix()
	if s := r.URL.Query().Get("to"); s != "" {
		parsed, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid to: "+err.Error())
			return
		}
		to = parsed
	}

	base := strings.ToUpper(stripMarketSuffix(symbol))
	feeds, err := h.pyth.SearchFeeds(PythEquity, base)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	feed := ResolveSymbol(feeds, base, market)
	if feed == nil {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("no equity feed for %s/%s", market, symbol))
		return
	}
	bars, err := h.pyth.HistoryBars(feed.Attributes["symbol"], resolution, from, to)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"market":     market,
		"symbol":     symbol,
		"feed_id":    feed.ID,
		"resolution": resolution,
		"bars":       bars,
		"source":     "pyth-benchmarks",
	})
}

// --- Stock list ------------------------------------------------------------

type stockListEntry struct {
	Symbol      string `json:"symbol"`
	Description string `json:"description"`
	FeedID      string `json:"feed_id"`
	PythSymbol  string `json:"pyth_symbol"`
}

func (h *MarketHandler) stockList(w http.ResponseWriter, r *http.Request, market string) {
	query := r.URL.Query().Get("q")
	limit := 100
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			if n > 2000 {
				n = 2000
			}
			limit = n
		}
	}

	feeds, err := h.pyth.SearchFeeds(PythEquity, query)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	countryCode := strings.ToUpper(market)
	results := make([]stockListEntry, 0, limit)
	for i := range feeds {
		f := feeds[i]
		if countryCode != "" && strings.ToUpper(f.Attributes["country"]) != countryCode {
			continue
		}
		sym := f.Attributes["symbol"]
		// Skip session-variant feeds; surface only the canonical ticker.
		if strings.HasSuffix(sym, ".PRE") || strings.HasSuffix(sym, ".POST") || strings.HasSuffix(sym, ".ON") {
			continue
		}
		results = append(results, stockListEntry{
			Symbol:      f.Attributes["base"],
			Description: f.Attributes["description"],
			FeedID:      f.ID,
			PythSymbol:  sym,
		})
		if len(results) >= limit {
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"market":  market,
		"count":   len(results),
		"results": results,
		"source":  "pyth-hermes",
	})
}

// --- Crypto / FX / Commodity ----------------------------------------------

func (h *MarketHandler) handleCryptoPrice(w http.ResponseWriter, r *http.Request) {
	h.priceBySymbol(w, r, PythCrypto, "/v1/crypto/price/")
}

func (h *MarketHandler) handleFxPrice(w http.ResponseWriter, r *http.Request) {
	h.priceBySymbol(w, r, PythFX, "/v1/fx/price/")
}

func (h *MarketHandler) handleCommodityPrice(w http.ResponseWriter, r *http.Request) {
	h.priceBySymbol(w, r, PythMetal, "/v1/commodity/price/")
}

type genericPriceResponse struct {
	Symbol      string  `json:"symbol"`
	FeedID      string  `json:"feed_id"`
	PythSymbol  string  `json:"pyth_symbol"`
	Price       float64 `json:"price"`
	Confidence  float64 `json:"confidence"`
	PublishTime int64   `json:"publish_time"`
	Source      string  `json:"source"`
}

func (h *MarketHandler) priceBySymbol(w http.ResponseWriter, r *http.Request, assetType PythAssetType, prefix string) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	rawSymbol := strings.TrimPrefix(r.URL.Path, prefix)
	if rawSymbol == "" {
		writeJSONError(w, http.StatusBadRequest, "missing symbol")
		return
	}
	base, quote := splitPair(rawSymbol)
	if base == "" || quote == "" {
		writeJSONError(w, http.StatusBadRequest, "symbol must be BASE-QUOTE (e.g. BTC-USD)")
		return
	}

	feeds, err := h.pyth.SearchFeeds(assetType, base)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	// Pick the feed whose quote matches.
	var target *PythFeed
	for i := range feeds {
		f := &feeds[i]
		if strings.EqualFold(f.Attributes["base"], base) && strings.EqualFold(f.Attributes["quote_currency"], quote) {
			target = f
			break
		}
	}
	if target == nil {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("no %s feed for %s/%s", assetType, base, quote))
		return
	}
	prices, err := h.pyth.LatestPrice([]string{target.ID})
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(prices) == 0 {
		writeJSONError(w, http.StatusBadGateway, "no price returned for feed "+target.ID)
		return
	}
	p := prices[0]
	writeJSON(w, http.StatusOK, genericPriceResponse{
		Symbol:      strings.ToUpper(base + "-" + quote),
		FeedID:      target.ID,
		PythSymbol:  target.Attributes["symbol"],
		Price:       p.FormattedPrice(),
		Confidence:  p.FormattedConfidence(),
		PublishTime: p.PublishTime,
		Source:      "pyth-hermes",
	})
}

// --- helpers ---------------------------------------------------------------

// stripMarketSuffix removes trailing "-XX" country suffixes that BlockRun's
// API uses for non-US tickers (e.g. "0700-HK" → "0700").
func stripMarketSuffix(symbol string) string {
	if i := strings.LastIndex(symbol, "-"); i > 0 {
		suffix := symbol[i+1:]
		if len(suffix) == 2 {
			return symbol[:i]
		}
	}
	return symbol
}

func splitPair(s string) (base, quote string) {
	s = strings.ToUpper(s)
	if i := strings.Index(s, "-"); i > 0 {
		return s[:i], s[i+1:]
	}
	if i := strings.Index(s, "/"); i > 0 {
		return s[:i], s[i+1:]
	}
	return "", ""
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

