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

if (!args.url || !args.fixture || !args['image-file']) {
  console.error('usage: node scripts/video-cursor-parity-capture.mjs --url <app-url> --fixture <json> --image-file <png> [--output <dir>]');
  process.exit(1);
}

const fixture = JSON.parse(await fs.readFile(path.resolve(args.fixture), 'utf8'));
if (fixture.name !== 'parity-cursor-v1') throw new Error(`unexpected fixture ${fixture.name}`);
const expectedFrames = [20, 21, 50, 79, 80];
if (fixture.timeline?.canvas?.width !== 640 || fixture.timeline?.canvas?.height !== 360 || fixture.timeline?.canvas?.fps !== 100) {
  throw new Error(`cursor fixture canvas drifted: ${JSON.stringify(fixture.timeline?.canvas)}`);
}
if (fixture.samples?.length !== expectedFrames.length || !fixture.samples.every((sample, index) => sample.frame_index === expectedFrames[index])) {
  throw new Error(`cursor fixture sample set drifted: ${JSON.stringify(fixture.samples)}`);
}

const output = path.resolve(args.output || 'output/video-cursor-parity/capture');
const previewDir = path.join(output, 'preview');
const renderedDir = path.join(output, 'rendered');
await fs.mkdir(previewDir, { recursive: true });
await fs.mkdir(renderedDir, { recursive: true });

const imagePath = path.resolve(args['image-file']);
const imageBytes = await fs.readFile(imagePath);
const sourceImageSHA256 = createHash('sha256').update(imageBytes).digest('hex');

function expectedCursorAt(timeMS) {
  const events = fixture.timeline.tracks[0].clips[0].cursor.events;
  let x = events[0].x;
  let y = events[0].y;
  if (timeMS <= events[0].time_ms) return { x, y };
  for (let index = 1; index < events.length; index += 1) {
    const previous = events[index - 1];
    const next = events[index];
    if (timeMS <= next.time_ms) {
      const span = Math.max(1, next.time_ms - previous.time_ms);
      const progress = (timeMS - previous.time_ms) / span;
      return {
        x: previous.x + (next.x - previous.x) * progress,
        y: previous.y + (next.y - previous.y) * progress,
      };
    }
  }
  return { x: events.at(-1).x, y: events.at(-1).y };
}

function expectedRing(frameIndex) {
  return frameIndex === 21 || frameIndex === 50 || frameIndex === 79;
}

