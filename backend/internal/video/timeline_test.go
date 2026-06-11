package video

import (
	"strings"
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
			{Type: "add_text_clip", StartMS: 0, DurationMS: 3000, Text: "Launch"},
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
	// With no track_id the text clip lands on the topmost (foreground) layer.
	textTrack := updated.Tracks[len(updated.Tracks)-1]
	if len(textTrack.Clips) != 1 || textTrack.Clips[0].Text == nil || textTrack.Clips[0].Text.Text != "Launch" {
		t.Fatalf("text clip not added: %+v", textTrack.Clips)
	}

	if _, err := ApplyEditPlanToTimeline(doc, EditPlan{Operations: []EditOperation{{Type: "write_raw_json"}}}); err == nil {
		t.Fatalf("expected unsupported operation to fail")
	}
}

func TestNewEmptyTimelineCreatesGenericLayers(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	if len(doc.Tracks) != 4 {
		t.Fatalf("expected 4 layers, got %d", len(doc.Tracks))
	}
	for i, track := range doc.Tracks {
		if track.Type != TrackTypeLayer {
			t.Fatalf("track %d is %q, want %q", i, track.Type, TrackTypeLayer)
		}
		if track.Locked || track.Muted || !track.Visible {
			t.Fatalf("layer %d has wrong default flags: %+v", i, track)
		}
	}
	if _, err := ValidateTimelineDocument(doc); err != nil {
		t.Fatalf("layer timeline failed validation: %v", err)
	}
}

func TestAddAssetToTimelineAnyKindOnAnyTrack(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	audio := models.VideoAsset{ID: "asset-audio", Kind: "music", MimeType: "audio/mpeg"}
	image := models.VideoAsset{ID: "asset-image", Kind: "image", MimeType: "image/png"}

	// Explicit target layer accepts audio.
	withAudio, audioClip, err := AddAssetToTimeline(doc, audio, TimelineImportAssetRequest{TrackID: "track-layer-2"})
	if err != nil {
		t.Fatalf("audio on layer rejected: %v", err)
	}
	if len(withAudio.Tracks[1].Clips) != 1 {
		t.Fatalf("audio clip not on layer 2: %+v", withAudio.Tracks)
	}
	if audioClip.Volume == nil || audioClip.Transform != nil {
		t.Fatalf("audio clip defaults wrong: %+v", audioClip)
	}
	if audioClip.DurationMS != 30000 {
		t.Fatalf("audio default duration wrong: %d", audioClip.DurationMS)
	}

	// Legacy typed track accepts a mismatched kind when explicitly targeted.
	legacy := TimelineDocument{
		Version: 1,
		Tracks: []TimelineTrack{
			{ID: "t-video", Type: TrackTypeVideo, Name: "Video 1", Visible: true, Clips: []TimelineClip{}},
		},
	}
	withImage, imageClip, err := AddAssetToTimeline(legacy, image, TimelineImportAssetRequest{TrackID: "t-video"})
	if err != nil {
		t.Fatalf("image on explicit video track rejected: %v", err)
	}
	if len(withImage.Tracks[0].Clips) != 1 {
		t.Fatalf("image clip not placed: %+v", withImage.Tracks)
	}
	if imageClip.Transform == nil || imageClip.Volume != nil {
		t.Fatalf("image clip defaults wrong: %+v", imageClip)
	}
	if imageClip.DurationMS != 5000 {
		t.Fatalf("image default duration wrong: %d", imageClip.DurationMS)
	}
}

func TestAddAssetToTimelineLockedTrack(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	doc.Tracks[0].Locked = true
	asset := models.VideoAsset{ID: "asset-1", Kind: "video", MimeType: "video/mp4"}

	if _, _, err := AddAssetToTimeline(doc, asset, TimelineImportAssetRequest{TrackID: "track-layer-1"}); err == nil {
		t.Fatalf("expected locked track to reject explicit placement")
	}

	// Auto-placement skips the locked layer.
	updated, _, err := AddAssetToTimeline(doc, asset, TimelineImportAssetRequest{})
	if err != nil {
		t.Fatalf("auto placement failed: %v", err)
	}
	if len(updated.Tracks[0].Clips) != 0 || len(updated.Tracks[1].Clips) != 1 {
		t.Fatalf("clip not routed around locked layer: %+v", updated.Tracks)
	}
}

