package video

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

type snapshotCaptureRenderer struct {
	entered  chan struct{}
	release  chan struct{}
	requests chan RenderRequest
}

type shutdownCaptureRenderer struct {
	entered chan struct{}
	exited  chan struct{}
}

func (r *shutdownCaptureRenderer) Render(ctx context.Context, _ RenderRequest, _ func(RenderProgress)) (*RenderResult, error) {
	close(r.entered)
	defer close(r.exited)
	<-ctx.Done()
	return nil, ctx.Err()
}

func waitForRenderTerminal(t *testing.T, service *Service, jobID string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, err := service.GetRenderJob("", jobID)
		if err == nil && job != nil && (job.Status == "completed" || job.Status == "failed" || job.Status == "cancelled") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("render job %s did not become terminal", jobID)
}

func (r *snapshotCaptureRenderer) Render(ctx context.Context, req RenderRequest, progress func(RenderProgress)) (*RenderResult, error) {
	close(r.entered)
	select {
	case <-r.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	r.requests <- req
	return &RenderResult{
		MimeType: "video/mp4", FileName: "snapshot-test.mp4", Data: []byte("test-render"),
		DurationMS: req.Timeline.DurationMS, Width: req.Project.Width, Height: req.Project.Height,
		FPS: float64(req.Settings.FPS), Metadata: map[string]any{"renderer": "snapshot-test"},
	}, nil
}

func TestRenderUsesSubmittedTimelineSnapshot(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "snapshot.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	attachmentsDir := filepath.Join(t.TempDir(), "attachments")
	service := newImportTestService(database, attachmentsDir)
	renderer := &snapshotCaptureRenderer{
		entered: make(chan struct{}), release: make(chan struct{}), requests: make(chan RenderRequest, 1),
	}
	service.renderer = renderer
	project, err := service.CreateProject("", "Snapshot Render", "openrouter", "test-model", 1280, 720, 30, "16:9")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	doc := NewEmptyTimeline(project.Width, project.Height, project.FPS)
	doc.Canvas.Background = "#112233"
	timeline, _, err := service.SaveTimeline(context.Background(), "", project.ID, doc)
	if err != nil {
		t.Fatalf("save submitted timeline: %v", err)
	}
	binding := RenderBinding{
		TimelineID: timeline.ID, TimelineRevision: timeline.Revision, TimelineSHA256: timeline.ContentSHA256,
	}
	job, err := service.StartRender(context.Background(), "", project.ID, ExportSettings{
		Format: "mp4", Codec: "h264", Resolution: "project", Quality: "standard",
	}, binding)
	if err != nil {
		t.Fatalf("start render: %v", err)
	}
	if job.SnapshotID == nil || job.TimelineSHA256 != binding.TimelineSHA256 {
		t.Fatalf("job was not bound to submitted timeline: %+v", job)
	}

	select {
	case <-renderer.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("renderer did not start")
	}
	doc.Canvas.Background = "#abcdef"
	current, _, err := service.SaveTimeline(context.Background(), "", project.ID, doc)
	if err != nil {
		t.Fatalf("save edited timeline: %v", err)
	}
	close(renderer.release)
	select {
	case request := <-renderer.requests:
		if request.Timeline.Canvas.Background != "#112233" {
			t.Fatalf("renderer background = %q, want submitted #112233", request.Timeline.Canvas.Background)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("renderer request was not captured")
	}
	if _, err := service.StartRender(context.Background(), "", project.ID, ExportSettings{
		Format: "mp4", Resolution: "project", Quality: "standard",
	}, binding); !errors.Is(err, ErrTimelineRevisionConflict) {
		t.Fatalf("stale render binding error = %v", err)
	}
	if current.ContentSHA256 == binding.TimelineSHA256 || current.Revision <= binding.TimelineRevision {
		t.Fatalf("timeline did not advance: submitted=%+v current=%+v", binding, current)
	}
	waitForRenderTerminal(t, service, job.ID)
}

func TestServiceShutdownWaitsForActiveRenderWorkers(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "shutdown.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	service := newImportTestService(database, filepath.Join(t.TempDir(), "attachments"))
	renderer := &shutdownCaptureRenderer{entered: make(chan struct{}), exited: make(chan struct{})}
	service.renderer = renderer
	project, err := service.CreateProject("", "Shutdown", "openrouter", "test-model", 1280, 720, 30, "16:9")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	timeline, _, err := service.SaveTimeline(context.Background(), "", project.ID, NewEmptyTimeline(project.Width, project.Height, project.FPS))
	if err != nil {
		t.Fatalf("save timeline: %v", err)
	}
	if _, err := service.StartRender(context.Background(), "", project.ID, ExportSettings{
		Format: "mp4", Resolution: "project", Quality: "standard",
	}, RenderBinding{
		TimelineID: timeline.ID, TimelineRevision: timeline.Revision, TimelineSHA256: timeline.ContentSHA256,
	}); err != nil {
		t.Fatalf("start render: %v", err)
	}
	select {
	case <-renderer.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("renderer did not start")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := service.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown service: %v", err)
	}
	select {
	case <-renderer.exited:
	default:
		t.Fatal("shutdown returned before the active render worker exited")
	}
}

func TestRecoverInterruptedRenderJobUsesPersistedSnapshot(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "recovery.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	service := newImportTestService(database, filepath.Join(t.TempDir(), "attachments"))
	renderer := &snapshotCaptureRenderer{
		entered: make(chan struct{}), release: make(chan struct{}), requests: make(chan RenderRequest, 1),
	}
	service.renderer = renderer
	project, err := service.CreateProject("", "Recovery", "openrouter", "test-model", 1280, 720, 30, "16:9")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	doc := NewEmptyTimeline(project.Width, project.Height, project.FPS)
	doc.Canvas.Background = "#445566"
	timeline, _, err := service.SaveTimeline(context.Background(), "", project.ID, doc)
	if err != nil {
		t.Fatalf("save timeline: %v", err)
	}
	settingsJSON := `{"format":"mp4","resolution":"project","quality":"standard"}`
	snapshot := &models.VideoRenderSnapshot{
		ProjectID: project.ID, TimelineID: timeline.ID, TimelineRevision: timeline.Revision,
		TimelineJSON: timeline.TimelineJSON, TimelineSHA256: timeline.ContentSHA256,
		AssetManifestJSON: `[]`, AssetManifestSHA256: contentSHA256([]byte(`[]`)), SettingsJSON: settingsJSON,
		RenderContractVersion: doc.Version, Renderer: legacySnapshotRenderer, RendererVersion: legacySnapshotRendererVersion,
	}
	job := &models.VideoRenderJob{ProjectID: project.ID, TimelineID: timeline.ID, Status: "queued", SettingsJSON: settingsJSON}
	if err := service.renderJobs.CreateWithSnapshot(job, snapshot, nil); err != nil {
		t.Fatalf("create interrupted job: %v", err)
	}

	service.RecoverInterruptedRenderJobs()
	select {
	case <-renderer.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("recovered renderer did not start")
	}
	close(renderer.release)
	select {
	case request := <-renderer.requests:
		if request.Timeline.Canvas.Background != "#445566" {
			t.Fatalf("recovered renderer background = %q", request.Timeline.Canvas.Background)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("recovered renderer request was not captured")
	}
	waitForRenderTerminal(t, service, job.ID)
}

func TestStrictParitySubmissionCreatesNoJobForKnownMismatch(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "strict.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	service := newImportTestService(database, filepath.Join(t.TempDir(), "attachments"))
	project, err := service.CreateProject("", "Strict Parity", "openrouter", "test-model", 1280, 720, 30, "16:9")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	doc := NewEmptyTimeline(project.Width, project.Height, project.FPS)
	doc.Tracks[0].Clips = []TimelineClip{{ID: "text-clip", DurationMS: 1000, Text: &TimelineText{Text: "Not exact yet"}}}
	timeline, _, err := service.SaveTimeline(context.Background(), "", project.ID, doc)
	if err != nil {
		t.Fatalf("save timeline: %v", err)
	}
	_, err = service.StartRender(context.Background(), "", project.ID, ExportSettings{
		Format: "mp4", Resolution: "project", Quality: "standard", StrictParity: true,
	}, RenderBinding{TimelineID: timeline.ID, TimelineRevision: timeline.Revision, TimelineSHA256: timeline.ContentSHA256})
	if err == nil || !strings.Contains(err.Error(), "strict parity blocked") || !strings.Contains(err.Error(), "tracks[0].clips[0].text") {
		t.Fatalf("strict parity submission error = %v", err)
	}
	jobs, err := service.renderJobs.ListByProject(project.ID)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("strict parity rejection created jobs: %+v", jobs)
	}
}

func TestRenderAssetManifestDetectsMissingAndChangedSources(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "assets.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	attachmentsDir := filepath.Join(t.TempDir(), "attachments")
	service := newImportTestService(database, attachmentsDir)
	project, err := service.CreateProject("", "Asset Snapshot", "openrouter", "test-model", 1280, 720, 30, "16:9")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	doc := NewEmptyTimeline(project.Width, project.Height, project.FPS)
	doc.Tracks[0].Clips = []TimelineClip{{ID: "missing-clip", AssetID: "missing-asset", DurationMS: 1000}}
	if _, _, _, err := service.buildRenderAssetManifest(context.Background(), project.ID, "00000000-0000-0000-0000-000000000001", doc); err == nil || !strings.Contains(err.Error(), "missing asset") {
		t.Fatalf("missing source error = %v", err)
	}
	projectID := project.ID
	assetRepo := repository.NewVideoAssetRepo(database)
	corruptRelativePath := filepath.ToSlash(filepath.Join("video", project.ID, "corrupt.mp4"))
	corruptAbsolutePath := filepath.Join(attachmentsDir, filepath.FromSlash(corruptRelativePath))
	if err := os.MkdirAll(filepath.Dir(corruptAbsolutePath), 0o755); err != nil {
		t.Fatalf("create corrupt asset directory: %v", err)
	}
	if err := os.WriteFile(corruptAbsolutePath, []byte("not an mp4"), 0o644); err != nil {
		t.Fatalf("write corrupt asset: %v", err)
	}
	corruptAsset := &models.VideoAsset{
		ProjectID: &projectID, SourceType: "upload", Kind: "video", FileName: "corrupt.mp4",
		FilePath: corruptRelativePath, MimeType: "video/mp4", SizeBytes: 10,
	}
	if err := assetRepo.Create(corruptAsset); err != nil {
		t.Fatalf("create corrupt asset record: %v", err)
	}
	doc.Tracks[0].Clips = []TimelineClip{{ID: "corrupt-clip", AssetID: corruptAsset.ID, DurationMS: 1000}}
	if _, _, _, err := service.buildRenderAssetManifest(context.Background(), project.ID, "00000000-0000-0000-0000-000000000003", doc); err == nil || !strings.Contains(err.Error(), "corrupt or undecodable") {
		t.Fatalf("corrupt source error = %v", err)
	}

	relativePath := filepath.ToSlash(filepath.Join("video", project.ID, "source.bin"))
	absolutePath := filepath.Join(attachmentsDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		t.Fatalf("create asset directory: %v", err)
	}
	if err := os.WriteFile(absolutePath, []byte("original"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	asset := &models.VideoAsset{
		ProjectID: &projectID, SourceType: "upload", Kind: "other", FileName: "source.bin",
		FilePath: relativePath, MimeType: "application/octet-stream", SizeBytes: 8,
	}
	if err := assetRepo.Create(asset); err != nil {
		t.Fatalf("create asset record: %v", err)
	}
	doc.Tracks[0].Clips = []TimelineClip{{ID: "source-clip", AssetID: asset.ID, DurationMS: 1000}}
	entries, manifestJSON, manifestHash, err := service.buildRenderAssetManifest(context.Background(), project.ID, "00000000-0000-0000-0000-000000000002", doc)
	if err != nil {
		t.Fatalf("build manifest: %v", err)
	}
	if len(entries) != 1 || entries[0].Asset.FilePath == relativePath {
		t.Fatalf("asset was not rebound to durable snapshot storage: %+v", entries)
	}
	snapshot := &models.VideoRenderSnapshot{AssetManifestJSON: manifestJSON, AssetManifestSHA256: manifestHash}
	if _, err := service.assetsFromRenderSnapshot(snapshot); err != nil {
		t.Fatalf("verify unchanged source: %v", err)
	}
	if err := os.WriteFile(absolutePath, []byte("modified"), 0o644); err != nil {
		t.Fatalf("modify asset: %v", err)
	}
	if _, err := service.assetsFromRenderSnapshot(snapshot); err != nil {
		t.Fatalf("original source mutation affected staged snapshot: %v", err)
	}
	if err := os.Remove(absolutePath); err != nil {
		t.Fatalf("delete original asset: %v", err)
	}
	if _, err := service.assetsFromRenderSnapshot(snapshot); err != nil {
		t.Fatalf("original source deletion affected staged snapshot: %v", err)
	}
	stagedPath := filepath.Join(attachmentsDir, filepath.FromSlash(entries[0].Asset.FilePath))
	if err := os.WriteFile(stagedPath, []byte("modified"), 0o644); err != nil {
		t.Fatalf("modify staged asset: %v", err)
	}
	if _, err := service.assetsFromRenderSnapshot(snapshot); err == nil || !strings.Contains(err.Error(), "changed after submission") {
		t.Fatalf("changed source error = %v", err)
	}
}

func TestResolveRenderAssetPathWithoutSymlinksPreservesContainment(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "video", "asset.bin")
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatalf("create nested storage: %v", err)
	}
	if err := os.WriteFile(nested, []byte("asset"), 0o644); err != nil {
		t.Fatalf("write nested asset: %v", err)
	}
	resolved, err := resolveRenderAssetPathWithoutSymlinks(root, nested)
	if err != nil || resolved != nested {
		t.Fatalf("resolve contained asset = %q, %v", resolved, err)
	}

	outside := filepath.Join(filepath.Dir(root), "outside.bin")
	if _, err := resolveRenderAssetPathWithoutSymlinks(root, outside); err == nil || !strings.Contains(err.Error(), "escapes storage root") {
		t.Fatalf("outside path error = %v", err)
	}

	link := filepath.Join(root, "linked.bin")
	if err := os.Symlink(nested, link); err != nil {
		t.Logf("symlink assertion skipped on this host: %v", err)
	} else if _, err := resolveRenderAssetPathWithoutSymlinks(root, link); err == nil || !strings.Contains(err.Error(), "symlink or reparse point") {
		t.Fatalf("symlink path error = %v", err)
	}
}
