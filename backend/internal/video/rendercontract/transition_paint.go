package rendercontract

import (
	"fmt"
	"math"
	"strings"
)

const (
	TransitionPaintContractV1                = "transition-paint-v1"
	TransitionPaintOwnerAlpha                = "owner-opacity"
	TransitionPaintCrossfade                 = "pair-crossfade"
	TransitionPaintDipBlack                  = "dip-to-black"
	TransitionPaintOwnerTranslate            = "owner-translate"
	TransitionPaintPairSlide                 = "pair-slide"
	TransitionPaintOwnerWipe                 = "owner-wipe"
	TransitionPaintPairWipe                  = "pair-wipe"
	TransitionPaintTranslationCanvasFraction = "canvas-fraction"
	TransitionPaintClipLayerFraction         = "layer-fraction"
)

// SupportsTransitionPaint reports whether transition-paint-v1 defines paint
// semantics for the authored transition type. Consumers use this boundary to
// distinguish a supported-but-invalid state (which must fail closed) from a
// valid transition family whose paint remains intentionally unresolved.
func SupportsTransitionPaint(transitionType string) bool {
	switch strings.ToLower(strings.TrimSpace(transitionType)) {
	case "fade", "crossfade", "dip_to_black", "slide", "wipe":
		return true
	default:
		return false
	}
}

// EvaluatedTransitionPaint is the renderer-independent paint instruction for
// one active transition. Contribution weights, normalized translation, and
// normalized clipping are explicit so preview and export can consume identical
// state without embedding CSS/Canvas/FFmpeg commands in the canonical contract.
type EvaluatedTransitionPaint struct {
	ContractVersion    string   `json:"contract_version"`
	TransitionID       string   `json:"transition_id"`
	Type               string   `json:"type"`
	Placement          string   `json:"placement"`
	Composition        string   `json:"composition"`
	OwnerClipID        string   `json:"owner_clip_id"`
	PeerClipID         string   `json:"peer_clip_id,omitempty"`
	Progress           float64  `json:"progress"`
	OwnerOpacity       *float64 `json:"owner_opacity,omitempty"`
	OutgoingClipID     string   `json:"outgoing_clip_id,omitempty"`
	IncomingClipID     string   `json:"incoming_clip_id,omitempty"`
	OutgoingWeight     *float64 `json:"outgoing_weight,omitempty"`
	IncomingWeight     *float64 `json:"incoming_weight,omitempty"`
	BlackWeight        *float64 `json:"black_weight,omitempty"`
	TranslationSpace   string   `json:"translation_space,omitempty"`
	OwnerOffsetX       *float64 `json:"owner_offset_x,omitempty"`
	OwnerOffsetY       *float64 `json:"owner_offset_y,omitempty"`
	OutgoingOffsetX    *float64 `json:"outgoing_offset_x,omitempty"`
	OutgoingOffsetY    *float64 `json:"outgoing_offset_y,omitempty"`
	IncomingOffsetX    *float64 `json:"incoming_offset_x,omitempty"`
	IncomingOffsetY    *float64 `json:"incoming_offset_y,omitempty"`
	ClipSpace          string   `json:"clip_space,omitempty"`
	OwnerClipTop       *float64 `json:"owner_clip_top,omitempty"`
	OwnerClipRight     *float64 `json:"owner_clip_right,omitempty"`
	OwnerClipBottom    *float64 `json:"owner_clip_bottom,omitempty"`
	OwnerClipLeft      *float64 `json:"owner_clip_left,omitempty"`
	IncomingClipTop    *float64 `json:"incoming_clip_top,omitempty"`
	IncomingClipRight  *float64 `json:"incoming_clip_right,omitempty"`
	IncomingClipBottom *float64 `json:"incoming_clip_bottom,omitempty"`
	IncomingClipLeft   *float64 `json:"incoming_clip_left,omitempty"`
}

