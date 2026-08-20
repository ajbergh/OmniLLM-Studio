package rendercontract

import "testing"

func TestVisualFrameStateProjectsCanonicalTextWithoutGenericTextDebt(t *testing.T) {
	boxWidth, boxHeight := 320.0, 90.0
	doc := TimelineV2Document{
		Version:    2,
		Canvas:     TimelineV2Canvas{Width: 640, Height: 360, FPS: 30, Background: "#000000"},
		DurationMS: 1000,
		Metadata:   Metadata{},
		Tracks: []TimelineV2Track{{
			ID: "text-track", Type: "text", Name: "Text", Visible: true,
			Clips: []TimelineV2Clip{{
				ID: "title", StartMS: 0, DurationMS: 1000, TrimInMS: 0, TrimOutMS: 1000,
				Text:    &TimelineV2Text{Text: "Canonical", Background: "#111111", BoxWidth: &boxWidth, BoxHeight: &boxHeight},
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
	if layer.Text == nil || layer.Text.ContractVersion != TextStateContractV1 || layer.Text.Text != "Canonical" {
		t.Fatalf("text state = %+v", layer.Text)
	}
	if layer.ContentBounds == nil || layer.ContentBounds.Width != boxWidth || layer.ContentBounds.Height != boxHeight {
		t.Fatalf("content bounds = %+v", layer.ContentBounds)
	}
	if !layer.Authoritative || !state.Authoritative || len(layer.Unresolved) != 0 || len(state.Unresolved) != 0 {
		t.Fatalf("authority layer=%v state=%v unresolved=%v/%v", layer.Authoritative, state.Authoritative, layer.Unresolved, state.Unresolved)
	}
}

func TestVisualFrameStateTextShapeProjectionLeavesOnlyCursorDebt(t *testing.T) {
	doc := TimelineV2Document{
		Version:    2,
		Canvas:     TimelineV2Canvas{Width: 640, Height: 360, FPS: 30, Background: "#000000"},
		DurationMS: 1000,
		Metadata:   Metadata{},
		Tracks: []TimelineV2Track{{
			ID: "track", Type: "layer", Name: "Layer", Visible: true,
			Clips: []TimelineV2Clip{{
				ID: "mixed", StartMS: 0, DurationMS: 1000, TrimInMS: 0, TrimOutMS: 1000,
				Text:    &TimelineV2Text{Text: "Text"},
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
	if len(state.Layers) != 1 || state.Layers[0].Text == nil || state.Layers[0].Shape == nil {
		t.Fatalf("layer = %+v", state.Layers)
	}
	if len(state.Unresolved) != 1 || state.Unresolved[0] != "mixed:cursor" {
		t.Fatalf("unresolved = %v", state.Unresolved)
	}
}
