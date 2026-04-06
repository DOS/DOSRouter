// Package retry provides HTTP request execution with exponential backoff.
// It retries on transient errors (429, 502, 503, 504) and respects the
// Retry-After response header.
package retry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultBaseDelay is the initial backoff delay.
	DefaultBaseDelay = 500 * time.Millisecond
	// DefaultMaxRetries is the maximum number of retry attempts.
	DefaultMaxRetries = 2
)

// retryableStatusCodes are HTTP status codes that trigger a retry.
var retryableStatusCodes = map[int]bool{
	http.StatusTooManyRequests:     true, // 429
	http.StatusBadGateway:         true, // 502
	http.StatusServiceUnavailable: true, // 503
	http.StatusGatewayTimeout:     true, // 504
}

// Config controls retry behavior.
type Config struct {
	// BaseDelay is the initial backoff delay (default 500ms).
	BaseDelay time.Duration
	// MaxRetries is the maximum number of retry attempts (default 2).
	MaxRetries int
	// Client is the HTTP client to use. If nil, http.DefaultClient is used.
	Client *http.Client
}

// Option configures retry behavior.
type Option func(*Config)

// WithBaseDelay sets the initial backoff delay.
func WithBaseDelay(d time.Duration) Option {
	return func(c *Config) { c.BaseDelay = d }
}

// WithMaxRetries sets the maximum number of retries.
func WithMaxRetries(n int) Option {
	return func(c *Config) { c.MaxRetries = n }
}

// WithClient sets the HTTP client.
func WithClient(cl *http.Client) Option {
	return func(c *Config) { c.Client = cl }
}

func defaultConfig() Config {
	return Config{
		BaseDelay:  DefaultBaseDelay,
		MaxRetries: DefaultMaxRetries,
	}
}

// Do sends an HTTP request with retry logic. The buildReq function is called
// before each attempt to produce a fresh *http.Request (since request bodies
// are consumed on send).
//
// It retries on network errors and retryable HTTP status codes (429, 502, 503,
// 504), using exponential backoff. If the response includes a Retry-After
// header, that value is used instead of the computed backoff.
func Do(ctx context.Context, buildReq func() (*http.Request, error), opts ...Option) (*http.Response, error) {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}
	client := cfg.Client
	if client == nil {
		client = http.DefaultClient
	}

	var lastErr error
	var lastResp *http.Response

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		req, err := buildReq()
		if err != nil {
			return nil, fmt.Errorf("retry: build request: %w", err)
		}
		req = req.WithContext(ctx)

		resp, err := client.Do(req)
		if err != nil {
			if !IsRetryableError(err) {
				return nil, err
			}
			lastErr = err
			if attempt < cfg.MaxRetries {
				if sleepErr := backoff(ctx, cfg.BaseDelay, attempt, nil); sleepErr != nil {
					return nil, sleepErr
				}
			}
			continue
		}

		if !IsRetryableStatus(resp.StatusCode) {
			return resp, nil
		}

		// Retryable status code - drain and close body before retry.
		lastResp = resp
		lastErr = fmt.Errorf("retry: HTTP %d", resp.StatusCode)

		if attempt < cfg.MaxRetries {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			// Drain body so the connection can be reused.
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if sleepErr := backoff(ctx, cfg.BaseDelay, attempt, retryAfter); sleepErr != nil {
				return nil, sleepErr
			}
		}
	}

	// Exhausted retries. Return the last response if available so the caller
	// can inspect the status code and body.
	if lastResp != nil {
		return lastResp, lastErr
	}
	return nil, lastErr
}

// IsRetryableError reports whether an error is transient and worth retrying
// (timeouts, connection resets, DNS failures).
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Context cancellation is not retryable.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Network errors (timeout, connection refused/reset, DNS).
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Check for common transient error strings as a fallback.
	msg := err.Error()
	transient := []string{"connection reset", "connection refused", "broken pipe", "EOF"}
	for _, t := range transient {
		if strings.Contains(msg, t) {
			return true
		}
	}

	return false
}

// IsRetryableStatus reports whether an HTTP status code is retryable.
func IsRetryableStatus(code int) bool {
	return retryableStatusCodes[code]
}

// backoff sleeps for the exponential backoff duration or the Retry-After
// duration, whichever is larger. Returns an error if the context is cancelled.
func backoff(ctx context.Context, base time.Duration, attempt int, retryAfter *time.Duration) error {
	delay := time.Duration(float64(base) * math.Pow(2, float64(attempt)))

	if retryAfter != nil && *retryAfter > delay {
		delay = *retryAfter
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// parseRetryAfter parses the Retry-After header value. It supports both
// delay-seconds (integer) and HTTP-date formats. Returns nil if the header
// is empty or unparseable.
func parseRetryAfter(val string) *time.Duration {
	if val == "" {
		return nil
	}

	// Try as seconds first.
	if secs, err := strconv.Atoi(val); err == nil && secs > 0 {
		d := time.Duration(secs) * time.Second
		return &d
	}

	// Try as HTTP-date (RFC 1123).
	if t, err := time.Parse(time.RFC1123, val); err == nil {
		d := time.Until(t)
		if d > 0 {
			return &d
		}
	}

	return nil
}
