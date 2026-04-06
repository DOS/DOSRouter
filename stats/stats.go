// Package stats provides usage statistics aggregation and ASCII formatting.
// It reads JSONL log files produced by the logger package and generates
// terminal-friendly reports.
package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DOS/DOSRouter/logger"
)

// DailyStats holds aggregated stats for a single day.
type DailyStats struct {
	Date             string                        `json:"date"`
	TotalRequests    int                           `json:"totalRequests"`
	TotalCost        float64                       `json:"totalCost"`
	TotalBaselineCost float64                      `json:"totalBaselineCost"`
	TotalSavings     float64                       `json:"totalSavings"`
	AvgLatencyMs     float64                       `json:"avgLatencyMs"`
	ByTier           map[string]*TierModelStat     `json:"byTier"`
	ByModel          map[string]*TierModelStat     `json:"byModel"`
}

// TierModelStat holds count and cost for a tier or model.
type TierModelStat struct {
	Count      int     `json:"count"`
	Cost       float64 `json:"cost"`
	Percentage float64 `json:"percentage,omitempty"`
}

// AggregatedStats holds stats aggregated over multiple days.
type AggregatedStats struct {
	Period              string                        `json:"period"`
	TotalRequests       int                           `json:"totalRequests"`
	TotalCost           float64                       `json:"totalCost"`
	TotalBaselineCost   float64                       `json:"totalBaselineCost"`
	TotalSavings        float64                       `json:"totalSavings"`
	SavingsPercentage   float64                       `json:"savingsPercentage"`
	AvgLatencyMs        float64                       `json:"avgLatencyMs"`
	AvgCostPerRequest   float64                       `json:"avgCostPerRequest"`
	ByTier              map[string]*TierModelStat     `json:"byTier"`
	ByModel             map[string]*TierModelStat     `json:"byModel"`
	DailyBreakdown      []DailyStats                  `json:"dailyBreakdown"`
	EntriesWithBaseline int                           `json:"entriesWithBaseline"`
}

