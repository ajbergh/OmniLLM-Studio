package rendercontract

import (
	"math"
	"testing"
)

func TestVisualFrameStateConsumesZoomPaint(t *testing.T) {
	fixture := loadTransitionStateFixture(t)
	doc := cloneTransitionPaintDocument(t, fixture.Document)
	doc.Tracks[0].Clips[0].Transitions[0].Type = "zoom"

	state, err := EvaluateVisualFrameState(doc, 15)
	if err != nil {
		t.Fatalf("EvaluateVisualFrameState: %v", err)
	}
	owner := frameLayerByClipID(state.Layers, "owner")
	if owner == nil || !state.Authoritative || len(state.Unresolved) != 0 || len(owner.TransitionPaint) != 1 {
		t.Fatalf("state=%+v owner=%+v", state, owner)
	}
	paint := owner.TransitionPaint[0]
	if paint.Composition != TransitionPaintOwnerZoom || paint.ScaleSpace != TransitionPaintScaleLayerMultiplier || paint.OwnerOpacity == nil || paint.OwnerScale == nil {
		t.Fatalf("zoom paint = %+v", paint)
	}
	if math.Abs(*paint.OwnerOpacity-0.5) > 1e-12 || math.Abs(*paint.OwnerScale-0.955) > 1e-12 {
		t.Fatalf("zoom opacity/scale = %.15f / %.15f", *paint.OwnerOpacity, *paint.OwnerScale)
	}
}
