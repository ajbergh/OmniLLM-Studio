package repository

import (
	"database/sql"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// BranchRepo handles CRUD operations for conversation branches.
type BranchRepo struct {
	db *sql.DB
}

// NewBranchRepo creates a new BranchRepo.
func NewBranchRepo(db *sql.DB) *BranchRepo {
	return &BranchRepo{db: db}
}

// Create inserts a new branch record.
func (r *BranchRepo) Create(conversationID, name, parentBranch, forkMessageID string) (*models.Branch, error) {
	b := &models.Branch{
		ID:             uuid.New().String(),
		ConversationID: conversationID,
		Name:           name,
		ParentBranch:   parentBranch,
		ForkMessageID:  forkMessageID,
		CreatedAt:      time.Now().UTC(),
	}

	_, err := r.db.Exec(
		`INSERT INTO conversation_branches (id, conversation_id, name, parent_branch, fork_message_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		b.ID, b.ConversationID, b.Name, b.ParentBranch, b.ForkMessageID, b.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// GetByID retrieves a single branch.
func (r *BranchRepo) GetByID(id string) (*models.Branch, error) {
	var b models.Branch
	err := r.db.QueryRow(
		`SELECT id, conversation_id, name, parent_branch, fork_message_id, created_at
		 FROM conversation_branches WHERE id = ?`, id,
	).Scan(&b.ID, &b.ConversationID, &b.Name, &b.ParentBranch, &b.ForkMessageID, &b.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// List returns all branches for a conversation.
func (r *BranchRepo) List(conversationID string) ([]models.Branch, error) {
	rows, err := r.db.Query(
		`SELECT id, conversation_id, name, parent_branch, fork_message_id, created_at
		 FROM conversation_branches WHERE conversation_id = ? ORDER BY created_at ASC`, conversationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	branches := make([]models.Branch, 0)
	for rows.Next() {
		var b models.Branch
		if err := rows.Scan(&b.ID, &b.ConversationID, &b.Name, &b.ParentBranch, &b.ForkMessageID, &b.CreatedAt); err != nil {
			return nil, err
		}
		branches = append(branches, b)
	}
	return branches, rows.Err()
}

// Rename updates a branch's name.
func (r *BranchRepo) Rename(id, name string) error {
	_, err := r.db.Exec(`UPDATE conversation_branches SET name = ? WHERE id = ?`, name, id)
	return err
}

// Delete removes a branch and all its messages.
func (r *BranchRepo) Delete(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get the branch to find its ID for message deletion
	var branchID string
	err = tx.QueryRow(`SELECT id FROM conversation_branches WHERE id = ?`, id).Scan(&branchID)
	if err != nil {
		return err
	}

	// Delete messages belonging to this branch
	_, err = tx.Exec(`DELETE FROM messages WHERE branch_id = ?`, branchID)
	if err != nil {
		return err
	}

	// Delete the branch record
	_, err = tx.Exec(`DELETE FROM conversation_branches WHERE id = ?`, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}
