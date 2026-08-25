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

type transitionPairPixelFixture struct {
	Version int `json:"version"`
	Cases   []struct {
		Name     string                                  `json:"name"`
		Surface  EvaluatedTransitionPairSurface          `json:"surface"`
		Paint    EvaluatedTransitionPaint                `json:"paint"`
		Expected EvaluatedTransitionPairPixelComposition `json:"expected"`
	} `json:"cases"`
}

func TestEvaluateTransitionPairPixelCompositionMatchesSharedFixture(t *testing.T) {
	fixture := loadTransitionPairPixelFixture(t)
	for _, sample := range fixture.Cases {
		t.Run(sample.Name, func(t *testing.T) {
			got, err := EvaluateTransitionPairPixelComposition(sample.Surface, sample.Paint)
			if err != nil {
				t.Fatalf("EvaluateTransitionPairPixelComposition: %v", err)
			}
			if !reflect.DeepEqual(got, sample.Expected) {
				gotJSON, _ := json.Marshal(got)
				wantJSON, _ := json.Marshal(sample.Expected)
				t.Fatalf("composition = %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestEvaluateTransitionPairPixelCompositionFailsClosedOnSurfaceMismatch(t *testing.T) {
	surface, paint := validPairPixelCrossfade()
	surface.UpperClipID = "unrelated"
	_, err := EvaluateTransitionPairPixelComposition(surface, paint)
	if err == nil || !strings.Contains(err.Error(), "lower/upper clips must be the outgoing/incoming pair") {
		t.Fatalf("surface membership error = %v", err)
	}
}

func TestEvaluateTransitionPairPixelCompositionFailsClosedOnPaintMismatch(t *testing.T) {
	surface, paint := validPairPixelCrossfade()
	paint.TransitionID = "different"
	_, err := EvaluateTransitionPairPixelComposition(surface, paint)
	if err == nil || !strings.Contains(err.Error(), "transition id must match surface") {
		t.Fatalf("paint transition mismatch error = %v", err)
	}
}

func TestEvaluateTransitionPairPixelCompositionRejectsInvalidWeightedSum(t *testing.T) {
	surface, paint := validPairPixelCrossfade()
	outgoing := 0.8
	incoming := 0.3
	paint.OutgoingWeight = &outgoing
	paint.IncomingWeight = &incoming
	_, err := EvaluateTransitionPairPixelComposition(surface, paint)
	if err == nil || !strings.Contains(err.Error(), "pair weights must sum to 1") {
		t.Fatalf("invalid weighted sum error = %v", err)
	}
}

func TestEvaluateTransitionPairPixelCompositionRejectsWeightsOnSourceOver(t *testing.T) {
	surface, paint := validPairPixelCrossfade()
	surface.Composition = TransitionPaintPairSlide
	paint.Composition = TransitionPaintPairSlide
	paint.OutgoingWeight = transitionPairPixelFloat(0.5)
	paint.IncomingWeight = nil
	_, err := EvaluateTransitionPairPixelComposition(surface, paint)
	if err == nil || !strings.Contains(err.Error(), "must not carry pair weights") {
		t.Fatalf("source-over weight error = %v", err)
	}
}

func TestEvaluateTransitionPairPixelCompositionRejectsUnsupportedComposition(t *testing.T) {
	surface, paint := validPairPixelCrossfade()
	surface.Composition = "pair-unknown"
	paint.Composition = "pair-unknown"
	paint.OutgoingWeight = nil
	paint.IncomingWeight = nil
	_, err := EvaluateTransitionPairPixelComposition(surface, paint)
	if err == nil || !strings.Contains(err.Error(), "does not have pair-pixel semantics") {
		t.Fatalf("unsupported composition error = %v", err)
	}
}

func validPairPixelCrossfade() (EvaluatedTransitionPairSurface, EvaluatedTransitionPaint) {
	outgoing := 0.75
	incoming := 0.25
	return EvaluatedTransitionPairSurface{
			TransitionID:          "crossfade",
			Composition:           TransitionPaintCrossfade,
			OwnerClipID:           "outgoing",
			PeerClipID:            "incoming",
			OutgoingClipID:        "outgoing",
			IncomingClipID:        "incoming",
			LowerClipID:           "outgoing",
			UpperClipID:           "incoming",
			LowerLayerIndex:       0,
			UpperLayerIndex:       1,
			ReplacementLayerIndex: 0,
		}, EvaluatedTransitionPaint{
			ContractVersion: TransitionPaintContractV1,
			TransitionID:    "crossfade",
			Type:            "crossfade",
			Placement:       "between",
			Composition:     TransitionPaintCrossfade,
			OwnerClipID:     "outgoing",
			PeerClipID:      "incoming",
			Progress:        0.25,
			OutgoingClipID:  "outgoing",
			IncomingClipID:  "incoming",
			OutgoingWeight:  &outgoing,
			IncomingWeight:  &incoming,
		}
}

func loadTransitionPairPixelFixture(t *testing.T) transitionPairPixelFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve transition pair pixel fixture path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "video-renderer", "test", "fixtures", "transition-pair-pixel-composition-v1.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read transition pair pixel fixture: %v", err)
	}
	var fixture transitionPairPixelFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode transition pair pixel fixture: %v", err)
	}
	if fixture.Version != 1 {
		t.Fatalf("fixture version = %d, want 1", fixture.Version)
	}
	return fixture
}
