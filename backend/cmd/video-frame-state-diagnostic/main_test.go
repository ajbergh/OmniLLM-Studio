package main

import (
	"slices"
	"testing"

	videopkg "github.com/ajbergh/omnillm-studio/internal/video"
	"github.com/ajbergh/omnillm-studio/internal/video/rendercontract"
)

func TestCompareDiagnosticSampleMatchesStructuredUnavailability(t *testing.T) {
	errorValue := &videopkg.VisualFrameStateDiagnosticError{
		Code: "V1_TRANSITION_PLACEMENT_AMBIGUOUS", Path: "tracks[0].clips[0].transitions[0]",
		Message: "runtime-specific wording", Remediation: "version placement semantics",
	}
	preview := videopkg.VisualFrameStateDiagnostic{
		ContractVersion: videopkg.VisualFrameStateDiagnosticV1, FrameIndex: 2, Error: errorValue,
	}
	backend := preview
	backend.Error = &videopkg.VisualFrameStateDiagnosticError{
		Code: errorValue.Code, Path: errorValue.Path, Message: "different explanatory wording", Remediation: "same semantic action",
	}

	result := compareDiagnosticSample(fixtureSample{Name: "transition", FrameIndex: 2}, preview, backend)
	if result.Status != "matched_unavailable" || len(result.MismatchPaths) != 0 {
		t.Fatalf("unexpected comparison: %+v", result)
	}
}

func TestCompareDiagnosticSampleUsesNumericTolerance(t *testing.T) {
	previewState := &rendercontract.VisualFrameState{
		ContractVersion: rendercontract.VisualFrameStateContractV1,
		FrameIndex:      1,
		FrameTime:       rendercontract.RationalTime{Numerator: 1, Denominator: 30},
		Canvas:          rendercontract.TimelineV2Canvas{Width: 640, Height: 360, FPS: 30, Background: "#000000"},
		Camera:          rendercontract.EvaluatedCamera{X: 1.1234567891, FieldOfView: 50},
		Layers:          []rendercontract.FrameLayerState{},
		Unresolved:      []string{},
		Authoritative:   true,
	}
	backendState := *previewState
	backendState.Camera.X += numericTolerance / 10
	preview := videopkg.VisualFrameStateDiagnostic{ContractVersion: videopkg.VisualFrameStateDiagnosticV1, FrameIndex: 1, Available: true, State: previewState}
	backend := videopkg.VisualFrameStateDiagnostic{ContractVersion: videopkg.VisualFrameStateDiagnosticV1, FrameIndex: 1, Available: true, State: &backendState}

	result := compareDiagnosticSample(fixtureSample{Name: "numeric", FrameIndex: 1}, preview, backend)
	if result.Status != "matched" || len(result.MismatchPaths) != 0 {
		t.Fatalf("unexpected comparison: %+v", result)
	}
	if result.PreviewFingerprint == "" || result.BackendFingerprint == "" {
		t.Fatal("expected diagnostic fingerprints")
	}
}

func TestCompareDiagnosticSampleReportsNumericDriftPath(t *testing.T) {
	previewState := &rendercontract.VisualFrameState{
		ContractVersion: rendercontract.VisualFrameStateContractV1,
		FrameIndex:      1,
		FrameTime:       rendercontract.RationalTime{Numerator: 1, Denominator: 30},
		Canvas:          rendercontract.TimelineV2Canvas{Width: 640, Height: 360, FPS: 30, Background: "#000000"},
		Camera:          rendercontract.EvaluatedCamera{X: 1, FieldOfView: 50},
		Layers:          []rendercontract.FrameLayerState{},
		Unresolved:      []string{},
		Authoritative:   true,
	}
	backendState := *previewState
	backendState.Camera.X = 1.01
	preview := videopkg.VisualFrameStateDiagnostic{ContractVersion: videopkg.VisualFrameStateDiagnosticV1, FrameIndex: 1, Available: true, State: previewState}
	backend := videopkg.VisualFrameStateDiagnostic{ContractVersion: videopkg.VisualFrameStateDiagnosticV1, FrameIndex: 1, Available: true, State: &backendState}

	result := compareDiagnosticSample(fixtureSample{Name: "drift", FrameIndex: 1}, preview, backend)
	if result.Status != "mismatch" || !slices.Contains(result.MismatchPaths, "state.camera.x") {
		t.Fatalf("unexpected comparison: %+v", result)
	}
}
