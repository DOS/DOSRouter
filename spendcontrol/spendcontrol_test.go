package spendcontrol

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestControl(t *testing.T, storage SpendControlStorage) *SpendControl {
	t.Helper()
	sc, err := New(storage)
	if err != nil {
		t.Fatal(err)
	}
	return sc
}

func TestConcurrentReservationsCannotOversubscribe(t *testing.T) {
	for _, window := range []SpendWindow{WindowSession, WindowHourly, WindowDaily} {
		t.Run(string(window), func(t *testing.T) {
			sc := newTestControl(t, nil)
			if err := sc.SetLimit(window, 1); err != nil {
				t.Fatal(err)
			}
			var wg sync.WaitGroup
			ids := make(chan uint64, 100)
			start := make(chan struct{})
			for i := 0; i < 100; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					if id, result := sc.Reserve(0.25); result.Allowed {
						ids <- id
					}
				}()
			}
			close(start)
			wg.Wait()
			close(ids)
			seen := make(map[uint64]bool)
			for id := range ids {
				if id == 0 || seen[id] {
					t.Fatalf("invalid or reused reservation: %d", id)
				}
				seen[id] = true
			}
			if len(seen) != 4 {
				t.Fatalf("admitted %d requests, want 4", len(seen))
			}
			if result := sc.Check(0.25); result.Allowed || result.BlockedBy != window {
				t.Fatalf("pending spend did not block: %+v", result)
			}
			status := sc.GetStatus()[window]
			if status.Spent != 1 || status.Remaining != 0 {
				t.Fatalf("inconsistent pending status: %+v", status)
			}
		})
	}
}

func TestReservationsSettleExactlyOnce(t *testing.T) {
	sc := newTestControl(t, nil)
	if err := sc.SetLimit(WindowDaily, 1); err != nil {
		t.Fatal(err)
	}
	first, firstResult := sc.Reserve(0.75)
	second, secondResult := sc.Reserve(0.25)
	if !firstResult.Allowed || !secondResult.Allowed {
		t.Fatal("initial reservations denied")
	}
	if err := sc.Commit(first, 0.5, "test-model", "chat"); err != nil {
		t.Fatal(err)
	}
	sc.Release(second)
	sc.Release(second)
	sc.Release(first)
	if err := sc.Commit(first, 0.5, "test-model", "chat"); err == nil {
		t.Fatal("duplicate commit accepted")
	}
	if err := sc.Commit(second, 0.25, "test-model", "chat"); err == nil {
		t.Fatal("released reservation committed")
	}
	if got := sc.GetSpending()[WindowDaily]; got != 0.5 {
		t.Fatalf("spent = %v, want 0.5", got)
	}
	if got := sc.GetRemaining()[WindowDaily]; got != 0.5 {
		t.Fatalf("remaining = %v, want 0.5", got)
	}
	history := sc.GetHistory()
	if len(history) != 1 || history[0].Amount != 0.5 || history[0].Model != "test-model" || history[0].Action != "chat" {
		t.Fatalf("unexpected committed history: %+v", history)
	}
}

func TestPendingReservationsNeverAgeOut(t *testing.T) {
	for _, window := range []SpendWindow{WindowHourly, WindowDaily} {
		t.Run(string(window), func(t *testing.T) {
			sc := newTestControl(t, nil)
			if err := sc.SetLimit(window, 1); err != nil {
				t.Fatal(err)
			}
			id, result := sc.Reserve(0.75)
			if !result.Allowed {
				t.Fatal(result.Reason)
			}
			if err := sc.Record(0.25, "test", "chat"); err != nil {
				t.Fatal(err)
			}
			sc.mu.Lock()
			future := time.Now().Add(2 * windowDuration(window))
			blocked := sc.checkLocked(0.5, future)
			allowed := sc.checkLocked(0.25, future)
			sc.history[0].Timestamp = time.Now().Add(-48 * time.Hour)
			sc.mu.Unlock()
			if blocked.Allowed || !allowed.Allowed {
				t.Fatalf("pending spend aged out or history did not expire: blocked=%+v allowed=%+v", blocked, allowed)
			}
			if err := sc.Cleanup(); err != nil {
				t.Fatal(err)
			}
			if result := sc.Check(0.5); result.Allowed {
				t.Fatal("cleanup removed pending reservation")
			}
			sc.Release(id)
			if result := sc.Check(1); !result.Allowed {
				t.Fatalf("released reservation still blocks: %+v", result)
			}
		})
	}
}

