from pathlib import Path


def replace_once(path: str, old: str, new: str, label: str) -> None:
    file_path = Path(path)
    text = file_path.read_text()
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{label}: expected one match in {path}, found {count}")
    file_path.write_text(text.replace(old, new, 1))


# sourceTiming.ts: map the continuously moving UI playhead to the canonical output
# frame containing it, without changing the playhead itself.
replace_once(
    "frontend/src/components/video/sourceTiming.ts",
    "import { sourceTimeMs } from '../../video/renderContract';",
    "import { sourceTimeMs, startFrame } from '../../video/renderContract';",
    "sourceTiming import",
)
replace_once(
    "frontend/src/components/video/sourceTiming.ts",
    """export function frameAddressMatchesTimelineMs(\n  frameIndex: number,\n  fps: number,\n  timelineMs: number,\n): boolean {\n  if (frameIndex < 0 || fps <= 0 || !Number.isFinite(timelineMs)) return false;\n  return Math.abs(timelineMs - (Math.trunc(frameIndex) * 1000) / Math.trunc(fps)) <= 1e-6;\n}\n\n""",
    """export function frameAddressMatchesTimelineMs(\n  frameIndex: number,\n  fps: number,\n  timelineMs: number,\n): boolean {\n  if (frameIndex < 0 || fps <= 0 || !Number.isFinite(timelineMs)) return false;\n  return Math.abs(timelineMs - (Math.trunc(frameIndex) * 1000) / Math.trunc(fps)) <= 1e-6;\n}\n\n/**\n * Return the canonical output-frame identity containing a free-running visual\n * playhead. The UI/audio clock remains continuous; only visual evaluation is\n * projected into the same integer frame domain used by export. Invalid or\n * negative playheads fail closed so callers can retain their time-domain path.\n */\nexport function playbackVisualFrameIndex(timelineMs: number, fps: number): number | null {\n  const normalizedFPS = Math.trunc(fps);\n  if (!Number.isFinite(timelineMs) || timelineMs < 0 || !Number.isFinite(fps) || normalizedFPS <= 0) return null;\n  return startFrame(timelineMs, normalizedFPS);\n}\n\n""",
    "playback frame helper",
)
replace_once(
    "frontend/src/components/video/sourceTiming.ts",
    """ * A successful canonical frame-addressed preview projection already contains\n * the evaluated source time and therefore owns that deterministic decision.\n * Free-running playback and explicit compatibility/fail-closed fallback keep\n * using the established address evaluator. Audio intentionally does not pass a\n * visual FrameState here; its timing migrates with AudioGraph consumption.\n""",
    """ * A successful canonical frame-addressed preview projection already contains\n * the evaluated source time and therefore owns that visual decision, including\n * free-running playback after it has been projected into an authoritative output\n * frame. Explicit compatibility/fail-closed fallback keeps the established time\n * evaluator. Audio intentionally receives a separate time-domain address until\n * AudioGraph consumption lands.\n""",
    "source timing comment",
)

# sourceTiming.test.ts: lock the visual-frame mapping independently of the UI/audio clock.
replace_once(
    "frontend/src/components/video/sourceTiming.test.ts",
    """  mediaSeekToleranceSeconds,\n  sourceTimeForAddressMs,\n""",
    """  mediaSeekToleranceSeconds,\n  playbackVisualFrameIndex,\n  sourceTimeForAddressMs,\n""",
    "sourceTiming test import",
)
replace_once(
    "frontend/src/components/video/sourceTiming.test.ts",
    """  it('derives deterministic source time directly from output-frame identity', () => {\n""",
    """  it('maps a free-running visual playhead to the containing canonical output frame', () => {\n    const frameDurationMs = 1000 / 30;\n    expect(playbackVisualFrameIndex(0, 30)).toBe(0);\n    expect(playbackVisualFrameIndex(frameDurationMs - 0.0001, 30)).toBe(0);\n    expect(playbackVisualFrameIndex(frameDurationMs, 30)).toBe(1);\n    expect(playbackVisualFrameIndex(1000.5, 120)).toBe(120);\n    expect(playbackVisualFrameIndex(-0.001, 30)).toBeNull();\n    expect(playbackVisualFrameIndex(Number.NaN, 30)).toBeNull();\n    expect(playbackVisualFrameIndex(100, 0)).toBeNull();\n  });\n\n  it('derives deterministic source time directly from output-frame identity', () => {\n""",
    "sourceTiming playback test",
)
replace_once(
    "frontend/src/components/video/sourceTiming.test.ts",
    """  it('keeps free-running visual media on timeline-time evaluation', () => {\n""",
    """  it('keeps the explicit time-address compatibility path sub-frame responsive', () => {\n""",
    "sourceTiming compatibility test title",
)

