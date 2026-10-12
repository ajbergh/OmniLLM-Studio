package api

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/auth"
)

func TestSessionCookieTransportPolicy(t *testing.T) {
	cases := []struct {
		name      string
		tls       bool
		forwarded string
		secure    bool
	}{
		{name: "direct localhost HTTP"},
		{name: "direct HTTPS", tls: true, secure: true},
		{name: "TLS terminating reverse proxy", forwarded: "https", secure: true},
		{name: "case insensitive forwarded HTTPS", forwarded: " HTTPS ", secure: true},
		{name: "forwarded plain HTTP", forwarded: "http"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/v1/auth/login", nil)
			if tc.tls {
				req.TLS = &tls.ConnectionState{}
			}
			if tc.forwarded != "" {
				req.Header.Set("X-Forwarded-Proto", tc.forwarded)
			}
			recorder := httptest.NewRecorder()
			setSessionCookie(recorder, req, "test-session", time.Now().Add(time.Hour))
			cookies := recorder.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("session cookie count = %d, want 1", len(cookies))
			}
			cookie := cookies[0]
			if cookie.Name != auth.SessionCookieName || cookie.Value != "test-session" ||
				cookie.Path != "/v1" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode ||
				cookie.Secure != tc.secure || cookie.MaxAge <= 0 {
				t.Fatalf("invalid session cookie in %s: %+v", tc.name, cookie)
			}

			logoutRecorder := httptest.NewRecorder()
			clearSessionCookie(logoutRecorder, req)
			cleared := logoutRecorder.Result().Cookies()
			if len(cleared) != 1 || cleared[0].Name != auth.SessionCookieName ||
				cleared[0].Path != "/v1" || cleared[0].MaxAge != -1 ||
				cleared[0].Secure != tc.secure || !cleared[0].HttpOnly ||
				cleared[0].SameSite != http.SameSiteLaxMode {
				t.Fatalf("logout cookie must expire identical transport scope: %+v", cleared)
			}
		})
	}
}

func TestCookieAuthCannotOverrideExplicitBearerToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/v1/users/me", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "browser-session"})
	if got := auth.TokenFromRequest(req); got != "browser-session" {
		t.Fatalf("cookie fallback = %q, want browser session", got)
	}
	req.Header.Set("Authorization", "Bearer api-client-token")
	if got := auth.TokenFromRequest(req); got != "api-client-token" {
		t.Fatalf("explicit API bearer = %q, want API client token", got)
	}
	req.Header.Set("Authorization", "not-a-bearer")
	if got := auth.TokenFromRequest(req); got != "browser-session" {
		t.Fatalf("malformed API header must not shadow cookie: %q", got)
	}
}
