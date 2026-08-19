package rendercontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

type transitionPaintFixture struct {
	Version int `json:"version"`
	Cases   []struct {
		Name        string                    `json:"name"`
		OwnerClipID string                    `json:"owner_clip_id"`
		State       EvaluatedTransitionState  `json:"state"`
		Expected    *EvaluatedTransitionPaint `json:"expected"`
	} `json:"cases"`
}

func TestEvaluateTransitionPaintMatchesSharedFixture(t *testing.T) {
	fixture := loadTransitionPaintFixture(t)
	for _, sample := range fixture.Cases {
		t.Run(sample.Name, func(t *testing.T) {
			paint, err := EvaluateTransitionPaint(sample.OwnerClipID, sample.State)
			if err != nil {
				t.Fatalf("EvaluateTransitionPaint: %v", err)
			}
			if !reflect.DeepEqual(paint, sample.Expected) {
				got, _ := json.Marshal(paint)
				want, _ := json.Marshal(sample.Expected)
				t.Fatalf("paint = %s, want %s", got, want)
			}
		})
	}
}

func TestEvaluateTransitionPaintFailsClosedOnInvalidOrUnsupportedSemantics(t *testing.T) {
	base := EvaluatedTransitionState{
		ContractVersion: TransitionStateContractV1,
		ID:              "transition",
		Type:            "crossfade",
		Placement:       "between",
		PeerClipID:      "peer",
		Role:            "outgoing",
		PeerRole:        "incoming",
		Progress:        0.5,
		Active:          true,
	}

	t.Run("crossfade-needs-between", func(t *testing.T) {
		state := base
		state.Placement = "in"
		state.PeerClipID = ""
		state.Role = "incoming"
		state.PeerRole = ""
		_, err := EvaluateTransitionPaint("owner", state)
		if err == nil || !strings.Contains(err.Error(), "crossfade requires between") {
			t.Fatalf("crossfade placement error = %v", err)
		}
	})

	t.Run("pair-needs-complementary-roles", func(t *testing.T) {
		state := base
		state.PeerRole = "outgoing"
		_, err := EvaluateTransitionPaint("owner", state)
		if err == nil || !strings.Contains(err.Error(), "complementary") {
			t.Fatalf("role error = %v", err)
		}
	})

	t.Run("unsupported-paint-family", func(t *testing.T) {
		state := base
		state.Type = "wipe"
		state.Direction = "left"
		state.Placement = "out"
		state.PeerClipID = ""
		state.Role = "outgoing"
		state.PeerRole = ""
		_, err := EvaluateTransitionPaint("owner", state)
		if err == nil || !strings.Contains(err.Error(), "does not yet have canonical paint semantics") {
			t.Fatalf("unsupported type error = %v", err)
		}
	})

	t.Run("slide-direction-must-be-canonical", func(t *testing.T) {
		state := base
		state.Type = "slide"
		state.Direction = "diagonal"
		state.Placement = "in"
		state.PeerClipID = ""
		state.Role = "incoming"
		state.PeerRole = ""
		_, err := EvaluateTransitionPaint("owner", state)
		if err == nil || !strings.Contains(err.Error(), "direction left, right, up, or down") {
			t.Fatalf("slide direction error = %v", err)
		}
	})

	t.Run("progress-must-be-canonical", func(t *testing.T) {
		state := base
		state.Progress = 1.1
		_, err := EvaluateTransitionPaint("owner", state)
		if err == nil || !strings.Contains(err.Error(), "within [0,1]") {
			t.Fatalf("progress error = %v", err)
		}
	})
}

func loadTransitionPaintFixture(t *testing.T) transitionPaintFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve transition paint fixture path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "video-renderer", "test", "fixtures", "transition-paint-v1.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read transition paint fixture: %v", err)
	}
	var fixture transitionPaintFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode transition paint fixture: %v", err)
	}
	if fixture.Version != 1 {
		t.Fatalf("fixture version = %d, want 1", fixture.Version)
	}
	return fixture
}
