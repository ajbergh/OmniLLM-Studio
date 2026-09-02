import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useState,
  useSyncExternalStore,
} from 'react';
import { createPortal } from 'react-dom';
import { useVideoStudioStore } from '../../stores/videoStudio';
import {
  applyDecoderBudget,
  buildTimelineIntervalIndex,
  compareIndexedTimelineClipOrder,
  queryActiveClipsAtFrameWithState,
} from './pro/timelineIndex';
import { playbackVisualFrameIndex } from './sourceTiming';
import { planPreviewFrameTransitionPairs } from './previewFrameTransitionPairs';
import {
  shouldConsumePreviewFrameWeightedPairs,
  weightedPairCanvasClipIds,
} from './previewFrameWeightedPairCanvas';
import {
  PreviewWeightedPairCanvas,
  type PreviewWeightedPairCanvasStatus,
} from './PreviewWeightedPairCanvas';
import {
  clearPreviewWeightedPlaybackRuntime,
  previewWeightedPlaybackPlanIdentity,
  previewWeightedPlaybackPlanKey,
  previewWeightedPlaybackRuntimeRevision,
  publishPreviewWeightedPlaybackRuntime,
  subscribePreviewWeightedPlaybackRuntime,
} from './previewWeightedPlaybackRuntime';

interface SurfaceStatusState {
  executionKey: string;
  byTransitionId: Record<string, PreviewWeightedPairCanvasStatus>;
}

const EMPTY_SURFACE_STATUS: SurfaceStatusState = { executionKey: '', byTransitionId: {} };

/**
 * Normal-playback bridge for already-defined weighted pair Canvas semantics.
 * It never owns media time: sources are the mounted legacy preview nodes. The
 * first frame of a pair topology is rasterized into hidden Canvas surfaces and
 * must prove readiness before canonical admission. Once that topology is warm,
 * each later frame still redraws exact frame-evaluated weights and geometry in a
 * layout effect before paint. Missing/poster/failed surfaces revoke readiness
 * and keep the complete visual frame on legacy time.
 */
