package bundle

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/rag"
	"github.com/google/uuid"
)

// importableSettingsKeys is the allowlist of settings keys that may be imported
// from a bundle. Sensitive keys are deliberately excluded.
var importableSettingsKeys = map[string]bool{
	"web_search_provider":      true,
	"jina_reader_enabled":      true,
	"jina_reader_max_len":      true,
	"rag_enabled":              true,
	"rag_embedding_model":      true,
	"rag_chunk_size":           true,
	"rag_chunk_overlap":        true,
	"rag_top_k":                true,
	"rag_similarity_threshold": true,
	"rag_embedding_provider":   true,
	"semantic_search_enabled":  true,
	"semantic_search_provider": true,
	"semantic_search_model":    true,
}

const (
	maxZipEntrySize  = 50 << 20 // 50 MB per decompressed entry
	maxZipTotalSize  = 2 << 30  // 2 GB total decompressed archive
	maxZipEntryCount = 100_000
)

// safeJoinPath returns a path guaranteed to be under baseDir.
func safeJoinPath(baseDir, untrustedPath string) (string, error) {
	if untrustedPath == "" {
		return "", fmt.Errorf("empty path")
	}
	cleaned := filepath.Clean(untrustedPath)
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("absolute path not allowed")
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal not allowed")
	}
	joined := filepath.Join(baseDir, cleaned)
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(absJoined, absBase+string(filepath.Separator)) && absJoined != absBase {
		return "", fmt.Errorf("path escapes base directory")
	}
	return joined, nil
}

// ImportStrategy controls how ID conflicts are resolved.
type ImportStrategy string

const (
	ImportSkip      ImportStrategy = "skip"
	ImportOverwrite ImportStrategy = "overwrite"
)

// ImportResult summarises what happened during import.
type ImportResult struct {
	ConversationsImported int      `json:"conversations_imported"`
	ConversationsSkipped  int      `json:"conversations_skipped"`
	MessagesImported      int      `json:"messages_imported"`
	AttachmentsImported   int      `json:"attachments_imported"`
	ProvidersImported     int      `json:"providers_imported"`
	ProvidersSkipped      int      `json:"providers_skipped"`
	SettingsImported      int      `json:"settings_imported"`
	RAGVectorsImported    bool     `json:"rag_vectors_imported,omitempty"`
	Warnings              []string `json:"warnings"`
}

// Importer reads a workspace bundle and inserts data into the database.
type Importer struct {
	database       *sql.DB
	attachmentsDir string
	vectorStore    *rag.VectorStore
}

func NewImporter(database *sql.DB, attachmentsDir string, vectorStore *rag.VectorStore) *Importer {
	return &Importer{database: database, attachmentsDir: attachmentsDir, vectorStore: vectorStore}
}

// Validate checks the bundle without importing anything.
func (imp *Importer) Validate(r io.ReaderAt, size int64) (*ValidationReport, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return &ValidationReport{Valid: false, Errors: []string{"invalid ZIP archive: " + err.Error()}}, nil
	}
	if err := validateZipShape(zr); err != nil {
		return &ValidationReport{Valid: false, Errors: []string{err.Error()}}, nil
	}

	manifest, err := readManifestFromZip(zr)
	if err != nil {
		return &ValidationReport{Valid: false, Errors: []string{err.Error()}}, nil
	}
	schemaVer, _ := db.SchemaVersion(imp.database)
	warnings := manifest.ValidateCompatibility(schemaVer)

	hasConversations := false
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "conversations/") && strings.HasSuffix(f.Name, ".json") {
			hasConversations = true
			break
		}
	}
	if !hasConversations && manifest.Stats.Conversations > 0 {
		warnings = append(warnings, "manifest reports conversations but no conversation files found")
	}

	return &ValidationReport{Manifest: manifest, Valid: true, Warnings: warnings}, nil
}

