package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/sandbox"
	"github.com/go-chi/chi/v5"
)

type sandboxStatusRuntime struct{}

func (sandboxStatusRuntime) Capabilities() sandbox.RuntimeCapabilities {
	return sandbox.RuntimeCapabilities{
		Name:                 "settings-test",
		OSIsolation:          true,
		FilesystemIsolation:  true,
		NetworkIsolation:     true,
		ProcessTreeIsolation: true,
	}
}
func (sandboxStatusRuntime) Create(context.Context, sandbox.RuntimeCreateRequest) (string, error) {
	return "runtime-1", nil
}
func (sandboxStatusRuntime) Exec(context.Context, string, sandbox.ExecRequest) (*sandbox.ExecResult, error) {
	return &sandbox.ExecResult{ExecutionID: "exec-1"}, nil
}
func (sandboxStatusRuntime) Cancel(context.Context, string, string) error { return nil }
func (sandboxStatusRuntime) Status(context.Context, string) (*sandbox.Status, error) {
	return &sandbox.Status{State: "ready"}, nil
}
func (sandboxStatusRuntime) Destroy(context.Context, string) error { return nil }

func setupSandboxHandlerTest(t *testing.T) *SandboxHandler {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "sandbox-settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close(database) })
	handler, err := NewSandboxHandler(database)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sandbox.SetDefaultWorkspaceRegistry(nil)
		sandbox.SetDefaultBroker(nil)
	})
	return handler
}

