import { RationalTime, rationalFromSeconds, frameCountFromDuration, frameIndexToTimeMs } from './timebase';
import { sampleKeyframeSequence, RenderKeyframe } from './curves';
import { sortClipsForRender, RenderOrderingItem } from './ordering';

export interface ContractTimelineTransform {
  x?: number;
  y?: number;
  z?: number;
  scaleX?: number;
  scaleY?: number;
  scaleZ?: number;
  rotationX?: number;
  rotationY?: number;
  rotationZ?: number;
  anchorX?: number;
  anchorY?: number;
  opacity?: number;
}

export interface ContractTimelineCrop {
  top?: number;
  right?: number;
  bottom?: number;
  left?: number;
  feather?: number;
  shape?: 'rectangle' | 'ellipse' | 'rounded';
  cornerRadius?: number;
}

export interface ContractTimelineText {
  text: string;
  fontFamily?: string;
  fontSize?: number;
  fontWeight?: string;
  fontStyle?: string;
  color?: string;
  background?: string;
  stroke?: string;
  strokeWidth?: number;
  shadow?: boolean;
  textAlign?: 'left' | 'center' | 'right' | 'justify';
  verticalAlign?: 'top' | 'middle' | 'bottom';
  lineHeight?: number;
  letterSpacing?: number;
  borderRadius?: number;
  padding?: number;
  wrapWidth?: number;
}

export interface ContractTimelineShape {
  kind: string;
  fill?: string;
  stroke?: string;
  strokeWidth?: number;
  cornerRadius?: number;
  opacity?: number;
  blurRadius?: number;
}

export interface ContractTimelineCursorPoint {
  timeMs: number;
  x: number;
  y: number;
  click?: boolean;
}

export interface ContractTimelineCursor {
  glyph?: string;
  size?: number;
  color?: string;
  highlight?: boolean;
  smoothing?: number;
  points?: ContractTimelineCursorPoint[];
}

export interface ContractTimelineEffect {
  id: string;
  type: string;
  amount?: number;
  enabled?: boolean;
  params?: Record<string, any>;
}

export interface ContractTimelineTransition {
  id: string;
  type: string;
  durationMs: number;
  edge?: 'in' | 'out' | 'edit_point';
  direction?: string;
  params?: Record<string, any>;
}

export interface ContractTimelineClip {
  id: string;
  assetId?: string;
  startMs: number;
  durationMs: number;
  trimInMs?: number;
  trimOutMs?: number;
  playbackRate?: number;
  volume?: number;
  pan?: number;
  fadeInMs?: number;
  fadeOutMs?: number;
  muted?: boolean;
  audioOnly?: boolean;
  zIndex?: number;
  fitMode?: 'contain' | 'cover' | 'fill' | 'none';
  transform?: ContractTimelineTransform;
  crop?: ContractTimelineCrop;
  text?: ContractTimelineText;
  shape?: ContractTimelineShape;
  cursor?: ContractTimelineCursor;
  effects?: ContractTimelineEffect[];
  transitions?: ContractTimelineTransition[];
  keyframes?: RenderKeyframe[];
}

export interface ContractTimelineTrack {
  id: string;
  name?: string;
  type: 'video' | 'audio' | 'text' | 'shape' | 'overlay';
  index?: number;
  muted?: boolean;
  solo?: boolean;
  locked?: boolean;
  visible?: boolean;
  volume?: number;
  pan?: number;
  clips: ContractTimelineClip[];
}

export interface ContractTimelineCamera {
  x?: number;
  y?: number;
  z?: number;
  rotationX?: number;
  rotationY?: number;
  rotationZ?: number;
  fieldOfView?: number;
  focusDepth?: number;
}

export interface ContractTimelineScene {
  id: string;
  name?: string;
  startMs: number;
  durationMs: number;
  color?: string;
  camera?: ContractTimelineCamera;
  effects?: ContractTimelineEffect[];
}

export interface ContractTimelineCaption {
  id: string;
  startMs: number;
  endMs: number;
  text: string;
}

export interface ContractTimelineDocument {
  version: number;
  width: number;
  height: number;
  fps: number;
  durationMs: number;
  aspectRatio: string;
  backgroundColor?: string;
  colorSpace?: 'srgb' | 'rec709' | 'p3' | 'rec2020';
  scenes?: ContractTimelineScene[];
  tracks: ContractTimelineTrack[];
  captions?: ContractTimelineCaption[];
  metadata?: Record<string, any>;
}
