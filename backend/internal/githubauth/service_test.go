package githubauth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type memoryCredentialStore struct {
	mu    sync.Mutex
	items map[string]Credential
}

func newMemoryCredentialStore() *memoryCredentialStore {
	return &memoryCredentialStore{items: map[string]Credential{}}
}

func (s *memoryCredentialStore) Get(userID string) (*Credential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	credential, ok := s.items[userID]
	if !ok {
		return nil, nil
	}
	copy := credential
	return &copy, nil
}

func (s *memoryCredentialStore) Save(userID string, credential Credential) error {
	s.mu.Lock()
	s.items[userID] = credential
	s.mu.Unlock()
	return nil
}

func (s *memoryCredentialStore) Clear(userID string) error {
	s.mu.Lock()
	delete(s.items, userID)
	s.mu.Unlock()
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func jsonResponse(status int, body interface{}) *http.Response {
	encoded, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(string(encoded))),
	}
}

func newTestService(t *testing.T, store CredentialStore, transport roundTripFunc) *Service {
	t.Helper()
	service, err := NewService(store, "Iv1.test-client")
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	service.client = &http.Client{Transport: transport}
	return service
}

func TestStartDeviceAuthorizationKeepsDeviceCodeBackendOnly(t *testing.T) {
	store := newMemoryCredentialStore()
	service := newTestService(t, store, func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != deviceCodeEndpoint || request.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL)
		}
		if request.Header.Get("Accept") != "application/json" {
			t.Fatalf("missing JSON accept header")
		}
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), "client_id=Iv1.test-client") {
			t.Fatalf("missing client ID form value: %s", body)
		}
		return jsonResponse(http.StatusOK, map[string]interface{}{
			"device_code": "provider-secret-device-code",
			"user_code": "ABCD-EFGH",
			"verification_uri": "https://github.com/login/device",
			"expires_in": 900,
			"interval": 5,
		}), nil
	})
	service.now = func() time.Time { return time.Date(2026, 8, 11, 20, 0, 0, 0, time.UTC) }

	result, err := service.StartDeviceAuthorization(context.Background(), "local")
	if err != nil {
		t.Fatalf("StartDeviceAuthorization() error = %v", err)
	}
	if result.UserCode != "ABCD-EFGH" || result.VerificationURI != "https://github.com/login/device" || result.IntervalSeconds != 5 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if strings.Contains(result.UserCode+result.VerificationURI, "provider-secret-device-code") {
		t.Fatalf("device code escaped backend state")
	}
	status, err := service.Status("local")
	if err != nil || !status.Configured || !status.Pending || status.Connected {
		t.Fatalf("unexpected pending status: %#v err=%v", status, err)
	}
}

func TestPollDeviceAuthorizationBindsGitHubIdentityAndStoresCredential(t *testing.T) {
	store := newMemoryCredentialStore()
	calls := 0
	service := newTestService(t, store, func(request *http.Request) (*http.Response, error) {
		calls++
		switch request.URL.String() {
		case deviceCodeEndpoint:
			return jsonResponse(http.StatusOK, map[string]interface{}{
				"device_code": "device-code",
				"user_code": "ABCD-EFGH",
				"verification_uri": "https://github.com/login/device",
				"expires_in": 900,
				"interval": 5,
			}), nil
		case tokenEndpoint:
			body, _ := io.ReadAll(request.Body)
			text := string(body)
			if !strings.Contains(text, "device_code=device-code") || !strings.Contains(text, "grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Adevice_code") {
				t.Fatalf("unexpected token form: %s", text)
			}
			return jsonResponse(http.StatusOK, map[string]interface{}{
				"access_token": "ghu_access_secret",
				"expires_in": 28800,
				"refresh_token": "ghr_refresh_secret",
				"refresh_token_expires_in": 15897600,
				"token_type": "bearer",
				"scope": "",
			}), nil
		case userEndpoint:
			if request.Header.Get("Authorization") != "Bearer ghu_access_secret" {
				t.Fatalf("unexpected authorization header")
			}
			if request.Header.Get("X-GitHub-Api-Version") != githubAPIVersion {
				t.Fatalf("missing pinned API version")
			}
			return jsonResponse(http.StatusOK, map[string]interface{}{"id": 12345, "login": "octocat"}), nil
		default:
			t.Fatalf("unexpected request URL %s", request.URL)
			return nil, nil
		}
	})
	fixed := time.Date(2026, 8, 11, 20, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixed }
	if _, err := service.StartDeviceAuthorization(context.Background(), "user-1"); err != nil {
		t.Fatalf("start error = %v", err)
	}

	result, err := service.PollDeviceAuthorization(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("poll error = %v", err)
	}
	if result.Status != "connected" || result.GitHubLogin != "octocat" {
		t.Fatalf("unexpected poll result: %#v", result)
	}
	credential, _ := store.Get("user-1")
	if credential == nil || credential.AccessToken != "ghu_access_secret" || credential.RefreshToken != "ghr_refresh_secret" || credential.GitHubUserID != 12345 || credential.GitHubLogin != "octocat" {
		t.Fatalf("unexpected stored credential: %#v", credential)
	}
	status, err := service.Status("user-1")
	if err != nil || !status.Connected || status.Pending || status.GitHubLogin != "octocat" || status.GitHubUserID != 12345 {
		t.Fatalf("unexpected connected status: %#v err=%v", status, err)
	}
	if calls != 3 {
		t.Fatalf("expected three provider requests, got %d", calls)
	}
}

