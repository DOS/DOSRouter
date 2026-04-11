// Package spendcontrol enforces spending limits for LLM requests using
// rolling time windows and per-session budgets.
package spendcontrol

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SpendWindow defines the time window over which a spending limit applies.
type SpendWindow string

const (
	WindowPerRequest SpendWindow = "perRequest"
	WindowHourly     SpendWindow = "hourly"
	WindowDaily      SpendWindow = "daily"
	WindowSession    SpendWindow = "session"
)

// windowDuration returns the rolling duration for time-based windows.
func windowDuration(w SpendWindow) time.Duration {
	switch w {
	case WindowHourly:
		return time.Hour
	case WindowDaily:
		return 24 * time.Hour
	default:
		return 0
	}
}

// SpendLimits maps each window to its maximum USD amount.
type SpendLimits map[SpendWindow]float64

// SpendRecord is a single spending event.
type SpendRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Amount    float64   `json:"amount"`
	Model     string    `json:"model,omitempty"`
	Action    string    `json:"action,omitempty"`
}

// CheckResult is the outcome of a spend check.
type CheckResult struct {
	Allowed   bool        `json:"allowed"`
	BlockedBy SpendWindow `json:"blockedBy,omitempty"`
	Remaining float64     `json:"remaining"`
	Reason    string      `json:"reason,omitempty"`
	ResetIn   string      `json:"resetIn,omitempty"`
}

// SpendControl tracks spending against configured limits.
type SpendControl struct {
	mu           sync.Mutex
	limits       SpendLimits
	history      []SpendRecord
	sessionSpent float64
	sessionCalls int
	storage      SpendControlStorage
}

// New creates a SpendControl with the given storage backend.
// It loads persisted state on creation.
func New(storage SpendControlStorage) (*SpendControl, error) {
	sc := &SpendControl{
		limits:  make(SpendLimits),
		storage: storage,
	}
	if err := sc.load(); err != nil {
		return nil, fmt.Errorf("spendcontrol: load: %w", err)
	}
	return sc, nil
}

// SetLimit sets the maximum USD for the given window.
func (sc *SpendControl) SetLimit(window SpendWindow, amount float64) error {
	sc.mu.Lock()
	sc.limits[window] = amount
	sc.mu.Unlock()
	return sc.save()
}

// ClearLimit removes the limit for the given window.
func (sc *SpendControl) ClearLimit(window SpendWindow) error {
	sc.mu.Lock()
	delete(sc.limits, window)
	sc.mu.Unlock()
	return sc.save()
}

// GetLimits returns a copy of the current limits.
func (sc *SpendControl) GetLimits() SpendLimits {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	out := make(SpendLimits, len(sc.limits))
	for k, v := range sc.limits {
		out[k] = v
	}
	return out
}

// Check evaluates whether a request costing estimatedCost USD is allowed.
func (sc *SpendControl) Check(estimatedCost float64) CheckResult {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	now := time.Now()

	// Per-request check
	if limit, ok := sc.limits[WindowPerRequest]; ok {
		if estimatedCost > limit {
			return CheckResult{
				Allowed:   false,
				BlockedBy: WindowPerRequest,
				Remaining: limit,
				Reason:    fmt.Sprintf("request cost $%.4f exceeds per-request limit $%.4f", estimatedCost, limit),
			}
		}
	}

	// Session check
	if limit, ok := sc.limits[WindowSession]; ok {
		remaining := limit - sc.sessionSpent
		if estimatedCost > remaining {
			return CheckResult{
				Allowed:   false,
				BlockedBy: WindowSession,
				Remaining: remaining,
				Reason:    fmt.Sprintf("session spend $%.4f + $%.4f would exceed limit $%.4f", sc.sessionSpent, estimatedCost, limit),
			}
		}
	}

	// Rolling window checks (hourly, daily)
	for _, w := range []SpendWindow{WindowHourly, WindowDaily} {
		limit, ok := sc.limits[w]
		if !ok {
			continue
		}
		d := windowDuration(w)
		cutoff := now.Add(-d)
		var spent float64
		for _, r := range sc.history {
			if r.Timestamp.After(cutoff) {
				spent += r.Amount
			}
		}
		remaining := limit - spent
		if estimatedCost > remaining {
			resetIn := sc.oldestInWindow(cutoff).Add(d).Sub(now)
			return CheckResult{
				Allowed:   false,
				BlockedBy: w,
				Remaining: remaining,
				Reason:    fmt.Sprintf("%s spend $%.4f + $%.4f would exceed limit $%.4f", w, spent, estimatedCost, limit),
				ResetIn:   formatDuration(resetIn),
			}
		}
	}

	// Compute smallest remaining across all active limits
	remaining := -1.0
	for w, limit := range sc.limits {
		var r float64
		switch w {
		case WindowPerRequest:
			r = limit
		case WindowSession:
			r = limit - sc.sessionSpent
		default:
			d := windowDuration(w)
			cutoff := now.Add(-d)
			var spent float64
			for _, rec := range sc.history {
				if rec.Timestamp.After(cutoff) {
					spent += rec.Amount
				}
			}
			r = limit - spent
		}
		if remaining < 0 || r < remaining {
			remaining = r
		}
	}
	if remaining < 0 {
		remaining = 0
	}

	return CheckResult{Allowed: true, Remaining: remaining}
}

