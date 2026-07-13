export type VideoProviderKey = 'openrouter' | 'gemini' | 'luma' | 'openai' | 'custom';

export type VideoCapability =
  | 'text_to_video'
  | 'image_to_video'
  | 'video_to_video'
  | 'extend_video'
  | 'reference_images'
  | 'reference_video'
  | 'negative_prompt'
  | 'seed'
  | 'camera_motion'
  | 'audio_generation'
  | 'first_last_frame'
  | 'person_generation';

export interface VideoModel {
  id: string;
  provider: VideoProviderKey;
  name: string;
  capabilities: VideoCapability[];
  aspect_ratios?: string[];
  resolutions?: string[];
  duration_min_seconds?: number;
  duration_max_seconds?: number;
  fps_options?: number[];
  max_prompt_chars?: number;
  max_reference_images?: number;
  notes?: string;
}

export interface VideoGenerationValidationIssue {
  code: string;
  field?: string;
  message: string;
  severity: 'error' | 'warning' | 'normalization';
  original?: unknown;
  normalized?: unknown;
}

export interface VideoGenerationValidationResult {
  valid: boolean;
  provider?: VideoProviderKey;
  model?: string;
  capabilities?: VideoCapability[];
  normalized_request: GenerateVideoRequest;
  errors: VideoGenerationValidationIssue[];
  warnings: VideoGenerationValidationIssue[];
  normalizations: VideoGenerationValidationIssue[];
}

export interface VideoProviderInfo {
  key: VideoProviderKey;
  display_name: string;
  configured: boolean;
  models?: VideoModel[];
}

export interface VideoProject {
  id: string;
  user_id?: string;
  title: string;
  active_timeline_id?: string;
  default_provider?: VideoProviderKey;
  default_model?: string;
  width: number;
  height: number;
  fps: number;
  duration_ms: number;
  aspect_ratio: string;
  metadata_json?: string;
  created_at: string;
  updated_at: string;
}

export interface VideoAsset {
  id: string;
  project_id?: string;
  source_type: string;
  source_studio?: string;
  source_id?: string;
  kind: 'video' | 'image' | 'audio' | 'music' | 'text' | 'caption' | 'export' | 'other';
  file_name: string;
  file_path: string;
  mime_type: string;
  size_bytes: number;
  duration_ms?: number;
  width?: number;
  height?: number;
  fps?: number;
  thumbnail_path?: string;
  waveform_path?: string;
  provider?: VideoProviderKey;
  model?: string;
  metadata_json?: string;
  created_at: string;
}

export interface VideoGenerationDetail {
  id: string;
  project_id: string;
  parent_id?: string;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled' | string;
  provider: VideoProviderKey;
  model: string;
  prompt: string;
  enhanced_prompt?: string;
  negative_prompt?: string;
  settings_json?: string;
  input_asset_ids_json?: string;
  input_assets_json?: string;
  output_asset_id?: string;
  upstream_job_id?: string;
  upstream_request_id?: string;
  usage_json?: string;
  asset_url?: string;
  mime_type?: string;
  cost_usd?: number;
  error?: string;
  created_at: string;
  completed_at?: string;
}

export interface VideoProjectDetail {
  project: VideoProject;
  generations: VideoGenerationDetail[];
  assets: VideoAsset[];
}

export interface VideoPromptForm {
  prompt: string;
  generation_mode?: 'text_to_video' | 'image_to_video' | 'reference_to_video' | 'edit';
  /** Local generation ID whose stored Gemini interaction should be continued. */
  parent_generation_id?: string;
  negative_prompt: string;
  aspect_ratio: string;
  duration_seconds: number;
  resolution: string;
  fps: number;
  seed?: number;
  camera_motion: string;
  shot_type: string;
  style_preset: string;
  production_notes: string;
  // Cinematic detail fields (assembled into the prompt at generation time)
  composition?: string;
  lens_effect?: string;
  lighting?: string;
  dialogue?: string;
  sound_effects?: string;
  ambient_noise?: string;
  continuity_notes?: string;
  enhance: boolean;
  place_on_timeline: boolean;
  start_image_asset_id?: string;
  last_frame_asset_id?: string;
  source_video_asset_id?: string;
  person_generation?: 'allow' | 'dont_allow';
  reference_asset_ids?: string[];
}

export interface InputAsset {
  asset_id: string;
  role: 'start_frame' | 'last_frame' | 'reference_image' | 'source_video';
}

