import { describe, expect, it } from 'vitest';
import { nextGitHubPollDelayMs, shouldContinueGitHubPolling } from './githubSettingsApi';

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
