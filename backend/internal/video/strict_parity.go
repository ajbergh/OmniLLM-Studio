package video

import (
	"fmt"
	"sort"
	"strings"
)

// StrictParityIssue identifies an authored timeline path whose current editor
// preview and legacy FFmpeg export are known to use different semantics.
type StrictParityIssue struct {
	Path    string `json:"path"`
	Feature string `json:"feature"`
	Detail  string `json:"detail"`
}

func StrictParityIssues(doc TimelineDocument) []StrictParityIssue {
	issues := make([]StrictParityIssue, 0)
	for trackIndex, track := range doc.Tracks {
		for clipIndex, clip := range track.Clips {
			path := fmt.Sprintf("tracks[%d].clips[%d]", trackIndex, clipIndex)
			if clip.Text != nil {
				issues = append(issues, StrictParityIssue{Path: path + ".text", Feature: RendererFeatureText, Detail: "browser and FFmpeg text layout are not exact"})
			}
			if clip.Shape != nil {
				issues = append(issues, StrictParityIssue{Path: path + ".shape", Feature: RendererFeatureAnnotations, Detail: "annotation geometry is partially normalized during export"})
			}
			if clip.Cursor != nil && len(clip.Cursor.Events) > 0 {
				issues = append(issues, StrictParityIssue{Path: path + ".cursor", Feature: RendererFeatureCursor, Detail: "cursor parity is bounded to the proven static-2D raster subset; complex parents and click audio remain partial"})
			}
			for effectIndex, effect := range clip.Effects {
				if effect.Enabled {
					issues = append(issues, StrictParityIssue{Path: fmt.Sprintf("%s.effects[%d]", path, effectIndex), Feature: RendererFeatureEffects, Detail: fmt.Sprintf("effect %q does not share one preview/export implementation", effect.Type)})
				}
			}
			for transitionIndex, transition := range clip.Transitions {
				issues = append(issues, StrictParityIssue{Path: fmt.Sprintf("%s.transitions[%d]", path, transitionIndex), Feature: RendererFeatureTransitions, Detail: fmt.Sprintf("transition %q is absent or approximated in preview/export", transition.Type)})
			}
			for keyframeIndex := range clip.Keyframes {
				issues = append(issues, StrictParityIssue{Path: fmt.Sprintf("%s.keyframes[%d]", path, keyframeIndex), Feature: RendererFeatureKeyframes, Detail: "animation curves are sampled by the legacy exporter"})
			}
			for _, key := range []string{"rotation_x", "rotation_y", "anchor_x", "anchor_y", "perspective", "crop"} {
				if transformValueIsAuthored(clip.Transform[key]) {
					issues = append(issues, StrictParityIssue{Path: path + ".transform." + key, Feature: RendererFeatureSpatial3D, Detail: "transform semantics differ between CSS and FFmpeg"})
				}
			}
		}
	}
	for sceneIndex, scene := range doc.Scenes {
		path := fmt.Sprintf("scenes[%d]", sceneIndex)
		if scene.Camera != nil {
			issues = append(issues, StrictParityIssue{Path: path + ".camera", Feature: RendererFeatureCameraMotion, Detail: "camera projection is sampled and does not include exact X/Y tilt"})
		}
		for effectIndex, effect := range scene.Effects {
			if effect.Enabled {
				issues = append(issues, StrictParityIssue{Path: fmt.Sprintf("%s.effects[%d]", path, effectIndex), Feature: RendererFeatureEffects, Detail: fmt.Sprintf("scene effect %q does not share one preview/export implementation", effect.Type)})
			}
		}
	}
	return issues
}

func transformValueIsAuthored(value any) bool {
	if value == nil {
		return false
	}
	switch typed := value.(type) {
	case float64:
		return typed != 0
	case float32:
		return typed != 0
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case map[string]any:
		for _, nested := range typed {
			if transformValueIsAuthored(nested) {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func strictParityError(issues []StrictParityIssue) error {
	if len(issues) == 0 {
		return nil
	}
	sort.SliceStable(issues, func(i, j int) bool { return issues[i].Path < issues[j].Path })
	parts := make([]string, 0, min(len(issues), 8))
	for index, issue := range issues {
		if index == 8 {
			break
		}
		parts = append(parts, fmt.Sprintf("%s: %s", issue.Path, issue.Detail))
	}
	if remaining := len(issues) - len(parts); remaining > 0 {
		parts = append(parts, fmt.Sprintf("and %d more", remaining))
	}
	return fmt.Errorf("strict parity blocked %d known preview/export mismatch(es): %s", len(issues), strings.Join(parts, "; "))
}
