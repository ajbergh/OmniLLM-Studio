#!/usr/bin/env node
import fs from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';
import { createHash } from 'node:crypto';
import { chromium } from 'playwright';

const args = Object.fromEntries(process.argv.slice(2).reduce((pairs, value, index, values) => {
  if (!value.startsWith('--')) return pairs;
  pairs.push([value.slice(2), values[index + 1]]);
  return pairs;
}, []));

if (!args.url || !args.snapshot || !args.fixture) {
  console.error('usage: node scripts/video-parity-capture.mjs --url <editor-url> --snapshot <id> --fixture <generated-json> [--output <dir>]');
  process.exit(1);
}

const output = path.resolve(args.output || 'output/video-parity-capture');
const previewDir = path.join(output, 'preview');
const renderedDir = path.join(output, 'rendered');
await fs.mkdir(previewDir, { recursive: true });
await fs.mkdir(renderedDir, { recursive: true });

const fixture = JSON.parse(await fs.readFile(path.resolve(args.fixture), 'utf8'));
const browser = await chromium.launch({ headless: true });
try {
  const context = await browser.newContext({ viewport: { width: 1600, height: 1100 }, deviceScaleFactor: 1 });
  const page = await context.newPage();
  const cdp = await context.newCDPSession(page);
  await page.goto(args.url, { waitUntil: 'networkidle' });
  const program = page.getByTestId('video-preview-program');
  await program.waitFor({ state: 'visible' });
  await page.addStyleTag({ content: '[data-testid="video-preview-program"]{border:0!important}' });

  for (const sample of fixture.samples) {
    let safeName = `${String(sample.frame_index).padStart(6, '0')}-${sample.name.replace(/[^a-zA-Z0-9._-]+/g, '-')}`;
    if (safeName.length > 112) safeName = `${safeName.slice(0, 96)}-${createHash('sha256').update(safeName).digest('hex').slice(0, 12)}`;
    const requestId = `capture-${safeName}`;
    await page.evaluate(({ frameIndex, id }) => new Promise((resolve) => {
      const ready = (event) => {
        if (event.detail?.requestId !== id) return;
        window.removeEventListener('omnillm:video-parity-ready', ready);
        resolve();
      };
      window.addEventListener('omnillm:video-parity-ready', ready);
      window.dispatchEvent(new CustomEvent('omnillm:video-parity-seek', { detail: { frameIndex, requestId: id } }));
    }), { frameIndex: sample.frame_index, id: requestId });
    await page.waitForFunction((frameIndex) => document.querySelector('[data-testid="video-preview-program"]')?.dataset.parityFrameIndex === String(frameIndex), sample.frame_index);
    const box = await program.boundingBox();
    if (!box) throw new Error(`preview program has no bounding box for frame ${sample.frame_index}`);
    const canvasWidth = fixture.timeline.canvas.width;
    const canvasHeight = fixture.timeline.canvas.height;
    const capture = await cdp.send('Page.captureScreenshot', {
      format: 'png',
      captureBeyondViewport: false,
      clip: { x: box.x, y: box.y, width: box.width, height: box.height, scale: canvasWidth / box.width },
    });
    const previewPNG = Buffer.from(capture.data, 'base64');
    await fs.writeFile(path.join(previewDir, `${safeName}.png`), previewPNG);
    if (Math.abs((box.height * canvasWidth) / box.width - canvasHeight) > 1) {
      throw new Error(`preview aspect ratio ${box.width}x${box.height} does not match fixture ${canvasWidth}x${canvasHeight}`);
    }

    const endpoint = new URL(`/v1/video/render-snapshots/${encodeURIComponent(args.snapshot)}/frames/${sample.frame_index}`, args.url).toString();
    const response = await context.request.get(endpoint);
    if (!response.ok()) throw new Error(`diagnostic frame ${sample.frame_index}: HTTP ${response.status()} ${await response.text()}`);
    await fs.writeFile(path.join(renderedDir, `${safeName}.png`), await response.body());
  }
  const audioEndpoint = new URL(`/v1/video/render-snapshots/${encodeURIComponent(args.snapshot)}/audio.pcm`, args.url).toString();
  const audioResponse = await context.request.get(audioEndpoint);
  let renderedAudio = null;
  if (audioResponse.ok()) {
    renderedAudio = path.join(output, 'rendered-audio-s16le-48k-stereo.pcm');
    await fs.writeFile(renderedAudio, await audioResponse.body());
  } else if (audioResponse.status() !== 400) {
    throw new Error(`diagnostic audio: HTTP ${audioResponse.status()} ${await audioResponse.text()}`);
  }
  console.log(JSON.stringify({ output, previewDir, renderedDir, renderedAudio, samples: fixture.samples.length }));
} finally {
  await browser.close();
}