const browser = await chromium.launch({ headless: true });
try {
  const context = await browser.newContext({ viewport: { width: 1600, height: 1100 }, deviceScaleFactor: 1 });
  const page = await context.newPage();
  await page.goto(args.url, { waitUntil: 'networkidle' });
  const authToken = await page.evaluate(() => window.localStorage.getItem('omnillm_auth_token'));
  const headers = authToken ? { Authorization: `Bearer ${authToken}` } : {};
  const apiURL = (pathname) => new URL(`/v1${pathname}`, args.url).toString();
  const jsonRequest = async (method, pathname, data) => {
    const response = await context.request.fetch(apiURL(pathname), { method, data, headers });
    if (!response.ok()) throw new Error(`${method} ${pathname}: HTTP ${response.status()} ${await response.text()}`);
    return response.json();
  };

  const project = await jsonRequest('POST', '/video/projects', {
    title: 'Canonical Cursor Parity',
    width: fixture.timeline.canvas.width,
    height: fixture.timeline.canvas.height,
    fps: fixture.timeline.canvas.fps,
    aspect_ratio: `${fixture.timeline.canvas.width}:${fixture.timeline.canvas.height}`,
  });

  const upload = await context.request.post(apiURL(`/video/projects/${encodeURIComponent(project.id)}/assets/upload`), {
    headers,
    multipart: { file: { name: 'cursor-backdrop.png', mimeType: 'image/png', buffer: imageBytes } },
  });
  if (!upload.ok()) throw new Error(`image upload: HTTP ${upload.status()} ${await upload.text()}`);
  const imageAsset = await upload.json();
  if (!imageAsset?.id) throw new Error(`image upload returned no asset id: ${JSON.stringify(imageAsset)}`);

  const imageDownload = await context.request.get(apiURL(`/video/assets/${encodeURIComponent(imageAsset.id)}/download`), { headers });
  if (!imageDownload.ok()) throw new Error(`image download: HTTP ${imageDownload.status()} ${await imageDownload.text()}`);
  const downloadedImageBytes = await imageDownload.body();
  const downloadedImageSHA256 = createHash('sha256').update(downloadedImageBytes).digest('hex');
  if (downloadedImageSHA256 !== sourceImageSHA256 || downloadedImageBytes.length !== imageBytes.length) {
    throw new Error(`uploaded cursor backdrop bytes changed: source=${sourceImageSHA256}/${imageBytes.length} downloaded=${downloadedImageSHA256}/${downloadedImageBytes.length}`);
  }

  const timeline = structuredClone(fixture.timeline);
  timeline.tracks[0].clips[0].asset_id = imageAsset.id;
  const saved = await jsonRequest('PUT', `/video/projects/${encodeURIComponent(project.id)}/timeline`, timeline);
  const render = await jsonRequest('POST', `/video/projects/${encodeURIComponent(project.id)}/render`, {
    format: 'mp4', codec: 'h264', resolution: 'project', fps: fixture.timeline.canvas.fps,
    quality: 'high', include_audio: false, burn_in_captions: true, strict_parity: false,
    timeline_id: saved.timeline.id,
    timeline_revision: saved.timeline.revision,
    timeline_sha256: saved.timeline.content_sha256,
  });
  if (!render.snapshot_id) throw new Error(`render submission returned no immutable snapshot: ${JSON.stringify(render)}`);
  await jsonRequest('POST', `/video/render-jobs/${encodeURIComponent(render.id)}/cancel`, {});

  const seedResult = {
    project_id: project.id,
    timeline_id: saved.timeline.id,
    timeline_revision: saved.timeline.revision,
    timeline_sha256: saved.timeline.content_sha256,
    render_job_id: render.id,
    snapshot_id: render.snapshot_id,
    image_asset_id: imageAsset.id,
    image_source_sha256: sourceImageSHA256,
    image_downloaded_sha256: downloadedImageSHA256,
    image_byte_length: imageBytes.length,
  };
  await fs.writeFile(path.join(output, 'seed-result.json'), `${JSON.stringify(seedResult, null, 2)}\n`);

  const editorURL = new URL(`/video/${encodeURIComponent(project.id)}/edit`, args.url).toString();
  await page.goto(editorURL, { waitUntil: 'networkidle' });
  const program = page.getByTestId('video-preview-program');
  await program.waitFor({ state: 'visible' });
  const canvas = fixture.timeline.canvas;
  await program.evaluate((element, dimensions) => {
    const fit = element.parentElement;
    if (!fit) throw new Error('preview program has no fit container');
    fit.style.setProperty('width', `${dimensions.width}px`, 'important');
    fit.style.setProperty('height', `${dimensions.height}px`, 'important');
    fit.style.setProperty('min-height', `${dimensions.height}px`, 'important');
    fit.style.setProperty('flex', '0 0 auto', 'important');
    fit.style.setProperty('padding', '0', 'important');
    element.style.setProperty('border', '0', 'important');
    element.style.setProperty('flex', '0 0 auto', 'important');
  }, { width: canvas.width, height: canvas.height });
  await page.waitForFunction(({ width, height }) => {
    const rect = document.querySelector('[data-testid="video-preview-program"]')?.getBoundingClientRect();
    return Boolean(rect && Math.abs(rect.width - width) < 0.01 && Math.abs(rect.height - height) < 0.01);
  }, { width: canvas.width, height: canvas.height });

  const frameEvidence = [];
  for (const sample of fixture.samples) {
    const requestId = `capture-${String(sample.frame_index).padStart(6, '0')}-${sample.name}`;
    await page.evaluate(({ frameIndex, id }) => new Promise((resolve, reject) => {
      const timeout = window.setTimeout(() => {
        window.removeEventListener('omnillm:video-parity-ready', ready);
        reject(new Error(`parity seek ${frameIndex} timed out`));
      }, 60_000);
      const ready = (event) => {
        if (event.detail?.requestId !== id) return;
        window.clearTimeout(timeout);
        window.removeEventListener('omnillm:video-parity-ready', ready);
        resolve();
      };
      window.addEventListener('omnillm:video-parity-ready', ready);
      window.dispatchEvent(new CustomEvent('omnillm:video-parity-seek', { detail: { frameIndex, requestId: id } }));
    }), { frameIndex: sample.frame_index, id: requestId });
    await page.waitForFunction((frameIndex) => document.querySelector('[data-testid="video-preview-program"]')?.dataset.parityFrameIndex === String(frameIndex), sample.frame_index);

    const cursorNodes = page.locator('[data-preview-canonical-cursor="true"]');
    if (await cursorNodes.count() !== 1) throw new Error(`frame ${sample.frame_index}: expected one canonical cursor, found ${await cursorNodes.count()}`);
    const cursorEvidence = await cursorNodes.first().evaluate((node) => {
      const rootStyle = getComputedStyle(node);
      const svg = node.querySelector('svg');
      const svgRect = svg?.getBoundingClientRect();
      const divs = Array.from(node.querySelectorAll(':scope > div')).map((element) => {
        const style = getComputedStyle(element);
        return {
          background_color: style.backgroundColor,
          border_top_color: style.borderTopColor,
          border_top_width: style.borderTopWidth,
          width: style.width,
          height: style.height,
          left: style.left,
          top: style.top,
        };
      });
      return {
        state_mode: node.dataset.previewCursorStateMode || '',
        left: Number.parseFloat(rootStyle.left),
        top: Number.parseFloat(rootStyle.top),
        svg_width: svgRect?.width ?? 0,
        svg_height: svgRect?.height ?? 0,
        child_divs: divs,
      };
    });
    const expected = expectedCursorAt(sample.time_ms);
    const ringExpected = expectedRing(sample.frame_index);
    const highlight = cursorEvidence.child_divs.find((item) =>
      item.border_top_width === '0px'
      && Math.abs(Number.parseFloat(item.width) - cursorEvidence.svg_width * 2.2) < 0.05
      && Math.abs(Number.parseFloat(item.height) - cursorEvidence.svg_height * 2.2) < 0.05);
    const ring = cursorEvidence.child_divs.find((item) =>
      item.border_top_width === '2px'
      && Math.abs(Number.parseFloat(item.width) - cursorEvidence.svg_width * 2.6) < 0.05
      && Math.abs(Number.parseFloat(item.height) - cursorEvidence.svg_height * 2.6) < 0.05);
    if (cursorEvidence.state_mode !== 'canonical-frame') throw new Error(`frame ${sample.frame_index}: cursor did not use canonical-frame state: ${JSON.stringify(cursorEvidence)}`);
    if (Math.abs(cursorEvidence.left - expected.x) > 0.02 || Math.abs(cursorEvidence.top - expected.y) > 0.02) {
      throw new Error(`frame ${sample.frame_index}: cursor interpolation drifted expected=${JSON.stringify(expected)} actual=${JSON.stringify(cursorEvidence)}`);
    }
    if (Math.abs(cursorEvidence.svg_width - 64) > 0.01 || Math.abs(cursorEvidence.svg_height - 64) > 0.01) {
      throw new Error(`frame ${sample.frame_index}: pointer size drifted: ${JSON.stringify(cursorEvidence)}`);
    }
    if (!highlight) throw new Error(`frame ${sample.frame_index}: canonical highlight missing: ${JSON.stringify(cursorEvidence)}`);
    if (Boolean(ring) !== ringExpected) {
      throw new Error(`frame ${sample.frame_index}: strict <300ms ring state drifted expected=${ringExpected}: ${JSON.stringify(cursorEvidence)}`);
    }

    const box = await program.boundingBox();
    if (!box || Math.abs(box.width - canvas.width) >= 0.01 || Math.abs(box.height - canvas.height) >= 0.01) {
      throw new Error(`preview program does not match fixture dimensions: ${JSON.stringify(box)}`);
    }
    const previewPNG = await program.screenshot({ type: 'png', animations: 'disabled', caret: 'hide' });
    if (previewPNG.readUInt32BE(16) !== canvas.width || previewPNG.readUInt32BE(20) !== canvas.height) {
      throw new Error('preview PNG dimensions do not match fixture canvas');
    }
    const safeName = `${String(sample.frame_index).padStart(6, '0')}-${sample.name}`;
    await fs.writeFile(path.join(previewDir, `${safeName}.png`), previewPNG);

    const frameEndpoint = apiURL(`/video/render-snapshots/${encodeURIComponent(render.snapshot_id)}/frames/${sample.frame_index}`);
    const rendered = await context.request.get(frameEndpoint, { headers });
    if (!rendered.ok()) throw new Error(`frame ${sample.frame_index} diagnostic render: HTTP ${rendered.status()} ${await rendered.text()}`);
    const renderedPNG = await rendered.body();
    if (renderedPNG.readUInt32BE(16) !== canvas.width || renderedPNG.readUInt32BE(20) !== canvas.height) {
      throw new Error(`frame ${sample.frame_index}: rendered PNG dimensions do not match fixture canvas`);
    }
    await fs.writeFile(path.join(renderedDir, `${safeName}.png`), renderedPNG);
    frameEvidence.push({
      frame_index: sample.frame_index,
      time_ms: sample.time_ms,
      request_id: requestId,
      expected_position: expected,
      expected_ring: ringExpected,
      ...cursorEvidence,
      highlight_present: Boolean(highlight),
      ring_present: Boolean(ring),
    });
  }

  await fs.writeFile(path.join(output, 'cursor-frame-evidence.json'), `${JSON.stringify({ fixture: fixture.name, frames: frameEvidence }, null, 2)}\n`);
  console.log(JSON.stringify({ output, seedResult, frames: frameEvidence }));
} finally {
  await browser.close();
}
