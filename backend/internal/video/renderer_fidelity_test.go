package video

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

func TestBuildFilterComplexHonorsTransformOpacityAndFades(t *testing.T) {
	volume := 0.5
	doc := NewEmptyTimeline(1920, 1080, 30)
	clips := []resolvedClip{
		{
			inputIdx: 1,
			isVideo:  true,
			clip: TimelineClip{
				ID:         "clip-video",
				AssetID:    "asset-video",
				StartMS:    1000,
				DurationMS: 4000,
				TrimInMS:   500,
				FadeInMS:   500,
				FadeOutMS:  1000,
				Transform: map[string]any{
					"x":       float64(120),
					"y":       float64(-40),
					"scale":   0.5,
					"opacity": 0.7,
				},
				Effects: []TimelineEffect{
					{ID: "fx-1", Type: "brightness", Enabled: true, Params: map[string]any{"amount": 1.2}},
					{ID: "fx-2", Type: "grayscale", Enabled: true, Params: map[string]any{}},
					{ID: "fx-3", Type: "blur", Enabled: false, Params: map[string]any{"amount": 4}},
				},
			},
		},
		{
			inputIdx: 2,
			isAudio:  true,
			hasAudio: true,
			clip: TimelineClip{
				ID:         "clip-audio",
				AssetID:    "asset-audio",
				StartMS:    2000,
				DurationMS: 3000,
				Volume:     &volume,
				FadeInMS:   250,
				FadeOutMS:  250,
			},
		},
	}

	filterStr, videoLabel, audioLabel := buildFilterComplex(doc, clips, 1920, 1080)

	for _, expect := range []string{
		"x='(W-w)/2+120'",
		"y='(H-h)/2-40'",
		"scale=960:540:force_original_aspect_ratio=decrease",
		"colorchannelmixer=aa=0.700",
		"fade=t=in:st=0:d=0.500:alpha=1",
		"fade=t=out:st=3.000:d=1.000:alpha=1",
		"eq=brightness=0.200",
		"hue=s=0",
		"volume=0.500",
		"afade=t=in:st=0:d=0.250",
		"afade=t=out:st=2.750:d=0.250",
		"adelay=2000|2000",
	} {
		if !strings.Contains(filterStr, expect) {
			t.Errorf("filter_complex missing %q\nfull: %s", expect, filterStr)
		}
	}
	if strings.Contains(filterStr, "boxblur") {
		t.Errorf("disabled effect should not be rendered: %s", filterStr)
	}
	if videoLabel == "" || audioLabel == "" {
		t.Fatalf("expected video and audio labels, got %q / %q", videoLabel, audioLabel)
	}
}

func TestBuildFilterComplexTransitionFallsBackToAlphaFade(t *testing.T) {
	doc := NewEmptyTimeline(1280, 720, 30)
	clips := []resolvedClip{{
		inputIdx: 1,
		isImage:  true,
		clip: TimelineClip{
			ID:         "clip-image",
			AssetID:    "asset-image",
			StartMS:    0,
			DurationMS: 4000,
			Transitions: []TimelineTransition{
				{ID: "tr-1", Type: "crossfade", DurationMS: 800},
				{ID: "tr-2", Type: "slide", DurationMS: 1200},
			},
		},
	}}
	filterStr, _, _ := buildFilterComplex(doc, clips, 1280, 720)
	if !strings.Contains(filterStr, "fade=t=in:st=0:d=0.800:alpha=1") {
		t.Errorf("expected crossfade rendered as alpha fade-in: %s", filterStr)
	}
	if !strings.Contains(filterStr, "fade=t=out:st=3.200:d=0.800:alpha=1") {
		t.Errorf("expected crossfade rendered as alpha fade-out: %s", filterStr)
	}
	// Slide renders as an animated overlay position, not as a fade.
	if strings.Contains(filterStr, "fade=t=in:st=0:d=1.200") {
		t.Errorf("slide transition must not contribute fades: %s", filterStr)
	}
	if !strings.Contains(filterStr, "-W*(1-min(max((t-0.000)/1.200\\,0)\\,1))") {
		t.Errorf("expected slide-in overlay position expression: %s", filterStr)
	}
}

