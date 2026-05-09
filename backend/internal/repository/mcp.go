package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/crypto"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// MCPServerRepo handles MCP server configuration and audit records.
type MCPServerRepo struct {
	db *sql.DB
}

// NewMCPServerRepo creates a new MCPServerRepo.
func NewMCPServerRepo(db *sql.DB) *MCPServerRepo {
	return &MCPServerRepo{db: db}
}

// CreateMCPServerInput is the request shape for creating an MCP server.
type CreateMCPServerInput struct {
	Name        string            `json:"name"`
	Transport   string            `json:"transport"`
	Command     *string           `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	URL         *string           `json:"url,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Enabled     bool              `json:"enabled"`
	WorkspaceID *string           `json:"workspace_id,omitempty"`
}

// UpdateMCPServerInput is the request shape for updating an MCP server.
type UpdateMCPServerInput struct {
	Name        *string            `json:"name,omitempty"`
	Transport   *string            `json:"transport,omitempty"`
	Command     *string            `json:"command,omitempty"`
	Args        *[]string          `json:"args,omitempty"`
	URL         *string            `json:"url,omitempty"`
	Env         *map[string]string `json:"env,omitempty"`
	Headers     *map[string]string `json:"headers,omitempty"`
	Enabled     *bool              `json:"enabled,omitempty"`
	WorkspaceID *string            `json:"workspace_id,omitempty"`
}

// MCPAuditFilter controls audit event listing.
type MCPAuditFilter struct {
	ServerID string
	Limit    int
}

// List returns configured MCP servers with secret values redacted.
func (r *MCPServerRepo) List() ([]models.MCPServer, error) {
	rows, err := r.db.Query(`
		SELECT id, name, transport, command, args_json, url, env_json, headers_json, enabled, workspace_id, created_at, updated_at
		FROM mcp_servers
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list mcp servers: %w", err)
	}
	defer rows.Close()

	var servers []models.MCPServer
	for rows.Next() {
		server, err := scanMCPServer(rows, false)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	return servers, rows.Err()
}

// ListRuntime returns configured MCP servers with decrypted environment values.
func (r *MCPServerRepo) ListRuntime() ([]models.MCPServer, error) {
	rows, err := r.db.Query(`
		SELECT id, name, transport, command, args_json, url, env_json, headers_json, enabled, workspace_id, created_at, updated_at
		FROM mcp_servers
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list mcp runtime servers: %w", err)
	}
	defer rows.Close()

	var servers []models.MCPServer
	for rows.Next() {
		server, err := scanMCPServer(rows, true)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	return servers, rows.Err()
}

// GetByID returns a configured MCP server with secret values redacted.
func (r *MCPServerRepo) GetByID(id string) (*models.MCPServer, error) {
	return r.getByID(id, false)
}

// GetRuntimeByID returns a configured MCP server with decrypted environment values.
func (r *MCPServerRepo) GetRuntimeByID(id string) (*models.MCPServer, error) {
	return r.getByID(id, true)
}

func (r *MCPServerRepo) getByID(id string, decryptEnv bool) (*models.MCPServer, error) {
	row := r.db.QueryRow(`
		SELECT id, name, transport, command, args_json, url, env_json, headers_json, enabled, workspace_id, created_at, updated_at
		FROM mcp_servers
		WHERE id = ?
	`, id)
	server, err := scanMCPServer(row, decryptEnv)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get mcp server: %w", err)
	}
	return &server, nil
}

