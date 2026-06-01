// Package api provides HTTP handlers and routing for the OmniLLM-Studio backend.
// This file contains handlers for cross-provider functionalities.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// llmCompleter is a narrow interface so CrossoverHandler is mockable in tests
// without depending on all of llm.Service.
type llmCompleter interface {
	ChatComplete(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

// CrossoverHandler handles cross-studio domain translation requests.
type CrossoverHandler struct {
	llm          llmCompleter
	providerRepo *repository.ProviderRepo
}

// NewCrossoverHandler creates a CrossoverHandler.
func NewCrossoverHandler(llm llmCompleter, providerRepo *repository.ProviderRepo) *CrossoverHandler {
	return &CrossoverHandler{llm: llm, providerRepo: providerRepo}
}

// translateRequest is the shape of POST /v1/crossover/translate.
type translateRequest struct {
	Source  string              `json:"source"` // "music" | "image" | "chat" | "video"
	Target  string              `json:"target"` // "music" | "image" | "video"
	Content translateContentReq `json:"content"`
}

type translateContentReq struct {
	Prompt      string   `json:"prompt"`
	Genre       string   `json:"genre,omitempty"`
	Mood        string   `json:"mood,omitempty"`
	Instruments []string `json:"instruments,omitempty"`
}

// Translate handles POST /v1/crossover/translate.
func (h *CrossoverHandler) Translate(w http.ResponseWriter, r *http.Request) {
	var req translateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondErrorWithCode(w, http.StatusBadRequest, "invalid_payload", "invalid request body", nil)
		return
	}

	// Validate source/target pair
	validSources := map[string]bool{"music": true, "image": true, "chat": true, "video": true}
	validTargets := map[string]bool{"music": true, "image": true, "video": true}
	if !validSources[req.Source] || !validTargets[req.Target] {
		respondErrorWithCode(w, http.StatusBadRequest, "invalid_payload", "source must be 'music', 'image', 'chat', or 'video'; target must be 'music', 'image', or 'video'", nil)
		return
	}
	if req.Source == req.Target {
		respondErrorWithCode(w, http.StatusBadRequest, "invalid_payload", "source and target must be different", nil)
		return
	}

	// Guard: short prompt is not useful for translation
	if len(strings.TrimSpace(req.Content.Prompt)) < 10 {
		respondErrorWithCode(w, http.StatusBadRequest, "invalid_payload", "prompt too short for translation (minimum 10 characters)", nil)
		return
	}

	// Resolve provider — pick first enabled provider
	provider, err := h.firstEnabledProvider(auth.UserIDFromContext(r.Context()))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if provider == "" {
		respondErrorWithCode(w, http.StatusPreconditionFailed, "no_provider_configured", "no enabled LLM provider found; add a provider in Settings first", nil)
		return
	}

	// Build prompt
	systemPrompt, userPrompt := buildTranslationPrompt(req)

	chatReq := llm.ChatRequest{
		Provider: provider,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	resp, err := h.llm.ChatComplete(r.Context(), chatReq)
	if err != nil {
		respondErrorWithCode(w, http.StatusBadGateway, "translation_failed", fmt.Sprintf("LLM call failed: %v", err), nil)
		return
	}

	// Parse JSON from response
	raw := strings.TrimSpace(resp.Content)
	// Strip markdown code fences if present
	raw = stripJSONFences(raw)

	if raw == "" {
		respondErrorWithCode(w, http.StatusBadGateway, "translation_failed", "LLM returned empty response", nil)
		return
	}

	// Validate it's parseable JSON and return it directly
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		respondErrorWithCode(w, http.StatusBadGateway, "translation_failed", "LLM did not return valid JSON", nil)
		return
	}

	respondJSON(w, http.StatusOK, parsed)
}

// firstEnabledProvider returns the provider name/id of the first enabled provider profile.
// In solo mode userID is empty; we still pick the first enabled provider.
func (h *CrossoverHandler) firstEnabledProvider(_ string) (string, error) {
	providers, err := h.providerRepo.List()
	if err != nil {
		return "", fmt.Errorf("list providers: %w", err)
	}
	for _, p := range providers {
		if p.Enabled {
			// Use the provider name as the routing key (matches llm.Service.resolveProvider logic)
			return p.Name, nil
		}
	}
	return "", nil
}

// buildTranslationPrompt constructs the system + user prompts for domain translation.
func buildTranslationPrompt(req translateRequest) (system, user string) {
	switch {
	case req.Source == "music" && req.Target == "image":
		system = `You are an album art director with expertise in translating music into striking square cover art concepts.
Your task: given a music prompt with optional genre, mood, and instruments, output a single JSON object with exactly this schema:
{"image_prompt": "<string>"}
The image_prompt must describe a square album cover (1:1 aspect ratio). Write a vivid, art-directed visual prompt (80-150 words) in the style of professional album artwork. Emphasize: a single bold focal image or abstract motif centered for square crop; a strong, cohesive color palette of 2-4 colors; a defined artistic style (e.g. digital illustration, painterly, graphic design, photography with heavy post-processing, glitch art, retro poster); mood and atmosphere that mirror the sonic identity of the music. Avoid cluttered scenes — album covers are graphic and immediate. Do NOT mention text, typography, or album titles. Do NOT include explanations or markdown — output ONLY the JSON object.`
		parts := []string{fmt.Sprintf("Music prompt: %s", req.Content.Prompt)}
		if req.Content.Genre != "" {
			parts = append(parts, fmt.Sprintf("Genre: %s", req.Content.Genre))
		}
		if req.Content.Mood != "" {
			parts = append(parts, fmt.Sprintf("Mood: %s", req.Content.Mood))
		}
		if len(req.Content.Instruments) > 0 {
			parts = append(parts, fmt.Sprintf("Instruments: %s", strings.Join(req.Content.Instruments, ", ")))
		}
		user = strings.Join(parts, "\n")

	case req.Source == "image" && req.Target == "music":
		system = `You are a music director specializing in translating visual concepts into music production briefs.
Your task: given an image prompt or visual description, output a single JSON object with exactly this schema:
{"prompt": "<string>", "genre": "<string>", "mood": "<string>", "instruments": ["<string>", ...], "tempo": "<string>"}
The prompt should be a concise music prompt (50-100 words). genre should be a specific music genre. mood should be one word or short phrase. instruments should be an array of 3-6 instrument names. tempo should be one of: slow, moderate, upbeat, fast. Do NOT include explanations or markdown — output ONLY the JSON object.`
		user = fmt.Sprintf("Image/visual description: %s", req.Content.Prompt)

	case req.Source == "chat" && req.Target == "music":
		system = `You are a Lyria music prompt specialist. Extract and distill only the musical description from the following text — which may be a full LLM chat response containing conversational preamble, explanations, and postscript.
Remove ALL of the following: greetings, commentary, meta-commentary ("here's a prompt", "I'd suggest", "this captures..."), markdown headers, and any non-musical sentences.
Return a single JSON object with exactly this schema:
{"prompt": "<string>", "genre": "<string>", "mood": "<string>", "instruments": ["<string>", ...], "tempo": "<string>"}
The prompt field must be a clean, self-contained Lyria music generation prompt (30-100 words) describing only sonic/musical qualities. genre should be a specific music genre. mood should be one word or short phrase. instruments should be an array of 3-6 instrument names inferred from the text. tempo should be one of: slow, moderate, upbeat, fast. If no clear musical content is found, infer a reasonable interpretation. Do NOT include explanations or markdown — output ONLY the JSON object.`
		user = fmt.Sprintf("Chat response to distill:\n%s", req.Content.Prompt)

	case req.Source == "image" && req.Target == "video":
		system = `You are a video director translating still-image concepts into image-to-video production briefs.
Output a single JSON object with exactly this schema:
{"video_prompt":"<string>","shot_type":"<string>","camera_motion":"<string>","duration_seconds":6,"aspect_ratio":"16:9"}
The video_prompt should describe how the still image comes alive through subject motion, environmental motion, camera movement, lighting continuity, and cinematic style. Keep it 70-140 words. Do NOT include explanations or markdown — output ONLY the JSON object.`
		user = fmt.Sprintf("Image/visual description: %s", req.Content.Prompt)

	case req.Source == "music" && req.Target == "video":
		system = `You are a music video creative director translating music prompts into video concepts.
Output a single JSON object with exactly this schema:
{"video_prompt":"<string>","shot_type":"<string>","camera_motion":"<string>","duration_seconds":8,"aspect_ratio":"16:9","timeline_notes":"<string>"}
The video_prompt should describe visuals, movement, pacing, lighting, and edit rhythm that match the music. timeline_notes should explain suggested beat cuts or visualizer treatment. Do NOT include explanations or markdown — output ONLY the JSON object.`
		parts := []string{fmt.Sprintf("Music prompt: %s", req.Content.Prompt)}
		if req.Content.Genre != "" {
			parts = append(parts, fmt.Sprintf("Genre: %s", req.Content.Genre))
		}
		if req.Content.Mood != "" {
			parts = append(parts, fmt.Sprintf("Mood: %s", req.Content.Mood))
		}
		if len(req.Content.Instruments) > 0 {
			parts = append(parts, fmt.Sprintf("Instruments: %s", strings.Join(req.Content.Instruments, ", ")))
		}
		user = strings.Join(parts, "\n")

	case req.Source == "chat" && req.Target == "video":
		system = `You are a storyboard producer turning prose into a concise video project brief.
Output a single JSON object with exactly this schema:
{"title":"<string>","video_prompt":"<string>","storyboard":["<string>","<string>","<string>"],"duration_seconds":30,"aspect_ratio":"16:9"}
The video_prompt should be suitable for AI video generation and timeline planning. Storyboard entries should be short shot descriptions. Do NOT include explanations or markdown — output ONLY the JSON object.`
		user = fmt.Sprintf("Text to turn into a video project:\n%s", req.Content.Prompt)

	case req.Source == "video" && req.Target == "image":
		system = `You are a video-to-image art director.
Output a single JSON object with exactly this schema:
{"image_prompt":"<string>"}
Condense the video concept into one strong keyframe or thumbnail image prompt with clear composition, lighting, subject, and style. Do NOT include explanations or markdown — output ONLY the JSON object.`
		user = fmt.Sprintf("Video concept or timeline description: %s", req.Content.Prompt)

	case req.Source == "video" && req.Target == "music":
		system = `You are a music supervisor translating a video concept into a music generation brief.
Output a single JSON object with exactly this schema:
{"prompt":"<string>","genre":"<string>","mood":"<string>","instruments":["<string>",...],"tempo":"<string>"}
The prompt should describe soundtrack pacing, sonic texture, and arrangement that fit the video. tempo should be one of: slow, moderate, upbeat, fast. Do NOT include explanations or markdown — output ONLY the JSON object.`
		user = fmt.Sprintf("Video concept or timeline description: %s", req.Content.Prompt)

	default:
		// Shouldn't reach here due to earlier validation
		system = "Translate the input to the target domain. Output only JSON."
		user = req.Content.Prompt
	}
	return system, user
}

// stripJSONFences removes ```json ... ``` or ``` ... ``` fences from LLM output.
func stripJSONFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// Remove first line (```json or ```)
		idx := strings.Index(s, "\n")
		if idx != -1 {
			s = s[idx+1:]
		}
		// Remove trailing ```
		if end := strings.LastIndex(s, "```"); end != -1 {
			s = s[:end]
		}
		s = strings.TrimSpace(s)
	}
	return s
}
