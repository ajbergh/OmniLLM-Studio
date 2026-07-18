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
		{Type: "move_clip", ClipID: "clip-a", StartMS: 1000, TrackID: "track-layer-1"},
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
		{Type: "move_clip", ClipID: "clip-a", StartMS: 4000, TrackID: "track-layer-2"},
	}})
	if err != nil {
		t.Fatalf("move_clip failed: %v", err)
	}
	ti, ci, found := findTimelineClip(moved, "clip-a")
	if !found {
		t.Fatalf("clip lost after move")
	}
	if moved.Tracks[ti].ID != "track-layer-2" || moved.Tracks[ti].Clips[ci].StartMS != 4000 {
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

func TestEditPlanVolumeAndMarkerOperations(t *testing.T) {
	doc := planTestDoc()
	half := 0.5
	tooLoud := 3.0
	valid, preview, issues := ValidateEditPlanOperations(doc, EditPlan{Operations: []EditOperation{
		{Type: "set_volume", ClipID: "clip-a", Volume: &half},
		{Type: "set_volume", ClipID: "clip-a", Volume: &tooLoud},
		{Type: "set_volume", ClipID: "clip-missing", Volume: &half},
		{Type: "add_marker", StartMS: 2500, Text: "Beat"},
	}})
	if len(valid) != 2 || len(preview) != 2 || len(issues) != 2 {
		t.Fatalf("validation = %d valid / %d issues, want 2/2 (%v)", len(valid), len(issues), issues)
	}

	updated, err := ApplyEditPlanToTimeline(doc, EditPlan{Operations: valid})
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	ti, ci, ok := findTimelineClip(updated, "clip-a")
	if !ok || updated.Tracks[ti].Clips[ci].Volume == nil || *updated.Tracks[ti].Clips[ci].Volume != 0.5 {
		t.Fatalf("clip volume not applied: %+v", updated.Tracks[ti].Clips[ci])
	}
	if len(updated.Markers) != 1 || updated.Markers[0].TimeMS != 2500 || updated.Markers[0].Label != "Beat" {
		t.Fatalf("marker not applied: %+v", updated.Markers)
	}
}

func TestEditPlanAssetClipAndTransformOperations(t *testing.T) {
	doc := planTestDoc()
	scale := 1.5
	opacity := 0.7
	valid, _, issues := ValidateEditPlanOperations(doc, EditPlan{Operations: []EditOperation{
		{Type: "add_asset_clip", AssetID: "asset-a", StartMS: 5000, DurationMS: 3000},
		{Type: "add_asset_clip", DurationMS: 3000},                                      // missing asset_id
		{Type: "add_asset_clip", AssetID: "asset-a", DurationMS: 3000, TrackID: "nope"}, // unknown track
		{Type: "set_transform", ClipID: "clip-a", Scale: &scale, Opacity: &opacity},
		{Type: "set_transform", ClipID: "clip-a"}, // no fields
	}})
	if len(valid) != 2 || len(issues) != 3 {
		t.Fatalf("validation = %d valid / %d issues, want 2/3 (%v)", len(valid), len(issues), issues)
	}

	updated, err := ApplyEditPlanToTimeline(doc, EditPlan{Operations: valid})
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	// New asset clip lands on the first unlocked track at the requested time.
	found := false
	for _, clip := range updated.Tracks[0].Clips {
		if clip.AssetID == "asset-a" && clip.StartMS == 5000 && clip.DurationMS == 3000 {
			found = true
		}
	}
	if !found {
		t.Fatalf("asset clip not added: %+v", updated.Tracks[0].Clips)
	}
	ti, ci, ok := findTimelineClip(updated, "clip-a")
	if !ok {
		t.Fatal("clip-a lost")
	}
	transform := updated.Tracks[ti].Clips[ci].Transform
	if transform["scale"] != 1.5 || transform["opacity"] != 0.7 {
		t.Fatalf("transform not applied: %+v", transform)
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
