package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/config"
	appdb "github.com/ajbergh/omnillm-studio/internal/db"
)

func TestAgentRuntimeProductionRoutesAndPolicies(t *testing.T) {
	t.Setenv("OMNILLM_MASTER_KEY", strings.Repeat("0", 64))
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	attachments := filepath.Join(root, "attachments")
	if err := os.MkdirAll(attachments, 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := appdb.Open(filepath.Join(root, "router.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := appdb.Migrate(database); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		BindAddress: "127.0.0.1", AttachmentsDir: attachments,
		ChromemDir: filepath.Join(root, "chromem"), CORSOrigins: []string{"http://localhost"},
		BrowserCacheDir: filepath.Join(root, "browser"), BrowserMaxSessions: 1,
		BrowserSessionTTL: time.Minute, MaxUploadBytes: 1 << 20,
	}
	router, shutdown := NewRouterWithShutdown(database, cfg, "test", "test")
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			t.Errorf("shutdown: %v", err)
		}
	}()

	for _, path := range []string{
		"/v1/apps/catalog", "/v1/apps/connections", "/v1/jobs/", "/v1/memories/",
		"/v1/tasks/", "/v1/tools/approvals", "/v1/eval/agent/scenarios",
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET %s returned %d: %s", path, recorder.Code, recorder.Body.String())
		}
	}

	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/v1/agent/runs/missing/events", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("expected event replay route to return 404 for missing run, got %d", missing.Code)
	}

	toolsResponse := httptest.NewRecorder()
	router.ServeHTTP(toolsResponse, httptest.NewRequest(http.MethodGet, "/v1/tools/", nil))
	if toolsResponse.Code != http.StatusOK {
		t.Fatalf("tool list returned %d: %s", toolsResponse.Code, toolsResponse.Body.String())
	}
	var definitions []struct {
		Name   string `json:"name"`
		Policy string `json:"policy"`
	}
	if err := json.Unmarshal(toolsResponse.Body.Bytes(), &definitions); err != nil {
		t.Fatal(err)
	}
	policies := make(map[string]string, len(definitions))
	for _, definition := range definitions {
		policies[definition.Name] = definition.Policy
	}
	for _, name := range []string{
		"date_time", "unit_convert", "weather_lookup", "currency_convert", "job_status",
		"memory_search", "task_list", "app_catalog", "image_generate", "music_generate",
		"video_generate", "artifact_generate",
	} {
		if _, ok := policies[name]; !ok {
			t.Errorf("runtime tool %s is not registered", name)
		}
	}
	for _, name := range []string{
		"job_cancel", "memory_save", "memory_delete", "image_generate", "music_generate",
		"video_generate", "artifact_generate", "python_analysis", "task_create", "task_update",
		"app_connect_mcp", "app_disconnect",
	} {
		if policies[name] != "ask" {
			t.Errorf("expected %s policy ask, got %q", name, policies[name])
		}
	}
}
