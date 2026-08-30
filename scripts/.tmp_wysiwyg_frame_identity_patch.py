from pathlib import Path

PR_NUMBER = "__PR_NUMBER__"
MERGE_SHA = "7d5f36c3230d23eef46de310d6ac785b1b998c33"


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text(encoding="utf-8")
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected exactly one match, found {count}: {old[:120]!r}")
    p.write_text(text.replace(old, new, 1), encoding="utf-8")


def replace_count(path: str, old: str, new: str, expected: int) -> None:
    p = Path(path)
    text = p.read_text(encoding="utf-8")
    count = text.count(old)
    if count != expected:
        raise SystemExit(f"{path}: expected {expected} matches, found {count}: {old[:120]!r}")
    p.write_text(text.replace(old, new), encoding="utf-8")


# Browser seek target: keep canonical source time exact, but ask HTMLVideoElement
# for a point just inside the requested frame interval so a floating-point value
# infinitesimally below the rational PTS cannot select the previous decoded frame.
source_timing = "frontend/src/components/video/sourceTiming.ts"
replace_once(
    source_timing,
    """/**\n * Deterministic frame capture needs a materially tighter seek tolerance than\n * interactive scrub/playback. Otherwise adjacent high-FPS frames can reuse a\n * stale media frame because the legacy 50 ms paused-scrub tolerance is wider\n * than an entire output frame.\n */\nexport function mediaSeekToleranceSeconds(address: TimelineSourceAddress): number {\n  return address.kind === 'frame' ? 0.0005 : 0.05;\n}\n""",
    """/**\n * Convert an exact canonical source-time boundary into a browser video seek\n * target that is safely inside the requested frame interval. JavaScript's\n * nearest Float64 representation of a rational PTS can sit infinitesimally\n * below that boundary (for example 59/30), which lets Chromium display the\n * previous decoded frame even though canonical frame identity is correct.\n *\n * The nudge is browser-consumer policy only: it never changes canonical source\n * time, never applies to free-running playback/audio, stays below the strict\n * deterministic seek tolerance, and remains far inside one output-frame span.\n */\nexport function deterministicVideoSeekTargetSeconds(\n  address: TimelineSourceAddress,\n  canonicalTargetSeconds: number,\n): number {\n  if (address.kind !== 'frame' || !Number.isFinite(canonicalTargetSeconds) || canonicalTargetSeconds < 0) {\n    return canonicalTargetSeconds;\n  }\n  const fps = Math.max(1, Math.trunc(address.fps));\n  const nudgeSeconds = Math.min(0.00025, 1 / (fps * 16));\n  return canonicalTargetSeconds + nudgeSeconds;\n}\n\n/**\n * Deterministic frame capture needs a materially tighter seek tolerance than\n * interactive scrub/playback. Otherwise adjacent high-FPS frames can reuse a\n * stale media frame because the legacy 50 ms paused-scrub tolerance is wider\n * than an entire output frame.\n */\nexport function mediaSeekToleranceSeconds(address: TimelineSourceAddress): number {\n  return address.kind === 'frame' ? 0.0005 : 0.05;\n}\n""",
)