func TestSlideTransitionExprDirections(t *testing.T) {
	base := TimelineClip{ID: "c", DurationMS: 4000, Transitions: []TimelineTransition{{ID: "tr", Type: "slide", DurationMS: 1000, Direction: "down"}}}
	xOff, yOff := slideTransitionExpr(base, 2, 6)
	if xOff != "" {
		t.Errorf("vertical slide should not move x: %q", xOff)
	}
	if !strings.Contains(yOff, "H*(1-min(max((t-2.000)/1.000\\,0)\\,1))") || !strings.Contains(yOff, "-H*min(max((t-5.000)/1.000\\,0)\\,1)") {
		t.Errorf("down slide expression wrong: %q", yOff)
	}

	base.Transitions[0].Direction = "right"
	xOff, yOff = slideTransitionExpr(base, 0, 4)
	if yOff != "" || !strings.HasPrefix(xOff, "W*(1-") {
		t.Errorf("right slide expression wrong: x=%q y=%q", xOff, yOff)
	}

	none, _ := slideTransitionExpr(TimelineClip{DurationMS: 1000}, 0, 1)
	if none != "" {
		t.Errorf("clip without slide should yield no offsets: %q", none)
	}
}

func TestBuildFilterComplexVolumeKeyframesAndVideoAudio(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	volume := 0.8
	clips := []resolvedClip{{
		inputIdx: 1,
		isVideo:  true,
		hasAudio: true, // video asset with an audio stream joins the mixdown
		clip: TimelineClip{
			ID:         "clip-video",
			AssetID:    "asset-video",
			StartMS:    1000,
			DurationMS: 4000,
			Volume:     &volume,
			Keyframes: []TimelineKeyframe{
				{ID: "kf-1", Property: "volume", TimeMS: 0, Value: 0},
				{ID: "kf-2", Property: "volume", TimeMS: 2000, Value: 1},
			},
		},
	}}
	filterStr, videoLabel, audioLabel := buildFilterComplex(doc, clips, 1920, 1080)
	if videoLabel == "" || audioLabel == "" {
		t.Fatalf("expected video and audio labels, got %q / %q", videoLabel, audioLabel)
	}
	// Keyframed volume exports as a frame-evaluated expression and overrides
	// the static volume.
	if !strings.Contains(filterStr, "volume=volume='") || !strings.Contains(filterStr, ":eval=frame") {
		t.Errorf("expected volume keyframe expression: %s", filterStr)
	}
	if strings.Contains(filterStr, "volume=0.800") {
		t.Errorf("static volume must not apply when volume keyframes exist: %s", filterStr)
	}
	if !strings.Contains(filterStr, "adelay=1000|1000") {
		t.Errorf("video audio should be delayed to the clip start: %s", filterStr)
	}
}

func TestEffectFiltersChromaKey(t *testing.T) {
	filters := effectFilters([]TimelineEffect{{
		ID: "fx", Type: "chroma_key", Enabled: true,
		Params: map[string]any{"color": "#00ff00", "similarity": 0.4, "blend": 0.1},
	}})
	if len(filters) != 1 || !strings.HasPrefix(filters[0], "chromakey=") {
		t.Fatalf("expected chromakey filter, got %v", filters)
	}
	if !strings.Contains(filters[0], "0.400") || !strings.Contains(filters[0], "0.100") {
		t.Errorf("chromakey params not applied: %s", filters[0])
	}
	defaults := effectFilters([]TimelineEffect{{ID: "fx", Type: "chroma_key", Enabled: true, Params: map[string]any{}}})
	if len(defaults) != 1 || !strings.Contains(defaults[0], "0x00FF00") {
		t.Errorf("expected green default key color, got %v", defaults)
	}
}

