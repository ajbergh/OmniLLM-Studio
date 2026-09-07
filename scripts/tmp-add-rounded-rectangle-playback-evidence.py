from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text()
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: anchor count={count}, want 1 for {old[:140]!r}")
    p.write_text(text.replace(old, new, 1))


fixture = 'backend/internal/video/parity_fixture_playback.go'
replace_once(
    fixture,
    'const PlaybackCanonicalParityFixtureName = "parity-playback-canonical-v6"',
    'const PlaybackCanonicalParityFixtureName = "parity-playback-canonical-v7"',
)
replace_once(
    fixture,
    '\tExpectedCursorClipID     string   `json:"expected_cursor_clip_id,omitempty"`\n\tRequireCursorSurface     bool     `json:"require_cursor_surface,omitempty"`',
    '\tExpectedCursorClipID     string   `json:"expected_cursor_clip_id,omitempty"`\n\tExpectedShapeConsumer    string   `json:"expected_shape_consumer,omitempty"`\n\tExpectedShapeClipID      string   `json:"expected_shape_clip_id,omitempty"`\n\tRequireShapeSurface      bool     `json:"require_shape_surface,omitempty"`\n\tRequireCursorSurface     bool     `json:"require_cursor_surface,omitempty"`',
)
replace_once(
    fixture,
    '// Mixed v6 cases explicitly prove that supported media/text/cursor and\n// weighted/text/cursor surfaces share one canonical frame decision. Exact cursor\n// samples must stay frame-addressed while any unsupported cursor parent or runtime\n// debt still revokes authority for the complete visual frame.',
    '// Mixed v7 cases explicitly prove that supported media/text/cursor/shape and\n// weighted/text/cursor surfaces share one canonical frame decision. Exact cursor\n// samples stay frame-addressed; the export-proven rounded-rectangle subset may\n// paint synchronously, while unsupported shape/cursor debt still revokes authority\n// for the complete visual frame.',
)
replace_once(
    fixture,
    '\timage := mediaClip("playback-image", "asset-square", 1400, 1200)\n\tresourceText := textClip(\n',
    '\troundedRectangle := TimelineClip{\n\t\tID:         "playback-rounded-rectangle",\n\t\tStartMS:    1200,\n\t\tDurationMS: 300,\n\t\tTrimOutMS:  300,\n\t\tTransform:  transform(),\n\t\tShape: &TimelineShape{\n\t\t\tKind: ShapeKindRoundedRectangle, Width: 240, Height: 120,\n\t\t\tFill: "rgba(10,20,30,0.5)", Stroke: "#f59e0b", StrokeWidth: 8, CornerRadius: 24,\n\t\t},\n\t\tEffects:   []TimelineEffect{},\n\t\tKeyframes: []TimelineKeyframe{},\n\t}\n\timage := mediaClip("playback-image", "asset-square", 1500, 1200)\n\tellipse := roundedRectangle\n\tellipse.ID = "playback-ellipse"\n\tellipse.StartMS = 2700\n\tellipse.Shape = &TimelineShape{\n\t\tKind: ShapeKindEllipse, Width: 240, Height: 120,\n\t\tFill: "rgba(10,20,30,0.5)", Stroke: "#f59e0b", StrokeWidth: 8, CornerRadius: 24,\n\t}\n\tresourceText := textClip(\n',
)
replace_once(
    fixture,
    '\t\t2800,\n\t)\n\tfamilyText := textClip(',
    '\t\t3000,\n\t)\n\tfamilyText := textClip(',
)
replace_once(
    fixture,
    '\t\ttrack("track-video", video),\n\t\ttrack("track-image", image),\n\t\ttrack("track-text-resource", resourceText),',
    '\t\ttrack("track-video", video),\n\t\ttrack("track-rounded-rectangle", roundedRectangle),\n\t\ttrack("track-image", image),\n\t\ttrack("track-ellipse", ellipse),\n\t\ttrack("track-text-resource", resourceText),',
)
replace_once(
    fixture,
    '\t\t{Name: "video-canonical-playback", FrameIndex: 6, ObserveMS: 500, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-none", RequireAdvancingFrames: true},\n\t\t{Name: "image-canonical-playback", FrameIndex: 48, ObserveMS: 500, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-none", RequireAdvancingFrames: true},\n\t\t{Name: "resource-text-canonical-playback", FrameIndex: 90, ObserveMS: 450,',
    '\t\t{Name: "video-canonical-playback", FrameIndex: 6, ObserveMS: 500, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-none", RequireAdvancingFrames: true},\n\t\t{Name: "rounded-rectangle-canonical-playback", FrameIndex: 37, ObserveMS: 250, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-none", ExpectedShapeConsumer: "canonical-inline", ExpectedShapeClipID: "playback-rounded-rectangle", RequireShapeSurface: true, RequireAdvancingFrames: true},\n\t\t{Name: "image-canonical-playback", FrameIndex: 48, ObserveMS: 500, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-none", RequireAdvancingFrames: true},\n\t\t{Name: "ellipse-shape-fallback", FrameIndex: 82, ObserveMS: 250, ExpectedMode: "legacy-time-fallback", ExpectedReason: "shape-playback-deferred:playback-ellipse:shape-kind-unsupported", ExpectedTransitionMode: "legacy", ExpectedShapeConsumer: "legacy-time-fallback", ExpectedShapeClipID: "playback-ellipse", RequireShapeSurface: true},\n\t\t{Name: "resource-text-canonical-playback", FrameIndex: 93, ObserveMS: 450,',
)

