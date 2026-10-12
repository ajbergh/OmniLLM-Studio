package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBrowserMutationOriginPolicy(t *testing.T) {
	testCases := []struct {
		name       string
		method     string
		host       string
		origin     string
		proto      string
		fetchSite  string
		bearer     string
		wantStatus int
	}{
		{name: "same origin", method: "POST", host: "localhost:8080", origin: "http://localhost:8080", wantStatus: http.StatusNoContent},
		{name: "configured Vite origin", method: "PATCH", host: "localhost:8080", origin: "http://localhost:5173", wantStatus: http.StatusNoContent},
		{name: "configured Playwright Vite proxy origin", method: "POST", host: "127.0.0.1:8090", origin: "http://127.0.0.1:4173", wantStatus: http.StatusNoContent},
		{name: "adjacent loopback port rejected", method: "POST", host: "127.0.0.1:8090", origin: "http://127.0.0.1:4174", wantStatus: http.StatusForbidden},
		{name: "HTTPS reverse proxy", method: "DELETE", host: "api.example.test", origin: "https://api.example.test", proto: "https", wantStatus: http.StatusNoContent},
		{name: "HTTP behind HTTPS proxy denied", method: "POST", host: "api.example.test", origin: "http://api.example.test", proto: "https", wantStatus: http.StatusForbidden},
		{name: "other localhost port", method: "POST", host: "localhost:8080", origin: "http://localhost:9090", wantStatus: http.StatusForbidden},
		{name: "untrusted remote site", method: "POST", host: "localhost:8080", origin: "https://attacker.example", wantStatus: http.StatusForbidden},
		{name: "host prefix is not origin match", method: "POST", host: "localhost:8080", origin: "http://localhost:8080.attacker.example", wantStatus: http.StatusForbidden},
		{name: "opaque null origin", method: "POST", host: "localhost:8080", origin: "null", wantStatus: http.StatusForbidden},
		{name: "malformed multi-origin header", method: "POST", host: "localhost:8080", origin: "http://localhost:8080 https://evil.example", wantStatus: http.StatusForbidden},
		{name: "cross-site fetch metadata without origin", method: "POST", host: "localhost:8080", fetchSite: "cross-site", wantStatus: http.StatusForbidden},
		{name: "same-site non-origin fetch metadata", method: "POST", host: "localhost:8080", fetchSite: "same-site", wantStatus: http.StatusForbidden},
		{name: "originless direct API", method: "POST", host: "localhost:8080", wantStatus: http.StatusNoContent},
		{name: "bearer API client from other origin", method: "POST", host: "localhost:8080", origin: "https://external.app", bearer: "test-api-token", wantStatus: http.StatusNoContent},
		{name: "bearer API client with cross-site metadata", method: "POST", host: "localhost:8080", fetchSite: "cross-site", bearer: "test-api-token", wantStatus: http.StatusNoContent},
		{name: "read-only requests do not require trusted origin", method: "GET", host: "localhost:8080", origin: "https://attacker.example", wantStatus: http.StatusNoContent},
		{name: "preflight handled independently", method: "OPTIONS", host: "localhost:8080", origin: "https://attacker.example", wantStatus: http.StatusNoContent},
	}

	handler := guardBrowserMutationOrigin([]string{"http://localhost:5173", "http://127.0.0.1:4173", "*"})(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "http://"+tc.host+"/v1/conversations", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.proto != "" {
				req.Header.Set("X-Forwarded-Proto", tc.proto)
			}
			if tc.fetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", tc.fetchSite)
			}
			if tc.bearer != "" {
				req.Header.Set("Authorization", "Bearer "+tc.bearer)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d, response = %s", w.Code, tc.wantStatus, w.Body.String())
			}
		})
	}
}

func TestBrowserOriginProtectsAuthenticatedLogout(t *testing.T) {
	database := newAgentRuntimeRouterTestDB(t)
	cfg := newAgentRuntimeRouterTestConfig(t)
	cfg.CORSOrigins = []string{"http://localhost:5173"}

	router, shutdown := NewRouterWithShutdown(database, cfg, "test", "test")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			t.Errorf("router shutdown: %v", err)
		}
	})
	server := httptest.NewServer(router)
	defer server.Close()
	browser := chatHTTPNewClient(t, server.URL)

	resp, payload := browser.request(t, http.MethodPost, "/v1/auth/register", map[string]interface{}{
		"username": "origin_protected_user", "password": "origin-test-password",
	})
	chatHTTPRequireStatus(t, resp, payload, http.StatusCreated)

	postLogout := func(origin string, want int) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/auth/logout", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", origin)
		res, err := browser.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		chatHTTPRequireStatus(t, res, body, want)
	}
	postLogout("http://localhost:9090", http.StatusForbidden)
	resp, payload = browser.request(t, http.MethodGet, "/v1/users/me", nil)
	chatHTTPRequireStatus(t, resp, payload, http.StatusOK)

	postLogout("http://localhost:5173", http.StatusOK)
	resp, payload = browser.request(t, http.MethodGet, "/v1/users/me", nil)
	chatHTTPRequireStatus(t, resp, payload, http.StatusUnauthorized)
}

func TestConfiguredViteProxyOriginCreatesAuthenticatedConversation(t *testing.T) {
	database := newAgentRuntimeRouterTestDB(t)
	cfg := newAgentRuntimeRouterTestConfig(t)
	cfg.CORSOrigins = []string{"http://127.0.0.1:4173"}

	router, shutdown := NewRouterWithShutdown(database, cfg, "test", "test")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			t.Errorf("router shutdown: %v", err)
		}
	})
	server := httptest.NewServer(router)
	defer server.Close()
	browser := chatHTTPNewClient(t, server.URL)

	resp, payload := browser.request(t, http.MethodPost, "/v1/auth/register", map[string]interface{}{
		"username": "vite_proxy_user", "password": "vite-proxy-test-password",
	})
	chatHTTPRequireStatus(t, resp, payload, http.StatusCreated)

	create := func(origin string, want int) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/conversations",
			strings.NewReader(`{"title":"Vite proxy origin regression"}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", origin)
		res, err := browser.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		chatHTTPRequireStatus(t, res, body, want)
		if want == http.StatusCreated && chatHTTPObject(t, body)["id"] == "" {
			t.Fatalf("authenticated conversation create returned no id: %s", body)
		}
	}
	// The Vite proxy forwards its backend Host but preserves the browser Origin.
	create("http://127.0.0.1:4173", http.StatusCreated)
	// A malicious page on an adjacent loopback port cannot mutate the session.
	create("http://127.0.0.1:4174", http.StatusForbidden)
}
