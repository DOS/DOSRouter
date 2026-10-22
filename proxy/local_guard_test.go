package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalTrustBoundary(t *testing.T) {
	for _, tc := range []struct {
		host, origin, site string
		want               int
	}{
		{"127.0.0.1:8080", "", "", 204},
		{"localhost:8080", "http://localhost:3000", "same-site", 204},
		{"[::1]:8080", "https://[::1]", "", 204},
		{"attacker.example", "", "", 403},
		{"127.0.0.1", "https://attacker.example", "", 403},
		{"127.0.0.1", "null", "", 403},
		{"127.0.0.1", "file://localhost", "", 403},
		{"127.0.0.1", "http://localhost/path", "", 403},
		{"127.0.0.1", "", "cross-site", 403},
	} {
		t.Run(tc.host+tc.origin+tc.site, func(t *testing.T) {
			req := httptest.NewRequest("POST", "http://"+tc.host+"/v1/images/generations", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			req.Header.Set("Sec-Fetch-Site", tc.site)
			w := httptest.NewRecorder()
			localOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })).ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d", w.Code, tc.want)
			}
		})
	}
}

func TestAllRoutesRejectRemoteBrowsers(t *testing.T) {
	srv, _ := syncTestServer(t, func(http.ResponseWriter, *http.Request) { t.Fatal("untrusted request reached upstream") }, nil)
	for _, path := range []string{"/health", "/v1/models", "/v1/chat/completions", "/v1/images/generations", "/v1/crypto/price/BTC"} {
		req := httptest.NewRequest("POST", "http://localhost"+path, nil)
		req.Header.Set("Origin", "https://attacker.example")
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)
		if w.Code != 403 {
			t.Errorf("%s: status %d", path, w.Code)
		}
	}
}
