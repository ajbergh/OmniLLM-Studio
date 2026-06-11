package video

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/google/uuid"
)

const (
	// TrackTypeLayer is a generic ordered layer that accepts any clip kind.
	// Media behavior (visual/audio, defaults, render treatment) comes from the
	// clip and its asset, not the track. Higher array indices stack on top.
	TrackTypeLayer   = "layer"
	TrackTypeVideo   = "video"
	TrackTypeImage   = "image"
	TrackTypeAudio   = "audio"
	TrackTypeMusic   = "music"
	TrackTypeText    = "text"
	TrackTypeCaption = "caption"
	TrackTypeShape   = "shape"
	TrackTypeCallout = "callout"
)

// CurrentTimelineVersion is the newest timeline document version this build
// can read and write. Documents from future builds fail with an actionable
// error instead of being silently mangled.
const CurrentTimelineVersion = 1

const (
	EffectTypeBlur           = "blur"
	EffectTypeBrightness     = "brightness"
	EffectTypeContrast       = "contrast"
	EffectTypeSaturation     = "saturation"
	EffectTypeGrayscale      = "grayscale"
	EffectTypeShadow         = "shadow"
	EffectTypeBackgroundBlur = "background_blur"
	EffectTypeChromaKey      = "chroma_key"
	EffectTypeSharpen        = "sharpen"
	EffectTypeVignette       = "vignette"
)

const (
	TransitionTypeFade       = "fade"
	TransitionTypeCrossfade  = "crossfade"
	TransitionTypeDipToBlack = "dip_to_black"
	TransitionTypeSlide      = "slide"
	TransitionTypeWipe       = "wipe"
	TransitionTypeZoom       = "zoom"
)

const (
	// ShapeKindRectangle draws an outlined box; ShapeKindHighlight a filled
	// translucent box. Both export via FFmpeg drawbox. ShapeKindBlur blurs
	// the composited region beneath it (redaction) via crop+boxblur+overlay.
	ShapeKindRectangle = "rectangle"
	ShapeKindHighlight = "highlight"
	ShapeKindBlur      = "blur"
	// Annotation kinds. ShapeKindPixelate exports via crop+downscale+upscale
	// (mosaic redaction); ShapeKindRoundedRectangle and ShapeKindLabel export
	// as square-corner drawbox approximations. The remaining annotation kinds
	// are preview-only until the renderer gains vector drawing.
	ShapeKindRoundedRectangle = "rounded_rectangle"
	ShapeKindEllipse          = "ellipse"
	ShapeKindArrow            = "arrow"
	ShapeKindLine             = "line"
	ShapeKindSpeechBubble     = "speech_bubble"
	ShapeKindSpotlight        = "spotlight"
	ShapeKindPixelate         = "pixelate"
	ShapeKindCheckmark        = "checkmark"
	ShapeKindXMark            = "x_mark"
	ShapeKindStepMarker       = "step_marker"
	ShapeKindLabel            = "label"
)

var knownShapeKinds = map[string]bool{
	ShapeKindRectangle:        true,
	ShapeKindHighlight:        true,
	ShapeKindBlur:             true,
	ShapeKindRoundedRectangle: true,
	ShapeKindEllipse:          true,
	ShapeKindArrow:            true,
	ShapeKindLine:             true,
	ShapeKindSpeechBubble:     true,
	ShapeKindSpotlight:        true,
	ShapeKindPixelate:         true,
	ShapeKindCheckmark:        true,
	ShapeKindXMark:            true,
	ShapeKindStepMarker:       true,
	ShapeKindLabel:            true,
}

var knownEffectTypes = map[string]bool{
	EffectTypeBlur:           true,
	EffectTypeBrightness:     true,
	EffectTypeContrast:       true,
	EffectTypeSaturation:     true,
	EffectTypeGrayscale:      true,
	EffectTypeShadow:         true,
	EffectTypeBackgroundBlur: true,
	EffectTypeChromaKey:      true,
	EffectTypeSharpen:        true,
	EffectTypeVignette:       true,
}

var knownTransitionTypes = map[string]bool{
	TransitionTypeFade:       true,
	TransitionTypeCrossfade:  true,
	TransitionTypeDipToBlack: true,
	TransitionTypeSlide:      true,
	TransitionTypeWipe:       true,
	TransitionTypeZoom:       true,
}

