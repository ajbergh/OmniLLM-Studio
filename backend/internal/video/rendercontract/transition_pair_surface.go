package rendercontract

import (
	"fmt"
	"strings"
)

const TransitionPairSurfacePlanV1 = "transition-pair-surface-plan-v1"

const (
	TransitionPairDeferredFrameNonAuthoritative = "frame-non-authoritative"
	TransitionPairDeferredInputMissing          = "pair-input-missing"
	TransitionPairDeferredInputNonAuthoritative = "pair-input-non-authoritative"
	TransitionPairDeferredInputsNotAdjacent     = "pair-inputs-not-adjacent"
)

// EvaluatedTransitionPairSurface identifies the two adjacent canonical input
// layers that may be replaced by one isolated transition surface without
// changing the ordering of any unrelated active layer. Pixel blend semantics
// remain owned by transition-paint-v1 and are deliberately not redefined here.
type EvaluatedTransitionPairSurface struct {
	TransitionID          string `json:"transition_id"`
	Composition           string `json:"composition"`
	OwnerClipID           string `json:"owner_clip_id"`
	PeerClipID            string `json:"peer_clip_id"`
	OutgoingClipID        string `json:"outgoing_clip_id"`
	IncomingClipID        string `json:"incoming_clip_id"`
	LowerClipID           string `json:"lower_clip_id"`
	UpperClipID           string `json:"upper_clip_id"`
	LowerLayerIndex       int    `json:"lower_layer_index"`
	UpperLayerIndex       int    `json:"upper_layer_index"`
	ReplacementLayerIndex int    `json:"replacement_layer_index"`
}

type DeferredTransitionPairSurface struct {
	TransitionID   string `json:"transition_id"`
	OwnerClipID    string `json:"owner_clip_id"`
	PeerClipID     string `json:"peer_clip_id"`
	OutgoingClipID string `json:"outgoing_clip_id"`
	IncomingClipID string `json:"incoming_clip_id"`
	Reason         string `json:"reason"`
}

type EvaluatedTransitionPairSurfacePlan struct {
	ContractVersion string                           `json:"contract_version"`
	FrameIndex      int64                            `json:"frame_index"`
	Surfaces        []EvaluatedTransitionPairSurface `json:"surfaces"`
	Deferred        []DeferredTransitionPairSurface  `json:"deferred"`
	Authoritative   bool                             `json:"authoritative"`
}

