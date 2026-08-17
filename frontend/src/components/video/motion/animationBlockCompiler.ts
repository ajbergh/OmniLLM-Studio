import type { VideoAnimationBlock, VideoTimelineClip, VideoTimelineKeyframe } from '../../../types/video';
import { animationBlock, animationBlockDefaults } from './animationBlockRegistry';

export interface CompileAnimationBlockOptions {
  startMs?: number;
  durationMs?: number;
  delayMs?: number;
  params?: Record<string, number | string>;
  effectId?: string;
  idFactory?: (prefix: string) => string;
}

export function compileAnimationBlock(
  clip: VideoTimelineClip,
  canvas: { width: number; height: number },
  keyName: string,
  options: CompileAnimationBlockOptions = {},
): { block: VideoAnimationBlock; keyframes: VideoTimelineKeyframe[] } {
  const definition = animationBlock(keyName);
  if (!definition) throw new Error(`Unknown animation block: ${keyName}`);
  const makeId = options.idFactory || ((prefix: string) => `${prefix}-${globalThis.crypto?.randomUUID?.() || Math.random().toString(16).slice(2)}`);
  const durationMs = Math.max(1, Math.min(clip.duration_ms, Math.round(options.durationMs ?? Math.min(definition.defaultDurationMs, clip.duration_ms))));
  const delayMs = Math.max(0, Math.round(options.delayMs || 0));
  const familyStart = definition.family === 'out' ? clip.duration_ms - durationMs : definition.family === 'during' ? Math.max(0, (clip.duration_ms - durationMs) / 2) : 0;
  const startMs = Math.max(0, Math.min(clip.duration_ms - durationMs, Math.round(options.startMs ?? familyStart) + delayMs));
  const params = { ...animationBlockDefaults(keyName), ...(options.params || {}) };
  const keyframes = definition.build({ clip, canvas, startMs, durationMs, params, effectId: options.effectId }).map((spec) => ({ ...spec, id: makeId('keyframe'), time_ms: Math.round(spec.time_ms) }));
  return {
    keyframes,
    block: {
      id: makeId('animation-block'),
      block_key: definition.key,
      family: definition.family,
      start_ms: startMs,
      duration_ms: durationMs,
      delay_ms: delayMs || undefined,
      params,
      generated_keyframe_ids: keyframes.map((keyframe) => keyframe.id),
    },
  };
}
