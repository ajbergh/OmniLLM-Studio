import { getAuthToken, resolveApiUrl } from './api';

export type SandboxMountMode = 'read_only' | 'read_write_no_delete' | 'read_write';

export interface SandboxRuntimeCapabilities {
  name: string;
  version?: string;
  os_isolation: boolean;
  filesystem_isolation: boolean;
  network_isolation: boolean;
  network_allowlist: boolean;
  process_tree_isolation: boolean;
  memory_limit: boolean;
  cpu_limit: boolean;
  pid_limit: boolean;
  disk_limit: boolean;
}

export interface SandboxRuntimeStatus {
  configured: boolean;
  capabilities: SandboxRuntimeCapabilities;
  extension_sandbox_mode: string;
  path_grants_configured: boolean;
  path_grant_available_here: boolean;
}

export interface SandboxWorkspace {
  id: string;
  mode: SandboxMountMode;
  created_at: string;
  updated_at: string;
}

export interface SandboxWorkspaceChange {
  id: string;
  workspace_id: string;
  relative_path: string;
  operation: string;
  before_exists: boolean;
  before_sha256?: string;
  after_exists: boolean;
  after_sha256?: string;
  revertable: boolean;
  created_at: string;
}

async function sandboxFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = getAuthToken();
  const response = await fetch(resolveApiUrl(path), {
    ...init,
    credentials: 'include',
    headers: {
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...init.headers,
    },
  });

  if (!response.ok) {
    let message = `Sandbox request failed (${response.status})`;
    try {
      const payload = (await response.json()) as { error?: string };
      if (payload.error) message = payload.error;
    } catch {
      // Keep the status-based message when the response is not JSON.
    }
    throw new Error(message);
  }

  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

export const sandboxApi = {
  status: () => sandboxFetch<SandboxRuntimeStatus>('/v1/sandbox/status'),
  workspaces: () => sandboxFetch<SandboxWorkspace[]>('/v1/sandbox/workspaces'),
  createWorkspace: (input: { id: string; root_path: string; mode: SandboxMountMode }) =>
    sandboxFetch<SandboxWorkspace>('/v1/sandbox/workspaces', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  removeWorkspace: (id: string) =>
    sandboxFetch<void>(`/v1/sandbox/workspaces/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  workspaceChanges: (id: string, limit = 30) =>
    sandboxFetch<SandboxWorkspaceChange[]>(
      `/v1/sandbox/workspaces/${encodeURIComponent(id)}/changes?limit=${Math.max(1, Math.min(limit, 200))}`,
    ),
};