fixture_test = 'backend/internal/video/parity_fixture_playback_test.go'
replace_once(fixture_test, 'if len(cases) != 19 {\n\t\tt.Fatalf("playback parity cases = %d, want 19", len(cases))', 'if len(cases) != 21 {\n\t\tt.Fatalf("playback parity cases = %d, want 21", len(cases))')
replace_once(
    fixture_test,
    '\t\tif testCase.ExpectedCursorConsumer != "" && testCase.ExpectedCursorClipID == "" {\n\t\t\tt.Fatalf("cursor consumer case %q is missing an expected clip id", testCase.Name)\n\t\t}\n\t\tif testCase.RequireCursorSurface {',
    '\t\tif testCase.ExpectedCursorConsumer != "" && testCase.ExpectedCursorClipID == "" {\n\t\t\tt.Fatalf("cursor consumer case %q is missing an expected clip id", testCase.Name)\n\t\t}\n\t\tif testCase.ExpectedShapeConsumer != "" && testCase.ExpectedShapeClipID == "" {\n\t\t\tt.Fatalf("shape consumer case %q is missing an expected clip id", testCase.Name)\n\t\t}\n\t\tif testCase.RequireShapeSurface {\n\t\t\tif testCase.ExpectedShapeClipID == "" {\n\t\t\t\tt.Fatalf("shape surface case %q is missing shape identity", testCase.Name)\n\t\t\t}\n\t\t\tif testCase.ExpectedShapeConsumer != "canonical-inline" && testCase.ExpectedShapeConsumer != "legacy-time-fallback" {\n\t\t\t\tt.Fatalf("shape surface case %q has invalid consumer %q", testCase.Name, testCase.ExpectedShapeConsumer)\n\t\t\t}\n\t\t\tif testCase.ExpectedMode == "canonical-playback" && testCase.ExpectedShapeConsumer != "canonical-inline" {\n\t\t\t\tt.Fatalf("canonical shape case %q is missing canonical consumer expectation", testCase.Name)\n\t\t\t}\n\t\t\tif testCase.ExpectedMode == "legacy-time-fallback" && testCase.ExpectedShapeConsumer != "legacy-time-fallback" {\n\t\t\t\tt.Fatalf("fallback shape case %q must retain legacy consumer", testCase.Name)\n\t\t\t}\n\t\t}\n\t\tif testCase.RequireCursorSurface {',
)
replace_once(
    fixture_test,
    '\t\t"video-canonical-playback",\n\t\t"image-canonical-playback",\n\t\t"resource-text-canonical-playback",',
    '\t\t"video-canonical-playback",\n\t\t"rounded-rectangle-canonical-playback",\n\t\t"image-canonical-playback",\n\t\t"ellipse-shape-fallback",\n\t\t"resource-text-canonical-playback",',
)