func TestBuildFilterComplexRotationKeyframes(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	clips := []resolvedClip{{
		inputIdx: 1,
		isVideo:  true,
		clip: TimelineClip{
			ID: "clip-video", AssetID: "asset-video", StartMS: 0, DurationMS: 4000,
			Transform: map[string]any{"rotation": float64(45)}, // static loses to keyframes
			Keyframes: []TimelineKeyframe{
				{ID: "kf-1", Property: "rotation", TimeMS: 0, Value: 0},
				{ID: "kf-2", Property: "rotation", TimeMS: 2000, Value: 90},
			},
		},
	}}
	filterStr, _, _ := buildFilterComplex(doc, clips, 1920, 1080)
	if !strings.Contains(filterStr, "rotate=a='(") || !strings.Contains(filterStr, ")*PI/180':c=black@0:ow='hypot(iw\\,ih)':oh=ow") {
		t.Errorf("expected keyframed rotate expression: %s", filterStr)
	}
	if strings.Contains(filterStr, "rotate=0.785398") {
		t.Errorf("static rotation must not apply when rotation keyframes exist: %s", filterStr)
	}
}

func TestBuildFilterComplexBlurRegion(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	doc.Tracks = []TimelineTrack{{
		ID: "l1", Type: TrackTypeLayer, Visible: true,
		Clips: []TimelineClip{{
			ID: "c-blur", StartMS: 1000, DurationMS: 2000,
			Transform: map[string]any{"x": float64(200), "y": float64(0)},
			Shape:     &TimelineShape{Kind: ShapeKindBlur, Width: 400, Height: 300, BlurRadius: 20},
		}},
	}}
	filterStr, videoLabel, _ := buildFilterComplex(doc, nil, 1920, 1080)
	for _, expect := range []string{
		"split[bbs0][bbb0]",
		"[bbs0]crop=400:300:960:390,boxblur=20[bbl0]",
		"[bbb0][bbl0]overlay=960:390:enable='between(t\\,1.000\\,3.000)'[t0_v]",
	} {
		if !strings.Contains(filterStr, expect) {
			t.Errorf("blur region graph missing %q: %s", expect, filterStr)
		}
	}
	if videoLabel != "[t0_v]" {
		t.Errorf("blur region should end the chain, got %s", videoLabel)
	}
}

func TestBuildFilterComplexRendersShapes(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	doc.Tracks = []TimelineTrack{{
		ID: "l1", Type: TrackTypeLayer, Visible: true,
		Clips: []TimelineClip{
			{
				ID: "c-highlight", StartMS: 1000, DurationMS: 2000,
				Transform: map[string]any{"x": float64(100), "y": float64(-50), "opacity": 0.4},
				Shape:     &TimelineShape{Kind: ShapeKindHighlight, Width: 400, Height: 200, Fill: "#facc15"},
			},
			{
				ID: "c-rect", StartMS: 0, DurationMS: 1000,
				Shape: &TimelineShape{Kind: ShapeKindRectangle, Width: 300, Height: 100, Stroke: "#ff0000", StrokeWidth: 6},
				Text:  &TimelineText{Text: "Look here"},
			},
		},
	}}
	filterStr, _, _ := buildFilterComplex(doc, nil, 1920, 1080)
	// Highlight: filled box at center offset (100,-50), 40% opacity.
	if !strings.Contains(filterStr, "drawbox=x=860:y=390:w=400:h=200:color=0xfacc15@0.400:t=fill:enable='between(t\\,1.000\\,3.000)'") {
		t.Errorf("highlight drawbox missing or wrong: %s", filterStr)
	}
	// Rectangle: outlined box with its label drawn after (on top of) the box.
	rectAt := strings.Index(filterStr, "drawbox=x=810:y=490:w=300:h=100:color=0xff0000@1.000:t=6:enable='between(t\\,0.000\\,1.000)'")
	textAt := strings.Index(filterStr, "drawtext=text='Look here'")
	if rectAt == -1 || textAt == -1 {
		t.Fatalf("rectangle drawbox or callout text missing: %s", filterStr)
	}
	if rectAt > textAt {
		t.Errorf("callout text must draw above its box: %s", filterStr)
	}
}

