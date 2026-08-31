import type { CanonicalFrameLayerState, CanonicalVisualFrameState } from '../../video/renderContractFrameState';
import type { PreviewTransitionPairPlan } from './previewFrameTransitionPairs';

export type PreviewPlaybackVisualMode = 'legacy-time' | 'legacy-time-fallback' | 'canonical-playback';

export interface PreviewPlaybackCanonicalizationDecision {
  mode: PreviewPlaybackVisualMode;
  canonicalFrame: number | null;
  deferredReason?: string;
}

type PlaybackLayer = {
  clip: {
    id: string;
    text?: unknown;
    shape?: unknown;
    cursor?: unknown;
  };
  asset?: { mime_type: string };
  canonicalState?: Pick<CanonicalFrameLayerState, 'authoritative'>;
};

type PlaybackFrameState = Pick<CanonicalVisualFrameState, 'authoritative'>;
type PlaybackTransitionPlan = Pick<PreviewTransitionPairPlan<PlaybackLayer>, 'mode' | 'deferredReasons'>;

/**
 * Admit normal playback into canonical frame-domain visual evaluation only when
 * the complete frame is representable by consumers already proven in playback.
 *
 * This first slice deliberately admits media-only layers. Text, shapes, cursor,
 * missing raster sources, non-authoritative FrameState, and weighted/deferred
 * transition composition fail the whole visual frame back to the established
 * continuous-time painter. The UI playhead and audio clock are not part of this
 * decision and remain continuous.
 */
export function resolvePreviewPlaybackCanonicalization(
  playbackFrame: number | null,
  frameState: PlaybackFrameState | undefined,
  layers: readonly PlaybackLayer[],
  transitionPlan: PlaybackTransitionPlan | null,
): PreviewPlaybackCanonicalizationDecision {
  if (playbackFrame === null) return { mode: 'legacy-time', canonicalFrame: null };
  if (!frameState) return fallback('canonical-frame-state-unavailable');
  if (frameState.authoritative !== true) return fallback('canonical-frame-state-nonauthoritative');

  for (const layer of layers) {
    if (!layer.canonicalState) return fallback(`canonical-layer-state-unavailable:${layer.clip.id}`);
    if (layer.canonicalState.authoritative !== true) return fallback(`canonical-layer-state-nonauthoritative:${layer.clip.id}`);
    if (!isPlaybackMediaLayer(layer)) return fallback(`unsupported-playback-painter:${layer.clip.id}`);
  }

  if (!transitionPlan) return fallback('transition-plan-unavailable');
  if (transitionPlan.mode === 'legacy') return fallback('transition-plan-legacy');
  if (transitionPlan.mode === 'canonical-weighted-deferred') {
    return fallback('transition-plan-weighted-deferred');
  }
  if (transitionPlan.mode === 'canonical-mixed') return fallback('transition-plan-mixed');
  if (transitionPlan.deferredReasons.length > 0) {
    return fallback(`transition-deferred:${transitionPlan.deferredReasons.join(',')}`);
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
