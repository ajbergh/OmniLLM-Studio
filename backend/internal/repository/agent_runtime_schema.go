package repository

import (
	"database/sql"
	"fmt"
)

// EnsureAgentRuntimeSchema installs additive tables used by the Chat Studio
// agent runtime. These tables are intentionally self-contained and use
// CREATE TABLE IF NOT EXISTS so older local databases can adopt the runtime
// without destructive migration behavior. A future schema-version migration
// can take ownership after the feature contract stabilizes.
func EnsureAgentRuntimeSchema(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS tool_invocations (
			id TEXT PRIMARY KEY,
			tool_call_id TEXT NOT NULL,
			tool_name TEXT NOT NULL,
			tool_version TEXT NOT NULL DEFAULT '1',
			user_id TEXT NOT NULL DEFAULT '',
			workspace_id TEXT NOT NULL DEFAULT '',
			conversation_id TEXT NOT NULL DEFAULT '',
			message_id TEXT NOT NULL DEFAULT '',
			run_id TEXT NOT NULL DEFAULT '',
			arguments_json TEXT NOT NULL DEFAULT '{}',
			status TEXT NOT NULL,
			approval_status TEXT NOT NULL DEFAULT '',
			result_json TEXT NOT NULL DEFAULT '{}',
			error_message TEXT NOT NULL DEFAULT '',
			duration_ms INTEGER NOT NULL DEFAULT 0,
			result_bytes INTEGER NOT NULL DEFAULT 0,
			retry_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			started_at DATETIME,
			completed_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tool_invocations_conversation ON tool_invocations(conversation_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_tool_invocations_run ON tool_invocations(run_id, created_at ASC)`,
		`CREATE INDEX IF NOT EXISTS idx_tool_invocations_user ON tool_invocations(user_id, created_at DESC)`,

		`CREATE TABLE IF NOT EXISTS agent_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			step_id TEXT NOT NULL DEFAULT '',
			event_type TEXT NOT NULL,
			data_json TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_events_run_cursor ON agent_events(run_id, id)`,

		`CREATE TABLE IF NOT EXISTS agent_jobs (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT '',
			workspace_id TEXT NOT NULL DEFAULT '',
			conversation_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			progress REAL NOT NULL DEFAULT 0,
			stage TEXT NOT NULL DEFAULT '',
			request_json TEXT NOT NULL DEFAULT '{}',
			result_json TEXT NOT NULL DEFAULT '{}',
			error_message TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			started_at DATETIME,
			completed_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_jobs_owner ON agent_jobs(user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_jobs_status ON agent_jobs(status, updated_at)`,

		`CREATE TABLE IF NOT EXISTS scheduled_tasks (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT '',
			conversation_id TEXT NOT NULL,
			title TEXT NOT NULL,
			prompt TEXT NOT NULL,
			profile TEXT NOT NULL DEFAULT 'agent',
			timezone TEXT NOT NULL DEFAULT 'UTC',
			schedule_kind TEXT NOT NULL,
			next_run_at DATETIME NOT NULL,
			interval_seconds INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'active',
			last_run_id TEXT NOT NULL DEFAULT '',
			last_error TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_due ON scheduled_tasks(status, next_run_at)`,
		`CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_user ON scheduled_tasks(user_id, created_at DESC)`,

		`CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT '',
			workspace_id TEXT NOT NULL DEFAULT '',
			conversation_id TEXT NOT NULL DEFAULT '',
			kind TEXT NOT NULL DEFAULT 'preference',
			content TEXT NOT NULL,
			source_message_id TEXT NOT NULL DEFAULT '',
			expires_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_memories_user_scope ON memories(user_id, workspace_id, conversation_id, updated_at DESC)`,

		`CREATE TABLE IF NOT EXISTS app_connections (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT '',
			workspace_id TEXT NOT NULL DEFAULT '',
			app_key TEXT NOT NULL,
			display_name TEXT NOT NULL,
			connection_type TEXT NOT NULL,
			server_id TEXT NOT NULL DEFAULT '',
			scopes_json TEXT NOT NULL DEFAULT '[]',
			status TEXT NOT NULL DEFAULT 'configured',
			metadata_json TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, workspace_id, app_key, server_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_app_connections_user ON app_connections(user_id, workspace_id, app_key)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("ensure agent runtime schema: %w", err)
		}
	}
	return nil
}
