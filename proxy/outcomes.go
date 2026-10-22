package proxy

import (
	"regexp"
	"sync"
	"time"
)

type modelOutcome struct {
	at time.Time
	ok bool
}
type outcomeEntry struct {
	samples []modelOutcome
	touched uint64
}

// outcomeMemory is a bounded, process-local hint, scoped to one upstream server.
type outcomeMemory struct {
	mu       sync.Mutex
	entries  map[string]outcomeEntry
	sequence uint64
}

func (m *outcomeMemory) recent(key string, now time.Time) outcomeEntry {
	entry := m.entries[key]
	live := entry.samples[:0]
	for _, sample := range entry.samples {
		if now.Sub(sample.at) < 30*time.Minute {
			live = append(live, sample)
		}
	}
	entry.samples = live
	if len(live) == 0 {
		delete(m.entries, key)
	} else {
		m.entries[key] = entry
	}
	return entry
}

func (m *outcomeMemory) record(model, kind string, ok bool, now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.entries == nil {
		m.entries = make(map[string]outcomeEntry)
	}
	key := model + "|" + kind
	entry := m.recent(key, now)
	entry.samples = append(entry.samples, modelOutcome{now, ok})
	if len(entry.samples) > 10 {
		entry.samples = entry.samples[len(entry.samples)-10:]
	}
	m.sequence++
	entry.touched = m.sequence
	m.entries[key] = entry
	if len(m.entries) > 512 {
		oldest := ""
		var seq uint64 = ^uint64(0)
		for k, e := range m.entries {
			if e.touched < seq {
				oldest = k
				seq = e.touched
			}
		}
		delete(m.entries, oldest)
	}
}

func (m *outcomeMemory) demote(chain []string, kind, keep string, now time.Time) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	good, bad := []string{}, []string{}
	for _, model := range chain {
		samples := m.recent(model+"|"+kind, now).samples
		failures := 0
		for _, sample := range samples {
			if !sample.ok {
				failures++
			}
		}
		if model != keep && failures >= 3 && failures*2 >= len(samples) {
			bad = append(bad, model)
		} else {
			good = append(good, model)
		}
	}
	if len(good) == 0 {
		return chain
	}
	return append(good, bad...)
}

var paymentFailurePattern = regexp.MustCompile(`(?is)payment.*(?:verification|settlement).*failed|insufficient.*funds|transaction_simulation_failed|payment_in_doubt`)

func paymentFailure(body []byte) bool { return paymentFailurePattern.Match(body) }
