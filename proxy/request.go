package proxy

import (
	"encoding/json"
	"fmt"
	"math"
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
func recoverToolCallsWithProse(content string, tools json.RawMessage) ([]map[string]interface{}, string) {
	var definitions []struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if json.Unmarshal(tools, &definitions) != nil {
		return nil, content
	}
	allowed := make(map[string]bool)
	for _, def := range definitions {
		if def.Type == "function" && def.Function.Name != "" {
			allowed[def.Function.Name] = true
		}
	}
	var calls []map[string]interface{}
	clean := func(match string) string {
		found := recoverStructuredToolCalls(match, "")
		if len(found) == 0 {
			return match
		}
		for _, call := range found {
			function, _ := call["function"].(map[string]interface{})
			name, _ := function["name"].(string)
			if !allowed[name] {
				return match
			}
		}
		for _, call := range found {
			call["id"] = fmt.Sprintf("call_recovered_%d", len(calls))
			calls = append(calls, call)
		}
		return ""
	}
	cleaned := jsonCodeBlockRegex.ReplaceAllStringFunc(content, clean)
	cleaned = callToolRegex.ReplaceAllStringFunc(cleaned, clean)
	return calls, strings.TrimSpace(cleaned)
}

func gatewayOrigin(base string) string {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || u.Scheme == "" {
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

// requestHasTools recognizes the function-tool contract used by routing and
// recovery. Unknown provider extensions are forwarded without inferring tool
// capability from an arbitrary nonempty array.
func requestHasTools(raw json.RawMessage) bool {
	var tools []json.RawMessage
	if json.Unmarshal(raw, &tools) != nil {
		return false
	}
	for _, rawTool := range tools {
		var tool struct {
			Type     string `json:"type"`
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		}
		if json.Unmarshal(rawTool, &tool) == nil && tool.Type == "function" && strings.TrimSpace(tool.Function.Name) != "" {
			return true
		}
	}
	return false
}

func validTokenCount(value any) (int, bool) {
	number, ok := value.(float64)
	if !ok || number < 0 || math.IsNaN(number) || math.IsInf(number, 0) || math.Trunc(number) != number || number >= float64(int(^uint(0)>>1)) {
		return 0, false
	}
	return int(number), true
}
