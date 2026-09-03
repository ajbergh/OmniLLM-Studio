from pathlib import Path
import re

path = Path("docs/VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md")
text = path.read_text()

handoff = """## Current handoff

Latest merged WYSIWYG program PR: **#307 — Gate immutable resource text glyph parity** — squash merge `0718f1069c0e4531a5a0ebd9334f74c9edb2ae68`. Its exact final PR head `8ea1c90ee41761706c797b646c37546881c84dd4` passed the complete exact-head Quality/Security/browser/renderer/platform matrix before merge. `parity-resource-text-v1` remains the retained immutable project-font Chromium↔FFmpeg acceptance gate; hard-gate artifact `9907747511` is SHA-256 `3afe45506bba1fc56a5cb8ac6f754623d6f930390607aedd5847c239ca754304`.

Current implementation slice: **canonical cursor export parity** on branch `feat/video-wysiwyg-phase3-cursor-export-parity`, created directly from #307's actual squash result. The supported static-2D media subset now evaluates `cursor-state-v1` at exact output-frame rational times, emits deterministic pointer/highlight/click-ring rasters on the owner track, preserves the strict `<300 ms` click-ring window, and inherits static x/y, uniform scale, Z rotation, and opacity. Smoothing, animated/3D/camera/effect/transition parents, ambiguous same-track overlaps, and clips beyond the bounded fidelity expansion remain on the compatibility path; click audio is still not synthesized, so renderer capability remains deliberately `Partial`.

The cursor slice pins visual palette semantics instead of inheriting mutable theme tokens: highlight `rgba(255, 223, 32, 0.3)` and ring `rgba(0, 188, 255, 0.8)` are explicit in canonical and compatibility preview painters and in the Go rasterizer. Hosted validation run `33800486954` passed `go test ./internal/video/...` plus the full frontend `tsc -b && vite build` before committing the palette correction as `9fa841a8e610e0021a3d194e079a17f20b2e54cd`.

`parity-cursor-v1` isolates one lossless 640×360 black PNG, 100 fps, one static media clip, and exact frames `20/21/50/79/80` around a click at 500 ms. Frames 20 and 80 are exactly ±300 ms and must omit the ring; 21/50/79 must include it. Two independent hosted attempts on run `33800619647` reproduced every numeric metric and changed bound exactly. Artifacts `9910959242` / `9911073577` are SHA-256 `8ea87cd181e399736580ded5c785f1d169ac416a23b2d7e37281ee7c82c9183e` / `49250eafc469cbe956bb83c08d6fb3146b458f29e486de8a25d4a26699155ca0`. No-ring frames measured pixel pass `0.9911935764`, SSIM `0.9867336425`, MAE `0.2817129630`, RMSE `3.6370401899`; ring frames measured `0.9851866319`, `0.9837122720`, `0.4196397569`, `4.3971392544`; maximum channel delta is `178`.

Hard-gate run `33801254401` passed on head `ed58353d555db10092421c494c02ebe6c65173ce`; retained artifact `9911228698` is SHA-256 `0c41d4996d66b44582d2627036fbc0aec1ddd065d6617a14063dc26b9327c9e1`. The gate requires no-ring ≥0.990 pixel pass / ≥0.985 SSIM / ≤0.31 MAE / ≤3.80 RMSE, ring ≥0.984 / ≥0.982 / ≤0.45 / ≤4.50, max channel delta ≤180, exact per-frame changed bounds, exact pinned palette, immutable backdrop identity, canonical frame identity, and strict ring-boundary structure.

"""
pattern = r"## Current handoff\n\n.*?(?=\*\*Phase 2)"
text, count = re.subn(pattern, handoff, text, count=1, flags=re.S)
if count != 1:
    raise SystemExit("current handoff block not found")

text = text.replace("### #307 resource-backed Chromium↔FFmpeg glyph evidence",
                    "### #307 merged resource-backed Chromium↔FFmpeg glyph evidence", 1)

