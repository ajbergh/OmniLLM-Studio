package video

type RendererFeatureSupport struct {
	Supported bool   `json:"supported"`
	Partial   bool   `json:"partial,omitempty"`
	Notes     string `json:"notes,omitempty"`
}
type RendererCapabilities struct {
	Renderer        string                 `json:"renderer"`
	Version         int                    `json:"version"`
	Clips           RendererFeatureSupport `json:"clips"`
	TrackOrdering   RendererFeatureSupport `json:"track_ordering"`
	TrackMute       RendererFeatureSupport `json:"track_mute"`
	TrackVisibility RendererFeatureSupport `json:"track_visibility"`
	TrackSolo       RendererFeatureSupport `json:"track_solo"`
	Trimming        RendererFeatureSupport `json:"trimming"`
	PlaybackRate    RendererFeatureSupport `json:"playback_rate"`
	Crop            RendererFeatureSupport `json:"crop"`
	AudioMix        RendererFeatureSupport `json:"audio_mix"`
	ClipVolume      RendererFeatureSupport `json:"clip_volume"`
	AudioFades      RendererFeatureSupport `json:"audio_fades"`
	Text            RendererFeatureSupport `json:"text"`
	Captions        RendererFeatureSupport `json:"captions"`
	Shapes          RendererFeatureSupport `json:"shapes"`
	Transitions     RendererFeatureSupport `json:"transitions"`
	Effects         RendererFeatureSupport `json:"effects"`
	Keyframes       RendererFeatureSupport `json:"keyframes"`
	Annotations     RendererFeatureSupport `json:"annotations"`
	CursorEffects   RendererFeatureSupport `json:"cursor_effects"`
}

func FFmpegRendererCapabilities() RendererCapabilities {
	return RendererCapabilities{Renderer: "ffmpeg", Version: 3, Clips: RendererFeatureSupport{Supported: true, Notes: "Media, image, text, caption, and annotation clips are composited."}, TrackOrdering: RendererFeatureSupport{Supported: true, Notes: "Later timeline layers render above earlier layers."}, TrackMute: RendererFeatureSupport{Supported: true}, TrackVisibility: RendererFeatureSupport{Supported: true}, TrackSolo: RendererFeatureSupport{Supported: false, Notes: "Solo remains a preview-only audition state."}, Trimming: RendererFeatureSupport{Supported: true}, PlaybackRate: RendererFeatureSupport{Supported: true, Notes: "Video and audio are retimed together."}, Crop: RendererFeatureSupport{Supported: true}, AudioMix: RendererFeatureSupport{Supported: true, Notes: "Audio mix supports optional denoise, EQ, compression, LUFS normalization, limiting, and channel conversion."}, ClipVolume: RendererFeatureSupport{Supported: true}, AudioFades: RendererFeatureSupport{Supported: true}, Text: RendererFeatureSupport{Supported: true, Partial: true, Notes: "Font, size, color, line height, stroke, shadow, background, transform, opacity, fades, and render-time alignment/letter-spacing approximation export. Rounded text-box corners remain preview-only."}, Captions: RendererFeatureSupport{Supported: true}, Shapes: RendererFeatureSupport{Supported: true, Partial: true, Notes: "Rectangle, highlight, label, blur, and pixelate render directly; other annotations normalize to deterministic exportable primitives."}, Transitions: RendererFeatureSupport{Supported: true, Partial: true, Notes: "Fade, dip, slide, sampled zoom, and directional wipe export. Crossfade remains alpha-fade approximation rather than a two-clip blend."}, Effects: RendererFeatureSupport{Supported: true, Partial: true, Notes: "Brightness, contrast, saturation, grayscale, blur, sharpen, vignette, and chroma key export; amount keyframes are sampled. Drop shadow and background blur remain skipped."}, Keyframes: RendererFeatureSupport{Supported: true, Notes: "x, y, scale, rotation, opacity, volume, and effect amount keyframes export with linear, ease-in, ease-out, ease-in-out, and step interpolation."}, Annotations: RendererFeatureSupport{Supported: true, Partial: true, Notes: "All annotation kinds produce export output; complex geometry is normalized until native FFmpeg path drawing has golden-frame coverage."}, CursorEffects: RendererFeatureSupport{Supported: true, Partial: true, Notes: "Cursor paths and click rings export through sampled overlay clips. Click audio is not synthesized."}}
}
