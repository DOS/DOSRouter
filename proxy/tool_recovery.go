package proxy

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// sanitizeHeaderValue removes non-ASCII and control characters from HTTP header values
// (upstream v0.12.208) to prevent Go http.Header from corrupting responses or crashing.
func sanitizeHeaderValue(val string) string {
	var b strings.Builder
	for _, r := range val {
		if r >= 32 && r <= 126 {
			b.WriteRune(r)
		} else if unicode.IsSpace(r) {
			b.WriteRune(' ')
		}
	}
	return strings.TrimSpace(b.String())
}

var (
	// Gemini / GPT-5.4 plain text tool call regex patterns (upstream v0.12.214, v0.12.215)
	jsonCodeBlockRegex = regexp.MustCompile(`(?s)` + "```" + `(?:json)?\s*(\{.*?\}|\[.*?\])\s*` + "```")
	callToolRegex      = regexp.MustCompile(`(?s)(?:call|invoke|tool_call):\s*([a-zA-Z0-9_\-]+)\s*\((.*?)\)`)
)

// recoverStructuredToolCalls attempts to parse tool calls from plain text content
// when the model output contains structured JSON or tool call signatures in text
// but the provider failed to populate the tool_calls field (e.g. Gemini, GPT-5.4, Kimi K3).
func recoverStructuredToolCalls(content string, defaultToolName string) []map[string]interface{} {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}

	var results []map[string]interface{}

	// 1. Try parsing JSON code blocks
	matches := jsonCodeBlockRegex.FindAllStringSubmatch(trimmed, -1)
	for i, match := range matches {
		if len(match) < 2 {
			continue
		}
		rawJSON := strings.TrimSpace(match[1])
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(rawJSON), &parsed); err == nil {
			// Check if it looks like a tool call { "name": ..., "arguments": ... } or { "tool": ..., "parameters": ... }
			name, _ := parsed["name"].(string)
			if name == "" {
				name, _ = parsed["tool"].(string)
			}
			if name == "" {
				name, _ = parsed["function"].(string)
			}
			if name == "" && defaultToolName != "" {
				name = defaultToolName
			}

			if name != "" {
				args := parsed["arguments"]
				if args == nil {
					args = parsed["parameters"]
				}
				if args == nil {
					args = parsed["input"]
				}
				var argsStr string
				if s, ok := args.(string); ok {
					argsStr = s
				} else if args != nil {
					b, _ := json.Marshal(args)
					argsStr = string(b)
				} else {
					argsStr = "{}"
				}

				results = append(results, map[string]interface{}{
					"id":   fmt.Sprintf("call_%d_%d", i, len(results)),
					"type": "function",
					"function": map[string]interface{}{
						"name":      name,
						"arguments": argsStr,
					},
				})
			}
		}
	}

	// 2. Try regex function call syntax: tool_name(args)
	callMatches := callToolRegex.FindAllStringSubmatch(trimmed, -1)
	for i, match := range callMatches {
		if len(match) < 3 {
			continue
		}
		name := strings.TrimSpace(match[1])
		argsRaw := strings.TrimSpace(match[2])
		if argsRaw == "" {
			argsRaw = "{}"
		} else if !strings.HasPrefix(argsRaw, "{") {
			// If not a JSON object, wrap as json or string arg
			argsRaw = fmt.Sprintf(`{"input": %q}`, argsRaw)
		}

		results = append(results, map[string]interface{}{
			"id":   fmt.Sprintf("call_recovered_%d_%d", i, len(results)),
			"type": "function",
			"function": map[string]interface{}{
				"name":      name,
				"arguments": argsRaw,
			},
		})
	}

	return results
}
