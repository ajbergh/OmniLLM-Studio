package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	ConversationKindChat  = "chat"
	ConversationKindImage = "image"
)

// Conversation represents a chat conversation.
type Conversation struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Archived        bool      `json:"archived"`
	Pinned          bool      `json:"pinned"`
	DefaultProvider *string   `json:"default_provider,omitempty"`
	DefaultModel    *string   `json:"default_model,omitempty"`
	SystemPrompt    *string   `json:"system_prompt,omitempty"`
	Kind            string    `json:"kind"`
	MetadataJSON    string    `json:"metadata_json,omitempty"`
	WorkspaceID     *string   `json:"workspace_id,omitempty"`
	UserID          *string   `json:"user_id,omitempty"`
}

// Message represents a single message in a conversation.
type Message struct {
	ID              string    `json:"id"`
	ConversationID  string    `json:"conversation_id"`
	Role            string    `json:"role"` // user, assistant, system, tool
	Content         string    `json:"content"`
	CreatedAt       time.Time `json:"created_at"`
	Provider        *string   `json:"provider,omitempty"`
	Model           *string   `json:"model,omitempty"`
	TokenInput      *int      `json:"token_input,omitempty"`
	TokenOutput     *int      `json:"token_output,omitempty"`
	LatencyMs       *int      `json:"latency_ms,omitempty"`
	MetadataJSON    string    `json:"metadata_json,omitempty"`
	BranchID        string    `json:"branch_id"`
	ParentMessageID *string   `json:"parent_message_id,omitempty"`
	UserID          *string   `json:"user_id,omitempty"`
}

// Attachment represents a file or image attached to a conversation/message.
type Attachment struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	MessageID      *string   `json:"message_id,omitempty"`
	Type           string    `json:"type"` // image, file
	MimeType       string    `json:"mime_type"`
	StoragePath    string    `json:"storage_path"`
	Bytes          int64     `json:"bytes"`
	Width          *int      `json:"width,omitempty"`
	Height         *int      `json:"height,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	MetadataJSON   string    `json:"metadata_json,omitempty"`
}

// ProviderProfile represents a configured LLM provider.
type ProviderProfile struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Type              string    `json:"type"`
	BaseURL           *string   `json:"base_url,omitempty"`
	DefaultModel      *string   `json:"default_model,omitempty"`
	DefaultImageModel *string   `json:"default_image_model,omitempty"`
	Enabled           bool      `json:"enabled"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	MetadataJSON      string    `json:"metadata_json,omitempty"`
}

// Setting represents a key-value application setting.
type Setting struct {
	Key       string `json:"key"`
	ValueJSON string `json:"value_json"`
}

// AppSettings represents the typed application settings.
type AppSettings struct {
	WebSearchProvider string `json:"web_search_provider"`
	BraveAPIKey       string `json:"brave_api_key"`
	JinaReaderEnabled bool   `json:"jina_reader_enabled"`
	JinaReaderMaxLen  int    `json:"jina_reader_max_len,omitempty"`
	// RAG settings
	RAGEnabled        bool   `json:"rag_enabled"`
	RAGEmbeddingModel string `json:"rag_embedding_model,omitempty"`
	RAGChunkSize      int    `json:"rag_chunk_size,omitempty"`
	RAGChunkOverlap   int    `json:"rag_chunk_overlap,omitempty"`
	RAGTopK           int    `json:"rag_top_k,omitempty"`
}

// DefaultAppSettings returns the default settings.
func DefaultAppSettings() AppSettings {
	return AppSettings{
		WebSearchProvider: "auto",
		BraveAPIKey:       "",
		JinaReaderEnabled: true,
		JinaReaderMaxLen:  3000,
		RAGEnabled:        false,
		RAGEmbeddingModel: "text-embedding-3-small",
		RAGChunkSize:      1000,
		RAGChunkOverlap:   200,
		RAGTopK:           5,
	}
}

