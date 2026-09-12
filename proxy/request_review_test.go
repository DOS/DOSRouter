package proxy

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DOS/DOSRouter/spendcontrol"
)

func TestRequestHasToolsRequiresDeclaredFunction(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      bool
	}{
		{"missing", "", false},
		{"null", `null`, false},
		{"empty array", `[]`, false},
		{"null entry", `[null]`, false},
		{"empty object", `[{}]`, false},
		{"non-object entry", `["function"]`, false},
		{"missing type", `[{"function":{"name":"read_file"}}]`, false},
		{"missing function", `[{"type":"function"}]`, false},
		{"missing name", `[{"type":"function","function":{}}]`, false},
		{"null name", `[{"type":"function","function":{"name":null}}]`, false},
		{"wrong name type", `[{"type":"function","function":{"name":7}}]`, false},
		{"empty name", `[{"type":"function","function":{"name":""}}]`, false},
		{"whitespace name", `[{"type":"function","function":{"name":" \t "}}]`, false},
		{"unknown provider extension", `[{"type":"provider_specific","name":"extension"}]`, false},
		{"declared function", `[{"type":"function","function":{"name":"read_file"}}]`, true},
		{"valid declaration among invalid entries", `[null,{},false,{"type":"function","function":{"name":"read_file"}}]`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := requestHasTools(json.RawMessage(tc.raw)); got != tc.want {
				t.Fatalf("requestHasTools(%s)=%v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestSpendCommitFailureLogsWithoutBackendDetails(t *testing.T) {
	storage := &spendcontrol.FileSpendControlStorage{Path: filepath.Join(t.TempDir(), "spending.json")}
	sc, err := spendcontrol.New(storage)
	if err != nil {
		t.Fatal(err)
	}
	if err := sc.SetLimit(spendcontrol.WindowSession, 1); err != nil {
		t.Fatal(err)
	}
	srv := New(Config{SpendControl: sc})
	t.Cleanup(srv.Close)
	payload := []byte(`{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"test"}]}`)
	var request chatRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatal(err)
	}
	spend, allowed := srv.reserveChat(httptest.NewRecorder(), request, payload, request.Model)
	if !allowed {
		t.Fatal("reservation denied before storage failure")
	}

	// Replacing a directory with the state file fails deterministically on all
	// platforms, without depending on elevated users respecting file modes.
	const privateDetail = "private-backend-detail-sentinel"
	storage.Path = filepath.Join(t.TempDir(), privateDetail)
	if err := os.Mkdir(storage.Path, 0o700); err != nil {
		t.Fatal(err)
	}
	previous := log.Writer()
	var captured bytes.Buffer
	log.SetOutput(&captured)
	t.Cleanup(func() { log.SetOutput(previous) })
	spend.finish([]byte(`{"usage":{"prompt_tokens":10,"completion_tokens":5}}`))
	spend.finish(nil)
	if count := strings.Count(captured.String(), "spend settlement failed"); count != 1 {
		t.Fatalf("expected one observable settlement failure, got %d", count)
	}
	if strings.Contains(captured.String(), privateDetail) || strings.Contains(captured.String(), storage.Path) {
		t.Fatal("backend details leaked into logs")
	}
	if len(sc.GetHistory()) != 1 {
		t.Fatal("settlement was lost or repeated after storage failure")
	}
	if sc.Check(0).Allowed {
		t.Fatal("persistence failure did not keep admission closed")
	}
}