export interface GenerateVideoRequest extends VideoPromptForm {
  provider: VideoProviderKey;
  model: string;
  project_id?: string;
  parent_id?: string;
  title?: string;
  enhanced_prompt?: string;
  reference_asset_ids?: string[];
  start_image_asset_id?: string;
  last_frame_asset_id?: string;
  source_video_asset_id?: string;
  person_generation?: 'allow' | 'dont_allow';
}

export interface VideoGenerationProgress {
  stage: string;
  message: string;
  project_id?: string;
  generation_id?: string;
  progress?: number;
}

export interface VideoGenerationDone {
  project: VideoProject;
  generation: VideoGenerationDetail;
  asset?: VideoAsset;
}

export interface VideoGenerationError {
  error: string;
  status?: number;
  project_id?: string;
  generation_id?: string;
}

export type VideoTimelineTrackType =
  // Generic ordered layer — accepts any clip kind; media behavior comes from
  // the clip and asset. Later tracks in the array stack on top.
  | 'layer'
  | 'video'
  | 'image'
  | 'audio'
  | 'music'
  | 'text'
  | 'caption'
  | 'shape'
  | 'callout';

export interface VideoTimelineCanvas {
  width: number;
  height: number;
  fps: number;
  background: string;
}

export interface VideoTimelineTransform {
  x: number;
  y: number;
  scale: number;
  rotation: number;
  opacity: number;
  crop?: { top: number; right: number; bottom: number; left: number };
}

export interface VideoTimelineText {
  text: string;
  font_family?: string;
  font_size?: number;
  font_weight?: string;
  color?: string;
  background?: string;
  stroke?: string;
  stroke_width?: number;
  shadow?: boolean;
  text_align?: 'left' | 'center' | 'right';
  line_height?: number;
  letter_spacing?: number;
  border_radius?: number;
  params?: Record<string, unknown>;
}

export type VideoTimelineShapeKind =
  | 'rectangle'
  | 'highlight'
  | 'blur'
  // Annotation kinds. blur/pixelate/rectangle/highlight/rounded_rectangle/label
  // export (rounded corners flatten to square); the rest are preview-only.
  | 'rounded_rectangle'
  | 'ellipse'
  | 'arrow'
  | 'line'
  | 'speech_bubble'
  | 'spotlight'
  | 'pixelate'
  | 'checkmark'
  | 'x_mark'
  | 'step_marker'
  | 'label';

/** Parameterized callout/annotation box; dimensions in canvas pixels, position via the clip transform. */
export interface VideoTimelineShape {
  kind: VideoTimelineShapeKind;
  width?: number;
  height?: number;
  fill?: string;
  stroke?: string;
  stroke_width?: number;
  /** Blur radius or pixelate block size (1–50, default 12). */
  blur_radius?: number;
  /** Corner rounding for rounded rectangles / speech bubbles / labels (0–200, preview-only at export). */
  corner_radius?: number;
}

/** Sampled cursor position (canvas px from top-left), clip-relative in time. */
export interface VideoTimelineCursorEvent {
  time_ms: number;
  x: number;
  y: number;
  click?: boolean;
}

/** Cursor metadata captured with screen recordings; persisted, preview-only at export. */
export interface VideoTimelineCursor {
  visible?: boolean;
  scale?: number;
  highlight?: boolean;
  click_rings?: boolean;
  smoothing?: boolean;
  events?: VideoTimelineCursorEvent[];
}

export interface VideoTimelineEffect {
  id: string;
  type:
    | 'blur'
    | 'brightness'
    | 'contrast'
    | 'saturation'
    | 'grayscale'
    | 'shadow'
    | 'background_blur'
    | 'chroma_key'
    | 'sharpen'
    | 'vignette';
  enabled: boolean;
  params: Record<string, unknown>;
}

export interface VideoTimelineTransition {
  id: string;
  type: 'fade' | 'crossfade' | 'dip_to_black' | 'slide' | 'wipe' | 'zoom';
  duration_ms: number;
  direction?: 'left' | 'right' | 'up' | 'down';
}

export interface VideoTimelineKeyframe {
  id: string;
  property: 'x' | 'y' | 'scale' | 'rotation' | 'opacity' | 'volume';
  time_ms: number;
  value: number;
  easing?: 'linear' | 'ease-in' | 'ease-out' | 'ease-in-out' | 'step';
}

