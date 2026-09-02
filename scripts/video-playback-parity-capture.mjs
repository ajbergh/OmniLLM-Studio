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
    await page.goto(editorURL, { waitUntil: 'networkidle' });
    await program.waitFor({ state: 'visible' });
    await seekParityFrame(page, testCase.frame_index);
    await page.getByRole('button', { name: 'Play preview' }).click();
    await page.getByRole('button', { name: 'Pause preview' }).waitFor({ state: 'visible', timeout: 5_000 });

    const observations = await page.evaluate(async (observeMs) => {
      const rows = [];
      const deadline = performance.now() + observeMs;
      while (performance.now() < deadline) {
        await new Promise((resolve) => requestAnimationFrame(resolve));
        const stage = document.querySelector('[data-testid="video-preview-program"]');
        if (!stage) throw new Error('video preview program disappeared during playback evidence');
        const audio = document.querySelector('audio');
        const videos = [...stage.querySelectorAll('video')];
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
    const result = gateCase(testCase, observations, fixture.timeline.canvas.fps);
    results.push(result);
    await fs.writeFile(
      path.join(output, 'playback-evidence-progress.json'),
      `${JSON.stringify({ schema_version: 1, fixture: fixture.name, cases: results }, null, 2)}\n`,
    );
  }

  const summary = {
    schema_version: 1,
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

function gateCase(testCase, observations, fps) {
  const errors = [];
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
    observations: stable,
    timeline_advance_ms: timelineAdvanceMs,
    audio_advance_seconds: audioAdvanceSeconds,
    unique_visual_frames: [...new Set(visualFrames)],
    pass: errors.length === 0,
    errors,
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
  };
  const assetIDs = {};
  for (const asset of fixture.assets) {
    const recipe = fileForAsset[asset.id];
    if (!recipe) throw new Error(`no generated media recipe for playback fixture asset ${asset.id}`);
    const filePath = path.join(mediaDir, recipe[0]);
    const response = await context.request.post(apiURL(`/video/projects/${encodeURIComponent(project.id)}/assets/upload`), {
      headers,
      multipart: { file: { name: recipe[0], mimeType: recipe[1], buffer: await fs.readFile(filePath) } },
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