func TestAddAssetToTimelineLegacyAutoRouting(t *testing.T) {
	legacy := TimelineDocument{
		Version: 1,
		Tracks: []TimelineTrack{
			{ID: "t-video", Type: TrackTypeVideo, Name: "Video 1", Visible: true, Clips: []TimelineClip{}},
			{ID: "t-audio", Type: TrackTypeAudio, Name: "Audio 1", Visible: true, Clips: []TimelineClip{}},
		},
	}
	audio := models.VideoAsset{ID: "asset-audio", Kind: "audio", MimeType: "audio/mpeg"}
	updated, _, err := AddAssetToTimeline(legacy, audio, TimelineImportAssetRequest{})
	if err != nil {
		t.Fatalf("legacy auto routing failed: %v", err)
	}
	if len(updated.Tracks[1].Clips) != 1 {
		t.Fatalf("audio not routed to legacy audio track: %+v", updated.Tracks)
	}

	// A kind no legacy track accepts creates a new generic layer.
	export := models.VideoAsset{ID: "asset-export", Kind: "export", MimeType: "video/mp4"}
	updated, clip, err := AddAssetToTimeline(updated, export, TimelineImportAssetRequest{})
	if err != nil {
		t.Fatalf("export asset placement failed: %v", err)
	}
	last := updated.Tracks[len(updated.Tracks)-1]
	if last.Type != TrackTypeLayer || len(last.Clips) != 1 {
		t.Fatalf("export asset did not create a generic layer: %+v", last)
	}
	if clip.Transform == nil || clip.Volume == nil {
		t.Fatalf("export clip should get transform and volume: %+v", clip)
	}
}

func TestValidateTimelineDocumentShapes(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	doc.Tracks[0].Clips = []TimelineClip{{
		ID:         "clip-shape",
		StartMS:    0,
		DurationMS: 3000,
		Shape:      &TimelineShape{Kind: "RECTANGLE", StrokeWidth: 500},
	}}
	validated, err := ValidateTimelineDocument(doc)
	if err != nil {
		t.Fatalf("shape clip failed validation: %v", err)
	}
	shape := validated.Tracks[0].Clips[0].Shape
	if shape.Kind != ShapeKindRectangle {
		t.Errorf("shape kind not normalized: %q", shape.Kind)
	}
	if shape.Width != 320 || shape.Height != 180 {
		t.Errorf("shape default dimensions not applied: %dx%d", shape.Width, shape.Height)
	}
	if shape.StrokeWidth != 100 {
		t.Errorf("stroke width not clamped: %f", shape.StrokeWidth)
	}

	doc.Tracks[0].Clips[0].Shape = &TimelineShape{Kind: "starburst"}
	if _, err := ValidateTimelineDocument(doc); err == nil {
		t.Fatalf("expected unknown shape kind to be rejected")
	}
}

func TestKindForAssetOrMimeFallback(t *testing.T) {
	cases := []struct {
		kind string
		mime string
		want string
	}{
		{"video", "text/plain", "video"},
		{"", "video/mp4", "video"},
		{"", "image/png", "image"},
		{"", "audio/mpeg", "audio"},
		{"", "application/json", "other"},
	}
	for _, tc := range cases {
		got := kindForAssetOrMime(models.VideoAsset{Kind: tc.kind, MimeType: tc.mime})
		if got != tc.want {
			t.Fatalf("kindForAssetOrMime(%q, %q) = %q, want %q", tc.kind, tc.mime, got, tc.want)
		}
	}
}

func TestUpgradeTimelineDocumentRejectsFutureVersion(t *testing.T) {
	_, err := ValidateTimelineDocument(TimelineDocument{Version: CurrentTimelineVersion + 1})
	if err == nil {
		t.Fatalf("expected future version to be rejected")
	}
	if !strings.Contains(err.Error(), "upgrade OmniLLM Studio") {
		t.Fatalf("expected actionable error message, got: %v", err)
	}
}

func TestValidateTimelineDocumentRejectsUnknownEffectTransitionKeyframeTypes(t *testing.T) {
	base := func() TimelineDocument {
		doc := NewEmptyTimeline(1920, 1080, 30)
		doc.Tracks[0].Clips = []TimelineClip{{ID: "clip-1", StartMS: 0, DurationMS: 2000}}
		return doc
	}

	doc := base()
	doc.Tracks[0].Clips[0].Effects = []TimelineEffect{{Type: "vhs_glitch", Enabled: true}}
	if _, err := ValidateTimelineDocument(doc); err == nil {
		t.Fatalf("expected unknown effect type to be rejected")
	}

	doc = base()
	doc.Tracks[0].Clips[0].Transitions = []TimelineTransition{{Type: "teleport", DurationMS: 500}}
	if _, err := ValidateTimelineDocument(doc); err == nil {
		t.Fatalf("expected unknown transition type to be rejected")
	}

	doc = base()
	doc.Tracks[0].Clips[0].Transitions = []TimelineTransition{{Type: TransitionTypeFade, DurationMS: 0}}
	if _, err := ValidateTimelineDocument(doc); err == nil {
		t.Fatalf("expected zero-duration transition to be rejected")
	}

	doc = base()
	doc.Tracks[0].Clips[0].Keyframes = []TimelineKeyframe{{Property: "blend_mode", TimeMS: 0, Value: 1}}
	if _, err := ValidateTimelineDocument(doc); err == nil {
		t.Fatalf("expected unknown keyframe property to be rejected")
	}

	doc = base()
	doc.Tracks[0].Clips[0].Keyframes = []TimelineKeyframe{{Property: "opacity", TimeMS: -5, Value: 1}}
	if _, err := ValidateTimelineDocument(doc); err == nil {
		t.Fatalf("expected negative keyframe time to be rejected")
	}
}

