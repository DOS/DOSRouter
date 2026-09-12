package proxy

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"

	"github.com/DOS/DOSRouter/logger"
)

// The typed view supports routing while Extra preserves the wire protocol.
func (r *chatRequest) UnmarshalJSON(data []byte) error {
	type plain chatRequest
	var v plain
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &v.Extra); err != nil {
		return err
	}
	*r = chatRequest(v)
	return nil
}

func (r chatRequest) MarshalJSON() ([]byte, error) {
	type plain chatRequest
	return mergeJSON(r.Extra, plain(r))
}

func (m *chatMessage) UnmarshalJSON(data []byte) error {
	type plain chatMessage
	var v plain
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &v.Extra); err != nil {
		return err
	}
	*m = chatMessage(v)
	return nil
}

func (m chatMessage) MarshalJSON() ([]byte, error) {
	type plain chatMessage
	return mergeJSON(m.Extra, plain(m))
}

func mergeJSON(extra map[string]json.RawMessage, typed any) ([]byte, error) {
	fields := make(map[string]json.RawMessage, len(extra))
	for k, v := range extra {
		fields[k] = v
	}
	data, err := json.Marshal(typed)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	return json.Marshal(fields)
}

// Compression cannot remove or stringify protocol-bearing messages.
func canCompressMessages(messages []chatMessage) bool {
	for _, m := range messages {
		if m.Role == "tool" || len(m.Content) == 0 || m.Content[0] != '"' {
			return false
		}
		for key := range m.Extra {
			if key != "role" && key != "content" {
				return false
			}
		}
		if m.ReasoningContent != nil {
			return false
		}
	}
	return true
}

func forwardToolCallProse() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv("DOSROUTER_TOOL_CALL_PROSE")), "off")
}

// Remove only syntax which actually recovered into a tool call.
func recoverToolCallsWithProse(content string) ([]map[string]interface{}, string) {
	var calls []map[string]interface{}
	clean := func(match string) string {
		found := recoverStructuredToolCalls(match, "")
		if len(found) == 0 {
			return match
		}
		calls = append(calls, found...)
		return ""
	}
	cleaned := jsonCodeBlockRegex.ReplaceAllStringFunc(content, clean)
	cleaned = callToolRegex.ReplaceAllStringFunc(cleaned, clean)
	return calls, strings.TrimSpace(cleaned)
}

func gatewayOrigin(base string) string {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func (s *Server) writeUsage(entry logger.UsageEntry) {
	if s.config.UsageLogger != nil {
		s.config.UsageLogger(entry)
		return
	}
	logger.LogUsage(entry)
}
