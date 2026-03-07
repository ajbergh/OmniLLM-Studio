package db

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/ajbergh/omnillm-studio/internal/crypto"
	_ "github.com/mattn/go-sqlite3"
)

// Migration represents a single versioned schema migration.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// Open initializes a SQLite database connection.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on&_synchronous=NORMAL&_cache_size=-64000&_mmap_size=268435456&_temp_store=MEMORY&_journal_size_limit=67108864",
		path,
	)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	// WAL mode supports 1 writer + N concurrent readers. Allow multiple
	// open connections so reads (SSE streams, analytics, search) don't
	// block behind writes.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	// SQLite is embedded — connections never go stale.
	db.SetConnMaxLifetime(0)

	return db, nil
}

// Close runs PRAGMA optimize (updates query-planner statistics) and then
// closes the database. Call this from main() during graceful shutdown.
func Close(db *sql.DB) error {
	if _, err := db.Exec("PRAGMA optimize"); err != nil {
		log.Printf("[db] PRAGMA optimize failed: %v", err)
	}
	return db.Close()
}

// Migrate runs all schema migrations using a versioned approach.
// It tracks applied migrations in a schema_versions table.
func Migrate(db *sql.DB) error {
	// Bootstrap: create the versioning table and foundation tables.
	// These use CREATE IF NOT EXISTS so they are safe to re-run on existing databases.
	bootstrap := []string{
		migrationSchemaVersions,
		migrationConversations,
		migrationMessages,
		migrationAttachments,
		migrationProviderProfiles,
		migrationSecrets,
		migrationSettings,
		migrationIndexes,
	}
	for i, m := range bootstrap {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("bootstrap migration %d failed: %w", i+1, err)
		}
	}

	// Mark version 1 (foundation) as applied if not already tracked.
	if err := ensureBaseVersion(db); err != nil {
		return fmt.Errorf("ensure base version: %w", err)
	}

	// Run versioned migrations starting from V2 onwards.
	if err := runVersionedMigrations(db); err != nil {
		return fmt.Errorf("versioned migrations: %w", err)
	}

	// Encrypt any existing plaintext API keys
	if err := migrateEncryptSecrets(db); err != nil {
		return fmt.Errorf("encrypt secrets migration: %w", err)
	}

	return nil
}

// versionedMigrations returns all migrations from V2 onwards.
// V1 is the foundation schema (conversations, messages, attachments, providers, secrets, settings).
func versionedMigrations() []Migration {
	return []Migration{
		{Version: 2, Name: "feature_flags", SQL: migrationFeatureFlags},
		{Version: 3, Name: "document_chunks", SQL: migrationDocumentChunks},
		{Version: 4, Name: "document_embeddings", SQL: migrationDocumentEmbeddings},
		{Version: 5, Name: "tool_permissions", SQL: migrationToolPermissions},
		{Version: 6, Name: "pricing_rules", SQL: migrationPricingRules},
		{Version: 7, Name: "prompt_templates", SQL: migrationPromptTemplates},
		{Version: 8, Name: "agent_runs", SQL: migrationAgentRuns},
		{Version: 9, Name: "agent_steps", SQL: migrationAgentSteps},
		{Version: 10, Name: "message_branches", SQL: migrationMessageBranches},
		{Version: 11, Name: "conversation_branches", SQL: migrationConversationBranches},
		{Version: 12, Name: "message_embeddings", SQL: migrationMessageEmbeddings},
		{Version: 13, Name: "workspaces", SQL: migrationWorkspaces},
		{Version: 14, Name: "conversations_workspace", SQL: migrationConversationsWorkspace},
		{Version: 15, Name: "templates_workspace", SQL: migrationTemplatesWorkspace},
		{Version: 16, Name: "users", SQL: migrationUsers},
		{Version: 17, Name: "sessions", SQL: migrationSessions},
		{Version: 18, Name: "workspace_members_and_user_refs", SQL: migrationWorkspaceMembersAndUserRefs},
		{Version: 19, Name: "installed_plugins", SQL: migrationInstalledPlugins},
		{Version: 20, Name: "eval_runs", SQL: migrationEvalRuns},
		{Version: 21, Name: "performance_indexes", SQL: migrationPerformanceIndexes},
		{Version: 22, Name: "agent_runs_awaiting_approval", SQL: migrationAgentRunsAwaitingApproval},
		{Version: 23, Name: "image_sessions_and_nodes", SQL: migrationImageSessionsAndNodes},
		{Version: 24, Name: "image_node_assets_and_references", SQL: migrationImageNodeAssetsAndReferences},
		{Version: 25, Name: "provider_default_image_model", SQL: migrationProviderDefaultImageModel},
		{Version: 26, Name: "conversation_kind", SQL: migrationConversationKind},
	}
}

