package rendercontract

import (
	"fmt"
	"math"
	"strings"
)

const TransitionPairPixelCompositionV1 = "transition-pair-pixel-composition-v1"

const (
	TransitionPairPixelWorkingColorLinearSRGB = "linear-srgb"
	TransitionPairPixelInputAlphaStraight      = "straight"
	TransitionPairPixelAccumulatorPremultiplied = "premultiplied"
	TransitionPairPixelOutputAlphaStraight     = "straight"
	TransitionPairPixelBlendWeightedSum        = "weighted-sum"
	TransitionPairPixelBlendSourceOverStack    = "source-over-stack"
	TransitionPairPixelBlackOpaque             = "opaque-linear-black"
)

// EvaluatedTransitionPairPixelComposition defines renderer-independent pixel
// composition semantics for one stack-safe isolated pair surface. Spatial
// transforms/clips/scales stay owned by transition-paint-v1; this contract only
// fixes how the resulting pair samples contribute to the replacement surface.
type EvaluatedTransitionPairPixelComposition struct {
	ContractVersion   string   `json:"contract_version"`
	TransitionID      string   `json:"transition_id"`
	Composition       string   `json:"composition"`
	WorkingColorSpace string   `json:"working_color_space"`
	InputAlpha        string   `json:"input_alpha"`
	AccumulatorAlpha  string   `json:"accumulator_alpha"`
	OutputAlpha       string   `json:"output_alpha"`
	BlendOperator     string   `json:"blend_operator"`
	LowerClipID       string   `json:"lower_clip_id"`
	UpperClipID       string   `json:"upper_clip_id"`
	OutgoingClipID    string   `json:"outgoing_clip_id"`
	IncomingClipID    string   `json:"incoming_clip_id"`
	OutgoingWeight    *float64 `json:"outgoing_weight,omitempty"`
	IncomingWeight    *float64 `json:"incoming_weight,omitempty"`
	BlackWeight       *float64 `json:"black_weight,omitempty"`
	BlackSource       string   `json:"black_source,omitempty"`
	StackBottomClipID string   `json:"stack_bottom_clip_id,omitempty"`
	StackTopClipID    string   `json:"stack_top_clip_id,omitempty"`
}

// EvaluateTransitionPairPixelComposition binds exact pair-pixel semantics to a
// previously approved stack-safe surface and its canonical transition paint.
// Weighted families multiply each already-rendered input's premultiplied
// linear-sRGB contribution and alpha exactly once, then sum and clamp to [0,1].
// Source-over families preserve canonical lower/upper order after paint-owned
// spatial operations and add no transition weight of their own.
func EvaluateTransitionPairPixelComposition(
	surface EvaluatedTransitionPairSurface,
	paint EvaluatedTransitionPaint,
) (EvaluatedTransitionPairPixelComposition, error) {
	if strings.TrimSpace(surface.TransitionID) == "" {
		return EvaluatedTransitionPairPixelComposition{}, fmt.Errorf("transition pair pixel composition requires a transition id")
	}
	if paint.ContractVersion != TransitionPaintContractV1 {
		return EvaluatedTransitionPairPixelComposition{}, fmt.Errorf("transition pair pixel composition requires %s paint", TransitionPaintContractV1)
	}
	if strings.TrimSpace(paint.TransitionID) != strings.TrimSpace(surface.TransitionID) {
		return EvaluatedTransitionPairPixelComposition{}, fmt.Errorf("transition pair pixel composition paint transition id must match surface")
	}
	if strings.TrimSpace(paint.Composition) != strings.TrimSpace(surface.Composition) {
		return EvaluatedTransitionPairPixelComposition{}, fmt.Errorf("transition pair pixel composition paint composition must match surface")
	}
	if strings.TrimSpace(paint.OutgoingClipID) != strings.TrimSpace(surface.OutgoingClipID) ||
		strings.TrimSpace(paint.IncomingClipID) != strings.TrimSpace(surface.IncomingClipID) {
		return EvaluatedTransitionPairPixelComposition{}, fmt.Errorf("transition pair pixel composition paint inputs must match surface")
	}
	if strings.TrimSpace(surface.LowerClipID) == "" || strings.TrimSpace(surface.UpperClipID) == "" || surface.LowerClipID == surface.UpperClipID {
		return EvaluatedTransitionPairPixelComposition{}, fmt.Errorf("transition pair pixel composition requires distinct lower and upper clips")
	}
	if !pairIDsMatchOwnerPeer(surface.OwnerClipID, surface.PeerClipID, surface.OutgoingClipID, surface.IncomingClipID) {
		return EvaluatedTransitionPairPixelComposition{}, fmt.Errorf("transition pair pixel composition surface inputs must be owner and peer")
	}

	result := EvaluatedTransitionPairPixelComposition{
		ContractVersion:   TransitionPairPixelCompositionV1,
		TransitionID:      surface.TransitionID,
		Composition:       surface.Composition,
		WorkingColorSpace: TransitionPairPixelWorkingColorLinearSRGB,
		InputAlpha:        TransitionPairPixelInputAlphaStraight,
		AccumulatorAlpha:  TransitionPairPixelAccumulatorPremultiplied,
		OutputAlpha:       TransitionPairPixelOutputAlphaStraight,
		LowerClipID:       surface.LowerClipID,
		UpperClipID:       surface.UpperClipID,
		OutgoingClipID:    surface.OutgoingClipID,
		IncomingClipID:    surface.IncomingClipID,
	}

	switch surface.Composition {
	case TransitionPaintCrossfade, TransitionPaintPairZoom:
		outgoing, incoming, err := requireTransitionPairWeights(paint, false)
		if err != nil {
			return EvaluatedTransitionPairPixelComposition{}, err
		}
		result.BlendOperator = TransitionPairPixelBlendWeightedSum
		result.OutgoingWeight = transitionPairPixelFloat(outgoing)
		result.IncomingWeight = transitionPairPixelFloat(incoming)
		return result, nil

	case TransitionPaintDipBlack:
		outgoing, incoming, black, err := requireTransitionPairBlackWeights(paint)
		if err != nil {
			return EvaluatedTransitionPairPixelComposition{}, err
		}
		result.BlendOperator = TransitionPairPixelBlendWeightedSum
		result.OutgoingWeight = transitionPairPixelFloat(outgoing)
		result.IncomingWeight = transitionPairPixelFloat(incoming)
		result.BlackWeight = transitionPairPixelFloat(black)
		result.BlackSource = TransitionPairPixelBlackOpaque
		return result, nil

	case TransitionPaintPairSlide, TransitionPaintPairWipe:
		if paint.OutgoingWeight != nil || paint.IncomingWeight != nil || paint.BlackWeight != nil {
			return EvaluatedTransitionPairPixelComposition{}, fmt.Errorf("transition %q source-over pair paint must not carry pair weights", surface.TransitionID)
		}
		result.BlendOperator = TransitionPairPixelBlendSourceOverStack
		result.StackBottomClipID = surface.LowerClipID
		result.StackTopClipID = surface.UpperClipID
		return result, nil

	default:
		return EvaluatedTransitionPairPixelComposition{}, fmt.Errorf("transition %q composition %q does not have pair-pixel semantics", surface.TransitionID, surface.Composition)
	}
}

