import type { VideoTimelineClip, VideoTimelineKeyframe } from '../../../types/video';

export type MotionKeyframeSpec = Omit<VideoTimelineKeyframe, 'id'>;

export interface MotionPreset {
  key: string;
  label: string;
  description: string;
  /** Keyframes to generate (clip-relative times); replaces existing x/y/scale keyframes. */
  build: (clip: VideoTimelineClip, canvas: { width: number; height: number }) => MotionKeyframeSpec[];
}

/**
 * Pan/zoom presets implemented as generated keyframes, so users can refine
 * them afterwards. Scale keyframes are preview-only at export today — the
 * renderer capability matrix reports it, and the inspector shows the badge.
 */
export const MOTION_PRESETS: MotionPreset[] = [
  {
    key: 'zoom_in',
    label: 'Zoom in',
    description: 'Slow push-in from 100% to 125%',
    build: (clip) => [
      { property: 'scale', time_ms: 0, value: 1, easing: 'ease-in-out' },
      { property: 'scale', time_ms: clip.duration_ms, value: 1.25, easing: 'ease-in-out' },
    ],
  },
  {
    key: 'zoom_out',
    label: 'Zoom out',
    description: 'Pull back from 125% to 100%',
    build: (clip) => [
      { property: 'scale', time_ms: 0, value: 1.25, easing: 'ease-in-out' },
      { property: 'scale', time_ms: clip.duration_ms, value: 1, easing: 'ease-in-out' },
    ],
  },
  {
    key: 'pan_left',
    label: 'Pan left',
    description: 'Drift the frame from right to left',
    build: (clip, canvas) => [
      { property: 'x', time_ms: 0, value: Math.round(canvas.width * 0.08), easing: 'linear' },
      { property: 'x', time_ms: clip.duration_ms, value: -Math.round(canvas.width * 0.08), easing: 'linear' },
    ],
  },
  {
    key: 'pan_right',
    label: 'Pan right',
    description: 'Drift the frame from left to right',
    build: (clip, canvas) => [
      { property: 'x', time_ms: 0, value: -Math.round(canvas.width * 0.08), easing: 'linear' },
      { property: 'x', time_ms: clip.duration_ms, value: Math.round(canvas.width * 0.08), easing: 'linear' },
    ],
  },
  {
    key: 'ken_burns',
    label: 'Ken Burns',
    description: 'Slow zoom with a gentle diagonal drift',
    build: (clip, canvas) => [
      { property: 'scale', time_ms: 0, value: 1, easing: 'ease-in-out' },
      { property: 'scale', time_ms: clip.duration_ms, value: 1.15, easing: 'ease-in-out' },
      { property: 'x', time_ms: 0, value: 0, easing: 'linear' },
      { property: 'x', time_ms: clip.duration_ms, value: -Math.round(canvas.width * 0.04), easing: 'linear' },
      { property: 'y', time_ms: 0, value: 0, easing: 'linear' },
      { property: 'y', time_ms: clip.duration_ms, value: -Math.round(canvas.height * 0.03), easing: 'linear' },
    ],
  },
  {
    key: 'restore',
    label: 'Restore full frame',
    description: 'Removes pan/zoom motion keyframes',
    build: () => [],
  },
];

export function motionPreset(key: string): MotionPreset | undefined {
  return MOTION_PRESETS.find((preset) => preset.key === key);
}
