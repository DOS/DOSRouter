package proxy

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

func localURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trusted := r.Host != "" && localURL("http://"+r.Host)
		origins, hasOrigin := r.Header["Origin"]
		if hasOrigin {
			trusted = trusted && len(origins) == 1 && localURL(origins[0])
		}
		if strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site") {
			trusted = false
		}
		if !trusted {
			http.Error(w, "Local clients only", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requiresStructuredOutput(req chatRequest) bool {
	var format struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(req.Extra["response_format"], &format) != nil {
		return false
	}
	return format.Type == "json_object" || format.Type == "json_schema"
}
