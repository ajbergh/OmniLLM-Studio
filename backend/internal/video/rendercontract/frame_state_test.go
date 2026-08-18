package rendercontract

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

type visualFrameStateFixture struct {
	Version  int                `json:"version"`
	Document TimelineV2Document `json:"document"`
	Cases    []struct {
		Name                        string  `json:"name"`
		FrameIndex                  int64   `json:"frame_index"`
		ActiveSceneID               string  `json:"active_scene_id"`
		ExpectedSourceTimeMS        float64 `json:"expected_source_time_ms"`
		ExpectedTransformX          float64 `json:"expected_transform_x"`
		ExpectedViewX               float64 `json:"expected_view_x"`
		ExpectedOpacity             float64 `json:"expected_opacity"`
		ExpectedCameraX             float64 `json:"expected_camera_x"`
		ExpectedCameraFOV           float64 `json:"expected_camera_fov"`
		ExpectedPerspectiveDistance float64 `json:"expected_perspective_distance"`
		ExpectedModelMatrix         Matrix4 `json:"expected_model_matrix"`
	} `json:"cases"`
	UnresolvedDocument TimelineV2Document `json:"unresolved_document"`
	ExpectedUnresolved []string           `json:"expected_unresolved"`
}

func loadVisualFrameStateFixture(t *testing.T) visualFrameStateFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve visual frame state fixture path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "video-renderer", "test", "fixtures", "visual-frame-state-v1.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read visual frame state fixture: %v", err)
	}
	var fixture visualFrameStateFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode visual frame state fixture: %v", err)
	}
	if fixture.Version != 1 {
		t.Fatalf("fixture version = %d, want 1", fixture.Version)
	}
	return fixture
}

func TestEvaluateVisualFrameStateMatchesSharedFixture(t *testing.T) {
	fixture := loadVisualFrameStateFixture(t)
	for _, sample := range fixture.Cases {
		t.Run(sample.Name, func(t *testing.T) {
			state, err := EvaluateVisualFrameState(fixture.Document, sample.FrameIndex)
			if err != nil {
				t.Fatalf("EvaluateVisualFrameState: %v", err)
			}
			if state.ContractVersion != VisualFrameStateContractV1 {
				t.Fatalf("contract version = %q", state.ContractVersion)
			}
			if state.ActiveSceneID != sample.ActiveSceneID {
				t.Fatalf("active scene = %q, want %q", state.ActiveSceneID, sample.ActiveSceneID)
			}
			if !state.Authoritative || len(state.Unresolved) != 0 {
				t.Fatalf("authoritative state = %v unresolved=%v", state.Authoritative, state.Unresolved)
			}
			if len(state.Layers) != 1 || state.Layers[0].ClipID != "media" {
				t.Fatalf("layers = %+v", state.Layers)
			}
			layer := state.Layers[0]
			assertClose(t, "source_time_ms", layer.SourceTimeMS, sample.ExpectedSourceTimeMS)
			assertClose(t, "transform.x", layer.Transform.X, sample.ExpectedTransformX)
			assertClose(t, "view_transform.x", layer.ViewTransform.X, sample.ExpectedViewX)
			assertClose(t, "transform.opacity", layer.Transform.Opacity, sample.ExpectedOpacity)
			assertClose(t, "camera.x", state.Camera.X, sample.ExpectedCameraX)
			assertClose(t, "camera.field_of_view", state.Camera.FieldOfView, sample.ExpectedCameraFOV)
			assertClose(t, "camera.perspective_distance", state.Camera.PerspectiveDistance, sample.ExpectedPerspectiveDistance)
			for index := range layer.ModelMatrix {
				assertClose(t, "model_matrix", layer.ModelMatrix[index], sample.ExpectedModelMatrix[index])
			}
			if layer.ContentBounds == nil || layer.ContentBounds.Width != 200 || layer.ContentBounds.Height != 100 {
				t.Fatalf("content bounds = %+v", layer.ContentBounds)
			}
			if layer.Transform.Crop == nil || layer.Transform.Crop.Top != 0.1 || layer.Transform.Crop.Right != 0.2 {
				t.Fatalf("crop = %+v", layer.Transform.Crop)
			}
		})
	}
}

func TestVisualFrameStateSurfacesUnresolvedSemantics(t *testing.T) {
	fixture := loadVisualFrameStateFixture(t)
	state, err := EvaluateVisualFrameState(fixture.UnresolvedDocument, 0)
	if err != nil {
		t.Fatalf("EvaluateVisualFrameState: %v", err)
	}
	if state.Authoritative {
		t.Fatalf("state unexpectedly authoritative: %+v", state)
	}
	if !reflect.DeepEqual(state.Unresolved, fixture.ExpectedUnresolved) {
		t.Fatalf("unresolved = %v, want %v", state.Unresolved, fixture.ExpectedUnresolved)
	}
	if len(state.Layers) != 1 || state.Layers[0].Authoritative {
		t.Fatalf("layer authority = %+v", state.Layers)
	}
}

func TestFrameRelativeMillisecondsPreservesSubMillisecondPresentation(t *testing.T) {
	time := FrameRelativeMilliseconds(1, 120, 5)
	if time.Numerator != 400 || time.Denominator != 120 {
		t.Fatalf("relative time = %+v, want 400/120 ms", time)
	}
	keyframes := []PropertyKeyframe{{Property: "x", TimeMS: 0, Value: 10}, {Property: "x", TimeMS: 10, Value: 20}}
	value, ok := SamplePropertyKeyframesAtRationalMS(keyframes, "x", time)
	if !ok {
		t.Fatal("expected x keyframes")
	}
	assertClose(t, "rational property value", value, 13.333333333333334)
}

func assertClose(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("%s = %.12f, want %.12f", label, got, want)
	}
}
