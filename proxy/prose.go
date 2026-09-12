package proxy

import (
	"regexp"
	"strings"
)

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
			lower := strings.ToLower(tag)
			if strings.Contains(lower, "begin") {
				f.hidden = true
			}
			if strings.Contains(lower, "end") {
				f.hidden = false
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
