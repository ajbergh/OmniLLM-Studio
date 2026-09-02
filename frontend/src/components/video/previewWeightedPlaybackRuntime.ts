import type { PreviewTransitionPairPlan } from './previewFrameTransitionPairs';

export type PreviewWeightedPlaybackRuntimeStatus = 'idle' | 'pending' | 'ready' | 'deferred' | 'failed';

export interface PreviewWeightedPlaybackRuntimeState {
  frameIndex: number | null;
  planKey: string;
  planIdentity: string;
  status: PreviewWeightedPlaybackRuntimeStatus;
  reason?: string;
}

export interface PreviewWeightedPlaybackRuntimeDecision {
  ready: boolean;
  deferredReason?: string;
}

type RuntimeLayer = { clip: { id: string } };

type RuntimePlan<T extends RuntimeLayer = RuntimeLayer> = PreviewTransitionPairPlan<T>;

const IDLE_RUNTIME: PreviewWeightedPlaybackRuntimeState = Object.freeze({
  frameIndex: null,
  planKey: '',
  planIdentity: '',
  status: 'idle',
});

let runtimeState: PreviewWeightedPlaybackRuntimeState = IDLE_RUNTIME;
let runtimeRevision = 0;
const listeners = new Set<() => void>();

/** Stable pair-topology identity shared by every frame of one weighted overlap. */
export function previewWeightedPlaybackPlanIdentity<T extends RuntimeLayer>(
  plan: RuntimePlan<T> | null,
): string {
  if (!plan || plan.mode !== 'canonical-weighted-deferred') return '';
  const pairs: string[] = [];
  for (const slot of plan.slots) {
    if (slot.kind !== 'pair' || slot.execution !== 'weighted-canvas-deferred') continue;
    pairs.push(`${slot.surface.transition_id}:${slot.surface.lower_clip_id}>${slot.surface.upper_clip_id}`);
  }
  return pairs.join('|');
}

/**
 * Exact-frame execution identity. Frame-evaluated weights/geometry still get a
 * distinct Canvas execution key even when source/runtime readiness is warm for
 * the stable pair topology.
 */
export function previewWeightedPlaybackPlanKey<T extends RuntimeLayer>(
  frameIndex: number | null,
  plan: RuntimePlan<T> | null,
): string {
  if (frameIndex === null) return '';
  const identity = previewWeightedPlaybackPlanIdentity(plan);
  return identity ? `${frameIndex}|${identity}` : '';
}

/** Publish renderer-runtime readiness. This state is transient and never authored. */
export function publishPreviewWeightedPlaybackRuntime(next: PreviewWeightedPlaybackRuntimeState): void {
  const normalized: PreviewWeightedPlaybackRuntimeState = {
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

export function clearPreviewWeightedPlaybackRuntime(): void {
  publishPreviewWeightedPlaybackRuntime(IDLE_RUNTIME);
}

/**
 * Resolve whether the weighted Canvas path can paint the current frame before
 * browser paint. The first frame of a topology must prove a successful draw.
 * After that, ready source/runtime capability stays warm across later frames of
 * the same pair topology: the Canvas still recomputes the exact current-frame
 * weights/geometry synchronously in a layout effect before it is revealed.
 * Any pending/deferred/failed status for that topology revokes warm readiness.
 */
export function resolvePreviewWeightedPlaybackRuntime<T extends RuntimeLayer>(
  frameIndex: number,
  plan: RuntimePlan<T>,
): PreviewWeightedPlaybackRuntimeDecision {
  const planKey = previewWeightedPlaybackPlanKey(frameIndex, plan);
  const planIdentity = previewWeightedPlaybackPlanIdentity(plan);
  if (!planKey || !planIdentity || runtimeState.planIdentity !== planIdentity) {
    return { ready: false, deferredReason: 'transition-weighted-runtime-not-ready' };
  }
  if (runtimeState.status === 'ready') return { ready: true };
  if (runtimeState.status === 'deferred') {
    return {
      ready: false,
      deferredReason: runtimeState.reason
        ? `transition-weighted-runtime-deferred:${runtimeState.reason}`
        : 'transition-weighted-runtime-deferred',
    };
  }
  if (runtimeState.status === 'failed') {
    return {
      ready: false,
      deferredReason: runtimeState.reason
        ? `transition-weighted-runtime-failed:${runtimeState.reason}`
        : 'transition-weighted-runtime-failed',
    };
  }
  return { ready: false, deferredReason: 'transition-weighted-runtime-not-ready' };
}

export function subscribePreviewWeightedPlaybackRuntime(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function previewWeightedPlaybackRuntimeRevision(): number {
  return runtimeRevision;
}

export function previewWeightedPlaybackRuntimeState(): PreviewWeightedPlaybackRuntimeState {
  return runtimeState;
}

/** Test-only reset kept explicit so module-level readiness cannot leak between cases. */
export function resetPreviewWeightedPlaybackRuntimeForTests(): void {
  runtimeState = IDLE_RUNTIME;
  runtimeRevision = 0;
  listeners.clear();
}

function sameRuntimeState(
  left: PreviewWeightedPlaybackRuntimeState,
  right: PreviewWeightedPlaybackRuntimeState,
): boolean {
  return left.frameIndex === right.frameIndex
    && left.planKey === right.planKey
    && left.planIdentity === right.planIdentity
    && left.status === right.status
    && left.reason === right.reason;
}
