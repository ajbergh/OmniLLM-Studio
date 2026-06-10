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
	if strings.Contains(filterStr, "1.200") {
		t.Errorf("slide transition should not contribute fades: %s", filterStr)
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
	if byFeature[RendererFeatureKeyframes].Supported {
		t.Errorf("keyframes should be reported as unsupported")
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
