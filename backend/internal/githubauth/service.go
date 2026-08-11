// Package githubauth provides user-scoped GitHub App authentication primitives.
// It owns authentication state only; Git and GitHub mutation authorization
// remains enforced by gitrepo, operator configuration, and tool approval gates.
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
	"strings"
	"sync"
	"time"
)

const (
	ClientIDEnv = "OMNILLM_GITHUB_APP_CLIENT_ID"

	deviceCodeEndpoint = "https://github.com/login/device/code"
	tokenEndpoint      = "https://github.com/login/oauth/access_token"
	userEndpoint       = "https://api.github.com/user"
	githubAPIVersion   = "2022-11-28"

	maxResponseBytes    int64 = 1 << 20
	defaultPollInterval       = 5 * time.Second
	slowDownIncrement         = 5 * time.Second
	maxPollInterval           = 60 * time.Second
	refreshSkew               = 5 * time.Minute
)

var (
	ErrNotConfigured           = errors.New("GitHub App authentication is not configured")
	ErrNotConnected            = errors.New("GitHub account is not connected")
	ErrReauthorizationRequired = errors.New("GitHub reauthorization is required")
)

// Credential is the backend-only decrypted runtime credential. CredentialStore
// implementations must encrypt AccessToken and RefreshToken at rest.
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
// model/tool results.
type CredentialStore interface {
	Get(userID string) (*Credential, error)
	Save(userID string, credential Credential) error
	Clear(userID string) error
}

// DeviceAuthorization is safe to return to a user. The provider device_code is
// retained only in backend memory.
type DeviceAuthorization struct {
	UserCode        string    `json:"user_code"`
	VerificationURI string    `json:"verification_uri"`
	ExpiresAt       time.Time `json:"expires_at"`
	IntervalSeconds int       `json:"interval_seconds"`
}

// PollResult is one bounded device-flow poll result. Polling never sleeps or
// loops; callers retry after RetryAfterSeconds.
type PollResult struct {
	Status            string `json:"status"`
	RetryAfterSeconds int    `json:"retry_after_seconds,omitempty"`
	GitHubLogin       string `json:"github_login,omitempty"`
}

