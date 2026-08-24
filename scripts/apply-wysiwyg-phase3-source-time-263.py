from pathlib import Path


def replace_once(text: str, old: str, new: str, label: str) -> str:
    count = text.count(old)
    if count != 1:
        raise RuntimeError(f"{label}: expected exactly one match, found {count}")
    return text.replace(old, new, 1)


canvas_path = Path("frontend/src/components/video/VideoPreviewCanvas.tsx")
canvas = canvas_path.read_text(encoding="utf-8")

canvas = replace_once(
    canvas,
    "import { evaluateCameraProperty, evaluateClipProperty } from '../../video/renderContractProperties';\n",
    "import { evaluateCameraProperty, evaluateClipProperty } from '../../video/renderContractProperties';\n"
    "import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';\n",
    "FrameState type import",
)
canvas = replace_once(
    canvas,
    "import { frameAddressMatchesTimelineMs, mediaSeekToleranceSeconds, sourceTimeForAddressMs } from './sourceTiming';\n",
    "import { frameAddressMatchesTimelineMs, mediaSeekToleranceSeconds, sourceTimeForPreviewMediaMs } from './sourceTiming';\n",
    "preview source-time import",
)
canvas = replace_once(
    canvas,
    "interface LayerEntry {\n"
    "  track: VideoTimelineTrack;\n"
    "  trackIndex: number;\n"
    "  clipIndex: number;\n"
    "  clip: VideoTimelineClip;\n"
    "  asset?: VideoAsset;\n"
    "}\n",
    "interface LayerEntry {\n"
    "  track: VideoTimelineTrack;\n"
    "  trackIndex: number;\n"
    "  clipIndex: number;\n"
    "  clip: VideoTimelineClip;\n"
    "  asset?: VideoAsset;\n"
    "  /** Exact canonical visual state, present only when frame projection succeeds. */\n"
    "  canonicalState?: CanonicalFrameLayerState;\n"
    "}\n",
    "LayerEntry canonical state",
)
canvas = replace_once(
    canvas,
    "  // Keep every mounted media element in sync with output-timeline time on every\n"
    "  // tick. Deterministic frame-addressed capture derives source time directly\n"
    "  // from output-frame identity; free-running playback keeps sub-frame playhead\n"
    "  // timing for responsiveness. Visual videos are muted; managed <audio>\n"
    "  // elements apply volume keyframes, fades, solo, and preview master gain.\n",
    "  // Keep every mounted media element in sync with output-timeline time on every\n"
    "  // tick. Deterministic visual media consumes canonical FrameState source time\n"
    "  // when the strict preview projection succeeds; free-running playback and the\n"
    "  // explicit compatibility fallback keep the established address evaluator.\n"
    "  // Audio remains outside visual FrameState until AudioGraph consumption lands.\n",
    "media synchronization comment",
)
canvas = replace_once(
    canvas,
    "    const syncElement = (element: HTMLMediaElement, clip: VideoTimelineClip) => {\n"
    "      const playbackRate = Math.min(4, Math.max(0.25, clip.playback_rate ?? 1));\n"
    "      const targetMs = sourceTimeForAddressMs(\n"
    "        address,\n"
    "        clip.start_ms,\n"
    "        clip.trim_in_ms ?? 0,\n"
    "        playbackRate,\n"
    "      );\n",
    "    const syncElement = (\n"
    "      element: HTMLMediaElement,\n"
    "      clip: VideoTimelineClip,\n"
    "      canonicalState?: CanonicalFrameLayerState,\n"
    "    ) => {\n"
    "      const playbackRate = Math.min(4, Math.max(0.25, clip.playback_rate ?? 1));\n"
    "      const targetMs = sourceTimeForPreviewMediaMs(\n"
    "        address,\n"
    "        canonicalState,\n"
    "        clip.start_ms,\n"
    "        clip.trim_in_ms ?? 0,\n"
    "        playbackRate,\n"
    "      );\n",
    "canonical media source-time resolver",
)
canvas = replace_once(
    canvas,
    "      if (entry) syncElement(video, entry.clip);\n",
    "      if (entry) syncElement(video, entry.clip, entry.canonicalState);\n",
    "visual video canonical source-time call",
)

canvas_path.write_text(canvas, encoding="utf-8")

