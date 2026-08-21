package rendercontract

import (
	"strings"
	"testing"
)

func cursorFrameStateDocument(cursor *TimelineV2Cursor) TimelineV2Document {
	return TimelineV2Document{
		Version:    2,
		Canvas:     TimelineV2Canvas{Width: 640, Height: 360, FPS: 120, Background: "#000000"},
		DurationMS: 1000,
		Metadata:   Metadata{},
		Tracks: []TimelineV2Track{{
			ID: "cursor-track", Type: "layer", Name: "Cursor", Visible: true,
			Clips: []TimelineV2Clip{{
				ID: "cursor-clip", StartMS: 5, DurationMS: 500, TrimInMS: 0, TrimOutMS: 500,
				Cursor: cursor, Effects: []TimelineV2Effect{}, Keyframes: []TimelineV2Keyframe{},
			}},
		}},
		Markers: []TimelineV2Marker{}, Scenes: []TimelineV2Scene{},
	}
}

func TestVisualFrameStateProjectsCanonicalCursorAtExactRationalTime(t *testing.T) {
	scale := 1.5
	cursor := &TimelineV2Cursor{
		Scale: &scale, Highlight: true, ClickRings: true,
		Events: []TimelineV2CursorEvent{
			{TimeMS: 0, X: 10, Y: 20},
			{TimeMS: 10, X: 40, Y: 50, Click: true},
		},
	}
	state, err := EvaluateVisualFrameState(cursorFrameStateDocument(cursor), 1)
	if err != nil {
		t.Fatalf("EvaluateVisualFrameState: %v", err)
	}
	if len(state.Layers) != 1 {
		t.Fatalf("layers = %+v", state.Layers)
	}
	layer := state.Layers[0]
	if layer.Cursor == nil || layer.Cursor.ContractVersion != CursorStateContractV1 {
		t.Fatalf("cursor = %+v", layer.Cursor)
	}
	if layer.Cursor.X != 20 || layer.Cursor.Y != 30 || layer.Cursor.Scale != scale || !layer.Cursor.Highlight || !layer.Cursor.ClickRings || !layer.Cursor.Click {
		t.Fatalf("cursor sample = %+v", layer.Cursor)
	}
	if !layer.Authoritative || !state.Authoritative || len(layer.Unresolved) != 0 || len(state.Unresolved) != 0 {
		t.Fatalf("authority layer=%v state=%v unresolved=%v/%v", layer.Authoritative, state.Authoritative, layer.Unresolved, state.Unresolved)
	}
}

func TestVisualFrameStateTreatsHiddenOrEmptyCursorAsResolvedNoPaint(t *testing.T) {
	hidden := false
	for _, cursor := range []*TimelineV2Cursor{
		{Visible: &hidden, Events: []TimelineV2CursorEvent{{TimeMS: 0, X: 1, Y: 2}}},
		{},
	} {
		state, err := EvaluateVisualFrameState(cursorFrameStateDocument(cursor), 1)
		if err != nil {
			t.Fatalf("EvaluateVisualFrameState: %v", err)
		}
		if len(state.Layers) != 1 || state.Layers[0].Cursor != nil {
			t.Fatalf("layers = %+v", state.Layers)
		}
		if !state.Layers[0].Authoritative || !state.Authoritative || len(state.Unresolved) != 0 {
			t.Fatalf("resolved no-paint state = %+v", state)
		}
	}
}

func TestVisualFrameStateFailsClosedOnInvalidCursorState(t *testing.T) {
	cursor := &TimelineV2Cursor{
		Smoothing: true,
		Events:    []TimelineV2CursorEvent{{TimeMS: 0, X: 1, Y: 2}},
	}
	_, err := EvaluateVisualFrameState(cursorFrameStateDocument(cursor), 1)
	if err == nil || !strings.Contains(err.Error(), "canonical cursor state for clip \"cursor-clip\"") || !strings.Contains(err.Error(), "smoothing") {
		t.Fatalf("error = %v", err)
	}
}
