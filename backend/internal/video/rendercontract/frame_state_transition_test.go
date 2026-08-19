package rendercontract

import (
	"reflect"
	"testing"
)

func TestVisualFrameStateTransitionPaintDebtIsFrameScoped(t *testing.T) {
	fixture := loadTransitionStateFixture(t)
	cases := []struct {
		name               string
		frameIndex         int64
		wantUnresolved     []string
		wantActiveID       string
		wantOwnerPresent   bool
		wantOwnerAuthority bool
		wantLayerCount     int
	}{
		{name: "between-transition-windows", frameIndex: 30, wantUnresolved: []string{}, wantOwnerPresent: true, wantOwnerAuthority: true, wantLayerCount: 1},
		{name: "in-transition-paint", frameIndex: 15, wantUnresolved: []string{"owner:transition_paint:fade-in"}, wantActiveID: "fade-in", wantOwnerPresent: true, wantLayerCount: 1},
		{name: "between-transition-paint", frameIndex: 55, wantUnresolved: []string{"owner:transition_paint:between"}, wantActiveID: "between", wantOwnerPresent: true, wantLayerCount: 2},
		{name: "out-transition-paint-after-between-end", frameIndex: 65, wantUnresolved: []string{"owner:transition_paint:slide-out"}, wantActiveID: "slide-out", wantOwnerPresent: true, wantLayerCount: 2},
		{name: "owner-end-is-exclusive", frameIndex: 70, wantUnresolved: []string{}, wantOwnerPresent: false, wantOwnerAuthority: true, wantLayerCount: 1},
	}

	for _, sample := range cases {
		t.Run(sample.name, func(t *testing.T) {
			state, err := EvaluateVisualFrameState(fixture.Document, sample.frameIndex)
			if err != nil {
				t.Fatalf("EvaluateVisualFrameState: %v", err)
			}
			if !reflect.DeepEqual(state.Unresolved, sample.wantUnresolved) {
				t.Fatalf("unresolved = %v, want %v", state.Unresolved, sample.wantUnresolved)
			}
			if state.Authoritative != (len(sample.wantUnresolved) == 0) {
				t.Fatalf("state authoritative = %v unresolved=%v", state.Authoritative, state.Unresolved)
			}
			if len(state.Layers) != sample.wantLayerCount {
				t.Fatalf("layer count = %d, want %d", len(state.Layers), sample.wantLayerCount)
			}

			owner := frameLayerByClipID(state.Layers, "owner")
			if (owner != nil) != sample.wantOwnerPresent {
				t.Fatalf("owner present = %v, want %v", owner != nil, sample.wantOwnerPresent)
			}
			if owner == nil {
				return
			}
			if len(owner.Transitions) != 3 {
				t.Fatalf("owner transitions = %+v", owner.Transitions)
			}
			activeIDs := []string{}
			for _, transition := range owner.Transitions {
				if transition.Active {
					activeIDs = append(activeIDs, transition.ID)
				}
			}
			if sample.wantActiveID == "" {
				if len(activeIDs) != 0 {
					t.Fatalf("active transitions = %v, want none", activeIDs)
				}
			} else if !reflect.DeepEqual(activeIDs, []string{sample.wantActiveID}) {
				t.Fatalf("active transitions = %v, want %q", activeIDs, sample.wantActiveID)
			}
			if owner.Authoritative != sample.wantOwnerAuthority {
				t.Fatalf("owner authoritative = %v, want %v unresolved=%v", owner.Authoritative, sample.wantOwnerAuthority, owner.Unresolved)
			}
		})
	}
}

func frameLayerByClipID(layers []FrameLayerState, clipID string) *FrameLayerState {
	for index := range layers {
		if layers[index].ClipID == clipID {
			return &layers[index]
		}
	}
	return nil
}
