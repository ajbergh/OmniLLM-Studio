package rendercontract

import (
	"fmt"
	"math"
	"strings"
)

const TextStateContractV1 = "text-state-v1"

const (
	TextFontFamilySourceAuthored           = "authored"
	TextFontFamilySourceCompositionDefault = "composition-default"
	TextLineHeightNormal                   = "normal"
	TextLineHeightMultiplier               = "multiplier"
)

// Font-face provenance for evaluated text. A packaged resource is the only
// deterministic face; a family name alone stays renderer-dependent until a
// packaged face binds it.
const (
	TextFontFaceSourcePackagedResource   = "packaged-resource"
	TextFontFaceSourceFamilyNameOnly     = "family-name-only"
	TextFontFaceSourceCompositionDefault = "composition-default"
)

// EvaluatedTextPadding is text box padding in canvas pixels.
type EvaluatedTextPadding struct {
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
}

// EvaluatedTextShadow makes the current preview shadow semantics explicit so a
// later shared compositor does not need to reinterpret the authored boolean.
type EvaluatedTextShadow struct {
	OffsetX    float64 `json:"offset_x"`
	OffsetY    float64 `json:"offset_y"`
	BlurRadius float64 `json:"blur_radius"`
	Color      string  `json:"color"`
}

// EvaluatedTextState is the renderer-independent static text styling contract.
// Timeline v2 currently defines no text-style keyframe properties, so this
// contract deliberately does not invent text animation semantics. Params stays
// non-authoritative extension metadata and is not projected into this state.
type EvaluatedTextState struct {
	ContractVersion  string               `json:"contract_version"`
	Text             string               `json:"text"`
	FontFamily       string               `json:"font_family"`
	FontFamilySource string               `json:"font_family_source"`
	FontResourceID   string               `json:"font_resource_id,omitempty"`
	FontFaceSource   string               `json:"font_face_source"`
	FontSize         int                  `json:"font_size"`
	FontWeight       string               `json:"font_weight"`
	Color            string               `json:"color"`
	Background       string               `json:"background,omitempty"`
	Stroke           string               `json:"stroke,omitempty"`
	StrokeWidth      float64              `json:"stroke_width"`
	Shadow           *EvaluatedTextShadow `json:"shadow,omitempty"`
	TextAlign        string               `json:"text_align"`
	VerticalAlign    string               `json:"vertical_align"`
	LineHeightMode   string               `json:"line_height_mode"`
	LineHeight       *float64             `json:"line_height,omitempty"`
	LetterSpacing    float64              `json:"letter_spacing"`
	BorderRadius     float64              `json:"border_radius"`
	BoxWidth         *float64             `json:"box_width,omitempty"`
	BoxHeight        *float64             `json:"box_height,omitempty"`
	Padding          EvaluatedTextPadding `json:"padding"`
}

