export interface AudioSourceWindow {
  clipId: string;
  assetId: string;
  startSample: number;
  sampleCount: number;
  sourceStartSample: number;
  playbackRate: number;
  volume: number;
  pan: number;
  fadeInSamples: number;
  fadeOutSamples: number;
}

export interface AudioTrackBus {
  trackId: string;
  muted: boolean;
  solo: boolean;
  volume: number;
  pan: number;
  sources: AudioSourceWindow[];
}

export interface AudioGraph {
  sampleRate: number;
  channels: number;
  totalSamples: number;
  tracks: AudioTrackBus[];
  masterVolume: number;
  masterProcessing?: {
    denoise?: boolean;
    eq?: boolean;
    compress?: boolean;
    normalize?: boolean;
    limiter?: boolean;
  };
}
