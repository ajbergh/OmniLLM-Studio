from pathlib import Path

path = Path('scripts/video-playback-parity-capture.mjs')
text = path.read_text()
start_marker = "    if (testCase.expected_text_runtime || testCase.expected_text_consumer || testCase.expected_text_clip_id) {\n"
end_marker = "\n    const observations = await page.evaluate(async (observeMs) => {"
start = text.index(start_marker)
end = text.index(end_marker, start)
replacement = r'''    if (testCase.expected_text_runtime || testCase.expected_text_consumer || testCase.expected_text_clip_id) {
      const expectedText = {
        runtime: testCase.expected_text_runtime || '',
        consumer: testCase.expected_text_consumer || '',
        clipId: testCase.expected_text_clip_id || '',
      };
      try {
        await page.waitForFunction((expected) => {
          const stage = document.querySelector('[data-testid="video-preview-program"]');
          if (!stage) return false;
          const planKey = stage.dataset.previewTextPlaybackPlanKey || '';
          return (!expected.runtime || stage.dataset.previewTextPlaybackRuntime === expected.runtime)
            && (!expected.consumer || stage.dataset.previewTextPlaybackConsumer === expected.consumer)
            && (!expected.clipId || planKey.includes(`\"clip_id\":\"${expected.clipId}\"`));
        }, expectedText, { timeout: 4_000 });
      } catch (error) {
        const diagnostic = await page.evaluate(() => {
          const stage = document.querySelector('[data-testid="video-preview-program"]');
          const surfaces = stage
            ? [...stage.querySelectorAll('[data-preview-text-playback-surface]')].map((surface) => ({
                clip_id: surface.dataset.previewTextPlaybackSurface || '',
                runtime_key: surface.dataset.previewTextPlaybackRuntimeKey || '',
                ready: surface.dataset.previewTextPlaybackReady || '',
                pending_reason: surface.dataset.previewTextPlaybackPendingReason || '',
                visibility: getComputedStyle(surface).visibility,
                opacity: getComputedStyle(surface).opacity,
                text_nodes: [...surface.querySelectorAll('[data-preview-text-state-mode="canonical-frame"]')].map((node) => ({
                  font_face_source: node.dataset.previewTextFontFaceSource || '',
                  font_face_runtime: node.dataset.previewTextFontFaceRuntime || '',
                  metrics_mode: node.dataset.previewTextMetricsMode || '',
                  layout_contract: node.dataset.previewTextLayoutContract || '',
                  layout_input: node.dataset.previewTextLayoutInput || '',
                  layout_width: node.dataset.previewTextLayoutWidth || '',
                  layout_height: node.dataset.previewTextLayoutHeight || '',
                  layout_fragments: node.dataset.previewTextLayoutLineFragments || '',
                })),
              }))
            : [];
          return {
            stage: stage ? {
              timeline_ms: stage.dataset.parityTimeMs || '',
              parity_frame_index: stage.dataset.parityFrameIndex || '',
              visual_frame_mode: stage.dataset.previewVisualFrameMode || '',
              visual_frame_index: stage.dataset.previewVisualFrameIndex || '',
              playback_frame_candidate: stage.dataset.previewPlaybackFrameCandidate || '',
              playback_deferred_reason: stage.dataset.previewPlaybackCanonicalDeferred || '',
              text_frame_index: stage.dataset.previewTextPlaybackFrameIndex || '',
              text_plan_key: stage.dataset.previewTextPlaybackPlanKey || '',
              text_runtime: stage.dataset.previewTextPlaybackRuntime || '',
              text_consumer: stage.dataset.previewTextPlaybackConsumer || '',
              text_deferred: stage.dataset.previewTextPlaybackDeferred || '',
              text_trace: stage.dataset.previewTextPlaybackReadinessTrace || '',
              deterministic_font_readiness: stage.dataset.previewFontFaceReadiness || '',
              deterministic_font_error: stage.dataset.previewFontFaceRuntimeError || '',
              deterministic_layout_readiness: stage.dataset.previewTextLayoutReadiness || '',
              deterministic_layout_error: stage.dataset.previewTextLayoutRuntimeError || '',
            } : null,
            surfaces,
          };
        });
        throw new Error(
          `text playback readiness wait failed for ${testCase.name}: ${JSON.stringify(diagnostic)}`,
          { cause: error },
        );
      }
    }
'''
path.write_text(text[:start] + replacement + text[end:])
print('text readiness diagnostic patch applied')
