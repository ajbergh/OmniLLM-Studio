package video

import (
	"strings"
	"testing"
)

func TestBlurRegionPartsPixelateUsesByteExactRGBNeighborSampling(t *testing.T) {
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
	// block policy: 403/20 -> 20 and 307/20 -> 15. Both passes explicitly use
	// libswscale nearest-neighbor with full_chroma_inp: packed RGBA otherwise
	// receives small RGB perturbations even when the geometric sample index is
	// correct, which breaks byte-exact browser/FFmpeg parity.
	want := "[bbs0]crop=403:307:758:386,scale=20:15:flags=neighbor+full_chroma_inp,scale=403:307:flags=neighbor+full_chroma_inp[bbl0]"
	if parts[1] != want {
		t.Fatalf("pixelate sampling graph mismatch\nwant: %s\n got: %s", want, parts[1])
	}
	if strings.Contains(parts[1], "scale=20:15,scale=") {
		t.Fatalf("pixelate downsample must never use FFmpeg's implicit scaler: %s", parts[1])
	}
	if !strings.Contains(parts[2], ":format=rgb[t0_v]") {
		t.Fatalf("pixelate patch must composite in RGB without an implicit YUV conversion: %s", parts[2])
	}
}

func TestBuildFilterComplexKeepsVisualCompositionInRGB(t *testing.T) {
	clip := TimelineClip{
		ID:         "image",
		StartMS:    0,
		DurationMS: 1000,
	}
	graph, videoLabel, audioLabel := buildFilterComplexWithAudio(
		TimelineDocument{},
		[]resolvedClip{{
			inputIdx:   1,
			trackIndex: 0,
			clip:       clip,
			isImage:    true,
		}},
		512,
		512,
		false,
	)

	if videoLabel != "[ov1_v]" {
		t.Fatalf("unexpected visual output label %q", videoLabel)
	}
	if audioLabel != "" {
		t.Fatalf("audio-disabled graph should not expose an audio label, got %q", audioLabel)
	}
	if !strings.Contains(graph, "[0:v]format=rgba,setpts=PTS-STARTPTS[base_v]") {
		t.Fatalf("visual base must enter the compositor as RGBA: %s", graph)
	}
	if !strings.Contains(graph, ":format=rgb[ov1_v]") {
		t.Fatalf("media overlay must preserve RGB until the output/codec boundary: %s", graph)
	}
}
