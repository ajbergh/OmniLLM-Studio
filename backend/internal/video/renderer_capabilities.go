package video

// RendererFeatureSupport describes how completely the export renderer honors a
// single timeline feature. It is the single source of truth for export-fidelity
// warnings shown in the frontend — keep it in sync with buildFilterComplex.
type RendererFeatureSupport struct {
	Feature   string `json:"feature"`
	Label     string `json:"label"`
	Supported bool   `json:"supported"`
	Partial   bool   `json:"partial,omitempty"`
	Notes     string `json:"notes,omitempty"`
}

// RendererCapabilities reports which timeline features the FFmpeg renderer
// applies during export.
type RendererCapabilities struct {
	Renderer string                   `json:"renderer"`
	Formats  []string                 `json:"formats"`
	Features []RendererFeatureSupport `json:"features"`
}

const (
	RendererFeatureClipTrim     = "clip_trim"
	RendererFeatureClipOrdering = "clip_ordering"
	RendererFeatureScaling      = "scaling"
	RendererFeaturePositioning  = "positioning"
	RendererFeatureCropping     = "cropping"
	RendererFeatureRotation     = "rotation"
	RendererFeatureOpacity      = "opacity"
	RendererFeatureVideoFades   = "video_fades"
	RendererFeatureText         = "text_overlays"
	RendererFeatureTransitions  = "transitions"
	RendererFeatureEffects      = "effects"
	RendererFeatureKeyframes    = "keyframes"
	RendererFeatureAudioMix     = "audio_mix"
	RendererFeatureClipVolume   = "clip_volume"
	RendererFeatureAudioFades   = "audio_fades"
	RendererFeatureTrackMute    = "track_mute"
	RendererFeatureTrackSolo    = "track_solo"
)

// FFmpegRendererCapabilities returns the feature support matrix for the
// built-in FFmpeg renderer.
func FFmpegRendererCapabilities() RendererCapabilities {
	return RendererCapabilities{
		Renderer: "ffmpeg",
		Formats:  []string{"mp4", "webm"},
		Features: []RendererFeatureSupport{
			{Feature: RendererFeatureClipTrim, Label: "Clip trim", Supported: true},
			{Feature: RendererFeatureClipOrdering, Label: "Clip ordering & timing", Supported: true},
			{Feature: RendererFeatureScaling, Label: "Scaling", Supported: true},
			{Feature: RendererFeaturePositioning, Label: "Position (x/y offset)", Supported: true},
			{Feature: RendererFeatureCropping, Label: "Cropping", Supported: true, Partial: true, Notes: "Crop values are fractions of the source frame (0–1)."},
			{Feature: RendererFeatureRotation, Label: "Rotation", Supported: false, Notes: "Rotation is previewed but not applied during export yet."},
			{Feature: RendererFeatureOpacity, Label: "Opacity", Supported: true},
			{Feature: RendererFeatureVideoFades, Label: "Video fade in/out", Supported: true},
			{Feature: RendererFeatureText, Label: "Text / caption / callout overlays", Supported: true},
			{Feature: RendererFeatureTransitions, Label: "Transitions", Supported: true, Partial: true, Notes: "Fade-style transitions (fade, crossfade, dip to black) are rendered as alpha fades; slide, wipe, and zoom are not applied."},
			{Feature: RendererFeatureEffects, Label: "Effects", Supported: true, Partial: true, Notes: "Brightness, contrast, saturation, blur, and grayscale render; other effect types are skipped."},
			{Feature: RendererFeatureKeyframes, Label: "Keyframes", Supported: false, Notes: "Keyframe animation is stored in the timeline but not applied during export yet."},
			{Feature: RendererFeatureAudioMix, Label: "Multi-track audio mix", Supported: true},
			{Feature: RendererFeatureClipVolume, Label: "Per-clip volume", Supported: true},
			{Feature: RendererFeatureAudioFades, Label: "Audio fade in/out", Supported: true},
			{Feature: RendererFeatureTrackMute, Label: "Track mute / hide", Supported: true},
			{Feature: RendererFeatureTrackSolo, Label: "Track solo", Supported: false, Notes: "Solo is not part of the timeline schema yet."},
		},
	}
}

// UnsupportedFeatureLabels returns the labels of features that are not (or only
// partially) honored at export, for compact warning copy.
func (c RendererCapabilities) UnsupportedFeatureLabels() []string {
	var labels []string
	for _, f := range c.Features {
		if !f.Supported || f.Partial {
			labels = append(labels, f.Label)
		}
	}
	return labels
}
