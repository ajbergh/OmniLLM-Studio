import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
}));

import { useVideoStudioStore } from '../../../stores/videoStudio';
import type { VideoAsset, VideoTimelineDocument } from '../../../types/video';
import { projectMediaReferences, replaceTimelineAsset } from './mediaTools';

const asset = (id: string, name: string): VideoAsset => ({
  id,
  source_type: 'upload',
  kind: 'video',
  file_name: name,
  file_path: `video/${name}`,
  mime_type: 'video/mp4',
  size_bytes: 100,
  duration_ms: 5_000,
  created_at: '2026-07-20T00:00:00Z',
});

const timeline = (): VideoTimelineDocument => ({
  version: 1,
  canvas: { width: 1920, height: 1080, fps: 30, background: '#000000' },
  duration_ms: 5_000,
  tracks: [{
    id: 'layer-1',
    type: 'layer',
    name: 'Layer 1',
    locked: false,
    muted: false,
    visible: true,
    clips: [{
      id: 'clip-1',
      asset_id: 'old',
      start_ms: 0,
      duration_ms: 5_000,
      trim_in_ms: 0,
      trim_out_ms: 5_000,
      effects: [],
      transitions: [],
      keyframes: [],
    }],
  }],
  markers: [],
  metadata: {},
});

beforeEach(() => {
  useVideoStudioStore.setState({
    activeProjectId: null,
    timeline: timeline(),
    assets: [asset('new', 'replacement.mp4')],
    timelineUndoStack: [],
    timelineRedoStack: [],
    selectedClipId: null,
    selectedClipIds: [],
    selectedTrackId: null,
  });
});

describe('projectMediaReferences', () => {
  it('reports missing media and reference counts', () => {
    const references = projectMediaReferences(timeline(), [asset('new', 'replacement.mp4')]);
    expect(references).toHaveLength(1);
    expect(references[0]).toMatchObject({ asset_id: 'old', clip_count: 1, missing: true });
  });
});

describe('replaceTimelineAsset', () => {
  it('relinks clips while preserving timing and creates undo history', async () => {
    await replaceTimelineAsset('old', 'new');
    const state = useVideoStudioStore.getState();
    const clip = state.timeline!.tracks[0].clips[0];
    expect(clip.asset_id).toBe('new');
    expect(clip.start_ms).toBe(0);
    expect(clip.duration_ms).toBe(5_000);
    expect(state.timelineUndoStack).toHaveLength(1);
    expect(state.selectedClipId).toBe('clip-1');
  });
});
