package video

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/models"
)

func newFontStagingTestService(t *testing.T) (*Service, string) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "font-staging.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	attachmentsDir := filepath.Join(t.TempDir(), "attachments")
	return newImportTestService(database, attachmentsDir), attachmentsDir
}

func createFontAsset(t *testing.T, service *Service, projectID, resourceID, fileName string, data []byte) models.VideoAsset {
	t.Helper()
	metadata, err := json.Marshal(map[string]any{"font_resource_id": resourceID})
	if err != nil {
		t.Fatalf("marshal font metadata: %v", err)
	}
	storageName := fileName + ".bin"
	writeTestFile(t, filepath.Join(service.attachmentsDir, storageName), data)
	asset := &models.VideoAsset{
		ProjectID:    &projectID,
		SourceType:   "upload",
		Kind:         "font",
		FileName:     fileName,
		FilePath:     storageName,
		MimeType:     "font/woff2",
		SizeBytes:    int64(len(data)),
		MetadataJSON: string(metadata),
	}
	if err := service.assets.Create(asset); err != nil {
		t.Fatalf("create font asset: %v", err)
	}
	return *asset
}

func fontStagingTimeline(resourceID string) TimelineDocument {
	doc := NewEmptyTimeline(640, 360, 30)
	doc.Tracks = append(doc.Tracks, TimelineTrack{
		ID: "text-track", Type: TrackTypeText, Name: "Text", Visible: true,
		Clips: []TimelineClip{{
			ID: "title-clip", StartMS: 0, DurationMS: 1000,
			Text: &TimelineText{Text: "Title", FontFamily: "Inter", FontResourceID: resourceID},
		}},
	})
	return doc
}

