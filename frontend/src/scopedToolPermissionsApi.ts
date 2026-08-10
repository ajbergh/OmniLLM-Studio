import { getAuthToken, resolveApiUrl } from './api';

export type ScopedToolPolicy = 'allow' | 'ask' | 'deny';
export type ToolScopeType = 'user' | 'workspace' | 'conversation';

export interface ScopedToolPermission {
  scope_type: ToolScopeType;
  scope_id: string;
  tool_name: string;
  policy: ScopedToolPolicy;
  updated_at?: string;
}

function headers(): Record<string, string> {
  const out: Record<string, string> = { 'Content-Type': 'application/json' };
  const token = getAuthToken();
  if (token) out.Authorization = `Bearer ${token}`;
  return out;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(resolveApiUrl(`/v1${path}`), {
    ...init,
    headers: { ...headers(), ...(init?.headers || {}) },
    credentials: 'include',
  });
  if (response.status === 204) return undefined as T;
  const body = await response.json().catch(() => ({ error: response.statusText }));
  if (!response.ok) throw new Error(body.error || `Request failed (${response.status})`);
  return body as T;
}

export const scopedToolPermissionsApi = {
  list: (scopeType: ToolScopeType, scopeId: string) => request<ScopedToolPermission[]>(`/tools/scoped-permissions?scope_type=${encodeURIComponent(scopeType)}&scope_id=${encodeURIComponent(scopeId)}`),
  upsert: (permission: ScopedToolPermission) => request<ScopedToolPermission[]>('/tools/scoped-permissions', { method: 'PUT', body: JSON.stringify(permission) }),
  delete: (permission: Pick<ScopedToolPermission, 'scope_type' | 'scope_id' | 'tool_name'>) => request<void>('/tools/scoped-permissions', { method: 'DELETE', body: JSON.stringify({ ...permission, policy: 'allow' }) }),
};
