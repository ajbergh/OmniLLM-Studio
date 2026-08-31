from pathlib import Path


def replace_once(text: str, old: str, new: str, label: str) -> str:
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{label}: expected one match, found {count}")
    return text.replace(old, new, 1)


workflow = Path(".github/workflows/video-pixelate-background-parity.yml")
w = workflow.read_text()
w = replace_once(w, "const backgroundChannelTolerance = 2;", "const backgroundChannelTolerance = 1;", "background tolerance")
w = replace_once(w, "throw new Error('project-background pixelate ±2 RGB gate failed');", "throw new Error('project-background pixelate ±1 RGB gate failed');", "background gate error")
workflow.write_text(w)

path = Path("docs/VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md")
text = path.read_text()

text = replace_once(
    text,
    """Latest merged WYSIWYG program PR: **#295 — Merge validated pixelate alpha composition** — squash merge `f6a08f72910677ed538e356d544a1d5d1b59d620` (2026-08-30). #295 promoted the exact validated #294 implementation head `66722d2cc863fd2b6eea17ed48abd714bbf783d7` without implementation or documentation changes.\n\nCurrent implementation PR: **#296 — Prove transparent video presentation and preserve VP9 alpha** on branch `fix/video-wysiwyg-phase3-transparent-video-presentation`, created directly from #295's actual squash result `f6a08f72910677ed538e356d544a1d5d1b59d620`.\n""",
    """Latest merged WYSIWYG program PR: **#296 — Prove transparent video presentation and preserve VP9 alpha** — squash merge `7e2888fc4ef2eaadff883d3b0b5d1542710c06d9` (2026-08-30), directly from #295's actual squash result.\n\nCurrent implementation PR: **#299 — Admit project background as deterministic pixelate raster** on branch `feat/video-wysiwyg-phase3-project-background-raster`, created directly from #296's actual squash result `7e2888fc4ef2eaadff883d3b0b5d1542710c06d9`.\n""",
    "handoff",
)

text = replace_once(
    text,
    "### #296 transparent-video presentation and VP9 alpha scope",
    "### #296 merged transparent-video presentation and VP9 alpha",
    "296 heading",
)

anchor = """- #295 promoted exact validated #294 head `66722d2cc863fd2b6eea17ed48abd714bbf783d7` and squash-merged it unchanged as `f6a08f72910677ed538e356d544a1d5d1b59d620`.\n- PR #296 has no comments, reviews, or unresolved review threads.\n\n## Phase tracker\n"""
insertion = """- #295 promoted exact validated #294 head `66722d2cc863fd2b6eea17ed48abd714bbf783d7` and squash-merged it unchanged as `f6a08f72910677ed538e356d544a1d5d1b59d620`.\n- Final tracker-bearing #296 head `ca97e06449600c77f443b380098bb139f34ca8ea` passed Quality Gate #1757, Security Scan #1762, Video Transparent Pixelate Parity Evidence #16, Video Pixelate Parity Evidence #82, Video Pixelate Alpha Parity Evidence #29, and all applicable Linux/macOS/browser/container assurance workflows. PR #296 had no comments, reviews, or unresolved review threads and squash-merged as `7e2888fc4ef2eaadff883d3b0b5d1542710c06d9`.\n\n### #299 project-background pixelate raster scope\n\n#299 broadens deterministic pixelate Canvas admission by exactly one source class whose renderer order and color semantics were already explicit: the canonical project background itself.\n\n- `preview-pixelate-backdrop-plan-v1` now represents the no-lower-visual-layer case as an explicit `project-background` raster source with `runtimeRequirements: []`; one-media image/video behavior remains unchanged and more than one lower layer still fails closed.\n- `PreviewPixelateCanvas` paints the renderer-matched project background unconditionally and only resolves/paints a mounted media source when a media backdrop actually exists. `PreviewPixelateBackdropConsumer` likewise makes poster and presentation-token logic conditional on a real media backdrop.\n- This is deliberately pixelate-specific. `previewFrameWeightedPairRaster.ts` remains media-only, and text, shape, cursor, effects, transitions, multi-layer backdrops, and other painter classes are not broadened in this slice.\n- Shape was inspected as the initially obvious next class and rejected for this slice because the current legacy FFmpeg shape path still contains renderer-specific approximations such as integerized box geometry and rounded-rectangle simplification. #299 does not promote those approximations into canonical Canvas semantics.\n- `parity-pixelate-background-v1` is a deterministic 512×512/30 fps fixture over `#19324A` with exactly one visible pixelate layer, the established `[71,94)-[474,401)` / 123,721-pixel region and block size `20`, and canonical frames `0/15/30/59`. There is no visual media asset or decoder dependency.\n- Pre-PR guarded run `33352378391` passed the focused pixelate planner Vitest and frontend production build before implementation commit `686844c17101c771dfad1a8bc1b18261885cd39f`. Guarded run `33352467556` then passed Go fixture validation, CLI fixture generation, the focused planner test, JavaScript syntax validation, and diff checks before evidence commit `156fa4c38a3e48247975561906d523c2d0ce5e11`.\n- First retained PR evidence on head `f0a9496f29b66f94bd7f318e8f7120c986dceac8` passed Video Pixelate Background Parity Evidence #1. Browser evidence proves `canonical-canvas`, `surface_background=#19324A`, `surface_backdrop_clip=project-background`, ready execution, no structural/runtime deferral or CSS fallback, and no video presentation request/ready token on all four samples.\n- Both whole-frame and region reports pass repository defaults. Every 123,721-pixel region on frames `0/15/30/59` reports `pixel_pass_rate=1`, `max_channel_delta=1`, `SSIM=0.9997632514784561`, `MAE=1`, and `RMSE=1`. Based on that retained measurement, #299 tightens the project-background fixture-specific acceptance contract to **100% of region pixels within ±1 RGB**; repository-global ±2/99.9% defaults remain unchanged.\n- Retained background artifact `9744150134` is 11,086,886 bytes with SHA-256 `f3374a3c6a6f1523910e0cdac831e2e223a751148b7bdb758ea36a467df35248`; retained timeline SHA-256 is `e2dd081bafd070c344acac4f84f4c2fe9db9b4272dd57360ffbdc34e40cd44cf`.\n- Independent controls are green on the same implementation head: Video Transparent Pixelate Parity Evidence #18, Video Pixelate Alpha Parity Evidence #31, and Video Pixelate Parity Evidence #84 all pass, preserving transparent VP9 ±4/presentation identity, partial-alpha PNG ±1, opaque-PNG exactness, and H.264 ±3/frame identity.\n\n## Phase tracker\n"""
text = replace_once(text, anchor, insertion, "299 section")

