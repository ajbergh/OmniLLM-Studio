package sandbox

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestHTTPRuntimeHTTPSWorkerWithPinnedCA(t *testing.T) {
	const token = "sandbox-test-token"
	worker := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet || r.URL.Path != "/v2/capabilities" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(RuntimeCapabilities{Name: "private-worker"})
	}))
	defer worker.Close()
	t.Setenv("OMNILLM_SANDBOX_CA_FILE", "")
	if _, err := NewHTTPRuntime(context.Background(), worker.URL, token); err == nil {
		t.Fatal("self-signed remote worker unexpectedly trusted without configured CA")
	}
	caFile := filepath.Join(t.TempDir(), "worker-ca.pem")
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: worker.Certificate().Raw})
	if err := os.WriteFile(caFile, certificate, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OMNILLM_SANDBOX_CA_FILE", caFile)
	runtime, err := NewHTTPRuntime(context.Background(), worker.URL, token)
	if err != nil {
		t.Fatalf("authenticated HTTPS worker with pinned CA: %v", err)
	}
	if runtime.Capabilities().Name != "private-worker" {
		t.Fatalf("wrong authenticated worker: %#v", runtime.Capabilities())
	}
	if _, err := NewHTTPRuntime(context.Background(), worker.URL, "wrong-token"); err == nil {
		t.Fatal("incorrect worker bearer token was accepted")
	}
	if _, err := NewHTTPRuntime(context.Background(), "http://127.0.0.1:8090", token); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("CA-configured worker unexpectedly permitted plaintext: %v", err)
	}
	t.Setenv("OMNILLM_SANDBOX_CA_FILE", filepath.Join(t.TempDir(), "missing.pem"))
	if _, err := NewHTTPRuntime(context.Background(), worker.URL, token); err == nil {
		t.Fatal("missing CA must not fall back to ambient trust")
	}
	invalidFile := filepath.Join(t.TempDir(), "invalid-ca.pem")
	if err := os.WriteFile(invalidFile, []byte("not PEM"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OMNILLM_SANDBOX_CA_FILE", invalidFile)
	if _, err := NewHTTPRuntime(context.Background(), worker.URL, token); err == nil {
		t.Fatal("invalid CA must fail closed")
	}
}
