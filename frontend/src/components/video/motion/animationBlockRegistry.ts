import type { VideoTimelineClip, VideoTimelineKeyframe } from '../../../types/video';

export type AnimationBlockFamily = 'in' | 'during' | 'out';
export type AnimationKeyframeSpec = Omit<VideoTimelineKeyframe, 'id'>;

export interface AnimationBlockBuildContext {
  clip: VideoTimelineClip;
  canvas: { width: number; height: number };
  startMs: number;
  durationMs: number;
  params: Record<string, number | string>;
  effectId?: string;
}

export interface AnimationBlockDefinition {
  key: string;
  label: string;
  description: string;
  family: AnimationBlockFamily;
  defaultDurationMs: number;
  requiredEffect?: 'blur';
  build: (context: AnimationBlockBuildContext) => AnimationKeyframeSpec[];
}

const key = (property: VideoTimelineKeyframe['property'], time_ms: number, value: number, easing: VideoTimelineKeyframe['easing'] = 'ease-in-out', curve?: VideoTimelineKeyframe['curve']): AnimationKeyframeSpec => ({ property, time_ms, value, easing, curve });

const range = (ctx: AnimationBlockBuildContext) => ({ start: ctx.startMs, end: ctx.startMs + ctx.durationMs });
const effectAmount = (ctx: AnimationBlockBuildContext): VideoTimelineKeyframe['property'] => `effect.${ctx.effectId || 'blur'}.amount`;