# Legacy preview imports.
replace_once(
    "frontend/src/components/video/VideoPreviewCanvasLegacy.tsx",
    "import { resolvePreviewFrameEffectPaint } from './previewFrameEffects';",
    "import { resolvePreviewFrameEffectPaint, resolvePreviewFrameSceneEffectPaint } from './previewFrameEffects';",
    "legacy scene effect import",
)
replace_once(
    "frontend/src/components/video/VideoPreviewCanvasLegacy.tsx",
    "import { applyDecoderBudget, buildTimelineIntervalIndex, compareIndexedTimelineClipOrder, queryActiveClips, queryActiveClipsAtFrame } from './pro/timelineIndex';",
    "import { applyDecoderBudget, buildTimelineIntervalIndex, compareIndexedTimelineClipOrder, queryActiveClips, queryActiveClipsAtFrameWithState } from './pro/timelineIndex';",
    "legacy frame query import",
)
replace_once(
    "frontend/src/components/video/VideoPreviewCanvasLegacy.tsx",
    "import { deterministicVideoSeekTargetSeconds, frameAddressMatchesTimelineMs, mediaSeekToleranceSeconds, sourceTimeForPreviewMediaMs } from './sourceTiming';",
    "import { deterministicVideoSeekTargetSeconds, frameAddressMatchesTimelineMs, mediaSeekToleranceSeconds, playbackVisualFrameIndex, sourceTimeForPreviewMediaMs, type TimelineSourceAddress } from './sourceTiming';",
    "legacy source timing import",
)

