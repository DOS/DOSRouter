package proxy

import "testing"

func TestSanitizeHeaderValue(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"normal ascii", "normal ascii"},
		{"utf8-vietnamese: Tiếng Việt", "utf8-vietnamese: Ting Vit"},
		{"control\n\rchars\t", "control  chars"},
		{"emoji 🚀 and symbol", "emoji  and symbol"},
	}

	for _, c := range cases {
		got := sanitizeHeaderValue(c.input)
		if got != c.want {
			t.Errorf("sanitizeHeaderValue(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestRecoverStructuredToolCalls(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		wantLen  int
		wantName string
	}{
		{
			name: "gemini markdown json codeblock",
			content: "Here is your tool call:\n```json\n{\n  \"name\": \"get_weather\",\n  \"arguments\": {\"location\": \"Hanoi\"}\n}\n```",
			wantLen:  1,
			wantName: "get_weather",
		},
		{
			name: "plain text call:function(args)",
			content: "call:search_database({\"query\": \"AI news\"})",
			wantLen:  1,
			wantName: "search_database",
		},
		{
			name:     "pure conversational text",
			content:  "Hello, how can I help you today?",
			wantLen:  0,
			wantName: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := recoverStructuredToolCalls(c.content, "")
			if len(got) != c.wantLen {
				t.Fatalf("recoverStructuredToolCalls() returned %d calls, want %d", len(got), c.wantLen)
			}
			if c.wantLen > 0 {
				fn, ok := got[0]["function"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected function object in tool call: %v", got[0])
				}
				if fn["name"] != c.wantName {
					t.Errorf("tool name = %v, want %v", fn["name"], c.wantName)
				}
			}
		})
	}
}
