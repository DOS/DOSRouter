// Package session provides in-memory session persistence with timeout-based
// expiry, three-strike escalation, and per-session cost tracking.
package session

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/DOS/DOSRouter/router"
)

const (
	defaultTimeoutMs       = 30 * 60 * 1000 // 30 minutes
	defaultCleanupInterval = 5 * time.Minute
	defaultHeaderName      = "x-session-id"
	recentHashWindowSize   = 3
	strikeThreshold        = 2
	maxContentLen          = 500
)

// Config controls session store behavior.
type Config struct {
	Enabled    bool
	TimeoutMs  int64
	HeaderName string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{Enabled: false, TimeoutMs: defaultTimeoutMs, HeaderName: defaultHeaderName}
}

// Cache-aware sticky routing constants.
// When a request has large context (code paste, document QA), we pin the model
// to maximize provider-side prefix cache hits. The sticky expires after the
// provider's prefix cache TTL so we don't lose dynamic routing benefits once
// the cache is cold anyway.
const (
	// Single message above this token estimate triggers immediate sticky.
	stickyMessageTokenThreshold = 3000
	// Cumulative conversation history above this triggers sticky.
	stickyHistoryTokenThreshold = 5000
)

// Provider prefix cache TTLs in milliseconds. Each provider evicts prefix
// cache at different intervals — sticky pinning beyond this is pointless
// because the provider KV cache is already cold.
var providerCacheTTLMs = map[string]int64{
	"anthropic": 5 * 60 * 1000,  // 5 min default (up to 1h with extended TTL)
	"openai":    5 * 60 * 1000,  // ~5-10 min (undocumented, conservative)
	"deepseek":  5 * 60 * 1000,  // ~5 min
	"google":    5 * 60 * 1000,  // ~5 min
	"groq":      5 * 60 * 1000,  // ~5 min
	"vllm":      10 * 60 * 1000, // 10 min (self-hosted, only evicts on memory pressure)
	"dosrouter": 10 * 60 * 1000, // 10 min (self-hosted vLLM behind DOSRouter)
	"default":   5 * 60 * 1000,  // fallback for unknown providers
}

// CacheTTLForProvider returns the prefix cache TTL for a given provider name.
func CacheTTLForProvider(provider string) int64 {
	if ttl, ok := providerCacheTTLMs[provider]; ok {
		return ttl
	}
	return providerCacheTTLMs["default"]
}

// ProviderFromModel extracts the provider prefix from a model ID.
// e.g. "anthropic/claude-sonnet-4.6" → "anthropic", "gpt-4o" → "default"
func ProviderFromModel(modelID string) string {
	if idx := strings.Index(modelID, "/"); idx > 0 {
		return modelID[:idx]
	}
	return "default"
}

// Entry holds the state of a single session.
type Entry struct {
	Model             string
	Tier              router.Tier
	CreatedAt         int64    // unix millis
	LastUsedAt        int64    // unix millis
	RequestCount      int
	RecentHashes      []string // sliding window of recent request hashes
	Strikes           int
	Escalated         bool
	UserExplicit      bool     // true when user explicitly selected a model (not a routing profile)
	CacheSticky       bool     // true when session is pinned for prefix cache optimization
	CacheStickyAt     int64    // when cache sticky was activated (unix millis)
	Provider          string   // provider name for cache TTL lookup (e.g. "anthropic", "vllm")
	SessionCostMicros int64    // cost in micro-USD (1 USD = 1_000_000 micros)
}

// Stats is a snapshot of session store state for debugging.
type Stats struct {
	Count    int           `json:"count"`
	Sessions []SessionInfo `json:"sessions"`
}

// SessionInfo is a summary of a single session.
type SessionInfo struct {
	ID    string `json:"id"`
	Model string `json:"model"`
	Age   int64  `json:"age"` // seconds
}

// Store is a concurrent-safe in-memory session store with automatic cleanup.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Entry
	config   Config
	stopCh   chan struct{}
}

// NewStore creates a session store. If config.Enabled is true, a background
// goroutine periodically evicts expired sessions.
func NewStore(cfg Config) *Store {
	s := &Store{sessions: make(map[string]*Entry), config: cfg, stopCh: make(chan struct{})}
	if cfg.Enabled {
		go s.cleanupLoop()
	}
	return s
}

