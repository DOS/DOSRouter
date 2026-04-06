package compression

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	// MaxToolResultLength is the maximum character length for a tool result
	// before it gets truncated.
	MaxToolResultLength = 2000
	// MaxToolResultLines is the maximum number of lines kept from a tool result.
	MaxToolResultLines = 50
	// TruncationMarker is appended when content is truncated.
	TruncationMarker = "\n...[truncated]"
)

// CompressObservations aggressively compresses tool result messages by
// extracting key information and truncating large results.
// It modifies messages in-place and returns total characters saved.
func CompressObservations(messages []NormalizedMessage) int {
	saved := 0

	for i := range messages {
		if messages[i].Role != "tool" {
			continue
		}

		original := messages[i].GetTextContent()
		if original == "" {
			continue
		}

		compressed := compressToolResult(original)
		if compressed != original {
			saved += len(original) - len(compressed)
			messages[i].SetTextContent(compressed)
		}
	}

	return saved
}

// compressToolResult applies multiple strategies to reduce the size of a
// tool result:
//  1. Remove excessive blank lines
//  2. Truncate to MaxToolResultLines
//  3. Truncate to MaxToolResultLength
//  4. Remove redundant whitespace in structured output
func compressToolResult(text string) string {
	result := text

	// Remove lines that are purely decorative (e.g. separator lines).
	lines := strings.Split(result, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip purely decorative separator lines (===, ---, ***).
		if len(trimmed) > 3 && isDecorativeLine(trimmed) {
			continue
		}
		filtered = append(filtered, line)
	}
	lines = filtered

	// Truncate by line count.
	if len(lines) > MaxToolResultLines {
		lines = lines[:MaxToolResultLines]
		result = strings.Join(lines, "\n") + fmt.Sprintf("\n...[truncated %d lines]", len(filtered)-MaxToolResultLines)
	} else {
		result = strings.Join(lines, "\n")
	}

	// Truncate by character count.
	if utf8.RuneCountInString(result) > MaxToolResultLength {
		runes := []rune(result)
		result = string(runes[:MaxToolResultLength]) + TruncationMarker
	}

	return result
}

// isDecorativeLine checks if a line consists only of repeated decorative
// characters (=, -, *, ~, #).
func isDecorativeLine(line string) bool {
	if len(line) < 3 {
		return false
	}
	first := rune(line[0])
	switch first {
	case '=', '-', '*', '~', '#':
		for _, r := range line {
			if r != first && r != ' ' {
				return false
			}
		}
		return true
	}
	return false
}