func TestBuildRenderFontManifestPackagesReferencedFaces(t *testing.T) {
	service, attachmentsDir := newFontStagingTestService(t)
	project, err := service.CreateProject("", "Font Project", "openrouter", "test-model", 640, 360, 30, "16:9")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	createFontAsset(t, service, project.ID, "inter-400-normal", "Inter-Regular.woff2", []byte("woff2-bytes-1"))
	createFontAsset(t, service, project.ID, "inter-700-bold", "Inter-Bold.woff2", []byte("woff2-bytes-2"))

	doc := fontStagingTimeline("inter-400-normal")
	entries, manifestJSON, manifestSHA256, err := service.buildRenderFontManifest(context.Background(), project.ID, "snap-fonts-1", doc)
	if err != nil {
		t.Fatalf("buildRenderFontManifest: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1 (unreferenced faces are not packaged)", len(entries))
	}
	entry := entries[0]
	if entry.Asset.FileName != "Inter-Regular.woff2" || len(entry.ClipIDs) != 1 || entry.ClipIDs[0] != "title-clip" {
		t.Fatalf("entry = %+v", entry)
	}
	if !strings.Contains(entry.Asset.FilePath, "/fonts/inter-400-normal/") && !strings.Contains(entry.Asset.FilePath, `\fonts\inter-400-normal\`) {
		t.Fatalf("staged path = %q, want fonts/<resource-id>/ segment", entry.Asset.FilePath)
	}
	stagedPath := filepath.Join(attachmentsDir, filepath.FromSlash(entry.Asset.FilePath))
	data, err := os.ReadFile(stagedPath)
	if err != nil {
		t.Fatalf("read staged font: %v", err)
	}
	if string(data) != "woff2-bytes-1" {
		t.Fatalf("staged bytes = %q", data)
	}
	if contentSHA256([]byte(manifestJSON)) != manifestSHA256 {
		t.Fatalf("manifest hash mismatch")
	}
}

func TestTimelineFontResourceClipIDsToleratesNonTextClips(t *testing.T) {
	// Regression: the collector dereferenced clip.Text before its nil check,
	// panicking StartRender for any timeline containing a non-text clip.
	doc := NewEmptyTimeline(640, 360, 30)
	doc.Tracks = append(doc.Tracks, TimelineTrack{
		ID: "media-track", Type: TrackTypeVideo, Name: "Media", Visible: true,
		Clips: []TimelineClip{{ID: "media-clip", AssetID: "asset-1", StartMS: 0, DurationMS: 1000}},
	})
	references := timelineFontResourceClipIDs(doc)
	if len(references) != 0 {
		t.Fatalf("references = %v, want empty", references)
	}
}

func TestBuildRenderFontManifestEmptyWithoutReferences(t *testing.T) {
	service, _ := newFontStagingTestService(t)
	project, err := service.CreateProject("", "Font Project", "openrouter", "test-model", 640, 360, 30, "16:9")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	doc := NewEmptyTimeline(640, 360, 30)
	entries, manifestJSON, manifestSHA256, err := service.buildRenderFontManifest(context.Background(), project.ID, "snap-fonts-2", doc)
	if err != nil {
		t.Fatalf("buildRenderFontManifest: %v", err)
	}
	if len(entries) != 0 || manifestJSON != "[]" || manifestSHA256 != contentSHA256([]byte("[]")) {
		t.Fatalf("empty manifest = (%d, %q, %q)", len(entries), manifestJSON, manifestSHA256)
	}
}

func TestBuildRenderFontManifestFailsClosedOnMissingResource(t *testing.T) {
	service, _ := newFontStagingTestService(t)
	project, err := service.CreateProject("", "Font Project", "openrouter", "test-model", 640, 360, 30, "16:9")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	doc := fontStagingTimeline("missing-face")
	if _, _, _, err := service.buildRenderFontManifest(context.Background(), project.ID, "snap-fonts-3", doc); err == nil ||
		!strings.Contains(err.Error(), `reference font resource "missing-face" that the project does not provide`) {
		t.Fatalf("error = %v", err)
	}
}

func TestBuildRenderFontManifestFailsClosedOnDuplicateResourceID(t *testing.T) {
	service, _ := newFontStagingTestService(t)
	project, err := service.CreateProject("", "Font Project", "openrouter", "test-model", 640, 360, 30, "16:9")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	createFontAsset(t, service, project.ID, "inter-400-normal", "First.woff2", []byte("a"))
	createFontAsset(t, service, project.ID, "inter-400-normal", "Second.woff2", []byte("b"))
	doc := fontStagingTimeline("inter-400-normal")
	if _, _, _, err := service.buildRenderFontManifest(context.Background(), project.ID, "snap-fonts-4", doc); err == nil ||
		!strings.Contains(err.Error(), `declares font resource "inter-400-normal" on both`) {
		t.Fatalf("error = %v", err)
	}
}

func TestStartRenderPersistsAndVerifiesFontManifest(t *testing.T) {
	service, _ := newFontStagingTestService(t)
	renderer := &snapshotCaptureRenderer{
		entered: make(chan struct{}), release: make(chan struct{}), requests: make(chan RenderRequest, 1),
	}
	service.renderer = renderer
	project, err := service.CreateProject("", "Font Render", "openrouter", "test-model", 640, 360, 30, "16:9")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	createFontAsset(t, service, project.ID, "inter-400-normal", "Inter-Regular.woff2", []byte("woff2-bytes"))
	doc := fontStagingTimeline("inter-400-normal")
	timeline, _, err := service.SaveTimeline(context.Background(), "", project.ID, doc)
	if err != nil {
		t.Fatalf("save timeline: %v", err)
	}
	binding := RenderBinding{TimelineID: timeline.ID, TimelineRevision: timeline.Revision, TimelineSHA256: timeline.ContentSHA256}
	job, err := service.StartRender(context.Background(), "", project.ID, ExportSettings{
		Format: "mp4", Codec: "h264", Resolution: "project", Quality: "standard",
	}, binding)
	if err != nil {
		t.Fatalf("start render: %v", err)
	}
	snapshot, err := service.renderJobs.GetSnapshot(*job.SnapshotID)
	if err != nil || snapshot == nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if !snapshotHasFontResources(snapshot) {
		t.Fatalf("snapshot did not package font resources: %q", snapshot.FontManifestJSON)
	}
	if contentSHA256([]byte(snapshot.FontManifestJSON)) != snapshot.FontManifestSHA256 {
		t.Fatalf("persisted font manifest hash mismatch")
	}
	if err := service.verifyFontsFromRenderSnapshot(snapshot); err != nil {
		t.Fatalf("verifyFontsFromRenderSnapshot: %v", err)
	}

	select {
	case <-renderer.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("renderer did not start")
	}
	close(renderer.release)
	waitForRenderTerminal(t, service, job.ID)
}

func TestVerifyFontsFromRenderSnapshotDetectsTamperedBytes(t *testing.T) {
	service, attachmentsDir := newFontStagingTestService(t)
	project, err := service.CreateProject("", "Font Tamper", "openrouter", "test-model", 640, 360, 30, "16:9")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	asset := createFontAsset(t, service, project.ID, "inter-400-normal", "Inter-Regular.woff2", []byte("original-bytes"))
	doc := fontStagingTimeline("inter-400-normal")
	entries, manifestJSON, manifestSHA256, err := service.buildRenderFontManifest(context.Background(), project.ID, "snap-fonts-5", doc)
	if err != nil {
		t.Fatalf("buildRenderFontManifest: %v", err)
	}
	snapshot := &models.VideoRenderSnapshot{
		ID: "snap-fonts-5", FontManifestJSON: manifestJSON, FontManifestSHA256: manifestSHA256,
	}
	if err := service.verifyFontsFromRenderSnapshot(snapshot); err != nil {
		t.Fatalf("clean verify failed: %v", err)
	}
	stagedPath := filepath.Join(attachmentsDir, filepath.FromSlash(entries[0].Asset.FilePath))
	if err := os.WriteFile(stagedPath, []byte("tampered-bytes"), 0o644); err != nil {
		t.Fatalf("tamper staged font: %v", err)
	}
	_ = asset
	if err := service.verifyFontsFromRenderSnapshot(snapshot); err == nil ||
		!strings.Contains(err.Error(), "changed after submission") {
		t.Fatalf("error = %v, want tamper rejection", err)
	}
}