// ToMap converts typed settings to a raw key-value map for storage.
func (s AppSettings) ToMap() map[string]string {
	m := make(map[string]string)
	m["web_search_provider"] = s.WebSearchProvider
	m["brave_api_key"] = s.BraveAPIKey
	if s.JinaReaderEnabled {
		m["jina_reader_enabled"] = "true"
	} else {
		m["jina_reader_enabled"] = "false"
	}
	if s.JinaReaderMaxLen > 0 {
		m["jina_reader_max_len"] = fmt.Sprintf("%d", s.JinaReaderMaxLen)
	}
	// RAG settings
	if s.RAGEnabled {
		m["rag_enabled"] = "true"
	} else {
		m["rag_enabled"] = "false"
	}
	if s.RAGEmbeddingModel != "" {
		m["rag_embedding_model"] = s.RAGEmbeddingModel
	}
	if s.RAGChunkSize > 0 {
		m["rag_chunk_size"] = fmt.Sprintf("%d", s.RAGChunkSize)
	}
	if s.RAGChunkOverlap > 0 {
		m["rag_chunk_overlap"] = fmt.Sprintf("%d", s.RAGChunkOverlap)
	}
	if s.RAGTopK > 0 {
		m["rag_top_k"] = fmt.Sprintf("%d", s.RAGTopK)
	}
	return m
}

// AppSettingsFromMap converts a raw key-value map to typed settings.
func AppSettingsFromMap(m map[string]string) AppSettings {
	s := DefaultAppSettings()

	if v, ok := m["web_search_provider"]; ok {
		v = strings.TrimSpace(strings.Trim(v, `"`))
		if v != "" {
			s.WebSearchProvider = v
		}
	}
	if v, ok := m["brave_api_key"]; ok {
		s.BraveAPIKey = strings.TrimSpace(strings.Trim(v, `"`))
	}
	if v, ok := m["jina_reader_enabled"]; ok {
		v = strings.TrimSpace(strings.Trim(v, `"`))
		s.JinaReaderEnabled = v != "false" && v != "off" && v != "0"
	}
	if v, ok := m["jina_reader_max_len"]; ok {
		v = strings.TrimSpace(strings.Trim(v, `"`))
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			s.JinaReaderMaxLen = n
		}
	}

	// RAG settings
	if v, ok := m["rag_enabled"]; ok {
		v = strings.TrimSpace(strings.Trim(v, `"`))
		s.RAGEnabled = v == "true" || v == "1"
	}
	if v, ok := m["rag_embedding_model"]; ok {
		v = strings.TrimSpace(strings.Trim(v, `"`))
		if v != "" {
			s.RAGEmbeddingModel = v
		}
	}
	if v, ok := m["rag_chunk_size"]; ok {
		v = strings.TrimSpace(strings.Trim(v, `"`))
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			s.RAGChunkSize = n
		}
	}
	if v, ok := m["rag_chunk_overlap"]; ok {
		v = strings.TrimSpace(strings.Trim(v, `"`))
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			s.RAGChunkOverlap = n
		}
	}
	if v, ok := m["rag_top_k"]; ok {
		v = strings.TrimSpace(strings.Trim(v, `"`))
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			s.RAGTopK = n
		}
	}

	return s
}

// ---------------------------------------------------------------------------
// RAG Models
// ---------------------------------------------------------------------------

// DocumentChunk represents a chunk of text from a processed attachment.
type DocumentChunk struct {
	ID             string    `json:"id"`
	AttachmentID   string    `json:"attachment_id"`
	ConversationID string    `json:"conversation_id"`
	ChunkIndex     int       `json:"chunk_index"`
	Content        string    `json:"content"`
	CharOffset     int       `json:"char_offset"`
	CharLength     int       `json:"char_length"`
	TokenCount     *int      `json:"token_count,omitempty"`
	MetadataJSON   string    `json:"metadata_json,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// DocumentEmbedding represents a vector embedding for a document chunk.
type DocumentEmbedding struct {
	ChunkID    string    `json:"chunk_id"`
	Embedding  []float32 `json:"-"`
	Model      string    `json:"model"`
	Dimensions int       `json:"dimensions"`
	CreatedAt  time.Time `json:"created_at"`
}

// ToolPermission configures the access policy for a specific tool.
type ToolPermission struct {
	ToolName  string `json:"tool_name"`
	Policy    string `json:"policy"` // "allow", "deny", "ask"
	UpdatedAt string `json:"updated_at,omitempty"`
}

// PricingRule defines cost per token for a provider/model combination.
type PricingRule struct {
	ID                string     `json:"id"`
	ProviderType      string     `json:"provider_type"`
	ModelPattern      string     `json:"model_pattern"`
	InputCostPerMTok  float64    `json:"input_cost_per_mtok"`
	OutputCostPerMTok float64    `json:"output_cost_per_mtok"`
	Currency          string     `json:"currency"`
	EffectiveFrom     *time.Time `json:"effective_from,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

// UsageSummary holds aggregated usage statistics for a given period.
type UsageSummary struct {
	Period         string          `json:"period"`
	TotalInputTok  int64           `json:"total_input_tokens"`
	TotalOutputTok int64           `json:"total_output_tokens"`
	TotalMessages  int             `json:"total_messages"`
	AvgLatencyMs   float64         `json:"avg_latency_ms"`
	EstimatedCost  float64         `json:"estimated_cost"`
	ByProvider     []ProviderUsage `json:"by_provider"`
	ByModel        []ModelUsage    `json:"by_model"`
}

// ProviderUsage is a usage breakdown for a single provider.
type ProviderUsage struct {
	Provider      string  `json:"provider"`
	InputTokens   int64   `json:"input_tokens"`
	OutputTokens  int64   `json:"output_tokens"`
	MessageCount  int     `json:"message_count"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	EstimatedCost float64 `json:"estimated_cost"`
}

// ModelUsage is a usage breakdown for a single model.
type ModelUsage struct {
	Provider      string  `json:"provider"`
	Model         string  `json:"model"`
	InputTokens   int64   `json:"input_tokens"`
	OutputTokens  int64   `json:"output_tokens"`
	MessageCount  int     `json:"message_count"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	EstimatedCost float64 `json:"estimated_cost"`
}

// TemplateVariable defines a variable placeholder in a prompt template.
type TemplateVariable struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Type     string   `json:"type"` // "text" or "select"
	Default  string   `json:"default,omitempty"`
	Required bool     `json:"required,omitempty"`
	Options  []string `json:"options,omitempty"` // for type "select"
}

