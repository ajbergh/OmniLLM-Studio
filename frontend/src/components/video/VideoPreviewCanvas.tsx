import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { useVideoStudioStore } from '../../stores/videoStudio';
import {
  CanonicalPreviewCursor,
  CanonicalPreviewShape,
  CanonicalPreviewText,
  canonicalPreviewShapeUsesText,
  previewClipHasMediaBase,
  resolvePreviewCanonicalPainterPlan,
} from './PreviewCanonicalPainters';
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
import { resolvePreviewFrameSceneEffectPaint } from './previewFrameEffects';
import { frameAddressMatchesTimelineMs } from './sourceTiming';
import { planPreviewFrameTransitionPairs } from './previewFrameTransitionPairs';
import {
  shouldConsumePreviewFrameWeightedPairs,
  weightedPairCanvasClipIds,
} from './previewFrameWeightedPairCanvas';
import { PreviewWeightedPairCanvas } from './PreviewWeightedPairCanvas';
import { PreviewPixelateBackdropConsumer } from './PreviewPixelateBackdropConsumer';
import { VideoPreviewCanvas as LegacyVideoPreviewCanvas } from './VideoPreviewCanvasLegacy';

type PreviewFontFaceStatus = 'none' | 'loading' | 'ready' | 'failed';
interface PreviewFontFaceReadinessState {
  signature: string;
  status: PreviewFontFaceStatus;
}

/**
 * Canonical deterministic consumer layered around the established interactive
 * preview. Free-running playback and editor gestures remain owned by the legacy
 * surface; explicit frame-addressed weighted transition pairs replace their two
 * adjacent DOM inputs in-place with one exact Canvas surface. Scene effects and
 * text/shape/cursor painter inputs consume the same already-evaluated FrameState.
 */
