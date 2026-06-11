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
	ID          string               `json:"id"`
	AssetID     string               `json:"asset_id,omitempty"`
	StartMS     int64                `json:"start_ms"`
	DurationMS  int64                `json:"duration_ms"`
	TrimInMS    int64                `json:"trim_in_ms"`
	TrimOutMS   int64                `json:"trim_out_ms"`
	ZIndex      *int                 `json:"z_index,omitempty"`
	GroupID     string               `json:"group_id,omitempty"`
	Transform   map[string]any       `json:"transform,omitempty"`
	Volume      *float64             `json:"volume,omitempty"`
	FadeInMS    int64                `json:"fade_in_ms,omitempty"`
	FadeOutMS   int64                `json:"fade_out_ms,omitempty"`
	Text        *TimelineText        `json:"text,omitempty"`
	Effects     []TimelineEffect     `json:"effects"`
	Transitions []TimelineTransition `json:"transitions,omitempty"`
	Keyframes   []TimelineKeyframe   `json:"keyframes"`
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
		Tracks: []TimelineTrack{
			{ID: "track-video-1", Type: TrackTypeVideo, Name: "Video 1", Visible: true, Clips: []TimelineClip{}},
			{ID: "track-overlay-1", Type: TrackTypeImage, Name: "Overlay 1", Visible: true, Clips: []TimelineClip{}},
			{ID: "track-audio-1", Type: TrackTypeAudio, Name: "Audio 1", Visible: true, Clips: []TimelineClip{}},
			{ID: "track-text-1", Type: TrackTypeText, Name: "Text 1", Visible: true, Clips: []TimelineClip{}},
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
	trackType := normalizeTrackType(req.TrackType)
	if trackType == "" {
		trackType = trackTypeForAssetKind(asset.Kind)
	}
	if trackType == "" {
		return TimelineDocument{}, TimelineClip{}, fmt.Errorf("unsupported asset kind %q", asset.Kind)
	}

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
		if !trackAcceptsKind(doc.Tracks[trackIndex].Type, asset.Kind) {
			return TimelineDocument{}, TimelineClip{}, fmt.Errorf("asset kind %q is not compatible with %s track", asset.Kind, doc.Tracks[trackIndex].Type)
		}
	} else {
		for i := range doc.Tracks {
			if !doc.Tracks[i].Locked && trackAcceptsKind(doc.Tracks[i].Type, asset.Kind) {
				trackIndex = i
				break
			}
		}
	}
	if trackIndex == -1 {
		doc.Tracks = append(doc.Tracks, TimelineTrack{
			ID:      fmt.Sprintf("track-%s-%d", trackType, len(doc.Tracks)+1),
			Type:    trackType,
			Name:    defaultTrackName(trackType, len(doc.Tracks)+1),
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
		duration = defaultDurationForTrack(trackType)
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
	if trackType == TrackTypeAudio || trackType == TrackTypeMusic {
		clip.Volume = &volume
	} else {
		clip.Transform = defaultTransform()
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

func trackAcceptsKind(trackType, kind string) bool {
	trackType = normalizeTrackType(trackType)
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

func defaultDurationForTrack(trackType string) int64 {
	switch trackType {
	case TrackTypeImage, TrackTypeText, TrackTypeCaption, TrackTypeShape, TrackTypeCallout:
		return 5000
	case TrackTypeAudio, TrackTypeMusic:
		return 30000
	default:
		return 8000
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
