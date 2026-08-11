package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/githubauth"
	"github.com/ajbergh/omnillm-studio/internal/models"
)

type fakeGitHubAuthService struct {
	statusByUser map[string]githubauth.Status
	startResult  githubauth.DeviceAuthorization
	pollResult   githubauth.PollResult
	err          error
	seenUsers    []string
	disconnected []string
}

func (f *fakeGitHubAuthService) StartDeviceAuthorization(_ context.Context, userID string) (githubauth.DeviceAuthorization, error) {
	f.seenUsers = append(f.seenUsers, userID)
	return f.startResult, f.err
}

func (f *fakeGitHubAuthService) PollDeviceAuthorization(_ context.Context, userID string) (githubauth.PollResult, error) {
	f.seenUsers = append(f.seenUsers, userID)
	return f.pollResult, f.err
}

func (f *fakeGitHubAuthService) Status(userID string) (githubauth.Status, error) {
	f.seenUsers = append(f.seenUsers, userID)
	if f.err != nil {
		return githubauth.Status{}, f.err
	}
	return f.statusByUser[userID], nil
}

func (f *fakeGitHubAuthService) Disconnect(userID string) error {
	f.seenUsers = append(f.seenUsers, userID)
	f.disconnected = append(f.disconnected, userID)
	return f.err
}

func withGitHubAuthUser(request *http.Request, userID string) *http.Request {
	ctx := context.WithValue(request.Context(), auth.ContextKeyUser, &models.User{ID: userID})
	return request.WithContext(ctx)
}

func TestGitHubAuthHandlerUnconfiguredStatusAndDisconnect(t *testing.T) {
	handler := NewGitHubAuthHandler(nil)
	statusRecorder := httptest.NewRecorder()
	handler.Status(statusRecorder, httptest.NewRequest(http.MethodGet, "/v1/github/auth", nil))
	if statusRecorder.Code != http.StatusOK || statusRecorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status response = %d headers=%v", statusRecorder.Code, statusRecorder.Header())
	}
	var status githubauth.Status
	if err := json.NewDecoder(statusRecorder.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.Configured || status.Connected || status.Pending {
		t.Fatalf("unexpected unconfigured status: %#v", status)
	}

	disconnectRecorder := httptest.NewRecorder()
	handler.Disconnect(disconnectRecorder, httptest.NewRequest(http.MethodDelete, "/v1/github/auth", nil))
	if disconnectRecorder.Code != http.StatusNoContent {
		t.Fatalf("disconnect response = %d", disconnectRecorder.Code)
	}
}

func TestGitHubAuthHandlerUsesAuthenticatedOwnerAndNeverReturnsSecrets(t *testing.T) {
	expiresAt := time.Date(2026, 8, 12, 2, 0, 0, 0, time.UTC)
	service := &fakeGitHubAuthService{
		statusByUser: map[string]githubauth.Status{
			"user-7": {Configured: true, Connected: true, GitHubUserID: 12345, GitHubLogin: "octocat", ExpiresAt: &expiresAt},
		},
		startResult: githubauth.DeviceAuthorization{
			UserCode: "ABCD-EFGH", VerificationURI: "https://github.com/login/device",
			ExpiresAt: expiresAt, IntervalSeconds: 5,
		},
		pollResult: githubauth.PollResult{Status: "connected", GitHubLogin: "octocat"},
	}
	handler := NewGitHubAuthHandler(service)

	statusRecorder := httptest.NewRecorder()
	handler.Status(statusRecorder, withGitHubAuthUser(httptest.NewRequest(http.MethodGet, "/v1/github/auth", nil), "user-7"))
	body := statusRecorder.Body.String()
	if statusRecorder.Code != http.StatusOK || !strings.Contains(body, `"github_login":"octocat"`) {
		t.Fatalf("unexpected status: code=%d body=%s", statusRecorder.Code, body)
	}
	for _, forbidden := range []string{"access_token", "refresh_token", "device_code", "ghu_", "ghr_"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("secret-bearing field %q escaped response: %s", forbidden, body)
		}
	}

	startRecorder := httptest.NewRecorder()
	handler.StartDeviceAuthorization(startRecorder, withGitHubAuthUser(httptest.NewRequest(http.MethodPost, "/v1/github/auth/device/start", nil), "user-7"))
	if startRecorder.Code != http.StatusOK || strings.Contains(startRecorder.Body.String(), "device_code") || !strings.Contains(startRecorder.Body.String(), "ABCD-EFGH") {
		t.Fatalf("unexpected start response: %d %s", startRecorder.Code, startRecorder.Body.String())
	}

	pollRecorder := httptest.NewRecorder()
	handler.PollDeviceAuthorization(pollRecorder, withGitHubAuthUser(httptest.NewRequest(http.MethodPost, "/v1/github/auth/device/poll", nil), "user-7"))
	if pollRecorder.Code != http.StatusOK || !strings.Contains(pollRecorder.Body.String(), `"status":"connected"`) {
		t.Fatalf("unexpected poll response: %d %s", pollRecorder.Code, pollRecorder.Body.String())
	}

	disconnectRecorder := httptest.NewRecorder()
	handler.Disconnect(disconnectRecorder, withGitHubAuthUser(httptest.NewRequest(http.MethodDelete, "/v1/github/auth", nil), "user-7"))
	if disconnectRecorder.Code != http.StatusNoContent || len(service.disconnected) != 1 || service.disconnected[0] != "user-7" {
		t.Fatalf("unexpected disconnect: code=%d users=%v", disconnectRecorder.Code, service.disconnected)
	}
	for _, seen := range service.seenUsers {
		if seen != "user-7" {
			t.Fatalf("handler escaped authenticated owner scope: %v", service.seenUsers)
		}
	}
}

func TestGitHubAuthHandlerUsesStableSoloOwner(t *testing.T) {
	service := &fakeGitHubAuthService{statusByUser: map[string]githubauth.Status{auth.LocalScopeUserID: {Configured: true}}}
	handler := NewGitHubAuthHandler(service)
	recorder := httptest.NewRecorder()
	handler.Status(recorder, httptest.NewRequest(http.MethodGet, "/v1/github/auth", nil))
	if recorder.Code != http.StatusOK || len(service.seenUsers) != 1 || service.seenUsers[0] != auth.LocalScopeUserID {
		t.Fatalf("solo owner was not used: code=%d users=%v", recorder.Code, service.seenUsers)
	}
}

func TestGitHubAuthHandlerSanitizesProviderErrors(t *testing.T) {
	service := &fakeGitHubAuthService{err: errors.New("provider leaked ghu_secret_token")}
	handler := NewGitHubAuthHandler(service)
	recorder := httptest.NewRecorder()
	handler.StartDeviceAuthorization(recorder, httptest.NewRequest(http.MethodPost, "/v1/github/auth/device/start", nil))
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("unexpected error status: %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "ghu_secret_token") || !strings.Contains(recorder.Body.String(), "GitHub authentication request failed") {
		t.Fatalf("provider error leaked: %s", recorder.Body.String())
	}
}
