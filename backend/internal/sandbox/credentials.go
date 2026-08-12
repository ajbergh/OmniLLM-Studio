package sandbox

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CredentialSource resolves a secret only inside trusted host-side code. The
// resolved value must never be returned by a model-facing tool or injected into
// an arbitrary sandbox environment.
type CredentialSource func(context.Context, OwnerScope) (string, error)

// CredentialHandle is an opaque, owner-bound reference to a host-side credential
// source. It intentionally contains no secret material.
type CredentialHandle struct {
	ID        string    `json:"id"`
	Service   string    `json:"service"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type credentialHandleRecord struct {
	CredentialHandle
	Owner OwnerScope
}

// CredentialBroker keeps secret resolution outside sandbox processes. Only
// trusted application code can register sources or redeem handles.
type CredentialBroker struct {
	mu      sync.RWMutex
	sources map[string]CredentialSource
	handles map[string]credentialHandleRecord
	now     func() time.Time
}

func NewCredentialBroker() *CredentialBroker {
	return &CredentialBroker{
		sources: make(map[string]CredentialSource),
		handles: make(map[string]credentialHandleRecord),
		now:     time.Now,
	}
}

// RegisterSource installs one trusted host-side credential resolver. Replacing
// an existing service is rejected so authority cannot silently change under an
// already-issued handle.
func (b *CredentialBroker) RegisterSource(service string, source CredentialSource) error {
	service = normalizeCredentialService(service)
	if service == "" || source == nil {
		return fmt.Errorf("credential service and source are required")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.sources[service]; exists {
		return fmt.Errorf("credential service %q is already registered", service)
	}
	b.sources[service] = source
	return nil
}

// Issue creates an owner-bound handle without resolving or storing the secret.
func (b *CredentialBroker) Issue(owner OwnerScope, service string, ttl time.Duration) (*CredentialHandle, error) {
	if owner.Empty() || owner.UserID == "" {
		return nil, fmt.Errorf("credential handle owner is required")
	}
	service = normalizeCredentialService(service)
	if service == "" {
		return nil, fmt.Errorf("credential service is required")
	}
	if ttl <= 0 || ttl > 30*time.Minute {
		ttl = 15 * time.Minute
	}
	b.mu.RLock()
	_, exists := b.sources[service]
	b.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("credential service %q is not registered", service)
	}
	now := b.now().UTC()
	record := credentialHandleRecord{
		CredentialHandle: CredentialHandle{
			ID:        "sch_" + uuid.NewString(),
			Service:   service,
			CreatedAt: now,
			ExpiresAt: now.Add(ttl),
		},
		Owner: owner,
	}
	b.mu.Lock()
	b.handles[record.ID] = record
	b.mu.Unlock()
	copy := record.CredentialHandle
	return &copy, nil
}

// Redeem resolves a credential only for the exact owner of a live handle. This
// method is intentionally host-only; no generic Tool exposes it.
func (b *CredentialBroker) Redeem(ctx context.Context, owner OwnerScope, handleID string) (string, error) {
	handleID = strings.TrimSpace(handleID)
	if handleID == "" {
		return "", fmt.Errorf("credential handle id is required")
	}
	b.mu.RLock()
	record, exists := b.handles[handleID]
	source := b.sources[record.Service]
	b.mu.RUnlock()
	if !exists {
		return "", fmt.Errorf("credential handle not found")
	}
	if !record.Owner.Equal(owner) {
		return "", fmt.Errorf("credential handle is not owned by the current scope")
	}
	if !b.now().UTC().Before(record.ExpiresAt) {
		return "", fmt.Errorf("credential handle has expired")
	}
	if source == nil {
		return "", fmt.Errorf("credential source is unavailable")
	}
	secret, err := source(ctx, owner)
	if err != nil {
		return "", fmt.Errorf("resolve credential service %q: %w", record.Service, err)
	}
	if strings.TrimSpace(secret) == "" {
		return "", fmt.Errorf("credential service %q returned an empty value", record.Service)
	}
	return secret, nil
}

// Revoke invalidates one exact owner-bound credential handle.
func (b *CredentialBroker) Revoke(owner OwnerScope, handleID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	record, exists := b.handles[strings.TrimSpace(handleID)]
	if !exists {
		return fmt.Errorf("credential handle not found")
	}
	if !record.Owner.Equal(owner) {
		return fmt.Errorf("credential handle is not owned by the current scope")
	}
	delete(b.handles, record.ID)
	return nil
}

func normalizeCredentialService(service string) string {
	service = strings.ToLower(strings.TrimSpace(service))
	if service == "" || len(service) > 64 {
		return ""
	}
	for _, r := range service {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.') {
			return ""
		}
	}
	return service
}

// IsSensitiveEnvironmentKey reports whether a key is credential-bearing or can
// redirect network/auth flows around the sandbox brokers.
func IsSensitiveEnvironmentKey(key string) bool {
	upper := strings.ToUpper(strings.TrimSpace(key))
	if upper == "" {
		return false
	}
	exact := map[string]struct{}{
		"SSH_AUTH_SOCK": {}, "SSH_AGENT_PID": {}, "GIT_ASKPASS": {}, "GIT_SSH_COMMAND": {},
		"GOOGLE_APPLICATION_CREDENTIALS": {}, "AWS_PROFILE": {}, "AWS_SHARED_CREDENTIALS_FILE": {},
		"AZURE_CONFIG_DIR": {}, "DOCKER_CONFIG": {}, "KUBECONFIG": {},
		"HTTP_PROXY": {}, "HTTPS_PROXY": {}, "ALL_PROXY": {}, "NO_PROXY": {},
	}
	if _, blocked := exact[upper]; blocked {
		return true
	}
	for _, fragment := range []string{"TOKEN", "SECRET", "PASSWORD", "PASSWD", "API_KEY", "APIKEY", "PRIVATE_KEY", "ACCESS_KEY", "CREDENTIAL"} {
		if strings.Contains(upper, fragment) {
			return true
		}
	}
	if strings.HasPrefix(upper, "OMNILLM_") && strings.Contains(upper, "KEY") {
		return true
	}
	return false
}

// validateSandboxEnvironmentEntry is stricter than the generic subprocess
// environment validator because arbitrary sandboxes must not receive secrets or
// proxy/auth escape hatches even from internal callers.
func validateSandboxEnvironmentEntry(key, value string) error {
	if err := validateEnvironmentEntry(key, value); err != nil {
		return err
	}
	if IsSensitiveEnvironmentKey(key) {
		return fmt.Errorf("sandbox environment key %q is credential-sensitive and must use a host-side broker", key)
	}
	return nil
}

var defaultCredentialBroker = NewCredentialBroker()

func DefaultCredentialBroker() *CredentialBroker { return defaultCredentialBroker }