// EvaluateTextState normalizes authored Timeline v2 text against the current
// Video Edit Studio preview semantics. It fails closed for invalid authorable
// values rather than relying on browser or FFmpeg error recovery.
func EvaluateTextState(text *TimelineV2Text, canvasHeight int) (*EvaluatedTextState, error) {
	if text == nil {
		return nil, nil
	}

	state := &EvaluatedTextState{
		ContractVersion:  TextStateContractV1,
		Text:             text.Text,
		FontFamily:       strings.TrimSpace(text.FontFamily),
		FontFamilySource: TextFontFamilySourceCompositionDefault,
		FontSize:         int(math.Round(float64(canvasHeight) / 18)),
		FontWeight:       strings.TrimSpace(text.FontWeight),
		Color:            strings.TrimSpace(text.Color),
		Background:       strings.TrimSpace(text.Background),
		Stroke:           strings.TrimSpace(text.Stroke),
		TextAlign:        strings.ToLower(strings.TrimSpace(text.TextAlign)),
		VerticalAlign:    strings.ToLower(strings.TrimSpace(text.VerticalAlign)),
		LineHeightMode:   TextLineHeightNormal,
	}
	if state.FontFamily != "" {
		state.FontFamilySource = TextFontFamilySourceAuthored
	}
	// Font-face provenance is resolved by the caller when manifest font
	// resources are available; without them a family name alone stays
	// renderer-dependent and never claims a packaged face.
	state.FontFaceSource = TextFontFaceSourceCompositionDefault
	if state.FontFamily != "" {
		state.FontFaceSource = TextFontFaceSourceFamilyNameOnly
	}
	if resourceID := strings.TrimSpace(text.FontResourceID); resourceID != "" {
		if text.FontResourceID != resourceID {
			return nil, fmt.Errorf("canonical text font_resource_id %q must not have surrounding whitespace", text.FontResourceID)
		}
		state.FontResourceID = resourceID
	}
	if text.FontSize != nil {
		if *text.FontSize < 0 {
			return nil, fmt.Errorf("canonical text font_size must be non-negative")
		}
		if *text.FontSize > 0 {
			state.FontSize = *text.FontSize
		}
	}
	if state.FontWeight == "" {
		state.FontWeight = "700"
	}
	if state.Color == "" {
		state.Color = "#ffffff"
	}
	if state.TextAlign == "" {
		state.TextAlign = "center"
	}
	if state.TextAlign != "left" && state.TextAlign != "center" && state.TextAlign != "right" {
		return nil, fmt.Errorf("unsupported canonical text_align %q", state.TextAlign)
	}
	if state.VerticalAlign == "" {
		state.VerticalAlign = "middle"
	}
	if state.VerticalAlign != "top" && state.VerticalAlign != "middle" && state.VerticalAlign != "bottom" {
		return nil, fmt.Errorf("unsupported canonical vertical_align %q", state.VerticalAlign)
	}

	if text.LineHeight != nil {
		if err := requireFiniteNonNegative("line_height", *text.LineHeight); err != nil {
			return nil, err
		}
		if *text.LineHeight > 0 {
			value := *text.LineHeight
			state.LineHeight = &value
			state.LineHeightMode = TextLineHeightMultiplier
		}
	}
	if text.LetterSpacing != nil {
		if err := requireFinite("letter_spacing", *text.LetterSpacing); err != nil {
			return nil, err
		}
		state.LetterSpacing = *text.LetterSpacing
	}
	if text.BorderRadius != nil {
		if err := requireFiniteNonNegative("border_radius", *text.BorderRadius); err != nil {
			return nil, err
		}
		state.BorderRadius = *text.BorderRadius
	}
	if text.BoxWidth != nil {
		if err := requireFinitePositive("box_width", *text.BoxWidth); err != nil {
			return nil, err
		}
		value := *text.BoxWidth
		state.BoxWidth = &value
	}
	if text.BoxHeight != nil {
		if err := requireFinitePositive("box_height", *text.BoxHeight); err != nil {
			return nil, err
		}
		value := *text.BoxHeight
		state.BoxHeight = &value
	}

	if state.Stroke != "" {
		state.StrokeWidth = 2
		if text.StrokeWidth != nil {
			if err := requireFiniteNonNegative("stroke_width", *text.StrokeWidth); err != nil {
				return nil, err
			}
			if *text.StrokeWidth > 0 {
				state.StrokeWidth = *text.StrokeWidth
			}
		}
	} else if text.StrokeWidth != nil {
		if err := requireFiniteNonNegative("stroke_width", *text.StrokeWidth); err != nil {
			return nil, err
		}
	}
	if text.Shadow {
		state.Shadow = &EvaluatedTextShadow{OffsetX: 2, OffsetY: 2, BlurRadius: 4, Color: "rgba(0,0,0,0.7)"}
	}

	baseVertical, baseHorizontal := 0.0, 0.0
	if state.Background != "" {
		baseVertical, baseHorizontal = 8, 18
	}
	state.Padding = EvaluatedTextPadding{Top: baseVertical, Right: baseHorizontal, Bottom: baseVertical, Left: baseHorizontal}
	padding := []struct {
		name     string
		authored *float64
		target   *float64
	}{
		{"padding_top", text.PaddingTop, &state.Padding.Top},
		{"padding_right", text.PaddingRight, &state.Padding.Right},
		{"padding_bottom", text.PaddingBottom, &state.Padding.Bottom},
		{"padding_left", text.PaddingLeft, &state.Padding.Left},
	}
	for _, entry := range padding {
		if entry.authored == nil {
			continue
		}
		if err := requireFiniteNonNegative(entry.name, *entry.authored); err != nil {
			return nil, err
		}
		*entry.target = *entry.authored
	}
	return state, nil
}

func requireFinite(name string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("canonical text %s must be finite", name)
	}
	return nil
}

func requireFiniteNonNegative(name string, value float64) error {
	if err := requireFinite(name, value); err != nil {
		return err
	}
	if value < 0 {
		return fmt.Errorf("canonical text %s must be non-negative", name)
	}
	return nil
}

func requireFinitePositive(name string, value float64) error {
	if err := requireFinite(name, value); err != nil {
		return err
	}
	if value <= 0 {
		return fmt.Errorf("canonical text %s must be positive", name)
	}
	return nil
}
