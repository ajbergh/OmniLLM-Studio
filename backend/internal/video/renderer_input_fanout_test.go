package video

import (
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

func TestAppendResolvedClipInputsDeduplicatesSourcesAndFansOutStreams(t *testing.T) {
	clips := []resolvedClip{
		{filePath: "input-000.mp4", isVideo: true, hasAudio: true},
		{filePath: "input-000.mp4", isVideo: true},
		{filePath: "input-000.mp4", isVideo: true, hasAudio: true},
		{filePath: "input-001.png", isImage: true},
	}

	args, nextIdx := appendResolvedClipInputs([]string{"-hide_banner"}, clips, 1)
	joinedArgs := strings.Join(args, " ")
	if got := strings.Count(joinedArgs, "-i "); got != 2 {
		t.Fatalf("expected two unique FFmpeg media inputs, got %d: %s", got, joinedArgs)
	}
	if nextIdx != 3 {
		t.Fatalf("expected next input index 3, got %d", nextIdx)
	}
	for i := 0; i < 3; i++ {
		if clips[i].sourceInputIdx != 1 {
			t.Fatalf("clip %d should share source input 1, got %d", i, clips[i].sourceInputIdx)
		}
		if clips[i].inputIdx != i+1 {
			t.Fatalf("clip %d logical filter index = %d, want %d", i, clips[i].inputIdx, i+1)
		}
	}
	if clips[3].sourceInputIdx != 2 || clips[3].videoInputLabel != "[2:v]" {
		t.Fatalf("single image source should remain direct: %+v", clips[3])
	}

	graph := strings.Join(resolvedInputFanoutParts(clips, true, true), ";")
	if !strings.Contains(graph, "[1:v]split=3[input1_v0][input1_v1][input1_v2]") {
		t.Fatalf("missing deterministic video fanout: %s", graph)
	}
	if !strings.Contains(graph, "[1:a]asplit=2[input1_a0][input1_a1]") {
		t.Fatalf("missing deterministic audio fanout: %s", graph)
	}
}

func TestAppendResolvedClipInputsSelectsAlphaPreservingVP9DecoderOnce(t *testing.T) {
	clips := []resolvedClip{
		{filePath: "input-alpha.webm", isVideo: true, inputDecoder: "libvpx-vp9"},
		{filePath: "input-alpha.webm", isVideo: true},
		{filePath: "input-opaque.mp4", isVideo: true},
	}
	args, _ := appendResolvedClipInputs(nil, clips, 1)
	joined := strings.Join(args, " ")
	if strings.Count(joined, "-c:v libvpx-vp9") != 1 {
		t.Fatalf("alpha decoder should be applied once to the shared source: %s", joined)
	}
	if !strings.Contains(joined, "-c:v libvpx-vp9 -i input-alpha.webm") {
		t.Fatalf("alpha decoder must be scoped before its input: %s", joined)
	}
	if strings.Contains(joined, "libvpx-vp9 -i input-opaque.mp4") {
		t.Fatalf("opaque input must not inherit alpha decoder: %s", joined)
	}
}

func TestVideoAssetInputDecoderUsesFrozenAlphaFacts(t *testing.T) {
	asset := models.VideoAsset{MetadataJSON: `{"video_codec":"vp9","video_pixel_format":"yuv420p","video_alpha_mode":"1"}`}
	if got := videoAssetInputDecoder(asset); got != "libvpx-vp9" {
		t.Fatalf("decoder = %q, want libvpx-vp9", got)
	}
	asset.MetadataJSON = `{"video_codec":"vp9","video_pixel_format":"yuv420p"}`
	if got := videoAssetInputDecoder(asset); got != "" {
		t.Fatalf("opaque decoder = %q, want default", got)
	}
}

func TestResolvedInputLabelsPreserveLegacyDirectGraphTests(t *testing.T) {
	clip := resolvedClip{inputIdx: 7, isVideo: true, hasAudio: true}
	if got := resolvedVideoInputLabel(clip); got != "[7:v]" {
		t.Fatalf("legacy video label = %q", got)
	}
	if got := resolvedAudioInputLabel(clip); got != "[7:a]" {
		t.Fatalf("legacy audio label = %q", got)
	}
}
