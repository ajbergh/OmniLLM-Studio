package video

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

type AssistantRequest struct {
	Prompt      string           `json:"prompt,omitempty"`
	Instruction string           `json:"instruction,omitempty"`
	Timeline    TimelineDocument `json:"timeline,omitempty"`
	// SelectedClipID / PlayheadMS describe the editor state so plans can target
	// what the user is actually looking at.
	SelectedClipID string `json:"selected_clip_id,omitempty"`
	PlayheadMS     int64  `json:"playhead_ms,omitempty"`
}

type StoryboardScene struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	DurationMS  int64  `json:"duration_ms"`
	Prompt      string `json:"prompt"`
}

type StoryboardResponse struct {
	Title      string            `json:"title"`
	Scenes     []StoryboardScene `json:"scenes"`
	ShotList   []string          `json:"shot_list"`
	Script     string            `json:"script"`
	PromptSeed string            `json:"prompt_seed"`
}

type EditPlan struct {
	Summary    string          `json:"summary"`
	Operations []EditOperation `json:"operations"`
	// Preview holds one human-readable line per valid operation so the user can
	// review exactly what will change before applying.
	Preview []string `json:"preview,omitempty"`
	// Issues lists operations that failed validation against the current
	// timeline; these are skipped when the plan is applied.
	Issues []string `json:"issues,omitempty"`
}

type EditOperation struct {
	Type       string `json:"type"`
	ClipID     string `json:"clip_id,omitempty"`
	TrackID    string `json:"track_id,omitempty"`
	StartMS    int64  `json:"start_ms,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	Text       string `json:"text,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	FPS        int    `json:"fps,omitempty"`
}

type SocialVariant struct {
	Name        string   `json:"name"`
	AspectRatio string   `json:"aspect_ratio"`
	Width       int      `json:"width"`
	Height      int      `json:"height"`
	Plan        EditPlan `json:"plan"`
}

func (s *Service) CreateStoryboard(ctx context.Context, userID, projectID string, req AssistantRequest) (*StoryboardResponse, error) {
	if _, err := s.ensureProjectOwned(userID, projectID); err != nil {
		return nil, err
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = strings.TrimSpace(req.Instruction)
	}
	if prompt == "" {
		prompt = "A concise product story with an opening hook, demonstration, and closing title card."
	}

	// Try LLM path first when a provider is available.
	if s.llm != nil {
		if provider := s.firstEnabledChatProvider(); provider != "" {
			result, err := s.llmStoryboard(ctx, provider, prompt)
			if err == nil {
				return result, nil
			}
			// Fall through to deterministic on error.
		}
	}

	// Deterministic fallback.
	title := DeriveTitle(prompt)
	scenes := []StoryboardScene{
		{
			ID:          "scene-" + uuid.New().String(),
			Title:       "Opening Hook",
			Description: "Establish the subject and visual promise immediately.",
			DurationMS:  6000,
			Prompt:      cinematicPrompt(prompt, "opening wide shot with a clean hook"),
		},
		{
			ID:          "scene-" + uuid.New().String(),
			Title:       "Core Moment",
			Description: "Show the primary action with clear motion and continuity.",
			DurationMS:  14000,
			Prompt:      cinematicPrompt(prompt, "medium tracking shot focused on the main action"),
		},
		{
			ID:          "scene-" + uuid.New().String(),
			Title:       "Close and Title",
			Description: "Resolve the beat with space for a title or callout overlay.",
			DurationMS:  6000,
			Prompt:      cinematicPrompt(prompt, "final composed shot with room for text overlay"),
		},
	}
	return &StoryboardResponse{
		Title:  title,
		Scenes: scenes,
		ShotList: []string{
			"Wide establishing shot",
			"Medium action shot",
			"Close detail or title-card hold",
		},
		Script:     fmt.Sprintf("%s. Open with the subject clearly framed, show the main action, then end on a concise title card.", title),
		PromptSeed: prompt,
	}, nil
}

