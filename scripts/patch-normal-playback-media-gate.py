from pathlib import Path

path = Path('frontend/src/components/video/VideoPreviewCanvasLegacy.tsx')
text = path.read_text()


def replace_once(old: str, new: str, label: str) -> None:
    global text
    count = text.count(old)
    if count != 1:
        raise SystemExit(f'{label}: expected one match, found {count}')
    text = text.replace(old, new, 1)

replace_once(
    "import { resolvePreviewFrameViewTransform } from './previewFrameViewTransform';\n",
    "import { resolvePreviewFrameViewTransform } from './previewFrameViewTransform';\nimport { resolvePreviewPlaybackCanonicalization } from './previewPlaybackCanonicalization';\n",
    'playback decision import',
)

replace_once(
    """  const playbackCanonicalReady = playbackFrame !== null\n    && Boolean(frameQuery?.frameState)\n    && playbackTransitionPlan !== null\n    && playbackTransitionPlan.mode !== 'legacy'\n    && playbackTransitionPlan.mode !== 'canonical-weighted-deferred'\n    && playbackTransitionPlan.mode !== 'canonical-mixed'\n    && playbackTransitionPlan.deferredReasons.length === 0;\n  // Playback consumes canonical visual state only as an all-frame decision. If\n  // strict projection or exact transition composition is unavailable, retain\n  // the established time-domain painter for the complete visual frame.\n  const canonicalVisualFrame = deterministicFrame ?? (playbackCanonicalReady ? playbackFrame : null);\n""",
    """  const playbackDecision = resolvePreviewPlaybackCanonicalization(\n    playbackFrame,\n    frameQuery?.frameState,\n    playbackFrameVisualCandidates,\n    playbackTransitionPlan,\n  );\n  // Playback consumes canonical visual state only as an all-frame decision. If\n  // strict projection, an exact playback painter, or transition composition is\n  // unavailable, retain the established time-domain painter for the whole frame.\n  const canonicalVisualFrame = deterministicFrame ?? playbackDecision.canonicalFrame;\n""",
    'playback gate decision',
)

replace_once(
    """          data-preview-visual-frame-mode={deterministicFrame !== null\n            ? 'deterministic-canonical'\n            : isPlaying && canonicalVisualFrame !== null\n              ? 'canonical-playback'\n              : isPlaying && playbackFrame !== null\n                ? 'legacy-time-fallback'\n                : 'legacy-time'}\n          data-preview-visual-frame-index={canonicalVisualFrame ?? undefined}\n          data-preview-playback-frame-candidate={isPlaying && playbackFrame !== null ? playbackFrame : undefined}\n          data-preview-scene-effect-state-mode={sceneEffectPaint.mode}\n""",
    """          data-preview-visual-frame-mode={deterministicFrame !== null\n            ? 'deterministic-canonical'\n            : playbackDecision.mode}\n          data-preview-visual-frame-index={canonicalVisualFrame ?? undefined}\n          data-preview-playback-frame-candidate={isPlaying && playbackFrame !== null ? playbackFrame : undefined}\n          data-preview-playback-canonical-deferred={isPlaying ? playbackDecision.deferredReason : undefined}\n          data-preview-scene-effect-state-mode={sceneEffectPaint.mode}\n""",
    'playback diagnostics',
)

path.write_text(text)
