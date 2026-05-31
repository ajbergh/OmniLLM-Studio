package video

import (
	"fmt"
	"strings"
)

func EnhancePrompt(req EnhancePromptRequest) string {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return ""
	}
	aspect := strings.TrimSpace(req.AspectRatio)
	if aspect == "" {
		aspect = DefaultAspectRatio
	}
	duration := req.DurationSeconds
	if duration <= 0 {
		duration = 6
	}
	negative := strings.TrimSpace(req.NegativePrompt)
	if negative == "" {
		negative = "low resolution, jittery motion, warped anatomy, unreadable text, flicker"
	}
	return fmt.Sprintf(`Subject:
%s

Scene:
Describe a cohesive cinematic scene with clear foreground, midground, and background detail.

Action:
Use natural, continuous motion with one primary action and no abrupt cuts.

Camera:
Smooth camera movement, stable framing, purposeful composition, realistic depth of field.

Lighting:
Motivated lighting with balanced contrast and production-ready color.

Style:
Cinematic, polished, high-detail, coherent temporal consistency.

Duration:
%d seconds

Aspect ratio:
%s

Negative prompt:
%s`, prompt, duration, aspect, negative)
}

func DeriveTitle(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return "Untitled Video Project"
	}
	words := strings.Fields(trimmed)
	if len(words) > 8 {
		words = words[:8]
	}
	title := strings.Join(words, " ")
	if len(title) > 80 {
		title = title[:80]
	}
	return title
}