if "### Canonical cursor export parity slice" not in text:
    cursor_section = """### Canonical cursor export parity slice

- `FidelityRenderer` no longer turns the supported cursor path into a sampled Unicode pointer plus rectangle click approximation. The admitted subset calls the backend `cursor-state-v1` evaluator for each exact output frame and emits deterministic render-only raster assets.
- Cursor pixels remain on the owning media track instead of a synthetic global topmost track, preserving track visibility/order and avoiding a second stacking model.
- Static owner x/y, uniform scale, Z rotation, and opacity are inherited. Smoothing, visual keyframes/animation, 3D/perspective/anchor transforms, scene camera overlap, enabled effects/transitions/fades, ambiguous same-track overlap, non-media owners, and durations beyond the bounded segment budget retain the compatibility fallback.
- Generated cursor PNGs are materialized after diagnostic-frame filtering, registered only for the render request, and removed after the delegate render.
- Pointer geometry remains 64 px at scale 1; highlight is 2.2× pointer size and click ring is 2.6× with an inward 2 px border. Preview and export pin highlight `#ffdf20` at 30% and ring `#00bcff` at 80%, preventing Tailwind-version palette drift.
- Focused backend tests cover exact ±300 ms click boundaries, owner-track preservation, parent affine transform/opacity, smoothing fallback, deterministic raster generation/cleanup, and byte-level pinned palette values. Hosted run `33800486954` passed focused Go video tests and the full frontend production build.
- `parity-cursor-v1` uses a source/download SHA-verified black PNG and samples frames 20/21/50/79/80 at 100 fps. Browser evidence requires canonical frame mode, exact interpolated position, 64×64 pointer geometry, pinned palette, highlight presence, and ring presence only at 21/50/79. FFmpeg evidence compares the full 640×360 diagnostic frame.
- Two independent measurement attempts on run `33800619647` reproduced all metrics and changed bounds exactly. Retained artifacts are `9910959242` (`8ea87cd181e399736580ded5c785f1d169ac416a23b2d7e37281ee7c82c9183e`) and `9911073577` (`49250eafc469cbe956bb83c08d6fb3146b458f29e486de8a25d4a26699155ca0`).
- Hard-gate run `33801254401` passed with `focused_pass=true` on all five samples. Artifact `9911228698` is SHA-256 `0c41d4996d66b44582d2627036fbc0aec1ddd065d6617a14063dc26b9327c9e1`. Repository-global ±2 diagnostics remain independent; the cursor gate records the stable Chromium-vs-Go antialiasing envelope rather than weakening global thresholds.
- Renderer capability remains `Partial`: this slice deliberately does not synthesize click audio and does not admit smoothing/animated/3D/camera/effect/transition parents or unbounded cursor expansion.

"""
    anchor = "### #289 merged result"
    if anchor not in text:
        raise SystemExit("cursor section insertion anchor not found")
    text = text.replace(anchor, cursor_section + anchor, 1)

row306 = "| #306 | FFmpeg immutable project-font selection for resource-backed text | `1edbd24ebfe66f990373526276228d0cc5ba5a29` |"
row307 = "| #307 | Immutable project-font Chromium↔FFmpeg glyph acceptance gate | `0718f1069c0e4531a5a0ebd9334f74c9edb2ae68` |"
if row307 not in text:
    if row306 not in text:
        raise SystemExit("phase 3 #306 row not found")
    text = text.replace(row306, row306 + "\n" + row307, 1)

lineage_re = r"#306 was created directly from that result and squash-merged as `1edbd24ebfe66f990373526276228d0cc5ba5a29`; \*\*current #307 is normalized directly onto #306 squash `1edbd24ebfe66f990373526276228d0cc5ba5a29` and is behind `main` by zero\*\*\."
lineage_new = "#306 was created directly from that result and squash-merged as `1edbd24ebfe66f990373526276228d0cc5ba5a29`; #307 was normalized directly onto #306, validated at exact head `8ea1c90ee41761706c797b646c37546881c84dd4`, and squash-merged as `0718f1069c0e4531a5a0ebd9334f74c9edb2ae68`; **the current cursor-export branch was created directly from that actual #307 squash result and was behind `main` by zero when its hard gate was frozen**."
text, count = re.subn(lineage_re, lineage_new, text, count=1)
if count != 1 and lineage_new not in text:
    raise SystemExit("lineage anchor not found")

