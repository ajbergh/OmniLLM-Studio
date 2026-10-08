package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Auth status is a browser bootstrap probe, not a credential attempt. Keeping
// it independent from the shared login/registration limiter prevents unrelated
// page loads from being mistaken for repeated password attempts.
func TestAuthStatusProbeDoesNotExhaustCredentialAttemptLimiter(t *testing.T) {
	database := newAgentRuntimeRouterTestDB(t)
	router, shutdown := NewRouterWithShutdown(database, newAgentRuntimeRouterTestConfig(t), "test", "test")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			t.Errorf("shutdown runtime: %v", err)
		}
	})

	for i := 0; i < 20; i++ {
		request := httptest.NewRequest(http.MethodGet, "/v1/auth/status", nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("status request %d = %d, want 200; body=%s", i+1, response.Code, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), "auth_enabled") {
			t.Fatalf("missing auth status response: %s", response.Body.String())
		}
	}

	// The status probes must not spend the budget for sensitive operations.
	for i := 0; i < 10; i++ {
		request := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"username":"nonexistent","password":"wrong"}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("login attempt %d = %d, want 401; body=%s", i+1, response.Code, response.Body.String())
		}
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"username":"nonexistent","password":"wrong"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("11th login attempt = %d, want 429; body=%s", response.Code, response.Body.String())
	}
}
