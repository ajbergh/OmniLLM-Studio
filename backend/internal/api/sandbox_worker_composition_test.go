package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/sandbox"
)

// Verify the real application composition root creates a durable task schema
// and shared Broker only after the authenticated protocol-v2 handshake.
func TestRouterComposesConfiguredDurableSandboxWorker(t *testing.T) {
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/capabilities" || r.Header.Get("Authorization") != "Bearer test-worker-secret" {
			http.Error(w, "invalid sandbox handshake", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"name": "integration-stub"})
	}))
	defer runtime.Close()
	t.Setenv("OMNILLM_CODE_SANDBOX_URL", runtime.URL)
	t.Setenv("OMNILLM_SANDBOX_TOKEN", "test-worker-secret")
	previous := sandbox.DefaultBroker()
	t.Cleanup(func() { sandbox.SetDefaultBroker(previous) })
	database := newAgentRuntimeRouterTestDB(t)
	cfg := newAgentRuntimeRouterTestConfig(t)
	_, shutdown := NewRouterWithShutdown(database, cfg, "test", "test")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			t.Errorf("durable worker shutdown: %v", err)
		}
	})
	if sandbox.DefaultBroker() == nil {
		t.Fatal("configured sandbox did not install an authenticated Broker")
	}
	var tableName string
	if err := database.QueryRow("SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'sandbox_tasks'").Scan(&tableName); err != nil {
		if err == sql.ErrNoRows {
			t.Fatal("durable sandbox task schema was not initialized")
		}
		t.Fatalf("query durable sandbox task schema: %v", err)
	}
	if tableName != "sandbox_tasks" {
		t.Fatalf("unexpected task table: %q", tableName)
	}
}