func TestResolveMediaClipsClipMuteAndAudioOnly(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.mp3", "b.mp4"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("stub"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	doc := NewEmptyTimeline(1280, 720, 30)
	doc.Tracks = []TimelineTrack{
		{ID: "l1", Type: TrackTypeLayer, Visible: true, Clips: []TimelineClip{
			{ID: "c-muted-audio", AssetID: "a1", DurationMS: 1000, Muted: true},
			// audio_only video keeps its (metadata-confirmed) audio but
			// contributes no visuals.
			{ID: "c-detached", AssetID: "a2", DurationMS: 1000, AudioOnly: true},
		}},
	}
	req := RenderRequest{
		Timeline:       doc,
		AttachmentsDir: dir,
		Assets: map[string]models.VideoAsset{
			"a1": {ID: "a1", FilePath: "a.mp3", MimeType: "audio/mpeg"},
			"a2": {ID: "a2", FilePath: "b.mp4", MimeType: "video/mp4", MetadataJSON: `{"has_audio":true}`},
		},
	}
	clips := resolveMediaClips(req)
	if len(clips) != 1 {
		t.Fatalf("expected only the detached audio clip, got %+v", clips)
	}
	rc := clips[0]
	if rc.clip.ID != "c-detached" || rc.isVideo || !rc.hasAudio {
		t.Errorf("audio_only video clip should be audio-only: %+v", rc)
	}
}

func TestClipFadeSecondsCapsAtHalfDuration(t *testing.T) {
	clip := TimelineClip{DurationMS: 2000, FadeInMS: 5000, FadeOutMS: 100}
	fadeIn, fadeOut := clipFadeSeconds(clip)
	if fadeIn != 1.0 {
		t.Errorf("fade-in should cap at half duration, got %v", fadeIn)
	}
	if fadeOut != 0.1 {
		t.Errorf("fade-out should pass through, got %v", fadeOut)
	}
}

func TestParseClipTransformCrop(t *testing.T) {
	tr := parseClipTransform(map[string]any{
		"crop": map[string]any{"top": 0.1, "left": 0.2},
	})
	if !tr.hasCrop || tr.cropTop != 0.1 || tr.cropLeft != 0.2 {
		t.Errorf("unexpected crop parse: %+v", tr)
	}
	none := parseClipTransform(map[string]any{"crop": map[string]any{}})
	if none.hasCrop {
		t.Errorf("empty crop should not enable cropping")
	}
}

func TestRenderDimensionsCustomOverride(t *testing.T) {
	req := RenderRequest{
		Project:  models.VideoProject{Width: 1920, Height: 1080},
		Timeline: NewEmptyTimeline(1920, 1080, 30),
		Settings: ExportSettings{Resolution: "custom", Width: 1080, Height: 1919},
	}
	width, height := renderDimensions(req)
	if width != 1080 || height != 1920 {
		t.Errorf("expected even custom dimensions 1080x1920, got %dx%d", width, height)
	}
}

func TestValidateExportSettingsCustomResolution(t *testing.T) {
	project := models.VideoProject{FPS: 30}
	if _, err := validateExportSettings(ExportSettings{Format: "mp4", Resolution: "custom"}, project); err == nil {
		t.Fatalf("custom resolution without dimensions should fail")
	}
	settings, err := validateExportSettings(ExportSettings{Format: "mp4", Resolution: "custom", Width: 1080, Height: 1920, Preset: "shorts_9_16"}, project)
	if err != nil {
		t.Fatalf("valid custom resolution rejected: %v", err)
	}
	if settings.Width != 1080 || settings.Height != 1920 {
		t.Errorf("dimensions not preserved: %+v", settings)
	}
	if _, err := validateExportSettings(ExportSettings{Format: "mp4", Resolution: "custom", Width: 8, Height: 100000}, project); err == nil {
		t.Fatalf("out-of-range dimensions should fail")
	}
}

