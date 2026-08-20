package rendercontract

import (
	"strings"
	"testing"
)

func TestVisualFrameStateProjectsCanonicalShapeWithoutGenericShapeDebt(t *testing.T) {
	width, height := 240, 120
	doc := TimelineV2Document{
		Version:    2,
		Canvas:     TimelineV2Canvas{Width: 640, Height: 360, FPS: 30, Background: "#000000"},
		DurationMS: 1000,
		Metadata:   Metadata{},
		Tracks: []TimelineV2Track{{
			ID: "shape-track", Type: "layer", Name: "Shape", Visible: true,
			Clips: []TimelineV2Clip{{
				ID: "callout", StartMS: 0, DurationMS: 1000, TrimInMS: 0, TrimOutMS: 1000,
				Shape:   &TimelineV2Shape{Kind: ShapeKindLabel, Width: &width, Height: &height},
				Effects: []TimelineV2Effect{}, Keyframes: []TimelineV2Keyframe{},
			}},
		}},
		Markers: []TimelineV2Marker{}, Scenes: []TimelineV2Scene{},
	}
	state, err := EvaluateVisualFrameState(doc, 0)
	if err != nil {
		t.Fatalf("EvaluateVisualFrameState: %v", err)
	}
	if len(state.Layers) != 1 {
		t.Fatalf("layers = %+v", state.Layers)
	}
	layer := state.Layers[0]
	if layer.Shape == nil || layer.Shape.ContractVersion != ShapeStateContractV1 || layer.Shape.Kind != ShapeKindLabel {
		t.Fatalf("shape state = %+v", layer.Shape)
	}
	if layer.Shape.Width != float64(width) || layer.Shape.Height != float64(height) || layer.Shape.Stroke != "" {
		t.Fatalf("shape dimensions/defaults = %+v", layer.Shape)
	}
	if layer.ContentBounds == nil || layer.ContentBounds.Width != float64(width) || layer.ContentBounds.Height != float64(height) {
		t.Fatalf("content bounds = %+v", layer.ContentBounds)
	}
	if !layer.Authoritative || !state.Authoritative || len(layer.Unresolved) != 0 || len(state.Unresolved) != 0 {
		t.Fatalf("authority layer=%v state=%v unresolved=%v/%v", layer.Authoritative, state.Authoritative, layer.Unresolved, state.Unresolved)
	}
}

func TestVisualFrameStateLeavesOnlyCursorDebtAfterShapeProjection(t *testing.T) {
	doc := TimelineV2Document{
		Version:    2,
		Canvas:     TimelineV2Canvas{Width: 640, Height: 360, FPS: 30, Background: "#000000"},
		DurationMS: 1000,
		Metadata:   Metadata{},
		Tracks: []TimelineV2Track{{
			ID: "track", Type: "layer", Name: "Layer", Visible: true,
			Clips: []TimelineV2Clip{{
				ID: "mixed", StartMS: 0, DurationMS: 1000, TrimInMS: 0, TrimOutMS: 1000,
				Shape:   &TimelineV2Shape{Kind: ShapeKindRectangle},
				Cursor:  &TimelineV2Cursor{Visible: true},
				Effects: []TimelineV2Effect{}, Keyframes: []TimelineV2Keyframe{},
			}},
		}},
		Markers: []TimelineV2Marker{}, Scenes: []TimelineV2Scene{},
	}
	state, err := EvaluateVisualFrameState(doc, 0)
	if err != nil {
		t.Fatalf("EvaluateVisualFrameState: %v", err)
	}
	if len(state.Layers) != 1 || state.Layers[0].Shape == nil {
		t.Fatalf("layers = %+v", state.Layers)
	}
	if len(state.Unresolved) != 1 || state.Unresolved[0] != "mixed:cursor" {
		t.Fatalf("unresolved = %v", state.Unresolved)
	}
}

func TestVisualFrameStateFailsClosedOnInvalidShapeState(t *testing.T) {
	zero := 0
	doc := TimelineV2Document{
		Version:    2,
		Canvas:     TimelineV2Canvas{Width: 640, Height: 360, FPS: 30, Background: "#000000"},
		DurationMS: 1000,
		Metadata:   Metadata{},
		Tracks: []TimelineV2Track{{
			ID: "track", Type: "layer", Name: "Layer", Visible: true,
			Clips: []TimelineV2Clip{{
				ID: "bad-shape", StartMS: 0, DurationMS: 1000, TrimInMS: 0, TrimOutMS: 1000,
				Shape: &TimelineV2Shape{Kind: ShapeKindRectangle, Width: &zero}, Effects: []TimelineV2Effect{}, Keyframes: []TimelineV2Keyframe{},
			}},
		}},
		Markers: []TimelineV2Marker{}, Scenes: []TimelineV2Scene{},
	}
	_, err := EvaluateVisualFrameState(doc, 0)
	if err == nil || !strings.Contains(err.Error(), "canonical shape state") || !strings.Contains(err.Error(), "width") {
		t.Fatalf("error = %v", err)
	}
}
