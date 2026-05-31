export type VideoProviderKey = 'mock' | 'openrouter' | 'gemini' | 'openai' | 'custom';

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
  | 'audio_generation';

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

export interface VideoProviderInfo {
  key: VideoProviderKey;
  display_name: string;
  configured: boolean;
  mock: boolean;
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
  output_asset_id?: string;
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
  enhance: boolean;
  place_on_timeline: boolean;
}

export interface GenerateVideoRequest extends VideoPromptForm {
  provider: VideoProviderKey;
  model: string;
  project_id?: string;
  parent_id?: string;
  title?: string;
  enhanced_prompt?: string;
  reference_asset_ids?: string[];
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
