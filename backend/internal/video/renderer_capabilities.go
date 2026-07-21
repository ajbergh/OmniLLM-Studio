package video

// RendererFeatureSupport describes how completely the export renderer honors a
// single timeline feature. It is the source of truth for export-fidelity
// warnings shown in the frontend; keep it synchronized with the FFmpeg graph
// and the fidelity expansion layer.
type RendererFeatureSupport struct {
	Feature   string `json:"feature"`
	Label     string `json:"label"`
	Supported bool   `json:"supported"`
	Partial   bool   `json:"partial,omitempty"`
	Notes     string `json:"notes,omitempty"`
}

// RendererCapabilities reports which timeline features the FFmpeg renderer
// applies during export. Keep this stable because the frontend and assistant
// consume the formats/features collection directly.
type RendererCapabilities struct {
	Renderer string                   `json:"renderer"`
	Formats  []string                 `json:"formats"`
	Features []RendererFeatureSupport `json:"features"`
}

const (
	RendererFeatureClipTrim     = "clip_trim"
	RendererFeaturePlaybackRate = "playback_rate"
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
	RendererFeatureAnnotations  = "annotations"
	RendererFeatureCursor       = "cursor_effects"
)

// FFmpegRendererCapabilities returns the conservative feature support matrix
// for the production renderer. A feature is upgraded only after the applicable
// render path is implemented and covered by renderer tests.
func FFmpegRendererCapabilities() RendererCapabilities {
	return RendererCapabilities{
		Renderer: "ffmpeg",
		Formats:  []string{"mp4", "webm"},
		Features: []RendererFeatureSupport{
			{Feature: RendererFeatureClipTrim, Label: "Clip trim", Supported: true},
			{Feature: RendererFeaturePlaybackRate, Label: "Constant clip speed", Supported: true, Notes: "Video and audio retime together from 0.25x to 4x; audio uses pitch-preserving atempo filters."},
			{Feature: RendererFeatureClipOrdering, Label: "Clip ordering & timing", Supported: true, Notes: "Later layers render above earlier layers, matching the preview."},
			{Feature: RendererFeatureScaling, Label: "Scaling", Supported: true},
			{Feature: RendererFeaturePositioning, Label: "Position (x/y offset)", Supported: true},
			{Feature: RendererFeatureCropping, Label: "Cropping", Supported: true, Partial: true, Notes: "Crop values are source-frame fractions. Wipe transitions are approximated by sampled crop segments."},
			{Feature: RendererFeatureRotation, Label: "Rotation", Supported: true},
			{Feature: RendererFeatureOpacity, Label: "Opacity", Supported: true},
			{Feature: RendererFeatureVideoFades, Label: "Video fade in/out", Supported: true},
			{Feature: RendererFeatureText, Label: "Text / caption / callout overlays", Supported: true, Partial: true, Notes: "Font, size, color, line height, stroke, shadow, background, transform, opacity, fades, and deterministic alignment/letter-spacing approximation export. Rounded text-box corners remain preview-only."},
			{Feature: RendererFeatureTransitions, Label: "Transitions", Supported: true, Partial: true, Notes: "Fade, dip, slide, sampled zoom, and directional wipe export. Crossfade remains an alpha-fade approximation rather than a true two-clip blend."},
			{Feature: RendererFeatureEffects, Label: "Effects", Supported: true, Partial: true, Notes: "Brightness, contrast, saturation, blur, grayscale, sharpen, vignette, and chroma key export; amount keyframes are sampled. Drop shadow and background blur remain unsupported."},
			{Feature: RendererFeatureKeyframes, Label: "Keyframes", Supported: true, Partial: true, Notes: "Position, scale, rotation, opacity, volume, and effect-amount keyframes export through deterministic sampled segments with linear, ease-in, ease-out, ease-in-out, and step interpolation; continuous curves remain approximations."},
			{Feature: RendererFeatureAudioMix, Label: "Multi-track audio mix", Supported: true, Notes: "Audio and music mix with video soundtracks and optional denoise, EQ, compression, LUFS normalization, limiting, and channel conversion."},
			{Feature: RendererFeatureClipVolume, Label: "Per-clip volume & mute", Supported: true},
			{Feature: RendererFeatureAudioFades, Label: "Audio fade in/out", Supported: true},
			{Feature: RendererFeatureTrackMute, Label: "Track mute / hide", Supported: true},
			{Feature: RendererFeatureTrackSolo, Label: "Track solo", Supported: false, Notes: "Solo is a preview-only monitoring control; exports mix all unmuted tracks."},
			{Feature: RendererFeatureAnnotations, Label: "Annotations", Supported: true, Partial: true, Notes: "Every annotation produces deterministic export output, but ellipse, arrow, speech-bubble, and other complex geometry currently normalize to simpler primitives."},
			{Feature: RendererFeatureCursor, Label: "Cursor effects", Supported: true, Partial: true, Notes: "Cursor paths and click rings export through sampled overlays. Click audio is not synthesized."},
		},
	}
}

// UnsupportedFeatureLabels returns the labels of features that are unsupported
// or only partially honored at export, for compact warning copy.
func (c RendererCapabilities) UnsupportedFeatureLabels() []string {
	labels := make([]string, 0)
	for _, feature := range c.Features {
		if !feature.Supported || feature.Partial {
			labels = append(labels, feature.Label)
		}
	}
	return labels
}
