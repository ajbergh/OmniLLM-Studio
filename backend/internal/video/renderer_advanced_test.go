package video

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFidelityExpansionSamplesEasingAndCursor(t *testing.T) {
	doc := NewEmptyTimeline(640, 360, 30)
	doc.DurationMS = 1000
	doc.Tracks[0].Clips = []TimelineClip{{ID: "clip", StartMS: 0, DurationMS: 1000, TrimOutMS: 1000, Transform: map[string]any{"scale": 1.0, "opacity": 1.0}, Keyframes: []TimelineKeyframe{{ID: "a", TimeMS: 0, Property: "scale", Value: 0.5, Easing: rendererEasingLinear}, {ID: "b", TimeMS: 1000, Property: "scale", Value: 1.5, Easing: rendererEasingEaseInOut}}, Cursor: &TimelineCursor{Visible: true, ClickRings: true, Events: []TimelineCursorEvent{{TimeMS: 0, X: 10, Y: 20}, {TimeMS: 500, X: 100, Y: 120, Click: true}}}}}
	expanded := ExpandTimelineForFidelity(doc, 30, 120)
	if len(expanded.Tracks[0].Clips) < 2 {
		t.Fatalf("expected sampled media clips")
	}
	foundCursor := false
	for _, expandedClip := range expanded.Tracks[0].Clips {
		kind, _ := fidelityGeneratedIdentity(expandedClip)
		if kind == rendererFidelityKindCursorPointer {
			foundCursor = true
			break
		}
	}
	if !foundCursor {
		t.Fatalf("expected cursor overlay on the owner track")
	}
	first := expanded.Tracks[0].Clips[0]
	if first.Keyframes != nil {
		t.Fatalf("render segments must have static transforms")
	}
	if scale, _ := numericTransform(first.Transform, "scale"); scale <= 0.5 {
		t.Fatalf("expected eased sampled scale, got %v", scale)
	}
}

func TestFidelityExpansionKeepsOneContinuousAudioSource(t *testing.T) {
	doc := NewEmptyTimeline(640, 360, 30)
	doc.DurationMS = 1000
	doc.Tracks = []TimelineTrack{
		{ID: "audio", Type: TrackTypeAudio, Visible: true, Clips: []TimelineClip{{
			ID: "audio-clip", AssetID: "audio-asset", DurationMS: 1000, TrimOutMS: 1000,
			Keyframes: []TimelineKeyframe{{ID: "v0", Property: "volume", TimeMS: 0, Value: .2}, {ID: "v1", Property: "volume", TimeMS: 1000, Value: 1.5, Easing: rendererEasingEaseInOut}},
		}}},
		{ID: "video", Type: TrackTypeLayer, Visible: true, Clips: []TimelineClip{{
			ID: "video-clip", AssetID: "video-asset", DurationMS: 1000, TrimOutMS: 1000,
			Transform: map[string]any{"x": 0.0},
			Keyframes: []TimelineKeyframe{{ID: "x0", Property: "x", TimeMS: 0, Value: 0}, {ID: "x1", Property: "x", TimeMS: 1000, Value: 100}},
		}}},
	}
	expanded := ExpandTimelineForFidelity(doc, 30, 120)
	if len(expanded.Tracks[0].Clips) != 1 || expanded.Tracks[0].Clips[0].ID != "audio-clip" {
		t.Fatalf("audio track was segmented: %+v", expanded.Tracks[0].Clips)
	}
	visual, audioCopies := 0, 0
	for _, expandedClip := range expanded.Tracks[1].Clips {
		if expandedClip.AudioOnly {
			audioCopies++
			if expandedClip.DurationMS != 1000 || len(expandedClip.Keyframes) == 0 {
				t.Fatalf("audio copy lost continuous timing/keyframes: %+v", expandedClip)
			}
		} else {
			visual++
			if !expandedClip.Muted {
				t.Fatalf("sampled visual segment still contributes audio: %+v", expandedClip)
			}
		}
	}
	if visual < 2 || audioCopies != 1 {
		t.Fatalf("expanded visual/audio counts = %d/%d", visual, audioCopies)
	}
}

func TestAliasResolvedClipInputsReusesShortHardLink(t *testing.T) {
	sourceDir := t.TempDir()
	workDir := t.TempDir()
	source := filepath.Join(sourceDir, "source.mp4")
	if err := os.WriteFile(source, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	clips := aliasResolvedClipInputs(workDir, []resolvedClip{{filePath: source}, {filePath: source}})
	if clips[0].filePath != clips[1].filePath || filepath.IsAbs(clips[0].filePath) {
		t.Fatalf("aliases were not short/reused: %+v", clips)
	}
	if _, err := os.Stat(filepath.Join(workDir, clips[0].filePath)); err != nil {
		t.Fatalf("alias is not readable: %v", err)
	}
}

func TestRenderTextLayout(t *testing.T) {
	got := renderTextLayout("AB\nC", "right", 2)
	if got == "AB\nC" {
		t.Fatalf("expected letter spacing/alignment transform")
	}
}