// PromptTemplate is a reusable, parameterized prompt template.
type PromptTemplate struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	Category     string             `json:"category"`
	TemplateBody string             `json:"template_body"`
	Variables    []TemplateVariable `json:"variables"`
	IsSystem     bool               `json:"is_system"`
	SortOrder    int                `json:"sort_order"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

// AgentRun represents a multi-step agent execution session.
type AgentRun struct {
	ID             string     `json:"id"`
	ConversationID string     `json:"conversation_id"`
	Status         string     `json:"status"` // planning, running, paused, completed, failed, cancelled
	Goal           string     `json:"goal"`
	PlanJSON       string     `json:"plan_json"`
	ResultSummary  string     `json:"result_summary"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

// AgentStep represents a single step in an agent run.
type AgentStep struct {
	ID          string     `json:"id"`
	RunID       string     `json:"run_id"`
	StepIndex   int        `json:"step_index"`
	Type        string     `json:"type"` // think, tool_call, approval, message
	Description string     `json:"description"`
	Status      string     `json:"status"` // pending, running, completed, failed, skipped, awaiting_approval
	InputJSON   string     `json:"input_json"`
	OutputJSON  string     `json:"output_json"`
	ToolName    *string    `json:"tool_name,omitempty"`
	MessageID   *string    `json:"message_id,omitempty"`
	DurationMs  *int       `json:"duration_ms,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Branch represents a conversation branch (fork point).
type Branch struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Name           string    `json:"name"`
	ParentBranch   string    `json:"parent_branch"`
	ForkMessageID  string    `json:"fork_message_id"`
	CreatedAt      time.Time `json:"created_at"`
}

// MessageEmbedding represents a vector embedding for a message (Semantic Search).
type MessageEmbedding struct {
	MessageID  string    `json:"message_id"`
	Embedding  []float32 `json:"-"`
	Model      string    `json:"model"`
	Dimensions int       `json:"dimensions"`
	CreatedAt  time.Time `json:"created_at"`
}

// Workspace represents a project workspace for organizing conversations and templates.
type Workspace struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Color       string    `json:"color"`
	Icon        string    `json:"icon"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WorkspaceStats holds aggregate statistics for a workspace.
type WorkspaceStats struct {
	ConversationCount int `json:"conversation_count"`
	MessageCount      int `json:"message_count"`
	TemplateCount     int `json:"template_count"`
}

// User represents a local user account (collaboration mode).
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	DisplayName  string    `json:"display_name"`
	PasswordHash string    `json:"-"`    // never serialized to JSON
	Role         string    `json:"role"` // admin, member, viewer
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Session represents an authenticated user session.
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// WorkspaceMember represents a user's membership in a workspace.
type WorkspaceMember struct {
	WorkspaceID string    `json:"workspace_id"`
	UserID      string    `json:"user_id"`
	Role        string    `json:"role"` // owner, admin, member, viewer
	JoinedAt    time.Time `json:"joined_at"`
}

// UserWithoutPassword is a view of User safe for JSON responses.
type UserWithoutPassword struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ---------------------------------------------------------------------------
// Plugin SDK Models
// ---------------------------------------------------------------------------

// InstalledPlugin represents a plugin registered in the database.
type InstalledPlugin struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Manifest    string    `json:"manifest"`
	Enabled     bool      `json:"enabled"`
	InstalledAt time.Time `json:"installed_at"`
}