source_timing_test = "frontend/src/components/video/sourceTiming.test.ts"
replace_once(
    source_timing_test,
    """  frameAddressMatchesTimelineMs,\n  mediaSeekToleranceSeconds,\n""",
    """  deterministicVideoSeekTargetSeconds,\n  frameAddressMatchesTimelineMs,\n  mediaSeekToleranceSeconds,\n""",
)
replace_once(
    source_timing_test,
    """  it('uses a sub-millisecond deterministic seek tolerance', () => {\n    expect(mediaSeekToleranceSeconds({ kind: 'frame', frameIndex: 1, fps: 120 })).toBeLessThan(1 / 120);\n    expect(mediaSeekToleranceSeconds({ kind: 'time', timelineMs: 8 })).toBe(0.05);\n  });\n""",
    """  it('nudges deterministic video seeks just inside an exact rational frame boundary', () => {\n    const address = { kind: 'frame' as const, frameIndex: 59, fps: 30 };\n    const canonicalTarget = 59 / 30;\n    const seekTarget = deterministicVideoSeekTargetSeconds(address, canonicalTarget);\n\n    expect(seekTarget).toBeGreaterThan(canonicalTarget);\n    expect(seekTarget - canonicalTarget).toBeLessThan(mediaSeekToleranceSeconds(address));\n    expect(seekTarget).toBeLessThan(canonicalTarget + 1 / address.fps);\n  });\n\n  it('does not nudge free-running or invalid video seek targets', () => {\n    expect(deterministicVideoSeekTargetSeconds({ kind: 'time', timelineMs: 1000 }, 1)).toBe(1);\n    expect(deterministicVideoSeekTargetSeconds({ kind: 'frame', frameIndex: 0, fps: 30 }, -1)).toBe(-1);\n  });\n\n  it('uses a sub-millisecond deterministic seek tolerance', () => {\n    expect(mediaSeekToleranceSeconds({ kind: 'frame', frameIndex: 1, fps: 120 })).toBeLessThan(1 / 120);\n    expect(mediaSeekToleranceSeconds({ kind: 'time', timelineMs: 8 })).toBe(0.05);\n  });\n""",
)

preview = "frontend/src/components/video/VideoPreviewCanvasLegacy.tsx"
replace_once(
    preview,
    """import { frameAddressMatchesTimelineMs, mediaSeekToleranceSeconds, sourceTimeForPreviewMediaMs } from './sourceTiming';\n""",
    """import { deterministicVideoSeekTargetSeconds, frameAddressMatchesTimelineMs, mediaSeekToleranceSeconds, sourceTimeForPreviewMediaMs } from './sourceTiming';\n""",
)
replace_once(
    preview,
    """      } else {\n        if (!element.paused) element.pause();\n        if (Math.abs(element.currentTime - target) > mediaSeekToleranceSeconds(address)) {\n          element.currentTime = target;\n        }\n      }\n""",
    """      } else {\n        if (!element.paused) element.pause();\n        if (Math.abs(element.currentTime - target) > mediaSeekToleranceSeconds(address)) {\n          element.currentTime = element instanceof HTMLVideoElement\n            ? deterministicVideoSeekTargetSeconds(address, target)\n            : target;\n        }\n      }\n""",
)

# The focused assertion now observes the decoded frame that Chromium actually
# submits to the compositor. Non-zero samples require requestVideoFrameCallback
# evidence; frame zero may use an explicit initial-current-time fallback because
# no new seek is required from the initial frame.
assert_script = Path("scripts/video-pixelate-parity-assert.mjs")
assert_script.write_text(r'''#!/usr/bin/env node
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
''', encoding="utf-8")

