package video

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

type canonicalAdapterFixture struct {
	Version      int `json:"version"`
	SuccessCases []struct {
		Name     string           `json:"name"`
		Input    TimelineDocument `json:"input"`
		Expected json.RawMessage  `json:"expected"`
	} `json:"success_cases"`
	ErrorCases []struct {
		Name  string           `json:"name"`
		Input TimelineDocument `json:"input"`
		Code  string           `json:"code"`
		Path  string           `json:"path"`
	} `json:"error_cases"`
}

func loadCanonicalAdapterFixture(t *testing.T) canonicalAdapterFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve canonical adapter fixture path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "video-renderer", "test", "fixtures", "v1-canonical-adapter-v1.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read canonical adapter fixture: %v", err)
	}
	var fixture canonicalAdapterFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode canonical adapter fixture: %v", err)
	}
	if fixture.Version != 1 {
		t.Fatalf("fixture version = %d, want 1", fixture.Version)
	}
	return fixture
}

func TestAdaptTimelineV1ToV2MatchesSharedFixture(t *testing.T) {
	fixture := loadCanonicalAdapterFixture(t)
	for _, sample := range fixture.SuccessCases {
		t.Run(sample.Name, func(t *testing.T) {
			before, err := json.Marshal(sample.Input)
			if err != nil {
				t.Fatalf("marshal input before adaptation: %v", err)
			}
			adapted, err := AdaptTimelineV1ToV2(sample.Input)
			if err != nil {
				t.Fatalf("AdaptTimelineV1ToV2() error = %v", err)
			}
			after, err := json.Marshal(sample.Input)
			if err != nil {
				t.Fatalf("marshal input after adaptation: %v", err)
			}
			if string(before) != string(after) {
				t.Fatalf("adapter mutated caller input\nbefore: %s\nafter:  %s", before, after)
			}

			gotJSON, err := json.Marshal(adapted)
			if err != nil {
				t.Fatalf("marshal adapted document: %v", err)
			}
			var got any
			var want any
			if err := json.Unmarshal(gotJSON, &got); err != nil {
				t.Fatalf("decode adapted JSON: %v", err)
			}
			if err := json.Unmarshal(sample.Expected, &want); err != nil {
				t.Fatalf("decode expected JSON: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				gotPretty, _ := json.MarshalIndent(got, "", "  ")
				wantPretty, _ := json.MarshalIndent(want, "", "  ")
				t.Fatalf("adapted document mismatch\ngot:\n%s\nwant:\n%s", gotPretty, wantPretty)
			}
		})
	}
}

func TestAdaptTimelineV1ToV2ErrorsMatchSharedFixture(t *testing.T) {
	fixture := loadCanonicalAdapterFixture(t)
	for _, sample := range fixture.ErrorCases {
		t.Run(sample.Name, func(t *testing.T) {
			_, err := AdaptTimelineV1ToV2(sample.Input)
			var adapterErr *CanonicalAdapterError
			if !errors.As(err, &adapterErr) {
				t.Fatalf("error = %v, want CanonicalAdapterError", err)
			}
			if adapterErr.Code != sample.Code || adapterErr.Path != sample.Path {
				t.Fatalf("adapter error = %+v, want code=%q path=%q", adapterErr, sample.Code, sample.Path)
			}
		})
	}
}
