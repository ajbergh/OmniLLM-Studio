package rendercontract

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestVisualFrameStateConsumesSupportedTransitionPaint(t *testing.T) {
	fixture := loadTransitionStateFixture(t)

	t.Run("fade-in", func(t *testing.T) {
		state, err := EvaluateVisualFrameState(fixture.Document, 15)
		if err != nil {
			t.Fatalf("EvaluateVisualFrameState: %v", err)
		}
		if !state.Authoritative || len(state.Unresolved) != 0 {
			t.Fatalf("state authoritative=%v unresolved=%v", state.Authoritative, state.Unresolved)
		}
		owner := frameLayerByClipID(state.Layers, "owner")
		if owner == nil || !owner.Authoritative || len(owner.TransitionPaint) != 1 {
			t.Fatalf("owner = %+v", owner)
		}
		paint := owner.TransitionPaint[0]
		if paint.ContractVersion != TransitionPaintContractV1 || paint.TransitionID != "fade-in" || paint.Composition != TransitionPaintOwnerAlpha || paint.OwnerOpacity == nil || math.Abs(*paint.OwnerOpacity-0.5) > 1e-9 {
			t.Fatalf("fade paint = %+v", paint)
		}
	})

	t.Run("crossfade-between", func(t *testing.T) {
		state, err := EvaluateVisualFrameState(fixture.Document, 55)
		if err != nil {
			t.Fatalf("EvaluateVisualFrameState: %v", err)
		}
		if !state.Authoritative || len(state.Unresolved) != 0 || len(state.Layers) != 2 {
			t.Fatalf("state = %+v", state)
		}
		owner := frameLayerByClipID(state.Layers, "owner")
		if owner == nil || len(owner.TransitionPaint) != 1 {
			t.Fatalf("owner = %+v", owner)
		}
		paint := owner.TransitionPaint[0]
		if paint.Composition != TransitionPaintCrossfade || paint.OutgoingClipID != "owner" || paint.IncomingClipID != "peer" || paint.OutgoingWeight == nil || paint.IncomingWeight == nil {
			t.Fatalf("crossfade paint = %+v", paint)
		}
		if math.Abs(*paint.OutgoingWeight-2.0/3.0) > 1e-9 || math.Abs(*paint.IncomingWeight-1.0/3.0) > 1e-9 {
			t.Fatalf("crossfade weights = outgoing %.12f incoming %.12f", *paint.OutgoingWeight, *paint.IncomingWeight)
		}
	})

	t.Run("unsupported-slide-retains-debt", func(t *testing.T) {
		state, err := EvaluateVisualFrameState(fixture.Document, 65)
		if err != nil {
			t.Fatalf("EvaluateVisualFrameState: %v", err)
		}
		want := []string{"owner:transition_paint:slide-out"}
		if state.Authoritative || !reflect.DeepEqual(state.Unresolved, want) {
			t.Fatalf("state authoritative=%v unresolved=%v, want %v", state.Authoritative, state.Unresolved, want)
		}
		owner := frameLayerByClipID(state.Layers, "owner")
		if owner == nil || len(owner.TransitionPaint) != 0 {
			t.Fatalf("owner transition paint = %+v", owner)
		}
	})
}

func TestVisualFrameStateConsumesDipToBlackPaint(t *testing.T) {
	fixture := loadTransitionStateFixture(t)
	doc := cloneTransitionPaintDocument(t, fixture.Document)
	doc.Tracks[0].Clips[0].Transitions[0].Type = "dip_to_black"

	state, err := EvaluateVisualFrameState(doc, 15)
	if err != nil {
		t.Fatalf("EvaluateVisualFrameState: %v", err)
	}
	owner := frameLayerByClipID(state.Layers, "owner")
	if owner == nil || !state.Authoritative || len(owner.TransitionPaint) != 1 {
		t.Fatalf("state=%+v owner=%+v", state, owner)
	}
	paint := owner.TransitionPaint[0]
	if paint.Composition != TransitionPaintDipBlack || paint.IncomingWeight == nil || paint.BlackWeight == nil || math.Abs(*paint.IncomingWeight-0.5) > 1e-9 || math.Abs(*paint.BlackWeight-0.5) > 1e-9 {
		t.Fatalf("dip paint = %+v", paint)
	}
}

func TestVisualFrameStateFailsClosedOnInvalidSupportedTransitionPaint(t *testing.T) {
	fixture := loadTransitionStateFixture(t)
	doc := cloneTransitionPaintDocument(t, fixture.Document)
	doc.Tracks[0].Clips[0].Transitions[0].Type = "crossfade"

	_, err := EvaluateVisualFrameState(doc, 15)
	if err == nil || !strings.Contains(err.Error(), "crossfade requires between") {
		t.Fatalf("paint validation error = %v", err)
	}
}

func cloneTransitionPaintDocument(t *testing.T, source TimelineV2Document) TimelineV2Document {
	t.Helper()
	data, err := json.Marshal(source)
	if err != nil {
		t.Fatalf("marshal document: %v", err)
	}
	var clone TimelineV2Document
	if err := json.Unmarshal(data, &clone); err != nil {
		t.Fatalf("unmarshal document: %v", err)
	}
	return clone
}