workflow = ".github/workflows/video-pixelate-parity.yml"
replace_count(
    workflow,
    """      - 'frontend/src/components/video/previewPixelateRaster.ts'\n      - 'scripts/video-pixelate-parity-assert.mjs'\n""",
    """      - 'frontend/src/components/video/previewPixelateRaster.ts'\n      - 'frontend/src/components/video/sourceTiming.ts'\n      - 'frontend/src/components/video/sourceTiming.test.ts'\n      - 'frontend/src/components/video/VideoPreviewCanvasLegacy.tsx'\n      - 'scripts/video-pixelate-parity-assert.mjs'\n""",
    2,
)
replace_once(
    workflow,
    """          const summary = {\n            schema_version: 1,\n            fixture: fixture.name,\n            samples: fixture.samples.length,\n            whole_frame_pass: whole.pass,\n            pixelate_region_pass: region.pass,\n            thresholds: region.thresholds,\n            frames: region.regions.map((entry) => ({\n              frame_index: entry.frame_index,\n              pixel_pass_rate: entry.metric.pixel_pass_rate,\n              ssim: entry.metric.ssim,\n              mean_absolute_error: entry.metric.mean_absolute_error,\n              root_mean_square_error: entry.metric.root_mean_square_error,\n              max_channel_delta: entry.metric.max_channel_delta,\n              pass: entry.metric.pass,\n            })),\n          };\n          fs.writeFileSync('output/video-pixelate-decoded/capture/pixelate-decoded-evidence-summary.json', `${JSON.stringify(summary, null, 2)}\\n`);\n          console.log(JSON.stringify(summary));\n""",
    """          const frameIdentity = consumer.frames.map((frame) => {\n            const presentations = frame.presentations || [];\n            const pass = presentations.length > 0\n              && presentations.every((presentation) => presentation.presented_frame_index === frame.frame_index)\n              && (frame.frame_index === 0 || presentations.every((presentation) => presentation.element_current_time_seconds > presentation.requested_media_time_seconds));\n            return { frame_index: frame.frame_index, pass, presentations };\n          });\n          const frameIdentityPass = frameIdentity.every((frame) => frame.pass);\n          const codecColorFrames = region.regions.map((entry) => ({\n            frame_index: entry.frame_index,\n            max_channel_delta: entry.metric.max_channel_delta,\n            pixel_pass_rate_at_repository_default: entry.metric.pixel_pass_rate,\n            ssim: entry.metric.ssim,\n            mean_absolute_error: entry.metric.mean_absolute_error,\n            pass: entry.metric.max_channel_delta <= 3,\n          }));\n          const codecColorGatePass = codecColorFrames.every((frame) => frame.pass);\n          const summary = {\n            schema_version: 2,\n            fixture: fixture.name,\n            samples: fixture.samples.length,\n            whole_frame_pass_at_repository_default: whole.pass,\n            pixelate_region_pass_at_repository_default: region.pass,\n            repository_default_thresholds: region.thresholds,\n            frame_identity_pass: frameIdentityPass,\n            codec_color_channel_tolerance: 3,\n            codec_color_gate_pass: codecColorGatePass,\n            frame_identity: frameIdentity,\n            frames: codecColorFrames,\n          };\n          fs.writeFileSync('output/video-pixelate-decoded/capture/pixelate-decoded-evidence-summary.json', `${JSON.stringify(summary, null, 2)}\\n`);\n          console.log(JSON.stringify(summary));\n          if (!frameIdentityPass) throw new Error('decoded-video presented-frame identity gate failed');\n          if (!codecColorGatePass) throw new Error('decoded-video ±3 RGB channel gate failed');\n""",
)

