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
    rippleEnabled: false,
    assets: [],
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

describe('ripple and gap operations', () => {
  it('ripple delete removes the clip and shifts later clips left on the same layer', async () => {
    await store().rippleDeleteClip('c1');
    const l1 = store().timeline!.tracks.find((track) => track.id === 'l1')!;
    expect(l1.clips.map((clip) => clip.id)).toEqual(['c2']);
    // c2 started at 3000; c1 was 2000ms long, so c2 lands at 1000.
    expect(l1.clips[0].start_ms).toBe(1000);
    // The other layer is untouched.
    const l2 = store().timeline!.tracks.find((track) => track.id === 'l2')!;
    expect(l2.clips[0].start_ms).toBe(1000);
    expect(store().timelineUndoStack).toHaveLength(1);
  });

  it('ripple delete respects locked layers', async () => {
    useVideoStudioStore.setState({
      timeline: makeDoc([makeTrack('l1', [makeClip('c1', 0, 2000), makeClip('c2', 3000, 2000)], { locked: true })]),
    });
    await store().rippleDeleteClip('c1');
    const l1 = store().timeline!.tracks[0];
    expect(l1.clips).toHaveLength(2);
    expect(store().timelineUndoStack).toHaveLength(0);
  });

  it('ripple delete of a multi-selection closes every removed span', async () => {
    useVideoStudioStore.setState({
      timeline: makeDoc([
        makeTrack('l1', [makeClip('a', 0, 1000), makeClip('b', 2000, 1000), makeClip('c', 4000, 1000), makeClip('d', 6000, 1000)]),
      ]),
    });
    store().setSelectedClips(['b', 'c']);
    await store().rippleDeleteClip();
    const l1 = store().timeline!.tracks[0];
    expect(l1.clips.map((clip) => clip.id)).toEqual(['a', 'd']);
    expect(l1.clips.find((clip) => clip.id === 'd')!.start_ms).toBe(4000);
  });

  it('removeGap closes the empty span and pulls later clips left', async () => {
    await store().removeGap('l1', 2000, 3000);
    const l1 = store().timeline!.tracks.find((track) => track.id === 'l1')!;
    expect(l1.clips.find((clip) => clip.id === 'c2')!.start_ms).toBe(2000);
    expect(l1.clips.find((clip) => clip.id === 'c1')!.start_ms).toBe(0);
  });

  it('removeAllGaps compacts a layer while keeping the first clip anchored', async () => {
    useVideoStudioStore.setState({
      timeline: makeDoc([
        makeTrack('l1', [makeClip('a', 500, 1000), makeClip('b', 3000, 1000), makeClip('c', 6000, 1000)]),
      ]),
    });
    await store().removeAllGaps('l1');
    const l1 = store().timeline!.tracks[0];
    expect(l1.clips.map((clip) => clip.start_ms)).toEqual([500, 1500, 2500]);
  });

  it('ripple trim end shifts later clips by the duration delta', async () => {
    // Shrink c1 from 2000ms to 1000ms — c2 should follow left by 1000.
    await store().rippleTrimClip('c1', 'end', 1000);
    const l1 = store().timeline!.tracks.find((track) => track.id === 'l1')!;
    const c1 = l1.clips.find((clip) => clip.id === 'c1')!;
    const c2 = l1.clips.find((clip) => clip.id === 'c2')!;
    expect(c1.duration_ms).toBe(1000);
    expect(c1.trim_out_ms).toBe(1000);
    expect(c2.start_ms).toBe(2000);
  });

  it('ripple trim start keeps the clip anchored and pulls later clips left', async () => {
    await store().rippleTrimClip('c1', 'start', 500);
    const l1 = store().timeline!.tracks.find((track) => track.id === 'l1')!;
    const c1 = l1.clips.find((clip) => clip.id === 'c1')!;
    const c2 = l1.clips.find((clip) => clip.id === 'c2')!;
    expect(c1.start_ms).toBe(0);
    expect(c1.duration_ms).toBe(1500);
    expect(c1.trim_in_ms).toBe(500);
    expect(c2.start_ms).toBe(2500);
  });
});