// Record logs a completed spend and updates the session totals.
func (sc *SpendControl) Record(amount float64, model, action string) error {
	sc.mu.Lock()
	sc.history = append(sc.history, SpendRecord{
		Timestamp: time.Now(),
		Amount:    amount,
		Model:     model,
		Action:    action,
	})
	sc.sessionSpent += amount
	sc.sessionCalls++
	sc.mu.Unlock()
	return sc.save()
}

// GetSpending returns total spent in each active window.
func (sc *SpendControl) GetSpending() map[SpendWindow]float64 {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	now := time.Now()
	out := make(map[SpendWindow]float64)
	for w := range sc.limits {
		switch w {
		case WindowPerRequest:
			// not cumulative
		case WindowSession:
			out[w] = sc.sessionSpent
		default:
			d := windowDuration(w)
			cutoff := now.Add(-d)
			var spent float64
			for _, r := range sc.history {
				if r.Timestamp.After(cutoff) {
					spent += r.Amount
				}
			}
			out[w] = spent
		}
	}
	return out
}

// GetRemaining returns remaining budget in each active window.
func (sc *SpendControl) GetRemaining() map[SpendWindow]float64 {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	now := time.Now()
	out := make(map[SpendWindow]float64)
	for w, limit := range sc.limits {
		switch w {
		case WindowPerRequest:
			out[w] = limit
		case WindowSession:
			out[w] = limit - sc.sessionSpent
		default:
			d := windowDuration(w)
			cutoff := now.Add(-d)
			var spent float64
			for _, r := range sc.history {
				if r.Timestamp.After(cutoff) {
					spent += r.Amount
				}
			}
			out[w] = limit - spent
		}
	}
	return out
}

// StatusEntry holds limit, spent, and remaining for a single window.
type StatusEntry struct {
	Limit     float64 `json:"limit"`
	Spent     float64 `json:"spent"`
	Remaining float64 `json:"remaining"`
}

// GetStatus returns a combined view of limits, spending, and remaining.
func (sc *SpendControl) GetStatus() map[SpendWindow]StatusEntry {
	spending := sc.GetSpending()
	remaining := sc.GetRemaining()
	limits := sc.GetLimits()

	out := make(map[SpendWindow]StatusEntry, len(limits))
	for w, limit := range limits {
		out[w] = StatusEntry{
			Limit:     limit,
			Spent:     spending[w],
			Remaining: remaining[w],
		}
	}
	return out
}

// GetHistory returns a copy of all spending records.
func (sc *SpendControl) GetHistory() []SpendRecord {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	out := make([]SpendRecord, len(sc.history))
	copy(out, sc.history)
	return out
}

// Cleanup prunes records older than 24 hours.
func (sc *SpendControl) Cleanup() error {
	sc.mu.Lock()
	cutoff := time.Now().Add(-24 * time.Hour)
	kept := sc.history[:0]
	for _, r := range sc.history {
		if r.Timestamp.After(cutoff) {
			kept = append(kept, r)
		}
	}
	sc.history = kept
	sc.mu.Unlock()
	return sc.save()
}

// oldestInWindow returns the oldest record timestamp within the window,
// or now if none found. Caller must hold mu.
func (sc *SpendControl) oldestInWindow(cutoff time.Time) time.Time {
	oldest := time.Now()
	for _, r := range sc.history {
		if r.Timestamp.After(cutoff) && r.Timestamp.Before(oldest) {
			oldest = r.Timestamp
		}
	}
	return oldest
}

// persistedState is the JSON shape for file storage.
type persistedState struct {
	Limits  SpendLimits   `json:"limits"`
	History []SpendRecord `json:"history"`
}

func (sc *SpendControl) save() error {
	if sc.storage == nil {
		return nil
	}
	sc.mu.Lock()
	state := persistedState{Limits: sc.limits, History: sc.history}
	sc.mu.Unlock()
	return sc.storage.Save(state)
}

func (sc *SpendControl) load() error {
	if sc.storage == nil {
		return nil
	}
	state, err := sc.storage.Load()
	if err != nil {
		return err
	}
	if state != nil {
		if state.Limits != nil {
			sc.limits = state.Limits
		}
		if state.History != nil {
			sc.history = state.History
		}
	}
	return nil
}

// ---------- Storage interface ----------

// SpendControlStorage persists spending state.
type SpendControlStorage interface {
	Save(state persistedState) error
	Load() (*persistedState, error)
}

// FileSpendControlStorage reads and writes JSON to disk.
type FileSpendControlStorage struct {
	Path string
}

// DefaultFilePath returns ~/.dosrouter/spending.json.
func DefaultFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dosrouter", "spending.json")
}

// NewFileStorage creates a FileSpendControlStorage at the default path.
func NewFileStorage() *FileSpendControlStorage {
	return &FileSpendControlStorage{Path: DefaultFilePath()}
}

func (fs *FileSpendControlStorage) Save(state persistedState) error {
	dir := filepath.Dir(fs.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fs.Path, data, 0o644)
}

func (fs *FileSpendControlStorage) Load() (*persistedState, error) {
	data, err := os.ReadFile(fs.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// InMemorySpendControlStorage keeps state in memory only.
type InMemorySpendControlStorage struct {
	mu    sync.Mutex
	state *persistedState
}

func (m *InMemorySpendControlStorage) Save(state persistedState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := state
	m.state = &cp
	return nil
}

func (m *InMemorySpendControlStorage) Load() (*persistedState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state, nil
}

// ---------- Helpers ----------

// formatDuration produces a human-readable duration string (e.g. "1h 23m 45s").
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	case h > 0:
		return fmt.Sprintf("%dh %ds", h, s)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