var knownKeyframeProperties = map[string]bool{
	"x":        true,
	"y":        true,
	"scale":    true,
	"rotation": true,
	"opacity":  true,
	"volume":   true,
}

var knownKeyframeEasings = map[string]bool{
	"linear":      true,
	"ease-in":     true,
	"ease-out":    true,
	"ease-in-out": true,
	"step":        true,
}

const (
	minTrackHeight = 32
	maxTrackHeight = 160
)

type TimelineDocument struct {
	Version    int              `json:"version"`
	Canvas     TimelineCanvas   `json:"canvas"`
	DurationMS int64            `json:"duration_ms"`
	Tracks     []TimelineTrack  `json:"tracks"`
	Markers    []TimelineMarker `json:"markers"`
	Metadata   map[string]any   `json:"metadata"`
}

type TimelineCanvas struct {
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	FPS        int    `json:"fps"`
	Background string `json:"background"`
}

type TimelineTrack struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Name    string         `json:"name"`
	Locked  bool           `json:"locked"`
	Muted   bool           `json:"muted"`
	Visible bool           `json:"visible"`
	Height  *int           `json:"height,omitempty"`
	Clips   []TimelineClip `json:"clips"`
}

type TimelineClip struct {
	ID         string `json:"id"`
	AssetID    string `json:"asset_id,omitempty"`
	StartMS    int64  `json:"start_ms"`
	DurationMS int64  `json:"duration_ms"`
	TrimInMS   int64  `json:"trim_in_ms"`
	TrimOutMS  int64  `json:"trim_out_ms"`
	ZIndex     *int   `json:"z_index,omitempty"`
	GroupID    string `json:"group_id,omitempty"`
	// Muted silences this clip's audio contribution without touching Volume.
	Muted bool `json:"muted,omitempty"`
	// AudioOnly suppresses a clip's visual output so a video asset can act as
	// a detached audio clip.
	AudioOnly   bool                 `json:"audio_only,omitempty"`
	Transform   map[string]any       `json:"transform,omitempty"`
	Volume      *float64             `json:"volume,omitempty"`
	FadeInMS    int64                `json:"fade_in_ms,omitempty"`
	FadeOutMS   int64                `json:"fade_out_ms,omitempty"`
	Text        *TimelineText        `json:"text,omitempty"`
	Shape       *TimelineShape       `json:"shape,omitempty"`
	Cursor      *TimelineCursor      `json:"cursor,omitempty"`
	Effects     []TimelineEffect     `json:"effects"`
	Transitions []TimelineTransition `json:"transitions,omitempty"`
	Keyframes   []TimelineKeyframe   `json:"keyframes"`
}

// TimelineShape is a parameterized callout/annotation box. Dimensions are in
// canvas pixels; position comes from the clip transform like any visual clip.
type TimelineShape struct {
	Kind        string  `json:"kind"`
	Width       int     `json:"width,omitempty"`
	Height      int     `json:"height,omitempty"`
	Fill        string  `json:"fill,omitempty"`
	Stroke      string  `json:"stroke,omitempty"`
	StrokeWidth float64 `json:"stroke_width,omitempty"`
	// BlurRadius applies to blur/pixelate regions: blur radius or pixel block
	// size respectively (clamped 1–50, default 12).
	BlurRadius float64 `json:"blur_radius,omitempty"`
	// CornerRadius applies to rounded rectangles, speech bubbles, and labels
	// (clamped 0–200). Preview-only; exports draw square corners.
	CornerRadius float64 `json:"corner_radius,omitempty"`
}

// TimelineCursor carries cursor metadata captured with screen recordings so
// cursor effects can layer onto footage. Persisted but preview-only today —
// the renderer reports it as unsupported until export support lands.
type TimelineCursor struct {
	Visible    bool                  `json:"visible,omitempty"`
	Scale      float64               `json:"scale,omitempty"`
	Highlight  bool                  `json:"highlight,omitempty"`
	ClickRings bool                  `json:"click_rings,omitempty"`
	Smoothing  bool                  `json:"smoothing,omitempty"`
	Events     []TimelineCursorEvent `json:"events,omitempty"`
}