export function PreviewWeightedPlaybackConsumer() {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const assets = useVideoStudioStore((state) => state.assets);
  const playheadMs = useVideoStudioStore((state) => state.playheadMs);
  const isPlaying = useVideoStudioStore((state) => state.isPlaying);
  const selectedClipId = useVideoStudioStore((state) => state.selectedClipId);
  const [stage, setStage] = useState<HTMLElement | null>(null);
  const [stageSize, setStageSize] = useState({ width: 0, height: 0 });
  const [surfaceStatus, setSurfaceStatus] = useState<SurfaceStatusState>(EMPTY_SURFACE_STATUS);
  const runtimeRevision = useSyncExternalStore(
    subscribePreviewWeightedPlaybackRuntime,
    previewWeightedPlaybackRuntimeRevision,
    previewWeightedPlaybackRuntimeRevision,
  );

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

  useEffect(() => () => clearPreviewWeightedPlaybackRuntime(), []);

  const fps = timeline?.canvas.fps || 30;
  const canvasWidth = timeline?.canvas.width || 1920;
  const canvasHeight = timeline?.canvas.height || 1080;
  const playbackFrame = isPlaying ? playbackVisualFrameIndex(playheadMs, fps) : null;
  const intervalIndex = useMemo(() => buildTimelineIntervalIndex(timeline, assets), [timeline, assets]);
  const previewFrame = useMemo(() => {
    if (playbackFrame === null) {
      return { layers: [], posterClipIds: new Set<string>() };
    }
    const frameQuery = queryActiveClipsAtFrameWithState(intervalIndex, playbackFrame, fps);
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
  }, [fps, intervalIndex, playbackFrame, selectedClipId]);
  const transitionPlan = useMemo(
    () => planPreviewFrameTransitionPairs(playbackFrame, previewFrame.layers),
    [playbackFrame, previewFrame.layers],
  );
  const weightedClipIds = useMemo(
    () => weightedPairCanvasClipIds(transitionPlan),
    [transitionPlan],
  );
  const posterDeferredReasons = useMemo(
    () => weightedClipIds
      .filter((clipId) => previewFrame.posterClipIds.has(clipId))
      .map((clipId) => `${clipId}:decoder-budget-poster`),
    [previewFrame.posterClipIds, weightedClipIds],
  );
  const structurallyConsumable = shouldConsumePreviewFrameWeightedPairs(transitionPlan);
  const planIdentity = previewWeightedPlaybackPlanIdentity(transitionPlan);
  const executionKey = previewWeightedPlaybackPlanKey(playbackFrame, transitionPlan);
  const weightedSlots = useMemo(
    () => structurallyConsumable && posterDeferredReasons.length === 0
      ? transitionPlan.slots.filter((slot) => slot.kind === 'pair')
      : [],
    [posterDeferredReasons.length, structurallyConsumable, transitionPlan.slots],
  );

  const onStatusChange = useCallback((status: PreviewWeightedPairCanvasStatus) => {
    if (!status.executionKey) return;
    setSurfaceStatus((current) => {
      const byTransitionId = current.executionKey === status.executionKey
        ? current.byTransitionId
        : {};
      const previous = byTransitionId[status.transitionId];
      if (current.executionKey === status.executionKey
        && previous?.status === status.status
        && previous?.reason === status.reason) {
        return current;
      }
      return {
        executionKey: status.executionKey,
        byTransitionId: {
          ...byTransitionId,
          [status.transitionId]: status,
        },
      };
    });
  }, []);

  const allReady = Boolean(
    executionKey
    && weightedSlots.length > 0
    && surfaceStatus.executionKey === executionKey
    && weightedSlots.every((slot) => surfaceStatus.byTransitionId[slot.surface.transition_id]?.status === 'ready'),
  );
  const failedStatus = surfaceStatus.executionKey === executionKey
    ? weightedSlots
      .map((slot) => surfaceStatus.byTransitionId[slot.surface.transition_id])
      .find((status) => status?.status === 'failed')
    : undefined;

  useLayoutEffect(() => {
    if (!isPlaying || playbackFrame === null || transitionPlan.mode !== 'canonical-weighted-deferred' || !executionKey || !planIdentity) {
      clearPreviewWeightedPlaybackRuntime();
      return;
    }
    if (posterDeferredReasons.length > 0) {
      publishPreviewWeightedPlaybackRuntime({
        frameIndex: playbackFrame,
        planKey: executionKey,
        planIdentity,
        status: 'deferred',
        reason: posterDeferredReasons.join(','),
      });
      return;
    }
    if (!structurallyConsumable) {
      publishPreviewWeightedPlaybackRuntime({
        frameIndex: playbackFrame,
        planKey: executionKey,
        planIdentity,
        status: 'deferred',
        reason: 'weighted-canvas-plan-not-consumable',
      });
      return;
    }
    if (failedStatus) {
      publishPreviewWeightedPlaybackRuntime({
        frameIndex: playbackFrame,
        planKey: executionKey,
        planIdentity,
        status: 'failed',
        reason: failedStatus.reason || `${failedStatus.transitionId}:weighted-canvas-failed`,
      });
      return;
    }
    publishPreviewWeightedPlaybackRuntime({
      frameIndex: playbackFrame,
      planKey: executionKey,
      planIdentity,
      status: allReady ? 'ready' : 'pending',
    });
  }, [
    allReady,
    executionKey,
    failedStatus,
    isPlaying,
    planIdentity,
    playbackFrame,
    playheadMs,
    posterDeferredReasons,
    structurallyConsumable,
    transitionPlan.mode,
  ]);

  const sourceForClip = useCallback((clipId: string): HTMLImageElement | HTMLVideoElement | null => {
    if (!stage) return null;
    const host = findPreviewClipNode(stage, clipId);
    if (!host) return null;
    return host.querySelector<HTMLVideoElement>('video')
      ?? host.querySelector<HTMLImageElement>('img');
  }, [stage]);

  useLayoutEffect(() => {
    if (!stage) return;
    const previousFrame = stage.getAttribute('data-preview-weighted-playback-frame-index');
    const previousKey = stage.getAttribute('data-preview-weighted-playback-plan-key');
    const previousRuntime = stage.getAttribute('data-preview-weighted-playback-runtime');
    const previousConsumer = stage.getAttribute('data-preview-weighted-playback-consumer');
    const previousDeferred = stage.getAttribute('data-preview-weighted-playback-deferred');

    if (playbackFrame !== null && transitionPlan.mode === 'canonical-weighted-deferred') {
      const canonicalPlaybackActive = allReady
        && stage.dataset.previewVisualFrameMode === 'canonical-playback'
        && stage.dataset.previewVisualFrameIndex === String(playbackFrame);
      stage.setAttribute('data-preview-weighted-playback-frame-index', String(playbackFrame));
      if (executionKey) stage.setAttribute('data-preview-weighted-playback-plan-key', executionKey);
      else stage.removeAttribute('data-preview-weighted-playback-plan-key');
      stage.setAttribute(
        'data-preview-weighted-playback-runtime',
        posterDeferredReasons.length > 0
          ? 'deferred'
          : failedStatus
            ? 'failed'
            : allReady
              ? 'ready'
              : 'pending',
      );
      stage.setAttribute(
        'data-preview-weighted-playback-consumer',
        canonicalPlaybackActive ? 'canonical-weighted-canvas' : 'legacy-time-fallback',
      );
      const deferred = posterDeferredReasons.length > 0
        ? posterDeferredReasons.join(',')
        : failedStatus?.reason;
      if (deferred) stage.setAttribute('data-preview-weighted-playback-deferred', deferred);
      else stage.removeAttribute('data-preview-weighted-playback-deferred');
    } else {
      stage.removeAttribute('data-preview-weighted-playback-frame-index');
      stage.removeAttribute('data-preview-weighted-playback-plan-key');
      stage.removeAttribute('data-preview-weighted-playback-runtime');
      stage.removeAttribute('data-preview-weighted-playback-consumer');
      stage.removeAttribute('data-preview-weighted-playback-deferred');
    }

    return () => {
      restoreAttribute(stage, 'data-preview-weighted-playback-frame-index', previousFrame);
      restoreAttribute(stage, 'data-preview-weighted-playback-plan-key', previousKey);
      restoreAttribute(stage, 'data-preview-weighted-playback-runtime', previousRuntime);
      restoreAttribute(stage, 'data-preview-weighted-playback-consumer', previousConsumer);
      restoreAttribute(stage, 'data-preview-weighted-playback-deferred', previousDeferred);
    };
  }, [
    allReady,
    executionKey,
    failedStatus,
    playbackFrame,
    playheadMs,
    posterDeferredReasons,
    runtimeRevision,
    stage,
    transitionPlan.mode,
  ]);

  useLayoutEffect(() => {
    if (!allReady || !stage || !executionKey || playbackFrame === null) return;
    if (stage.dataset.previewVisualFrameMode !== 'canonical-playback'
      || stage.dataset.previewVisualFrameIndex !== String(playbackFrame)) return;
    const restorers: Array<() => void> = [];
    for (const slot of weightedSlots) {
      const lower = findPreviewClipNode(stage, slot.lower.clip.id);
      const upper = findPreviewClipNode(stage, slot.upper.clip.id);
      if (!lower || !upper) continue;
      const lowerStyle = lower.getAttribute('style');
      const upperStyle = upper.getAttribute('style');
      const previousHost = lower.getAttribute('data-preview-weighted-playback-host');
      const childVisibility = [...lower.children].map((child) => ({
        child: child as HTMLElement,
        visibility: (child as HTMLElement).style.visibility,
      }));

      lower.setAttribute('data-preview-weighted-playback-host', slot.surface.transition_id);
      lower.style.left = '0';
      lower.style.top = '0';
      lower.style.width = '100%';
      lower.style.height = '100%';
      lower.style.maxWidth = 'none';
      lower.style.transform = 'none';
      lower.style.transformOrigin = '50% 50%';
      lower.style.opacity = '1';
      lower.style.clipPath = 'none';
      lower.style.filter = 'none';
      lower.style.pointerEvents = 'none';
      lower.style.outline = 'none';
      for (const child of [...lower.children]) {
        const element = child as HTMLElement;
        const isCurrentPlaybackCanvas = element.dataset.previewTransitionPairSurfaceRole === 'playback'
          && element.dataset.previewTransitionPairRuntimeKey === executionKey;
        element.style.visibility = isCurrentPlaybackCanvas ? 'visible' : 'hidden';
      }
      upper.style.visibility = 'hidden';

      restorers.push(() => {
        restoreAttribute(lower, 'style', lowerStyle);
        restoreAttribute(upper, 'style', upperStyle);
        restoreAttribute(lower, 'data-preview-weighted-playback-host', previousHost);
        for (const { child, visibility } of childVisibility) child.style.visibility = visibility;
      });
    }
    return () => restorers.reverse().forEach((restore) => restore());
  }, [allReady, executionKey, playbackFrame, playheadMs, runtimeRevision, stage, weightedSlots]);

  if (!isPlaying
    || playbackFrame === null
    || !structurallyConsumable
    || posterDeferredReasons.length > 0
    || !executionKey
    || !stage) {
    return null;
  }

  return (
    <>
      {weightedSlots.map((slot) => {
        const host = findPreviewClipNode(stage, slot.lower.clip.id);
        if (!host) return null;
        return createPortal(
          <PreviewWeightedPairCanvas
            slot={slot}
            canvasWidth={canvasWidth}
            canvasHeight={canvasHeight}
            stageWidth={stageSize.width}
            stageHeight={stageSize.height}
            sourceForClip={sourceForClip}
            executionKey={executionKey}
            active={false}
            surfaceRole="playback"
            onStatusChange={onStatusChange}
          />,
          host,
          `playback-weighted-pair-${slot.surface.transition_id}`,
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

function restoreAttribute(node: HTMLElement, name: string, value: string | null): void {
  if (value === null) node.removeAttribute(name);
  else node.setAttribute(name, value);
}
