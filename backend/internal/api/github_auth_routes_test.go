package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestMountGitHubAuthRoutesExposesOnlyBoundedConnectionSurface(t *testing.T) {
	router := chi.NewRouter()
	MountGitHubAuthRoutes(router, NewGitHubAuthHandler(nil))

	tests := []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodGet, "/github/auth", http.StatusOK},
		{http.MethodPost, "/github/auth/device/start", http.StatusServiceUnavailable},
		{http.MethodPost, "/github/auth/device/poll", http.StatusServiceUnavailable},
		{http.MethodDelete, "/github/auth", http.StatusNoContent},
	}
	for _, test := range tests {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
		if recorder.Code != test.want {
			t.Fatalf("%s %s = %d, want %d", test.method, test.path, recorder.Code, test.want)
		}
	}

	for _, path := range []string{"/github/auth/token", "/github/auth/device/code", "/github/auth/credentials"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("unexpected secret-bearing route %s returned %d", path, recorder.Code)
		}
	}
}
