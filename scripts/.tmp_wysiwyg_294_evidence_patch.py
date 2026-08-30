from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text(encoding="utf-8")
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected one match, found {count}: {old[:160]!r}")
    p.write_text(text.replace(old, new, 1), encoding="utf-8")


assert_script = "scripts/video-pixelate-parity-assert.mjs"
replace_once(
    assert_script,
    """      surface_status: surface?.getAttribute('data-preview-pixelate-status') ?? null,\n      surface_reason: surface?.getAttribute('data-preview-pixelate-reason') ?? null,\n      css_fallback_marker_present: Boolean(fallback),\n""",
    """      surface_status: surface?.getAttribute('data-preview-pixelate-status') ?? null,\n      surface_reason: surface?.getAttribute('data-preview-pixelate-reason') ?? null,\n      surface_background: surface?.getAttribute('data-preview-pixelate-background') ?? null,\n      css_fallback_marker_present: Boolean(fallback),\n""",
)
replace_once(assert_script, "  schema_version: 3,\n", "  schema_version: 4,\n")

plan = "docs/VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md"
replace_once(
    plan,
    """#292 closes the decoded-video frame-selection debt before promoting the measured codec/color envelope into a gate:\n""",
    """The validated #292 implementation tree, merged unchanged by #293, closes the decoded-video frame-selection debt before promoting the measured codec/color envelope into a gate:\n""",
)
replace_once(
    plan,
    """- Add deterministic `parity-pixelate-alpha-png-v1` evidence over non-black `#19324A`, using an NRGBA PNG with hidden RGB and alpha `0/64/128/192/255`, the established `403x307` region/block-20 raster, and frames `0/15/30/59`.\n- Preserve #289's byte-exact opaque-PNG gate and #293's H.264 requested/presented frame-identity plus focused ±3 RGB gate as independent controls.\n""",
    """- Add deterministic `parity-pixelate-alpha-png-v1` evidence over non-black `#19324A`, using an NRGBA PNG with hidden RGB and alpha `0/64/128/192/255`, the established `403x307` region/block-20 raster, and frames `0/15/30/59`.\n- Preserve #289's byte-exact opaque-PNG gate and #293's H.264 requested/presented frame-identity plus focused ±3 RGB gate as independent controls.\n- First retained alpha evidence on head `d2825fc7f679d668d71cf9c02b4e348afc52797f` succeeded in Video Pixelate Alpha Parity Evidence #1. Artifact `9727309505` is 10,861,044 bytes with SHA-256 `853f9d6d8ccd19b1f2a6d3105441d9d9bd68446e9bef03f6f51a252e9fd30273`; timeline SHA-256 is `e4b77a9f0c019618ee801e76f33a63944102d998d6710e43b2db833524fa7976`.\n- Browser evidence proves `canonical-canvas`, a ready surface/normalized target host, no structural/runtime deferral or CSS fallback, on all four samples. The project background is `#19324A`; the retained evidence schema is extended to record the Canvas surface's resolved background so the next exact-head run proves that consumer value directly.\n- Every alpha pixelate region covers `[71,94)-[474,401)` = `123,721` pixels. Frames `0/15/30/59` are static-source identical and each reports `pixel_pass_rate=1`, `max_channel_delta=1`, `SSIM=0.9999377268`, `MAE=0.3303723701`, and `RMSE=0.5747802798`. The whole-frame and region reports both pass repository defaults.\n- Because this lossless alpha path differs only by one RGB code value after browser-vs-FFmpeg source-over rounding, #294 freezes a **fixture-specific ±1 RGB gate with 100% pixel pass**. It does not weaken #289's zero-tolerance opaque PNG gate, #293's H.264 ±3 gate, or repository-global ±2/99.9% diagnostics.\n""",
)
replace_once(
    plan,
    """| Alpha/pixel-format conversion is assumed byte-identical across browser and FFmpeg | #294 mirrors renderer order for transparent images by compositing the decoded image over the canonical opaque project background before pixelate; the NRGBA fixture carries hidden RGB and partial alpha specifically to catch premultiplication/source-over mistakes. Transparent video remains deferred. |\n""",
    """| Alpha/pixel-format conversion is assumed byte-identical across browser and FFmpeg | #294 mirrors renderer order for transparent images by compositing the decoded image over the canonical opaque project background before pixelate. The NRGBA fixture carries hidden RGB and partial alpha specifically to catch premultiplication/source-over mistakes; retained evidence bounds browser↔FFmpeg source-over rounding to ±1 RGB at 100% pixel pass. Transparent video remains deferred. |\n""",
)
replace_once(
    plan,
    """1. **Execute #294: transparent-image project-background composition.** Retain alpha PNG browser↔renderer region evidence proving hidden RGB under alpha `0` does not leak and partial-alpha pixels follow the same source-over result before pixelate.\n2. Keep transparent decoded video fail-closed until a reliable presentation/readiness proof can distinguish legitimate transparent decoded pixels from Chromium's post-seek not-yet-rasterizable state. Do not remove the H.264 source-only guard just to broaden alpha support.\n""",
    """1. **Finish #294's exact-head gate.** Enforce the retained alpha PNG contract at `max_channel_delta <= 1` and 100% pixels within that envelope, while also retaining the resolved `#19324A` Canvas background value. Re-run the independent opaque-PNG zero-tolerance and H.264 frame-identity/±3 controls on the same exact head before merge.\n2. **Next slice after #294: transparent decoded-video readiness/alpha proof.** Keep transparent decoded video fail-closed until a reliable presentation/readiness signal can distinguish legitimate transparent decoded pixels from Chromium's post-seek not-yet-rasterizable state. Do not remove the H.264 source-only guard merely to broaden alpha support.\n""",
)

print("PR 294 retained alpha evidence patch applied")