text = replace_once(
    text,
    """| Phase 0 — Reproducible parity baseline | **In progress** | Deterministic 103-frame visual/audio/delivery evidence exists. #289 added the byte-exact opaque-PNG gate; #291 added H.264 measurement; #293 proves decoded frame identity on 0/15/30/59 and gates the focused ±3 H.264/yuv420p envelope only after identity proof. #295 adds deterministic partial-alpha PNG/background-composition evidence; #296 adds retained VP9 alpha-decoder, requested/presented-frame, and focused ±4 transparent-video evidence. Resource-font fixture coverage and second-platform retained evidence remain. |""",
    """| Phase 0 — Reproducible parity baseline | **In progress** | Deterministic 103-frame visual/audio/delivery evidence exists. #289 added the byte-exact opaque-PNG gate; #291 added H.264 measurement; #293 proves decoded frame identity on 0/15/30/59 and gates the focused ±3 H.264/yuv420p envelope only after identity proof. #295 adds deterministic partial-alpha PNG/background-composition evidence; #296 adds retained VP9 alpha-decoder, requested/presented-frame, and focused ±4 transparent-video evidence; #299 adds retained zero-readiness project-background evidence at focused ±1. Resource-font fixture coverage and second-platform retained evidence remain. |""",
    "phase 0",
)

text = replace_once(
    text,
    """| Phase 3 — Shared preview composition | **In progress** | #261–#289 merged deterministic activity/source/transform/view/perspective/media geometry/effects/transitions, canonical text/shape/cursor painters, font readiness/layout snapshots, deterministic pixelate semantics, exact opaque-PNG evidence, and sampled-render fan-out. #291/#293 close retained H.264 measurement and deterministic frame identity. #295 merges renderer-ordered project-background/source-over composition for transparent images; #296 adds mounted-video presentation tokens, transparent VP9 alpha preservation, source-over video composition, and browser/FFmpeg boundary alignment without changing canonical source/audio time. Weighted-raster broadening, normal-playback canonicalization, diagnostics/rollback, and audio consumption remain. |""",
    """| Phase 3 — Shared preview composition | **In progress** | #261–#289 merged deterministic activity/source/transform/view/perspective/media geometry/effects/transitions, canonical text/shape/cursor painters, font readiness/layout snapshots, deterministic pixelate semantics, exact opaque-PNG evidence, and sampled-render fan-out. #291/#293 close retained H.264 measurement and deterministic frame identity. #295 merges renderer-ordered project-background/source-over composition for transparent images; #296 adds mounted-video presentation tokens, transparent VP9 alpha preservation, source-over video composition, and browser/FFmpeg boundary alignment without changing canonical source/audio time; #299 admits project background itself as an explicit zero-readiness pixelate raster source while leaving shared weighted-pair and unsupported painter classes fail-closed. Normal-playback canonicalization, diagnostics/rollback, further independently evidenced raster classes, and audio consumption remain. |""",
    "phase 3",
)

