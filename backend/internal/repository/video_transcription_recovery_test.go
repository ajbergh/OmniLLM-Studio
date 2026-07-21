package repository

import (
	"path/filepath"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/models"
)

func TestVideoTranscriptionRepoFailInterrupted(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "transcription-recovery.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	repo := NewVideoTranscriptionRepo(database)
	queued := &models.VideoTranscript{
		ProjectID: "project", AssetID: "asset", ProviderProfileID: "provider",
		Provider: "openai", Model: "model", Status: "queued",
	}
	completed := &models.VideoTranscript{
		ProjectID: "project", AssetID: "asset", ProviderProfileID: "provider",
		Provider: "openai", Model: "model", Status: "completed",
	}
	if err := repo.Create(queued); err != nil {
		t.Fatalf("create queued transcript: %v", err)
	}
	if err := repo.Create(completed); err != nil {
		t.Fatalf("create completed transcript: %v", err)
	}

	const message = "interrupted; retry"
	count, err := repo.FailInterrupted(message)
	if err != nil {
		t.Fatalf("recover interrupted transcripts: %v", err)
	}
	if count != 1 {
		t.Fatalf("recovered %d transcripts, want 1", count)
	}

	recovered, err := repo.GetByID(queued.ID)
	if err != nil {
		t.Fatalf("get recovered transcript: %v", err)
	}
	if recovered == nil || recovered.Status != "failed" || recovered.Error == nil || *recovered.Error != message || recovered.CompletedAt == nil {
		t.Fatalf("unexpected recovered transcript: %#v", recovered)
	}
	untouched, err := repo.GetByID(completed.ID)
	if err != nil {
		t.Fatalf("get completed transcript: %v", err)
	}
	if untouched == nil || untouched.Status != "completed" || untouched.Error != nil {
		t.Fatalf("completed transcript was modified: %#v", untouched)
	}
}
