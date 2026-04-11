package models

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// allowlistPath is the path to the user's custom model allowlist.
var (
	allowlistMu   sync.Mutex
	allowlistPath string
)

func init() {
	home, _ := os.UserHomeDir()
	allowlistPath = filepath.Join(home, ".openclaw", "DOS", "allow-models.json")
}

// AllowlistEntry represents a user-added model in the allowlist.
type AllowlistEntry struct {
	ID string `json:"id"`
}

// LoadAllowlist reads user-added model IDs from disk.
func LoadAllowlist() ([]string, error) {
	allowlistMu.Lock()
	fp := allowlistPath
	allowlistMu.Unlock()

	data, err := os.ReadFile(fp)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

// SaveAllowlist writes the allowlist to disk.
func SaveAllowlist(ids []string) error {
	allowlistMu.Lock()
	fp := allowlistPath
	allowlistMu.Unlock()

	dir := filepath.Dir(fp)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ids, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fp, data, 0644)
}

// InjectModelsConfig merges the built-in model catalog with user-added allowlist
// entries. User entries with a "dosrouter/" prefix are preserved across restarts
// (upstream v0.12.24 - preserve user-defined allowlist entries).
func InjectModelsConfig(builtinIDs []string) []string {
	userIDs, err := LoadAllowlist()
	if err != nil || len(userIDs) == 0 {
		return builtinIDs
	}

	// Build set of built-in IDs
	builtinSet := make(map[string]bool, len(builtinIDs))
	for _, id := range builtinIDs {
		builtinSet[id] = true
	}

	// Merge: start with built-in, then add any user entries not already present
	merged := make([]string, len(builtinIDs))
	copy(merged, builtinIDs)
	for _, id := range userIDs {
		if !builtinSet[id] {
			merged = append(merged, id)
		}
	}

	return merged
}
