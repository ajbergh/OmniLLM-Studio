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

if (!args.url || !args.fixture) {
  console.error('usage: node scripts/video-rounded-rectangle-parity-capture.mjs --url <app-url> --fixture <json> [--output <dir>]');
  process.exit(1);
}

const fixture = JSON.parse(await fs.readFile(path.resolve(args.fixture), 'utf8'));
if (fixture.name !== 'parity-rounded-rectangle-v1') throw new Error(`unexpected fixture ${fixture.name}`);
if (fixture.timeline?.canvas?.width !== 640 || fixture.timeline?.canvas?.height !== 360 || fixture.timeline?.canvas?.fps !== 30) {
  throw new Error(`rounded rectangle fixture canvas drifted: ${JSON.stringify(fixture.timeline?.canvas)}`);
}
if (fixture.samples?.length !== 1 || fixture.samples[0]?.frame_index !== 15 || fixture.samples[0]?.time_ms !== 500) {
  throw new Error(`rounded rectangle fixture sample drifted: ${JSON.stringify(fixture.samples)}`);
}
const authoredShape = fixture.timeline?.tracks?.[0]?.clips?.[0]?.shape;
if (authoredShape?.kind !== 'rounded_rectangle' || authoredShape.width !== 240 || authoredShape.height !== 120 || authoredShape.corner_radius !== 24 || authoredShape.stroke_width !== 8) {
  throw new Error(`rounded rectangle authored state drifted: ${JSON.stringify(authoredShape)}`);
}

const output = path.resolve(args.output || 'output/video-rounded-rectangle-parity/capture');
const previewDir = path.join(output, 'preview');
const renderedDir = path.join(output, 'rendered');
await fs.mkdir(previewDir, { recursive: true });
await fs.mkdir(renderedDir, { recursive: true });

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
    title: 'Canonical Rounded Rectangle Parity',
    width: fixture.timeline.canvas.width,
    height: fixture.timeline.canvas.height,
    fps: fixture.timeline.canvas.fps,
    aspect_ratio: `${fixture.timeline.canvas.width}:${fixture.timeline.canvas.height}`,
  });
  const saved = await jsonRequest('PUT', `/video/projects/${encodeURIComponent(project.id)}/timeline`, structuredClone(fixture.timeline));
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

  const sample = fixture.samples[0];
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

  const shapeNodes = page.locator('[data-preview-canonical-content="shape"]');
  if (await shapeNodes.count() !== 1) throw new Error(`frame ${sample.frame_index}: expected one canonical shape, found ${await shapeNodes.count()}`);
  const shapeEvidence = await shapeNodes.first().evaluate((node) => {
    const rect = node.getBoundingClientRect();
    const child = node.querySelector(':scope > div');
    const style = child ? getComputedStyle(child) : null;
    return {
      state_mode: node.dataset.previewShapeStateMode || '',
      width: rect.width,
      height: rect.height,
      background_color: style?.backgroundColor || '',
      border_top_color: style?.borderTopColor || '',
      border_top_width: style?.borderTopWidth || '',
      border_top_left_radius: style?.borderTopLeftRadius || '',
    };
  });
  if (shapeEvidence.state_mode !== 'canonical-frame') throw new Error(`rounded rectangle did not use canonical-frame shape state: ${JSON.stringify(shapeEvidence)}`);
  if (Math.abs(shapeEvidence.width - 240) > 0.01 || Math.abs(shapeEvidence.height - 120) > 0.01) throw new Error(`rounded rectangle dimensions drifted: ${JSON.stringify(shapeEvidence)}`);
  if (shapeEvidence.border_top_width !== '8px' || shapeEvidence.border_top_left_radius !== '24px') throw new Error(`rounded rectangle border geometry drifted: ${JSON.stringify(shapeEvidence)}`);

  const box = await program.boundingBox();
  if (!box || Math.abs(box.width - canvas.width) >= 0.01 || Math.abs(box.height - canvas.height) >= 0.01) throw new Error(`preview program does not match fixture dimensions: ${JSON.stringify(box)}`);
  const previewPNG = await program.screenshot({ type: 'png', animations: 'disabled', caret: 'hide' });
  if (previewPNG.readUInt32BE(16) !== canvas.width || previewPNG.readUInt32BE(20) !== canvas.height) throw new Error('preview PNG dimensions do not match fixture canvas');
  const safeName = `${String(sample.frame_index).padStart(6, '0')}-${sample.name}`;
  await fs.writeFile(path.join(previewDir, `${safeName}.png`), previewPNG);

  const frameEndpoint = apiURL(`/video/render-snapshots/${encodeURIComponent(render.snapshot_id)}/frames/${sample.frame_index}`);
  const rendered = await context.request.get(frameEndpoint, { headers });
  if (!rendered.ok()) throw new Error(`frame ${sample.frame_index} diagnostic render: HTTP ${rendered.status()} ${await rendered.text()}`);
  const renderedPNG = await rendered.body();
  if (renderedPNG.readUInt32BE(16) !== canvas.width || renderedPNG.readUInt32BE(20) !== canvas.height) throw new Error('rendered PNG dimensions do not match fixture canvas');
  await fs.writeFile(path.join(renderedDir, `${safeName}.png`), renderedPNG);

  const evidence = { fixture: fixture.name, seed: seedResult, frame: { frame_index: sample.frame_index, time_ms: sample.time_ms, request_id: requestId, ...shapeEvidence } };
  await fs.writeFile(path.join(output, 'rounded-rectangle-frame-evidence.json'), `${JSON.stringify(evidence, null, 2)}\n`);
  console.log(JSON.stringify(evidence));
} finally {
  await browser.close();
}