// TimelineCursorEvent is a sampled cursor position (canvas pixels from the
// top-left), clip-relative in time. Click marks press events for click rings.
type TimelineCursorEvent struct {
	TimeMS int64   `json:"time_ms"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Click  bool    `json:"click,omitempty"`
}

type TimelineMarker struct {
	ID     string `json:"id"`
	TimeMS int64  `json:"time_ms"`
	Label  string `json:"label"`
}

type TimelineText struct {
	Text          string         `json:"text"`
	FontFamily    string         `json:"font_family,omitempty"`
	FontSize      int            `json:"font_size,omitempty"`
	FontWeight    string         `json:"font_weight,omitempty"`
	Color         string         `json:"color,omitempty"`
	Background    string         `json:"background,omitempty"`
	Stroke        string         `json:"stroke,omitempty"`
	StrokeWidth   float64        `json:"stroke_width,omitempty"`
	Shadow        bool           `json:"shadow,omitempty"`
	TextAlign     string         `json:"text_align,omitempty"`
	LineHeight    float64        `json:"line_height,omitempty"`
	LetterSpacing float64        `json:"letter_spacing,omitempty"`
	BorderRadius  float64        `json:"border_radius,omitempty"`
	Params        map[string]any `json:"params,omitempty"`
}

type TimelineEffect struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Enabled bool           `json:"enabled"`
	Params  map[string]any `json:"params"`
}

type TimelineTransition struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	DurationMS int64  `json:"duration_ms"`
	Direction  string `json:"direction,omitempty"`
}

type TimelineKeyframe struct {
	ID       string  `json:"id"`
	Property string  `json:"property"`
	TimeMS   int64   `json:"time_ms"`
	Value    float64 `json:"value"`
	Easing   string  `json:"easing,omitempty"`
}

// SliceTimelineRange returns a copy of the document trimmed to the window
// [startMS, endMS): clips outside the window drop, straddling clips trim
// (advancing their source trim window and rebasing clip-relative keyframes),
// and everything shifts so the window starts at 0. Used for export ranges.
func SliceTimelineRange(doc TimelineDocument, startMS, endMS int64) TimelineDocument {
	if endMS <= startMS || startMS < 0 {
		return doc
	}
	if endMS > doc.DurationMS && doc.DurationMS > 0 {
		endMS = doc.DurationMS
	}
	if endMS <= startMS {
		return doc
	}
	out := doc
	out.Tracks = make([]TimelineTrack, len(doc.Tracks))
	for ti, track := range doc.Tracks {
		copied := track
		copied.Clips = nil
		for _, clip := range track.Clips {
			clipEnd := clip.StartMS + clip.DurationMS
			if clipEnd <= startMS || clip.StartMS >= endMS {
				continue
			}
			next := clip
			if lead := startMS - next.StartMS; lead > 0 {
				next.TrimInMS += lead
				next.DurationMS -= lead
				next.StartMS = startMS
				next.FadeInMS = 0
				var keyframes []TimelineKeyframe
				for _, keyframe := range next.Keyframes {
					if keyframe.TimeMS < lead {
						continue
					}
					rebased := keyframe
					rebased.TimeMS -= lead
					keyframes = append(keyframes, rebased)
				}
				next.Keyframes = keyframes
			}
			if over := (next.StartMS + next.DurationMS) - endMS; over > 0 {
				next.DurationMS -= over
				next.TrimOutMS = next.TrimInMS + next.DurationMS
				next.FadeOutMS = 0
				var keyframes []TimelineKeyframe
				for _, keyframe := range next.Keyframes {
					if keyframe.TimeMS > next.DurationMS {
						continue
					}
					keyframes = append(keyframes, keyframe)
				}
				next.Keyframes = keyframes
			}
			next.StartMS -= startMS
			copied.Clips = append(copied.Clips, next)
		}
		if copied.Clips == nil {
			copied.Clips = []TimelineClip{}
		}
		out.Tracks[ti] = copied
	}
	out.Markers = nil
	for _, marker := range doc.Markers {
		if marker.TimeMS < startMS || marker.TimeMS >= endMS {
			continue
		}
		shifted := marker
		shifted.TimeMS -= startMS
		out.Markers = append(out.Markers, shifted)
	}
	if out.Markers == nil {
		out.Markers = []TimelineMarker{}
	}
	out.DurationMS = endMS - startMS
	return out
}

// StripCaptionOverlays returns a copy with caption-track clips removed so the
// renderer skips burning them into the frame (burn_in_captions=false).
func StripCaptionOverlays(doc TimelineDocument) TimelineDocument {
	out := doc
	out.Tracks = make([]TimelineTrack, len(doc.Tracks))
	for ti, track := range doc.Tracks {
		copied := track
		if track.Type == TrackTypeCaption {
			copied.Clips = []TimelineClip{}
		}
		out.Tracks[ti] = copied
	}
	return out
}

type TimelineImportAssetRequest struct {
	AssetID    string `json:"asset_id"`
	TrackID    string `json:"track_id,omitempty"`
	TrackType  string `json:"track_type,omitempty"`
	StartMS    int64  `json:"start_ms,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
}

