package rendercontract

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type propertyEvaluationFixture struct {
	Version       int `json:"version"`
	KeyframeCases []struct {
		Name      string             `json:"name"`
		Property  string             `json:"property"`
		TimeMS    int64              `json:"time_ms"`
		Keyframes []PropertyKeyframe `json:"keyframes"`
		Expected  float64            `json:"expected"`
		Found     *bool              `json:"found,omitempty"`
	} `json:"keyframe_cases"`
	ClipCases []struct {
		Name     string         `json:"name"`
		Property string         `json:"property"`
		TimeMS   int64          `json:"time_ms"`
		Clip     TimelineV2Clip `json:"clip"`
		Expected float64        `json:"expected"`
	} `json:"clip_cases"`
	CameraCases []struct {
		Name     string           `json:"name"`
		Property string           `json:"property"`
		TimeMS   int64            `json:"time_ms"`
		Camera   TimelineV2Camera `json:"camera"`
		Expected float64          `json:"expected"`
	} `json:"camera_cases"`
	UnsupportedCases []struct {
		Scope    string `json:"scope"`
		Property string `json:"property"`
	} `json:"unsupported_cases"`
}

func loadPropertyEvaluationFixture(t *testing.T) propertyEvaluationFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve property evaluation fixture path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "video-renderer", "test", "fixtures", "property-evaluation-v1.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read property evaluation fixture: %v", err)
	}
	var fixture propertyEvaluationFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode property evaluation fixture: %v", err)
	}
	if fixture.Version != 1 {
		t.Fatalf("fixture version = %d, want 1", fixture.Version)
	}
	return fixture
}

func TestSamplePropertyKeyframesMatchesSharedFixture(t *testing.T) {
	fixture := loadPropertyEvaluationFixture(t)
	for _, sample := range fixture.KeyframeCases {
		t.Run(sample.Name, func(t *testing.T) {
			got, found := SamplePropertyKeyframes(sample.Keyframes, sample.Property, sample.TimeMS)
			wantFound := true
			if sample.Found != nil {
				wantFound = *sample.Found
			}
			if found != wantFound {
				t.Fatalf("found = %v, want %v", found, wantFound)
			}
			if found && math.Abs(got-sample.Expected) > 1e-9 {
				t.Fatalf("value = %.12f, want %.12f", got, sample.Expected)
			}
		})
	}
}

func TestEvaluateClipPropertyMatchesSharedFixture(t *testing.T) {
	fixture := loadPropertyEvaluationFixture(t)
	for _, sample := range fixture.ClipCases {
		t.Run(sample.Name, func(t *testing.T) {
			got, err := EvaluateClipProperty(sample.Clip, sample.Property, sample.TimeMS)
			if err != nil {
				t.Fatalf("EvaluateClipProperty: %v", err)
			}
			if math.Abs(got-sample.Expected) > 1e-9 {
				t.Fatalf("value = %.12f, want %.12f", got, sample.Expected)
			}
		})
	}
}

func TestEvaluateCameraPropertyMatchesSharedFixture(t *testing.T) {
	fixture := loadPropertyEvaluationFixture(t)
	for _, sample := range fixture.CameraCases {
		t.Run(sample.Name, func(t *testing.T) {
			got, err := EvaluateCameraProperty(&sample.Camera, sample.Property, sample.TimeMS)
			if err != nil {
				t.Fatalf("EvaluateCameraProperty: %v", err)
			}
			if math.Abs(got-sample.Expected) > 1e-9 {
				t.Fatalf("value = %.12f, want %.12f", got, sample.Expected)
			}
		})
	}
}

func TestSemanticPropertyEvaluatorsFailClosed(t *testing.T) {
	fixture := loadPropertyEvaluationFixture(t)
	for _, sample := range fixture.UnsupportedCases {
		t.Run(sample.Scope+"-"+sample.Property, func(t *testing.T) {
			var err error
			switch sample.Scope {
			case "clip":
				_, err = EvaluateClipProperty(TimelineV2Clip{}, sample.Property, 0)
			case "camera":
				_, err = EvaluateCameraProperty(&TimelineV2Camera{}, sample.Property, 0)
			default:
				t.Fatalf("unknown fixture scope %q", sample.Scope)
			}
			if err == nil {
				t.Fatalf("expected unsupported property error")
			}
		})
	}
}
