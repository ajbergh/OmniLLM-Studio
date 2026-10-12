package api

import (
	"net/http"
	"net/url"
	"strings"
)

// guardBrowserMutationOrigin protects cookie-authenticated mutations from
// cross-origin form posts, including requests from another localhost port.
// CORS alone cannot stop a browser from submitting a request whose response
// it cannot read. API clients with an explicit bearer token retain their
// existing cross-origin transport contract.
func guardBrowserMutationOrigin(trustedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(trustedOrigins))
	for _, origin := range trustedOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" && origin != "*" {
			allowed[origin] = true
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
				next.ServeHTTP(w, r)
				return
			}

			// Native desktop and programmatic API calls may explicitly supply
			// bearer credentials. A forged bearer header cannot authenticate
			// with the session cookie if its token is invalid.
			parts := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") && strings.TrimSpace(parts[1]) != "" {
				next.ServeHTTP(w, r)
				return
			}

			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin == "" {
				// Origin-less API clients remain supported. Browser Fetch
				// Metadata explicitly identifies suspicious site-originated
				// mutations even when Origin is unexpectedly absent.
				switch strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))) {
				case "cross-site", "same-site":
					respondError(w, http.StatusForbidden, "untrusted request origin")
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			parsed, err := url.Parse(origin)
			if err != nil || parsed.Host == "" || parsed.User != nil ||
				parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" ||
				(parsed.Scheme != "http" && parsed.Scheme != "https") {
				respondError(w, http.StatusForbidden, "untrusted request origin")
				return
			}

			if allowed[origin] {
				next.ServeHTTP(w, r)
				return
			}
			scheme := "http"
			if requestIsHTTPS(r) {
				scheme = "https"
			}
			if parsed.Scheme == scheme && strings.EqualFold(parsed.Host, r.Host) {
				next.ServeHTTP(w, r)
				return
			}
			respondError(w, http.StatusForbidden, "untrusted request origin")
		})
	}
}