// EvaluateTransitionPairSurfacePlan resolves stack-safe pair-transition
// placement from an already-evaluated visual FrameState. Pair inputs must be
// adjacent in canonical bottom-to-top layer order; otherwise grouping them
// would reorder an unrelated layer and therefore remains explicitly deferred.
func EvaluateTransitionPairSurfacePlan(frame VisualFrameState) (EvaluatedTransitionPairSurfacePlan, error) {
	if frame.ContractVersion != VisualFrameStateContractV1 {
		return EvaluatedTransitionPairSurfacePlan{}, fmt.Errorf("transition pair surfaces require %s input", VisualFrameStateContractV1)
	}
	plan := EvaluatedTransitionPairSurfacePlan{
		ContractVersion: TransitionPairSurfacePlanV1,
		FrameIndex:      frame.FrameIndex,
		Surfaces:        []EvaluatedTransitionPairSurface{},
		Deferred:        []DeferredTransitionPairSurface{},
		Authoritative:   frame.Authoritative,
	}
	indexByClip := make(map[string]int, len(frame.Layers))
	for index, layer := range frame.Layers {
		clipID := strings.TrimSpace(layer.ClipID)
		if clipID == "" {
			return EvaluatedTransitionPairSurfacePlan{}, fmt.Errorf("layers[%d].clip_id: pair surface clip id must not be empty", index)
		}
		if _, exists := indexByClip[clipID]; exists {
			return EvaluatedTransitionPairSurfacePlan{}, fmt.Errorf("layers[%d].clip_id: duplicate active clip id %q", index, clipID)
		}
		indexByClip[clipID] = index
	}

	claimed := map[string]bool{}
	for ownerIndex, ownerLayer := range frame.Layers {
		for _, paint := range ownerLayer.TransitionPaint {
			pair, err := transitionPaintRequiresPairSurface(paint)
			if err != nil {
				return EvaluatedTransitionPairSurfacePlan{}, err
			}
			if !pair {
				continue
			}
			if strings.TrimSpace(paint.TransitionID) == "" {
				return EvaluatedTransitionPairSurfacePlan{}, fmt.Errorf("layers[%d].transition_paint: transition id must not be empty", ownerIndex)
			}
			ownerClipID := strings.TrimSpace(ownerLayer.ClipID)
			if strings.TrimSpace(paint.OwnerClipID) != ownerClipID {
				return EvaluatedTransitionPairSurfacePlan{}, fmt.Errorf("layers[%d].transition_paint: owner clip id must match containing layer", ownerIndex)
			}
			outgoing := strings.TrimSpace(paint.OutgoingClipID)
			incoming := strings.TrimSpace(paint.IncomingClipID)
			peer := strings.TrimSpace(paint.PeerClipID)
			if outgoing == "" || incoming == "" || peer == "" || outgoing == incoming {
				return EvaluatedTransitionPairSurfacePlan{}, fmt.Errorf("layers[%d].transition_paint: pair paint requires distinct outgoing/incoming clips and a peer", ownerIndex)
			}
			if !pairIDsMatchOwnerPeer(ownerClipID, peer, outgoing, incoming) {
				return EvaluatedTransitionPairSurfacePlan{}, fmt.Errorf("layers[%d].transition_paint: pair paint inputs must be owner and peer", ownerIndex)
			}
			if claimed[outgoing] || claimed[incoming] {
				return EvaluatedTransitionPairSurfacePlan{}, fmt.Errorf("layers[%d].transition_paint: a clip cannot participate in multiple pair surfaces at one frame", ownerIndex)
			}

			deferredBase := DeferredTransitionPairSurface{
				TransitionID:   paint.TransitionID,
				OwnerClipID:    ownerClipID,
				PeerClipID:     peer,
				OutgoingClipID: outgoing,
				IncomingClipID: incoming,
			}
			outgoingIndex, outgoingOK := indexByClip[outgoing]
			incomingIndex, incomingOK := indexByClip[incoming]
			if !outgoingOK || !incomingOK {
				deferredBase.Reason = TransitionPairDeferredInputMissing
				plan.Deferred = append(plan.Deferred, deferredBase)
				plan.Authoritative = false
				continue
			}
			if !frame.Authoritative {
				deferredBase.Reason = TransitionPairDeferredFrameNonAuthoritative
				plan.Deferred = append(plan.Deferred, deferredBase)
				plan.Authoritative = false
				continue
			}
			if !frame.Layers[outgoingIndex].Authoritative || !frame.Layers[incomingIndex].Authoritative {
				deferredBase.Reason = TransitionPairDeferredInputNonAuthoritative
				plan.Deferred = append(plan.Deferred, deferredBase)
				plan.Authoritative = false
				continue
			}

			lowerIndex, upperIndex := outgoingIndex, incomingIndex
			if lowerIndex > upperIndex {
				lowerIndex, upperIndex = upperIndex, lowerIndex
			}
			if upperIndex != lowerIndex+1 {
				deferredBase.Reason = TransitionPairDeferredInputsNotAdjacent
				plan.Deferred = append(plan.Deferred, deferredBase)
				plan.Authoritative = false
				continue
			}

			claimed[outgoing] = true
			claimed[incoming] = true
			plan.Surfaces = append(plan.Surfaces, EvaluatedTransitionPairSurface{
				TransitionID:          paint.TransitionID,
				Composition:           paint.Composition,
				OwnerClipID:           ownerClipID,
				PeerClipID:            peer,
				OutgoingClipID:        outgoing,
				IncomingClipID:        incoming,
				LowerClipID:           frame.Layers[lowerIndex].ClipID,
				UpperClipID:           frame.Layers[upperIndex].ClipID,
				LowerLayerIndex:       lowerIndex,
				UpperLayerIndex:       upperIndex,
				ReplacementLayerIndex: lowerIndex,
			})
		}
	}
	return plan, nil
}

func transitionPaintRequiresPairSurface(paint EvaluatedTransitionPaint) (bool, error) {
	if paint.ContractVersion != TransitionPaintContractV1 {
		return false, fmt.Errorf("transition pair surfaces require %s instructions", TransitionPaintContractV1)
	}
	switch paint.Composition {
	case TransitionPaintCrossfade, TransitionPaintPairSlide, TransitionPaintPairWipe, TransitionPaintPairZoom:
		return true, nil
	case TransitionPaintDipBlack:
		return strings.TrimSpace(paint.OutgoingClipID) != "" && strings.TrimSpace(paint.IncomingClipID) != "", nil
	default:
		return false, nil
	}
}

func pairIDsMatchOwnerPeer(owner, peer, outgoing, incoming string) bool {
	if owner == peer || outgoing == incoming {
		return false
	}
	return (outgoing == owner && incoming == peer) || (outgoing == peer && incoming == owner)
}
