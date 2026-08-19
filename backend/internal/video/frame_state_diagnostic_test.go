package video

import (
	"strings"
	"testing"
)

func diagnosticCompatibleTimeline() TimelineDocument {
	doc := NewEmptyTimeline(640, 360, 30)
	doc.DurationMS = 1000
	doc.Tracks[0].Clips = []TimelineClip{{
		ID:           "clip-1",
		AssetID:      "asset-1",
		StartMS:      0,
		DurationMS:   1000,
		TrimInMS:     100,
		TrimOutMS:    1100,
		PlaybackRate: 1,
		Transform: map[string]any{
			"x": 12.0, "y": -4.0, "scale": 1.0, "rotation": 0.0, "opacity": 1.0,
		},
		Effects:   []TimelineEffect{},
		Keyframes: []TimelineKeyframe{},
	}}
	return doc
}

func TestEvaluateVisualFrameStateDiagnosticAvailable(t *testing.T) {
	result := EvaluateVisualFrameStateDiagnostic(diagnosticCompatibleTimeline(), 3)
	if result.ContractVersion != VisualFrameStateDiagnosticV1 {
		t.Fatalf("contract version = %q", result.ContractVersion)
	}
	if !result.Available || result.Error != nil || result.State == nil {
		t.Fatalf("expected available state, got %+v", result)
	}
	if result.State.FrameIndex != 3 || len(result.State.Layers) != 1 {
		t.Fatalf("unexpected state identity: %+v", result.State)
	}
	layer := result.State.Layers[0]
	if layer.SourceTimeMS != 200 {
		t.Fatalf("source time = %v, want 200", layer.SourceTimeMS)
	}
	if layer.Transform.X != 12 {
		t.Fatalf("transform x = %v, want 12", layer.Transform.X)
	}
}

func TestEvaluateVisualFrameStateDiagnosticPreservesTransitionAmbiguity(t *testing.T) {
	doc := diagnosticCompatibleTimeline()
	doc.Tracks[0].Clips[0].Transitions = []TimelineTransition{{
		ID: "transition-1", Type: TransitionTypeFade, DurationMS: 100,
	}}

	result := EvaluateVisualFrameStateDiagnostic(doc, 0)
	if result.Available || result.State != nil || result.Error == nil {
		t.Fatalf("expected unavailable diagnostic, got %+v", result)
	}
	if result.Error.Code != canonicalAdapterTransitionCode {
		t.Fatalf("error code = %q, want %q", result.Error.Code, canonicalAdapterTransitionCode)
	}
	if !strings.Contains(result.Error.Path, "transitions[0]") {
		t.Fatalf("error path = %q", result.Error.Path)
	}
	if result.Error.Remediation == "" {
		t.Fatal("expected remediation")
	}
}

func TestEvaluateVisualFrameStateDiagnosticContainsEvaluatorFailure(t *testing.T) {
	result := EvaluateVisualFrameStateDiagnostic(diagnosticCompatibleTimeline(), 9999)
	if result.Available || result.Error == nil {
		t.Fatalf("expected unavailable diagnostic, got %+v", result)
	}
	if result.Error.Code != FrameStateEvaluationFailed {
		t.Fatalf("error code = %q, want %q", result.Error.Code, FrameStateEvaluationFailed)
	}
	if !strings.Contains(result.Error.Message, "outside timeline frame range") {
		t.Fatalf("error message = %q", result.Error.Message)
	}
}