func TestValidateTimelineDocumentNormalizesNewFields(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	height := 999
	doc.Tracks[0].Height = &height
	zIndex := 3
	doc.Tracks[0].Clips = []TimelineClip{{
		ID:         "clip-1",
		StartMS:    0,
		DurationMS: 2000,
		ZIndex:     &zIndex,
		GroupID:    "  group-a  ",
		Effects:    []TimelineEffect{{Type: "SHARPEN", Enabled: true}},
		Transitions: []TimelineTransition{
			{Type: "Fade", DurationMS: 400},
		},
		Keyframes: []TimelineKeyframe{{Property: "OPACITY", TimeMS: 100, Value: 0.5, Easing: "bounce"}},
		Text: &TimelineText{
			Text:       "Styled",
			FontFamily: "Inter",
			TextAlign:  "center",
		},
	}}
	doc.Markers = []TimelineMarker{
		{TimeMS: 5000, Label: " Later "},
		{TimeMS: -100, Label: "Start"},
	}

	validated, err := ValidateTimelineDocument(doc)
	if err != nil {
		t.Fatalf("ValidateTimelineDocument returned error: %v", err)
	}
	if got := *validated.Tracks[0].Height; got != maxTrackHeight {
		t.Fatalf("expected track height clamped to %d, got %d", maxTrackHeight, got)
	}
	clip := validated.Tracks[0].Clips[0]
	if clip.ZIndex == nil || *clip.ZIndex != 3 {
		t.Fatalf("z_index not preserved: %+v", clip.ZIndex)
	}
	if clip.GroupID != "group-a" {
		t.Fatalf("group_id not trimmed: %q", clip.GroupID)
	}
	if clip.Effects[0].Type != EffectTypeSharpen {
		t.Fatalf("effect type not normalized: %q", clip.Effects[0].Type)
	}
	if clip.Transitions[0].Type != TransitionTypeFade || clip.Transitions[0].ID == "" {
		t.Fatalf("transition not normalized: %+v", clip.Transitions[0])
	}
	if clip.Keyframes[0].Property != "opacity" || clip.Keyframes[0].Easing != "linear" {
		t.Fatalf("keyframe not normalized: %+v", clip.Keyframes[0])
	}
	if clip.Text.FontFamily != "Inter" || clip.Text.TextAlign != "center" {
		t.Fatalf("text style fields not preserved: %+v", clip.Text)
	}
	if validated.Markers[0].TimeMS != 0 || validated.Markers[1].TimeMS != 5000 {
		t.Fatalf("markers not clamped/sorted: %+v", validated.Markers)
	}
	if validated.Markers[1].Label != "Later" {
		t.Fatalf("marker label not trimmed: %q", validated.Markers[1].Label)
	}
	if validated.Markers[0].ID == "" || validated.Markers[1].ID == "" {
		t.Fatalf("marker IDs not generated: %+v", validated.Markers)
	}
}

func TestValidateTimelineDocumentLegacyDocumentStillValid(t *testing.T) {
	raw := `{"version":1,"canvas":{"width":1280,"height":720,"fps":24,"background":"#111111"},"duration_ms":10000,` +
		`"tracks":[{"id":"t1","type":"video","name":"Video 1","visible":true,` +
		`"clips":[{"id":"c1","asset_id":"a1","start_ms":0,"duration_ms":4000,"trim_in_ms":0,"trim_out_ms":4000,` +
		`"transform":{"x":0,"y":0,"scale":1,"rotation":0,"opacity":1},"effects":[],"keyframes":[]}]}],"markers":[],"metadata":{}}`
	doc, err := TimelineFromJSON(raw, NewEmptyTimeline(1920, 1080, 30))
	if err != nil {
		t.Fatalf("legacy v1 document failed validation: %v", err)
	}
	if doc.Tracks[0].Clips[0].ZIndex != nil || doc.Tracks[0].Clips[0].GroupID != "" {
		t.Fatalf("legacy document grew unexpected values: %+v", doc.Tracks[0].Clips[0])
	}
}
