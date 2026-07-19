package repository

import (
	"fmt"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

// ReplaceAttachmentChunks atomically replaces the relational chunk set
// after the replacement vectors have been indexed. It returns only stale
// chunk IDs that no longer exist in the replacement set.
func (r *ChunkRepo) ReplaceAttachmentChunks(attachmentID string, chunks []models.DocumentChunk) ([]string, error) {
	oldChunks, err := r.ListByAttachment(attachmentID)
	if err != nil {
		return nil, err
	}
	newIDs := make(map[string]struct{}, len(chunks))
	for index := range chunks {
		if chunks[index].ID == "" {
			chunks[index].ID = uuid.NewString()
		}
		newIDs[chunks[index].ID] = struct{}{}
	}

	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin chunk replacement: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM document_chunks WHERE attachment_id = ?", attachmentID); err != nil {
		return nil, fmt.Errorf("delete previous attachment chunks: %w", err)
	}
	stmt, err := tx.Prepare(`
    INSERT INTO document_chunks (
      id, attachment_id, conversation_id, library_file_id, scope, workspace_id, source_type,
      chunk_index, content, char_offset, char_length, token_count, page_number, section_title,
      chunk_metadata_json, metadata_json, created_at
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, fmt.Errorf("prepare replacement insert: %w", err)
	}
	defer stmt.Close()
	now := time.Now().UTC()
	for index := range chunks {
		chunk := &chunks[index]
		chunk.CreatedAt = now
		if chunk.MetadataJSON == "" {
			chunk.MetadataJSON = "{}"
		}
		if chunk.ChunkMetaJSON == "" {
			chunk.ChunkMetaJSON = "{}"
		}
		if _, err := stmt.Exec(
			chunk.ID, chunk.AttachmentID, chunk.ConversationID, chunk.LibraryFileID,
			chunkScopeOrDefault(chunk.Scope), chunk.WorkspaceID, chunk.SourceType,
			chunk.ChunkIndex, chunk.Content, chunk.CharOffset, chunk.CharLength,
			chunk.TokenCount, chunk.PageNumber, chunk.SectionTitle,
			chunkMetaOrDefault(chunk.ChunkMetaJSON), chunk.MetadataJSON, chunk.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("insert replacement chunk %d: %w", index, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit chunk replacement: %w", err)
	}

	stale := make([]string, 0)
	for _, old := range oldChunks {
		if _, retained := newIDs[old.ID]; !retained {
			stale = append(stale, old.ID)
		}
	}
	return stale, nil
}