old_active = """  const intervalIndex = useMemo(() => buildTimelineIntervalIndex(timeline, assets), [timeline, assets]);\n  const fps = timeline?.canvas.fps || 30;\n  const deterministicFrame = !isPlaying\n    && frameAddress !== null\n    && frameAddressMatchesTimelineMs(frameAddress, fps, playheadMs)\n    ? frameAddress\n    : null;\n  const activeIndexed = (deterministicFrame !== null\n    ? queryActiveClipsAtFrame(intervalIndex, deterministicFrame, fps)\n    : queryActiveClips(intervalIndex, playheadMs))\n    .filter(({ track }) => track.visible)\n    .sort(compareIndexedTimelineClipOrder);\n  const visualIndexed = activeIndexed.filter(({ clip, asset }) => (\n    !clip.audio_only && (Boolean(clip.text) || Boolean(clip.shape) || !asset || !asset.mime_type.startsWith('audio/'))\n  ));\n  const decoderLimit = Math.max(1, Math.min(12, Number(window.localStorage.getItem('omnillm-video-decoder-budget') || 4)));\n  const budgeted = applyDecoderBudget(visualIndexed, decoderLimit, selectedClipId);\n  const layers: LayerEntry[] = budgeted.mounted;\n  const posterLayers: LayerEntry[] = budgeted.posters;\n  const posterClipIds = new Set(posterLayers.map(({ clip }) => clip.id));\n  const previewLayers = [...layers, ...posterLayers]\n    .sort(compareIndexedTimelineClipOrder);\n  const transitionPairPlan = planPreviewFrameTransitionPairs(\n    deterministicFrame !== null && !liveTransform && !cropMode && !editingTextClipId\n      ? deterministicFrame\n      : null,\n    previewLayers,\n  );\n  const consumeSourceOverTransitionPairs = shouldConsumePreviewFrameSourceOverPairs(transitionPairPlan);\n  const useCanonicalPerspective = shouldUseCanonicalPreviewPerspective(\n    deterministicFrame,\n    previewLayers.map((entry) => entry.canonicalState),\n    Boolean(liveTransform),\n  );\n\n  // Audio uses the same interval index as visuals, eliminating full timeline\n  // scans and repeated asset lookups on every playhead update.\n  const audioLayers = activeIndexed\n    .filter(({ track }) => !track.muted && (!soloTrackId || track.id === soloTrackId))\n"""
new_active = """  const intervalIndex = useMemo(() => buildTimelineIntervalIndex(timeline, assets), [timeline, assets]);\n  const fps = timeline?.canvas.fps || 30;\n  const deterministicFrame = !isPlaying\n    && frameAddress !== null\n    && frameAddressMatchesTimelineMs(frameAddress, fps, playheadMs)\n    ? frameAddress\n    : null;\n  const playbackFrame = isPlaying ? playbackVisualFrameIndex(playheadMs, fps) : null;\n  const frameQuery = deterministicFrame !== null || playbackFrame !== null\n    ? queryActiveClipsAtFrameWithState(\n      intervalIndex,\n      deterministicFrame ?? playbackFrame ?? 0,\n      fps,\n    )\n    : null;\n  const frameIndexed = (frameQuery?.clips ?? [])\n    .filter(({ track }) => track.visible)\n    .sort(compareIndexedTimelineClipOrder);\n  const timeIndexed = queryActiveClips(intervalIndex, playheadMs)\n    .filter(({ track }) => track.visible)\n    .sort(compareIndexedTimelineClipOrder);\n  const playbackFrameVisualCandidates = playbackFrame !== null && frameQuery?.frameState\n    ? frameIndexed.filter(({ clip, asset }) => (\n      !clip.audio_only && (Boolean(clip.text) || Boolean(clip.shape) || !asset || !asset.mime_type.startsWith('audio/'))\n    ))\n    : [];\n  const playbackTransitionPlan = playbackFrame !== null && frameQuery?.frameState\n    ? planPreviewFrameTransitionPairs(playbackFrame, playbackFrameVisualCandidates)\n    : null;\n  const playbackCanonicalReady = playbackFrame !== null\n    && Boolean(frameQuery?.frameState)\n    && playbackTransitionPlan !== null\n    && playbackTransitionPlan.mode !== 'legacy'\n    && playbackTransitionPlan.mode !== 'canonical-weighted-deferred'\n    && playbackTransitionPlan.mode !== 'canonical-mixed'\n    && playbackTransitionPlan.deferredReasons.length === 0;\n  // Playback consumes canonical visual state only as an all-frame decision. If\n  // strict projection or exact transition composition is unavailable, retain\n  // the established time-domain painter for the complete visual frame.\n  const canonicalVisualFrame = deterministicFrame ?? (playbackCanonicalReady ? playbackFrame : null);\n  const activeVisualIndexed = canonicalVisualFrame !== null ? frameIndexed : timeIndexed;\n  const visualIndexed = activeVisualIndexed.filter(({ clip, asset }) => (\n    !clip.audio_only && (Boolean(clip.text) || Boolean(clip.shape) || !asset || !asset.mime_type.startsWith('audio/'))\n  ));\n  const decoderLimit = Math.max(1, Math.min(12, Number(window.localStorage.getItem('omnillm-video-decoder-budget') || 4)));\n  const budgeted = applyDecoderBudget(visualIndexed, decoderLimit, selectedClipId);\n  const layers: LayerEntry[] = budgeted.mounted;\n  const posterLayers: LayerEntry[] = budgeted.posters;\n  const posterClipIds = new Set(posterLayers.map(({ clip }) => clip.id));\n  const previewLayers = [...layers, ...posterLayers]\n    .sort(compareIndexedTimelineClipOrder);\n  const transitionPairPlan = planPreviewFrameTransitionPairs(\n    canonicalVisualFrame !== null && !liveTransform && !cropMode && !editingTextClipId\n      ? canonicalVisualFrame\n      : null,\n    previewLayers,\n  );\n  const consumeSourceOverTransitionPairs = shouldConsumePreviewFrameSourceOverPairs(transitionPairPlan);\n  const useCanonicalPerspective = shouldUseCanonicalPreviewPerspective(\n    canonicalVisualFrame,\n    previewLayers.map((entry) => entry.canonicalState),\n    Boolean(liveTransform),\n  );\n  const sceneEffectPaint = resolvePreviewFrameSceneEffectPaint(\n    canonicalVisualFrame !== null ? frameQuery?.frameState : undefined,\n    activeScene?.effects,\n  );\n\n  // Normal playback keeps audio activity on continuous playhead time even when\n  // visual layers consume an integer canonical output frame. Deterministic parity\n  // seeks retain their prior frame-addressed audio membership.\n  const audioIndexed = deterministicFrame !== null ? frameIndexed : timeIndexed;\n  const audioLayers = audioIndexed\n    .filter(({ track }) => !track.muted && (!soloTrackId || track.id === soloTrackId))\n"""
replace_once(
    "frontend/src/components/video/VideoPreviewCanvasLegacy.tsx",
    old_active,
    new_active,
    "legacy canonical playback query",
)

