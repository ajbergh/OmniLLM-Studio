package rendercontract

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type contractFixture struct {
	Version int `json:"version"`
	Easing  []struct {
		Name string  `json:"name"`
		T    float64 `json:"t"`
		Want float64 `json:"want"`
	} `json:"easing"`
	Curves []struct {
		Type      string  `json:"type"`
		T         float64 `json:"t"`
		X1        float64 `json:"x1"`
		Y1        float64 `json:"y1"`
		X2        float64 `json:"x2"`
		Y2        float64 `json:"y2"`
		Stiffness float64 `json:"stiffness"`
		Damping   float64 `json:"damping"`
		Mass      float64 `json:"mass"`
		Want      float64 `json:"want"`
	} `json:"curves"`
	FrameRanges []struct {
		StartMS    int64 `json:"start_ms"`
		DurationMS int64 `json:"duration_ms"`
		FPS        int   `json:"fps"`
		StartFrame int64 `json:"start_frame"`
		EndFrame   int64 `json:"end_frame"`
	} `json:"frame_ranges"`
	SourceTimes []struct {
		TimelineMS   int64   `json:"timeline_ms"`
		ClipStartMS  int64   `json:"clip_start_ms"`
		TrimInMS     int64   `json:"trim_in_ms"`
		PlaybackRate float64 `json:"playback_rate"`
		WantMS       float64 `json:"want_ms"`
	} `json:"source_times"`
}

func loadContractFixture(t *testing.T) contractFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve render contract test path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "video-renderer", "test", "fixtures", "render-contract-v1.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shared render contract fixture: %v", err)
	}
	var fixture contractFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode shared render contract fixture: %v", err)
	}
	if fixture.Version != 1 {
		t.Fatalf("fixture version = %d, want 1", fixture.Version)
	}
	return fixture
}

func TestEaseProgressMatchesSharedFixture(t *testing.T) {
	fixture := loadContractFixture(t)
	for _, sample := range fixture.Easing {
		got := EaseProgress(sample.T, sample.Name)
		if math.Abs(got-sample.Want) > 1e-12 {
			t.Errorf("EaseProgress(%v, %q) = %.12f, want %.12f", sample.T, sample.Name, got, sample.Want)
		}
	}
}

func TestCurveProgressMatchesSharedFixture(t *testing.T) {
	fixture := loadContractFixture(t)
	for _, sample := range fixture.Curves {
		curve := &MotionCurve{
			Type: sample.Type, X1: sample.X1, Y1: sample.Y1, X2: sample.X2, Y2: sample.Y2,
			Stiffness: sample.Stiffness, Damping: sample.Damping, Mass: sample.Mass,
		}
		got := CurveProgress(sample.T, curve, EasingLinear)
		if math.Abs(got-sample.Want) > 1e-12 {
			t.Errorf("CurveProgress(%v, %+v) = %.12f, want %.12f", sample.T, curve, got, sample.Want)
		}
	}
}

func TestFrameBoundariesMatchSharedFixture(t *testing.T) {
	fixture := loadContractFixture(t)
	for _, sample := range fixture.FrameRanges {
		if got := StartFrame(sample.StartMS, sample.FPS); got != sample.StartFrame {
			t.Errorf("StartFrame(%d, %d) = %d, want %d", sample.StartMS, sample.FPS, got, sample.StartFrame)
		}
		endMS := sample.StartMS + sample.DurationMS
		if got := EndFrame(endMS, sample.FPS); got != sample.EndFrame {
			t.Errorf("EndFrame(%d, %d) = %d, want %d", endMS, sample.FPS, got, sample.EndFrame)
		}
		if sample.StartFrame < sample.EndFrame {
			if !ActiveAtFrame(sample.StartFrame, sample.StartMS, sample.DurationMS, sample.FPS) {
				t.Errorf("first frame %d unexpectedly inactive for %+v", sample.StartFrame, sample)
			}
			if ActiveAtFrame(sample.EndFrame, sample.StartMS, sample.DurationMS, sample.FPS) {
				t.Errorf("exclusive end frame %d unexpectedly active for %+v", sample.EndFrame, sample)
			}
		}
	}
}

func TestSourceTimeMatchesSharedFixture(t *testing.T) {
	fixture := loadContractFixture(t)
	for _, sample := range fixture.SourceTimes {
		got := SourceTimeMS(sample.TimelineMS, sample.ClipStartMS, sample.TrimInMS, sample.PlaybackRate)
		if math.Abs(got-sample.WantMS) > 1e-12 {
			t.Errorf("SourceTimeMS(%d, %d, %d, %v) = %.12f, want %.12f", sample.TimelineMS, sample.ClipStartMS, sample.TrimInMS, sample.PlaybackRate, got, sample.WantMS)
		}
	}
}

func TestFrameTimeIsRational(t *testing.T) {
	got := FrameTime(1000001, 120)
	if got.Numerator != 1000001 || got.Denominator != 120 {
		t.Fatalf("FrameTime = %+v, want 1000001/120", got)
	}
}
