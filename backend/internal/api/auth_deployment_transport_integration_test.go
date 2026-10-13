package api

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestAuthDeploymentTransportMatrix exercises actual session middleware, Origin
// protection, and cookie scope over plain HTTP, TLS, and TLS termination at an
// HTTP reverse proxy. It does not depend on a live provider or public network.
func TestAuthDeploymentTransportMatrix(t *testing.T) {
	for _, mode := range []string{"localhost HTTP", "direct HTTPS", "HTTPS reverse proxy"} {
		t.Run(mode, func(t *testing.T) {
			database := newAgentRuntimeRouterTestDB(t)
			cfg := newAgentRuntimeRouterTestConfig(t)
			cfg.AllowPublicReg = true
			router, shutdown := NewRouterWithShutdown(database, cfg, "test", "test")
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := shutdown(ctx); err != nil {
					t.Errorf("shutdown: %v", err)
				}
			})
			backend := httptest.NewServer(router)
			t.Cleanup(backend.Close)
			public := backend
			switch mode {
			case "direct HTTPS":
				public = httptest.NewTLSServer(router)
				t.Cleanup(public.Close)
			case "HTTPS reverse proxy":
				target, err := url.Parse(backend.URL)
				if err != nil {
					t.Fatal(err)
				}
				proxy := httputil.NewSingleHostReverseProxy(target)
				director := proxy.Director
				proxy.Director = func(r *http.Request) {
					director(r)
					// A real trusted ingress overwrites, never forwards, a
					// client-supplied protocol assertion.
					r.Header.Set("X-Forwarded-Proto", "https")
				}
				public = httptest.NewTLSServer(proxy)
				t.Cleanup(public.Close)
			}
			jar, err := cookiejar.New(nil)
			if err != nil {
				t.Fatal(err)
			}
			aliceHTTP := *public.Client()
			aliceHTTP.Jar = jar
			alice := chatHTTPTestClient{client: &aliceHTTP, base: public.URL}
			bobJar, err := cookiejar.New(nil)
			if err != nil {
				t.Fatal(err)
			}
			bobHTTP := *public.Client()
			bobHTTP.Jar = bobJar
			bob := chatHTTPTestClient{client: &bobHTTP, base: public.URL}
			apiClient := *public.Client()
			apiClient.Jar = nil
			api := chatHTTPTestClient{client: &apiClient, base: public.URL}
			secure := mode != "localhost HTTP"

			send := func(client chatHTTPTestClient, method, path, origin, bearer, body string, fetchSite string) (*http.Response, []byte) {
				t.Helper()
				req, err := http.NewRequest(method, client.base+path, strings.NewReader(body))
				if err != nil {
					t.Fatal(err)
				}
				if body != "" {
					req.Header.Set("Content-Type", "application/json")
				}
				if origin != "" {
					req.Header.Set("Origin", origin)
				}
				if fetchSite != "" {
					req.Header.Set("Sec-Fetch-Site", fetchSite)
				}
				if bearer != "" {
					req.Header.Set("Authorization", "Bearer "+bearer)
				}
				res, err := client.client.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				payload, err := io.ReadAll(res.Body)
				res.Body.Close()
				if err != nil {
					t.Fatal(err)
				}
				return res, payload
			}
			require := func(resp *http.Response, body []byte, expected int) {
				t.Helper()
				chatHTTPRequireStatus(t, resp, body, expected)
			}

			resp, body := send(alice, http.MethodPost, "/v1/auth/register", public.URL, "",
				`{"username":"matrix_alice","password":"matrix-alice-password"}`, "")
			require(resp, body, http.StatusCreated)
			firstToken, ok := chatHTTPObject(t, body)["token"].(string)
			if !ok || firstToken == "" {
				t.Fatalf("registration returned no token: %s", body)
			}
			cookieAssert := func(resp *http.Response) {
				t.Helper()
				for _, cookie := range resp.Cookies() {
					if cookie.Name != "omnillm_session" {
						continue
					}
					if !cookie.HttpOnly || cookie.Secure != secure ||
						cookie.Path != "/v1" || cookie.Domain != "" ||
						cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge <= 0 {
						t.Fatalf("session cookie transport mismatch (%s): %+v", mode, cookie)
					}
					return
				}
				t.Fatal("missing HttpOnly session cookie")
			}
			cookieAssert(resp)

			resp, body = send(alice, http.MethodGet, "/v1/users/me", "", "", "", "")
			require(resp, body, http.StatusOK)
			if chatHTTPObject(t, body)["username"] != "matrix_alice" {
				t.Fatalf("cookie failed across %s: %s", mode, body)
			}
			resp, body = send(api, http.MethodGet, "/v1/users/me", "", "", "", "")
			require(resp, body, http.StatusUnauthorized)
			resp, body = send(api, http.MethodGet, "/v1/users/me", "", firstToken, "", "")
			require(resp, body, http.StatusOK)

			createPath := "/v1/conversations/"
			createJSON := `{"title":"Protected via transport matrix"}`
			for _, origin := range []string{"https://attacker.example", "null", public.URL + ".attacker.example"} {
				resp, body = send(alice, http.MethodPost, createPath, origin, "", createJSON, "")
				require(resp, body, http.StatusForbidden)
			}
			resp, body = send(alice, http.MethodPost, createPath, "", "", createJSON, "cross-site")
			require(resp, body, http.StatusForbidden)
			resp, body = send(alice, http.MethodPost, createPath, "", "", createJSON, "same-site")
			require(resp, body, http.StatusForbidden)
			resp, body = send(alice, http.MethodPost, createPath, public.URL, "invalid-api-token", createJSON, "")
			require(resp, body, http.StatusUnauthorized)
			resp, body = send(alice, http.MethodGet, "/v1/users/me", "", "", "", "")
			require(resp, body, http.StatusOK)

			resp, body = send(alice, http.MethodPost, createPath, public.URL, "", createJSON, "")
			require(resp, body, http.StatusCreated)
			conversationID, ok := chatHTTPObject(t, body)["id"].(string)
			if !ok || conversationID == "" {
				t.Fatalf("missing conversation id: %s", body)
			}
			resp, body = send(api, http.MethodPost, createPath, "https://external-api.example", firstToken, createJSON, "")
			require(resp, body, http.StatusCreated)

			resp, body = send(bob, http.MethodPost, "/v1/auth/register", public.URL, "",
				`{"username":"matrix_bob","password":"matrix-bob-password"}`, "")
			require(resp, body, http.StatusCreated)
			messages := "/v1/conversations/" + conversationID + "/messages"
			resp, body = send(bob, http.MethodGet, messages, "", "", "", "")
			require(resp, body, http.StatusNotFound)
			resp, body = send(bob, http.MethodPost, messages+"/stream", public.URL, "",
				`{"content":"cross-user SSE","web_search":false,"tool_mode":"none"}`, "")
			require(resp, body, http.StatusNotFound)
			resp, body = send(api, http.MethodPost, messages+"/stream", public.URL, "",
				`{"content":"anonymous SSE","web_search":false,"tool_mode":"none"}`, "")
			require(resp, body, http.StatusUnauthorized)

			resp, body = send(alice, http.MethodPost, "/v1/auth/login", public.URL, "",
				`{"username":"matrix_alice","password":"matrix-alice-password"}`, "")
			require(resp, body, http.StatusOK)
			cookieAssert(resp)
			secondToken, ok := chatHTTPObject(t, body)["token"].(string)
			if !ok || secondToken == firstToken || secondToken == "" {
				t.Fatal("login did not rotate session")
			}
			resp, body = send(api, http.MethodGet, "/v1/users/me", "", firstToken, "", "")
			require(resp, body, http.StatusUnauthorized)
			resp, body = send(api, http.MethodGet, "/v1/users/me", "", secondToken, "", "")
			require(resp, body, http.StatusOK)

			resp, body = send(alice, http.MethodPost, "/v1/auth/logout", "https://attacker.example", "", "", "")
			require(resp, body, http.StatusForbidden)
			resp, body = send(alice, http.MethodGet, "/v1/users/me", "", "", "", "")
			require(resp, body, http.StatusOK)
			resp, body = send(alice, http.MethodPost, "/v1/auth/logout", public.URL, "", "", "")
			require(resp, body, http.StatusOK)
			cleared := false
			for _, cookie := range resp.Cookies() {
				if cookie.Name == "omnillm_session" && cookie.MaxAge == -1 &&
					cookie.HttpOnly && cookie.Secure == secure && cookie.Path == "/v1" {
					cleared = true
				}
			}
			if !cleared {
				t.Fatal("logout failed to clear cookie with matching transport scope")
			}
			resp, body = send(api, http.MethodGet, "/v1/users/me", "", secondToken, "", "")
			require(resp, body, http.StatusUnauthorized)
			resp, body = send(alice, http.MethodGet, "/v1/users/me", "", "", "", "")
			require(resp, body, http.StatusUnauthorized)

			resp, body = send(alice, http.MethodPost, "/v1/auth/login", public.URL, "",
				`{"username":"matrix_alice","password":"matrix-alice-password"}`, "")
			require(resp, body, http.StatusOK)
			expiring, ok := chatHTTPObject(t, body)["token"].(string)
			if !ok || expiring == "" {
				t.Fatalf("missing expiry test token: %s", body)
			}
			if _, err := database.Exec("UPDATE sessions SET expires_at = ? WHERE user_id = (SELECT id FROM users WHERE username = ?)",
				time.Now().UTC().Add(-time.Hour), "matrix_alice"); err != nil {
				t.Fatal(err)
			}
			resp, body = send(alice, http.MethodGet, "/v1/users/me", "", "", "", "")
			require(resp, body, http.StatusUnauthorized)
			resp, body = send(api, http.MethodGet, "/v1/users/me", "", expiring, "", "")
			require(resp, body, http.StatusUnauthorized)
		})
	}
}