// GetSession returns the session entry for the given ID, or nil if expired.
func (s *Store) GetSession(sessionID string) *Entry {
	if !s.config.Enabled || sessionID == "" {
		return nil
	}
	s.mu.RLock()
	entry, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok {
		return nil
	}
	now := timeMillis()
	if now-entry.LastUsedAt > s.config.TimeoutMs {
		s.mu.Lock()
		delete(s.sessions, sessionID)
		s.mu.Unlock()
		return nil
	}
	return entry
}

// SetSession pins a model to a session. Creates or updates.
// If userExplicit is true, the user explicitly chose this model (not a routing
// profile), and session pinning will skip tier escalation and profile-based
// routing for the lifetime of the session.
func (s *Store) SetSession(sessionID, model string, tier router.Tier, userExplicit bool) {
	if !s.config.Enabled || sessionID == "" {
		return
	}
	now := timeMillis()
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.sessions[sessionID]; ok {
		existing.LastUsedAt = now
		existing.RequestCount++
		if existing.Model != model {
			existing.Model = model
			existing.Tier = tier
		}
		// UserExplicit is sticky: once set, only cleared on session expiry.
		if userExplicit {
			existing.UserExplicit = true
		}
	} else {
		s.sessions[sessionID] = &Entry{
			Model: model, Tier: tier, CreatedAt: now, LastUsedAt: now,
			RequestCount: 1, UserExplicit: userExplicit,
		}
	}
}

// TouchSession extends a session timeout and increments its request count.
func (s *Store) TouchSession(sessionID string) {
	if !s.config.Enabled || sessionID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.sessions[sessionID]; ok {
		entry.LastUsedAt = timeMillis()
		entry.RequestCount++
	}
}

// ClearSession removes a specific session.
func (s *Store) ClearSession(sessionID string) {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
}

// ClearAll removes every session.
func (s *Store) ClearAll() {
	s.mu.Lock()
	s.sessions = make(map[string]*Entry)
	s.mu.Unlock()
}

// GetStats returns a snapshot of all active sessions for debugging.
func (s *Store) GetStats() Stats {
	now := timeMillis()
	s.mu.RLock()
	defer s.mu.RUnlock()
	infos := make([]SessionInfo, 0, len(s.sessions))
	for id, entry := range s.sessions {
		truncID := id
		if len(truncID) > 8 {
			truncID = truncID[:8] + "..."
		}
		infos = append(infos, SessionInfo{
			ID: truncID, Model: entry.Model, Age: (now - entry.CreatedAt) / 1000,
		})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].ID < infos[j].ID })
	return Stats{Count: len(s.sessions), Sessions: infos}
}

