import { describe, expect, it } from 'vitest';
import type { VideoAsset, VideoTimelineDocument, VideoTimelineEffect } from '../../types/video';
import {
  buildTimelineIntervalIndex,
  queryActiveClipsAtFrameWithState,
} from './pro/timelineIndex';
import {
  composeCanonicalPreviewSceneFilter,
  resolvePreviewFrameSceneEffectPaint,
} from './previewFrameEffects';

const legacyBrightness: VideoTimelineEffect[] = [
  { id: 'legacy-bright', type: 'brightness', enabled: true, params: { amount: 1.8 } },
];

function documentWithSceneEffects(): VideoTimelineDocument {
  return {
    version: 1,
    canvas: { width: 640, height: 360, fps: 30, background: '#000000' },
    duration_ms: 1000,
    markers: [],
    metadata: {},
    tracks: [{
      id: 'track',
      type: 'layer',
      name: 'Layer',
      locked: false,
      muted: false,
      visible: true,
      clips: [{
        id: 'clip',
        start_ms: 0,
        duration_ms: 1000,
        trim_in_ms: 0,
        trim_out_ms: 1000,
        transform: { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1 },
        effects: [],
        keyframes: [],
        transitions: [],
      }],
    }],
    scenes: [{
      id: 'scene',
      name: 'Scene',
      start_ms: 0,
      duration_ms: 1000,
      effects: [
        { id: 'scene-bright', type: 'brightness', enabled: true, params: {} },
        { id: 'scene-off', type: 'blur', enabled: false, params: { amount: 20 } },
        { id: 'scene-contrast', type: 'contrast', enabled: true, params: { amount: 2.5 } },
      ],
    }],
  };
}

describe('canonical preview scene effects', () => {
  it('carries top-level scene effects through the same deterministic frame query', () => {
    const frame = queryActiveClipsAtFrameWithState(
      buildTimelineIntervalIndex(documentWithSceneEffects(), []),
      0,
      30,
    );

    expect(frame.clips[0].canonicalState?.clip_id).toBe('clip');
    expect(frame.frameState?.scene_effects).toEqual([
      {
        contract_version: 'effect-state-v1',
        id: 'scene-bright',
        type: 'brightness',
        scope: 'scene',
        order: 0,
        params: { amount: 1.1 },
      },
      {
        contract_version: 'effect-state-v1',
        id: 'scene-contrast',
        type: 'contrast',
        scope: 'scene',
        order: 2,
        params: { amount: 2.5 },
      },
    ]);
    expect(resolvePreviewFrameSceneEffectPaint(frame.frameState, legacyBrightness)).toEqual({
      filter: 'brightness(1.1) contrast(2.5)',
      mode: 'canonical-frame',
    });
  });

  it('shares one synchronous canonical composition across wrapper-style indexes', () => {
    const document = documentWithSceneEffects();
    const assets: VideoAsset[] = [];
    const wrapperFrame = queryActiveClipsAtFrameWithState(
      buildTimelineIntervalIndex(document, assets),
      0,
      30,
    );
    const legacyFrame = queryActiveClipsAtFrameWithState(
      buildTimelineIntervalIndex(document, assets),
      0,
      30,
    );

    expect(wrapperFrame.frameState).toBeDefined();
    expect(legacyFrame.frameState).toBe(wrapperFrame.frameState);
    expect(legacyFrame.clips[0].canonicalState).toBe(wrapperFrame.clips[0].canonicalState);
  });

  it('treats canonical omission as authoritative zero scene effects', () => {
    expect(resolvePreviewFrameSceneEffectPaint({ scene_effects: undefined }, legacyBrightness)).toEqual({
      filter: undefined,
      mode: 'canonical-frame',
    });
    expect(resolvePreviewFrameSceneEffectPaint(undefined, legacyBrightness)).toEqual({
      filter: 'brightness(1.8)',
      mode: 'legacy-authored',
    });
  });

  it('fails closed on malformed canonical scene scope or ordering', () => {
    expect(() => composeCanonicalPreviewSceneFilter([{
      contract_version: 'effect-state-v1',
      id: 'clip-effect',
      type: 'brightness',
      scope: 'clip',
      order: 0,
      params: { amount: 1.2 },
    }])).toThrow(/invalid scope/);

    expect(() => composeCanonicalPreviewSceneFilter([
      {
        contract_version: 'effect-state-v1',
        id: 'one',
        type: 'brightness',
        scope: 'scene',
        order: 0,
        params: { amount: 1.1 },
      },
      {
        contract_version: 'effect-state-v1',
        id: 'two',
        type: 'contrast',
        scope: 'scene',
        order: 0,
        params: { amount: 1.2 },
      },
    ])).toThrow(/invalid or duplicate order/);
  });

  it('omits top-level canonical state when v1 semantics cannot be projected', () => {
    const ambiguous = documentWithSceneEffects();
    ambiguous.tracks[0].clips[0].transitions = [
      { id: 'legacy-transition', type: 'fade', duration_ms: 250 },
    ];

    const frame = queryActiveClipsAtFrameWithState(
      buildTimelineIntervalIndex(ambiguous, []),
      0,
      30,
    );

    expect(frame.clips.map((entry) => entry.clip.id)).toEqual(['clip']);
    expect(frame.clips[0].canonicalState).toBeUndefined();
    expect(frame.frameState).toBeUndefined();
    expect(resolvePreviewFrameSceneEffectPaint(frame.frameState, legacyBrightness)).toEqual({
      filter: 'brightness(1.8)',
      mode: 'legacy-authored',
    });
  });
});
