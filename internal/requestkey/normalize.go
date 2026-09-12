// Package requestkey normalizes only timestamps injected into message content.
package requestkey

import (
	"maps"
	"regexp"
)

var timestamp = regexp.MustCompile(`^\[\w{3}\s+\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}\s+\w+\]\s*`)

// Normalize removes an injected prefix from root request messages' string
// content or first text block. Tool results, metadata, tool arguments and later
// text blocks are preserved, and the input request is never mutated.
func Normalize(value any) any {
	request, ok := value.(map[string]any)
	if !ok {
		return value
	}
	messages, ok := request["messages"].([]any)
	if !ok {
		return value
	}

	out := maps.Clone(request)
	normalized := make([]any, len(messages))
	for i, item := range messages {
		normalized[i] = item
		message, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role, _ := message["role"].(string)
		if role == "tool" || role == "function" {
			continue
		}
		messageContent, exists := message["content"]
		if !exists {
			continue
		}
		messageCopy := maps.Clone(message)
		messageCopy["content"] = content(messageContent)
		normalized[i] = messageCopy
	}
	out["messages"] = normalized
	return out
}

func content(value any) any {
	if text, ok := value.(string); ok {
		return timestamp.ReplaceAllString(text, "")
	}
	blocks, ok := value.([]any)
	if !ok {
		return value
	}
	out := make([]any, len(blocks))
	copy(out, blocks)
	for i, block := range blocks {
		obj, ok := block.(map[string]any)
		if !ok || obj["type"] != "text" {
			continue
		}
		text, ok := obj["text"].(string)
		if !ok {
			continue
		}
		copy := make(map[string]any, len(obj))
		for key, value := range obj {
			copy[key] = value
		}
		copy["text"] = timestamp.ReplaceAllString(text, "")
		out[i] = copy
		break
	}
	return out
}