func NewEmptyTimeline(width, height, fps int) TimelineDocument {
	if width <= 0 {
		width = DefaultProjectWidth
	}
	if height <= 0 {
		height = DefaultProjectHeight
	}
	if fps <= 0 {
		fps = DefaultProjectFPS
	}
	return TimelineDocument{
		Version: 1,
		Canvas: TimelineCanvas{
			Width:      width,
			Height:     height,
			FPS:        fps,
			Background: "#000000",
		},
		DurationMS: 30000,
		// Generic layers: index 0 (Layer 1) is the background; later layers
		// stack on top, matching the preview compositor and FFmpeg renderer.
		Tracks: []TimelineTrack{
			{ID: "track-layer-1", Type: TrackTypeLayer, Name: "Layer 1", Visible: true, Clips: []TimelineClip{}},
			{ID: "track-layer-2", Type: TrackTypeLayer, Name: "Layer 2", Visible: true, Clips: []TimelineClip{}},
			{ID: "track-layer-3", Type: TrackTypeLayer, Name: "Layer 3", Visible: true, Clips: []TimelineClip{}},
			{ID: "track-layer-4", Type: TrackTypeLayer, Name: "Layer 4", Visible: true, Clips: []TimelineClip{}},
		},
		Markers:  []TimelineMarker{},
		Metadata: map[string]any{},
	}
}

func TimelineToJSON(doc TimelineDocument) (string, error) {
	normalized, err := ValidateTimelineDocument(doc)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("marshal timeline: %w", err)
	}
	return string(data), nil
}

func TimelineFromJSON(raw string, fallback TimelineDocument) (TimelineDocument, error) {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "{}" {
		return ValidateTimelineDocument(fallback)
	}
	var doc TimelineDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return TimelineDocument{}, fmt.Errorf("parse timeline: %w", err)
	}
	return ValidateTimelineDocument(doc)
}

// UpgradeTimelineDocument normalizes the document version and applies
// stepwise upgrades for older versions. All schema changes so far have been
// additive (optional fields), so version 1 is still current; when the first
// breaking change lands, chain v1->v2 (etc.) upgraders here and bump
// CurrentTimelineVersion.
func UpgradeTimelineDocument(doc TimelineDocument) (TimelineDocument, error) {
	if doc.Version == 0 {
		doc.Version = 1
	}
	if doc.Version > CurrentTimelineVersion {
		return TimelineDocument{}, fmt.Errorf("unsupported timeline version %d: this build supports versions up to %d — upgrade OmniLLM Studio to open this timeline", doc.Version, CurrentTimelineVersion)
	}
	if doc.Version < 1 {
		return TimelineDocument{}, fmt.Errorf("unsupported timeline version %d", doc.Version)
	}
	return doc, nil
}