func TestBuildFilterComplexRotationNewEffectsAndPositionKeyframes(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	clips := []resolvedClip{{
		inputIdx: 1,
		isVideo:  true,
		clip: TimelineClip{
			ID:         "clip-video",
			AssetID:    "asset-video",
			StartMS:    2000,
			DurationMS: 4000,
			Transform: map[string]any{
				"rotation": float64(90),
			},
			Effects: []TimelineEffect{
				{ID: "fx-sharpen", Type: "sharpen", Enabled: true, Params: map[string]any{"amount": 1.5}},
				{ID: "fx-vignette", Type: "vignette", Enabled: true, Params: map[string]any{"amount": 0.5}},
			},
			// Keyframe times are clip-relative; the clip starts at 2s, so the
			// segment boundaries land at 2s and 4s on the output timeline.
			Keyframes: []TimelineKeyframe{
				{ID: "kf-1", Property: "x", TimeMS: 0, Value: 0},
				{ID: "kf-2", Property: "x", TimeMS: 2000, Value: 300},
			},
		},
	}}

	filterStr, _, _ := buildFilterComplex(doc, clips, 1920, 1080)

	for _, expect := range []string{
		"rotate=1.570796:c=black@0:ow=rotw(1.570796):oh=roth(1.570796)",
		"unsharp=5:5:1.50",
		"vignette=a=0.7854",
		"x='(W-w)/2+if(lt(t\\,2.000)\\,0.000\\,if(lt(t\\,4.000)\\,(0.000+(300.000)*(t-2.000)/2.000)\\,300.000))'",
	} {
		if !strings.Contains(filterStr, expect) {
			t.Errorf("filter_complex missing %q\nfull: %s", expect, filterStr)
		}
	}
	// y has no keyframes — the static offset form must remain.
	if !strings.Contains(filterStr, "y='(H-h)/2+0'") {
		t.Errorf("expected static y placement, got: %s", filterStr)
	}
}

func TestDrawTextHonorsMuteScaleAndFades(t *testing.T) {
	// Muted tracks keep their text at export — mute only silences audio.
	doc := NewEmptyTimeline(1920, 1080, 30)
	doc.Tracks = []TimelineTrack{{
		ID: "t-text", Type: TrackTypeText, Visible: true, Muted: true,
		Clips: []TimelineClip{{
			ID: "c-text", StartMS: 0, DurationMS: 3000,
			Text: &TimelineText{Text: "Hello"},
		}},
	}}
	filterStr, _, _ := buildFilterComplex(doc, nil, 1920, 1080)
	if !strings.Contains(filterStr, "drawtext=text='Hello'") {
		t.Errorf("muted text track should still render text: %s", filterStr)
	}

	// transform.scale multiplies the font size (matching the preview).
	scaled := drawTextFilter(TimelineClip{
		StartMS: 0, DurationMS: 2000,
		Transform: map[string]any{"scale": 2.0},
	}, TimelineText{Text: "Big", FontSize: 50}, 1920, 1080)
	if !strings.Contains(scaled, "fontsize=100") {
		t.Errorf("expected fontsize=100 for scale 2.0, got: %s", scaled)
	}

	// Fades and opacity export as a drawtext alpha expression.
	faded := drawTextFilter(TimelineClip{
		StartMS: 1000, DurationMS: 4000, FadeInMS: 500, FadeOutMS: 1000,
		Transform: map[string]any{"opacity": 0.8},
	}, TimelineText{Text: "Fade"}, 1920, 1080)
	if !strings.Contains(faded, "alpha='0.800*clip(min((t-1.000)/0.500\\,(5.000-t)/1.000)\\,0\\,1)'") {
		t.Errorf("expected fade+opacity alpha expression, got: %s", faded)
	}
	plain := drawTextFilter(TimelineClip{StartMS: 0, DurationMS: 2000}, TimelineText{Text: "Plain"}, 1920, 1080)
	if strings.Contains(plain, "alpha=") {
		t.Errorf("fully opaque unfaded text should have no alpha option: %s", plain)
	}
}

