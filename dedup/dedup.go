// Package dedup provides request deduplication for LLM proxy requests.
// Multiple identical in-flight requests share a single upstream call,
// and recently completed responses are cached for a short TTL.
package dedup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"sync"
	"time"
)

const (
	// DefaultTTL is how long completed responses stay cached.
	DefaultTTL = 30 * time.Second
	// DefaultMaxBodySize is the max response body size to cache (1 MB).
	DefaultMaxBodySize = 1 << 20
)

// timestampRe matches log-style timestamps like "[Mon 2024-01-15 09:30 UTC]".
var timestampRe = regexp.MustCompile(`^\[\w{3}\s+\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}\s+\w+\]\s*`)

// Response holds a cached upstream response.
type Response struct {
	StatusCode int
	Header     map[string][]string
	Body       []byte
}

// completedEntry is a time-stamped cached response.
type completedEntry struct {
	resp      Response
	expiresAt time.Time
}

// inflightEntry tracks waiters for an in-flight request.
type inflightEntry struct {
	done chan struct{} // closed when the response is ready
	resp Response
	err  error
}

// Deduplicator coalesces identical requests and caches recent responses.
type Deduplicator struct {
	mu          sync.Mutex
	inflight    map[string]*inflightEntry
	completed   map[string]completedEntry
	ttl         time.Duration
	maxBodySize int
}

// Option configures a Deduplicator.
type Option func(*Deduplicator)

// WithTTL sets the completed-response cache TTL.
func WithTTL(d time.Duration) Option {
	return func(dd *Deduplicator) { dd.ttl = d }
}

// WithMaxBodySize sets the maximum response body size to cache.
func WithMaxBodySize(n int) Option {
	return func(dd *Deduplicator) { dd.maxBodySize = n }
}

// New creates a Deduplicator with the given options.
func New(opts ...Option) *Deduplicator {
	d := &Deduplicator{
		inflight:    make(map[string]*inflightEntry),
		completed:   make(map[string]completedEntry),
		ttl:         DefaultTTL,
		maxBodySize: DefaultMaxBodySize,
	}
	for _, o := range opts {
		o(d)
	}
	return d
}

// Do executes fn at most once for concurrent identical requests identified by
// the JSON body. If another goroutine is already executing a request with the
// same key, Do blocks until that request completes and returns the same result.
// Recently completed responses are returned from cache without calling fn.
//
// Returns the response, whether it was a cache/dedup hit, and any error.
func (d *Deduplicator) Do(body []byte, fn func() (Response, error)) (Response, bool, error) {
	key, err := HashBody(body)
	if err != nil {
		// Cannot canonicalize; just call fn directly.
		resp, fnErr := fn()
		return resp, false, fnErr
	}

	d.mu.Lock()

	// Check completed cache.
	if entry, ok := d.completed[key]; ok {
		if time.Now().Before(entry.expiresAt) {
			d.mu.Unlock()
			return entry.resp, true, nil
		}
		delete(d.completed, key)
	}

	// Check in-flight.
	if inf, ok := d.inflight[key]; ok {
		d.mu.Unlock()
		<-inf.done
		return inf.resp, true, inf.err
	}

	// We are the leader for this key.
	inf := &inflightEntry{done: make(chan struct{})}
	d.inflight[key] = inf
	d.mu.Unlock()

	resp, fnErr := fn()

	inf.resp = resp
	inf.err = fnErr
	close(inf.done)

	d.mu.Lock()
	delete(d.inflight, key)
	if fnErr == nil && len(resp.Body) <= d.maxBodySize {
		d.completed[key] = completedEntry{
			resp:      resp,
			expiresAt: time.Now().Add(d.ttl),
		}
	}
	d.mu.Unlock()

	return resp, false, fnErr
}

// Prune removes expired entries from the completed cache.
func (d *Deduplicator) Prune() int {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	pruned := 0
	for k, entry := range d.completed {
		if now.After(entry.expiresAt) {
			delete(d.completed, k)
			pruned++
		}
	}
	return pruned
}

// Len returns the number of entries in the completed cache.
func (d *Deduplicator) Len() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.completed)
}

// HashBody returns a hex-encoded SHA-256 hash of the canonicalized JSON body.
// Keys are sorted recursively and timestamp prefixes are stripped from string
// values.
func HashBody(body []byte) (string, error) {
	var raw interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", fmt.Errorf("dedup: invalid JSON body: %w", err)
	}
	canonical := canonicalize(raw)
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("dedup: marshal error: %w", err)
	}
	h := sha256.Sum256(encoded)
	return hex.EncodeToString(h[:]), nil
}

// canonicalize recursively sorts object keys and strips timestamp prefixes
// from string values.
func canonicalize(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		// Use an ordered representation: []interface{} of [key, value] pairs.
		// This ensures json.Marshal produces a deterministic byte sequence.
		pairs := make([][2]interface{}, 0, len(keys))
		for _, k := range keys {
			pairs = append(pairs, [2]interface{}{k, canonicalize(val[k])})
		}
		return pairs

	case []interface{}:
		out := make([]interface{}, len(val))
		for i, item := range val {
			out[i] = canonicalize(item)
		}
		return out

	case string:
		return timestampRe.ReplaceAllString(val, "")

	default:
		return val
	}
}