func ValidateTimelineDocument(doc TimelineDocument) (TimelineDocument, error) {
	doc, err := UpgradeTimelineDocument(doc)
	if err != nil {
		return TimelineDocument{}, err
	}
	if doc.Canvas.Width <= 0 {
		doc.Canvas.Width = DefaultProjectWidth
	}
	if doc.Canvas.Height <= 0 {
		doc.Canvas.Height = DefaultProjectHeight
	}
	if doc.Canvas.FPS <= 0 {
		doc.Canvas.FPS = DefaultProjectFPS
	}
	if strings.TrimSpace(doc.Canvas.Background) == "" {
		doc.Canvas.Background = "#000000"
	}
	if doc.Metadata == nil {
		doc.Metadata = map[string]any{}
	}
	if doc.Markers == nil {
		doc.Markers = []TimelineMarker{}
	}
	markerIDs := map[string]bool{}
	for mi := range doc.Markers {
		marker := &doc.Markers[mi]
		marker.ID = strings.TrimSpace(marker.ID)
		if marker.ID == "" {
			marker.ID = "marker-" + uuid.New().String()
		}
		if markerIDs[marker.ID] {
			return TimelineDocument{}, fmt.Errorf("duplicate marker id %q", marker.ID)
		}
		markerIDs[marker.ID] = true
		if marker.TimeMS < 0 {
			marker.TimeMS = 0
		}
		marker.Label = strings.TrimSpace(marker.Label)
	}
	sort.SliceStable(doc.Markers, func(i, j int) bool {
		return doc.Markers[i].TimeMS < doc.Markers[j].TimeMS
	})
	if doc.Tracks == nil {
		doc.Tracks = []TimelineTrack{}
	}
	trackIDs := map[string]bool{}
	clipIDs := map[string]bool{}
	maxEnd := int64(0)
	for ti := range doc.Tracks {
		track := &doc.Tracks[ti]
		track.ID = strings.TrimSpace(track.ID)
		if track.ID == "" {
			track.ID = fmt.Sprintf("track-%s-%d", normalizeTrackType(track.Type), ti+1)
		}
		if trackIDs[track.ID] {
			return TimelineDocument{}, fmt.Errorf("duplicate track id %q", track.ID)
		}
		trackIDs[track.ID] = true
		track.Type = normalizeTrackType(track.Type)
		if track.Type == "" {
			return TimelineDocument{}, fmt.Errorf("track %q has unsupported type", track.ID)
		}
		if strings.TrimSpace(track.Name) == "" {
			track.Name = defaultTrackName(track.Type, ti+1)
		}
		if track.Height != nil {
			height := *track.Height
			if height < minTrackHeight {
				height = minTrackHeight
			}
			if height > maxTrackHeight {
				height = maxTrackHeight
			}
			track.Height = &height
		}
		if track.Clips == nil {
			track.Clips = []TimelineClip{}
		}
		for ci := range track.Clips {
			clip := &track.Clips[ci]
			clip.ID = strings.TrimSpace(clip.ID)
			if clip.ID == "" {
				clip.ID = "clip-" + uuid.New().String()
			}
			if clipIDs[clip.ID] {
				return TimelineDocument{}, fmt.Errorf("duplicate clip id %q", clip.ID)
			}
			clipIDs[clip.ID] = true
			if clip.StartMS < 0 {
				return TimelineDocument{}, fmt.Errorf("clip %q start_ms cannot be negative", clip.ID)
			}
			if clip.DurationMS <= 0 {
				return TimelineDocument{}, fmt.Errorf("clip %q duration_ms must be greater than zero", clip.ID)
			}
			if clip.TrimInMS < 0 || clip.TrimOutMS < 0 {
				return TimelineDocument{}, fmt.Errorf("clip %q trim values cannot be negative", clip.ID)
			}
			if clip.TrimOutMS == 0 {
				clip.TrimOutMS = clip.TrimInMS + clip.DurationMS
			}
			clip.GroupID = strings.TrimSpace(clip.GroupID)
			if clip.Transform == nil && track.Type != TrackTypeAudio && track.Type != TrackTypeMusic {
				clip.Transform = defaultTransform()
			}
			if clip.Shape != nil {
				clip.Shape.Kind = strings.ToLower(strings.TrimSpace(clip.Shape.Kind))
				if !knownShapeKinds[clip.Shape.Kind] {
					return TimelineDocument{}, fmt.Errorf("clip %q has unsupported shape kind %q", clip.ID, clip.Shape.Kind)
				}
				if clip.Shape.Width <= 0 {
					clip.Shape.Width = 320
				}
				if clip.Shape.Height <= 0 {
					clip.Shape.Height = 180
				}
				if clip.Shape.Width > 8192 {
					clip.Shape.Width = 8192
				}
				if clip.Shape.Height > 8192 {
					clip.Shape.Height = 8192
				}
				clip.Shape.StrokeWidth = clampFloat(clip.Shape.StrokeWidth, 0, 100)
				if clip.Shape.Kind == ShapeKindBlur || clip.Shape.Kind == ShapeKindPixelate {
					if clip.Shape.BlurRadius <= 0 {
						clip.Shape.BlurRadius = 12
					}
					clip.Shape.BlurRadius = clampFloat(clip.Shape.BlurRadius, 1, 50)
				}
				clip.Shape.CornerRadius = clampFloat(clip.Shape.CornerRadius, 0, 200)
			}
			if clip.Cursor != nil {
				if clip.Cursor.Scale <= 0 {
					clip.Cursor.Scale = 1
				}
				clip.Cursor.Scale = clampFloat(clip.Cursor.Scale, 0.25, 4)
				events := clip.Cursor.Events[:0]
				for _, event := range clip.Cursor.Events {
					if event.TimeMS < 0 {
						continue
					}
					events = append(events, event)
				}
				clip.Cursor.Events = events
				sort.SliceStable(clip.Cursor.Events, func(i, j int) bool {
					return clip.Cursor.Events[i].TimeMS < clip.Cursor.Events[j].TimeMS
				})
			}
			if clip.Effects == nil {
				clip.Effects = []TimelineEffect{}
			}
			if clip.Keyframes == nil {
				clip.Keyframes = []TimelineKeyframe{}
			}
			if clip.Transitions == nil {
				clip.Transitions = []TimelineTransition{}
			}
			effectIDs := map[string]bool{}
			for ei := range clip.Effects {
				effect := &clip.Effects[ei]
				if effect.ID == "" {
					effect.ID = "effect-" + uuid.New().String()
				}
				if effectIDs[effect.ID] {
					return TimelineDocument{}, fmt.Errorf("clip %q has duplicate effect id %q", clip.ID, effect.ID)
				}
				effectIDs[effect.ID] = true
				effect.Type = strings.ToLower(strings.TrimSpace(effect.Type))
				if !knownEffectTypes[effect.Type] {
					return TimelineDocument{}, fmt.Errorf("clip %q has unsupported effect type %q", clip.ID, effect.Type)
				}
				if effect.Params == nil {
					effect.Params = map[string]any{}
				}
			}
			transitionIDs := map[string]bool{}
			for xi := range clip.Transitions {
				transition := &clip.Transitions[xi]
				if transition.ID == "" {
					transition.ID = "transition-" + uuid.New().String()
				}
				if transitionIDs[transition.ID] {
					return TimelineDocument{}, fmt.Errorf("clip %q has duplicate transition id %q", clip.ID, transition.ID)
				}
				transitionIDs[transition.ID] = true
				transition.Type = strings.ToLower(strings.TrimSpace(transition.Type))
				if !knownTransitionTypes[transition.Type] {
					return TimelineDocument{}, fmt.Errorf("clip %q has unsupported transition type %q", clip.ID, transition.Type)
				}
				if transition.DurationMS <= 0 {
					return TimelineDocument{}, fmt.Errorf("clip %q transition %q duration_ms must be greater than zero", clip.ID, transition.ID)
				}
			}
			keyframeIDs := map[string]bool{}
			for ki := range clip.Keyframes {
				keyframe := &clip.Keyframes[ki]
				if keyframe.ID == "" {
					keyframe.ID = "keyframe-" + uuid.New().String()
				}
				if keyframeIDs[keyframe.ID] {
					return TimelineDocument{}, fmt.Errorf("clip %q has duplicate keyframe id %q", clip.ID, keyframe.ID)
				}
				keyframeIDs[keyframe.ID] = true
				keyframe.Property = strings.ToLower(strings.TrimSpace(keyframe.Property))
				if !knownKeyframeProperties[keyframe.Property] {
					return TimelineDocument{}, fmt.Errorf("clip %q has unsupported keyframe property %q", clip.ID, keyframe.Property)
				}
				if keyframe.TimeMS < 0 {
					return TimelineDocument{}, fmt.Errorf("clip %q keyframe %q time_ms cannot be negative", clip.ID, keyframe.ID)
				}
				keyframe.Easing = strings.ToLower(strings.TrimSpace(keyframe.Easing))
				if keyframe.Easing != "" && !knownKeyframeEasings[keyframe.Easing] {
					keyframe.Easing = "linear"
				}
			}
			if end := clip.StartMS + clip.DurationMS; end > maxEnd {
				maxEnd = end
			}
		}
	}
	if doc.DurationMS <= 0 {
		doc.DurationMS = maxInt64(maxEnd, 30000)
	}
	if maxEnd > doc.DurationMS {
		doc.DurationMS = maxEnd
	}
	return doc, nil
}

