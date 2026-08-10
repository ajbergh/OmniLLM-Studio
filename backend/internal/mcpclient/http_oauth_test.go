package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

type staticBearerTokenProvider struct {
	token string
	err   error
}

func (p staticBearerTokenProvider) AccessToken(_ context.Context, _ string) (string, error) {
	return p.token, p.err
}

func newOAuthHeaderTestServer(t *testing.T, inspect func(*http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if inspect != nil {
			inspect(r)
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		var request rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "bad JSON-RPC request", http.StatusBadRequest)
			return
		}
		if request.ID == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		response := rpcResponse{JSONRPC: "2.0", ID: json.RawMessage(fmt.Sprintf("%d", request.ID))}
		if request.Method == "initialize" {
			response.Result, _ = json.Marshal(map[string]interface{}{
				"protocolVersion": ProtocolVersion,
				"serverInfo":      map[string]interface{}{"name": "oauth-test", "version": "1.0"},
				"capabilities":    map[string]interface{}{},
			})
		} else {
			response.Result = json.RawMessage(`{}`)
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
}

func TestHTTPClientInjectsOAuthBearerHeaderWithoutQueryLeak(t *testing.T) {
	const accessToken = "sensitive-access-token"
	var receivedAuthorization string
	var receivedRawQuery string
	server := newOAuthHeaderTestServer(t, func(r *http.Request) {
		receivedAuthorization = r.Header.Get("Authorization")
		receivedRawQuery = r.URL.RawQuery
	})
	defer server.Close()

	url := server.URL
	config := models.MCPServer{
		ID:                  "oauth-header-test",
		Name:                "oauth",
		Transport:           "http",
		URL:                 &url,
		AllowPrivateNetwork: true,
		Headers:             map[string]string{"Authorization": "Bearer stale-manual-token"},
	}
	client := NewHTTPClientWithTokenProvider(config, staticBearerTokenProvider{token: accessToken})
	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("start OAuth HTTP client: %v", err)
	}
	if receivedAuthorization != "Bearer "+accessToken {
		t.Fatalf("Authorization = %q, want OAuth bearer token", receivedAuthorization)
	}
	if strings.Contains(receivedRawQuery, accessToken) {
		t.Fatalf("access token leaked into query string: %q", receivedRawQuery)
	}
}

func TestHTTPClientPropagatesOAuthRequirementBeforeNetwork(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("network request should not occur when OAuth provider rejects token access")
	}))
	defer server.Close()

	url := server.URL
	config := models.MCPServer{ID: "oauth-required", Name: "oauth", Transport: "http", URL: &url, AllowPrivateNetwork: true}
	client := NewHTTPClientWithTokenProvider(config, staticBearerTokenProvider{err: ErrMCPOAuthRequired})
	err := client.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), ErrMCPOAuthRequired.Error()) {
		t.Fatalf("expected OAuth requirement error, got %v", err)
	}
}
