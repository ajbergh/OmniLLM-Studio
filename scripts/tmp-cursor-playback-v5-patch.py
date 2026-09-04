from pathlib import Path


def replace_once(text: str, old: str, new: str, label: str) -> str:
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{label} anchor count={count}, want 1")
    return text.replace(old, new)


# Retained Go fixture: v5 cursor expectations and composition cases.
path = Path("backend/internal/video/parity_fixture_playback.go")
text = path.read_text()
text = replace_once(
    text,
    'const PlaybackCanonicalParityFixtureName = "parity-playback-canonical-v4"',
    'const PlaybackCanonicalParityFixtureName = "parity-playback-canonical-v5"',
    "fixture version",
)
text = replace_once(
    text,
    '\tExpectedTextTrace        []string `json:"expected_text_trace,omitempty"`\n\tRequireWeightedCanvas    bool     `json:"require_weighted_canvas,omitempty"`',
    '\tExpectedTextTrace        []string `json:"expected_text_trace,omitempty"`\n\tExpectedCursorConsumer   string   `json:"expected_cursor_consumer,omitempty"`\n\tExpectedCursorClipID     string   `json:"expected_cursor_clip_id,omitempty"`\n\tRequireCursorSurface     bool     `json:"require_cursor_surface,omitempty"`\n\tRequireCursorMotion      bool     `json:"require_cursor_motion,omitempty"`\n\tRequireCursorHighlight   bool     `json:"require_cursor_highlight,omitempty"`\n\tRequireCursorClickToggle bool     `json:"require_cursor_click_toggle,omitempty"`\n\tRequireWeightedCanvas    bool     `json:"require_weighted_canvas,omitempty"`',
    "cursor case fields",
)
text = replace_once(
    text,
    "// Mixed v4 cases explicitly prove that supported media/text and weighted/text\n// surfaces share one canonical frame decision and that either runtime can revoke\n// authority for the complete visual frame without partial canonical promotion.",
    "// Mixed v5 cases explicitly prove that supported media/text/cursor and\n// weighted/text/cursor surfaces share one canonical frame decision. Exact cursor\n// samples must stay frame-addressed while any unsupported cursor parent or runtime\n// debt still revokes authority for the complete visual frame.",
    "fixture comment",
)
text = replace_once(
    text,
    '''\tweightedText := textClip(\n\t\t"playback-weighted-text",\n\t\t"Canonical weighted media plus text",\n\t\t"DejaVu Sans",\n\t\t"playback-font-v1",\n\t\t28200,\n\t)\n\n\tweightedInvalidOut''',
    '''\tweightedText := textClip(\n\t\t"playback-weighted-text",\n\t\t"Canonical weighted media plus text plus cursor",\n\t\t"DejaVu Sans",\n\t\t"playback-font-v1",\n\t\t28200,\n\t)\n\tweightedTextCursor := mediaClip("playback-weighted-text-cursor", "asset-landscape", 28200, 1200)\n\tweightedTextCursor.Transform["x"] = 180.0\n\tweightedTextCursor.Transform["y"] = -110.0\n\tweightedTextCursor.Transform["scale"] = 0.22\n\tweightedTextCursor.Cursor = &TimelineCursor{\n\t\tVisible: true,\n\t\tScale:   0.75,\n\t\tEvents: []TimelineCursorEvent{\n\t\t\t{TimeMS: 0, X: 80, Y: 80},\n\t\t\t{TimeMS: 600, X: 320, Y: 180},\n\t\t\t{TimeMS: 1199, X: 560, Y: 280},\n\t\t},\n\t}\n\n\tweightedInvalidOut''',
    "weighted cursor composition",
)
text = replace_once(
    text,
    '''\tweightedBudgetText := textClip(\n\t\t"playback-weighted-budget-text",\n\t\t"Ready text must fall back when weighted runtime defers",\n\t\t"DejaVu Sans",\n\t\t"playback-font-v1",\n\t\t34600,\n\t)\n\n\taudio :=''',
    '''\tweightedBudgetText := textClip(\n\t\t"playback-weighted-budget-text",\n\t\t"Ready text must fall back when weighted runtime defers",\n\t\t"DejaVu Sans",\n\t\t"playback-font-v1",\n\t\t34600,\n\t)\n\n\tunsupportedCursor := mediaClip("playback-cursor-unsupported-fade", "asset-landscape", 36200, 1200)\n\tunsupportedCursor.FadeInMS = 200\n\tunsupportedCursor.Cursor = &TimelineCursor{\n\t\tVisible: true,\n\t\tScale:   1,\n\t\tEvents: []TimelineCursorEvent{\n\t\t\t{TimeMS: 0, X: 120, Y: 100},\n\t\t\t{TimeMS: 1199, X: 520, Y: 260},\n\t\t},\n\t}\n\n\taudio :=''',
    "unsupported cursor fixture",
)
text = replace_once(
    text,
    '''\t\ttrack("track-weighted-text-out", weightedTextOut),\n\t\ttrack("track-weighted-text-in", weightedTextIn),\n\t\ttrack("track-weighted-text", weightedText),''',
    '''\t\ttrack("track-weighted-text-out", weightedTextOut),\n\t\ttrack("track-weighted-text-in", weightedTextIn),\n\t\ttrack("track-weighted-text-cursor", weightedTextCursor),\n\t\ttrack("track-weighted-text", weightedText),''',
    "weighted cursor track",
)
text = replace_once(
    text,
    '\t\ttrack("track-weighted-budget-text", weightedBudgetText),\n\t\taudio,',
    '\t\ttrack("track-weighted-budget-text", weightedBudgetText),\n\t\ttrack("track-cursor-unsupported-fade", unsupportedCursor),\n\t\taudio,',
    "unsupported cursor track",
)
text = replace_once(
    text,
    '\t\t{Name: "cursor-fallback", FrameIndex: 216, ObserveMS: 300, ExpectedMode: "legacy-time-fallback", ExpectedReason: "unsupported-playback-painter:playback-cursor", ExpectedTransitionMode: "legacy"},',
    '\t\t{Name: "cursor-canonical-playback", FrameIndex: 216, ObserveMS: 380, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-none", ExpectedCursorConsumer: "canonical-inline", ExpectedCursorClipID: "playback-cursor", RequireCursorSurface: true, RequireCursorMotion: true, RequireCursorHighlight: true, RequireCursorClickToggle: true, RequireAdvancingFrames: true},',
    "standalone cursor case",
)
text = replace_once(
    text,
    '\t\t{Name: "mixed-text-cursor-fallback", FrameIndex: 258, ObserveMS: 350, ExpectedMode: "legacy-time-fallback", ExpectedReason: "unsupported-playback-painter:playback-mixed-cursor", ExpectedTransitionMode: "legacy", ExpectedTextRuntime: "ready", ExpectedTextConsumer: "legacy-time-fallback", ExpectedTextClipID: "playback-mixed-text", ExpectedTextTrace: []string{"font-face-not-ready", "font-face-ready", "text-layout-not-ready", "ready"}, RequireTextLayout: true},',
    '\t\t{Name: "mixed-text-cursor-canonical", FrameIndex: 258, ObserveMS: 350, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-none", ExpectedTextRuntime: "ready", ExpectedTextConsumer: "canonical-text-dom", ExpectedTextClipID: "playback-mixed-text", ExpectedTextTrace: []string{"font-face-not-ready", "font-face-ready", "text-layout-not-ready", "ready"}, ExpectedCursorConsumer: "canonical-inline", ExpectedCursorClipID: "playback-mixed-cursor", RequireCursorSurface: true, RequireCursorMotion: true, RequireTextLayout: true, RequireAdvancingFrames: true},',
    "mixed cursor text case",
)
text = replace_once(
    text,
    '\t\t{Name: "weighted-text-canonical-playback", FrameIndex: 852, ObserveMS: 350, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-weighted-deferred", ExpectedWeightedRuntime: "ready", ExpectedWeightedConsumer: "canonical-weighted-canvas", ExpectedWeightedPairID: "weighted-text-crossfade", ExpectedTextRuntime: "ready", ExpectedTextConsumer: "canonical-text-dom", ExpectedTextClipID: "playback-weighted-text", ExpectedTextTrace: []string{"font-face-not-ready", "font-face-ready", "text-layout-not-ready", "ready"}, RequireWeightedCanvas: true, RequireTextLayout: true, RequireAdvancingFrames: true},',
    '\t\t{Name: "weighted-text-canonical-playback", FrameIndex: 852, ObserveMS: 350, ExpectedMode: "canonical-playback", ExpectedTransitionMode: "canonical-weighted-deferred", ExpectedWeightedRuntime: "ready", ExpectedWeightedConsumer: "canonical-weighted-canvas", ExpectedWeightedPairID: "weighted-text-crossfade", ExpectedTextRuntime: "ready", ExpectedTextConsumer: "canonical-text-dom", ExpectedTextClipID: "playback-weighted-text", ExpectedTextTrace: []string{"font-face-not-ready", "font-face-ready", "text-layout-not-ready", "ready"}, ExpectedCursorConsumer: "canonical-inline", ExpectedCursorClipID: "playback-weighted-text-cursor", RequireCursorSurface: true, RequireCursorMotion: true, RequireWeightedCanvas: true, RequireTextLayout: true, RequireAdvancingFrames: true},',
    "weighted cursor text case",
)
text = replace_once(
    text,
    '\t\t{Name: "weighted-text-decoder-budget-fallback", FrameIndex: 1044, ObserveMS: 350, ExpectedMode: "legacy-time-fallback", ExpectedReason: "transition-weighted-runtime-deferred:weighted-text-budget-crossfade-out:decoder-budget-poster", ExpectedTransitionMode: "legacy", ExpectedWeightedRuntime: "deferred", ExpectedWeightedConsumer: "legacy-time-fallback", ExpectedWeightedPairID: "weighted-text-budget-crossfade", ExpectedTextRuntime: "ready", ExpectedTextConsumer: "legacy-time-fallback", ExpectedTextClipID: "playback-weighted-budget-text", ExpectedTextTrace: []string{"font-face-not-ready", "font-face-ready", "text-layout-not-ready", "ready"}, RequireTextLayout: true, DecoderBudget: 1},\n\t}',
    '\t\t{Name: "weighted-text-decoder-budget-fallback", FrameIndex: 1044, ObserveMS: 350, ExpectedMode: "legacy-time-fallback", ExpectedReason: "transition-weighted-runtime-deferred:weighted-text-budget-crossfade-out:decoder-budget-poster", ExpectedTransitionMode: "legacy", ExpectedWeightedRuntime: "deferred", ExpectedWeightedConsumer: "legacy-time-fallback", ExpectedWeightedPairID: "weighted-text-budget-crossfade", ExpectedTextRuntime: "ready", ExpectedTextConsumer: "legacy-time-fallback", ExpectedTextClipID: "playback-weighted-budget-text", ExpectedTextTrace: []string{"font-face-not-ready", "font-face-ready", "text-layout-not-ready", "ready"}, RequireTextLayout: true, DecoderBudget: 1},\n\t\t{Name: "unsupported-cursor-fade-fallback", FrameIndex: 1092, ObserveMS: 300, ExpectedMode: "legacy-time-fallback", ExpectedReason: "cursor-playback-deferred:playback-cursor-unsupported-fade:fade-unsupported", ExpectedTransitionMode: "legacy", ExpectedCursorConsumer: "legacy-time-fallback", ExpectedCursorClipID: "playback-cursor-unsupported-fade", RequireCursorSurface: true},\n\t}',
    "unsupported cursor case",
)
path.write_text(text)