// Status is the non-secret user connection view.
type Status struct {
	Configured   bool       `json:"configured"`
	Connected    bool       `json:"connected"`
	Pending      bool       `json:"pending"`
	GitHubUserID int64      `json:"github_user_id,omitempty"`
	GitHubLogin  string     `json:"github_login,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
}

type pendingAuthorization struct {
	DeviceCode string
	ExpiresAt  time.Time
	Interval   time.Duration
	NextPollAt time.Time
}

// Service owns GitHub App device authorization, GitHub identity verification,
// and expiring user-token refresh. It does not decide repository or mutation
// permissions.
type Service struct {
	clientID string
	store    CredentialStore
	client   *http.Client
	now      func() time.Time

	pendingMu sync.Mutex
	pending   map[string]pendingAuthorization
	refreshMu sync.Mutex
}

// NewService creates a GitHub App device-flow service. Device flow requires the
// GitHub App client ID but not a client secret.
func NewService(store CredentialStore, clientID string) (*Service, error) {
	clientID = strings.TrimSpace(clientID)
	if store == nil || clientID == "" {
		return nil, ErrNotConfigured
	}
	transport := &http.Transport{
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}
	return &Service{
		clientID: clientID,
		store:    store,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return fmt.Errorf("GitHub authentication redirects are disabled")
			},
		},
		now:     func() time.Time { return time.Now().UTC() },
		pending: map[string]pendingAuthorization{},
	}, nil
}

// NewServiceFromEnvironment reads the operator-owned GitHub App client ID.
func NewServiceFromEnvironment(store CredentialStore) (*Service, error) {
	return NewService(store, os.Getenv(ClientIDEnv))
}

// StartDeviceAuthorization requests a new device/user-code pair for one OmniLLM
// user and replaces any previous pending authorization for that user.
func (s *Service) StartDeviceAuthorization(ctx context.Context, userID string) (DeviceAuthorization, error) {
	userID, err := requireUserID(userID)
	if err != nil {
		return DeviceAuthorization{}, err
	}
	var response deviceCodeResponse
	if err := s.postFormJSON(ctx, deviceCodeEndpoint, url.Values{"client_id": {s.clientID}}, &response); err != nil {
		return DeviceAuthorization{}, fmt.Errorf("GitHub device authorization could not be started")
	}
	if response.Error != "" {
		return DeviceAuthorization{}, deviceError(response.Error)
	}
	if response.DeviceCode == "" || response.UserCode == "" || response.VerificationURI != "https://github.com/login/device" || response.ExpiresIn <= 0 {
		return DeviceAuthorization{}, fmt.Errorf("GitHub returned an invalid device authorization response")
	}
	interval := time.Duration(response.Interval) * time.Second
	if interval <= 0 {
		interval = defaultPollInterval
	}
	if interval > maxPollInterval {
		return DeviceAuthorization{}, fmt.Errorf("GitHub returned an invalid device polling interval")
	}
	now := s.now()
	pending := pendingAuthorization{
		DeviceCode: response.DeviceCode,
		ExpiresAt:  now.Add(time.Duration(response.ExpiresIn) * time.Second),
		Interval:   interval,
		NextPollAt: now,
	}
	s.pendingMu.Lock()
	s.pruneExpiredLocked(now)
	s.pending[userID] = pending
	s.pendingMu.Unlock()
	return DeviceAuthorization{
		UserCode:        response.UserCode,
		VerificationURI: response.VerificationURI,
		ExpiresAt:       pending.ExpiresAt,
		IntervalSeconds: int(interval / time.Second),
	}, nil
}

// PollDeviceAuthorization performs at most one GitHub token request and never
// sleeps. The device code remains backend-only.
func (s *Service) PollDeviceAuthorization(ctx context.Context, userID string) (PollResult, error) {
	userID, err := requireUserID(userID)
	if err != nil {
		return PollResult{}, err
	}
	now := s.now()
	s.pendingMu.Lock()
	s.pruneExpiredLocked(now)
	pending, ok := s.pending[userID]
	if !ok {
		s.pendingMu.Unlock()
		return PollResult{Status: "not_started"}, nil
	}
	if now.Before(pending.NextPollAt) {
		retry := secondsUntil(now, pending.NextPollAt)
		s.pendingMu.Unlock()
		return PollResult{Status: "pending", RetryAfterSeconds: retry}, nil
	}
	pending.NextPollAt = now.Add(pending.Interval)
	s.pending[userID] = pending
	s.pendingMu.Unlock()

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
		return s.handlePollError(userID, response)
	}
	credential, err := s.credentialFromTokenResponse(ctx, response)
	if err != nil {
		return PollResult{}, err
	}
	if err := s.store.Save(userID, credential); err != nil {
		return PollResult{}, fmt.Errorf("save GitHub connection: %w", err)
	}
	s.clearPending(userID)
	return PollResult{Status: "connected", GitHubLogin: credential.GitHubLogin}, nil
}

// Status returns non-secret GitHub connection state.
func (s *Service) Status(userID string) (Status, error) {
	userID, err := requireUserID(userID)
	if err != nil {
		return Status{}, err
	}
	credential, err := s.store.Get(userID)
	if err != nil {
		return Status{}, err
	}
	now := s.now()
	s.pendingMu.Lock()
	s.pruneExpiredLocked(now)
	_, pending := s.pending[userID]
	s.pendingMu.Unlock()
	status := Status{Configured: true, Pending: pending}
	if credential == nil {
		return status, nil
	}
	status.GitHubUserID = credential.GitHubUserID
	status.GitHubLogin = credential.GitHubLogin
	status.ExpiresAt = credential.AccessExpiresAt
	status.Connected = credentialUsable(credential, now)
	return status, nil
}

// Disconnect clears local credentials and pending state. Provider-side token
// revocation is a separate future capability.
func (s *Service) Disconnect(userID string) error {
	userID, err := requireUserID(userID)
	if err != nil {
		return err
	}
	if err := s.store.Clear(userID); err != nil {
		return err
	}
	s.clearPending(userID)
	return nil
}

// AccessToken returns a backend-only user token, refreshing it when needed. It is
// intended for a later request-scoped gitrepo credential resolver and must never
// be returned in model/tool/API output.
func (s *Service) AccessToken(ctx context.Context, userID string) (string, error) {
	userID, err := requireUserID(userID)
	if err != nil {
		return "", err
	}
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	credential, err := s.store.Get(userID)
	if err != nil {
		return "", err
	}
	if credential == nil || strings.TrimSpace(credential.AccessToken) == "" {
		return "", ErrNotConnected
	}
	now := s.now()
	if credential.AccessExpiresAt == nil || credential.AccessExpiresAt.After(now.Add(refreshSkew)) {
		return credential.AccessToken, nil
	}
	if credential.RefreshToken == "" || (credential.RefreshExpiresAt != nil && !credential.RefreshExpiresAt.After(now)) {
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

func (s *Service) handlePollError(userID string, response tokenResponse) (PollResult, error) {
	now := s.now()
	switch response.Error {
	case "authorization_pending":
		s.pendingMu.Lock()
		pending, ok := s.pending[userID]
		s.pendingMu.Unlock()
		if !ok {
			return PollResult{Status: "not_started"}, nil
		}
		return PollResult{Status: "pending", RetryAfterSeconds: secondsUntil(now, pending.NextPollAt)}, nil
	case "slow_down":
		s.pendingMu.Lock()
		pending, ok := s.pending[userID]
		if ok {
			interval := pending.Interval + slowDownIncrement
			if response.Interval > 0 {
				interval = time.Duration(response.Interval) * time.Second
			}
			if interval > maxPollInterval {
				interval = maxPollInterval
			}
			pending.Interval = interval
			pending.NextPollAt = now.Add(interval)
			s.pending[userID] = pending
		}
		s.pendingMu.Unlock()
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
		return PollResult{}, deviceError(response.Error)
	default:
		return PollResult{}, fmt.Errorf("GitHub device authorization was rejected")
	}
}

func (s *Service) credentialFromTokenResponse(ctx context.Context, response tokenResponse) (Credential, error) {
	if response.AccessToken == "" || !strings.EqualFold(response.TokenType, "bearer") {
		return Credential{}, fmt.Errorf("GitHub returned an invalid access token response")
	}
	identity, err := s.userIdentity(ctx, response.AccessToken)
	if err != nil {
		return Credential{}, err
	}
	now := s.now()
	credential := Credential{
		AccessToken:  response.AccessToken,
		RefreshToken: response.RefreshToken,
		TokenType:    "bearer",
		Scope:        response.Scope,
		GitHubUserID: identity.ID,
		GitHubLogin:  identity.Login,
	}
	if response.ExpiresIn > 0 {
		expiresAt := now.Add(time.Duration(response.ExpiresIn) * time.Second)
		credential.AccessExpiresAt = &expiresAt
	}
	if response.RefreshTokenExpiresIn > 0 {
		expiresAt := now.Add(time.Duration(response.RefreshTokenExpiresIn) * time.Second)
		credential.RefreshExpiresAt = &expiresAt
	}
	return credential, nil
}

func (s *Service) userIdentity(ctx context.Context, token string) (githubUserResponse, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, userEndpoint, nil)
	if err != nil {
		return githubUserResponse{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("User-Agent", "OmniLLM-Studio")
	request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	var response githubUserResponse
	if err := s.doJSON(request, &response); err != nil {
		return githubUserResponse{}, fmt.Errorf("GitHub user identity could not be verified")
	}
	if response.ID <= 0 || strings.TrimSpace(response.Login) == "" {
		return githubUserResponse{}, fmt.Errorf("GitHub returned an invalid user identity")
	}
	return response, nil
}

func (s *Service) postFormJSON(ctx context.Context, endpoint string, form url.Values, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "OmniLLM-Studio")
	return s.doJSON(request, target)
}

func (s *Service) doJSON(request *http.Request, target any) error {
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("GitHub authentication request failed")
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return err
	}
	if int64(len(payload)) > maxResponseBytes {
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
	s.pendingMu.Lock()
	delete(s.pending, userID)
	s.pendingMu.Unlock()
}

func credentialUsable(credential *Credential, now time.Time) bool {
	if credential == nil || credential.AccessToken == "" {
		return false
	}
	if credential.AccessExpiresAt == nil || credential.AccessExpiresAt.After(now) {
		return true
	}
	return credential.RefreshToken != "" && (credential.RefreshExpiresAt == nil || credential.RefreshExpiresAt.After(now))
}

func requireUserID(userID string) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", fmt.Errorf("user ID is required")
	}
	return userID, nil
}

func secondsUntil(now, target time.Time) int {
	if !target.After(now) {
		return 0
	}
	return int((target.Sub(now) + time.Second - 1) / time.Second)
}

func deviceError(code string) error {
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
