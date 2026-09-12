package proxy

import (
	"regexp"
	"strings"
)

var thinkingTagPrefix = regexp.MustCompile(`(?i)^<\s*/?\s*(think(?:ing)?|thought|antthinking|antml:thinking)\b`)

var thinkingTag = regexp.MustCompile(`(?i)^<\s*(/?)\s*(think(?:ing)?|thought|antthinking|antml:thinking)\b[^>]*>$`)

// proseFilter tracks split thinking tags across SSE chunks. Tagged reasoning
// stays private while ordinary assistant prose and tool_calls remain visible.
type proseFilter struct {
	pending string
	hidden  bool
}

func (f *proseFilter) filter(text string, final bool) string {
	f.pending += text
	var out strings.Builder
	for len(f.pending) > 0 {
		start := strings.IndexByte(f.pending, '<')
		if start < 0 {
			if !f.hidden {
				out.WriteString(f.pending)
			}
			f.pending = ""
			break
		}
		if !f.hidden {
			out.WriteString(f.pending[:start])
		}
		f.pending = f.pending[start:]
		end := strings.IndexByte(f.pending, '>')
		if end < 0 {
			if !couldBeThinkingTag(f.pending) {
				if !f.hidden {
					out.WriteByte('<')
				}
				f.pending = f.pending[1:]
				continue
			}
			if final {
				if !f.hidden {
					out.WriteString(f.pending)
				}
				f.pending = ""
			}
			break
		}
		tag := f.pending[:end+1]
		f.pending = f.pending[end+1:]
		if match := thinkingTag.FindStringSubmatch(tag); match != nil {
			f.hidden = match[1] != "/"
		} else if strings.HasPrefix(tag, "<|") || strings.HasPrefix(tag, "<｜") {
			token := strings.ToLower(strings.Trim(tag, "<>|｜"))
			token = strings.ReplaceAll(token, "▁", "_")
			switch token {
			case "begin_of_thinking", "thinking":
				f.hidden = true
			case "end_of_thinking":
				f.hidden = false
			default:
				if !f.hidden {
					out.WriteString(tag)
				}
			}
		} else if !f.hidden {
			out.WriteString(tag)
		}
	}
	return out.String()
}

func stripThinking(content string) string {
	var filter proseFilter
	return filter.filter(content, true)
}

// couldBeThinkingTag buffers only prefixes that can still form a private tag.
// Ordinary comparisons must flow immediately without waiting for a closing >.
func couldBeThinkingTag(pending string) bool {
	if thinkingTagPrefix.MatchString(pending) {
		return true
	}
	candidate := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(pending, "<")))
	candidate = strings.TrimSpace(strings.TrimPrefix(candidate, "/"))
	candidate = strings.ReplaceAll(candidate, "｜", "|")
	candidate = strings.ReplaceAll(candidate, "▁", "_")
	for _, name := range []string{"think", "thinking", "thought", "antthinking", "antml:thinking", "|begin_of_thinking|", "|thinking|", "|end_of_thinking|"} {
		if strings.HasPrefix(name, candidate) {
			return true
		}
	}
	return false
}
