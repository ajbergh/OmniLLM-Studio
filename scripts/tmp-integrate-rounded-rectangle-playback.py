from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text()
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: anchor count={count}, want 1 for {old[:120]!r}")
    p.write_text(text.replace(old, new, 1))


legacy = 'frontend/src/components/video/VideoPreviewCanvasLegacy.tsx'
replace_once(
    legacy,
    "import { ShapePreview } from './ShapePreview';\n",
    "import { ShapePreview } from './ShapePreview';\nimport { CanonicalPreviewShape } from './PreviewCanonicalPainters';\n",
)
replace_once(
    legacy,
    "    const effectPaint = resolvePreviewFrameEffectPaint(entry.canonicalState, clip.effects);\n\n    const wrapperStyle: CSSProperties = {",
    "    const effectPaint = resolvePreviewFrameEffectPaint(entry.canonicalState, clip.effects);\n    // Deterministic paused parity already paints canonical shapes through the\n    // outer portal. Normal playback consumes the same FrameState shape here\n    // only after the whole-frame admission gate has accepted the exact subset.\n    const canonicalPlaybackShape = deterministicFrame === null\n      && playbackDecision.canonicalFrame !== null\n      && clip.shape\n      && !hasLiveOverride\n      ? entry.canonicalState?.shape\n      : undefined;\n\n    const wrapperStyle: CSSProperties = {",
)
replace_once(
    legacy,
    "    } else if (clip.shape) {\n      content = (\n        <ShapePreview\n          shape={clip.shape}\n          clip={clip}\n          stageScale={stageScale}\n          canvasHeight={canvasHeight}\n          liveWidth={liveShapeWidth}\n          liveHeight={liveShapeHeight}\n        />\n      );\n",
    "    } else if (clip.shape) {\n      content = canonicalPlaybackShape ? (\n        <CanonicalPreviewShape shape={canonicalPlaybackShape} stageScale={stageScale} />\n      ) : (\n        <ShapePreview\n          shape={clip.shape}\n          clip={clip}\n          stageScale={stageScale}\n          canvasHeight={canvasHeight}\n          liveWidth={liveShapeWidth}\n          liveHeight={liveShapeHeight}\n        />\n      );\n",
)
replace_once(
    legacy,
    "        data-preview-media-geometry-mode={isMedia ? (canonicalMediaGeometry ? 'canonical-frame' : 'legacy-object-fit') : undefined}\n        data-preview-effect-state-mode={effectPaint.mode}",
    "        data-preview-media-geometry-mode={isMedia ? (canonicalMediaGeometry ? 'canonical-frame' : 'legacy-object-fit') : undefined}\n        data-preview-shape-state-mode={clip.shape ? (canonicalPlaybackShape ? 'canonical-frame' : 'legacy-time') : undefined}\n        data-preview-shape-playback-consumer={isPlaying && clip.shape ? (canonicalPlaybackShape ? 'canonical-inline' : 'legacy-time-fallback') : undefined}\n        data-preview-shape-playback-clip-id={clip.shape ? clip.id : undefined}\n        data-preview-shape-kind={canonicalPlaybackShape?.kind}\n        data-preview-shape-width={canonicalPlaybackShape?.width}\n        data-preview-shape-height={canonicalPlaybackShape?.height}\n        data-preview-shape-stroke-width={canonicalPlaybackShape?.stroke_width}\n        data-preview-shape-corner-radius={canonicalPlaybackShape?.corner_radius}\n        data-preview-shape-fill={canonicalPlaybackShape?.fill}\n        data-preview-shape-stroke={canonicalPlaybackShape?.stroke}\n        data-preview-effect-state-mode={effectPaint.mode}",
)