// parseLogFile reads and parses a JSONL log file.
func parseLogFile(filePath string) []logger.UsageEntry {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var entries []logger.UsageEntry
	for _, line := range lines {
		if line == "" {
			continue
		}
		var entry logger.UsageEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// getLogFiles returns available log files sorted newest first.
func getLogFiles() []string {
	logDir := logger.GetLogDir()
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil
	}

	var files []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "usage-") && strings.HasSuffix(name, ".jsonl") {
			files = append(files, name)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	return files
}

// aggregateDay computes stats for a single day's entries.
func aggregateDay(date string, entries []logger.UsageEntry) DailyStats {
	byTier := make(map[string]*TierModelStat)
	byModel := make(map[string]*TierModelStat)
	var totalLatency int64

	for _, e := range entries {
		tier := e.Tier
		if tier == "" {
			tier = "UNKNOWN"
		}
		if byTier[tier] == nil {
			byTier[tier] = &TierModelStat{}
		}
		byTier[tier].Count++
		byTier[tier].Cost += e.Cost

		model := e.Model
		if model == "" {
			model = "unknown"
		}
		if byModel[model] == nil {
			byModel[model] = &TierModelStat{}
		}
		byModel[model].Count++
		byModel[model].Cost += e.Cost

		totalLatency += e.LatencyMs
	}

	totalCost := 0.0
	totalBaseline := 0.0
	for _, e := range entries {
		totalCost += e.Cost
		bl := e.BaselineCost
		if bl == 0 {
			bl = e.Cost
		}
		totalBaseline += bl
	}

	avgLatency := 0.0
	if len(entries) > 0 {
		avgLatency = float64(totalLatency) / float64(len(entries))
	}

	return DailyStats{
		Date:              date,
		TotalRequests:     len(entries),
		TotalCost:         totalCost,
		TotalBaselineCost: totalBaseline,
		TotalSavings:      totalBaseline - totalCost,
		AvgLatencyMs:      avgLatency,
		ByTier:            byTier,
		ByModel:           byModel,
	}
}

// GetStats returns aggregated statistics for the last N days.
func GetStats(days int) AggregatedStats {
	if days <= 0 {
		days = 7
	}

	logFiles := getLogFiles()
	if len(logFiles) > days {
		logFiles = logFiles[:days]
	}

	logDir := logger.GetLogDir()
	var dailyBreakdown []DailyStats
	allByTier := make(map[string]*TierModelStat)
	allByModel := make(map[string]*TierModelStat)
	var totalRequests int
	var totalCost, totalBaselineCost, totalLatency float64

	for _, file := range logFiles {
		date := strings.TrimSuffix(strings.TrimPrefix(file, "usage-"), ".jsonl")
		entries := parseLogFile(filepath.Join(logDir, file))
		if len(entries) == 0 {
			continue
		}

		dayStats := aggregateDay(date, entries)
		dailyBreakdown = append(dailyBreakdown, dayStats)

		totalRequests += dayStats.TotalRequests
		totalCost += dayStats.TotalCost
		totalBaselineCost += dayStats.TotalBaselineCost
		totalLatency += dayStats.AvgLatencyMs * float64(dayStats.TotalRequests)

		for tier, s := range dayStats.ByTier {
			if allByTier[tier] == nil {
				allByTier[tier] = &TierModelStat{}
			}
			allByTier[tier].Count += s.Count
			allByTier[tier].Cost += s.Cost
		}
		for model, s := range dayStats.ByModel {
			if allByModel[model] == nil {
				allByModel[model] = &TierModelStat{}
			}
			allByModel[model].Count += s.Count
			allByModel[model].Cost += s.Cost
		}
	}

	// Calculate percentages
	for _, s := range allByTier {
		if totalRequests > 0 {
			s.Percentage = float64(s.Count) / float64(totalRequests) * 100
		}
	}
	for _, s := range allByModel {
		if totalRequests > 0 {
			s.Percentage = float64(s.Count) / float64(totalRequests) * 100
		}
	}

	totalSavings := totalBaselineCost - totalCost
	savingsPercentage := 0.0
	if totalBaselineCost > 0 {
		savingsPercentage = totalSavings / totalBaselineCost * 100
	}

	entriesWithBaseline := 0
	for _, day := range dailyBreakdown {
		if day.TotalBaselineCost != day.TotalCost {
			entriesWithBaseline += day.TotalRequests
		}
	}

	// Reverse daily breakdown (oldest first)
	for i, j := 0, len(dailyBreakdown)-1; i < j; i, j = i+1, j-1 {
		dailyBreakdown[i], dailyBreakdown[j] = dailyBreakdown[j], dailyBreakdown[i]
	}

	period := "today"
	if days != 1 {
		period = fmt.Sprintf("last %d days", days)
	}

	avgLatency := 0.0
	avgCost := 0.0
	if totalRequests > 0 {
		avgLatency = totalLatency / float64(totalRequests)
		avgCost = totalCost / float64(totalRequests)
	}

	return AggregatedStats{
		Period:              period,
		TotalRequests:       totalRequests,
		TotalCost:           totalCost,
		TotalBaselineCost:   totalBaselineCost,
		TotalSavings:        totalSavings,
		SavingsPercentage:   savingsPercentage,
		AvgLatencyMs:        avgLatency,
		AvgCostPerRequest:   avgCost,
		ByTier:              allByTier,
		ByModel:             allByModel,
		DailyBreakdown:      dailyBreakdown,
		EntriesWithBaseline: entriesWithBaseline,
	}
}

// Version placeholder (set at build time or from main).
var Version = "dev"

// FormatStatsASCII formats stats as an ASCII table for terminal display.
func FormatStatsASCII(s AggregatedStats) string {
	var b strings.Builder

	b.WriteString("+" + strings.Repeat("=", 60) + "+\n")
	b.WriteString(fmt.Sprintf("|  DOSRouter v%-47s|\n", Version))
	b.WriteString("|  Usage Statistics" + strings.Repeat(" ", 42) + "|\n")
	b.WriteString("+" + strings.Repeat("=", 60) + "+\n")

	b.WriteString(fmt.Sprintf("|  Period: %-51s|\n", s.Period))
	b.WriteString(fmt.Sprintf("|  Total Requests: %-42d|\n", s.TotalRequests))
	b.WriteString(fmt.Sprintf("|  Total Cost: $%-46.4f|\n", s.TotalCost))
	b.WriteString(fmt.Sprintf("|  Baseline Cost (Opus 4.5): $%-31.4f|\n", s.TotalBaselineCost))
	b.WriteString(fmt.Sprintf("|  Total Saved: $%.4f (%.1f%%)%s|\n",
		s.TotalSavings, s.SavingsPercentage,
		strings.Repeat(" ", max(0, 38-len(fmt.Sprintf("%.4f (%.1f%%)", s.TotalSavings, s.SavingsPercentage))))))
	b.WriteString(fmt.Sprintf("|  Avg Latency: %.0fms%s|\n",
		s.AvgLatencyMs, strings.Repeat(" ", max(0, 44-len(fmt.Sprintf("%.0fms", s.AvgLatencyMs))))))

	// Tier breakdown
	b.WriteString("+" + strings.Repeat("=", 60) + "+\n")
	b.WriteString("|  Routing by Tier:" + strings.Repeat(" ", 42) + "|\n")

	knownTiers := []string{"SIMPLE", "MEDIUM", "COMPLEX", "REASONING", "DIRECT"}
	for _, tier := range knownTiers {
		if data, ok := s.ByTier[tier]; ok {
			barLen := int(data.Percentage / 5)
			if barLen > 20 {
				barLen = 20
			}
			bar := strings.Repeat("#", barLen)
			line := fmt.Sprintf("|    %-10s %-20s %5.1f%% (%d)", tier, bar, data.Percentage, data.Count)
			b.WriteString(line + strings.Repeat(" ", max(0, 61-len(line))) + "|\n")
		}
	}

	// Top models
	b.WriteString("+" + strings.Repeat("=", 60) + "+\n")
	b.WriteString("|  Top Models:" + strings.Repeat(" ", 47) + "|\n")

	type modelEntry struct {
		name string
		stat *TierModelStat
	}
	var models []modelEntry
	for name, stat := range s.ByModel {
		models = append(models, modelEntry{name, stat})
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].stat.Count > models[j].stat.Count
	})
	if len(models) > 5 {
		models = models[:5]
	}
	for _, m := range models {
		shortModel := m.name
		if len(shortModel) > 25 {
			shortModel = shortModel[:22] + "..."
		}
		line := fmt.Sprintf("|    %-25s %5d reqs  $%.4f", shortModel, m.stat.Count, m.stat.Cost)
		b.WriteString(line + strings.Repeat(" ", max(0, 61-len(line))) + "|\n")
	}

	// Daily breakdown
	if len(s.DailyBreakdown) > 0 {
		b.WriteString("+" + strings.Repeat("=", 60) + "+\n")
		b.WriteString("|  Daily Breakdown:" + strings.Repeat(" ", 42) + "|\n")
		b.WriteString("|    Date        Requests    Cost      Saved" + strings.Repeat(" ", 17) + "|\n")

		breakdown := s.DailyBreakdown
		if len(breakdown) > 7 {
			breakdown = breakdown[len(breakdown)-7:]
		}
		for _, day := range breakdown {
			saved := day.TotalBaselineCost - day.TotalCost
			line := fmt.Sprintf("|    %s   %6d    $%8.4f  $%.4f", day.Date, day.TotalRequests, day.TotalCost, saved)
			b.WriteString(line + strings.Repeat(" ", max(0, 61-len(line))) + "|\n")
		}
	}

	b.WriteString("+" + strings.Repeat("=", 60) + "+\n")
	return b.String()
}

