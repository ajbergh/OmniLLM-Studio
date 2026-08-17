import { describe, expect, it } from 'vitest';
import type { VideoTimelineClip } from '../../../types/video';
import { ANIMATION_BLOCKS } from './animationBlockRegistry';
import { compileAnimationBlock } from './animationBlockCompiler';

const clip: VideoTimelineClip = { id: 'clip', start_ms: 0, duration_ms: 5000, trim_in_ms: 0, trim_out_ms: 5000, transform: { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1 }, effects: [], transitions: [], keyframes: [] };

describe('animation block compiler', () => {
  it('compiles every registry block deterministically inside clip bounds', () => {
    for (const definition of ANIMATION_BLOCKS) {
      let sequence = 0;
      const result = compileAnimationBlock(clip, { width: 1920, height: 1080 }, definition.key, { effectId: 'blur-effect', idFactory: (prefix) => `${prefix}-${sequence++}` });
      expect(result.keyframes.length).toBeGreaterThan(0);
      expect(result.keyframes.every((keyframe) => keyframe.time_ms >= 0 && keyframe.time_ms <= clip.duration_ms)).toBe(true);
      expect(result.block.generated_keyframe_ids).toEqual(result.keyframes.map((keyframe) => keyframe.id));
    }
  });

  it('retains nearest string fallbacks for spring curves', () => {
    const result = compileAnimationBlock(clip, { width: 1920, height: 1080 }, 'pop', { idFactory: (prefix) => `${prefix}-id` });
    const spring = result.keyframes.find((keyframe) => keyframe.curve?.type === 'spring');
    expect(spring?.easing).toBe('ease-out');
  });
});
