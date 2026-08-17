// Backward-compatible view over the single semantic animation registry. New
// code should use animationBlock/compileAnimationBlock directly.
import { ANIMATION_BLOCKS, animationBlock } from '../motion/animationBlockRegistry';

export const MOTION_PRESETS = ANIMATION_BLOCKS.filter((block) => ['slow_zoom', 'zoom_out', 'pan', 'ken_burns'].includes(block.key));
export const motionPreset = animationBlock;
