package video

import (
	"image/png"
	"math"
	"os"
	"strings"
	"testing"
)

func TestCanonicalCursorRasterExpansionUsesExactClickWindowAndSameTrack(t *testing.T) {
	doc := NewEmptyTimeline(640, 360, 100)
	doc.DurationMS = 1000
	doc.Tracks[0].Clips = []TimelineClip{{
		ID: "cursor-owner", AssetID: "media", StartMS: 0, DurationMS: 1000, TrimOutMS: 1000,
		Transform: map[string]any{"x": 0.0, "y": 0.0, "scale": 1.0, "rotation": 0.0, "opacity": 1.0},
		Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}, Transitions: []TimelineTransition{},
		Cursor: &TimelineCursor{
			Visible: true, Scale: 1, Highlight: true, ClickRings: true,
			Events: []TimelineCursorEvent{{TimeMS: 0, X: 100, Y: 100}, {TimeMS: 500, X: 200, Y: 140, Click: true}, {TimeMS: 999, X: 300, Y: 180}},
		},
	}}

	expanded := ExpandTimelineForFidelity(doc, 100, 120)
	if len(expanded.Tracks) != 1 {
		t.Fatalf("canonical cursor must stay on the owner track, got %d tracks", len(expanded.Tracks))
	}
	if got := len(expanded.Tracks[0].Clips); got != 101 {
		t.Fatalf("expanded clip count = %d, want owner + 100 frame-addressed cursor clips", got)
	}
	for _, clip := range expanded.Tracks[0].Clips[1:] {
		if clip.Text != nil || clip.Shape != nil || !strings.HasPrefix(clip.AssetID, cursorRasterAssetPrefix) {
			t.Fatalf("canonical cursor frame is not an image raster: %+v", clip)
		}
		if clip.Metadata[cursorRasterMetadataKey] != cursorRasterContractVersion {
			t.Fatalf("canonical cursor metadata = %+v", clip.Metadata)
		}
	}

	assertRing := func(frame int64, want bool) {
		t.Helper()
		filtered := FilterTimelineAtDiagnosticFrame(expanded, frame, 100, 0)
		found := false
		for _, clip := range filtered.Tracks[0].Clips {
			if clip.Metadata == nil || clip.Metadata[cursorRasterMetadataKey] != cursorRasterContractVersion {
				continue
			}
			found = true
			got, _ := clip.Metadata[cursorRasterClickRingKey].(bool)
			if got != want {
				t.Fatalf("frame %d ring = %v, want %v", frame, got, want)
			}
		}
		if !found {
			t.Fatalf("frame %d has no canonical cursor raster", frame)
		}
	}
	// The contract is strict <300 ms. Exactly 300 ms from the click is false.
	assertRing(20, false)
	assertRing(21, true)
	assertRing(79, true)
	assertRing(80, false)
}

func TestCanonicalCursorRasterAppliesStaticParentAffineToOrigin(t *testing.T) {
	doc := NewEmptyTimeline(640, 360, 100)
	doc.DurationMS = 10
	doc.Tracks[0].Clips = []TimelineClip{{
		ID: "cursor-owner", AssetID: "media", StartMS: 0, DurationMS: 10, TrimOutMS: 10,
		Transform: map[string]any{"x": 5.0, "y": 7.0, "scale": 2.0, "rotation": 90.0, "opacity": 0.75},
		Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}, Transitions: []TimelineTransition{},
		Cursor: &TimelineCursor{Visible: true, Scale: 1, Events: []TimelineCursorEvent{{TimeMS: 0, X: 330, Y: 180}}},
	}}

	expanded := ExpandTimelineForFidelity(doc, 100, 10)
	if len(expanded.Tracks[0].Clips) != 2 {
		t.Fatalf("expanded clips = %+v", expanded.Tracks[0].Clips)
	}
	cursor := expanded.Tracks[0].Clips[1]
	x, _ := numericTransform(cursor.Transform, "x")
	y, _ := numericTransform(cursor.Transform, "y")
	scale, _ := numericTransform(cursor.Transform, "scale")
	rotation, _ := numericTransform(cursor.Transform, "rotation")
	opacity, _ := numericTransform(cursor.Transform, "opacity")
	if math.Abs(x-5) > 1e-9 || math.Abs(y-27) > 1e-9 || scale != 2 || rotation != 90 || opacity != 0.75 {
		t.Fatalf("cursor affine transform = x=%v y=%v scale=%v rotation=%v opacity=%v", x, y, scale, rotation, opacity)
	}
}

func TestCanonicalCursorRasterFallsBackForSmoothing(t *testing.T) {
	doc := NewEmptyTimeline(640, 360, 30)
	doc.DurationMS = 1000
	doc.Tracks[0].Clips = []TimelineClip{{
		ID: "cursor-owner", AssetID: "media", StartMS: 0, DurationMS: 1000, TrimOutMS: 1000,
		Transform: map[string]any{"x": 0.0, "y": 0.0, "scale": 1.0, "rotation": 0.0, "opacity": 1.0},
		Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}, Transitions: []TimelineTransition{},
		Cursor: &TimelineCursor{Visible: true, Scale: 1, Smoothing: true, Events: []TimelineCursorEvent{{TimeMS: 0, X: 10, Y: 20}}},
	}}

	expanded := ExpandTimelineForFidelity(doc, 30, 120)
	foundLegacy := false
	for _, clip := range expanded.Tracks[0].Clips {
		if clip.Text != nil && clip.Text.Text == "➤" {
			foundLegacy = true
			break
		}
	}
	if !foundLegacy {
		t.Fatalf("smoothing cursor did not retain compatibility fallback: %+v", expanded.Tracks[0].Clips)
	}
}

func TestMaterializeCanonicalCursorRasterAssets(t *testing.T) {
	doc := NewEmptyTimeline(640, 360, 100)
	doc.DurationMS = 10
	doc.Tracks[0].Clips = []TimelineClip{{
		ID: "cursor-owner", AssetID: "media", StartMS: 0, DurationMS: 10, TrimOutMS: 10,
		Transform: map[string]any{"x": 0.0, "y": 0.0, "scale": 1.0, "rotation": 0.0, "opacity": 1.0},
		Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}, Transitions: []TimelineTransition{},
		Cursor: &TimelineCursor{Visible: true, Scale: 1, Highlight: true, ClickRings: true, Events: []TimelineCursorEvent{{TimeMS: 0, X: 320, Y: 180, Click: true}}},
	}}
	expanded := ExpandTimelineForFidelity(doc, 100, 10)
	req := RenderRequest{Timeline: expanded, Assets: map[string]models.VideoAsset{"media": {ID: "media"}}}
	cleanup, err := materializeCanonicalCursorRasterAssets(&req)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	var rasterPath string
	for id, asset := range req.Assets {
		if strings.HasPrefix(id, cursorRasterAssetPrefix) {
			rasterPath = asset.FilePath
			break
		}
	}
	if rasterPath == "" {
		t.Fatal("canonical cursor raster asset was not materialized")
	}
	file, err := os.Open(rasterPath)
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(file)
	_ = file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != 640 || config.Height != 360 {
		t.Fatalf("cursor raster dimensions = %dx%d", config.Width, config.Height)
	}
	cleanup()
	if _, err := os.Stat(rasterPath); !os.IsNotExist(err) {
		t.Fatalf("cursor raster cleanup left %q: %v", rasterPath, err)
	}
}