// ensureBaseVersion marks V1 as applied if schema_versions is empty.
func ensureBaseVersion(db *sql.DB) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_versions").Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		_, err := db.Exec(
			"INSERT INTO schema_versions (version, name) VALUES (1, 'foundation')",
		)
		return err
	}
	return nil
}

// runVersionedMigrations applies all unapplied migrations in version order.
func runVersionedMigrations(db *sql.DB) error {
	var maxVersion int
	if err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_versions").Scan(&maxVersion); err != nil {
		return fmt.Errorf("query max version: %w", err)
	}

	for _, m := range versionedMigrations() {
		if m.Version <= maxVersion {
			continue
		}
		log.Printf("[db] applying migration V%d: %s", m.Version, m.Name)

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin tx for V%d: %w", m.Version, err)
		}

		if _, err := tx.Exec(m.SQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration V%d (%s) failed: %w", m.Version, m.Name, err)
		}

		if _, err := tx.Exec(
			"INSERT INTO schema_versions (version, name) VALUES (?, ?)",
			m.Version, m.Name,
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("record V%d: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit V%d: %w", m.Version, err)
		}
		log.Printf("[db] migration V%d applied successfully", m.Version)
	}

	return nil
}

// SchemaVersion returns the current schema version.
func SchemaVersion(db *sql.DB) (int, error) {
	var v int
	err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_versions").Scan(&v)
	return v, err
}

