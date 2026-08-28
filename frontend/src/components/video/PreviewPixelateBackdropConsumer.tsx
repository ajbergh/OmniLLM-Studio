import { useCallback, useEffect, useLayoutEffect, useMemo, useState } from 'react';
import { createPortal } from 'react-dom';
import { useVideoStudioStore } from '../../stores/videoStudio';
import {
  applyDecoderBudget,
  buildTimelineIntervalIndex,
  compareIndexedTimelineClipOrder,
  queryActiveClipsAtFrameWithState,
} from './pro/timelineIndex';
import { frameAddressMatchesTimelineMs } from './sourceTiming';
import {
  planPreviewPixelateBackdrop,
  type PreviewPixelateBackdropLayer,
} from './previewPixelateBackdrop';
import {
  PreviewPixelateCanvas,
  type PreviewPixelateCanvasStatus,
  type PreviewPixelateCanvasStatusKind,
} from './PreviewPixelateCanvas';

interface RuntimeState {
  executionKey: string;
  status: PreviewPixelateCanvasStatusKind;
  reason?: string;
}

interface PixelateParitySurfaceState {
  status: PreviewPixelateCanvasStatusKind | null;
  active: boolean;
}

const IDLE_RUNTIME: RuntimeState = { executionKey: '', status: 'pending' };

/**
 * Deterministic pixelate Canvas consumer layered onto VideoPreviewCanvas.
 * Structural admission and runtime decoded-pixel proof both fail closed: normal
 * playback, poster-budget sources, transparent target regions, and unsupported
 * canonical state leave the established CSS approximation untouched.
 */
