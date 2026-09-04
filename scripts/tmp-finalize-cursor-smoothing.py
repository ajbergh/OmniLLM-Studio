from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text()
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: anchor count={count}, want 1 for {old[:120]!r}")
    p.write_text(text.replace(old, new, 1))


def replace_between(path: str, start: str, end: str, body: str) -> None:
    p = Path(path)
    text = p.read_text()
    start_index = text.find(start)
    if start_index < 0:
        raise SystemExit(f"{path}: start anchor not found: {start!r}")
    end_index = text.find(end, start_index + len(start))
    if end_index < 0:
        raise SystemExit(f"{path}: end anchor not found: {end!r}")
    p.write_text(text[:start_index] + start + body + text[end_index:])


# Preserve v1 no-paint semantics for explicit hidden cursors and document v1/v2 sampling.
replace_once(
    "backend/internal/video/renderer_cursor.go",
    "// click state come from cursor-state-v1 at the exact rational presentation\n// time; the generated full-canvas sprite then inherits the owner's static",
    "// click state come from cursor-state-v1 or cursor-state-v2 at the exact rational\n// presentation time; the generated full-canvas sprite then inherits the owner's static",
)
replace_once(
    "backend/internal/video/renderer_cursor.go",
    "\tif clip.Cursor == nil || len(clip.Cursor.Events) == 0 || clip.DurationMS <= 0 {\n\t\treturn nil, true\n\t}\n\tif clip.AssetID == \"\" || clip.Text != nil || clip.Shape != nil || clip.AudioOnly {",
    "\tif clip.Cursor == nil || len(clip.Cursor.Events) == 0 || clip.DurationMS <= 0 {\n\t\treturn nil, true\n\t}\n\t// Timeline v1 stores visibility as a bool, so false is an explicit no-paint\n\t// state on this adapter boundary. Do not convert it to the v2 omitted/default\n\t// visibility semantics used by the renderer-independent contract.\n\tif !clip.Cursor.Visible {\n\t\treturn nil, true\n\t}\n\tif clip.AssetID == \"\" || clip.Text != nil || clip.Shape != nil || clip.AudioOnly {",
)

hidden_test = r'''func TestCanonicalCursorRasterKeepsHiddenCursorNoPaint(t *testing.T) {
	doc := NewEmptyTimeline(640, 360, 30)
	doc.DurationMS = 1000
	doc.Tracks[0].Clips = []TimelineClip{{
		ID: "cursor-owner", AssetID: "media", StartMS: 0, DurationMS: 1000, TrimOutMS: 1000,
		Transform: map[string]any{"x": 0.0, "y": 0.0, "scale": 1.0, "rotation": 0.0, "opacity": 1.0},
		Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}, Transitions: []TimelineTransition{},
		Cursor: &TimelineCursor{Visible: false, Scale: 1, Events: []TimelineCursorEvent{
			{TimeMS: 0, X: 100, Y: 100},
			{TimeMS: 999, X: 500, Y: 260},
		}},
	}}

	expanded := ExpandTimelineForFidelity(doc, 30, 120)
	if len(expanded.Tracks[0].Clips) != 1 {
		t.Fatalf("hidden cursor emitted render-only overlays: %+v", expanded.Tracks[0].Clips)
	}
	if expanded.Tracks[0].Clips[0].ID != "cursor-owner" {
		t.Fatalf("hidden cursor changed owner clip: %+v", expanded.Tracks[0].Clips[0])
	}
}

'''
replace_once(
    "backend/internal/video/renderer_cursor_test.go",
    "func TestCanonicalCursorRasterSupportsSmoothstepV2(t *testing.T) {",
    hidden_test + "func TestCanonicalCursorRasterSupportsSmoothstepV2(t *testing.T) {",
)

# Playback admission must bind authored smoothing to the matching computed contract version.
replace_once(
    "frontend/src/components/video/previewCursorPlayback.ts",
    "import { endFrame, startFrame } from '../../video/renderContract';\n",
    "import { endFrame, startFrame } from '../../video/renderContract';\nimport { CURSOR_STATE_CONTRACT_V1, CURSOR_STATE_CONTRACT_V2 } from '../../video/renderContractCursor';\n",
)
replace_once(
    "frontend/src/components/video/previewCursorPlayback.ts",
    "  if (!layer.canonicalState?.cursor) return `${clip.id}:canonical-cursor-state-unavailable`;\n  return undefined;",
    "  const canonicalCursor = layer.canonicalState?.cursor;\n  if (!canonicalCursor) return `${clip.id}:canonical-cursor-state-unavailable`;\n  const expectedContract = cursor.smoothing === true ? CURSOR_STATE_CONTRACT_V2 : CURSOR_STATE_CONTRACT_V1;\n  if (canonicalCursor.contract_version !== expectedContract) {\n    return `${clip.id}:canonical-cursor-contract-mismatch`;\n  }\n  return undefined;",
)
contract_mismatch_test = r'''
  it('fails closed when authored smoothing and computed cursor contract versions disagree', () => {
    const smoothedOwner = clip({ cursor: { ...clip().cursor!, smoothing: true } });
    const smoothedWithV1 = layer(smoothedOwner);
    expect(previewCursorPlaybackStructuralDeferredReason(smoothedWithV1, context()))
      .toBe('cursor-owner:canonical-cursor-contract-mismatch');

    const linearWithV2 = layer();
    linearWithV2.canonicalState = {
      cursor: {
        contract_version: CURSOR_STATE_CONTRACT_V2,
        visible: true,
        scale: 1,
        highlight: true,
        click_rings: true,
        x: 160,
        y: 120,
        click: false,
      },
    };
    expect(previewCursorPlaybackStructuralDeferredReason(linearWithV2, context()))
      .toBe('cursor-owner:canonical-cursor-contract-mismatch');
  });
'''
replace_once(
    "frontend/src/components/video/previewCursorPlayback.test.ts",
    "\n  it('retains the renderer exact-frame and expansion bounds', () => {",
    contract_mismatch_test + "\n  it('retains the renderer exact-frame and expansion bounds', () => {",
)