// llmStoryboard calls the LLM to generate a structured storyboard.
func (s *Service) llmStoryboard(ctx context.Context, provider, prompt string) (*StoryboardResponse, error) {
	system := `You are a professional video director and storyboard artist.
Given a video concept, produce a detailed 3–5 scene storyboard in JSON.
Output ONLY a JSON object matching this schema (no markdown, no explanation):
{
  "title": "<short title for the video>",
  "prompt_seed": "<the original concept>",
  "script": "<one paragraph describing the full narrative arc>",
  "shot_list": ["<shot description 1>", "<shot description 2>", "..."],
  "scenes": [
    {
      "id": "<unique-id>",
      "title": "<scene title>",
      "description": "<director's note: what happens in this scene>",
      "duration_ms": <milliseconds as integer, e.g. 6000>,
      "prompt": "<ready-to-use text-to-video generation prompt for this scene>"
    }
  ]
}
Make each scene prompt cinematic and self-contained (25–60 words). Durations should add up to a coherent short video (15–60 seconds total).`

	resp, err := s.llm.ChatComplete(ctx, llm.ChatRequest{
		Provider: provider,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: "Video concept: " + prompt},
		},
	})
	if err != nil {
		return nil, err
	}
	// Strip potential markdown fences.
	raw := strings.TrimSpace(resp.Content)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var result StoryboardResponse
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("parse storyboard JSON: %w (raw: %.200s)", err, raw)
	}
	// Ensure scene IDs are set.
	for i := range result.Scenes {
		if result.Scenes[i].ID == "" {
			result.Scenes[i].ID = "scene-" + uuid.New().String()
		}
	}
	return &result, nil
}

func (s *Service) CreateTimelinePlan(ctx context.Context, userID, projectID string, req AssistantRequest) (*EditPlan, error) {
	_, doc, err := s.GetOrCreateTimeline(ctx, userID, projectID)
	if err != nil {
		return nil, err
	}
	doc = preferClientTimeline(doc, req)
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = "Untitled video"
	}
	plan := &EditPlan{
		Summary: "Create a simple timeline scaffold with a title card and a 30 second project duration.",
		Operations: []EditOperation{
			{Type: "set_duration", DurationMS: 30000},
			{Type: "add_text_clip", TrackID: "track-text-1", StartMS: 0, DurationMS: 3500, Text: DeriveTitle(prompt)},
		},
	}
	annotatePlan(doc, plan)
	return plan, nil
}

// preferClientTimeline uses the timeline the client sent (which may contain
// unsaved edits) when it has content, falling back to the persisted document.
func preferClientTimeline(persisted TimelineDocument, req AssistantRequest) TimelineDocument {
	if len(req.Timeline.Tracks) == 0 {
		return persisted
	}
	validated, err := ValidateTimelineDocument(req.Timeline)
	if err != nil {
		return persisted
	}
	return validated
}

