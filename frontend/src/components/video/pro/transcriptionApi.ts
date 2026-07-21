import { getAuthToken, resolveApiUrl } from '../../../api';
import type { VideoTimelineClip } from '../../../types/video';

export interface TranscriptSegment {
  id: string;
  transcript_id: string;
  segment_index: number;
  start_ms: number;
  end_ms: number;
  text: string;
  speaker?: string;
  confidence?: number;
  words_json?: string;
}

export interface VideoTranscript {
  id: string;
  project_id: string;
  asset_id: string;
  provider_profile_id: string;
  provider: string;
  model: string;
  status: 'queued' | 'running' | 'completed' | 'failed' | string;
  language?: string;
  translated_language?: string;
  text?: string;
  cost_usd?: number;
  privacy_json?: string;
  metadata_json?: string;
  error?: string;
  created_at: string;
  updated_at?: string;
  completed_at?: string;
  segments?: TranscriptSegment[];
}

export interface StartTranscriptRequest {
  asset_id: string;
  provider_profile_id: string;
  model?: string;
  language?: string;
  translate_to?: string;
  prompt?: string;
  diarization?: boolean;
  word_timestamps?: boolean;
  allow_remote_processing: boolean;
  retain_provider_data?: boolean;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(init?.headers as Record<string, string>),
  };
  const token = getAuthToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  const response = await fetch(resolveApiUrl(`/v1${path}`), { ...init, headers });
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(body.error || `API error ${response.status}`);
  }
  return response.json() as Promise<T>;
}

export const transcriptionApi = {
  start: (projectId: string, payload: StartTranscriptRequest) =>
    request<VideoTranscript>(`/video/projects/${encodeURIComponent(projectId)}/transcriptions`, {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  list: (projectId: string) =>
    request<VideoTranscript[]>(`/video/projects/${encodeURIComponent(projectId)}/transcriptions`),
  get: (id: string) =>
    request<VideoTranscript>(`/video/transcriptions/${encodeURIComponent(id)}`),
  captions: (id: string) =>
    request<{ api_version: string; clips: VideoTimelineClip[] }>(
      `/video/transcriptions/${encodeURIComponent(id)}/captions`,
      { method: 'POST' },
    ),
};
