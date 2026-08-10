package mcpclient

import (
	"context"
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

func TestHTTPClientInjectsOAuthBearerHeaderWithoutQueryLeak(t *testing.T) {
	const accessToken = "sensitive-access-token"
	resourceURL := "https://mcp.example.com/tools"
	config := models.MCPServer{ID: "oauth-header-test", Name: "oauth", Transport: "http", URL: &resourceURL, Headers: map[string]string{"Authorization": "Bearer stale-manual-token"}}
	client := NewHTTPClientWithTokenProvider(config, staticBearerTokenProvider{token: accessToken})
	req := httptest.NewRequest(http.MethodPost, resourceURL, nil)
	if err := client.applyAuthHeaders(context.Background(), req); err != nil {
		t.Fatalf("apply OAuth header: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer "+accessToken {
		t.Fatalf("Authorization = %q, want OAuth bearer token", got)
	}
	if strings.Contains(req.URL.RawQuery, accessToken) {
		t.Fatalf("access token leaked into query string: %q", req.URL.RawQuery)
	}
}

func TestHTTPClientRefusesOAuthBearerOverHTTP(t *testing.T) {
	resourceURL := "http://127.0.0.1:9999/mcp"
	config := models.MCPServer{ID: "oauth-http", Name: "oauth", Transport: "http", URL: &resourceURL, AllowPrivateNetwork: true}
	client := NewHTTPClientWithTokenProvider(config, staticBearerTokenProvider{token: "access"})
	req := httptest.NewRequest(http.MethodPost, resourceURL, nil)
	err := client.applyAuthHeaders(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "non-HTTPS") {
		t.Fatalf("expected non-HTTPS bearer rejection, got %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("bearer header was attached to plaintext request: %q", got)
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

type recordingBearerProvider struct {
	token  string
	scopes []string
}

func (p *recordingBearerProvider) AccessToken(_ context.Context, _ string) (string, error) {
	return p.token, nil
}
func (p *recordingBearerProvider) RecordScopeChallenge(_ string, scopes []string) error {
	p.scopes = append([]string{}, scopes...)
	return nil
}

func TestHTTPClientRecordsInsufficientScopeChallenge(t *testing.T) {
	provider := &recordingBearerProvider{token: "access"}
	resourceURL := "https://mcp.example.com"
	client := NewHTTPClientWithTokenProvider(models.MCPServer{ID: "scope-test", URL: &resourceURL}, provider)
	response := &http.Response{StatusCode: http.StatusForbidden, Header: make(http.Header)}
	response.Header.Set("WWW-Authenticate", `Bearer error="insufficient_scope", scope="files.write files.read"`)
	err := client.handleOAuthScopeChallenge(response)
	if err == nil || !strings.Contains(err.Error(), ErrMCPOAuthInsufficientScope.Error()) {
		t.Fatalf("expected insufficient-scope error, got %v", err)
	}
	if strings.Join(provider.scopes, " ") != "files.read files.write" {
		t.Fatalf("recorded scopes = %#v", provider.scopes)
	}
}
