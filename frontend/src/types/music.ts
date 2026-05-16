export type MusicProviderKey = 'openrouter' | 'gemini' | 'elevenlabs';

export type MusicCapability = 'text_to_music';

export interface MusicModel {
  id: string;
  provider: MusicProviderKey;
  name: string;
  capabilities: MusicCapability[];
  input_modalities?: string[];
  output_modalities?: string[];
  supported_formats?: string[];
  supports_streaming: boolean;
  default_output_format?: string;
  pricing?: Record<string, string>;
  notes?: string;
}

export interface MusicProvidersResponse {
  openrouter: boolean;
  gemini: boolean;
  elevenlabs: boolean;
}

export interface MusicOptions {
  genre?: string;
  mood?: string;
  era?: string;
  instruments?: string[];
  bpm?: number;
  scale?: string;
  duration?: string;
  structure?: string;
  language?: string;
  energy_curve?: string;
  production_notes?: string;
  negative_steer?: string;
  seed?: number;
  temperature?: number;
}

export interface MusicPromptForm {
  prompt: string;
  lyrics: string;
  instrumental: boolean;
  vocal_mode: 'auto' | 'instrumental' | 'lyrics' | 'custom';
  options: MusicOptions;
}

export interface GenerateMusicRequest {
  provider: MusicProviderKey;
  model: string;
  prompt: string;
  lyrics?: string;
  instrumental?: boolean;
  vocal_mode?: string;
  options?: MusicOptions;
  session_id?: string;
  parent_id?: string;
  title?: string;
  enhance?: boolean;
}

export interface MusicSession {
  id: string;
  user_id?: string;
  title: string;
  active_generation_id?: string;
  default_provider?: MusicProviderKey;
  default_model?: string;
  metadata_json?: string;
  created_at: string;
  updated_at: string;
}

export interface MusicGenerationDetail {
  id: string;
  session_id: string;
  parent_id?: string;
  title: string;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled' | string;
  provider: MusicProviderKey;
  model: string;
  prompt: string;
  assembled_prompt: string;
  lyrics?: string;
  structure?: string;
  error?: string;
  asset_id?: string;
  asset_url?: string;
  mime_type?: string;
  cost_usd?: number;
  duration_ms?: number;
  output_bytes: number;
  created_at: string;
  completed_at?: string;
}

export interface MusicAsset {
  id: string;
  session_id: string;
  generation_id?: string;
  kind: string;
  file_name: string;
  file_path: string;
  mime_type: string;
  size_bytes: number;
  duration_ms?: number;
  sample_rate_hz?: number;
  channels?: number;
  provider: MusicProviderKey;
  model: string;
  metadata_json?: string;
  created_at: string;
}

export interface MusicSessionDetail {
  session: MusicSession;
  generations: MusicGenerationDetail[];
}

export interface MusicGenerationProgress {
  stage: string;
  message: string;
  session_id?: string;
  generation_id?: string;
}

export interface MusicGenerationDone {
  session: MusicSession;
  generation: MusicGenerationDetail;
  asset?: MusicAsset;
}

export interface MusicGenerationError {
  error: string;
  status?: number;
  session_id?: string;
  generation_id?: string;
}
