package video

import (
	"strings"
	"testing"
)

func TestBlurRegionPartsPixelateUsesNeighborForBothScalePasses(t *testing.T) {
	clip := TimelineClip{
		ID:         "c-pixelate",
		StartMS:    1000,
		DurationMS: 2000,
	}
	shape := TimelineShape{
		Kind:       ShapeKindPixelate,
		Width:      403,
		Height:     307,
		BlurRadius: 20,
	}

	parts, outLabel := blurRegionParts("[base_v]", clip, shape, 1920, 1080, 0)
	if len(parts) != 3 {
		t.Fatalf("pixelate region should emit split, sample, and overlay stages; got %d: %v", len(parts), parts)
	}
	if outLabel != "[t0_v]" {
		t.Fatalf("pixelate region should end the chain at [t0_v], got %q", outLabel)
	}

	// Non-divisible dimensions prove the renderer keeps the canonical floor
	// block policy: 403/20 -> 20 and 307/20 -> 15. Both passes must explicitly
	// use libswscale nearest-neighbor so export cannot fall back to FFmpeg's
	// default scaler on the reduction stage.
	want := "[bbs0]crop=403:307:758:386,scale=20:15:flags=neighbor,scale=403:307:flags=neighbor[bbl0]"
	if parts[1] != want {
		t.Fatalf("pixelate sampling graph mismatch\nwant: %s\n got: %s", want, parts[1])
	}
	if strings.Contains(parts[1], "scale=20:15,scale=") {
		t.Fatalf("pixelate downsample must never use FFmpeg's implicit scaler: %s", parts[1])
	}
}