export function PreviewPixelateBackdropConsumer() {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const assets = useVideoStudioStore((state) => state.assets);
  const playheadMs = useVideoStudioStore((state) => state.playheadMs);
  const isPlaying = useVideoStudioStore((state) => state.isPlaying);
  const selectedClipId = useVideoStudioStore((state) => state.selectedClipId);
  const [frameAddress, setFrameAddress] = useState<number | null>(null);
  const [stage, setStage] = useState<HTMLElement | null>(null);
  const [stageSize, setStageSize] = useState({ width: 0, height: 0 });
  const [runtime, setRuntime] = useState<RuntimeState>(IDLE_RUNTIME);

  useLayoutEffect(() => {
    const onParitySeek = (event: Event) => {
      const detail = (event as CustomEvent<{ frameIndex?: number }>).detail || {};
      setFrameAddress(Math.max(0, Math.floor(detail.frameIndex ?? 0)));
    };
    window.addEventListener('omnillm:video-parity-seek', onParitySeek, true);
    return () => window.removeEventListener('omnillm:video-parity-seek', onParitySeek, true);
  }, []);

  useEffect(() => {
    setFrameAddress(null);
  }, [timeline]);

  useLayoutEffect(() => {
    const resolveStage = () => {
      const next = document.querySelector<HTMLElement>('[data-testid="video-preview-program"]');
      setStage((current) => current === next ? current : next);
    };
    resolveStage();
    const observer = new MutationObserver(resolveStage);
    observer.observe(document.body, { childList: true, subtree: true });
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    if (!stage) {
      setStageSize({ width: 0, height: 0 });
      return;
    }
    const update = () => setStageSize({ width: stage.clientWidth, height: stage.clientHeight });
    update();
    const observer = new ResizeObserver(update);
    observer.observe(stage);
    return () => observer.disconnect();
  }, [stage]);

  const fps = timeline?.canvas.fps || 30;
  const canvasWidth = timeline?.canvas.width || 1920;
  const canvasHeight = timeline?.canvas.height || 1080;
  const stageScale = stageSize.width > 0 ? stageSize.width / canvasWidth : 0;
  const deterministicFrame = !isPlaying
    && frameAddress !== null
    && frameAddressMatchesTimelineMs(frameAddress, fps, playheadMs)
    ? frameAddress
    : null;
  const intervalIndex = useMemo(() => buildTimelineIntervalIndex(timeline, assets), [timeline, assets]);
  const previewFrame = useMemo(() => {
    if (deterministicFrame === null) {
      return { layers: [] as PreviewPixelateBackdropLayer[], posterClipIds: new Set<string>() };
    }
    const frameQuery = queryActiveClipsAtFrameWithState(intervalIndex, deterministicFrame, fps);
    const visualIndexed = frameQuery.clips
      .filter(({ track }) => track.visible)
      .filter(({ clip, asset }) => (
        !clip.audio_only && (Boolean(clip.text) || Boolean(clip.shape) || !asset || !asset.mime_type.startsWith('audio/'))
      ))
      .sort(compareIndexedTimelineClipOrder);
    const decoderLimit = Math.max(1, Math.min(12, Number(window.localStorage.getItem('omnillm-video-decoder-budget') || 4)));
    const budgeted = applyDecoderBudget(visualIndexed, decoderLimit, selectedClipId);
    return {
      layers: [...budgeted.mounted, ...budgeted.posters].sort(compareIndexedTimelineClipOrder),
      posterClipIds: new Set(budgeted.posters.map(({ clip }) => clip.id)),
    };
  }, [deterministicFrame, fps, intervalIndex, selectedClipId]);
  const plan = useMemo(
    () => planPreviewPixelateBackdrop(deterministicFrame, previewFrame.layers),
    [deterministicFrame, previewFrame.layers],
  );
  const posterDeferredReason = plan.mode === 'canonical-ready'
    && previewFrame.posterClipIds.has(plan.backdrop.clip.id)
    ? `${plan.backdrop.clip.id}:decoder-budget-poster`
    : undefined;
  const consume = plan.mode === 'canonical-ready' && !posterDeferredReason;
  const executionKey = consume
    ? `${deterministicFrame}:${plan.target.clip.id}:${plan.backdrop.clip.id}:${canvasWidth}x${canvasHeight}`
    : '';
  const runtimeStatus = consume && runtime.executionKey === executionKey ? runtime.status : 'pending';
  const runtimeReason = consume && runtime.executionKey === executionKey ? runtime.reason : undefined;
  const canvasReady = consume && runtimeStatus === 'ready';

  useEffect(() => {
    setRuntime(executionKey ? { executionKey, status: 'pending' } : IDLE_RUNTIME);
  }, [executionKey]);

  const sourceForClip = useCallback((clipId: string): HTMLImageElement | HTMLVideoElement | null => {
    if (!stage) return null;
    const host = findPreviewClipNode(stage, clipId);
    if (!host) return null;
    return host.querySelector<HTMLVideoElement>('video')
      ?? host.querySelector<HTMLImageElement>('img');
  }, [stage]);

  const onStatusChange = useCallback((next: PreviewPixelateCanvasStatus) => {
    setRuntime((current) => {
      if (next.executionKey !== executionKey) return current;
      if (current.executionKey === next.executionKey
        && current.status === next.status
        && current.reason === next.reason) {
        return current;
      }
      return {
        executionKey: next.executionKey,
        status: next.status,
        ...(next.reason ? { reason: next.reason } : {}),
      };
    });
  }, [executionKey]);

  useLayoutEffect(() => {
    if (!stage) return;
    const previousPlan = stage.getAttribute('data-preview-pixelate-plan-mode');
    const previousConsumer = stage.getAttribute('data-preview-pixelate-consumer');
    const previousStructural = stage.getAttribute('data-preview-pixelate-structural-deferred');
    const previousRuntime = stage.getAttribute('data-preview-pixelate-runtime-deferred');
    const previousError = stage.getAttribute('data-preview-pixelate-runtime-error');

    stage.setAttribute('data-preview-pixelate-plan-mode', plan.mode);
    stage.setAttribute('data-preview-pixelate-consumer', canvasReady ? 'canonical-canvas' : 'css-fallback');
    if (plan.mode === 'canonical-deferred' && plan.deferredReasons.length > 0) {
      stage.setAttribute('data-preview-pixelate-structural-deferred', plan.deferredReasons.join(','));
    } else {
      stage.removeAttribute('data-preview-pixelate-structural-deferred');
    }
    const deferredReason = posterDeferredReason
      ?? (runtimeStatus === 'deferred' ? runtimeReason : undefined);
    if (deferredReason) stage.setAttribute('data-preview-pixelate-runtime-deferred', deferredReason);
    else stage.removeAttribute('data-preview-pixelate-runtime-deferred');
    if (runtimeStatus === 'failed' && runtimeReason) {
      stage.setAttribute('data-preview-pixelate-runtime-error', runtimeReason);
    } else {
      stage.removeAttribute('data-preview-pixelate-runtime-error');
    }

    return () => {
      restoreAttribute(stage, 'data-preview-pixelate-plan-mode', previousPlan);
      restoreAttribute(stage, 'data-preview-pixelate-consumer', previousConsumer);
      restoreAttribute(stage, 'data-preview-pixelate-structural-deferred', previousStructural);
      restoreAttribute(stage, 'data-preview-pixelate-runtime-deferred', previousRuntime);
      restoreAttribute(stage, 'data-preview-pixelate-runtime-error', previousError);
    };
  }, [canvasReady, plan, posterDeferredReason, runtimeReason, runtimeStatus, stage]);

  useLayoutEffect(() => {
    if (!canvasReady || !stage || plan.mode !== 'canonical-ready') return;
    const host = findPreviewClipNode(stage, plan.target.clip.id);
    if (!host) return;
    const previousStyle = host.getAttribute('style');
    const previousHost = host.getAttribute('data-preview-pixelate-host');
    const childState = [...host.children].map((child) => {
      const element = child as HTMLElement;
      return {
        element,
        visibility: element.style.visibility,
        deferredMarker: element.getAttribute('data-preview-shape-painter-deferred'),
      };
    });

    host.setAttribute('data-preview-pixelate-host', 'canonical-canvas');
    host.style.left = '0';
    host.style.top = '0';
    host.style.width = '100%';
    host.style.height = '100%';
    host.style.maxWidth = 'none';
    host.style.transform = 'none';
    host.style.transformOrigin = '50% 50%';
    host.style.opacity = '1';
    host.style.clipPath = 'none';
    host.style.filter = 'none';
    host.style.pointerEvents = 'none';
    host.style.outline = 'none';
    for (const { element } of childState) {
      if (element.dataset.previewPixelateExecution !== 'canvas') {
        element.style.visibility = 'hidden';
      }
      if (element.getAttribute('data-preview-shape-painter-deferred') === 'pixelate-css-approximation') {
        element.removeAttribute('data-preview-shape-painter-deferred');
      }
    }

    return () => {
      restoreAttribute(host, 'style', previousStyle);
      restoreAttribute(host, 'data-preview-pixelate-host', previousHost);
      for (const { element, visibility, deferredMarker } of childState) {
        element.style.visibility = visibility;
        restoreAttribute(element, 'data-preview-shape-painter-deferred', deferredMarker);
      }
    };
  }, [canvasReady, plan, stage]);

  useLayoutEffect(() => {
    if (!consume || !stage || plan.mode !== 'canonical-ready') return;
    const targetClipId = plan.target.clip.id;
    const previousError = stage.getAttribute('data-preview-pixelate-runtime-error');
    const onParityReady = (event: Event) => {
      const detail = (event as CustomEvent<Record<string, unknown>>).detail || {};
      if (detail.pixelateCanvasResume === true) return;
      const current = pixelateParitySurfaceState(stage, targetClipId);
      if (current.status === 'deferred' || (current.status === 'ready' && current.active)) return;
      event.stopImmediatePropagation();
      if (current.status === 'failed') {
        stage.setAttribute('data-preview-pixelate-runtime-error', 'pixelate-canvas-failed');
        return;
      }
      const deadline = performance.now() + 2000;
      const signalWhenSettled = () => {
        const settled = pixelateParitySurfaceState(stage, targetClipId);
        if (settled.status === 'deferred' || (settled.status === 'ready' && settled.active)) {
          stage.removeAttribute('data-preview-pixelate-runtime-error');
          window.dispatchEvent(new CustomEvent('omnillm:video-parity-ready', {
            detail: { ...detail, pixelateCanvasResume: true },
          }));
          return;
        }
        if (settled.status === 'failed') {
          stage.setAttribute('data-preview-pixelate-runtime-error', 'pixelate-canvas-failed');
          return;
        }
        if (performance.now() < deadline) {
          requestAnimationFrame(signalWhenSettled);
          return;
        }
        stage.setAttribute(
          'data-preview-pixelate-runtime-error',
          settled.status === 'ready' ? 'pixelate-canvas-not-visible' : 'pixelate-canvas-not-ready',
        );
      };
      requestAnimationFrame(signalWhenSettled);
    };
    window.addEventListener('omnillm:video-parity-ready', onParityReady, true);
    return () => {
      window.removeEventListener('omnillm:video-parity-ready', onParityReady, true);
      restoreAttribute(stage, 'data-preview-pixelate-runtime-error', previousError);
    };
  }, [consume, plan, stage]);

  if (!consume || !stage || plan.mode !== 'canonical-ready') return null;
  const host = findPreviewClipNode(stage, plan.target.clip.id);
  if (!host) return null;
  return createPortal(
    <PreviewPixelateCanvas
      plan={plan}
      canvasWidth={canvasWidth}
      canvasHeight={canvasHeight}
      stageScale={stageScale}
      sourceForClip={sourceForClip}
      executionKey={executionKey}
      active={canvasReady}
      onStatusChange={onStatusChange}
    />,
    host,
    `pixelate-canvas-${executionKey}`,
  );
}