// FormatRecentLogs formats per-request log entries as an ASCII table.
func FormatRecentLogs(days int) string {
	if days <= 0 {
		days = 1
	}

	logFiles := getLogFiles()
	if len(logFiles) > days {
		logFiles = logFiles[:days]
	}

	logDir := logger.GetLogDir()
	var allEntries []logger.UsageEntry
	for _, file := range logFiles {
		entries := parseLogFile(filepath.Join(logDir, file))
		allEntries = append(allEntries, entries...)
	}

	// Sort chronologically
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Timestamp < allEntries[j].Timestamp
	})

	var b strings.Builder
	periodLabel := "24h"
	if days != 1 {
		periodLabel = fmt.Sprintf("%d days", days)
	}

	b.WriteString("+" + strings.Repeat("=", 72) + "+\n")
	b.WriteString(fmt.Sprintf("|  DOSRouter Request Log - last %-41s|\n", periodLabel))
	b.WriteString("+" + strings.Repeat("-", 18) + "+" + strings.Repeat("-", 26) + "+" +
		strings.Repeat("-", 9) + "+" + strings.Repeat("-", 6) + "+" + strings.Repeat("-", 8) + "+\n")
	b.WriteString("|  Time            |  Model                   |  Cost   |  ms  | Status |\n")
	b.WriteString("+" + strings.Repeat("-", 18) + "+" + strings.Repeat("-", 26) + "+" +
		strings.Repeat("-", 9) + "+" + strings.Repeat("-", 6) + "+" + strings.Repeat("-", 8) + "+\n")

	if len(allEntries) == 0 {
		b.WriteString("|  No requests found" + strings.Repeat(" ", 53) + "|\n")
	}

	totalCost := 0.0
	for _, e := range allEntries {
		ts := e.Timestamp
		displayTime := ""
		if len(ts) >= 19 {
			displayTime = ts[5:10] + " " + ts[11:19]
		}
		model := e.Model
		if len(model) > 24 {
			model = model[:21] + "..."
		}
		cost := fmt.Sprintf("$%.4f", e.Cost)
		ms := fmt.Sprintf("%dms", e.LatencyMs)
		if e.LatencyMs > 9999 {
			ms = fmt.Sprintf("%.1fs", float64(e.LatencyMs)/1000)
		}
		status := " OK     "
		if e.Status == "error" {
			status = " ERROR  "
		}
		totalCost += e.Cost

		b.WriteString(fmt.Sprintf("|  %-16s|  %-24s|  %7s|  %4s|%s|\n",
			displayTime, model, cost, ms, status))
	}

	b.WriteString("+" + strings.Repeat("=", 72) + "+\n")
	plural := "s"
	if len(allEntries) == 1 {
		plural = ""
	}
	b.WriteString(fmt.Sprintf("|  %d request%s  Total spent: $%.4f%s|\n",
		len(allEntries), plural, totalCost,
		strings.Repeat(" ", max(0, 45-len(fmt.Sprintf("%d request%s  Total spent: $%.4f", len(allEntries), plural, totalCost))))))
	b.WriteString("|  Logs: ~/.openclaw/blockrun/logs/  (JSONL)" + strings.Repeat(" ", 29) + "|\n")
	b.WriteString("+" + strings.Repeat("=", 72) + "+\n")

	return b.String()
}

// ClearStats deletes all usage log files.
func ClearStats() int {
	logDir := logger.GetLogDir()
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return 0
	}

	deleted := 0
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "usage-") && strings.HasSuffix(name, ".jsonl") {
			if os.Remove(filepath.Join(logDir, name)) == nil {
				deleted++
			}
		}
	}
	return deleted
}