# Fixture contract test.
path = Path("backend/internal/video/parity_fixture_playback_test.go")
text = path.read_text()
text = replace_once(text, "if len(cases) != 17 {", "if len(cases) != 18 {", "fixture case count")
text = replace_once(
    text,
    '''\t\tif testCase.RequireTextLayout {\n\t\t\tif testCase.ExpectedTextRuntime != "ready" {''',
    '''\t\tif testCase.ExpectedCursorConsumer != "" && testCase.ExpectedCursorClipID == "" {\n\t\t\tt.Fatalf("cursor consumer case %q is missing an expected clip id", testCase.Name)\n\t\t}\n\t\tif testCase.RequireCursorSurface {\n\t\t\tif testCase.ExpectedCursorClipID == "" {\n\t\t\t\tt.Fatalf("cursor surface case %q is missing cursor identity", testCase.Name)\n\t\t\t}\n\t\t\tif testCase.ExpectedCursorConsumer != "canonical-inline" && testCase.ExpectedCursorConsumer != "legacy-time-fallback" {\n\t\t\t\tt.Fatalf("cursor surface case %q has invalid consumer %q", testCase.Name, testCase.ExpectedCursorConsumer)\n\t\t\t}\n\t\t\tif testCase.ExpectedMode == "canonical-playback" && testCase.ExpectedCursorConsumer != "canonical-inline" {\n\t\t\t\tt.Fatalf("canonical cursor case %q is missing canonical consumer expectation", testCase.Name)\n\t\t\t}\n\t\t\tif testCase.ExpectedMode == "legacy-time-fallback" && testCase.ExpectedCursorConsumer != "legacy-time-fallback" {\n\t\t\t\tt.Fatalf("fallback cursor case %q must retain legacy consumer", testCase.Name)\n\t\t\t}\n\t\t}\n\t\tif (testCase.RequireCursorMotion || testCase.RequireCursorHighlight || testCase.RequireCursorClickToggle) && !testCase.RequireCursorSurface {\n\t\t\tt.Fatalf("cursor behavior case %q must require a cursor surface", testCase.Name)\n\t\t}\n\t\tif testCase.RequireTextLayout {\n\t\t\tif testCase.ExpectedTextRuntime != "ready" {''',
    "cursor fixture validation",
)
text = replace_once(
    text,
    '\t\t"cursor-fallback",\n\t\t"mixed-text-cursor-fallback",',
    '\t\t"cursor-canonical-playback",\n\t\t"mixed-text-cursor-canonical",',
    "required cursor cases",
)
text = replace_once(
    text,
    '\t\t"weighted-text-decoder-budget-fallback",\n\t}',
    '\t\t"weighted-text-decoder-budget-fallback",\n\t\t"unsupported-cursor-fade-fallback",\n\t}',
    "required unsupported cursor case",
)
path.write_text(text)


