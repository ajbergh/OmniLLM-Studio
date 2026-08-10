import { getAuthToken, resolveApiUrl } from './api';

export interface AssistantProfile {
  id?: string;
  owner_user_id?: string;
  workspace_id?: string;
  name: string;
  description: string;
  provider?: string;
  model?: string;
  system_prompt?: string;
  tool_names: string[];
  skill_ids: string[];
  created_at?: string;
  updated_at?: string;
}

export interface Skill {
  id?: string;
  owner_user_id?: string;
  workspace_id?: string;
  name: string;
  description: string;
  body_markdown?: string;
  enabled: boolean;
  created_at?: string;
  updated_at?: string;
}

function headers(): Record<string, string> {
  const h: Record<string, string> = { 'Content-Type': 'application/json' };
  const token = getAuthToken();
  if (token) h.Authorization = `Bearer ${token}`;
  return h;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(resolveApiUrl(`/v1${path}`), { ...init, headers: { ...headers(), ...(init?.headers || {}) }, credentials: 'include' });
  if (response.status === 204) return undefined as T;
  const body = await response.json().catch(() => ({ error: response.statusText }));
  if (!response.ok) throw new Error(body.error || `Request failed (${response.status})`);
  return body as T;
}

export const assistantProfilesApi = {
  listProfiles: () => request<AssistantProfile[]>('/assistant-profiles'),
  saveProfile: (profile: AssistantProfile) => request<AssistantProfile>('/assistant-profiles', { method: 'PUT', body: JSON.stringify(profile) }),
  deleteProfile: (id: string) => request<void>(`/assistant-profiles/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  listSkills: () => request<Skill[]>('/skills'),
  getSkill: (id: string) => request<Skill>(`/skills/${encodeURIComponent(id)}`),
  saveSkill: (skill: Skill) => request<Skill>('/skills', { method: 'PUT', body: JSON.stringify(skill) }),
  deleteSkill: (id: string) => request<void>(`/skills/${encodeURIComponent(id)}`, { method: 'DELETE' }),
};