func TestInvalidAmountsFailClosed(t *testing.T) {
	sc := newTestControl(t, nil)
	if err := sc.SetLimit(WindowSession, 1); err != nil {
		t.Fatal(err)
	}
	id, result := sc.Reserve(0.75)
	if !result.Allowed {
		t.Fatal(result.Reason)
	}
	for _, amount := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if sc.Check(amount).Allowed {
			t.Fatalf("invalid check accepted: %v", amount)
		}
		if nextID, result := sc.Reserve(amount); result.Allowed || nextID != 0 {
			t.Fatalf("invalid reservation accepted: %v", amount)
		}
		if err := sc.Commit(id, amount, "", ""); err == nil {
			t.Fatalf("invalid commit accepted: %v", amount)
		}
		if err := sc.Record(amount, "", ""); err == nil {
			t.Fatalf("invalid record accepted: %v", amount)
		}
		if err := sc.SetLimit(WindowSession, amount); err == nil {
			t.Fatalf("invalid limit accepted: %v", amount)
		}
	}
	if err := sc.SetLimit(SpendWindow("typo"), 1); err == nil {
		t.Fatal("unknown spending window accepted")
	}
	if len(sc.GetHistory()) != 0 || sc.GetLimits()[WindowSession] != 1 || sc.GetSpending()[WindowSession] != 0.75 {
		t.Fatal("invalid input mutated valid state")
	}
	if err := sc.Commit(id, 0.5, "", ""); err != nil {
		t.Fatalf("invalid commit destroyed the reservation: %v", err)
	}
}

func TestActualSpendAboveEstimateIsRetained(t *testing.T) {
	sc := newTestControl(t, nil)
	if err := sc.SetLimit(WindowDaily, 1); err != nil {
		t.Fatal(err)
	}
	id, result := sc.Reserve(0.5)
	if !result.Allowed {
		t.Fatal(result.Reason)
	}
	if err := sc.Commit(id, 1.5, "", ""); err != nil {
		t.Fatal(err)
	}
	if sc.Check(0).Allowed || sc.GetSpending()[WindowDaily] != 1.5 {
		t.Fatal("actual overspend was dropped")
	}
}

type failingStorage struct {
	state *persistedState
	fail  bool
}

func (s *failingStorage) Save(state persistedState) error {
	if s.fail {
		return errors.New("injected storage failure")
	}
	cp := cloneState(state)
	s.state = &cp
	return nil
}

func (s *failingStorage) Load() (*persistedState, error) { return s.state, nil }

func TestPersistenceFailureRetainsSpendAndBlocksAdmission(t *testing.T) {
	storage := &failingStorage{}
	sc := newTestControl(t, storage)
	if err := sc.SetLimit(WindowDaily, 1); err != nil {
		t.Fatal(err)
	}
	id, _ := sc.Reserve(0.75)
	storage.fail = true
	if err := sc.Commit(id, 0.5, "test", "chat"); err == nil {
		t.Fatal("commit did not report persistence failure")
	}
	if got := sc.GetSpending()[WindowDaily]; got != 0.5 {
		t.Fatalf("lost completed spend on failed save: %v", got)
	}
	if sc.Check(0).Allowed {
		t.Fatal("admission remained open after persistence failure")
	}
	if id, result := sc.Reserve(0); id != 0 || result.Allowed {
		t.Fatal("reservation accepted after persistence failure")
	}
	storage.fail = false
	if err := sc.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if !sc.Check(0.5).Allowed || sc.Check(0.75).Allowed {
		t.Fatal("successful save did not recover the correct remaining budget")
	}
	if len(storage.state.History) != 1 || storage.state.History[0].Amount != 0.5 {
		t.Fatal("recovered storage lost committed spend")
	}
}

func TestMemoryStorageDoesNotShareSnapshots(t *testing.T) {
	storage := &InMemorySpendControlStorage{}
	original := persistedState{
		Limits:  SpendLimits{WindowDaily: 1},
		History: []SpendRecord{{Timestamp: time.Now(), Amount: 0.25}},
	}
	if err := storage.Save(original); err != nil {
		t.Fatal(err)
	}
	original.Limits[WindowDaily] = 100
	original.History[0].Amount = 100
	loaded, err := storage.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Limits[WindowDaily] != 1 || loaded.History[0].Amount != 0.25 {
		t.Fatal("saved state aliases caller memory")
	}
	loaded.Limits[WindowDaily] = 200
	loaded.History[0].Amount = 200
	again, err := storage.Load()
	if err != nil {
		t.Fatal(err)
	}
	if again.Limits[WindowDaily] != 1 || again.History[0].Amount != 0.25 {
		t.Fatal("loaded state aliases stored memory")
	}
}

// Retaining every passed snapshot detects sharing with later controller writes.
type snapshotStorage struct {
	snapshots []persistedState
}

func (s *snapshotStorage) Save(state persistedState) error {
	s.snapshots = append(s.snapshots, state)
	return nil
}
func (s *snapshotStorage) Load() (*persistedState, error) { return nil, nil }

