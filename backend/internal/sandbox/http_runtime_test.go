package sandbox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPRuntimeAuthenticatesAndExecutesProtocolV2(t *testing.T) {
	const token = "sandbox-service-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Fatalf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2/capabilities":
			_ = json.NewEncoder(w).Encode(RuntimeCapabilities{Name: "test-runtime", OSIsolation: true})
		case r.Method == http.MethodPost && r.URL.Path == "/v2/sandboxes":
			var request RuntimeCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if request.SessionID == "" || request.Owner.UserID != "user-1" {
				t.Fatalf("create request = %#v", request)
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"runtime_id": "worker-session"})
		case r.Method == http.MethodPost && r.URL.Path == "/v2/sandboxes/worker-session/exec":
			_ = json.NewEncoder(w).Encode(ExecResult{ExecutionID: "exec-1", Stdout: "ok", ExitCode: 0})
		case r.Method == http.MethodDelete && r.URL.Path == "/v2/sandboxes/worker-session":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()

	runtime, err := NewHTTPRuntime(context.Background(), server.URL, token)
	if err != nil {
		t.Fatalf("NewHTTPRuntime() error = %v", err)
	}
	if !runtime.Capabilities().OSIsolation {
		t.Fatalf("capabilities = %#v", runtime.Capabilities())
	}
	runtimeID, err := runtime.Create(context.Background(), RuntimeCreateRequest{
		SessionID: "sbx-test",
		Owner:     OwnerScope{UserID: "user-1"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	result, err := runtime.Exec(context.Background(), runtimeID, ExecRequest{Command: "echo"})
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	if result.ExecutionID != "exec-1" || result.Stdout != "ok" {
		t.Fatalf("Exec() result = %#v", result)
	}
	if err := runtime.Destroy(context.Background(), runtimeID); err != nil {
		t.Fatalf("Destroy() error = %v", err)
	}
}

func TestHTTPRuntimeRequiresAuthenticatedSecureEndpoint(t *testing.T) {
	if _, err := NewHTTPRuntime(context.Background(), "http://127.0.0.1:8090", ""); err == nil || !strings.Contains(err.Error(), "token") {
		t.Fatalf("missing token error = %v", err)
	}
	if _, err := NewHTTPRuntime(context.Background(), "http://example.com", "token"); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("insecure remote endpoint error = %v", err)
	}
	if _, err := NewHTTPRuntime(context.Background(), "file:///tmp/sandbox", "token"); err == nil {
		t.Fatal("expected non-http sandbox endpoint to be rejected")
	}
}