function findPreviewClipNode(stage: HTMLElement, clipId: string): HTMLElement | null {
  for (const node of stage.querySelectorAll<HTMLElement>('[data-preview-clip-id]')) {
    if (node.dataset.previewClipId === clipId) return node;
  }
  return null;
}

function pixelateParitySurfaceState(
  stage: HTMLElement,
  targetClipId: string,
): PixelateParitySurfaceState {
  for (const surface of stage.querySelectorAll<HTMLElement>('[data-preview-pixelate-execution="canvas"]')) {
    if (surface.dataset.previewPixelateTargetClip !== targetClipId) continue;
    const status = surface.dataset.previewPixelateStatus;
    if (status !== 'pending' && status !== 'ready' && status !== 'deferred' && status !== 'failed') {
      return { status: null, active: false };
    }
    if (status !== 'ready') return { status, active: false };
    const host = findPreviewClipNode(stage, targetClipId);
    const cssFallbackPresent = Boolean(
      host?.querySelector('[data-preview-shape-painter-deferred="pixelate-css-approximation"]'),
    );
    return {
      status,
      active: stage.dataset.previewPixelateConsumer === 'canonical-canvas'
        && host?.dataset.previewPixelateHost === 'canonical-canvas'
        && surface.style.visibility === 'visible'
        && !cssFallbackPresent,
    };
  }
  return { status: null, active: false };
}

function restoreAttribute(node: HTMLElement, name: string, value: string | null): void {
  if (value === null) node.removeAttribute(name);
  else node.setAttribute(name, value);
}