text = replace_once(
    text,
    """- `shape-state-v1` owns shape geometry/style. #285 defines `preview-pixelate-raster-v1`; #286 defines fail-closed backdrop admission; #287 aligns both FFmpeg scale passes; #288 consumes only decoded opaque browser regions that pass structural/runtime proof; #289 proves byte-exact retained output for the isolated opaque-PNG static path; #291 extends retained evidence to decoded H.264 without weakening alpha or PNG exactness; #292 proves decoded frame identity before freezing the codec/color gate.""",
    """- `shape-state-v1` owns shape geometry/style. #285 defines `preview-pixelate-raster-v1`; #286 defines fail-closed backdrop admission; #287 aligns both FFmpeg scale passes; #288 consumes only decoded opaque browser regions that pass structural/runtime proof; #289 proves byte-exact retained output for the isolated opaque-PNG static path; #291 extends retained evidence to decoded H.264 without weakening alpha or PNG exactness; #293 freezes decoded H.264 frame identity/color acceptance; #295 admits renderer-ordered transparent-image source-over composition; #296 proves mounted transparent-video presentation readiness and alpha-preserving VP9 decode; #299 adds the project background itself as an explicit zero-readiness pixelate raster source without broadening legacy shape approximations.""",
    "shape history",
)

text = replace_once(
    text,
    """| #295 | Renderer-ordered partial-alpha image composition and focused ±1 PNG gate | `f6a08f72910677ed538e356d544a1d5d1b59d620` |""",
    """| #295 | Renderer-ordered partial-alpha image composition and focused ±1 PNG gate | `f6a08f72910677ed538e356d544a1d5d1b59d620` |\n| #296 | Mounted-video presentation tokens, VP9 alpha preservation, and focused ±4 transparent-video gate | `7e2888fc4ef2eaadff883d3b0b5d1542710c06d9` |""",
    "merged sequence",
)

text = replace_once(
    text,
    """Recent lineage: #284 from #283 squash `3543ddf7...`; #285 from #284 `7884fef8...`; #286 from #285 `64e34450...`; #287 from #286 `40e895ee...`; #288 from #287 `2774ee76...`; #289 from #288 `7be8e86f...`; #290 validated from #289 squash; #291 mirrored that exact validated #290 head and merged as `7d5f36c3...`; #292 validated directly from #291; #293 mirrored exact validated #292 head and squash-merged as `9850370c2aa25a076f3272077062aeab08c1f326`; #294 validated directly from #293; #295 promoted exact validated #294 head `66722d2c...` and squash-merged as `f6a08f72910677ed538e356d544a1d5d1b59d620`; **#296 is directly from #295 squash `f6a08f72910677ed538e356d544a1d5d1b59d620`**.""",
    """Recent lineage: #284 from #283 squash `3543ddf7...`; #285 from #284 `7884fef8...`; #286 from #285 `64e34450...`; #287 from #286 `40e895ee...`; #288 from #287 `2774ee76...`; #289 from #288 `7be8e86f...`; #290 validated from #289 squash; #291 mirrored that exact validated #290 head and merged as `7d5f36c3...`; #292 validated directly from #291; #293 mirrored exact validated #292 head and squash-merged as `9850370c2aa25a076f3272077062aeab08c1f326`; #294 validated directly from #293; #295 promoted exact validated #294 head `66722d2c...` and squash-merged as `f6a08f72910677ed538e356d544a1d5d1b59d620`; #296 was directly from #295 and squash-merged as `7e2888fc4ef2eaadff883d3b0b5d1542710c06d9`; **#299 is directly from #296 squash `7e2888fc4ef2eaadff883d3b0b5d1542710c06d9`**.""",
    "lineage",
)

