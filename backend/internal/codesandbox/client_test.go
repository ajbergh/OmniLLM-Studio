package codesandbox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/sandbox"
)

func TestNewCreatesAuthenticatedProtocolV2Broker(t *testing.T) {
	const token = "worker-token"
	t.Setenv("OMNILLM_SANDBOX_TOKEN", token)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Fatalf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2/capabilities":
			_ = json.NewEncoder(w).Encode(sandbox.RuntimeCapabilities{
				Name:                 "test-runtime",
				OSIsolation:          true,
				FilesystemIsolation:  true,
				NetworkIsolation:     true,
				ProcessTreeIsolation: true,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v2/sandboxes":
			_ = json.NewEncoder(w).Encode(map[string]string{"runtime_id": "runtime-1"})
		default:
			http.Error(w, "unexpected endpoint", http.StatusNotFound)
		}
	}))
	defer server.Close()

	broker, err := New(server.URL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if broker.Capabilities().Name != "test-runtime" {
		t.Fatalf("capabilities = %#v", broker.Capabilities())
	}
	if _, err := broker.Create(context.Background(), sandbox.OwnerScope{UserID: "user-1"}, sandbox.CreateRequest{
		Network: sandbox.NetworkPolicy{Mode: sandbox.NetworkNone},
		Requirements: sandbox.RuntimeRequirements{
			OSIsolation:          true,
			FilesystemIsolation:  true,
			NetworkIsolation:     true,
			ProcessTreeIsolation: true,
		},
	}); err != nil {
		t.Fatalf("Broker.Create() error = %v", err)
	}
}

func TestNewRejectsMissingTokenAndInvalidURL(t *testing.T) {
	t.Setenv("OMNILLM_SANDBOX_TOKEN", "")
	if _, err := New("http://127.0.0.1:8090"); err == nil || !strings.Contains(err.Error(), "TOKEN") {
		t.Fatalf("missing-token error = %v", err)
	}
	t.Setenv("OMNILLM_SANDBOX_TOKEN", "token")
	if _, err := New("file:///tmp/sandbox"); err == nil {
		t.Fatal("expected invalid sandbox URL to be rejected")
	}
}
