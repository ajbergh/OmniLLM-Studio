import type {
  RationalTime,
} from './timebase';
import {
  isFrameInHalfOpenInterval,
  frameIndexToTimeMs,
} from './timebase';
import {
  sampleKeyframeSequence,
  RenderKeyframe,
} from './curves';
import {
  sortClipsForRender,
  RenderOrderingItem,
} from './ordering';

export interface EvaluatedTransform {
  x: number;
  y: number;
  z: number;
  scaleX: number;
  scaleY: number;
  scaleZ: number;
  rotationX: number;
  rotationY: number;
  rotationZ: number;
  anchorX: number;
  anchorY: number;
  opacity: number;
}

export interface EvaluatedClipState extends RenderOrderingItem {
  id: string;
  assetId?: string;
  clipTimeMs: number;
  sourceTimeMs: number;
  volume: number;
  transform: EvaluatedTransform;
  visible: boolean;
}

export interface FrameState {
  frameIndex: number;
  timeMs: number;
  fps: number;
  width: number;
  height: number;
  backgroundColor?: string;
  activeClips: EvaluatedClipState[];
}
