package video

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func roundedRectangleTestClip() TimelineClip {
	return TimelineClip{
		ID:         "rounded-owner",
		StartMS:    0,
		DurationMS: 1000,
		TrimOutMS:  1000,
		Transform: map[string]any{
			"x": 0.0, "y": 0.0, "scale": 1.0, "rotation": 0.0, "opacity": 1.0,
		},
		Shape: &TimelineShape{
			Kind: ShapeKindRoundedRectangle, Width: 240, Height: 120,
			Fill: "rgba(10,20,30,0.5)", Stroke: "#f59e0b", StrokeWidth: 8, CornerRadius: 24,
		},
		Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}, Transitions: []TimelineTransition{},
	}
}

func TestCanonicalRoundedRectangleRasterClipUsesCanonicalShapeState(t *testing.T) {
	clip := roundedRectangleTestClip()
	generated, ok := canonicalRoundedRectangleRasterClip(clip, TimelineCanvas{Width: 640, Height: 360, FPS: 30}, nil)
	if !ok {
		t.Fatal("expected static rounded rectangle to use canonical raster path")
	}
	if generated.Shape != nil {
		t.Fatalf("generated clip retained authored shape: %+v", generated.Shape)
	}
	if !strings.HasPrefix(generated.AssetID, shapeRasterAssetPrefix) {
		t.Fatalf("generated asset id = %q", generated.AssetID)
	}
	if generated.Metadata[shapeRasterMetadataKey] != shapeRasterContractVersion {
		t.Fatalf("generated raster metadata = %+v", generated.Metadata)
	}
	if got, _ := generated.Metadata[shapeRasterCornerRadiusKey].(float64); got != 24 {
		t.Fatalf("corner radius metadata = %v, want 24", got)
	}
	if generated.Transform["scale"] != 1.0 || generated.Transform["rotation"] != 0.0 {
		t.Fatalf("parent transform changed: %+v", generated.Transform)
	}
}

func TestCanonicalRoundedRectangleRasterClipFailsClosedForUnsupportedParent(t *testing.T) {
	cases := []struct {
		name string
		edit func(*TimelineClip)
	}{
		{"effect", func(clip *TimelineClip) { clip.Effects = []TimelineEffect{{ID: "fx", Type: EffectTypeBlur, Enabled: true}} }},
		{"keyframe", func(clip *TimelineClip) { clip.Keyframes = []TimelineKeyframe{{ID: "kf", Property: "x", TimeMS: 0, Value: 20}} }},
		{"crop", func(clip *TimelineClip) { clip.Transform["crop"] = map[string]any{"left": 0.1} }},
		{"3d", func(clip *TimelineClip) { clip.Transform["rotation_x"] = 12.0 }},
		{"cursor", func(clip *TimelineClip) { clip.Cursor = &TimelineCursor{Visible: true, Events: []TimelineCursorEvent{{TimeMS: 0, X: 1, Y: 1}}} }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			clip := roundedRectangleTestClip()
			testCase.edit(&clip)
			if _, ok := canonicalRoundedRectangleRasterClip(clip, TimelineCanvas{Width: 640, Height: 360, FPS: 30}, nil); ok {
				t.Fatalf("unsupported %s parent entered canonical shape raster path", testCase.name)
			}
		})
	}
}

func TestWriteRoundedRectangleRasterPNGPreservesRoundedCornersAndStroke(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rounded.png")
	spec := roundedRectangleRasterSpec{Width: 240, Height: 120, Fill: "rgba(10,20,30,0.5)", Stroke: "#f59e0b", StrokeWidth: 8, CornerRadius: 24}
	if err := writeRoundedRectangleRasterPNG(path, 640, 360, spec); err != nil {
		t.Fatalf("write rounded rectangle raster: %v", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatalf("decode rounded rectangle raster: %v", err)
	}
	// Outer box spans x=200..440, y=120..240. A true rounded corner must
	// leave the extreme corner transparent while the top-center is stroke.
	_, _, _, cornerA := img.At(200, 120).RGBA()
	if cornerA != 0 {
		t.Fatalf("rounded outer corner alpha = %d, want 0", cornerA)
	}
	strokeR, strokeG, strokeB, strokeA := img.At(320, 121).RGBA()
	if strokeA == 0 || strokeR <= strokeG || strokeG <= strokeB {
		t.Fatalf("top-center stroke pixel = (%d,%d,%d,%d), want opaque amber-dominant stroke", strokeR, strokeG, strokeB, strokeA)
	}
	fillR, fillG, fillB, fillA := img.At(320, 180).RGBA()
	if fillA == 0 || fillA >= 0xffff || fillR >= fillG || fillG >= fillB {
		t.Fatalf("center fill pixel = (%d,%d,%d,%d), want semi-transparent 10/20/30 fill", fillR, fillG, fillB, fillA)
	}
}

func TestMaterializeCanonicalShapeRasterAssetsRegistersAndCleansGeneratedPNG(t *testing.T) {
	clip := roundedRectangleTestClip()
	generated, ok := canonicalRoundedRectangleRasterClip(clip, TimelineCanvas{Width: 640, Height: 360, FPS: 30}, nil)
	if !ok {
		t.Fatal("expected canonical raster clip")
	}
	req := RenderRequest{Timeline: TimelineDocument{Canvas: TimelineCanvas{Width: 640, Height: 360, FPS: 30}, Tracks: []TimelineTrack{{ID: "shape", Type: TrackTypeShape, Visible: true, Clips: []TimelineClip{generated}}}}, Assets: map[string]models.VideoAsset{}}
	cleanup, err := materializeCanonicalShapeRasterAssets(&req)
	if err != nil {
		t.Fatalf("materialize shape raster: %v", err)
	}
	asset, ok := req.Assets[generated.AssetID]
	if !ok || asset.FilePath == "" {
		cleanup()
		t.Fatalf("generated raster asset not registered: %+v", req.Assets)
	}
	if _, err := os.Stat(asset.FilePath); err != nil {
		cleanup()
		t.Fatalf("generated raster path not materialized: %v", err)
	}
	cleanup()
	if _, err := os.Stat(asset.FilePath); !os.IsNotExist(err) {
		t.Fatalf("generated raster path survived cleanup: %v", err)
	}
}

func TestParseShapeRasterColorSupportsCanonicalSRGBForms(t *testing.T) {
	for _, value := range []string{"transparent", "#abc", "#abcd", "#112233", "#11223344", "rgb(1, 2, 3)", "rgba(1,2,3,0.5)"} {
		if _, ok := parseShapeRasterColor(value); !ok {
			t.Fatalf("expected %q to parse", value)
		}
	}
	for _, value := range []string{"red", "hsl(1,2%,3%)", "rgba(1,2,3,2)"} {
		if _, ok := parseShapeRasterColor(value); ok {
			t.Fatalf("expected unsupported color %q to fail closed", value)
		}
	}
}
