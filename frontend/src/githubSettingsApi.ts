import { getAuthToken, resolveApiUrl } from './api';

export interface GitHubAuthStatus {
  configured: boolean;
  connected: boolean;
  pending: boolean;
  github_user_id?: number;
  github_login?: string;
  expires_at?: string;
}

export interface GitHubDeviceAuthorization {
  user_code: string;
  verification_uri: string;
  expires_at: string;
  interval_seconds: number;
}

export interface GitHubDevicePollResult {
  status: 'pending' | 'connected' | 'expired' | 'denied' | 'not_started' | string;
  retry_after_seconds?: number;
  github_login?: string;
}

export interface GitHubRepositoryPermissions {
  admin: boolean;
  maintain: boolean;
  push: boolean;
  triage: boolean;
  pull: boolean;
}

export interface GitHubRepository {
  id: number;
  name: string;
  full_name: string;
  private: boolean;
  fork: boolean;
  archived: boolean;
  disabled: boolean;
  default_branch: string;
  permissions: GitHubRepositoryPermissions;
}

export interface GitHubRepositoryPage {
  repositories: GitHubRepository[];
  page: number;
  per_page: number;
  has_more: boolean;
}

export interface GitHubRepositoryBinding {
  local_repository_id: string;
  github_user_id: number;
  github_repository_id: number;
  github_full_name: string;
  default_branch: string;
  private: boolean;
  fork: boolean;
  archived: boolean;
  disabled: boolean;
  account_matches: boolean;
  local_configured: boolean;
}

export interface GitHubRepositoryBindingsResponse {
  local_repositories: string[];
  bindings: GitHubRepositoryBinding[];
}

async function githubRequest<T>(
  path: string,
  init: RequestInit = {},
  signal?: AbortSignal,
): Promise<T> {
  const token = getAuthToken();
  const headers = new Headers(init.headers);
  if (token) headers.set('Authorization', `Bearer ${token}`);
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');

  const response = await fetch(resolveApiUrl(path), {
    ...init,
    headers,
    credentials: 'include',
    signal,
  });
  if (!response.ok) {
    let message = `Request failed (${response.status})`;
    try {
      const body = (await response.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // Keep the bounded status fallback when the backend returned no JSON body.
    }
    throw new Error(message);
  }
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

export function getGitHubAuthStatus(signal?: AbortSignal): Promise<GitHubAuthStatus> {
  return githubRequest('/github/auth', {}, signal);
}

export function startGitHubDeviceAuthorization(signal?: AbortSignal): Promise<GitHubDeviceAuthorization> {
  return githubRequest('/github/auth/device/start', { method: 'POST' }, signal);
}

export function pollGitHubDeviceAuthorization(signal?: AbortSignal): Promise<GitHubDevicePollResult> {
  return githubRequest('/github/auth/device/poll', { method: 'POST' }, signal);
}

export function disconnectGitHub(signal?: AbortSignal): Promise<void> {
  return githubRequest('/github/auth', { method: 'DELETE' }, signal);
}

export function listGitHubRepositories(
  page = 1,
  perPage = 30,
  signal?: AbortSignal,
): Promise<GitHubRepositoryPage> {
  const params = new URLSearchParams({ page: String(page), per_page: String(perPage) });
  return githubRequest(`/github/repositories?${params.toString()}`, {}, signal);
}

export function getGitHubRepositoryBindings(signal?: AbortSignal): Promise<GitHubRepositoryBindingsResponse> {
  return githubRequest('/github/repository-bindings', {}, signal);
}

export function bindGitHubRepository(
  localRepositoryId: string,
  githubRepositoryId: number,
  signal?: AbortSignal,
): Promise<GitHubRepositoryBinding> {
  return githubRequest(
    `/github/repository-bindings/${encodeURIComponent(localRepositoryId)}`,
    { method: 'PUT', body: JSON.stringify({ github_repository_id: githubRepositoryId }) },
    signal,
  );
}

export function deleteGitHubRepositoryBinding(localRepositoryId: string, signal?: AbortSignal): Promise<void> {
  return githubRequest(
    `/github/repository-bindings/${encodeURIComponent(localRepositoryId)}`,
    { method: 'DELETE' },
    signal,
  );
}

export function nextGitHubPollDelayMs(
  result: Pick<GitHubDevicePollResult, 'retry_after_seconds'>,
  fallbackSeconds: number,
): number {
  const seconds = result.retry_after_seconds ?? fallbackSeconds;
  const boundedSeconds = Math.min(60, Math.max(1, Number.isFinite(seconds) ? seconds : fallbackSeconds));
  return boundedSeconds * 1000;
}

export function shouldContinueGitHubPolling(status: GitHubDevicePollResult['status']): boolean {
  return status === 'pending';
}