export function VideoPreviewCanvas() {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const assets = useVideoStudioStore((state) => state.assets);
  const playheadMs = useVideoStudioStore((state) => state.playheadMs);
  const isPlaying = useVideoStudioStore((state) => state.isPlaying);
  const selectedClipId = useVideoStudioStore((state) => state.selectedClipId);
  const rootRef = useRef<HTMLDivElement | null>(null);
  const [frameAddress, setFrameAddress] = useState<number | null>(null);
  const [stage, setStage] = useState<HTMLElement | null>(null);
  const [stageSize, setStageSize] = useState({ width: 0, height: 0 });
  const [fontFaceReadiness, setFontFaceReadiness] = useState<PreviewFontFaceReadinessState>({
    signature: '',
    status: 'none',
  });

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
    const root = rootRef.current;
    if (!root) return;
    const resolveStage = () => {
      const next = root.querySelector<HTMLElement>('[data-testid="video-preview-program"]');
      setStage((current) => current === next ? current : next);
    };
    resolveStage();
    const observer = new MutationObserver(resolveStage);
    observer.observe(root, { childList: true, subtree: true });
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
      return { layers: [], posterClipIds: new Set<string>(), frameState: undefined };
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
      frameState: frameQuery.frameState,
    };
  }, [deterministicFrame, fps, intervalIndex, selectedClipId]);
  const sceneEffectPaint = useMemo(
    () => previewFrame.frameState
      ? resolvePreviewFrameSceneEffectPaint(previewFrame.frameState, undefined)
      : null,
    [previewFrame.frameState],
  );
  const canonicalPainterPlans = useMemo(
    () => previewFrame.layers
      .map((layer) => ({
        layer,
        plan: resolvePreviewCanonicalPainterPlan(layer.canonicalState, {
          hasMediaBase: previewClipHasMediaBase(layer.clip, layer.asset?.mime_type),
          hasShape: Boolean(layer.clip.shape),
          hasText: Boolean(layer.clip.text),
          hasCursor: Boolean(layer.clip.cursor),
        }),
      }))
      .filter(({ plan }) => plan.mode === 'canonical-frame' && (plan.content !== 'none' || plan.cursor !== 'none')),
    [previewFrame.layers],
  );
  const fontFacePlan = useMemo(() => {
    const bindingsByKey = new Map<string, PreviewFontFaceBinding>();
    const bindingByClipId = new Map<string, PreviewFontFaceBinding>();
    const errors: string[] = [];
    for (const { layer, plan } of canonicalPainterPlans) {
      const text = layer.canonicalState?.text;
      if (!text?.font_resource_id) continue;
      const paintsText = plan.content === 'canonical-text'
        || (plan.content === 'canonical-shape' && canonicalPreviewShapeUsesText(layer.canonicalState?.shape));
      if (!paintsText) continue;
      try {
        const binding = resolvePreviewFontFaceBinding(text, layer.fontAsset);
        if (!binding) continue;
        bindingsByKey.set(binding.key, binding);
        bindingByClipId.set(layer.clip.id, binding);
      } catch (reason) {
        errors.push(`${layer.clip.id}:${reason instanceof Error ? reason.message : String(reason)}`);
      }
    }
    const bindings = [...bindingsByKey.values()].sort((left, right) => left.key.localeCompare(right.key));
    errors.sort();
    return {
      bindings,
      bindingByClipId,
      errors,
      signature: JSON.stringify({ bindings: bindings.map(({ key }) => key), errors }),
    };
  }, [canonicalPainterPlans]);
  const fontFaceRequired = fontFacePlan.bindings.length > 0 || fontFacePlan.errors.length > 0;
  const fontFaceStatus: PreviewFontFaceStatus = !fontFaceRequired
    ? 'none'
    : fontFaceReadiness.signature === fontFacePlan.signature
      ? fontFaceReadiness.status
      : 'loading';

  useEffect(() => {
    let cancelled = false;
    if (!fontFaceRequired) {
      setFontFaceReadiness({ signature: fontFacePlan.signature, status: 'none' });
      return;
    }
    if (fontFacePlan.errors.length > 0) {
      setFontFaceReadiness({ signature: fontFacePlan.signature, status: 'failed' });
      return;
    }
    setFontFaceReadiness({ signature: fontFacePlan.signature, status: 'loading' });
    void Promise.all(fontFacePlan.bindings.map((binding) => browserPreviewFontFaceLoader.ensure(binding)))
      .then(() => {
        if (!cancelled) setFontFaceReadiness({ signature: fontFacePlan.signature, status: 'ready' });
      })
      .catch(() => {
        if (!cancelled) setFontFaceReadiness({ signature: fontFacePlan.signature, status: 'failed' });
      });
    return () => {
      cancelled = true;
    };
  }, [fontFacePlan, fontFaceRequired]);

  const transitionPairPlan = useMemo(() => planPreviewFrameTransitionPairs(
    deterministicFrame,
    previewFrame.layers,
  ), [deterministicFrame, previewFrame.layers]);
  const weightedClipIds = useMemo(
    () => weightedPairCanvasClipIds(transitionPairPlan),
    [transitionPairPlan],
  );
  const runtimeDeferredReasons = useMemo(
    () => weightedClipIds
      .filter((clipId) => previewFrame.posterClipIds.has(clipId))
      .map((clipId) => `${clipId}:decoder-budget-poster`),
    [previewFrame.posterClipIds, weightedClipIds],
  );
  const consumeWeightedPairs = shouldConsumePreviewFrameWeightedPairs(transitionPairPlan)
    && runtimeDeferredReasons.length === 0;
  const weightedSlots = useMemo(
    () => consumeWeightedPairs
      ? transitionPairPlan.slots.filter((slot) => slot.kind === 'pair')
      : [],
    [consumeWeightedPairs, transitionPairPlan],
  );
  const weightedRasterDeferredReasons = transitionPairPlan.weightedRasterDeferredReasons;

  const sourceForClip = useCallback((clipId: string): HTMLImageElement | HTMLVideoElement | null => {
    if (!stage) return null;
    const node = findPreviewClipNode(stage, clipId);
    if (!node) return null;
    return node.querySelector<HTMLVideoElement>('video')
      ?? node.querySelector<HTMLImageElement>('img');
  }, [stage]);

  useLayoutEffect(() => {
    if (!stage || deterministicFrame === null || !sceneEffectPaint || sceneEffectPaint.mode !== 'canonical-frame') return;
    const previousFilter = stage.style.filter;
    const previousMode = stage.getAttribute('data-preview-scene-effect-state-mode');
    if (sceneEffectPaint.filter) stage.style.setProperty('filter', sceneEffectPaint.filter);
    else stage.style.removeProperty('filter');
    stage.setAttribute('data-preview-scene-effect-state-mode', 'canonical-frame');
    return () => {
      if (previousFilter) stage.style.setProperty('filter', previousFilter);
      else stage.style.removeProperty('filter');
      restoreAttribute(stage, 'data-preview-scene-effect-state-mode', previousMode);
    };
  }, [deterministicFrame, sceneEffectPaint, stage]);

  useLayoutEffect(() => {
    if (!stage || deterministicFrame === null || canonicalPainterPlans.length === 0) return;
    const restorers: Array<() => void> = [];
    for (const { layer, plan } of canonicalPainterPlans) {
      const host = findPreviewClipNode(stage, layer.clip.id);
      if (!host) continue;

      const previousContentMode = host.getAttribute('data-preview-content-state-mode');
      const previousCursorMode = host.getAttribute('data-preview-cursor-state-mode');
      const restoredStyles: Array<{
        node: HTMLElement;
        property: 'display' | 'visibility';
        value: string;
      }> = [];

      if (plan.content === 'canonical-text' || plan.content === 'canonical-shape' || plan.content === 'canonical-omit') {
        const legacyContent = findLegacyBaseContentNode(host);
        if (legacyContent) {
          restoredStyles.push({ node: legacyContent, property: 'display', value: legacyContent.style.display });
          // Removing the old flex item from layout is required: visibility:hidden
          // would keep its authored dimensions and offset the canonical sibling.
          legacyContent.style.setProperty('display', 'none');
        }
        host.setAttribute('data-preview-content-state-mode', plan.content);
      }

      if (plan.cursor === 'canonical-cursor' || plan.cursor === 'canonical-omit') {
        const legacyCursor = findLegacyCursorOverlay(host);
        if (legacyCursor) {
          restoredStyles.push({ node: legacyCursor, property: 'visibility', value: legacyCursor.style.visibility });
          legacyCursor.style.setProperty('visibility', 'hidden');
        }
        host.setAttribute('data-preview-cursor-state-mode', plan.cursor);
      }

      restorers.push(() => {
        restoreAttribute(host, 'data-preview-content-state-mode', previousContentMode);
        restoreAttribute(host, 'data-preview-cursor-state-mode', previousCursorMode);
        for (const { node, property, value } of restoredStyles) {
          if (value) node.style.setProperty(property, value);
          else node.style.removeProperty(property);
        }
      });
    }
    return () => restorers.reverse().forEach((restore) => restore());
  }, [canonicalPainterPlans, deterministicFrame, stage]);

  useLayoutEffect(() => {
    if (!stage || deterministicFrame === null) return;
    const previousReadiness = stage.getAttribute('data-preview-font-face-readiness');
    const previousCount = stage.getAttribute('data-preview-font-face-count');
    const previousBindingErrors = stage.getAttribute('data-preview-font-face-binding-errors');
    const previousRuntimeError = stage.getAttribute('data-preview-font-face-runtime-error');
    if (fontFaceRequired) {
      stage.setAttribute('data-preview-font-face-readiness', fontFaceStatus);
      stage.setAttribute('data-preview-font-face-count', String(fontFacePlan.bindings.length));
      if (fontFacePlan.errors.length > 0) {
        stage.setAttribute('data-preview-font-face-binding-errors', String(fontFacePlan.errors.length));
      } else {
        stage.removeAttribute('data-preview-font-face-binding-errors');
      }
      if (fontFaceStatus === 'failed') {
        stage.setAttribute('data-preview-font-face-runtime-error', 'font-face-load-failed');
      } else {
        stage.removeAttribute('data-preview-font-face-runtime-error');
      }
    } else {
      stage.removeAttribute('data-preview-font-face-readiness');
      stage.removeAttribute('data-preview-font-face-count');
      stage.removeAttribute('data-preview-font-face-binding-errors');
      stage.removeAttribute('data-preview-font-face-runtime-error');
    }
    return () => {
      restoreAttribute(stage, 'data-preview-font-face-readiness', previousReadiness);
      restoreAttribute(stage, 'data-preview-font-face-count', previousCount);
      restoreAttribute(stage, 'data-preview-font-face-binding-errors', previousBindingErrors);
      restoreAttribute(stage, 'data-preview-font-face-runtime-error', previousRuntimeError);
    };
  }, [deterministicFrame, fontFacePlan.bindings.length, fontFacePlan.errors.length, fontFaceRequired, fontFaceStatus, stage]);

  useLayoutEffect(() => {
    if (!stage) return;
    const previousConsumer = stage.getAttribute('data-preview-transition-pair-consumer');
    const previousRasterDeferred = stage.getAttribute('data-preview-transition-pair-weighted-raster-deferred');
    const previousRuntimeDeferred = stage.getAttribute('data-preview-transition-pair-runtime-deferred');
    if (consumeWeightedPairs) {
      stage.setAttribute('data-preview-transition-pair-consumer', 'canonical-weighted-canvas');
    }
    if (weightedRasterDeferredReasons.length > 0) {
      stage.setAttribute('data-preview-transition-pair-weighted-raster-deferred', weightedRasterDeferredReasons.join(','));
    } else {
      stage.removeAttribute('data-preview-transition-pair-weighted-raster-deferred');
    }
    if (runtimeDeferredReasons.length > 0) {
      stage.setAttribute('data-preview-transition-pair-runtime-deferred', runtimeDeferredReasons.join(','));
    } else {
      stage.removeAttribute('data-preview-transition-pair-runtime-deferred');
    }
    return () => {
      restoreAttribute(stage, 'data-preview-transition-pair-consumer', previousConsumer);
      restoreAttribute(stage, 'data-preview-transition-pair-weighted-raster-deferred', previousRasterDeferred);
      restoreAttribute(stage, 'data-preview-transition-pair-runtime-deferred', previousRuntimeDeferred);
    };
  }, [consumeWeightedPairs, runtimeDeferredReasons, stage, weightedRasterDeferredReasons]);

  useLayoutEffect(() => {
    if (!consumeWeightedPairs || !stage) return;
    const restorers: Array<() => void> = [];
    for (const slot of weightedSlots) {
      const lower = findPreviewClipNode(stage, slot.lower.clip.id);
      const upper = findPreviewClipNode(stage, slot.upper.clip.id);
      if (!lower || !upper) continue;
      const lowerStyle = lower.getAttribute('style');
      const upperStyle = upper.getAttribute('style');
      const lowerHost = lower.getAttribute('data-preview-weighted-pair-host');
      const childVisibility = [...lower.children].map((child) => ({
        child: child as HTMLElement,
        visibility: (child as HTMLElement).style.visibility,
      }));
      lower.setAttribute('data-preview-weighted-pair-host', slot.surface.transition_id);
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
        if (element.dataset.previewTransitionPairExecution !== 'weighted-canvas') {
          element.style.visibility = 'hidden';
        }
      }
      upper.style.visibility = 'hidden';
      restorers.push(() => {
        restoreAttribute(lower, 'style', lowerStyle);
        restoreAttribute(upper, 'style', upperStyle);
        restoreAttribute(lower, 'data-preview-weighted-pair-host', lowerHost);
        for (const { child, visibility } of childVisibility) child.style.visibility = visibility;
      });
    }
    return () => restorers.reverse().forEach((restore) => restore());
  }, [consumeWeightedPairs, stage, weightedSlots]);

  // Register before the weighted-Canvas readiness gate so a resumed font event
  // can still flow into weighted-pair readiness when both consumers are active.
  useLayoutEffect(() => {
    if (!fontFaceRequired || !stage || deterministicFrame === null) return;
    const onParityReady = (event: Event) => {
      const detail = (event as CustomEvent<Record<string, unknown>>).detail || {};
      if (detail.fontFaceResume === true) return;
      if (stage.dataset.previewFontFaceReadiness === 'ready') return;
      event.stopImmediatePropagation();
      if (stage.dataset.previewFontFaceReadiness === 'failed') {
        stage.setAttribute('data-preview-font-face-runtime-error', 'font-face-load-failed');
        return;
      }
      const deadline = performance.now() + 2000;
      const signalWhenSettled = () => {
        const readiness = stage.dataset.previewFontFaceReadiness;
        if (readiness === 'ready') {
          stage.removeAttribute('data-preview-font-face-runtime-error');
          window.dispatchEvent(new CustomEvent('omnillm:video-parity-ready', {
            detail: { ...detail, fontFaceResume: true },
          }));
          return;
        }
        if (readiness === 'failed') {
          stage.setAttribute('data-preview-font-face-runtime-error', 'font-face-load-failed');
          return;
        }
        if (performance.now() < deadline) {
          requestAnimationFrame(signalWhenSettled);
          return;
        }
        stage.setAttribute('data-preview-font-face-runtime-error', 'font-face-not-ready');
      };
      requestAnimationFrame(signalWhenSettled);
    };
    window.addEventListener('omnillm:video-parity-ready', onParityReady, true);
    return () => window.removeEventListener('omnillm:video-parity-ready', onParityReady, true);
  }, [deterministicFrame, fontFaceRequired, stage]);

  useLayoutEffect(() => {
    if (!consumeWeightedPairs || !stage) return;
    const previousRuntimeError = stage.getAttribute('data-preview-transition-pair-runtime-error');
    const onParityReady = (event: Event) => {
      const detail = (event as CustomEvent<Record<string, unknown>>).detail || {};
      if (detail.weightedCanvasResume === true) return;
      if (weightedPairSurfacesReady(stage, weightedSlots.length)) return;
      event.stopImmediatePropagation();
      const deadline = performance.now() + 2000;
      const signalWhenSettled = () => {
        if (weightedPairSurfacesReady(stage, weightedSlots.length)) {
          stage.removeAttribute('data-preview-transition-pair-runtime-error');
          window.dispatchEvent(new CustomEvent('omnillm:video-parity-ready', {
            detail: { ...detail, weightedCanvasResume: true },
          }));
          return;
        }
        if (performance.now() < deadline) {
          requestAnimationFrame(signalWhenSettled);
          return;
        }
        stage.setAttribute('data-preview-transition-pair-runtime-error', 'weighted-canvas-not-ready');
      };
      requestAnimationFrame(signalWhenSettled);
    };
    window.addEventListener('omnillm:video-parity-ready', onParityReady, true);
    return () => {
      window.removeEventListener('omnillm:video-parity-ready', onParityReady, true);
      restoreAttribute(stage, 'data-preview-transition-pair-runtime-error', previousRuntimeError);
    };
  }, [consumeWeightedPairs, stage, weightedSlots]);

  return (
    <div ref={rootRef} className="contents">
      <LegacyVideoPreviewCanvas />
      {consumeWeightedPairs && stage && weightedSlots.map((slot) => {
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
          />,
          host,
          `weighted-pair-${slot.surface.transition_id}`,
        );
      })}
      {stage && canonicalPainterPlans.flatMap(({ layer, plan }) => {
        const host = findPreviewClipNode(stage, layer.clip.id);
        if (!host) return [];
        const portals = [];
        const text = layer.canonicalState?.text;
        const resourceBackedTextPaint = Boolean(text?.font_resource_id) && (
          plan.content === 'canonical-text'
          || (plan.content === 'canonical-shape' && canonicalPreviewShapeUsesText(layer.canonicalState?.shape))
        );
        const fontBinding = fontFacePlan.bindingByClipId.get(layer.clip.id);
        const resourceFaceReady = !resourceBackedTextPaint || (fontFaceStatus === 'ready' && Boolean(fontBinding));
        const fontFamilyOverride = resourceFaceReady ? fontBinding?.familyAlias : undefined;

        if (plan.content === 'canonical-text' && text && resourceFaceReady) {
          portals.push(createPortal(
            <CanonicalPreviewText
              text={text}
              stageScale={stageScale}
              fontFamilyOverride={fontFamilyOverride}
            />,
            host,
            `canonical-text-${layer.clip.id}`,
          ));
        } else if (plan.content === 'canonical-shape' && layer.canonicalState?.shape) {
          portals.push(createPortal(
            <CanonicalPreviewShape
              shape={layer.canonicalState.shape}
              text={resourceFaceReady ? text : undefined}
              textFontFamilyOverride={fontFamilyOverride}
              stageScale={stageScale}
            />,
            host,
            `canonical-shape-${layer.clip.id}`,
          ));
        }
        if (plan.cursor === 'canonical-cursor' && layer.canonicalState?.cursor) {
          portals.push(createPortal(
            <CanonicalPreviewCursor cursor={layer.canonicalState.cursor} stageScale={stageScale} />,
            host,
            `canonical-cursor-${layer.clip.id}`,
          ));
        }
        return portals;
      })}
      <PreviewPixelateBackdropConsumer />
    </div>
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
    if (element.dataset.previewCanonicalContent) continue;
    if (element.dataset.previewCanonicalCursor === 'true') continue;
    if (findCursorPath(element)) continue;
    if (element.tagName === 'BUTTON') continue;
    return element;
  }
  return null;
}

