// Package excludemodels manages a user-configurable list of excluded models
// persisted at ~/.openclaw/blockrun/exclude-models.json.
package excludemodels

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/DOS/DOSRouter/models"
)

var (
	mu       sync.Mutex
	filePath string
)

func init() {
	home, _ := os.UserHomeDir()
	filePath = filepath.Join(home, ".openclaw", "blockrun", "exclude-models.json")
}

// SetFilePath overrides the default exclusion list path (useful for testing).
func SetFilePath(path string) {
	mu.Lock()
	defer mu.Unlock()
	filePath = path
}

// LoadExcludeList reads the JSON array from disk and returns a set of excluded
// model IDs. Returns an empty map if the file does not exist.
func LoadExcludeList() (map[string]bool, error) {
	mu.Lock()
	fp := filePath
	mu.Unlock()

	data, err := os.ReadFile(fp)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]bool), nil
		}
		return nil, err
	}

	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, err
	}

	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set, nil
}

// AddExclusion adds a model to the exclusion list. The model string is first
// resolved through models.ResolveModelAlias so aliases work. Returns the
// resolved model ID.
func AddExclusion(model string) (string, error) {
	resolved := models.ResolveModelAlias(model)

	set, err := LoadExcludeList()
	if err != nil {
		return "", err
	}
	set[resolved] = true
	if err := saveList(set); err != nil {
		return "", err
	}
	return resolved, nil
}

// RemoveExclusion removes a model from the exclusion list. Returns true if the
// model was present.
func RemoveExclusion(model string) (bool, error) {
	resolved := models.ResolveModelAlias(model)

	set, err := LoadExcludeList()
	if err != nil {
		return false, err
	}
	if !set[resolved] {
		return false, nil
	}
	delete(set, resolved)
	if err := saveList(set); err != nil {
		return false, err
	}
	return true, nil
}

// ClearExclusions removes all entries from the exclusion list.
func ClearExclusions() error {
	return saveList(make(map[string]bool))
}

// saveList writes the set back to disk as a JSON array.
func saveList(set map[string]bool) error {
	mu.Lock()
	fp := filePath
	mu.Unlock()

	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}

	dir := filepath.Dir(fp)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ids, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fp, data, 0o644)
}
