package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/video"
	"github.com/go-chi/chi/v5"
)

func TestVideoStartRenderReturnsConflictForStaleTimelineRevision(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "video-handler.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	projectRepo := repository.NewVideoProjectRepo(database)
	generationRepo := repository.NewVideoGenerationRepo(database)
	assetRepo := repository.NewVideoAssetRepo(database)
	timelineRepo := repository.NewVideoTimelineRepo(database)
	renderJobRepo := repository.NewVideoRenderJobRepo(database)
	attachmentsDir := filepath.Join(t.TempDir(), "attachments")
	service := video.NewService(projectRepo, generationRepo, assetRepo, timelineRepo, renderJobRepo, nil, attachmentsDir, nil)
	handler := NewVideoHandler(service, projectRepo, generationRepo, assetRepo, timelineRepo, renderJobRepo, nil, nil, nil, attachmentsDir)

	project, err := service.CreateProject("", "HTTP Revision", "openrouter", "test-model", 1280, 720, 30, "16:9")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	document := video.NewEmptyTimeline(project.Width, project.Height, project.FPS)
	first, _, err := service.SaveTimeline(context.Background(), "", project.ID, document)
	if err != nil {
		t.Fatalf("save first timeline: %v", err)
	}
	document.Canvas.Background = "#123456"
	if _, _, err := service.SaveTimeline(context.Background(), "", project.ID, document); err != nil {
		t.Fatalf("save newer timeline: %v", err)
	}
	payload, err := json.Marshal(video.StartRenderRequest{
		ExportSettings: video.ExportSettings{Format: "mp4", Resolution: "project", Quality: "standard"},
		RenderBinding:  video.RenderBinding{TimelineID: first.ID, TimelineRevision: first.Revision, TimelineSHA256: first.ContentSHA256},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/video/projects/"+project.ID+"/render", bytes.NewReader(payload))
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("projectId", project.ID)
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
	recorder := httptest.NewRecorder()

	handler.StartRender(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	jobs, err := renderJobRepo.ListByProject(project.ID)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("stale request created jobs: %+v", jobs)
	}
}

func TestVideoDiagnosticFrameRejectsInvalidIndex(t *testing.T) {
	handler := &VideoHandler{}
	request := httptest.NewRequest(http.MethodGet, "/v1/video/render-snapshots/snapshot/frames/not-a-number", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("snapshotId", "snapshot")
	routeContext.URLParams.Add("frameIndex", "not-a-number")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
	recorder := httptest.NewRecorder()
	handler.GetDiagnosticFrame(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}
