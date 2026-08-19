package video

import (
	"errors"

	"github.com/ajbergh/omnillm-studio/internal/video/rendercontract"
)

const (
	VisualFrameStateDiagnosticV1 = "visual-frame-state-diagnostic-v1"
	FrameStateEvaluationFailed   = "FRAME_STATE_EVALUATION_FAILED"
)

// VisualFrameStateDiagnosticError is a machine-comparable reason why a v1
// timeline cannot currently produce canonical visual FrameState. Adapter and
// runtime-normalization errors preserve their shared code/path/remediation;
// generic evaluator failures use FrameStateEvaluationFailed.
type VisualFrameStateDiagnosticError struct {
	Code        string `json:"code"`
	Path        string `json:"path"`
	Message     string `json:"message"`
	Remediation string `json:"remediation"`
}

// VisualFrameStateDiagnostic is an observational envelope used by parity
// tooling. Available=false is a valid result: unsupported/ambiguous v1
// semantics remain fail-closed instead of being silently projected into v2.
type VisualFrameStateDiagnostic struct {
	ContractVersion string                           `json:"contract_version"`
	FrameIndex      int64                            `json:"frame_index"`
	Available       bool                             `json:"available"`
	State           *rendercontract.VisualFrameState `json:"state,omitempty"`
	Error           *VisualFrameStateDiagnosticError `json:"error,omitempty"`
}

// EvaluateVisualFrameStateDiagnostic adapts the persisted editor-compatible
// v1 timeline through the canonical v1→v2 adapter, then evaluates FrameState.
// It never weakens adapter validation for diagnostic convenience.
func EvaluateVisualFrameStateDiagnostic(doc TimelineDocument, frameIndex int64) VisualFrameStateDiagnostic {
	result := VisualFrameStateDiagnostic{
		ContractVersion: VisualFrameStateDiagnosticV1,
		FrameIndex:      frameIndex,
	}
	canonical, err := AdaptTimelineV1ToV2(doc)
	if err != nil {
		result.Error = frameStateDiagnosticError(err)
		return result
	}
	state, err := rendercontract.EvaluateVisualFrameState(canonical, frameIndex)
	if err != nil {
		result.Error = frameStateDiagnosticError(err)
		return result
	}
	result.Available = true
	result.State = &state
	return result
}

func frameStateDiagnosticError(err error) *VisualFrameStateDiagnosticError {
	var adapterErr *CanonicalAdapterError
	if errors.As(err, &adapterErr) {
		return &VisualFrameStateDiagnosticError{
			Code:        adapterErr.Code,
			Path:        adapterErr.Path,
			Message:     adapterErr.Message,
			Remediation: adapterErr.Remediation,
		}
	}
	var runtimeErr *rendercontract.TimelineV2RuntimeError
	if errors.As(err, &runtimeErr) {
		return &VisualFrameStateDiagnosticError{
			Code:        runtimeErr.Code,
			Path:        runtimeErr.Path,
			Message:     runtimeErr.Message,
			Remediation: runtimeErr.Remediation,
		}
	}
	message := "canonical FrameState evaluation failed"
	if err != nil {
		message = err.Error()
	}
	return &VisualFrameStateDiagnosticError{
		Code:        FrameStateEvaluationFailed,
		Path:        "",
		Message:     message,
		Remediation: "inspect the canonical FrameState evaluator input and keep unsupported semantics fail-closed",
	}
}