func (s *Service) CreateEditPlan(ctx context.Context, userID, projectID string, req AssistantRequest) (*EditPlan, error) {
	_, doc, err := s.GetOrCreateTimeline(ctx, userID, projectID)
	if err != nil {
		return nil, err
	}
	doc = preferClientTimeline(doc, req)
	assetList, _ := s.assets.ListByProject(projectID)
	timelineContext := timelineContextSummary(doc, assetList, req.SelectedClipID, req.PlayheadMS)

	// Try LLM path first when a provider is available.
	if s.llm != nil {
		if provider := s.firstEnabledChatProvider(); provider != "" {
			result, err := s.llmEditPlan(ctx, provider, req, timelineContext)
			if err == nil {
				annotatePlan(doc, result)
				return result, nil
			}
			// Fall through to deterministic on error.
		}
	}

	// Deterministic fallback.
	instruction := strings.ToLower(strings.TrimSpace(req.Instruction + " " + req.Prompt))
	if instruction == "" {
		instruction = "tighten timeline"
	}
	plan := EditPlan{Summary: "Validated deterministic edit plan", Operations: []EditOperation{}}
	if strings.Contains(instruction, "vertical") || strings.Contains(instruction, "9:16") {
		plan.Operations = append(plan.Operations, EditOperation{Type: "set_canvas", Width: 1080, Height: 1920, FPS: DefaultProjectFPS})
	}
	if strings.Contains(instruction, "square") || strings.Contains(instruction, "1:1") {
		plan.Operations = append(plan.Operations, EditOperation{Type: "set_canvas", Width: 1080, Height: 1080, FPS: DefaultProjectFPS})
	}
	if strings.Contains(instruction, "15") || strings.Contains(instruction, "fifteen") || strings.Contains(instruction, "teaser") {
		plan.Operations = append(plan.Operations, EditOperation{Type: "set_duration", DurationMS: 15000})
	} else if strings.Contains(instruction, "30") || strings.Contains(instruction, "thirty") || strings.Contains(instruction, "social cut") {
		plan.Operations = append(plan.Operations, EditOperation{Type: "set_duration", DurationMS: 30000})
	}
	if strings.Contains(instruction, "title") || strings.Contains(instruction, "intro") {
		plan.Operations = append(plan.Operations, EditOperation{Type: "add_text_clip", TrackID: "track-text-1", StartMS: 0, DurationMS: 3000, Text: "Opening Title"})
	}
	if strings.Contains(instruction, "lower third") || strings.Contains(instruction, "lower-third") {
		plan.Operations = append(plan.Operations, EditOperation{Type: "add_text_clip", TrackID: "track-caption-1", StartMS: maxInt64(0, req.PlayheadMS), DurationMS: 4000, Text: DeriveTitle(strings.TrimSpace(req.Prompt + " " + req.Instruction))})
	} else if strings.Contains(instruction, "caption") {
		captionText := strings.TrimSpace(req.Prompt)
		if captionText == "" {
			captionText = "Caption"
		}
		plan.Operations = append(plan.Operations, EditOperation{Type: "add_text_clip", TrackID: "track-caption-1", StartMS: maxInt64(0, req.PlayheadMS), DurationMS: 4000, Text: captionText})
	}
	if strings.Contains(instruction, "tighten") || strings.Contains(instruction, "pacing") {
		// Trim trailing dead space: shrink the timeline to the last clip end.
		contentEnd := int64(0)
		for _, track := range doc.Tracks {
			for _, clip := range track.Clips {
				if end := clip.StartMS + clip.DurationMS; end > contentEnd {
					contentEnd = end
				}
			}
		}
		if contentEnd >= 1000 && contentEnd < doc.DurationMS {
			plan.Operations = append(plan.Operations, EditOperation{Type: "set_duration", DurationMS: contentEnd})
		}
	}
	if len(plan.Operations) == 0 {
		plan.Operations = append(plan.Operations, EditOperation{Type: "set_duration", DurationMS: 30000})
	}
	annotatePlan(doc, &plan)
	return &plan, nil
}

// llmEditPlan calls the LLM to generate a structured edit plan grounded in the
// current timeline context.
func (s *Service) llmEditPlan(ctx context.Context, provider string, req AssistantRequest, timelineContext string) (*EditPlan, error) {
	instruction := strings.TrimSpace(req.Instruction)
	if instruction == "" {
		instruction = strings.TrimSpace(req.Prompt)
	}
	if instruction == "" {
		instruction = "Suggest useful timeline edits."
	}

	system := `You are an expert video editor. Given an editing instruction and the current timeline state, output a JSON edit plan.
Output ONLY a JSON object matching this schema (no markdown, no explanation):
{
  "summary": "<one sentence describing what this edit plan does>",
  "operations": [
    {
      "type": "<operation type>",
      "clip_id": "<optional clip id>",
      "track_id": "<optional track id, e.g. 'track-text-1'>",
      "start_ms": <optional integer milliseconds>,
      "duration_ms": <optional integer milliseconds>,
      "text": "<optional text content>",
      "width": <optional integer>,
      "height": <optional integer>,
      "fps": <optional integer>
    }
  ]
}
Valid operation types: set_canvas, set_duration, add_text_clip, move_clip, trim_clip, delete_clip.
- set_canvas: provide width, height, fps
- set_duration: provide duration_ms
- add_text_clip: provide track_id, start_ms, duration_ms, text
- move_clip: provide clip_id, start_ms, and optionally track_id
- trim_clip: provide clip_id and duration_ms (optionally start_ms)
- delete_clip: provide clip_id
Reference ONLY clip ids and track ids that appear in the timeline context. Never invent ids.
Only include fields relevant to the operation type. Do not include null or zero values for optional fields.`

	userMessage := "Editing instruction: " + instruction
	if strings.TrimSpace(timelineContext) != "" {
		userMessage += "\n\nCurrent timeline context:\n" + timelineContext
	}

	resp, err := s.llm.ChatComplete(ctx, llm.ChatRequest{
		Provider: provider,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: userMessage},
		},
	})
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(resp.Content)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var plan EditPlan
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		return nil, fmt.Errorf("parse edit plan JSON: %w (raw: %.200s)", err, raw)
	}
	return &plan, nil
}

