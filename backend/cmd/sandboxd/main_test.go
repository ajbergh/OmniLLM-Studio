package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/sandbox"
)

type fakeWorkerRuntime struct {
	create sandbox.RuntimeCreateRequest
	exec   sandbox.ExecRequest
}

func (f *fakeWorkerRuntime) Capabilities() sandbox.RuntimeCapabilities {
	return sandbox.RuntimeCapabilities{Name: "test-runtime", OSIsolation: true}
}
func (f *fakeWorkerRuntime) Create(_ context.Context, request sandbox.RuntimeCreateRequest) (string, error) {
	f.create = request
	return "runtime-1", nil
}
func (f *fakeWorkerRuntime) Exec(_ context.Context, _ string, request sandbox.ExecRequest) (*sandbox.ExecResult, error) {
	f.exec = request
	return &sandbox.ExecResult{ExecutionID: "exec-1", Stdout: "ok"}, nil
}
func (f *fakeWorkerRuntime) Cancel(context.Context, string, string) error { return nil }
func (f *fakeWorkerRuntime) Status(context.Context, string) (*sandbox.Status, error) {
	return &sandbox.Status{State: "ready"}, nil
}
func (f *fakeWorkerRuntime) Destroy(context.Context, string) error { return nil }

func TestWorkerRequiresBearerAuthentication(t *testing.T) {
	runtime := &fakeWorkerRuntime{}
	handler := authenticated("secret-token", newHandler(runtime))

	request := httptest.NewRequest(http.MethodGet, "/v2/capabilities", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/v2/capabilities", nil)
	request.Header.Set("Authorization", "Bearer secret-token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestWorkerCreateAndExecLifecycle(t *testing.T) {
	runtime := &fakeWorkerRuntime{}
	handler := authenticated("secret-token", newHandler(runtime))

	createBody := `{"session_id":"sbx-1","owner":{"user_id":"user-1"},"spec":{"network":{"mode":"none"}}}`
	request := httptest.NewRequest(http.MethodPost, "/v2/sandboxes", strings.NewReader(createBody))
	request.Header.Set("Authorization", "Bearer secret-token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", response.Code, response.Body.String())
	}
	if runtime.create.SessionID != "sbx-1" || runtime.create.Owner.UserID != "user-1" {
		t.Fatalf("create request = %#v", runtime.create)
	}

	execBody := `{"command":"echo","args":["hello"]}`
	request = httptest.NewRequest(http.MethodPost, "/v2/sandboxes/runtime-1/exec", strings.NewReader(execBody))
	request.Header.Set("Authorization", "Bearer secret-token")
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("exec status = %d body=%s", response.Code, response.Body.String())
	}
	if runtime.exec.Command != "echo" || len(runtime.exec.Args) != 1 || runtime.exec.Args[0] != "hello" {
		t.Fatalf("exec request = %#v", runtime.exec)
	}
	var result sandbox.ExecResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.ExecutionID != "exec-1" || result.Stdout != "ok" {
		t.Fatalf("exec result = %#v", result)
	}
}

func TestWorkerRejectsUnknownFieldsAndMultipleJSONValues(t *testing.T) {
	runtime := &fakeWorkerRuntime{}
	handler := authenticated("secret-token", newHandler(runtime))
	for _, body := range []string{
		`{"session_id":"sbx-1","owner":{"user_id":"user-1"},"spec":{"network":{"mode":"none"}},"unexpected":true}`,
		`{"session_id":"sbx-1","owner":{"user_id":"user-1"},"spec":{"network":{"mode":"none"}}} {}`,
	} {
		request := httptest.NewRequest(http.MethodPost, "/v2/sandboxes", strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer secret-token")
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body %q status = %d, want 400", body, response.Code)
		}
	}
}