plan = "docs/VIDEO_EDIT_STUDIO_WYSIWYG_RENDERING_IMPLEMENTATION_PLAN_2026-08.md"
replace_once(
    plan,
    """Latest merged WYSIWYG program PR: **#289 — Retain opaque pixelate preview-render evidence** — squash merge `86dc6a6eaa4eb5f494f8d935f5535da8fb517b0b` (2026-08-29).\n\nCurrent implementation PR: **#290 — Measure codec-decoded pixelate preview-render parity** on branch `test/video-wysiwyg-phase3-pixelate-codec-parity-evidence-pr`, created directly from #289's actual squash result `86dc6a6eaa4eb5f494f8d935f5535da8fb517b0b`.\n""",
    f"""Latest merged WYSIWYG program PR: **#291 — Merge validated codec-decoded pixelate parity evidence** — squash merge `{MERGE_SHA}` (2026-08-29). #291 contains the exact validated head from draft #290; #290 was closed only because the connector's ready-for-review mutation failed against GitHub's current GraphQL schema.\n\nCurrent implementation PR: **#{PR_NUMBER} — Prove deterministic decoded-video frame identity** on branch `fix/video-wysiwyg-phase3-decoded-frame-identity`, created directly from #291's actual squash result `{MERGE_SHA}`.\n""",
)
replace_once(plan, "### #290 current scope and evidence\n\n#290 adds codec-decoded video evidence without weakening #289's byte-exact opaque-PNG control:\n", "### #291 merged codec-decoded evidence\n\n#291 merged the exact validated #290 head and adds codec-decoded video evidence without weakening #289's byte-exact opaque-PNG control:\n")
replace_once(
    plan,
    """- Exact-head Video Pixelate Parity Evidence #35 succeeded on head `dc772c079be9749eae75a15d0aade73785ff5f52`. All four H.264 samples independently reported `plan_mode=canonical-ready`, `consumer=canonical-canvas`, `surface_status=ready`, no structural/runtime deferral, and no CSS fallback marker.\n- Retained decoded-video artifact `9725313935` is 14,094,593 bytes with SHA-256 `b06d60b20780dcc704845121655e72fa9f5dfd11d9fabba0a5636ad90888f798`. Timeline SHA-256 is `fbc96eb288c716b9862127fb484d7ea0f05a19122dac18096c364644afe36eea`.\n""",
    """- Tracker-bearing exact head `0edcce0018e7afe68b304b0c11c0c8cfd650400f` passed the complete PR workflow set: Quality #1704, Security #1709, Video Pixelate Parity Evidence #36, the full Chromium smoke suite, backend tests/race detector, frontend lint/unit/build, and all platform/sandbox assurance workflows.\n- Evidence #36 retained decoded-video artifact `9725388648` (14,094,549 bytes, SHA-256 `69de2792b4958357732a817aae9daa628d6e89cfac55bd83b81b1aa2a443df03`) and the unchanged exact-PNG artifact `9725388161` (8,632,693 bytes, SHA-256 `72c42eff1842e38fad7b06cb7745b7468f099a0c5ca099aee3765d84e3055274`). Timeline SHA-256 remains `fbc96eb288c716b9862127fb484d7ea0f05a19122dac18096c364644afe36eea`.\n""",
)
replace_once(
    plan,
    """- Before #290 merges, the tracker-bearing exact head must again pass the complete PR workflow set; only actually executed checks on that exact head count as final validation.\n\n## Phase tracker\n""",
    f"""- #291 squash-merged that exact validated head as `{MERGE_SHA}`.\n\n### #{PR_NUMBER} current scope\n\n#{PR_NUMBER} closes the decoded-video frame-selection debt before promoting the measured codec/color envelope into a gate:\n\n- Keep canonical `source_time_ms` unchanged. For paused deterministic `<video>` seeks only, request a point just inside the rational frame boundary so a Float64 value infinitesimally below the source PTS cannot select the preceding decoded frame. Audio and free-running playback remain untouched.\n- Keep the nudge below the existing 0.5 ms deterministic seek tolerance and well inside one output-frame interval.\n- Extend the focused browser evidence with `requestVideoFrameCallback` presentation timestamps so Chromium's submitted decoded frame, not only `currentTime`/`seeked`, is retained.\n- Require presented frame identity to match canonical frames `0`, `15`, `30`, and `59`; non-zero samples must also prove the media element sought past the exact boundary rather than landing infinitesimally below it.\n- Only after frame identity passes, enforce the measured H.264/yuv420p codec/color envelope as an explicit `max_channel_delta <= 3` pixelate-region gate. Repository-global ±2/99.9% defaults and #289's byte-exact PNG gate remain unchanged.\n- Preserve the original 103-sample longitudinal baseline and transparent/premultiplied-alpha deferral.\n\n## Phase tracker\n""",
)
replace_once(
    plan,
    "| Phase 0 — Reproducible parity baseline | **In progress** | Deterministic 103-frame visual/audio/delivery evidence exists. #266 merged fail-closed frame-indexed region-policy inputs. #289 added a separate byte-exact retained opaque-PNG pixelate gate without changing the longitudinal torture baseline. #290 adds retained H.264 decoded-video evidence and isolates aligned codec/color variation from a final-frame browser seek-boundary mismatch. Freeze explicit codec/color policy after deterministic decoded-frame identity is closed; resource-font fixture coverage and second-platform retained evidence remain. |",
    f"| Phase 0 — Reproducible parity baseline | **In progress** | Deterministic 103-frame visual/audio/delivery evidence exists. #266 merged fail-closed frame-indexed region-policy inputs. #289 added the byte-exact opaque-PNG gate; #291 merged retained H.264 decoded-video evidence and isolated codec/color variation from the final-frame browser seek-boundary mismatch. #{PR_NUMBER} now closes decoded-frame identity and, only after that proof, promotes the measured ±3 channel envelope into a focused codec/color gate. Resource-font fixture coverage and second-platform retained evidence remain. |",
)
replace_once(
    plan,
    "| Phase 3 — Shared preview composition | **In progress** | #261–#289 merged deterministic activity/source/transform/view/perspective/media geometry/effects/transitions, canonical text/shape/cursor painters, exact browser font readiness, Chromium text-layout snapshots, deterministic pixelate raster/backdrop semantics, runtime-proven opaque Canvas consumption, byte-exact opaque-PNG evidence, and sampled-render source fan-out. #290 extends evidence to decoded H.264 while preserving fail-closed alpha behavior. Deterministic decoded-video frame identity, transparent-alpha semantics, weighted-raster broadening, normal-playback canonicalization, diagnostics/rollback, and audio consumption remain. |",
    f"| Phase 3 — Shared preview composition | **In progress** | #261–#289 merged deterministic activity/source/transform/view/perspective/media geometry/effects/transitions, canonical text/shape/cursor painters, exact browser font readiness, Chromium text-layout snapshots, deterministic pixelate raster/backdrop semantics, runtime-proven opaque Canvas consumption, byte-exact opaque-PNG evidence, and sampled-render source fan-out. #291 extends retained evidence to decoded H.264 while preserving fail-closed alpha behavior; #{PR_NUMBER} addresses deterministic decoded-video frame identity. Transparent-alpha semantics, weighted-raster broadening, normal-playback canonicalization, diagnostics/rollback, and audio consumption remain. |",
)
replace_once(
    plan,
    "- `shape-state-v1` owns shape geometry/style. #285 defines `preview-pixelate-raster-v1`; #286 defines fail-closed backdrop admission; #287 aligns both FFmpeg scale passes; #288 consumes only decoded opaque browser regions that pass structural/runtime proof; #289 proves byte-exact retained output for the isolated opaque-PNG static path; #290 extends retained evidence to decoded H.264 without weakening alpha or PNG exactness.",
    f"- `shape-state-v1` owns shape geometry/style. #285 defines `preview-pixelate-raster-v1`; #286 defines fail-closed backdrop admission; #287 aligns both FFmpeg scale passes; #288 consumes only decoded opaque browser regions that pass structural/runtime proof; #289 proves byte-exact retained output for the isolated opaque-PNG static path; #291 extends retained evidence to decoded H.264 without weakening alpha or PNG exactness; #{PR_NUMBER} proves decoded frame identity before freezing the codec/color gate.",
)
replace_once(
    plan,
    "| #289 | Retained byte-exact opaque-PNG pixelate evidence and immutable-source fan-out | `86dc6a6eaa4eb5f494f8d935f5535da8fb517b0b` |\n",
    f"| #289 | Retained byte-exact opaque-PNG pixelate evidence and immutable-source fan-out | `86dc6a6eaa4eb5f494f8d935f5535da8fb517b0b` |\n| #291 | Retained H.264 decoded-video pixelate evidence and bounded video Canvas readiness | `{MERGE_SHA}` |\n",
)
replace_once(
    plan,
    "Recent lineage: #284 from #283 squash `3543ddf7...`; #285 from #284 `7884fef8...`; #286 from #285 `64e34450...`; #287 from #286 `40e895ee...`; #288 from #287 `2774ee76...`; #289 from #288 `7be8e86f...`; **#290 directly from #289 squash `86dc6a6eaa4eb5f494f8d935f5535da8fb517b0b`**.",
    f"Recent lineage: #284 from #283 squash `3543ddf7...`; #285 from #284 `7884fef8...`; #286 from #285 `64e34450...`; #287 from #286 `40e895ee...`; #288 from #287 `2774ee76...`; #289 from #288 `7be8e86f...`; #290 validated directly from #289 squash `86dc6a6eaa4eb5f494f8d935f5535da8fb517b0b`; #291 mirrored that exact validated #290 head because the ready-for-review connector mutation failed, then squash-merged as `{MERGE_SHA}`; **#{PR_NUMBER} is directly from #291 squash `{MERGE_SHA}`**.",
)
replace_once(
    plan,
    "| Pixelate raster math is confused with backdrop-source parity | #285 owns grid/sample-index math; #286 owns structural backdrop admission; #287 owns FFmpeg scaler selection; #288 owns browser runtime acquisition/opaque proof; #289 owns byte-exact PNG evidence; #290 owns decoded-video measurement. None is substituted for another. |",
    f"| Pixelate raster math is confused with backdrop-source parity | #285 owns grid/sample-index math; #286 owns structural backdrop admission; #287 owns FFmpeg scaler selection; #288 owns browser runtime acquisition/opaque proof; #289 owns byte-exact PNG evidence; #291 owns decoded-video measurement; #{PR_NUMBER} owns decoded-frame identity and the focused codec/color acceptance gate. None is substituted for another. |",
)
replace_once(
    plan,
    "| MIME type or codec is treated as proof of an opaque backdrop | #288/#290 scan the sampled RGBA target rectangle and activate exact Canvas only when every alpha byte is `255`; no MIME-only or codec-only shortcut exists. |",
    "| MIME type or codec is treated as proof of an opaque backdrop | #288/#291 scan the sampled RGBA target rectangle and activate exact Canvas only when every alpha byte is `255`; no MIME-only or codec-only shortcut exists. |",
)
replace_once(
    plan,
    "| Newly sought video reports data before its frame is Canvas-rasterizable | #290 keeps video-only opacity misses pending for a bounded retry window, still requiring exact alpha before activation and failing closed after exhaustion. |",
    "| Newly sought video reports data before its frame is Canvas-rasterizable | #291 keeps video-only opacity misses pending for a bounded retry window, still requiring exact alpha before activation and failing closed after exhaustion. |",
)
replace_once(
    plan,
    "| Browser seek lands on a neighboring decoded frame at an exact boundary | #290 retains frame 59 as explicit timing evidence and excludes it from codec/color inference; the next slice must prove decoded frame identity before finalizing encoded-video thresholds. |",
    f"| Browser seek lands on a neighboring decoded frame at an exact boundary | #291 retains frame 59 as explicit timing evidence and excludes it from codec/color inference; #{PR_NUMBER} applies a sub-tolerance video-only boundary nudge and requires `requestVideoFrameCallback` evidence before the ±3 codec/color gate can pass. |",
)
replace_once(
    plan,
    """1. **Close deterministic decoded-video frame identity first.** Instrument browser media evidence so the requested canonical source frame can be proven, then fix the exact-boundary seek behavior that currently selects source frame 58 for canonical frame 59 while FFmpeg selects source frame 59.\n2. After frame identity is proven, freeze an explicit H.264/yuv420p browser↔FFmpeg codec/color gate from aligned evidence. Current data supports a ±3 RGB channel envelope with 100% pixel pass on frames 0/15/30, but do not finalize that contract until the timing probe also addresses the intended frame.\n3. Keep #289's opaque-PNG exactness unchanged as the zero-tolerance non-regression control and keep the 103-frame `parity-torture-v1` baseline additive and longitudinal.\n""",
    f"""1. **Execute #{PR_NUMBER}: deterministic decoded-video frame identity.** Retain browser `requestVideoFrameCallback` timestamps and require canonical frames 0/15/30/59 to present the same decoded frame identity after the sub-tolerance boundary-safe seek.\n2. In the same focused gate, accept H.264/yuv420p browser↔FFmpeg pixelate-region color variation only when every channel delta is ≤3 **after** frame identity passes. Keep repository-global ±2/99.9% diagnostics unchanged so this codec-specific evidence does not silently redefine other parity classes.\n3. Keep #289's opaque-PNG exactness unchanged as the zero-tolerance non-regression control and keep the 103-frame `parity-torture-v1` baseline additive and longitudinal.\n""",
)

print("decoded-frame identity patch applied")
