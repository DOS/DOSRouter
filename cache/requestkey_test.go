package cache

import (
	"bytes"
	"strings"
	"testing"
)

const multimodalRequest = `{"model":"test/model","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://example.test/a.png","detail":"high"}},{"type":"text","text":"[Sat 2026-09-12 09:00 ICT] Describe this image"},{"type":"text","text":"[Sat 2026-09-12 10:00 ICT] Event"}]},{"role":"assistant","content":null,"tool_calls":[{"type":"function","function":{"name":"record_event","arguments":"[Sat 2026-09-12 11:00 ICT] Event"}}]}]}`

func TestCacheReusesMultimodalResponseAcrossInjectedTimestamps(t *testing.T) {
	first := []byte(multimodalRequest)
	original := bytes.Clone(first)
	second := []byte(strings.Replace(multimodalRequest, "09:00", "09:30", 1))
	c := New()
	c.Set(first, Entry{Body: []byte("cached answer"), StatusCode: 200})
	got, ok := c.Get(second, false)
	if !ok || string(got.Body) != "cached answer" {
		t.Fatalf("timestamp-only change missed cached response: hit=%v, body=%q", ok, got.Body)
	}
	if !bytes.Equal(first, original) {
		t.Error("cache key normalization mutated request bytes")
	}
}

func TestCacheKeyPreservesRequestSemantics(t *testing.T) {
	tests := []struct{ name, before, after string }{
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
