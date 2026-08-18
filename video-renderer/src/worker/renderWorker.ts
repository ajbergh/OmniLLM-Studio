import { ContractTimelineDocument } from '../contract/timeline';
import { evaluateFrame, FrameState } from '../core/evaluateFrame';

export interface RenderJobProgress {
  frameIndex: number;
  totalFrames: number;
  percent: number;
}

export interface WorkerRenderOptions {
  manifestPath: string;
  outputPath: string;
  onProgress?: (progress: RenderJobProgress) => void;
}

/**
 * Headless Frame Renderer stub / core evaluator for Chromium worker execution.
 */
export class HeadlessFrameRenderer {
  private timeline: ContractTimelineDocument;

  constructor(timeline: ContractTimelineDocument) {
    this.timeline = timeline;
  }

  public getFrameState(frameIndex: number): FrameState {
    return evaluateFrame(this.timeline, frameIndex);
  }

  public getTotalFrames(): number {
    const fps = this.timeline.fps > 0 ? this.timeline.fps : 30;
    return Math.max(1, Math.round((this.timeline.durationMs * fps) / 1000));
  }
}
