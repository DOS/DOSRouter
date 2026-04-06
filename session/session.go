// Package session provides session persistence for maintaining model selections.
// It tracks model selections per session to prevent model switching mid-task,
// implements three-strike escalation for repetitive requests, and tracks
// per-session cost accumulation.
package session

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Entry holds the state of a single session.
type Entry struct {
	Model            string   `json:"model"`
	Tier             string   `json:"tier"`
	CreatedAt        int64    `json:"createdAt"`
	LastUsedAt       int64    `json:"lastUsedAt"`
	RequestCount     int      `json:"requestCount"`
	RecentHashes     []string `json:"recentHashes"`
	Strikes          int      `json:"strikes"`
	Escalated        bool     `json:"escalated"`
	SessionCostMicro int64    `json:"sessionCostMicros"` // USDC 6-decimal micros
}

// Config controls session behavior.
type Config struct {
	Enabled   bool          `json:"enabled"`
	Timeout   time.Duration `json:"timeoutMs"`
	HeaderName string       `json:"headerName"`
}

// DefaultConfig returns the default session configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:    true,
		Timeout:    30 * time.Minute,
		HeaderName: "x-session-id",
	}
}

// Store manages session entries with automatic cleanup.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Entry
	config   Config
	stopCh   chan struct{}
}

// NewStore creates a new session store.
func NewStore(config *Config) *Store {
	cfg := DefaultConfig()
	if config != nil {
		if config.Timeout > 0 {
			cfg.Timeout = config.Timeout
		}
		if config.HeaderName != "" {
			cfg.HeaderName = config.HeaderName
		}
		cfg.Enabled = config.Enabled
	}

	s := &Store{
		sessions: make(map[string]*Entry),
		config:   cfg,
		stopCh:   make(chan struct{}),
	}

	if cfg.Enabled {
		go s.cleanupLoop()
	}

	return s
}

func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
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
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()
	timeout := s.config.Timeout.Milliseconds()
	for id, entry := range s.sessions {
		if now-entry.LastUsedAt > timeout {
			delete(s.sessions, id)
		}
	}
}

// Get returns the pinned session entry, or nil if not found/expired.
func (s *Store) Get(sessionID string) *Entry {
	if !s.config.Enabled || sessionID == "" {
		return nil
	}
	s.mu.RLock()
	entry, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok {
		return nil
	}
	now := time.Now().UnixMilli()
	if now-entry.LastUsedAt > s.config.Timeout.Milliseconds() {
		s.mu.Lock()
		delete(s.sessions, sessionID)
		s.mu.Unlock()
		return nil
	}
	return entry
}

// Set pins a model to a session.
func (s *Store) Set(sessionID, model, tier string) {
	if !s.config.Enabled || sessionID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()

	if existing, ok := s.sessions[sessionID]; ok {
		existing.LastUsedAt = now
		existing.RequestCount++
		if existing.Model != model {
			existing.Model = model
			existing.Tier = tier
		}
	} else {
		s.sessions[sessionID] = &Entry{
			Model:        model,
			Tier:         tier,
			CreatedAt:    now,
			LastUsedAt:   now,
			RequestCount: 1,
		}
	}
}

// Touch extends a session's timeout.
func (s *Store) Touch(sessionID string) {
	if !s.config.Enabled || sessionID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.sessions[sessionID]; ok {
		entry.LastUsedAt = time.Now().UnixMilli()
		entry.RequestCount++
	}
}

// Clear removes a specific session.
func (s *Store) Clear(sessionID string) {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
}

// ClearAll removes all sessions.
func (s *Store) ClearAll() {
	s.mu.Lock()
	s.sessions = make(map[string]*Entry)
	s.mu.Unlock()
}

// Stats returns session stats for debugging.
type SessionStat struct {
	ID    string `json:"id"`
	Model string `json:"model"`
	Age   int64  `json:"age"`
}

// GetStats returns current session statistics.
func (s *Store) GetStats() (int, []SessionStat) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now().UnixMilli()
	stats := make([]SessionStat, 0, len(s.sessions))
	for id, entry := range s.sessions {
		short := id
		if len(id) > 8 {
			short = id[:8] + "..."
		}
		stats = append(stats, SessionStat{
			ID:    short,
			Model: entry.Model,
			Age:   (now - entry.CreatedAt) / 1000,
		})
	}
	return len(s.sessions), stats
}

