import { describe, expect, it } from 'vitest';
import { fitHistoryToBudget } from './VideoEditStudioUltimate';
import type { VideoTimelineDocument } from '../../types/video';

function document(label: string, padding = ''): VideoTimelineDocument {
  return {
    version: 1,
    canvas: { width: 1920, height: 1080, fps: 30, background: '#000000' },
    duration_ms: 1_000,
    tracks: [],
    markers: [],
    metadata: { label, padding },
  };
}

describe('fitHistoryToBudget', () => {
  it('retains the newest snapshots within the byte budget', () => {
    const history = [document('old', 'a'.repeat(500)), document('middle', 'b'.repeat(500)), document('new', 'c'.repeat(500))];
    const oneDocumentBudget = new TextEncoder().encode(JSON.stringify(history[2])).byteLength + 5;
    const retained = fitHistoryToBudget(history, oneDocumentBudget);
    expect(retained).toHaveLength(1);
    expect(retained[0].metadata.label).toBe('new');
  });

  it('always keeps the newest snapshot when it alone exceeds the budget', () => {
    const retained = fitHistoryToBudget([document('large', 'x'.repeat(2_000))], 16);
    expect(retained).toHaveLength(1);
    expect(retained[0].metadata.label).toBe('large');
  });
});
