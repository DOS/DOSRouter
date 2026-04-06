// Package cache provides a TTL + LRU response cache for LLM completions.
// Cache keys are derived from canonicalized request JSON, skipping
// non-deterministic fields (stream, user, request_id) and stripping
// timestamp prefixes from message content.
package cache

import (
	"container/list"
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
	// DefaultTTL is the default time-to-live for cached entries.
	DefaultTTL = 10 * time.Minute
	// DefaultMaxSize is the default maximum number of cached entries.
	DefaultMaxSize = 200
	// DefaultMaxItemSize is the maximum size of a single cached item (1 MB).
	DefaultMaxItemSize = 1 << 20
)

// skipFields are request fields excluded from the cache key because they
// do not affect the LLM response content.
var skipFields = map[string]bool{
	"stream":     true,
	"user":       true,
	"request_id": true,
}

// timestampRe matches log-style timestamps like "[Mon 2024-01-15 09:30 UTC]".
var timestampRe = regexp.MustCompile(`^\[\w{3}\s+\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}\s+\w+\]\s*`)

// Entry is a cached response.
type Entry struct {
	Body       []byte
	StatusCode int
	Header     map[string][]string
	CreatedAt  time.Time
}

type cacheItem struct {
	key       string
	entry     Entry
	expiresAt time.Time
}

// Stats holds cache performance counters.
type Stats struct {
	Hits      int64   `json:"hits"`
	Misses    int64   `json:"misses"`
	Evictions int64   `json:"evictions"`
	HitRate   float64 `json:"hitRate"`
}

// Cache is a concurrency-safe TTL + LRU response cache.
type Cache struct {
	mu          sync.Mutex
	items       map[string]*list.Element
	evictList   *list.List
	maxSize     int
	maxItemSize int
	ttl         time.Duration
	enabled     bool

	hits      int64
	misses    int64
	evictions int64
}

// Option configures a Cache.
type Option func(*Cache)

// WithTTL sets the cache entry TTL.
func WithTTL(d time.Duration) Option {
	return func(c *Cache) { c.ttl = d }
}

// WithMaxSize sets the maximum number of cached entries.
func WithMaxSize(n int) Option {
	return func(c *Cache) { c.maxSize = n }
}

// WithMaxItemSize sets the maximum byte size of a single cached item.
func WithMaxItemSize(n int) Option {
	return func(c *Cache) { c.maxItemSize = n }
}

// WithEnabled sets whether the cache is active.
func WithEnabled(b bool) Option {
	return func(c *Cache) { c.enabled = b }
}

// New creates a Cache with the given options.
func New(opts ...Option) *Cache {
	c := &Cache{
		items:       make(map[string]*list.Element),
		evictList:   list.New(),
		maxSize:     DefaultMaxSize,
		maxItemSize: DefaultMaxItemSize,
		ttl:         DefaultTTL,
		enabled:     true,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Get retrieves a cached response for the given request body.
// Returns the entry and true on hit, or a zero Entry and false on miss.
// If noCache is true (e.g. from a Cache-Control: no-cache header), it
// always returns a miss.
func (c *Cache) Get(body []byte, noCache bool) (Entry, bool) {
	if !c.enabled || noCache {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return Entry{}, false
	}

	key, err := CacheKey(body)
	if err != nil {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return Entry{}, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		c.misses++
		return Entry{}, false
	}

	item := elem.Value.(*cacheItem)
	if time.Now().After(item.expiresAt) {
		c.removeLocked(elem)
		c.misses++
		return Entry{}, false
	}

	// Move to front (most recently used).
	c.evictList.MoveToFront(elem)
	c.hits++
	return item.entry, true
}

// Set stores a response in the cache, keyed by the request body.
func (c *Cache) Set(body []byte, entry Entry) {
	if !c.enabled {
		return
	}
	if len(entry.Body) > c.maxItemSize {
		return
	}

	key, err := CacheKey(body)
	if err != nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing entry.
	if elem, ok := c.items[key]; ok {
		item := elem.Value.(*cacheItem)
		item.entry = entry
		item.expiresAt = time.Now().Add(c.ttl)
		c.evictList.MoveToFront(elem)
		return
	}

	// Evict LRU if at capacity.
	for c.evictList.Len() >= c.maxSize {
		c.evictOldest()
	}

	item := &cacheItem{
		key:       key,
		entry:     entry,
		expiresAt: time.Now().Add(c.ttl),
	}
	elem := c.evictList.PushFront(item)
	c.items[key] = elem
}

// SetEnabled enables or disables the cache at runtime.
func (c *Cache) SetEnabled(b bool) {
	c.mu.Lock()
	c.enabled = b
	c.mu.Unlock()
}

// Stats returns cache performance statistics.
func (c *Cache) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()

	total := c.hits + c.misses
	var rate float64
	if total > 0 {
		rate = float64(c.hits) / float64(total)
	}
	return Stats{
		Hits:      c.hits,
		Misses:    c.misses,
		Evictions: c.evictions,
		HitRate:   rate,
	}
}

// Len returns the number of items currently in the cache.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.evictList.Len()
}

// Prune removes expired entries and returns the count removed.
func (c *Cache) Prune() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	pruned := 0
	for elem := c.evictList.Back(); elem != nil; {
		prev := elem.Prev()
		item := elem.Value.(*cacheItem)
		if now.After(item.expiresAt) {
			c.removeLocked(elem)
			pruned++
		}
		elem = prev
	}
	return pruned
}

// Clear removes all entries and resets stats.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.evictList.Init()
	c.hits = 0
	c.misses = 0
	c.evictions = 0
}

func (c *Cache) evictOldest() {
	elem := c.evictList.Back()
	if elem == nil {
		return
	}
	c.removeLocked(elem)
	c.evictions++
}

func (c *Cache) removeLocked(elem *list.Element) {
	item := elem.Value.(*cacheItem)
	delete(c.items, item.key)
	c.evictList.Remove(elem)
}

// CacheKey returns a hex-encoded SHA-256 hash of the canonicalized request
// JSON, omitting non-deterministic fields and stripping timestamps.
func CacheKey(body []byte) (string, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", fmt.Errorf("cache: invalid JSON body: %w", err)
	}

	// Remove skip fields.
	for f := range skipFields {
		delete(raw, f)
	}

	canonical := canonicalize(raw)
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("cache: marshal error: %w", err)
	}
	h := sha256.Sum256(encoded)
	return hex.EncodeToString(h[:]), nil
}

// canonicalize recursively sorts object keys and strips timestamp prefixes
// from string values, producing a deterministic structure for hashing.
func canonicalize(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
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