func TestFFmpegRendererCapabilitiesMatrix(t *testing.T) {
	caps := FFmpegRendererCapabilities()
	byFeature := map[string]RendererFeatureSupport{}
	for _, f := range caps.Features {
		byFeature[f.Feature] = f
	}
	if !byFeature[RendererFeatureOpacity].Supported {
		t.Errorf("opacity should be reported as supported")
	}
	if !byFeature[RendererFeaturePositioning].Supported {
		t.Errorf("positioning should be reported as supported")
	}
	if !byFeature[RendererFeatureKeyframes].Supported || !byFeature[RendererFeatureKeyframes].Partial {
		t.Errorf("keyframes should be reported as partially supported (position only)")
	}
	if !byFeature[RendererFeatureRotation].Supported {
		t.Errorf("rotation should be reported as supported")
	}
	if !byFeature[RendererFeatureTransitions].Partial {
		t.Errorf("transitions should be reported as partial")
	}
	labels := caps.UnsupportedFeatureLabels()
	if len(labels) == 0 {
		t.Errorf("expected unsupported/partial feature labels for warning copy")
	}
}

func TestResolveMediaClipsHiddenAndMutedTracks(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.mp4", "b.mp3", "c.mp3"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("stub"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	doc := NewEmptyTimeline(1280, 720, 30)
	doc.Tracks = []TimelineTrack{
		{ID: "t-video", Type: TrackTypeVideo, Visible: false, Clips: []TimelineClip{{ID: "c1", AssetID: "a1", DurationMS: 1000}}},
		{ID: "t-audio", Type: TrackTypeAudio, Visible: true, Muted: true, Clips: []TimelineClip{{ID: "c2", AssetID: "a2", DurationMS: 1000}}},
		{ID: "t-music", Type: TrackTypeMusic, Visible: true, Clips: []TimelineClip{{ID: "c3", AssetID: "a3", DurationMS: 1000}}},
	}
	req := RenderRequest{
		Timeline:       doc,
		AttachmentsDir: dir,
		Assets: map[string]models.VideoAsset{
			"a1": {ID: "a1", FilePath: "a.mp4", MimeType: "video/mp4"},
			"a2": {ID: "a2", FilePath: "b.mp3", MimeType: "audio/mpeg"},
			"a3": {ID: "a3", FilePath: "c.mp3", MimeType: "audio/mpeg"},
		},
	}
	clips := resolveMediaClips(req)
	if len(clips) != 1 || clips[0].clip.ID != "c3" {
		t.Errorf("expected only the visible unmuted music clip to resolve, got %+v", clips)
	}
}

