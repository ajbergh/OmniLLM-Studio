import { describe, it, expect } from 'vitest';
import { adaptTimelineV1ToV2 } from './normalizeTimeline';
import { evaluateFrame } from './evaluateFrame';
import { compileAudioGraph } from './evaluateAudio';
import { VideoTimelineDocument } from '../../../frontend/src/types/video';

describe('normalizeTimeline and evaluateFrame', () => {
  const v1Doc: VideoTimelineDocument = {
    version: 1,
    width: 1920,
    height: 1080,
    fps: 30,
    duration_ms: 5000,
    tracks: [
      {
        id: 'track-1',
        name: 'Video Track',
        type: 'video',
        visible: true,
        muted: false,
        clips: [
          {
            id: 'clip-1',
            asset_id: 'asset-1',
            start_ms: 0,
            duration_ms: 2000,
            effects: [],
            keyframes: [],
            transform: { x: 10, y: 20, scale_x: 1, scale_y: 1, opacity: 1 },
          },
          {
            id: 'clip-2',
            text: { text: 'Hello World', font_size: 40 },
            start_ms: 1000,
            duration_ms: 3000,
            effects: [],
            keyframes: [],
          },
        ],
      },
    ],
  };

  it('adapts v1 document to v2 contract', () => {
    const v2 = adaptTimelineV1ToV2(v1Doc);
    expect(v2.version).toBe(2);
    expect(v2.tracks[0].clips).toHaveLength(2);
    expect(v2.tracks[0].clips[1].text?.text).toBe('Hello World');
  });

  it('evaluates active clips at integer frame index', () => {
    const v2 = adaptTimelineV1ToV2(v1Doc);
    // At frame 0 (0ms), only clip-1 is active
    const frame0 = evaluateFrame(v2, 0);
    expect(frame0.activeClips.map((c) => c.id)).toEqual(['clip-1']);

    // At frame 45 (1500ms), both clip-1 and clip-2 are active
    const frame45 = evaluateFrame(v2, 45);
    expect(frame45.activeClips.map((c) => c.id)).toEqual(['clip-1', 'clip-2']);

    // At frame 90 (3000ms), only clip-2 is active
    const frame90 = evaluateFrame(v2, 90);
    expect(frame90.activeClips.map((c) => c.id)).toEqual(['clip-2']);
  });

  it('compiles audio graph from timeline', () => {
    const v2 = adaptTimelineV1ToV2(v1Doc);
    const audioGraph = compileAudioGraph(v2, 48000);
    expect(audioGraph.sampleRate).toBe(48000);
    expect(audioGraph.totalSamples).toBe(240000); // 5s * 48000
    expect(audioGraph.tracks).toHaveLength(1);
    expect(audioGraph.tracks[0].sources).toHaveLength(1);
    expect(audioGraph.tracks[0].sources[0].clipId).toBe('clip-1');
  });
});
