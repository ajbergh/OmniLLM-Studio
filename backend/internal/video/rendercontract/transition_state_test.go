package rendercontract

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type transitionStateFixture struct {
	Version  int                `json:"version"`
	Document TimelineV2Document `json:"document"`
	Cases    []struct {
		Name       string `json:"name"`
		FrameIndex int64  `json:"frame_index"`
		Expected   []struct {
			ID             string  `json:"id"`
			Placement      string  `json:"placement"`
			Role           string  `json:"role"`
			PeerRole       string  `json:"peer_role"`
			PeerClipID     string  `json:"peer_clip_id"`
			PeerTrackIndex *int    `json:"peer_track_index"`
			PeerClipIndex  *int    `json:"peer_clip_index"`
			Direction      string  `json:"direction"`
			StartFrame     int64   `json:"start_frame"`
			EndFrame       int64   `json:"end_frame"`
			Progress       float64 `json:"progress"`
			Active         bool    `json:"active"`
		} `json:"expected"`
	} `json:"cases"`
}

func TestEvaluateClipTransitionsAtFrameMatchesSharedFixture(t *testing.T) {
	fixture := loadTransitionStateFixture(t)
	for _, sample := range fixture.Cases {
		t.Run(sample.Name, func(t *testing.T) {
			states, err := EvaluateClipTransitionsAtFrame(fixture.Document, 0, 0, sample.FrameIndex)
			if err != nil {
				t.Fatalf("EvaluateClipTransitionsAtFrame: %v", err)
			}
			if len(states) != len(sample.Expected) {
				t.Fatalf("states length = %d, want %d", len(states), len(sample.Expected))
			}
			for index, expected := range sample.Expected {
				state := states[index]
				if state.ContractVersion != TransitionStateContractV1 || state.ID != expected.ID || state.Placement != expected.Placement || state.Role != expected.Role || state.PeerRole != expected.PeerRole || state.PeerClipID != expected.PeerClipID || state.Direction != expected.Direction || state.StartFrame != expected.StartFrame || state.EndFrame != expected.EndFrame || state.Active != expected.Active {
					t.Fatalf("state[%d] = %+v, expected %+v", index, state, expected)
				}
				assertOptionalIntEqual(t, "peer_track_index", state.PeerTrackIndex, expected.PeerTrackIndex)
				assertOptionalIntEqual(t, "peer_clip_index", state.PeerClipIndex, expected.PeerClipIndex)
				if math.Abs(state.Progress-expected.Progress) > 1e-9 {
					t.Fatalf("state[%d].progress = %.12f, want %.12f", index, state.Progress, expected.Progress)
				}
			}
		})
	}
}

func TestEvaluateClipTransitionsAtFrameFailsClosedOnInvalidBetweenPeer(t *testing.T) {
	fixture := loadTransitionStateFixture(t)
	doc := fixture.Document
	doc.Tracks[0].Clips[0].Transitions[1].PeerClipID = "missing"
	_, err := EvaluateClipTransitionsAtFrame(doc, 0, 0, 55)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("missing peer error = %v", err)
	}
}

func TestEvaluateClipTransitionsAtFrameFailsClosedWithoutRequiredOverlap(t *testing.T) {
	fixture := loadTransitionStateFixture(t)
	doc := fixture.Document
	doc.Tracks[1].Clips[0].StartMS = 650
	_, err := EvaluateClipTransitionsAtFrame(doc, 0, 0, 65)
	if err == nil || !strings.Contains(err.Error(), "real owner/peer overlap") {
		t.Fatalf("overlap error = %v", err)
	}
}

func TestEvaluateClipTransitionsAtFrameFailsClosedOnUnknownRuntimeSemantics(t *testing.T) {
	fixture := loadTransitionStateFixture(t)
	t.Run("type", func(t *testing.T) {
		doc := fixture.Document
		doc.Tracks[0].Clips[0].Transitions[0].Type = "future-transition"
		_, err := EvaluateClipTransitionsAtFrame(doc, 0, 0, 15)
		if err == nil || !strings.Contains(err.Error(), "unsupported transition type") {
			t.Fatalf("type error = %v", err)
		}
	})
	t.Run("direction", func(t *testing.T) {
		doc := fixture.Document
		doc.Tracks[0].Clips[0].Transitions[2].Direction = "diagonal"
		_, err := EvaluateClipTransitionsAtFrame(doc, 0, 0, 65)
		if err == nil || !strings.Contains(err.Error(), "unsupported transition direction") {
			t.Fatalf("direction error = %v", err)
		}
	})
}

func loadTransitionStateFixture(t *testing.T) transitionStateFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve transition state fixture path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "video-renderer", "test", "fixtures", "transition-state-v1.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read transition state fixture: %v", err)
	}
	var fixture transitionStateFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode transition state fixture: %v", err)
	}
	if fixture.Version != 1 {
		t.Fatalf("fixture version = %d, want 1", fixture.Version)
	}
	return fixture
}

func assertOptionalIntEqual(t *testing.T, label string, got, want *int) {
	t.Helper()
	if got == nil || want == nil {
		if got != nil || want != nil {
			t.Fatalf("%s = %v, want %v", label, got, want)
		}
		return
	}
	if *got != *want {
		t.Fatalf("%s = %d, want %d", label, *got, *want)
	}
}
