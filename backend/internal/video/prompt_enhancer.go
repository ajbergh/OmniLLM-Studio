package video

import (
	"context"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/llm"
)

// EnhancePromptWithLLM uses an LLM to rewrite req.Prompt into a structured,
// Veo-optimised prompt.  Falls back to the deterministic EnhancePrompt if the
// LLM call fails or returns an empty result.
func EnhancePromptWithLLM(ctx context.Context, svc llmCompleter, req EnhancePromptRequest) string {
	if svc == nil {
		return EnhancePrompt(req)
	}
	systemPrompt := `You are a creative director specialising in AI video generation with Google Veo 3.1.
Your task is to rewrite a user's rough idea into a polished, detailed, production-quality video prompt.

Guidelines:
- Use vivid, concrete visual language (colours, lighting, textures, motion).
- Specify camera work: angle, movement, focal length, depth of field.
- Describe temporal arc: beginning, middle, and end of the clip.
- Keep the prompt under 900 words.
- Do NOT include instructions, explanations, or markdown formatting — output the prompt text only.`

	inputMode := "text-to-video"
	if req.InputMode != "" {
		inputMode = req.InputMode
	}
	userMsg := fmt.Sprintf("Rewrite the following %s prompt for Google Veo 3.1:\n\n%s", inputMode, strings.TrimSpace(req.Prompt))
	if req.NegativePrompt != "" {
		userMsg += "\n\nThings to avoid: " + req.NegativePrompt
	}
	if req.AspectRatio != "" {
		userMsg += "\n\nAspect ratio: " + req.AspectRatio
	}
	if req.DurationSeconds > 0 {
		userMsg += fmt.Sprintf("\nTarget duration: %d seconds", req.DurationSeconds)
	}
	if req.ProductionNotes != "" {
		userMsg += "\n\nCinematic details to incorporate:\n" + req.ProductionNotes
	}

	maxTok := 1024
	temp := 0.7
	resp, err := svc.ChatComplete(ctx, llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMsg},
		},
		MaxTokens:   &maxTok,
		Temperature: &temp,
	})
	if err != nil || resp == nil || strings.TrimSpace(resp.Content) == "" {
		return EnhancePrompt(req)
	}
	return strings.TrimSpace(resp.Content)
}

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
