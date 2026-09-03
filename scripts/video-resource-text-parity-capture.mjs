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

if (!args.url || !args.fixture || !args['font-file']) {
  console.error('usage: node scripts/video-resource-text-parity-capture.mjs --url <app-url> --fixture <json> --font-file <ttf> [--output <dir>]');
  process.exit(1);
}

const fixture = JSON.parse(await fs.readFile(path.resolve(args.fixture), 'utf8'));
if (fixture.name !== 'parity-resource-text-v1') throw new Error(`unexpected fixture ${fixture.name}`);
if (fixture.samples?.length !== 1 || fixture.samples[0].frame_index !== 15) {
  throw new Error(`focused fixture must contain only canonical frame 15: ${JSON.stringify(fixture.samples)}`);
}

const output = path.resolve(args.output || 'output/video-text-parity/capture');
const previewDir = path.join(output, 'preview');
const renderedDir = path.join(output, 'rendered');
await fs.mkdir(previewDir, { recursive: true });
await fs.mkdir(renderedDir, { recursive: true });

const fontPath = path.resolve(args['font-file']);
const fontBytes = await fs.readFile(fontPath);
const sourceFontSHA256 = createHash('sha256').update(fontBytes).digest('hex');
const fontResourceID = 'parity-text-face-v1';

const browser = await chromium.launch({ headless: true });
try {
  const context = await browser.newContext({
    viewport: { width: 1600, height: 1100 },
    deviceScaleFactor: 1,
  });
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
    title: 'Resource Text Parity',
    width: fixture.timeline.canvas.width,
    height: fixture.timeline.canvas.height,
    fps: fixture.timeline.canvas.fps,
    aspect_ratio: `${fixture.timeline.canvas.width}:${fixture.timeline.canvas.height}`,
  });

  const upload = await context.request.post(apiURL(`/video/projects/${encodeURIComponent(project.id)}/assets/upload`), {
    headers,
    multipart: {
      file: { name: 'asset-font.ttf', mimeType: 'font/ttf', buffer: fontBytes },
      font_resource_id: fontResourceID,
    },
  });
  if (!upload.ok()) throw new Error(`font upload: HTTP ${upload.status()} ${await upload.text()}`);
  const fontAsset = await upload.json();
  if (!fontAsset?.id) throw new Error(`font upload returned no asset id: ${JSON.stringify(fontAsset)}`);

  const fontDownload = await context.request.get(apiURL(`/video/assets/${encodeURIComponent(fontAsset.id)}/download`), { headers });
  if (!fontDownload.ok()) throw new Error(`font download: HTTP ${fontDownload.status()} ${await fontDownload.text()}`);
  const downloadedFontBytes = await fontDownload.body();
  const downloadedFontSHA256 = createHash('sha256').update(downloadedFontBytes).digest('hex');
  if (downloadedFontSHA256 !== sourceFontSHA256 || downloadedFontBytes.length !== fontBytes.length) {
    throw new Error(`uploaded font bytes changed: source=${sourceFontSHA256}/${fontBytes.length} downloaded=${downloadedFontSHA256}/${downloadedFontBytes.length}`);
  }

  const saved = await jsonRequest('PUT', `/video/projects/${encodeURIComponent(project.id)}/timeline`, structuredClone(fixture.timeline));
  const render = await jsonRequest('POST', `/video/projects/${encodeURIComponent(project.id)}/render`, {
    format: 'mp4',
    codec: 'h264',
    resolution: 'project',
    fps: fixture.timeline.canvas.fps,
    quality: 'high',
    include_audio: false,
    burn_in_captions: true,
    strict_parity: false,
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
    font_resource_id: fontResourceID,
    font_asset_id: fontAsset.id,
    font_source_sha256: sourceFontSHA256,
    font_downloaded_sha256: downloadedFontSHA256,
    font_byte_length: fontBytes.length,
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

  const textNodes = page.locator('[data-preview-canonical-content="text"]');
  if (await textNodes.count() !== 1) throw new Error(`expected exactly one canonical text node, found ${await textNodes.count()}`);
  const browserEvidence = await textNodes.first().evaluate((node) => {
    const style = getComputedStyle(node);
    const rect = node.getBoundingClientRect();
    return {
      text: node.textContent || '',
      state_mode: node.dataset.previewTextStateMode || '',
      font_face_source: node.dataset.previewTextFontFaceSource || '',
      font_face_runtime: node.dataset.previewTextFontFaceRuntime || '',
      metrics_mode: node.dataset.previewTextMetricsMode || '',
      font_family: style.fontFamily,
      font_size: style.fontSize,
      font_weight: style.fontWeight,
      color: style.color,
      rect: { x: rect.x, y: rect.y, width: rect.width, height: rect.height },
    };
  });
  const expectedAlias = `OmniLLMPreview_${fontResourceID}_${fontAsset.id}_400`;
  if (browserEvidence.text !== 'WYSIWYG 42' || browserEvidence.state_mode !== 'canonical-frame') {
    throw new Error(`browser did not consume canonical text state: ${JSON.stringify(browserEvidence)}`);
  }
  if (browserEvidence.font_face_source !== 'family-name-only' || browserEvidence.font_face_runtime !== 'editor-resource-loaded') {
    throw new Error(`browser resource runtime/provenance evidence drifted: ${JSON.stringify(browserEvidence)}`);
  }
  if (browserEvidence.font_family !== expectedAlias || browserEvidence.font_size !== '48px' || browserEvidence.font_weight !== '400') {
    throw new Error(`browser did not bind exact uploaded resource alias/style ${expectedAlias}: ${JSON.stringify(browserEvidence)}`);
  }
  await fs.writeFile(path.join(output, 'font-frame-evidence.json'), `${JSON.stringify({
    fixture: fixture.name,
    frame_index: sample.frame_index,
    request_id: requestId,
    expected_font_alias: expectedAlias,
    ...browserEvidence,
  }, null, 2)}\n`);

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
  if (!rendered.ok()) throw new Error(`diagnostic frame: HTTP ${rendered.status()} ${await rendered.text()}`);
  await fs.writeFile(path.join(renderedDir, `${safeName}.png`), await rendered.body());

  console.log(JSON.stringify({ output, previewDir, renderedDir, seedResult, browserEvidence }));
} finally {
  await browser.close();
}
