// Package githubauth provides user-scoped GitHub App authentication primitives.
// It deliberately owns only authentication state; Git and GitHub mutation policy
// remains enforced by gitrepo and the tool approval/runtime gates.
package githubauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	ClientIDEnv                  = "OMNILLM_GITHUB_APP_CLIENT_ID"
	deviceCodeEndpoint           = "https://github.com/login/device/code"
	tokenEndpoint                = "https://github.com/login/oauth/access_token"
	userEndpoint                 = "https://api.github.com/user"
	githubAPIVersion             = "2022-11-28"
	maxGitHubAuthResponseBytes   = 1 << 20
	defaultDevicePollInterval    = 5 * time.Second
	deviceSlowDownIncrement      = 5 * time.Second
	accessTokenRefreshSkew       = 5 * time.Minute
	githubAuthHTTPTimeout        = 20 * time.Second
	maxGitHubDevicePollInterval  = 60 * time.Second
)

var (
	ErrNotConfigured           = errors.New("GitHub App authentication is not configured")
	ErrNotConnected            = errors.New("GitHub account is not connected")
	ErrReauthorizationRequired = errors.New("GitHub reauthorization is required")
)

// Credential is the backend-only decrypted runtime credential. Implementations
// of CredentialStore must encrypt AccessToken and RefreshToken at rest.
type Credential struct {
	AccessToken      string
	RefreshToken     string
	TokenType        string
	Scope            string
	AccessExpiresAt  *time.Time
	RefreshExpiresAt *time.Time
	GitHubUserID     int64
	GitHubLogin      string
}

// CredentialStore is the persistence boundary for user-scoped GitHub App
// credentials. Token-bearing values must never be returned by API handlers or
// tool results.
type CredentialStore interface {
	Get(userID string) (*Credential, error)
	Save(userID string, credential Credential) error
	Clear(userID string) error
}

// DeviceAuthorization is safe to return to a user. The device_code is retained
// only in backend memory and is never exposed through this result.
type DeviceAuthorization struct {
	UserCode         string    `json:"user_code"`
	VerificationURI  string    `json:"verification_uri"`
	ExpiresAt        time.Time `json:"expires_at"`
	IntervalSeconds  int       `json:"interval_seconds"`
}

// PollResult describes one bounded device-flow poll. Poll never loops or waits;
// callers may invoke it again after RetryAfterSeconds.
type PollResult struct {
	Status            string `json:"status"`
	RetryAfterSeconds int    `json:"retry_after_seconds,omitempty"`
	GitHubLogin       string `json:"github_login,omitempty"`
}

