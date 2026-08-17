package video

import (
	"strings"
	"testing"
)

func TestStrictParityIssuesIncludeStableTimelinePaths(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	doc.Tracks[0].Clips = []TimelineClip{{
		ID: "clip-1", DurationMS: 1000,
		Text:        &TimelineText{Text: "Hello"},
		Transitions: []TimelineTransition{{ID: "transition-1", Type: TransitionTypeWipe, DurationMS: 250}},
		Keyframes:   []TimelineKeyframe{{ID: "keyframe-1", Property: "x", TimeMS: 0, Value: 0}},
		Transform:   map[string]any{"rotation_x": 10.0},
	}}
	issues := StrictParityIssues(doc)
	for _, path := range []string{
		"tracks[0].clips[0].text",
		"tracks[0].clips[0].transitions[0]",
		"tracks[0].clips[0].keyframes[0]",
		"tracks[0].clips[0].transform.rotation_x",
	} {
		found := false
		for _, issue := range issues {
			if issue.Path == path {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing strict parity path %q in %+v", path, issues)
		}
	}
	err := strictParityError(issues)
	if err == nil || !strings.Contains(err.Error(), "tracks[0].clips[0]") {
		t.Fatalf("strict parity error = %v", err)
	}
}

func TestStrictParityAllowsTimelineWithoutKnownLegacyMismatches(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	doc.Tracks[0].Clips = []TimelineClip{{ID: "plain-media", AssetID: "asset-1", DurationMS: 1000}}
	if issues := StrictParityIssues(doc); len(issues) != 0 {
		t.Fatalf("plain timeline issues = %+v", issues)
	}
}
