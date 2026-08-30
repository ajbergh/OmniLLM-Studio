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
if (!seedResult.project_id || !seedResult.timeline_sha256 || !seedResult.snapshot_id) {
  throw new Error('seed result is missing project/timeline/snapshot identity');
}
const output = path.resolve(args.output);
const editorURL = new URL(`/video/${encodeURIComponent(seedResult.project_id)}/edit`, args.url).toString();
const canvasWidth = fixture.timeline.canvas.width;
const canvasHeight = fixture.timeline.canvas.height;
const fps = fixture.timeline.canvas.fps;
const decodedVideoFixture = fixture.name === 'parity-pixelate-decoded-video-v1';
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
  await page.waitForFunction(({ width, height }) => {
    const rect = document.querySelector('[data-testid="video-preview-program"]')?.getBoundingClientRect();
    return Boolean(rect && Math.abs(rect.width - width) < 0.01 && Math.abs(rect.height - height) < 0.01);
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
      const callbackIDs = new Map();
      const deadline = performance.now() + 10_000;

      const cleanup = () => {
        window.removeEventListener('omnillm:video-parity-ready', ready);
        if (initialFallbackTimer !== null) window.clearTimeout(initialFallbackTimer);
        for (const [video, callbackID] of callbackIDs.entries()) {
          video.cancelVideoFrameCallback?.(callbackID);
        }
        callbackIDs.clear();
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
        const callbackID = video.requestVideoFrameCallback((_now, metadata) => {
          callbackIDs.delete(video);
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
        callbackIDs.set(video, callbackID);
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
        // The initial decoded frame can already be current before an rVFC is
        // registered. Retain that one explicit zero-frame fallback only; every
        // later deterministic sample must be proven by presentation metadata.
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

    if (decodedVideoFixture) {
      if (presentations.length === 0) {
        throw new Error(`frame ${sample.frame_index} has no decoded-video presentation evidence`);
      }
      for (const presentation of presentations) {
        if (presentation.presented_frame_index !== sample.frame_index) {
          throw new Error(`frame ${sample.frame_index} presented decoded frame ${presentation.presented_frame_index}`);
        }
        if (sample.frame_index > 0
          && presentation.element_current_time_seconds <= presentation.requested_media_time_seconds) {
          throw new Error(`frame ${sample.frame_index} did not seek inside the requested decoded-frame interval: ${JSON.stringify(presentation)}`);
        }
      }
    }

    const codecRegion = decodedVideoFixture
      ? await page.evaluate(async ({ frameIndex, snapshotID, canvasWidth }) => {
        const stage = document.querySelector('[data-testid="video-preview-program"]');
        if (!(stage instanceof HTMLElement)) throw new Error('preview program missing while measuring codec region');
        const output = stage.querySelector('[data-preview-pixelate-execution="canvas"] canvas');
        if (!(output instanceof HTMLCanvasElement)) throw new Error('canonical pixelate output canvas missing');
        const previewContext = output.getContext('2d', { willReadFrequently: true });
        if (!previewContext) throw new Error('canonical pixelate output has no 2D context');
        const stageRect = stage.getBoundingClientRect();
        const stageScale = stageRect.width / canvasWidth;
        if (!Number.isFinite(stageScale) || stageScale <= 0) throw new Error(`invalid preview stage scale ${stageScale}`);
        const minX = Math.round(Number.parseFloat(output.style.left || '0') / stageScale);
        const minY = Math.round(Number.parseFloat(output.style.top || '0') / stageScale);
        const width = output.width;
        const height = output.height;
        const preview = previewContext.getImageData(0, 0, width, height).data;

        const token = window.localStorage.getItem('omnillm_auth_token');
        const headers = token ? { Authorization: `Bearer ${token}` } : {};
        const response = await fetch(`/v1/video/render-snapshots/${encodeURIComponent(snapshotID)}/frames/${frameIndex}`, { headers });
        if (!response.ok) throw new Error(`diagnostic rendered frame ${frameIndex}: HTTP ${response.status} ${await response.text()}`);
        const bitmap = await createImageBitmap(await response.blob());
        try {
          const renderedCanvas = document.createElement('canvas');
          renderedCanvas.width = bitmap.width;
          renderedCanvas.height = bitmap.height;
          const renderedContext = renderedCanvas.getContext('2d', { willReadFrequently: true });
          if (!renderedContext) throw new Error('rendered-frame comparison has no 2D context');
          renderedContext.drawImage(bitmap, 0, 0);
          const rendered = renderedContext.getImageData(minX, minY, width, height).data;
          let maxChannelDelta = 0;
          let pixelsWithinTolerance = 0;
          const comparedPixels = width * height;
          for (let pixel = 0; pixel < comparedPixels; pixel += 1) {
            const offset = pixel * 4;
            let pixelWithin = true;
            for (let channel = 0; channel < 3; channel += 1) {
              const delta = Math.abs(preview[offset + channel] - rendered[offset + channel]);
              if (delta > 3) pixelWithin = false;
              if (delta > maxChannelDelta) maxChannelDelta = delta;
            }
            if (pixelWithin) pixelsWithinTolerance += 1;
          }
          return {
            bounds: { min_x: minX, min_y: minY, max_x: minX + width, max_y: minY + height },
            compared_pixels: comparedPixels,
            channel_tolerance: 3,
            pixels_within_tolerance: pixelsWithinTolerance,
            pixel_pass_rate: pixelsWithinTolerance / comparedPixels,
            max_channel_delta: maxChannelDelta,
          };
        } finally {
          bitmap.close();
        }
      }, { frameIndex: sample.frame_index, snapshotID: seedResult.snapshot_id, canvasWidth })
      : null;

    if (codecRegion && (codecRegion.max_channel_delta > 3 || codecRegion.pixel_pass_rate !== 1)) {
      throw new Error(`frame ${sample.frame_index} exceeded decoded H.264 ±3 RGB gate: ${JSON.stringify(codecRegion)}`);
    }

    evidence.push({
      frame_index: sample.frame_index,
      time_ms: sample.time_ms,
      name: sample.name,
      presentations,
      ...(codecRegion ? { codec_region: codecRegion } : {}),
      ...state,
    });
  }
} finally {
  await browser.close();
}

await fs.mkdir(path.dirname(output), { recursive: true });
await fs.writeFile(output, `${JSON.stringify({
  schema_version: 3,
  fixture: fixture.name,
  timeline_sha256: seedResult.timeline_sha256,
  snapshot_id: seedResult.snapshot_id,
  fps,
  decoded_frame_identity_gate: decodedVideoFixture,
  codec_color_channel_tolerance: decodedVideoFixture ? 3 : null,
  frames: evidence,
}, null, 2)}\n`);
console.log(`pixelate Canvas evidence: ${output} (${evidence.length} frames)`);
