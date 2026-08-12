import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  getGitHubAuthStatus,
  listGitHubRepositories,
  nextGitHubPollDelayMs,
  shouldContinueGitHubPolling,
} from './githubSettingsApi';

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('GitHub settings polling helpers', () => {
  it('uses provider retry guidance when present', () => {
    expect(nextGitHubPollDelayMs({ retry_after_seconds: 9 }, 5)).toBe(9000);
  });

  it('bounds retry delays to safe service limits', () => {
    expect(nextGitHubPollDelayMs({ retry_after_seconds: 0 }, 5)).toBe(1000);
    expect(nextGitHubPollDelayMs({ retry_after_seconds: 600 }, 5)).toBe(60000);
  });

  it('falls back to the device authorization interval', () => {
    expect(nextGitHubPollDelayMs({}, 7)).toBe(7000);
  });

  it('continues only while GitHub reports pending', () => {
    expect(shouldContinueGitHubPolling('pending')).toBe(true);
    expect(shouldContinueGitHubPolling('connected')).toBe(false);
    expect(shouldContinueGitHubPolling('expired')).toBe(false);
    expect(shouldContinueGitHubPolling('denied')).toBe(false);
    expect(shouldContinueGitHubPolling('not_started')).toBe(false);
  });
});

describe('GitHub settings API routing', () => {
  it('keeps auth requests on the authenticated v1 API surface', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ configured: true, connected: false, pending: false }),
    });
    vi.stubGlobal('fetch', fetchMock);

    await getGitHubAuthStatus();

    expect(fetchMock).toHaveBeenCalledWith(
      '/v1/github/auth',
      expect.objectContaining({ credentials: 'include' }),
    );
  });

  it('keeps repository discovery on the authenticated v1 API surface', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ repositories: [], page: 2, per_page: 20, has_more: false }),
    });
    vi.stubGlobal('fetch', fetchMock);

    await listGitHubRepositories(2, 20);

    expect(fetchMock).toHaveBeenCalledWith(
      '/v1/github/repositories?page=2&per_page=20',
      expect.objectContaining({ credentials: 'include' }),
    );
  });
});
