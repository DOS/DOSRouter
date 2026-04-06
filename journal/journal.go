// Package journal provides session memory for tracking key actions
// performed during a session, enabling agents to recall earlier work.
package journal

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Entry is a single recorded action.
type Entry struct {
	Timestamp int64  `json:"timestamp"`
	Action    string `json:"action"`
	Model     string `json:"model,omitempty"`
}

// Config controls journal behavior.
type Config struct {
	MaxEntries          int   `json:"maxEntries"`
	MaxAgeMs            int64 `json:"maxAgeMs"`
	MaxEventsPerResponse int  `json:"maxEventsPerResponse"`
}

// DefaultConfig returns default journal configuration.
func DefaultConfig() Config {
	return Config{
		MaxEntries:          100,
		MaxAgeMs:            24 * 60 * 60 * 1000, // 24 hours
		MaxEventsPerResponse: 5,
	}
}

// SessionJournal maintains a compact record of key actions per session.
type SessionJournal struct {
	mu       sync.RWMutex
	journals map[string][]Entry
	config   Config
}

// New creates a new SessionJournal.
func New(config *Config) *SessionJournal {
	cfg := DefaultConfig()
	if config != nil {
		if config.MaxEntries > 0 {
			cfg.MaxEntries = config.MaxEntries
		}
		if config.MaxAgeMs > 0 {
			cfg.MaxAgeMs = config.MaxAgeMs
		}
		if config.MaxEventsPerResponse > 0 {
			cfg.MaxEventsPerResponse = config.MaxEventsPerResponse
		}
	}
	return &SessionJournal{
		journals: make(map[string][]Entry),
		config:   cfg,
	}
}

// Pre-compiled patterns for extracting key actions
var actionPatterns = []*regexp.Regexp{
	// Creation patterns
	regexp.MustCompile(`(?i)I (?:also |then |have )?(?:created|implemented|added|wrote|built|generated|set up|initialized) ([^.!?\n]{10,150})`),
	// Fix patterns
	regexp.MustCompile(`(?i)I (?:also |then |have )?(?:fixed|resolved|solved|patched|corrected|addressed|debugged) ([^.!?\n]{10,150})`),
	// Completion patterns
	regexp.MustCompile(`(?i)I (?:also |then |have )?(?:completed|finished|done with|wrapped up) ([^.!?\n]{10,150})`),
	// Update patterns
	regexp.MustCompile(`(?i)I (?:also |then |have )?(?:updated|modified|changed|refactored|improved|enhanced|optimized) ([^.!?\n]{10,150})`),
	// Success patterns
	regexp.MustCompile(`(?i)Successfully ([^.!?\n]{10,150})`),
	// Tool usage patterns
	regexp.MustCompile(`(?i)I (?:also |then |have )?(?:ran|executed|called|invoked) ([^.!?\n]{10,100})`),
}

// ExtractEvents extracts key events from assistant response content.
func (j *SessionJournal) ExtractEvents(content string) []string {
	if content == "" {
		return nil
	}

	var events []string
	seen := make(map[string]bool)

	for _, pattern := range actionPatterns {
		matches := pattern.FindAllString(content, -1)
		for _, action := range matches {
			action = strings.TrimSpace(action)
			normalized := strings.ToLower(action)
			if seen[normalized] {
				continue
			}
			if len(action) >= 15 && len(action) <= 200 {
				events = append(events, action)
				seen[normalized] = true
			}
			if len(events) >= j.config.MaxEventsPerResponse {
				return events
			}
		}
		if len(events) >= j.config.MaxEventsPerResponse {
			return events
		}
	}

	return events
}

// Record adds events to the session journal.
func (j *SessionJournal) Record(sessionID string, events []string, model string) {
	if sessionID == "" || len(events) == 0 {
		return
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	journal := j.journals[sessionID]
	now := time.Now().UnixMilli()

	for _, action := range events {
		journal = append(journal, Entry{
			Timestamp: now,
			Action:    action,
			Model:     model,
		})
	}

	// Trim old entries and enforce max count
	cutoff := now - j.config.MaxAgeMs
	filtered := make([]Entry, 0, len(journal))
	for _, e := range journal {
		if e.Timestamp > cutoff {
			filtered = append(filtered, e)
		}
	}
	if len(filtered) > j.config.MaxEntries {
		filtered = filtered[len(filtered)-j.config.MaxEntries:]
	}

	j.journals[sessionID] = filtered
}

// Trigger phrases that indicate user wants historical context.
var contextTriggers = []string{
	"what did you do", "what have you done",
	"what did we do", "what have we done",
	"earlier", "before", "previously",
	"this session", "today", "so far",
	"remind me", "summarize", "summary of", "recap",
	"your work", "your progress", "accomplished",
	"achievements", "completed tasks",
}

// NeedsContext checks if the user message indicates a need for historical context.
func (j *SessionJournal) NeedsContext(lastUserMessage string) bool {
	if lastUserMessage == "" {
		return false
	}
	lower := strings.ToLower(lastUserMessage)
	for _, trigger := range contextTriggers {
		if strings.Contains(lower, trigger) {
			return true
		}
	}
	return false
}

// Format returns the journal formatted for injection into system message.
// Returns empty string if journal is empty.
func (j *SessionJournal) Format(sessionID string) string {
	j.mu.RLock()
	defer j.mu.RUnlock()
	journal := j.journals[sessionID]
	if len(journal) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("[Session Memory - Key Actions]\n")
	for _, e := range journal {
		t := time.UnixMilli(e.Timestamp)
		sb.WriteString(fmt.Sprintf("- %s: %s\n", t.Format("03:04 PM"), e.Action))
	}
	return sb.String()
}

// GetEntries returns raw journal entries for a session.
func (j *SessionJournal) GetEntries(sessionID string) []Entry {
	j.mu.RLock()
	defer j.mu.RUnlock()
	entries := j.journals[sessionID]
	result := make([]Entry, len(entries))
	copy(result, entries)
	return result
}

// Clear removes journal for a specific session.
func (j *SessionJournal) Clear(sessionID string) {
	j.mu.Lock()
	delete(j.journals, sessionID)
	j.mu.Unlock()
}

// ClearAll removes all journals.
func (j *SessionJournal) ClearAll() {
	j.mu.Lock()
	j.journals = make(map[string][]Entry)
	j.mu.Unlock()
}

// GetStats returns journal statistics.
func (j *SessionJournal) GetStats() (sessions int, totalEntries int) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	for _, entries := range j.journals {
		totalEntries += len(entries)
	}
	return len(j.journals), totalEntries
}