# Retained live playback evidence v6: add an asymmetric smoothstep cursor window.
replace_once(
    "backend/internal/video/parity_fixture_playback.go",
    'const PlaybackCanonicalParityFixtureName = "parity-playback-canonical-v5"',
    'const PlaybackCanonicalParityFixtureName = "parity-playback-canonical-v6"',
)
replace_once(
    "backend/internal/video/parity_fixture_playback.go",
    "\tRequireCursorMotion      bool     `json:\"require_cursor_motion,omitempty\"`\n",
    "\tRequireCursorMotion      bool     `json:\"require_cursor_motion,omitempty\"`\n\tRequireCursorSmoothing   bool     `json:\"require_cursor_smoothing,omitempty\"`\n",
)
replace_once(
    "backend/internal/video/parity_fixture_playback.go",
    "// Mixed v5 cases explicitly prove that supported media/text/cursor and\n",
    "// Mixed v6 cases explicitly prove that supported media/text/cursor and\n",
)
replace_once(
    "backend/internal/video/parity_fixture_playback.go",
    "\tdoc.DurationMS = 38000\n",
    "\tdoc.DurationMS = 39600\n",
)
smoothing_clip = r'''
	cursorSmoothing := mediaClip("playback-cursor-smoothing", "asset-landscape", 38000, 1200)
	cursorSmoothing.Cursor = &TimelineCursor{
		Visible:    true,
		Scale:      1,
		Highlight:  true,
		ClickRings: false,
		Smoothing:  true,
		Events: []TimelineCursorEvent{
			{TimeMS: 0, X: 120, Y: 90},
			{TimeMS: 1000, X: 520, Y: 270},
		},
	}

'''
replace_once(
    "backend/internal/video/parity_fixture_playback.go",
    "\n\taudio := TimelineTrack{",
    "\n" + smoothing_clip + "\taudio := TimelineTrack{",
)
replace_once(
    "backend/internal/video/parity_fixture_playback.go",
    "\t\ttrack(\"track-cursor-unsupported-fade\", unsupportedCursor),\n\t\taudio,",
    "\t\ttrack(\"track-cursor-unsupported-fade\", unsupportedCursor),\n\t\ttrack(\"track-cursor-smoothing\", cursorSmoothing),\n\t\taudio,",
)
replace_once(
    "backend/internal/video/parity_fixture_playback.go",
    "\t\t{Name: \"unsupported-cursor-fade-fallback\", FrameIndex: 1092, ObserveMS: 300, ExpectedMode: \"legacy-time-fallback\", ExpectedReason: \"cursor-playback-deferred:playback-cursor-unsupported-fade:fade-unsupported\", ExpectedTransitionMode: \"legacy\", ExpectedCursorConsumer: \"legacy-time-fallback\", ExpectedCursorClipID: \"playback-cursor-unsupported-fade\", RequireCursorSurface: true},\n",
    "\t\t{Name: \"unsupported-cursor-fade-fallback\", FrameIndex: 1092, ObserveMS: 300, ExpectedMode: \"legacy-time-fallback\", ExpectedReason: \"cursor-playback-deferred:playback-cursor-unsupported-fade:fade-unsupported\", ExpectedTransitionMode: \"legacy\", ExpectedCursorConsumer: \"legacy-time-fallback\", ExpectedCursorClipID: \"playback-cursor-unsupported-fade\", RequireCursorSurface: true},\n\t\t{Name: \"cursor-smoothing-v2-canonical-playback\", FrameIndex: 1146, ObserveMS: 600, ExpectedMode: \"canonical-playback\", ExpectedTransitionMode: \"canonical-none\", ExpectedCursorConsumer: \"canonical-inline\", ExpectedCursorClipID: \"playback-cursor-smoothing\", RequireCursorSurface: true, RequireCursorMotion: true, RequireCursorSmoothing: true, RequireCursorHighlight: true, RequireAdvancingFrames: true},\n",
)

