import { describe, it, expect } from 'vitest';
import { HeadlessFrameRenderer } from './renderWorker';
import { ContractTimelineDocument } from '../contract/timeline';

describe('HeadlessFrameRenderer', () => {
  const doc: ContractTimelineDocument = {
    version: 2,
    width: 640,
    height: 360,
    fps: 30,
    durationMs: 2000,
    aspectRatio: '16:9',
    tracks: [
      {
        id: 'track-v',
        type: 'video',
        visible: true,
        muted: false,
        clips: [
          {
            id: 'clip-1',
            startMs: 0,
            durationMs: 1000,
            transform: { x: 50, y: -50, scaleX: 1.5, scaleY: 1.5, opacity: 0.9 },
          },
          {
            id: 'clip-2',
            startMs: 1000,
            durationMs: 1000,
            text: { text: 'Worker Frame' },
          },
        ],
      },
    ],
  };

  it('calculates total frames accurately', () => {
    const worker = new HeadlessFrameRenderer(doc);
    expect(worker.getTotalFrames()).toBe(60); // 2s * 30fps
  });

  it('evaluates exact frame state across timeline', () => {
    const worker = new HeadlessFrameRenderer(doc);

    // Frame 15 (500ms)
    const frame15 = worker.getFrameState(15);
    expect(frame15.frameIndex).toBe(15);
    expect(frame15.activeClips).toHaveLength(1);
    expect(frame15.activeClips[0].id).toBe('clip-1');
    expect(frame15.activeClips[0].transform.x).toBe(50);
    expect(frame15.activeClips[0].transform.scaleX).toBe(1.5);

    // Frame 45 (1500ms)
    const frame45 = worker.getFrameState(45);
    expect(frame45.frameIndex).toBe(45);
    expect(frame45.activeClips).toHaveLength(1);
    expect(frame45.activeClips[0].id).toBe('clip-2');
    expect(frame45.activeClips[0].text?.text).toBe('Worker Frame');
  });
});