test = 'frontend/src/components/video/previewPlaybackCanonicalization.test.ts'
replace_once(
    test,
    "type Plan = NonNullable<Parameters<typeof resolvePreviewPlaybackCanonicalization>[3]>;\n\nfunction mediaLayer",
    "type Plan = NonNullable<Parameters<typeof resolvePreviewPlaybackCanonicalization>[3]>;\n\nconst playbackContext: NonNullable<Parameters<typeof resolvePreviewPlaybackCanonicalization>[4]> = {\n  fps: 30,\n  canvasWidth: 640,\n  canvasHeight: 360,\n  scenes: [],\n};\n\nfunction roundedShapeLayer(id = 'shape'): Layer {\n  return {\n    clip: {\n      id,\n      start_ms: 0,\n      duration_ms: 1200,\n      shape: {\n        kind: 'rounded_rectangle', width: 240, height: 120,\n        fill: 'rgba(10,20,30,0.5)', stroke: '#f59e0b', stroke_width: 8, corner_radius: 24,\n      },\n      transform: { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1 },\n      effects: [],\n      transitions: [],\n      keyframes: [],\n      animation_blocks: [],\n    },\n    canonicalState: {\n      authoritative: true,\n      shape: {\n        contract_version: 'shape-state-v1',\n        kind: 'rounded_rectangle',\n        width: 240, height: 120, fill: 'rgba(10,20,30,0.5)', stroke: '#f59e0b',\n        stroke_width: 8, blur_radius: 12, corner_radius: 24,\n      },\n    },\n  } as Layer;\n}\n\nfunction mediaLayer",
)
replace_once(
    test,
    "  it('admits a clean source-over transition plan for media-only layers', () => {",
    "  it('admits the export-proven standalone rounded rectangle subset', () => {\n    expect(resolvePreviewPlaybackCanonicalization(\n      12,\n      { authoritative: true },\n      [roundedShapeLayer()],\n      plan(),\n      playbackContext,\n    )).toEqual({ mode: 'canonical-playback', canonicalFrame: 12 });\n  });\n\n  it('fails the whole frame closed when a rounded rectangle leaves the export-proven subset', () => {\n    const shape = roundedShapeLayer();\n    shape.clip.fade_in_ms = 100;\n    expect(resolvePreviewPlaybackCanonicalization(\n      12,\n      { authoritative: true },\n      [shape],\n      plan(),\n      playbackContext,\n    )).toEqual({\n      mode: 'legacy-time-fallback',\n      canonicalFrame: null,\n      deferredReason: 'shape-playback-deferred:shape:fade-unsupported',\n    });\n  });\n\n  it('keeps other canonical shape kinds fail-closed', () => {\n    const shape = roundedShapeLayer('ellipse');\n    shape.clip.shape = { ...shape.clip.shape!, kind: 'ellipse' };\n    if (shape.canonicalState?.shape) shape.canonicalState.shape.kind = 'ellipse';\n    expect(resolvePreviewPlaybackCanonicalization(\n      12,\n      { authoritative: true },\n      [shape],\n      plan(),\n      playbackContext,\n    ).deferredReason).toBe('shape-playback-deferred:ellipse:shape-kind-unsupported');\n  });\n\n  it('admits a clean source-over transition plan for media-only layers', () => {",
)
replace_once(
    test,
    "  it.each([\n    ['shape', { ...mediaLayer(), clip: { id: 'shape', shape: {} } }],\n    ['cursor',",
    "  it.each([\n    ['cursor',",
)
replace_once(
    test,
    "  it('keeps mixed supported text plus unsupported painter frames on one deterministic fallback', () => {",
    "  it('keeps ready text plus an unsupported shape on one deterministic fallback', () => {\n    const title = textLayer();\n    const shape = roundedShapeLayer();\n    shape.clip.fade_in_ms = 100;\n    const layers = [title, shape];\n    const identity = previewTextPlaybackPlanIdentity(layers);\n    publishPreviewTextPlaybackRuntime({\n      frameIndex: 8,\n      planKey: previewTextPlaybackPlanKey(8, layers),\n      planIdentity: identity,\n      status: 'ready',\n    });\n    expect(resolvePreviewPlaybackCanonicalization(\n      8,\n      { authoritative: true },\n      layers,\n      plan(),\n      playbackContext,\n    )).toEqual({\n      mode: 'legacy-time-fallback',\n      canonicalFrame: null,\n      deferredReason: 'shape-playback-deferred:shape:fade-unsupported',\n    });\n  });\n\n  it('keeps mixed supported text plus unsupported painter frames on one deterministic fallback', () => {",
)
