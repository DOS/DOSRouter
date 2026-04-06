package compression

import (
	"encoding/json"
)

// CompactJSON takes a JSON string and re-encodes it without any extra
// whitespace. If the input is not valid JSON it is returned unchanged.
func CompactJSON(text string) string {
	if text == "" {
		return text
	}

	var parsed interface{}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return text
	}

	compacted, err := json.Marshal(parsed)
	if err != nil {
		return text
	}

	return string(compacted)
}

// CompressJSON compacts JSON found in tool_call arguments and tool message
// content. It modifies messages in-place and returns total characters saved.
func CompressJSON(messages []NormalizedMessage) int {
	saved := 0

	for i := range messages {
		// Compact tool_call arguments (usually JSON).
		for j := range messages[i].ToolCalls {
			original := messages[i].ToolCalls[j].Function.Arguments
			compacted := CompactJSON(original)
			if len(compacted) < len(original) {
				saved += len(original) - len(compacted)
				messages[i].ToolCalls[j].Function.Arguments = compacted
			}
		}

		// Compact tool message content (often JSON results).
		if messages[i].Role == "tool" {
			original := messages[i].GetTextContent()
			compacted := CompactJSON(original)
			if len(compacted) < len(original) {
				saved += len(original) - len(compacted)
				messages[i].SetTextContent(compacted)
			}
		}
	}

	return saved
}
