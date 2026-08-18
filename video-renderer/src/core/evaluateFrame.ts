import {
  ContractTimelineDocument,
  ContractTimelineClip,
  ContractTimelineTrack,
  ContractTimelineScene,
  ContractTimelineTransform,
  ContractTimelineEffect,
} from '../contract/timeline';
import {
  RationalTime,
  frameCountFromDuration,
  frameIndexToTimeMs,
  isFrameInHalfOpenInterval,
} from './timebase';
import { sampleKeyframeSequence } from './curves';
import { sortClipsForRender, RenderOrderingItem } from './ordering';

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
  crop?: any;
  text?: any;
  shape?: any;
  cursor?: any;
  fitMode: 'contain' | 'cover' | 'fill' | 'none';
  effects: ContractTimelineEffect[];
}

export interface EvaluatedSceneState {
  id: string;
  name?: string;
  camera?: any;
  effects: ContractTimelineEffect[];
}

export interface FrameState {
  frameIndex: number;
  timeMs: number;
  fps: number;
  width: number;
  height: number;
  backgroundColor: string;
  activeScene?: EvaluatedSceneState;
  activeClips: EvaluatedClipState[];
}

export function evaluateTransform(
  clip: ContractTimelineClip,
  clipTimeMs: number
): EvaluatedTransform {
  const base = clip.transform || {};
  const kfs = clip.keyframes || [];

  const x = sampleKeyframeSequence(kfs, 'x', clipTimeMs, base.x ?? 0);
  const y = sampleKeyframeSequence(kfs, 'y', clipTimeMs, base.y ?? 0);
  const z = sampleKeyframeSequence(kfs, 'z', clipTimeMs, base.z ?? 0);
  const scaleX = sampleKeyframeSequence(kfs, 'scale_x', clipTimeMs, base.scaleX ?? 1);
  const scaleY = sampleKeyframeSequence(kfs, 'scale_y', clipTimeMs, base.scaleY ?? 1);
  const scaleZ = sampleKeyframeSequence(kfs, 'scale_z', clipTimeMs, base.scaleZ ?? 1);
  const rotationX = sampleKeyframeSequence(kfs, 'rotation_x', clipTimeMs, base.rotationX ?? 0);
  const rotationY = sampleKeyframeSequence(kfs, 'rotation_y', clipTimeMs, base.rotationY ?? 0);
  const rotationZ = sampleKeyframeSequence(kfs, 'rotation_z', clipTimeMs, base.rotationZ ?? 0);
  const anchorX = base.anchorX ?? 0;
  const anchorY = base.anchorY ?? 0;

  let opacity = sampleKeyframeSequence(kfs, 'opacity', clipTimeMs, base.opacity ?? 1);
  if ((clip.fadeInMs ?? 0) > 0 && clipTimeMs < (clip.fadeInMs as number)) {
    opacity *= Math.max(0, Math.min(1, clipTimeMs / (clip.fadeInMs as number)));
  }
  if ((clip.fadeOutMs ?? 0) > 0 && clipTimeMs > clip.durationMs - (clip.fadeOutMs as number)) {
    opacity *= Math.max(0, Math.min(1, (clip.durationMs - clipTimeMs) / (clip.fadeOutMs as number)));
  }

  return {
    x,
    y,
    z,
    scaleX,
    scaleY,
    scaleZ,
    rotationX,
    rotationY,
    rotationZ,
    anchorX,
    anchorY,
    opacity: Math.max(0, Math.min(1, opacity)),
  };
}

export function evaluateFrame(
  doc: ContractTimelineDocument,
  frameIndex: number
): FrameState {
  const fps = doc.fps > 0 ? doc.fps : 30;
  const timeMs = frameIndexToTimeMs(frameIndex, fps);

  let activeScene: EvaluatedSceneState | undefined;
  if (doc.scenes) {
    for (const scene of doc.scenes) {
      if (isFrameInHalfOpenInterval(frameIndex, scene.startMs, scene.durationMs, fps)) {
        activeScene = {
          id: scene.id,
          name: scene.name,
          camera: scene.camera,
          effects: scene.effects ?? [],
        };
        break;
      }
    }
  }

  const anySolo = doc.tracks.some((t) => t.solo);
  const candidates: EvaluatedClipState[] = [];

  doc.tracks.forEach((track, trackIndex) => {
    if (track.visible === false || track.muted || (anySolo && !track.solo)) {
      return;
    }

    track.clips.forEach((clip, clipIndex) => {
      if (clip.muted || clip.audioOnly) return;
      if (isFrameInHalfOpenInterval(frameIndex, clip.startMs, clip.durationMs, fps)) {
        const clipTimeMs = timeMs - clip.startMs;
        const rate = clip.playbackRate ?? 1;
        const trimIn = clip.trimInMs ?? 0;
        const sourceTimeMs = trimIn + clipTimeMs * rate;

        const transform = evaluateTransform(clip, clipTimeMs);
        const zIndex = clip.zIndex ?? 0;

        candidates.push({
          id: clip.id,
          assetId: clip.assetId,
          trackIndex,
          zIndex,
          clipIndex,
          clipTimeMs,
          sourceTimeMs,
          volume: clip.volume ?? 1,
          transform,
          visible: transform.opacity > 0,
          crop: clip.crop,
          text: clip.text,
          shape: clip.shape,
          cursor: clip.cursor,
          fitMode: clip.fitMode ?? 'contain',
          effects: clip.effects ?? [],
        });
      }
    });
  });

  const activeClips = sortClipsForRender(candidates);

  return {
    frameIndex,
    timeMs,
    fps,
    width: doc.width,
    height: doc.height,
    backgroundColor: doc.backgroundColor || '#000000',
    activeScene,
    activeClips,
  };
}
