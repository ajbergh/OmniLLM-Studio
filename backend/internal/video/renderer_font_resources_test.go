package video

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

func rendererFontAsset(t *testing.T, root, resourceID string) models.VideoAsset {
	t.Helper()
	fontDir := filepath.Join(root, "video", "render-snapshots", "snap-font", "fonts", resourceID)
	if err := os.MkdirAll(fontDir, 0o755); err != nil {
		t.Fatalf("mkdir font dir: %v", err)
	}
	fontPath := filepath.Join(fontDir, "Face.ttf")
	if err := os.WriteFile(fontPath, []byte("fixture-font-bytes"), 0o644); err != nil {
		t.Fatalf("write font: %v", err)
	}
	metadata, err := json.Marshal(map[string]any{"font_resource_id": resourceID})
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	return models.VideoAsset{
		ID: "font-asset", Kind: "font", FileName: "Face.ttf",
		FilePath: filepath.ToSlash(filepath.Join("video", "render-snapshots", "snap-font", "fonts", resourceID, "Face.ttf")),
		MimeType: "font/ttf", MetadataJSON: string(metadata),
	}
}

func TestValidateRenderFontResourcesRequiresImmutableFace(t *testing.T) {
	root := t.TempDir()
	doc := NewEmptyTimeline(640, 360, 30)
	doc.Tracks = append(doc.Tracks, TimelineTrack{ID: "text", Type: TrackTypeText, Visible: true, Clips: []TimelineClip{{
		ID: "title", StartMS: 0, DurationMS: 1000,
		Text: &TimelineText{Text: "Glyph parity", FontFamily: "DejaVu Sans", FontResourceID: "face-v1", FontSize: 42, Color: "#ffffff"},
	}}})
	req := RenderRequest{Timeline: doc, AttachmentsDir: root, Assets: map[string]models.VideoAsset{}}
	if err := validateRenderFontResources(req); err == nil || !strings.Contains(err.Error(), `require renderer font resource "face-v1"`) {
		t.Fatalf("missing font error = %v", err)
	}
	asset := rendererFontAsset(t, root, "face-v1")
	req.Assets[asset.ID] = asset
	if err := validateRenderFontResources(req); err != nil {
		t.Fatalf("valid font rejected: %v", err)
	}
}

func TestDrawTextFilterUsesExactResourceFile(t *testing.T) {
	root := t.TempDir()
	asset := rendererFontAsset(t, root, "face-v1")
	resources := renderFontResources{attachmentsDir: root, assets: map[string]models.VideoAsset{asset.ID: asset}}
	text := TimelineText{
		Text: "Glyph parity", FontFamily: "Some installed family", FontResourceID: "face-v1",
		FontSize: 42, Color: "#ffffff",
	}
	filter := drawTextFilterWithFontResources(TimelineClip{StartMS: 0, DurationMS: 1000}, text, 640, 360, resources)
	wantPath := filepath.Join(root, filepath.FromSlash(asset.FilePath))
	if !strings.Contains(filter, "fontfile='"+escapeDrawText(wantPath)+"'") {
		t.Fatalf("filter does not select immutable font file: %s", filter)
	}
	if strings.Contains(filter, "font='"+escapeDrawText(text.FontFamily)+"'") {
		t.Fatalf("resource-backed filter still permits fontconfig family selection: %s", filter)
	}
}
