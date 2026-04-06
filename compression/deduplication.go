package compression

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"
)

// DeduplicateMessages removes duplicate assistant messages while preserving
// system, user, and tool messages unconditionally. Assistant messages that
// have tool_calls referenced by subsequent tool messages are also kept.
//
// Returns the deduplicated slice and the number of messages removed.
func DeduplicateMessages(messages []NormalizedMessage) ([]NormalizedMessage, int) {
	if len(messages) == 0 {
		return messages, 0
	}

	// Collect tool_call IDs that are referenced by tool-role messages.
	referencedToolCallIDs := make(map[string]bool)
	for _, msg := range messages {
		if msg.Role == "tool" && msg.ToolCallID != "" {
			referencedToolCallIDs[msg.ToolCallID] = true
		}
	}

	seen := make(map[string]bool)
	result := make([]NormalizedMessage, 0, len(messages))

	for _, msg := range messages {
		// Always keep system, user, and tool messages.
		if msg.Role == "system" || msg.Role == "user" || msg.Role == "tool" {
			result = append(result, msg)
			continue
		}

		// For assistant messages with tool_calls referenced by tool messages, keep them.
		if msg.Role == "assistant" && hasReferencedToolCalls(msg, referencedToolCallIDs) {
			result = append(result, msg)
			continue
		}

		// Compute hash for deduplication.
		hash := hashMessage(msg)
		if seen[hash] {
			continue
		}
		seen[hash] = true
		result = append(result, msg)
	}

	removed := len(messages) - len(result)
	return result, removed
}

// hasReferencedToolCalls checks whether any of the message's tool_calls
// are referenced by a tool-role message.
func hasReferencedToolCalls(msg NormalizedMessage, refs map[string]bool) bool {
	for _, tc := range msg.ToolCalls {
		if refs[tc.ID] {
			return true
		}
	}
	return false
}

// hashMessage produces an MD5 hex digest of role|content|tool_call_id|name|tool_calls.
func hashMessage(msg NormalizedMessage) string {
	var parts []string
	parts = append(parts, msg.Role)
	parts = append(parts, msg.GetTextContent())
	parts = append(parts, msg.ToolCallID)
	parts = append(parts, msg.Name)

	// Serialize tool_calls deterministically.
	if len(msg.ToolCalls) > 0 {
		tc, _ := json.Marshal(msg.ToolCalls)
		parts = append(parts, string(tc))
	} else {
		parts = append(parts, "")
	}

	data := strings.Join(parts, "|")
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}