plan_path = Path("docs/VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md")
plan = plan_path.read_text(encoding="utf-8")

old_handoff = """Latest merged WYSIWYG feature PR: **#261 — Add canonical preview composition projection** — squash merge `1fa4a2c9fb0ba02b00a194374dc363fe5f796199` (2026-08-24).

**Phase 2 — Canonical contract is complete. Phase 3 — Shared preview composition is active.** The renderer-independent visual contract (`visual-frame-state-v1` and its subcontracts) and audio contract (`audio-graph-v1`) define semantics; Phase 3 is migrating actual program-monitor consumers onto those canonical decisions in small, reversible slices.

Current implementation PR: **#262 — Verify canonical preview composition in parity diagnostics** on branch `feat/video-wysiwyg-phase3-preview-diagnostics`.

#262 was initially stacked on #261 while #261's final hosted matrix completed. After #261 passed its exact-head matrix and squash-merged, #262 was rebuilt from the actual new `main` (`1fa4a2c9fb0ba02b00a194374dc363fe5f796199`), its single diagnostics delta was reapplied, and the PR was retargeted to `main`.

#262 is evidence-only and deliberately does not change editor painting. It extends `scripts/video-frame-state-diagnostics.mjs` so every parity fixture sample evaluates both canonical FrameState diagnostics and `preview-composition-frame-v1` and fails if:

- canonical availability differs between FrameState and the preview projection;
- an available preview projection has a different ordered clip identity list than FrameState;
- the preview projection stops preserving the saved-timeline fail-closed / transition-free positive-control behavior.

The artifact envelope is versioned to 2 and records compact preview-composition contract, availability, ordered clip IDs, and error evidence beside the existing FrameState diagnostic output.

The normal free-running preview, camera evaluation, source seeks, transform/geometry/effect/text/shape/cursor painting, and Web Audio gain/fade scheduling are **not yet fully canonical consumers**. They remain the next Phase 3 implementation slices.
"""
new_handoff = """Latest merged WYSIWYG feature PR: **#262 — Verify canonical preview composition in parity diagnostics** — squash merge `3c2acfc9d5bc1fb1e97d3ef32c14d4707b62f8ec` (2026-08-24).

**Phase 2 — Canonical contract is complete. Phase 3 — Shared preview composition is active.** The renderer-independent visual contract (`visual-frame-state-v1` and its subcontracts) and audio contract (`audio-graph-v1`) define semantics; Phase 3 is migrating actual program-monitor consumers onto those canonical decisions in small, reversible slices.

Current implementation PR: **#263 — Consume canonical preview source time** on branch `feat/video-wysiwyg-phase3-canonical-source-time`, created directly from current `main` `3c2acfc9d5bc1fb1e97d3ef32c14d4707b62f8ec`.

#263 advances the first production consumer introduced by #261 instead of adding another independent timing interpretation:

- `sourceTimeForPreviewMediaMs` consumes `canonicalState.source_time_ms` for deterministic frame-addressed visual media when the strict preview projection succeeds;
- free-running playback remains time-addressed and sub-frame responsive;
- deterministic compatibility/fail-closed fallback keeps the established rational frame-address source-time helper when no canonical visual state is available;
- `VideoPreviewCanvas.tsx` passes the exact `canonicalState` carried by the mounted visual entry into media synchronization;
- managed audio deliberately does not consume visual FrameState and remains unchanged until AudioGraph runtime adoption.

Direct-manipulation gesture state, transforms/opacity painting, camera/projection, geometry, transitions/effects, text/shape/cursor painting, and Web Audio gain/fade scheduling are **not changed by #263**. They remain separate Phase 3 slices.
"""
plan = replace_once(plan, old_handoff, new_handoff, "current handoff")

plan = replace_once(
    plan,
    "| Phase 3 — Shared preview composition | **In progress** | #261 merged the canonical preview projection and deterministic visual activity consumer. #262 adds retained parity evidence that projection availability and ordered identities remain mechanically aligned with FrameState. Source-time/transform/camera/painter/audio consumption remains. |",
    "| Phase 3 — Shared preview composition | **In progress** | #261 merged canonical visual activity/projection consumption; #262 retained parity evidence for projection identity/availability; #263 routes deterministic visual-media source time through FrameState while preserving free-running/fallback/audio boundaries. Transform/camera/painter/audio consumption remains. |",
    "Phase 3 tracker row",
)

