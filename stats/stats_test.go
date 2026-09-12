package stats

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetStatsBoundsEmptyPeriods(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name   string
		days   int
		period string
	}{
		{"negative defaults to week", -5, "last 7 days"},
		{"zero defaults to week", 0, "last 7 days"},
		{"one day", 1, "today"},
		{"month", 30, "last 30 days"},
		{"over a month", 31, "last 30 days"},
		{"maximum integer", int(^uint(0) >> 1), "last 30 days"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getStats(tt.days, dir)
			if got.Period != tt.period {
				t.Errorf("Period = %q, want %q", got.Period, tt.period)
			}
			if got.TotalRequests != 0 || got.TotalCost != 0 || len(got.DailyBreakdown) != 0 {
				t.Fatalf("empty log directory produced usage: %+v", got)
			}
		})
	}
}

func TestGetStatsReadsAtMostThirtyNewestLogFiles(t *testing.T) {
	dir := t.TempDir()
	first := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 35; i++ {
		date := first.AddDate(0, 0, i).Format("2006-01-02")
		entry := `{"timestamp":"` + date + `T12:00:00Z","model":"test/model","tier":"SIMPLE","cost":0.25,"baselineCost":1,"latencyMs":100}` + "\n"
		if err := os.WriteFile(filepath.Join(dir, "usage-"+date+".jsonl"), []byte(entry), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "unrelated.jsonl"), []byte("invalid log"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := getStats(365, dir)
	if got.Period != "last 30 days" || got.TotalRequests != 30 || len(got.DailyBreakdown) != 30 {
		t.Fatalf("GetStats exceeded the 30-file window: period=%q requests=%d days=%d", got.Period, got.TotalRequests, len(got.DailyBreakdown))
	}
	if got.DailyBreakdown[0].Date != first.AddDate(0, 0, 5).Format("2006-01-02") || got.DailyBreakdown[29].Date != first.AddDate(0, 0, 34).Format("2006-01-02") {
		t.Errorf("unexpected oldest/newest dates: %q / %q", got.DailyBreakdown[0].Date, got.DailyBreakdown[29].Date)
	}
	if got.TotalCost != 7.5 || got.TotalSavings != 22.5 || got.AvgLatencyMs != 100 {
		t.Errorf("aggregates included data outside the selected window: cost=%v savings=%v latency=%v", got.TotalCost, got.TotalSavings, got.AvgLatencyMs)
	}
}
