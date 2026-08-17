import { describe, expect, it } from 'vitest';
import type { VideoAsset, VideoTimelineClip, VideoTimelineDocument } from '../../../types/video';
import { collectAudibleTimelineClips, sampleClipGain } from './previewAudioRenderer';

const audioAsset = (id: string, mime = 'audio/wav'): VideoAsset => ({ id, mime_type: mime } as VideoAsset);
const clip = (id: string, assetId: string, patch: Partial<VideoTimelineClip> = {}): VideoTimelineClip => ({
  id, asset_id: assetId, start_ms: 0, duration_ms: 1000, trim_in_ms: 0, trim_out_ms: 1000,
  effects: [], keyframes: [], ...patch,
});

describe('preview parity audio renderer', () => {
  it('uses persisted mute and solo state without suppressing hidden-track audio', () => {
    const timeline = {
      duration_ms: 1000,
      tracks: [
        { id: 'hidden-solo', muted: false, solo: true, visible: false, clips: [clip('solo', 'a')] },
        { id: 'normal', muted: false, visible: true, clips: [clip('normal', 'b')] },
        { id: 'muted', muted: true, solo: true, visible: true, clips: [clip('muted', 'c')] },
      ],
    } as VideoTimelineDocument;
    expect(collectAudibleTimelineClips(timeline, [audioAsset('a'), audioAsset('b'), audioAsset('c')]).map(({ clip }) => clip.id)).toEqual(['solo']);
  });

  it('samples authored gain keyframes and clip fades above unity', () => {
    const value = clip('gain', 'a', {
      duration_ms: 1000,
      volume: 2,
      fade_in_ms: 200,
      fade_out_ms: 200,
      keyframes: [
        { id: 'v0', property: 'volume', time_ms: 0, value: 1, easing: 'linear' },
        { id: 'v1', property: 'volume', time_ms: 1000, value: 2, easing: 'linear' },
      ],
    });
    expect(sampleClipGain(value, 0)).toBe(0);
    expect(sampleClipGain(value, 500)).toBeCloseTo(1.5, 6);
    expect(sampleClipGain(value, 1000)).toBe(0);
  });
});