func AddAssetToTimeline(doc TimelineDocument, asset models.VideoAsset, req TimelineImportAssetRequest) (TimelineDocument, TimelineClip, error) {
	doc, err := ValidateTimelineDocument(doc)
	if err != nil {
		return TimelineDocument{}, TimelineClip{}, err
	}
	kind := kindForAssetOrMime(asset)

	trackIndex := -1
	if req.TrackID != "" {
		for i := range doc.Tracks {
			if doc.Tracks[i].ID == req.TrackID {
				trackIndex = i
				break
			}
		}
		if trackIndex == -1 {
			return TimelineDocument{}, TimelineClip{}, fmt.Errorf("track not found")
		}
		// Any clip kind is accepted on an explicit target track; only a lock
		// blocks placement.
		if doc.Tracks[trackIndex].Locked {
			return TimelineDocument{}, TimelineClip{}, fmt.Errorf("track %q is locked", doc.Tracks[trackIndex].Name)
		}
	} else {
		// Honor a legacy track_type hint first, then prefer tracks that
		// naturally accept the kind (generic layers accept everything, so new
		// timelines pick the first unlocked layer).
		if requested := normalizeTrackType(req.TrackType); requested != "" {
			for i := range doc.Tracks {
				if !doc.Tracks[i].Locked && doc.Tracks[i].Type == requested {
					trackIndex = i
					break
				}
			}
		}
		if trackIndex == -1 {
			for i := range doc.Tracks {
				if !doc.Tracks[i].Locked && trackAcceptsKind(doc.Tracks[i].Type, kind) {
					trackIndex = i
					break
				}
			}
		}
	}
	if trackIndex == -1 {
		doc.Tracks = append(doc.Tracks, TimelineTrack{
			ID:      "track-" + uuid.New().String(),
			Type:    TrackTypeLayer,
			Name:    fmt.Sprintf("Layer %d", len(doc.Tracks)+1),
			Visible: true,
			Clips:   []TimelineClip{},
		})
		trackIndex = len(doc.Tracks) - 1
	}

	start := req.StartMS
	if start < 0 {
		start = 0
	}
	duration := req.DurationMS
	if duration <= 0 && asset.DurationMS != nil && *asset.DurationMS > 0 {
		duration = *asset.DurationMS
	}
	if duration <= 0 {
		duration = defaultDurationForAssetKind(kind)
	}
	trimOut := duration
	volume := 1.0
	clip := TimelineClip{
		ID:         "clip-" + uuid.New().String(),
		AssetID:    asset.ID,
		StartMS:    start,
		DurationMS: duration,
		TrimInMS:   0,
		TrimOutMS:  trimOut,
		Effects:    []TimelineEffect{},
		Keyframes:  []TimelineKeyframe{},
	}
	if isVisualAssetKind(kind) {
		clip.Transform = defaultTransform()
	}
	if isAudioAssetKind(kind) {
		clip.Volume = &volume
	}
	doc.Tracks[trackIndex].Clips = append(doc.Tracks[trackIndex].Clips, clip)
	if end := clip.StartMS + clip.DurationMS; end > doc.DurationMS {
		doc.DurationMS = end
	}
	return doc, clip, nil
}

