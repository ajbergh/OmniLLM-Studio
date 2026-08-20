package rendercontract

// Metadata is the explicit extension point for non-authoritative metadata and effect parameter payloads.
type Metadata map[string]any

// TimelineV2Document is the Go projection of timeline-v2.schema.json.
type TimelineV2Document struct {
	Version           int                `json:"version"`
	Canvas            TimelineV2Canvas   `json:"canvas"`
	DurationMS        int64              `json:"duration_ms"`
	Tracks            []TimelineV2Track  `json:"tracks"`
	Markers           []TimelineV2Marker `json:"markers"`
	Scenes            []TimelineV2Scene  `json:"scenes,omitempty"`
	WorkingColorSpace string             `json:"working_color_space,omitempty"`
	Metadata          Metadata           `json:"metadata"`
}
type TimelineV2Canvas struct {
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	FPS        int    `json:"fps"`
	Background string `json:"background"`
}
type TimelineV2Track struct {
	ID      string           `json:"id"`
	Type    string           `json:"type"`
	Name    string           `json:"name"`
	Locked  bool             `json:"locked"`
	Muted   bool             `json:"muted"`
	Solo    bool             `json:"solo,omitempty"`
	Visible bool             `json:"visible"`
	Height  *int             `json:"height,omitempty"`
	Clips   []TimelineV2Clip `json:"clips"`
}
type TimelineV2Clip struct {
	ID              string                     `json:"id"`
	AssetID         string                     `json:"asset_id,omitempty"`
	StartMS         int64                      `json:"start_ms"`
	DurationMS      int64                      `json:"duration_ms"`
	TrimInMS        int64                      `json:"trim_in_ms"`
	TrimOutMS       int64                      `json:"trim_out_ms"`
	PlaybackRate    *float64                   `json:"playback_rate,omitempty"`
	ZIndex          *int                       `json:"z_index,omitempty"`
	GroupID         string                     `json:"group_id,omitempty"`
	TemplateSlot    string                     `json:"template_slot,omitempty"`
	Muted           bool                       `json:"muted,omitempty"`
	AudioOnly       bool                       `json:"audio_only,omitempty"`
	Transform       *TimelineV2Transform       `json:"transform,omitempty"`
	MediaFit        string                     `json:"media_fit,omitempty"`
	MaskSourceCrop  *TimelineV2Crop            `json:"mask_source_crop,omitempty"`
	ContentBounds   *TimelineV2ContentBounds   `json:"content_bounds,omitempty"`
	Volume          *float64                   `json:"volume,omitempty"`
	FadeInMS        int64                      `json:"fade_in_ms,omitempty"`
	FadeOutMS       int64                      `json:"fade_out_ms,omitempty"`
	Text            *TimelineV2Text            `json:"text,omitempty"`
	Shape           *TimelineV2Shape           `json:"shape,omitempty"`
	Cursor          *TimelineV2Cursor          `json:"cursor,omitempty"`
	Effects         []TimelineV2Effect         `json:"effects"`
	Transitions     []TimelineV2Transition     `json:"transitions,omitempty"`
	Keyframes       []TimelineV2Keyframe       `json:"keyframes"`
	AnimationBlocks []TimelineV2AnimationBlock `json:"animation_blocks,omitempty"`
	Metadata        Metadata                   `json:"metadata,omitempty"`
}
type TimelineV2Transform struct {
	X           *float64        `json:"x,omitempty"`
	Y           *float64        `json:"y,omitempty"`
	Z           *float64        `json:"z,omitempty"`
	Scale       *float64        `json:"scale,omitempty"`
	ScaleX      *float64        `json:"scale_x,omitempty"`
	ScaleY      *float64        `json:"scale_y,omitempty"`
	Rotation    *float64        `json:"rotation,omitempty"`
	RotationX   *float64        `json:"rotation_x,omitempty"`
	RotationY   *float64        `json:"rotation_y,omitempty"`
	RotationZ   *float64        `json:"rotation_z,omitempty"`
	Opacity     *float64        `json:"opacity,omitempty"`
	AnchorX     *float64        `json:"anchor_x,omitempty"`
	AnchorY     *float64        `json:"anchor_y,omitempty"`
	Perspective *float64        `json:"perspective,omitempty"`
	Crop        *TimelineV2Crop `json:"crop,omitempty"`
}
type TimelineV2Crop struct {
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
}
type TimelineV2ContentBounds struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}
type TimelineV2Text struct {
	Text          string   `json:"text"`
	FontFamily    string   `json:"font_family,omitempty"`
	FontSize      *int     `json:"font_size,omitempty"`
	FontWeight    string   `json:"font_weight,omitempty"`
	Color         string   `json:"color,omitempty"`
	Background    string   `json:"background,omitempty"`
	Stroke        string   `json:"stroke,omitempty"`
	StrokeWidth   *float64 `json:"stroke_width,omitempty"`
	Shadow        bool     `json:"shadow,omitempty"`
	TextAlign     string   `json:"text_align,omitempty"`
	VerticalAlign string   `json:"vertical_align,omitempty"`
	LineHeight    *float64 `json:"line_height,omitempty"`
	LetterSpacing *float64 `json:"letter_spacing,omitempty"`
	BorderRadius  *float64 `json:"border_radius,omitempty"`
	BoxWidth      *float64 `json:"box_width,omitempty"`
	BoxHeight     *float64 `json:"box_height,omitempty"`
	PaddingTop    *float64 `json:"padding_top,omitempty"`
	PaddingRight  *float64 `json:"padding_right,omitempty"`
	PaddingBottom *float64 `json:"padding_bottom,omitempty"`
	PaddingLeft   *float64 `json:"padding_left,omitempty"`
	Params        Metadata `json:"params,omitempty"`
}
type TimelineV2Shape struct {
	Kind         string   `json:"kind"`
	Width        *int     `json:"width,omitempty"`
	Height       *int     `json:"height,omitempty"`
	Fill         string   `json:"fill,omitempty"`
	Stroke       string   `json:"stroke,omitempty"`
	StrokeWidth  *float64 `json:"stroke_width,omitempty"`
	BlurRadius   *float64 `json:"blur_radius,omitempty"`
	CornerRadius *float64 `json:"corner_radius,omitempty"`
}
type TimelineV2Cursor struct {
	Visible    *bool                   `json:"visible,omitempty"`
	Scale      *float64                `json:"scale,omitempty"`
	Highlight  bool                    `json:"highlight,omitempty"`
	ClickRings bool                    `json:"click_rings,omitempty"`
	Smoothing  bool                    `json:"smoothing,omitempty"`
	Events     []TimelineV2CursorEvent `json:"events,omitempty"`
}
type TimelineV2CursorEvent struct {
	TimeMS int64   `json:"time_ms"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Click  bool    `json:"click,omitempty"`
}
type TimelineV2Effect struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Enabled bool     `json:"enabled"`
	Params  Metadata `json:"params"`
}
type TimelineV2Transition struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	DurationMS int64  `json:"duration_ms"`
	Direction  string `json:"direction,omitempty"`
	Placement  string `json:"placement"`
	PeerClipID string `json:"peer_clip_id,omitempty"`
}
type TimelineV2Keyframe struct {
	ID       string       `json:"id"`
	Property string       `json:"property"`
	TimeMS   int64        `json:"time_ms"`
	Value    float64      `json:"value"`
	Easing   string       `json:"easing,omitempty"`
	Curve    *MotionCurve `json:"curve,omitempty"`
}
type TimelineV2AnimationBlock struct {
	ID                   string   `json:"id"`
	BlockKey             string   `json:"block_key"`
	Family               string   `json:"family"`
	StartMS              int64    `json:"start_ms"`
	DurationMS           int64    `json:"duration_ms"`
	DelayMS              int64    `json:"delay_ms,omitempty"`
	Params               Metadata `json:"params,omitempty"`
	GeneratedKeyframeIDs []string `json:"generated_keyframe_ids"`
}
type TimelineV2Camera struct {
	X           *float64             `json:"x,omitempty"`
	Y           *float64             `json:"y,omitempty"`
	Z           *float64             `json:"z,omitempty"`
	RotationX   *float64             `json:"rotation_x,omitempty"`
	RotationY   *float64             `json:"rotation_y,omitempty"`
	RotationZ   *float64             `json:"rotation_z,omitempty"`
	FieldOfView *float64             `json:"field_of_view,omitempty"`
	FocusDepth  *float64             `json:"focus_depth,omitempty"`
	Keyframes   []TimelineV2Keyframe `json:"keyframes,omitempty"`
}
type TimelineV2Scene struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	StartMS    int64              `json:"start_ms"`
	DurationMS int64              `json:"duration_ms"`
	Camera     *TimelineV2Camera  `json:"camera,omitempty"`
	Effects    []TimelineV2Effect `json:"effects,omitempty"`
	Metadata   Metadata           `json:"metadata,omitempty"`
}
type TimelineV2Marker struct {
	ID     string `json:"id"`
	TimeMS int64  `json:"time_ms"`
	Label  string `json:"label"`
}

// RenderManifestV1 is the Go projection of render-manifest-v1.schema.json.
type RenderManifestV1 struct {
	Version             int                    `json:"version"`
	ContractVersion     string                 `json:"contract_version"`
	SnapshotID          string                 `json:"snapshot_id"`
	TimelineID          string                 `json:"timeline_id"`
	TimelineRevision    int64                  `json:"timeline_revision"`
	TimelineSHA256      string                 `json:"timeline_sha256"`
	AssetManifestSHA256 string                 `json:"asset_manifest_sha256"`
	Timeline            TimelineV2Document     `json:"timeline"`
	Assets              []RenderManifestAsset  `json:"assets"`
	Settings            RenderManifestSettings `json:"settings"`
	Metadata            Metadata               `json:"metadata,omitempty"`
}
type RenderManifestAsset struct {
	AssetID    string                    `json:"asset_id"`
	ClipIDs    []string                  `json:"clip_ids"`
	StagedPath string                    `json:"staged_path"`
	FileSHA256 string                    `json:"file_sha256"`
	SizeBytes  int64                     `json:"size_bytes"`
	Kind       string                    `json:"kind"`
	MimeType   string                    `json:"mime_type,omitempty"`
	Media      *RenderManifestMediaProbe `json:"media,omitempty"`
}
type RenderManifestMediaProbe struct {
	DurationMS     *int64 `json:"duration_ms,omitempty"`
	Width          *int   `json:"width,omitempty"`
	Height         *int   `json:"height,omitempty"`
	FPSNum         *int   `json:"fps_num,omitempty"`
	FPSDen         *int   `json:"fps_den,omitempty"`
	SampleRate     *int   `json:"sample_rate,omitempty"`
	Channels       *int   `json:"channels,omitempty"`
	ChannelLayout  string `json:"channel_layout,omitempty"`
	PixelFormat    string `json:"pixel_format,omitempty"`
	ColorSpace     string `json:"color_space,omitempty"`
	ColorPrimaries string `json:"color_primaries,omitempty"`
	ColorTransfer  string `json:"color_transfer,omitempty"`
}
type RenderManifestSettings struct {
	Width             int    `json:"width"`
	Height            int    `json:"height"`
	FPS               int    `json:"fps"`
	RangeStartFrame   int64  `json:"range_start_frame"`
	RangeEndFrame     int64  `json:"range_end_frame"`
	BurnInCaptions    bool   `json:"burn_in_captions"`
	AudioSampleRate   int    `json:"audio_sample_rate"`
	AudioChannels     int    `json:"audio_channels"`
	WorkingColorSpace string `json:"working_color_space,omitempty"`
	OutputContainer   string `json:"output_container,omitempty"`
	VideoCodec        string `json:"video_codec,omitempty"`
	AudioCodec        string `json:"audio_codec,omitempty"`
}