func (s *Service) ApplyEditPlan(ctx context.Context, userID, projectID string, plan EditPlan) (*models.VideoTimeline, TimelineDocument, error) {
	timeline, doc, err := s.GetOrCreateTimeline(ctx, userID, projectID)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	// Validate against the current timeline and apply only the valid subset so
	// one stale clip reference does not block the rest of the plan.
	validOps, _, issues := ValidateEditPlanOperations(doc, plan)
	if len(validOps) == 0 {
		if len(issues) > 0 {
			return nil, TimelineDocument{}, fmt.Errorf("no valid operations to apply: %s", strings.Join(issues, "; "))
		}
		return nil, TimelineDocument{}, fmt.Errorf("edit plan has no operations")
	}
	doc, err = ApplyEditPlanToTimeline(doc, EditPlan{Summary: plan.Summary, Operations: validOps})
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	raw, err := TimelineToJSON(doc)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	timeline.TimelineJSON = raw
	timeline.DurationMS = doc.DurationMS
	timeline.Active = true
	if err := s.timelines.Save(timeline); err != nil {
		return nil, TimelineDocument{}, err
	}
	return timeline, doc, nil
}

func (s *Service) CreateSocialVariants(ctx context.Context, userID, projectID string, req AssistantRequest) ([]SocialVariant, error) {
	_, doc, err := s.GetOrCreateTimeline(ctx, userID, projectID)
	if err != nil {
		return nil, err
	}
	doc = preferClientTimeline(doc, req)
	fps := doc.Canvas.FPS
	if fps <= 0 {
		fps = DefaultProjectFPS
	}
	// Cap social variants at 30s, but never extend past the actual content.
	variantDuration := doc.DurationMS
	if variantDuration > 30000 {
		variantDuration = 30000
	}
	if variantDuration < 1000 {
		variantDuration = 30000
	}
	variants := []SocialVariant{
		{
			Name:        "Vertical short",
			AspectRatio: "9:16",
			Width:       1080,
			Height:      1920,
			Plan:        EditPlan{Summary: "Create a vertical short", Operations: []EditOperation{{Type: "set_canvas", Width: 1080, Height: 1920, FPS: fps}, {Type: "set_duration", DurationMS: variantDuration}}},
		},
		{
			Name:        "Square feed",
			AspectRatio: "1:1",
			Width:       1080,
			Height:      1080,
			Plan:        EditPlan{Summary: "Create a square feed cut", Operations: []EditOperation{{Type: "set_canvas", Width: 1080, Height: 1080, FPS: fps}, {Type: "set_duration", DurationMS: variantDuration}}},
		},
		{
			Name:        "Widescreen",
			AspectRatio: "16:9",
			Width:       1920,
			Height:      1080,
			Plan:        EditPlan{Summary: "Create a widescreen edit", Operations: []EditOperation{{Type: "set_canvas", Width: 1920, Height: 1080, FPS: fps}}},
		},
	}
	for i := range variants {
		annotatePlan(doc, &variants[i].Plan)
	}
	return variants, nil
}

