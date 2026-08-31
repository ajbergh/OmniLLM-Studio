from pathlib import Path


def replace_once(path: str, old: str, new: str, label: str) -> None:
    file_path = Path(path)
    text = file_path.read_text()
    count = text.count(old)
    if count != 1:
        raise SystemExit(f'{label}: expected one match in {path}, found {count}')
    file_path.write_text(text.replace(old, new, 1))

replace_once(
    'frontend/src/components/video/previewFrameTransform.ts',
    """ * Explicit frame-addressed canonical state owns deterministic transform and\n * opacity decisions. Interactive/free-running playback keeps the established\n * property evaluator. An in-flight direct-manipulation gesture deliberately\n * bypasses canonical state so the existing responsive live overlay remains the\n * interaction authority until the gesture commits.\n""",
    """ * Frame-addressed canonical state owns transform and opacity decisions for both\n * deterministic capture and admitted media-only normal-playback frames. Missing\n * canonical projection retains the established time-domain property evaluator.\n * An in-flight direct-manipulation gesture deliberately bypasses canonical state\n * so the responsive live overlay remains the interaction authority until commit.\n""",
    'transform contract comment',
)
replace_once(
    'frontend/src/components/video/previewFrameViewTransform.ts',
    """ * Deterministic canonical frames already contain `view_transform`, so the\n * preview must not independently subtract/interpolate camera state again. The\n * established local camera subtraction remains the explicit compatibility path\n * for free-running playback, unavailable canonical projection, and live direct\n * manipulation while a gesture is in progress.\n""",
    """ * Canonical visual frames already contain `view_transform`, so the preview must\n * not independently subtract/interpolate camera state again, including admitted\n * media-only normal playback. Established local camera subtraction remains the\n * compatibility path for unavailable projection and live direct manipulation.\n""",
    'view transform contract comment',
)
replace_once(
    'frontend/src/components/video/previewFrameMediaGeometry.ts',
    """ * Resolve canonical media placement only for deterministic, non-interactive\n * preview frames. Free-running playback and direct manipulation/crop editing\n * intentionally retain the established browser painter until those paths are\n * canonicalized separately.\n */\nexport function resolveCanonicalPreviewMediaGeometry(\n  deterministicFrame: number | null,\n""",
    """ * Resolve canonical media placement for an admitted canonical visual frame.\n * Deterministic capture and media-only normal playback may both consume it;\n * direct manipulation/crop editing retain the established browser painter.\n */\nexport function resolveCanonicalPreviewMediaGeometry(\n  canonicalFrame: number | null,\n""",
    'media geometry comment and parameter',
)
replace_once(
    'frontend/src/components/video/previewFrameMediaGeometry.ts',
    "  if (deterministicFrame === null || !isMedia || hasLiveOverride || inCropEdit) return null;\n",
    "  if (canonicalFrame === null || !isMedia || hasLiveOverride || inCropEdit) return null;\n",
    'media geometry frame guard',
)
replace_once(
    'frontend/src/components/video/previewFrameMediaGeometry.test.ts',
    "  it('uses canonical geometry only for deterministic non-interactive media', () => {\n",
    "  it('uses canonical geometry only for an admitted non-interactive visual frame', () => {\n",
    'media geometry test title',
)
replace_once(
    'frontend/src/components/video/previewFramePerspectiveProjection.ts',
    """ * Decide whether one deterministic preview frame can leave the legacy shared\n * stage perspective and consume canonical per-layer projection instead.\n""",
    """ * Decide whether one admitted canonical visual frame can leave the legacy shared\n * stage perspective and consume canonical per-layer projection instead.\n""",
    'perspective contract comment',
)
replace_once(
    'frontend/src/components/video/previewFramePerspectiveProjection.ts',
    "  deterministicFrame: number | null,\n",
    "  canonicalFrame: number | null,\n",
    'perspective frame parameter',
)
replace_once(
    'frontend/src/components/video/previewFramePerspectiveProjection.ts',
    "  if (deterministicFrame === null || hasLiveOverride || canonicalStates.length === 0) return false;\n",
    "  if (canonicalFrame === null || hasLiveOverride || canonicalStates.length === 0) return false;\n",
    'perspective frame guard',
)
replace_once(
    'frontend/src/components/video/previewFrameTransitionPaint.ts',
    """ * Canonical omission is authoritative: an evaluated layer with no active\n * transition_paint returns identity canonical paint. Live manipulation stays\n * on the established interactive path until normal-playback canonicalization.\n""",
    """ * Canonical omission is authoritative: an evaluated layer with no active\n * transition_paint returns identity canonical paint. Live manipulation stays on\n * the established interactive path; admitted media-only normal-playback frames\n * may consume this same canonical paint contract.\n""",
    'transition paint contract comment',
)
replace_once(
    'frontend/src/components/video/previewFrameEffects.ts',
    """ * Legacy authored effects are consulted only when canonical FrameState itself\n * is unavailable (free-running playback or fail-closed projection fallback).\n""",
    """ * Legacy authored effects are consulted only when canonical FrameState itself\n * is unavailable through the explicit fail-closed projection fallback.\n""",
    'clip effect fallback comment',
)
replace_once(
    'frontend/src/components/video/previewFrameEffects.ts',
    """ * enabled scene effects; authored scene effects remain the free-running/fallback\n * path only when the top-level canonical frame itself is unavailable.\n""",
    """ * enabled scene effects; authored scene effects remain the explicit fallback path\n * only when the top-level canonical frame itself is unavailable.\n""",
    'scene effect fallback comment',
)