risk_anchor = "| Frame-addressed preview silently mixes canonical and fallback semantics | #261 attaches `canonicalState` only when the strict projection succeeds; fallback entries remain explicitly without it. |"
plan = replace_once(
    plan,
    risk_anchor,
    risk_anchor + "\n| Visual media recomputes source time after canonical FrameState evaluation | #263 consumes `canonicalState.source_time_ms` only for frame-addressed canonical visual entries; free-running/fallback paths retain `sourceTiming.ts`, and audio waits for AudioGraph. |",
    "source-time risk control",
)

old_log_tail = """- Draft #262 was rebuilt from that exact new `main`, retargeted to `main`, and reapplied only the diagnostics delta. It extends retained parity diagnostics to fail on preview-composition vs FrameState availability/order drift.
- Review of `backend/internal/video/parity_report.go` and `backend/cmd/video-parity-report/main.go` confirmed the Phase 0 structural-region gap: exact region metrics exist, but loaded parity frame pairs currently receive no regions from the CLI.
- This tracker commit creates a new exact #262 head; the complete required hosted matrix must pass before #262 is marked ready or merged.
"""
new_log_tail = """- #262 was rebuilt from the #261 merge, retargeted to `main`, and retained only its diagnostics/tracker delta. Exact head `64a9a034b6e77334980c87bfcbc73416e1fd927e` passed Quality Gate #1506, Security #1512, Playwright, deterministic renderer parity, CodeQL, and the platform/sandbox assurances, then squash-merged as `3c2acfc9d5bc1fb1e97d3ef32c14d4707b62f8ec`.
- Review of `backend/internal/video/parity_report.go` and `backend/cmd/video-parity-report/main.go` confirmed the Phase 0 structural-region gap: exact region metrics exist, but loaded parity frame pairs currently receive no regions from the CLI.
- Created `feat/video-wysiwyg-phase3-canonical-source-time` directly from merged #262 `main`. #263 adds a tested visual-media source-time bridge and wires mounted deterministic visual media to consume `canonicalState.source_time_ms`; free-running/fallback timing and AudioGraph-deferred audio remain unchanged.
- This tracker update moves #263 to its exact hosted validation gate; no transform/camera/painter/audio semantic expansion is bundled into this slice.
"""
plan = replace_once(plan, old_log_tail, new_log_tail, "implementation log tail")

old_next = """1. Finish exact-head CI/review for #262, verify `compare main...branch` contains only the diagnostics script and this tracker, and squash-merge only if current/scoped/green.
2. From resulting `main`, route deterministic media **source-time/seek** in `VideoPreviewCanvas.tsx` through `canonicalState.source_time_ms`, preserving the existing source-timing helper for free-running/fallback paths.
3. Next consume canonical evaluated transform/opacity state while preserving `liveTransform` as a direct-manipulation overlay; then continue with camera/projection, geometry, transitions/effects, text/shape/cursor, and AudioGraph scheduling in separate reviewable slices.
4. In parallel, implement Phase 0 structural-region policy/wiring and second-platform evidence rather than treating current global numeric thresholds as full visual sign-off."""
new_next = """1. Run exact-head Quality/Security/platform/Playwright/renderer-parity validation for #263, inspect `compare main...branch` and review threads, then squash-merge only if the source-time consumer remains current/scoped/green.
2. From resulting `main`, consume canonical evaluated **transform/opacity** state in `VideoPreviewCanvas.tsx` while preserving `liveTransform` as the direct-manipulation overlay and keeping crop-edit interaction responsive.
3. Continue camera/projection, geometry, transition/effect, text/shape/cursor, and AudioGraph consumption as separate reviewable slices rather than combining painter migration into one rewrite.
4. In parallel, wire Phase 0 structural-region policy/evidence and add second-platform parity evidence; current global numeric thresholds alone are not visual sign-off."""
plan = replace_once(plan, old_next, new_next, "next recommended slice")

plan_path.write_text(plan, encoding="utf-8")

print("patched VideoPreviewCanvas.tsx and WYSIWYG tracker")
