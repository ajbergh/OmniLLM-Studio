package rendercontract

import (
	"strings"
	"testing"
)

func TestEvaluateClipEffectStackAtFramePreservesOrderAndAutomationPrecedence(t *testing.T) {
	clip := TimelineV2Clip{
		ID:         "clip",
		StartMS:    0,
		DurationMS: 1000,
		Effects: []TimelineV2Effect{
			{ID: "bright", Type: "brightness", Enabled: true, Params: Metadata{}},
			{ID: "disabled", Type: "blur", Enabled: false, Params: Metadata{"amount": 20.0}},
			{ID: "contrast", Type: "contrast", Enabled: true, Params: Metadata{"amount": 2.5}},
		},
		Keyframes: []TimelineV2Keyframe{
			{ID: "type0", Property: "effect.brightness.amount", TimeMS: 0, Value: 0.5},
			{ID: "type1", Property: "effect.brightness.amount", TimeMS: 1000, Value: 0.7},
			{ID: "id0", Property: "effect.bright.amount", TimeMS: 0, Value: 1.1},
			{ID: "id1", Property: "effect.bright.amount", TimeMS: 1000, Value: 1.9},
			{ID: "contrast0", Property: "effect.contrast.amount", TimeMS: 0, Value: 1},
			{ID: "contrast1", Property: "effect.contrast.amount", TimeMS: 1000, Value: 3},
		},
	}

	stack, err := EvaluateClipEffectStackAtFrame(clip, 15, 30)
	if err != nil {
		t.Fatalf("EvaluateClipEffectStackAtFrame: %v", err)
	}
	if len(stack) != 2 {
		t.Fatalf("stack = %+v", stack)
	}
	if stack[0].ID != "bright" || stack[0].Order != 0 || stack[0].Scope != EffectScopeClip || stack[0].ContractVersion != EffectStateContractV1 {
		t.Fatalf("first effect = %+v", stack[0])
	}
	if amount, ok := stack[0].Params["amount"].(float64); !ok || amount != 1.5 {
		t.Fatalf("brightness amount = %#v, want 1.5", stack[0].Params["amount"])
	}
	if stack[1].ID != "contrast" || stack[1].Order != 2 {
		t.Fatalf("second effect = %+v", stack[1])
	}
	if amount, ok := stack[1].Params["amount"].(float64); !ok || amount != 2 {
		t.Fatalf("contrast amount = %#v, want 2", stack[1].Params["amount"])
	}
}

func TestEvaluateSceneEffectStackAppliesCanonicalDefaults(t *testing.T) {
	scene := TimelineV2Scene{Effects: []TimelineV2Effect{
		{ID: "vignette", Type: "vignette", Enabled: true, Params: Metadata{}},
		{ID: "off", Type: "grayscale", Enabled: false, Params: Metadata{}},
	}}
	stack, err := EvaluateSceneEffectStack(&scene)
	if err != nil {
		t.Fatalf("EvaluateSceneEffectStack: %v", err)
	}
	if len(stack) != 1 || stack[0].Scope != EffectScopeScene || stack[0].Order != 0 || stack[0].Params["amount"] != 0.4 {
		t.Fatalf("scene stack = %+v", stack)
	}
}

func TestEvaluateEffectStackFailsClosedOnUnknownParamsAndUndefinedAmountAutomation(t *testing.T) {
	clip := TimelineV2Clip{Effects: []TimelineV2Effect{{ID: "blur", Type: "blur", Enabled: true, Params: Metadata{"mystery": 1.0}}}}
	if _, err := EvaluateClipEffectStackAtFrame(clip, 0, 30); err == nil || !strings.Contains(err.Error(), "unsupported parameter") {
		t.Fatalf("unknown parameter error = %v", err)
	}

	clip = TimelineV2Clip{
		Effects:   []TimelineV2Effect{{ID: "gray", Type: "grayscale", Enabled: true, Params: Metadata{}}},
		Keyframes: []TimelineV2Keyframe{{ID: "amount", Property: "effect.gray.amount", TimeMS: 0, Value: 1}},
	}
	if _, err := EvaluateClipEffectStackAtFrame(clip, 0, 30); err == nil || !strings.Contains(err.Error(), "does not define canonical amount automation") {
		t.Fatalf("undefined amount error = %v", err)
	}
}

func TestVisualFrameStateProjectsClipAndSceneEffects(t *testing.T) {
	doc := TimelineV2Document{
		Version:    2,
		Canvas:     TimelineV2Canvas{Width: 640, Height: 360, FPS: 30, Background: "#000000"},
		DurationMS: 1000,
		Metadata:   Metadata{},
		Tracks: []TimelineV2Track{{
			ID: "track", Type: "layer", Name: "Layer", Visible: true,
			Clips: []TimelineV2Clip{{
				ID: "clip", StartMS: 0, DurationMS: 1000, TrimInMS: 0, TrimOutMS: 1000,
				Effects:   []TimelineV2Effect{{ID: "bright", Type: "brightness", Enabled: true, Params: Metadata{}}},
				Keyframes: []TimelineV2Keyframe{},
			}},
		}},
		Scenes: []TimelineV2Scene{{ID: "scene", Name: "Scene", StartMS: 0, DurationMS: 1000, Effects: []TimelineV2Effect{{ID: "vignette", Type: "vignette", Enabled: true, Params: Metadata{}}}}},
	}
	state, err := EvaluateVisualFrameState(doc, 0)
	if err != nil {
		t.Fatalf("EvaluateVisualFrameState: %v", err)
	}
	if !state.Authoritative || len(state.Unresolved) != 0 || len(state.SceneEffects) != 1 || len(state.Layers) != 1 || len(state.Layers[0].Effects) != 1 {
		t.Fatalf("state = %+v", state)
	}
}
