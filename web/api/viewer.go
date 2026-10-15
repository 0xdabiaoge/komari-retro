package api

import (
	"net/http"
	"strings"
)

// ViewerHandler is a separate, deliberately credential-blind data transport.
// A viewer tab must not inherit a privileged session from another browser tab.
func ViewerHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/viewer-api/") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "private, no-store, max-age=0")
		w.Header().Set("Vary", "Cookie")
		rel := strings.TrimPrefix(r.URL.Path, "/viewer-api")
		allowed := rel == "/rpc2" || rel == "/public" || rel == "/nodes" || rel == "/version" || rel == "/clients" || rel == "/records/load" || rel == "/records/ping" || rel == "/task/ping" || (strings.HasPrefix(rel, "/recent/") && !strings.Contains(strings.TrimPrefix(rel, "/recent/"), "/"))
		if !allowed || strings.Contains(rel, "..") || strings.Contains(rel, "\\") || (r.Method != http.MethodGet && !(rel == "/rpc2" && r.Method == http.MethodPost)) {
			http.NotFound(w, r)
			return
		}
		clone := r.Clone(r.Context())
		clone.URL.Path = "/api" + rel
		clone.URL.RawPath = ""
		clone.Header.Del("Authorization")
		clone.Header.Del("Cookie")
		for _, cookie := range r.Cookies() {
			if cookie.Name == "temp_key" || cookie.Name == "language" {
				clone.AddCookie(cookie)
			}
		}
		query := clone.URL.Query()
		query.Del("token")
		query.Del("Authorization")
		clone.URL.RawQuery = query.Encode()
		next.ServeHTTP(w, clone)
	})
}
