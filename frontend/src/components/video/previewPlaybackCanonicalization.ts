import type { CanonicalFrameLayerState, CanonicalVisualFrameState } from '../../video/renderContractFrameState';
import type { PreviewTransitionPairPlan } from './previewFrameTransitionPairs';
import {
  hasPreviewCursorPlaybackMetadata,
  previewCursorPlaybackStructuralDeferredReason,
  type PreviewCursorPlaybackContext,
  type PreviewCursorPlaybackLayer,
} from './previewCursorPlayback';
import {
  isPreviewTextPlaybackLayer,
  resolvePreviewTextPlaybackRuntime,
  type PreviewTextPlaybackLayer,
} from './previewTextPlaybackRuntime';
import { resolvePreviewWeightedPlaybackRuntime } from './previewWeightedPlaybackRuntime';

export type PreviewPlaybackVisualMode = 'legacy-time' | 'legacy-time-fallback' | 'canonical-playback';

export interface PreviewPlaybackCanonicalizationDecision {
  mode: PreviewPlaybackVisualMode;
  canonicalFrame: number | null;
  deferredReason?: string;
}

type PlaybackLayer = PreviewTextPlaybackLayer & PreviewCursorPlaybackLayer & {
  clip: PreviewTextPlaybackLayer['clip'] & PreviewCursorPlaybackLayer['clip'] & { id: string };
  canonicalState?: Pick<CanonicalFrameLayerState, 'authoritative' | 'text' | 'cursor'>;
};

type PlaybackFrameState = Pick<CanonicalVisualFrameState, 'authoritative'>;
type PlaybackTransitionPlan = Pick<
  PreviewTransitionPairPlan<PlaybackLayer>,
  'mode' | 'slots' | 'deferredReasons' | 'weightedRasterDeferredReasons'
>;

/**
 * Admit normal playback into canonical frame-domain visual evaluation only when
 * the complete frame is representable by consumers already proven in playback
 * or by the exact static-2D cursor subset already proven against export.
 *
 * Media-only none/source-over frames are admitted immediately. Standalone text
 * joins them only after exact resource-font and Chromium layout readiness has
 * been proved for the active canonical text inputs. Static cursor owners are
 * admitted synchronously only when previewCursorPlayback mirrors every relevant
 * FidelityRenderer cursor-raster exclusion and the exact canonical cursor sample
 * is present. All-weighted media pairs are admitted only when the renderer-runtime
 * registry proves that exact Canvas topology is ready.
 *
 * Shapes, unsupported cursor parents, missing raster sources, non-authoritative
 * FrameState, mixed/deferred transition composition, and stale/not-ready runtime
 * consumers fail the whole visual frame back to the established continuous-time
 * painter. The UI playhead and audio clock are not part of this decision and
 * remain continuous.
 */
export function resolvePreviewPlaybackCanonicalization(
  playbackFrame: number | null,
  frameState: PlaybackFrameState | undefined,
  layers: readonly PlaybackLayer[],
  transitionPlan: PlaybackTransitionPlan | null,
  cursorContext: PreviewCursorPlaybackContext | null = null,
): PreviewPlaybackCanonicalizationDecision {
  if (playbackFrame === null) return { mode: 'legacy-time', canonicalFrame: null };
  if (!frameState) return fallback('canonical-frame-state-unavailable');
  if (frameState.authoritative !== true) return fallback('canonical-frame-state-nonauthoritative');

  let hasPlaybackText = false;
  for (const layer of layers) {
    if (!layer.canonicalState) return fallback(`canonical-layer-state-unavailable:${layer.clip.id}`);
    if (layer.canonicalState.authoritative !== true) return fallback(`canonical-layer-state-nonauthoritative:${layer.clip.id}`);
    if (hasPreviewCursorPlaybackMetadata(layer)) {
      if (!cursorContext) return fallback(`cursor-playback-deferred:${layer.clip.id}:context-unavailable`);
      const cursorDeferred = previewCursorPlaybackStructuralDeferredReason(layer, cursorContext);
      if (cursorDeferred) return fallback(`cursor-playback-deferred:${cursorDeferred}`);
      continue;
    }
    if (isPlaybackMediaLayer(layer)) continue;
    if (isPreviewTextPlaybackLayer(layer)) {
      hasPlaybackText = true;
      continue;
    }
    return fallback(`unsupported-playback-painter:${layer.clip.id}`);
  }

  // Painter support is decided before runtime readiness so a mixed frame never
  // changes its fallback reason merely because a supported text surface warms.
  if (hasPlaybackText) {
    const textRuntime = resolvePreviewTextPlaybackRuntime(playbackFrame, layers);
    if (!textRuntime.ready) return fallback(textRuntime.deferredReason ?? 'text-playback-runtime-not-ready');
  }

  if (!transitionPlan) return fallback('transition-plan-unavailable');
  if (transitionPlan.mode === 'legacy') return fallback('transition-plan-legacy');
  if (transitionPlan.mode === 'canonical-mixed') return fallback('transition-plan-mixed');
  if (transitionPlan.deferredReasons.length > 0) {
    return fallback(`transition-deferred:${transitionPlan.deferredReasons.join(',')}`);
  }
  if (transitionPlan.mode === 'canonical-weighted-deferred') {
    if (transitionPlan.weightedRasterDeferredReasons.length > 0) {
      return fallback(`transition-weighted-raster-deferred:${transitionPlan.weightedRasterDeferredReasons.join(',')}`);
    }
    const runtime = resolvePreviewWeightedPlaybackRuntime(playbackFrame, transitionPlan);
    if (!runtime.ready) return fallback(runtime.deferredReason ?? 'transition-weighted-runtime-not-ready');
  }

  return { mode: 'canonical-playback', canonicalFrame: playbackFrame };
}

function isPlaybackMediaLayer(layer: PlaybackLayer): boolean {
  if (layer.clip.text || layer.clip.shape || layer.clip.cursor) return false;
  const mime = layer.asset?.mime_type;
  return Boolean(mime && (mime.startsWith('video/') || mime.startsWith('image/')));
}

function fallback(deferredReason: string): PreviewPlaybackCanonicalizationDecision {
  return { mode: 'legacy-time-fallback', canonicalFrame: null, deferredReason };
}
