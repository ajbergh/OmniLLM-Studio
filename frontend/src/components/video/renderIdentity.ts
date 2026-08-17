import type { VideoRenderJob } from '../../types/video';

/**
 * Render jobs are stored newest-first. Selecting by creation order instead of
 * completion order prevents an older overlapping job that finishes late from
 * replacing the identity of a newer completed render.
 */
export function newestCompletedTimelineSHA256(jobs: VideoRenderJob[]): string | undefined {
  return jobs.find((job) => job.status === 'completed' && Boolean(job.timeline_sha256))?.timeline_sha256;
}
