from pathlib import Path

path = Path("docs/VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md")
text = path.read_text()

handoff_start = text.index("Latest merged WYSIWYG program PR:")
handoff_end = text.index("\n\n**Phase 2 — Canonical contract is complete.", handoff_start)
handoff = """Latest merged WYSIWYG program PR: **#306 — Bind FFmpeg text to immutable project font resources** — squash merge `1edbd24ebfe66f990373526276228d0cc5ba5a29`. The renderer now consumes the exact staged Render Manifest face for every text clip that declares `font_resource_id`; unresolved resource-backed text fails closed instead of falling back to a host family name. #306's exact final head `89ab620dddc3a2db1d0b78f90e3f1d7a1a548152` passed the complete Quality/Security/playback/pixelate/platform matrix before merge.

Current implementation PR: **#307 — Gate immutable resource text glyph parity** on branch `feat/video-wysiwyg-phase3-font-glyph-parity`, normalized directly onto #306's actual squash result. `parity-resource-text-v1` isolates one 640×360/30 fps black canvas, one static `WYSIWYG 42` text clip at 48 px/400/white, and one immutable DejaVu Sans project font resource. Two independent hosted measurements produced identical frame-15 metrics before the envelope was frozen. Hard-gate run `33792035551` passed on head `dda1873e9f5e167c855b187056c1863dda403d1e`; retained artifact `9907747511` is SHA-256 `3afe45506bba1fc56a5cb8ac6f754623d6f930390607aedd5847c239ca754304`. The exact face SHA-256 is `ae7b7855e115a5966d8b1b3f80f254ccc117ec86f9965e202ee2940453837280` (759,720 bytes). The focused gate preserves the repository-global ±2 diagnostics but accepts this known Chromium/FFmpeg rasterizer envelope only when pixel pass is ≥0.988, SSIM ≥0.900, MAE ≤1.20, RMSE ≤13.20, and changed bounds are exactly `[165,161)-[471,199)` after exact font-resource, runtime-alias, and canonical-frame identity checks."""
text = text[:handoff_start] + handoff + text[handoff_end:]

marker = "### #289 merged result\n"
if "### #306 merged immutable project-font selection" not in text:
    sections = """### #306 merged immutable project-font selection

#306 closed the prerequisite renderer bug exposed by the glyph-parity program:

- Render snapshots already staged, hashed, and verified immutable project-font bytes, but the FFmpeg compositor did not select those staged bytes.
- `RenderRequest` now carries the staged font-resource map into both diagnostic and delivery renders.
- Resource-backed `drawtext` uses the exact staged `fontfile=`. A missing declared `font_resource_id` fails closed; only text without a resource binding retains family-name compatibility behavior.
- Focused renderer and snapshot-input tests prove the staged face is threaded into the renderer and selected by resource ID.
- Exact final head `89ab620dddc3a2db1d0b78f90e3f1d7a1a548152` passed Quality Gate #1866 (including backend race tests, Playwright smoke, and the full renderer baseline), Security #1872, playback #90, every pixelate evidence workflow, and all applicable platform/sandbox assurance before squash merge `1edbd24ebfe66f990373526276228d0cc5ba5a29`.

### #307 resource-backed Chromium↔FFmpeg glyph evidence

#307 turns the first immutable project-font browser/export comparison into a retained non-regression gate without redefining authored text semantics:

- `parity-resource-text-v1` contains one static text clip, one immutable regular DejaVu Sans resource, black `#000000` canvas, white `#ffffff` glyphs, and exactly one interior sample at frame `15` / `500 ms`. There is no media, audio, animation, shadow, stroke, background, explicit line-height, letter spacing, or family fallback.
- The dedicated capture path uploads the font with `font_resource_id=parity-text-face-v1`, downloads it again, and requires source/download SHA-256 identity before timeline submission. Chromium must report canonical frame 15, `editor-resource-loaded`, and the isolated `OmniLLMPreview_<resource>_<asset>_400` runtime alias.
- The editor FrameState intentionally continues to report `font_face_source=family-name-only`; #307 does not mutate `text-state-v1` provenance. Exact face identity is consumer/runtime evidence plus the immutable byte hash.
- Measurement run `33791589303` was executed twice. Both attempts produced identical timeline SHA-256 `add27dde832b827c470d7e9b9b3cec432bb29456d5f50120161c4857d21c6754`, pixel pass `0.9889149305555556`, SSIM `0.9036478181900497`, MAE `1.1240668402777778`, RMSE `12.992389432268501`, maximum channel delta `255`, and changed bounds `[165,161)-[471,199)`.
- The retained Chromium visible glyph extent is approximately 304×37 while FFmpeg is 306×38, demonstrating a bounded rasterizer/shaping difference rather than byte-identical glyph output. Repository-global visual thresholds remain unchanged and the global report remains intentionally diagnostic-failing for this fixture.
- Hard-gate run `33792035551` passed with a fixture-specific envelope derived from those repeated measurements: ±2 pass rate ≥0.988, SSIM ≥0.900, MAE ≤1.20, RMSE ≤13.20, and exact changed bounds `[165,161)-[471,199)` after font/frame/runtime identity checks. Artifact `9907747511` is SHA-256 `3afe45506bba1fc56a5cb8ac6f754623d6f930390607aedd5847c239ca754304`.

"""
    text = text.replace(marker, sections + marker, 1)