capture = 'scripts/video-playback-parity-capture.mjs'
replace_once(capture, 'schema_version: 3, fixture: fixture.name, cases: results', 'schema_version: 4, fixture: fixture.name, cases: results')
replace_once(capture, 'schema_version: 3,\n    fixture: fixture.name,', 'schema_version: 4,\n    fixture: fixture.name,')
replace_once(
    capture,
    '    if (testCase.expected_cursor_consumer || testCase.expected_cursor_clip_id) {\n      await page.waitForFunction((expected) => {\n        const surfaces = [...document.querySelectorAll(\'[data-preview-cursor-playback-clip-id]\')];\n        return surfaces.some((surface) => (!expected.clipId || surface.dataset.previewCursorPlaybackClipId === expected.clipId)\n          && (!expected.consumer || surface.dataset.previewCursorPlaybackConsumer === expected.consumer));\n      }, {\n        consumer: testCase.expected_cursor_consumer || \'\',\n        clipId: testCase.expected_cursor_clip_id || \'\',\n      }, { timeout: 3_000 });\n    }\n\n    const observations =',
    '    if (testCase.expected_cursor_consumer || testCase.expected_cursor_clip_id) {\n      await page.waitForFunction((expected) => {\n        const surfaces = [...document.querySelectorAll(\'[data-preview-cursor-playback-clip-id]\')];\n        return surfaces.some((surface) => (!expected.clipId || surface.dataset.previewCursorPlaybackClipId === expected.clipId)\n          && (!expected.consumer || surface.dataset.previewCursorPlaybackConsumer === expected.consumer));\n      }, {\n        consumer: testCase.expected_cursor_consumer || \'\',\n        clipId: testCase.expected_cursor_clip_id || \'\',\n      }, { timeout: 3_000 });\n    }\n    if (testCase.expected_shape_consumer || testCase.expected_shape_clip_id) {\n      await page.waitForFunction((expected) => {\n        const surfaces = [...document.querySelectorAll(\'[data-preview-shape-playback-clip-id]\')];\n        return surfaces.some((surface) => (!expected.clipId || surface.dataset.previewShapePlaybackClipId === expected.clipId)\n          && (!expected.consumer || surface.dataset.previewShapePlaybackConsumer === expected.consumer));\n      }, {\n        consumer: testCase.expected_shape_consumer || \'\',\n        clipId: testCase.expected_shape_clip_id || \'\',\n      }, { timeout: 3_000 });\n    }\n\n    const observations =',
)
replace_once(
    capture,
    '        const cursorSurfaces = [...stage.querySelectorAll(\'[data-preview-cursor-playback-clip-id]\')];\n        rows.push({',
    '        const cursorSurfaces = [...stage.querySelectorAll(\'[data-preview-cursor-playback-clip-id]\')];\n        const shapeSurfaces = [...stage.querySelectorAll(\'[data-preview-shape-playback-clip-id]\')];\n        rows.push({',
)
replace_once(
    capture,
    '          cursor_surfaces: cursorSurfaces.map((surface) => ({\n            clip_id: surface.dataset.previewCursorPlaybackClipId || \'\',\n            state_mode: surface.dataset.previewCursorStateMode || \'\',\n            consumer: surface.dataset.previewCursorPlaybackConsumer || \'\',\n            x: numberOrNull(surface.dataset.previewCursorX),\n            y: numberOrNull(surface.dataset.previewCursorY),\n            click: surface.dataset.previewCursorClick === \'true\',\n            scale: numberOrNull(surface.dataset.previewCursorScale),\n            highlight: surface.dataset.previewCursorHighlight === \'true\',\n            click_rings: surface.dataset.previewCursorClickRings === \'true\',\n          })),\n          audio_current_time:',
    '          cursor_surfaces: cursorSurfaces.map((surface) => ({\n            clip_id: surface.dataset.previewCursorPlaybackClipId || \'\',\n            state_mode: surface.dataset.previewCursorStateMode || \'\',\n            consumer: surface.dataset.previewCursorPlaybackConsumer || \'\',\n            x: numberOrNull(surface.dataset.previewCursorX),\n            y: numberOrNull(surface.dataset.previewCursorY),\n            click: surface.dataset.previewCursorClick === \'true\',\n            scale: numberOrNull(surface.dataset.previewCursorScale),\n            highlight: surface.dataset.previewCursorHighlight === \'true\',\n            click_rings: surface.dataset.previewCursorClickRings === \'true\',\n          })),\n          shape_surfaces: shapeSurfaces.map((surface) => ({\n            clip_id: surface.dataset.previewShapePlaybackClipId || \'\',\n            state_mode: surface.dataset.previewShapeStateMode || \'\',\n            consumer: surface.dataset.previewShapePlaybackConsumer || \'\',\n            kind: surface.dataset.previewShapeKind || \'\',\n            width: numberOrNull(surface.dataset.previewShapeWidth),\n            height: numberOrNull(surface.dataset.previewShapeHeight),\n            stroke_width: numberOrNull(surface.dataset.previewShapeStrokeWidth),\n            corner_radius: numberOrNull(surface.dataset.previewShapeCornerRadius),\n            fill: surface.dataset.previewShapeFill || \'\',\n            stroke: surface.dataset.previewShapeStroke || \'\',\n          })),\n          audio_current_time:',
)
replace_once(
    capture,
    '    if (testCase.require_cursor_surface) {\n      const cursor = row.cursor_surfaces.find((surface) => surface.clip_id === testCase.expected_cursor_clip_id);',
    '    if (testCase.require_shape_surface) {\n      const shape = row.shape_surfaces.find((surface) => surface.clip_id === testCase.expected_shape_clip_id);\n      if (!shape) {\n        errors.push(`shape surface ${testCase.expected_shape_clip_id || \'<unspecified>\'} is missing`);\n        break;\n      }\n      if (shape.consumer !== testCase.expected_shape_consumer) {\n        errors.push(`shape consumer ${shape.consumer || \'<empty>\'}, want ${testCase.expected_shape_consumer}`);\n        break;\n      }\n      const expectedStateMode = testCase.expected_mode === \'canonical-playback\' ? \'canonical-frame\' : \'legacy-time\';\n      if (shape.state_mode !== expectedStateMode) {\n        errors.push(`shape state mode ${shape.state_mode || \'<empty>\'}, want ${expectedStateMode}`);\n        break;\n      }\n      if (testCase.expected_mode === \'canonical-playback\') {\n        const expected = expectedShapeStateAtFrame(timeline, testCase.expected_shape_clip_id, row.visual_frame_index);\n        if (!expected) {\n          errors.push(`canonical shape expectation ${testCase.expected_shape_clip_id} is unavailable`);\n          break;\n        }\n        for (const field of [\'width\', \'height\', \'stroke_width\', \'corner_radius\']) {\n          if (!Number.isFinite(shape[field]) || Math.abs(shape[field] - expected[field]) > 1e-9) {\n            errors.push(`shape ${field} ${shape[field]}, want ${expected[field]}`);\n            break;\n          }\n        }\n        if (errors.length > 0) break;\n        for (const field of [\'kind\', \'fill\', \'stroke\']) {\n          if (shape[field] !== expected[field]) {\n            errors.push(`shape ${field} ${shape[field] || \'<empty>\'}, want ${expected[field] || \'<empty>\'}`);\n            break;\n          }\n        }\n        if (errors.length > 0) break;\n      }\n    }\n    if (testCase.require_cursor_surface) {\n      const cursor = row.cursor_surfaces.find((surface) => surface.clip_id === testCase.expected_cursor_clip_id);',
)
replace_once(
    capture,
    '  if (testCase.require_cursor_surface) {\n    const cursorRows = stable',
    '  if (testCase.require_shape_surface) {\n    const shapeRows = stable\n      .map((row) => row.shape_surfaces.find((surface) => surface.clip_id === testCase.expected_shape_clip_id))\n      .filter(Boolean);\n    if (shapeRows.length !== stable.length) errors.push(`shape surface observed ${shapeRows.length}/${stable.length} frames`);\n  }\n\n  if (testCase.require_cursor_surface) {\n    const cursorRows = stable',
)
replace_once(
    capture,
    '    expected_cursor_consumer: testCase.expected_cursor_consumer || \'\',\n    expected_cursor_clip_id: testCase.expected_cursor_clip_id || \'\',\n    decoder_budget:',
    '    expected_cursor_consumer: testCase.expected_cursor_consumer || \'\',\n    expected_cursor_clip_id: testCase.expected_cursor_clip_id || \'\',\n    expected_shape_consumer: testCase.expected_shape_consumer || \'\',\n    expected_shape_clip_id: testCase.expected_shape_clip_id || \'\',\n    decoder_budget:',
)
replace_once(
    capture,
    'function expectedCursorSampleAtFrame(timeline, clipId, frameIndex) {',
    '''function expectedShapeStateAtFrame(timeline, clipId, frameIndex) {\n  if (!Number.isFinite(frameIndex)) return null;\n  let clip = null;\n  for (const track of timeline.tracks || []) {\n    const found = (track.clips || []).find((candidate) => candidate.id === clipId);\n    if (found) { clip = found; break; }\n  }\n  const shape = clip?.shape;\n  if (!clip || !shape) return null;\n  const fps = timeline.canvas.fps;\n  const presentation = frameIndex * 1000;\n  if (!(clip.start_ms * fps <= presentation && presentation < (clip.start_ms + clip.duration_ms) * fps)) return null;\n  const kind = String(shape.kind || '').trim().toLowerCase();\n  const defaultFill = { highlight: '#facc15', spotlight: 'rgba(0,0,0,0.6)', step_marker: '#2563eb', speech_bubble: '#ffffff', label: '#1e293b' };\n  const defaultStroke = { checkmark: '#22c55e', x_mark: '#ef4444', speech_bubble: '', label: '' };\n  const defaultRadius = { rounded_rectangle: 12, speech_bubble: 18, label: 10 };\n  const strokeWidth = Number(shape.stroke_width);\n  const cornerRadius = Number(shape.corner_radius);\n  return {\n    kind,\n    width: shape.width ?? 320,\n    height: shape.height ?? 180,\n    fill: String(shape.fill || '').trim() || defaultFill[kind] || 'transparent',\n    stroke: String(shape.stroke || '').trim() || (Object.prototype.hasOwnProperty.call(defaultStroke, kind) ? defaultStroke[kind] : '#f59e0b'),\n    stroke_width: Number.isFinite(strokeWidth) && strokeWidth > 0 ? Math.max(1, strokeWidth) : 6,\n    corner_radius: Number.isFinite(cornerRadius) && cornerRadius > 0 ? cornerRadius : (defaultRadius[kind] || 0),\n  };\n}\n\nfunction expectedCursorSampleAtFrame(timeline, clipId, frameIndex) {''',
)

