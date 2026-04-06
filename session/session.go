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
func (s *Store) SetSession(sessionID, model string, tier router.Tier) {
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
	} else {
		s.sessions[sessionID] = &Entry{
			Model: model, Tier: tier, CreatedAt: now, LastUsedAt: now, RequestCount: 1,
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
	return fmt.Sprintf("model=%s tier=%s reqs=%d strikes=%d cost=$%.6f",
		e.Model, e.Tier, e.RequestCount, e.Strikes,
		float64(e.SessionCostMicros)/1_000_000.0)
}
