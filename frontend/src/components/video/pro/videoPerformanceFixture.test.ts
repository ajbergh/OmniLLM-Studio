import { describe, expect, it } from 'vitest';
import { applyDecoderBudget, buildTimelineIntervalIndex, queryActiveClips, visibleClips } from './timelineIndex';
import { createTimelinePatch } from './patchHistory';
import { createLargeVideoFixture } from './videoPerformanceFixture';

declare const process: { memoryUsage: () => { heapUsed: number } };

describe('large motion timeline performance evidence', () => {
  it('keeps indexing, windowing, and decoder mounting within review budgets', () => {
    const fixture = createLargeVideoFixture();
    const heapBefore = process.memoryUsage().heapUsed;
    const started = performance.now();
    const index = buildTimelineIntervalIndex(fixture.document, fixture.assets);
    const indexedAt = performance.now();
    let queries = 0;
    const frameTimes: number[] = [];
    for (let time = 0; time < fixture.document.duration_ms; time += 250) {
      const frameStarted = performance.now();
      const active = queryActiveClips(index, time);
      const budget = applyDecoderBudget(active, 4);
      expect(budget.mounted.filter((item) => item.asset?.mime_type.startsWith('video/')).length).toBeLessThanOrEqual(4);
      // Exercise the bounded per-frame composition data shape used by React.
      budget.mounted.map((item) => `${item.clip.id}:${item.clip.transform?.x || 0}:${item.clip.transform?.z || 0}`);
      frameTimes.push(performance.now() - frameStarted);
      queries += 1;
    }
    const queriedAt = performance.now();
    const visible = fixture.document.tracks.flatMap((track) => visibleClips(track.clips, 30_000, 40_000));
    const finished = performance.now();
    const patched = structuredClone(fixture.document);
    patched.tracks[0].clips[0].keyframes.push(
      { id: 'perf-pop-a', property: 'scale', time_ms: 0, value: 0.8, easing: 'ease-out' },
      { id: 'perf-pop-b', property: 'scale', time_ms: 420, value: 1.06, easing: 'ease-out' },
      { id: 'perf-pop-c', property: 'scale', time_ms: 650, value: 1, easing: 'ease-out' },
    );
    const blockPatchBytes = createTimelinePatch(fixture.document, patched).bytes;
    const sortedFrameTimes = [...frameTimes].sort((left, right) => left - right);
    const frameP95Ms = sortedFrameTimes[Math.floor(sortedFrameTimes.length * 0.95)] || 0;
    const evidence = { clips: index.clips.length, queries, visible: visible.length, index_ms: indexedAt - started, query_ms: queriedAt - indexedAt, window_ms: finished - queriedAt, frame_compute_p95_ms: frameP95Ms, heap_delta_bytes: Math.max(0, process.memoryUsage().heapUsed - heapBefore), worst_block_patch_bytes: blockPatchBytes, document_bytes: new TextEncoder().encode(JSON.stringify(fixture.document)).byteLength };
    console.info(`VIDEO_PERFORMANCE_EVIDENCE ${JSON.stringify(evidence)}`);
    expect(index.clips).toHaveLength(2000);
    expect(evidence.document_bytes).toBeLessThan(10_000_000);
    expect(evidence.index_ms).toBeLessThan(1500);
    expect(evidence.query_ms).toBeLessThan(3000);
    expect(evidence.window_ms).toBeLessThan(1000);
    expect(evidence.frame_compute_p95_ms).toBeLessThan(16.7);
    expect(evidence.heap_delta_bytes).toBeLessThan(128 * 1024 * 1024);
    expect(evidence.worst_block_patch_bytes).toBeLessThan(256 * 1024);
  });
});