// RecordRequestHash tracks a request content hash in the session sliding
// window. If 2+ consecutive identical hashes, increment strikes. Returns strike count.
func (s *Store) RecordRequestHash(sessionID, hash string) int {
	if !s.config.Enabled || sessionID == "" {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.sessions[sessionID]
	if !ok {
		return 0
	}
	entry.RecentHashes = append(entry.RecentHashes, hash)
	if len(entry.RecentHashes) > recentHashWindowSize {
		entry.RecentHashes = entry.RecentHashes[len(entry.RecentHashes)-recentHashWindowSize:]
	}
	consecutive := 0
	for i := len(entry.RecentHashes) - 1; i >= 0; i-- {
		if entry.RecentHashes[i] == hash {
			consecutive++
		} else {
			break
		}
	}
	if consecutive >= strikeThreshold {
		entry.Strikes++
	}
	return entry.Strikes
}

// EscalateSession bumps the session tier one level up.
// If the session has UserExplicit set, escalation is skipped entirely -
// the user's explicit model choice wins unconditionally.
func (s *Store) EscalateSession(sessionID string) {
	if !s.config.Enabled || sessionID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.sessions[sessionID]
	if !ok {
		return
	}
	if entry.UserExplicit {
		return
	}
	entry.Escalated = true
	switch r := router.TierRank(entry.Tier); {
	case r <= 0:
		entry.Tier = router.TierMedium
	case r == 1:
		entry.Tier = router.TierComplex
	case r == 2:
		entry.Tier = router.TierReasoning
	}
}

// AddSessionCost adds cost in micro-USD to the session.
func (s *Store) AddSessionCost(sessionID string, micros int64) {
	if !s.config.Enabled || sessionID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.sessions[sessionID]; ok {
		entry.SessionCostMicros += micros
	}
}

// GetSessionCostUSD returns the session cost in USD.
func (s *Store) GetSessionCostUSD(sessionID string) float64 {
	if !s.config.Enabled || sessionID == "" {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if entry, ok := s.sessions[sessionID]; ok {
		return float64(entry.SessionCostMicros) / 1_000_000.0
	}
	return 0
}

// GetSessionID extracts a session ID from HTTP headers.
func GetSessionID(header http.Header, headerName string) string {
	if headerName == "" {
		headerName = defaultHeaderName
	}
	if v := header.Get(headerName); v != "" {
		return v
	}
	return header.Get(strings.ToLower(headerName))
}

// DeriveSessionID derives a session ID from the first user message via SHA-256.
func DeriveSessionID(firstUserMessage string) string {
	h := sha256.Sum256([]byte(firstUserMessage))
	return hex.EncodeToString(h[:])
}

var wsRe = regexp.MustCompile(`s+`)

// HashRequestContent produces a short hash from request content. It normalizes
// whitespace, truncates to 500 chars, and includes tool names.
func HashRequestContent(content string, toolNames []string) string {
	normalized := wsRe.ReplaceAllString(strings.TrimSpace(content), " ")
	if len(normalized) > maxContentLen {
		normalized = normalized[:maxContentLen]
	}
	if len(toolNames) > 0 {
		sorted := make([]string, len(toolNames))
		copy(sorted, toolNames)
		sort.Strings(sorted)
		normalized += "|tools:" + strings.Join(sorted, ",")
	}
	h := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(h[:16]) // 32 hex chars
}

// ShouldCacheSticky checks whether a request's context is large enough to
// warrant pinning the model for prefix cache optimization. It estimates token
// count from the raw message content (1 token ≈ 4 chars for English, 2 chars
// for CJK/code — we use 3 as a conservative average).
func ShouldCacheSticky(messages []MessageInfo) bool {
	for _, m := range messages {
		estTokens := len(m.Content) / 3
		if estTokens >= stickyMessageTokenThreshold {
			return true // Single large message (code paste, document)
		}
	}
	// Check cumulative history
	totalChars := 0
	for _, m := range messages {
		totalChars += len(m.Content)
	}
	return totalChars/3 >= stickyHistoryTokenThreshold
}

// MessageInfo is a minimal message representation for cache-sticky evaluation.
type MessageInfo struct {
	Role    string
	Content string
}

// SetCacheSticky marks a session as cache-sticky for a specific provider.
// The model is pinned until the provider's prefix cache TTL expires.
func (s *Store) SetCacheSticky(sessionID, provider string) {
	if !s.config.Enabled || sessionID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.sessions[sessionID]; ok {
		if !entry.CacheSticky {
			entry.CacheSticky = true
			entry.CacheStickyAt = timeMillis()
			entry.Provider = provider
		}
	}
}

// IsCacheSticky returns true if the session has an active cache-sticky pin
// (i.e., the pin hasn't expired past the provider's prefix cache TTL).
func (s *Store) IsCacheSticky(sessionID string) bool {
	if !s.config.Enabled || sessionID == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.sessions[sessionID]
	if !ok || !entry.CacheSticky {
		return false
	}
	// Expire sticky if idle longer than this provider's prefix cache TTL
	ttl := CacheTTLForProvider(entry.Provider)
	if timeMillis()-entry.LastUsedAt > ttl {
		return false
	}
	return true
}

// Close stops the background cleanup goroutine.
func (s *Store) Close() {
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}

func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(defaultCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-s.stopCh:
			return
		}
	}
}

func (s *Store) cleanup() {
	now := timeMillis()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, entry := range s.sessions {
		if now-entry.LastUsedAt > s.config.TimeoutMs {
			delete(s.sessions, id)
		}
	}
}

func timeMillis() int64 {
	return time.Now().UnixMilli()
}

// String returns a one-line summary of a session entry.
func (e *Entry) String() string {
	return fmt.Sprintf("model=%s tier=%s reqs=%d strikes=%d explicit=%v cost=$%.6f",
		e.Model, e.Tier, e.RequestCount, e.Strikes, e.UserExplicit,
		float64(e.SessionCostMicros)/1_000_000.0)
}