// Import reads a ZIP bundle and inserts data into the database.
func (imp *Importer) Import(r io.ReaderAt, size int64, strategy ImportStrategy) (*ImportResult, error) {
	if strategy != ImportSkip && strategy != ImportOverwrite {
		return nil, fmt.Errorf("unsupported import strategy %q", strategy)
	}
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("invalid ZIP archive: %w", err)
	}
	if err := validateZipShape(zr); err != nil {
		return nil, err
	}

	manifest, err := readManifestFromZip(zr)
	if err != nil {
		return nil, err
	}
	schemaVer, _ := db.SchemaVersion(imp.database)
	result := &ImportResult{Warnings: manifest.ValidateCompatibility(schemaVer)}

	tx, err := imp.database.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "conversations/") || !strings.HasSuffix(f.Name, ".json") {
			continue
		}
		data, err := readZipFile(f)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("skip %s: %v", f.Name, err))
			continue
		}
		var conversationBundle ConversationBundle
		if err := json.Unmarshal(data, &conversationBundle); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("skip %s: invalid JSON: %v", f.Name, err))
			continue
		}
		imported, err := imp.importConversation(tx, &conversationBundle, strategy)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("error importing %s: %v", conversationBundle.Conversation.ID, err))
			continue
		}
		if imported {
			result.ConversationsImported++
			result.MessagesImported += len(conversationBundle.Messages)
		} else {
			result.ConversationsSkipped++
		}
	}

	if attachMeta, err := readZipEntry(zr, "attachments/metadata.json"); err == nil {
		var attachments []models.Attachment
		if err := json.Unmarshal(attachMeta, &attachments); err == nil {
			for _, attachment := range attachments {
				imported, err := imp.importAttachment(tx, zr, &attachment, strategy)
				if err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("skip attachment %s: %v", attachment.ID, err))
					continue
				}
				if imported {
					result.AttachmentsImported++
				}
			}
		} else {
			result.Warnings = append(result.Warnings, "invalid attachments/metadata.json: "+err.Error())
		}
	}

	if providerData, err := readZipEntry(zr, "providers.json"); err == nil {
		var providers []models.ProviderProfile
		if err := json.Unmarshal(providerData, &providers); err == nil {
			for _, provider := range providers {
				imported, err := imp.importProvider(tx, &provider, strategy)
				if err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("skip provider %s: %v", provider.Name, err))
					continue
				}
				if imported {
					result.ProvidersImported++
				} else {
					result.ProvidersSkipped++
				}
			}
		} else {
			result.Warnings = append(result.Warnings, "invalid providers.json: "+err.Error())
		}
	}

	if settingsData, err := readZipEntry(zr, "settings.json"); err == nil {
		var settings map[string]string
		if err := json.Unmarshal(settingsData, &settings); err == nil {
			for key, value := range settings {
				if !importableSettingsKeys[key] {
					log.Printf("[import] skipping non-importable setting %q", key)
					result.Warnings = append(result.Warnings, fmt.Sprintf("skipped non-importable setting: %s", key))
					continue
				}
				if _, execErr := tx.Exec(`INSERT OR REPLACE INTO settings (key, value_json) VALUES (?, ?)`, key, value); execErr != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("skip setting %s: %v", key, execErr))
					continue
				}
				result.SettingsImported++
			}
		} else {
			result.Warnings = append(result.Warnings, "invalid settings.json: "+err.Error())
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	if imp.vectorStore != nil {
		if data, err := readZipEntry(zr, "rag/chromem.gob"); err == nil {
			if err := imp.vectorStore.ImportFromReader(bytes.NewReader(data), ""); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("rag vectors: %v", err))
			} else {
				result.RAGVectorsImported = true
			}
		}
	}
	return result, nil
}