function findLegacyCursorOverlay(host: HTMLElement): HTMLElement | null {
  for (const child of [...host.children]) {
    const element = child as HTMLElement;
    if (element.dataset.previewCanonicalCursor === 'true') continue;
    if (element.getAttribute('aria-hidden') !== 'true') continue;
    if (findCursorPath(element)) return element;
  }
  return null;
}

function findCursorPath(node: HTMLElement): SVGPathElement | null {
  for (const path of node.querySelectorAll<SVGPathElement>('svg[viewBox="0 0 16 16"] path')) {
    if (path.getAttribute('d') === 'M2 1 L2 12 L5.5 9.5 L7.5 14 L9.5 13 L7.5 8.8 L12 8.5 Z') return path;
  }
  return null;
}

function weightedPairSurfacesReady(stage: HTMLElement, expected: number): boolean {
  const surfaces = [...stage.querySelectorAll<HTMLElement>('[data-preview-transition-pair-execution="weighted-canvas"]')];
  return surfaces.length === expected
    && expected > 0
    && surfaces.every((surface) => surface.dataset.previewTransitionPairReady === 'true');
}

function restoreAttribute(node: HTMLElement, name: string, value: string | null): void {
  if (value === null) node.removeAttribute(name);
  else node.setAttribute(name, value);
}