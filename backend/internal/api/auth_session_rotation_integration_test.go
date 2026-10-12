package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Exercise the browser cookie and direct API bearer paths against the same
// authenticated router. Re-login must invalidate the previous bearer while
// keeping the browser's HttpOnly session usable.
func TestSessionRotationCookieAndAPIClientRegression(t *testing.T) {
	database := newAgentRuntimeRouterTestDB(t)
	router, shutdown := NewRouterWithShutdown(database, newAgentRuntimeRouterTestConfig(t), "test", "test")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			t.Errorf("shutdown: %v", err)
		}
	})
	server := httptest.NewServer(router)
	defer server.Close()
	browser := chatHTTPNewClient(t, server.URL)

	resp, payload := browser.request(t, http.MethodPost, "/v1/auth/register", map[string]interface{}{
		"username": "session_rotation_user",
		"password": "rotation-test-password",
	})
	chatHTTPRequireStatus(t, resp, payload, http.StatusCreated)
	first, ok := chatHTTPObject(t, payload)["token"].(string)
	if !ok || first == "" {
		t.Fatal("missing first session token")
	}
	resp, payload = browser.request(t, http.MethodGet, "/v1/users/me", nil)
	chatHTTPRequireStatus(t, resp, payload, http.StatusOK)
	if chatHTTPObject(t, payload)["username"] != "session_rotation_user" {
		t.Fatalf("browser cookie user mismatch: %s", payload)
	}

	apiClient := &http.Client{Timeout: 5 * time.Second}
	checkBearer := func(token string, want int) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/users/me", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := apiClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != want {
			t.Fatalf("bearer status %d, want %d", res.StatusCode, want)
		}
	}
	checkBearer(first, http.StatusOK)

	resp, payload = browser.request(t, http.MethodPost, "/v1/auth/login", map[string]interface{}{
		"username": "session_rotation_user",
		"password": "rotation-test-password",
	})
	chatHTTPRequireStatus(t, resp, payload, http.StatusOK)
	second, ok := chatHTTPObject(t, payload)["token"].(string)
	if !ok || second == "" || second == first {
		t.Fatalf("login failed to rotate credentials")
	}
	checkBearer(first, http.StatusUnauthorized)
	checkBearer(second, http.StatusOK)
	resp, payload = browser.request(t, http.MethodGet, "/v1/users/me", nil)
	chatHTTPRequireStatus(t, resp, payload, http.StatusOK)

	resp, payload = browser.request(t, http.MethodPost, "/v1/auth/logout", map[string]interface{}{})
	chatHTTPRequireStatus(t, resp, payload, http.StatusOK)
	cleared := false
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "omnillm_session" && cookie.MaxAge < 0 && cookie.HttpOnly {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("logout did not clear the HttpOnly session cookie")
	}
	checkBearer(second, http.StatusUnauthorized)
	resp, payload = browser.request(t, http.MethodGet, "/v1/users/me", nil)
	chatHTTPRequireStatus(t, resp, payload, http.StatusUnauthorized)
}
