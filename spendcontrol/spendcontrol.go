// Package spendcontrol enforces spending limits for LLM requests using
// rolling time windows and per-session budgets.
package spendcontrol

import (
	"encoding/json"
	"fmt"
	"math"
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

func validWindow(w SpendWindow) bool {
	return w == WindowPerRequest || w == WindowHourly || w == WindowDaily || w == WindowSession
}

func validAmount(amount float64) bool {
	return amount >= 0 && !math.IsNaN(amount) && !math.IsInf(amount, 0)
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
	reservations map[uint64]float64
	nextID       uint64
	storageErr   error
}

// New loads persisted limits and history. Session totals start at zero.
func New(storage SpendControlStorage) (*SpendControl, error) {
	sc := &SpendControl{
		limits:       make(SpendLimits),
		storage:      storage,
		reservations: make(map[uint64]float64),
	}
	if err := sc.load(); err != nil {
		return nil, fmt.Errorf("spendcontrol: load: %w", err)
	}
	return sc, nil
}

// SetLimit sets a finite, nonnegative maximum USD amount for a known window.
func (sc *SpendControl) SetLimit(window SpendWindow, amount float64) error {
	if !validWindow(window) || !validAmount(amount) {
		return fmt.Errorf("spendcontrol: invalid spending limit")
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.limits[window] = amount
	return sc.saveLocked()
}

// ClearLimit removes the limit for the given window.
func (sc *SpendControl) ClearLimit(window SpendWindow) error {
	if !validWindow(window) {
		return fmt.Errorf("spendcontrol: invalid spending window")
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	delete(sc.limits, window)
	return sc.saveLocked()
}

// GetLimits returns a copy of the current limits.
func (sc *SpendControl) GetLimits() SpendLimits {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return cloneState(persistedState{Limits: sc.limits}).Limits
}

// Check evaluates a cost against committed spend and all pending reservations.
// Use Reserve before dispatching a request to make admission atomic.
func (sc *SpendControl) Check(estimatedCost float64) CheckResult {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.checkLocked(estimatedCost, time.Now())
}

// Reserve atomically checks and holds a request's estimated maximum cost.
// Pending reservations count in every cumulative window until Commit or Release,
// even if a request remains in flight longer than a rolling window.
func (sc *SpendControl) Reserve(cost float64) (uint64, CheckResult) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	result := sc.checkLocked(cost, time.Now())
	if !result.Allowed {
		return 0, result
	}
	if sc.nextID == ^uint64(0) {
		return 0, CheckResult{Reason: "spend reservation identifiers exhausted"}
	}
	sc.nextID++
	sc.reservations[sc.nextID] = cost
	return sc.nextID, result
}

func (sc *SpendControl) checkLocked(cost float64, now time.Time) CheckResult {
	if !validAmount(cost) {
		return CheckResult{Reason: "request cost must be finite and nonnegative"}
	}
	if sc.storageErr != nil {
		return CheckResult{Reason: "spending state could not be persisted"}
	}
	pending := sc.pendingLocked()
	if !validAmount(sc.sessionSpent + pending + cost) {
		return CheckResult{Reason: "spending total exceeds supported range"}
	}
	remaining := math.Inf(1)
	for _, w := range []SpendWindow{WindowPerRequest, WindowSession, WindowHourly, WindowDaily} {
		limit, ok := sc.limits[w]
		if !ok {
			continue
		}
		spent := sc.spentLocked(w, now, pending)
		available := math.Max(0, limit-spent)
		if !validAmount(spent) || cost > limit-spent {
			result := CheckResult{
				BlockedBy: w,
				Remaining: available,
				Reason:    fmt.Sprintf("%s spend $%.4f + $%.4f would exceed limit $%.4f", w, spent, cost, limit),
			}
			if d := windowDuration(w); d > 0 {
				if oldest, ok := sc.oldestInWindow(now.Add(-d)); ok {
					result.ResetIn = formatDuration(oldest.Add(d).Sub(now))
				}
			}
			return result
		}
		remaining = math.Min(remaining, available)
	}
	if math.IsInf(remaining, 1) {
		remaining = 0
	}
	return CheckResult{Allowed: true, Remaining: remaining}
}

// Commit replaces a reservation with actual spend exactly once. Actual spend is
// recorded even when it exceeds the estimate; subsequent admission sees it.
// Invalid amounts retain the reservation. A persistence failure retains the
// recorded spend in memory and blocks admission until a later successful save.
func (sc *SpendControl) Commit(id uint64, amount float64, model, action string) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if _, ok := sc.reservations[id]; !ok {
		return fmt.Errorf("spendcontrol: unknown reservation")
	}
	if !validAmount(amount) || !validAmount(sc.sessionSpent+amount) {
		return fmt.Errorf("spendcontrol: invalid spending amount")
	}
	delete(sc.reservations, id)
	sc.recordLocked(amount, model, action)
	return sc.saveLocked()
}

// Release cancels a reservation when it is known that no spend occurred.
// Releasing an already settled or unknown reservation has no effect.
func (sc *SpendControl) Release(id uint64) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	delete(sc.reservations, id)
}