// RecordRequestHash records a request hash and detects repetitive patterns.
// Returns true if escalation should be triggered (3+ consecutive similar).
func (s *Store) RecordRequestHash(sessionID, hash string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.sessions[sessionID]
	if !ok {
		return false
	}

	prev := entry.RecentHashes
	if len(prev) > 0 && prev[len(prev)-1] == hash {
		entry.Strikes++
	} else {
		entry.Strikes = 0
	}

	entry.RecentHashes = append(entry.RecentHashes, hash)
	if len(entry.RecentHashes) > 3 {
		entry.RecentHashes = entry.RecentHashes[1:]
	}

	return entry.Strikes >= 2 && !entry.Escalated
}

// TierConfig is used for escalation lookups.
type TierConfig struct {
	Primary  string   `json:"primary"`
	Fallback []string `json:"fallback"`
}

// Escalate promotes a session to the next tier.
// Returns the new model/tier or empty strings if already at max.
func (s *Store) Escalate(sessionID string, tierConfigs map[string]TierConfig) (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.sessions[sessionID]
	if !ok {
		return "", ""
	}

	tierOrder := []string{"SIMPLE", "MEDIUM", "COMPLEX", "REASONING"}
	currentIdx := -1
	for i, t := range tierOrder {
		if t == entry.Tier {
			currentIdx = i
			break
		}
	}
	if currentIdx < 0 || currentIdx >= len(tierOrder)-1 {
		return "", ""
	}

	nextTier := tierOrder[currentIdx+1]
	nextConfig, ok := tierConfigs[nextTier]
	if !ok {
		return "", ""
	}

	entry.Model = nextConfig.Primary
	entry.Tier = nextTier
	entry.Strikes = 0
	entry.Escalated = true

	return nextConfig.Primary, nextTier
}

// AddCost adds cost (in USDC micros) to a session's running total.
func (s *Store) AddCost(sessionID string, additionalMicros int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.sessions[sessionID]
	if !ok {
		now := time.Now().UnixMilli()
		entry = &Entry{
			Tier:       "DIRECT",
			CreatedAt:  now,
			LastUsedAt: now,
		}
		s.sessions[sessionID] = entry
	}
	entry.SessionCostMicro += additionalMicros
}

// GetCostUSD returns the total accumulated cost for a session in USD.
func (s *Store) GetCostUSD(sessionID string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.sessions[sessionID]
	if !ok {
		return 0
	}
	return float64(entry.SessionCostMicro) / 1_000_000
}

// Close stops the cleanup goroutine.
func (s *Store) Close() {
	close(s.stopCh)
}

// GetSessionID extracts a session ID from request headers.
func GetSessionID(headers map[string]string, headerName string) string {
	if headerName == "" {
		headerName = "x-session-id"
	}
	if v, ok := headers[headerName]; ok && v != "" {
		return v
	}
	return ""
}

// Message is a minimal message struct for session ID derivation.
type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// DeriveSessionID generates a stable session ID from the first user message.
func DeriveSessionID(messages []Message) string {
	for _, m := range messages {
		if m.Role == "user" {
			var content string
			switch v := m.Content.(type) {
			case string:
				content = v
			default:
				b, _ := json.Marshal(v)
				content = string(b)
			}
			h := sha256.Sum256([]byte(content))
			return fmt.Sprintf("%x", h[:4]) // 8-char hex
		}
	}
	return ""
}

// HashRequestContent generates a short hash fingerprint from request content.
func HashRequestContent(lastUserContent string, toolCallNames []string) string {
	// Normalize whitespace
	normalized := collapseWhitespace(lastUserContent)
	if len(normalized) > 500 {
		normalized = normalized[:500]
	}

	toolSuffix := ""
	if len(toolCallNames) > 0 {
		sorted := make([]string, len(toolCallNames))
		copy(sorted, toolCallNames)
		// Simple sort
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[j] < sorted[i] {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
		toolSuffix = "|tools:"
		for k, name := range sorted {
			if k > 0 {
				toolSuffix += ","
			}
			toolSuffix += name
		}
	}

	h := sha256.Sum256([]byte(normalized + toolSuffix))
	return fmt.Sprintf("%x", h[:6]) // 12-char hex
}

func collapseWhitespace(s string) string {
	buf := make([]byte, 0, len(s))
	inSpace := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			if !inSpace {
				buf = append(buf, ' ')
				inSpace = true
			}
		} else {
			buf = append(buf, c)
			inSpace = false
		}
	}
	// Trim
	start, end := 0, len(buf)
	for start < end && buf[start] == ' ' {
		start++
	}
	for end > start && buf[end-1] == ' ' {
		end--
	}
	return string(buf[start:end])
}
