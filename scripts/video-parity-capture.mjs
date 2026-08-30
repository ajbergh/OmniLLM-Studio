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
  const canvasWidth = fixture.timeline.canvas.width;
  const canvasHeight = fixture.timeline.canvas.height;
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
    if (Math.abs(box.width - canvasWidth) >= 0.01 || Math.abs(box.height - canvasHeight) >= 0.01) {
      throw new Error(`preview program ${box.width}x${box.height} does not match fixture ${canvasWidth}x${canvasHeight}`);
    }
    const previewPNG = await program.screenshot({ type: 'png', animations: 'disabled', caret: 'hide' });
    const pngWidth = previewPNG.readUInt32BE(16);
    const pngHeight = previewPNG.readUInt32BE(20);
    if (pngWidth !== canvasWidth || pngHeight !== canvasHeight) {
      throw new Error(`preview PNG ${pngWidth}x${pngHeight} does not match fixture ${canvasWidth}x${canvasHeight}`);
    }
    await fs.writeFile(path.join(previewDir, `${safeName}.png`), previewPNG);

    const endpoint = new URL(`/v1/video/render-snapshots/${encodeURIComponent(snapshotID)}/frames/${sample.frame_index}`, args.url).toString();
    const response = await context.request.get(endpoint, { headers: requestHeaders });
    if (!response.ok()) throw new Error(`diagnostic frame ${sample.frame_index}: HTTP ${response.status()} ${await response.text()}`);
    await fs.writeFile(path.join(renderedDir, `${safeName}.png`), await response.body());
  }
  const previewAudioRequestID = `preview-audio-${snapshotID}`;
  const previewAudioResult = await page.evaluate((requestId) => new Promise((resolve, reject) => {
    const timeout = window.setTimeout(() => {
      window.removeEventListener('omnillm:video-parity-audio-ready', ready);
      reject(new Error('preview audio render timed out'));
    }, 60_000);
    const ready = (event) => {
      if (event.detail?.requestId !== requestId) return;
      window.clearTimeout(timeout);
      window.removeEventListener('omnillm:video-parity-audio-ready', ready);
      if (event.detail.error) reject(new Error(event.detail.error));
      else resolve(event.detail);
    };
    window.addEventListener('omnillm:video-parity-audio-ready', ready);
    window.dispatchEvent(new CustomEvent('omnillm:video-parity-audio-request', { detail: { requestId } }));
  }), previewAudioRequestID);
  const previewAudioBase64 = await page.evaluate(async (url) => {
    const bytes = new Uint8Array(await (await fetch(url)).arrayBuffer());
    const chunks = [];
    for (let offset = 0; offset < bytes.length; offset += 0x8000) {
      chunks.push(String.fromCharCode(...bytes.subarray(offset, offset + 0x8000)));
    }
    return btoa(chunks.join(''));
  }, previewAudioResult.url);
  const previewAudio = path.join(output, 'preview-audio-s16le-48k-stereo.pcm');
  await fs.writeFile(previewAudio, Buffer.from(previewAudioBase64, 'base64'));
  await fs.writeFile(path.join(output, 'preview-audio-metadata.json'), `${JSON.stringify(previewAudioResult.metadata, null, 2)}\n`);

  const audioEndpoint = new URL(`/v1/video/render-snapshots/${encodeURIComponent(snapshotID)}/audio.pcm`, args.url).toString();
  const audioResponse = await context.request.get(audioEndpoint, { headers: requestHeaders });
  let renderedAudio = null;
  if (audioResponse.ok()) {
    renderedAudio = path.join(output, 'rendered-audio-s16le-48k-stereo.pcm');
    await fs.writeFile(renderedAudio, await audioResponse.body());
  } else {
    throw new Error(`diagnostic audio: HTTP ${audioResponse.status()} ${await audioResponse.text()}`);
  }
  let deliveryResult = null;
  if (args['capture-delivery'] === 'true') {
    if (!seedResult) throw new Error('--capture-delivery true requires --media-dir project seeding');
    deliveryResult = await captureDelivery({ context, baseURL: args.url, headers: requestHeaders, seedResult, fixture, output });
    await fs.writeFile(path.join(output, 'delivery-result.json'), `${JSON.stringify(deliveryResult, null, 2)}\n`);
  }
  console.log(JSON.stringify({ output, previewDir, renderedDir, previewAudio, renderedAudio, deliveryResult, snapshotID, editorURL, seedResult, samples: samples.length }));
} finally {
  await browser.close();
}

async function captureDelivery({ context, baseURL, headers, seedResult, fixture, output }) {
  const apiURL = (pathname) => new URL(`/v1${pathname}`, baseURL).toString();
  const response = await context.request.post(apiURL(`/video/projects/${encodeURIComponent(seedResult.project_id)}/render`), {
    headers,
    data: {
      format: 'mp4', codec: 'h264', resolution: 'project', fps: fixture.timeline.canvas.fps,
      quality: 'high', include_audio: true, burn_in_captions: true, strict_parity: false,
      timeline_id: seedResult.timeline_id,
      timeline_revision: seedResult.timeline_revision,
      timeline_sha256: seedResult.timeline_sha256,
    },
  });
  if (!response.ok()) throw new Error(`delivery submission: HTTP ${response.status()} ${await response.text()}`);
  let job = await response.json();
  const deadline = Date.now() + 10 * 60_000;
  while (job.status === 'queued' || job.status === 'running') {
    if (Date.now() >= deadline) throw new Error(`delivery job ${job.id} timed out after 10 minutes`);
    await new Promise((resolve) => setTimeout(resolve, 1000));
    const poll = await context.request.get(apiURL(`/video/render-jobs/${encodeURIComponent(job.id)}`), { headers });
    if (!poll.ok()) throw new Error(`delivery poll: HTTP ${poll.status()} ${await poll.text()}`);
    job = await poll.json();
  }
  if (job.status !== 'completed' || !job.output_asset_id) {
    throw new Error(`delivery job ${job.id} ended ${job.status}: ${job.error || 'no output asset'}`);
  }
  const download = await context.request.get(apiURL(`/video/assets/${encodeURIComponent(job.output_asset_id)}/download`), { headers });
  if (!download.ok()) throw new Error(`delivery download: HTTP ${download.status()} ${await download.text()}`);
  const deliveryPath = path.join(output, 'delivery.mp4');
  await fs.writeFile(deliveryPath, await download.body());
  return {
    path: deliveryPath,
    job_id: job.id,
    snapshot_id: job.snapshot_id,
    output_asset_id: job.output_asset_id,
    timeline_sha256: job.timeline_sha256,
  };
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
    'asset-alpha': ['asset-alpha.png', 'image/png'],
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
