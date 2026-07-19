// Package memory provides explicit, user-controlled long-term memory.
package memory

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	KindPreference         = "preference"
	KindWorkspaceKnowledge = "workspace_knowledge"
	KindProjectDecision    = "project_decision"
	KindConversation       = "conversation"
	KindTemporary          = "temporary"
)

var validKinds = map[string]bool{
	KindPreference: true, KindWorkspaceKnowledge: true, KindProjectDecision: true,
	KindConversation: true, KindTemporary: true,
}

// Scope defines who owns a memory and where it can be recalled.
type Scope struct {
	UserID         string `json:"user_id,omitempty"`
	WorkspaceID    string `json:"workspace_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
}

// Memory is a visible, editable long-term fact or preference.
type Memory struct {
	ID              string     `json:"id"`
	UserID          string     `json:"user_id,omitempty"`
	WorkspaceID     string     `json:"workspace_id,omitempty"`
	ConversationID  string     `json:"conversation_id,omitempty"`
	Kind            string     `json:"kind"`
	Content         string     `json:"content"`
	SourceMessageID string     `json:"source_message_id,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type Service struct{ db *sql.DB }

func NewService(db *sql.DB) *Service { return &Service{db: db} }

var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(api[_ -]?key|secret|password|passwd|private[_ -]?key|bearer[_ -]?token|access[_ -]?token|refresh[_ -]?token)\b\s*[:=]\s*\S+`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`),
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`),
	regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)\b\d{3}-\d{2}-\d{4}\b`),
}

func (s *Service) Save(scope Scope, kind, content, sourceMessageID string, expiresAt *time.Time) (*Memory, error) {
	content = strings.TrimSpace(content)
	if scope.UserID == "" {
		return nil, fmt.Errorf("user scope is required")
	}
	if content == "" {
		return nil, fmt.Errorf("memory content is required")
	}
	if len(content) > 4000 {
		return nil, fmt.Errorf("memory content exceeds 4000 characters")
	}
	if kind == "" {
		kind = KindPreference
	}
	if !validKinds[kind] {
		return nil, fmt.Errorf("unsupported memory kind %q", kind)
	}
	for _, pattern := range sensitivePatterns {
		if pattern.MatchString(content) {
			return nil, fmt.Errorf("memory appears to contain a credential or highly sensitive identifier and was not saved")
		}
	}
	if kind == KindWorkspaceKnowledge || kind == KindProjectDecision {
		if scope.WorkspaceID == "" {
			return nil, fmt.Errorf("workspace_id is required for %s memory", kind)
		}
		scope.ConversationID = ""
	}
	if kind == KindConversation || kind == KindTemporary {
		if scope.ConversationID == "" {
			return nil, fmt.Errorf("conversation_id is required for %s memory", kind)
		}
	}
	if kind == KindPreference {
		scope.WorkspaceID = ""
		scope.ConversationID = ""
	}
	if kind == KindTemporary && expiresAt == nil {
		defaultExpiry := time.Now().UTC().Add(7 * 24 * time.Hour)
		expiresAt = &defaultExpiry
	}
	now := time.Now().UTC()
	memory := &Memory{
		ID: uuid.NewString(), UserID: scope.UserID, WorkspaceID: scope.WorkspaceID,
		ConversationID: scope.ConversationID, Kind: kind, Content: content,
		SourceMessageID: sourceMessageID, ExpiresAt: expiresAt, CreatedAt: now, UpdatedAt: now,
	}
	_, err := s.db.Exec(`
		INSERT INTO memories (id, user_id, workspace_id, conversation_id, kind, content, source_message_id, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, memory.ID, memory.UserID, memory.WorkspaceID, memory.ConversationID, memory.Kind,
		memory.Content, memory.SourceMessageID, memory.ExpiresAt, memory.CreatedAt, memory.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("save memory: %w", err)
	}
	return memory, nil
}

func (s *Service) List(scope Scope, query string, limit int) ([]Memory, error) {
	if scope.UserID == "" {
		return []Memory{}, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	_, _ = s.db.Exec(`DELETE FROM memories WHERE expires_at IS NOT NULL AND expires_at <= ?`, time.Now().UTC())
	like := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
	rows, err := s.db.Query(`
		SELECT id, user_id, workspace_id, conversation_id, kind, content, source_message_id,
			expires_at, created_at, updated_at
		FROM memories
		WHERE user_id = ?
		  AND (expires_at IS NULL OR expires_at > ?)
		  AND (? = '%%' OR lower(content) LIKE ? OR lower(kind) LIKE ?)
		  AND (
			(conversation_id = '' AND workspace_id = '')
			OR (? != '' AND workspace_id = ? AND conversation_id = '')
			OR (? != '' AND conversation_id = ?)
		  )
		ORDER BY CASE kind WHEN 'conversation' THEN 0 WHEN 'project_decision' THEN 1 WHEN 'workspace_knowledge' THEN 2 ELSE 3 END,
			updated_at DESC
		LIMIT ?
	`, scope.UserID, time.Now().UTC(), like, like, like,
		scope.WorkspaceID, scope.WorkspaceID, scope.ConversationID, scope.ConversationID, limit)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()
	out := make([]Memory, 0)
	for rows.Next() {
		var memory Memory
		if err := rows.Scan(&memory.ID, &memory.UserID, &memory.WorkspaceID, &memory.ConversationID,
			&memory.Kind, &memory.Content, &memory.SourceMessageID, &memory.ExpiresAt,
			&memory.CreatedAt, &memory.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		out = append(out, memory)
	}
	return out, rows.Err()
}

func (s *Service) Get(id, userID string) (*Memory, error) {
	var memory Memory
	err := s.db.QueryRow(`
		SELECT id, user_id, workspace_id, conversation_id, kind, content, source_message_id,
			expires_at, created_at, updated_at
		FROM memories WHERE id = ? AND user_id = ?
	`, id, userID).Scan(&memory.ID, &memory.UserID, &memory.WorkspaceID, &memory.ConversationID,
		&memory.Kind, &memory.Content, &memory.SourceMessageID, &memory.ExpiresAt,
		&memory.CreatedAt, &memory.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get memory: %w", err)
	}
	return &memory, nil
}

func (s *Service) Update(id, userID, content string, expiresAt *time.Time) (*Memory, error) {
	existing, err := s.Get(id, userID)
	if err != nil || existing == nil {
		return nil, fmt.Errorf("memory not found")
	}
	content = strings.TrimSpace(content)
	if content == "" || len(content) > 4000 {
		return nil, fmt.Errorf("memory content must be between 1 and 4000 characters")
	}
	for _, pattern := range sensitivePatterns {
		if pattern.MatchString(content) {
			return nil, fmt.Errorf("memory appears to contain a credential or highly sensitive identifier")
		}
	}
	_, err = s.db.Exec(`UPDATE memories SET content = ?, expires_at = ?, updated_at = ? WHERE id = ? AND user_id = ?`, content, expiresAt, time.Now().UTC(), id, userID)
	if err != nil {
		return nil, fmt.Errorf("update memory: %w", err)
	}
	return s.Get(id, userID)
}

func (s *Service) Delete(id, userID string) error {
	result, err := s.db.Exec(`DELETE FROM memories WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return fmt.Errorf("memory not found")
	}
	return nil
}
