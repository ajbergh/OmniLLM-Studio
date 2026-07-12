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
	if clip := doc.Tracks[0].Clips[0]; clip.PlaybackRate != 1 || clip.TrimOutMS != 2000 {
		t.Fatalf("legacy clip timing defaults not normalized: %+v", clip)
	}
	if doc.DurationMS < 3000 {
		t.Fatalf("duration did not include clip end: %d", doc.DurationMS)
	}
}

func TestValidateTimelineDocumentPlaybackRate(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	doc.Tracks[0].Clips = []TimelineClip{{
		ID:           "retimed",
		DurationMS:   2000,
		TrimInMS:     500,
		TrimOutMS:    123, // canonicalized from duration and playback rate
		PlaybackRate: 2,
	}}
	validated, err := ValidateTimelineDocument(doc)
	if err != nil {
		t.Fatalf("valid playback rate rejected: %v", err)
	}
	clip := validated.Tracks[0].Clips[0]
	if clip.PlaybackRate != 2 || clip.TrimOutMS != 4500 {
		t.Fatalf("retimed source window not normalized: %+v", clip)
	}

	for _, rate := range []float64{0.1, 4.1} {
		invalid := NewEmptyTimeline(1920, 1080, 30)
		invalid.Tracks[0].Clips = []TimelineClip{{ID: "bad-rate", DurationMS: 1000, PlaybackRate: rate}}
		if _, err := ValidateTimelineDocument(invalid); err == nil {
			t.Fatalf("expected playback rate %v to be rejected", rate)
		}
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

func TestValidateTimelineDocumentAnnotationKinds(t *testing.T) {
	kinds := []string{
		ShapeKindRoundedRectangle, ShapeKindEllipse, ShapeKindArrow, ShapeKindLine,
		ShapeKindSpeechBubble, ShapeKindSpotlight, ShapeKindPixelate,
		ShapeKindCheckmark, ShapeKindXMark, ShapeKindStepMarker, ShapeKindLabel,
	}
	for _, kind := range kinds {
		doc := NewEmptyTimeline(1920, 1080, 30)
		doc.Tracks[0].Clips = []TimelineClip{{
			ID:         "clip-" + kind,
			StartMS:    0,
			DurationMS: 2000,
			Shape:      &TimelineShape{Kind: kind, CornerRadius: 999},
		}}
		validated, err := ValidateTimelineDocument(doc)
		if err != nil {
			t.Fatalf("annotation kind %q failed validation: %v", kind, err)
		}
		shape := validated.Tracks[0].Clips[0].Shape
		if shape.CornerRadius != 200 {
			t.Errorf("kind %q: corner radius not clamped: %f", kind, shape.CornerRadius)
		}
		if kind == ShapeKindPixelate && shape.BlurRadius != 12 {
			t.Errorf("pixelate did not default blur radius: %f", shape.BlurRadius)
		}
	}
}

func TestValidateTimelineDocumentCursorMetadata(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	doc.Tracks[0].Clips = []TimelineClip{{
		ID:         "clip-cursor",
		StartMS:    0,
		DurationMS: 5000,
		Cursor: &TimelineCursor{
			Scale: 99,
			Events: []TimelineCursorEvent{
				{TimeMS: 2000, X: 100, Y: 100},
				{TimeMS: -5, X: 0, Y: 0},
				{TimeMS: 1000, X: 50, Y: 50, Click: true},
			},
		},
	}}
	validated, err := ValidateTimelineDocument(doc)
	if err != nil {
		t.Fatalf("cursor metadata failed validation: %v", err)
	}
	cursor := validated.Tracks[0].Clips[0].Cursor
	if cursor.Scale != 4 {
		t.Errorf("cursor scale not clamped: %f", cursor.Scale)
	}
	if len(cursor.Events) != 2 {
		t.Fatalf("negative-time cursor event not dropped: %+v", cursor.Events)
	}
	if cursor.Events[0].TimeMS != 1000 || !cursor.Events[0].Click {
		t.Errorf("cursor events not sorted by time: %+v", cursor.Events)
	}

	// Older documents without cursor metadata stay valid untouched.
	plain := NewEmptyTimeline(1920, 1080, 30)
	if _, err := ValidateTimelineDocument(plain); err != nil {
		t.Fatalf("document without cursor metadata failed validation: %v", err)
	}
}

func TestSliceTimelineRange(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	doc.DurationMS = 10000
	doc.Tracks[0].Clips = []TimelineClip{
		{ID: "before", StartMS: 0, DurationMS: 1000, TrimOutMS: 1000},
		{ID: "straddle-start", StartMS: 1500, DurationMS: 2000, TrimOutMS: 2000, FadeInMS: 300,
			Keyframes: []TimelineKeyframe{
				{ID: "k1", Property: "x", TimeMS: 100, Value: 5},
				{ID: "k2", Property: "x", TimeMS: 1500, Value: 50},
			}},
		{ID: "inside", StartMS: 4000, DurationMS: 1000, TrimOutMS: 1000},
		{ID: "straddle-end", StartMS: 5500, DurationMS: 2000, TrimOutMS: 2000, FadeOutMS: 300},
		{ID: "after", StartMS: 8000, DurationMS: 1000, TrimOutMS: 1000},
	}
	doc.Markers = []TimelineMarker{
		{ID: "m1", TimeMS: 500}, {ID: "m2", TimeMS: 4500}, {ID: "m3", TimeMS: 9000},
	}
	out := SliceTimelineRange(doc, 2000, 6000)
	if out.DurationMS != 4000 {
		t.Fatalf("sliced duration: %d", out.DurationMS)
	}
	clips := out.Tracks[0].Clips
	if len(clips) != 3 {
		t.Fatalf("expected 3 clips, got %d: %+v", len(clips), clips)
	}
	left := clips[0]
	if left.ID != "straddle-start" || left.StartMS != 0 || left.DurationMS != 1500 || left.TrimInMS != 500 {
		t.Errorf("straddle-start wrong: %+v", left)
	}
	if left.FadeInMS != 0 {
		t.Errorf("fade-in should drop when the head is cut: %+v", left)
	}
	if len(left.Keyframes) != 1 || left.Keyframes[0].TimeMS != 1000 {
		t.Errorf("keyframes not rebased: %+v", left.Keyframes)
	}
	if clips[1].ID != "inside" || clips[1].StartMS != 2000 {
		t.Errorf("inside clip wrong: %+v", clips[1])
	}
	tail := clips[2]
	if tail.ID != "straddle-end" || tail.StartMS != 3500 || tail.DurationMS != 500 || tail.FadeOutMS != 0 {
		t.Errorf("straddle-end wrong: %+v", tail)
	}
	if len(out.Markers) != 1 || out.Markers[0].TimeMS != 2500 {
		t.Errorf("markers not sliced/shifted: %+v", out.Markers)
	}
	// Invalid range returns the document unchanged.
	same := SliceTimelineRange(doc, 5000, 5000)
	if same.DurationMS != doc.DurationMS || len(same.Tracks[0].Clips) != 5 {
		t.Errorf("invalid range should be a no-op")
	}
}

func TestSliceTimelineRangePreservesRetimedSourceWindow(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	doc.DurationMS = 6000
	doc.Tracks[0].Clips = []TimelineClip{{
		ID:           "fast",
		StartMS:      1000,
		DurationMS:   4000,
		TrimInMS:     500,
		TrimOutMS:    8500,
		PlaybackRate: 2,
		Keyframes: []TimelineKeyframe{
			{ID: "before", Property: "x", TimeMS: 500},
			{ID: "inside", Property: "x", TimeMS: 1500},
			{ID: "after", Property: "x", TimeMS: 3500},
		},
		Cursor: &TimelineCursor{Events: []TimelineCursorEvent{
			{TimeMS: 500, X: 1}, {TimeMS: 1500, X: 2}, {TimeMS: 3500, X: 3},
		}},
	}}

	out := SliceTimelineRange(doc, 2000, 4000)
	if len(out.Tracks[0].Clips) != 1 {
		t.Fatalf("expected one sliced clip: %+v", out.Tracks[0].Clips)
	}
	clip := out.Tracks[0].Clips[0]
	if clip.StartMS != 0 || clip.DurationMS != 2000 || clip.TrimInMS != 2500 || clip.TrimOutMS != 6500 || clip.PlaybackRate != 2 {
		t.Fatalf("retimed slice timing wrong: %+v", clip)
	}
	if len(clip.Keyframes) != 1 || clip.Keyframes[0].ID != "inside" || clip.Keyframes[0].TimeMS != 500 {
		t.Fatalf("retimed keyframes not sliced/rebased: %+v", clip.Keyframes)
	}
	if clip.Cursor == nil || len(clip.Cursor.Events) != 1 || clip.Cursor.Events[0].TimeMS != 500 || clip.Cursor.Events[0].X != 2 {
		t.Fatalf("retimed cursor events not sliced/rebased: %+v", clip.Cursor)
	}
}

func TestSplitClipAtPreservesRetimedSourceWindow(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	doc.Tracks[0].Clips = []TimelineClip{{
		ID:           "fast",
		DurationMS:   4000,
		TrimInMS:     500,
		TrimOutMS:    8500,
		PlaybackRate: 2,
		FadeInMS:     200,
		FadeOutMS:    300,
		Keyframes: []TimelineKeyframe{
			{ID: "left", Property: "x", TimeMS: 500},
			{ID: "right", Property: "x", TimeMS: 2000},
		},
		Cursor: &TimelineCursor{Events: []TimelineCursorEvent{
			{TimeMS: 500, X: 1}, {TimeMS: 2000, X: 2},
		}},
	}}

	out, err := SplitClipAt(doc, "fast", 1500)
	if err != nil {
		t.Fatalf("SplitClipAt returned error: %v", err)
	}
	if len(out.Tracks[0].Clips) != 2 {
		t.Fatalf("expected two clips: %+v", out.Tracks[0].Clips)
	}
	left, right := out.Tracks[0].Clips[0], out.Tracks[0].Clips[1]
	if left.DurationMS != 1500 || left.TrimInMS != 500 || left.TrimOutMS != 3500 || left.FadeOutMS != 0 {
		t.Fatalf("left retimed split wrong: %+v", left)
	}
	if right.DurationMS != 2500 || right.TrimInMS != 3500 || right.TrimOutMS != 8500 || right.PlaybackRate != 2 || right.FadeInMS != 0 {
		t.Fatalf("right retimed split wrong: %+v", right)
	}
	if len(right.Keyframes) != 1 || right.Keyframes[0].TimeMS != 500 || right.Keyframes[0].ID == "right" {
		t.Fatalf("right keyframes not rebased/reidentified: %+v", right.Keyframes)
	}
	if right.Cursor == nil || len(right.Cursor.Events) != 1 || right.Cursor.Events[0].TimeMS != 500 {
		t.Fatalf("right cursor events not rebased: %+v", right.Cursor)
	}
}

func TestStripCaptionOverlays(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	doc.Tracks = append(doc.Tracks, TimelineTrack{
		ID: "captions", Type: TrackTypeCaption, Name: "Captions", Visible: true,
		Clips: []TimelineClip{{ID: "cap1", StartMS: 0, DurationMS: 2000, Text: &TimelineText{Text: "hello"}}},
	})
	doc.Tracks[0].Clips = []TimelineClip{{ID: "title", StartMS: 0, DurationMS: 2000, Text: &TimelineText{Text: "Title"}}}
	out := StripCaptionOverlays(doc)
	if len(out.Tracks[len(out.Tracks)-1].Clips) != 0 {
		t.Errorf("caption clips not stripped")
	}
	if len(out.Tracks[0].Clips) != 1 {
		t.Errorf("non-caption text clips must survive")
	}
	if len(doc.Tracks[len(doc.Tracks)-1].Clips) != 1 {
		t.Errorf("original document mutated")
	}
}

func TestSerializeCaptionsSidecar(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	doc.Tracks = append(doc.Tracks, TimelineTrack{
		ID: "captions", Type: TrackTypeCaption, Name: "Captions", Visible: true,
		Clips: []TimelineClip{
			{ID: "c2", StartMS: 3000, DurationMS: 1000, Text: &TimelineText{Text: "second"}},
			{ID: "c1", StartMS: 500, DurationMS: 1500, Text: &TimelineText{Text: "first"}},
			{ID: "empty", StartMS: 6000, DurationMS: 1000, Text: &TimelineText{Text: "  "}},
		},
	})
	cues := CaptionCuesFromTimeline(doc)
	if len(cues) != 2 || cues[0].Text != "first" {
		t.Fatalf("cue extraction wrong: %+v", cues)
	}
	srt := SerializeCaptions(cues, "srt")
	if !strings.Contains(srt, "00:00:00,500 --> 00:00:02,000") || !strings.HasPrefix(srt, "1\n") {
		t.Errorf("srt output wrong:\n%s", srt)
	}
	vtt := SerializeCaptions(cues, "vtt")
	if !strings.HasPrefix(vtt, "WEBVTT") || !strings.Contains(vtt, "00:00:03.000 --> 00:00:04.000") {
		t.Errorf("vtt output wrong:\n%s", vtt)
	}
	if SerializeCaptions(nil, "srt") != "" || SerializeCaptions(cues, "ass") != "" {
		t.Errorf("empty/unknown formats must serialize to empty string")
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
