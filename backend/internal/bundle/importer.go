package bundle

import (
	"archive/zip"
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
)

// importableSettingsKeys is the allowlist of settings keys that may be imported from a bundle.
// Sensitive keys like brave_api_key are deliberately excluded.
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

// safeJoinPath returns a path guaranteed to be under baseDir.
func safeJoinPath(baseDir, untrustedPath string) (string, error) {
	if untrustedPath == "" {
		return "", fmt.Errorf("empty path")
	}
	cleaned := filepath.Clean(untrustedPath)
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("absolute path not allowed")
	}
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, string(filepath.Separator)+"..") {
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

// maxZipEntrySize limits the decompressed size of a single ZIP entry (50 MB).
const maxZipEntrySize = 50 << 20

// ImportStrategy controls how ID conflicts are resolved.
type ImportStrategy string

const (
	ImportSkip      ImportStrategy = "skip"      // skip existing IDs
	ImportOverwrite ImportStrategy = "overwrite" // replace existing IDs
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
	Warnings              []string `json:"warnings"`
}

// Importer reads a workspace bundle and inserts data into the database.
type Importer struct {
	database       *sql.DB
	attachmentsDir string
}

// NewImporter creates an Importer.
func NewImporter(database *sql.DB, attachmentsDir string) *Importer {
	return &Importer{database: database, attachmentsDir: attachmentsDir}
}

// Validate checks the bundle without importing anything.
func (imp *Importer) Validate(r io.ReaderAt, size int64) (*ValidationReport, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return &ValidationReport{Valid: false, Errors: []string{"invalid ZIP archive: " + err.Error()}}, nil
	}

	// Read manifest
	manifest, err := readManifestFromZip(zr)
	if err != nil {
		return &ValidationReport{Valid: false, Errors: []string{err.Error()}}, nil
	}

	schemaVer, _ := db.SchemaVersion(imp.database)
	warnings := manifest.ValidateCompatibility(schemaVer)

	// Check required files exist
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

	return &ValidationReport{
		Manifest: manifest,
		Valid:    true,
		Warnings: warnings,
	}, nil
}

