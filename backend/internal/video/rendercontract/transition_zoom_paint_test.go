package rendercontract

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type transitionZoomPaintFixture struct {
	Version int `json:"version"`
	Cases   []struct {
		Name        string                   `json:"name"`
		OwnerClipID string                   `json:"owner_clip_id"`
		State       EvaluatedTransitionState `json:"state"`
		Expected    EvaluatedTransitionPaint `json:"expected"`
	} `json:"cases"`
}

func TestEvaluateTransitionZoomPaintMatchesSharedFixture(t *testing.T) {
	fixture := loadTransitionZoomPaintFixture(t)
	for _, sample := range fixture.Cases {
		t.Run(sample.Name, func(t *testing.T) {
			paint, err := EvaluateTransitionPaint(sample.OwnerClipID, sample.State)
			if err != nil {
				t.Fatalf("EvaluateTransitionPaint: %v", err)
			}
			if paint == nil {
				t.Fatal("paint is nil")
			}
			assertZoomPaintNear(t, *paint, sample.Expected)
		})
	}
}

func assertZoomPaintNear(t *testing.T, got, want EvaluatedTransitionPaint) {
	t.Helper()
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	var gotMap, wantMap map[string]any
	if err := json.Unmarshal(gotJSON, &gotMap); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(wantJSON, &wantMap); err != nil {
		t.Fatal(err)
	}
	for key, wantValue := range wantMap {
		gotValue, ok := gotMap[key]
		if !ok {
			t.Fatalf("missing %q in %s", key, gotJSON)
		}
		wantNumber, wantIsNumber := wantValue.(float64)
		gotNumber, gotIsNumber := gotValue.(float64)
		if wantIsNumber && gotIsNumber {
			if math.Abs(gotNumber-wantNumber) > 1e-12 {
				t.Fatalf("%s = %.15f, want %.15f", key, gotNumber, wantNumber)
			}
			continue
		}
		if gotValue != wantValue {
			t.Fatalf("%s = %#v, want %#v", key, gotValue, wantValue)
		}
	}
}

func loadTransitionZoomPaintFixture(t *testing.T) transitionZoomPaintFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve transition zoom paint fixture path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "video-renderer", "test", "fixtures", "transition-zoom-paint-v1.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read transition zoom paint fixture: %v", err)
	}
	var fixture transitionZoomPaintFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode transition zoom paint fixture: %v", err)
	}
	if fixture.Version != 1 {
		t.Fatalf("fixture version = %d, want 1", fixture.Version)
	}
	return fixture
}
