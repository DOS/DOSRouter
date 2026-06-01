package models

import "testing"

// metaProfiles are the smart-routing meta-model IDs that are valid alias
// targets even though they are not concrete provider models.
var metaProfiles = map[string]bool{
	"auto": true, "eco": true, "premium": true, "free": true,
}

// TestNoDanglingAliases guards that every ModelAliases target resolves to a
// real catalog ModelDef ID, another alias (chain), or a smart-routing meta
// profile. A dangling target would leave the router with no pricing /
// reasoning metadata for that model (the o1 -> openai/o1 bug, upstream sync).
func TestNoDanglingAliases(t *testing.T) {
	ids := make(map[string]bool, len(Models))
	for _, m := range Models {
		ids[m.ID] = true
	}
	for alias, target := range ModelAliases {
		if ids[target] || metaProfiles[target] {
			continue
		}
		if _, isAlias := ModelAliases[target]; isAlias {
			continue // chained alias — resolves on a second pass
		}
		t.Errorf("dangling alias %q -> %q: target is not a ModelDef ID, alias, or meta profile", alias, target)
	}
}

// TestFlagshipAliasesResolve locks the current flagship redirects so a future
// edit cannot silently regress them.
func TestFlagshipAliasesResolve(t *testing.T) {
	want := map[string]string{
		"opus":     "anthropic/claude-opus-4.8",
		"gpt5":     "openai/gpt-5.5",
		"kimi":     "moonshot/kimi-k2.6",
		"moonshot": "moonshot/kimi-k2.6",
		"o1":       "openai/o3",
	}
	for alias, target := range want {
		if got := ResolveModelAlias(alias); got != target {
			t.Errorf("ResolveModelAlias(%q) = %q, want %q", alias, got, target)
		}
	}
}
