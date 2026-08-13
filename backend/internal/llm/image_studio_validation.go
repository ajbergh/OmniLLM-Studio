package llm

import "fmt"

func validateImageStudioRequestCapabilities(caps ImageCapabilities, req ImageStudioRequest) error {
	n := req.N
	if n <= 0 {
		n = 1
	}
	if caps.MaxVariants > 0 && n > caps.MaxVariants {
		return fmt.Errorf("selected provider/model supports at most %d image variant(s)", caps.MaxVariants)
	}

	contentRefs := len(req.ReferenceImages)
	if req.OperationType != "edit" && req.ReferenceImage != nil {
		contentRefs++
	}
	styleRefs := len(req.StyleReferenceImages)

	if contentRefs > 0 && !caps.SupportsContentReference {
		return fmt.Errorf("selected provider/model does not support content reference images")
	}
	if styleRefs > 0 && !caps.SupportsStyleReference {
		return fmt.Errorf("selected provider/model does not support style reference images")
	}
	if total := contentRefs + styleRefs; total > 0 {
		if caps.MaxReferenceImages <= 0 {
			return fmt.Errorf("selected provider/model does not accept reference images")
		}
		if total > caps.MaxReferenceImages {
			return fmt.Errorf("selected provider/model accepts at most %d reference image(s), got %d", caps.MaxReferenceImages, total)
		}
	}

	if req.Seed != nil && !caps.SupportsSeed {
		return fmt.Errorf("selected provider/model does not support seed control")
	}
	if req.Guidance != nil && !caps.SupportsGuidance {
		return fmt.Errorf("selected provider/model does not support guidance control")
	}
	return nil
}
