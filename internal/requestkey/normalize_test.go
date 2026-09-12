package requestkey

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNormalizeInjectedMessageTimestamps(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "string message",
			input: `{"messages":[{"role":"user","content":"[Sat 2026-09-12 09:00 ICT] Hello"}]}`,
			want:  `{"messages":[{"role":"user","content":"Hello"}]}`,
		},
		{
			name:  "first text after image",
			input: `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://example.test/image.png","detail":"high"}},{"type":"text","text":"[Sat 2026-09-12 09:00 ICT] Describe this image"},{"type":"text","text":"[Sat 2026-09-12 10:00 ICT] The event happened then"}]}]}`,
			want:  `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://example.test/image.png","detail":"high"}},{"type":"text","text":"Describe this image"},{"type":"text","text":"[Sat 2026-09-12 10:00 ICT] The event happened then"}]}]}`,
		},
		{
			name:  "later text is never stripped",
			input: `{"messages":[{"content":[{"type":"text","text":"Keep the quoted timestamp"},{"type":"text","text":"[Sat 2026-09-12 10:00 ICT] Event"}]}]}`,
			want:  `{"messages":[{"content":[{"type":"text","text":"Keep the quoted timestamp"},{"type":"text","text":"[Sat 2026-09-12 10:00 ICT] Event"}]}]}`,
		},
		{
			name:  "tool arguments are preserved",
			input: `{"messages":[{"role":"assistant","content":null,"tool_calls":[{"type":"function","function":{"name":"record_event","arguments":"[Sat 2026-09-12 10:00 ICT] Event"}}]}]}`,
			want:  `{"messages":[{"role":"assistant","content":null,"tool_calls":[{"type":"function","function":{"name":"record_event","arguments":"[Sat 2026-09-12 10:00 ICT] Event"}}]}]}`,
		},
		{
			name:  "embedded timestamp is preserved",
			input: `{"messages":[{"content":"Quote [Sat 2026-09-12 10:00 ICT] Event"}]}`,
			want:  `{"messages":[{"content":"Quote [Sat 2026-09-12 10:00 ICT] Event"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var input, original, want any
			for _, value := range []struct {
				text   string
				target *any
			}{
				{tt.input, &input}, {tt.input, &original}, {tt.want, &want},
			} {
				if err := json.Unmarshal([]byte(value.text), value.target); err != nil {
					t.Fatal(err)
				}
			}
			got := Normalize(input)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Normalize() = %#v, want %#v", got, want)
			}
			if !reflect.DeepEqual(input, original) {
				t.Error("Normalize mutated its input")
			}
		})
	}
}

func TestNormalizePreservesNonMessageContent(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"root content", `{"content":"[Sat 2026-09-12 09:00 ICT] Root data"}`},
		{"metadata content", `{"metadata":{"content":"[Sat 2026-09-12 09:00 ICT] Event"},"messages":[{"role":"user","content":"Hello"}]}`},
		{"nested message history", `{"metadata":{"messages":[{"role":"user","content":"[Sat 2026-09-12 09:00 ICT] Historical event"}]},"messages":[]}`},
		{"message metadata", `{"messages":[{"role":"user","content":"Hello","metadata":{"content":"[Sat 2026-09-12 09:00 ICT] Event"}}]}`},
		{"object tool arguments", `{"messages":[{"role":"assistant","content":null,"tool_calls":[{"type":"function","function":{"name":"record_event","arguments":{"content":"[Sat 2026-09-12 09:00 ICT] Event"}}}]}]}`},
		{"tool result string", `{"messages":[{"role":"tool","tool_call_id":"call_1","content":"[Sat 2026-09-12 09:00 ICT] Observed event"}]}`},
		{"tool result text block", `{"messages":[{"role":"tool","tool_call_id":"call_1","content":[{"type":"text","text":"[Sat 2026-09-12 09:00 ICT] Observed event"}]}]}`},
		{"legacy function result", `{"messages":[{"role":"function","name":"record_event","content":"[Sat 2026-09-12 09:00 ICT] Observed event"}]}`},
		{"non-request array", `[{"role":"user","content":"[Sat 2026-09-12 09:00 ICT] Data"}]`},
		{"empty text block array", `{"messages":[{"role":"user","content":[]}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var input, original any
			if err := json.Unmarshal([]byte(tt.input), &input); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tt.input), &original); err != nil {
				t.Fatal(err)
			}
			if got := Normalize(input); !reflect.DeepEqual(got, original) {
				t.Errorf("Normalize changed content outside injected message prefixes: got %#v, want %#v", got, original)
			}
			if !reflect.DeepEqual(input, original) {
				t.Error("Normalize mutated its input")
			}
		})
	}
}

func TestNormalizePreservesSupportedMessageRoles(t *testing.T) {
	var input any
	if err := json.Unmarshal([]byte(`{"messages":[{"role":"system","content":"[Sat 2026-09-12 09:00 ICT] System"},{"role":"assistant","content":"[Sat 2026-09-12 09:00 ICT] Assistant"},{"role":"user","content":"[Sat 2026-09-12 09:00 ICT] User"}]}`), &input); err != nil {
		t.Fatal(err)
	}
	var want any
	if err := json.Unmarshal([]byte(`{"messages":[{"role":"system","content":"System"},{"role":"assistant","content":"Assistant"},{"role":"user","content":"User"}]}`), &want); err != nil {
		t.Fatal(err)
	}
	if got := Normalize(input); !reflect.DeepEqual(got, want) {
		t.Errorf("injected timestamps on supported roles were not stripped: got %#v, want %#v", got, want)
	}
}
