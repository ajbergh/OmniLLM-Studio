package video

import (
	"context"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

type AssistantRequest struct {
	Prompt      string           `json:"prompt,omitempty"`
	Instruction string           `json:"instruction,omitempty"`
	Timeline    TimelineDocument `json:"timeline,omitempty"`
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
	_ = ctx
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = strings.TrimSpace(req.Instruction)
	}
	if prompt == "" {
		prompt = "A concise product story with an opening hook, demonstration, and closing title card."
	}
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

func (s *Service) CreateTimelinePlan(ctx context.Context, userID, projectID string, req AssistantRequest) (*EditPlan, error) {
	if _, _, err := s.GetOrCreateTimeline(ctx, userID, projectID); err != nil {
		return nil, err
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = "Untitled video"
	}
	return &EditPlan{
		Summary: "Create a simple timeline scaffold with a title card and a 30 second project duration.",
		Operations: []EditOperation{
			{Type: "set_duration", DurationMS: 30000},
			{Type: "add_text_clip", TrackID: "track-text-1", StartMS: 0, DurationMS: 3500, Text: DeriveTitle(prompt)},
		},
	}, nil
}

func (s *Service) CreateEditPlan(ctx context.Context, userID, projectID string, req AssistantRequest) (*EditPlan, error) {
	if _, _, err := s.GetOrCreateTimeline(ctx, userID, projectID); err != nil {
		return nil, err
	}
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
	if strings.Contains(instruction, "30") || strings.Contains(instruction, "thirty") {
		plan.Operations = append(plan.Operations, EditOperation{Type: "set_duration", DurationMS: 30000})
	}
	if strings.Contains(instruction, "title") || strings.Contains(instruction, "intro") {
		plan.Operations = append(plan.Operations, EditOperation{Type: "add_text_clip", TrackID: "track-text-1", StartMS: 0, DurationMS: 3000, Text: "Opening Title"})
	}
	if len(plan.Operations) == 0 {
		plan.Operations = append(plan.Operations, EditOperation{Type: "set_duration", DurationMS: 30000})
	}
	return &plan, nil
}

func (s *Service) ApplyEditPlan(ctx context.Context, userID, projectID string, plan EditPlan) (*models.VideoTimeline, TimelineDocument, error) {
	timeline, doc, err := s.GetOrCreateTimeline(ctx, userID, projectID)
	if err != nil {
		return nil, TimelineDocument{}, err
	}
	doc, err = ApplyEditPlanToTimeline(doc, plan)
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
	if _, _, err := s.GetOrCreateTimeline(ctx, userID, projectID); err != nil {
		return nil, err
	}
	_ = req
	return []SocialVariant{
		{
			Name:        "Vertical short",
			AspectRatio: "9:16",
			Width:       1080,
			Height:      1920,
			Plan:        EditPlan{Summary: "Create a vertical short", Operations: []EditOperation{{Type: "set_canvas", Width: 1080, Height: 1920, FPS: DefaultProjectFPS}, {Type: "set_duration", DurationMS: 30000}}},
		},
		{
			Name:        "Square feed",
			AspectRatio: "1:1",
			Width:       1080,
			Height:      1080,
			Plan:        EditPlan{Summary: "Create a square feed cut", Operations: []EditOperation{{Type: "set_canvas", Width: 1080, Height: 1080, FPS: DefaultProjectFPS}, {Type: "set_duration", DurationMS: 30000}}},
		},
		{
			Name:        "Widescreen",
			AspectRatio: "16:9",
			Width:       1920,
			Height:      1080,
			Plan:        EditPlan{Summary: "Create a widescreen edit", Operations: []EditOperation{{Type: "set_canvas", Width: 1920, Height: 1080, FPS: DefaultProjectFPS}}},
		},
	}, nil
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
