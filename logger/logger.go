// Package logger provides append-only JSONL usage logging to daily files
// under ~/.dosrouter/logs/.
package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// logDir is the directory for usage log files.
var logDir string

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	logDir = filepath.Join(home, ".dosrouter", "logs")
}

var dirOnce sync.Once

func ensureDir() {
	dirOnce.Do(func() { _ = os.MkdirAll(logDir, 0o755) })
}

// UsageEntry represents a single usage log record.
type UsageEntry struct {
	Timestamp    string  `json:"timestamp"`
	Model        string  `json:"model"`
	Tier         string  `json:"tier"`
	Cost         float64 `json:"cost"`
	BaselineCost float64 `json:"baselineCost"`
	Savings      float64 `json:"savings"`
	LatencyMs    int64   `json:"latencyMs"`
	Status       string  `json:"status,omitempty"`
	InputTokens  int     `json:"inputTokens,omitempty"`
	OutputTokens int     `json:"outputTokens,omitempty"`
	PartnerID    string  `json:"partnerId,omitempty"`
	Service      string  `json:"service,omitempty"`
}

// LogUsage appends a JSON line to the daily log file. It silently swallows
// errors so it never breaks the request flow.
func LogUsage(entry UsageEntry) {
	ensureDir()
	date := "unknown"
	if len(entry.Timestamp) >= 10 {
		date = entry.Timestamp[:10]
	}
	filePath := filepath.Join(logDir, "usage-"+date+".jsonl")
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	data = append(data, 10) // newline
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(data)
}

// LogDir returns the path to the log directory.
func LogDir() string { return logDir }
