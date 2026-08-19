package rendercontract

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type mediaGeometryFixture struct {
	Version int              `json:"version"`
	Canvas  TimelineV2Canvas `json:"canvas"`
	Cases   []struct {
		Name                        string                  `json:"name"`
		Fit                         string                  `json:"fit"`
		ContentBounds               TimelineV2ContentBounds `json:"content_bounds"`
		MaskSourceCrop              *TimelineV2Crop         `json:"mask_source_crop"`
		TransformCrop               *TimelineV2Crop         `json:"transform_crop"`
		ExpectedScaleX              float64                 `json:"expected_scale_x"`
		ExpectedScaleY              float64                 `json:"expected_scale_y"`
		ExpectedVisibleSourceBounds TimelineV2ContentBounds `json:"expected_visible_source_bounds"`
		ExpectedPaintedBounds       TimelineV2ContentBounds `json:"expected_painted_bounds"`
		ExpectedClipBounds          TimelineV2ContentBounds `json:"expected_clip_bounds"`
	} `json:"cases"`
}

func TestEvaluateMediaGeometryMatchesSharedFixture(t *testing.T) {
	fixture := loadMediaGeometryFixture(t)
	for _, sample := range fixture.Cases {
		t.Run(sample.Name, func(t *testing.T) {
			clip := TimelineV2Clip{
				ID: "media", AssetID: "asset", MediaFit: sample.Fit,
				ContentBounds: &sample.ContentBounds, MaskSourceCrop: sample.MaskSourceCrop,
				Transform: &TimelineV2Transform{Crop: sample.TransformCrop},
			}
			geometry, err := EvaluateMediaGeometry(fixture.Canvas, clip)
			if err != nil {
				t.Fatalf("EvaluateMediaGeometry: %v", err)
			}
			if geometry.ContractVersion != MediaGeometryContractV1 || geometry.Fit != sample.Fit {
				t.Fatalf("geometry contract/fit = %q/%q", geometry.ContractVersion, geometry.Fit)
			}
			assertGeometryClose(t, "scale_x", geometry.ScaleX, sample.ExpectedScaleX)
			assertGeometryClose(t, "scale_y", geometry.ScaleY, sample.ExpectedScaleY)
			assertBoundsClose(t, "visible_source_bounds", geometry.VisibleSourceBounds, sample.ExpectedVisibleSourceBounds)
			assertBoundsClose(t, "painted_bounds", geometry.PaintedBounds, sample.ExpectedPaintedBounds)
			assertBoundsClose(t, "clip_bounds", geometry.ClipBounds, sample.ExpectedClipBounds)
		})
	}
}

func TestEvaluateMediaGeometryRequiresExplicitSourceBounds(t *testing.T) {
	_, err := EvaluateMediaGeometry(TimelineV2Canvas{Width: 200, Height: 100, FPS: 30}, TimelineV2Clip{AssetID: "asset", MediaFit: "contain"})
	if err == nil {
		t.Fatal("expected missing content_bounds to fail closed")
	}
}

func loadMediaGeometryFixture(t *testing.T) mediaGeometryFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve media geometry fixture path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "video-renderer", "test", "fixtures", "media-geometry-v1.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read media geometry fixture: %v", err)
	}
	var fixture mediaGeometryFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode media geometry fixture: %v", err)
	}
	if fixture.Version != 1 {
		t.Fatalf("fixture version = %d, want 1", fixture.Version)
	}
	return fixture
}

func assertBoundsClose(t *testing.T, label string, got, want TimelineV2ContentBounds) {
	t.Helper()
	assertGeometryClose(t, label+".x", got.X, want.X)
	assertGeometryClose(t, label+".y", got.Y, want.Y)
	assertGeometryClose(t, label+".width", got.Width, want.Width)
	assertGeometryClose(t, label+".height", got.Height, want.Height)
}

func assertGeometryClose(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("%s = %.12f, want %.12f", label, got, want)
	}
}
