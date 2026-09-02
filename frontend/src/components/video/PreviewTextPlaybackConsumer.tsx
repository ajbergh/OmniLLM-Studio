import { useEffect, useLayoutEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react';
import { createPortal } from 'react-dom';
import { useVideoStudioStore } from '../../stores/videoStudio';
import { CanonicalPreviewText } from './PreviewCanonicalPainters';
import {
  browserPreviewFontFaceLoader,
  resolvePreviewFontFaceBinding,
  type PreviewFontFaceBinding,
} from './previewFontFaceReadiness';
import {
  applyDecoderBudget,
  buildTimelineIntervalIndex,
  compareIndexedTimelineClipOrder,
  queryActiveClipsAtFrameWithState,
} from './pro/timelineIndex';
import { playbackVisualFrameIndex } from './sourceTiming';
import { settlePreviewTextLayouts } from './previewTextLayoutSnapshot';
import {
  clearPreviewTextPlaybackRuntime,
  isPreviewTextPlaybackLayer,
  previewTextPlaybackPlanIdentity,
  previewTextPlaybackPlanKey,
  previewTextPlaybackRuntimeRevision,
  previewTextPlaybackRuntimeState,
  previewTextPlaybackStructuralDeferredReason,
  publishPreviewTextPlaybackRuntime,
  subscribePreviewTextPlaybackRuntime,
} from './previewTextPlaybackRuntime';

type PreparationStatus = 'idle' | 'loading-fonts' | 'fonts-ready' | 'ready' | 'deferred' | 'failed';
interface PreparationState {
  planIdentity: string;
  status: PreparationStatus;
  reason?: string;
  trace: string[];
}

const IDLE_PREPARATION: PreparationState = { planIdentity: '', status: 'idle', trace: [] };
const PLAYBACK_TEXT_SELECTOR = '[data-preview-text-playback-surface] [data-preview-text-state-mode="canonical-frame"]';

/**
 * Prewarms standalone resource-backed canonical text during ordinary playback.
 * The established preview keeps sole ownership of playhead/source time and
 * remains visually authoritative until exact FontFace bytes and a stable
 * Chromium layout snapshot are proven for the active canonical text topology.
 * Readiness is transient browser state; no measurements flow back into authored
 * text-state-v1.
 */
export function PreviewTextPlaybackConsumer() {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const assets = useVideoStudioStore((state) => state.assets);
  const playheadMs = useVideoStudioStore((state) => state.playheadMs);
  const isPlaying = useVideoStudioStore((state) => state.isPlaying);
  const selectedClipId = useVideoStudioStore((state) => state.selectedClipId);
  const [stage, setStage] = useState<HTMLElement | null>(null);
  const [stageSize, setStageSize] = useState({ width: 0, height: 0 });
  const [preparation, setPreparation] = useState<PreparationState>(IDLE_PREPARATION);
  const preparationRef = useRef<PreparationState>(IDLE_PREPARATION);
  const runtimeRevision = useSyncExternalStore(
    subscribePreviewTextPlaybackRuntime,
    previewTextPlaybackRuntimeRevision,
    previewTextPlaybackRuntimeRevision,
  );

  const updatePreparation = (next: PreparationState) => {
    preparationRef.current = next;
    setPreparation(next);
  };

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

  useEffect(() => () => clearPreviewTextPlaybackRuntime(), []);

  const fps = timeline?.canvas.fps || 30;
  const playbackFrame = isPlaying ? playbackVisualFrameIndex(playheadMs, fps) : null;
  const intervalIndex = useMemo(() => buildTimelineIntervalIndex(timeline, assets), [timeline, assets]);
  const previewFrame = useMemo(() => {
    if (playbackFrame === null) return { layers: [] };
    const frameQuery = queryActiveClipsAtFrameWithState(intervalIndex, playbackFrame, fps);
    const visualIndexed = frameQuery.clips
      .filter(({ track }) => track.visible)
      .filter(({ clip, asset }) => (
        !clip.audio_only && (Boolean(clip.text) || Boolean(clip.shape) || !asset || !asset.mime_type.startsWith('audio/'))
      ))
      .sort(compareIndexedTimelineClipOrder);
    const decoderLimit = Math.max(1, Math.min(12, Number(window.localStorage.getItem('omnillm-video-decoder-budget') || 4)));
    const budgeted = applyDecoderBudget(visualIndexed, decoderLimit, selectedClipId);
    return { layers: [...budgeted.mounted, ...budgeted.posters].sort(compareIndexedTimelineClipOrder) };
  }, [fps, intervalIndex, playbackFrame, selectedClipId]);
  const textLayers = useMemo(() => previewFrame.layers.filter(isPreviewTextPlaybackLayer), [previewFrame.layers]);
  const planIdentity = previewTextPlaybackPlanIdentity(textLayers);
  const executionKey = previewTextPlaybackPlanKey(playbackFrame, textLayers);
  const structuralDeferred = previewTextPlaybackStructuralDeferredReason(textLayers);
  const stageScale = stageSize.width > 0 && timeline?.canvas.width
    ? stageSize.width / timeline.canvas.width
    : 0;

  const bindingPlan = useMemo(() => {
    const bindingsByKey = new Map<string, PreviewFontFaceBinding>();
    const bindingByClipId = new Map<string, PreviewFontFaceBinding>();
    const errors: string[] = [];
    for (const layer of textLayers) {
      const text = layer.canonicalState?.text;
      if (!text) continue;
      try {
        const binding = resolvePreviewFontFaceBinding(text, layer.fontAsset);
        if (!binding) {
          errors.push(`${layer.clip.id}:resource-font-required`);
          continue;
        }
        bindingsByKey.set(binding.key, binding);
        bindingByClipId.set(layer.clip.id, binding);
      } catch (reason) {
        errors.push(`${layer.clip.id}:${reason instanceof Error ? reason.message : String(reason)}`);
      }
    }
    return {
      bindings: [...bindingsByKey.values()].sort((left, right) => left.key.localeCompare(right.key)),
      bindingByClipId,
      errors: errors.sort(),
    };
  }, [textLayers]);

  useEffect(() => {
    if (!isPlaying || playbackFrame === null || textLayers.length === 0 || !planIdentity || !executionKey) {
      clearPreviewTextPlaybackRuntime();
      if (preparationRef.current.status !== 'idle') updatePreparation(IDLE_PREPARATION);
      return;
    }
    const current = preparationRef.current;
    if (current.planIdentity === planIdentity && current.status !== 'idle') return;

    if (structuralDeferred) {
      publishPreviewTextPlaybackRuntime({
        frameIndex: playbackFrame,
        planKey: executionKey,
        planIdentity,
        status: 'deferred',
        reason: structuralDeferred,
      });
      updatePreparation({
        planIdentity,
        status: 'deferred',
        reason: structuralDeferred,
        trace: [`deferred:${structuralDeferred}`],
      });
      return;
    }
    if (bindingPlan.errors.length > 0) {
      const reason = bindingPlan.errors[0];
      publishPreviewTextPlaybackRuntime({ frameIndex: playbackFrame, planKey: executionKey, planIdentity, status: 'failed', reason });
      updatePreparation({ planIdentity, status: 'failed', reason, trace: ['font-face-not-ready', `failed:${reason}`] });
      return;
    }

    let cancelled = false;
    publishPreviewTextPlaybackRuntime({
      frameIndex: playbackFrame,
      planKey: executionKey,
      planIdentity,
      status: 'pending',
      reason: 'font-face-not-ready',
    });
    updatePreparation({
      planIdentity,
      status: 'loading-fonts',
      reason: 'font-face-not-ready',
      trace: ['font-face-not-ready'],
    });
    void Promise.all(bindingPlan.bindings.map((binding) => browserPreviewFontFaceLoader.ensure(binding)))
      .then(() => {
        if (cancelled || preparationRef.current.planIdentity !== planIdentity) return;
        publishPreviewTextPlaybackRuntime({
          frameIndex: playbackFrame,
          planKey: executionKey,
          planIdentity,
          status: 'pending',
          reason: 'text-layout-not-ready',
        });
        updatePreparation({
          planIdentity,
          status: 'fonts-ready',
          reason: 'text-layout-not-ready',
          trace: ['font-face-not-ready', 'font-face-ready', 'text-layout-not-ready'],
        });
      })
      .catch(() => {
        if (cancelled || preparationRef.current.planIdentity !== planIdentity) return;
        const reason = `${textLayers[0]?.clip.id || 'text'}:font-face-load-failed`;
        publishPreviewTextPlaybackRuntime({ frameIndex: playbackFrame, planKey: executionKey, planIdentity, status: 'failed', reason });
        updatePreparation({ planIdentity, status: 'failed', reason, trace: ['font-face-not-ready', `failed:${reason}`] });
      });
    return () => {
      cancelled = true;
    };
  }, [bindingPlan.bindings, bindingPlan.errors, executionKey, isPlaying, planIdentity, playbackFrame, structuralDeferred, textLayers]);

  useEffect(() => {
    if (preparation.status !== 'fonts-ready'
      || preparation.planIdentity !== planIdentity
      || !stage
      || stageScale <= 0
      || playbackFrame === null
      || !executionKey) return;
    let cancelled = false;
    const deadline = performance.now() + 2000;
    void settlePreviewTextLayouts(stage, deadline, PLAYBACK_TEXT_SELECTOR)
      .then((snapshots) => {
        if (cancelled || preparationRef.current.planIdentity !== planIdentity) return;
        if (snapshots.length !== textLayers.length) {
          throw new Error(`text-layout-surface-count:${snapshots.length}/${textLayers.length}`);
        }
        publishPreviewTextPlaybackRuntime({ frameIndex: playbackFrame, planKey: executionKey, planIdentity, status: 'ready' });
        updatePreparation({ planIdentity, status: 'ready', trace: [...preparation.trace, 'ready'] });
      })
      .catch((reason) => {
        if (cancelled || preparationRef.current.planIdentity !== planIdentity) return;
        const detail = reason instanceof Error ? reason.message : String(reason);
        const runtimeReason = `${textLayers[0]?.clip.id || 'text'}:${detail}`;
        publishPreviewTextPlaybackRuntime({
          frameIndex: playbackFrame,
          planKey: executionKey,
          planIdentity,
          status: 'failed',
          reason: runtimeReason,
        });
        updatePreparation({
          planIdentity,
          status: 'failed',
          reason: runtimeReason,
          trace: [...preparation.trace, `failed:${runtimeReason}`],
        });
      });
    return () => {
      cancelled = true;
    };
  }, [executionKey, planIdentity, playbackFrame, preparation, stage, stageScale, textLayers.length]);

  const runtimeState = previewTextPlaybackRuntimeState();
  const runtimeReady = Boolean(planIdentity && runtimeState.planIdentity === planIdentity && runtimeState.status === 'ready');

  useLayoutEffect(() => {
    if (!stage) return;
    const previousFrame = stage.getAttribute('data-preview-text-playback-frame-index');
    const previousKey = stage.getAttribute('data-preview-text-playback-plan-key');
    const previousRuntime = stage.getAttribute('data-preview-text-playback-runtime');
    const previousConsumer = stage.getAttribute('data-preview-text-playback-consumer');
    const previousDeferred = stage.getAttribute('data-preview-text-playback-deferred');
    const previousTrace = stage.getAttribute('data-preview-text-playback-readiness-trace');

    if (playbackFrame !== null && textLayers.length > 0) {
      const canonicalPlaybackActive = runtimeReady
        && stage.dataset.previewVisualFrameMode === 'canonical-playback'
        && stage.dataset.previewVisualFrameIndex === String(playbackFrame);
      stage.setAttribute('data-preview-text-playback-frame-index', String(playbackFrame));
      if (executionKey) stage.setAttribute('data-preview-text-playback-plan-key', executionKey);
      else stage.removeAttribute('data-preview-text-playback-plan-key');
      const runtime = structuralDeferred
        ? 'deferred'
        : runtimeState.planIdentity === planIdentity
          ? runtimeState.status
          : 'pending';
      stage.setAttribute('data-preview-text-playback-runtime', runtime);
      stage.setAttribute('data-preview-text-playback-consumer', canonicalPlaybackActive ? 'canonical-text-dom' : 'legacy-time-fallback');
      const deferred = structuralDeferred
        ? `text-playback-runtime-deferred:${structuralDeferred}`
        : runtimeState.planIdentity === planIdentity && runtimeState.status !== 'ready'
          ? runtimeState.reason || 'text-playback-runtime-not-ready'
          : '';
      if (deferred) stage.setAttribute('data-preview-text-playback-deferred', deferred);
      else stage.removeAttribute('data-preview-text-playback-deferred');
      if (preparation.planIdentity === planIdentity && preparation.trace.length > 0) {
        stage.setAttribute('data-preview-text-playback-readiness-trace', preparation.trace.join(','));
      } else {
        stage.removeAttribute('data-preview-text-playback-readiness-trace');
      }
    } else {
      stage.removeAttribute('data-preview-text-playback-frame-index');
      stage.removeAttribute('data-preview-text-playback-plan-key');
      stage.removeAttribute('data-preview-text-playback-runtime');
      stage.removeAttribute('data-preview-text-playback-consumer');
      stage.removeAttribute('data-preview-text-playback-deferred');
      stage.removeAttribute('data-preview-text-playback-readiness-trace');
    }

    return () => {
      restoreAttribute(stage, 'data-preview-text-playback-frame-index', previousFrame);
      restoreAttribute(stage, 'data-preview-text-playback-plan-key', previousKey);
      restoreAttribute(stage, 'data-preview-text-playback-runtime', previousRuntime);
      restoreAttribute(stage, 'data-preview-text-playback-consumer', previousConsumer);
      restoreAttribute(stage, 'data-preview-text-playback-deferred', previousDeferred);
      restoreAttribute(stage, 'data-preview-text-playback-readiness-trace', previousTrace);
    };
  }, [executionKey, planIdentity, playbackFrame, preparation, runtimeReady, runtimeRevision, runtimeState, stage, structuralDeferred, textLayers.length]);

  useLayoutEffect(() => {
    if (!runtimeReady || !stage || playbackFrame === null) return;
    if (stage.dataset.previewVisualFrameMode !== 'canonical-playback'
      || stage.dataset.previewVisualFrameIndex !== String(playbackFrame)) return;
    const restorers: Array<() => void> = [];
    for (const layer of textLayers) {
      const host = findPreviewClipNode(stage, layer.clip.id);
      const surface = host?.querySelector<HTMLElement>('[data-preview-text-playback-surface]');
      if (!host || !surface || surface.dataset.previewTextPlaybackSurface !== layer.clip.id) continue;
      const legacyContent = findLegacyBaseContentNode(host);
      const previousSurfaceVisibility = surface.style.visibility;
      const previousSurfaceOpacity = surface.style.opacity;
      const previousLegacyVisibility = legacyContent?.style.visibility ?? '';
      const previousHost = host.getAttribute('data-preview-text-playback-host');
      surface.style.visibility = 'visible';
      surface.style.opacity = '1';
      if (legacyContent) legacyContent.style.visibility = 'hidden';
      host.setAttribute('data-preview-text-playback-host', 'canonical-text-dom');
      restorers.push(() => {
        surface.style.visibility = previousSurfaceVisibility;
        surface.style.opacity = previousSurfaceOpacity;
        if (legacyContent) legacyContent.style.visibility = previousLegacyVisibility;
        restoreAttribute(host, 'data-preview-text-playback-host', previousHost);
      });
    }
    return () => restorers.reverse().forEach((restore) => restore());
  }, [playbackFrame, playheadMs, runtimeReady, runtimeRevision, stage, textLayers]);

  if (!isPlaying
    || playbackFrame === null
    || textLayers.length === 0
    || structuralDeferred
    || !planIdentity
    || !executionKey
    || !stage
    || stageScale <= 0
    || (preparation.status !== 'fonts-ready' && preparation.status !== 'ready')) {
    return null;
  }

  return (
    <>
      {textLayers.map((layer) => {
        const host = findPreviewClipNode(stage, layer.clip.id);
        const text = layer.canonicalState?.text;
        const binding = bindingPlan.bindingByClipId.get(layer.clip.id);
        if (!host || !text || !binding) return null;
        return createPortal(
          <div
            data-preview-text-playback-surface={layer.clip.id}
            data-preview-text-playback-runtime-key={executionKey}
            data-preview-text-playback-ready={preparation.status === 'ready' ? 'true' : 'false'}
            data-preview-text-playback-pending-reason={preparation.status === 'fonts-ready' ? 'text-layout-not-ready' : undefined}
            className="pointer-events-none absolute inset-0 flex items-center justify-center"
            style={{ visibility: 'hidden', opacity: 0 }}
          >
            <CanonicalPreviewText text={text} stageScale={stageScale} fontFamilyOverride={binding.familyAlias} />
          </div>,
          host,
          `playback-text-${layer.clip.id}`,
        );
      })}
    </>
  );
}

function findPreviewClipNode(stage: HTMLElement, clipId: string): HTMLElement | null {
  for (const node of stage.querySelectorAll<HTMLElement>('[data-preview-clip-id]')) {
    if (node.dataset.previewClipId === clipId) return node;
  }
  return null;
}

function findLegacyBaseContentNode(host: HTMLElement): HTMLElement | null {
  for (const child of [...host.children]) {
    const element = child as HTMLElement;
    if (element.dataset.previewTextPlaybackSurface) continue;
    if (element.dataset.previewCanonicalContent) continue;
    if (element.dataset.previewCanonicalCursor === 'true') continue;
    if (element.tagName === 'BUTTON') continue;
    return element;
  }
  return null;
}

function restoreAttribute(node: HTMLElement, name: string, value: string | null): void {
  if (value === null) node.removeAttribute(name);
  else node.setAttribute(name, value);
}
