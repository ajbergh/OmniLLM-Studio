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

if (!args.url || !args.fixture || !args['seed-result']) {
  console.error('usage: node scripts/video-playback-shape-parity-capture.mjs --url <app-url> --fixture <generated-json> --seed-result <seed-result-json> [--output <dir>]');
  process.exit(1);
}

const fixture = JSON.parse(await fs.readFile(path.resolve(args.fixture), 'utf8'));
const seed = JSON.parse(await fs.readFile(path.resolve(args['seed-result']), 'utf8'));
const output = path.resolve(args.output || 'output/video-playback-canonical/capture/shape');
await fs.mkdir(output, { recursive: true });

const retainedCases = [
  {
    caseName: 'rounded-rectangle-canonical-playback',
    clipId: 'playback-shape-rounded-rectangle',
    expectedMode: 'canonical-playback',
    expectedConsumer: 'canonical-inline',
    expectedStateMode: 'canonical-frame',
    requireGeometry: true,
  },
  {
    caseName: 'unsupported-ellipse-fallback',
    clipId: 'playback-shape-ellipse',
    expectedMode: 'legacy-time-fallback',
    expectedConsumer: 'legacy-time-fallback',
    expectedStateMode: 'legacy-time',
    expectedReason: 'shape-playback-deferred:playback-shape-ellipse:shape-kind-unsupported',
    requireGeometry: false,
  },
];

const browser = await chromium.launch({ headless: true });
try {
  const context = await browser.newContext({ viewport: { width: 1600, height: 1100 }, deviceScaleFactor: 1 });
  const page = await context.newPage();
  const editorURL = new URL(`/video/${encodeURIComponent(seed.project_id)}/edit`, args.url).toString();
  const results = [];

  for (const retained of retainedCases) {
    const testCase = fixture.cases.find((candidate) => candidate.name === retained.caseName);
    if (!testCase) throw new Error(`playback shape parity case ${retained.caseName} is missing`);
    const authored = findClip(fixture.timeline, retained.clipId);
    if (!authored?.shape) throw new Error(`playback shape parity clip ${retained.clipId} is missing`);

    await page.goto(editorURL, { waitUntil: 'networkidle' });
    const program = page.getByTestId('video-preview-program');
    await program.waitFor({ state: 'visible' });
    await seekParityFrame(page, testCase.frame_index);
    await page.getByRole('button', { name: 'Play preview' }).click();
    await page.getByRole('button', { name: 'Pause preview' }).waitFor({ state: 'visible', timeout: 5_000 });

    await page.waitForFunction((expected) => {
      const stage = document.querySelector('[data-testid="video-preview-program"]');
      if (!stage || stage.dataset.previewVisualFrameMode !== expected.mode) return false;
      if ((stage.dataset.previewPlaybackCanonicalDeferred || '') !== (expected.reason || '')) return false;
      const surfaces = visibleShapeConsumers(expected.clipId);
      return surfaces.length === 1
        && surfaces[0].dataset.previewShapePlaybackConsumer === expected.consumer
        && surfaces[0].dataset.previewShapeStateMode === expected.stateMode;

      function visibleShapeConsumers(clipId) {
        return [...document.querySelectorAll('[data-preview-shape-playback-consumer]')].filter((surface) => {
          const owner = surface.closest('[data-preview-shape-playback-clip-id]');
          if (owner?.dataset.previewShapePlaybackClipId !== clipId) return false;
          const style = getComputedStyle(surface);
          return style.display !== 'none' && style.visibility !== 'hidden' && Number.parseFloat(style.opacity || '1') > 0;
        });
      }
    }, {
      clipId: retained.clipId,
      mode: retained.expectedMode,
      reason: retained.expectedReason || '',
      consumer: retained.expectedConsumer,
      stateMode: retained.expectedStateMode,
    }, { timeout: 4_000 });

    const observations = await page.evaluate(async ({ clipId, sampleMs }) => {
      const rows = [];
      const deadline = performance.now() + sampleMs;
      while (performance.now() < deadline) {
        await new Promise((resolve) => requestAnimationFrame(resolve));
        const stage = document.querySelector('[data-testid="video-preview-program"]');
        if (!stage) throw new Error('video preview program disappeared during shape playback evidence');
        const consumers = [...document.querySelectorAll('[data-preview-shape-playback-consumer]')]
          .filter((surface) => surface.closest('[data-preview-shape-playback-clip-id]')?.dataset.previewShapePlaybackClipId === clipId)
          .map((surface) => {
            const style = getComputedStyle(surface);
            const visible = style.display !== 'none' && style.visibility !== 'hidden' && Number.parseFloat(style.opacity || '1') > 0;
            return {
              visible,
              consumer: surface.dataset.previewShapePlaybackConsumer || '',
              state_mode: surface.dataset.previewShapeStateMode || '',
              kind: surface.dataset.previewShapeKind || '',
              width: numberOrNull(surface.dataset.previewShapeWidth),
              height: numberOrNull(surface.dataset.previewShapeHeight),
              corner_radius: numberOrNull(surface.dataset.previewShapeCornerRadius),
              stroke_width: numberOrNull(surface.dataset.previewShapeStrokeWidth),
            };
          });
        rows.push({
          timeline_ms: Number(stage.dataset.parityTimeMs),
          visual_frame_mode: stage.dataset.previewVisualFrameMode || '',
          visual_frame_index: numberOrNull(stage.dataset.previewVisualFrameIndex),
          playback_frame_candidate: numberOrNull(stage.dataset.previewPlaybackFrameCandidate),
          playback_deferred_reason: stage.dataset.previewPlaybackCanonicalDeferred || '',
          consumers,
        });
      }
      return rows;

      function numberOrNull(value) {
        if (value === undefined || value === '') return null;
        const number = Number(value);
        return Number.isFinite(number) ? number : null;
      }
    }, { clipId: retained.clipId, sampleMs: Math.max(250, testCase.observe_ms) });

    await page.getByRole('button', { name: 'Pause preview' }).click();
    const errors = gateShapeCase(retained, authored, observations);
    results.push({
      name: retained.caseName,
      clip_id: retained.clipId,
      expected_mode: retained.expectedMode,
      expected_consumer: retained.expectedConsumer,
      expected_state_mode: retained.expectedStateMode,
      observations,
      pass: errors.length === 0,
      errors,
    });
  }

  const summary = {
    schema_version: 1,
    fixture: fixture.name,
    project_id: seed.project_id,
    cases: results,
    pass: results.every((result) => result.pass),
  };
  await fs.writeFile(path.join(output, 'shape-playback-evidence.json'), `${JSON.stringify(summary, null, 2)}\n`);
  console.log(JSON.stringify(summary));
  if (!summary.pass) throw new Error('normal-playback shape consumer evidence gate failed');
} finally {
  await browser.close();
}