// PluginManifest describes a plugin's capabilities and metadata.
type PluginManifest struct {
	Name         string          `json:"name"`
	Version      string          `json:"version"`
	Description  string          `json:"description"`
	Author       string          `json:"author"`
	Capabilities []string        `json:"capabilities"` // "tool", "provider", "processor"
	Tools        []PluginToolDef `json:"tools,omitempty"`
	Runtime      string          `json:"runtime"`    // "executable", "wasm"
	Entrypoint   string          `json:"entrypoint"` // relative path to binary/script
	Permissions  []string        `json:"permissions,omitempty"`
}

// PluginToolDef describes a tool provided by a plugin.
type PluginToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"` // JSON Schema
}

// ---------------------------------------------------------------------------
// Evaluation Harness Models
// ---------------------------------------------------------------------------

// EvalRun represents the result of running an evaluation suite.
type EvalRun struct {
	ID          string    `json:"id"`
	SuiteName   string    `json:"suite_name"`
	Provider    string    `json:"provider"`
	Model       string    `json:"model"`
	TotalScore  *float64  `json:"total_score,omitempty"`
	ResultsJSON string    `json:"results_json,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// EvalSuite describes a suite of evaluation test cases loaded from JSON.
type EvalSuite struct {
	Name    string     `json:"name"`
	Version string     `json:"version"`
	Cases   []EvalCase `json:"cases"`
}

// EvalCase is a single test case within an eval suite.
type EvalCase struct {
	ID                string             `json:"id"`
	Input             string             `json:"input"`
	ExpectedKeywords  []string           `json:"expected_keywords,omitempty"`
	ExpectedToolCalls []string           `json:"expected_tool_calls,omitempty"`
	Scoring           map[string]float64 `json:"scoring,omitempty"`
}

// EvalCaseResult holds the score for a single evaluated case.
type EvalCaseResult struct {
	CaseID           string             `json:"case_id"`
	Input            string             `json:"input"`
	Response         string             `json:"response"`
	Score            float64            `json:"score"`
	KeywordHits      []string           `json:"keyword_hits,omitempty"`
	KeywordMisses    []string           `json:"keyword_misses,omitempty"`
	ToolCallsMatched []string           `json:"tool_calls_matched,omitempty"`
	Breakdown        map[string]float64 `json:"breakdown,omitempty"`
}

// ── Image Edit Mode ─────────────────────────────────────────────────────

type ImageSession struct {
	ID             string  `json:"id"`
	ConversationID string  `json:"conversation_id"`
	Title          string  `json:"title"`
	ActiveNodeID   *string `json:"active_node_id,omitempty"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

type ImageNode struct {
	ID            string  `json:"id"`
	SessionID     string  `json:"session_id"`
	ParentNodeID  *string `json:"parent_node_id,omitempty"`
	OperationType string  `json:"operation_type"`
	Instruction   string  `json:"instruction"`
	Provider      string  `json:"provider"`
	Model         string  `json:"model"`
	Seed          *int    `json:"seed,omitempty"`
	ParamsJSON    *string `json:"params_json,omitempty"`
	CreatedAt     string  `json:"created_at"`
}

type ImageNodeAsset struct {
	ID           string `json:"id"`
	NodeID       string `json:"node_id"`
	AttachmentID string `json:"attachment_id"`
	VariantIndex int    `json:"variant_index"`
	IsSelected   bool   `json:"is_selected"`
	CreatedAt    string `json:"created_at"`
}

type ImageMask struct {
	ID           string  `json:"id"`
	NodeID       string  `json:"node_id"`
	AttachmentID string  `json:"attachment_id"`
	StrokeJSON   *string `json:"stroke_json,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

type ImageReference struct {
	ID           string `json:"id"`
	NodeID       string `json:"node_id"`
	AttachmentID string `json:"attachment_id"`
	RefRole      string `json:"ref_role"`
	SortOrder    int    `json:"sort_order"`
}
