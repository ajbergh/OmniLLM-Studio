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

func (o OwnerScope) Equal(other OwnerScope) bool { return o == other }
func (o OwnerScope) Empty() bool                 { return o == (OwnerScope{}) }

type MountMode string

const (
	MountReadOnly          MountMode = "read_only"
	MountReadWriteNoDelete MountMode = "read_write_no_delete"
	MountReadWrite         MountMode = "read_write"
)

// WorkspaceMount is model/application-facing and contains only an opaque ID.
type WorkspaceMount struct {
	WorkspaceID string    `json:"workspace_id"`
	Mode        MountMode `json:"mode"`
}

// RuntimeMount exists only across the trusted Broker/runtime boundary. SourcePath
// is never accepted from model tool arguments and is never returned to clients.
type RuntimeMount struct {
	WorkspaceID string    `json:"workspace_id"`
	SourcePath  string    `json:"source_path"`
	Mode        MountMode `json:"mode"`
}

type NetworkMode string

const (
	NetworkNone             NetworkMode = "none"
	NetworkAllowlist        NetworkMode = "allowlist"
	NetworkApprovalRequired NetworkMode = "approval_required"
)

type NetworkPolicy struct {
	Mode           NetworkMode `json:"mode"`
	AllowedDomains []string    `json:"allowed_domains,omitempty"`
	AllowedPorts   []int       `json:"allowed_ports,omitempty"`
}

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

type RuntimeRequirements struct {
	OSIsolation          bool `json:"os_isolation,omitempty"`
	FilesystemIsolation  bool `json:"filesystem_isolation,omitempty"`
	NetworkIsolation     bool `json:"network_isolation,omitempty"`
	NetworkAllowlist     bool `json:"network_allowlist,omitempty"`
	ProcessTreeIsolation bool `json:"process_tree_isolation,omitempty"`
	MemoryLimit          bool `json:"memory_limit,omitempty"`
	CPULimit             bool `json:"cpu_limit,omitempty"`
	PIDLimit             bool `json:"pid_limit,omitempty"`
	DiskLimit            bool `json:"disk_limit,omitempty"`
}

type RuntimeCapabilities struct {
	Name                 string `json:"name"`
	Version              string `json:"version,omitempty"`
	OSIsolation          bool   `json:"os_isolation"`
	FilesystemIsolation  bool   `json:"filesystem_isolation"`
	NetworkIsolation     bool   `json:"network_isolation"`
	NetworkAllowlist     bool   `json:"network_allowlist"`
	ProcessTreeIsolation bool   `json:"process_tree_isolation"`
	MemoryLimit          bool   `json:"memory_limit"`
	CPULimit             bool   `json:"cpu_limit"`
	PIDLimit             bool   `json:"pid_limit"`
	DiskLimit            bool   `json:"disk_limit"`
}

type CreateRequest struct {
	Mounts       []WorkspaceMount    `json:"mounts,omitempty"`
	Network      NetworkPolicy       `json:"network"`
	Resources    ResourceLimits      `json:"resources,omitempty"`
	Environment  map[string]string   `json:"environment,omitempty"`
	Profile      string              `json:"profile,omitempty"`
	TTLSeconds   int                 `json:"ttl_seconds,omitempty"`
	Requirements RuntimeRequirements `json:"requirements,omitempty"`
}

// RuntimeCreateRequest is trusted control-plane data. ResolvedMounts is derived
// by Broker from owner-scoped workspace grants; it never comes from CreateRequest.
type RuntimeCreateRequest struct {
	SessionID      string         `json:"session_id"`
	Owner          OwnerScope     `json:"owner"`
	Spec           CreateRequest  `json:"spec"`
	ResolvedMounts []RuntimeMount `json:"resolved_mounts,omitempty"`
}

type Session struct {
	ID        string        `json:"id"`
	Owner     OwnerScope    `json:"owner"`
	Spec      CreateRequest `json:"spec"`
	RuntimeID string        `json:"runtime_id"`
	CreatedAt time.Time     `json:"created_at"`
	ExpiresAt time.Time     `json:"expires_at"`
}

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

type Artifact struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MimeType string `json:"mime_type,omitempty"`
	Bytes    int64 `json:"bytes,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
}

type ExecResult struct {
	ExecutionID string         `json:"execution_id"`
	Stdout      string         `json:"stdout,omitempty"`
	Stderr      string         `json:"stderr,omitempty"`
	ExitCode    int            `json:"exit_code"`
	DurationMS  int64          `json:"duration_ms,omitempty"`
	Artifacts   []Artifact     `json:"artifacts,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type Status struct {
	SessionID    string              `json:"session_id"`
	State        string              `json:"state"`
	Capabilities RuntimeCapabilities `json:"capabilities"`
	CreatedAt    time.Time           `json:"created_at,omitempty"`
	ExpiresAt    time.Time           `json:"expires_at,omitempty"`
	Metadata     map[string]any      `json:"metadata,omitempty"`
}
