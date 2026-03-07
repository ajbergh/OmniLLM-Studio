package bundle

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// ExportOptions controls what gets included in the bundle.
type ExportOptions struct {
	IncludeAttachments bool     `json:"include_attachments"`
	ConversationIDs    []string `json:"conversation_ids,omitempty"` // empty = all
}

// ConversationBundle groups a conversation with its messages.
type ConversationBundle struct {
	Conversation models.Conversation `json:"conversation"`
	Messages     []models.Message    `json:"messages"`
}

// Exporter writes workspace data to a ZIP archive.
type Exporter struct {
	database       *sql.DB
	convoRepo      *repository.ConversationRepo
	msgRepo        *repository.MessageRepo
	attachRepo     *repository.AttachmentRepo
	providerRepo   *repository.ProviderRepo
	settingsRepo   *repository.SettingsRepo
	attachmentsDir string
	appVersion     string
}

// NewExporter creates an Exporter.
func NewExporter(
	database *sql.DB,
	convoRepo *repository.ConversationRepo,
	msgRepo *repository.MessageRepo,
	attachRepo *repository.AttachmentRepo,
	providerRepo *repository.ProviderRepo,
	settingsRepo *repository.SettingsRepo,
	attachmentsDir string,
	appVersion string,
) *Exporter {
	return &Exporter{
		database:       database,
		convoRepo:      convoRepo,
		msgRepo:        msgRepo,
		attachRepo:     attachRepo,
		providerRepo:   providerRepo,
		settingsRepo:   settingsRepo,
		attachmentsDir: attachmentsDir,
		appVersion:     appVersion,
	}
}

// Export writes a complete workspace bundle to the given writer.
func (e *Exporter) Export(w io.Writer, opts ExportOptions) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	stats := ManifestStats{}

	// 1. Export conversations + messages
	conversations, err := e.convoRepo.ListAll("", true) // include archived, no user scoping for export
	if err != nil {
		return fmt.Errorf("list conversations: %w", err)
	}

	// Filter to requested IDs if specified
	if len(opts.ConversationIDs) > 0 {
		idSet := make(map[string]bool, len(opts.ConversationIDs))
		for _, id := range opts.ConversationIDs {
			idSet[id] = true
		}
		filtered := make([]models.Conversation, 0, len(opts.ConversationIDs))
		for _, c := range conversations {
			if idSet[c.ID] {
				filtered = append(filtered, c)
			}
		}
		conversations = filtered
	}

	for _, convo := range conversations {
		msgs, err := e.msgRepo.ListByConversation(convo.ID)
		if err != nil {
			return fmt.Errorf("list messages for %s: %w", convo.ID, err)
		}

		bundle := ConversationBundle{
			Conversation: convo,
			Messages:     msgs,
		}

		data, err := json.MarshalIndent(bundle, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal conversation %s: %w", convo.ID, err)
		}

		f, err := zw.Create(fmt.Sprintf("conversations/%s.json", convo.ID))
		if err != nil {
			return err
		}
		if _, err := f.Write(data); err != nil {
			return err
		}

		stats.Conversations++
		stats.Messages += len(msgs)
	}

	// 2. Export attachments
	var allAttachments []models.Attachment
	for _, convo := range conversations {
		attachments, err := e.attachRepo.ListByConversation(convo.ID)
		if err != nil {
			return fmt.Errorf("list attachments for %s: %w", convo.ID, err)
		}
		allAttachments = append(allAttachments, attachments...)
	}

	// Attachment metadata
	if len(allAttachments) > 0 {
		metaData, err := json.MarshalIndent(allAttachments, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal attachment metadata: %w", err)
		}
		f, err := zw.Create("attachments/metadata.json")
		if err != nil {
			return err
		}
		if _, err := f.Write(metaData); err != nil {
			return err
		}
	}

	// Attachment files (if requested)
	if opts.IncludeAttachments {
		for _, att := range allAttachments {
			srcPath := filepath.Join(e.attachmentsDir, att.StoragePath)
			if err := addFileToZip(zw, srcPath, "attachments/files/"+att.StoragePath); err != nil {
				// Skip missing files with a warning rather than failing
				continue
			}
			stats.Attachments++
		}
	}

	// 3. Export providers (redact API keys)
	providers, err := e.providerRepo.List()
	if err != nil {
		return fmt.Errorf("list providers: %w", err)
	}
	stats.Providers = len(providers)

	provData, err := json.MarshalIndent(providers, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal providers: %w", err)
	}
	f, err := zw.Create("providers.json")
	if err != nil {
		return err
	}
	if _, err := f.Write(provData); err != nil {
		return err
	}

	// 4. Export settings (non-sensitive only)
	settings, err := e.exportSafeSettings()
	if err != nil {
		return fmt.Errorf("export settings: %w", err)
	}
	settingsData, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	f, err = zw.Create("settings.json")
	if err != nil {
		return err
	}
	if _, err := f.Write(settingsData); err != nil {
		return err
	}

	// 5. Write manifest (last, since we need final stats)
	schemaVer, _ := db.SchemaVersion(e.database)
	manifest := &Manifest{
		FormatVersion: CurrentFormatVersion,
		AppVersion:    e.appVersion,
		SchemaVersion: schemaVer,
		CreatedAt:     time.Now().UTC(),
		Stats:         stats,
	}

	manifestData, err := MarshalManifest(manifest)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	f, err = zw.Create("manifest.json")
	if err != nil {
		return err
	}
	if _, err := f.Write(manifestData); err != nil {
		return err
	}

	return nil
}

// sensitiveSettings are keys that should NOT be exported.
var sensitiveSettings = map[string]bool{
	"brave_api_key": true,
}

// exportSafeSettings returns settings with sensitive keys redacted.
func (e *Exporter) exportSafeSettings() (map[string]string, error) {
	all, err := e.settingsRepo.GetAll()
	if err != nil {
		return nil, err
	}

	safe := make(map[string]string, len(all))
	for k, v := range all {
		if sensitiveSettings[k] {
			continue // omit sensitive keys entirely
		}
		safe[k] = v
	}
	return safe, nil
}

// addFileToZip copies a file from disk into the ZIP archive.
func addFileToZip(zw *zip.Writer, srcPath, zipPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = zipPath
	header.Method = zip.Deflate

	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, src)
	return err
}
