package compression

import (
	"regexp"
	"strings"
)

var (
	// crlfRe matches Windows-style line endings.
	crlfRe = regexp.MustCompile(`\r\n`)
	// multiNewlineRe matches 3+ consecutive newlines.
	multiNewlineRe = regexp.MustCompile(`\n{3,}`)
	// trailingSpaceRe matches trailing spaces on each line.
	trailingSpaceRe = regexp.MustCompile(`(?m)[ \t]+$`)
	// multiSpaceRe collapses multiple spaces (not at line start) into one.
	multiSpaceRe = regexp.MustCompile(`([^ \n]) {2,}`)
	// tabRe matches tab characters.
	tabRe = regexp.MustCompile(`\t`)
	// deepIndentRe matches lines indented more than 8 spaces.
	deepIndentRe = regexp.MustCompile(`(?m)^( {8,})`)
)

// NormalizeWhitespace applies whitespace normalization to a string:
//   - CRLF -> LF
//   - Limit consecutive newlines to 2
//   - Trim trailing spaces per line
//   - Collapse multiple spaces (mid-line) to single
//   - Reduce deep indentation (>8 spaces) to 2-space levels
//   - Tabs -> 2 spaces
func NormalizeWhitespace(text string) string {
	if text == "" {
		return text
	}

	// CRLF -> LF
	result := crlfRe.ReplaceAllString(text, "\n")

	// Tabs -> 2 spaces
	result = tabRe.ReplaceAllString(result, "  ")

	// Trailing spaces per line
	result = trailingSpaceRe.ReplaceAllString(result, "")

	// Reduce deep indentation
	result = deepIndentRe.ReplaceAllStringFunc(result, func(match string) string {
		spaces := len(match)
		level := spaces / 4
		if level < 1 {
			level = 1
		}
		return strings.Repeat("  ", level)
	})

	// Collapse multiple mid-line spaces
	result = multiSpaceRe.ReplaceAllString(result, "$1 ")

	// Max 2 consecutive newlines
	result = multiNewlineRe.ReplaceAllString(result, "\n\n")

	return result
}

// CompressWhitespace applies whitespace normalization to all messages.
// Returns the total characters saved.
func CompressWhitespace(messages []NormalizedMessage) int {
	saved := 0

	for i := range messages {
		original := messages[i].GetTextContent()
		compressed := NormalizeWhitespace(original)
		if compressed != original {
			saved += len(original) - len(compressed)
			messages[i].SetTextContent(compressed)
		}

		// Also normalize tool_call arguments.
		for j := range messages[i].ToolCalls {
			origArgs := messages[i].ToolCalls[j].Function.Arguments
			compArgs := NormalizeWhitespace(origArgs)
			if compArgs != origArgs {
				saved += len(origArgs) - len(compArgs)
				messages[i].ToolCalls[j].Function.Arguments = compArgs
			}
		}
	}

	return saved
}
