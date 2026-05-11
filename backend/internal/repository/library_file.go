package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// LibraryFileRepo handles file-library metadata persistence.
type LibraryFileRepo struct {
	db *sql.DB
}

// NewLibraryFileRepo creates a new LibraryFileRepo.
func NewLibraryFileRepo(db *sql.DB) *LibraryFileRepo {
	return &LibraryFileRepo{db: db}
}

// Create inserts a new library file record.
func (r *LibraryFileRepo) Create(f *models.LibraryFile) error {
	if f.ID == "" {
		f.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	f.CreatedAt = now
	f.UpdatedAt = now
	if strings.TrimSpace(f.Status) == "" {
		f.Status = "queued"
	}
	if strings.TrimSpace(f.MetadataJSON) == "" {
		f.MetadataJSON = "{}"
	}

	_, err := r.db.Exec(`
		INSERT INTO library_files (
			id, owner_user_id, workspace_id, conversation_id, attachment_id,
			source_type, scope, display_name, original_filename, mime_type,
			file_ext, storage_path, source_url, size_bytes, checksum_sha256,
			status, error_message, indexed_at, created_at, updated_at, metadata_json
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.OwnerUserID, f.WorkspaceID, f.ConversationID, f.AttachmentID,
		f.SourceType, f.Scope, f.DisplayName, f.OriginalFilename, f.MimeType,
		f.FileExt, f.StoragePath, f.SourceURL, f.SizeBytes, f.ChecksumSHA256,
		f.Status, f.ErrorMessage, f.IndexedAt, f.CreatedAt, f.UpdatedAt, f.MetadataJSON,
	)
	if err != nil {
		return fmt.Errorf("create library file: %w", err)
	}
	return nil
}

// GetByID returns a library file by ID.
func (r *LibraryFileRepo) GetByID(id string) (*models.LibraryFile, error) {
	row := r.db.QueryRow(`
		SELECT id, owner_user_id, workspace_id, conversation_id, attachment_id,
		       source_type, scope, display_name, original_filename, mime_type,
		       file_ext, storage_path, source_url, size_bytes, checksum_sha256,
		       status, error_message, indexed_at, created_at, updated_at, metadata_json
		FROM library_files
		WHERE id = ?`, id)

	f, err := scanLibraryFile(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get library file: %w", err)
	}
	return f, nil
}

// ListByScope lists file records for an owner filtered by scope and optional scope IDs.
func (r *LibraryFileRepo) ListByScope(ownerUserID, scope string, workspaceID, conversationID *string) ([]models.LibraryFile, error) {
	args := []interface{}{scope}
	query := `
		SELECT id, owner_user_id, workspace_id, conversation_id, attachment_id,
		       source_type, scope, display_name, original_filename, mime_type,
		       file_ext, storage_path, source_url, size_bytes, checksum_sha256,
		       status, error_message, indexed_at, created_at, updated_at, metadata_json
		FROM library_files
		WHERE scope = ? AND status <> 'deleted'`

	if strings.TrimSpace(ownerUserID) == "" {
		query += " AND owner_user_id IS NULL"
	} else {
		query += " AND owner_user_id = ?"
		args = append(args, ownerUserID)
	}

	if workspaceID != nil {
		query += " AND workspace_id = ?"
		args = append(args, *workspaceID)
	}
	if conversationID != nil {
		query += " AND conversation_id = ?"
		args = append(args, *conversationID)
	}
	query += " ORDER BY updated_at DESC, created_at DESC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list library files by scope: %w", err)
	}
	defer rows.Close()

	return scanLibraryFiles(rows)
}

// SearchMetadata searches display name, original filename, and metadata payload.
func (r *LibraryFileRepo) SearchMetadata(ownerUserID, queryText string, scopes []string) ([]models.LibraryFile, error) {
	args := []interface{}{}
	query := `
		SELECT id, owner_user_id, workspace_id, conversation_id, attachment_id,
		       source_type, scope, display_name, original_filename, mime_type,
		       file_ext, storage_path, source_url, size_bytes, checksum_sha256,
		       status, error_message, indexed_at, created_at, updated_at, metadata_json
		FROM library_files
		WHERE status <> 'deleted'`

	if strings.TrimSpace(ownerUserID) == "" {
		query += " AND owner_user_id IS NULL"
	} else {
		query += " AND owner_user_id = ?"
		args = append(args, ownerUserID)
	}

	if len(scopes) > 0 {
		placeholders := make([]string, 0, len(scopes))
		for _, s := range scopes {
			placeholders = append(placeholders, "?")
			args = append(args, s)
		}
		query += " AND scope IN (" + strings.Join(placeholders, ",") + ")"
	}

	if q := strings.TrimSpace(queryText); q != "" {
		like := "%" + q + "%"
		query += " AND (display_name LIKE ? OR original_filename LIKE ? OR metadata_json LIKE ?)"
		args = append(args, like, like, like)
	}

	query += " ORDER BY updated_at DESC, created_at DESC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("search library metadata: %w", err)
	}
	defer rows.Close()

	return scanLibraryFiles(rows)
}

// UpdateStatus updates status, optional error details, and optional indexed timestamp.
func (r *LibraryFileRepo) UpdateStatus(id, status string, errorMessage *string, indexedAt *time.Time) error {
	_, err := r.db.Exec(`
		UPDATE library_files
		SET status = ?, error_message = ?, indexed_at = ?, updated_at = ?
		WHERE id = ?`,
		status, errorMessage, indexedAt, time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("update library file status: %w", err)
	}
	return nil
}

// MarkDeleted soft-deletes a library file record.
func (r *LibraryFileRepo) MarkDeleted(id string) error {
	_, err := r.db.Exec(`
		UPDATE library_files
		SET status = 'deleted', updated_at = ?
		WHERE id = ?`,
		time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("mark library file deleted: %w", err)
	}
	return nil
}

// Delete hard-deletes a library file record.
func (r *LibraryFileRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM library_files WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete library file: %w", err)
	}
	return nil
}

// UpdateFields updates editable file metadata fields.
func (r *LibraryFileRepo) UpdateFields(id string, displayName *string, scope *string, metadata map[string]interface{}) error {
	updates := []string{"updated_at = ?"}
	args := []interface{}{time.Now().UTC()}

	if displayName != nil {
		updates = append(updates, "display_name = ?")
		args = append(args, strings.TrimSpace(*displayName))
	}
	if scope != nil {
		updates = append(updates, "scope = ?")
		args = append(args, strings.TrimSpace(*scope))
	}
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
		updates = append(updates, "metadata_json = ?")
		args = append(args, string(b))
	}

	if len(updates) == 1 {
		return nil
	}

	args = append(args, id)
	query := "UPDATE library_files SET " + strings.Join(updates, ", ") + " WHERE id = ?"
	if _, err := r.db.Exec(query, args...); err != nil {
		return fmt.Errorf("update library file fields: %w", err)
	}
	return nil
}

// GetByChecksum returns a non-deleted file record with matching owner, scope, and checksum.
func (r *LibraryFileRepo) GetByChecksum(ownerUserID, scope, checksumSHA256 string) (*models.LibraryFile, error) {
	query := `
		SELECT id, owner_user_id, workspace_id, conversation_id, attachment_id,
		       source_type, scope, display_name, original_filename, mime_type,
		       file_ext, storage_path, source_url, size_bytes, checksum_sha256,
		       status, error_message, indexed_at, created_at, updated_at, metadata_json
		FROM library_files
		WHERE scope = ? AND checksum_sha256 = ? AND status <> 'deleted'`
	args := []interface{}{scope, checksumSHA256}
	if strings.TrimSpace(ownerUserID) == "" {
		query += " AND owner_user_id IS NULL"
	} else {
		query += " AND owner_user_id = ?"
		args = append(args, ownerUserID)
	}
	query += `
		ORDER BY created_at DESC
		LIMIT 1`

	row := r.db.QueryRow(query, args...)

	f, err := scanLibraryFile(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get library file by checksum: %w", err)
	}
	return f, nil
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanLibraryFile(s scanner) (*models.LibraryFile, error) {
	var f models.LibraryFile
	var ownerUserID sql.NullString
	var workspaceID sql.NullString
	var conversationID sql.NullString
	var attachmentID sql.NullString
	var originalFilename sql.NullString
	var mimeType sql.NullString
	var fileExt sql.NullString
	var storagePath sql.NullString
	var sourceURL sql.NullString
	var checksum sql.NullString
	var errorMessage sql.NullString
	var indexedAt sql.NullTime

	if err := s.Scan(
		&f.ID, &ownerUserID, &workspaceID, &conversationID, &attachmentID,
		&f.SourceType, &f.Scope, &f.DisplayName, &originalFilename, &mimeType,
		&fileExt, &storagePath, &sourceURL, &f.SizeBytes, &checksum,
		&f.Status, &errorMessage, &indexedAt, &f.CreatedAt, &f.UpdatedAt, &f.MetadataJSON,
	); err != nil {
		return nil, err
	}

	if ownerUserID.Valid {
		f.OwnerUserID = &ownerUserID.String
	}
	if workspaceID.Valid {
		f.WorkspaceID = &workspaceID.String
	}
	if conversationID.Valid {
		f.ConversationID = &conversationID.String
	}
	if attachmentID.Valid {
		f.AttachmentID = &attachmentID.String
	}
	if originalFilename.Valid {
		f.OriginalFilename = &originalFilename.String
	}
	if mimeType.Valid {
		f.MimeType = &mimeType.String
	}
	if fileExt.Valid {
		f.FileExt = &fileExt.String
	}
	if storagePath.Valid {
		f.StoragePath = &storagePath.String
	}
	if sourceURL.Valid {
		f.SourceURL = &sourceURL.String
	}
	if checksum.Valid {
		f.ChecksumSHA256 = &checksum.String
	}
	if errorMessage.Valid {
		f.ErrorMessage = &errorMessage.String
	}
	if indexedAt.Valid {
		t := indexedAt.Time
		f.IndexedAt = &t
	}
	return &f, nil
}

func scanLibraryFiles(rows *sql.Rows) ([]models.LibraryFile, error) {
	files := []models.LibraryFile{}
	for rows.Next() {
		f, err := scanLibraryFile(rows)
		if err != nil {
			return nil, fmt.Errorf("scan library file: %w", err)
		}
		files = append(files, *f)
	}
	return files, rows.Err()
}