func TestSandboxStatusReportsCapabilitiesWithoutSecretsOrPaths(t *testing.T) {
	handler := setupSandboxHandlerTest(t)
	broker, err := sandbox.NewBroker(sandboxStatusRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	sandbox.SetDefaultBroker(broker)
	t.Setenv("OMNILLM_SANDBOX_ROOTFS", "/must/not/appear")
	t.Setenv("OMNILLM_SANDBOX_TOKEN", "must-not-appear")
	t.Setenv("OMNILLM_EXTENSION_SANDBOX_MODE", "required")
	t.Setenv("OMNILLM_SANDBOX_ALLOW_PATH_GRANTS", "true")

	req := httptest.NewRequest(http.MethodGet, "/v1/sandbox/status", nil)
	req.RemoteAddr = "127.0.0.1:4242"
	recorder := httptest.NewRecorder()
	handler.Status(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d; body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, forbidden := range []string{"/must/not/appear", "must-not-appear"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("status response exposed sensitive value %q: %s", forbidden, body)
		}
	}
	var response sandboxStatusResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Configured || response.Capabilities.Name != "settings-test" || response.ExtensionSandboxMode != "required" {
		t.Fatalf("status response = %#v", response)
	}
	if !response.PathGrantsConfigured || !response.PathGrantAvailableHere {
		t.Fatalf("path grant status = %#v", response)
	}
}

func TestSandboxWorkspaceGrantIsLoopbackOnlyAndNeverReturnsRoot(t *testing.T) {
	handler := setupSandboxHandlerTest(t)
	t.Setenv("OMNILLM_SANDBOX_ALLOW_PATH_GRANTS", "true")
	root := t.TempDir()
	payload, _ := json.Marshal(createSandboxWorkspaceRequest{
		ID:       "project",
		RootPath: root,
		Mode:     sandbox.MountReadWrite,
	})

	remoteRequest := httptest.NewRequest(http.MethodPost, "/v1/sandbox/workspaces", strings.NewReader(string(payload)))
	remoteRequest.RemoteAddr = "203.0.113.10:443"
	remoteRecorder := httptest.NewRecorder()
	handler.CreateWorkspace(remoteRecorder, remoteRequest)
	if remoteRecorder.Code != http.StatusForbidden {
		t.Fatalf("remote grant code = %d; body=%s", remoteRecorder.Code, remoteRecorder.Body.String())
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/sandbox/workspaces", strings.NewReader(string(payload)))
	request.RemoteAddr = "127.0.0.1:4242"
	recorder := httptest.NewRecorder()
	handler.CreateWorkspace(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("grant code = %d; body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), root) {
		t.Fatalf("grant response exposed root path: %s", recorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	handler.ListWorkspaces(listRecorder, httptest.NewRequest(http.MethodGet, "/v1/sandbox/workspaces", nil))
	if listRecorder.Code != http.StatusOK || strings.Contains(listRecorder.Body.String(), root) {
		t.Fatalf("workspace list code=%d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
}

func TestSandboxWorkspaceGrantRequiresOperatorEnablement(t *testing.T) {
	handler := setupSandboxHandlerTest(t)
	t.Setenv("OMNILLM_SANDBOX_ALLOW_PATH_GRANTS", "false")
	request := httptest.NewRequest(http.MethodPost, "/v1/sandbox/workspaces", strings.NewReader(`{"id":"project","root_path":"/tmp","mode":"read_only"}`))
	request.RemoteAddr = "127.0.0.1:4242"
	recorder := httptest.NewRecorder()
	handler.CreateWorkspace(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("grant code = %d; body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSandboxPathGrantLoopbackRejectsForwardedAddressHeaders(t *testing.T) {
	direct := httptest.NewRequest(http.MethodGet, "/", nil)
	direct.RemoteAddr = "127.0.0.1:4242"
	if !requestIsLoopback(direct) {
		t.Fatal("direct loopback request should be accepted")
	}
	for _, header := range []string{
		"Forwarded",
		"X-Forwarded-For",
		"X-Real-IP",
		"True-Client-IP",
		"CF-Connecting-IP",
	} {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = "127.0.0.1:4242"
		request.Header.Set(header, "127.0.0.1")
		if requestIsLoopback(request) {
			t.Fatalf("forwarded header %s must make the loopback grant check fail closed", header)
		}
	}
}

func TestSandboxWorkspaceChangesRequireOwnedWorkspace(t *testing.T) {
	handler := setupSandboxHandlerTest(t)
	request := withSandboxWorkspaceRouteParam(
		httptest.NewRequest(http.MethodGet, "/v1/sandbox/workspaces/missing/changes", nil),
		"missing",
	)
	recorder := httptest.NewRecorder()
	handler.ListWorkspaceChanges(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("changes code = %d; body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSafeSandboxWorkspaceChangeOmitsInternalScopeIdentifiers(t *testing.T) {
	response := safeSandboxWorkspaceChange(sandbox.WorkspaceChange{
		ID:             "wch_test",
		WorkspaceID:    "project",
		UserID:         "must-not-appear-user",
		ConversationID: "must-not-appear-conversation",
		AgentRunID:     "must-not-appear-run",
		TaskID:         "must-not-appear-task",
		SandboxID:      "must-not-appear-sandbox",
		ExecutionID:    "must-not-appear-execution",
		RelativePath:   "file.txt",
		Operation:      "write",
		BeforeExists:   true,
		BeforeSHA256:   "before",
		AfterExists:    true,
		AfterSHA256:    "after",
		Revertable:     true,
	})
	body, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(body)
	for _, forbidden := range []string{
		"must-not-appear-user",
		"must-not-appear-conversation",
		"must-not-appear-run",
		"must-not-appear-task",
		"must-not-appear-sandbox",
		"must-not-appear-execution",
	} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("safe change response exposed internal scope value %q: %s", forbidden, encoded)
		}
	}
	for _, required := range []string{"wch_test", "project", "file.txt", "before", "after"} {
		if !strings.Contains(encoded, required) {
			t.Fatalf("safe change response omitted required review value %q: %s", required, encoded)
		}
	}
}

func withSandboxWorkspaceRouteParam(request *http.Request, id string) *http.Request {
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("workspaceId", id)
	ctx := context.WithValue(request.Context(), chi.RouteCtxKey, routeContext)
	return request.WithContext(ctx)
}