// Create inserts a new MCP server configuration.
func (r *MCPServerRepo) Create(input CreateMCPServerInput) (*models.MCPServer, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	transport := strings.TrimSpace(input.Transport)
	if transport == "" {
		transport = "stdio"
	}

	argsJSON, err := marshalStringSlice(input.Args)
	if err != nil {
		return nil, err
	}
	envJSON, err := marshalEncryptedEnv(input.Env)
	if err != nil {
		return nil, err
	}
	headersJSON, err := marshalEncryptedEnv(input.Headers)
	if err != nil {
		return nil, err
	}

	enabled := 0
	if input.Enabled {
		enabled = 1
	}

	_, err = r.db.Exec(`
		INSERT INTO mcp_servers (id, name, transport, command, args_json, url, env_json, headers_json, enabled, workspace_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, strings.TrimSpace(input.Name), transport, nullableString(input.Command), argsJSON, nullableString(input.URL), envJSON, headersJSON, enabled, input.WorkspaceID, now, now)
	if err != nil {
		return nil, fmt.Errorf("create mcp server: %w", err)
	}
	return r.GetByID(id)
}

// Update modifies an MCP server configuration.
func (r *MCPServerRepo) Update(id string, input UpdateMCPServerInput) (*models.MCPServer, error) {
	sets := []string{}
	args := []interface{}{}

	if input.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, strings.TrimSpace(*input.Name))
	}
	if input.Transport != nil {
		sets = append(sets, "transport = ?")
		args = append(args, strings.TrimSpace(*input.Transport))
	}
	if input.Command != nil {
		sets = append(sets, "command = ?")
		args = append(args, nullableString(input.Command))
	}
	if input.Args != nil {
		argsJSON, err := marshalStringSlice(*input.Args)
		if err != nil {
			return nil, err
		}
		sets = append(sets, "args_json = ?")
		args = append(args, argsJSON)
	}
	if input.URL != nil {
		sets = append(sets, "url = ?")
		args = append(args, nullableString(input.URL))
	}
	if input.Env != nil {
		envJSON, err := marshalEncryptedEnv(*input.Env)
		if err != nil {
			return nil, err
		}
		sets = append(sets, "env_json = ?")
		args = append(args, envJSON)
	}
	if input.Headers != nil {
		headersJSON, err := marshalEncryptedEnv(*input.Headers)
		if err != nil {
			return nil, err
		}
		sets = append(sets, "headers_json = ?")
		args = append(args, headersJSON)
	}
	if input.Enabled != nil {
		sets = append(sets, "enabled = ?")
		if *input.Enabled {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	if input.WorkspaceID != nil {
		sets = append(sets, "workspace_id = ?")
		args = append(args, input.WorkspaceID)
	}

	if len(sets) == 0 {
		return r.GetByID(id)
	}

	sets = append(sets, "updated_at = ?")
	args = append(args, time.Now().UTC(), id)

	query := "UPDATE mcp_servers SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	res, err := r.db.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("update mcp server: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, nil
	}
	return r.GetByID(id)
}

// Delete removes an MCP server configuration.
func (r *MCPServerRepo) Delete(id string) error {
	res, err := r.db.Exec("DELETE FROM mcp_servers WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete mcp server: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// InsertAudit stores an MCP audit event.
func (r *MCPServerRepo) InsertAudit(event models.MCPAuditEvent) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	_, err := r.db.Exec(`
		INSERT INTO mcp_audit_log
			(id, server_id, event_type, tool_name, input_json, output_json, duration_ms, error_msg, user_id, workspace_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.ID, event.ServerID, event.EventType, event.ToolName, event.InputJSON, event.OutputJSON, event.DurationMs, event.ErrorMsg, event.UserID, event.WorkspaceID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("insert mcp audit: %w", err)
	}
	return nil
}

// ListAudit returns recent MCP audit events, newest first.
func (r *MCPServerRepo) ListAudit(filter MCPAuditFilter) ([]models.MCPAuditEvent, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	query := `
		SELECT id, server_id, event_type, tool_name, input_json, output_json, duration_ms, error_msg, user_id, workspace_id, created_at
		FROM mcp_audit_log
	`
	args := []interface{}{}
	if strings.TrimSpace(filter.ServerID) != "" {
		query += " WHERE server_id = ?"
		args = append(args, strings.TrimSpace(filter.ServerID))
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list mcp audit: %w", err)
	}
	defer rows.Close()

	events := []models.MCPAuditEvent{}
	for rows.Next() {
		event, err := scanMCPAuditEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

type mcpServerScanner interface {
	Scan(dest ...interface{}) error
}

type mcpAuditScanner interface {
	Scan(dest ...interface{}) error
}

func scanMCPServer(scanner mcpServerScanner, decryptEnv bool) (models.MCPServer, error) {
	var server models.MCPServer
	var command sql.NullString
	var url sql.NullString
	var workspaceID sql.NullString
	var argsJSON string
	var envJSON string
	var headersJSON string
	var enabled int

	if err := scanner.Scan(
		&server.ID,
		&server.Name,
		&server.Transport,
		&command,
		&argsJSON,
		&url,
		&envJSON,
		&headersJSON,
		&enabled,
		&workspaceID,
		&server.CreatedAt,
		&server.UpdatedAt,
	); err != nil {
		return server, err
	}

	if command.Valid {
		server.Command = &command.String
	}
	if url.Valid {
		server.URL = &url.String
	}
	if workspaceID.Valid {
		server.WorkspaceID = &workspaceID.String
	}
	server.Enabled = enabled != 0

	args, err := unmarshalStringSlice(argsJSON)
	if err != nil {
		return server, fmt.Errorf("decode mcp args: %w", err)
	}
	server.Args = args

	env, err := unmarshalEnv(envJSON, decryptEnv)
	if err != nil {
		return server, fmt.Errorf("decode mcp env: %w", err)
	}
	if decryptEnv {
		server.Env = env
	} else {
		server.EnvKeys = envKeys(env)
	}

	headers, err := unmarshalEnv(headersJSON, decryptEnv)
	if err != nil {
		return server, fmt.Errorf("decode mcp headers: %w", err)
	}
	if decryptEnv {
		server.Headers = headers
	} else {
		server.HeaderKeys = envKeys(headers)
	}

	return server, nil
}

func marshalStringSlice(values []string) (string, error) {
	if values == nil {
		values = []string{}
	}
	data, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshal string slice: %w", err)
	}
	return string(data), nil
}

func unmarshalStringSlice(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, err
	}
	if values == nil {
		return []string{}, nil
	}
	return values, nil
}

func marshalEncryptedEnv(env map[string]string) (string, error) {
	if env == nil {
		env = map[string]string{}
	}
	encrypted := make(map[string]string, len(env))
	for key, value := range env {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		enc, err := crypto.Encrypt(value)
		if err != nil {
			return "", fmt.Errorf("encrypt env %s: %w", key, err)
		}
		encrypted[key] = enc
	}
	data, err := json.Marshal(encrypted)
	if err != nil {
		return "", fmt.Errorf("marshal env: %w", err)
	}
	return string(data), nil
}

func unmarshalEnv(raw string, decrypt bool) (map[string]string, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]string{}, nil
	}
	var env map[string]string
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return nil, err
	}
	if env == nil {
		return map[string]string{}, nil
	}
	if !decrypt {
		return env, nil
	}
	out := make(map[string]string, len(env))
	for key, value := range env {
		plain, err := crypto.Decrypt(value)
		if err != nil {
			return nil, fmt.Errorf("decrypt env %s: %w", key, err)
		}
		out[key] = plain
	}
	return out, nil
}

