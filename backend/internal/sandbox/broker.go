package sandbox

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	defaultSessionTTL = 30 * time.Minute
	maxSessionTTL     = 24 * time.Hour
)

// Broker is the application-owned sandbox control plane. It creates opaque
// session IDs, binds them to authenticated owner scope, validates requested
// policy against runtime capabilities, and revalidates ownership on every
// operation before dispatching to the execution runtime.
type Broker struct {
	runtime Runtime

	mu       sync.RWMutex
	sessions map[string]Session
	now      func() time.Time
}

// NewBroker creates a sandbox Broker for one runtime.
func NewBroker(runtime Runtime) (*Broker, error) {
	if runtime == nil {
		return nil, fmt.Errorf("sandbox runtime is required")
	}
	return &Broker{
		runtime:  runtime,
		sessions: make(map[string]Session),
		now:      time.Now,
	}, nil
}

// Capabilities returns controls actually enforced by the configured runtime.
func (b *Broker) Capabilities() RuntimeCapabilities {
	if b == nil || b.runtime == nil {
		return RuntimeCapabilities{}
	}
	return b.runtime.Capabilities()
}

// Create creates an application-issued, ownership-bound sandbox session.
func (b *Broker) Create(ctx context.Context, owner OwnerScope, request CreateRequest) (*Session, error) {
	if owner.Empty() {
		return nil, fmt.Errorf("sandbox owner scope is required")
	}
	if err := validateCreateRequest(request); err != nil {
		return nil, err
	}
	if err := requireCapabilities(b.runtime.Capabilities(), request.Requirements); err != nil {
		return nil, err
	}

	now := b.now().UTC()
	ttl := defaultSessionTTL
	if request.TTLSeconds > 0 {
		ttl = time.Duration(request.TTLSeconds) * time.Second
	}
	if ttl > maxSessionTTL {
		return nil, fmt.Errorf("sandbox ttl exceeds %s", maxSessionTTL)
	}
	request = cloneCreateRequest(request)
	request.TTLSeconds = int(ttl / time.Second)

	sessionID := "sbx_" + uuid.NewString()
	runtimeID, err := b.runtime.Create(ctx, RuntimeCreateRequest{
		SessionID: sessionID,
		Owner:     owner,
		Spec:      request,
	})
	if err != nil {
		return nil, fmt.Errorf("create sandbox runtime session: %w", err)
	}
	if strings.TrimSpace(runtimeID) == "" {
		return nil, fmt.Errorf("sandbox runtime returned an empty session id")
	}

	session := Session{
		ID:        sessionID,
		Owner:     owner,
		Spec:      request,
		RuntimeID: runtimeID,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
	b.mu.Lock()
	b.sessions[session.ID] = session
	b.mu.Unlock()
	clone := cloneSession(session)
	return &clone, nil
}

// Exec runs one code or terminal execution after ownership and expiry checks.
func (b *Broker) Exec(ctx context.Context, owner OwnerScope, sessionID string, request ExecRequest) (*ExecResult, error) {
	session, err := b.authorize(owner, sessionID)
	if err != nil {
		return nil, err
	}
	if err := validateExecRequest(request); err != nil {
		return nil, err
	}
	request.Args = append([]string(nil), request.Args...)
	request.Stdin = append([]byte(nil), request.Stdin...)
	request.Env = cloneStringMap(request.Env)
	return b.runtime.Exec(ctx, session.RuntimeID, request)
}

// Cancel requests cancellation of one execution inside an owned session.
func (b *Broker) Cancel(ctx context.Context, owner OwnerScope, sessionID, executionID string) error {
	session, err := b.authorize(owner, sessionID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(executionID) == "" {
		return fmt.Errorf("sandbox execution id is required")
	}
	return b.runtime.Cancel(ctx, session.RuntimeID, executionID)
}

// Status returns runtime state only after ownership validation.
func (b *Broker) Status(ctx context.Context, owner OwnerScope, sessionID string) (*Status, error) {
	session, err := b.authorize(owner, sessionID)
	if err != nil {
		return nil, err
	}
	status, err := b.runtime.Status(ctx, session.RuntimeID)
	if err != nil {
		return nil, err
	}
	if status == nil {
		status = &Status{}
	}
	status.SessionID = session.ID
	status.CreatedAt = session.CreatedAt
	status.ExpiresAt = session.ExpiresAt
	status.Capabilities = b.runtime.Capabilities()
	return status, nil
}

// Destroy tears down one owned session. The Broker keeps the ownership record if
// runtime teardown fails so a caller can inspect/retry rather than losing track
// of potentially live execution state.
func (b *Broker) Destroy(ctx context.Context, owner OwnerScope, sessionID string) error {
	session, err := b.authorize(owner, sessionID)
	if err != nil {
		return err
	}
	if err := b.runtime.Destroy(ctx, session.RuntimeID); err != nil {
		return err
	}
	b.mu.Lock()
	delete(b.sessions, session.ID)
	b.mu.Unlock()
	return nil
}

func (b *Broker) authorize(owner OwnerScope, sessionID string) (Session, error) {
	if owner.Empty() {
		return Session{}, fmt.Errorf("sandbox owner scope is required")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return Session{}, fmt.Errorf("sandbox session id is required")
	}
	b.mu.RLock()
	session, ok := b.sessions[sessionID]
	b.mu.RUnlock()
	if !ok {
		return Session{}, fmt.Errorf("sandbox session not found")
	}
	if !session.Owner.Equal(owner) {
		return Session{}, fmt.Errorf("sandbox session is not owned by the current scope")
	}
	if !b.now().UTC().Before(session.ExpiresAt) {
		return Session{}, fmt.Errorf("sandbox session has expired")
	}
	return cloneSession(session), nil
}

func validateCreateRequest(request CreateRequest) error {
	seenMounts := make(map[string]struct{}, len(request.Mounts))
	for _, mount := range request.Mounts {
		workspaceID := strings.TrimSpace(mount.WorkspaceID)
		if workspaceID == "" {
			return fmt.Errorf("sandbox workspace id is required")
		}
		if _, exists := seenMounts[workspaceID]; exists {
			return fmt.Errorf("sandbox workspace %q is mounted more than once", workspaceID)
		}
		seenMounts[workspaceID] = struct{}{}
		switch mount.Mode {
		case MountReadOnly, MountReadWriteNoDelete, MountReadWrite:
		default:
			return fmt.Errorf("unsupported sandbox mount mode %q", mount.Mode)
		}
	}

	switch request.Network.Mode {
	case "", NetworkNone, NetworkAllowlist, NetworkApprovalRequired:
	default:
		return fmt.Errorf("unsupported sandbox network mode %q", request.Network.Mode)
	}
	for _, domain := range request.Network.AllowedDomains {
		domain = strings.TrimSpace(domain)
		if domain == "" || strings.ContainsAny(domain, "/\\@") {
			return fmt.Errorf("invalid sandbox network domain %q", domain)
		}
	}
	for _, port := range request.Network.AllowedPorts {
		if port < 1 || port > 65535 {
			return fmt.Errorf("invalid sandbox network port %d", port)
		}
	}
	for key, value := range request.Environment {
		if err := validateEnvironmentEntry(key, value); err != nil {
			return err
		}
	}
	if request.TTLSeconds < 0 {
		return fmt.Errorf("sandbox ttl cannot be negative")
	}
	if request.TTLSeconds > int(maxSessionTTL/time.Second) {
		return fmt.Errorf("sandbox ttl exceeds %s", maxSessionTTL)
	}
	if err := validateResourceLimits(request.Resources); err != nil {
		return err
	}
	return nil
}

func validateExecRequest(request ExecRequest) error {
	hasCode := strings.TrimSpace(request.Language) != "" || strings.TrimSpace(request.Code) != ""
	hasCommand := strings.TrimSpace(request.Command) != ""
	if hasCode == hasCommand {
		return fmt.Errorf("sandbox execution must specify exactly one of code or command mode")
	}
	if hasCode {
		if strings.TrimSpace(request.Language) == "" {
			return fmt.Errorf("sandbox language is required for code execution")
		}
		if strings.TrimSpace(request.Code) == "" {
			return fmt.Errorf("sandbox code is required")
		}
	}
	if strings.ContainsRune(request.Command, '\x00') || strings.ContainsRune(request.Language, '\x00') || strings.ContainsRune(request.Code, '\x00') {
		return fmt.Errorf("sandbox execution input contains NUL")
	}
	for _, arg := range request.Args {
		if strings.ContainsRune(arg, '\x00') {
			return fmt.Errorf("sandbox argument contains NUL")
		}
	}
	if request.TimeoutMS < 0 {
		return fmt.Errorf("sandbox timeout cannot be negative")
	}
	for key, value := range request.Env {
		if err := validateEnvironmentEntry(key, value); err != nil {
			return err
		}
	}
	return nil
}

func validateResourceLimits(limits ResourceLimits) error {
	if limits.WallTimeMS < 0 || limits.CPUTimeMS < 0 || limits.MemoryBytes < 0 || limits.DiskBytes < 0 ||
		limits.MaxProcesses < 0 || limits.MaxFiles < 0 || limits.MaxStdoutBytes < 0 || limits.MaxStderrBytes < 0 ||
		limits.MaxArtifactBytes < 0 {
		return fmt.Errorf("sandbox resource limits cannot be negative")
	}
	return nil
}

func requireCapabilities(cap RuntimeCapabilities, required RuntimeRequirements) error {
	missing := make([]string, 0, 8)
	if required.OSIsolation && !cap.OSIsolation {
		missing = append(missing, "os_isolation")
	}
	if required.FilesystemIsolation && !cap.FilesystemIsolation {
		missing = append(missing, "filesystem_isolation")
	}
	if required.NetworkIsolation && !cap.NetworkIsolation {
		missing = append(missing, "network_isolation")
	}
	if required.ProcessTreeIsolation && !cap.ProcessTreeIsolation {
		missing = append(missing, "process_tree_isolation")
	}
	if required.MemoryLimit && !cap.MemoryLimit {
		missing = append(missing, "memory_limit")
	}
	if required.CPULimit && !cap.CPULimit {
		missing = append(missing, "cpu_limit")
	}
	if required.PIDLimit && !cap.PIDLimit {
		missing = append(missing, "pid_limit")
	}
	if required.DiskLimit && !cap.DiskLimit {
		missing = append(missing, "disk_limit")
	}
	if len(missing) > 0 {
		return fmt.Errorf("sandbox runtime %q cannot satisfy required controls: %s", cap.Name, strings.Join(missing, ", "))
	}
	return nil
}

func cloneCreateRequest(request CreateRequest) CreateRequest {
	request.Mounts = append([]WorkspaceMount(nil), request.Mounts...)
	request.Network.AllowedDomains = append([]string(nil), request.Network.AllowedDomains...)
	request.Network.AllowedPorts = append([]int(nil), request.Network.AllowedPorts...)
	request.Environment = cloneStringMap(request.Environment)
	if request.Network.Mode == "" {
		request.Network.Mode = NetworkNone
	}
	return request
}

func cloneSession(session Session) Session {
	session.Spec = cloneCreateRequest(session.Spec)
	return session
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
