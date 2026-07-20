import { describe, expect, it } from 'vitest';
import { analyzeTimeline } from './timelineAnalysis';
import type { VideoAsset, VideoTimelineClip, VideoTimelineDocument, VideoTimelineTrack } from '../../../types/video';

function clip(id: string, start: number, duration: number, extra: Partial<VideoTimelineClip> = {}): VideoTimelineClip {
  return {
    id,
    start_ms: start,
    duration_ms: duration,
    trim_in_ms: 0,
    trim_out_ms: duration,
    effects: [],
    transitions: [],
    keyframes: [],
    ...extra,
  };
}

function track(id: string, clips: VideoTimelineClip[], type: VideoTimelineTrack['type'] = 'layer'): VideoTimelineTrack {
  return { id, name: id, type, locked: false, muted: false, visible: true, clips };
}

function document(tracks: VideoTimelineTrack[]): VideoTimelineDocument {
  return {
    version: 1,
    canvas: { width: 1920, height: 1080, fps: 30, background: '#000000' },
    duration_ms: 10_000,
    tracks,
    markers: [],
    metadata: {},
  };
}

const asset: VideoAsset = {
  id: 'asset-1',
  source_type: 'upload',
  kind: 'video',
  file_name: 'source.mp4',
  file_path: 'video/source.mp4',
  mime_type: 'video/mp4',
  size_bytes: 100,
  duration_ms: 10_000,
  created_at: '2026-07-20T00:00:00Z',
};

describe('analyzeTimeline', () => {
  it('counts timeline complexity and simultaneous visual layers', () => {
    const result = analyzeTimeline(document([
      track('a', [clip('a1', 0, 5_000, { asset_id: asset.id })]),
      track('b', [clip('b1', 1_000, 5_000, { asset_id: asset.id })]),
      track('c', [clip('c1', 2_000, 2_000, { text: { text: 'Title' } })]),
    ]), [asset], null, 3);

    expect(result.metrics.clips).toBe(3);
    expect(result.metrics.max_visual_overlap).toBe(3);
    expect(result.metrics.estimated_undo_bytes).toBe(result.metrics.estimated_document_bytes * 3);
    expect(result.metrics.complexity_score).toBeGreaterThan(0);
  });

  it('reports caption readability problems', () => {
    const result = analyzeTimeline(document([
      track('captions', [clip('caption-1', 0, 500, {
        text: { text: 'This caption contains far too many characters to read comfortably in half a second.' },
      })], 'caption'),
    ]), []);

    expect(result.issues.some((issue) => issue.id.startsWith('caption-readability'))).toBe(true);
    expect(result.issues.some((issue) => issue.fix === 'format_captions')).toBe(true);
  });

  it('reports missing and unused media separately', () => {
    const result = analyzeTimeline(document([
      track('video', [clip('missing', 0, 1_000, { asset_id: 'missing-asset' })]),
    ]), [asset]);

    expect(result.issues.some((issue) => issue.id.startsWith('missing-asset'))).toBe(true);
    expect(result.metrics.unused_assets).toBe(1);
  });
});
