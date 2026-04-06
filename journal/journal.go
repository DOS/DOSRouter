// Package journal provides per-session memory of key actions extracted from
// assistant responses, enabling context recall when users ask about earlier work.
package journal

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	defaultMaxEntries           = 100
	defaultMaxAgeMs             = 24 * 60 * 60 * 1000 // 24 hours
	defaultMaxEventsPerResponse = 5
)

// Config controls journal behavior.
type Config struct {
	MaxEntries           int
	MaxAgeMs             int64
	MaxEventsPerResponse int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxEntries:           defaultMaxEntries,
		MaxAgeMs:             defaultMaxAgeMs,
		MaxEventsPerResponse: defaultMaxEventsPerResponse,
	}
}

// Entry is a single recorded action.
type Entry struct {
	Timestamp int64  // unix millis
	Action    string
	Model     string
}

// Stats holds journal-level statistics.
type Stats struct {
	Sessions     int `json:"sessions"`
	TotalEntries int `json:"totalEntries"`
}

// SessionJournal stores per-session action journals.
type SessionJournal struct {
	mu       sync.Mutex
	journals map[string][]Entry
	config   Config
}

// New creates a SessionJournal with the given config.
func New(cfg Config) *SessionJournal {
	if cfg.MaxEntries == 0 {
		cfg.MaxEntries = defaultMaxEntries
	}
	if cfg.MaxAgeMs == 0 {
		cfg.MaxAgeMs = defaultMaxAgeMs
	}
	if cfg.MaxEventsPerResponse == 0 {
		cfg.MaxEventsPerResponse = defaultMaxEventsPerResponse
	}
	return &SessionJournal{journals: make(map[string][]Entry), config: cfg}
}

// Compiled regex patterns for event extraction.
var eventPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)I (?:also |then |have )?(?:created|implemented|added|wrote|built|generated|set up|initialized) ([^.!?
]{10,150})`),
	regexp.MustCompile(`(?i)I (?:also |then |have )?(?:fixed|resolved|solved|patched|corrected|addressed|debugged) ([^.!?
]{10,150})`),
	regexp.MustCompile(`(?i)I (?:also |then |have )?(?:completed|finished|done with|wrapped up) ([^.!?
]{10,150})`),
	regexp.MustCompile(`(?i)I (?:also |then |have )?(?:updated|modified|changed|refactored|improved|enhanced|optimized) ([^.!?
]{10,150})`),
	regexp.MustCompile(`(?i)Successfully ([^.!?
]{10,150})`),
	regexp.MustCompile(`(?i)I (?:also |then |have )?(?:ran|executed|called|invoked) ([^.!?
]{10,100})`),
}

var contextTriggers = []string{
	"what did you do", "what have you done", "what did we do", "what have we done",
	"earlier", "before", "previously", "this session", "today", "so far",
	"remind me", "summarize", "summary of", "recap",
	"your work", "your progress", "accomplished", "achievements", "completed tasks",
}

// ExtractEvents scans assistant response content for action patterns.
func (sj *SessionJournal) ExtractEvents(content string) []string {
	if content == "" {
		return nil
	}
	var events []string
	seen := make(map[string]struct{})
	limit := sj.config.MaxEventsPerResponse
	for _, pattern := range eventPatterns {
		for _, action := range pattern.FindAllString(content, -1) {
			action = strings.TrimSpace(action)
			norm := strings.ToLower(action)
			if _, dup := seen[norm]; dup {
				continue
			}
			if len(action) >= 15 && len(action) <= 200 {
				events = append(events, action)
				seen[norm] = struct{}{}
			}
			if len(events) >= limit {
				break
			}
		}
		if len(events) >= limit {
			break
		}
	}
	return events
}

// Record adds extracted events to the session journal, trimming by maxAge and maxEntries.
func (sj *SessionJournal) Record(sessionID string, events []string, model string) {
	if sessionID == "" || len(events) == 0 {
		return
	}
	now := time.Now().UnixMilli()
	sj.mu.Lock()
	defer sj.mu.Unlock()
	journal := sj.journals[sessionID]
	for _, action := range events {
		journal = append(journal, Entry{Timestamp: now, Action: action, Model: model})
	}
	cutoff := now - sj.config.MaxAgeMs
	trimmed := make([]Entry, 0, len(journal))
	for _, e := range journal {
		if e.Timestamp > cutoff {
			trimmed = append(trimmed, e)
		}
	}
	if len(trimmed) > sj.config.MaxEntries {
		trimmed = trimmed[len(trimmed)-sj.config.MaxEntries:]
	}
	sj.journals[sessionID] = trimmed
}

// NeedsContext returns true if the user message triggers session history recall.
func (sj *SessionJournal) NeedsContext(lastUserMessage string) bool {
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

// Format renders the session journal as a block for system message injection.
func (sj *SessionJournal) Format(sessionID string) string {
	sj.mu.Lock()
	journal := sj.journals[sessionID]
	cp := make([]Entry, len(journal))
	copy(cp, journal)
	sj.mu.Unlock()
	if len(cp) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("[Session Memory - Key Actions]\n")
	for _, e := range cp {
		t := time.UnixMilli(e.Timestamp)
		fmt.Fprintf(&b, "- %s: %s\n", t.Format("03:04 PM"), e.Action)
	}
	return b.String()
}

// GetEntries returns raw journal entries for a session.
func (sj *SessionJournal) GetEntries(sessionID string) []Entry {
	sj.mu.Lock()
	defer sj.mu.Unlock()
	out := make([]Entry, len(sj.journals[sessionID]))
	copy(out, sj.journals[sessionID])
	return out
}

// Clear removes the journal for a specific session.
func (sj *SessionJournal) Clear(sessionID string) {
	sj.mu.Lock()
	delete(sj.journals, sessionID)
	sj.mu.Unlock()
}

// ClearAll removes all journals.
func (sj *SessionJournal) ClearAll() {
	sj.mu.Lock()
	sj.journals = make(map[string][]Entry)
	sj.mu.Unlock()
}

// GetStats returns aggregate statistics about the journal store.
func (sj *SessionJournal) GetStats() Stats {
	sj.mu.Lock()
	defer sj.mu.Unlock()
	total := 0
	for _, entries := range sj.journals {
		total += len(entries)
	}
	return Stats{Sessions: len(sj.journals), TotalEntries: total}
}
