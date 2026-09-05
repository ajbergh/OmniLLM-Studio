from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text()
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: anchor count={count}, want 1 for {old[:140]!r}")
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
    p.write_text(text[:start_index] + body + text[end_index:])


replace_once(
    "backend/internal/video/strict_parity.go",
    '\t\t\tif clip.Shape != nil {\n\t\t\t\tissues = append(issues, StrictParityIssue{Path: path + ".shape", Feature: RendererFeatureAnnotations, Detail: "annotation geometry is partially normalized during export"})\n\t\t\t}\n',
    '\t\t\tif clip.Shape != nil {\n\t\t\t\tif _, exact := canonicalRoundedRectangleRasterClip(clip, doc.Canvas, doc.Scenes); !exact {\n\t\t\t\t\tissues = append(issues, StrictParityIssue{Path: path + ".shape", Feature: RendererFeatureAnnotations, Detail: "annotation geometry is outside the proven exact rounded-rectangle raster subset and may be normalized during export"})\n\t\t\t\t}\n\t\t\t}\n',
)

strict_test = r'''
func TestStrictParityAllowsProvenRoundedRectangleRasterSubset(t *testing.T) {
	doc := NewEmptyTimeline(640, 360, 30)
	doc.DurationMS = 1000
	doc.Tracks[0].Clips = []TimelineClip{roundedRectangleTestClip()}
	if issues := StrictParityIssues(doc); len(issues) != 0 {
		t.Fatalf("proven rounded rectangle issues = %+v", issues)
	}

	unsupported := roundedRectangleTestClip()
	unsupported.FadeInMS = 100
	doc.Tracks[0].Clips = []TimelineClip{unsupported}
	issues := StrictParityIssues(doc)
	foundShape := false
	for _, issue := range issues {
		if issue.Path == "tracks[0].clips[0].shape" && issue.Feature == RendererFeatureAnnotations {
			foundShape = true
			break
		}
	}
	if !foundShape {
		t.Fatalf("unsupported rounded rectangle did not retain strict-parity shape issue: %+v", issues)
	}
}

'''
replace_once(
    "backend/internal/video/strict_parity_test.go",
    "func TestStrictParityAllowsTimelineWithoutKnownLegacyMismatches(t *testing.T) {",
    strict_test + "func TestStrictParityAllowsTimelineWithoutKnownLegacyMismatches(t *testing.T) {",
)

replace_once(
    "backend/internal/video/renderer_capabilities.go",
    '{Feature: RendererFeatureAnnotations, Label: "Annotations", Supported: true, Partial: true, Notes: "Every annotation produces deterministic export output, but ellipse, arrow, speech-bubble, and other complex geometry currently normalize to simpler primitives."},',
    '{Feature: RendererFeatureAnnotations, Label: "Annotations", Supported: true, Partial: true, Notes: "Static 2D rounded rectangles in the proven shape-state-v1 raster subset preserve radius, fill, stroke, transform, opacity, timing, and owner-track ordering. Ellipse, arrow, speech-bubble, label, other complex geometry, and unsupported rounded-rectangle parents still normalize to simpler compatibility primitives."},',
)

replace_once(
    "backend/internal/video/renderer.go",
    '\tcase ShapeKindRectangle, ShapeKindRoundedRectangle:\n\t\t// Rounded rectangles export with square corners — drawbox cannot\n\t\t// round; the capability matrix reports this as partial.\n',
    '\tcase ShapeKindRectangle, ShapeKindRoundedRectangle:\n\t\t// Compatibility fallback. FidelityRenderer converts the proven static-2D\n\t\t// rounded_rectangle subset to an exact raster asset before this point;\n\t\t// unsupported rounded rectangles still flatten through drawbox.\n',
)

replace_once(
    "backend/internal/video/timeline.go",
    '\t// CornerRadius applies to rounded rectangles, speech bubbles, and labels\n\t// (clamped 0–200). Preview-only; exports draw square corners.\n',
    '\t// CornerRadius applies to rounded rectangles, speech bubbles, and labels\n\t// (clamped 0–200). The proven static-2D rounded_rectangle export subset\n\t// preserves it; speech bubbles, labels, and unsupported parents remain partial.\n',
)

replace_once(
    "frontend/src/types/video.ts",
    "  // Annotation kinds. blur/pixelate/rectangle/highlight/rounded_rectangle/label\n  // export (rounded corners flatten to square); the rest are preview-only.\n",
    "  // Annotation kinds. blur/pixelate/rectangle/highlight export directly. The\n  // proven static-2D rounded_rectangle subset preserves canonical rounded geometry;\n  // labels and other complex/unsupported annotation cases remain partial.\n",
)

replace_once(
    "frontend/src/components/video/effects/annotationRegistry.ts",
    "  { kind: 'rounded_rectangle', label: 'Rounded rectangle', category: 'callout', exportSupport: 'partial', exportNote: 'Exports with square corners' },",
    "  { kind: 'rounded_rectangle', label: 'Rounded rectangle', category: 'callout', exportSupport: 'partial', exportNote: 'Static 2D cases preserve rounded geometry; animated/effect/camera/crop cases retain compatibility export' },",
)

