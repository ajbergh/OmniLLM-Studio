import { describe, expect, it } from 'vitest';
import type {
  VideoAsset,
  VideoTimelineDocument,
  VideoTimelineTransform,
} from '../../../types/video';
import {
  applyDecoderBudget,
  buildTimelineIntervalIndex,
  compareIndexedTimelineClipOrder,
  queryActiveClips,
  queryActiveClipsAtFrame,
  visibleClips,
} from './timelineIndex';

const transform = (): VideoTimelineTransform => ({
  x: 0,
  y: 0,
  scale: 1,
  rotation: 0,
  opacity: 1,
});

const document: VideoTimelineDocument = {
  version: 1,
  canvas: { width: 100, height: 100, fps: 30, background: '#000' },
  duration_ms: 20_000,
  markers: [],
  metadata: {},
  tracks: [{
    id: 'track',
    type: 'layer',
    name: 'Layer',
    locked: false,
    muted: false,
    visible: true,
    clips: [
      {
        id: 'long',
        asset_id: 'video-long',
        start_ms: 0,
        duration_ms: 10_000,
        trim_in_ms: 0,
        trim_out_ms: 10_000,
        transform: transform(),
        effects: [],
        keyframes: [],
        transitions: [],
      },
      {
        id: 'short',
        asset_id: 'video-short',
        start_ms: 1000,
        duration_ms: 1000,
        trim_in_ms: 0,
        trim_out_ms: 1000,
        transform: transform(),
        effects: [],
        keyframes: [],
        transitions: [],
      },
      {
        id: 'later',
        start_ms: 12_000,
        duration_ms: 1000,
        trim_in_ms: 0,
        trim_out_ms: 1000,
        transform: transform(),
        effects: [],
        keyframes: [],
        transitions: [],
      },
    ],
  }],
};

const orderingDocument: VideoTimelineDocument = {
  version: 1,
  canvas: { width: 100, height: 100, fps: 30, background: '#000' },
  duration_ms: 5000,
  markers: [],
  metadata: {},
  tracks: [
    {
      id: 'track-zero',
      type: 'layer',
      name: 'Track zero',
      locked: false,
      muted: false,
      visible: true,
      clips: [
        {
          id: 'authored-first',
          start_ms: 1000,
          duration_ms: 3000,
          trim_in_ms: 0,
          trim_out_ms: 3000,
          z_index: 0,
          transform: transform(),
          effects: [],
          keyframes: [],
          transitions: [],
        },
        {
          id: 'authored-second',
          start_ms: 0,
          duration_ms: 4000,
          trim_in_ms: 0,
          trim_out_ms: 4000,
          z_index: 0,
          transform: transform(),
          effects: [],
          keyframes: [],
          transitions: [],
        },
        {
          id: 'z-top',
          start_ms: 0,
          duration_ms: 4000,
          trim_in_ms: 0,
          trim_out_ms: 4000,
          z_index: 5,
          transform: transform(),
          effects: [],
          keyframes: [],
          transitions: [],
        },
      ],
    },
    {
      id: 'track-one',
      type: 'layer',
      name: 'Track one',
      locked: false,
      muted: false,
      visible: true,
      clips: [{
        id: 'next-track-low-z',
        start_ms: 0,
        duration_ms: 4000,
        trim_in_ms: 0,
        trim_out_ms: 4000,
        z_index: -10,
        transform: transform(),
        effects: [],
        keyframes: [],
        transitions: [],
      }],
    },
  ],
};

const assets: VideoAsset[] = [
  {
    id: 'video-long',
    project_id: 'project',
    source_type: 'upload',
    kind: 'video',
    file_name: 'long.mp4',
    mime_type: 'video/mp4',
    file_path: 'long.mp4',
    size_bytes: 1,
    created_at: new Date(0).toISOString(),
  },
  {
    id: 'video-short',
    project_id: 'project',
    source_type: 'upload',
    kind: 'video',
    file_name: 'short.mp4',
    mime_type: 'video/mp4',
    file_path: 'short.mp4',
    size_bytes: 1,
    created_at: new Date(0).toISOString(),
  },
];

