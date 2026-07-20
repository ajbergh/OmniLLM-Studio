package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestDesktopLoopbackHandlerRejectsRequestsWithoutSecretPath(t *testing.T) {
	handler := desktopLoopbackHandler("/__desktop/launch-secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			t.Fatalf("router received path %q, want /v1/health", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, requestPath := range []string{"/v1/health", "/__desktop/wrong-secret/v1/health"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, requestPath, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s returned %d, want %d", requestPath, recorder.Code, http.StatusNotFound)
		}
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/__desktop/launch-secret/v1/health", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("protected request returned %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestSetDesktopDefaultsEnablesBrowserRuntime(t *testing.T) {
	t.Setenv("OMNILLM_CORS_ORIGINS", "http://wails.localhost")
	t.Setenv("OMNILLM_DB_PATH", "test.db")
	t.Setenv("OMNILLM_ATTACHMENTS_DIR", "attachments")
	t.Setenv("OMNILLM_BROWSER_ENABLED", "")

	setDesktopDefaults()

	if got := os.Getenv("OMNILLM_BROWSER_ENABLED"); got != "true" {
		t.Fatalf("OMNILLM_BROWSER_ENABLED = %q, want true", got)
	}
}

func TestSetDesktopDefaultsPreservesExplicitBrowserSetting(t *testing.T) {
	t.Setenv("OMNILLM_CORS_ORIGINS", "http://wails.localhost")
	t.Setenv("OMNILLM_DB_PATH", "test.db")
	t.Setenv("OMNILLM_ATTACHMENTS_DIR", "attachments")
	t.Setenv("OMNILLM_BROWSER_ENABLED", "false")

	setDesktopDefaults()

	if got := os.Getenv("OMNILLM_BROWSER_ENABLED"); got != "false" {
		t.Fatalf("OMNILLM_BROWSER_ENABLED = %q, want false", got)
	}
}