// migrateEncryptSecrets finds any plaintext (non-encrypted) API keys and encrypts them in place.
func migrateEncryptSecrets(db *sql.DB) error {
	rows, err := db.Query("SELECT provider_profile_id, api_key_encrypted FROM secrets")
	if err != nil {
		return err
	}
	defer rows.Close()

	type secret struct {
		id  string
		key string
	}
	var toEncrypt []secret

	for rows.Next() {
		var s secret
		if err := rows.Scan(&s.id, &s.key); err != nil {
			return err
		}
		if s.key != "" && !crypto.IsEncrypted(s.key) {
			toEncrypt = append(toEncrypt, s)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if len(toEncrypt) == 0 {
		return nil
	}

	log.Printf("[db] encrypting %d plaintext API key(s)...", len(toEncrypt))

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, s := range toEncrypt {
		encrypted, err := crypto.Encrypt(s.key)
		if err != nil {
			return fmt.Errorf("encrypt key for provider %s: %w", s.id, err)
		}
		if _, err := tx.Exec("UPDATE secrets SET api_key_encrypted = ? WHERE provider_profile_id = ?", encrypted, s.id); err != nil {
			return fmt.Errorf("update secret for provider %s: %w", s.id, err)
		}
	}

	return tx.Commit()
}

const migrationConversations = `
CREATE TABLE IF NOT EXISTS conversations (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL DEFAULT 'New Conversation',
	created_at DATETIME NOT NULL DEFAULT (datetime('now')),
	updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
	archived INTEGER NOT NULL DEFAULT 0,
	pinned INTEGER NOT NULL DEFAULT 0,
	default_provider TEXT,
	default_model TEXT,
	system_prompt TEXT,
	metadata_json TEXT DEFAULT '{}'
);
`

const migrationMessages = `
CREATE TABLE IF NOT EXISTS messages (
	id TEXT PRIMARY KEY,
	conversation_id TEXT NOT NULL,
	role TEXT NOT NULL CHECK(role IN ('user','assistant','system','tool')),
	content TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT (datetime('now')),
	provider TEXT,
	model TEXT,
	token_input INTEGER,
	token_output INTEGER,
	latency_ms INTEGER,
	metadata_json TEXT DEFAULT '{}',
	FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
);
`

const migrationAttachments = `
CREATE TABLE IF NOT EXISTS attachments (
	id TEXT PRIMARY KEY,
	conversation_id TEXT NOT NULL,
	message_id TEXT,
	type TEXT NOT NULL CHECK(type IN ('image','file')),
	mime_type TEXT NOT NULL DEFAULT 'application/octet-stream',
	storage_path TEXT NOT NULL,
	bytes INTEGER DEFAULT 0,
	width INTEGER,
	height INTEGER,
	created_at DATETIME NOT NULL DEFAULT (datetime('now')),
	metadata_json TEXT DEFAULT '{}',
	FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
	FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE SET NULL
);
`

const migrationProviderProfiles = `
CREATE TABLE IF NOT EXISTS provider_profiles (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	base_url TEXT,
	default_model TEXT,
	enabled INTEGER NOT NULL DEFAULT 1,
	created_at DATETIME NOT NULL DEFAULT (datetime('now')),
	updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
	metadata_json TEXT DEFAULT '{}'
);
`

const migrationSecrets = `
CREATE TABLE IF NOT EXISTS secrets (
	provider_profile_id TEXT PRIMARY KEY,
	api_key_encrypted TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT (datetime('now')),
	updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
	FOREIGN KEY (provider_profile_id) REFERENCES provider_profiles(id) ON DELETE CASCADE
);
`

const migrationSettings = `
CREATE TABLE IF NOT EXISTS settings (
	key TEXT PRIMARY KEY,
	value_json TEXT NOT NULL DEFAULT '{}'
);
`

const migrationIndexes = `
CREATE INDEX IF NOT EXISTS idx_messages_conversation_id ON messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_attachments_conversation_id ON attachments(conversation_id);
CREATE INDEX IF NOT EXISTS idx_attachments_message_id ON attachments(message_id);
CREATE INDEX IF NOT EXISTS idx_conversations_updated_at ON conversations(updated_at);
CREATE INDEX IF NOT EXISTS idx_conversations_pinned ON conversations(pinned);
`

// ---------------------------------------------------------------------------
// Schema versioning table (bootstrapped before all versioned migrations)
// ---------------------------------------------------------------------------

const migrationSchemaVersions = `
CREATE TABLE IF NOT EXISTS schema_versions (
	version    INTEGER PRIMARY KEY,
	name       TEXT NOT NULL,
	applied_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
`

// ---------------------------------------------------------------------------
// V2: Feature flags
// ---------------------------------------------------------------------------

const migrationFeatureFlags = `
CREATE TABLE IF NOT EXISTS feature_flags (
	key      TEXT PRIMARY KEY,
	enabled  INTEGER NOT NULL DEFAULT 0,
	metadata TEXT DEFAULT '{}'
);
`

// ---------------------------------------------------------------------------
// V3: Document chunks (RAG v1)
// ---------------------------------------------------------------------------

const migrationDocumentChunks = `
CREATE TABLE IF NOT EXISTS document_chunks (
	id              TEXT PRIMARY KEY,
	attachment_id   TEXT NOT NULL,
	conversation_id TEXT NOT NULL,
	chunk_index     INTEGER NOT NULL,
	content         TEXT NOT NULL,
	char_offset     INTEGER NOT NULL DEFAULT 0,
	char_length     INTEGER NOT NULL DEFAULT 0,
	token_count     INTEGER,
	metadata_json   TEXT DEFAULT '{}',
	created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
	FOREIGN KEY (attachment_id) REFERENCES attachments(id) ON DELETE CASCADE,
	FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_chunks_attachment ON document_chunks(attachment_id);
CREATE INDEX IF NOT EXISTS idx_chunks_conversation ON document_chunks(conversation_id);
`

// ---------------------------------------------------------------------------
// V4: Document embeddings (RAG v1)
// ---------------------------------------------------------------------------

const migrationDocumentEmbeddings = `
CREATE TABLE IF NOT EXISTS document_embeddings (
	chunk_id    TEXT PRIMARY KEY,
	embedding   BLOB NOT NULL,
	model       TEXT NOT NULL,
	dimensions  INTEGER NOT NULL,
	created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
	FOREIGN KEY (chunk_id) REFERENCES document_chunks(id) ON DELETE CASCADE
);
`

// ---------------------------------------------------------------------------
// V5: Tool permissions (Tool Calling Framework)
// ---------------------------------------------------------------------------

const migrationToolPermissions = `
CREATE TABLE IF NOT EXISTS tool_permissions (
	tool_name   TEXT PRIMARY KEY,
	policy      TEXT NOT NULL DEFAULT 'allow' CHECK(policy IN ('allow','deny','ask')),
	updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);
`

// ---------------------------------------------------------------------------
// V6: Pricing rules (Usage & Cost Dashboard)
// ---------------------------------------------------------------------------

const migrationPricingRules = `
CREATE TABLE IF NOT EXISTS pricing_rules (
	id                   TEXT PRIMARY KEY,
	provider_type        TEXT NOT NULL,
	model_pattern        TEXT NOT NULL,
	input_cost_per_mtok  REAL NOT NULL DEFAULT 0,
	output_cost_per_mtok REAL NOT NULL DEFAULT 0,
	currency             TEXT NOT NULL DEFAULT 'USD',
	effective_from       DATETIME,
	created_at           DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_pricing_provider ON pricing_rules(provider_type, model_pattern);
`

// V7: Prompt templates (Prompt Templates & Reusable Presets)
const migrationPromptTemplates = `
CREATE TABLE IF NOT EXISTS prompt_templates (
	id              TEXT PRIMARY KEY,
	name            TEXT NOT NULL,
	description     TEXT DEFAULT '',
	category        TEXT DEFAULT 'general',
	template_body   TEXT NOT NULL,
	variables       TEXT DEFAULT '[]',
	is_system       INTEGER NOT NULL DEFAULT 0,
	sort_order      INTEGER DEFAULT 0,
	created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
	updated_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_templates_category ON prompt_templates(category);
`

// V8: Agent runs (Agent Mode)
const migrationAgentRuns = `
CREATE TABLE IF NOT EXISTS agent_runs (
	id              TEXT PRIMARY KEY,
	conversation_id TEXT NOT NULL,
	status          TEXT NOT NULL DEFAULT 'planning'
	                CHECK(status IN ('planning','running','paused','completed','failed','cancelled')),
	goal            TEXT NOT NULL,
	plan_json       TEXT DEFAULT '[]',
	result_summary  TEXT DEFAULT '',
	created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
	updated_at      DATETIME NOT NULL DEFAULT (datetime('now')),
	completed_at    DATETIME,
	FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_agent_runs_conversation ON agent_runs(conversation_id);
`

// V9: Agent steps (Agent Mode)
const migrationAgentSteps = `
CREATE TABLE IF NOT EXISTS agent_steps (
	id              TEXT PRIMARY KEY,
	run_id          TEXT NOT NULL,
	step_index      INTEGER NOT NULL,
	type            TEXT NOT NULL CHECK(type IN ('think','tool_call','approval','message')),
	description     TEXT NOT NULL,
	status          TEXT NOT NULL DEFAULT 'pending'
	                CHECK(status IN ('pending','running','completed','failed','skipped','awaiting_approval')),
	input_json      TEXT DEFAULT '{}',
	output_json     TEXT DEFAULT '{}',
	tool_name       TEXT,
	message_id      TEXT,
	duration_ms     INTEGER,
	created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
	completed_at    DATETIME,
	FOREIGN KEY (run_id) REFERENCES agent_runs(id) ON DELETE CASCADE,
	FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_steps_run ON agent_steps(run_id);
`

// V10: Add branch columns to messages (Conversation Branching)
const migrationMessageBranches = `
ALTER TABLE messages ADD COLUMN branch_id TEXT DEFAULT 'main';
ALTER TABLE messages ADD COLUMN parent_message_id TEXT;
CREATE INDEX IF NOT EXISTS idx_messages_branch ON messages(conversation_id, branch_id);
`

// V11: Conversation branches table (Conversation Branching)
const migrationConversationBranches = `
CREATE TABLE IF NOT EXISTS conversation_branches (
	id              TEXT PRIMARY KEY,
	conversation_id TEXT NOT NULL,
	name            TEXT NOT NULL DEFAULT 'Branch',
	parent_branch   TEXT DEFAULT 'main',
	fork_message_id TEXT NOT NULL,
	created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
	FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
	FOREIGN KEY (fork_message_id) REFERENCES messages(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_branches_conversation ON conversation_branches(conversation_id);
`

// V12: Message embeddings (Semantic Search)
const migrationMessageEmbeddings = `
CREATE TABLE IF NOT EXISTS message_embeddings (
	message_id  TEXT PRIMARY KEY,
	embedding   BLOB NOT NULL,
	model       TEXT NOT NULL,
	dimensions  INTEGER NOT NULL,
	created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
	FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE
);
`

// V13: Workspaces table
const migrationWorkspaces = `
CREATE TABLE IF NOT EXISTS workspaces (
	id          TEXT PRIMARY KEY,
	name        TEXT NOT NULL,
	description TEXT DEFAULT '',
	color       TEXT DEFAULT '#6366f1',
	icon        TEXT DEFAULT 'folder',
	sort_order  INTEGER DEFAULT 0,
	created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
	updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);
`

// V14: Add workspace_id to conversations
const migrationConversationsWorkspace = `
ALTER TABLE conversations ADD COLUMN workspace_id TEXT REFERENCES workspaces(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_conversations_workspace ON conversations(workspace_id);
`

// V15: Add workspace_id to prompt_templates
const migrationTemplatesWorkspace = `
ALTER TABLE prompt_templates ADD COLUMN workspace_id TEXT REFERENCES workspaces(id) ON DELETE SET NULL;
`

// V16: Users (Local Collaboration)
const migrationUsers = `
CREATE TABLE IF NOT EXISTS users (
	id            TEXT PRIMARY KEY,
	username      TEXT NOT NULL UNIQUE,
	display_name  TEXT NOT NULL,
	password_hash TEXT NOT NULL,
	role          TEXT NOT NULL DEFAULT 'member' CHECK(role IN ('admin','member','viewer')),
	created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
	updated_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);
`

// V17: Sessions (Local Collaboration)
const migrationSessions = `
CREATE TABLE IF NOT EXISTS sessions (
	id          TEXT PRIMARY KEY,
	user_id     TEXT NOT NULL,
	token       TEXT NOT NULL UNIQUE,
	expires_at  DATETIME NOT NULL,
	created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
	FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
`

// V18: Workspace members + user references on conversations/messages (Local Collaboration)
const migrationWorkspaceMembersAndUserRefs = `
CREATE TABLE IF NOT EXISTS workspace_members (
	workspace_id TEXT NOT NULL,
	user_id      TEXT NOT NULL,
	role         TEXT NOT NULL DEFAULT 'member' CHECK(role IN ('owner','admin','member','viewer')),
	joined_at    DATETIME NOT NULL DEFAULT (datetime('now')),
	PRIMARY KEY (workspace_id, user_id),
	FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
	FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
ALTER TABLE conversations ADD COLUMN user_id TEXT REFERENCES users(id);
ALTER TABLE messages ADD COLUMN user_id TEXT REFERENCES users(id);
CREATE INDEX IF NOT EXISTS idx_conversations_user ON conversations(user_id);
CREATE INDEX IF NOT EXISTS idx_messages_user ON messages(user_id);
`

// V19: Installed plugins (Plugin SDK)
const migrationInstalledPlugins = `
CREATE TABLE IF NOT EXISTS installed_plugins (
	name         TEXT PRIMARY KEY,
	version      TEXT NOT NULL,
	manifest     TEXT NOT NULL,
	enabled      INTEGER NOT NULL DEFAULT 1,
	installed_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
`

// V20: Eval runs (Evaluation Harness)
const migrationEvalRuns = `
CREATE TABLE IF NOT EXISTS eval_runs (
	id           TEXT PRIMARY KEY,
	suite_name   TEXT NOT NULL,
	provider     TEXT NOT NULL,
	model        TEXT NOT NULL,
	total_score  REAL,
	results_json TEXT,
	created_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);
`

// V21: Performance indexes – fills indexing gaps identified during audit
const migrationPerformanceIndexes = `
-- Branch-only deletes scan the entire messages table without this
CREATE INDEX IF NOT EXISTS idx_messages_branch_id ON messages(branch_id);

-- workspace_members PK is (workspace_id, user_id); queries by user_id alone need a separate index
CREATE INDEX IF NOT EXISTS idx_workspace_members_user ON workspace_members(user_id);

-- prompt_templates filtered/cleared by workspace_id in workspace delete & stats
CREATE INDEX IF NOT EXISTS idx_templates_workspace ON prompt_templates(workspace_id);

-- Expired-session cleanup needs a range scan on expires_at
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);

-- Analytics aggregation filters on role + created_at range
CREATE INDEX IF NOT EXISTS idx_messages_role_created ON messages(role, created_at);

-- Eval runs listed by suite with a created_at sort
CREATE INDEX IF NOT EXISTS idx_eval_runs_suite ON eval_runs(suite_name, created_at DESC);

-- Covering index for the most frequent query: messages by conversation ordered by time
CREATE INDEX IF NOT EXISTS idx_messages_conv_created ON messages(conversation_id, created_at);

-- Covering index for the landing-page conversation list query
CREATE INDEX IF NOT EXISTS idx_conversations_list ON conversations(archived, pinned DESC, updated_at DESC);
`

// V22: Add 'awaiting_approval' to agent_runs status CHECK constraint.
// SQLite doesn't support ALTER TABLE to modify CHECK constraints, so we
// recreate the table with the correct constraint.
const migrationAgentRunsAwaitingApproval = `
CREATE TABLE IF NOT EXISTS agent_runs_new (
	id              TEXT PRIMARY KEY,
	conversation_id TEXT NOT NULL,
	status          TEXT NOT NULL DEFAULT 'planning'
	                CHECK(status IN ('planning','running','awaiting_approval','paused','completed','failed','cancelled')),
	goal            TEXT NOT NULL,
	plan_json       TEXT DEFAULT '[]',
	result_summary  TEXT DEFAULT '',
	created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
	updated_at      DATETIME NOT NULL DEFAULT (datetime('now')),
	completed_at    DATETIME,
	FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
);
INSERT OR IGNORE INTO agent_runs_new SELECT * FROM agent_runs;
DROP TABLE IF EXISTS agent_runs;
ALTER TABLE agent_runs_new RENAME TO agent_runs;
CREATE INDEX IF NOT EXISTS idx_agent_runs_conversation ON agent_runs(conversation_id);
`

const migrationImageSessionsAndNodes = `
CREATE TABLE IF NOT EXISTS image_sessions (
	id              TEXT PRIMARY KEY,
	conversation_id TEXT NOT NULL,
	title           TEXT NOT NULL DEFAULT 'Untitled Session',
	active_node_id  TEXT,
	created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_image_sessions_conversation ON image_sessions(conversation_id);

CREATE TABLE IF NOT EXISTS image_nodes (
	id              TEXT PRIMARY KEY,
	session_id      TEXT NOT NULL,
	parent_node_id  TEXT,
	operation_type  TEXT NOT NULL DEFAULT 'generate',
	instruction     TEXT NOT NULL DEFAULT '',
	provider        TEXT NOT NULL DEFAULT '',
	model           TEXT NOT NULL DEFAULT '',
	seed            INTEGER,
	params_json     TEXT,
	created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (session_id) REFERENCES image_sessions(id) ON DELETE CASCADE,
	FOREIGN KEY (parent_node_id) REFERENCES image_nodes(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_image_nodes_session ON image_nodes(session_id);
CREATE INDEX IF NOT EXISTS idx_image_nodes_parent ON image_nodes(parent_node_id);
`

const migrationImageNodeAssetsAndReferences = `
CREATE TABLE IF NOT EXISTS image_node_assets (
	id            TEXT PRIMARY KEY,
	node_id       TEXT NOT NULL,
	attachment_id TEXT NOT NULL,
	variant_index INTEGER NOT NULL DEFAULT 0,
	is_selected   INTEGER NOT NULL DEFAULT 0,
	created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (node_id) REFERENCES image_nodes(id) ON DELETE CASCADE,
	FOREIGN KEY (attachment_id) REFERENCES attachments(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_image_node_assets_node ON image_node_assets(node_id);

CREATE TABLE IF NOT EXISTS image_masks (
	id            TEXT PRIMARY KEY,
	node_id       TEXT NOT NULL,
	attachment_id TEXT NOT NULL,
	stroke_json   TEXT,
	created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (node_id) REFERENCES image_nodes(id) ON DELETE CASCADE,
	FOREIGN KEY (attachment_id) REFERENCES attachments(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS image_references (
	id            TEXT PRIMARY KEY,
	node_id       TEXT NOT NULL,
	attachment_id TEXT NOT NULL,
	ref_role      TEXT NOT NULL DEFAULT 'content',
	sort_order    INTEGER NOT NULL DEFAULT 0,
	FOREIGN KEY (node_id) REFERENCES image_nodes(id) ON DELETE CASCADE,
	FOREIGN KEY (attachment_id) REFERENCES attachments(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_image_references_node ON image_references(node_id);
`

// V25: Provider-level default image generation model
const migrationProviderDefaultImageModel = `
ALTER TABLE provider_profiles ADD COLUMN default_image_model TEXT;
`

// V26: Distinguish chat conversations from image-studio backing conversations.
const migrationConversationKind = `
ALTER TABLE conversations ADD COLUMN kind TEXT NOT NULL DEFAULT 'chat' CHECK(kind IN ('chat','image'));

CREATE INDEX IF NOT EXISTS idx_conversations_kind ON conversations(kind);

UPDATE conversations
SET kind = 'image'
WHERE id IN (
	SELECT DISTINCT s.conversation_id
	FROM image_sessions s
)
AND NOT EXISTS (
	SELECT 1
	FROM messages m
	WHERE m.conversation_id = conversations.id
);
`
