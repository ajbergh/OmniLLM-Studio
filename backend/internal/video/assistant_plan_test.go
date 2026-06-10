package video

import (
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

func planTestDoc() TimelineDocument {
	doc := NewEmptyTimeline(1920, 1080, 30)
	doc.Tracks[0].Clips = []TimelineClip{{
		ID:         "clip-a",
		AssetID:    "asset-a",
		StartMS:    0,
		DurationMS: 5000,
		TrimOutMS:  5000,
	}}
	doc.DurationMS = 30000
	validated, err := ValidateTimelineDocument(doc)
	if err != nil {
		panic(err)
	}
	return validated
}

func TestValidateEditPlanOperations(t *testing.T) {
	doc := planTestDoc()
	plan := EditPlan{Operations: []EditOperation{
		{Type: "trim_clip", ClipID: "clip-a", DurationMS: 2000},
		{Type: "move_clip", ClipID: "clip-a", StartMS: 1000, TrackID: "track-video-1"},
		{Type: "move_clip", ClipID: "clip-a", StartMS: 1000, TrackID: "track-missing"},
		{Type: "delete_clip", ClipID: "clip-missing"},
		{Type: "add_text_clip", Text: "Hello", StartMS: 0, DurationMS: 2000},
		{Type: "explode_timeline"},
	}}
	valid, preview, issues := ValidateEditPlanOperations(doc, plan)
	if len(valid) != 3 {
		t.Fatalf("expected 3 valid operations, got %d (issues: %v)", len(valid), issues)
	}
	if len(preview) != 3 {
		t.Fatalf("expected 3 preview lines, got %v", preview)
	}
	if len(issues) != 3 {
		t.Fatalf("expected 3 issues, got %v", issues)
	}
	joined := strings.Join(issues, "\n")
	for _, expect := range []string{"track-missing", "clip-missing", "unsupported operation type"} {
		if !strings.Contains(joined, expect) {
			t.Errorf("issues missing %q: %v", expect, issues)
		}
	}
}

func TestApplyEditPlanToTimelineMoveAndDelete(t *testing.T) {
	doc := planTestDoc()
	moved, err := ApplyEditPlanToTimeline(doc, EditPlan{Operations: []EditOperation{
		{Type: "move_clip", ClipID: "clip-a", StartMS: 4000, TrackID: "track-overlay-1"},
	}})
	if err != nil {
		t.Fatalf("move_clip failed: %v", err)
	}
	ti, ci, found := findTimelineClip(moved, "clip-a")
	if !found {
		t.Fatalf("clip lost after move")
	}
	if moved.Tracks[ti].ID != "track-overlay-1" || moved.Tracks[ti].Clips[ci].StartMS != 4000 {
		t.Fatalf("clip not moved correctly: track=%s start=%d", moved.Tracks[ti].ID, moved.Tracks[ti].Clips[ci].StartMS)
	}

	deleted, err := ApplyEditPlanToTimeline(moved, EditPlan{Operations: []EditOperation{
		{Type: "delete_clip", ClipID: "clip-a"},
	}})
	if err != nil {
		t.Fatalf("delete_clip failed: %v", err)
	}
	if _, _, found := findTimelineClip(deleted, "clip-a"); found {
		t.Fatalf("clip should be deleted")
	}
}

func TestTimelineContextSummaryIncludesStructure(t *testing.T) {
	doc := planTestDoc()
	durationMS := int64(5000)
	assets := []models.VideoAsset{{ID: "asset-a", FileName: "intro.mp4", Kind: "video", DurationMS: &durationMS}}
	summary := timelineContextSummary(doc, assets, "clip-a", 2500)
	for _, expect := range []string{
		"Canvas: 1920x1080 @ 30 fps",
		"intro.mp4",
		"clip id=clip-a",
		"[SELECTED]",
		"Playhead: 2.5s",
		"Export renderer limitations",
	} {
		if !strings.Contains(summary, expect) {
			t.Errorf("summary missing %q\n%s", expect, summary)
		}
	}
}
