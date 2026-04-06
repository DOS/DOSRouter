package compression

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	// MinPathSegments is the minimum number of path segments required
	// for a string to be considered a filesystem path.
	MinPathSegments = 3
	// MinPrefixOccurrences is the minimum number of times a path prefix
	// must appear to warrant replacement with a short variable.
	MinPrefixOccurrences = 3
	// MaxPathVariables is the maximum number of path variables ($P1-$P5).
	MaxPathVariables = 5
)

// pathRe matches Unix-style and Windows-style filesystem paths with 3+
// segments. It captures paths like /home/user/project or C:\Users\foo\bar.
var pathRe = regexp.MustCompile(`(?:(?:[A-Za-z]:)?[/\])(?:[^\s/\]+[/\]){2,}[^\s/\]*`)

// PathsResult holds the outcome of path compression.
type PathsResult struct {
	Mappings   map[string]string
	CharsSaved int
}

// CompressPaths extracts filesystem paths from all messages, identifies
// common prefixes appearing 3+ times, and replaces them with $P1-$P5
// variables. It modifies messages in-place.
func CompressPaths(messages []NormalizedMessage) PathsResult {
	// Step 1: Extract all paths and count their directory prefixes.
	prefixCount := make(map[string]int)

	for _, msg := range messages {
		extractPathPrefixes(msg.GetTextContent(), prefixCount)
		for _, tc := range msg.ToolCalls {
			extractPathPrefixes(tc.Function.Arguments, prefixCount)
		}
	}

	// Step 2: Filter prefixes with 3+ occurrences, sort by savings descending.
	type prefixEntry struct {
		prefix  string
		count   int
		savings int
	}

	var candidates []prefixEntry
	for prefix, count := range prefixCount {
		if count >= MinPrefixOccurrences {
			candidates = append(candidates, prefixEntry{
				prefix:  prefix,
				count:   count,
				savings: count * len(prefix),
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].savings > candidates[j].savings
	})

	// Step 3: Assign $P1-$P5 to top prefixes.
	mappings := make(map[string]string)
	limit := MaxPathVariables
	if len(candidates) < limit {
		limit = len(candidates)
	}

	for i := 0; i < limit; i++ {
		varName := fmt.Sprintf("$P%d", i+1)
		mappings[candidates[i].prefix] = varName
	}

	if len(mappings) == 0 {
		return PathsResult{Mappings: nil, CharsSaved: 0}
	}

	// Step 4: Replace prefixes in messages (longest first).
	sortedPrefixes := make([]string, 0, len(mappings))
	for p := range mappings {
		sortedPrefixes = append(sortedPrefixes, p)
	}
	sort.Slice(sortedPrefixes, func(i, j int) bool {
		return len(sortedPrefixes[i]) > len(sortedPrefixes[j])
	})

	totalSaved := 0

	replaceInText := func(text string) string {
		result := text
		for _, prefix := range sortedPrefixes {
			varName := mappings[prefix]
			count := strings.Count(result, prefix)
			if count > 0 {
				saved := count * (len(prefix) - len(varName))
				totalSaved += saved
				result = strings.ReplaceAll(result, prefix, varName)
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

		for j := range messages[i].ToolCalls {
			origArgs := messages[i].ToolCalls[j].Function.Arguments
			compArgs := replaceInText(origArgs)
			if compArgs != origArgs {
				messages[i].ToolCalls[j].Function.Arguments = compArgs
			}
		}
	}

	return PathsResult{
		Mappings:   mappings,
		CharsSaved: totalSaved,
	}
}

// extractPathPrefixes finds all filesystem paths in text and counts their
// directory prefixes (everything up to and including the last separator
// before the final segment).
func extractPathPrefixes(text string, counts map[string]int) {
	matches := pathRe.FindAllString(text, -1)
	for _, path := range matches {
		// Normalize separators to forward slash for prefix extraction.
		normalized := strings.ReplaceAll(path, "\\", "/")
		parts := strings.Split(normalized, "/")
		if len(parts) < MinPathSegments {
			continue
		}

		// Use the directory portion (all but the last segment) as the prefix.
		dirParts := parts[:len(parts)-1]
		prefix := strings.Join(dirParts, "/") + "/"

		// Try to find the original form in text.
		if strings.Contains(text, prefix) {
			counts[prefix]++
		} else {
			// Try with backslashes (Windows paths).
			winPrefix := strings.ReplaceAll(prefix, "/", "\\")
			if strings.Contains(text, winPrefix) {
				counts[winPrefix]++
			} else {
				counts[prefix]++
			}
		}
	}
}