describe('timeline interval index', () => {
  it('queries overlapping active clips without losing a long earlier interval', () => {
    const index = buildTimelineIntervalIndex(document, assets);
    expect(queryActiveClips(index, 1500).map((item) => item.clip.id)).toEqual(['long', 'short']);
    expect(queryActiveClips(index, 5000).map((item) => item.clip.id)).toEqual(['long']);
    expect(queryActiveClips(index, 11_000)).toEqual([]);
  });

  it('returns canonical composition order after temporal lookup', () => {
    const index = buildTimelineIntervalIndex(orderingDocument, []);
    expect(index.clips.filter((item) => item.clip.start_ms <= 1500).map((item) => item.clip.id)).toEqual([
      'authored-second',
      'z-top',
      'next-track-low-z',
      'authored-first',
    ]);

    const active = queryActiveClips(index, 1500);
    expect(active.map((item) => item.clip.id)).toEqual([
      'authored-first',
      'authored-second',
      'z-top',
      'next-track-low-z',
    ]);
    expect(active.map((item) => item.clipIndex)).toEqual([0, 1, 2, 0]);
    expect([...active].reverse().sort(compareIndexedTimelineClipOrder).map((item) => item.clip.id)).toEqual(
      active.map((item) => item.clip.id),
    );
  });

  it('uses canonical FrameState activity for deterministic high-fps visual evaluation', () => {
    const frameDocument: VideoTimelineDocument = {
      version: 1,
      canvas: { width: 100, height: 100, fps: 120, background: '#000' },
      duration_ms: 20,
      markers: [],
      metadata: {},
      tracks: [{
        id: 'frame-track',
        type: 'layer',
        name: 'Frame track',
        locked: false,
        muted: false,
        visible: true,
        clips: [{
          id: 'starts-inside-frame-zero',
          start_ms: 5,
          duration_ms: 5,
          trim_in_ms: 0,
          trim_out_ms: 5,
          transform: transform(),
          effects: [],
          keyframes: [],
          transitions: [],
        }],
      }],
    };
    const index = buildTimelineIntervalIndex(frameDocument, []);

    expect(queryActiveClips(index, 0)).toEqual([]);
    const frameZero = queryActiveClipsAtFrame(index, 0, 120);
    expect(frameZero.map((item) => item.clip.id)).toEqual(['starts-inside-frame-zero']);
    expect(frameZero[0].canonicalState?.clip_id).toBe('starts-inside-frame-zero');
    expect(frameZero[0].canonicalState?.start_frame).toBe(0);
    expect(queryActiveClipsAtFrame(index, 1, 120).map((item) => item.clip.id))
      .toEqual(['starts-inside-frame-zero']);
    expect(queryActiveClipsAtFrame(index, 2, 120)).toEqual([]);
  });

  it('carries the exact canonical preview font asset into frame-addressed indexed clips', () => {
    const fontDocument: VideoTimelineDocument = {
      version: 1,
      canvas: { width: 100, height: 100, fps: 30, background: '#000' },
      duration_ms: 1000,
      markers: [],
      metadata: {},
      tracks: [{
        id: 'font-track',
        type: 'layer',
        name: 'Font track',
        locked: false,
        muted: false,
        visible: true,
        clips: [{
          id: 'font-clip',
          start_ms: 0,
          duration_ms: 1000,
          trim_in_ms: 0,
          trim_out_ms: 1000,
          transform: transform(),
          text: { text: 'Title', font_family: 'Inter', font_resource_id: 'inter-400-normal' },
          effects: [],
          keyframes: [],
          transitions: [],
        }],
      }],
    };
    const font: VideoAsset = {
      id: 'font-asset',
      project_id: 'project',
      source_type: 'upload',
      kind: 'font',
      file_name: 'Inter-Regular.woff2',
      file_path: 'fonts/Inter-Regular.woff2',
      mime_type: 'font/woff2',
      size_bytes: 100,
      metadata_json: JSON.stringify({ font_resource_id: 'inter-400-normal' }),
      created_at: new Date(0).toISOString(),
    };

    const active = queryActiveClipsAtFrame(buildTimelineIntervalIndex(fontDocument, [font]), 0, 30);

    expect(active).toHaveLength(1);
    expect(active[0].fontAsset).toBe(font);
    expect(active[0].canonicalState?.text?.font_resource_id).toBe('inter-400-normal');
    expect(active[0].canonicalState?.text?.font_face_source).toBe('family-name-only');
  });

  it('preserves the legacy deterministic frame path when v1 semantics are not canonically representable', () => {
    const ambiguous: VideoTimelineDocument = {
      version: 1,
      canvas: { width: 100, height: 100, fps: 30, background: '#000' },
      duration_ms: 1000,
      markers: [],
      metadata: {},
      tracks: [{
        id: 'transition-track',
        type: 'layer',
        name: 'Transition track',
        locked: false,
        muted: false,
        visible: true,
        clips: [{
          id: 'transition-clip',
          start_ms: 0,
          duration_ms: 1000,
          trim_in_ms: 0,
          trim_out_ms: 1000,
          transform: transform(),
          effects: [],
          keyframes: [],
          transitions: [{ id: 'legacy-transition', type: 'fade', duration_ms: 250 }],
        }],
      }],
    };

    const active = queryActiveClipsAtFrame(buildTimelineIntervalIndex(ambiguous, []), 0, 30);
    expect(active.map((item) => item.clip.id)).toEqual(['transition-clip']);
    expect(active[0].canonicalState).toBeUndefined();
  });

  it('virtualizes clips intersecting the visible window', () => {
    expect(visibleClips(document.tracks[0].clips, 9500, 12_100, 0).map((clip) => clip.id))
      .toEqual(['long', 'later']);
  });

  it('budgets video decoders while retaining poster candidates', () => {
    const index = buildTimelineIntervalIndex(document, assets);
    const active = queryActiveClips(index, 1500);
    const result = applyDecoderBudget(active, 1);
    expect(result.mounted.filter((item) => item.asset?.mime_type.startsWith('video/'))).toHaveLength(1);
    expect(result.posters).toHaveLength(1);
  });

  it('promotes the selected video into the decoder budget', () => {
    const index = buildTimelineIntervalIndex(document, assets);
    const active = queryActiveClips(index, 1500);
    const result = applyDecoderBudget(active, 1, 'long');
    expect(result.mounted.find((item) => item.asset?.mime_type.startsWith('video/'))?.clip.id).toBe('long');
    expect(result.posters.map((item) => item.clip.id)).toContain('short');
  });
});