// EvaluateTransitionPaint converts canonical transition timing/ownership state
// into canonical visual composition state. Inactive transitions produce no
// paint. Fade is one-sided opacity; crossfade is a true two-input blend;
// dip_to_black uses explicit black contribution; slide uses normalized
// canvas-fraction translation; wipe uses normalized clipping of the isolated
// layer surface. Direction names the entry/reveal edge for slide and wipe.
func EvaluateTransitionPaint(ownerClipID string, state EvaluatedTransitionState) (*EvaluatedTransitionPaint, error) {
	ownerClipID = strings.TrimSpace(ownerClipID)
	if ownerClipID == "" {
		return nil, fmt.Errorf("transition paint owner clip id must not be empty")
	}
	if state.ContractVersion != TransitionStateContractV1 {
		return nil, fmt.Errorf("transition paint requires %s input", TransitionStateContractV1)
	}
	transitionID := strings.TrimSpace(state.ID)
	if transitionID == "" {
		return nil, fmt.Errorf("transition paint transition id must not be empty")
	}
	if !state.Active {
		return nil, nil
	}
	if math.IsNaN(state.Progress) || math.IsInf(state.Progress, 0) || state.Progress < 0 || state.Progress > 1 {
		return nil, fmt.Errorf("transition %q progress must be finite and within [0,1]", transitionID)
	}

	paint := &EvaluatedTransitionPaint{
		ContractVersion: TransitionPaintContractV1,
		TransitionID:    transitionID,
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

	case "slide":
		entryX, entryY, err := transitionSlideEntryVector(state.Direction)
		if err != nil {
			return nil, invalidTransitionPaintState(state, err.Error())
		}
		exitX, exitY := -entryX, -entryY
		paint.TranslationSpace = TransitionPaintTranslationCanvasFraction
		switch paint.Placement {
		case "in":
			if state.Role != "incoming" || paint.PeerClipID != "" {
				return nil, invalidTransitionPaintState(state, "slide-in requires an incoming owner with no peer")
			}
			paint.Composition = TransitionPaintOwnerTranslate
			paint.OwnerOffsetX = transitionPaintSignedFloat(entryX * (1 - state.Progress))
			paint.OwnerOffsetY = transitionPaintSignedFloat(entryY * (1 - state.Progress))
			return paint, nil
		case "out":
			if state.Role != "outgoing" || paint.PeerClipID != "" {
				return nil, invalidTransitionPaintState(state, "slide-out requires an outgoing owner with no peer")
			}
			paint.Composition = TransitionPaintOwnerTranslate
			paint.OwnerOffsetX = transitionPaintSignedFloat(exitX * state.Progress)
			paint.OwnerOffsetY = transitionPaintSignedFloat(exitY * state.Progress)
			return paint, nil
		case "between":
			if paint.PeerClipID == "" {
				return nil, invalidTransitionPaintState(state, "slide between requires an explicit peer")
			}
			outgoingID, incomingID, err := transitionPaintPairRoles(ownerClipID, state)
			if err != nil {
				return nil, err
			}
			paint.Composition = TransitionPaintPairSlide
			paint.OutgoingClipID = outgoingID
			paint.IncomingClipID = incomingID
			paint.OutgoingOffsetX = transitionPaintSignedFloat(exitX * state.Progress)
			paint.OutgoingOffsetY = transitionPaintSignedFloat(exitY * state.Progress)
			paint.IncomingOffsetX = transitionPaintSignedFloat(entryX * (1 - state.Progress))
			paint.IncomingOffsetY = transitionPaintSignedFloat(entryY * (1 - state.Progress))
			return paint, nil
		default:
			return nil, invalidTransitionPaintState(state, "slide requires in, out, or between placement")
		}

	case "wipe":
		paint.ClipSpace = TransitionPaintClipLayerFraction
		switch paint.Placement {
		case "in":
			if state.Role != "incoming" || paint.PeerClipID != "" {
				return nil, invalidTransitionPaintState(state, "wipe-in requires an incoming owner with no peer")
			}
			top, right, bottom, left, err := transitionWipeInsets(state.Direction, state.Progress, false)
			if err != nil {
				return nil, invalidTransitionPaintState(state, err.Error())
			}
			paint.Composition = TransitionPaintOwnerWipe
			paint.OwnerClipTop = transitionPaintSignedFloat(top)
			paint.OwnerClipRight = transitionPaintSignedFloat(right)
			paint.OwnerClipBottom = transitionPaintSignedFloat(bottom)
			paint.OwnerClipLeft = transitionPaintSignedFloat(left)
			return paint, nil
		case "out":
			if state.Role != "outgoing" || paint.PeerClipID != "" {
				return nil, invalidTransitionPaintState(state, "wipe-out requires an outgoing owner with no peer")
			}
			top, right, bottom, left, err := transitionWipeInsets(state.Direction, state.Progress, true)
			if err != nil {
				return nil, invalidTransitionPaintState(state, err.Error())
			}
			paint.Composition = TransitionPaintOwnerWipe
			paint.OwnerClipTop = transitionPaintSignedFloat(top)
			paint.OwnerClipRight = transitionPaintSignedFloat(right)
			paint.OwnerClipBottom = transitionPaintSignedFloat(bottom)
			paint.OwnerClipLeft = transitionPaintSignedFloat(left)
			return paint, nil
		case "between":
			if paint.PeerClipID == "" {
				return nil, invalidTransitionPaintState(state, "wipe between requires an explicit peer")
			}
			outgoingID, incomingID, err := transitionPaintPairRoles(ownerClipID, state)
			if err != nil {
				return nil, err
			}
			top, right, bottom, left, err := transitionWipeInsets(state.Direction, state.Progress, false)
			if err != nil {
				return nil, invalidTransitionPaintState(state, err.Error())
			}
			paint.Composition = TransitionPaintPairWipe
			paint.OutgoingClipID = outgoingID
			paint.IncomingClipID = incomingID
			paint.IncomingClipTop = transitionPaintSignedFloat(top)
			paint.IncomingClipRight = transitionPaintSignedFloat(right)
			paint.IncomingClipBottom = transitionPaintSignedFloat(bottom)
			paint.IncomingClipLeft = transitionPaintSignedFloat(left)
			return paint, nil
		default:
			return nil, invalidTransitionPaintState(state, "wipe requires in, out, or between placement")
		}

	default:
		return nil, fmt.Errorf("transition %q type %q does not yet have canonical paint semantics", transitionID, state.Type)
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

func transitionSlideEntryVector(direction string) (float64, float64, error) {
	switch strings.ToLower(strings.TrimSpace(direction)) {
	case "left":
		return -1, 0, nil
	case "right":
		return 1, 0, nil
	case "up":
		return 0, -1, nil
	case "down":
		return 0, 1, nil
	default:
		return 0, 0, fmt.Errorf("slide requires direction left, right, up, or down")
	}
}

func transitionWipeInsets(direction string, progress float64, outgoing bool) (top, right, bottom, left float64, err error) {
	hidden := 1 - progress
	if outgoing {
		hidden = progress
	}
	switch strings.ToLower(strings.TrimSpace(direction)) {
	case "left":
		if outgoing {
			left = hidden
		} else {
			right = hidden
		}
	case "right":
		if outgoing {
			right = hidden
		} else {
			left = hidden
		}
	case "up":
		if outgoing {
			top = hidden
		} else {
			bottom = hidden
		}
	case "down":
		if outgoing {
			bottom = hidden
		} else {
			top = hidden
		}
	default:
		return 0, 0, 0, 0, fmt.Errorf("wipe requires direction left, right, up, or down")
	}
	return top, right, bottom, left, nil
}

func invalidTransitionPaintState(state EvaluatedTransitionState, message string) error {
	return fmt.Errorf("transition %q: %s", strings.TrimSpace(state.ID), message)
}

func transitionPaintFloat(value float64) *float64 {
	value = math.Max(0, math.Min(1, value))
	return &value
}

func transitionPaintSignedFloat(value float64) *float64 {
	if value == 0 {
		value = 0
	}
	return &value
}
