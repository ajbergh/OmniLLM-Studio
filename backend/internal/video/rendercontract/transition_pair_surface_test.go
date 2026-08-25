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

type transitionPairSurfaceFixture struct {
	Version int `json:"version"`
	Cases   []struct {
		Name     string                             `json:"name"`
		State    VisualFrameState                   `json:"state"`
		Expected EvaluatedTransitionPairSurfacePlan `json:"expected"`
	} `json:"cases"`
}

func TestEvaluateTransitionPairSurfacePlanMatchesSharedFixture(t *testing.T) {
	fixture := loadTransitionPairSurfaceFixture(t)
	for _, sample := range fixture.Cases {
		t.Run(sample.Name, func(t *testing.T) {
			plan, err := EvaluateTransitionPairSurfacePlan(sample.State)
			if err != nil {
				t.Fatalf("EvaluateTransitionPairSurfacePlan: %v", err)
			}
			if !reflect.DeepEqual(plan, sample.Expected) {
				got, _ := json.Marshal(plan)
				want, _ := json.Marshal(sample.Expected)
				t.Fatalf("plan = %s, want %s", got, want)
			}
		})
	}
}

func TestEvaluateTransitionPairSurfacePlanFailsClosedOnInvalidOwnership(t *testing.T) {
	frame := VisualFrameState{
		ContractVersion: VisualFrameStateContractV1,
		FrameIndex:      5,
		Authoritative:   true,
		Layers: []FrameLayerState{
			{
				ClipID:        "owner",
				Authoritative: true,
				TransitionPaint: []EvaluatedTransitionPaint{{
					ContractVersion: TransitionPaintContractV1,
					TransitionID:    "crossfade",
					Type:            "crossfade",
					Placement:       "between",
					Composition:     TransitionPaintCrossfade,
					OwnerClipID:     "wrong-owner",
					PeerClipID:      "peer",
					Progress:        0.5,
					OutgoingClipID:  "owner",
					IncomingClipID:  "peer",
				}},
			},
			{ClipID: "peer", Authoritative: true},
		},
	}
	_, err := EvaluateTransitionPairSurfacePlan(frame)
	if err == nil || !strings.Contains(err.Error(), "owner clip id must match containing layer") {
		t.Fatalf("owner mismatch error = %v", err)
	}
}

func TestEvaluateTransitionPairSurfacePlanRejectsOverlappingPairClaims(t *testing.T) {
	paint := func(id, owner, peer, outgoing, incoming string) EvaluatedTransitionPaint {
		return EvaluatedTransitionPaint{
			ContractVersion: TransitionPaintContractV1,
			TransitionID:    id,
			Type:            "crossfade",
			Placement:       "between",
			Composition:     TransitionPaintCrossfade,
			OwnerClipID:     owner,
			PeerClipID:      peer,
			Progress:        0.5,
			OutgoingClipID:  outgoing,
			IncomingClipID:  incoming,
		}
	}
	frame := VisualFrameState{
		ContractVersion: VisualFrameStateContractV1,
		FrameIndex:      6,
		Authoritative:   true,
		Layers: []FrameLayerState{
			{ClipID: "a", Authoritative: true, TransitionPaint: []EvaluatedTransitionPaint{paint("ab", "a", "b", "a", "b")}},
			{ClipID: "b", Authoritative: true, TransitionPaint: []EvaluatedTransitionPaint{paint("bc", "b", "c", "b", "c")}},
			{ClipID: "c", Authoritative: true},
		},
	}
	_, err := EvaluateTransitionPairSurfacePlan(frame)
	if err == nil || !strings.Contains(err.Error(), "multiple pair surfaces") {
		t.Fatalf("pair claim conflict error = %v", err)
	}
}

func loadTransitionPairSurfaceFixture(t *testing.T) transitionPairSurfaceFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve transition pair surface fixture path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "video-renderer", "test", "fixtures", "transition-pair-surface-plan-v1.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read transition pair surface fixture: %v", err)
	}
	var fixture transitionPairSurfaceFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode transition pair surface fixture: %v", err)
	}
	if fixture.Version != 1 {
		t.Fatalf("fixture version = %d, want 1", fixture.Version)
	}
	return fixture
}
