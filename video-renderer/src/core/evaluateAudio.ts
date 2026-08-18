import {
  ContractTimelineDocument,
  ContractTimelineClip,
  ContractTimelineTrack,
} from '../contract/timeline';

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

export function compileAudioGraph(
  doc: ContractTimelineDocument,
  sampleRate = 48000
): AudioGraph {
  const durationSec = (doc.durationMs || 0) / 1000;
  const totalSamples = Math.max(0, Math.round(durationSec * sampleRate));
  const anySolo = doc.tracks.some((t) => t.solo);

  const tracks: AudioTrackBus[] = doc.tracks.map((track) => {
    const isMuted = track.muted || (anySolo && !track.solo);
    const sources: AudioSourceWindow[] = [];

    if (!isMuted && track.clips) {
      track.clips.forEach((clip) => {
        if (clip.muted || !clip.assetId) return;

        const clipStartSec = clip.startMs / 1000;
        const clipDurSec = clip.durationMs / 1000;
        const rate = clip.playbackRate ?? 1;
        const trimInSec = (clip.trimInMs ?? 0) / 1000;

        const startSample = Math.round(clipStartSec * sampleRate);
        const sampleCount = Math.round(clipDurSec * sampleRate);
        const sourceStartSample = Math.round(trimInSec * sampleRate);

        const fadeInSamples = Math.round(((clip.fadeInMs ?? 0) / 1000) * sampleRate);
        const fadeOutSamples = Math.round(((clip.fadeOutMs ?? 0) / 1000) * sampleRate);

        sources.push({
          clipId: clip.id,
          assetId: clip.assetId,
          startSample,
          sampleCount,
          sourceStartSample,
          playbackRate: rate,
          volume: clip.volume ?? 1,
          pan: clip.pan ?? 0,
          fadeInSamples,
          fadeOutSamples,
        });
      });
    }

    return {
      trackId: track.id,
      muted: !!track.muted,
      solo: !!track.solo,
      volume: track.volume ?? 1,
      pan: track.pan ?? 0,
      sources,
    };
  });

  return {
    sampleRate,
    channels: 2,
    totalSamples,
    tracks,
    masterVolume: 1,
    masterProcessing: doc.metadata?.render_audio_processing,
  };
}
