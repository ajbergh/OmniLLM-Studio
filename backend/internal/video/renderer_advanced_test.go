package video

import "testing"

func TestFidelityExpansionSamplesEasingAndCursor(t *testing.T) {
	doc := NewEmptyTimeline(640, 360, 30)
	doc.DurationMS = 1000
	doc.Tracks[0].Clips = []TimelineClip{{ID: "clip", StartMS: 0, DurationMS: 1000, TrimOutMS: 1000, Transform: map[string]any{"scale": 1.0, "opacity": 1.0}, Keyframes: []TimelineKeyframe{{ID: "a", TimeMS: 0, Property: "scale", Value: 0.5, Easing: rendererEasingLinear}, {ID: "b", TimeMS: 1000, Property: "scale", Value: 1.5, Easing: rendererEasingEaseInOut}}, Cursor: &TimelineCursor{Visible: true, ClickRings: true, Events: []TimelineCursorEvent{{TimeMS: 0, X: 10, Y: 20}, {TimeMS: 500, X: 100, Y: 120, Click: true}}}}}
	expanded := ExpandTimelineForFidelity(doc, 30, 120)
	if len(expanded.Tracks[0].Clips) < 2 {
		t.Fatalf("expected sampled media clips")
	}
	if len(expanded.Tracks) < 2 || len(expanded.Tracks[len(expanded.Tracks)-1].Clips) == 0 {
		t.Fatalf("expected cursor overlay track")
	}
	first := expanded.Tracks[0].Clips[0]
	if first.Keyframes != nil {
		t.Fatalf("render segments must have static transforms")
	}
	if scale, _ := numericTransform(first.Transform, "scale"); scale <= 0.5 {
		t.Fatalf("expected eased sampled scale, got %v", scale)
	}
}

func TestRenderTextLayout(t *testing.T) {
	got := renderTextLayout("AB\nC", "right", 2)
	if got == "AB\nC" {
		t.Fatalf("expected letter spacing/alignment transform")
	}
}