// Record logs completed spend without an admission check. New request paths
// should use Reserve and Commit to avoid check-then-record concurrency races.
func (sc *SpendControl) Record(amount float64, model, action string) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if !validAmount(amount) || !validAmount(sc.sessionSpent+amount) {
		return fmt.Errorf("spendcontrol: invalid spending amount")
	}
	sc.recordLocked(amount, model, action)
	return sc.saveLocked()
}

func (sc *SpendControl) recordLocked(amount float64, model, action string) {
	sc.history = append(sc.history, SpendRecord{
		Timestamp: time.Now(), Amount: amount, Model: model, Action: action,
	})
	sc.sessionSpent += amount
	sc.sessionCalls++
}

func (sc *SpendControl) pendingLocked() float64 {
	var pending float64
	for _, amount := range sc.reservations {
		pending += amount
	}
	return pending
}

func (sc *SpendControl) spentLocked(window SpendWindow, now time.Time, pending float64) float64 {
	switch window {
	case WindowPerRequest:
		return 0
	case WindowSession:
		return sc.sessionSpent + pending
	default:
		spent := pending
		cutoff := now.Add(-windowDuration(window))
		for _, record := range sc.history {
			if record.Timestamp.After(cutoff) {
				spent += record.Amount
			}
		}
		return spent
	}
}

// GetSpending returns committed and reserved spend in each active window.
func (sc *SpendControl) GetSpending() map[SpendWindow]float64 {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	now, pending := time.Now(), sc.pendingLocked()
	out := make(map[SpendWindow]float64)
	for window := range sc.limits {
		if window != WindowPerRequest {
			out[window] = sc.spentLocked(window, now, pending)
		}
	}
	return out
}

// GetRemaining returns budget available after committed and reserved spend.
func (sc *SpendControl) GetRemaining() map[SpendWindow]float64 {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	now, pending := time.Now(), sc.pendingLocked()
	out := make(map[SpendWindow]float64)
	for window, limit := range sc.limits {
		out[window] = limit - sc.spentLocked(window, now, pending)
	}
	return out
}

// StatusEntry holds limit, spent, and remaining for a single window.
type StatusEntry struct {
	Limit     float64 `json:"limit"`
	Spent     float64 `json:"spent"`
	Remaining float64 `json:"remaining"`
}

// GetStatus returns one consistent snapshot of limits, spend, and remaining.
func (sc *SpendControl) GetStatus() map[SpendWindow]StatusEntry {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	now, pending := time.Now(), sc.pendingLocked()
	out := make(map[SpendWindow]StatusEntry, len(sc.limits))
	for window, limit := range sc.limits {
		spent := sc.spentLocked(window, now, pending)
		out[window] = StatusEntry{Limit: limit, Spent: spent, Remaining: limit - spent}
	}
	return out
}

// GetHistory returns a copy of all completed spending records.
func (sc *SpendControl) GetHistory() []SpendRecord {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	out := make([]SpendRecord, len(sc.history))
	copy(out, sc.history)
	return out
}

// Cleanup prunes completed records older than 24 hours. Pending reservations and
// the current session's totals remain intact.
func (sc *SpendControl) Cleanup() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	cutoff := time.Now().Add(-24 * time.Hour)
	kept := make([]SpendRecord, 0, len(sc.history))
	for _, record := range sc.history {
		if record.Timestamp.After(cutoff) {
			kept = append(kept, record)
		}
	}
	sc.history = kept
	return sc.saveLocked()
}

func (sc *SpendControl) oldestInWindow(cutoff time.Time) (time.Time, bool) {
	var oldest time.Time
	for _, record := range sc.history {
		if record.Timestamp.After(cutoff) && (oldest.IsZero() || record.Timestamp.Before(oldest)) {
			oldest = record.Timestamp
		}
	}
	return oldest, !oldest.IsZero()
}