// Import reads a ZIP bundle and inserts data into the database.
func (imp *Importer) Import(r io.ReaderAt, size int64, strategy ImportStrategy) (*ImportResult, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("invalid ZIP archive: %w", err)
	}

	// Read manifest for validation
	manifest, err := readManifestFromZip(zr)
	if err != nil {
		return nil, err
	}

	schemaVer, _ := db.SchemaVersion(imp.database)
	warnings := manifest.ValidateCompatibility(schemaVer)

	result := &ImportResult{Warnings: warnings}

	// Run all inserts in a single transaction
	tx, err := imp.database.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Import conversations + messages
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "conversations/") || !strings.HasSuffix(f.Name, ".json") {
			continue
		}

		data, err := readZipFile(f)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("skip %s: %v", f.Name, err))
			continue
		}

		var bundle ConversationBundle
		if err := json.Unmarshal(data, &bundle); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("skip %s: invalid JSON: %v", f.Name, err))
			continue
		}

		imported, err := imp.importConversation(tx, &bundle, strategy)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("error importing %s: %v", bundle.Conversation.ID, err))
			continue
		}
		if imported {
			result.ConversationsImported++
			result.MessagesImported += len(bundle.Messages)
		} else {
			result.ConversationsSkipped++
		}
	}

	// 2. Import attachments
	attachMeta, err := readZipEntry(zr, "attachments/metadata.json")
	if err == nil {
		var attachments []models.Attachment
		if err := json.Unmarshal(attachMeta, &attachments); err == nil {
			for _, att := range attachments {
				imported, err := imp.importAttachment(tx, zr, &att, strategy)
				if err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("skip attachment %s: %v", att.ID, err))
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

	// 3. Import providers
	provData, err := readZipEntry(zr, "providers.json")
	if err == nil {
		var providers []models.ProviderProfile
		if err := json.Unmarshal(provData, &providers); err == nil {
			for _, p := range providers {
				imported, err := imp.importProvider(tx, &p, strategy)
				if err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("skip provider %s: %v", p.Name, err))
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

	// 4. Import settings (allowlisted keys only)
	settingsData, err := readZipEntry(zr, "settings.json")
	if err == nil {
		var settings map[string]string
		if err := json.Unmarshal(settingsData, &settings); err == nil {
			for k, v := range settings {
				if !importableSettingsKeys[k] {
					log.Printf("[import] skipping non-importable setting %q", k)
					result.Warnings = append(result.Warnings, fmt.Sprintf("skipped non-importable setting: %s", k))
					continue
				}
				if _, execErr := tx.Exec(
					`INSERT OR REPLACE INTO settings (key, value_json) VALUES (?, ?)`, k, v,
				); execErr != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("skip setting %s: %v", k, execErr))
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

	return result, nil
}

// importConversation inserts a conversation + messages, respecting strategy.
func (imp *Importer) importConversation(tx *sql.Tx, bundle *ConversationBundle, strategy ImportStrategy) (bool, error) {
	c := bundle.Conversation

	// Check if it already exists
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM conversations WHERE id = ?`, c.ID).Scan(&exists); err != nil {
		return false, err
	}

	if exists > 0 {
		if strategy == ImportSkip {
			return false, nil
		}
		// Overwrite: delete existing conversation + cascade messages
		if _, err := tx.Exec(`DELETE FROM messages WHERE conversation_id = ?`, c.ID); err != nil {
			return false, err
		}
		if _, err := tx.Exec(`DELETE FROM conversations WHERE id = ?`, c.ID); err != nil {
			return false, err
		}
	}

	// Insert conversation
	kind := c.Kind
	if kind == "" {
		kind = models.ConversationKindChat
	}
	if _, err := tx.Exec(`
		INSERT INTO conversations (id, title, created_at, updated_at, archived, pinned,
			default_provider, default_model, system_prompt, kind, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Title, c.CreatedAt.Format(time.RFC3339), c.UpdatedAt.Format(time.RFC3339),
		boolToInt(c.Archived), boolToInt(c.Pinned),
		c.DefaultProvider, c.DefaultModel, c.SystemPrompt, kind, c.MetadataJSON,
	); err != nil {
		return false, fmt.Errorf("insert conversation: %w", err)
	}

	// Insert messages
	for _, m := range bundle.Messages {
		if _, err := tx.Exec(`
			INSERT INTO messages (id, conversation_id, role, content, created_at,
				provider, model, token_input, token_output, latency_ms, metadata_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			m.ID, m.ConversationID, m.Role, m.Content, m.CreatedAt.Format(time.RFC3339),
			m.Provider, m.Model, m.TokenInput, m.TokenOutput, m.LatencyMs, m.MetadataJSON,
		); err != nil {
			return false, fmt.Errorf("insert message %s: %w", m.ID, err)
		}
	}

	return true, nil
}

// importAttachment inserts an attachment record and copies the file if present.
func (imp *Importer) importAttachment(tx *sql.Tx, zr *zip.Reader, att *models.Attachment, strategy ImportStrategy) (bool, error) {
	// Sanitize StoragePath: force basename-only to prevent path traversal
	att.StoragePath = filepath.Base(att.StoragePath)
	if att.StoragePath == "." || att.StoragePath == string(filepath.Separator) {
		return false, fmt.Errorf("invalid storage path")
	}

	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM attachments WHERE id = ?`, att.ID).Scan(&exists); err != nil {
		return false, err
	}

	if exists > 0 {
		if strategy == ImportSkip {
			return false, nil
		}
		if _, err := tx.Exec(`DELETE FROM attachments WHERE id = ?`, att.ID); err != nil {
			return false, err
		}
	}

	// Insert attachment record
	if _, err := tx.Exec(`
		INSERT INTO attachments (id, conversation_id, message_id, type, mime_type,
			storage_path, bytes, width, height, created_at, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		att.ID, att.ConversationID, att.MessageID, att.Type, att.MimeType,
		att.StoragePath, att.Bytes, att.Width, att.Height,
		att.CreatedAt.Format(time.RFC3339), att.MetadataJSON,
	); err != nil {
		return false, fmt.Errorf("insert attachment: %w", err)
	}

	// Copy file from ZIP if present — use safe path joining
	fileEntry := "attachments/files/" + att.StoragePath
	if data, err := readZipEntry(zr, fileEntry); err == nil {
		dstPath, pathErr := safeJoinPath(imp.attachmentsDir, att.StoragePath)
		if pathErr != nil {
			log.Printf("[import] skipping file write for attachment %s: unsafe path %q", att.ID, att.StoragePath)
			return true, nil // record inserted, skip unsafe file write
		}
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return true, nil // record inserted, file copy best-effort
		}
		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			return true, nil // record inserted, file copy best-effort
		}
	}

	return true, nil
}

// importProvider inserts a provider profile, respecting strategy.
func (imp *Importer) importProvider(tx *sql.Tx, p *models.ProviderProfile, strategy ImportStrategy) (bool, error) {
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM provider_profiles WHERE id = ?`, p.ID).Scan(&exists); err != nil {
		return false, err
	}

	if exists > 0 {
		if strategy == ImportSkip {
			return false, nil
		}
		if _, err := tx.Exec(`DELETE FROM provider_profiles WHERE id = ?`, p.ID); err != nil {
			return false, err
		}
	}

	if _, err := tx.Exec(`
		INSERT INTO provider_profiles (id, name, type, base_url, default_model, default_image_model,
			enabled, created_at, updated_at, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Type, p.BaseURL, p.DefaultModel, p.DefaultImageModel,
		boolToInt(p.Enabled), p.CreatedAt.Format(time.RFC3339), p.UpdatedAt.Format(time.RFC3339),
		p.MetadataJSON,
	); err != nil {
		return false, fmt.Errorf("insert provider: %w", err)
	}

	return true, nil
}

// --- Helpers ---

func readManifestFromZip(zr *zip.Reader) (*Manifest, error) {
	data, err := readZipEntry(zr, "manifest.json")
	if err != nil {
		return nil, fmt.Errorf("manifest.json not found in bundle")
	}
	return UnmarshalManifest(data)
}

func readZipEntry(zr *zip.Reader, name string) ([]byte, error) {
	for _, f := range zr.File {
		if f.Name == name {
			return readZipFile(f)
		}
	}
	return nil, fmt.Errorf("entry %q not found", name)
}

func readZipFile(f *zip.File) ([]byte, error) {
	// Guard against decompression bombs
	if f.UncompressedSize64 > maxZipEntrySize {
		return nil, fmt.Errorf("zip entry %q too large: %d bytes", f.Name, f.UncompressedSize64)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, maxZipEntrySize))
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
