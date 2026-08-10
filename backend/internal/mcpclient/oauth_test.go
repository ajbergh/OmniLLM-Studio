package mcpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAuthorizationMetadataCandidatesMatchMCPDiscoveryOrder(t *testing.T) {
	candidates, err := authorizationMetadataCandidates("https://auth.example.com/tenant")
	if err != nil {
		t.Fatalf("authorization metadata candidates: %v", err)
	}
	want := []string{
		"https://auth.example.com/.well-known/oauth-authorization-server/tenant",
		"https://auth.example.com/.well-known/openid-configuration/tenant",
		"https://auth.example.com/tenant/.well-known/openid-configuration",
	}
	if len(candidates) != len(want) {
		t.Fatalf("candidates = %#v, want %#v", candidates, want)
	}
	for index := range want {
		if candidates[index] != want[index] {
			t.Fatalf("candidate %d = %q, want %q", index, candidates[index], want[index])
		}
	}
}

func TestProtectedResourceMetadataCandidatesRespectPath(t *testing.T) {
	candidates, err := protectedResourceMetadataCandidates("https://mcp.example.com/team/mcp")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 || candidates[0] != "https://mcp.example.com/.well-known/oauth-protected-resource/team/mcp" || candidates[1] != "https://mcp.example.com/.well-known/oauth-protected-resource" {
		t.Fatalf("unexpected protected-resource candidates: %#v", candidates)
	}
}

func TestParseBearerChallenge(t *testing.T) {
	metadata, scopes := parseBearerChallenge(`Bearer resource_metadata="https://mcp.example.com/.well-known/oauth-protected-resource", scope="tools.read tools.write"`)
	if metadata != "https://mcp.example.com/.well-known/oauth-protected-resource" {
		t.Fatalf("resource metadata = %q", metadata)
	}
	if strings.Join(scopes, " ") != "tools.read tools.write" {
		t.Fatalf("scopes = %#v", scopes)
	}
}

func TestCanonicalResourceURIAndAuthorizationQuery(t *testing.T) {
	resource, err := canonicalResourceURI("HTTPS://MCP.EXAMPLE.COM/tools#fragment")
	if err != nil {
		t.Fatal(err)
	}
	if resource != "https://mcp.example.com/tools" {
		t.Fatalf("resource URI = %q", resource)
	}
	verifier := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~"
	challenge := codeChallenge(verifier)
	if challenge == "" || strings.Contains(challenge, "=") {
		t.Fatalf("invalid S256 challenge %q", challenge)
	}
	values := url.Values{}
	values.Set("resource", resource)
	values.Set("code_challenge", challenge)
	values.Set("code_challenge_method", "S256")
	if values.Get("resource") != resource || values.Get("code_challenge_method") != "S256" {
		t.Fatalf("unexpected query values: %v", values)
	}
}

func TestCanonicalResourceURIRequiresHTTPS(t *testing.T) {
	if _, err := canonicalResourceURI("http://mcp.example.com/tools"); err == nil {
		t.Fatal("plaintext OAuth resource URI was accepted")
	}
	if _, err := canonicalResourceURI("https://user:secret@mcp.example.com/tools"); err == nil {
		t.Fatal("credential-bearing OAuth resource URI was accepted")
	}
}

func TestValidateOAuthEndpointRequiresHTTPS(t *testing.T) {
	if err := validateOAuthEndpoint("https://auth.example.com/token"); err != nil {
		t.Fatalf("HTTPS endpoint rejected: %v", err)
	}
	for _, raw := range []string{"http://auth.example.com/token", "https://user:secret@auth.example.com/token", "https://auth.example.com/token#fragment"} {
		if err := validateOAuthEndpoint(raw); err == nil {
			t.Fatalf("expected endpoint %q to be rejected", raw)
		}
	}
}

func TestValidateCIMDClientID(t *testing.T) {
	if err := validateCIMDClientID("https://client.example/oauth/metadata.json"); err != nil {
		t.Fatalf("valid CIMD client ID rejected: %v", err)
	}
	for _, raw := range []string{"http://client.example/metadata.json", "https://client.example", "https://client.example/metadata.json?x=1", "https://client.example/metadata.json#fragment"} {
		if err := validateCIMDClientID(raw); err == nil {
			t.Fatalf("invalid CIMD client ID accepted: %q", raw)
		}
	}
}

func TestAuthorizationResponseIssuerValidation(t *testing.T) {
	pending := oauthPendingState{ExpectedIssuer: "https://auth.example/tenant", IssuerParameterRequired: true}
	if err := validateAuthorizationResponseIssuer(pending, "https://auth.example/tenant"); err != nil {
		t.Fatalf("matching issuer rejected: %v", err)
	}
	if err := validateAuthorizationResponseIssuer(pending, ""); err == nil {
		t.Fatal("required issuer omission accepted")
	}
	if err := validateAuthorizationResponseIssuer(pending, "https://auth.example/tenant/"); err == nil {
		t.Fatal("RFC9207 simple-string mismatch accepted")
	}
	pending.IssuerParameterRequired = false
	if err := validateAuthorizationResponseIssuer(pending, ""); err != nil {
		t.Fatalf("optional absent issuer should be accepted: %v", err)
	}
}

func TestDynamicPublicClientRegistrationContract(t *testing.T) {
	var received dynamicClientRegistrationRequest
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"client_id":"dynamic-123","token_endpoint_auth_method":"none"}`))
	}))
	defer server.Close()

	result, err := registerDynamicPublicClient(context.Background(), server.Client(), server.URL, "http://127.0.0.1:54321/v1/mcp/oauth/callback")
	if err != nil {
		t.Fatalf("register dynamic public client: %v", err)
	}
	if result.ClientID != "dynamic-123" || received.TokenEndpointAuthMethod != "none" || received.ApplicationType != "native" {
		t.Fatalf("unexpected DCR contract: result=%#v request=%#v", result, received)
	}
	if len(received.RedirectURIs) != 1 || received.RedirectURIs[0] != "http://127.0.0.1:54321/v1/mcp/oauth/callback" {
		t.Fatalf("redirect URIs = %#v", received.RedirectURIs)
	}
}

func TestOAuthApplicationTypeUsesWebForHTTPSCallback(t *testing.T) {
	if got := oauthApplicationType("https://chat.example/v1/mcp/oauth/callback"); got != "web" {
		t.Fatalf("application type = %q, want web", got)
	}
}