func TestConcurrentMutationsPersistDetachedOrderedSnapshots(t *testing.T) {
	storage := &snapshotStorage{}
	sc := newTestControl(t, storage)
	if err := sc.SetLimit(WindowDaily, 1); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := sc.Record(0.25, "test", "chat"); err != nil {
				t.Error(err)
			}
			status := sc.GetStatus()[WindowDaily]
			if status.Limit-status.Spent != status.Remaining {
				t.Errorf("inconsistent status: %+v", status)
			}
		}()
	}
	wg.Wait()
	if err := sc.SetLimit(WindowDaily, 20); err != nil {
		t.Fatal(err)
	}
	if len(storage.snapshots) != 42 || storage.snapshots[0].Limits[WindowDaily] != 1 {
		t.Fatal("limit snapshots were lost or mutated")
	}
	for index := 1; index <= 40; index++ {
		if len(storage.snapshots[index].History) != index {
			t.Fatalf("out-of-order snapshot %d has %d records", index, len(storage.snapshots[index].History))
		}
	}
	sc.mu.Lock()
	sc.history[0].Amount = 100
	sc.mu.Unlock()
	if storage.snapshots[1].History[0].Amount != 0.25 {
		t.Fatal("history snapshots alias controller memory")
	}
}

func TestRestartResetsSessionButPreservesRollingSpend(t *testing.T) {
	storage := &InMemorySpendControlStorage{}
	first := newTestControl(t, storage)
	for _, window := range []SpendWindow{WindowSession, WindowHourly, WindowDaily} {
		if err := first.SetLimit(window, 1); err != nil {
			t.Fatal(err)
		}
	}
	if err := first.Record(0.75, "test", "chat"); err != nil {
		t.Fatal(err)
	}
	second := newTestControl(t, storage)
	spending := second.GetSpending()
	if spending[WindowSession] != 0 || spending[WindowHourly] != 0.75 || spending[WindowDaily] != 0.75 {
		t.Fatalf("unexpected restart totals: %+v", spending)
	}
}

func TestFileStorageRejectsCorruptionAndPreservesLastGoodWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spending.json")
	storage := &FileSpendControlStorage{Path: path}
	sc := newTestControl(t, storage)
	if err := sc.SetLimit(WindowDaily, 1); err != nil {
		t.Fatal(err)
	}
	if err := sc.Record(0.25, "test", "chat"); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Save(persistedState{Limits: SpendLimits{WindowDaily: math.NaN()}}); err == nil {
		t.Fatal("invalid JSON amount saved")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("failed save changed the previous state")
	}
	if restarted := newTestControl(t, storage); restarted.GetSpending()[WindowDaily] != 0.25 {
		t.Fatal("file round trip lost spending")
	}
	for _, invalid := range []string{
		`{`,
		`null`,
		`{}`,
		`{"limits":{"daily":-1},"history":[]}`,
		`{"limits":{"typo":1},"history":[]}`,
		`{"limits":null,"history":[]}`,
		`{"limits":{"daily":null},"history":[]}`,
		`{"limits":{"daily":1},"history":[{"amount":0.5}]}`,
		`{"limits":{"daily":1},"history":[{"timestamp":"2026-01-01T00:00:00Z","amount":null}]}`,
		`{"limits":{"daily":1},"history":[{"timestamp":"2026-01-01T00:00:00Z","amount":-1}]}`,
	} {
		if err := os.WriteFile(path, []byte(invalid), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := New(storage); err == nil {
			t.Errorf("corrupt persisted state accepted: %s", invalid)
		}
	}
}

func TestPerRequestLimitDoesNotCombineSeparateReservations(t *testing.T) {
	sc := newTestControl(t, nil)
	if err := sc.SetLimit(WindowPerRequest, 0.5); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, result := sc.Reserve(0.5); !result.Allowed {
			t.Fatalf("per-request cap incorrectly accumulated reservations: %+v", result)
		}
	}
	if _, result := sc.Reserve(0.75); result.Allowed || result.BlockedBy != WindowPerRequest {
		t.Fatalf("per-request limit not enforced: %+v", result)
	}
}

func TestReservationTotalOverflowFailsClosed(t *testing.T) {
	sc := newTestControl(t, nil)
	id, result := sc.Reserve(math.MaxFloat64)
	if !result.Allowed {
		t.Fatal(result.Reason)
	}
	if _, result := sc.Reserve(math.MaxFloat64); result.Allowed {
		t.Fatal("overflowing reservation total accepted")
	}
	sc.Release(id)
	if _, result := sc.Reserve(0); !result.Allowed {
		t.Fatal("zero-cost reservation should remain valid")
	}
}
