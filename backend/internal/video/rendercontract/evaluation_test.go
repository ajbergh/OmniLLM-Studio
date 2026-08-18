package rendercontract

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"testing"
)

type activeClipFixture struct {
	Version     int                `json:"version"`
	Timeline    TimelineV2Document `json:"timeline"`
	ActiveCases []struct {
		FrameIndex int64        `json:"frame_index"`
		Expected   []ActiveClip `json:"expected"`
	} `json:"active_cases"`
	RangeCases []struct {
		StartMS  int64      `json:"start_ms"`
		EndMS    int64      `json:"end_ms"`
		FPS      int        `json:"fps"`
		Expected FrameRange `json:"expected"`
	} `json:"range_cases"`
	SourceCases []struct {
		FrameIndex   int64   `json:"frame_index"`
		FPS          int     `json:"fps"`
		ClipStartMS  int64   `json:"clip_start_ms"`
		TrimInMS     int64   `json:"trim_in_ms"`
		PlaybackRate float64 `json:"playback_rate"`
		ExpectedMS   float64 `json:"expected_ms"`
	} `json:"source_cases"`
}

func loadActiveClipFixture(t *testing.T) activeClipFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve active clip fixture path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "video-renderer", "test", "fixtures", "active-clips-v1.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read active clip fixture: %v", err)
	}
	var fixture activeClipFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode active clip fixture: %v", err)
	}
	if fixture.Version != 1 {
		t.Fatalf("fixture version = %d, want 1", fixture.Version)
	}
	return fixture
}

func TestActiveClipsAtFrameMatchesSharedFixture(t *testing.T) {
	fixture := loadActiveClipFixture(t)
	for _, sample := range fixture.ActiveCases {
		t.Run("frame-"+strconv.FormatInt(sample.FrameIndex, 10), func(t *testing.T) {
			got := ActiveClipsAtFrame(fixture.Timeline, sample.FrameIndex)
			if !reflect.DeepEqual(got, sample.Expected) {
				gotJSON, _ := json.MarshalIndent(got, "", "  ")
				wantJSON, _ := json.MarshalIndent(sample.Expected, "", "  ")
				t.Fatalf("active clips mismatch\ngot:\n%s\nwant:\n%s", gotJSON, wantJSON)
			}
		})
	}
}

func TestFrameRangeFromMSMatchesSharedFixture(t *testing.T) {
	fixture := loadActiveClipFixture(t)
	for _, sample := range fixture.RangeCases {
		got := FrameRangeFromMS(sample.StartMS, sample.EndMS, sample.FPS)
		if got != sample.Expected {
			t.Errorf("FrameRangeFromMS(%d, %d, %d) = %+v, want %+v", sample.StartMS, sample.EndMS, sample.FPS, got, sample.Expected)
		}
		if got.EndFrame > got.StartFrame {
			if !got.Contains(got.StartFrame) {
				t.Errorf("range %+v must contain its first frame", got)
			}
			if got.Contains(got.EndFrame) {
				t.Errorf("range %+v must exclude its end frame", got)
			}
		}
	}
}

func TestSourceTimeAtFrameMSMatchesSharedFixture(t *testing.T) {
	fixture := loadActiveClipFixture(t)
	for _, sample := range fixture.SourceCases {
		got := SourceTimeAtFrameMS(sample.FrameIndex, sample.FPS, sample.ClipStartMS, sample.TrimInMS, sample.PlaybackRate)
		if math.Abs(got-sample.ExpectedMS) > 1e-9 {
			t.Errorf("SourceTimeAtFrameMS(%d, %d, %d, %d, %v) = %.12f, want %.12f", sample.FrameIndex, sample.FPS, sample.ClipStartMS, sample.TrimInMS, sample.PlaybackRate, got, sample.ExpectedMS)
		}
	}
}