func envKeys(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func nullableString(value *string) interface{} {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func scanMCPAuditEvent(scanner mcpAuditScanner) (models.MCPAuditEvent, error) {
	var event models.MCPAuditEvent
	var toolName sql.NullString
	var inputJSON sql.NullString
	var outputJSON sql.NullString
	var durationMs sql.NullInt64
	var errorMsg sql.NullString
	var userID sql.NullString
	var workspaceID sql.NullString

	if err := scanner.Scan(
		&event.ID,
		&event.ServerID,
		&event.EventType,
		&toolName,
		&inputJSON,
		&outputJSON,
		&durationMs,
		&errorMsg,
		&userID,
		&workspaceID,
		&event.CreatedAt,
	); err != nil {
		return event, err
	}

	if toolName.Valid {
		event.ToolName = &toolName.String
	}
	if inputJSON.Valid {
		event.InputJSON = &inputJSON.String
	}
	if outputJSON.Valid {
		event.OutputJSON = &outputJSON.String
	}
	if durationMs.Valid {
		value := int(durationMs.Int64)
		event.DurationMs = &value
	}
	if errorMsg.Valid {
		event.ErrorMsg = &errorMsg.String
	}
	if userID.Valid {
		event.UserID = &userID.String
	}
	if workspaceID.Valid {
		event.WorkspaceID = &workspaceID.String
	}

	return event, nil
}
