from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    file = Path(path)
    text = file.read_text()
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected one replacement, found {count}: {old[:120]!r}")
    file.write_text(text.replace(old, new, 1))


def replace_all(path: str, old: str, new: str, expected: int) -> None:
    file = Path(path)
    text = file.read_text()
    count = text.count(old)
    if count != expected:
        raise SystemExit(f"{path}: expected {expected} replacements, found {count}: {old[:120]!r}")
    file.write_text(text.replace(old, new))


consumer = "frontend/src/components/video/PreviewTextPlaybackConsumer.tsx"
replace_once(
    consumer,
    "import { useEffect, useLayoutEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react';",
    "import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react';",
)
replace_once(
    consumer,
    "  const preparationRef = useRef<PreparationState>(IDLE_PREPARATION);\n",
    "  const preparationRef = useRef<PreparationState>(IDLE_PREPARATION);\n  const layoutSettlementIdentityRef = useRef('');\n",
)
replace_once(
    consumer,
    "  const updatePreparation = (next: PreparationState) => {\n    preparationRef.current = next;\n    setPreparation(next);\n  };",
    "  const updatePreparation = useCallback((next: PreparationState) => {\n    preparationRef.current = next;\n    setPreparation(next);\n  }, []);",
)
replace_once(
    consumer,
    "      clearPreviewTextPlaybackRuntime();\n      if (preparationRef.current.status !== 'idle') updatePreparation(IDLE_PREPARATION);\n      return;",
    "      clearPreviewTextPlaybackRuntime();\n      layoutSettlementIdentityRef.current = '';\n      if (preparationRef.current.status !== 'idle') updatePreparation(IDLE_PREPARATION);\n      return;",
)
replace_once(
    consumer,
    "    const current = preparationRef.current;\n    if (current.planIdentity === planIdentity && current.status !== 'idle') return;\n\n    if (structuralDeferred) {",
    "    const current = preparationRef.current;\n    if (current.planIdentity === planIdentity && current.status !== 'idle') return;\n    layoutSettlementIdentityRef.current = '';\n\n    if (structuralDeferred) {",
)
replace_once(consumer, "    let cancelled = false;\n    publishPreviewTextPlaybackRuntime({", "    publishPreviewTextPlaybackRuntime({")
replace_all(
    consumer,
    "if (cancelled || preparationRef.current.planIdentity !== planIdentity) return;",
    "if (preparationRef.current.planIdentity !== planIdentity) return;",
    3,
)
replace_once(
    consumer,
    "    return () => {\n      cancelled = true;\n    };\n  }, [bindingPlan.bindings, bindingPlan.errors, executionKey, isPlaying, planIdentity, playbackFrame, structuralDeferred, textLayers]);",
    "  }, [bindingPlan.bindings, bindingPlan.errors, executionKey, isPlaying, planIdentity, playbackFrame, structuralDeferred, textLayers, updatePreparation]);",
)
replace_once(
    consumer,
    "      || playbackFrame === null\n      || !executionKey) return;\n    let cancelled = false;\n    const deadline = performance.now() + 2000;",
    "      || playbackFrame === null\n      || !executionKey) return;\n    if (layoutSettlementIdentityRef.current === planIdentity) return;\n    layoutSettlementIdentityRef.current = planIdentity;\n    const deadline = performance.now() + 2000;",
)
replace_once(
    consumer,
    "    return () => {\n      cancelled = true;\n    };\n  }, [executionKey, planIdentity, playbackFrame, preparation, stage, stageScale, textLayers.length]);",
    "  }, [executionKey, planIdentity, playbackFrame, preparation, stage, stageScale, textLayers.length, updatePreparation]);",
)