func requireTransitionPairWeights(paint EvaluatedTransitionPaint, allowBlack bool) (float64, float64, error) {
	if paint.OutgoingWeight == nil || paint.IncomingWeight == nil {
		return 0, 0, fmt.Errorf("transition %q weighted pair paint requires outgoing and incoming weights", paint.TransitionID)
	}
	if !allowBlack && paint.BlackWeight != nil {
		return 0, 0, fmt.Errorf("transition %q weighted pair paint must not carry black weight", paint.TransitionID)
	}
	outgoing := *paint.OutgoingWeight
	incoming := *paint.IncomingWeight
	if err := validateTransitionPairPixelWeight("outgoing", outgoing); err != nil {
		return 0, 0, fmt.Errorf("transition %q %w", paint.TransitionID, err)
	}
	if err := validateTransitionPairPixelWeight("incoming", incoming); err != nil {
		return 0, 0, fmt.Errorf("transition %q %w", paint.TransitionID, err)
	}
	if !transitionPairPixelUnitSum(outgoing + incoming) {
		return 0, 0, fmt.Errorf("transition %q pair weights must sum to 1", paint.TransitionID)
	}
	return outgoing, incoming, nil
}

func requireTransitionPairBlackWeights(paint EvaluatedTransitionPaint) (float64, float64, float64, error) {
	if paint.OutgoingWeight == nil || paint.IncomingWeight == nil || paint.BlackWeight == nil {
		return 0, 0, 0, fmt.Errorf("transition %q dip-to-black pair paint requires outgoing, incoming, and black weights", paint.TransitionID)
	}
	outgoing := *paint.OutgoingWeight
	incoming := *paint.IncomingWeight
	black := *paint.BlackWeight
	for label, weight := range map[string]float64{"outgoing": outgoing, "incoming": incoming, "black": black} {
		if err := validateTransitionPairPixelWeight(label, weight); err != nil {
			return 0, 0, 0, fmt.Errorf("transition %q %w", paint.TransitionID, err)
		}
	}
	if !transitionPairPixelUnitSum(outgoing + incoming + black) {
		return 0, 0, 0, fmt.Errorf("transition %q dip-to-black weights must sum to 1", paint.TransitionID)
	}
	return outgoing, incoming, black, nil
}

func validateTransitionPairPixelWeight(label string, weight float64) error {
	if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 || weight > 1 {
		return fmt.Errorf("%s weight must be finite and within [0,1]", label)
	}
	return nil
}

func transitionPairPixelUnitSum(value float64) bool {
	return math.Abs(value-1) <= 1e-9
}

func transitionPairPixelFloat(value float64) *float64 {
	copy := value
	return &copy
}
