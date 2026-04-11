// Package payment implements the x402 micropayment protocol for DOSRouter.
// x402 enables pay-per-request LLM access without API keys or accounts.
package payment

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/DOS/DOSRouter/wallet"
)

// PaymentRequired represents a 402 response from the upstream server.
type PaymentRequired struct {
	Amount       float64 `json:"amount"`
	Currency     string  `json:"currency"`
	PayTo        string  `json:"payTo"`
	Network      string  `json:"network"`
	Description  string  `json:"description"`
	ExpiresAt    string  `json:"expiresAt,omitempty"`
	PaymentToken string  `json:"paymentToken,omitempty"` // Token to include after payment
}

// PaymentResult is the outcome of a payment attempt.
type PaymentResult struct {
	Success bool    `json:"success"`
	TxHash  string  `json:"txHash,omitempty"`
	Cost    float64 `json:"cost"`
	Error   string  `json:"error,omitempty"`
}

// Client handles x402 payment flows.
type Client struct {
	wallet     *wallet.Wallet
	httpClient *http.Client
	preAuth    *preAuthCache
}

// New creates a new payment client.
func New(w *wallet.Wallet) *Client {
	return &Client{
		wallet:     w,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		preAuth:    newPreAuthCache(),
	}
}

// WrapRequest adds payment headers to an outgoing request if pre-auth is cached.
// Returns true if payment headers were added.
func (c *Client) WrapRequest(req *http.Request, model string) bool {
	cacheKey := req.URL.Path + ":" + model
	if cached := c.preAuth.get(cacheKey); cached != nil {
		req.Header.Set("X-Payment-Token", cached.PaymentToken)
		return true
	}
	return false
}

// HandlePaymentRequired processes a 402 response by signing and submitting payment.
func (c *Client) HandlePaymentRequired(resp *http.Response, originalReq *http.Request, model string) (*PaymentResult, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading 402 response: %w", err)
	}

	var pr PaymentRequired
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, fmt.Errorf("parsing 402 response: %w", err)
	}

	// Sign and submit the payment
	result, err := c.submitPayment(pr)
	if err != nil {
		return nil, err
	}

	// Cache the payment token for future requests
	if result.Success && pr.PaymentToken != "" {
		cacheKey := originalReq.URL.Path + ":" + model
		c.preAuth.set(cacheKey, &pr)
	}

	return result, nil
}

// IsPaymentRequired checks if a response is a 402 payment required.
func IsPaymentRequired(resp *http.Response) bool {
	return resp.StatusCode == http.StatusPaymentRequired
}

// submitPayment signs a payment transaction and submits it.
func (c *Client) submitPayment(pr PaymentRequired) (*PaymentResult, error) {
	// Build payment message
	// In production, this would use proper EVM transaction signing
	// via go-ethereum's crypto.Sign with the wallet's private key
	cc := c.wallet.ChainConfig()

	return &PaymentResult{
		Success: false,
		Cost:    pr.Amount,
		Error: fmt.Sprintf(
			"x402 payment signing not yet implemented for %s (chain_id=%d). "+
				"Amount: $%.6f to %s. "+
				"Install go-ethereum for full EVM transaction support.",
			cc.Name, cc.ChainID, pr.Amount, pr.PayTo,
		),
	}, nil
}

// --- Pre-auth cache ---

type preAuthCache struct {
	mu      sync.RWMutex
	entries map[string]*preAuthEntry
}

type preAuthEntry struct {
	pr        *PaymentRequired
	timestamp time.Time
}

func newPreAuthCache() *preAuthCache {
	return &preAuthCache{entries: make(map[string]*preAuthEntry)}
}

func (c *preAuthCache) get(key string) *PaymentRequired {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok || time.Since(entry.timestamp) > 5*time.Minute {
		return nil
	}
	return entry.pr
}

func (c *preAuthCache) set(key string, pr *PaymentRequired) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = &preAuthEntry{pr: pr, timestamp: time.Now()}
}

// FormatPaymentInfo returns a human-readable payment summary.
func FormatPaymentInfo(pr PaymentRequired) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Payment Required: $%.6f %s\n", pr.Amount, pr.Currency))
	b.WriteString(fmt.Sprintf("  Pay to:  %s\n", pr.PayTo))
	b.WriteString(fmt.Sprintf("  Network: %s\n", pr.Network))
	if pr.Description != "" {
		b.WriteString(fmt.Sprintf("  Reason:  %s\n", pr.Description))
	}
	return b.String()
}
