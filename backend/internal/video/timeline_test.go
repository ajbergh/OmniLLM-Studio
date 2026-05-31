package video

import (
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

func TestValidateTimelineDocumentNormalizesDefaults(t *testing.T) {
	doc, err := ValidateTimelineDocument(TimelineDocument{
		Version: 1,
		Canvas:  TimelineCanvas{},
		Tracks: []TimelineTrack{
			{Type: TrackTypeVideo, Clips: []TimelineClip{{StartMS: 1000, DurationMS: 2000}}},
		},
	})
	if err != nil {
		t.Fatalf("ValidateTimelineDocument returned error: %v", err)
	}
	if doc.Canvas.Width != DefaultProjectWidth || doc.Canvas.Height != DefaultProjectHeight || doc.Canvas.FPS != DefaultProjectFPS {
		t.Fatalf("canvas defaults not applied: %+v", doc.Canvas)
	}
	if doc.Tracks[0].ID == "" || doc.Tracks[0].Clips[0].ID == "" {
		t.Fatalf("expected generated track and clip IDs: %+v", doc.Tracks[0])
	}
	if doc.DurationMS < 3000 {
		t.Fatalf("duration did not include clip end: %d", doc.DurationMS)
	}
}

func TestAddSplitDeleteAssetClip(t *testing.T) {
	duration := int64(8000)
	projectTimeline := NewEmptyTimeline(1920, 1080, 30)
	projectID := "project-1"
	asset := models.VideoAsset{
		ID:         "asset-1",
		ProjectID:  &projectID,
		SourceType: "generation",
		Kind:       "video",
		FileName:   "clip.txt",
		FilePath:   "video/project-1/gen/clip.txt",
		MimeType:   "text/plain",
		DurationMS: &duration,
	}

	withClip, clip, err := AddAssetToTimeline(projectTimeline, asset, TimelineImportAssetRequest{StartMS: 1000})
	if err != nil {
		t.Fatalf("AddAssetToTimeline returned error: %v", err)
	}
	if clip.AssetID != asset.ID || clip.DurationMS != duration {
		t.Fatalf("clip did not reflect asset: %+v", clip)
	}

	split, err := SplitClipAt(withClip, clip.ID, 5000)
	if err != nil {
		t.Fatalf("SplitClipAt returned error: %v", err)
	}
	if got := len(split.Tracks[0].Clips); got != 2 {
		t.Fatalf("expected 2 clips after split, got %d", got)
	}

	deleted, err := DeleteClip(split, clip.ID)
	if err != nil {
		t.Fatalf("DeleteClip returned error: %v", err)
	}
	if got := len(deleted.Tracks[0].Clips); got != 1 {
		t.Fatalf("expected 1 clip after delete, got %d", got)
	}
}

func TestApplyEditPlanToTimelineValidatesOperations(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	plan := EditPlan{
		Summary: "vertical title",
		Operations: []EditOperation{
			{Type: "set_canvas", Width: 1080, Height: 1920, FPS: 30},
			{Type: "set_duration", DurationMS: 30000},
			{Type: "add_text_clip", TrackID: "track-text-1", StartMS: 0, DurationMS: 3000, Text: "Launch"},
		},
	}
	updated, err := ApplyEditPlanToTimeline(doc, plan)
	if err != nil {
		t.Fatalf("ApplyEditPlanToTimeline returned error: %v", err)
	}
	if updated.Canvas.Width != 1080 || updated.Canvas.Height != 1920 {
		t.Fatalf("canvas not updated: %+v", updated.Canvas)
	}
	if updated.DurationMS != 30000 {
		t.Fatalf("duration not updated: %d", updated.DurationMS)
	}
	textTrack := updated.Tracks[3]
	if len(textTrack.Clips) != 1 || textTrack.Clips[0].Text == nil || textTrack.Clips[0].Text.Text != "Launch" {
		t.Fatalf("text clip not added: %+v", textTrack.Clips)
	}

	if _, err := ApplyEditPlanToTimeline(doc, EditPlan{Operations: []EditOperation{{Type: "write_raw_json"}}}); err == nil {
		t.Fatalf("expected unsupported operation to fail")
	}
}
