package proxy

import "testing"

// TestChoiceEndsWithToolCalls guards the tool-call planning-prose suppression
// predicate (upstream v0.12.165/166/169): a turn is a tool-call turn when
// finish_reason is "tool_calls" OR a non-empty tool_calls array is present on
// message (non-streaming) or delta (streaming).
func TestChoiceEndsWithToolCalls(t *testing.T) {
	cases := []struct {
		name   string
		choice map[string]interface{}
		want   bool
	}{
		{
			name:   "finish_reason tool_calls, no array",
			choice: map[string]interface{}{"finish_reason": "tool_calls"},
			want:   true,
		},
		{
			name: "non-streaming message with tool_calls array",
			choice: map[string]interface{}{
				"message": map[string]interface{}{
					"content":    "let me call a tool",
					"tool_calls": []interface{}{map[string]interface{}{"id": "1"}},
				},
			},
			want: true,
		},
		{
			name: "streaming delta with tool_calls array",
			choice: map[string]interface{}{
				"delta": map[string]interface{}{
					"tool_calls": []interface{}{map[string]interface{}{"index": float64(0)}},
				},
			},
			want: true,
		},
		{
			name: "plain assistant content, finish stop",
			choice: map[string]interface{}{
				"finish_reason": "stop",
				"message":       map[string]interface{}{"content": "hello"},
			},
			want: false,
		},
		{
			name: "empty tool_calls array is not a tool-call turn",
			choice: map[string]interface{}{
				"message": map[string]interface{}{
					"content":    "hi",
					"tool_calls": []interface{}{},
				},
			},
			want: false,
		},
		{
			name:   "no finish_reason, no tool_calls",
			choice: map[string]interface{}{"delta": map[string]interface{}{"content": "streaming text"}},
			want:   false,
		},
		{
			name:   "finish_reason length (not tool_calls)",
			choice: map[string]interface{}{"finish_reason": "length"},
			want:   false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := choiceEndsWithToolCalls(c.choice); got != c.want {
				t.Errorf("choiceEndsWithToolCalls(%v) = %v, want %v", c.choice, got, c.want)
			}
		})
	}
}

// TestPerModelTimeout guards the reasoning-aware per-model timeout selection
// (upstream v0.12.182): reasoning models get the longer window, others the short one.
func TestPerModelTimeout(t *testing.T) {
	// Reasoning-capable models (per models catalog) get the long window.
	for _, id := range []string{"anthropic/claude-opus-4.8", "openai/gpt-5.5", "deepseek/deepseek-reasoner"} {
		if got := perModelTimeout(id); got != perModelTimeoutReasoning {
			t.Errorf("perModelTimeout(%q) = %v, want reasoning window %v", id, got, perModelTimeoutReasoning)
		}
	}
	// Non-reasoning / unknown models get the short window.
	for _, id := range []string{"openai/gpt-4o-mini", "anthropic/claude-haiku-4.5", "totally-unknown-model"} {
		if got := perModelTimeout(id); got != perModelTimeoutNonReasoning {
			t.Errorf("perModelTimeout(%q) = %v, want non-reasoning window %v", id, got, perModelTimeoutNonReasoning)
		}
	}
}
