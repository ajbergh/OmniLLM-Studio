package rendercontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

type transitionWipePaintFixture struct {
	Version int `json:"version"`
	Cases   []struct {
		Name        string                   `json:"name"`
		OwnerClipID string                   `json:"owner_clip_id"`
		State       EvaluatedTransitionState `json:"state"`
		Expected    EvaluatedTransitionPaint `json:"expected"`
	} `json:"cases"`
}

func TestEvaluateTransitionWipePaintMatchesSharedFixture(t *testing.T) {
	fixture := loadTransitionWipePaintFixture(t)
	for _, sample := range fixture.Cases {
		t.Run(sample.Name, func(t *testing.T) {
			paint, err := EvaluateTransitionPaint(sample.OwnerClipID, sample.State)
			if err != nil {
				t.Fatalf("EvaluateTransitionPaint: %v", err)
			}
			if paint == nil || !reflect.DeepEqual(*paint, sample.Expected) {
				got, _ := json.Marshal(paint)
				want, _ := json.Marshal(sample.Expected)
				t.Fatalf("paint = %s, want %s", got, want)
			}
		})
	}
}

func loadTransitionWipePaintFixture(t *testing.T) transitionWipePaintFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve transition wipe paint fixture path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "video-renderer", "test", "fixtures", "transition-wipe-paint-v1.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read transition wipe paint fixture: %v", err)
	}
	var fixture transitionWipePaintFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode transition wipe paint fixture: %v", err)
	}
	if fixture.Version != 1 {
		t.Fatalf("fixture version = %d, want 1", fixture.Version)
	}
	return fixture
}