// timelineContextSummary renders compact structured context (canvas, tracks,
// clips, assets, selection, renderer capabilities) for LLM planning prompts.
func timelineContextSummary(doc TimelineDocument, assets []models.VideoAsset, selectedClipID string, playheadMS int64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Canvas: %dx%d @ %d fps, duration %.1fs\n", doc.Canvas.Width, doc.Canvas.Height, doc.Canvas.FPS, float64(doc.DurationMS)/1000)
	assetNames := make(map[string]string, len(assets))
	if len(assets) > 0 {
		b.WriteString("Assets:\n")
		for _, asset := range assets {
			assetNames[asset.ID] = asset.FileName
			duration := ""
			if asset.DurationMS != nil && *asset.DurationMS > 0 {
				duration = fmt.Sprintf(", %.1fs", float64(*asset.DurationMS)/1000)
			}
			fmt.Fprintf(&b, "- %s (%s%s) id=%s\n", asset.FileName, asset.Kind, duration, asset.ID)
		}
	}
	b.WriteString("Tracks:\n")
	for _, track := range doc.Tracks {
		flags := ""
		if track.Locked {
			flags += " locked"
		}
		if track.Muted {
			flags += " muted"
		}
		if !track.Visible {
			flags += " hidden"
		}
		fmt.Fprintf(&b, "- %s (%s, id=%s%s): %d clip(s)\n", track.Name, track.Type, track.ID, flags, len(track.Clips))
		for _, clip := range track.Clips {
			label := clip.AssetID
			if name, ok := assetNames[clip.AssetID]; ok {
				label = name
			}
			if clip.Text != nil && strings.TrimSpace(clip.Text.Text) != "" {
				label = fmt.Sprintf("text %q", clip.Text.Text)
			}
			selected := ""
			if clip.ID == selectedClipID {
				selected = " [SELECTED]"
			}
			fmt.Fprintf(&b, "  - clip id=%s %s at %.1fs for %.1fs%s\n", clip.ID, label, float64(clip.StartMS)/1000, float64(clip.DurationMS)/1000, selected)
		}
	}
	if playheadMS > 0 {
		fmt.Fprintf(&b, "Playhead: %.1fs\n", float64(playheadMS)/1000)
	}
	caps := FFmpegRendererCapabilities()
	if unsupported := caps.UnsupportedFeatureLabels(); len(unsupported) > 0 {
		fmt.Fprintf(&b, "Export renderer limitations (avoid relying on these): %s\n", strings.Join(unsupported, ", "))
	}
	return b.String()
}

// findTimelineClip locates a clip by ID across all tracks.
func findTimelineClip(doc TimelineDocument, clipID string) (trackIndex, clipIndex int, found bool) {
	for ti := range doc.Tracks {
		for ci := range doc.Tracks[ti].Clips {
			if doc.Tracks[ti].Clips[ci].ID == clipID {
				return ti, ci, true
			}
		}
	}
	return 0, 0, false
}

