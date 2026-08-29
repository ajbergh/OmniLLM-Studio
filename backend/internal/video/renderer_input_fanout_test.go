package video

import (
	"strings"
	"testing"
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

func TestResolvedInputLabelsPreserveLegacyDirectGraphTests(t *testing.T) {
	clip := resolvedClip{inputIdx: 7, isVideo: true, hasAudio: true}
	if got := resolvedVideoInputLabel(clip); got != "[7:v]" {
		t.Fatalf("legacy video label = %q", got)
	}
	if got := resolvedAudioInputLabel(clip); got != "[7:a]" {
		t.Fatalf("legacy audio label = %q", got)
	}
}
