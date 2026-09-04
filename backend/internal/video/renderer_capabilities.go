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
	RendererFeatureClipTrim       = "clip_trim"
	RendererFeaturePlaybackRate   = "playback_rate"
	RendererFeatureClipOrdering   = "clip_ordering"
	RendererFeatureScaling        = "scaling"
	RendererFeaturePositioning    = "positioning"
	RendererFeatureCropping       = "cropping"
	RendererFeatureRotation       = "rotation"
	RendererFeatureOpacity        = "opacity"
	RendererFeatureVideoFades     = "video_fades"
	RendererFeatureText           = "text_overlays"
	RendererFeatureTransitions    = "transitions"
	RendererFeatureEffects        = "effects"
	RendererFeatureKeyframes      = "keyframes"
	RendererFeatureAudioMix       = "audio_mix"
	RendererFeatureClipVolume     = "clip_volume"
	RendererFeatureAudioFades     = "audio_fades"
	RendererFeatureTrackMute      = "track_mute"
	RendererFeatureTrackSolo      = "track_solo"
	RendererFeatureAnnotations    = "annotations"
	RendererFeatureCursor         = "cursor_effects"
	RendererFeatureBezierCurves   = "bezier_curves"
	RendererFeatureSpringCurves   = "spring_curves"
	RendererFeatureSpatial3D      = "spatial_3d_transform"
	RendererFeatureCameraMotion   = "camera_motion"
	RendererFeatureCameraParallax = "camera_parallax"
	RendererFeatureFilmGrain      = "film_grain"
	RendererFeatureBloom          = "bloom"
	RendererFeatureColorGrade     = "color_grade"
	RendererFeatureEdgeFade       = "edge_fade"
	RendererFeatureRGBSplit       = "rgb_split"
	RendererFeatureGhostTrail     = "ghost_trail"
	RendererFeatureMotionBlur     = "motion_blur"
	RendererFeatureDepthOfField   = "depth_of_field"
	RendererFeatureRackFocus      = "rack_focus"
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
			{Feature: RendererFeatureEffects, Label: "Effects", Supported: true, Partial: true, Notes: "Brightness, contrast, saturation, blur, grayscale, sharpen, vignette, chroma key, film grain, color grade, edge fade, RGB split, ghost trail, and sampled depth effects export. Bloom is a blur approximation; drop shadow and background blur remain unsupported."},
			{Feature: RendererFeatureKeyframes, Label: "Keyframes", Supported: true, Partial: true, Notes: "Position, spatial scale/depth, rotation, opacity, volume, and validated effect-amount keyframes export through deterministic sampled segments."},
			{Feature: RendererFeatureAudioMix, Label: "Multi-track audio mix", Supported: true, Notes: "Audio and music mix with video soundtracks and optional denoise, EQ, compression, LUFS normalization, limiting, and channel conversion."},
			{Feature: RendererFeatureClipVolume, Label: "Per-clip volume & mute", Supported: true},
			{Feature: RendererFeatureAudioFades, Label: "Audio fade in/out", Supported: true},
			{Feature: RendererFeatureTrackMute, Label: "Track mute / hide", Supported: true},
			{Feature: RendererFeatureTrackSolo, Label: "Track solo", Supported: true, Notes: "Persisted solo state limits the exported audio mix while leaving visual layer visibility unchanged."},
			{Feature: RendererFeatureAnnotations, Label: "Annotations", Supported: true, Partial: true, Notes: "Every annotation produces deterministic export output, but ellipse, arrow, speech-bubble, and other complex geometry currently normalize to simpler primitives."},
			{Feature: RendererFeatureCursor, Label: "Cursor effects", Supported: true, Partial: true, Notes: "Static 2D media cursors up to the bounded fidelity segment limit use cursor-state-v1 linear or cursor-state-v2 deterministic smoothstep output-frame sampling plus pointer, highlight, and click-ring rasters on the owner track. Animated/3D/camera/effect/transition/fade parents and longer clips retain the compatibility fallback. Click audio is not synthesized."},
			{Feature: RendererFeatureBezierCurves, Label: "Bezier motion curves", Supported: true, Partial: true, Notes: "Bezier curves are deterministically sampled into bounded render segments."},
			{Feature: RendererFeatureSpringCurves, Label: "Spring motion curves", Supported: true, Partial: true, Notes: "Segment-local zero-velocity springs are deterministically sampled into bounded render segments."},
			{Feature: RendererFeatureSpatial3D, Label: "2.5D transforms", Supported: true, Partial: true, Notes: "Depth, non-uniform scale, and Z rotation export; X/Y tilt and perspective remain preview-only."},
			{Feature: RendererFeatureCameraMotion, Label: "Camera motion", Supported: true, Partial: true, Notes: "Camera position, depth, Z rotation, and field-of-view keyframes export through sampled projection."},
			{Feature: RendererFeatureCameraParallax, Label: "Camera parallax", Supported: true, Partial: true, Notes: "Depth layers project with deterministic parallax; X/Y camera tilt remains preview-only."},
			{Feature: RendererFeatureFilmGrain, Label: "Film grain", Supported: true},
			{Feature: RendererFeatureBloom, Label: "Bloom", Supported: true, Partial: true, Notes: "Export uses a bounded Gaussian glow approximation."},
			{Feature: RendererFeatureColorGrade, Label: "Color grade", Supported: true, Notes: "Export maps the authored intensity to bounded contrast and saturation."},
			{Feature: RendererFeatureEdgeFade, Label: "Edge fade", Supported: true, Notes: "Export uses the renderer's vignette primitive."},
			{Feature: RendererFeatureRGBSplit, Label: "RGB split", Supported: true, Notes: "Export shifts red and blue channels with edge smearing."},
			{Feature: RendererFeatureGhostTrail, Label: "Ghost trail", Supported: true, Partial: true, Notes: "Export uses a bounded three-frame temporal mix."},
			{Feature: RendererFeatureMotionBlur, Label: "Motion blur", Supported: true, Partial: true, Notes: "Export uses a bounded three-frame temporal mix."},
			{Feature: RendererFeatureDepthOfField, Label: "Depth of field", Supported: true, Partial: true, Notes: "Export uses sampled depth blur rather than a full depth map."},
			{Feature: RendererFeatureRackFocus, Label: "Rack focus", Supported: true, Partial: true, Notes: "Focus-depth animation is sampled into bounded blur segments."},
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