// ValidateEditPlanOperations checks every operation against the current
// timeline and returns the valid operations, human-readable previews for them,
// and issues describing the operations that were rejected.
func ValidateEditPlanOperations(doc TimelineDocument, plan EditPlan) (valid []EditOperation, preview []string, issues []string) {
	clipLabel := func(clipID string) string {
		ti, ci, ok := findTimelineClip(doc, clipID)
		if !ok {
			return clipID
		}
		clip := doc.Tracks[ti].Clips[ci]
		if clip.Text != nil && strings.TrimSpace(clip.Text.Text) != "" {
			return fmt.Sprintf("%q", clip.Text.Text)
		}
		return clipID
	}
	for i, op := range plan.Operations {
		describeIdx := fmt.Sprintf("operation %d (%s)", i+1, op.Type)
		switch op.Type {
		case "set_canvas":
			if op.Width <= 0 || op.Height <= 0 {
				issues = append(issues, describeIdx+": requires width and height")
				continue
			}
			line := fmt.Sprintf("Set canvas to %dx%d", op.Width, op.Height)
			if op.FPS > 0 {
				line += fmt.Sprintf(" @ %d fps", op.FPS)
			}
			valid = append(valid, op)
			preview = append(preview, line)
		case "set_duration":
			if op.DurationMS <= 0 {
				issues = append(issues, describeIdx+": requires duration_ms")
				continue
			}
			valid = append(valid, op)
			preview = append(preview, fmt.Sprintf("Set timeline duration to %.1fs", float64(op.DurationMS)/1000))
		case "trim_clip":
			if op.ClipID == "" || op.DurationMS <= 0 {
				issues = append(issues, describeIdx+": requires clip_id and duration_ms")
				continue
			}
			if _, _, ok := findTimelineClip(doc, op.ClipID); !ok {
				issues = append(issues, describeIdx+": clip "+op.ClipID+" does not exist in the timeline")
				continue
			}
			valid = append(valid, op)
			preview = append(preview, fmt.Sprintf("Trim clip %s to %.1fs", clipLabel(op.ClipID), float64(op.DurationMS)/1000))
		case "move_clip":
			if op.ClipID == "" {
				issues = append(issues, describeIdx+": requires clip_id")
				continue
			}
			if _, _, ok := findTimelineClip(doc, op.ClipID); !ok {
				issues = append(issues, describeIdx+": clip "+op.ClipID+" does not exist in the timeline")
				continue
			}
			if op.StartMS < 0 {
				issues = append(issues, describeIdx+": start_ms cannot be negative")
				continue
			}
			line := fmt.Sprintf("Move clip %s to %.1fs", clipLabel(op.ClipID), float64(op.StartMS)/1000)
			if op.TrackID != "" {
				trackExists := false
				for _, track := range doc.Tracks {
					if track.ID == op.TrackID {
						trackExists = true
						break
					}
				}
				if !trackExists {
					issues = append(issues, describeIdx+": track "+op.TrackID+" does not exist")
					continue
				}
				line += " on track " + op.TrackID
			}
			valid = append(valid, op)
			preview = append(preview, line)
		case "delete_clip":
			if op.ClipID == "" {
				issues = append(issues, describeIdx+": requires clip_id")
				continue
			}
			if _, _, ok := findTimelineClip(doc, op.ClipID); !ok {
				issues = append(issues, describeIdx+": clip "+op.ClipID+" does not exist in the timeline")
				continue
			}
			valid = append(valid, op)
			preview = append(preview, "Delete clip "+clipLabel(op.ClipID))
		case "add_text_clip":
			if strings.TrimSpace(op.Text) == "" {
				issues = append(issues, describeIdx+": requires text")
				continue
			}
			duration := op.DurationMS
			if duration <= 0 {
				duration = 3000
			}
			valid = append(valid, op)
			preview = append(preview, fmt.Sprintf("Add text %q at %.1fs for %.1fs", op.Text, float64(maxInt64(0, op.StartMS))/1000, float64(duration)/1000))
		default:
			issues = append(issues, describeIdx+": unsupported operation type")
		}
	}
	return valid, preview, issues
}

// annotatePlan validates a plan against the timeline and fills Preview/Issues.
func annotatePlan(doc TimelineDocument, plan *EditPlan) {
	if plan == nil {
		return
	}
	_, preview, issues := ValidateEditPlanOperations(doc, *plan)
	plan.Preview = preview
	plan.Issues = issues
}

