import { beforeEach, describe, expect, it, vi } from 'vitest';

// sonner touches the DOM when toasts fire; the store calls it on success
// paths, so stub it out for node.
vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
}));

import { useVideoStudioStore } from './videoStudio';
import type { VideoTimelineClip, VideoTimelineDocument, VideoTimelineTrack } from '../types/video';

function makeClip(id: string, startMs: number, durationMs: number, extra: Partial<VideoTimelineClip> = {}): VideoTimelineClip {
  return {
    id,
    start_ms: startMs,
    duration_ms: durationMs,
    trim_in_ms: 0,
    trim_out_ms: durationMs,
    effects: [],
    keyframes: [],
    transitions: [],
    ...extra,
  };
}

function makeTrack(id: string, clips: VideoTimelineClip[] = [], extra: Partial<VideoTimelineTrack> = {}): VideoTimelineTrack {
  return { id, type: 'layer', name: id, locked: false, muted: false, visible: true, clips, ...extra };
}

function makeDoc(tracks: VideoTimelineTrack[]): VideoTimelineDocument {
  return {
    version: 1,
    canvas: { width: 1920, height: 1080, fps: 30, background: '#000000' },
    duration_ms: 30000,
    tracks,
    markers: [],
    metadata: {},
  };
}

const store = () => useVideoStudioStore.getState();

beforeEach(() => {
  useVideoStudioStore.setState({
    // No active project: saveTimeline no-ops, so actions stay local.
    activeProjectId: null,
    timeline: makeDoc([
      makeTrack('l1', [makeClip('c1', 0, 2000), makeClip('c2', 3000, 2000)]),
      makeTrack('l2', [makeClip('c3', 1000, 1000)]),
    ]),
    timelineUndoStack: [],
    timelineRedoStack: [],
    selectedClipId: null,
    selectedClipIds: [],
    selectedTrackId: null,
    clipClipboard: null,
    attributeClipboard: null,
    playheadMs: 0,
    soloTrackId: null,
  });
});

describe('selection', () => {
  it('selects every clip across all layers', () => {
    store().selectAllClips();
    expect(store().selectedClipIds.sort()).toEqual(['c1', 'c2', 'c3']);
  });

  it('selects clips relative to the playhead', () => {
    useVideoStudioStore.setState({ playheadMs: 2500 });
    store().selectClipsRelativeToPlayhead('after');
    expect(store().selectedClipIds).toEqual(['c2']);
    store().selectClipsRelativeToPlayhead('before');
    expect(store().selectedClipIds.sort()).toEqual(['c1', 'c3']);
  });

  it('selects all clips on a layer', () => {
    store().selectClipsOnTrack('l1');
    expect(store().selectedClipIds).toEqual(['c1', 'c2']);
    expect(store().selectedTrackId).toBe('l1');
  });
});

describe('clipboard', () => {
  it('pastes copied clips at the playhead preserving relative offsets', async () => {
    store().setSelectedClips(['c1', 'c2']);
    store().copySelection();
    expect(store().clipClipboard).toHaveLength(2);

    useVideoStudioStore.setState({ playheadMs: 10000 });
    await store().pasteClips();

    const l1 = store().timeline!.tracks.find((track) => track.id === 'l1')!;
    expect(l1.clips).toHaveLength(4);
    const starts = l1.clips.map((clip) => clip.start_ms).sort((a, b) => a - b);
    expect(starts).toEqual([0, 3000, 10000, 13000]);
    // Pasted clips get fresh ids.
    const ids = new Set(l1.clips.map((clip) => clip.id));
    expect(ids.size).toBe(4);
    expect(store().timelineUndoStack).toHaveLength(1);
  });

  it('cut removes the source clips after copying', async () => {
    store().setSelectedClips(['c3']);
    await store().cutSelection();
    expect(store().clipClipboard).toHaveLength(1);
    const l2 = store().timeline!.tracks.find((track) => track.id === 'l2')!;
    expect(l2.clips).toHaveLength(0);
  });
});

describe('layer operations', () => {
  it('duplicates a layer with fresh clip ids directly above the source', async () => {
    await store().duplicateTrack('l1');
    const tracks = store().timeline!.tracks;
    expect(tracks).toHaveLength(3);
    expect(tracks[1].name).toBe('l1 copy');
    expect(tracks[1].clips).toHaveLength(2);
    expect(tracks[1].clips.map((clip) => clip.id)).not.toContain('c1');
  });

  it('inserts a generic layer adjacent to a track', async () => {
    await store().insertTrackAdjacent('l1', 'above');
    const tracks = store().timeline!.tracks;
    expect(tracks).toHaveLength(3);
    expect(tracks[1].type).toBe('layer');
    expect(tracks[1].clips).toHaveLength(0);
  });

  it('clears a layer and prunes the selection', async () => {
    store().setSelectedClips(['c1', 'c3']);
    await store().clearTrack('l1');
    expect(store().timeline!.tracks[0].clips).toHaveLength(0);
    expect(store().selectedClipIds).toEqual(['c3']);
  });
});

describe('audio workflows', () => {
  it('toggles clip mute without touching volume', async () => {
    await store().toggleClipMute('c1');
    let c1 = store().timeline!.tracks[0].clips.find((clip) => clip.id === 'c1')!;
    expect(c1.muted).toBe(true);
    await store().toggleClipMute('c1');
    c1 = store().timeline!.tracks[0].clips.find((clip) => clip.id === 'c1')!;
    expect(c1.muted).toBe(false);
  });

  it('detaches audio into an audio-only twin on a new layer below', async () => {
    await store().detachClipAudio('c1');
    const tracks = store().timeline!.tracks;
    expect(tracks).toHaveLength(3);
    const detached = tracks[0];
    expect(detached.name).toBe('Detached audio');
    expect(detached.clips).toHaveLength(1);
    expect(detached.clips[0].audio_only).toBe(true);
    expect(detached.clips[0].muted).toBe(false);
    const original = tracks[1].clips.find((clip) => clip.id === 'c1')!;
    expect(original.muted).toBe(true);
  });
});

describe('split and undo', () => {
  it('splits every clip under the playhead', async () => {
    useVideoStudioStore.setState({ playheadMs: 1500 });
    await store().splitAllAtPlayhead();
    const [l1, l2] = store().timeline!.tracks;
    expect(l1.clips).toHaveLength(3); // c1 split, c2 untouched
    expect(l2.clips).toHaveLength(2); // c3 split
    const c1 = l1.clips.find((clip) => clip.id === 'c1')!;
    expect(c1.duration_ms).toBe(1500);
  });

  it('undo restores the previous document', async () => {
    await store().clearTrack('l1');
    expect(store().timeline!.tracks[0].clips).toHaveLength(0);
    await store().undoTimeline();
    expect(store().timeline!.tracks[0].clips).toHaveLength(2);
  });
});