replace_once(
    "docs/VIDEO_RENDERING.md",
    "- Rectangle, highlight, pixelate, blur-region, and normalized fallback annotation output. Complex geometry such as ellipse, arrow, and speech bubble currently exports as simpler deterministic primitives.\n",
    "- Rectangle, highlight, pixelate, blur-region, and normalized fallback annotation output. The proven static-2D `rounded_rectangle` subset is converted to a deterministic full-canvas `shape-state-v1` raster that preserves radius, fill, stroke, owner transform/opacity, timing, and layer order. Complex geometry such as ellipse, arrow, speech bubble, label, and unsupported rounded-rectangle parents still use simpler compatibility primitives.\n",
)
replace_once(
    "docs/VIDEO_RENDERING.md",
    "- Complex annotation geometry normalizes to simpler primitives.\n",
    "- Static 2D rounded rectangles in the retained parity subset preserve canonical geometry; other complex annotation geometry and unsupported rounded-rectangle parents normalize to simpler primitives.\n",
)

replace_once(
    "docs/VIDEO_EDIT_STUDIO_MEDIA_AUDIO_CAPTIONS_FAQ.md",
    "Blur and pixelate redact the composited image beneath them in preview and export. Rectangles, highlights, pixelate, blur regions, and normalized fallback annotations export. Rounded rectangles and labels export with square corners; complex shapes such as ellipses, arrows, and speech bubbles currently normalize to simpler primitives. Check the annotation's export badge before a final render.",
    "Blur and pixelate redact the composited image beneath them in preview and export. Rectangles, highlights, pixelate, blur regions, and normalized fallback annotations export. Static 2D rounded rectangles in the proven parity subset preserve their authored corner radius, fill, stroke, transform, opacity, timing, and layer order. Labels, complex shapes such as ellipses/arrows/speech bubbles, and rounded rectangles with unsupported animation/effect/camera/crop parents still normalize to compatibility primitives. Check the annotation's export badge before a final render.",
)

replace_once(
    "docs/VIDEO_STUDIO.md",
    "pixelate regions redact whatever composites beneath them in both preview and export; rounded rectangles and labels export with square corners; the remaining kinds are preview-only and reported as such.",
    "pixelate regions redact whatever composites beneath them in both preview and export; proven static 2D rounded rectangles preserve canonical rounded geometry while labels and unsupported rounded-rectangle parents remain compatibility approximations; the remaining complex kinds are reported with their current partial/preview support.",
)

plan = "docs/VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md"
replace_between(
    plan,
    "Latest merged WYSIWYG program PR:",
    "**Phase 2 — Canonical contract is complete.",
    """Latest merged WYSIWYG program PR: **#310 — Canonical cursor smoothing parity** — squash merge `14ced79493380a24d6597c8c0b00979d51165710`. Its exact cleaned head `6185864d42aa71407b9d43a3e5caf832c984c7a0` passed the complete triggered Quality/Security/browser/renderer/platform matrix, including retained `Video Cursor Smoothing Parity Evidence` and `parity-playback-canonical-v6`, before merge.\n\nCurrent implementation slice: **canonical rounded-rectangle export parity** on branch `feat/video-wysiwyg-phase3-rounded-rectangle-parity`, created directly from #310's squash result. The supported static-2D `rounded_rectangle` subset now evaluates `shape-state-v1`, emits a deterministic transparent full-canvas raster asset, and delegates x/y, uniform scale, Z rotation, opacity, timing, track visibility/order, and z-index to the existing media compositor. Fades, effects, transitions, animation/keyframes, crop, scene camera, 3D/non-static parents, unsupported CSS colors, and shapes larger than the canvas remain on the compatibility path.\n\n`parity-rounded-rectangle-v1` isolates a 240×120 shape with 24 px radius, 8 px `#f59e0b` stroke, semi-transparent `rgba(10,20,30,0.5)` fill, black 640×360 canvas, and one interior frame at 15 / 500 ms. Measurement workflow run `33935836159` was executed twice against exact head `a703942a2e3601bd122ca4d03b6b2e996f93ad8c`; both attempts reproduced the metrics and changed bounds exactly: pixel pass `0.9988194444444445`, SSIM `0.9998658039962363`, MAE `0.010321180555555556`, RMSE `0.42565309297073783`, maximum channel delta `46`, and bounds `[200,120)-[440,240)`. Retained artifacts are `9960144207` (`a1ff5b92fea5ab204cfe6ee046a521ccce998539b1535b5e2b48898821b38ec0`) and `9960194115` (`befb19594470177fafeb148a827b4d9b0b0a9123b991054a10d0d0be1ef9b78c`).\n\nThe permanent `Video Rounded Rectangle Parity Evidence` gate freezes a fixture-specific envelope without changing repository-global thresholds: pixel pass ≥`0.9985`, SSIM ≥`0.9995`, MAE ≤`0.02`, RMSE ≤`0.50`, max channel delta ≤`64`, exact changed bounds `[200,120)-[440,240)`, plus browser canonical-state, dimensions, border-width, and border-radius assertions. Strict parity is relaxed only when the authored clip satisfies the exact raster eligibility predicate; unsupported shape cases remain blocking/partial.\n\n""",
)

