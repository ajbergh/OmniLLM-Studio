package video

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

func TestMotionDesignTimelineContract(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	doc.DurationMS = 6000
	doc.Scenes = []TimelineScene{{
		ID: "scene-a", Name: "Opening", StartMS: 0, DurationMS: 6000,
		Camera: &TimelineCamera{FieldOfView: 55, Keyframes: []TimelineKeyframe{
			{ID: "camera-a", Property: "z", TimeMS: 0, Value: 0, Easing: "linear"},
			{ID: "camera-b", Property: "z", TimeMS: 6000, Value: 120, Easing: "ease-in-out", Curve: &MotionCurve{Type: "bezier", X1: .42, Y1: 0, X2: .58, Y2: 1}},
		}},
		Effects: []TimelineEffect{{ID: "grain", Type: EffectTypeFilmGrain, Enabled: true, Params: map[string]any{"amount": 4.0}}},
	}}
	doc.Tracks[0].Solo = true
	doc.Tracks[0].Clips = []TimelineClip{{
		ID: "clip-a", StartMS: 0, DurationMS: 6000, TemplateSlot: "Hero Image",
		Transform: map[string]any{"x": 0.0, "y": 0.0, "z": 200.0, "scale": 1.0, "scale_x": 1.2, "scale_y": .8, "rotation_x": 8.0},
		Effects:   []TimelineEffect{{ID: "blur-a", Type: EffectTypeBlur, Enabled: true, Params: map[string]any{"amount": 0.0}}},
		Keyframes: []TimelineKeyframe{
			{ID: "x-a", Property: "x", TimeMS: 0, Value: 0, Easing: "linear"},
			{ID: "x-b", Property: "x", TimeMS: 1000, Value: 200, Easing: "ease-out", Curve: &MotionCurve{Type: "spring", Stiffness: 170, Damping: 18, Mass: 1}},
			{ID: "blur-start", Property: "effect.blur-a.amount", TimeMS: 0, Value: 12, Easing: "ease-out"},
			{ID: "blur-end", Property: "effect.blur-a.amount", TimeMS: 800, Value: 0, Easing: "ease-out"},
		},
		AnimationBlocks: []TimelineAnimationBlock{{ID: "block-a", BlockKey: "blur_reveal", Family: "in", StartMS: 0, DurationMS: 1000, GeneratedKeyframeIDs: []string{"blur-start", "blur-end"}}},
	}}

	validated, err := ValidateTimelineDocument(doc)
	if err != nil {
		t.Fatalf("motion-design document rejected: %v", err)
	}
	clip := validated.Tracks[0].Clips[0]
	if clip.TemplateSlot != "Hero Image" || len(clip.AnimationBlocks) != 1 || clip.Keyframes[1].Curve == nil {
		t.Fatalf("motion-design fields were not preserved: %+v", clip)
	}
}

func TestMotionDesignTimelineRejectsOverlappingScenesAndBrokenProvenance(t *testing.T) {
	doc := NewEmptyTimeline(1920, 1080, 30)
	doc.DurationMS = 5000
	doc.Scenes = []TimelineScene{{ID: "a", StartMS: 0, DurationMS: 3000}, {ID: "b", StartMS: 2000, DurationMS: 2000}}
	if _, err := ValidateTimelineDocument(doc); err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("expected overlap rejection, got %v", err)
	}
	doc.Scenes = nil
	doc.Tracks[0].Clips = []TimelineClip{{ID: "clip", DurationMS: 2000, AnimationBlocks: []TimelineAnimationBlock{{ID: "block", BlockKey: "fade_in", Family: "in", DurationMS: 500, GeneratedKeyframeIDs: []string{"missing"}}}}}
	if _, err := ValidateTimelineDocument(doc); err == nil || !strings.Contains(err.Error(), "missing keyframe") {
		t.Fatalf("expected provenance rejection, got %v", err)
	}
}

func TestMotionCurvesAreDeterministicAndSegmentLocal(t *testing.T) {
	bezier := &MotionCurve{Type: "bezier", X1: .25, Y1: .1, X2: .25, Y2: 1}
	spring := &MotionCurve{Type: "spring", Stiffness: 170, Damping: 18, Mass: 1}
	for _, curve := range []*MotionCurve{bezier, spring} {
		if got := curveProgress(0, curve, "linear"); math.Abs(got) > 1e-10 {
			t.Errorf("%s start = %v, want 0", curve.Type, got)
		}
		if got := curveProgress(1, curve, "linear"); math.Abs(got-1) > 1e-10 {
			t.Errorf("%s end = %v, want 1", curve.Type, got)
		}
		if a, b := curveProgress(.4, curve, "linear"), curveProgress(.4, curve, "linear"); a != b {
			t.Errorf("%s sampling is not deterministic: %v != %v", curve.Type, a, b)
		}
	}
	keyframes := []TimelineKeyframe{{Property: "x", TimeMS: 0, Value: 0}, {Property: "x", TimeMS: 1000, Value: 100, Easing: "linear", Curve: spring}, {Property: "x", TimeMS: 2000, Value: 200, Easing: "linear", Curve: spring}}
	if got, _ := evaluateTimelineKeyframes(keyframes, "x", 1000); math.Abs(got-100) > 1e-10 {
		t.Fatalf("middle keyframe boundary = %v, want 100", got)
	}
}

