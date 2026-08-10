package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestMCPOAuthServerIDMatchesRouterParameter(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/v1/mcp/servers/{serverId}/oauth", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, mcpOAuthServerID(r))
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/mcp/servers/server-123/oauth", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "server-123" {
		t.Fatalf("OAuth route server id = %q status=%d", recorder.Body.String(), recorder.Code)
	}
}