// importConversation restores all current ownership, workspace, branch, and
// parent-message fields rather than silently downgrading to the legacy schema.
func (imp *Importer) importConversation(tx *sql.Tx, bundle *ConversationBundle, strategy ImportStrategy) (bool, error) {
	conversation := bundle.Conversation
	if strings.TrimSpace(conversation.ID) == "" {
		return false, fmt.Errorf("conversation ID is required")
	}

	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM conversations WHERE id = ?`, conversation.ID).Scan(&exists); err != nil {
		return false, err
	}
	if exists > 0 {
		if strategy == ImportSkip {
			return false, nil
		}
		if _, err := tx.Exec(`DELETE FROM messages WHERE conversation_id = ?`, conversation.ID); err != nil {
			return false, err
		}
		if _, err := tx.Exec(`DELETE FROM conversations WHERE id = ?`, conversation.ID); err != nil {
			return false, err
		}
	}

	kind := strings.TrimSpace(conversation.Kind)
	if kind == "" {
		kind = models.ConversationKindChat
	}
	if _, err := tx.Exec(`
		INSERT INTO conversations (
			id, title, created_at, updated_at, archived, pinned,
			default_provider, default_model, system_prompt, kind, metadata_json,
			workspace_id, user_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		conversation.ID, conversation.Title,
		conversation.CreatedAt.Format(time.RFC3339), conversation.UpdatedAt.Format(time.RFC3339),
		boolToInt(conversation.Archived), boolToInt(conversation.Pinned),
		conversation.DefaultProvider, conversation.DefaultModel, conversation.SystemPrompt,
		kind, conversation.MetadataJSON, conversation.WorkspaceID, conversation.UserID,
	); err != nil {
		return false, fmt.Errorf("insert conversation: %w", err)
	}

	for _, message := range bundle.Messages {
		if message.ConversationID != "" && message.ConversationID != conversation.ID {
			return false, fmt.Errorf("message %s references conversation %s, expected %s", message.ID, message.ConversationID, conversation.ID)
		}
		branchID := strings.TrimSpace(message.BranchID)
		if branchID == "" {
			branchID = "main"
		}
		if _, err := tx.Exec(`
			INSERT INTO messages (
				id, conversation_id, role, content, created_at, provider, model,
				token_input, token_output, latency_ms, metadata_json,
				branch_id, parent_message_id, user_id
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			message.ID, conversation.ID, message.Role, message.Content,
			message.CreatedAt.Format(time.RFC3339), message.Provider, message.Model,
			message.TokenInput, message.TokenOutput, message.LatencyMs, message.MetadataJSON,
			branchID, message.ParentMessageID, message.UserID,
		); err != nil {
			return false, fmt.Errorf("insert message %s: %w", message.ID, err)
		}
	}
	return true, nil
}

func (imp *Importer) importAttachment(tx *sql.Tx, zr *zip.Reader, attachment *models.Attachment, strategy ImportStrategy) (bool, error) {
	archiveStoragePath := filepath.Base(attachment.StoragePath)
	if archiveStoragePath == "." || archiveStoragePath == string(filepath.Separator) || archiveStoragePath == "" {
		return false, fmt.Errorf("invalid storage path")
	}
	// Never reuse an archive-controlled name as a filesystem path. Imported
	// attachments receive the same opaque, server-generated storage names as
	// newly uploaded attachments.
	storagePath := uuid.NewString()
	attachment.StoragePath = storagePath

	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM attachments WHERE id = ?`, attachment.ID).Scan(&exists); err != nil {
		return false, err
	}
	if exists > 0 {
		if strategy == ImportSkip {
			return false, nil
		}
		if _, err := tx.Exec(`DELETE FROM attachments WHERE id = ?`, attachment.ID); err != nil {
			return false, err
		}
	}

	if _, err := tx.Exec(`
		INSERT INTO attachments (id, conversation_id, message_id, type, mime_type,
			storage_path, bytes, width, height, created_at, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		attachment.ID, attachment.ConversationID, attachment.MessageID, attachment.Type, attachment.MimeType,
		storagePath, attachment.Bytes, attachment.Width, attachment.Height,
		attachment.CreatedAt.Format(time.RFC3339), attachment.MetadataJSON,
	); err != nil {
		return false, fmt.Errorf("insert attachment: %w", err)
	}

	fileEntry := "attachments/files/" + archiveStoragePath
	if data, err := readZipEntry(zr, fileEntry); err == nil {
		if err := os.MkdirAll(imp.attachmentsDir, 0700); err != nil {
			return true, nil
		}
		destination := filepath.Join(imp.attachmentsDir, storagePath)
		if err := os.WriteFile(destination, data, 0600); err != nil {
			return true, nil
		}
	}
	return true, nil
}

func (imp *Importer) importProvider(tx *sql.Tx, provider *models.ProviderProfile, strategy ImportStrategy) (bool, error) {
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM provider_profiles WHERE id = ?`, provider.ID).Scan(&exists); err != nil {
		return false, err
	}
	if exists > 0 {
		if strategy == ImportSkip {
			return false, nil
		}
		if _, err := tx.Exec(`DELETE FROM provider_profiles WHERE id = ?`, provider.ID); err != nil {
			return false, err
		}
	}

	if _, err := tx.Exec(`
		INSERT INTO provider_profiles (id, name, type, base_url, default_model, default_image_model,
			enabled, created_at, updated_at, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		provider.ID, provider.Name, provider.Type, provider.BaseURL, provider.DefaultModel, provider.DefaultImageModel,
		boolToInt(provider.Enabled), provider.CreatedAt.Format(time.RFC3339), provider.UpdatedAt.Format(time.RFC3339),
		provider.MetadataJSON,
	); err != nil {
		return false, fmt.Errorf("insert provider: %w", err)
	}
	return true, nil
}

func validateZipShape(zr *zip.Reader) error {
	if len(zr.File) > maxZipEntryCount {
		return fmt.Errorf("ZIP archive contains too many entries: %d", len(zr.File))
	}
	var total uint64
	for _, file := range zr.File {
		if file.UncompressedSize64 > maxZipEntrySize {
			return fmt.Errorf("zip entry %q too large: %d bytes", file.Name, file.UncompressedSize64)
		}
		if strings.HasPrefix(filepath.Clean(file.Name), "..") || filepath.IsAbs(file.Name) {
			return fmt.Errorf("zip entry %q has an unsafe path", file.Name)
		}
		total += file.UncompressedSize64
		if total > maxZipTotalSize {
			return fmt.Errorf("ZIP archive expands beyond the %d byte limit", maxZipTotalSize)
		}
	}
	return nil
}

func readManifestFromZip(zr *zip.Reader) (*Manifest, error) {
	data, err := readZipEntry(zr, "manifest.json")
	if err != nil {
		return nil, fmt.Errorf("manifest.json not found in bundle")
	}
	return UnmarshalManifest(data)
}

func readZipEntry(zr *zip.Reader, name string) ([]byte, error) {
	for _, file := range zr.File {
		if file.Name == name {
			return readZipFile(file)
		}
	}
	return nil, fmt.Errorf("entry %q not found", name)
}

func readZipFile(file *zip.File) ([]byte, error) {
	if file.UncompressedSize64 > maxZipEntrySize {
		return nil, fmt.Errorf("zip entry %q too large: %d bytes", file.Name, file.UncompressedSize64)
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, maxZipEntrySize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxZipEntrySize {
		return nil, fmt.Errorf("zip entry %q exceeded the decompressed size limit", file.Name)
	}
	return data, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