text = replace_once(
    text,
    """The focused `parity-pixelate-opaque-v1` fixture is a byte-exact non-regression gate for the isolated opaque-PNG static pixelate path. `parity-pixelate-decoded-video-v1` is additive decoded-media evidence: #293 proves deterministic decoded-frame identity first and then applies the explicit ±3 RGB H.264/yuv420p pixelate-region envelope without changing repository-global parity defaults. `parity-pixelate-alpha-png-v1` is #295's additive straight-alpha/source-over project-background control. `parity-pixelate-alpha-video-v1` is #296's additive transparent VP9 control: it freezes the decoder-alpha contract, requires mounted-video presentation-token identity on frames `0/15/30/59`, and gates the pixelate region at 100% within ±4 RGB.""",
    """The focused `parity-pixelate-opaque-v1` fixture is a byte-exact non-regression gate for the isolated opaque-PNG static pixelate path. `parity-pixelate-decoded-video-v1` is additive decoded-media evidence: #293 proves deterministic decoded-frame identity first and then applies the explicit ±3 RGB H.264/yuv420p pixelate-region envelope without changing repository-global parity defaults. `parity-pixelate-alpha-png-v1` is #295's additive straight-alpha/source-over project-background control. `parity-pixelate-alpha-video-v1` is #296's additive transparent VP9 control: it freezes the decoder-alpha contract, requires mounted-video presentation-token identity on frames `0/15/30/59`, and gates the pixelate region at 100% within ±4 RGB. `parity-pixelate-background-v1` is #299's zero-media control: it requires the explicit `project-background` raster source, no decoder/presentation token, and 100% of the retained region within ±1 RGB.""",
    "phase 0 fixtures",
)

text = replace_once(
    text,
    """| Pixelate raster math is confused with backdrop-source parity | #285 owns grid/sample-index math; #286 owns structural admission; #287 owns FFmpeg scaler selection; #288 owns runtime acquisition; #289 owns byte-exact PNG evidence; #291 owns decoded-video measurement; #293 owns decoded-frame identity/codec acceptance; #294 owns project-background/source-over image-alpha composition. None is substituted for another. |""",
    """| Pixelate raster math is confused with backdrop-source parity | #285 owns grid/sample-index math; #286 owns structural admission; #287 owns FFmpeg scaler selection; #288 owns runtime acquisition; #289 owns byte-exact PNG evidence; #291 owns decoded-video measurement; #293 owns decoded-frame identity/codec acceptance; #295 owns project-background/source-over image-alpha composition; #296 owns mounted-video presentation/transparent-alpha evidence; #299 owns explicit zero-readiness project-background raster admission. None is substituted for another. |""",
    "pixelate risk",
)

text = replace_once(
    text,
    "| CI scheduling hides code state | Only actually executed checks count. |",
    "| A renderer-specific painter approximation is mistaken for an exact raster source | #299 explicitly rejects broad shape admission after inspecting current FFmpeg shape approximations. New painter/source classes require independent ordering, readiness, geometry, and retained pixel evidence before classifier admission. |\n| CI scheduling hides code state | Only actually executed checks count. |",
    "new source-class risk",
)

start = text.index("## Next recommended slice\n")
text = text[:start] + """## Next recommended slice

1. **Merge #299 only after the tracker-bearing exact-head wave repeats the measured project-background contract at 100% within ±1 RGB and keeps all existing pixelate controls green.** Preserve the independent opaque-PNG exact, partial-alpha PNG ±1, H.264 ±3/frame-identity, and transparent VP9 ±4/presentation-identity contracts.
2. **Next Phase 3 slice: audit one non-media painter class for exact preview/export raster eligibility, starting with cursor because `cursor-state-v1` already owns exact rational sampling and renderer behavior is narrower than general shape/text.** Do not admit cursor unless source-over order, transform/geometry, readiness, and renderer pixels can be retained without inventing new semantics.
3. If cursor cannot meet that bar without renderer-specific approximation, leave it fail-closed and move directly to normal-playback canonicalization rather than broadening a weak class. General shape remains deferred until legacy renderer approximations are removed or replaced by a shared deterministic painter.
4. Add resource-font fixture coverage and second-supported-OS/FFmpeg retained evidence before calling cross-machine Phase 0 visual identity closed.
5. Continue Phase 3 with normal-playback canonicalization and explicit diagnostics/rollback once the next safe raster-source decision is resolved.
6. Then make preview/export consume `audio-graph-v1` exactly as the entry point to Phase 6 audio parity closure.
"""

path.write_text(text)
