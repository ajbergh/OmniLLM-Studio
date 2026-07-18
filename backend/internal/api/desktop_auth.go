package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

const (
	desktopTokenHeader = "X-OmniLLM-Desktop-Token"
	desktopTokenQuery  = "desktop_token"
)

// DesktopTokenAuth protects the desktop loopback API with a random per-launch
// token. Normal server/web deployments pass an empty token and skip this layer.
//
// Header authentication is used for API calls. GET and HEAD requests may also
// supply the token in the query string because browser media elements cannot set
// custom request headers. Query-token requests are explicitly non-cacheable.
func DesktopTokenAuth(expected string) func(http.Handler) http.Handler {
	expected = strings.TrimSpace(expected)
	return func(next http.Handler) http.Handler {
		if expected == "" {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := strings.TrimSpace(r.Header.Get(desktopTokenHeader))
			usedQuery := false
			if provided == "" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
				provided = strings.TrimSpace(r.URL.Query().Get(desktopTokenQuery))
				usedQuery = provided != ""
			}
			if !constantTimeStringEqual(provided, expected) {
				respondError(w, http.StatusUnauthorized, "desktop API authorization required")
				return
			}
			if usedQuery {
				w.Header().Set("Cache-Control", "no-store, private")
				w.Header().Set("Referrer-Policy", "no-referrer")
			}
			next.ServeHTTP(w, r)
		})
	}
}

func constantTimeStringEqual(a, b string) bool {
	if len(a) != len(b) || len(a) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
