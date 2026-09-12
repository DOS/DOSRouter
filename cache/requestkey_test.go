package cache

import (
	"bytes"
	"strings"
	"testing"
)

const multimodalRequest = `{"model":"test/model","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://example.test/a.png","detail":"high"}},{"type":"text","text":"[Sat 2026-09-12 09:00 ICT] Describe this image"},{"type":"text","text":"[Sat 2026-09-12 10:00 ICT] Event"}]},{"role":"assistant","content":null,"tool_calls":[{"type":"function","function":{"name":"record_event","arguments":"[Sat 2026-09-12 11:00 ICT] Event"}}]}]}`

func TestCacheSeparatesMultimodalResponsesByClientTimestamp(t *testing.T) {
	first := []byte(multimodalRequest)
	original := bytes.Clone(first)
	second := []byte(strings.Replace(multimodalRequest, "09:00", "09:30", 1))
	c := New()
	c.Set(first, Entry{Body: []byte("cached answer"), StatusCode: 200})
	got, ok := c.Get(second, false)
	if ok {
		t.Fatalf("different client timestamp reused cached response: hit=%v, body=%q", ok, got.Body)
	}
	if !bytes.Equal(first, original) {
		t.Error("cache key generation mutated request bytes")
	}
}

func TestCacheKeyPreservesRequestSemantics(t *testing.T) {
	tests := []struct{ name, before, after string }{
		{"first text timestamp", "09:00", "09:30"},
		{"later text timestamp", "10:00", "10:30"},
		{"tool arguments", "11:00", "11:30"},
		{"image URL", "a.png", "b.png"},
		{"image detail", `"detail":"high"`, `"detail":"low"`},
		{"model", "test/model", "test/other-model"},
	}
	base, err := CacheKey([]byte(multimodalRequest))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed := strings.Replace(multimodalRequest, tt.before, tt.after, 1)
			key, err := CacheKey([]byte(changed))
			if err != nil {
				t.Fatal(err)
			}
			if key == base {
				t.Error("different response-affecting content shared a cache key")
			}
		})
	}
}

func TestCacheKeyDistinguishesObjectsFromArrays(t *testing.T) {
	object, err := CacheKey([]byte(`{"model":"test/model","payload":{"a":1}}`))
	if err != nil {
		t.Fatal(err)
	}
	array, err := CacheKey([]byte(`{"model":"test/model","payload":[["a",1]]}`))
	if err != nil {
		t.Fatal(err)
	}
	if object == array {
		t.Fatal("object and array payloads shared a cache key")
	}
}

func TestCacheKeyIgnoresObjectKeyOrder(t *testing.T) {
	first, err := CacheKey([]byte(`{"model":"test/model","payload":{"a":1,"b":[2,3]}}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := CacheKey([]byte(`{"payload":{"b":[2,3],"a":1},"model":"test/model"}`))
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("equivalent JSON objects had different cache keys")
	}
}

func TestCacheKeyPreservesTimestampContent(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"user string", `{"messages":[{"role":"user","content":"[Sat 2026-09-12 09:00 ICT] Event"}]}`},
		{"system string", `{"messages":[{"role":"system","content":"[Sat 2026-09-12 09:00 ICT] Event"}]}`},
		{"assistant string", `{"messages":[{"role":"assistant","content":"[Sat 2026-09-12 09:00 ICT] Event"}]}`},
		{"first text block", `{"messages":[{"role":"user","content":[{"type":"text","text":"[Sat 2026-09-12 09:00 ICT] Event"}]}]}`},
		{"tool result", `{"messages":[{"role":"tool","tool_call_id":"call_1","content":"[Sat 2026-09-12 09:00 ICT] Event"}]}`},
		{"function result", `{"messages":[{"role":"function","name":"record_event","content":"[Sat 2026-09-12 09:00 ICT] Event"}]}`},
		{"metadata", `{"metadata":{"content":"[Sat 2026-09-12 09:00 ICT] Event"},"messages":[]}`},
		{"object tool arguments", `{"messages":[{"role":"assistant","content":null,"tool_calls":[{"type":"function","function":{"name":"record_event","arguments":{"content":"[Sat 2026-09-12 09:00 ICT] Event"}}}]}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original, err := CacheKey([]byte(tt.body))
			if err != nil {
				t.Fatal(err)
			}
			for _, changed := range []string{
				strings.ReplaceAll(tt.body, "09:00", "09:30"),
				strings.ReplaceAll(tt.body, "[Sat 2026-09-12 09:00 ICT] ", ""),
			} {
				key, err := CacheKey([]byte(changed))
				if err != nil {
					t.Fatal(err)
				}
				if original == key {
					t.Error("different client timestamp content shared a cache key")
				}
			}
		})
	}
}

func TestCacheKeyPreservesNumericLiterals(t *testing.T) {
	for _, tt := range []struct {
		name, first, second string
	}{
		{"adjacent integers beyond float64 precision", "9007199254740992", "9007199254740993"},
		{"integer and decimal representation", "1", "1.0"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			first, err := CacheKey([]byte(`{"payload":{"number":` + tt.first + `}}`))
			if err != nil {
				t.Fatal(err)
			}
			second, err := CacheKey([]byte(`{"payload":{"number":` + tt.second + `}}`))
			if err != nil {
				t.Fatal(err)
			}
			if first == second {
				t.Fatalf("distinct numeric literals %s and %s shared a key", tt.first, tt.second)
			}
		})
	}
}

func TestCacheKeyRejectsTrailingJSONValues(t *testing.T) {
	for _, body := range []string{
		`{"model":"test/model"} {"model":"second"}`,
		`{"model":"test/model"} 1`,
		`{"model":"test/model"} garbage`,
	} {
		if key, err := CacheKey([]byte(body)); err == nil || key != "" {
			t.Errorf("trailing JSON data accepted: key=%q err=%v", key, err)
		}
	}
	if _, err := CacheKey([]byte("{\"model\":\"test/model\"} \n\t")); err != nil {
		t.Errorf("valid trailing whitespace rejected: %v", err)
	}
}
