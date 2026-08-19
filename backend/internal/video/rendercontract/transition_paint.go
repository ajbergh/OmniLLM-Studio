package rendercontract

import (
	"fmt"
	"math"
	"strings"
)

const (
	TransitionPaintContractV1 = "transition-paint-v1"
	TransitionPaintOwnerAlpha = "owner-opacity"
	TransitionPaintCrossfade  = "pair-crossfade"
	TransitionPaintDipBlack   = "dip-to-black"
)

// EvaluatedTransitionPaint is the renderer-independent paint instruction for
// one active fade-family transition. It deliberately describes contribution
// weights instead of CSS/Canvas/FFmpeg operations so preview and export can
// consume identical semantics. Unsupported transition families fail closed.
type EvaluatedTransitionPaint struct {
	ContractVersion string   `json:"contract_version"`
	TransitionID    string   `json:"transition_id"`
	Type            string   `json:"type"`
	Placement       string   `json:"placement"`
	Composition     string   `json:"composition"`
	OwnerClipID     string   `json:"owner_clip_id"`
	PeerClipID      string   `json:"peer_clip_id,omitempty"`
	Progress        float64  `json:"progress"`
	OwnerOpacity    *float64 `json:"owner_opacity,omitempty"`
	OutgoingClipID  string   `json:"outgoing_clip_id,omitempty"`
	IncomingClipID  string   `json:"incoming_clip_id,omitempty"`
	OutgoingWeight  *float64 `json:"outgoing_weight,omitempty"`
	IncomingWeight  *float64 `json:"incoming_weight,omitempty"`
	BlackWeight     *float64 `json:"black_weight,omitempty"`
}