replace_once(
    "backend/internal/video/parity_fixture_playback_test.go",
    "\tif validated.DurationMS != 38000 || validated.Canvas.FPS != 30 {",
    "\tif validated.DurationMS != 39600 || validated.Canvas.FPS != 30 {",
)
replace_once(
    "backend/internal/video/parity_fixture_playback_test.go",
    "\tif len(cases) != 18 {\n\t\tt.Fatalf(\"playback parity cases = %d, want 17\", len(cases))\n\t}",
    "\tif len(cases) != 19 {\n\t\tt.Fatalf(\"playback parity cases = %d, want 19\", len(cases))\n\t}",
)
replace_once(
    "backend/internal/video/parity_fixture_playback_test.go",
    "\t\tif (testCase.RequireCursorMotion || testCase.RequireCursorHighlight || testCase.RequireCursorClickToggle) && !testCase.RequireCursorSurface {\n\t\t\tt.Fatalf(\"cursor behavior case %q must require a cursor surface\", testCase.Name)\n\t\t}\n",
    "\t\tif (testCase.RequireCursorMotion || testCase.RequireCursorSmoothing || testCase.RequireCursorHighlight || testCase.RequireCursorClickToggle) && !testCase.RequireCursorSurface {\n\t\t\tt.Fatalf(\"cursor behavior case %q must require a cursor surface\", testCase.Name)\n\t\t}\n\t\tif testCase.RequireCursorSmoothing && (testCase.ExpectedMode != \"canonical-playback\" || !testCase.RequireCursorMotion) {\n\t\t\tt.Fatalf(\"cursor smoothing case %q must require canonical moving cursor evidence\", testCase.Name)\n\t\t}\n",
)
replace_once(
    "backend/internal/video/parity_fixture_playback_test.go",
    "\t\t\"unsupported-cursor-fade-fallback\",\n",
    "\t\t\"unsupported-cursor-fade-fallback\",\n\t\t\"cursor-smoothing-v2-canonical-playback\",\n",
)

# Independent playback evidence helper mirrors the documented v2 smoothstep rule without calling production code.
replace_once(
    "scripts/video-playback-parity-capture.mjs",
    "        const span = Math.max(1, next.time_ms - previous.time_ms);\n        const progress = (numerator - previous.time_ms * denominator) / (span * denominator);\n        x = previous.x + (next.x - previous.x) * progress;\n        y = previous.y + (next.y - previous.y) * progress;",
    "        const span = Math.max(1, next.time_ms - previous.time_ms);\n        const linearProgress = (numerator - previous.time_ms * denominator) / (span * denominator);\n        linearX = previous.x + (next.x - previous.x) * linearProgress;\n        linearY = previous.y + (next.y - previous.y) * linearProgress;\n        const progress = cursor.smoothing === true\n          ? linearProgress * linearProgress * (3 - 2 * linearProgress)\n          : linearProgress;\n        x = previous.x + (next.x - previous.x) * progress;\n        y = previous.y + (next.y - previous.y) * progress;",
)
replace_once(
    "scripts/video-playback-parity-capture.mjs",
    "  let previous = events[0];\n  let x = previous.x;\n  let y = previous.y;",
    "  let previous = events[0];\n  let x = previous.x;\n  let y = previous.y;\n  let linearX = x;\n  let linearY = y;",
)
replace_once(
    "scripts/video-playback-parity-capture.mjs",
    "    if (previous) { x = previous.x; y = previous.y; }\n",
    "    if (previous) { x = previous.x; y = previous.y; linearX = x; linearY = y; }\n",
)
replace_once(
    "scripts/video-playback-parity-capture.mjs",
    "    click_rings: cursor.click_rings === true,\n  };",
    "    click_rings: cursor.click_rings === true,\n    smoothing: cursor.smoothing === true,\n    linear_x: linearX,\n    linear_y: linearY,\n  };",
)
smoothing_evidence = r'''    if (testCase.require_cursor_smoothing) {
      let discriminatingRows = 0;
      for (const row of stable) {
        const cursor = row.cursor_surfaces.find((surface) => surface.clip_id === testCase.expected_cursor_clip_id);
        const expected = expectedCursorSampleAtFrame(timeline, testCase.expected_cursor_clip_id, row.visual_frame_index);
        if (!cursor || !expected?.smoothing) continue;
        const divergence = Math.max(Math.abs(expected.x - expected.linear_x), Math.abs(expected.y - expected.linear_y));
        if (divergence < 5) continue;
        discriminatingRows += 1;
        const observedLinearDistance = Math.max(Math.abs(cursor.x - expected.linear_x), Math.abs(cursor.y - expected.linear_y));
        if (observedLinearDistance < 4) {
          errors.push(`smoothed cursor remained too close to linear position at frame ${row.visual_frame_index}`);
          break;
        }
      }
      if (discriminatingRows === 0) errors.push('cursor smoothing evidence never crossed an asymmetric smoothstep sample');
    }
'''
replace_once(
    "scripts/video-playback-parity-capture.mjs",
    "    if (testCase.require_cursor_highlight && !cursorRows.every((row) => row.highlight === true)) {",
    smoothing_evidence + "    if (testCase.require_cursor_highlight && !cursorRows.every((row) => row.highlight === true)) {",
)

