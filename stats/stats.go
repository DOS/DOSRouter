// Package stats parses JSONL usage log files and computes aggregate statistics
// with ASCII-formatted output.
package stats

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/DOS/DOSRouter/logger"
)

// TierStats holds per-tier aggregate data.
type TierStats struct {
	Count      int     `json:"count"`
	Cost       float64 `json:"cost"`
	Percentage float64 `json:"percentage"`
}

// ModelStats holds per-model aggregate data.
type ModelStats struct {
	Count      int     `json:"count"`
	Cost       float64 `json:"cost"`
	Percentage float64 `json:"percentage"`
}

// DayStats holds one day of aggregated usage.
type DayStats struct {
	Date              string                `json:"date"`
	TotalRequests     int                   `json:"totalRequests"`
	TotalCost         float64               `json:"totalCost"`
	TotalBaselineCost float64               `json:"totalBaselineCost"`
	TotalSavings      float64               `json:"totalSavings"`
	AvgLatencyMs      float64               `json:"avgLatencyMs"`
	ByTier            map[string]TierStats  `json:"byTier"`
	ByModel           map[string]ModelStats `json:"byModel"`
}

// AggregatedStats is the top-level stats result.
type AggregatedStats struct {
	Period              string                `json:"period"`
	TotalRequests       int                   `json:"totalRequests"`
	TotalCost           float64               `json:"totalCost"`
	TotalBaselineCost   float64               `json:"totalBaselineCost"`
	TotalSavings        float64               `json:"totalSavings"`
	SavingsPercentage   float64               `json:"savingsPercentage"`
	AvgLatencyMs        float64               `json:"avgLatencyMs"`
	AvgCostPerRequest   float64               `json:"avgCostPerRequest"`
	ByTier              map[string]TierStats  `json:"byTier"`
	ByModel             map[string]ModelStats `json:"byModel"`
	DailyBreakdown      []DayStats            `json:"dailyBreakdown"`
	EntriesWithBaseline int                   `json:"entriesWithBaseline"`
}

type logEntry struct {
	Timestamp    string  `json:"timestamp"`
	Model        string  `json:"model"`
	Tier         string  `json:"tier"`
	Cost         float64 `json:"cost"`
	BaselineCost float64 `json:"baselineCost"`
	Savings      float64 `json:"savings"`
	LatencyMs    int64   `json:"latencyMs"`
	Status       string  `json:"status"`
	InputTokens  int     `json:"inputTokens"`
	OutputTokens int     `json:"outputTokens"`
}

func parseLogFile(filePath string) []logEntry {
	data, err := os.ReadFile(filePath)
	if err != nil { return nil }
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	entries := make([]logEntry, 0, len(lines))
	for _, line := range lines {
		if line == "" { continue }
		var e logEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil { continue }
		if e.Timestamp == "" { e.Timestamp = time.Now().Format(time.RFC3339) }
		if e.Model == "" { e.Model = "unknown" }
		if e.Tier == "" { e.Tier = "UNKNOWN" }
		if e.BaselineCost == 0 { e.BaselineCost = e.Cost }
		entries = append(entries, e)
	}
	return entries
}