func SplitClipAt(doc TimelineDocument, clipID string, timeMS int64) (TimelineDocument, error) {
	doc, err := ValidateTimelineDocument(doc)
	if err != nil {
		return TimelineDocument{}, err
	}
	for ti := range doc.Tracks {
		for ci := range doc.Tracks[ti].Clips {
			clip := doc.Tracks[ti].Clips[ci]
			if clip.ID != clipID {
				continue
			}
			offset := timeMS - clip.StartMS
			if offset <= 0 || offset >= clip.DurationMS {
				return TimelineDocument{}, fmt.Errorf("split point must be inside the clip")
			}
			left := clip
			right := clip
			left.DurationMS = offset
			left.TrimOutMS = left.TrimInMS + offset
			right.ID = "clip-" + uuid.New().String()
			right.StartMS = timeMS
			right.DurationMS = clip.DurationMS - offset
			right.TrimInMS = left.TrimOutMS
			right.TrimOutMS = clip.TrimOutMS
			clips := append([]TimelineClip{}, doc.Tracks[ti].Clips[:ci+1]...)
			clips[ci] = left
			clips = append(clips, right)
			clips = append(clips, doc.Tracks[ti].Clips[ci+1:]...)
			doc.Tracks[ti].Clips = clips
			return ValidateTimelineDocument(doc)
		}
	}
	return TimelineDocument{}, fmt.Errorf("clip not found")
}

func DeleteClip(doc TimelineDocument, clipID string) (TimelineDocument, error) {
	doc, err := ValidateTimelineDocument(doc)
	if err != nil {
		return TimelineDocument{}, err
	}
	for ti := range doc.Tracks {
		filtered := doc.Tracks[ti].Clips[:0]
		for _, clip := range doc.Tracks[ti].Clips {
			if clip.ID != clipID {
				filtered = append(filtered, clip)
			}
		}
		doc.Tracks[ti].Clips = filtered
	}
	return ValidateTimelineDocument(doc)
}

