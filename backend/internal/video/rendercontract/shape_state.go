package rendercontract

import (
	"fmt"
	"math"
	"strings"
)

const ShapeStateContractV1 = "shape-state-v1"

const (
	ShapeKindRectangle        = "rectangle"
	ShapeKindHighlight        = "highlight"
	ShapeKindBlur             = "blur"
	ShapeKindRoundedRectangle = "rounded_rectangle"
	ShapeKindEllipse          = "ellipse"
	ShapeKindArrow            = "arrow"
	ShapeKindLine             = "line"
	ShapeKindSpeechBubble     = "speech_bubble"
	ShapeKindSpotlight        = "spotlight"
	ShapeKindPixelate         = "pixelate"
	ShapeKindCheckmark        = "checkmark"
	ShapeKindXMark            = "x_mark"
	ShapeKindStepMarker       = "step_marker"
	ShapeKindLabel            = "label"
)

var canonicalShapeKinds = map[string]bool{
	ShapeKindRectangle: true, ShapeKindHighlight: true, ShapeKindBlur: true,
	ShapeKindRoundedRectangle: true, ShapeKindEllipse: true, ShapeKindArrow: true,
	ShapeKindLine: true, ShapeKindSpeechBubble: true, ShapeKindSpotlight: true,
	ShapeKindPixelate: true, ShapeKindCheckmark: true, ShapeKindXMark: true,
	ShapeKindStepMarker: true, ShapeKindLabel: true,
}

// EvaluatedShapeState is renderer-independent static annotation state. Shape
// dimensions and style are expressed in canvas pixels. An empty Stroke means
// that the kind has no default stroke; an authored stroke still overrides it.
// Text nested in callout shapes remains the separate text-state-v1 projection.
type EvaluatedShapeState struct {
	ContractVersion string  `json:"contract_version"`
	Kind            string  `json:"kind"`
	Width           float64 `json:"width"`
	Height          float64 `json:"height"`
	Fill            string  `json:"fill"`
	Stroke          string  `json:"stroke"`
	StrokeWidth     float64 `json:"stroke_width"`
	BlurRadius      float64 `json:"blur_radius"`
	CornerRadius    float64 `json:"corner_radius"`
}

// EvaluateShapeState normalizes authored Timeline v2 shape state against the
// current Video Edit Studio preview semantics. Legacy FFmpeg approximations are
// deliberately not semantic authority for this contract.
func EvaluateShapeState(shape *TimelineV2Shape) (*EvaluatedShapeState, error) {
	if shape == nil {
		return nil, nil
	}
	kind := strings.ToLower(strings.TrimSpace(shape.Kind))
	if !canonicalShapeKinds[kind] {
		return nil, fmt.Errorf("unsupported canonical shape kind %q", kind)
	}

	state := &EvaluatedShapeState{
		ContractVersion: ShapeStateContractV1,
		Kind:            kind,
		Width:           320,
		Height:          180,
		Stroke:          defaultShapeStroke(kind),
		StrokeWidth:     6,
		BlurRadius:      12,
		CornerRadius:    defaultShapeCornerRadius(kind),
	}
	if shape.Width != nil {
		if *shape.Width <= 0 {
			return nil, fmt.Errorf("canonical shape width must be positive")
		}
		state.Width = float64(*shape.Width)
	}
	if shape.Height != nil {
		if *shape.Height <= 0 {
			return nil, fmt.Errorf("canonical shape height must be positive")
		}
		state.Height = float64(*shape.Height)
	}
	if fill := strings.TrimSpace(shape.Fill); fill != "" {
		state.Fill = fill
	} else {
		state.Fill = defaultShapeFill(kind)
	}
	if stroke := strings.TrimSpace(shape.Stroke); stroke != "" {
		state.Stroke = stroke
	}
	if shape.StrokeWidth != nil {
		if err := requireFiniteShapeNonNegative("stroke_width", *shape.StrokeWidth); err != nil {
			return nil, err
		}
		if *shape.StrokeWidth > 0 {
			state.StrokeWidth = math.Max(1, *shape.StrokeWidth)
		}
	}
	if shape.BlurRadius != nil {
		if err := requireFiniteShapeNonNegative("blur_radius", *shape.BlurRadius); err != nil {
			return nil, err
		}
		if *shape.BlurRadius > 0 {
			state.BlurRadius = math.Max(1, *shape.BlurRadius)
		}
	}
	if shape.CornerRadius != nil {
		if err := requireFiniteShapeNonNegative("corner_radius", *shape.CornerRadius); err != nil {
			return nil, err
		}
		if *shape.CornerRadius > 0 {
			state.CornerRadius = *shape.CornerRadius
		}
	}
	return state, nil
}

func defaultShapeFill(kind string) string {
	switch kind {
	case ShapeKindHighlight:
		return "#facc15"
	case ShapeKindSpotlight:
		return "rgba(0,0,0,0.6)"
	case ShapeKindStepMarker:
		return "#2563eb"
	case ShapeKindSpeechBubble:
		return "#ffffff"
	case ShapeKindLabel:
		return "#1e293b"
	default:
		return "transparent"
	}
}

func defaultShapeStroke(kind string) string {
	switch kind {
	case ShapeKindCheckmark:
		return "#22c55e"
	case ShapeKindXMark:
		return "#ef4444"
	case ShapeKindSpeechBubble, ShapeKindLabel:
		return ""
	default:
		return "#f59e0b"
	}
}

func defaultShapeCornerRadius(kind string) float64 {
	switch kind {
	case ShapeKindRoundedRectangle:
		return 12
	case ShapeKindSpeechBubble:
		return 18
	case ShapeKindLabel:
		return 10
	default:
		return 0
	}
}

func requireFiniteShapeNonNegative(name string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("canonical shape %s must be finite", name)
	}
	if value < 0 {
		return fmt.Errorf("canonical shape %s must be non-negative", name)
	}
	return nil
}
