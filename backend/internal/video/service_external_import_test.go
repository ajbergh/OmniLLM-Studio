package video

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

func TestImportExternalAssetCopiesFileLibraryBytes(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	attachmentsDir := t.TempDir()
	storagePath := filepath.Join("library", "script.txt")
	sourceBytes := []byte("shot one\nshot two\n")
	writeTestFile(t, filepath.Join(attachmentsDir, storagePath), sourceBytes)

	libraryRepo := repository.NewLibraryFileRepo(database)
	originalName := "storyboard.txt"
	mimeType := "text/plain"
	file := &models.LibraryFile{
		SourceType:       "attachment",
		Scope:            "conversation",
		DisplayName:      "Storyboard",
		OriginalFilename: &originalName,
		MimeType:         &mimeType,
		StoragePath:      &storagePath,
		SizeBytes:        int64(len(sourceBytes)),
		Status:           "indexed",
		MetadataJSON:     "{}",
	}
	if err := libraryRepo.Create(file); err != nil {
		t.Fatalf("create library file: %v", err)
	}

	service := newImportTestService(database, attachmentsDir)
	service.ConfigureExternalAssetSources(libraryRepo, nil, nil, nil, nil, nil, attachmentsDir)
	project, err := service.CreateProject("", "Import Test", "mock", "mock-video-v1", 1280, 720, 30, "16:9")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	asset, err := service.ImportExternalAsset(context.Background(), "", project.ID, ExternalAssetImportRequest{
		SourceStudio: "file_library",
		SourceID:     file.ID,
	})
	if err != nil {
		t.Fatalf("import external asset: %v", err)
	}
	if asset.Kind != "text" {
		t.Fatalf("kind: got %q, want text", asset.Kind)
	}
	if asset.FileName != originalName {
		t.Fatalf("file name: got %q, want %q", asset.FileName, originalName)
	}
	if asset.MimeType != "text/plain" {
		t.Fatalf("mime type: got %q, want text/plain", asset.MimeType)
	}
	if asset.SizeBytes != int64(len(sourceBytes)) {
		t.Fatalf("size bytes: got %d, want %d", asset.SizeBytes, len(sourceBytes))
	}
	copied := readTestFile(t, filepath.Join(attachmentsDir, asset.FilePath))
	if !bytes.Equal(copied, sourceBytes) {
		t.Fatalf("imported bytes mismatch: got %q, want %q", copied, sourceBytes)
	}
	if filepath.Ext(asset.FilePath) == ".txt" && filepath.Base(asset.FilePath) == "storyboard.txt.ref.txt" {
		t.Fatalf("import wrote stale metadata placeholder path: %s", asset.FilePath)
	}
}

func TestImportExternalAssetCopiesMusicBytes(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	attachmentsDir := t.TempDir()
	musicPath := filepath.Join("music", "session-1", "generation-1", "output.mp3")
	sourceBytes := []byte("fake mp3 bytes")
	writeTestFile(t, filepath.Join(attachmentsDir, musicPath), sourceBytes)

	musicSessions := repository.NewMusicSessionRepo(database)
	musicAssets := repository.NewMusicAssetRepo(database)
	session, err := musicSessions.Create("", "Track", "mock", "mock")
	if err != nil {
		t.Fatalf("create music session: %v", err)
	}
	duration := int64(12000)
	musicAsset := &models.MusicAsset{
		SessionID:    session.ID,
		GenerationID: "generation-1",
		Kind:         "music",
		FileName:     "output.mp3",
		FilePath:     musicPath,
		MimeType:     "audio/mpeg",
		SizeBytes:    int64(len(sourceBytes)),
		DurationMS:   duration,
		Provider:     "mock",
		Model:        "mock",
		MetadataJSON: "{}",
	}
	if err := musicAssets.Create(musicAsset); err != nil {
		t.Fatalf("create music asset: %v", err)
	}

	service := newImportTestService(database, attachmentsDir)
	service.ConfigureExternalAssetSources(nil, musicSessions, musicAssets, nil, nil, nil, attachmentsDir)
	project, err := service.CreateProject("", "Import Test", "mock", "mock-video-v1", 1280, 720, 30, "16:9")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	asset, err := service.ImportExternalAsset(context.Background(), "", project.ID, ExternalAssetImportRequest{
		SourceStudio: "music",
		SourceID:     musicAsset.ID,
	})
	if err != nil {
		t.Fatalf("import external asset: %v", err)
	}
	if asset.Kind != "music" {
		t.Fatalf("kind: got %q, want music", asset.Kind)
	}
	if asset.DurationMS == nil || *asset.DurationMS != duration {
		t.Fatalf("duration not preserved: %+v", asset.DurationMS)
	}
	if copied := readTestFile(t, filepath.Join(attachmentsDir, asset.FilePath)); !bytes.Equal(copied, sourceBytes) {
		t.Fatalf("imported music bytes mismatch: got %q, want %q", copied, sourceBytes)
	}
}

func newImportTestService(database *sql.DB, attachmentsDir string) *Service {
	return NewService(
		repository.NewVideoProjectRepo(database),
		repository.NewVideoGenerationRepo(database),
		repository.NewVideoAssetRepo(database),
		repository.NewVideoTimelineRepo(database),
		repository.NewVideoRenderJobRepo(database),
		nil,
		attachmentsDir,
	)
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir test file dir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read test file: %v", err)
	}
	return data
}
