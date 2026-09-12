package dedup

import (
	"bytes"
	"strings"
	"testing"
)

const multimodalRequest = `{"model":"test/model","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://example.test/a.png","detail":"high"}},{"type":"text","text":"[Sat 2026-09-12 09:00 ICT] Describe this image"},{"type":"text","text":"[Sat 2026-09-12 10:00 ICT] Event"}]},{"role":"assistant","content":null,"tool_calls":[{"type":"function","function":{"name":"record_event","arguments":"[Sat 2026-09-12 11:00 ICT] Event"}}]}]}`

func TestDedupReusesMultimodalResponseAcrossInjectedTimestamps(t *testing.T) {
	first := []byte(multimodalRequest)
	original := bytes.Clone(first)
	second := []byte(strings.Replace(multimodalRequest, "09:00", "09:30", 1))
	d := New()
	calls := 0
	call := func() (Response, error) {
		calls++
		return Response{Body: []byte("upstream answer"), StatusCode: 200}, nil
	}
	if _, hit, err := d.Do(first, call); err != nil || hit {
		t.Fatalf("first request: hit=%v err=%v", hit, err)
	}
	got, hit, err := d.Do(second, call)
	if err != nil || !hit || calls != 1 || string(got.Body) != "upstream answer" {
		t.Fatalf("timestamp-only retry: hit=%v calls=%d body=%q err=%v", hit, calls, got.Body, err)
	}
	if !bytes.Equal(first, original) {
		t.Error("dedup normalization mutated request bytes")
	}
}

func TestHashBodyPreservesRequestSemantics(t *testing.T) {
	tests := []struct{ name, before, after string }{
		{"later text timestamp", "10:00", "10:30"},
		{"tool arguments", "11:00", "11:30"},
		{"image URL", "a.png", "b.png"},
		{"image detail", `"detail":"high"`, `"detail":"low"`},
		{"model", "test/model", "test/other-model"},
	}
	base, err := HashBody([]byte(multimodalRequest))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed := strings.Replace(multimodalRequest, tt.before, tt.after, 1)
			key, err := HashBody([]byte(changed))
			if err != nil {
				t.Fatal(err)
			}
			if key == base {
				t.Error("different response-affecting content shared a dedup key")
			}
		})
	}
}

func TestHashBodyDistinguishesObjectsFromArrays(t *testing.T) {
	object, err := HashBody([]byte(`{"model":"test/model","payload":{"a":1}}`))
	if err != nil {
		t.Fatal(err)
	}
	array, err := HashBody([]byte(`{"model":"test/model","payload":[["a",1]]}`))
	if err != nil {
		t.Fatal(err)
	}
	if object == array {
		t.Fatal("object and array payloads shared a dedup key")
	}
}

func TestHashBodyIgnoresObjectKeyOrder(t *testing.T) {
	first, err := HashBody([]byte(`{"model":"test/model","payload":{"a":1,"b":[2,3]}}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashBody([]byte(`{"payload":{"b":[2,3],"a":1},"model":"test/model"}`))
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("equivalent JSON objects had different dedup keys")
	}
}