# Playback retained workflow moves to v6 and prebuilds the server before readiness polling.
replace_once(
    ".github/workflows/video-playback-canonical-parity.yml",
    "          (\n            cd backend\n            go run ./cmd/video-parity-fixture \\\n",
    "          mkdir -p output/video-playback-canonical/bin\n          (\n            cd backend\n            go run ./cmd/video-parity-fixture \\\n",
)
replace_once(
    ".github/workflows/video-playback-canonical-parity.yml",
    "            go run ./cmd/video-playback-parity-fixture \\\n              --output-dir ../output/video-playback-canonical/fixture\n          )",
    "            go run ./cmd/video-playback-parity-fixture \\\n              --output-dir ../output/video-playback-canonical/fixture\n            go build -o ../output/video-playback-canonical/bin/server ./cmd/server\n          )",
)
replace_once(
    ".github/workflows/video-playback-canonical-parity.yml",
    "          (\n            cd backend\n            go run ./cmd/server >\"$runtime/backend.log\" 2>\"$runtime/backend.err.log\"\n          ) &\n          backend_pid=$!",
    "          \"$GITHUB_WORKSPACE/output/video-playback-canonical/bin/server\" >\"$runtime/backend.log\" 2>\"$runtime/backend.err.log\" &\n          backend_pid=$!",
)
replace_once(
    ".github/workflows/video-playback-canonical-parity.yml",
    "              echo \"playback parity servers did not become ready\" >&2\n              exit 1",
    "              echo \"playback parity servers did not become ready\" >&2\n              cat \"$runtime/backend.err.log\" >&2 || true\n              cat \"$runtime/backend.log\" >&2 || true\n              exit 1",
)
replace_once(
    ".github/workflows/video-playback-canonical-parity.yml",
    "parity-playback-canonical-v5.json",
    "parity-playback-canonical-v6.json",
)

# Capability/strict-parity language reflects the bounded canonical path without claiming universal parity.
replace_once(
    "backend/internal/video/renderer_capabilities.go",
    "Static 2D media cursors up to the bounded fidelity segment limit use cursor-state-v1 output-frame sampling plus deterministic pointer, highlight, and click-ring rasters on the owner track. Smoothing, animated/3D/camera/effect/transition parents and longer clips retain the sampled compatibility fallback. Click audio is not synthesized.",
    "Static 2D media cursors up to the bounded fidelity segment limit use cursor-state-v1 linear or cursor-state-v2 deterministic smoothstep output-frame sampling plus pointer, highlight, and click-ring rasters on the owner track. Animated/3D/camera/effect/transition/fade parents and longer clips retain the compatibility fallback. Click audio is not synthesized.",
)
replace_once(
    "backend/internal/video/strict_parity.go",
    'Detail: "cursor paths and click rings use sampled export overlays"',
    'Detail: "cursor parity is bounded to the proven static-2D raster subset; complex parents and click audio remain partial"',
)

replace_once(
    "docs/VIDEO_RENDERING.md",
    "- Cursor paths, highlights, and click rings through sampled overlays. Click audio is not synthesized.",
    "- Static 2D media cursors within the bounded fidelity segment limit use canonical output-frame raster overlays: `cursor-state-v1` for linear motion and `cursor-state-v2` for deterministic smoothstep motion, with matching pointer/highlight/click-ring state. Complex animated/3D/camera/effect/transition/fade parents and longer clips stay on the compatibility path. Click audio is not synthesized.",
)
replace_once(
    "docs/VIDEO_TIMELINE_SCHEMA.md",
    "- `text`, `shape`, and `cursor`: styled overlay, annotation, and captured cursor data. Cursor paths/click rings export through sampled overlays; click audio is not synthesized.",
    "- `text`, `shape`, and `cursor`: styled overlay, annotation, and captured cursor data. The bounded static-2D cursor export subset uses `cursor-state-v1` linear or `cursor-state-v2` deterministic smoothstep output-frame rasters with click rings/highlights; complex parents and longer clips retain compatibility rendering. Click audio is not synthesized.",
)
replace_once(
    "docs/VIDEO_EDIT_STUDIO_MEDIA_AUDIO_CAPTIONS_FAQ.md",
    "Cursor metadata can store sampled positions, clicks, scale, highlight, and click-ring choices—useful for future screen-recording integrations. Preview shows an interpolated cursor; export renders sampled paths, highlights, and click rings. Click-audio synthesis is not available. Browser recording APIs do not supply cursor coordinates, so browser captures do not automatically create cursor metadata.",
    "Cursor metadata stores sampled positions, clicks, scale, highlight, click-ring choices, and optional smoothing for screen-recording workflows. For the bounded static-2D media subset, preview and export share exact frame-addressed cursor state: linear motion uses `cursor-state-v1`, while smoothing uses deterministic cubic smoothstep timing in `cursor-state-v2`; highlights and click rings use the same sampled state. Animated/3D/camera/effect/transition/fade parents and longer clips still use compatibility rendering, and click-audio synthesis is not available. Browser recording APIs do not supply cursor coordinates, so browser captures do not automatically create cursor metadata.",
)