# Product observability: publish exact sample values without changing paint.
path = Path("frontend/src/components/video/VideoPreviewCanvasLegacy.tsx")
text = path.read_text()
text = replace_once(
    text,
    '''              data-preview-cursor-playback-clip-id={clip.id}\n              style={{ left, top }}''',
    '''              data-preview-cursor-playback-clip-id={clip.id}\n              data-preview-cursor-x={sample.x}\n              data-preview-cursor-y={sample.y}\n              data-preview-cursor-click={sample.click ? 'true' : 'false'}\n              data-preview-cursor-scale={canonicalCursor?.scale ?? clip.cursor.scale ?? 1}\n              data-preview-cursor-highlight={highlight ? 'true' : 'false'}\n              data-preview-cursor-click-rings={clickRings ? 'true' : 'false'}\n              style={{ left, top }}''',
    "cursor sample observability",
)
path.write_text(text)


# Live browser evidence capture and exact cursor gate.
path = Path("scripts/video-playback-parity-capture.mjs")
text = path.read_text()
text = replace_once(
    text,
    "    const observations = await page.evaluate(async (observeMs) => {",
    '''    if (testCase.expected_cursor_consumer || testCase.expected_cursor_clip_id) {\n      await page.waitForFunction((expected) => {\n        const surfaces = [...document.querySelectorAll('[data-preview-cursor-playback-clip-id]')];\n        return surfaces.some((surface) => (!expected.clipId || surface.dataset.previewCursorPlaybackClipId === expected.clipId)\n          && (!expected.consumer || surface.dataset.previewCursorPlaybackConsumer === expected.consumer));\n      }, {\n        consumer: testCase.expected_cursor_consumer || '',\n        clipId: testCase.expected_cursor_clip_id || '',\n      }, { timeout: 3_000 });\n    }\n\n    const observations = await page.evaluate(async (observeMs) => {''',
    "cursor readiness wait",
)
text = replace_once(
    text,
    "        const textSurfaces = [...stage.querySelectorAll('[data-preview-text-playback-surface]')];\n        rows.push({",
    "        const textSurfaces = [...stage.querySelectorAll('[data-preview-text-playback-surface]')];\n        const cursorSurfaces = [...stage.querySelectorAll('[data-preview-cursor-playback-clip-id]')];\n        rows.push({",
    "cursor surface collection",
)
text = replace_once(
    text,
    '''          text_surface_font_ready_count: textSurfaces.filter((surface) => Boolean(\n            surface.querySelector('[data-preview-text-font-face-runtime="editor-resource-loaded"]'),\n          )).length,\n          audio_current_time:''',
    '''          text_surface_font_ready_count: textSurfaces.filter((surface) => Boolean(\n            surface.querySelector('[data-preview-text-font-face-runtime="editor-resource-loaded"]'),\n          )).length,\n          cursor_surfaces: cursorSurfaces.map((surface) => ({\n            clip_id: surface.dataset.previewCursorPlaybackClipId || '',\n            state_mode: surface.dataset.previewCursorStateMode || '',\n            consumer: surface.dataset.previewCursorPlaybackConsumer || '',\n            x: numberOrNull(surface.dataset.previewCursorX),\n            y: numberOrNull(surface.dataset.previewCursorY),\n            click: surface.dataset.previewCursorClick === 'true',\n            scale: numberOrNull(surface.dataset.previewCursorScale),\n            highlight: surface.dataset.previewCursorHighlight === 'true',\n            click_rings: surface.dataset.previewCursorClickRings === 'true',\n          })),\n          audio_current_time:''',
    "cursor observation rows",
)
text = replace_once(
    text,
    "const result = gateCase(testCase, observations, fixture.timeline.canvas.fps);",
    "const result = gateCase(testCase, observations, fixture.timeline);",
    "gate timeline input",
)
if text.count("schema_version: 2") != 2:
    raise SystemExit(f'schema version anchor count={text.count("schema_version: 2")}, want 2')