func TestCameraProjectionAndSceneEffectsExpandForExport(t *testing.T) {
	doc := NewEmptyTimeline(1000, 1000, 10)
	doc.DurationMS = 1000
	doc.Scenes = []TimelineScene{{ID: "scene", StartMS: 0, DurationMS: 1000, Camera: &TimelineCamera{FieldOfView: 50, Keyframes: []TimelineKeyframe{{ID: "a", Property: "x", TimeMS: 0, Value: 0}, {ID: "b", Property: "x", TimeMS: 1000, Value: 100}}}, Effects: []TimelineEffect{{ID: "grain", Type: EffectTypeFilmGrain, Enabled: true, Params: map[string]any{"amount": 3.0}}}}}
	doc.Tracks[0].Clips = []TimelineClip{{ID: "clip", StartMS: 0, DurationMS: 1000, Transform: map[string]any{"x": 0.0, "y": 0.0, "z": 200.0, "scale": 1.0}}}
	expanded := ExpandTimelineForFidelity(doc, 10, 20)
	if len(expanded.Tracks[0].Clips) < 2 {
		t.Fatalf("animated camera did not sample the clip: %d segments", len(expanded.Tracks[0].Clips))
	}
	first, last := expanded.Tracks[0].Clips[0], expanded.Tracks[0].Clips[len(expanded.Tracks[0].Clips)-1]
	firstX, _ := numericTransform(first.Transform, "x")
	lastX, _ := numericTransform(last.Transform, "x")
	if !(lastX < firstX) {
		t.Fatalf("camera pan did not project layer position: first=%v last=%v", firstX, lastX)
	}
	near := applySceneCamera(TimelineClip{Transform: map[string]any{"x": 0.0, "y": 0.0, "z": 0.0, "scale": 1.0}}, doc.Scenes, doc.Canvas, 750)
	far := applySceneCamera(TimelineClip{Transform: map[string]any{"x": 0.0, "y": 0.0, "z": 300.0, "scale": 1.0}}, doc.Scenes, doc.Canvas, 750)
	nearX, _ := numericTransform(near.Transform, "x")
	farX, _ := numericTransform(far.Transform, "x")
	if math.Abs(nearX-farX) < 1 {
		t.Fatalf("camera pan did not create depth parallax: near=%v far=%v", nearX, farX)
	}
	filters := sceneEffectFilters(doc.Scenes[0])
	if len(filters) == 0 || !strings.Contains(filters[0], "noise=") || !strings.Contains(filters[0], "between(t\\,0.000\\,1.000)") {
		t.Fatalf("scene effect is not time-bounded in export: %v", filters)
	}
}

func TestPersistedTrackSoloControlsExportAudio(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"solo.mp3", "other.mp3"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	doc := NewEmptyTimeline(1280, 720, 30)
	doc.Tracks = []TimelineTrack{
		{ID: "solo", Type: TrackTypeAudio, Visible: true, Solo: true, Clips: []TimelineClip{{ID: "solo-clip", AssetID: "solo-asset", DurationMS: 1000}}},
		{ID: "other", Type: TrackTypeAudio, Visible: true, Clips: []TimelineClip{{ID: "other-clip", AssetID: "other-asset", DurationMS: 1000}}},
	}
	clips := resolveMediaClips(RenderRequest{Timeline: doc, AttachmentsDir: dir, Assets: map[string]models.VideoAsset{
		"solo-asset":  {ID: "solo-asset", FilePath: "solo.mp3", MimeType: "audio/mpeg"},
		"other-asset": {ID: "other-asset", FilePath: "other.mp3", MimeType: "audio/mpeg"},
	}})
	if len(clips) != 1 || clips[0].clip.ID != "solo-clip" || !clips[0].hasAudio {
		t.Fatalf("persisted solo did not limit export mix: %+v", clips)
	}
}

func TestMotionDesignCapabilityMatrixIsHonest(t *testing.T) {
	byFeature := map[string]RendererFeatureSupport{}
	for _, feature := range FFmpegRendererCapabilities().Features {
		byFeature[feature.Feature] = feature
	}
	for _, name := range []string{RendererFeatureTrackSolo, RendererFeatureBezierCurves, RendererFeatureSpringCurves, RendererFeatureCameraMotion, RendererFeatureFilmGrain} {
		if !byFeature[name].Supported {
			t.Errorf("%s should report supported after its export path is tested", name)
		}
	}
	if feature := byFeature[RendererFeatureSpatial3D]; !feature.Supported || !feature.Partial || !strings.Contains(feature.Notes, "preview-only") {
		t.Errorf("spatial fidelity must disclose its remaining tilt limitation: %+v", feature)
	}
}