func ApplyEditPlanToTimeline(doc TimelineDocument, plan EditPlan) (TimelineDocument, error) {
	doc, err := ValidateTimelineDocument(doc)
	if err != nil {
		return TimelineDocument{}, err
	}
	for _, op := range plan.Operations {
		switch op.Type {
		case "set_canvas":
			if op.Width <= 0 || op.Height <= 0 {
				return TimelineDocument{}, fmt.Errorf("set_canvas requires width and height")
			}
			doc.Canvas.Width = op.Width
			doc.Canvas.Height = op.Height
			if op.FPS > 0 {
				doc.Canvas.FPS = op.FPS
			}
		case "set_duration":
			if op.DurationMS <= 0 {
				return TimelineDocument{}, fmt.Errorf("set_duration requires duration_ms")
			}
			doc.DurationMS = op.DurationMS
		case "trim_clip":
			if op.ClipID == "" || op.DurationMS <= 0 {
				return TimelineDocument{}, fmt.Errorf("trim_clip requires clip_id and duration_ms")
			}
			found := false
			for ti := range doc.Tracks {
				for ci := range doc.Tracks[ti].Clips {
					if doc.Tracks[ti].Clips[ci].ID == op.ClipID {
						if op.StartMS >= 0 {
							doc.Tracks[ti].Clips[ci].StartMS = op.StartMS
						}
						doc.Tracks[ti].Clips[ci].DurationMS = op.DurationMS
						doc.Tracks[ti].Clips[ci].TrimOutMS = doc.Tracks[ti].Clips[ci].TrimInMS + op.DurationMS
						found = true
					}
				}
			}
			if !found {
				return TimelineDocument{}, fmt.Errorf("clip %q not found", op.ClipID)
			}
		case "move_clip":
			if op.ClipID == "" {
				return TimelineDocument{}, fmt.Errorf("move_clip requires clip_id")
			}
			ti, ci, ok := findTimelineClip(doc, op.ClipID)
			if !ok {
				return TimelineDocument{}, fmt.Errorf("clip %q not found", op.ClipID)
			}
			clip := doc.Tracks[ti].Clips[ci]
			clip.StartMS = maxInt64(0, op.StartMS)
			targetIdx := ti
			if op.TrackID != "" && op.TrackID != doc.Tracks[ti].ID {
				targetIdx = -1
				for i := range doc.Tracks {
					if doc.Tracks[i].ID == op.TrackID {
						targetIdx = i
						break
					}
				}
				if targetIdx == -1 {
					return TimelineDocument{}, fmt.Errorf("track %q not found", op.TrackID)
				}
			}
			doc.Tracks[ti].Clips = append(doc.Tracks[ti].Clips[:ci], doc.Tracks[ti].Clips[ci+1:]...)
			doc.Tracks[targetIdx].Clips = append(doc.Tracks[targetIdx].Clips, clip)
		case "delete_clip":
			if op.ClipID == "" {
				return TimelineDocument{}, fmt.Errorf("delete_clip requires clip_id")
			}
			ti, ci, ok := findTimelineClip(doc, op.ClipID)
			if !ok {
				return TimelineDocument{}, fmt.Errorf("clip %q not found", op.ClipID)
			}
			doc.Tracks[ti].Clips = append(doc.Tracks[ti].Clips[:ci], doc.Tracks[ti].Clips[ci+1:]...)
		case "add_text_clip":
			if strings.TrimSpace(op.Text) == "" {
				return TimelineDocument{}, fmt.Errorf("add_text_clip requires text")
			}
			trackID := op.TrackID
			if trackID == "" {
				trackID = "track-text-1"
			}
			trackIndex := -1
			for i := range doc.Tracks {
				if doc.Tracks[i].ID == trackID {
					trackIndex = i
					break
				}
			}
			if trackIndex == -1 {
				doc.Tracks = append(doc.Tracks, TimelineTrack{ID: trackID, Type: TrackTypeText, Name: "Text 1", Visible: true, Clips: []TimelineClip{}})
				trackIndex = len(doc.Tracks) - 1
			}
			duration := op.DurationMS
			if duration <= 0 {
				duration = 3000
			}
			doc.Tracks[trackIndex].Clips = append(doc.Tracks[trackIndex].Clips, TimelineClip{
				ID:         "clip-" + uuid.New().String(),
				StartMS:    maxInt64(0, op.StartMS),
				DurationMS: duration,
				TrimInMS:   0,
				TrimOutMS:  duration,
				Text:       &TimelineText{Text: op.Text, FontSize: 48, FontWeight: "700", Color: "#ffffff", Shadow: true},
				Transform:  defaultTransform(),
				Effects:    []TimelineEffect{},
				Keyframes:  []TimelineKeyframe{},
			})
		default:
			return TimelineDocument{}, fmt.Errorf("unsupported edit operation %q", op.Type)
		}
	}
	return ValidateTimelineDocument(doc)
}

func cinematicPrompt(seed, shot string) string {
	return EnhancePrompt(EnhancePromptRequest{
		Prompt:          fmt.Sprintf("%s, %s", seed, shot),
		DurationSeconds: 6,
		AspectRatio:     "16:9",
	})
}