preview = "frontend/src/components/video/VideoPreviewCanvas.tsx"
replace_once(
    preview,
    "import { PreviewWeightedPlaybackConsumer } from './PreviewWeightedPlaybackConsumer';\n",
    "import { PreviewWeightedPlaybackConsumer } from './PreviewWeightedPlaybackConsumer';\nimport { PreviewTextPlaybackConsumer } from './PreviewTextPlaybackConsumer';\n",
)
replace_once(
    preview,
    "import {\n  previewWeightedPlaybackRuntimeRevision,\n  subscribePreviewWeightedPlaybackRuntime,\n} from './previewWeightedPlaybackRuntime';\n",
    "import {\n  previewWeightedPlaybackRuntimeRevision,\n  subscribePreviewWeightedPlaybackRuntime,\n} from './previewWeightedPlaybackRuntime';\nimport {\n  previewTextPlaybackRuntimeRevision,\n  subscribePreviewTextPlaybackRuntime,\n} from './previewTextPlaybackRuntime';\n",
)
replace_once(
    preview,
    "  useSyncExternalStore(\n    subscribePreviewWeightedPlaybackRuntime,\n    previewWeightedPlaybackRuntimeRevision,\n    previewWeightedPlaybackRuntimeRevision,\n  );\n",
    "  useSyncExternalStore(\n    subscribePreviewWeightedPlaybackRuntime,\n    previewWeightedPlaybackRuntimeRevision,\n    previewWeightedPlaybackRuntimeRevision,\n  );\n  useSyncExternalStore(\n    subscribePreviewTextPlaybackRuntime,\n    previewTextPlaybackRuntimeRevision,\n    previewTextPlaybackRuntimeRevision,\n  );\n",
)
replace_once(
    preview,
    "      <PreviewWeightedPlaybackConsumer />\n      <PreviewPixelateBackdropConsumer />",
    "      <PreviewTextPlaybackConsumer />\n      <PreviewWeightedPlaybackConsumer />\n      <PreviewPixelateBackdropConsumer />",
)
replace_once(
    preview,
    " * bridge prepares the same Canvas kernel hidden and promotes only exact ready\n * weighted frames. Scene effects and text/shape/cursor painter inputs consume\n * the same already-evaluated FrameState.",
    " * bridge prepares weighted Canvas and resource-backed text surfaces hidden,\n * then promotes only exact ready canonical frames. Scene effects and deterministic\n * text/shape/cursor painter inputs consume the same already-evaluated FrameState.",
)

