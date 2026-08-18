package video

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/ajbergh/omnillm-studio/internal/video/rendercontract"
)

const (
	canonicalAdapterVersionCode              = "V1_VERSION_UNSUPPORTED"
	canonicalAdapterTransitionCode           = "V1_TRANSITION_PLACEMENT_AMBIGUOUS"
	canonicalAdapterUnsupportedTransformCode = "V1_TRANSFORM_FIELD_UNSUPPORTED"
	canonicalAdapterTransformValueCode       = "V1_TRANSFORM_VALUE_INVALID"
)

// CanonicalAdapterError is a fail-closed, path-addressed incompatibility found
// while translating a validated v1 timeline into Timeline v2.
type CanonicalAdapterError struct {
	Code        string
	Path        string
	Message     string
	Remediation string
}

func (e *CanonicalAdapterError) Error() string {
	if e == nil {
		return ""
	}
	if e.Path == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// AdaptTimelineV1ToV2 translates the current editor-compatible v1 document to
// Timeline v2 without mutating the caller. Validation happens on a JSON deep
// copy so v1 defaults are the same defaults persistence and preview consume.
func AdaptTimelineV1ToV2(doc TimelineDocument) (rendercontract.TimelineV2Document, error) {
	if doc.Version > CurrentTimelineVersion {
		return rendercontract.TimelineV2Document{}, &CanonicalAdapterError{
			Code:        canonicalAdapterVersionCode,
			Path:        "version",
			Message:     fmt.Sprintf("timeline version %d cannot be adapted by the v1 adapter", doc.Version),
			Remediation: "use a contract adapter for the authored timeline version",
		}
	}

	validated, err := validatedTimelineCopy(doc)
	if err != nil {
		return rendercontract.TimelineV2Document{}, err
	}
	if err := validateV1CanonicalCompatibility(validated); err != nil {
		return rendercontract.TimelineV2Document{}, err
	}

	data, err := json.Marshal(validated)
	if err != nil {
		return rendercontract.TimelineV2Document{}, fmt.Errorf("marshal validated v1 timeline: %w", err)
	}
	var out rendercontract.TimelineV2Document
	if err := json.Unmarshal(data, &out); err != nil {
		return rendercontract.TimelineV2Document{}, fmt.Errorf("project v1 timeline into Timeline v2: %w", err)
	}
	out.Version = rendercontract.TimelineV2Version
	out.WorkingColorSpace = rendercontract.RenderWorkingColorSpaceSRGB

	for trackIndex := range validated.Tracks {
		for clipIndex := range validated.Tracks[trackIndex].Clips {
			clip := validated.Tracks[trackIndex].Clips[clipIndex]
			if clip.AssetID != "" && validated.Tracks[trackIndex].Type != TrackTypeAudio && validated.Tracks[trackIndex].Type != TrackTypeMusic && !clip.AudioOnly {
				out.Tracks[trackIndex].Clips[clipIndex].MediaFit = "contain"
			}
		}
	}
	for sceneIndex := range validated.Scenes {
		camera := validated.Scenes[sceneIndex].Camera
		if camera == nil {
			continue
		}
		outCamera := out.Scenes[sceneIndex].Camera
		if outCamera == nil {
			outCamera = &rendercontract.TimelineV2Camera{}
			out.Scenes[sceneIndex].Camera = outCamera
		}
		outCamera.X = canonicalFloatPointer(camera.X)
		outCamera.Y = canonicalFloatPointer(camera.Y)
		outCamera.Z = canonicalFloatPointer(camera.Z)
		outCamera.RotationX = canonicalFloatPointer(camera.RotationX)
		outCamera.RotationY = canonicalFloatPointer(camera.RotationY)
		outCamera.RotationZ = canonicalFloatPointer(camera.RotationZ)
		outCamera.FieldOfView = canonicalFloatPointer(camera.FieldOfView)
		outCamera.FocusDepth = canonicalFloatPointer(camera.FocusDepth)
	}
	return out, nil
}

func validatedTimelineCopy(doc TimelineDocument) (TimelineDocument, error) {
	data, err := json.Marshal(doc)
	if err != nil {
		return TimelineDocument{}, fmt.Errorf("copy v1 timeline: %w", err)
	}
	var copied TimelineDocument
	if err := json.Unmarshal(data, &copied); err != nil {
		return TimelineDocument{}, fmt.Errorf("copy v1 timeline: %w", err)
	}
	return ValidateTimelineDocument(copied)
}

func validateV1CanonicalCompatibility(doc TimelineDocument) error {
	for trackIndex, track := range doc.Tracks {
		for clipIndex, clip := range track.Clips {
			path := fmt.Sprintf("tracks[%d].clips[%d]", trackIndex, clipIndex)
			if len(clip.Transitions) > 0 {
				return &CanonicalAdapterError{
					Code:        canonicalAdapterTransitionCode,
					Path:        path + ".transitions[0]",
					Message:     "v1 transition placement is not explicit enough for Timeline v2",
					Remediation: "remove the transition or migrate it after canonical transition placement and peer semantics are defined",
				}
			}
			if err := validateV1Transform(clip.Transform, path+".transform"); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateV1Transform(transform map[string]any, path string) error {
	for key, raw := range transform {
		switch key {
		case "x", "y", "z", "scale", "scale_x", "scale_y", "rotation", "rotation_x", "rotation_y", "rotation_z", "opacity", "anchor_x", "anchor_y", "perspective":
			if err := validateCanonicalNumber(raw, path+"."+key); err != nil {
				return err
			}
		case "crop":
			crop, ok := raw.(map[string]any)
			if !ok {
				return &CanonicalAdapterError{Code: canonicalAdapterTransformValueCode, Path: path + ".crop", Message: "v1 crop must be an object", Remediation: "replace crop with top/right/bottom/left numeric fields"}
			}
			for side, value := range crop {
				if side != "top" && side != "right" && side != "bottom" && side != "left" {
					return unsupportedV1Transform(path+".crop."+side, side)
				}
				if err := validateCanonicalNumber(value, path+".crop."+side); err != nil {
					return err
				}
			}
		default:
			return unsupportedV1Transform(path+"."+key, key)
		}
	}
	return nil
}

func validateCanonicalNumber(raw any, path string) error {
	var value float64
	switch typed := raw.(type) {
	case float64:
		value = typed
	case float32:
		value = float64(typed)
	case int:
		value = float64(typed)
	case int8:
		value = float64(typed)
	case int16:
		value = float64(typed)
	case int32:
		value = float64(typed)
	case int64:
		value = float64(typed)
	case uint:
		value = float64(typed)
	case uint8:
		value = float64(typed)
	case uint16:
		value = float64(typed)
	case uint32:
		value = float64(typed)
	case uint64:
		value = float64(typed)
	default:
		return invalidV1TransformValue(path)
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return invalidV1TransformValue(path)
	}
	return nil
}

func invalidV1TransformValue(path string) *CanonicalAdapterError {
	return &CanonicalAdapterError{
		Code: canonicalAdapterTransformValueCode, Path: path,
		Message: "v1 transform value must be a finite number", Remediation: "replace the value with a finite numeric transform parameter",
	}
}

func unsupportedV1Transform(path, key string) *CanonicalAdapterError {
	return &CanonicalAdapterError{
		Code: canonicalAdapterUnsupportedTransformCode, Path: path,
		Message:     fmt.Sprintf("v1 transform field %q has no canonical Timeline v2 semantics", key),
		Remediation: "remove the unsupported field or define its Timeline v2 semantics before rendering",
	}
}

func canonicalFloatPointer(value float64) *float64 {
	copied := value
	return &copied
}