function gateShapeCase(expected, authored, observations) {
  const errors = [];
  const stable = observations.filter((row) => Number.isFinite(row.timeline_ms));
  if (stable.length < 5) errors.push(`captured ${stable.length} observations, want at least 5`);
  for (const row of stable) {
    if (row.visual_frame_mode !== expected.expectedMode) {
      errors.push(`mode ${row.visual_frame_mode}, want ${expected.expectedMode}`);
      break;
    }
    if (row.playback_deferred_reason !== (expected.expectedReason || '')) {
      errors.push(`deferred reason ${row.playback_deferred_reason}, want ${expected.expectedReason || '<empty>'}`);
      break;
    }
    const visible = row.consumers.filter((consumer) => consumer.visible);
    if (visible.length !== 1) {
      errors.push(`visible shape consumers ${visible.length}, want exactly 1`);
      break;
    }
    const consumer = visible[0];
    if (consumer.consumer !== expected.expectedConsumer || consumer.state_mode !== expected.expectedStateMode) {
      errors.push(`shape consumer ${consumer.consumer}/${consumer.state_mode}, want ${expected.expectedConsumer}/${expected.expectedStateMode}`);
      break;
    }
    if (expected.requireGeometry) {
      if (consumer.kind !== authored.shape.kind
        || consumer.width !== Math.max(2, authored.shape.width || 320)
        || consumer.height !== Math.max(2, authored.shape.height || 180)
        || consumer.corner_radius !== Math.max(0, authored.shape.corner_radius || 0)
        || consumer.stroke_width !== Math.max(0, authored.shape.stroke_width || 0)) {
        errors.push(`canonical shape geometry ${JSON.stringify(consumer)} does not match authored ${JSON.stringify(authored.shape)}`);
        break;
      }
      if (row.visual_frame_index !== row.playback_frame_candidate) {
        errors.push(`canonical shape frame identity drift ${row.visual_frame_index}/${row.playback_frame_candidate}`);
        break;
      }
    }
  }
  return errors;
}

async function seekParityFrame(page, frameIndex) {
  const requestId = `shape-playback-${frameIndex}-${Date.now()}`;
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
        reject(new Error(`shape playback parity seek ${targetFrame} did not reach deterministic frame`));
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

function findClip(timeline, clipId) {
  for (const track of timeline.tracks || []) {
    const clip = (track.clips || []).find((candidate) => candidate.id === clipId);
    if (clip) return clip;
  }
  return null;
}