# Durable tracker handoff and cursor subsections.
current_handoff = r'''Latest merged WYSIWYG program PR: **#309 — Canonical cursor normal-playback parity** — squash merge `505dda173065a52d218bdb05ead9ebe377699048`. Its exact final head passed the complete 16/16 triggered Quality/Security/browser/renderer/platform matrix before merge. `parity-cursor-v1` remains the retained linear-cursor Chromium↔Go/FFmpeg export control, and `parity-playback-canonical-v5` remains the merged normal-playback control from #309.

Current implementation slice: **canonical cursor smoothing parity** on branch `feat/video-wysiwyg-phase3-cursor-smoothing-parity`, created directly from #309's squash result. Smoothing is now an additive computed contract, `cursor-state-v2`, using deterministic cubic smoothstep timing (`3t²−2t³`) between the same authored event coordinates. Unsmoothed authoring remains byte-for-byte `cursor-state-v1`; exact rational-time sampling, endpoint hold, scale/highlight/click-ring state, and the strict `<300 ms` click window are unchanged.

The backend FrameState evaluator, FidelityRenderer raster path, frontend render contract, normal-playback admission, and canonical preview painter all consume v2. Static 2D media cursors remain the only newly admitted export/playback shape; animated/3D/camera/effect/transition/fade parents and clips beyond the bounded segment budget still fail closed to compatibility behavior. This slice also preserves explicit v1 `visible=false` as no-paint at the renderer adapter and requires authored smoothing to match the computed v1/v2 contract version before normal playback can claim authority.

`parity-cursor-smoothing-v2` is an independent 640×360/100 fps black-backdrop fixture with two authored endpoints and asymmetric frame samples at 25/50/75. The browser must land at `(210,125)`, `(320,180)`, and `(430,235)`; the quarter/three-quarter samples are deliberately distinct from linear `(240,140)` and `(400,220)`. Measurement run `33890125771` was executed twice against commit `3dc5cad80472e386e594d6d8908702ea01eda2b0`. Both attempts reproduced the frame metrics and changed bounds exactly: pixel pass `0.9911935763888889`, MAE `0.28171296296296294`, RMSE `3.6370401898511644`, maximum channel delta `178`, and SSIM `0.9867336424633555` or better. Bounds are `[139,54)-[282,197)`, `[249,109)-[392,252)`, and `[359,164)-[502,307)`. The immutable source/download PNG SHA-256 was `a05b1c08b14e2591ac125198907adc259f84cdbb1a641cf50c80dddd1bbf557e` in both attempts. Seeded timeline hashes differ because uploaded asset/timeline identifiers are regenerated per run; each attempt is individually bound to its saved immutable revision/hash instead of asserting a false cross-run SHA identity. Retained measurement artifacts are `9944183824` (`3c43e586af02aaa9df266ba4c108787ce57f76a2f1e1b2db897dc2fe3ec558dc`) and `9944281767` (`29733f07ddf681af8aa4bf5e052568bceb6dff231f302904c4064c18b65c9fe9`).

The permanent `Video Cursor Smoothing Parity Evidence` gate freezes the already-proven no-ring envelope without weakening repository-global visual thresholds: pixel pass ≥`0.990`, SSIM ≥`0.985`, MAE ≤`0.31`, RMSE ≤`3.80`, max channel delta ≤`180`, plus the exact per-frame changed bounds above and independent browser smoothstep checks. Normal-playback evidence advances to `parity-playback-canonical-v6` with a dedicated continuously advancing smoothing case whose independent capture helper computes smoothstep itself and requires an asymmetric sample away from the linear path.

**Phase 2 — Canonical contract is complete. Phase 3 — Shared preview composition is active. Phase 0 parity-evidence hardening continues in parallel.** Renderer-independent contracts own authored semantics. Browser/FFmpeg consumers may produce renderer-specific evidence, but they must not silently redefine canonical intent.
'''
replace_between(
    "docs/VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md",
    "## Current handoff\n\n",
    "\n### #306 merged immutable project-font selection",
    current_handoff,
)
replace_once(
    "docs/VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md",
    "The admitted subset calls the backend `cursor-state-v1` evaluator for each exact output frame",
    "The admitted subset calls the backend `cursor-state-v1` or `cursor-state-v2` evaluator for each exact output frame",
)
replace_once(
    "docs/VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md",
    "Static owner x/y, uniform scale, Z rotation, and opacity are inherited. Smoothing, visual keyframes/animation, 3D/perspective/anchor transforms, scene camera overlap, enabled effects/transitions/fades, ambiguous same-track overlap, non-media owners, and durations beyond the bounded segment budget retain the compatibility fallback.",
    "Static owner x/y, uniform scale, Z rotation, and opacity are inherited. Linear `cursor-state-v1` and deterministic smoothstep `cursor-state-v2` motion are canonical; visual keyframes/animation, 3D/perspective/anchor transforms, scene camera overlap, enabled effects/transitions/fades, ambiguous same-track overlap, non-media owners, and durations beyond the bounded segment budget retain the compatibility fallback.",
)
replace_once(
    "docs/VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md",
    "Focused backend tests cover exact ±300 ms click boundaries, owner-track preservation, parent affine transform/opacity, smoothing fallback, deterministic raster generation/cleanup, and byte-level pinned palette values.",
    "Focused backend tests cover exact ±300 ms click boundaries, owner-track preservation, parent affine transform/opacity, hidden-cursor no-paint, deterministic smoothstep v2 sampling, raster generation/cleanup, and byte-level pinned palette values.",
)
replace_once(
    "docs/VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md",
    "Renderer capability remains `Partial`: this slice deliberately does not synthesize click audio and does not admit smoothing/animated/3D/camera/effect/transition parents or unbounded cursor expansion.",
    "Renderer capability remains `Partial`: linear v1 and smoothstep v2 are proven only for the bounded static-2D media subset; this slice deliberately does not synthesize click audio and does not admit animated/3D/camera/effect/transition/fade parents or unbounded cursor expansion.",
)
replace_once(
    "docs/VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md",
    "- `parity-playback-canonical-v5` proves exact rational-time samples during continuously advancing playback, cursor motion, click-window state changes, resource-text + cursor atomic authority, weighted-transition + text + cursor composition, and explicit unsupported fade-parent fallback.",
    "- `parity-playback-canonical-v6` retains every v5 case and adds continuously advancing `cursor-state-v2` smoothing evidence with an independently computed smoothstep expectation and an asymmetric non-linear sample, while preserving explicit unsupported fade-parent fallback.",
)
replace_once(
    "docs/VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md",
    "- Deliberate remaining cursor debt is unchanged from export: click audio is not synthesized; smoothing and animated/3D/camera/effect/transition/fade parents remain compatibility-only. The next cursor-specific expansion should be justified by matching export support and retained browser/export evidence, not by preview-only admission.",
    "- Deliberate remaining cursor debt is now narrower: click audio is not synthesized; animated/3D/camera/effect/transition/fade parents and longer/unbounded expansion remain compatibility-only. The next cursor-specific expansion should be justified by matching export support and retained browser/export evidence, not by preview-only admission.",
)