workflow = '.github/workflows/video-playback-canonical-parity.yml'
replace_once(
    workflow,
    "      - 'frontend/src/components/video/previewCursorPlayback.ts'\n      - 'frontend/src/components/video/previewCursorPlayback.test.ts'",
    "      - 'frontend/src/components/video/previewCursorPlayback.ts'\n      - 'frontend/src/components/video/previewCursorPlayback.test.ts'\n      - 'frontend/src/components/video/previewShapePlayback.ts'\n      - 'frontend/src/components/video/previewShapePlayback.test.ts'\n      - 'frontend/src/components/video/PreviewCanonicalPainters.tsx'\n      - 'frontend/src/components/video/PreviewCanonicalPainters.test.ts'\n      - 'frontend/test/renderContractFrameStateShape.test.ts'",
)
# The same path block appears once in pull_request and once in push; update the second occurrence separately.
p = Path(workflow)
text = p.read_text()
old = "      - 'frontend/src/components/video/previewCursorPlayback.ts'\n      - 'frontend/src/components/video/previewCursorPlayback.test.ts'"
new = "      - 'frontend/src/components/video/previewCursorPlayback.ts'\n      - 'frontend/src/components/video/previewCursorPlayback.test.ts'\n      - 'frontend/src/components/video/previewShapePlayback.ts'\n      - 'frontend/src/components/video/previewShapePlayback.test.ts'\n      - 'frontend/src/components/video/PreviewCanonicalPainters.tsx'\n      - 'frontend/src/components/video/PreviewCanonicalPainters.test.ts'\n      - 'frontend/test/renderContractFrameStateShape.test.ts'"
if text.count(old) != 1:
    raise SystemExit(f"{workflow}: second path block anchor count={text.count(old)}, want 1")
p.write_text(text.replace(old, new, 1))
replace_once(
    workflow,
    '              src/components/video/previewCursorPlayback.test.ts \\\n              src/components/video/previewPlaybackCursorCanonicalization.test.ts \\\n              src/components/video/previewTextPlaybackRuntime.test.ts',
    '              src/components/video/previewCursorPlayback.test.ts \\\n              src/components/video/previewShapePlayback.test.ts \\\n              src/components/video/PreviewCanonicalPainters.test.ts \\\n              test/renderContractFrameStateShape.test.ts \\\n              src/components/video/previewPlaybackCursorCanonicalization.test.ts \\\n              src/components/video/previewTextPlaybackRuntime.test.ts',
)
replace_once(workflow, '--fixture output/video-playback-canonical/fixture/parity-playback-canonical-v6.json', '--fixture output/video-playback-canonical/fixture/parity-playback-canonical-v7.json')