capture = "scripts/video-playback-parity-capture.mjs"
replace_once(
    capture,
    "    if (testCase.expected_weighted_runtime || testCase.expected_weighted_consumer || testCase.expected_weighted_pair_id) {\n      await page.waitForFunction((expected) => {\n        const stage = document.querySelector('[data-testid=\"video-preview-program\"]');\n        if (!stage) return false;\n        return (!expected.runtime || stage.dataset.previewWeightedPlaybackRuntime === expected.runtime)\n          && (!expected.consumer || stage.dataset.previewWeightedPlaybackConsumer === expected.consumer)\n          && (!expected.pairId || (stage.dataset.previewWeightedPlaybackPlanKey || '').includes(`${expected.pairId}:`));\n      }, {\n        runtime: testCase.expected_weighted_runtime || '',\n        consumer: testCase.expected_weighted_consumer || '',\n        pairId: testCase.expected_weighted_pair_id || '',\n      }, { timeout: 3_000 });\n    }\n\n    const observations = await page.evaluate(async (observeMs) => {",
    "    if (testCase.expected_weighted_runtime || testCase.expected_weighted_consumer || testCase.expected_weighted_pair_id) {\n      await page.waitForFunction((expected) => {\n        const stage = document.querySelector('[data-testid=\"video-preview-program\"]');\n        if (!stage) return false;\n        return (!expected.runtime || stage.dataset.previewWeightedPlaybackRuntime === expected.runtime)\n          && (!expected.consumer || stage.dataset.previewWeightedPlaybackConsumer === expected.consumer)\n          && (!expected.pairId || (stage.dataset.previewWeightedPlaybackPlanKey || '').includes(`${expected.pairId}:`));\n      }, {\n        runtime: testCase.expected_weighted_runtime || '',\n        consumer: testCase.expected_weighted_consumer || '',\n        pairId: testCase.expected_weighted_pair_id || '',\n      }, { timeout: 3_000 });\n    }\n    if (testCase.expected_text_runtime || testCase.expected_text_consumer || testCase.expected_text_clip_id) {\n      await page.waitForFunction((expected) => {\n        const stage = document.querySelector('[data-testid=\"video-preview-program\"]');\n        if (!stage) return false;\n        const planKey = stage.dataset.previewTextPlaybackPlanKey || '';\n        return (!expected.runtime || stage.dataset.previewTextPlaybackRuntime === expected.runtime)\n          && (!expected.consumer || stage.dataset.previewTextPlaybackConsumer === expected.consumer)\n          && (!expected.clipId || planKey.includes(`\\\"clip_id\\\":\\\"${expected.clipId}\\\"`));\n      }, {\n        runtime: testCase.expected_text_runtime || '',\n        consumer: testCase.expected_text_consumer || '',\n        clipId: testCase.expected_text_clip_id || '',\n      }, { timeout: 4_000 });\n    }\n\n    const observations = await page.evaluate(async (observeMs) => {",
)
replace_once(
    capture,
    "        const weightedSurfaces = [...stage.querySelectorAll('[data-preview-transition-pair-surface-role=\"playback\"]')];\n        rows.push({",
    "        const weightedSurfaces = [...stage.querySelectorAll('[data-preview-transition-pair-surface-role=\"playback\"]')];\n        const textSurfaces = [...stage.querySelectorAll('[data-preview-text-playback-surface]')];\n        rows.push({",
)
replace_once(
    capture,
    "          weighted_surface_runtime_keys: weightedSurfaces\n            .map((surface) => surface.dataset.previewTransitionPairRuntimeKey || '')\n            .filter(Boolean),\n          audio_current_time: audio ? audio.currentTime : null,",
    "          weighted_surface_runtime_keys: weightedSurfaces\n            .map((surface) => surface.dataset.previewTransitionPairRuntimeKey || '')\n            .filter(Boolean),\n          text_playback_runtime: stage.dataset.previewTextPlaybackRuntime || '',\n          text_playback_consumer: stage.dataset.previewTextPlaybackConsumer || '',\n          text_playback_plan_key: stage.dataset.previewTextPlaybackPlanKey || '',\n          text_playback_deferred: stage.dataset.previewTextPlaybackDeferred || '',\n          text_playback_readiness_trace: stage.dataset.previewTextPlaybackReadinessTrace || '',\n          text_surface_count: textSurfaces.length,\n          text_surface_ready_count: textSurfaces.filter((surface) => surface.dataset.previewTextPlaybackReady === 'true').length,\n          text_surface_visible_count: textSurfaces.filter((surface) => {\n            const style = getComputedStyle(surface);\n            return style.visibility !== 'hidden' && Number.parseFloat(style.opacity || '1') > 0;\n          }).length,\n          text_surface_layout_count: textSurfaces.filter((surface) => Boolean(\n            surface.querySelector('[data-preview-text-layout-contract=\"preview-text-layout-snapshot-v1\"]'),\n          )).length,\n          text_surface_font_ready_count: textSurfaces.filter((surface) => Boolean(\n            surface.querySelector('[data-preview-text-font-face-runtime=\"editor-resource-loaded\"]'),\n          )).length,\n          audio_current_time: audio ? audio.currentTime : null,",
)
replace_once(
    capture,
    "    if (testCase.expected_weighted_pair_id\n      && !row.weighted_playback_plan_key.includes(`${testCase.expected_weighted_pair_id}:`)) {\n      errors.push(`weighted pair ${row.weighted_playback_plan_key || '<empty>'}, want ${testCase.expected_weighted_pair_id}`);\n      break;\n    }\n    if (testCase.require_weighted_canvas) {",
    "    if (testCase.expected_weighted_pair_id\n      && !row.weighted_playback_plan_key.includes(`${testCase.expected_weighted_pair_id}:`)) {\n      errors.push(`weighted pair ${row.weighted_playback_plan_key || '<empty>'}, want ${testCase.expected_weighted_pair_id}`);\n      break;\n    }\n    if (testCase.expected_text_runtime && row.text_playback_runtime !== testCase.expected_text_runtime) {\n      errors.push(`text runtime ${row.text_playback_runtime}, want ${testCase.expected_text_runtime}`);\n      break;\n    }\n    if (testCase.expected_text_consumer && row.text_playback_consumer !== testCase.expected_text_consumer) {\n      errors.push(`text consumer ${row.text_playback_consumer}, want ${testCase.expected_text_consumer}`);\n      break;\n    }\n    if (testCase.expected_text_clip_id\n      && !row.text_playback_plan_key.includes(`\\\"clip_id\\\":\\\"${testCase.expected_text_clip_id}\\\"`)) {\n      errors.push(`text plan ${row.text_playback_plan_key || '<empty>'}, want ${testCase.expected_text_clip_id}`);\n      break;\n    }\n    for (const trace of testCase.expected_text_trace || []) {\n      if (!row.text_playback_readiness_trace.includes(trace)) {\n        errors.push(`text readiness trace ${row.text_playback_readiness_trace || '<empty>'} is missing ${trace}`);\n        break;\n      }\n    }\n    if (errors.length > 0) break;\n    if (testCase.require_text_layout) {\n      if (row.text_surface_count < 1) {\n        errors.push('text playback has no canonical prewarm surface');\n        break;\n      }\n      if (row.text_surface_ready_count !== row.text_surface_count) {\n        errors.push(`text surfaces ready ${row.text_surface_ready_count}/${row.text_surface_count}`);\n        break;\n      }\n      if (row.text_surface_layout_count !== row.text_surface_count) {\n        errors.push(`text layouts ready ${row.text_surface_layout_count}/${row.text_surface_count}`);\n        break;\n      }\n      if (row.text_surface_font_ready_count !== row.text_surface_count) {\n        errors.push(`text resource fonts ready ${row.text_surface_font_ready_count}/${row.text_surface_count}`);\n        break;\n      }\n      if (testCase.expected_text_consumer === 'canonical-text-dom'\n        && row.text_surface_visible_count !== row.text_surface_count) {\n        errors.push(`canonical text surfaces visible ${row.text_surface_visible_count}/${row.text_surface_count}`);\n        break;\n      }\n      if (testCase.expected_text_consumer === 'legacy-time-fallback' && row.text_surface_visible_count !== 0) {\n        errors.push(`fallback frame exposed ${row.text_surface_visible_count} canonical text surfaces`);\n        break;\n      }\n    }\n    if (testCase.require_weighted_canvas) {",
)
replace_once(
    capture,
    "    expected_weighted_pair_id: testCase.expected_weighted_pair_id || '',\n    decoder_budget: testCase.decoder_budget || 0,",
    "    expected_weighted_pair_id: testCase.expected_weighted_pair_id || '',\n    expected_text_runtime: testCase.expected_text_runtime || '',\n    expected_text_consumer: testCase.expected_text_consumer || '',\n    expected_text_clip_id: testCase.expected_text_clip_id || '',\n    expected_text_trace: testCase.expected_text_trace || [],\n    decoder_budget: testCase.decoder_budget || 0,",
)
replace_once(
    capture,
    "  const fileForAsset = {\n    'asset-landscape': ['asset-landscape.mp4', 'video/mp4'],\n    'asset-square': ['asset-square.png', 'image/png'],\n    'asset-audio': ['asset-audio.wav', 'audio/wav'],\n  };",
    "  const fileForAsset = {\n    'asset-landscape': ['asset-landscape.mp4', 'video/mp4'],\n    'asset-square': ['asset-square.png', 'image/png'],\n    'asset-audio': ['asset-audio.wav', 'audio/wav'],\n    'asset-font': ['asset-font.ttf', 'font/ttf', 'playback-font-v1'],\n    'asset-font-invalid': ['asset-font-invalid.ttf', 'font/ttf', 'playback-font-invalid-v1'],\n  };",
)
replace_once(
    capture,
    "    const response = await context.request.post(apiURL(`/video/projects/${encodeURIComponent(project.id)}/assets/upload`), {\n      headers,\n      multipart: { file: { name: recipe[0], mimeType: recipe[1], buffer: await fs.readFile(filePath) } },\n    });",
    "    const multipart = { file: { name: recipe[0], mimeType: recipe[1], buffer: await fs.readFile(filePath) } };\n    if (recipe[2]) multipart.font_resource_id = recipe[2];\n    const response = await context.request.post(apiURL(`/video/projects/${encodeURIComponent(project.id)}/assets/upload`), {\n      headers,\n      multipart,\n    });",
)

