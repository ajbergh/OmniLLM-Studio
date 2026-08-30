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

if (!args.url || !args.fixture || !args['seed-result'] || !args.output) {
  console.error('usage: node scripts/video-pixelate-parity-assert.mjs --url <app-url> --fixture <fixture-json> --seed-result <seed-result-json> --output <evidence-json>');
  process.exit(1);
}

const fixture = JSON.parse(await fs.readFile(path.resolve(args.fixture), 'utf8'));
const seedResult = JSON.parse(await fs.readFile(path.resolve(args['seed-result']), 'utf8'));
const output = path.resolve(args.output);
const editorURL = new URL(`/video/${encodeURIComponent(seedResult.project_id)}/edit`, args.url).toString();
const canvasWidth = fixture.timeline.canvas.width;
const canvasHeight = fixture.timeline.canvas.height;
const fps = fixture.timeline.canvas.fps;
const evidence = [];

const browser = await chromium.launch({ headless: true });
try {
  const context = await browser.newContext({ viewport: { width: 1400, height: 1000 }, deviceScaleFactor: 1 });
  const page = await context.newPage();
  await page.goto(editorURL, { waitUntil: 'networkidle' });
  const program = page.getByTestId('video-preview-program');
  await program.waitFor({ state: 'visible' });
  await program.evaluate((element, canvas) => {
    const fit = element.parentElement;
    if (!fit) throw new Error('preview program has no fit container');
    fit.style.setProperty('width', `${canvas.width}px`, 'important');
    fit.style.setProperty('height', `${canvas.height}px`, 'important');
    fit.style.setProperty('min-height', `${canvas.height}px`, 'important');
    fit.style.setProperty('flex', '0 0 auto', 'important');
    fit.style.setProperty('padding', '0', 'important');
    element.style.setProperty('border', '0', 'important');
    element.style.setProperty('flex', '0 0 auto', 'important');
  }, { width: canvasWidth, height: canvasHeight });

  const readPixelateState = () => program.evaluate((stage) => {
    const host = stage.querySelector('[data-preview-pixelate-host="canonical-canvas"]');
    const surface = stage.querySelector('[data-preview-pixelate-execution="canvas"]');
    const fallback = stage.querySelector('[data-preview-shape-painter-deferred="pixelate-css-approximation"]');
    return {
      parity_frame_index: stage.dataset.parityFrameIndex ?? null,
      pixelate_frame_index: stage.dataset.previewPixelateFrameIndex ?? null,
      plan_mode: stage.dataset.previewPixelatePlanMode ?? null,
      consumer: stage.dataset.previewPixelateConsumer ?? null,
      structural_deferred: stage.dataset.previewPixelateStructuralDeferred ?? null,
      runtime_deferred: stage.dataset.previewPixelateRuntimeDeferred ?? null,
      runtime_error: stage.dataset.previewPixelateRuntimeError ?? null,
      target_host: host?.getAttribute('data-preview-pixelate-host') ?? null,
      surface_status: surface?.getAttribute('data-preview-pixelate-status') ?? null,
      surface_reason: surface?.getAttribute('data-preview-pixelate-reason') ?? null,
      css_fallback_marker_present: Boolean(fallback),
    };
  });

  for (const sample of fixture.samples) {
    const requestId = `pixelate-evidence-${sample.frame_index}`;
    const presentations = await page.evaluate(({ frameIndex, id, fps }) => new Promise((resolve, reject) => {
      const videos = [...document.querySelectorAll('[data-video-preview-media="true"]')]
        .filter((media) => media instanceof HTMLVideoElement);
      const expectedMediaTimeSeconds = frameIndex / fps;
      const matched = new Map();
      let readySeen = false;
      let initialFallbackTimer = null;
      const deadline = performance.now() + 10_000;

      const cleanup = () => {
        window.removeEventListener('omnillm:video-parity-ready', ready);
        if (initialFallbackTimer !== null) window.clearTimeout(initialFallbackTimer);
      };
      const snapshot = (video, index, method, metadata) => {
        const mediaTime = metadata?.mediaTime ?? video.currentTime;
        return {
          video_index: index,
          method,
          requested_frame_index: frameIndex,
          requested_media_time_seconds: expectedMediaTimeSeconds,
          element_current_time_seconds: video.currentTime,
          presented_media_time_seconds: mediaTime,
          presented_frame_index: Math.round(mediaTime * fps),
          presented_frames: metadata?.presentedFrames ?? null,
          width: metadata?.width ?? video.videoWidth,
          height: metadata?.height ?? video.videoHeight,
        };
      };
      const finishIfComplete = () => {
        if (!readySeen || matched.size !== videos.length) return;
        cleanup();
        resolve([...matched.entries()].sort((a, b) => a[0] - b[0]).map(([, value]) => value));
      };
      const observe = (video, index) => {
        if (typeof video.requestVideoFrameCallback !== 'function') {
          cleanup();
          reject(new Error(`requestVideoFrameCallback unavailable for video ${index}`));
          return;
        }
        video.requestVideoFrameCallback((_now, metadata) => {
          const current = snapshot(video, index, 'requestVideoFrameCallback', metadata);
          if (current.presented_frame_index === frameIndex) {
            matched.set(index, current);
            finishIfComplete();
            return;
          }
          if (performance.now() >= deadline) {
            cleanup();
            reject(new Error(`video ${index} presented frame ${current.presented_frame_index}, want ${frameIndex}`));
            return;
          }
          observe(video, index);
        });
      };

      for (const [index, video] of videos.entries()) observe(video, index);

      const timeout = window.setTimeout(() => {
        cleanup();
        reject(new Error(`parity-ready/presented-frame timed out for frame ${frameIndex}`));
      }, 10_000);
      const ready = (event) => {
        if (event.detail?.requestId !== id) return;
        window.clearTimeout(timeout);
        readySeen = true;
        if (videos.length === 0) {
          cleanup();
          resolve([]);
          return;
        }
        if (frameIndex === 0 && matched.size !== videos.length) {
          initialFallbackTimer = window.setTimeout(() => {
            for (const [index, video] of videos.entries()) {
              if (!matched.has(index)) matched.set(index, snapshot(video, index, 'initial-current-time', null));
            }
            finishIfComplete();
          }, 250);
        }
        finishIfComplete();
      };
      window.addEventListener('omnillm:video-parity-ready', ready);
      window.dispatchEvent(new CustomEvent('omnillm:video-parity-seek', { detail: { frameIndex, requestId: id } }));
    }), { frameIndex: sample.frame_index, id: requestId, fps });

    try {
      await page.waitForFunction((frameIndex) => {
        const stage = document.querySelector('[data-testid="video-preview-program"]');
        const surface = stage?.querySelector('[data-preview-pixelate-execution="canvas"]');
        return stage?.dataset.parityFrameIndex === String(frameIndex)
          && stage?.dataset.previewPixelateConsumer === 'canonical-canvas'
          && surface?.getAttribute('data-preview-pixelate-status') === 'ready';
      }, sample.frame_index, { timeout: 5000 });
    } catch (error) {
      const state = await readPixelateState();
      throw new Error(`frame ${sample.frame_index} did not activate canonical pixelate Canvas: ${JSON.stringify(state)}`, { cause: error });
    }

    const state = await readPixelateState();
    if (state.plan_mode !== 'canonical-ready'
      || state.consumer !== 'canonical-canvas'
      || state.target_host !== 'canonical-canvas'
      || state.surface_status !== 'ready'
      || state.structural_deferred
      || state.runtime_deferred
      || state.runtime_error
      || state.css_fallback_marker_present) {
      throw new Error(`frame ${sample.frame_index} did not prove exact pixelate Canvas execution: ${JSON.stringify(state)}`);
    }
    evidence.push({
      frame_index: sample.frame_index,
      time_ms: sample.time_ms,
      name: sample.name,
      presentations,
      ...state,
    });
  }
} finally {
  await browser.close();
}

await fs.mkdir(path.dirname(output), { recursive: true });
await fs.writeFile(output, `${JSON.stringify({
  schema_version: 2,
  fixture: fixture.name,
  timeline_sha256: seedResult.timeline_sha256,
  fps,
  frames: evidence,
}, null, 2)}\n`);
console.log(`pixelate Canvas evidence: ${output} (${evidence.length} frames)`);
