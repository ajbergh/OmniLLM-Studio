package rendercontract

import (
	"math"
	"testing"
)

func TestVisualFrameStateConsumesWipePaint(t *testing.T) {
	fixture := loadTransitionStateFixture(t)
	doc := cloneTransitionPaintDocument(t, fixture.Document)
	doc.Tracks[0].Clips[0].Transitions[0].Type = "wipe"
	doc.Tracks[0].Clips[0].Transitions[0].Direction = "left"

	state, err := EvaluateVisualFrameState(doc, 15)
	if err != nil {
		t.Fatalf("EvaluateVisualFrameState: %v", err)
	}
	owner := frameLayerByClipID(state.Layers, "owner")
	if owner == nil || !state.Authoritative || len(state.Unresolved) != 0 || len(owner.TransitionPaint) != 1 {
		t.Fatalf("state=%+v owner=%+v", state, owner)
	}
	paint := owner.TransitionPaint[0]
	if paint.Composition != TransitionPaintOwnerWipe || paint.ClipSpace != TransitionPaintClipLayerFraction || paint.OwnerClipTop == nil || paint.OwnerClipRight == nil || paint.OwnerClipBottom == nil || paint.OwnerClipLeft == nil {
		t.Fatalf("wipe paint = %+v", paint)
	}
	if math.Abs(*paint.OwnerClipTop) > 1e-9 || math.Abs(*paint.OwnerClipRight-0.5) > 1e-9 || math.Abs(*paint.OwnerClipBottom) > 1e-9 || math.Abs(*paint.OwnerClipLeft) > 1e-9 {
		t.Fatalf("wipe insets = top %.12f right %.12f bottom %.12f left %.12f", *paint.OwnerClipTop, *paint.OwnerClipRight, *paint.OwnerClipBottom, *paint.OwnerClipLeft)
	}
}