text = text.replace("schema_version: 2", "schema_version: 3")
text = replace_once(
    text,
    "function gateCase(testCase, observations, fps) {\n  const errors = [];",
    "function gateCase(testCase, observations, timeline) {\n  const errors = [];\n  const fps = timeline.canvas.fps;",
    "gate signature",
)
text = replace_once(
    text,
    "    if (errors.length > 0) break;\n    if (testCase.require_text_layout) {",
    '''    if (errors.length > 0) break;\n    if (testCase.require_cursor_surface) {\n      const cursor = row.cursor_surfaces.find((surface) => surface.clip_id === testCase.expected_cursor_clip_id);\n      if (!cursor) {\n        errors.push(`cursor surface ${testCase.expected_cursor_clip_id || '<unspecified>'} is missing`);\n        break;\n      }\n      if (cursor.consumer !== testCase.expected_cursor_consumer) {\n        errors.push(`cursor consumer ${cursor.consumer || '<empty>'}, want ${testCase.expected_cursor_consumer}`);\n        break;\n      }\n      const expectedStateMode = testCase.expected_mode === 'canonical-playback' ? 'canonical-frame' : 'legacy-time';\n      if (cursor.state_mode !== expectedStateMode) {\n        errors.push(`cursor state mode ${cursor.state_mode || '<empty>'}, want ${expectedStateMode}`);\n        break;\n      }\n      if (testCase.expected_mode === 'canonical-playback') {\n        const expected = expectedCursorSampleAtFrame(timeline, testCase.expected_cursor_clip_id, row.visual_frame_index);\n        if (!expected) {\n          errors.push(`canonical cursor expectation ${testCase.expected_cursor_clip_id} is unavailable`);\n          break;\n        }\n        if (!Number.isFinite(cursor.x) || Math.abs(cursor.x - expected.x) > 1e-6\n          || !Number.isFinite(cursor.y) || Math.abs(cursor.y - expected.y) > 1e-6) {\n          errors.push(`cursor sample (${cursor.x},${cursor.y}), want (${expected.x},${expected.y}) at frame ${row.visual_frame_index}`);\n          break;\n        }\n        if (cursor.click !== expected.click\n          || cursor.highlight !== expected.highlight\n          || cursor.click_rings !== expected.click_rings\n          || !Number.isFinite(cursor.scale)\n          || Math.abs(cursor.scale - expected.scale) > 1e-9) {\n          errors.push(`cursor state ${JSON.stringify(cursor)}, want ${JSON.stringify(expected)}`);\n          break;\n        }\n      }\n    }\n    if (testCase.require_text_layout) {''',
    "cursor per-frame gate",
)
text = replace_once(
    text,
    "  const timelineTimes = stable.map((row) => row.timeline_ms);",
    '''  if (testCase.require_cursor_surface) {\n    const cursorRows = stable\n      .map((row) => row.cursor_surfaces.find((surface) => surface.clip_id === testCase.expected_cursor_clip_id))\n      .filter(Boolean);\n    if (cursorRows.length !== stable.length) errors.push(`cursor surface observed ${cursorRows.length}/${stable.length} frames`);\n    if (testCase.require_cursor_motion && cursorRows.length > 1) {\n      const motion = Math.max(range(cursorRows.map((row) => row.x).filter(Number.isFinite)), range(cursorRows.map((row) => row.y).filter(Number.isFinite)));\n      if (motion < 1) errors.push(`cursor motion advanced only ${motion}px in canonical sample space`);\n    }\n    if (testCase.require_cursor_highlight && !cursorRows.every((row) => row.highlight === true)) {\n      errors.push('cursor highlight was not retained for every observation');\n    }\n    if (testCase.require_cursor_click_toggle) {\n      const clickStates = new Set(cursorRows.map((row) => row.click));\n      if (!clickStates.has(false) || !clickStates.has(true)) errors.push('cursor click-ring window did not cross both false and true states');\n    }\n  }\n\n  const timelineTimes = stable.map((row) => row.timeline_ms);''',
    "cursor aggregate gate",
)
text = replace_once(
    text,
    "    expected_text_trace: testCase.expected_text_trace || [],\n    decoder_budget:",
    "    expected_text_trace: testCase.expected_text_trace || [],\n    expected_cursor_consumer: testCase.expected_cursor_consumer || '',\n    expected_cursor_clip_id: testCase.expected_cursor_clip_id || '',\n    decoder_budget:",
    "cursor result metadata",
)
text = replace_once(
    text,
    "function range(values) {",
    '''function expectedCursorSampleAtFrame(timeline, clipId, frameIndex) {\n  if (!Number.isFinite(frameIndex)) return null;\n  let clip = null;\n  for (const track of timeline.tracks || []) {\n    const found = (track.clips || []).find((candidate) => candidate.id === clipId);\n    if (found) { clip = found; break; }\n  }\n  const cursor = clip?.cursor;\n  const events = cursor?.events || [];\n  if (!clip || cursor?.visible === false || events.length === 0) return null;\n  const fps = timeline.canvas.fps;\n  const numerator = frameIndex * 1000 - clip.start_ms * fps;\n  const denominator = fps;\n  let previous = events[0];\n  let x = previous.x;\n  let y = previous.y;\n  if (numerator > previous.time_ms * denominator) {\n    for (let index = 1; index < events.length; index += 1) {\n      const next = events[index];\n      if (numerator <= next.time_ms * denominator) {\n        const span = Math.max(1, next.time_ms - previous.time_ms);\n        const progress = (numerator - previous.time_ms * denominator) / (span * denominator);\n        x = previous.x + (next.x - previous.x) * progress;\n        y = previous.y + (next.y - previous.y) * progress;\n        previous = null;\n        break;\n      }\n      previous = next;\n    }\n    if (previous) { x = previous.x; y = previous.y; }\n  }\n  const clickWindow = 300 * denominator;\n  const click = events.some((event) => event.click === true\n    && Math.abs(event.time_ms * denominator - numerator) < clickWindow);\n  return {\n    x,\n    y,\n    click,\n    scale: cursor.scale ?? 1,\n    highlight: cursor.highlight === true,\n    click_rings: cursor.click_rings === true,\n  };\n}\n\nfunction range(values) {''',
    "cursor expected sample helper",
)
path.write_text(text)
