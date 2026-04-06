package compression

import (
	"sort"
	"strings"
)

// DictionaryResult holds the outcome of dictionary compression.
type DictionaryResult struct {
	UsedCodes  []string
	CharsSaved int
}

// CompressDictionary replaces known phrases from the static codebook with
// their short codes. Phrases are replaced longest-first so that longer
// matches take priority over substrings.
//
// It modifies messages in-place and returns stats about which codes were
// used and how many characters were saved.
func CompressDictionary(messages []NormalizedMessage) DictionaryResult {
	inverse := GetInverseCodebook()

	// Sort phrases longest first for greedy matching.
	phrases := make([]string, 0, len(inverse))
	for phrase := range inverse {
		phrases = append(phrases, phrase)
	}
	sort.Slice(phrases, func(i, j int) bool {
		return len(phrases[i]) > len(phrases[j])
	})

	usedSet := make(map[string]bool)
	totalSaved := 0

	replaceInText := func(text string) string {
		result := text
		for _, phrase := range phrases {
			if !strings.Contains(result, phrase) {
				continue
			}
			code := inverse[phrase]
			count := strings.Count(result, phrase)
			if count > 0 {
				saved := count * (len(phrase) - len(code))
				if saved > 0 {
					result = strings.ReplaceAll(result, phrase, code)
					usedSet[code] = true
					totalSaved += saved
				}
			}
		}
		return result
	}

	for i := range messages {
		original := messages[i].GetTextContent()
		compressed := replaceInText(original)
		if compressed != original {
			messages[i].SetTextContent(compressed)
		}

		// Also compress tool_call arguments.
		for j := range messages[i].ToolCalls {
			origArgs := messages[i].ToolCalls[j].Function.Arguments
			compArgs := replaceInText(origArgs)
			if compArgs != origArgs {
				messages[i].ToolCalls[j].Function.Arguments = compArgs
			}
		}
	}

	used := make([]string, 0, len(usedSet))
	for code := range usedSet {
		used = append(used, code)
	}
	sort.Strings(used)

	return DictionaryResult{
		UsedCodes:  used,
		CharsSaved: totalSaved,
	}
}