func defaultTransform() map[string]any {
	return map[string]any{
		"x":        0,
		"y":        0,
		"scale":    1,
		"rotation": 0,
		"opacity":  1,
	}
}

func normalizeTrackType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case TrackTypeLayer:
		return TrackTypeLayer
	case TrackTypeVideo:
		return TrackTypeVideo
	case TrackTypeImage, "overlay":
		return TrackTypeImage
	case TrackTypeAudio:
		return TrackTypeAudio
	case TrackTypeMusic:
		return TrackTypeMusic
	case TrackTypeText:
		return TrackTypeText
	case TrackTypeCaption:
		return TrackTypeCaption
	case TrackTypeShape:
		return TrackTypeShape
	case TrackTypeCallout:
		return TrackTypeCallout
	default:
		return ""
	}
}

func trackTypeForAssetKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "video":
		return TrackTypeVideo
	case "image":
		return TrackTypeImage
	case "audio":
		return TrackTypeAudio
	case "music":
		return TrackTypeMusic
	case "text":
		return TrackTypeText
	case "caption":
		return TrackTypeCaption
	default:
		return ""
	}
}

// trackAcceptsKind reports whether a track naturally accepts an asset kind.
// Generic layers accept everything; for legacy typed tracks this preserves
// the old media routing. Used only for automatic placement preference — an
// explicit track_id bypasses it entirely.
func trackAcceptsKind(trackType, kind string) bool {
	trackType = normalizeTrackType(trackType)
	if trackType == TrackTypeLayer {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "video":
		return trackType == TrackTypeVideo
	case "image":
		return trackType == TrackTypeImage || trackType == TrackTypeVideo
	case "audio":
		return trackType == TrackTypeAudio || trackType == TrackTypeMusic
	case "music":
		return trackType == TrackTypeMusic || trackType == TrackTypeAudio
	case "text":
		return trackType == TrackTypeText || trackType == TrackTypeCallout
	case "caption":
		return trackType == TrackTypeCaption || trackType == TrackTypeText
	default:
		return false
	}
}

func defaultTrackName(trackType string, index int) string {
	switch trackType {
	case TrackTypeLayer:
		return fmt.Sprintf("Layer %d", index)
	case TrackTypeVideo:
		return fmt.Sprintf("Video %d", index)
	case TrackTypeImage:
		return fmt.Sprintf("Overlay %d", index)
	case TrackTypeAudio:
		return fmt.Sprintf("Audio %d", index)
	case TrackTypeMusic:
		return fmt.Sprintf("Music %d", index)
	case TrackTypeText:
		return fmt.Sprintf("Text %d", index)
	case TrackTypeCaption:
		return fmt.Sprintf("Captions %d", index)
	case TrackTypeShape:
		return fmt.Sprintf("Shape %d", index)
	case TrackTypeCallout:
		return fmt.Sprintf("Callout %d", index)
	default:
		return fmt.Sprintf("Track %d", index)
	}
}

func defaultDurationForAssetKind(kind string) int64 {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "image", "text", "caption", "shape", "callout":
		return 5000
	case "audio", "music":
		return 30000
	default:
		return 8000
	}
}

// kindForAssetOrMime returns the effective asset kind, falling back to the
// MIME type prefix when the kind column is empty.
func kindForAssetOrMime(asset models.VideoAsset) string {
	if kind := strings.ToLower(strings.TrimSpace(asset.Kind)); kind != "" {
		return kind
	}
	mime := strings.ToLower(strings.TrimSpace(asset.MimeType))
	switch {
	case strings.HasPrefix(mime, "video/"):
		return "video"
	case strings.HasPrefix(mime, "image/"):
		return "image"
	case strings.HasPrefix(mime, "audio/"):
		return "audio"
	default:
		return "other"
	}
}

// isVisualAssetKind reports whether clips of this kind produce visual output.
// Unknown kinds default to visual — a spurious transform on a non-visual clip
// is harmless, while a missing one degrades editing.
func isVisualAssetKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "audio", "music":
		return false
	default:
		return true
	}
}

// isAudioAssetKind reports whether clips of this kind can contribute audio to
// the mix. Video and export assets carry audio streams.
func isAudioAssetKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "audio", "music", "video", "export":
		return true
	default:
		return false
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
