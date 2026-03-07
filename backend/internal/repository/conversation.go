package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// ConversationRepo handles conversation CRUD operations.
type ConversationRepo struct {
	db *sql.DB
}

// NewConversationRepo creates a new ConversationRepo.
func NewConversationRepo(db *sql.DB) *ConversationRepo {
	return &ConversationRepo{db: db}
}

func normalizeConversationKind(kind string) (string, error) {
	switch kind {
	case "", models.ConversationKindChat:
		return models.ConversationKindChat, nil
	case models.ConversationKindImage:
		return models.ConversationKindImage, nil
	default:
		return "", fmt.Errorf("invalid conversation kind: %s", kind)
	}
}

func (r *ConversationRepo) list(userID string, includeArchived bool, kind *string, workspaceID ...string) ([]models.Conversation, error) {
	query := `
		SELECT id, title, created_at, updated_at, archived, pinned,
		       default_provider, default_model, system_prompt, kind, metadata_json,
		       workspace_id, user_id
		FROM conversations
	`
	var conditions []string
	var args []interface{}

	if userID != "" {
		conditions = append(conditions, "user_id = ?")
		args = append(args, userID)
	}
	if !includeArchived {
		conditions = append(conditions, "archived = 0")
	}
	if kind != nil && *kind != "" {
		conditions = append(conditions, "kind = ?")
		args = append(args, *kind)
	}
	if len(workspaceID) > 0 && workspaceID[0] != "" {
		conditions = append(conditions, "workspace_id = ?")
		args = append(args, workspaceID[0])
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY pinned DESC, updated_at DESC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()

	var convos []models.Conversation
	for rows.Next() {
		var c models.Conversation
		var archived, pinned int
		if err := rows.Scan(
			&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt,
			&archived, &pinned,
			&c.DefaultProvider, &c.DefaultModel, &c.SystemPrompt, &c.Kind, &c.MetadataJSON,
			&c.WorkspaceID, &c.UserID,
		); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		c.Archived = archived != 0
		c.Pinned = pinned != 0
		convos = append(convos, c)
	}
	return convos, rows.Err()
}

// List returns chat conversations ordered by pinned first, then by updated_at desc.
// When userID is non-empty, only conversations belonging to that user are returned.
// Optional workspaceID filters to a specific workspace.
func (r *ConversationRepo) List(userID string, includeArchived bool, workspaceID ...string) ([]models.Conversation, error) {
	kind := models.ConversationKindChat
	return r.list(userID, includeArchived, &kind, workspaceID...)
}

// ListAll returns conversations of all kinds.
func (r *ConversationRepo) ListAll(userID string, includeArchived bool, workspaceID ...string) ([]models.Conversation, error) {
	return r.list(userID, includeArchived, nil, workspaceID...)
}

// ListByKind returns conversations for a specific studio kind.
func (r *ConversationRepo) ListByKind(userID string, includeArchived bool, kind string, workspaceID ...string) ([]models.Conversation, error) {
	normalizedKind, err := normalizeConversationKind(kind)
	if err != nil {
		return nil, err
	}
	return r.list(userID, includeArchived, &normalizedKind, workspaceID...)
}

// GetByID retrieves a single conversation by ID.
func (r *ConversationRepo) GetByID(id string) (*models.Conversation, error) {
	return r.GetByIDForUser(id, "")
}

// GetByIDForUser retrieves a conversation by ID, optionally scoped to a user.
// When userID is non-empty, the conversation must belong to that user.
func (r *ConversationRepo) GetByIDForUser(id, userID string) (*models.Conversation, error) {
	query := `
		SELECT id, title, created_at, updated_at, archived, pinned,
		       default_provider, default_model, system_prompt, kind, metadata_json,
		       workspace_id, user_id
		FROM conversations WHERE id = ?
	`
	args := []interface{}{id}
	if userID != "" {
		query += " AND user_id = ?"
		args = append(args, userID)
	}
	var c models.Conversation
	var archived, pinned int
	err := r.db.QueryRow(query, args...).Scan(
		&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt,
		&archived, &pinned,
		&c.DefaultProvider, &c.DefaultModel, &c.SystemPrompt, &c.Kind, &c.MetadataJSON,
		&c.WorkspaceID, &c.UserID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	c.Archived = archived != 0
	c.Pinned = pinned != 0
	return &c, nil
}

// Create inserts a new conversation.
func (r *ConversationRepo) Create(userID, title string, defaultProvider, defaultModel, systemPrompt *string, workspaceID ...*string) (*models.Conversation, error) {
	return r.CreateWithKind(userID, title, models.ConversationKindChat, defaultProvider, defaultModel, systemPrompt, workspaceID...)
}

// CreateWithKind inserts a new conversation for a specific studio kind.
func (r *ConversationRepo) CreateWithKind(userID, title, kind string, defaultProvider, defaultModel, systemPrompt *string, workspaceID ...*string) (*models.Conversation, error) {
	normalizedKind, err := normalizeConversationKind(kind)
	if err != nil {
		return nil, err
	}

	id := uuid.New().String()
	now := time.Now().UTC()

	var wsID *string
	if len(workspaceID) > 0 {
		wsID = workspaceID[0]
	}

	var uid *string
	if userID != "" {
		uid = &userID
	}

	_, err = r.db.Exec(`
		INSERT INTO conversations (id, title, created_at, updated_at, default_provider, default_model, system_prompt, kind, workspace_id, user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, title, now, now, defaultProvider, defaultModel, systemPrompt, normalizedKind, wsID, uid)
	if err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}

	return r.GetByID(id)
}

// Update modifies an existing conversation (title, pinned, archived, defaults, workspace).
type ConversationUpdate struct {
	Title           *string `json:"title,omitempty"`
	Pinned          *bool   `json:"pinned,omitempty"`
	Archived        *bool   `json:"archived,omitempty"`
	DefaultProvider *string `json:"default_provider,omitempty"`
	DefaultModel    *string `json:"default_model,omitempty"`
	SystemPrompt    *string `json:"system_prompt,omitempty"`
	WorkspaceID     *string `json:"workspace_id,omitempty"`
}

func (r *ConversationRepo) Update(id, userID string, upd ConversationUpdate) (*models.Conversation, error) {
	sets := []string{}
	args := []interface{}{}

	if upd.Title != nil {
		sets = append(sets, "title = ?")
		args = append(args, *upd.Title)
	}
	if upd.Pinned != nil {
		sets = append(sets, "pinned = ?")
		if *upd.Pinned {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	if upd.Archived != nil {
		sets = append(sets, "archived = ?")
		if *upd.Archived {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	if upd.DefaultProvider != nil {
		sets = append(sets, "default_provider = ?")
		args = append(args, *upd.DefaultProvider)
	}
	if upd.DefaultModel != nil {
		sets = append(sets, "default_model = ?")
		args = append(args, *upd.DefaultModel)
	}
	if upd.SystemPrompt != nil {
		sets = append(sets, "system_prompt = ?")
		args = append(args, *upd.SystemPrompt)
	}
	if upd.WorkspaceID != nil {
		sets = append(sets, "workspace_id = ?")
		args = append(args, *upd.WorkspaceID)
	}

	if len(sets) == 0 {
		return r.GetByIDForUser(id, userID)
	}

	sets = append(sets, "updated_at = ?")
	args = append(args, time.Now().UTC())
	args = append(args, id)

	query := "UPDATE conversations SET "
	for i, s := range sets {
		if i > 0 {
			query += ", "
		}
		query += s
	}
	query += " WHERE id = ?"
	if userID != "" {
		query += " AND user_id = ?"
		args = append(args, userID)
	}

	_, err := r.db.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("update conversation: %w", err)
	}
	return r.GetByIDForUser(id, userID)
}

// Delete removes a conversation and cascades to messages/attachments.
// When userID is non-empty, only deletes if the conversation belongs to that user.
func (r *ConversationRepo) Delete(id, userID string) error {
	query := "DELETE FROM conversations WHERE id = ?"
	args := []interface{}{id}
	if userID != "" {
		query += " AND user_id = ?"
		args = append(args, userID)
	}
	_, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	return nil
}

// Search performs full-text search across conversation titles and messages.
// When userID is non-empty, results are scoped to that user.
func (r *ConversationRepo) Search(userID, query string) ([]models.Conversation, error) {
	return r.SearchByKind(userID, query, models.ConversationKindChat)
}

// SearchByKind performs full-text search across conversations of a single kind.
func (r *ConversationRepo) SearchByKind(userID, query, kind string) ([]models.Conversation, error) {
	normalizedKind, err := normalizeConversationKind(kind)
	if err != nil {
		return nil, err
	}

	sqlQuery := `
		SELECT DISTINCT c.id, c.title, c.created_at, c.updated_at, c.archived, c.pinned,
		       c.default_provider, c.default_model, c.system_prompt, c.kind, c.metadata_json,
		       c.workspace_id, c.user_id
		FROM conversations c
		LEFT JOIN messages m ON m.conversation_id = c.id
		WHERE (c.title LIKE ? ESCAPE '\\' OR m.content LIKE ? ESCAPE '\\')
		  AND c.kind = ?
	`
	// Escape SQL wildcard characters in user input
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
	pattern := "%" + escaped + "%"
	args := []interface{}{pattern, pattern, normalizedKind}
	if userID != "" {
		sqlQuery += " AND c.user_id = ?"
		args = append(args, userID)
	}
	sqlQuery += `
		ORDER BY c.updated_at DESC
		LIMIT 100
	`
	rows, err := r.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("search conversations: %w", err)
	}
	defer rows.Close()

	var convos []models.Conversation
	for rows.Next() {
		var c models.Conversation
		var archived, pinned int
		if err := rows.Scan(
			&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt,
			&archived, &pinned,
			&c.DefaultProvider, &c.DefaultModel, &c.SystemPrompt, &c.Kind, &c.MetadataJSON,
			&c.WorkspaceID, &c.UserID,
		); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		c.Archived = archived != 0
		c.Pinned = pinned != 0
		convos = append(convos, c)
	}
	return convos, rows.Err()
}

// TouchUpdatedAt bumps the updated_at timestamp for a conversation.
func (r *ConversationRepo) TouchUpdatedAt(id string) error {
	_, err := r.db.Exec("UPDATE conversations SET updated_at = ? WHERE id = ?", time.Now().UTC(), id)
	return err
}
