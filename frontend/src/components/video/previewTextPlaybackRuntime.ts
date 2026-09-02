import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';

export type PreviewTextPlaybackRuntimeStatus = 'idle' | 'pending' | 'ready' | 'deferred' | 'failed';

export interface PreviewTextPlaybackRuntimeState {
  frameIndex: number | null;
  planKey: string;
  planIdentity: string;
  status: PreviewTextPlaybackRuntimeStatus;
  reason?: string;
}

export interface PreviewTextPlaybackRuntimeDecision {
  ready: boolean;
  deferredReason?: string;
}

export type PreviewTextPlaybackLayer = {
  clip: {
    id: string;
    text?: unknown;
    shape?: unknown;
    cursor?: unknown;
  };
  asset?: { mime_type: string };
  fontAsset?: { id: string; kind: string };
  canonicalState?: Pick<CanonicalFrameLayerState, 'authoritative' | 'text'>;
};

const IDLE_RUNTIME: PreviewTextPlaybackRuntimeState = Object.freeze({
  frameIndex: null,
  planKey: '',
  planIdentity: '',
  status: 'idle',
});

let runtimeState: PreviewTextPlaybackRuntimeState = IDLE_RUNTIME;
let runtimeRevision = 0;
const listeners = new Set<() => void>();

/** True only for standalone text whose canonical painter replaces legacy text. */
export function isPreviewTextPlaybackLayer(layer: PreviewTextPlaybackLayer): boolean {
  return Boolean(layer.clip.text)
    && !layer.clip.shape
    && !layer.clip.cursor
    && !layer.asset;
}

/**
 * Return deterministic readiness debt that can be decided without browser
 * runtime state. Editor FrameState deliberately keeps font_face_source at
 * family-name-only even when a current project font_resource_id is bound; the
 * browser FontFace loader is the consumer that proves that exact face. Normal
 * playback therefore requires the resource id plus its exact current font asset,
 * not render-manifest packaged-resource provenance. A family name with no
 * resource id deliberately remains renderer-dependent and fail-closed.
 */
export function previewTextPlaybackStructuralDeferredReason(
  layers: readonly PreviewTextPlaybackLayer[],
): string | undefined {
  for (const layer of layers) {
    if (!isPreviewTextPlaybackLayer(layer)) continue;
    const text = layer.canonicalState?.text;
    if (!text) return `${layer.clip.id}:canonical-text-state-unavailable`;
    if (!text.font_resource_id) return `${layer.clip.id}:resource-font-required`;
    if (!layer.fontAsset) return `${layer.clip.id}:font-asset-unavailable`;
    if (layer.fontAsset.kind !== 'font') return `${layer.clip.id}:font-asset-kind-${layer.fontAsset.kind || 'unknown'}`;
  }
  return undefined;
}

/**
 * Stable identity for one active standalone-text topology and its exact
 * canonical text/font inputs. Browser layout measurements are intentionally not
 * part of this identity; they are consumer evidence, never authored semantics.
 */
export function previewTextPlaybackPlanIdentity(
  layers: readonly PreviewTextPlaybackLayer[],
): string {
  const textLayers = layers.filter(isPreviewTextPlaybackLayer);
  if (textLayers.length === 0) return '';
  return JSON.stringify(textLayers.map((layer) => ({
    clip_id: layer.clip.id,
    text: layer.canonicalState?.text ?? null,
    font_asset_id: layer.fontAsset?.id ?? '',
  })));
}

/** Exact output-frame key layered on top of stable text/font identity. */
export function previewTextPlaybackPlanKey(
  frameIndex: number | null,
  layers: readonly PreviewTextPlaybackLayer[],
): string {
  if (frameIndex === null) return '';
  const identity = previewTextPlaybackPlanIdentity(layers);
  return identity ? `${frameIndex}|${identity}` : '';
}

/** Publish transient browser readiness. This state is never persisted. */
export function publishPreviewTextPlaybackRuntime(next: PreviewTextPlaybackRuntimeState): void {
  const normalized: PreviewTextPlaybackRuntimeState = {
    frameIndex: next.frameIndex,
    planKey: next.planKey,
    planIdentity: next.planIdentity,
    status: next.status,
    ...(next.reason ? { reason: next.reason } : {}),
  };
  if (sameRuntimeState(runtimeState, normalized)) return;
  runtimeState = normalized;
  runtimeRevision += 1;
  for (const listener of listeners) listener();
}

export function clearPreviewTextPlaybackRuntime(): void {
  publishPreviewTextPlaybackRuntime(IDLE_RUNTIME);
}

/**
 * Gate canonical playback on exact font-resource binding plus proven browser
 * FontFace/layout readiness. Once one static text topology is ready, readiness
 * remains warm across later output frames carrying identical canonical text/font
 * inputs. Any pending/deferred/failed state revokes that authority.
 */
export function resolvePreviewTextPlaybackRuntime(
  frameIndex: number,
  layers: readonly PreviewTextPlaybackLayer[],
): PreviewTextPlaybackRuntimeDecision {
  const structuralDeferred = previewTextPlaybackStructuralDeferredReason(layers);
  if (structuralDeferred) {
    return {
      ready: false,
      deferredReason: `text-playback-runtime-deferred:${structuralDeferred}`,
    };
  }
  const planIdentity = previewTextPlaybackPlanIdentity(layers);
  const planKey = previewTextPlaybackPlanKey(frameIndex, layers);
  if (!planIdentity || !planKey || runtimeState.planIdentity !== planIdentity) {
    return { ready: false, deferredReason: 'text-playback-runtime-not-ready' };
  }
  if (runtimeState.status === 'ready') return { ready: true };
  if (runtimeState.status === 'deferred') {
    return {
      ready: false,
      deferredReason: runtimeState.reason
        ? `text-playback-runtime-deferred:${runtimeState.reason}`
        : 'text-playback-runtime-deferred',
    };
  }
  if (runtimeState.status === 'failed') {
    return {
      ready: false,
      deferredReason: runtimeState.reason
        ? `text-playback-runtime-failed:${runtimeState.reason}`
        : 'text-playback-runtime-failed',
    };
  }
  return { ready: false, deferredReason: 'text-playback-runtime-not-ready' };
}

export function subscribePreviewTextPlaybackRuntime(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function previewTextPlaybackRuntimeRevision(): number {
  return runtimeRevision;
}

export function previewTextPlaybackRuntimeState(): PreviewTextPlaybackRuntimeState {
  return runtimeState;
}

/** Test-only reset so module-level readiness cannot leak between cases. */
export function resetPreviewTextPlaybackRuntimeForTests(): void {
  runtimeState = IDLE_RUNTIME;
  runtimeRevision = 0;
  listeners.clear();
}

function sameRuntimeState(
  left: PreviewTextPlaybackRuntimeState,
  right: PreviewTextPlaybackRuntimeState,
): boolean {
  return left.frameIndex === right.frameIndex
    && left.planKey === right.planKey
    && left.planIdentity === right.planIdentity
    && left.status === right.status
    && left.reason === right.reason;
}