smoothing_section = r'''
### Canonical cursor smoothing parity slice

- `cursor-state-v2` is additive and only emitted for authored `smoothing=true`; v1 linear timing and all shared cursor defaults/click semantics remain unchanged.
- The renderer and browser consume the same exact rational-time v2 state for the bounded static-2D media subset, while the dedicated evidence capture independently recomputes cubic smoothstep and rejects quarter/three-quarter samples that stay near linear motion.
- Two same-head hosted measurement attempts on run `33890125771` reproduced all three full-frame metrics and changed bounds exactly. The permanent gate therefore reuses the existing proven no-ring cursor envelope and locks the exact v2 bounds instead of weakening global parity thresholds.
- `parity-playback-canonical-v6` adds live smoothing evidence to continuously advancing playback without creating a second cursor runtime. Authored smoothing must match `cursor-state-v2` before canonical playback can claim authority.
- Explicit v1 hidden cursors remain no-paint at the renderer adapter; this prevents the v1 bool from being misread as omitted v2 visibility.
- Remaining cursor work after this slice: animated/static-parent interaction beyond the current static-2D boundary, 3D/camera/effect/transition/fade parents, longer/unbounded expansion, and click-audio synthesis.

'''
replace_once(
    "docs/VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md",
    "\n### #289 merged result\n",
    "\n" + smoothing_section + "### #289 merged result\n",
)

