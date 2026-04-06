// Package logger provides usage logging as JSONL files.
// Each LLM request is logged as a JSON line to a daily log file
// at ~/.openclaw/blockrun/logs/usage-YYYY-MM-DD.jsonl
package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// UsageEntry represents a single logged LLM request.
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

var (
	logDir   string
	dirReady bool
	mu       sync.Mutex
)

func init() {
	home, _ := os.UserHomeDir()
	logDir = filepath.Join(home, ".openclaw", "blockrun", "logs")
}

// GetLogDir returns the log directory path.
func GetLogDir() string {
	return logDir
}

func ensureDir() error {
	if dirReady {
		return nil
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}
	dirReady = true
	return nil
}

// LogUsage logs a usage entry as a JSON line. Never returns errors to
// avoid breaking the request flow.
func LogUsage(entry UsageEntry) {
	mu.Lock()
	defer mu.Unlock()

	if err := ensureDir(); err != nil {
		return
	}

	date := ""
	if len(entry.Timestamp) >= 10 {
		date = entry.Timestamp[:10]
	} else {
		date = "unknown"
	}

	file := filepath.Join(logDir, fmt.Sprintf("usage-%s.jsonl", date))
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	f.Write(data)
	f.Write([]byte("\n"))
}
