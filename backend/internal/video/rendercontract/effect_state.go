package rendercontract

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

const (
	EffectStateContractV1 = "effect-state-v1"
	EffectScopeClip       = "clip"
	EffectScopeScene      = "scene"
)

type effectParamSpec struct {
	Default float64
	Min     float64
	Max     float64
}

var canonicalEffectParams = map[string]map[string]effectParamSpec{
	"brightness":      {"amount": {Default: 1.1, Min: 0, Max: 2}},
	"contrast":        {"amount": {Default: 1.2, Min: 0, Max: 3}},
	"saturation":      {"amount": {Default: 1.3, Min: 0, Max: 3}},
	"blur":            {"amount": {Default: 6, Min: 0, Max: 30}},
	"grayscale":       {},
	"sharpen":         {"amount": {Default: 1, Min: 0, Max: 3}},
	"vignette":        {"amount": {Default: 0.4, Min: 0, Max: 1}},
	"shadow":          {},
	"background_blur": {"amount": {Default: 10, Min: 0, Max: 30}},
	"chroma_key": {
		"similarity": {Default: 0.3, Min: 0.01, Max: 1},
		"blend":      {Default: 0.05, Min: 0, Max: 0.5},
	},
	"film_grain":     {"amount": {Default: 8, Min: 0, Max: 40}},
	"bloom":          {"amount": {Default: 0.25, Min: 0, Max: 1}},
	"color_grade":    {"amount": {Default: 1.08, Min: 0.5, Max: 2}},
	"edge_fade":      {"amount": {Default: 0.35, Min: 0, Max: 1}},
	"rgb_split":      {"amount": {Default: 3, Min: 0, Max: 20}},
	"ghost_trail":    {"amount": {Default: 3, Min: 2, Max: 5}},
	"motion_blur":    {"amount": {Default: 0.5, Min: 0, Max: 1}},
	"depth_of_field": {"amount": {Default: 2, Min: 0, Max: 12}},
	"rack_focus":     {"amount": {Default: 2, Min: 0, Max: 12}},
}

// EvaluatedEffectState is one enabled renderer-independent effect operation.
// Order is the effect's authored array index so disabled entries can be omitted
// without changing the relative identity of the remaining stack.
type EvaluatedEffectState struct {
	ContractVersion string   `json:"contract_version"`
	ID              string   `json:"id"`
	Type            string   `json:"type"`
	Scope           string   `json:"scope"`
	Order           int      `json:"order"`
	Params          Metadata `json:"params"`
}

// EvaluateClipEffectStackAtFrame returns enabled clip effects in authored order.
// effect.<id>.amount automation wins over the legacy effect.<type>.amount alias,
// and both are sampled at the exact output-frame presentation time.
func EvaluateClipEffectStackAtFrame(clip TimelineV2Clip, frameIndex int64, fps int) ([]EvaluatedEffectState, error) {
	time := FrameRelativeMilliseconds(frameIndex, fps, clip.StartMS)
	return evaluateEffectStack(clip.Effects, clip.Keyframes, &time, EffectScopeClip)
}

// EvaluateSceneEffectStack returns enabled scene effects in authored order.
// Timeline v2 currently has no scene-effect keyframe collection, so scene
// effects are intentionally static until the schema defines automation.
func EvaluateSceneEffectStack(scene *TimelineV2Scene) ([]EvaluatedEffectState, error) {
	if scene == nil {
		return []EvaluatedEffectState{}, nil
	}
	return evaluateEffectStack(scene.Effects, nil, nil, EffectScopeScene)
}

func evaluateEffectStack(effects []TimelineV2Effect, keyframes []TimelineV2Keyframe, time *RationalMilliseconds, scope string) ([]EvaluatedEffectState, error) {
	out := make([]EvaluatedEffectState, 0, len(effects))
	propertyKeyframes := timelineV2PropertyKeyframes(keyframes)
	for order, effect := range effects {
		if !effect.Enabled {
			continue
		}
		id := strings.TrimSpace(effect.ID)
		typeName := strings.ToLower(strings.TrimSpace(effect.Type))
		specs, ok := canonicalEffectParams[typeName]
		if !ok {
			return nil, fmt.Errorf("unsupported canonical effect type %q", typeName)
		}
		if id == "" {
			return nil, fmt.Errorf("canonical %s effect at order %d has empty id", scope, order)
		}
		params, err := canonicalizeEffectParams(typeName, effect.Params, specs)
		if err != nil {
			return nil, fmt.Errorf("canonical %s effect %q: %w", scope, id, err)
		}
		if time != nil {
			amountSpec, supportsAmount := specs["amount"]
			idProperty := "effect." + strings.ToLower(id) + ".amount"
			typeProperty := "effect." + typeName + ".amount"
			amount, animated := SamplePropertyKeyframesAtRationalMS(propertyKeyframes, idProperty, *time)
			if !animated {
				amount, animated = SamplePropertyKeyframesAtRationalMS(propertyKeyframes, typeProperty, *time)
			}
			if animated {
				if !supportsAmount {
					return nil, fmt.Errorf("effect type %q does not define canonical amount automation", typeName)
				}
				if math.IsNaN(amount) || math.IsInf(amount, 0) {
					return nil, fmt.Errorf("animated amount must be finite")
				}
				params["amount"] = clampFloat64(amount, amountSpec.Min, amountSpec.Max)
			}
		}
		out = append(out, EvaluatedEffectState{
			ContractVersion: EffectStateContractV1,
			ID:              id,
			Type:            typeName,
			Scope:           scope,
			Order:           order,
			Params:          params,
		})
	}
	return out, nil
}

func canonicalizeEffectParams(typeName string, authored Metadata, specs map[string]effectParamSpec) (Metadata, error) {
	params := Metadata{}
	for key, spec := range specs {
		params[key] = spec.Default
	}
	if typeName == "chroma_key" {
		params["color"] = "#00FF00"
	}
	for rawKey, raw := range authored {
		key := strings.ToLower(strings.TrimSpace(rawKey))
		if key == "color" && typeName == "chroma_key" {
			color, ok := raw.(string)
			if !ok || strings.TrimSpace(color) == "" {
				return nil, fmt.Errorf("parameter %q must be a non-empty string", rawKey)
			}
			params["color"] = strings.TrimSpace(color)
			continue
		}
		spec, ok := specs[key]
		if !ok {
			return nil, fmt.Errorf("unsupported parameter %q for effect type %q", rawKey, typeName)
		}
		value, ok := effectNumericValue(raw)
		if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("parameter %q must be a finite number", rawKey)
		}
		params[key] = clampFloat64(value, spec.Min, spec.Max)
	}
	return params, nil
}

func effectNumericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}
