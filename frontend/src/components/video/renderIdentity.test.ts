import { describe, expect, it } from 'vitest';
import type { VideoRenderJob } from '../../types/video';
import { newestCompletedTimelineSHA256 } from './renderIdentity';

function job(id: string, status: VideoRenderJob['status'], hash?: string): VideoRenderJob {
  return { id, status, timeline_sha256: hash } as VideoRenderJob;
}

describe('newestCompletedTimelineSHA256', () => {
  it('uses the newest completed immutable render identity', () => {
    const jobs = [
      job('newer', 'completed', 'new-hash'),
      job('older', 'completed', 'old-hash'),
    ];
    expect(newestCompletedTimelineSHA256(jobs)).toBe('new-hash');
  });

  it('does not let an older late completion replace a newer completed job', () => {
    const jobs = [
      job('newer', 'completed', 'new-hash'),
      job('older-that-finished-last', 'completed', 'old-hash'),
    ];
    expect(newestCompletedTimelineSHA256(jobs)).toBe('new-hash');
  });

  it('skips running and legacy jobs without snapshot hashes', () => {
    const jobs = [
      job('new-running', 'running', 'running-hash'),
      job('legacy-completed', 'completed'),
      job('completed', 'completed', 'completed-hash'),
    ];
    expect(newestCompletedTimelineSHA256(jobs)).toBe('completed-hash');
  });
});
