package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/config"
	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/go-chi/chi/v5"
)

func newAgentRuntimeRouterTestDB(t *testing.T) *sql.DB {
	t.Helper()
	// A SQLite :memory: database is private to each physical connection. The
	// production connection pool may open another connection during router
	// composition, especially under -race, which would observe an empty schema.
	// Use a temporary file so every pooled connection shares one migrated DB.
	database, err := db.Open(filepath.Join(t.TempDir(), "agent-runtime-router.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		database.Close()
		t.Fatalf("migrate test database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func newAgentRuntimeRouterTestConfig(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("OMNILLM_PLUGIN_DIR", filepath.Join(root, "plugins"))
	return &config.Config{
		BindAddress:        "127.0.0.1",
		AttachmentsDir:     filepath.Join(root, "attachments"),
		ChromemDir:         filepath.Join(root, "chromem"),
		BrowserCacheDir:    filepath.Join(root, "browser"),
		BrowserMaxSessions: 1,
		BrowserSessionTTL:  time.Minute,
		CORSOrigins:        []string{"http://localhost"},
		MaxUploadBytes:     16 << 20,
	}
}

func TestAgentRuntimeProductionRouterWiring(t *testing.T) {
	database := newAgentRuntimeRouterTestDB(t)
	handler, shutdown := NewRouterWithShutdown(database, newAgentRuntimeRouterTestConfig(t), "test", "test")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			t.Errorf("shutdown runtime services: %v", err)
		}
	})

	routes, ok := handler.(chi.Routes)
	if !ok {
		t.Fatalf("router does not implement chi.Routes: %T", handler)
	}

	registered := map[string]struct{}{}
	if err := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		registered[method+" "+route] = struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("walk routes: %v", err)
	}

	required := []string{
		"POST /v1/conversations/{conversationId}/agent/run",
		"GET /v1/conversations/{conversationId}/agent/runs",
		"GET /v1/agent/runs/{runId}/",
		"GET /v1/agent/runs/{runId}/events",
		"POST /v1/agent/runs/{runId}/approve/{stepId}",
		"POST /v1/agent/runs/{runId}/cancel",
		"POST /v1/agent/runs/{runId}/resume",
		"GET /v1/jobs/",
		"GET /v1/jobs/{jobId}",
		"POST /v1/jobs/{jobId}/cancel",
		"GET /v1/memories/",
		"POST /v1/memories/",
		"PATCH /v1/memories/{memoryId}",
		"DELETE /v1/memories/{memoryId}",
		"GET /v1/tasks/",
		"POST /v1/tasks/",
		"PATCH /v1/tasks/{taskId}",
		"DELETE /v1/tasks/{taskId}",
		"GET /v1/apps/catalog",
		"GET /v1/apps/connections",
		"POST /v1/apps/connections/mcp",
		"DELETE /v1/apps/connections/{connectionId}",
		"GET /v1/tools/approvals",
		"POST /v1/tools/approvals/{approvalId}",
		"GET /v1/eval/agent/scenarios",
		"POST /v1/eval/agent/run",
	}
	for _, route := range required {
		if _, exists := registered[route]; !exists {
			t.Errorf("required production route is not registered: %s", route)
		}
	}
}

func TestAgentRuntimeToolsAndDefaultPoliciesAreReachable(t *testing.T) {
	database := newAgentRuntimeRouterTestDB(t)
	handler, shutdown := NewRouterWithShutdown(database, newAgentRuntimeRouterTestConfig(t), "test", "test")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			t.Errorf("shutdown runtime services: %v", err)
		}
	})

	request := httptest.NewRequest(http.MethodGet, "/v1/tools/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("list tools status = %d, body = %s", response.Code, response.Body.String())
	}

	var toolsPayload []struct {
		Name   string `json:"name"`
		Policy string `json:"policy"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &toolsPayload); err != nil {
		t.Fatalf("decode tools response: %v", err)
	}
	actual := make(map[string]string, len(toolsPayload))
	for _, tool := range toolsPayload {
		actual[tool.Name] = tool.Policy
	}

	expectedPolicies := map[string]string{
		"job_status":        "allow",
		"job_cancel":        "ask",
		"memory_search":     "allow",
		"memory_save":       "ask",
		"memory_delete":     "ask",
		"image_generate":    "ask",
		"music_generate":    "ask",
		"video_generate":    "ask",
		"artifact_generate": "ask",
		"task_list":         "allow",
		"task_create":       "ask",
		"task_update":       "ask",
		"app_catalog":       "allow",
		"app_connections":   "allow",
		"app_connect_mcp":   "ask",
		"app_disconnect":    "ask",
	}
	missing := make([]string, 0)
	for name, policy := range expectedPolicies {
		if actual[name] != policy {
			missing = append(missing, name+"="+actual[name]+" (want "+policy+")")
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("agent runtime tool registration/policy mismatches: %v", missing)
	}
}