// EvaluateTransitionPaint converts canonical transition timing/ownership state
// into canonical visual contribution weights. Inactive transitions produce no
// paint. Fade is intentionally one-sided and valid only for in/out placement;
// crossfade is a true two-input blend and therefore requires between placement;
// dip_to_black supports in, out, and between with a black midpoint.
func EvaluateTransitionPaint(ownerClipID string, state EvaluatedTransitionState) (*EvaluatedTransitionPaint, error) {
	ownerClipID = strings.TrimSpace(ownerClipID)
	if ownerClipID == "" {
		return nil, fmt.Errorf("transition paint owner clip id must not be empty")
	}
	if state.ContractVersion != TransitionStateContractV1 {
		return nil, fmt.Errorf("transition paint requires %s input", TransitionStateContractV1)
	}
	if !state.Active {
		return nil, nil
	}
	if math.IsNaN(state.Progress) || math.IsInf(state.Progress, 0) || state.Progress < 0 || state.Progress > 1 {
		return nil, fmt.Errorf("transition %q progress must be finite and within [0,1]", state.ID)
	}

	paint := &EvaluatedTransitionPaint{
		ContractVersion: TransitionPaintContractV1,
		TransitionID:    state.ID,
		Type:            strings.ToLower(strings.TrimSpace(state.Type)),
		Placement:       strings.ToLower(strings.TrimSpace(state.Placement)),
		OwnerClipID:     ownerClipID,
		PeerClipID:      strings.TrimSpace(state.PeerClipID),
		Progress:        state.Progress,
	}

	switch paint.Type {
	case "fade":
		if paint.Placement == "in" {
			if state.Role != "incoming" || paint.PeerClipID != "" {
				return nil, invalidTransitionPaintState(state, "fade-in requires an incoming owner with no peer")
			}
			paint.Composition = TransitionPaintOwnerAlpha
			paint.OwnerOpacity = transitionPaintFloat(state.Progress)
			return paint, nil
		}
		if paint.Placement == "out" {
			if state.Role != "outgoing" || paint.PeerClipID != "" {
				return nil, invalidTransitionPaintState(state, "fade-out requires an outgoing owner with no peer")
			}
			paint.Composition = TransitionPaintOwnerAlpha
			paint.OwnerOpacity = transitionPaintFloat(1 - state.Progress)
			return paint, nil
		}
		return nil, invalidTransitionPaintState(state, "fade supports only in or out placement; use crossfade for a two-clip blend")

	case "crossfade":
		if paint.Placement != "between" || paint.PeerClipID == "" {
			return nil, invalidTransitionPaintState(state, "crossfade requires between placement with an explicit peer")
		}
		outgoingID, incomingID, err := transitionPaintPairRoles(ownerClipID, state)
		if err != nil {
			return nil, err
		}
		paint.Composition = TransitionPaintCrossfade
		paint.OutgoingClipID = outgoingID
		paint.IncomingClipID = incomingID
		paint.OutgoingWeight = transitionPaintFloat(1 - state.Progress)
		paint.IncomingWeight = transitionPaintFloat(state.Progress)
		return paint, nil

	case "dip_to_black":
		paint.Composition = TransitionPaintDipBlack
		switch paint.Placement {
		case "in":
			if state.Role != "incoming" || paint.PeerClipID != "" {
				return nil, invalidTransitionPaintState(state, "dip-to-black in requires an incoming owner with no peer")
			}
			paint.IncomingClipID = ownerClipID
			paint.IncomingWeight = transitionPaintFloat(state.Progress)
			paint.BlackWeight = transitionPaintFloat(1 - state.Progress)
			return paint, nil
		case "out":
			if state.Role != "outgoing" || paint.PeerClipID != "" {
				return nil, invalidTransitionPaintState(state, "dip-to-black out requires an outgoing owner with no peer")
			}
			paint.OutgoingClipID = ownerClipID
			paint.OutgoingWeight = transitionPaintFloat(1 - state.Progress)
			paint.BlackWeight = transitionPaintFloat(state.Progress)
			return paint, nil
		case "between":
			if paint.PeerClipID == "" {
				return nil, invalidTransitionPaintState(state, "dip-to-black between requires an explicit peer")
			}
			outgoingID, incomingID, err := transitionPaintPairRoles(ownerClipID, state)
			if err != nil {
				return nil, err
			}
			paint.OutgoingClipID = outgoingID
			paint.IncomingClipID = incomingID
			if state.Progress < 0.5 {
				paint.OutgoingWeight = transitionPaintFloat(1 - 2*state.Progress)
				paint.IncomingWeight = transitionPaintFloat(0)
				paint.BlackWeight = transitionPaintFloat(2 * state.Progress)
			} else {
				paint.OutgoingWeight = transitionPaintFloat(0)
				paint.IncomingWeight = transitionPaintFloat(2*state.Progress - 1)
				paint.BlackWeight = transitionPaintFloat(2 * (1 - state.Progress))
			}
			return paint, nil
		default:
			return nil, invalidTransitionPaintState(state, "dip-to-black requires in, out, or between placement")
		}

	default:
		return nil, fmt.Errorf("transition %q type %q does not yet have canonical paint semantics", state.ID, state.Type)
	}
}

func transitionPaintPairRoles(ownerClipID string, state EvaluatedTransitionState) (outgoingID, incomingID string, err error) {
	peerID := strings.TrimSpace(state.PeerClipID)
	if peerID == "" || peerID == ownerClipID {
		return "", "", invalidTransitionPaintState(state, "between paint requires a distinct peer")
	}
	switch {
	case state.Role == "outgoing" && state.PeerRole == "incoming":
		return ownerClipID, peerID, nil
	case state.Role == "incoming" && state.PeerRole == "outgoing":
		return peerID, ownerClipID, nil
	default:
		return "", "", invalidTransitionPaintState(state, "between paint requires complementary outgoing/incoming roles")
	}
}

func invalidTransitionPaintState(state EvaluatedTransitionState, message string) error {
	return fmt.Errorf("transition %q: %s", state.ID, message)
}

func transitionPaintFloat(value float64) *float64 {
	value = math.Max(0, math.Min(1, value))
	return &value
}