func TestPollDeviceAuthorizationEnforcesProviderIntervalWithoutSleeping(t *testing.T) {
	store := newMemoryCredentialStore()
	providerPolls := 0
	service := newTestService(t, store, func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == deviceCodeEndpoint {
			return jsonResponse(http.StatusOK, map[string]interface{}{
				"device_code": "device-code", "user_code": "ABCD-EFGH",
				"verification_uri": "https://github.com/login/device", "expires_in": 900, "interval": 5,
			}), nil
		}
		if request.URL.String() == tokenEndpoint {
			providerPolls++
			return jsonResponse(http.StatusOK, map[string]interface{}{"error": "authorization_pending"}), nil
		}
		t.Fatalf("unexpected request URL %s", request.URL)
		return nil, nil
	})
	fixed := time.Date(2026, 8, 11, 20, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixed }
	if _, err := service.StartDeviceAuthorization(context.Background(), "local"); err != nil {
		t.Fatal(err)
	}
	first, err := service.PollDeviceAuthorization(context.Background(), "local")
	if err != nil || first.Status != "pending" || first.RetryAfterSeconds != 5 {
		t.Fatalf("unexpected first poll: %#v err=%v", first, err)
	}
	second, err := service.PollDeviceAuthorization(context.Background(), "local")
	if err != nil || second.Status != "pending" || second.RetryAfterSeconds != 5 {
		t.Fatalf("unexpected second poll: %#v err=%v", second, err)
	}
	if providerPolls != 1 {
		t.Fatalf("second immediate poll should stay local; provider polls=%d", providerPolls)
	}
}

func TestAccessTokenRefreshesAndRotatesDeviceFlowCredential(t *testing.T) {
	store := newMemoryCredentialStore()
	fixed := time.Date(2026, 8, 11, 20, 0, 0, 0, time.UTC)
	oldAccessExpiry := fixed.Add(time.Minute)
	oldRefreshExpiry := fixed.Add(30 * 24 * time.Hour)
	_ = store.Save("user-1", Credential{
		AccessToken: "ghu_old", RefreshToken: "ghr_old", TokenType: "bearer",
		AccessExpiresAt: &oldAccessExpiry, RefreshExpiresAt: &oldRefreshExpiry,
		GitHubUserID: 12345, GitHubLogin: "octocat",
	})
	service := newTestService(t, store, func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case tokenEndpoint:
			body, _ := io.ReadAll(request.Body)
			text := string(body)
			if !strings.Contains(text, "grant_type=refresh_token") || !strings.Contains(text, "refresh_token=ghr_old") || strings.Contains(text, "client_secret") {
				t.Fatalf("unexpected refresh form: %s", text)
			}
			return jsonResponse(http.StatusOK, map[string]interface{}{
				"access_token": "ghu_new", "expires_in": 28800,
				"refresh_token": "ghr_new", "refresh_token_expires_in": 15897600,
				"token_type": "bearer", "scope": "",
			}), nil
		case userEndpoint:
			return jsonResponse(http.StatusOK, map[string]interface{}{"id": 12345, "login": "octocat"}), nil
		default:
			t.Fatalf("unexpected URL %s", request.URL)
			return nil, nil
		}
	})
	service.now = func() time.Time { return fixed }

	token, err := service.AccessToken(context.Background(), "user-1")
	if err != nil || token != "ghu_new" {
		t.Fatalf("AccessToken() = %q, %v", token, err)
	}
	credential, _ := store.Get("user-1")
	if credential == nil || credential.AccessToken != "ghu_new" || credential.RefreshToken != "ghr_new" || credential.AccessExpiresAt == nil || !credential.AccessExpiresAt.After(fixed) {
		t.Fatalf("refresh did not rotate credential: %#v", credential)
	}
}

func TestDisconnectClearsCredentialAndPendingState(t *testing.T) {
	store := newMemoryCredentialStore()
	_ = store.Save("user-1", Credential{AccessToken: "secret", TokenType: "bearer"})
	service := newTestService(t, store, func(request *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, map[string]interface{}{
			"device_code": "device-code", "user_code": "ABCD-EFGH",
			"verification_uri": "https://github.com/login/device", "expires_in": 900, "interval": 5,
		}), nil
	})
	if _, err := service.StartDeviceAuthorization(context.Background(), "user-1"); err != nil {
		t.Fatal(err)
	}
	if err := service.Disconnect("user-1"); err != nil {
		t.Fatal(err)
	}
	status, err := service.Status("user-1")
	if err != nil || status.Connected || status.Pending {
		t.Fatalf("unexpected status after disconnect: %#v err=%v", status, err)
	}
}
