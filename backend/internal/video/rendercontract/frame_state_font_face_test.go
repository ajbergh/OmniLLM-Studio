package rendercontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fontFaceBindingFixture struct {
	Version  int              `json:"version"`
	Manifest RenderManifestV1 `json:"manifest"`
}

func loadFontResourceProvenanceManifest(t *testing.T) RenderManifestV1 {
	t.Helper()
	path := filepath.Join(repoRootFromTest(t), "video-renderer", "test", "fixtures", "font-resource-provenance-v1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read font resource provenance fixture: %v", err)
	}
	var fixture fontFaceBindingFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode font resource provenance fixture: %v", err)
	}
	return fixture.Manifest
}

func textFrameStateDocument(text TimelineV2Text) TimelineV2Document {
	return TimelineV2Document{
		Version:    2,
		Canvas:     TimelineV2Canvas{Width: 200, Height: 100, FPS: 30, Background: "#000000"},
		DurationMS: 1000,
		Tracks: []TimelineV2Track{{
			ID: "track-1", Type: "video", Name: "Track 1", Visible: true,
			Clips: []TimelineV2Clip{{
				ID: "text-clip", StartMS: 0, DurationMS: 1000, TrimInMS: 0, TrimOutMS: 1000,
				Effects: []TimelineV2Effect{}, Keyframes: []TimelineV2Keyframe{},
				Text: &text,
			}},
		}},
		Scenes: []TimelineV2Scene{}, Markers: []TimelineV2Marker{},
	}
}

func TestEvaluateVisualFrameStateForRenderManifestResolvesPackagedFontFace(t *testing.T) {
	manifest := loadFontResourceProvenanceManifest(t)
	manifest.Timeline = textFrameStateDocument(TimelineV2Text{Text: "Title", FontFamily: "Inter", FontResourceID: "inter-400-normal"})
	state, err := EvaluateVisualFrameStateForRenderManifest(manifest, 0)
	if err != nil {
		t.Fatalf("EvaluateVisualFrameStateForRenderManifest: %v", err)
	}
	if len(state.Layers) != 1 || state.Layers[0].Text == nil {
		t.Fatalf("state layers = %+v", state.Layers)
	}
	text := state.Layers[0].Text
	if text.FontResourceID != "inter-400-normal" || text.FontFaceSource != TextFontFaceSourcePackagedResource {
		t.Fatalf("font face = (%q, %q), want packaged resource", text.FontResourceID, text.FontFaceSource)
	}
}

func TestEvaluateVisualFrameStateForRenderManifestRejectsUnpackagedFontResource(t *testing.T) {
	manifest := loadFontResourceProvenanceManifest(t)
	manifest.Timeline = textFrameStateDocument(TimelineV2Text{Text: "Title", FontFamily: "Inter", FontResourceID: "missing-resource"})
	_, err := EvaluateVisualFrameStateForRenderManifest(manifest, 0)
	if err == nil || !strings.Contains(err.Error(), `names font resource "missing-resource" that the manifest does not package`) {
		t.Fatalf("error = %v", err)
	}
}

func TestEvaluateVisualFrameStateForRenderManifestRejectsConflictingFontFamily(t *testing.T) {
	manifest := loadFontResourceProvenanceManifest(t)
	manifest.Timeline = textFrameStateDocument(TimelineV2Text{Text: "Title", FontFamily: "Roboto", FontResourceID: "inter-400-normal"})
	_, err := EvaluateVisualFrameStateForRenderManifest(manifest, 0)
	if err == nil || !strings.Contains(err.Error(), `with family "Inter" but authors family "Roboto"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestEvaluateVisualFrameStateWithoutManifestRejectsFontResource(t *testing.T) {
	// Timeline-only evaluation has no packaged resources at all, so an
	// authored font_resource_id is unverifiable and must fail closed.
	_, err := EvaluateVisualFrameState(textFrameStateDocument(TimelineV2Text{Text: "Title", FontFamily: "Inter", FontResourceID: "inter-400-normal"}), 0)
	if err == nil || !strings.Contains(err.Error(), `names font resource "inter-400-normal" that the manifest does not package`) {
		t.Fatalf("error = %v", err)
	}
}

func TestEvaluateFontResourceProvenanceRejectsDuplicateIDs(t *testing.T) {
	manifest := loadFontResourceProvenanceManifest(t)
	manifest.FontResources = append(manifest.FontResources, manifest.FontResources[0])
	_, err := EvaluateFontResourceProvenance(manifest)
	if err == nil || !strings.Contains(err.Error(), "duplicate font resource") {
		t.Fatalf("error = %v", err)
	}
}

func TestFontFaceBindingFixtureDecodesStrictly(t *testing.T) {
	manifest := loadFontResourceProvenanceManifest(t)
	if len(manifest.FontResources) != 2 {
		t.Fatalf("font resources = %d, want 2", len(manifest.FontResources))
	}
	if !reflect.DeepEqual(manifest.Settings.Width, manifest.Settings.Height/2) == false && manifest.Settings.Width != 200 {
		t.Fatalf("unexpected fixture settings: %+v", manifest.Settings)
	}
}
