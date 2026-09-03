package video

import (
	"context"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

func TestAssetsFromRenderSnapshotIncludesVerifiedFontEntries(t *testing.T) {
	service, _ := newFontStagingTestService(t)
	project, err := service.CreateProject("", "Font Inputs", "openrouter", "test-model", 640, 360, 30, "16:9")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	font := createFontAsset(t, service, project.ID, "face-v1", "Face.ttf", []byte("immutable-font-bytes"))
	doc := fontStagingTimeline("face-v1")
	_, fontManifestJSON, fontManifestSHA256, err := service.buildRenderFontManifest(context.Background(), project.ID, "snap-font-inputs", doc)
	if err != nil {
		t.Fatalf("build font manifest: %v", err)
	}
	snapshot := &models.VideoRenderSnapshot{
		ID:                  "snap-font-inputs",
		AssetManifestJSON:   "[]",
		AssetManifestSHA256: contentSHA256([]byte("[]")),
		FontManifestJSON:    fontManifestJSON,
		FontManifestSHA256:  fontManifestSHA256,
	}
	inputs, err := service.assetsFromRenderSnapshot(snapshot)
	if err != nil {
		t.Fatalf("assetsFromRenderSnapshot: %v", err)
	}
	staged, ok := inputs[font.ID]
	if !ok {
		t.Fatalf("verified staged font %q missing from renderer inputs: %+v", font.ID, inputs)
	}
	if staged.Kind != "font" || fontResourceIDFromMetadata(staged) != "face-v1" {
		t.Fatalf("staged renderer font = %+v", staged)
	}
	if staged.FilePath == font.FilePath {
		t.Fatalf("renderer received mutable source path %q instead of staged snapshot path", staged.FilePath)
	}
}
