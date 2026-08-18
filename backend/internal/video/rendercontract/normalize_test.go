package rendercontract

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

type timelineV2NormalizationFixture struct {
	Version      int `json:"version"`
	SuccessCases []struct {
		Name     string             `json:"name"`
		Input    TimelineV2Document `json:"input"`
		Expected json.RawMessage    `json:"expected"`
	} `json:"success_cases"`
	ErrorCases []struct {
		Name  string             `json:"name"`
		Input TimelineV2Document `json:"input"`
		Path  string             `json:"path"`
	} `json:"error_cases"`
}

func loadTimelineV2NormalizationFixture(t *testing.T) timelineV2NormalizationFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve Timeline v2 normalization fixture path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "video-renderer", "test", "fixtures", "timeline-v2-normalization-v1.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Timeline v2 normalization fixture: %v", err)
	}
	var fixture timelineV2NormalizationFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode Timeline v2 normalization fixture: %v", err)
	}
	if fixture.Version != 1 {
		t.Fatalf("fixture version = %d, want 1", fixture.Version)
	}
	return fixture
}

func TestNormalizeTimelineV2EvaluationInputsMatchesSharedFixture(t *testing.T) {
	fixture := loadTimelineV2NormalizationFixture(t)
	for _, sample := range fixture.SuccessCases {
		t.Run(sample.Name, func(t *testing.T) {
			before, err := json.Marshal(sample.Input)
			if err != nil {
				t.Fatalf("marshal input before normalization: %v", err)
			}
			normalized, err := NormalizeTimelineV2EvaluationInputs(sample.Input)
			if err != nil {
				t.Fatalf("NormalizeTimelineV2EvaluationInputs() error = %v", err)
			}
			after, err := json.Marshal(sample.Input)
			if err != nil {
				t.Fatalf("marshal input after normalization: %v", err)
			}
			if string(before) != string(after) {
				t.Fatalf("normalizer mutated caller input\nbefore: %s\nafter:  %s", before, after)
			}

			gotJSON, err := json.Marshal(normalized)
			if err != nil {
				t.Fatalf("marshal normalized timeline: %v", err)
			}
			var got any
			var want any
			if err := json.Unmarshal(gotJSON, &got); err != nil {
				t.Fatalf("decode normalized JSON: %v", err)
			}
			if err := json.Unmarshal(sample.Expected, &want); err != nil {
				t.Fatalf("decode expected JSON: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				gotPretty, _ := json.MarshalIndent(got, "", "  ")
				wantPretty, _ := json.MarshalIndent(want, "", "  ")
				t.Fatalf("normalized timeline mismatch\ngot:\n%s\nwant:\n%s", gotPretty, wantPretty)
			}
		})
	}
}

func TestNormalizeTimelineV2EvaluationInputsErrorsMatchSharedFixture(t *testing.T) {
	fixture := loadTimelineV2NormalizationFixture(t)
	for _, sample := range fixture.ErrorCases {
		t.Run(sample.Name, func(t *testing.T) {
			_, err := NormalizeTimelineV2EvaluationInputs(sample.Input)
			var runtimeErr *TimelineV2RuntimeError
			if !errors.As(err, &runtimeErr) {
				t.Fatalf("error = %v, want TimelineV2RuntimeError", err)
			}
			if runtimeErr.Code != TimelineV2RuntimeInvalidCode || runtimeErr.Path != sample.Path {
				t.Fatalf("runtime error = %+v, want code=%q path=%q", runtimeErr, TimelineV2RuntimeInvalidCode, sample.Path)
			}
		})
	}
}