baseline_sentence = "Normal-playback v4 includes deterministic project-font bytes and proves exact Chromium `FontFace` readiness, stabilized layout evidence, mixed consumer ownership, and atomic revocation for the supported fixture. #306 now binds FFmpeg resource-backed text to the immutable staged face. #307 adds `parity-resource-text-v1`, a focused browser↔export glyph acceptance gate on Ubuntu 24.04 after two identical measurement runs. The 103-sample torture baseline still does not include a project font resource, and no second supported OS/FFmpeg retained font run exists yet. Do not treat cross-machine glyph identity as closed."
cursor_baseline = "\n\n`parity-cursor-v1` is the focused static-2D cursor browser↔export acceptance gate. It independently proves exact `cursor-state-v1` output-frame sampling, owner-track export, pinned cursor palette, immutable backdrop identity, and strict `<300 ms` click-ring boundaries. It does not admit cursor during normal playback and does not cover smoothing, animated/3D/camera/effect/transition parents, click audio, or unbounded cursor expansion."
if cursor_baseline.strip() not in text:
    if baseline_sentence not in text:
        raise SystemExit("phase 0 baseline anchor not found")
    text = text.replace(baseline_sentence, baseline_sentence + cursor_baseline, 1)

risk_anchor = "| A renderer-specific painter approximation is mistaken for an exact raster source | #299 explicitly rejects broad shape admission after inspecting current FFmpeg shape approximations. New painter/source classes require independent ordering, readiness, geometry, and retained pixel evidence before classifier admission. |"
risk_rows = """| Cursor theme tokens drift independently from export raster colors | The supported cursor visual contract pins explicit sRGB highlight/ring values in canonical preview, compatibility preview, and Go export. `parity-cursor-v1` hard-gates those exact palette values plus the stable Chromium-vs-Go edge envelope. |
| Partial cursor support is mistaken for universal cursor parity | Capability remains `Partial`; smoothing, animated/3D/camera/effect/transition parents, ambiguous overlap, long unbounded expansion, and click audio are not claimed by the canonical export path. |"""
if "Cursor theme tokens drift independently from export raster colors" not in text:
    if risk_anchor not in text:
        raise SystemExit("risk anchor not found")
    text = text.replace(risk_anchor, risk_anchor + "\n" + risk_rows, 1)

next_heading = "## Next recommended slice"
idx = text.find(next_heading)
if idx < 0:
    raise SystemExit("next recommended slice heading not found")
next_section = """## Next recommended slice

1. **Open and merge the cursor-export parity PR only after the final tracker-bearing exact head passes Video Cursor Parity Evidence plus every Quality/Security/browser/renderer/platform workflow triggered by the PR diff.** Pre-PR hard-gate run `33801254401` proves the frozen envelope but does not exempt the final PR head from exact validation.
2. **Start the next slice directly from the cursor PR's actual squash result and admit only the proven static-2D cursor subset during normal playback.** Extend complete-frame atomic composition so a ready canonical cursor can coexist with supported media/resource-text/weighted consumers, while any unsupported cursor case still revokes the entire visual frame to legacy-time fallback.
3. Add retained normal-playback cursor authority/readiness evidence before broadening the playback classifier. Preserve exact output-frame identity, owner-layer ordering, pinned palette, strict `<300 ms` ring state, and whole-frame revocation controls; do not treat the export-only hard gate as playback proof.
4. Keep general shape playback deferred until FFmpeg shape approximations are replaced or independently proven. Cursor smoothing/animated-parent semantics and click-audio synthesis remain separate follow-on renderer slices. In parallel, add a second supported OS/toolchain run for the resource-font fixture to reduce the remaining cross-machine font-evidence debt.
"""
text = text[:idx] + next_section
path.write_text(text)