export const ANIMATION_BLOCKS: AnimationBlockDefinition[] = [
  { key: 'fade_in', label: 'Fade In', description: 'Reveal from transparent.', family: 'in', defaultDurationMs: 650, build: (ctx) => { const t = range(ctx); return [key('opacity', t.start, 0), key('opacity', t.end, 1, 'ease-out')]; } },
  { key: 'slide_in', label: 'Slide In', description: 'Enter from the left.', family: 'in', defaultDurationMs: 700, build: (ctx) => { const t = range(ctx); return [key('x', t.start, -ctx.canvas.width * 0.18), key('x', t.end, 0, 'ease-out')]; } },
  { key: 'scale_in', label: 'Scale In', description: 'Grow gently into place.', family: 'in', defaultDurationMs: 650, build: (ctx) => { const t = range(ctx); return [key('scale', t.start, 0.72), key('scale', t.end, 1, 'ease-out')]; } },
  { key: 'pop', label: 'Pop', description: 'Scale in with spring overshoot.', family: 'in', defaultDurationMs: 650, build: (ctx) => { const t = range(ctx); return [key('scale', t.start, 0.8), key('scale', t.end, 1, 'ease-out', { type: 'spring', stiffness: 210, damping: 18, mass: 1 })]; } },
  { key: 'blur_reveal', label: 'Blur Reveal', description: 'Resolve from soft focus.', family: 'in', defaultDurationMs: 700, requiredEffect: 'blur', build: (ctx) => { const t = range(ctx); return [key(effectAmount(ctx), t.start, 18), key(effectAmount(ctx), t.end, 0, 'ease-out'), key('opacity', t.start, 0.35), key('opacity', t.end, 1, 'ease-out')]; } },
  { key: 'rise', label: 'Rise', description: 'Lift upward into place.', family: 'in', defaultDurationMs: 700, build: (ctx) => { const t = range(ctx); return [key('y', t.start, ctx.canvas.height * 0.1), key('y', t.end, 0, 'ease-out'), key('opacity', t.start, 0), key('opacity', t.end, 1, 'ease-out')]; } },
  { key: 'drop', label: 'Drop', description: 'Drop into place from above.', family: 'in', defaultDurationMs: 700, build: (ctx) => { const t = range(ctx); return [key('y', t.start, -ctx.canvas.height * 0.12), key('y', t.end, 0, 'ease-out', { type: 'spring', stiffness: 180, damping: 20, mass: 1 })]; } },

  { key: 'float', label: 'Float', description: 'Loop a slow vertical float.', family: 'during', defaultDurationMs: 3000, build: (ctx) => { const t = range(ctx); return [key('y', t.start, -18), key('y', t.start + ctx.durationMs / 2, 18), key('y', t.end, -18)]; } },
  { key: 'drift', label: 'Drift', description: 'Drift gently across frame.', family: 'during', defaultDurationMs: 4000, build: (ctx) => { const t = range(ctx); return [key('x', t.start, -ctx.canvas.width * 0.035, 'linear'), key('x', t.end, ctx.canvas.width * 0.035, 'linear')]; } },
  { key: 'pulse', label: 'Pulse', description: 'Rhythmic scale emphasis.', family: 'during', defaultDurationMs: 1800, build: (ctx) => { const t = range(ctx); return [key('scale', t.start, 1), key('scale', t.start + ctx.durationMs / 2, 1.08), key('scale', t.end, 1)]; } },
  { key: 'breathe', label: 'Breathe', description: 'Subtle slow scale cycle.', family: 'during', defaultDurationMs: 3200, build: (ctx) => { const t = range(ctx); return [key('scale', t.start, 0.98), key('scale', t.start + ctx.durationMs / 2, 1.03), key('scale', t.end, 0.98)]; } },
  { key: 'slow_zoom', label: 'Slow Zoom', description: 'Slow push toward the subject.', family: 'during', defaultDurationMs: 5000, build: (ctx) => { const t = range(ctx); return [key('scale', t.start, 1), key('scale', t.end, 1.25)]; } },
  { key: 'zoom_out', label: 'Slow Pull Out', description: 'Slow pull back from the subject.', family: 'during', defaultDurationMs: 5000, build: (ctx) => { const t = range(ctx); return [key('scale', t.start, 1.25), key('scale', t.end, 1)]; } },
  { key: 'pan', label: 'Pan', description: 'Pan horizontally in either direction.', family: 'during', defaultDurationMs: 5000, build: (ctx) => { const t = range(ctx); const direction = ctx.params.direction === 'right' ? 1 : -1; const amount = ctx.canvas.width * 0.08; return [key('x', t.start, -direction * amount, 'linear'), key('x', t.end, direction * amount, 'linear')]; } },
  { key: 'gentle_rotate', label: 'Gentle Rotate', description: 'A quiet rotational drift.', family: 'during', defaultDurationMs: 4500, build: (ctx) => { const t = range(ctx); return [key('rotation', t.start, -2), key('rotation', t.end, 2)]; } },
  { key: 'ken_burns', label: 'Ken Burns', description: 'Slow zoom with diagonal drift.', family: 'during', defaultDurationMs: 5000, build: (ctx) => { const t = range(ctx); return [key('scale', t.start, 1), key('scale', t.end, 1.15), key('x', t.start, 0, 'linear'), key('x', t.end, -ctx.canvas.width * 0.04, 'linear'), key('y', t.start, 0, 'linear'), key('y', t.end, -ctx.canvas.height * 0.03, 'linear')]; } },

  { key: 'fade_out', label: 'Fade Out', description: 'Fade away at the end.', family: 'out', defaultDurationMs: 650, build: (ctx) => { const t = range(ctx); return [key('opacity', t.start, 1), key('opacity', t.end, 0, 'ease-in')]; } },
  { key: 'slide_out', label: 'Slide Out', description: 'Exit to the right.', family: 'out', defaultDurationMs: 700, build: (ctx) => { const t = range(ctx); return [key('x', t.start, 0), key('x', t.end, ctx.canvas.width * 0.18, 'ease-in')]; } },
  { key: 'scale_out', label: 'Scale Out', description: 'Shrink away at the end.', family: 'out', defaultDurationMs: 650, build: (ctx) => { const t = range(ctx); return [key('scale', t.start, 1), key('scale', t.end, 0.7, 'ease-in'), key('opacity', t.start, 1), key('opacity', t.end, 0, 'ease-in')]; } },
  { key: 'drop_away', label: 'Drop Away', description: 'Fall out below frame.', family: 'out', defaultDurationMs: 700, build: (ctx) => { const t = range(ctx); return [key('y', t.start, 0), key('y', t.end, ctx.canvas.height * 0.14, 'ease-in'), key('opacity', t.start, 1), key('opacity', t.end, 0, 'ease-in')]; } },
  { key: 'blur_out', label: 'Blur Out', description: 'Lose focus while fading.', family: 'out', defaultDurationMs: 700, requiredEffect: 'blur', build: (ctx) => { const t = range(ctx); return [key(effectAmount(ctx), t.start, 0), key(effectAmount(ctx), t.end, 18, 'ease-in'), key('opacity', t.start, 1), key('opacity', t.end, 0, 'ease-in')]; } },
];

const ALIASES: Record<string, { key: string; params?: Record<string, number | string> }> = {
  zoom_in: { key: 'slow_zoom' },
  pan_left: { key: 'pan', params: { direction: 'left' } },
  pan_right: { key: 'pan', params: { direction: 'right' } },
};

export function animationBlock(keyName: string): AnimationBlockDefinition | undefined {
  const resolved = ALIASES[keyName]?.key || keyName;
  return ANIMATION_BLOCKS.find((definition) => definition.key === resolved);
}

export function animationBlockDefaults(keyName: string): Record<string, number | string> {
  return { ...(ALIASES[keyName]?.params || {}) };
}
