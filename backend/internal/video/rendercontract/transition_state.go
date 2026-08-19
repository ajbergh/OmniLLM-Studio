package rendercontract

import (
	"fmt"
	"math"
	"strings"
)

const TransitionStateContractV1 = "transition-state-v1"

var canonicalTransitionTypes = map[string]bool{
	"fade": true, "crossfade": true, "dip_to_black": true,
	"slide": true, "wipe": true, "zoom": true,
}

var canonicalTransitionDirections = map[string]bool{
	"left": true, "right": true, "up": true, "down": true,
}

// EvaluatedTransitionState describes one transition's exact output-frame window
// and ownership relationship without defining renderer-specific paint. Between
// transitions require a real temporal overlap and an explicit peer clip so a
// compositor never has to infer hidden source handles or adjacency semantics.
type EvaluatedTransitionState struct {
	ContractVersion string  `json:"contract_version"`
	ID              string  `json:"id"`
	Type            string  `json:"type"`
	Placement       string  `json:"placement"`
	Direction       string  `json:"direction,omitempty"`
	PeerClipID      string  `json:"peer_clip_id,omitempty"`
	PeerTrackIndex  *int    `json:"peer_track_index,omitempty"`
	PeerClipIndex   *int    `json:"peer_clip_index,omitempty"`
	Role            string  `json:"role"`
	PeerRole        string  `json:"peer_role,omitempty"`
	StartFrame      int64   `json:"start_frame"`
	EndFrame        int64   `json:"end_frame"`
	Progress        float64 `json:"progress"`
	Active          bool    `json:"active"`
}

// EvaluateClipTransitionsAtFrame normalizes the Timeline v2 document and
// evaluates all authored transitions for one clip at an exact output frame.
func EvaluateClipTransitionsAtFrame(doc TimelineV2Document, trackIndex, clipIndex int, frameIndex int64) ([]EvaluatedTransitionState, error) {
	normalized, err := NormalizeTimelineV2EvaluationInputs(doc)
	if err != nil {
		return nil, err
	}
	return evaluateClipTransitionsAtFrameNormalized(normalized, trackIndex, clipIndex, frameIndex)
}