func getLogFiles() []string {
	dir := logger.LogDir()
	dirEntries, err := os.ReadDir(dir)
	if err != nil { return nil }
	var files []string
	for _, de := range dirEntries {
		name := de.Name()
		if strings.HasPrefix(name, "usage-") && strings.HasSuffix(name, ".jsonl") {
			files = append(files, name)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	return files
}

func aggregateDay(date string, entries []logEntry) DayStats {
	byTier := make(map[string]TierStats)
	byModel := make(map[string]ModelStats)
	var totalLatency int64
	var totalCost, totalBaselineCost float64
	for _, e := range entries {
		ts := byTier[e.Tier]; ts.Count++; ts.Cost += e.Cost; byTier[e.Tier] = ts
		ms := byModel[e.Model]; ms.Count++; ms.Cost += e.Cost; byModel[e.Model] = ms
		totalLatency += e.LatencyMs
		totalCost += e.Cost
		totalBaselineCost += e.BaselineCost
	}
	avgLat := 0.0
	if len(entries) > 0 { avgLat = float64(totalLatency) / float64(len(entries)) }
	return DayStats{
		Date: date, TotalRequests: len(entries), TotalCost: totalCost,
		TotalBaselineCost: totalBaselineCost, TotalSavings: totalBaselineCost - totalCost,
		AvgLatencyMs: avgLat, ByTier: byTier, ByModel: byModel,
	}
}

// GetStats reads log files and returns aggregated statistics for the given number of days.
func GetStats(days int) AggregatedStats {
	if days <= 0 { days = 7 }
	logFiles := getLogFiles()
	if len(logFiles) > days { logFiles = logFiles[:days] }
	dir := logger.LogDir()
	var dailyBreakdown []DayStats
	allByTier := make(map[string]TierStats)
	allByModel := make(map[string]ModelStats)
	var totalRequests int
	var totalCost, totalBaselineCost, totalLatency float64
	for _, file := range logFiles {
		date := strings.TrimSuffix(strings.TrimPrefix(file, "usage-"), ".jsonl")
		entries := parseLogFile(filepath.Join(dir, file))
		if len(entries) == 0 { continue }
		day := aggregateDay(date, entries)
		dailyBreakdown = append(dailyBreakdown, day)
		totalRequests += day.TotalRequests
		totalCost += day.TotalCost
		totalBaselineCost += day.TotalBaselineCost
		totalLatency += day.AvgLatencyMs * float64(day.TotalRequests)
		for tier, ts := range day.ByTier {
			a := allByTier[tier]; a.Count += ts.Count; a.Cost += ts.Cost; allByTier[tier] = a
		}
		for model, ms := range day.ByModel {
			a := allByModel[model]; a.Count += ms.Count; a.Cost += ms.Cost; allByModel[model] = a
		}
	}
	for k, v := range allByTier {
		if totalRequests > 0 { v.Percentage = float64(v.Count) / float64(totalRequests) * 100 }
		allByTier[k] = v
	}
	for k, v := range allByModel {
		if totalRequests > 0 { v.Percentage = float64(v.Count) / float64(totalRequests) * 100 }
		allByModel[k] = v
	}
	totalSavings := totalBaselineCost - totalCost
	savingsPct := 0.0
	if totalBaselineCost > 0 { savingsPct = totalSavings / totalBaselineCost * 100 }
	avgLatency, avgCost := 0.0, 0.0
	if totalRequests > 0 {
		avgLatency = totalLatency / float64(totalRequests)
		avgCost = totalCost / float64(totalRequests)
	}
	var entriesWithBaseline int
	for _, day := range dailyBreakdown {
		if day.TotalBaselineCost != day.TotalCost { entriesWithBaseline += day.TotalRequests }
	}
	// Reverse so oldest first.
	for i, j := 0, len(dailyBreakdown)-1; i < j; i, j = i+1, j-1 {
		dailyBreakdown[i], dailyBreakdown[j] = dailyBreakdown[j], dailyBreakdown[i]
	}
	period := "today"
	if days != 1 { period = fmt.Sprintf("last %d days", days) }
	return AggregatedStats{
		Period: period, TotalRequests: totalRequests, TotalCost: totalCost,
		TotalBaselineCost: totalBaselineCost, TotalSavings: totalSavings,
		SavingsPercentage: savingsPct, AvgLatencyMs: avgLatency, AvgCostPerRequest: avgCost,
		ByTier: allByTier, ByModel: allByModel, DailyBreakdown: dailyBreakdown,
		EntriesWithBaseline: entriesWithBaseline,
	}
}

// FormatStatsASCII renders aggregated stats as an ASCII box art table.
func FormatStatsASCII(s AggregatedStats) string {
	var b strings.Builder
	topBot := "+--------------------------------------------------+"
	b.WriteString(topBot + "\n")
	b.WriteString(fmt.Sprintf("| Usage Stats (%s)%s|\n", s.Period, pad(36-len(s.Period))))
	b.WriteString(topBot + "\n")
	b.WriteString(fmt.Sprintf("| Requests:  %-37d|\n", s.TotalRequests))
	b.WriteString(fmt.Sprintf("| Cost:      $%-36.4f|\n", s.TotalCost))
	if s.EntriesWithBaseline > 0 {
		b.WriteString(fmt.Sprintf("| Baseline:  $%-36.4f|\n", s.TotalBaselineCost))
		b.WriteString(fmt.Sprintf("| Savings:   $%-33.4f(%4.1f%%)|\n", s.TotalSavings, s.SavingsPercentage))
	}
	b.WriteString(fmt.Sprintf("| Avg Cost:  $%-36.6f|\n", s.AvgCostPerRequest))
	b.WriteString(fmt.Sprintf("| Avg Latency: %-34.0fms|\n", s.AvgLatencyMs))
	b.WriteString(topBot + "\n")
	if len(s.ByTier) > 0 {
		b.WriteString("| Tier Breakdown:                                  |\n")
		b.WriteString("|   Tier       Count    Cost      Pct              |\n")
		for _, tier := range sortedKeys(s.ByTier) {
			ts := s.ByTier[tier]
			bar := makeBar(ts.Percentage, 15)
			b.WriteString(fmt.Sprintf("|   %-10s %5d  $%7.4f  %5.1f%% %s |\n", tier, ts.Count, ts.Cost, ts.Percentage, bar))
		}
		b.WriteString(topBot + "\n")
	}
	if len(s.ByModel) > 0 {
		b.WriteString("| Model Breakdown:                                 |\n")
		for _, model := range sortedKeysByCount(s.ByModel) {
			ms := s.ByModel[model]
			name := model
			if len(name) > 25 { name = name[:22] + "..." }
			b.WriteString(fmt.Sprintf("|   %-25s %4d  $%7.4f %5.1f%%|\n", name, ms.Count, ms.Cost, ms.Percentage))
		}
		b.WriteString(topBot + "\n")
	}
	return b.String()
}

// FormatRecentLogs renders individual log entries as a per-request table.
func FormatRecentLogs(days, limit int) string {
	if days <= 0 { days = 1 }
	if limit <= 0 { limit = 20 }
	logFiles := getLogFiles()
	if len(logFiles) > days { logFiles = logFiles[:days] }
	dir := logger.LogDir()
	var all []logEntry
	for _, file := range logFiles {
		all = append(all, parseLogFile(filepath.Join(dir, file))...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Timestamp > all[j].Timestamp })
	if len(all) > limit { all = all[:limit] }
	if len(all) == 0 { return "No recent logs found.\n" }
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Recent Requests (last %d):\n", len(all)))
	b.WriteString("+-----+----------------------+----------+----------+--------+\n")
	b.WriteString("|  #  | Model                | Tier     | Cost     | Lat ms |\n")
	b.WriteString("+-----+----------------------+----------+----------+--------+\n")
	for i, e := range all {
		model := e.Model
		if len(model) > 20 { model = model[:17] + "..." }
		b.WriteString(fmt.Sprintf("| %3d | %-20s | %-8s | $%7.5f | %6d |\n", i+1, model, e.Tier, e.Cost, e.LatencyMs))
	}
	b.WriteString("+-----+----------------------+----------+----------+--------+\n")
	return b.String()
}

// ClearStats deletes all log files in the usage log directory.
func ClearStats() error {
	dir := logger.LogDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) { return nil }
		return err
	}
	for _, de := range entries {
		name := de.Name()
		if strings.HasPrefix(name, "usage-") && strings.HasSuffix(name, ".jsonl") {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
	return nil
}

func pad(n int) string {
	if n <= 0 { return "" }
	return strings.Repeat(" ", n)
}

func makeBar(pct float64, maxWidth int) string {
	filled := int(math.Round(pct / 100.0 * float64(maxWidth)))
	if filled < 0 { filled = 0 }
	if filled > maxWidth { filled = maxWidth }
	return strings.Repeat("#", filled) + strings.Repeat(".", maxWidth-filled)
}

func sortedKeys(m map[string]TierStats) []string {
	keys := make([]string, 0, len(m))
	for k := range m { keys = append(keys, k) }
	sort.Strings(keys)
	return keys
}

func sortedKeysByCount(m map[string]ModelStats) []string {
	keys := make([]string, 0, len(m))
	for k := range m { keys = append(keys, k) }
	sort.Slice(keys, func(i, j int) bool { return m[keys[i]].Count > m[keys[j]].Count })
	return keys
}