func TestResolveMediaClipsOrdersByTrackThenZIndexThenStart(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.mp4", "b.mp4", "c.mp4"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("stub"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	zTop := 1
	doc := NewEmptyTimeline(1280, 720, 30)
	doc.Tracks = []TimelineTrack{
		// Bottom layer clip starts last — start time must not promote it.
		{ID: "l1", Type: TrackTypeLayer, Visible: true, Clips: []TimelineClip{{ID: "c-bottom", AssetID: "a1", StartMS: 5000, DurationMS: 1000}}},
		{ID: "l2", Type: TrackTypeLayer, Visible: true, Clips: []TimelineClip{
			{ID: "c-z1", AssetID: "a2", StartMS: 0, DurationMS: 1000, ZIndex: &zTop},
			{ID: "c-z0", AssetID: "a3", StartMS: 0, DurationMS: 1000},
		}},
	}
	req := RenderRequest{
		Timeline:       doc,
		AttachmentsDir: dir,
		Assets: map[string]models.VideoAsset{
			"a1": {ID: "a1", FilePath: "a.mp4", MimeType: "video/mp4"},
			"a2": {ID: "a2", FilePath: "b.mp4", MimeType: "video/mp4"},
			"a3": {ID: "a3", FilePath: "c.mp4", MimeType: "video/mp4"},
		},
	}
	clips := resolveMediaClips(req)
	if len(clips) != 3 {
		t.Fatalf("expected 3 clips, got %d", len(clips))
	}
	got := []string{clips[0].clip.ID, clips[1].clip.ID, clips[2].clip.ID}
	want := []string{"c-bottom", "c-z0", "c-z1"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stacking order = %v, want %v", got, want)
		}
	}
}

func TestBuildFilterComplexStacksByLayerOrderNotStartTime(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	// The top-layer clip starts earlier than the bottom-layer clip; layer
	// order must still control compositing order.
	clips := []resolvedClip{
		{inputIdx: 1, trackIndex: 1, isImage: true, clip: TimelineClip{ID: "c-top", AssetID: "a-top", StartMS: 0, DurationMS: 2000}},
		{inputIdx: 2, trackIndex: 0, isVideo: true, clip: TimelineClip{ID: "c-bottom", AssetID: "a-bottom", StartMS: 1000, DurationMS: 2000}},
	}
	filterStr, videoLabel, _ := buildFilterComplex(doc, clips, 1920, 1080)
	bottomAt := strings.Index(filterStr, "[base_v][c2_v]overlay")
	topAt := strings.Index(filterStr, "[c1_v]overlay")
	if bottomAt == -1 || topAt == -1 {
		t.Fatalf("expected both overlays in chain: %s", filterStr)
	}
	if bottomAt > topAt {
		t.Errorf("bottom layer must composite before top layer: %s", filterStr)
	}
	if videoLabel != "[ov1_v]" {
		t.Errorf("expected top layer to finish the chain, got %s", videoLabel)
	}
}

func TestBuildFilterComplexInterleavesTextByLayerOrder(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	doc.Tracks = []TimelineTrack{
		{ID: "l1", Type: TrackTypeLayer, Visible: true, Clips: []TimelineClip{{ID: "c-text-bottom", StartMS: 0, DurationMS: 2000, Text: &TimelineText{Text: "Bottom"}}}},
		{ID: "l2", Type: TrackTypeLayer, Visible: true, Clips: []TimelineClip{{ID: "c-media", AssetID: "a1", StartMS: 0, DurationMS: 2000}}},
		{ID: "l3", Type: TrackTypeLayer, Visible: true, Clips: []TimelineClip{{ID: "c-text-top", StartMS: 0, DurationMS: 2000, Text: &TimelineText{Text: "Top"}}}},
	}
	clips := []resolvedClip{{inputIdx: 1, trackIndex: 1, isVideo: true, clip: doc.Tracks[1].Clips[0]}}
	filterStr, _, _ := buildFilterComplex(doc, clips, 1920, 1080)
	bottomAt := strings.Index(filterStr, "drawtext=text='Bottom'")
	mediaAt := strings.Index(filterStr, "[c1_v]overlay")
	topAt := strings.Index(filterStr, "drawtext=text='Top'")
	if bottomAt == -1 || mediaAt == -1 || topAt == -1 {
		t.Fatalf("expected text below, media, text above in chain: %s", filterStr)
	}
	if !(bottomAt < mediaAt && mediaAt < topAt) {
		t.Errorf("text must interleave with media by layer order (got %d, %d, %d): %s", bottomAt, mediaAt, topAt, filterStr)
	}
}
