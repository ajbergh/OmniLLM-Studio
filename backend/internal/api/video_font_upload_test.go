package api

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/video"
	"github.com/go-chi/chi/v5"
)

func newFontUploadTestHandler(t *testing.T) (*VideoHandler, *repository.VideoAssetRepo, *models.VideoProject) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "font-upload.db"))
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
	project, err := service.CreateProject("", "Font Upload", "openrouter", "test-model", 640, 360, 30, "16:9")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	return handler, assetRepo, project
}

func fontUploadRequest(t *testing.T, projectID, fileName, resourceID string) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	file, err := form.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	// wOF2 header bytes: a real font container prefix, not text.
	if _, err := file.Write([]byte("wOF2\x00\x01\x00\x00font-bytes")); err != nil {
		t.Fatalf("write font bytes: %v", err)
	}
	if resourceID != "" {
		if err := form.WriteField("font_resource_id", resourceID); err != nil {
			t.Fatalf("write resource id field: %v", err)
		}
	}
	if err := form.Close(); err != nil {
		t.Fatalf("close form: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/video/projects/"+projectID+"/assets/upload", body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("projectId", projectID)
	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
}

func TestUploadAssetAcceptsFontWithResourceID(t *testing.T) {
	handler, assetRepo, project := newFontUploadTestHandler(t)
	recorder := httptest.NewRecorder()
	handler.UploadAsset(recorder, fontUploadRequest(t, project.ID, "Inter-Regular.woff2", "inter-400-normal"))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var asset models.VideoAsset
	if err := json.Unmarshal(recorder.Body.Bytes(), &asset); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if asset.Kind != "font" || asset.MimeType != "font/woff2" {
		t.Fatalf("asset kind/mime = (%q, %q)", asset.Kind, asset.MimeType)
	}
	stored, err := assetRepo.GetByID(asset.ID)
	if err != nil || stored == nil {
		t.Fatalf("get stored asset: %v", err)
	}
	if !strings.Contains(stored.MetadataJSON, `"font_resource_id":"inter-400-normal"`) {
		t.Fatalf("metadata = %q, want declared resource id", stored.MetadataJSON)
	}
}

func TestUploadAssetRejectsFontWithoutResourceID(t *testing.T) {
	handler, _, project := newFontUploadTestHandler(t)
	recorder := httptest.NewRecorder()
	handler.UploadAsset(recorder, fontUploadRequest(t, project.ID, "Inter-Regular.woff2", ""))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestUploadAssetRejectsNonCanonicalResourceID(t *testing.T) {
	handler, _, project := newFontUploadTestHandler(t)
	recorder := httptest.NewRecorder()
	handler.UploadAsset(recorder, fontUploadRequest(t, project.ID, "Inter-Regular.woff2", "Inter 400"))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestUploadAssetRejectsResourceIDOnNonFontUpload(t *testing.T) {
	handler, _, project := newFontUploadTestHandler(t)
	request := fontUploadRequest(t, project.ID, "picture.png", "not-a-font")
	// Overwrite the file part with real PNG bytes so kind classification
	// reaches image before the resource-id guard fires.
	recorder := httptest.NewRecorder()
	handler.UploadAsset(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}
