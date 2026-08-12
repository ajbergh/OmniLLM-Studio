package sandbox

import "time"

// OwnerScope binds one sandbox to the authenticated application context that
// created it. A sandbox ID is only a reference; every Broker operation also
// requires the same owner scope.
type OwnerScope struct {
	UserID         string `json:"user_id,omitempty"`
	WorkspaceID    string `json:"workspace_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	MessageID      string `json:"message_id,omitempty"`
	AgentRunID     string `json:"agent_run_id,omitempty"`
	TaskID         string `json:"task_id,omitempty"`
}

// Equal reports whether two owner scopes are identical.
func (o OwnerScope) Equal(other OwnerScope) bool {
	return o == other
}

// Empty reports whether no application ownership dimension is present.
func (o OwnerScope) Empty() bool {
	return o == (OwnerScope{})
}

// MountMode describes the maximum filesystem access granted to one opaque
// application-owned workspace mount.
type MountMode string

const (
	MountReadOnly          MountMode = "read_only"
	MountReadWriteNoDelete MountMode = "read_write_no_delete"
	MountReadWrite         MountMode = "read_write"
)

// WorkspaceMount grants a sandbox access to a registered workspace by opaque
// ID. Physical host paths never appear in model-facing sandbox requests.
type WorkspaceMount struct {
	WorkspaceID string    `json:"workspace_id"`
	Mode        MountMode `json:"mode"`
}

// NetworkMode controls whether a sandbox may open outbound network connections.
type NetworkMode string

const (
	NetworkNone             NetworkMode = "none"
	NetworkAllowlist        NetworkMode = "allowlist"
	NetworkApprovalRequired NetworkMode = "approval_required"
)

// NetworkPolicy is application-derived. AllowedDomains and AllowedPorts are
// meaningful only for allowlisted/approved network access; hard deployment
// denies remain authoritative outside this structure.
type NetworkPolicy struct {
	Mode           NetworkMode `json:"mode"`
	AllowedDomains []string    `json:"allowed_domains,omitempty"`
	AllowedPorts   []int       `json:"allowed_ports,omitempty"`
}

// ResourceLimits bounds one sandbox. A zero field means the Broker/runtime
// default applies; it never means unlimited when a hardened deployment requires
// that resource control.
type ResourceLimits struct {
	WallTimeMS       int   `json:"wall_time_ms,omitempty"`
	CPUTimeMS        int   `json:"cpu_time_ms,omitempty"`
	MemoryBytes      int64 `json:"memory_bytes,omitempty"`
	DiskBytes        int64 `json:"disk_bytes,omitempty"`
	MaxProcesses     int   `json:"max_processes,omitempty"`
	MaxFiles         int   `json:"max_files,omitempty"`
	MaxStdoutBytes   int64 `json:"max_stdout_bytes,omitempty"`
	MaxStderrBytes   int64 `json:"max_stderr_bytes,omitempty"`
	MaxArtifactBytes int64 `json:"max_artifact_bytes,omitempty"`
}

// RuntimeRequirements allows a deployment/tool to fail closed when a selected
// runtime cannot enforce security properties required for that workload.
type RuntimeRequirements struct {
	OSIsolation          bool `json:"os_isolation,omitempty"`
	FilesystemIsolation  bool `json:"filesystem_isolation,omitempty"`
	NetworkIsolation     bool `json:"network_isolation,omitempty"`
	ProcessTreeIsolation bool `json:"process_tree_isolation,omitempty"`
	MemoryLimit          bool `json:"memory_limit,omitempty"`
	CPULimit             bool `json:"cpu_limit,omitempty"`
	PIDLimit             bool `json:"pid_limit,omitempty"`
	DiskLimit            bool `json:"disk_limit,omitempty"`
}

// RuntimeCapabilities describes controls actually enforced by a runtime, not
// merely controls requested by configuration.
type RuntimeCapabilities struct {
	Name                 string `json:"name"`
	Version              string `json:"version,omitempty"`
	OSIsolation          bool   `json:"os_isolation"`
	FilesystemIsolation  bool   `json:"filesystem_isolation"`
	NetworkIsolation     bool   `json:"network_isolation"`
	ProcessTreeIsolation bool   `json:"process_tree_isolation"`
	MemoryLimit          bool   `json:"memory_limit"`
	CPULimit             bool   `json:"cpu_limit"`
	PIDLimit             bool   `json:"pid_limit"`
	DiskLimit            bool   `json:"disk_limit"`
}

// CreateRequest describes application-approved capabilities for a new sandbox.
// Owner identity is deliberately not present; Broker.Create receives owner
// scope separately from authenticated request context.
type CreateRequest struct {
	Mounts       []WorkspaceMount    `json:"mounts,omitempty"`
	Network      NetworkPolicy       `json:"network"`
	Resources    ResourceLimits      `json:"resources,omitempty"`
	Environment  map[string]string   `json:"environment,omitempty"`
	Profile      string              `json:"profile,omitempty"`
	TTLSeconds   int                 `json:"ttl_seconds,omitempty"`
	Requirements RuntimeRequirements `json:"requirements,omitempty"`
}

// RuntimeCreateRequest is sent only across the trusted Broker/runtime boundary.
// It includes application ownership for worker auditing but workers must still
// treat the authenticated Broker as the authority.
type RuntimeCreateRequest struct {
	SessionID string        `json:"session_id"`
	Owner     OwnerScope    `json:"owner"`
	Spec      CreateRequest `json:"spec"`
}

// Session is Broker-owned metadata for a created sandbox.
type Session struct {
	ID        string        `json:"id"`
	Owner     OwnerScope    `json:"owner"`
	Spec      CreateRequest `json:"spec"`
	RuntimeID string        `json:"runtime_id"`
	CreatedAt time.Time     `json:"created_at"`
	ExpiresAt time.Time     `json:"expires_at"`
}

// ExecRequest describes one execution inside an existing sandbox. Code tools
// use Language+Code; terminal/process tools use Command+Args. A request must use
// exactly one execution mode.
type ExecRequest struct {
	Language  string            `json:"language,omitempty"`
	Code      string            `json:"code,omitempty"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Directory string            `json:"directory,omitempty"`
	Stdin     []byte            `json:"stdin,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	TimeoutMS int               `json:"timeout_ms,omitempty"`
}

// Artifact is a sandbox-produced object referenced by application-owned ID.
// Runtime workers do not return arbitrary URLs as the artifact trust boundary.
type Artifact struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MimeType string `json:"mime_type,omitempty"`
	Bytes    int64  `json:"bytes,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
}

// ExecResult is the bounded result of one sandbox execution.
type ExecResult struct {
	ExecutionID string         `json:"execution_id"`
	Stdout      string         `json:"stdout,omitempty"`
	Stderr      string         `json:"stderr,omitempty"`
	ExitCode    int            `json:"exit_code"`
	DurationMS  int64          `json:"duration_ms,omitempty"`
	Artifacts   []Artifact     `json:"artifacts,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Status reports Broker/runtime-observed sandbox state.
type Status struct {
	SessionID    string              `json:"session_id"`
	State        string              `json:"state"`
	Capabilities RuntimeCapabilities `json:"capabilities"`
	CreatedAt    time.Time           `json:"created_at,omitempty"`
	ExpiresAt    time.Time           `json:"expires_at,omitempty"`
	Metadata     map[string]any      `json:"metadata,omitempty"`
}
