package repository_test

import (
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

func TestVideoTimelineRevisionAndRenderSnapshotLease(t *testing.T) {
	database := newTestDB(t)
	projects := repository.NewVideoProjectRepo(database)
	timelines := repository.NewVideoTimelineRepo(database)
	renderJobs := repository.NewVideoRenderJobRepo(database)

	project, err := projects.Create("", "Snapshot Test", "openrouter", "test-model", 1280, 720, 30, "16:9")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	timeline := &models.VideoTimeline{
		ProjectID: project.ID, Active: true, TimelineJSON: `{"version":1}`, DurationMS: 1000,
	}
	if err := timelines.Create(timeline); err != nil {
		t.Fatalf("create timeline: %v", err)
	}
	if timeline.Revision != 1 || len(timeline.ContentSHA256) != 64 {
		t.Fatalf("initial timeline identity = revision %d hash %q", timeline.Revision, timeline.ContentSHA256)
	}
	firstHash := timeline.ContentSHA256
	timeline.TimelineJSON = `{"version":2}`
	if err := timelines.Save(timeline); err != nil {
		t.Fatalf("save timeline: %v", err)
	}
	if timeline.Revision != 2 || timeline.ContentSHA256 == firstHash {
		t.Fatalf("saved timeline identity = revision %d hash %q", timeline.Revision, timeline.ContentSHA256)
	}

	snapshot := &models.VideoRenderSnapshot{
		ProjectID: project.ID, TimelineID: timeline.ID, TimelineRevision: timeline.Revision,
		TimelineJSON: timeline.TimelineJSON, TimelineSHA256: timeline.ContentSHA256,
		AssetManifestJSON: `[]`, AssetManifestSHA256: "manifest-hash", SettingsJSON: `{}`,
		RenderContractVersion: 1, Renderer: "ffmpeg", RendererVersion: "test-v1",
	}
	job := &models.VideoRenderJob{
		ProjectID: project.ID, TimelineID: timeline.ID, SettingsJSON: `{}`,
	}
	if err := renderJobs.CreateWithSnapshot(job, snapshot, []models.VideoRenderSnapshotAsset{{
		AssetID: "asset-1", FileSHA256: "asset-hash", SizeBytes: 42,
	}}); err != nil {
		t.Fatalf("create job with snapshot: %v", err)
	}
	fetched, err := renderJobs.GetByID(job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if fetched == nil || fetched.SnapshotID == nil || *fetched.SnapshotID != snapshot.ID {
		t.Fatalf("job snapshot binding = %+v", fetched)
	}
	if fetched.TimelineRevision != timeline.Revision || fetched.TimelineSHA256 != timeline.ContentSHA256 {
		t.Fatalf("job timeline identity = revision %d hash %q", fetched.TimelineRevision, fetched.TimelineSHA256)
	}
	active, err := renderJobs.HasActiveAssetReference("asset-1")
	if err != nil || !active {
		t.Fatalf("active asset lease = %v, %v", active, err)
	}
	if err := renderJobs.MarkFailed(job.ID, "test complete"); err != nil {
		t.Fatalf("finish job: %v", err)
	}
	active, err = renderJobs.HasActiveAssetReference("asset-1")
	if err != nil || active {
		t.Fatalf("terminal asset lease = %v, %v", active, err)
	}
	if err := renderJobs.Delete(job.ID); err != nil {
		t.Fatalf("delete job: %v", err)
	}
	deletedSnapshot, err := renderJobs.GetSnapshot(snapshot.ID)
	if err != nil || deletedSnapshot != nil {
		t.Fatalf("snapshot after job deletion = %+v, %v", deletedSnapshot, err)
	}
}

func TestLegacyRenderJobIsExplicitlyLabeled(t *testing.T) {
	database := newTestDB(t)
	projects := repository.NewVideoProjectRepo(database)
	timelines := repository.NewVideoTimelineRepo(database)
	renderJobs := repository.NewVideoRenderJobRepo(database)
	project, err := projects.Create("", "Legacy Test", "openrouter", "test-model", 1280, 720, 30, "16:9")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	timeline := &models.VideoTimeline{ProjectID: project.ID, Active: true, TimelineJSON: `{}`, DurationMS: 1000}
	if err := timelines.Create(timeline); err != nil {
		t.Fatalf("create timeline: %v", err)
	}
	job := &models.VideoRenderJob{ProjectID: project.ID, TimelineID: timeline.ID, SettingsJSON: `{}`}
	if err := renderJobs.Create(job); err != nil {
		t.Fatalf("create legacy job: %v", err)
	}
	fetched, err := renderJobs.GetByID(job.ID)
	if err != nil {
		t.Fatalf("get legacy job: %v", err)
	}
	if fetched.RenderSourceMode != "legacy_mutable_source" || fetched.ExactSourceAvailable {
		t.Fatalf("legacy source identity = %+v", fetched)
	}
}
