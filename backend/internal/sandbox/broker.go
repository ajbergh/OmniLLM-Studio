package sandbox

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	defaultSessionTTL = 30 * time.Minute
	maxSessionTTL     = 24 * time.Hour
)

type Broker struct {
	runtime Runtime

	mu       sync.RWMutex
	sessions map[string]Session
	now      func() time.Time
}

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

func (b *Broker) Capabilities() RuntimeCapabilities {
	if b == nil || b.runtime == nil {
		return RuntimeCapabilities{}
	}
	return b.runtime.Capabilities()
}

func (b *Broker) Create(ctx context.Context, owner OwnerScope, request CreateRequest) (*Session, error) {
	if owner.Empty() {
		return nil, fmt.Errorf("sandbox owner scope is required")
	}
	if err := validateCreateRequest(request); err != nil {
		return nil, err
	}
	capabilities := b.runtime.Capabilities()
	if err := requireCapabilities(capabilities, request.Requirements); err != nil {
		return nil, err
	}
	if request.Network.Mode == NetworkApprovalRequired {
		return nil, fmt.Errorf("sandbox network approval must be resolved to an owner-bound grant before runtime creation")
	}
	if request.Network.Mode == NetworkAllowlist && !capabilities.NetworkAllowlist {
		return nil, fmt.Errorf("sandbox runtime %q cannot enforce destination network allowlists", capabilities.Name)
	}
	resolvedMounts, err := resolveRuntimeMounts(owner, request.Mounts)
	if err != nil {
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
		SessionID:      sessionID,
		Owner:          owner,
		Spec:           request,
		ResolvedMounts: resolvedMounts,
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

func resolveRuntimeMounts(owner OwnerScope, mounts []WorkspaceMount) ([]RuntimeMount, error) {
	if len(mounts) == 0 {
		return nil, nil
	}
	registry, err := DefaultWorkspaceRegistry()
	if err != nil {
		return nil, fmt.Errorf("resolve sandbox workspaces: %w", err)
	}
	if err := seedConfiguredWorkspaces(owner.UserID, registry); err != nil {
		return nil, err
	}
	resolved := make([]RuntimeMount, 0, len(mounts))
	for _, requested := range mounts {
		workspace, err := registry.Get(owner.UserID, requested.WorkspaceID)
		if err != nil {
			return nil, err
		}
		// A caller may narrow a stored grant, but never widen it.
		effective, err := intersectMountMode(workspace.Mode, requested.Mode)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, RuntimeMount{
			WorkspaceID: workspace.ID,
			SourcePath:  workspace.RootPath,
			Mode:        effective,
		})
	}
	return resolved, nil
}

func intersectMountMode(granted, requested MountMode) (MountMode, error) {
	if err := validateMountMode(granted); err != nil {
		return "", err
	}
	if err := validateMountMode(requested); err != nil {
		return "", err
	}
	rank := func(mode MountMode) int {
		switch mode {
		case MountReadOnly:
			return 0
		case MountReadWriteNoDelete:
			return 1
		case MountReadWrite:
			return 2
		default:
			return -1
		}
	}
	if rank(requested) > rank(granted) {
		return "", fmt.Errorf("sandbox mount request would widen workspace grant")
	}
	return requested, nil
}

func (b *Broker) Exec(ctx context.Context, owner OwnerScope, sessionID string, request ExecRequest) (*ExecResult, error) {
	session, err := b.authorize(owner, sessionID)
	if err != nil {
		return nil, err
	}
	if err := validateExecRequest(request); err != nil {
		return nil, err
	}
	executionID, err := executionIDOrNew(request.ExecutionID)
	if err != nil {
		return nil, err
	}
	request.ExecutionID = executionID
	request.Args = append([]string(nil), request.Args...)
	request.Stdin = append([]byte(nil), request.Stdin...)
	request.Env = cloneStringMap(request.Env)
	result, err := b.runtime.Exec(ctx, session.RuntimeID, request)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("sandbox runtime returned an empty execution result")
	}
	if result.ExecutionID != executionID {
		return nil, fmt.Errorf("sandbox runtime returned mismatched execution id")
	}
	return result, nil
}

func (b *Broker) Cancel(ctx context.Context, owner OwnerScope, sessionID, executionID string) error {
	session, err := b.authorize(owner, sessionID)
	if err != nil {
		return err
	}
	if err := validateExecutionID(executionID); err != nil {
		return err
	}
	return b.runtime.Cancel(ctx, session.RuntimeID, executionID)
}

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
		if err := validateMountMode(mount.Mode); err != nil {
			return err
		}
	}

	switch request.Network.Mode {
	case "", NetworkNone, NetworkAllowlist, NetworkApprovalRequired:
	default:
		return fmt.Errorf("unsupported sandbox network mode %q", request.Network.Mode)
	}
	for _, domain := range request.Network.AllowedDomains {
		domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
		if domain == "" || domain == "localhost" || net.ParseIP(domain) != nil || strings.ContainsAny(domain, "/\\@") {
			return fmt.Errorf("invalid sandbox network domain %q", domain)
		}
	}
	for _, port := range request.Network.AllowedPorts {
		if port < 1 || port > 65535 {
			return fmt.Errorf("invalid sandbox network port %d", port)
		}
	}
	for key, value := range request.Environment {
		if err := validateSandboxEnvironmentEntry(key, value); err != nil {
			return err
		}
	}
	if request.TTLSeconds < 0 || request.TTLSeconds > int(maxSessionTTL/time.Second) {
		return fmt.Errorf("invalid sandbox ttl")
	}
	return validateResourceLimits(request.Resources)
}

func validateExecRequest(request ExecRequest) error {
	hasCode := strings.TrimSpace(request.Language) != "" || strings.TrimSpace(request.Code) != ""
	hasCommand := strings.TrimSpace(request.Command) != ""
	if hasCode == hasCommand {
		return fmt.Errorf("sandbox execution must specify exactly one of code or command mode")
	}
	if hasCode && (strings.TrimSpace(request.Language) == "" || strings.TrimSpace(request.Code) == "") {
		return fmt.Errorf("sandbox language and code are required")
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
		if err := validateSandboxEnvironmentEntry(key, value); err != nil {
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
	missing := make([]string, 0, 9)
	if required.OSIsolation && !cap.OSIsolation {
		missing = append(missing, "os_isolation")
	}
	if required.FilesystemIsolation && !cap.FilesystemIsolation {
		missing = append(missing, "filesystem_isolation")
	}
	if required.NetworkIsolation && !cap.NetworkIsolation {
		missing = append(missing, "network_isolation")
	}
	if required.NetworkAllowlist && !cap.NetworkAllowlist {
		missing = append(missing, "network_allowlist")
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
