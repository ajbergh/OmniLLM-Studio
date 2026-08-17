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

if (!args.url || !args.fixture || (!args.snapshot && !args['media-dir'])) {
  console.error('usage: node scripts/video-parity-capture.mjs --url <app-url> --fixture <generated-json> (--snapshot <id> | --media-dir <generated-media-dir>) [--output <dir>] [--storage-state <json>]');
  process.exit(1);
}

const output = path.resolve(args.output || 'output/video-parity-capture');
const previewDir = path.join(output, 'preview');
const renderedDir = path.join(output, 'rendered');
await fs.mkdir(previewDir, { recursive: true });
await fs.mkdir(renderedDir, { recursive: true });

const fixture = JSON.parse(await fs.readFile(path.resolve(args.fixture), 'utf8'));
const sampleLimit = args['sample-limit'] ? Number.parseInt(args['sample-limit'], 10) : fixture.samples.length;
if (!Number.isInteger(sampleLimit) || sampleLimit < 1) throw new Error('--sample-limit must be a positive integer');
const samples = fixture.samples.slice(0, sampleLimit);
const browser = await chromium.launch({ headless: true });
try {
  const context = await browser.newContext({
    viewport: { width: 1600, height: 1100 },
    deviceScaleFactor: 1,
    ...(args['storage-state'] ? { storageState: path.resolve(args['storage-state']) } : {}),
  });
  const page = await context.newPage();
  const cdp = await context.newCDPSession(page);
  await page.goto(args.url, { waitUntil: 'networkidle' });
  const authToken = await page.evaluate(() => window.localStorage.getItem('omnillm_auth_token'));
  const requestHeaders = authToken ? { Authorization: `Bearer ${authToken}` } : {};
  let snapshotID = args.snapshot;
  let editorURL = args.url;
  let seedResult = null;
  if (!snapshotID) {
    seedResult = await seedFixtureProject({ context, fixture, mediaDir: path.resolve(args['media-dir']), baseURL: args.url, headers: requestHeaders });
    snapshotID = seedResult.snapshot_id;
    editorURL = new URL(`/video/${encodeURIComponent(seedResult.project_id)}/edit`, args.url).toString();
    await fs.writeFile(path.join(output, 'seed-result.json'), `${JSON.stringify(seedResult, null, 2)}\n`);
    await page.goto(editorURL, { waitUntil: 'networkidle' });
  }
  const program = page.getByTestId('video-preview-program');
  await program.waitFor({ state: 'visible' });
  await page.addStyleTag({ content: '[data-testid="video-preview-program"]{border:0!important}' });

  for (const sample of samples) {
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

    const endpoint = new URL(`/v1/video/render-snapshots/${encodeURIComponent(snapshotID)}/frames/${sample.frame_index}`, args.url).toString();
    const response = await context.request.get(endpoint, { headers: requestHeaders });
    if (!response.ok()) throw new Error(`diagnostic frame ${sample.frame_index}: HTTP ${response.status()} ${await response.text()}`);
    await fs.writeFile(path.join(renderedDir, `${safeName}.png`), await response.body());
  }
  const audioEndpoint = new URL(`/v1/video/render-snapshots/${encodeURIComponent(snapshotID)}/audio.pcm`, args.url).toString();
  const audioResponse = await context.request.get(audioEndpoint, { headers: requestHeaders });
  let renderedAudio = null;
  if (audioResponse.ok()) {
    renderedAudio = path.join(output, 'rendered-audio-s16le-48k-stereo.pcm');
    await fs.writeFile(renderedAudio, await audioResponse.body());
  } else {
    throw new Error(`diagnostic audio: HTTP ${audioResponse.status()} ${await audioResponse.text()}`);
  }
  console.log(JSON.stringify({ output, previewDir, renderedDir, renderedAudio, snapshotID, editorURL, seedResult, samples: samples.length }));
} finally {
  await browser.close();
}

async function seedFixtureProject({ context, fixture, mediaDir, baseURL, headers }) {
  const apiURL = (pathname) => new URL(`/v1${pathname}`, baseURL).toString();
  const jsonRequest = async (method, pathname, data) => {
    const response = await context.request.fetch(apiURL(pathname), { method, data, headers });
    if (!response.ok()) throw new Error(`${method} ${pathname}: HTTP ${response.status()} ${await response.text()}`);
    return response.json();
  };

  const project = await jsonRequest('POST', '/video/projects', {
    title: 'Parity Torture Baseline',
    width: fixture.timeline.canvas.width,
    height: fixture.timeline.canvas.height,
    fps: fixture.timeline.canvas.fps,
    aspect_ratio: `${fixture.timeline.canvas.width}:${fixture.timeline.canvas.height}`,
  });
  const fileForAsset = {
    'asset-landscape': ['asset-landscape.mp4', 'video/mp4'],
    'asset-portrait': ['asset-portrait.mp4', 'video/mp4'],
    'asset-square': ['asset-square.png', 'image/png'],
    'asset-audio': ['asset-audio.wav', 'audio/wav'],
  };
  const assetIDs = {};
  for (const asset of fixture.assets) {
    const recipe = fileForAsset[asset.id];
    if (!recipe) throw new Error(`no generated media recipe for fixture asset ${asset.id}`);
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
      if (clip.asset_id) {
        const mapped = assetIDs[clip.asset_id];
        if (!mapped) throw new Error(`timeline references unmapped fixture asset ${clip.asset_id}`);
        clip.asset_id = mapped;
      }
    }
  }
  const saved = await jsonRequest('PUT', `/video/projects/${encodeURIComponent(project.id)}/timeline`, timeline);
  const render = await jsonRequest('POST', `/video/projects/${encodeURIComponent(project.id)}/render`, {
    format: 'mp4', codec: 'h264', resolution: 'project', fps: timeline.canvas.fps,
    quality: 'high', include_audio: true, burn_in_captions: true, strict_parity: false,
    timeline_id: saved.timeline.id,
    timeline_revision: saved.timeline.revision,
    timeline_sha256: saved.timeline.content_sha256,
  });
  if (!render.snapshot_id) throw new Error('render submission did not return an immutable snapshot id');
  await jsonRequest('POST', `/video/render-jobs/${encodeURIComponent(render.id)}/cancel`, {});
  return {
    project_id: project.id,
    timeline_id: saved.timeline.id,
    timeline_revision: saved.timeline.revision,
    timeline_sha256: saved.timeline.content_sha256,
    render_job_id: render.id,
    snapshot_id: render.snapshot_id,
    asset_ids: assetIDs,
  };
}
