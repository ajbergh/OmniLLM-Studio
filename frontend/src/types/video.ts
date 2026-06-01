export type VideoProviderKey = 'openrouter' | 'gemini' | 'openai' | 'custom';

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
  font_size?: number;
  font_weight?: string;
  color?: string;
  background?: string;
  stroke?: string;
  shadow?: boolean;
  params?: Record<string, unknown>;
}

export interface VideoTimelineEffect {
  id: string;
  type: 'blur' | 'brightness' | 'contrast' | 'saturation' | 'grayscale' | 'shadow' | 'background_blur' | 'chroma_key';
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
  easing?: 'linear' | 'ease-in' | 'ease-out' | 'ease-in-out';
}

export interface VideoTimelineClip {
  id: string;
  asset_id?: string;
  start_ms: number;
  duration_ms: number;
  trim_in_ms: number;
  trim_out_ms: number;
  transform?: VideoTimelineTransform;
  volume?: number;
  fade_in_ms?: number;
  fade_out_ms?: number;
  text?: VideoTimelineText;
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
  resolution: '720p' | '1080p' | 'project';
  fps?: number;
  quality?: 'draft' | 'standard' | 'high';
  include_audio: boolean;
  register_in_file_library?: boolean;
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
  created_at: string;
  started_at?: string;
  completed_at?: string;
}

export interface VideoAssistantRequest {
  prompt?: string;
  instruction?: string;
  timeline?: VideoTimelineDocument;
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
  type: 'set_canvas' | 'set_duration' | 'trim_clip' | 'add_text_clip' | string;
  clip_id?: string;
  track_id?: string;
  start_ms?: number;
  duration_ms?: number;
  text?: string;
  width?: number;
  height?: number;
  fps?: number;
}

export interface VideoEditPlan {
  summary: string;
  operations: VideoEditOperation[];
}

export interface VideoSocialVariant {
  name: string;
  aspect_ratio: string;
  width: number;
  height: number;
  plan: VideoEditPlan;
}
