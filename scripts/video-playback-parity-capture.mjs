#!/usr/bin/env node
import fs from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';
import { chromium } from 'playwright';

const args = Object.fromEntries(process.argv.slice(2).reduce((pairs, value, index, values) => {
  if (!value.startsWith('--')) return pairs;
  pairs.push([value.slice(2), values[index + 1]]);
  return pairs;
}, []));

if (!args.url || !args.fixture || !args['media-dir']) {
  console.error('usage: node scripts/video-playback-parity-capture.mjs --url <app-url> --fixture <generated-json> --media-dir <generated-media-dir> [--output <dir>] [--storage-state <json>]');
  process.exit(1);
}

const output = path.resolve(args.output || 'output/video-playback-canonical');
await fs.mkdir(output, { recursive: true });
const fixture = JSON.parse(await fs.readFile(path.resolve(args.fixture), 'utf8'));
if (!Array.isArray(fixture.cases) || fixture.cases.length === 0) throw new Error('playback fixture must contain cases');

const browser = await chromium.launch({ headless: true });
try {
  const context = await browser.newContext({
    viewport: { width: 1600, height: 1100 },
    deviceScaleFactor: 1,
    ...(args['storage-state'] ? { storageState: path.resolve(args['storage-state']) } : {}),
  });
  const page = await context.newPage();
  await page.goto(args.url, { waitUntil: 'networkidle' });
  const authToken = await page.evaluate(() => window.localStorage.getItem('omnillm_auth_token'));
  const requestHeaders = authToken ? { Authorization: `Bearer ${authToken}` } : {};
  const seedResult = await seedFixtureProject({
    context,
    fixture,
    mediaDir: path.resolve(args['media-dir']),
    baseURL: args.url,
    headers: requestHeaders,
  });
  await fs.writeFile(path.join(output, 'seed-result.json'), `${JSON.stringify(seedResult, null, 2)}\n`);

  const editorURL = new URL(`/video/${encodeURIComponent(seedResult.project_id)}/edit`, args.url).toString();
  const program = page.getByTestId('video-preview-program');
  const results = [];
  for (const testCase of fixture.cases) {
    // Each retained observation starts from a fresh editor instance. Normal
    // playback intentionally mutates transient playhead/media state; reloading
    // prevents one case's decoder/store lifecycle from becoming the next
    // case's readiness authority while keeping the same immutable saved timeline.
    await page.evaluate((decoderBudget) => {
      if (Number.isInteger(decoderBudget) && decoderBudget > 0) {
        window.localStorage.setItem('omnillm-video-decoder-budget', String(decoderBudget));
      } else {
        window.localStorage.removeItem('omnillm-video-decoder-budget');
      }
    }, Number(testCase.decoder_budget || 0));
    await page.goto(editorURL, { waitUntil: 'networkidle' });
    await program.waitFor({ state: 'visible' });
    await seekParityFrame(page, testCase.frame_index);
    await page.getByRole('button', { name: 'Play preview' }).click();
    await page.getByRole('button', { name: 'Pause preview' }).waitFor({ state: 'visible', timeout: 5_000 });

    if (testCase.expected_weighted_runtime || testCase.expected_weighted_consumer || testCase.expected_weighted_pair_id) {
      await page.waitForFunction((expected) => {
        const stage = document.querySelector('[data-testid="video-preview-program"]');
        if (!stage) return false;
        return (!expected.runtime || stage.dataset.previewWeightedPlaybackRuntime === expected.runtime)
          && (!expected.consumer || stage.dataset.previewWeightedPlaybackConsumer === expected.consumer)
          && (!expected.pairId || (stage.dataset.previewWeightedPlaybackPlanKey || '').includes(`${expected.pairId}:`));
      }, {
        runtime: testCase.expected_weighted_runtime || '',
        consumer: testCase.expected_weighted_consumer || '',
        pairId: testCase.expected_weighted_pair_id || '',
      }, { timeout: 3_000 });
    }
    if (testCase.expected_text_runtime || testCase.expected_text_consumer || testCase.expected_text_clip_id) {
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

    if (testCase.expected_cursor_consumer || testCase.expected_cursor_clip_id) {
      await page.waitForFunction((expected) => {
        const surfaces = [...document.querySelectorAll('[data-preview-cursor-playback-clip-id]')];
        return surfaces.some((surface) => (!expected.clipId || surface.dataset.previewCursorPlaybackClipId === expected.clipId)
          && (!expected.consumer || surface.dataset.previewCursorPlaybackConsumer === expected.consumer));
      }, {
        consumer: testCase.expected_cursor_consumer || '',
        clipId: testCase.expected_cursor_clip_id || '',
      }, { timeout: 3_000 });
    }

    const observations = await page.evaluate(async (observeMs) => {
      const rows = [];
      const deadline = performance.now() + observeMs;
      while (performance.now() < deadline) {
        await new Promise((resolve) => requestAnimationFrame(resolve));
        const stage = document.querySelector('[data-testid="video-preview-program"]');
        if (!stage) throw new Error('video preview program disappeared during playback evidence');
        const audio = document.querySelector('audio');
        const videos = [...stage.querySelectorAll('video')];
        const weightedSurfaces = [...stage.querySelectorAll('[data-preview-transition-pair-surface-role="playback"]')];
        const textSurfaces = [...stage.querySelectorAll('[data-preview-text-playback-surface]')];
        const cursorSurfaces = [...stage.querySelectorAll('[data-preview-cursor-playback-clip-id]')];
        rows.push({
          performance_ms: performance.now(),
          timeline_ms: Number(stage.dataset.parityTimeMs),
          parity_frame_index: numberOrNull(stage.dataset.parityFrameIndex),
          visual_frame_mode: stage.dataset.previewVisualFrameMode || '',
          visual_frame_index: numberOrNull(stage.dataset.previewVisualFrameIndex),
          playback_frame_candidate: numberOrNull(stage.dataset.previewPlaybackFrameCandidate),
          playback_deferred_reason: stage.dataset.previewPlaybackCanonicalDeferred || '',
          transition_plan_mode: stage.dataset.previewTransitionPairPlanMode || '',
          transition_deferred: stage.dataset.previewTransitionPairDeferred || '',
          weighted_playback_runtime: stage.dataset.previewWeightedPlaybackRuntime || '',
          weighted_playback_consumer: stage.dataset.previewWeightedPlaybackConsumer || '',
          weighted_playback_plan_key: stage.dataset.previewWeightedPlaybackPlanKey || '',
          weighted_playback_deferred: stage.dataset.previewWeightedPlaybackDeferred || '',
          weighted_surface_count: weightedSurfaces.length,
          weighted_surface_ready_count: weightedSurfaces.filter((surface) => surface.dataset.previewTransitionPairReady === 'true').length,
          weighted_surface_errors: weightedSurfaces
            .map((surface) => surface.dataset.previewTransitionPairError || '')
            .filter(Boolean),
          weighted_surface_pending_reasons: weightedSurfaces
            .map((surface) => surface.dataset.previewTransitionPairPendingReason || '')
            .filter(Boolean),
          weighted_surface_runtime_keys: weightedSurfaces
            .map((surface) => surface.dataset.previewTransitionPairRuntimeKey || '')
            .filter(Boolean),
          text_playback_runtime: stage.dataset.previewTextPlaybackRuntime || '',
          text_playback_consumer: stage.dataset.previewTextPlaybackConsumer || '',
          text_playback_plan_key: stage.dataset.previewTextPlaybackPlanKey || '',
          text_playback_deferred: stage.dataset.previewTextPlaybackDeferred || '',
          text_playback_readiness_trace: stage.dataset.previewTextPlaybackReadinessTrace || '',
          text_surface_count: textSurfaces.length,
          text_surface_ready_count: textSurfaces.filter((surface) => surface.dataset.previewTextPlaybackReady === 'true').length,
          text_surface_visible_count: textSurfaces.filter((surface) => {
            const style = getComputedStyle(surface);
            return style.visibility !== 'hidden' && Number.parseFloat(style.opacity || '1') > 0;
          }).length,
          text_surface_layout_count: textSurfaces.filter((surface) => Boolean(
            surface.querySelector('[data-preview-text-layout-contract="preview-text-layout-snapshot-v1"]'),
          )).length,
          text_surface_font_ready_count: textSurfaces.filter((surface) => Boolean(
            surface.querySelector('[data-preview-text-font-face-runtime="editor-resource-loaded"]'),
          )).length,
          cursor_surfaces: cursorSurfaces.map((surface) => ({
            clip_id: surface.dataset.previewCursorPlaybackClipId || '',
            state_mode: surface.dataset.previewCursorStateMode || '',
            consumer: surface.dataset.previewCursorPlaybackConsumer || '',
            x: numberOrNull(surface.dataset.previewCursorX),
            y: numberOrNull(surface.dataset.previewCursorY),
            click: surface.dataset.previewCursorClick === 'true',
            scale: numberOrNull(surface.dataset.previewCursorScale),
            highlight: surface.dataset.previewCursorHighlight === 'true',
            click_rings: surface.dataset.previewCursorClickRings === 'true',
          })),
          audio_current_time: audio ? audio.currentTime : null,
          audio_paused: audio ? audio.paused : null,
          video_current_times: videos.map((video) => ({
            label: video.getAttribute('aria-label') || '',
            current_time: video.currentTime,
            paused: video.paused,
          })),
        });
      }
      return rows;

      function numberOrNull(value) {
        if (value === undefined || value === '') return null;
        const number = Number(value);
        return Number.isFinite(number) ? number : null;
      }
    }, testCase.observe_ms);

    await page.getByRole('button', { name: 'Pause preview' }).click();
    const result = gateCase(testCase, observations, fixture.timeline);
    results.push(result);
    await fs.writeFile(
      path.join(output, 'playback-evidence-progress.json'),
      `${JSON.stringify({ schema_version: 3, fixture: fixture.name, cases: results }, null, 2)}\n`,
    );
  }
  await page.evaluate(() => window.localStorage.removeItem('omnillm-video-decoder-budget'));

  const summary = {
    schema_version: 3,
    fixture: fixture.name,
    project_id: seedResult.project_id,
    timeline_id: seedResult.timeline_id,
    timeline_revision: seedResult.timeline_revision,
    timeline_sha256: seedResult.timeline_sha256,
    fps: fixture.timeline.canvas.fps,
    cases: results,
    pass: results.every((result) => result.pass),
  };
  await fs.writeFile(path.join(output, 'playback-evidence.json'), `${JSON.stringify(summary, null, 2)}\n`);
  console.log(JSON.stringify(summary));
  if (!summary.pass) throw new Error('normal-playback canonicalization evidence gate failed');
} finally {
  await browser.close();
}

async function seekParityFrame(page, frameIndex) {
  const requestId = `playback-${frameIndex}-${Date.now()}`;
  // The editor's parity event is the production seek command, but its separate
  // ready event is intentionally not used as the acknowledgement here. A fresh
  // route can expose the preview DOM one turn before the effect listener is
  // registered. Re-issue the idempotent seek command until the editor's own DOM
  // state reports the requested deterministic canonical frame, then separately
  // prove any mounted video decoder has settled.
  await page.evaluate(({ frameIndex: targetFrame, requestId: id }) => new Promise((resolve, reject) => {
    const deadline = performance.now() + 10_000;
    const attempt = () => {
      const stage = document.querySelector('[data-testid="video-preview-program"]');
      if (stage?.dataset.previewVisualFrameMode === 'deterministic-canonical'
        && stage.dataset.parityFrameIndex === String(targetFrame)) {
        resolve();
        return;
      }
      if (performance.now() >= deadline) {
        reject(new Error(`playback parity seek ${targetFrame} did not reach deterministic frame: ${JSON.stringify({
          mode: stage?.dataset.previewVisualFrameMode || '',
          parity_frame_index: stage?.dataset.parityFrameIndex || '',
          parity_time_ms: stage?.dataset.parityTimeMs || '',
        })}`));
        return;
      }
      window.dispatchEvent(new CustomEvent('omnillm:video-parity-seek', {
        detail: { frameIndex: targetFrame, requestId: id },
      }));
      window.setTimeout(attempt, 100);
    };
    attempt();
  }), { frameIndex, requestId });

  await page.waitForFunction(() => [...document.querySelectorAll('[data-video-preview-media="true"]')]
    .every((media) => media instanceof HTMLVideoElement && media.readyState >= 2 && !media.seeking), null, { timeout: 5_000 });
}

function gateCase(testCase, observations, timeline) {
  const errors = [];
  const fps = timeline.canvas.fps;
  const stable = observations.filter((row) => Number.isFinite(row.timeline_ms));
  if (stable.length < 5) errors.push(`captured ${stable.length} observations, want at least 5`);

  for (const row of stable) {
    if (row.visual_frame_mode !== testCase.expected_mode) {
      errors.push(`mode ${row.visual_frame_mode}, want ${testCase.expected_mode}`);
      break;
    }
    if (row.transition_plan_mode !== testCase.expected_transition_mode) {
      errors.push(`transition mode ${row.transition_plan_mode}, want ${testCase.expected_transition_mode}`);
      break;
    }
    if ((testCase.expected_reason || '') !== row.playback_deferred_reason) {
      errors.push(`deferred reason ${row.playback_deferred_reason}, want ${testCase.expected_reason || '<empty>'}`);
      break;
    }
    if (testCase.expected_weighted_runtime
      && row.weighted_playback_runtime !== testCase.expected_weighted_runtime) {
      errors.push(`weighted runtime ${row.weighted_playback_runtime}, want ${testCase.expected_weighted_runtime}`);
      break;
    }
    if (testCase.expected_weighted_consumer
      && row.weighted_playback_consumer !== testCase.expected_weighted_consumer) {
      errors.push(`weighted consumer ${row.weighted_playback_consumer}, want ${testCase.expected_weighted_consumer}`);
      break;
    }
    if (testCase.expected_weighted_pair_id
      && !row.weighted_playback_plan_key.includes(`${testCase.expected_weighted_pair_id}:`)) {
      errors.push(`weighted pair ${row.weighted_playback_plan_key || '<empty>'}, want ${testCase.expected_weighted_pair_id}`);
      break;
    }
    if (testCase.expected_text_runtime && row.text_playback_runtime !== testCase.expected_text_runtime) {
      errors.push(`text runtime ${row.text_playback_runtime}, want ${testCase.expected_text_runtime}`);
      break;
    }
    if (testCase.expected_text_consumer && row.text_playback_consumer !== testCase.expected_text_consumer) {
      errors.push(`text consumer ${row.text_playback_consumer}, want ${testCase.expected_text_consumer}`);
      break;
    }
    if (testCase.expected_text_clip_id
      && !row.text_playback_plan_key.includes(`\"clip_id\":\"${testCase.expected_text_clip_id}\"`)) {
      errors.push(`text plan ${row.text_playback_plan_key || '<empty>'}, want ${testCase.expected_text_clip_id}`);
      break;
    }
    for (const trace of testCase.expected_text_trace || []) {
      if (!row.text_playback_readiness_trace.includes(trace)) {
        errors.push(`text readiness trace ${row.text_playback_readiness_trace || '<empty>'} is missing ${trace}`);
        break;
      }
    }
    if (errors.length > 0) break;
    if (testCase.require_cursor_surface) {
      const cursor = row.cursor_surfaces.find((surface) => surface.clip_id === testCase.expected_cursor_clip_id);
      if (!cursor) {
        errors.push(`cursor surface ${testCase.expected_cursor_clip_id || '<unspecified>'} is missing`);
        break;
      }
      if (cursor.consumer !== testCase.expected_cursor_consumer) {
        errors.push(`cursor consumer ${cursor.consumer || '<empty>'}, want ${testCase.expected_cursor_consumer}`);
        break;
      }
      const expectedStateMode = testCase.expected_mode === 'canonical-playback' ? 'canonical-frame' : 'legacy-time';
      if (cursor.state_mode !== expectedStateMode) {
        errors.push(`cursor state mode ${cursor.state_mode || '<empty>'}, want ${expectedStateMode}`);
        break;
      }
      if (testCase.expected_mode === 'canonical-playback') {
        const expected = expectedCursorSampleAtFrame(timeline, testCase.expected_cursor_clip_id, row.visual_frame_index);
        if (!expected) {
          errors.push(`canonical cursor expectation ${testCase.expected_cursor_clip_id} is unavailable`);
          break;
        }
        if (!Number.isFinite(cursor.x) || Math.abs(cursor.x - expected.x) > 1e-6
          || !Number.isFinite(cursor.y) || Math.abs(cursor.y - expected.y) > 1e-6) {
          errors.push(`cursor sample (${cursor.x},${cursor.y}), want (${expected.x},${expected.y}) at frame ${row.visual_frame_index}`);
          break;
        }
        if (cursor.click !== expected.click
          || cursor.highlight !== expected.highlight
          || cursor.click_rings !== expected.click_rings
          || !Number.isFinite(cursor.scale)
          || Math.abs(cursor.scale - expected.scale) > 1e-9) {
          errors.push(`cursor state ${JSON.stringify(cursor)}, want ${JSON.stringify(expected)}`);
          break;
        }
      }
    }
    if (testCase.require_text_layout) {
      if (row.text_surface_count < 1) {
        errors.push('text playback has no canonical prewarm surface');
        break;
      }
      if (row.text_surface_ready_count !== row.text_surface_count) {
        errors.push(`text surfaces ready ${row.text_surface_ready_count}/${row.text_surface_count}`);
        break;
      }
      if (row.text_surface_layout_count !== row.text_surface_count) {
        errors.push(`text layouts ready ${row.text_surface_layout_count}/${row.text_surface_count}`);
        break;
      }
      if (row.text_surface_font_ready_count !== row.text_surface_count) {
        errors.push(`text resource fonts ready ${row.text_surface_font_ready_count}/${row.text_surface_count}`);
        break;
      }
      if (testCase.expected_text_consumer === 'canonical-text-dom'
        && row.text_surface_visible_count !== row.text_surface_count) {
        errors.push(`canonical text surfaces visible ${row.text_surface_visible_count}/${row.text_surface_count}`);
        break;
      }
      if (testCase.expected_text_consumer === 'legacy-time-fallback' && row.text_surface_visible_count !== 0) {
        errors.push(`fallback frame exposed ${row.text_surface_visible_count} canonical text surfaces`);
        break;
      }
    }
    if (testCase.require_weighted_canvas) {
      if (row.weighted_surface_count < 1) {
        errors.push('weighted canonical playback has no playback Canvas surface');
        break;
      }
      if (row.weighted_surface_ready_count !== row.weighted_surface_count) {
        errors.push(`weighted Canvas ready ${row.weighted_surface_ready_count}/${row.weighted_surface_count}`);
        break;
      }
      if (row.weighted_surface_errors.length > 0) {
        errors.push(`weighted Canvas errors: ${row.weighted_surface_errors.join(',')}`);
        break;
      }
      if (row.weighted_surface_pending_reasons.length > 0) {
        errors.push(`weighted Canvas pending: ${row.weighted_surface_pending_reasons.join(',')}`);
        break;
      }
      if (row.weighted_surface_runtime_keys.length !== row.weighted_surface_count) {
        errors.push('weighted Canvas runtime key is missing');
        break;
      }
      if (testCase.expected_weighted_pair_id
        && row.weighted_surface_runtime_keys.some((key) => !key.includes(`${testCase.expected_weighted_pair_id}:`))) {
        errors.push(`weighted Canvas runtime key does not match ${testCase.expected_weighted_pair_id}`);
        break;
      }
    }
  }

  if (testCase.require_cursor_surface) {
    const cursorRows = stable
      .map((row) => row.cursor_surfaces.find((surface) => surface.clip_id === testCase.expected_cursor_clip_id))
      .filter(Boolean);
    if (cursorRows.length !== stable.length) errors.push(`cursor surface observed ${cursorRows.length}/${stable.length} frames`);
    if (testCase.require_cursor_motion && cursorRows.length > 1) {
      const motion = Math.max(range(cursorRows.map((row) => row.x).filter(Number.isFinite)), range(cursorRows.map((row) => row.y).filter(Number.isFinite)));
      if (motion < 1) errors.push(`cursor motion advanced only ${motion}px in canonical sample space`);
    }
    if (testCase.require_cursor_highlight && !cursorRows.every((row) => row.highlight === true)) {
      errors.push('cursor highlight was not retained for every observation');
    }
    if (testCase.require_cursor_click_toggle) {
      const clickStates = new Set(cursorRows.map((row) => row.click));
      if (!clickStates.has(false) || !clickStates.has(true)) errors.push('cursor click-ring window did not cross both false and true states');
    }
  }

  const timelineTimes = stable.map((row) => row.timeline_ms);
  const timelineAdvanceMs = range(timelineTimes);
  if (timelineAdvanceMs < Math.min(150, testCase.observe_ms * 0.35)) {
    errors.push(`timeline clock advanced ${timelineAdvanceMs}ms during ${testCase.observe_ms}ms observation`);
  }

  const audioTimes = stable.map((row) => row.audio_current_time).filter(Number.isFinite);
  const audioAdvanceSeconds = range(audioTimes);
  if (audioTimes.length < 3) errors.push('continuous audio element was not observed');
  if (audioAdvanceSeconds < 0.1) errors.push(`audio clock advanced only ${audioAdvanceSeconds}s`);

  const visualFrames = stable.map((row) => row.visual_frame_index).filter(Number.isFinite);
  const candidates = stable.map((row) => row.playback_frame_candidate).filter(Number.isFinite);
  if (testCase.expected_mode === 'canonical-playback') {
    if (visualFrames.length !== stable.length || candidates.length !== stable.length) {
      errors.push('canonical playback observation is missing visual frame identity');
    }
    for (const row of stable) {
      if (row.visual_frame_index !== row.playback_frame_candidate || row.parity_frame_index !== row.visual_frame_index) {
        errors.push(`frame identity drift at timeline ${row.timeline_ms}ms`);
        break;
      }
    }
    const uniqueFrames = new Set(visualFrames);
    if (testCase.require_advancing_frames && uniqueFrames.size < 4) {
      errors.push(`canonical visual frame advanced through only ${uniqueFrames.size} unique frames`);
    }
    const hasContinuousUITick = stable.some((row) => (
      Number.isFinite(row.visual_frame_index)
      && Math.abs(row.timeline_ms - (row.visual_frame_index * 1000) / fps) > 1
    ));
    if (!hasContinuousUITick) errors.push('timeline UI clock appears quantized to canonical visual frame boundaries');
    const hasIndependentAudioTick = stable.some((row) => (
      Number.isFinite(row.audio_current_time)
      && Number.isFinite(row.visual_frame_index)
      && Math.abs(row.audio_current_time * 1000 - (row.visual_frame_index * 1000) / fps) > 2
    ));
    if (!hasIndependentAudioTick) errors.push('audio clock appears quantized to canonical visual frame boundaries');
  } else if (visualFrames.length > 0) {
    errors.push('fallback playback unexpectedly published canonical visual frame identity');
  }

  return {
    name: testCase.name,
    expected_mode: testCase.expected_mode,
    expected_reason: testCase.expected_reason || '',
    expected_transition_mode: testCase.expected_transition_mode,
    expected_weighted_runtime: testCase.expected_weighted_runtime || '',
    expected_weighted_consumer: testCase.expected_weighted_consumer || '',
    expected_weighted_pair_id: testCase.expected_weighted_pair_id || '',
    expected_text_runtime: testCase.expected_text_runtime || '',
    expected_text_consumer: testCase.expected_text_consumer || '',
    expected_text_clip_id: testCase.expected_text_clip_id || '',
    expected_text_trace: testCase.expected_text_trace || [],
    expected_cursor_consumer: testCase.expected_cursor_consumer || '',
    expected_cursor_clip_id: testCase.expected_cursor_clip_id || '',
    decoder_budget: testCase.decoder_budget || 0,
    observations: stable,
    timeline_advance_ms: timelineAdvanceMs,
    audio_advance_seconds: audioAdvanceSeconds,
    unique_visual_frames: [...new Set(visualFrames)],
    pass: errors.length === 0,
    errors,
  };
}

function expectedCursorSampleAtFrame(timeline, clipId, frameIndex) {
  if (!Number.isFinite(frameIndex)) return null;
  let clip = null;
  for (const track of timeline.tracks || []) {
    const found = (track.clips || []).find((candidate) => candidate.id === clipId);
    if (found) { clip = found; break; }
  }
  const cursor = clip?.cursor;
  const events = cursor?.events || [];
  if (!clip || cursor?.visible === false || events.length === 0) return null;
  const fps = timeline.canvas.fps;
  const numerator = frameIndex * 1000 - clip.start_ms * fps;
  const denominator = fps;
  let previous = events[0];
  let x = previous.x;
  let y = previous.y;
  if (numerator > previous.time_ms * denominator) {
    for (let index = 1; index < events.length; index += 1) {
      const next = events[index];
      if (numerator <= next.time_ms * denominator) {
        const span = Math.max(1, next.time_ms - previous.time_ms);
        const progress = (numerator - previous.time_ms * denominator) / (span * denominator);
        x = previous.x + (next.x - previous.x) * progress;
        y = previous.y + (next.y - previous.y) * progress;
        previous = null;
        break;
      }
      previous = next;
    }
    if (previous) { x = previous.x; y = previous.y; }
  }
  const clickWindow = 300 * denominator;
  const click = events.some((event) => event.click === true
    && Math.abs(event.time_ms * denominator - numerator) < clickWindow);
  return {
    x,
    y,
    click,
    scale: cursor.scale ?? 1,
    highlight: cursor.highlight === true,
    click_rings: cursor.click_rings === true,
  };
}

function range(values) {
  if (values.length === 0) return 0;
  return Math.max(...values) - Math.min(...values);
}

async function seedFixtureProject({ context, fixture, mediaDir, baseURL, headers }) {
  const apiURL = (pathname) => new URL(`/v1${pathname}`, baseURL).toString();
  const jsonRequest = async (method, pathname, data) => {
    const response = await context.request.fetch(apiURL(pathname), { method, data, headers });
    if (!response.ok()) throw new Error(`${method} ${pathname}: HTTP ${response.status()} ${await response.text()}`);
    return response.json();
  };

  const project = await jsonRequest('POST', '/video/projects', {
    title: 'Playback Canonical Parity',
    width: fixture.timeline.canvas.width,
    height: fixture.timeline.canvas.height,
    fps: fixture.timeline.canvas.fps,
    aspect_ratio: `${fixture.timeline.canvas.width}:${fixture.timeline.canvas.height}`,
  });
  const fileForAsset = {
    'asset-landscape': ['asset-landscape.mp4', 'video/mp4'],
    'asset-square': ['asset-square.png', 'image/png'],
    'asset-audio': ['asset-audio.wav', 'audio/wav'],
    'asset-font': ['asset-font.ttf', 'font/ttf', 'playback-font-v1'],
    'asset-font-invalid': ['asset-font-invalid.ttf', 'font/ttf', 'playback-font-invalid-v1'],
  };
  const assetIDs = {};
  for (const asset of fixture.assets) {
    const recipe = fileForAsset[asset.id];
    if (!recipe) throw new Error(`no generated media recipe for playback fixture asset ${asset.id}`);
    const filePath = path.join(mediaDir, recipe[0]);
    const multipart = { file: { name: recipe[0], mimeType: recipe[1], buffer: await fs.readFile(filePath) } };
    if (recipe[2]) multipart.font_resource_id = recipe[2];
    const response = await context.request.post(apiURL(`/video/projects/${encodeURIComponent(project.id)}/assets/upload`), {
      headers,
      multipart,
    });
    if (!response.ok()) throw new Error(`upload ${asset.id}: HTTP ${response.status()} ${await response.text()}`);
    const uploaded = await response.json();
    assetIDs[asset.id] = uploaded.id;
  }

  const timeline = structuredClone(fixture.timeline);
  for (const track of timeline.tracks) {
    for (const clip of track.clips) {
      if (!clip.asset_id) continue;
      const mapped = assetIDs[clip.asset_id];
      if (!mapped) throw new Error(`timeline references unmapped playback fixture asset ${clip.asset_id}`);
      clip.asset_id = mapped;
    }
  }
  const saved = await jsonRequest('PUT', `/video/projects/${encodeURIComponent(project.id)}/timeline`, timeline);
  return {
    project_id: project.id,
    timeline_id: saved.timeline.id,
    timeline_revision: saved.timeline.revision,
    timeline_sha256: saved.timeline.content_sha256,
    asset_ids: assetIDs,
  };
}