workflow = ".github/workflows/video-playback-canonical-parity.yml"
for block in ["pull_request", "push"]:
    pass
replace_all(
    workflow,
    "      - 'frontend/src/components/video/VideoPreviewCanvasLegacy.tsx'\n",
    "      - 'frontend/src/components/video/VideoPreviewCanvas.tsx'\n      - 'frontend/src/components/video/VideoPreviewCanvasLegacy.tsx'\n      - 'frontend/src/components/video/PreviewTextPlaybackConsumer.tsx'\n      - 'frontend/src/components/video/previewTextPlaybackRuntime.ts'\n      - 'frontend/src/components/video/previewTextPlaybackRuntime.test.ts'\n      - 'frontend/src/components/video/previewTextLayoutSnapshot.ts'\n      - 'frontend/src/components/video/previewFontFaceReadiness.ts'\n",
    2,
)
replace_once(
    workflow,
    "          bash scripts/ci-apt-install.sh ffmpeg\n",
    "          bash scripts/ci-apt-install.sh ffmpeg fonts-dejavu-core\n",
)
replace_once(
    workflow,
    "              src/components/video/previewPlaybackCanonicalization.test.ts \\\n              src/components/video/previewWeightedPlaybackRuntime.test.ts \\",
    "              src/components/video/previewPlaybackCanonicalization.test.ts \\\n              src/components/video/previewTextPlaybackRuntime.test.ts \\\n              src/components/video/previewWeightedPlaybackRuntime.test.ts \\",
)
replace_once(
    workflow,
    "            go run ./cmd/video-playback-parity-fixture \\\n              --output-dir ../output/video-playback-canonical/fixture\n          )",
    "            go run ./cmd/video-playback-parity-fixture \\\n              --output-dir ../output/video-playback-canonical/fixture\n          )\n          cp /usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf \\\n            output/video-playback-canonical/fixture/media/asset-font.ttf\n          dd if=/dev/zero \\\n            of=output/video-playback-canonical/fixture/media/asset-font-invalid.ttf \\\n            bs=1024 count=1 status=none",
)
replace_once(
    workflow,
    "--fixture output/video-playback-canonical/fixture/parity-playback-canonical-v2.json",
    "--fixture output/video-playback-canonical/fixture/parity-playback-canonical-v3.json",
)

print('text playback integration patch applied')