export interface VideoTimelineClip {
  id: string;
  asset_id?: string;
  start_ms: number;
  duration_ms: number;
  trim_in_ms: number;
  trim_out_ms: number;
  /**
   * Constant source playback rate. Timeline duration stays in output time;
   * the consumed source window is duration_ms * playback_rate.
   */
  playback_rate?: number;
  z_index?: number;
  group_id?: string;
  /** Silences this clip's audio without touching volume. */
  muted?: boolean;
  /** Suppresses visuals so a video asset acts as detached audio. */
  audio_only?: boolean;
  transform?: VideoTimelineTransform;
  volume?: number;
  fade_in_ms?: number;
  fade_out_ms?: number;
  text?: VideoTimelineText;
  shape?: VideoTimelineShape;
  cursor?: VideoTimelineCursor;
  effects: VideoTimelineEffect[];
  transitions?: VideoTimelineTransition[];
  keyframes: VideoTimelineKeyframe[];
}

export interface VideoTimelineTrack {
  id: string;
  type: VideoTimelineTrackType;
  name: string;
  locked: boolean;
  muted: boolean;
  visible: boolean;
  height?: number;
  clips: VideoTimelineClip[];
}

export interface VideoTimelineMarker {
  id: string;
  time_ms: number;
  label: string;
}

export interface VideoTimelineDocument {
  version: number;
  canvas: VideoTimelineCanvas;
  duration_ms: number;
  tracks: VideoTimelineTrack[];
  markers: VideoTimelineMarker[];
  metadata: Record<string, unknown>;
}

export interface VideoTimelineRecord {
  id: string;
  project_id: string;
  name: string;
  active: boolean;
  timeline_json: string;
  duration_ms: number;
  created_at: string;
  updated_at: string;
}

export interface VideoTimelineDetail {
  timeline: VideoTimelineRecord;
  document: VideoTimelineDocument;
}

export interface VideoExportSettings {
  format: 'mp4' | 'webm';
  codec?: 'h264' | 'h265' | 'vp9';
  resolution: '720p' | '1080p' | 'project' | 'custom';
  preset?: string;
  width?: number;
  height?: number;
  fps?: number;
  quality?: 'draft' | 'standard' | 'high';
  include_audio: boolean;
  register_in_file_library?: boolean;
  estimated_duration_ms?: number;
  /** Draw caption-track text into the frame (default true when omitted). */
  burn_in_captions?: boolean;
  /** Also write captions as a sibling asset: '', 'srt', or 'vtt'. */
  sidecar_captions?: '' | 'srt' | 'vtt';
  /** Export only this timeline window when end > start. */
  range_start_ms?: number;
  range_end_ms?: number;
  /** Audio bitrate override in kbps (32–512). */
  audio_bitrate_kbps?: number;
}

export interface VideoRenderJob {
  id: string;
  project_id: string;
  timeline_id: string;
  status: 'queued' | 'running' | 'completed' | 'failed' | 'cancelled' | string;
  progress: number;
  settings_json: string;
  output_asset_id?: string;
  error?: string;
  metadata_json?: string;
  created_at: string;
  started_at?: string;
  completed_at?: string;
}

export interface VideoRendererFeatureSupport {
  feature: string;
  label: string;
  supported: boolean;
  partial?: boolean;
  notes?: string;
}

export interface VideoRendererCapabilities {
  renderer: string;
  formats: string[];
  features: VideoRendererFeatureSupport[];
}

export interface VideoAssistantRequest {
  prompt?: string;
  instruction?: string;
  timeline?: VideoTimelineDocument;
  selected_clip_id?: string;
  playhead_ms?: number;
}

export interface VideoStoryboardScene {
  id: string;
  title: string;
  description: string;
  duration_ms: number;
  prompt: string;
}

export interface VideoStoryboardResponse {
  title: string;
  scenes: VideoStoryboardScene[];
  shot_list: string[];
  script: string;
  prompt_seed: string;
}

export interface VideoEditOperation {
  type: 'set_canvas' | 'set_duration' | 'trim_clip' | 'add_text_clip' | 'move_clip' | 'delete_clip' | 'set_volume' | 'add_marker' | 'add_asset_clip' | 'set_transform' | string;
  clip_id?: string;
  track_id?: string;
  asset_id?: string;
  start_ms?: number;
  duration_ms?: number;
  text?: string;
  width?: number;
  height?: number;
  fps?: number;
  volume?: number;
  x?: number;
  y?: number;
  scale?: number;
  opacity?: number;
}

export interface VideoEditPlan {
  summary: string;
  operations: VideoEditOperation[];
  /** Human-readable per-operation descriptions for valid operations. */
  preview?: string[];
  /** Operations that failed validation against the current timeline (skipped on apply). */
  issues?: string[];
}

export interface VideoSocialVariant {
  name: string;
  aspect_ratio: string;
  width: number;
  height: number;
  plan: VideoEditPlan;
}