describe('insert and overwrite', () => {
  const withAsset = () => {
    useVideoStudioStore.setState({
      assets: [{
        id: 'asset-1',
        source_type: 'upload',
        kind: 'video',
        file_name: 'b-roll.mp4',
        file_path: '/tmp/b-roll.mp4',
        mime_type: 'video/mp4',
        size_bytes: 1,
        duration_ms: 1000,
        created_at: '',
      }],
    });
  };

  it('insert with ripple splits the straddled clip and shifts the rest right', async () => {
    withAsset();
    await store().insertClipAt('asset-1', 'l1', 1000, true);
    const l1 = store().timeline!.tracks.find((track) => track.id === 'l1')!;
    // c1 (0–2000) splits at 1000; right half and c2 shift right by 1000.
    expect(l1.clips).toHaveLength(4);
    const starts = l1.clips.map((clip) => clip.start_ms);
    expect(starts).toEqual([0, 1000, 2000, 4000]);
    const inserted = l1.clips[1];
    expect(inserted.asset_id).toBe('asset-1');
    expect(inserted.duration_ms).toBe(1000);
    expect(store().selectedClipId).toBe(inserted.id);
    expect(store().timelineUndoStack).toHaveLength(1);
  });

  it('insert without ripple places the clip without shifting others', async () => {
    withAsset();
    await store().insertClipAt('asset-1', 'l1', 2100, false);
    const l1 = store().timeline!.tracks.find((track) => track.id === 'l1')!;
    expect(l1.clips).toHaveLength(3);
    expect(l1.clips.find((clip) => clip.id === 'c2')!.start_ms).toBe(3000);
  });

  it('overwrite carves out the covered range', async () => {
    withAsset();
    // Asset is 1000ms at t=1500: covers the tail of c1 (0–2000) only.
    await store().overwriteClipAt('asset-1', 'l1', 1500);
    const l1 = store().timeline!.tracks.find((track) => track.id === 'l1')!;
    expect(l1.clips).toHaveLength(3);
    const c1 = l1.clips.find((clip) => clip.id === 'c1')!;
    expect(c1.duration_ms).toBe(1500);
    const inserted = l1.clips.find((clip) => clip.asset_id === 'asset-1')!;
    expect(inserted.start_ms).toBe(1500);
    // c2 (3000–5000) is beyond the overwrite range and stays put.
    expect(l1.clips.find((clip) => clip.id === 'c2')!.start_ms).toBe(3000);
  });

  it('overwrite splits a clip that spans the whole range', async () => {
    useVideoStudioStore.setState({
      timeline: makeDoc([makeTrack('l1', [makeClip('long', 0, 5000)])]),
    });
    withAsset();
    await store().overwriteClipAt('asset-1', 'l1', 2000);
    const l1 = store().timeline!.tracks[0];
    expect(l1.clips).toHaveLength(3);
    const [leftPart, inserted, rightPart] = l1.clips;
    expect(leftPart.duration_ms).toBe(2000);
    expect(inserted.asset_id).toBe('asset-1');
    expect(inserted.start_ms).toBe(2000);
    expect(rightPart.start_ms).toBe(3000);
    expect(rightPart.duration_ms).toBe(2000);
    expect(rightPart.trim_in_ms).toBe(3000);
  });
});

describe('audio cleanup', () => {
  it('ducks music under narration with deterministic volume keyframes', async () => {
    useVideoStudioStore.setState({
      assets: [
        { id: 'a-music', source_type: 'upload', kind: 'music', file_name: 'bed.mp3', file_path: '', mime_type: 'audio/mpeg', size_bytes: 1, created_at: '' },
        { id: 'a-voice', source_type: 'upload', kind: 'audio', file_name: 'vo.mp3', file_path: '', mime_type: 'audio/mpeg', size_bytes: 1, created_at: '' },
      ],
      timeline: makeDoc([
        makeTrack('music', [makeClip('m1', 0, 10000, { asset_id: 'a-music' })]),
        makeTrack('voice', [makeClip('v1', 2000, 2000, { asset_id: 'a-voice' })]),
      ]),
    });
    await store().duckMusicUnderNarration(0.3, 250);
    const music = store().timeline!.tracks[0].clips[0];
    const volumeKfs = music.keyframes.filter((keyframe) => keyframe.property === 'volume').sort((a, b) => a.time_ms - b.time_ms);
    expect(volumeKfs.map((keyframe) => keyframe.time_ms)).toEqual([1750, 2000, 4000, 4250]);
    expect(volumeKfs.map((keyframe) => keyframe.value)).toEqual([1, 0.3, 0.3, 1]);
    expect(store().timelineUndoStack).toHaveLength(1);
  });

  it('removes fades from the selection in one undo step', async () => {
    useVideoStudioStore.setState({
      timeline: makeDoc([
        makeTrack('l1', [makeClip('c1', 0, 2000, { fade_in_ms: 500 }), makeClip('c2', 3000, 2000, { fade_out_ms: 400 })]),
      ]),
    });
    store().setSelectedClips(['c1', 'c2']);
    await store().removeClipFades();
    const clips = store().timeline!.tracks[0].clips;
    expect(clips.every((clip) => !clip.fade_in_ms && !clip.fade_out_ms)).toBe(true);
    expect(store().timelineUndoStack).toHaveLength(1);
  });
});

describe('motion presets', () => {
  it('generates editable keyframes and replaces previous pan/zoom motion', async () => {
    useVideoStudioStore.setState({
      timeline: makeDoc([
        makeTrack('l1', [makeClip('c1', 0, 4000, {
          keyframes: [
            { id: 'old-scale', property: 'scale', time_ms: 0, value: 2 },
            { id: 'vol', property: 'volume', time_ms: 0, value: 0.5 },
          ],
        })]),
      ]),
    });
    await store().applyMotionPreset('c1', 'ken_burns');
    const clip = store().timeline!.tracks[0].clips[0];
    expect(clip.keyframes.find((keyframe) => keyframe.id === 'old-scale')).toBeUndefined();
    // Volume keyframes survive motion presets.
    expect(clip.keyframes.find((keyframe) => keyframe.id === 'vol')).toBeDefined();
    const scaleKfs = clip.keyframes.filter((keyframe) => keyframe.property === 'scale');
    expect(scaleKfs).toHaveLength(2);
    expect(scaleKfs[1].time_ms).toBe(4000);
    expect(store().timelineUndoStack).toHaveLength(1);

    await store().applyMotionPreset('c1', 'restore');
    const restored = store().timeline!.tracks[0].clips[0];
    expect(restored.keyframes.filter((keyframe) => ['x', 'y', 'scale'].includes(keyframe.property))).toHaveLength(0);
    expect(restored.keyframes.find((keyframe) => keyframe.id === 'vol')).toBeDefined();
  });
});

describe('ripple mode state', () => {
  it('toggles ripple mode', () => {
    expect(store().rippleEnabled).toBe(false);
    store().toggleRipple();
    expect(store().rippleEnabled).toBe(true);
    store().toggleRipple();
    expect(store().rippleEnabled).toBe(false);
  });
});