lines = text.splitlines()
phase0 = "| Phase 0 — Reproducible parity baseline | **In progress** | Deterministic 103-frame visual/audio/delivery evidence exists. Focused retained controls now include opaque PNG exactness, H.264 decoded-frame/±3 color, partial-alpha PNG ±1, transparent VP9 presentation/±4, project-background ±1, normal-playback media/weighted/text ownership, #306 immutable FFmpeg project-font selection, and #307 resource-backed Chromium↔FFmpeg glyph acceptance. The 103-frame torture fixture still lacks a project-font resource and second-platform retained font/glyph evidence. |"
phase3 = "| Phase 3 — Shared preview composition | **In progress** | Canonical deterministic media, transforms/view/perspective, geometry, effects, transitions, resource text, pixelate raster classes, free-running media/weighted/text playback, and supported mixed composition are retained. #306 binds export text to immutable project-font bytes and #307 freezes the first focused browser↔export glyph envelope. Shape/cursor normal-playback authority, further independently evidenced raster classes, and AudioGraph consumption remain before Phase 3 can close. |"
for i, line in enumerate(lines):
    if line.startswith("| Phase 0 — Reproducible parity baseline |"):
        lines[i] = phase0
    elif line.startswith("| Phase 3 — Shared preview composition |"):
        lines[i] = phase3
text = "\n".join(lines) + "\n"

font_marker = "- Immutable static-font identity remains Render Manifest-backed by `font-resource-provenance-v1`.\n"
font_add = "- #306 makes FFmpeg resource-backed text select the exact staged Render Manifest face and fail closed if that face is unavailable.\n- #307 retains the first exact-resource Chromium↔FFmpeg glyph-pixel envelope while keeping Chromium layout as consumer evidence rather than authored semantics.\n"
if font_add not in text:
    text = text.replace(font_marker, font_add + font_marker, 1)

row304 = "| #304 | Ready resource-backed text DOM authority during normal playback | `2633b51c94d0077902b0cf2a11e925277ec78582` |\n"
if "| #305 |" not in text:
    text = text.replace(row304, row304 + "| #305 | Supported media+resource-text and weighted-pair+resource-text atomic playback composition | `9ed56ddaa3a142b2a3747bf850233597a2e43f18` |\n| #306 | FFmpeg immutable project-font selection for resource-backed text | `1edbd24ebfe66f990373526276228d0cc5ba5a29` |\n", 1)

old_lineage = "**#305 is directly from #304 squash `2633b51c94d0077902b0cf2a11e925277ec78582`**."
new_lineage = "#305 was directly from #304 and squash-merged as `9ed56ddaa3a142b2a3747bf850233597a2e43f18`; #306 was created directly from that result and squash-merged as `1edbd24ebfe66f990373526276228d0cc5ba5a29`; **current #307 is normalized directly onto #306 squash `1edbd24ebfe66f990373526276228d0cc5ba5a29` and is behind `main` by zero**."
if old_lineage in text:
    text = text.replace(old_lineage, new_lineage, 1)

old_baseline = "Normal-playback v4 now includes deterministic project-font bytes and proves exact Chromium `FontFace` readiness, stabilized layout evidence, mixed consumer ownership, and atomic revocation for the supported fixture. The 103-sample torture baseline still does not include a project font resource, and no browser↔export glyph-pixel acceptance gate or second supported OS/FFmpeg retained run exists yet. Do not treat cross-machine glyph identity as closed. Retained visual evidence remains diagnostic until structural zero-tolerance policy and codec-aware decoded thresholds are frozen and retained on a second supported environment."
new_baseline = "Normal-playback v4 includes deterministic project-font bytes and proves exact Chromium `FontFace` readiness, stabilized layout evidence, mixed consumer ownership, and atomic revocation for the supported fixture. #306 now binds FFmpeg resource-backed text to the immutable staged face. #307 adds `parity-resource-text-v1`, a focused browser↔export glyph acceptance gate on Ubuntu 24.04 after two identical measurement runs. The 103-sample torture baseline still does not include a project font resource, and no second supported OS/FFmpeg retained font run exists yet. Do not treat cross-machine glyph identity as closed."
if old_baseline in text:
    text = text.replace(old_baseline, new_baseline, 1)

risk_marker = "| Family-name-only text is mistaken for deterministic face identity | Snapshot records provenance/runtime; #304 explicitly defers normal-playback family-only text with `resource-font-required`. Exact cross-machine identity still requires a resource-backed face. |\n"
risk_add = "| FFmpeg and Chromium use the same font bytes but different glyph rasterizers/shapers | #307 requires exact immutable font/runtime/frame identity first, then gates the twice-measured fixture-specific bounds and pixel envelope without weakening repository-global thresholds. |\n"
if risk_add not in text:
    text = text.replace(risk_marker, risk_marker + risk_add, 1)

next_marker = "## Next recommended slice\n"
next_start = text.index(next_marker)
next_text = """## Next recommended slice

1. **Merge #307 only after the final tracker-bearing exact head passes Video Resource Text Parity Evidence plus every Quality/Security/browser/renderer/platform workflow triggered by the PR diff.** The pre-tracker hard-gate run proves the envelope but does not exempt the documentation head from exact validation.
2. **Start the next implementation slice directly from #307's actual squash result and replace the current renderer cursor approximation with `cursor-state-v1`-aligned export semantics before admitting cursor during normal playback.** The current FidelityRenderer samples at 12–30 Hz, paints a Unicode `➤`, uses a renderer-specific text face/stroke/shadow, and approximates click rings with a rectangle; those pixels are not canonical cursor evidence.
3. Build the cursor slice around exact output-frame sampling, the existing strict `<300 ms` click window, canonical scale/highlight/click-ring state, and a focused retained browser↔export fixture. Do not promote cursor playback until export evidence passes and the complete-frame fallback control remains green.
4. Keep general shape playback deferred until the current FFmpeg shape approximations are replaced or independently proven. In parallel, add a second supported OS/toolchain run for the resource-font fixture so Phase 0 can retire the remaining cross-machine font-evidence debt without delaying cursor implementation.
"""
text = text[:next_start] + next_text

path.write_text(text)
