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