// persistedState intentionally excludes process-local sessions and reservations.
type persistedState struct {
	Limits  SpendLimits   `json:"limits"`
	History []SpendRecord `json:"history"`
}

func cloneState(state persistedState) persistedState {
	cp := persistedState{Limits: make(SpendLimits, len(state.Limits))}
	for window, amount := range state.Limits {
		cp.Limits[window] = amount
	}
	cp.History = append([]SpendRecord(nil), state.History...)
	return cp
}

// saveLocked serializes saves with mutations, preventing an older snapshot from
// overwriting a newer one. Storage receives a detached snapshot.
func (sc *SpendControl) saveLocked() error {
	if sc.storage == nil {
		return nil
	}
	sc.storageErr = sc.storage.Save(cloneState(persistedState{Limits: sc.limits, History: sc.history}))
	if sc.storageErr != nil {
		return fmt.Errorf("spendcontrol: save: %w", sc.storageErr)
	}
	return nil
}

func (sc *SpendControl) load() error {
	if sc.storage == nil {
		return nil
	}
	state, err := sc.storage.Load()
	if err != nil || state == nil {
		return err
	}
	for window, limit := range state.Limits {
		if !validWindow(window) || !validAmount(limit) {
			return fmt.Errorf("invalid persisted spending limit")
		}
	}
	var total float64
	for _, record := range state.History {
		if !validAmount(record.Amount) || record.Timestamp.IsZero() {
			return fmt.Errorf("invalid persisted spending record")
		}
		total += record.Amount
		if !validAmount(total) {
			return fmt.Errorf("persisted spending total exceeds supported range")
		}
	}
	cp := cloneState(*state)
	sc.limits, sc.history = cp.Limits, cp.History
	return nil
}

// SpendControlStorage persists spending state. Implementations must not call back
// into the controller while saving; Save executes inside the controller's lock.
type SpendControlStorage interface {
	Save(state persistedState) error
	Load() (*persistedState, error)
}

// FileSpendControlStorage reads and writes JSON to disk.
type FileSpendControlStorage struct {
	Path string
}

// DefaultFilePath returns ~/.openclaw/DOS/spending.json.
func DefaultFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".openclaw", "DOS", "spending.json")
}

// NewFileStorage creates a FileSpendControlStorage at the default path.
func NewFileStorage() *FileSpendControlStorage {
	return &FileSpendControlStorage{Path: DefaultFilePath()}
}

// Save writes a complete temporary file before replacing the previous state.
func (fs *FileSpendControlStorage) Save(state persistedState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(fs.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".spending-*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(file.Name()) }()
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), fs.Path)
}

func (fs *FileSpendControlStorage) Load() (*persistedState, error) {
	data, err := os.ReadFile(fs.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	// Pointer amounts distinguish an explicit zero from a missing or null
	// value, which could otherwise silently erase previously recorded spend.
	var input struct {
		Limits  map[SpendWindow]*float64 `json:"limits"`
		History []struct {
			Timestamp time.Time `json:"timestamp"`
			Amount    *float64  `json:"amount"`
			Model     string    `json:"model"`
			Action    string    `json:"action"`
		} `json:"history"`
	}
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, err
	}
	if input.Limits == nil {
		return nil, fmt.Errorf("missing or null persisted spending limits")
	}
	state := persistedState{Limits: make(SpendLimits, len(input.Limits))}
	for window, amount := range input.Limits {
		if amount == nil {
			return nil, fmt.Errorf("null persisted spending limit")
		}
		state.Limits[window] = *amount
	}
	for _, record := range input.History {
		if record.Amount == nil {
			return nil, fmt.Errorf("missing or null persisted spending amount")
		}
		state.History = append(state.History, SpendRecord{
			Timestamp: record.Timestamp, Amount: *record.Amount,
			Model: record.Model, Action: record.Action,
		})
	}
	return &state, nil
}

// InMemorySpendControlStorage keeps detached state in memory only.
type InMemorySpendControlStorage struct {
	mu    sync.Mutex
	state *persistedState
}

func (m *InMemorySpendControlStorage) Save(state persistedState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := cloneState(state)
	m.state = &cp
	return nil
}

func (m *InMemorySpendControlStorage) Load() (*persistedState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == nil {
		return nil, nil
	}
	cp := cloneState(*m.state)
	return &cp, nil
}

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
