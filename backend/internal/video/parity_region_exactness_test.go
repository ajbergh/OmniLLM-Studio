package video

import (
	"image"
	"image/color"
	"testing"
)

func TestParityStructuralRegionRequiresEntireConfiguredArea(t *testing.T) {
	preview := image.NewRGBA(image.Rect(0, 0, 4, 4))
	rendered := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			pixel := color.RGBA{R: 20, G: 40, B: 60, A: 255}
			preview.SetRGBA(x, y, pixel)
			rendered.SetRGBA(x, y, pixel)
		}
	}
	pair := ParityFramePair{
		Name: "clipped-structural-region",
		Preview: preview,
		Rendered: rendered,
		Regions: []ParityRegion{{
			Name: "must-be-fully-compared",
			Bounds: ParityBounds{MinX: 3, MinY: 3, MaxX: 6, MaxY: 6},
		}},
	}
	metric := CompareParityFrame(pair, DefaultParityThresholds())
	if len(metric.StructuralRegions) != 1 {
		t.Fatalf("structural regions = %+v", metric.StructuralRegions)
	}
	region := metric.StructuralRegions[0]
	if region.ComparedPixels != 1 || region.Exact {
		t.Fatalf("clipped structural region incorrectly passed: %+v", region)
	}
	if metric.Pass {
		t.Fatalf("frame with clipped structural policy region must fail: %+v", metric)
	}
}