rounded_section = """### Canonical rounded-rectangle export parity slice\n\n- The static-2D `rounded_rectangle` subset no longer reaches FFmpeg `drawbox`; FidelityRenderer converts it to a deterministic transparent full-canvas `shape-state-v1` raster asset and reuses the existing media compositor for parent transform/opacity/timing/layer order.\n- The raster path fails closed for crop, fades, effects, transitions, animation/keyframes, scene-camera overlap, 3D/non-static parents, unsupported CSS color forms, mixed clip content, and shapes that exceed the canvas. Those cases retain the compatibility renderer and the annotations capability remains `Partial`.\n- The focused browser capture requires canonical frame state, exact 240×120 geometry, 8 px border, 24 px radius, immutable render-snapshot identity, and a complete 640×360 Chromium/FFmpeg frame comparison.\n- Two same-head hosted measurements on run `33935836159` reproduced pixel metrics and `[200,120)-[440,240)` bounds exactly. The permanent gate uses a small fixture-specific margin and leaves repository-global thresholds unchanged.\n- Strict parity permits only this same exact subset; all other shape geometry continues to report annotation parity debt. Normal playback remains deferred until a separate admission predicate and retained live playback case prove the same subset.\n\n"""
replace_once(plan, "### #289 merged result\n", rounded_section + "### #289 merged result\n")

old_next = """## Next recommended slice\n\n1. **Open and merge the cursor-export parity PR only after the final tracker-bearing exact head passes Video Cursor Parity Evidence plus every Quality/Security/browser/renderer/platform workflow triggered by the PR diff.** Pre-PR hard-gate run `33801254401` proves the frozen envelope but does not exempt the final PR head from exact validation.\n2. **Start the next slice directly from the cursor PR's actual squash result and admit only the proven static-2D cursor subset during normal playback.** Extend complete-frame atomic composition so a ready canonical cursor can coexist with supported media/resource-text/weighted consumers, while any unsupported cursor case still revokes the entire visual frame to legacy-time fallback.\n3. Add retained normal-playback cursor authority/readiness evidence before broadening the playback classifier. Preserve exact output-frame identity, owner-layer ordering, pinned palette, strict `<300 ms` ring state, and whole-frame revocation controls; do not treat the export-only hard gate as playback proof.\n4. Keep general shape playback deferred until FFmpeg shape approximations are replaced or independently proven. Cursor smoothing/animated-parent semantics and click-audio synthesis remain separate follow-on renderer slices. In parallel, add a second supported OS/toolchain run for the resource-font fixture to reduce the remaining cross-machine font-evidence debt.\n"""
new_next = """## Next recommended slice\n\n1. **Open and merge the rounded-rectangle export-parity PR only after the final tracker-bearing exact head passes Video Rounded Rectangle Parity Evidence plus every Quality/Security/browser/renderer/platform workflow triggered by the PR diff.** The two same-head measurement attempts establish the frozen envelope but do not exempt the final PR head from exact validation.\n2. **Start the next slice from that PR's actual squash result and admit only the proven static-2D rounded-rectangle subset during normal playback.** Keep complete-frame atomic authority: any unsupported shape, parent transform/effect/camera/crop case, or unavailable canonical shape state must revoke the whole visual frame to legacy-time fallback.\n3. Extend retained normal-playback evidence to a v7 fixture with one canonical rounded-rectangle case and at least one unsupported-shape fallback control before broadening any other annotation kind. Reuse `CanonicalPreviewShape`; do not create a second shape runtime unless evidence shows one is required.\n4. After rounded-rectangle playback is proven, continue annotation geometry one kind at a time (ellipse is the next obvious candidate) with export-first browser↔FFmpeg evidence. In parallel, add a second supported OS/toolchain run for the resource-font fixture. Cursor click audio should not be invented until an authored sound contract exists.\n"""
replace_once(plan, old_next, new_next)

for path in [
    "backend/internal/video/strict_parity.go",
    "backend/internal/video/strict_parity_test.go",
    "backend/internal/video/renderer_capabilities.go",
    "backend/internal/video/renderer.go",
    "backend/internal/video/timeline.go",
    "frontend/src/types/video.ts",
    "frontend/src/components/video/effects/annotationRegistry.ts",
    "docs/VIDEO_RENDERING.md",
    "docs/VIDEO_EDIT_STUDIO_MEDIA_AUDIO_CAPTIONS_FAQ.md",
    "docs/VIDEO_STUDIO.md",
    plan,
]:
    p = Path(path)
    p.write_text(p.read_text().rstrip() + "\n")

print("rounded rectangle truth/docs patch applied")