old_sync = """  // Keep every mounted media element in sync with output-timeline time on every\n  // tick. Deterministic visual media consumes canonical FrameState source time\n  // when the strict preview projection succeeds; free-running playback and the\n  // explicit compatibility fallback keep the established address evaluator.\n  // Audio remains outside visual FrameState until AudioGraph consumption lands.\n  useEffect(() => {\n    const address = deterministicFrame !== null\n      ? { kind: 'frame' as const, frameIndex: deterministicFrame, fps }\n      : { kind: 'time' as const, timelineMs: playheadMs };\n\n    const syncElement = (\n      element: HTMLMediaElement,\n      clip: VideoTimelineClip,\n      canonicalState?: CanonicalFrameLayerState,\n    ) => {\n      const playbackRate = Math.min(4, Math.max(0.25, clip.playback_rate ?? 1));\n      const targetMs = sourceTimeForPreviewMediaMs(\n        address,\n        canonicalState,\n        clip.start_ms,\n        clip.trim_in_ms ?? 0,\n        playbackRate,\n      );\n      const target = targetMs / 1000;\n      element.playbackRate = playbackRate;\n      element.preservesPitch = true;\n      if (isPlaying) {\n        if (element instanceof HTMLVideoElement) resetPreviewVideoPresentation(element);\n        if (element.paused && !element.ended) {\n          element.currentTime = target;\n          element.play().catch(() => { /* autoplay policy */ });\n        } else if (Math.abs(element.currentTime - target) > 0.35) {\n          // Drift correction (tab throttling, slow decode).\n          element.currentTime = target;\n        }\n      } else {\n        if (!element.paused) element.pause();\n        if (element instanceof HTMLVideoElement && address.kind === 'frame' && canonicalState) {\n          ensurePreviewVideoPresentation({\n            video: element,\n            token: previewVideoPresentationToken(clip.id, address.frameIndex, targetMs),\n            sourceTimeMs: targetMs,\n            seekSeconds: deterministicVideoSeekTargetSeconds(address, target),\n            toleranceSeconds: mediaSeekToleranceSeconds(address),\n          });\n          return;\n        }\n        if (element instanceof HTMLVideoElement) resetPreviewVideoPresentation(element);\n        if (Math.abs(element.currentTime - target) > mediaSeekToleranceSeconds(address)) {\n          element.currentTime = element instanceof HTMLVideoElement\n            ? deterministicVideoSeekTargetSeconds(address, target)\n            : target;\n        }\n      }\n    };\n    for (const [clipId, video] of videoRefs.current) {\n      const entry = layersRef.current.find((layer) => layer.clip.id === clipId);\n      if (entry) syncElement(video, entry.clip, entry.canonicalState);\n    }\n    for (const [clipId, audio] of audioRefs.current) {\n      const entry = audioLayersRef.current.find((layer) => layer.clip.id === clipId);\n      if (!entry) continue;\n      const clipTimeMs = Math.max(0, playheadMs - entry.clip.start_ms);\n      const clipVolume = evaluateClipProperty(entry.clip, 'volume', clipTimeMs);\n      // Element volume caps at 1; gains above unity remain export-only.\n      audio.volume = Math.min(1, Math.max(0, clipVolume * fadeFactor(entry.clip, playheadMs) * previewVolume));\n      syncElement(audio, entry.clip);\n    }\n  }, [deterministicFrame, fps, playheadMs, isPlaying, previewVolume]);\n"""
new_sync = """  // Keep every mounted media element in sync with output-timeline time on every\n  // tick. Canonical visual playback addresses media by output frame when strict\n  // projection succeeds, while the UI and audio clocks remain continuous. Exact\n  // paused video presentation proof remains limited to deterministic parity seeks.\n  useEffect(() => {\n    const visualAddress: TimelineSourceAddress = canonicalVisualFrame !== null\n      ? { kind: 'frame', frameIndex: canonicalVisualFrame, fps }\n      : { kind: 'time', timelineMs: playheadMs };\n    const audioAddress: TimelineSourceAddress = deterministicFrame !== null\n      ? { kind: 'frame', frameIndex: deterministicFrame, fps }\n      : { kind: 'time', timelineMs: playheadMs };\n\n    const syncElement = (\n      element: HTMLMediaElement,\n      clip: VideoTimelineClip,\n      address: TimelineSourceAddress,\n      canonicalState?: CanonicalFrameLayerState,\n    ) => {\n      const playbackRate = Math.min(4, Math.max(0.25, clip.playback_rate ?? 1));\n      const targetMs = sourceTimeForPreviewMediaMs(\n        address,\n        canonicalState,\n        clip.start_ms,\n        clip.trim_in_ms ?? 0,\n        playbackRate,\n      );\n      const target = targetMs / 1000;\n      element.playbackRate = playbackRate;\n      element.preservesPitch = true;\n      if (isPlaying) {\n        if (element instanceof HTMLVideoElement) resetPreviewVideoPresentation(element);\n        if (element.paused && !element.ended) {\n          element.currentTime = target;\n          element.play().catch(() => { /* autoplay policy */ });\n        } else if (Math.abs(element.currentTime - target) > 0.35) {\n          // Drift correction (tab throttling, slow decode).\n          element.currentTime = target;\n        }\n      } else {\n        if (!element.paused) element.pause();\n        if (element instanceof HTMLVideoElement && address.kind === 'frame' && canonicalState) {\n          ensurePreviewVideoPresentation({\n            video: element,\n            token: previewVideoPresentationToken(clip.id, address.frameIndex, targetMs),\n            sourceTimeMs: targetMs,\n            seekSeconds: deterministicVideoSeekTargetSeconds(address, target),\n            toleranceSeconds: mediaSeekToleranceSeconds(address),\n          });\n          return;\n        }\n        if (element instanceof HTMLVideoElement) resetPreviewVideoPresentation(element);\n        if (Math.abs(element.currentTime - target) > mediaSeekToleranceSeconds(address)) {\n          element.currentTime = element instanceof HTMLVideoElement\n            ? deterministicVideoSeekTargetSeconds(address, target)\n            : target;\n        }\n      }\n    };\n    for (const [clipId, video] of videoRefs.current) {\n      const entry = layersRef.current.find((layer) => layer.clip.id === clipId);\n      if (entry) syncElement(video, entry.clip, visualAddress, entry.canonicalState);\n    }\n    for (const [clipId, audio] of audioRefs.current) {\n      const entry = audioLayersRef.current.find((layer) => layer.clip.id === clipId);\n      if (!entry) continue;\n      const clipTimeMs = Math.max(0, playheadMs - entry.clip.start_ms);\n      const clipVolume = evaluateClipProperty(entry.clip, 'volume', clipTimeMs);\n      // Element volume caps at 1; gains above unity remain export-only.\n      audio.volume = Math.min(1, Math.max(0, clipVolume * fadeFactor(entry.clip, playheadMs) * previewVolume));\n      syncElement(audio, entry.clip, audioAddress);\n    }\n  }, [canonicalVisualFrame, deterministicFrame, fps, playheadMs, isPlaying, previewVolume]);\n"""
replace_once(
    "frontend/src/components/video/VideoPreviewCanvasLegacy.tsx",
    old_sync,
    new_sync,
    "legacy media synchronization",
)
replace_once(
    "frontend/src/components/video/VideoPreviewCanvasLegacy.tsx",
    """    const canonicalMediaGeometry = resolveCanonicalPreviewMediaGeometry(\n      deterministicFrame,\n""",
    """    const canonicalMediaGeometry = resolveCanonicalPreviewMediaGeometry(\n      canonicalVisualFrame,\n""",
    "legacy media geometry frame",
)
replace_once(
    "frontend/src/components/video/VideoPreviewCanvasLegacy.tsx",
    """          data-parity-frame-index={deterministicFrame ?? Math.floor((playheadMs * fps) / 1000)}\n          data-parity-time-ms={Math.round(playheadMs)}\n          data-preview-perspective-mode={useCanonicalPerspective ? 'canonical-per-layer' : 'legacy-shared'}\n""",
    """          data-parity-frame-index={canonicalVisualFrame ?? Math.floor((playheadMs * fps) / 1000)}\n          data-parity-time-ms={Math.round(playheadMs)}\n          data-preview-visual-frame-mode={deterministicFrame !== null\n            ? 'deterministic-canonical'\n            : isPlaying && canonicalVisualFrame !== null\n              ? 'canonical-playback'\n              : isPlaying && playbackFrame !== null\n                ? 'legacy-time-fallback'\n                : 'legacy-time'}\n          data-preview-visual-frame-index={canonicalVisualFrame ?? undefined}\n          data-preview-playback-frame-candidate={isPlaying && playbackFrame !== null ? playbackFrame : undefined}\n          data-preview-scene-effect-state-mode={sceneEffectPaint.mode}\n          data-preview-perspective-mode={useCanonicalPerspective ? 'canonical-per-layer' : 'legacy-shared'}\n""",
    "legacy playback diagnostics",
)
replace_once(
    "frontend/src/components/video/VideoPreviewCanvasLegacy.tsx",
    """            filter: composePreviewFilter(activeScene?.effects),\n""",
    """            filter: sceneEffectPaint.filter,\n""",
    "legacy canonical scene effects",
)
replace_once(
    "frontend/src/components/video/VideoPreviewCanvasLegacy.tsx",
    """ * elements synchronized to the timeline.\n */\n""",
    """ * elements synchronized to the timeline. During normal playback, visual layers\n * consume authoritative canonical output-frame state when the complete frame can\n * be represented exactly; unsupported projection/transition cases automatically\n * fall back to the established continuous-time painter.\n */\n""",
    "legacy top comment",
)

