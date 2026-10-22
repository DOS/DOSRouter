package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/DOS/DOSRouter/models"
	"github.com/DOS/DOSRouter/spendcontrol"
)

func TestCatalogProjectionKeepsProductPolicy(t *testing.T) {
	body := []byte(`{"data":[
 {"id":"openai/gpt-6-sol","name":"Updated Sol","categories":["chat"],"billing_mode":"paid","pricing":{"input":3,"output":11},"context_window":900000,"max_output":100000},
 {"id":"vendor/new","name":"New","categories":["chat","vision","coding"],"billing_mode":"flat","pricing":{"flat":0.25}},
 {"id":"nvidia/nemotron-3.5-lightning","name":"Free","categories":["chat"],"billing_mode":"free","pricing":{}}
 ]}`)
	next, err := projectCatalog(body)
	if err != nil {
		t.Fatal(err)
	}
	if m := next.model("openai/gpt-6-sol"); m == nil || m.InputPrice != 3 || !m.ToolCalling || m.ContextWindow != 900000 {
		t.Fatalf("known capability or metadata lost: %+v", m)
	}
	if m := next.model("vendor/new"); m == nil || m.ToolCalling || m.Agentic || !m.Vision {
		t.Fatalf("new model acquired unverified tools: %+v", m)
	}
	if p := next.pricing["vendor/new"]; p.FlatPrice == nil || *p.FlatPrice != 0.25 {
		t.Fatal("flat price omitted")
	}
	if models.GetModel("openai/gpt-6-sol").InputPrice != 2 || models.GetModel("vendor/new") != nil {
		t.Fatal("catalog escaped server ownership")
	}
	if models.ResolveModelAlias("gpt5") != "openai/gpt-5.6-terra" || next.pricing["free/nemotron-3.5-lightning"].InputPrice != 0 {
		t.Fatal("product alias/free policy changed")
	}
}

func TestCatalogRejectsWholeInvalidUpdate(t *testing.T) {
	for _, body := range []string{
		`{"data":[]}`,
		`{"data":[{"id":"new/model","name":"New","categories":["chat"],"billing_mode":"paid","pricing":{"input":1}}]}`,
		`{"data":[{"id":"new/model","name":"New","categories":["chat"],"billing_mode":"flat","pricing":{"flat":-1}}]}`,
		`{"data":[{"id":"nvidia/nemotron-3.5-lightning","name":"Free","categories":["chat"],"billing_mode":"paid","pricing":{"input":1,"output":1}}]}`,
		`{"data":[{"id":"bad\n/model","name":"Bad","categories":["chat"],"billing_mode":"free","pricing":{}}]}`,
	} {
		if _, err := projectCatalog([]byte(body)); err == nil {
			t.Errorf("accepted invalid catalog: %s", body)
		}
	}
}

func TestCatalogRefreshIsolatedAndKeepsLastGood(t *testing.T) {
	var valid atomic.Bool
	valid.Store(true)
	srv, _ := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.URL.Query().Get("format") != "json" {
			t.Errorf("wrong catalog URL: %s", r.URL)
		}
		if valid.Load() {
			io.WriteString(w, `{"data":[{"id":"vendor/new","name":"New","categories":["chat"],"billing_mode":"paid","pricing":{"input":1,"output":2}}]}`)
		} else {
			io.WriteString(w, `{"data":[]}`)
		}
	}, nil)
	other, _ := syncTestServer(t, func(http.ResponseWriter, *http.Request) { t.Fatal("another server was contacted") }, nil)
	if err := srv.refreshCatalog(context.Background()); err != nil {
		t.Fatal(err)
	}
	old := srv.catalogView()
	if old.model("vendor/new") == nil || other.catalogView().model("vendor/new") != nil {
		t.Fatal("cross-server catalog contamination")
	}
	valid.Store(false)
	if err := srv.refreshCatalog(context.Background()); err == nil {
		t.Fatal("bad update accepted")
	}
	if srv.catalogView().model("vendor/new") == nil {
		t.Fatal("last good catalog discarded")
	}
	w := httptest.NewRecorder()
	srv.handleModels(w, httptest.NewRequest("GET", "/v1/models", nil))
	if !strings.Contains(w.Body.String(), "vendor/new") {
		t.Fatal("new model absent from public listing")
	}
}

func TestCatalogRefreshDoesNotFollowRedirects(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("catalog followed redirect") }))
	defer destination.Close()
	srv, _ := syncTestServer(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) }, nil)
	if err := srv.refreshCatalog(context.Background()); err == nil {
		t.Fatal("redirect accepted")
	}
}

func TestRemoteFreeClaimCannotBypassSpendLimits(t *testing.T) {
	srv, sc := syncTestServer(t, nil, nil)
	if err := sc.SetLimit(spendcontrol.WindowSession, 1); err != nil {
		t.Fatal(err)
	}
	next, err := projectCatalog([]byte(`{"data":[{"id":"vendor/free-claim","name":"Free claim","categories":["chat"],"billing_mode":"free","pricing":{}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	srv.catalog = &next
	w := httptest.NewRecorder()
	if _, allowed := srv.reserveChat(w, chatRequest{}, []byte(`{}`), "vendor/free-claim"); allowed || w.Code != 429 {
		t.Fatalf("untrusted free claim bypassed limits: %d", w.Code)
	}
}