func evaluateClipTransitionsAtFrameNormalized(doc TimelineV2Document, trackIndex, clipIndex int, frameIndex int64) ([]EvaluatedTransitionState, error) {
	if trackIndex < 0 || trackIndex >= len(doc.Tracks) || clipIndex < 0 || clipIndex >= len(doc.Tracks[trackIndex].Clips) {
		return nil, fmt.Errorf("transition owner index is outside timeline bounds")
	}
	if frameIndex < 0 || frameIndex >= FrameCount(doc.DurationMS, doc.Canvas.FPS) {
		return nil, fmt.Errorf("frame index %d is outside timeline frame range", frameIndex)
	}
	owner := doc.Tracks[trackIndex].Clips[clipIndex]
	states := make([]EvaluatedTransitionState, 0, len(owner.Transitions))
	seenIDs := map[string]bool{}
	for transitionIndex, transition := range owner.Transitions {
		path := fmt.Sprintf("tracks[%d].clips[%d].transitions[%d]", trackIndex, clipIndex, transitionIndex)
		id := strings.TrimSpace(transition.ID)
		if id == "" {
			return nil, fmt.Errorf("%s.id: transition id must not be empty", path)
		}
		if seenIDs[id] {
			return nil, fmt.Errorf("%s.id: duplicate transition id %q", path, id)
		}
		seenIDs[id] = true
		if transition.DurationMS < 1 {
			return nil, fmt.Errorf("%s.duration_ms: transition duration must be positive", path)
		}
		typeName := strings.ToLower(strings.TrimSpace(transition.Type))
		if !canonicalTransitionTypes[typeName] {
			return nil, fmt.Errorf("%s.type: unsupported transition type %q", path, transition.Type)
		}
		direction, err := normalizedTransitionDirection(transition)
		if err != nil {
			return nil, fmt.Errorf("%s.direction: %w", path, err)
		}
		state := EvaluatedTransitionState{
			ContractVersion: TransitionStateContractV1,
			ID: id,
			Type: typeName,
			Placement: strings.ToLower(strings.TrimSpace(transition.Placement)),
			Direction: direction,
		}
		var startMS, endMS int64
		switch state.Placement {
		case "in":
			if strings.TrimSpace(transition.PeerClipID) != "" {
				return nil, fmt.Errorf("%s.peer_clip_id: in transitions must not declare a peer", path)
			}
			startMS = owner.StartMS
			// Clip boundaries are authoritative; an authored transition longer
			// than the clip saturates across the full clip rather than requiring
			// hidden source handles.
			endMS = minInt64Transition(owner.StartMS+transition.DurationMS, owner.StartMS+owner.DurationMS)
			state.Role = "incoming"
		case "out":
			if strings.TrimSpace(transition.PeerClipID) != "" {
				return nil, fmt.Errorf("%s.peer_clip_id: out transitions must not declare a peer", path)
			}
			endMS = owner.StartMS + owner.DurationMS
			startMS = maxInt64Transition(owner.StartMS, endMS-transition.DurationMS)
			state.Role = "outgoing"
		case "between":
			peerID := strings.TrimSpace(transition.PeerClipID)
			if peerID == "" {
				return nil, fmt.Errorf("%s.peer_clip_id: between transitions require an explicit peer", path)
			}
			if peerID == owner.ID {
				return nil, fmt.Errorf("%s.peer_clip_id: transition peer must differ from owner", path)
			}
			peerTrackIndex, peerClipIndex, peer, ok := findTransitionPeer(doc, peerID)
			if !ok {
				return nil, fmt.Errorf("%s.peer_clip_id: peer clip %q does not exist", path, peerID)
			}
			if peer.StartMS == owner.StartMS {
				return nil, fmt.Errorf("%s.peer_clip_id: between transition owner and peer must have distinct start times", path)
			}
			overlapStart := maxInt64Transition(owner.StartMS, peer.StartMS)
			overlapEnd := minInt64Transition(owner.StartMS+owner.DurationMS, peer.StartMS+peer.DurationMS)
			overlapDuration := overlapEnd - overlapStart
			if overlapDuration < transition.DurationMS {
				return nil, fmt.Errorf("%s.duration_ms: between transition requires at least %dms of real owner/peer overlap", path, transition.DurationMS)
			}
			startMS = overlapStart
			endMS = overlapStart + transition.DurationMS
			state.PeerClipID = peerID
			state.PeerTrackIndex = intPointerTransition(peerTrackIndex)
			state.PeerClipIndex = intPointerTransition(peerClipIndex)
			if owner.StartMS < peer.StartMS {
				state.Role, state.PeerRole = "outgoing", "incoming"
			} else {
				state.Role, state.PeerRole = "incoming", "outgoing"
			}
		default:
			return nil, fmt.Errorf("%s.placement: unsupported transition placement %q", path, transition.Placement)
		}
		if endMS <= startMS {
			return nil, fmt.Errorf("%s.duration_ms: transition window is empty", path)
		}
		state.StartFrame = StartFrame(startMS, doc.Canvas.FPS)
		state.EndFrame = EndFrame(endMS, doc.Canvas.FPS)
		state.Active = state.StartFrame <= frameIndex && frameIndex < state.EndFrame
		presentationMS := float64(frameIndex*1000) / float64(doc.Canvas.FPS)
		state.Progress = clampTransitionProgress((presentationMS - float64(startMS)) / float64(endMS-startMS))
		states = append(states, state)
	}
	return states, nil
}

func findTransitionPeer(doc TimelineV2Document, peerID string) (int, int, TimelineV2Clip, bool) {
	for trackIndex, track := range doc.Tracks {
		for clipIndex, clip := range track.Clips {
			if clip.ID == peerID {
				return trackIndex, clipIndex, clip, true
			}
		}
	}
	return 0, 0, TimelineV2Clip{}, false
}

func normalizedTransitionDirection(transition TimelineV2Transition) (string, error) {
	direction := strings.ToLower(strings.TrimSpace(transition.Direction))
	if direction == "" && (strings.EqualFold(transition.Type, "slide") || strings.EqualFold(transition.Type, "wipe")) {
		return "left", nil
	}
	if direction != "" && !canonicalTransitionDirections[direction] {
		return "", fmt.Errorf("unsupported transition direction %q", transition.Direction)
	}
	return direction, nil
}

func clampTransitionProgress(value float64) float64 {
	if math.IsNaN(value) || value <= 0 {
		return 0
	}
	if value >= 1 {
		return 1
	}
	return value
}

func intPointerTransition(value int) *int {
	copied := value
	return &copied
}

func minInt64Transition(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func maxInt64Transition(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