# Update helper comments/parameter naming now that exact canonical frames can be
# consumed during playback as well as deterministic paused capture.
replace_once(
    "frontend/src/components/video/previewFrameTransform.ts",
    """ * Explicit frame-addressed canonical state owns deterministic transform and\n * opacity decisions. Interactive/free-running playback keeps the established\n * property evaluator. An in-flight direct-manipulation gesture deliberately\n * bypasses canonical state so the existing responsive live overlay remains the\n * interaction authority until the gesture commits.\n""",
    """ * Frame-addressed canonical state owns transform and opacity decisions for both\n * deterministic capture and admitted normal-playback frames. A missing canonical\n * projection retains the established time-domain property evaluator. An in-flight\n * direct-manipulation gesture deliberately bypasses canonical state so the existing\n * responsive live overlay remains the interaction authority until commit.\n""",
    "transform comment",
)
replace_once(
    "frontend/src/components/video/previewFrameViewTransform.ts",
    """ * Deterministic canonical frames already contain `view_transform`, so the\n * preview must not independently subtract/interpolate camera state again. The\n * established local camera subtraction remains the explicit compatibility path\n * for free-running playback, unavailable canonical projection, and live direct\n * manipulation while a gesture is in progress.\n""",
    """ * Canonical visual frames already contain `view_transform`, so the preview must\n * not independently subtract/interpolate camera state again, including during\n * admitted normal playback. The established local camera subtraction remains the\n * explicit compatibility path for unavailable projection and live direct\n * manipulation while a gesture is in progress.\n""",
    "view transform comment",
)
replace_once(
    "frontend/src/components/video/previewFrameMediaGeometry.ts",
    """ * Resolve canonical media placement only for deterministic, non-interactive\n * preview frames. Free-running playback and direct manipulation/crop editing\n * intentionally retain the established browser painter until those paths are\n * canonicalized separately.\n */\nexport function resolveCanonicalPreviewMediaGeometry(\n  deterministicFrame: number | null,\n""",
    """ * Resolve canonical media placement for an admitted canonical visual frame.\n * Deterministic capture and normal playback may both consume it; direct\n * manipulation/crop editing intentionally retain the established browser painter.\n */\nexport function resolveCanonicalPreviewMediaGeometry(\n  canonicalFrame: number | null,\n""",
    "media geometry comment and parameter",
)
replace_once(
    "frontend/src/components/video/previewFrameMediaGeometry.ts",
    """  if (deterministicFrame === null || !isMedia || hasLiveOverride || inCropEdit) return null;\n""",
    """  if (canonicalFrame === null || !isMedia || hasLiveOverride || inCropEdit) return null;\n""",
    "media geometry guard",
)
replace_once(
    "frontend/src/components/video/previewFrameMediaGeometry.test.ts",
    """  it('uses canonical geometry only for deterministic non-interactive media', () => {\n""",
    """  it('uses canonical geometry only for an admitted non-interactive visual frame', () => {\n""",
    "media geometry test title",
)
replace_once(
    "frontend/src/components/video/previewFramePerspectiveProjection.ts",
    """ * Decide whether one deterministic preview frame can leave the legacy shared\n * stage perspective and consume canonical per-layer projection instead.\n""",
    """ * Decide whether one admitted canonical visual frame can leave the legacy shared\n * stage perspective and consume canonical per-layer projection instead.\n""",
    "perspective comment",
)
replace_once(
    "frontend/src/components/video/previewFramePerspectiveProjection.ts",
    """  deterministicFrame: number | null,\n""",
    """  canonicalFrame: number | null,\n""",
    "perspective parameter",
)
replace_once(
    "frontend/src/components/video/previewFramePerspectiveProjection.ts",
    """  if (deterministicFrame === null || hasLiveOverride || canonicalStates.length === 0) return false;\n""",
    """  if (canonicalFrame === null || hasLiveOverride || canonicalStates.length === 0) return false;\n""",
    "perspective guard",
)
replace_once(
    "frontend/src/components/video/previewFrameTransitionPaint.ts",
    """ * Legacy authored effects are consulted only when canonical FrameState itself\n * is unavailable (free-running playback or fail-closed projection fallback).\n""",
    """ * Legacy authored effects are consulted only when canonical FrameState itself\n * is unavailable through the explicit fail-closed projection fallback.\n""",
    "effect comment marker",
)
replace_once(
    "frontend/src/components/video/previewFrameTransitionPaint.ts",
    """ * Canonical omission is authoritative: an evaluated layer with no active\n * transition_paint returns identity canonical paint. Live manipulation stays\n * on the established interactive path until normal-playback canonicalization.\n""",
    """ * Canonical omission is authoritative: an evaluated layer with no active\n * transition_paint returns identity canonical paint. Live manipulation stays on\n * the established interactive path; admitted normal-playback frames may consume\n * this same canonical paint contract.\n""",
    "transition paint comment",
)

# previewFrameEffects.ts owns the comment replaced above, not transition paint.
# Correct that replacement here if the marker was not present in transition file.