// Status is the non-secret connection view suitable for API responses.
type Status struct {
	Configured   bool       `json:"configured"`
	Connected    bool       `json:"connected"`
	Pending      bool       `json:"pending"`
	GitHubUserID int64      `json:"github_user_id,omitempty"`
	GitHubLogin  string     `json:"github_login,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
}

type pendingDeviceAuthorization struct {
	DeviceCode string
	UserCode   string
	ExpiresAt  time.Time
	Interval   time.Duration
	NextPollAt time.Time
}

// Service owns GitHub App device authorization, user identity verification, and
// expiring user-token refresh. It does not decide which repositories or Git/PR
// mutations are allowed.
type Service struct {
	clientID string
	store    CredentialStore
	client   *http.Client
	now      func() time.Time

	mu      sync.Mutex
	pending map[string]pendingDeviceAuthorization
}

// NewService creates a GitHub App auth service. Device flow requires only the
// GitHub App client ID; no client secret is needed for this flow.
func NewService(store CredentialStore, clientID string) (*Service, error) {
	clientID = strings.TrimSpace(clientID)
	if store == nil || clientID == "" {
		return nil, ErrNotConfigured
	}
	return &Service{
		clientID: clientID,
		store:    store,
		client: &http.Client{
			Timeout: githubAuthHTTPTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return fmt.Errorf("GitHub auth redirects are disabled")
			},
		},
		now:     func() time.Time { return time.Now().UTC() },
		pending: map[string]pendingDeviceAuthorization{},
	}, nil
}

// NewServiceFromEnvironment reads the operator-owned GitHub App client ID.
func NewServiceFromEnvironment(store CredentialStore) (*Service, error) {
	return NewService(store, os.Getenv(ClientIDEnv))
}

// StartDeviceAuthorization requests one device/user-code pair and replaces any
// previous pending authorization for this OmniLLM user.
func (s *Service) StartDeviceAuthorization(ctx context.Context, userID string) (DeviceAuthorization, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return DeviceAuthorization{}, fmt.Errorf("user ID is required")
	}
	form := url.Values{"client_id": {s.clientID}}
	var response deviceCodeResponse
	if err := s.postFormJSON(ctx, deviceCodeEndpoint, form, &response); err != nil {
		return DeviceAuthorization{}, fmt.Errorf("GitHub device authorization could not be started")
	}
	if response.Error != "" {
		return DeviceAuthorization{}, githubDeviceError(response.Error)
	}
	if strings.TrimSpace(response.DeviceCode) == "" || strings.TrimSpace(response.UserCode) == "" || response.VerificationURI != "https://github.com/login/device" || response.ExpiresIn <= 0 {
		return DeviceAuthorization{}, fmt.Errorf("GitHub returned an invalid device authorization response")
	}
	interval := time.Duration(response.Interval) * time.Second
	if interval <= 0 {
		interval = defaultDevicePollInterval
	}
	if interval > maxGitHubDevicePollInterval {
		return DeviceAuthorization{}, fmt.Errorf("GitHub returned an invalid device polling interval")
	}
	now := s.now()
	pending := pendingDeviceAuthorization{
		DeviceCode: response.DeviceCode,
		UserCode: response.UserCode,
		ExpiresAt: now.Add(time.Duration(response.ExpiresIn) * time.Second),
		Interval: interval,
		NextPollAt: now,
	}
	s.mu.Lock()
	s.pruneExpiredLocked(now)
	s.pending[userID] = pending
	s.mu.Unlock()
	return DeviceAuthorization{
		UserCode: response.UserCode,
		VerificationURI: response.VerificationURI,
		ExpiresAt: pending.ExpiresAt,
		IntervalSeconds: int(interval / time.Second),
	}, nil
}

// PollDeviceAuthorization performs at most one provider request and never sleeps.
func (s *Service) PollDeviceAuthorization(ctx context.Context, userID string) (PollResult, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return PollResult{}, fmt.Errorf("user ID is required")
	}
	now := s.now()
	s.mu.Lock()
	s.pruneExpiredLocked(now)
	pending, ok := s.pending[userID]
	if !ok {
		s.mu.Unlock()
		return PollResult{Status: "not_started"}, nil
	}
	if now.Before(pending.NextPollAt) {
		retry := secondsUntil(now, pending.NextPollAt)
		s.mu.Unlock()
		return PollResult{Status: "pending", RetryAfterSeconds: retry}, nil
	}
	pending.NextPollAt = now.Add(pending.Interval)
	s.pending[userID] = pending
	s.mu.Unlock()

	form := url.Values{
		"client_id":   {s.clientID},
		"device_code": {pending.DeviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}
	var response tokenResponse
	if err := s.postFormJSON(ctx, tokenEndpoint, form, &response); err != nil {
		return PollResult{}, fmt.Errorf("GitHub device authorization could not be checked")
	}
	if response.Error != "" {
		return s.handleDevicePollError(userID, response)
	}
	credential, err := s.credentialFromTokenResponse(ctx, response)
	if err != nil {
		return PollResult{}, err
	}
	if err := s.store.Save(userID, credential); err != nil {
		return PollResult{}, fmt.Errorf("save GitHub connection: %w", err)
	}
	s.mu.Lock()
	delete(s.pending, userID)
	s.mu.Unlock()
	return PollResult{Status: "connected", GitHubLogin: credential.GitHubLogin}, nil
}

// Status returns non-secret GitHub connection state for one OmniLLM user.
func (s *Service) Status(userID string) (Status, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return Status{}, fmt.Errorf("user ID is required")
	}
	credential, err := s.store.Get(userID)
	if err != nil {
		return Status{}, err
	}
	now := s.now()
	s.mu.Lock()
	s.pruneExpiredLocked(now)
	_, pending := s.pending[userID]
	s.mu.Unlock()
	status := Status{Configured: s.clientID != "", Pending: pending}
	if credential == nil {
		return status, nil
	}
	status.GitHubUserID = credential.GitHubUserID
	status.GitHubLogin = credential.GitHubLogin
	status.ExpiresAt = credential.AccessExpiresAt
	status.Connected = strings.TrimSpace(credential.AccessToken) != "" && (credential.AccessExpiresAt == nil || credential.AccessExpiresAt.After(now))
	return status, nil
}

// Disconnect removes local credentials and pending device-flow state. Remote
// GitHub authorization revocation remains an explicit future capability.
func (s *Service) Disconnect(userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("user ID is required")
	}
	if err := s.store.Clear(userID); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.pending, userID)
	s.mu.Unlock()
	return nil
}

// AccessToken returns a usable backend-only user token and refreshes an expiring
// token when necessary. It is intended for a later gitrepo credential resolver;
// it must never be surfaced as model or API output.
func (s *Service) AccessToken(ctx context.Context, userID string) (string, error) {
	credential, err := s.store.Get(strings.TrimSpace(userID))
	if err != nil {
		return "", err
	}
	if credential == nil || strings.TrimSpace(credential.AccessToken) == "" {
		return "", ErrNotConnected
	}
	now := s.now()
	if credential.AccessExpiresAt == nil || credential.AccessExpiresAt.After(now.Add(accessTokenRefreshSkew)) {
		return credential.AccessToken, nil
	}
	if strings.TrimSpace(credential.RefreshToken) == "" || (credential.RefreshExpiresAt != nil && !credential.RefreshExpiresAt.After(now)) {
		return "", ErrReauthorizationRequired
	}
	form := url.Values{
		"client_id":     {s.clientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {credential.RefreshToken},
	}
	var response tokenResponse
	if err := s.postFormJSON(ctx, tokenEndpoint, form, &response); err != nil {
		return "", fmt.Errorf("GitHub access token could not be refreshed")
	}
	if response.Error != "" {
		if response.Error == "bad_refresh_token" {
			return "", ErrReauthorizationRequired
		}
		return "", fmt.Errorf("GitHub access token refresh was rejected")
	}
	refreshed, err := s.credentialFromTokenResponse(ctx, response)
	if err != nil {
		return "", err
	}
	if err := s.store.Save(userID, refreshed); err != nil {
		return "", fmt.Errorf("save refreshed GitHub connection: %w", err)
	}
	return refreshed.AccessToken, nil
}

func (s *Service) handleDevicePollError(userID string, response tokenResponse) (PollResult, error) {
	now := s.now()
	switch response.Error {
	case "authorization_pending":
		s.mu.Lock()
		pending, ok := s.pending[userID]
		s.mu.Unlock()
		if !ok {
			return PollResult{Status: "not_started"}, nil
		}
		return PollResult{Status: "pending", RetryAfterSeconds: secondsUntil(now, pending.NextPollAt)}, nil
	case "slow_down":
		s.mu.Lock()
		pending, ok := s.pending[userID]
		if ok {
			interval := pending.Interval + deviceSlowDownIncrement
			if response.Interval > 0 {
				interval = time.Duration(response.Interval) * time.Second
			}
			if interval > maxGitHubDevicePollInterval {
				interval = maxGitHubDevicePollInterval
			}
			pending.Interval = interval
			pending.NextPollAt = now.Add(interval)
			s.pending[userID] = pending
		}
		s.mu.Unlock()
		if !ok {
			return PollResult{Status: "not_started"}, nil
		}
		return PollResult{Status: "pending", RetryAfterSeconds: int(pending.Interval / time.Second)}, nil
	case "expired_token", "token_expired", "bad_verification_code", "incorrect_device_code":
		s.clearPending(userID)
		return PollResult{Status: "expired"}, nil
	case "access_denied":
		s.clearPending(userID)
		return PollResult{Status: "denied"}, nil
	case "device_flow_disabled", "incorrect_client_credentials", "unsupported_grant_type":
		s.clearPending(userID)
		return PollResult{}, githubDeviceError(response.Error)
	default:
		return PollResult{}, fmt.Errorf("GitHub device authorization was rejected")
	}
}

func (s *Service) credentialFromTokenResponse(ctx context.Context, response tokenResponse) (Credential, error) {
	if strings.TrimSpace(response.AccessToken) == "" || !strings.EqualFold(strings.TrimSpace(response.TokenType), "bearer") {
		return Credential{}, fmt.Errorf("GitHub returned an invalid access token response")
	}
	identity, err := s.getUserIdentity(ctx, response.AccessToken)
	if err != nil {
		return Credential{}, err
	}
	now := s.now()
	credential := Credential{
		AccessToken: response.AccessToken,
		RefreshToken: response.RefreshToken,
		TokenType: "bearer",
		Scope: response.Scope,
		GitHubUserID: identity.ID,
		GitHubLogin: identity.Login,
	}
	if response.ExpiresIn > 0 {
		expires := now.Add(time.Duration(response.ExpiresIn) * time.Second)
		credential.AccessExpiresAt = &expires
	}
	if response.RefreshTokenExpiresIn > 0 {
		expires := now.Add(time.Duration(response.RefreshTokenExpiresIn) * time.Second)
		credential.RefreshExpiresAt = &expires
	}
	return credential, nil
}

func (s *Service) getUserIdentity(ctx context.Context, token string) (githubUserResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userEndpoint, nil)
	if err != nil {
		return githubUserResponse{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	var response githubUserResponse
	if err := s.doJSON(req, &response); err != nil {
		return githubUserResponse{}, fmt.Errorf("GitHub user identity could not be verified")
	}
	if response.ID <= 0 || strings.TrimSpace(response.Login) == "" {
		return githubUserResponse{}, fmt.Errorf("GitHub returned an invalid user identity")
	}
	return response, nil
}

func (s *Service) postFormJSON(ctx context.Context, endpoint string, form url.Values, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return s.doJSON(req, target)
}

func (s *Service) doJSON(req *http.Request, target interface{}) error {
	response, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("GitHub authentication request failed")
	}
	limited := io.LimitReader(response.Body, maxGitHubAuthResponseBytes+1)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if len(payload) > maxGitHubAuthResponseBytes {
		return fmt.Errorf("GitHub authentication response exceeded the configured limit")
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("GitHub returned an invalid authentication response")
	}
	return nil
}

func (s *Service) pruneExpiredLocked(now time.Time) {
	for userID, pending := range s.pending {
		if !pending.ExpiresAt.After(now) {
			delete(s.pending, userID)
		}
	}
}

func (s *Service) clearPending(userID string) {
	s.mu.Lock()
	delete(s.pending, userID)
	s.mu.Unlock()
}

func secondsUntil(now, target time.Time) int {
	if !target.After(now) {
		return 0
	}
	return int((target.Sub(now) + time.Second - 1) / time.Second)
}

func githubDeviceError(code string) error {
	switch code {
	case "device_flow_disabled":
		return fmt.Errorf("GitHub App device flow is disabled")
	case "incorrect_client_credentials":
		return fmt.Errorf("GitHub App client ID is invalid")
	case "unsupported_grant_type":
		return fmt.Errorf("GitHub rejected the configured device authorization grant")
	default:
		return fmt.Errorf("GitHub device authorization was rejected")
	}
}

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int64  `json:"expires_in"`
	Interval        int64  `json:"interval"`
	Error           string `json:"error"`
}

type tokenResponse struct {
	AccessToken           string `json:"access_token"`
	ExpiresIn             int64  `json:"expires_in"`
	RefreshToken          string `json:"refresh_token"`
	RefreshTokenExpiresIn int64  `json:"refresh_token_expires_in"`
	Scope                 string `json:"scope"`
	TokenType             string `json:"token_type"`
	Error                 string `json:"error"`
	Interval              int64  `json:"interval"`
}

type githubUserResponse struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

// String form is useful for operator diagnostics without ever formatting token
// material. It intentionally includes only non-secret configuration state.
func (s *Service) String() string {
	return "githubauth.Service{configured:" + strconv.FormatBool(s != nil && s.clientID != "") + "}"
}
