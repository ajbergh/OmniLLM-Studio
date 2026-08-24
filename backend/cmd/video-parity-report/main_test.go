package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/video"
)

func TestLoadPairsAttachesRegionsByCanonicalFrameIndex(t *testing.T) {
	previewDir := filepath.Join(t.TempDir(), "preview")
	renderedDir := filepath.Join(t.TempDir(), "rendered")
	if err := os.MkdirAll(previewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(renderedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{previewDir, renderedDir} {
		writeTestPNG(t, filepath.Join(dir, "15-camera.png"))
		writeTestPNG(t, filepath.Join(dir, "30-title.png"))
	}

	policy := map[int64][]video.ParityRegion{
		15: {{
			Name:   "camera-structure",
			Bounds: video.ParityBounds{MinX: 0, MinY: 0, MaxX: 1, MaxY: 1},
		}},
	}
	pairs, err := loadPairs(previewDir, renderedDir, 30, policy)
	if err != nil {
		t.Fatalf("loadPairs: %v", err)
	}
	if len(pairs) != 2 {
		t.Fatalf("pair count = %d, want 2", len(pairs))
	}
	if pairs[0].FrameIndex != 15 || len(pairs[0].Regions) != 1 || pairs[0].Regions[0].Name != "camera-structure" {
		t.Fatalf("frame 15 pair = %#v", pairs[0])
	}
	if pairs[1].FrameIndex != 30 || len(pairs[1].Regions) != 0 {
		t.Fatalf("frame 30 pair = %#v", pairs[1])
	}

	pairs[0].Regions[0].Name = "mutated"
	if policy[15][0].Name != "camera-structure" {
		t.Fatalf("loadPairs mutated region policy: %#v", policy[15])
	}
}

func TestLoadPairsRejectsRegionOutsideDecodedFrame(t *testing.T) {
	previewDir := filepath.Join(t.TempDir(), "preview")
	renderedDir := filepath.Join(t.TempDir(), "rendered")
	if err := os.MkdirAll(previewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(renderedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestPNG(t, filepath.Join(previewDir, "15-camera.png"))
	writeTestPNG(t, filepath.Join(renderedDir, "15-camera.png"))

	policy := map[int64][]video.ParityRegion{
		15: {{
			Name:   "outside",
			Bounds: video.ParityBounds{MinX: 0, MinY: 0, MaxX: 2, MaxY: 1},
		}},
	}
	_, err := loadPairs(previewDir, renderedDir, 30, policy)
	if err == nil || !strings.Contains(err.Error(), "exceed decoded frame dimensions") {
		t.Fatalf("error = %v, want decoded-frame bounds failure", err)
	}
}

func TestLoadPairsWithoutRegionPolicyPreservesExistingBehavior(t *testing.T) {
	previewDir := filepath.Join(t.TempDir(), "preview")
	renderedDir := filepath.Join(t.TempDir(), "rendered")
	if err := os.MkdirAll(previewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(renderedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestPNG(t, filepath.Join(previewDir, "0-start.png"))
	writeTestPNG(t, filepath.Join(renderedDir, "0-start.png"))

	pairs, err := loadPairs(previewDir, renderedDir, 30, nil)
	if err != nil {
		t.Fatalf("loadPairs: %v", err)
	}
	if len(pairs) != 1 || pairs[0].Name != "start" || pairs[0].FrameIndex != 0 || pairs[0].TimeMS != 0 {
		t.Fatalf("pair = %#v", pairs)
	}
	if pairs[0].Regions != nil {
		t.Fatalf("regions = %#v, want nil", pairs[0].Regions)
	}
}

func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}
