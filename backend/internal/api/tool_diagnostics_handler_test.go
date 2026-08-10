package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	_ "modernc.org/sqlite"
)

func TestToolDiagnosticsHandlerOmitsPayloadBearingAuditFields(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if err := repository.EnsureAgentRuntimeSchema(db); err != nil {
		t.Fatalf("ensure agent runtime schema: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO tool_invocations (
			id, tool_call_id, tool_name, user_id, status, approval_status,
			arguments_json, result_json, error_message, duration_ms, result_bytes, retry_count
		) VALUES ('diag-1', 'call-diag-1', 'calculator', ?, 'tool_completed', 'approved', ?, ?, ?, 12, 64, 0)
	`, auth.LocalScopeUserID, `{"secret":"argument"}`, `{"secret":"result"}`, "private provider failure"); err != nil {
		t.Fatalf("insert tool invocation: %v", err)
	}

	handler := NewToolDiagnosticsHandler(repository.NewToolInvocationRepo(db))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/tools/diagnostics?limit=10", nil)
	handler.Get(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, forbidden := range []string{"arguments_json", "result_json", "error_message", "argument", "private provider failure"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("diagnostics response leaked %q: %s", forbidden, body)
		}
	}

	var response struct {
		Scope       string                             `json:"scope"`
		Invocations []repository.ToolInvocationSummary `json:"invocations"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode diagnostics response: %v", err)
	}
	if response.Scope != "user" || len(response.Invocations) != 1 || response.Invocations[0].ToolName != "calculator" {
		t.Fatalf("unexpected diagnostics response: %#v", response)
	}
}
