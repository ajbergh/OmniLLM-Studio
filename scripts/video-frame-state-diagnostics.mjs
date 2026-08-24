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
  console.error('usage: node scripts/video-frame-state-diagnostics.mjs --url <app-url> --fixture <fixture-json> --seed-result <seed-result-json> --output <preview-diagnostic-json> [--storage-state <json>] [--transition-free-control true] [--timeline-output <json>]');
  process.exit(1);
}

const fixture = JSON.parse(await fs.readFile(path.resolve(args.fixture), 'utf8'));
const seedResult = JSON.parse(await fs.readFile(path.resolve(args['seed-result']), 'utf8'));
if (!Array.isArray(fixture.samples) || fixture.samples.length === 0) throw new Error('fixture must contain at least one sample');
if (!seedResult.project_id || !seedResult.timeline_sha256 || !seedResult.snapshot_id) throw new Error('seed result is missing project/timeline/snapshot identity');

const outputPath = path.resolve(args.output);
await fs.mkdir(path.dirname(outputPath), { recursive: true });
const transitionFreeControl = args['transition-free-control'] === 'true';
const timelinePath = args['timeline-output']
  ? path.resolve(args['timeline-output'])
  : path.join(path.dirname(outputPath), transitionFreeControl ? 'timeline-v1-transition-free-control.json' : 'timeline-v1.json');

const browser = await chromium.launch({ headless: true });
try {
  const context = await browser.newContext({
    ...(args['storage-state'] ? { storageState: path.resolve(args['storage-state']) } : {}),
  });
  const page = await context.newPage();
  await page.goto(args.url, { waitUntil: 'networkidle' });
  const authToken = await page.evaluate(() => window.localStorage.getItem('omnillm_auth_token'));
  const headers = authToken ? { Authorization: `Bearer ${authToken}` } : {};
  const timelineURL = new URL(`/v1/video/projects/${encodeURIComponent(seedResult.project_id)}/timeline`, args.url).toString();
  const timelineResponse = await context.request.get(timelineURL, { headers });
  if (!timelineResponse.ok()) {
    throw new Error(`timeline fetch: HTTP ${timelineResponse.status()} ${await timelineResponse.text()}`);
  }
  const timelinePayload = await timelineResponse.json();
  const savedTimeline = timelinePayload.document;
  if (!savedTimeline || savedTimeline.version !== 1) throw new Error('timeline response did not contain a Timeline v1 document');

  // The real saved timeline remains the first diagnostic input. A second,
  // explicitly named control removes only transition records so the current
  // v1→v2 adapter can produce actual FrameState values. This does not weaken
  // production adapter semantics: it creates a diagnostic positive control
  // beside the real fail-closed result.
  const timeline = structuredClone(savedTimeline);
  if (transitionFreeControl) {
    for (const track of timeline.tracks || []) {
      for (const clip of track.clips || []) clip.transitions = [];
    }
  }
  await fs.mkdir(path.dirname(timelinePath), { recursive: true });
  await fs.writeFile(timelinePath, `${JSON.stringify(timeline, null, 2)}\n`);

  const diagnostics = await page.evaluate(async ({ document, samples }) => {
    const frameStateModule = await import('/src/video/renderContractFrameStateDiagnostics.ts');
    const previewCompositionModule = await import('/src/video/renderContractPreviewComposition.ts');
    return samples.map((sample) => {
      const diagnostic = frameStateModule.evaluateVisualFrameStateDiagnostic(document, sample.frame_index);
      const composition = previewCompositionModule.evaluateCanonicalPreviewCompositionFrame(document, [], sample.frame_index);
      if (composition.available !== diagnostic.available) {
        throw new Error(`preview composition availability drift at frame ${sample.frame_index}: FrameState=${diagnostic.available} preview=${composition.available}`);
      }
      const frameStateClipIDs = diagnostic.state?.layers?.map((layer) => layer.clip_id) || [];
      const previewClipIDs = composition.layers?.map((layer) => layer.clip.id) || [];
      if (composition.available && JSON.stringify(previewClipIDs) !== JSON.stringify(frameStateClipIDs)) {
        throw new Error(`preview composition clip identity/order drift at frame ${sample.frame_index}: FrameState=${JSON.stringify(frameStateClipIDs)} preview=${JSON.stringify(previewClipIDs)}`);
      }
      return {
        name: sample.name,
        frame_index: sample.frame_index,
        diagnostic,
        preview_composition: {
          contract_version: composition.contract_version,
          available: composition.available,
          clip_ids: previewClipIDs,
          ...(composition.error ? { error: composition.error } : {}),
        },
      };
    });
  }, { document: timeline, samples: fixture.samples });

  const output = {
    version: 2,
    source: 'browser-typescript',
    diagnostic_contract: 'visual-frame-state-diagnostic-v1',
    preview_composition_contract: 'preview-composition-frame-v1',
    mode: transitionFreeControl ? 'transition-free-control' : 'saved-timeline',
    timeline_sha256: seedResult.timeline_sha256,
    snapshot_id: seedResult.snapshot_id,
    timeline_path: timelinePath,
    samples: diagnostics,
  };
  await fs.writeFile(outputPath, `${JSON.stringify(output, null, 2)}\n`);
  console.log(JSON.stringify({ output: outputPath, timeline: timelinePath, mode: output.mode, samples: diagnostics.length }));
} finally {
  await browser.close();
}