# Permanent v2 evidence gate. Global parity thresholds remain untouched.
Path(".github/workflows/video-cursor-smoothing-parity.yml").write_text(r'''name: Video Cursor Smoothing Parity Evidence

on:
  pull_request:
    paths:
      - '.github/workflows/video-cursor-smoothing-parity.yml'
      - 'backend/cmd/video-cursor-smoothing-parity-fixture/**'
      - 'backend/internal/video/parity_fixture_cursor_smoothing.go'
      - 'backend/internal/video/parity_fixture_cursor_smoothing_test.go'
      - 'backend/internal/video/rendercontract/cursor_state.go'
      - 'backend/internal/video/rendercontract/cursor_state_test.go'
      - 'backend/internal/video/rendercontract/frame_state_cursor_test.go'
      - 'backend/internal/video/renderer_cursor.go'
      - 'backend/internal/video/renderer_cursor_test.go'
      - 'frontend/src/video/renderContractCursor.ts'
      - 'frontend/test/renderContractCursor.test.ts'
      - 'frontend/test/renderContractFrameStateCursor.test.ts'
      - 'frontend/src/components/video/PreviewCanonicalPainters.tsx'
      - 'frontend/src/components/video/PreviewCanonicalPainters.test.ts'
      - 'frontend/src/components/video/previewCursorPlayback.ts'
      - 'frontend/src/components/video/previewCursorPlayback.test.ts'
      - 'scripts/video-cursor-smoothing-parity-capture.mjs'
      - 'docs/VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md'
  push:
    branches: [main]
    paths:
      - '.github/workflows/video-cursor-smoothing-parity.yml'
      - 'backend/cmd/video-cursor-smoothing-parity-fixture/**'
      - 'backend/internal/video/parity_fixture_cursor_smoothing.go'
      - 'backend/internal/video/parity_fixture_cursor_smoothing_test.go'
      - 'backend/internal/video/rendercontract/cursor_state.go'
      - 'backend/internal/video/rendercontract/cursor_state_test.go'
      - 'backend/internal/video/rendercontract/frame_state_cursor_test.go'
      - 'backend/internal/video/renderer_cursor.go'
      - 'backend/internal/video/renderer_cursor_test.go'
      - 'frontend/src/video/renderContractCursor.ts'
      - 'frontend/test/renderContractCursor.test.ts'
      - 'frontend/test/renderContractFrameStateCursor.test.ts'
      - 'frontend/src/components/video/PreviewCanonicalPainters.tsx'
      - 'frontend/src/components/video/PreviewCanonicalPainters.test.ts'
      - 'frontend/src/components/video/previewCursorPlayback.ts'
      - 'frontend/src/components/video/previewCursorPlayback.test.ts'
      - 'scripts/video-cursor-smoothing-parity-capture.mjs'
      - 'docs/VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md'
  workflow_dispatch:

permissions:
  contents: read

concurrency:
  group: video-cursor-smoothing-parity-${{ github.ref }}
  cancel-in-progress: true

env:
  GO_VERSION: '1.25.13'
  NODE_VERSION: '24'

jobs:
  smoothing-evidence:
    name: Cursor smoothstep browser export parity
    runs-on: ubuntu-24.04
    timeout-minutes: 30
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: backend/go.sum
      - uses: actions/setup-node@v7
        with:
          node-version: ${{ env.NODE_VERSION }}
          cache: npm
          cache-dependency-path: |
            package-lock.json
            frontend/package-lock.json
      - run: npm ci
      - working-directory: frontend
        run: npm ci
      - name: Install parity dependencies
        run: |
          set -euo pipefail
          bash scripts/ci-apt-install.sh ffmpeg
          bash scripts/ci-playwright-install.sh
          command -v ffmpeg
          command -v ffprobe
      - name: Validate cursor v2 contracts
        run: |
          set -euo pipefail
          (
            cd backend
            go test ./internal/video -run '^TestParityCursorSmoothingFixtureIsNarrowAndAsymmetric$|^TestCanonicalCursorRasterSupportsSmoothstepV2$|^TestCanonicalCursorRasterKeepsHiddenCursorNoPaint$'
            go test ./internal/video/rendercontract -run '^TestEvaluateCursorStateDefinesDeterministicSmoothstepV2$|^TestVisualFrameStateProjectsSmoothedCursorV2AtExactRationalTime$'
          )
          (
            cd frontend
            npx vitest run \
              test/renderContractCursor.test.ts \
              test/renderContractFrameStateCursor.test.ts \
              src/components/video/PreviewCanonicalPainters.test.ts \
              src/components/video/previewCursorPlayback.test.ts
          )
          node --check scripts/video-cursor-smoothing-parity-capture.mjs
      - name: Generate fixture, server, and deterministic backdrop
        run: |
          set -euo pipefail
          mkdir -p output/video-cursor-smoothing-parity/bin
          (
            cd backend
            go run ./cmd/video-cursor-smoothing-parity-fixture --output-dir ../output/video-cursor-smoothing-parity/fixture
            go build -o ../output/video-cursor-smoothing-parity/bin/server ./cmd/server
          )
          mkdir -p output/video-cursor-smoothing-parity/fixture/media
          ffmpeg -hide_banner -loglevel error -y \
            -f lavfi -i color=c=black:s=640x360:r=1 \
            -frames:v 1 -c:v png -threads 1 \
            output/video-cursor-smoothing-parity/fixture/media/cursor-smoothing-backdrop.png
          sha256sum output/video-cursor-smoothing-parity/fixture/media/cursor-smoothing-backdrop.png \
            > output/video-cursor-smoothing-parity/fixture/backdrop-source.sha256
      - name: Capture and gate cursor smoothing parity
        shell: bash
        run: |
          set -euo pipefail
          runtime="$GITHUB_WORKSPACE/output/video-cursor-smoothing-parity/runtime"
          mkdir -p "$runtime/attachments"
          export OMNILLM_PORT=18108
          export OMNILLM_DB_PATH="$runtime/parity.db"
          export OMNILLM_ATTACHMENTS_DIR="$runtime/attachments"
          "$GITHUB_WORKSPACE/output/video-cursor-smoothing-parity/bin/server" >"$runtime/backend.log" 2>"$runtime/backend.err.log" &
          backend_pid=$!
          (
            cd frontend
            OMNILLM_API_PROXY_TARGET=http://127.0.0.1:18108 npm run dev -- --host 127.0.0.1 --port 14191 >"$runtime/frontend.log" 2>"$runtime/frontend.err.log"
          ) &
          frontend_pid=$!
          cleanup() {
            kill "$frontend_pid" "$backend_pid" 2>/dev/null || true
            wait "$frontend_pid" "$backend_pid" 2>/dev/null || true
          }
          trap cleanup EXIT
          for attempt in {1..60}; do
            if curl --fail --silent http://127.0.0.1:18108/v1/health >/dev/null && curl --fail --silent http://127.0.0.1:14191/ >/dev/null; then
              break
            fi
            if [ "$attempt" -eq 60 ]; then
              echo "cursor smoothing parity servers did not become ready" >&2
              cat "$runtime/backend.err.log" >&2 || true
              cat "$runtime/backend.log" >&2 || true
              exit 1
            fi
            sleep 1
          done

          fixture=output/video-cursor-smoothing-parity/fixture/parity-cursor-smoothing-v2.json
          capture=output/video-cursor-smoothing-parity/capture
          node scripts/video-cursor-smoothing-parity-capture.mjs \
            --url http://127.0.0.1:14191 \
            --fixture "$fixture" \
            --image-file output/video-cursor-smoothing-parity/fixture/media/cursor-smoothing-backdrop.png \
            --output "$capture"

          timeline_hash="$(node -p "require('./$capture/seed-result.json').timeline_sha256")"
          (
            cd backend
            go run ./cmd/video-parity-report \
              --preview-dir ../$capture/preview \
              --rendered-dir ../$capture/rendered \
              --output-dir ../$capture/smoothing-report \
              --fps 100 \
              --fixture parity-cursor-smoothing-v2 \
              --timeline-sha256 "$timeline_hash" \
              --allow-fail
          )
          node - <<'NODE'
          const fs = require('fs');
          const evidence = JSON.parse(fs.readFileSync('output/video-cursor-smoothing-parity/capture/cursor-smoothing-frame-evidence.json', 'utf8'));
          const report = JSON.parse(fs.readFileSync('output/video-cursor-smoothing-parity/capture/smoothing-report/parity-report.json', 'utf8'));
          const expectedBounds = new Map([
            [25, { min_x: 139, min_y: 54, max_x: 281, max_y: 196 }],
            [50, { min_x: 249, min_y: 109, max_x: 391, max_y: 251 }],
            [75, { min_x: 359, min_y: 164, max_x: 501, max_y: 306 }],
          ]);
          const expectedPositions = new Map([[25, [210, 125]], [50, [320, 180]], [75, [430, 235]]]);
          const errors = [];
          if (evidence.fixture !== 'parity-cursor-smoothing-v2') errors.push(`fixture=${evidence.fixture}`);
          if (evidence.frames.length !== 3) errors.push(`browser frame count=${evidence.frames.length}`);
          for (const frame of evidence.frames) {
            const expected = expectedPositions.get(frame.frame_index);
            if (!expected) { errors.push(`unexpected browser frame ${frame.frame_index}`); continue; }
            if (frame.state_mode !== 'canonical-frame') errors.push(`frame ${frame.frame_index} state=${frame.state_mode}`);
            if (Math.abs(frame.left - expected[0]) > 0.02 || Math.abs(frame.top - expected[1]) > 0.02) {
              errors.push(`frame ${frame.frame_index} position=${frame.left},${frame.top}`);
            }
            if (frame.svg_width !== 64 || frame.svg_height !== 64 || !frame.highlight_present || frame.ring_present) {
              errors.push(`frame ${frame.frame_index} cursor geometry/style drift`);
            }
            if ((frame.frame_index === 25 || frame.frame_index === 75)
              && Math.max(Math.abs(frame.left - frame.linear_position.x), Math.abs(frame.top - frame.linear_position.y)) < 20) {
              errors.push(`frame ${frame.frame_index} did not prove asymmetric smoothing`);
            }
          }
          if (report.frames.length !== 3) errors.push(`report frame count=${report.frames.length}`);
          for (const frame of report.frames) {
            const bounds = expectedBounds.get(frame.frame_index);
            if (!bounds) { errors.push(`unexpected report frame ${frame.frame_index}`); continue; }
            if (frame.pixel_pass_rate < 0.990) errors.push(`frame ${frame.frame_index} pixel pass ${frame.pixel_pass_rate}`);
            if (frame.ssim < 0.985) errors.push(`frame ${frame.frame_index} ssim ${frame.ssim}`);
            if (frame.mean_absolute_error > 0.31) errors.push(`frame ${frame.frame_index} mae ${frame.mean_absolute_error}`);
            if (frame.root_mean_square_error > 3.80) errors.push(`frame ${frame.frame_index} rmse ${frame.root_mean_square_error}`);
            if (frame.max_channel_delta > 180) errors.push(`frame ${frame.frame_index} max delta ${frame.max_channel_delta}`);
            if (JSON.stringify(frame.changed_bounds) !== JSON.stringify(bounds)) {
              errors.push(`frame ${frame.frame_index} bounds ${JSON.stringify(frame.changed_bounds)} want ${JSON.stringify(bounds)}`);
            }
          }
          const focused = { fixture: evidence.fixture, timeline_sha256: report.timeline_sha256, pass: errors.length === 0, errors, frames: report.frames };
          fs.writeFileSync('output/video-cursor-smoothing-parity/capture/smoothing-focused-gate.json', `${JSON.stringify(focused, null, 2)}\n`);
          console.log(`CURSOR_SMOOTHING_FOCUSED_GATE=${JSON.stringify(focused)}`);
          if (errors.length) process.exit(1);
          NODE
      - name: Record toolchain
        if: always()
        run: |
          mkdir -p output/video-cursor-smoothing-parity
          {
            go version
            node --version
            npm --version
            ffmpeg -version | head -n 1
            ffprobe -version | head -n 1
            node -e "console.log('playwright ' + require('playwright/package.json').version)"
          } > output/video-cursor-smoothing-parity/toolchain.txt
      - name: Upload cursor smoothing parity evidence
        if: always()
        uses: actions/upload-artifact@v7
        with:
          name: video-cursor-smoothing-parity-evidence
          path: output/video-cursor-smoothing-parity
          if-no-files-found: error
          retention-days: 14
''')

# Ensure generated and edited files end in exactly one newline where practical.
for path in [
    "backend/internal/video/renderer_cursor.go",
    "backend/internal/video/renderer_cursor_test.go",
    "frontend/src/components/video/previewCursorPlayback.ts",
    "frontend/src/components/video/previewCursorPlayback.test.ts",
    "backend/internal/video/parity_fixture_playback.go",
    "backend/internal/video/parity_fixture_playback_test.go",
    "scripts/video-playback-parity-capture.mjs",
    ".github/workflows/video-playback-canonical-parity.yml",
    "backend/internal/video/renderer_capabilities.go",
    "backend/internal/video/strict_parity.go",
    "docs/VIDEO_RENDERING.md",
    "docs/VIDEO_TIMELINE_SCHEMA.md",
    "docs/VIDEO_EDIT_STUDIO_MEDIA_AUDIO_CAPTIONS_FAQ.md",
    "docs/VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md",
    ".github/workflows/video-cursor-smoothing-parity.yml",
]:
    p = Path(path)
    p.write_text(p.read_text().rstrip() + "\n")

print("cursor smoothing finalization patch applied")
