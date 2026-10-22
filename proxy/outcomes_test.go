package proxy

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestOutcomeWindowExpiryAndKinds(t *testing.T) {
	m := &outcomeMemory{}
	now := time.Now()
	chain := []string{"bad", "good"}
	for i := 0; i < 3; i++ {
		m.record("bad", "tools", false, now)
	}
	if got := m.demote(chain, "tools", "", now); !reflect.DeepEqual(got, []string{"good", "bad"}) {
		t.Fatalf("demotion=%v", got)
	}
	for _, tc := range []struct {
		kind, keep string
		at         time.Time
	}{{"chat", "", now}, {"tools", "bad", now}, {"tools", "", now.Add(30 * time.Minute)}} {
		if got := m.demote(chain, tc.kind, tc.keep, tc.at); !reflect.DeepEqual(got, chain) {
			t.Fatalf("unexpected demotion=%v", got)
		}
	}
	for i := 0; i < 600; i++ {
		m.record(fmt.Sprint(i), "chat", false, now)
	}
	if len(m.entries) != 512 {
		t.Fatalf("unbounded outcomes: %d", len(m.entries))
	}
}

func TestOutcomeMemoryConcurrent(t *testing.T) {
	var memory outcomeMemory
	var workers sync.WaitGroup
	for i := 0; i < 12; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 30; j++ {
				memory.record("model", "chat", j%2 == 0, time.Now())
				memory.demote([]string{"model", "other"}, "chat", "", time.Now())
			}
		}()
	}
	workers.Wait()
	if len(memory.entries["model|chat"].samples) != 10 {
		t.Fatal("outcome window was not bounded")
	}
}
