import type { VideoAsset, VideoTimelineClip, VideoTimelineDocument, VideoTimelineTrack } from '../../../types/video';
import { evaluateClipProperty } from '../../../video/renderContractProperties';

export const PARITY_AUDIO_SAMPLE_RATE = 48_000;
export const PARITY_AUDIO_CHANNELS = 2;

export interface AudibleTimelineClip {
  track: VideoTimelineTrack;
  clip: VideoTimelineClip;
  asset: VideoAsset;
}

export interface PreviewPCMResult {
  pcm: ArrayBuffer;
  sampleRate: number;
  channels: number;
  sampleFormat: 's16le';
  frameCount: number;
  durationMs: number;
  audibleClipIDs: string[];
  limitations: string[];
}

export function collectAudibleTimelineClips(
  timeline: VideoTimelineDocument,
  assets: VideoAsset[],
): AudibleTimelineClip[] {
  const byID = new Map(assets.map((asset) => [asset.id, asset]));
  const anySolo = timeline.tracks.some((track) => track.solo);
  const audible: AudibleTimelineClip[] = [];
  for (const track of timeline.tracks) {
    if (track.muted || (anySolo && !track.solo)) continue;
    for (const clip of track.clips) {
      if (clip.muted || !clip.asset_id || clip.duration_ms <= 0) continue;
      const asset = byID.get(clip.asset_id);
      if (!asset || (!asset.mime_type.startsWith('audio/') && !asset.mime_type.startsWith('video/'))) continue;
      audible.push({ track, clip, asset });
    }
  }
  return audible;
}

export function sampleClipGain(clip: VideoTimelineClip, clipTimeMs: number): number {
  const boundedTime = Math.max(0, Math.min(clip.duration_ms, clipTimeMs));
  const volume = Math.max(0, evaluateClipProperty(clip, 'volume', boundedTime));
  let fade = 1;
  if ((clip.fade_in_ms ?? 0) > 0) fade = Math.min(fade, boundedTime / (clip.fade_in_ms as number));
  if ((clip.fade_out_ms ?? 0) > 0) fade = Math.min(fade, (clip.duration_ms - boundedTime) / (clip.fade_out_ms as number));
  return Math.max(0, volume * Math.max(0, Math.min(1, fade)));
}

function clipGainCurve(clip: VideoTimelineClip, outputDurationSeconds: number): Float32Array {
  // Gain automation at one point per 128-sample Web Audio render quantum keeps
  // the browser reference deterministic while accurately sampling authored
  // fades and nonlinear volume keyframes.
  const pointCount = Math.max(2, Math.ceil((outputDurationSeconds * PARITY_AUDIO_SAMPLE_RATE) / 128) + 1);
  const curve = new Float32Array(pointCount);
  for (let index = 0; index < pointCount; index += 1) {
    const clipTimeMs = outputDurationSeconds * 1000 * index / (pointCount - 1);
    curve[index] = sampleClipGain(clip, clipTimeMs);
  }
  return curve;
}

function interleavePCM16(rendered: AudioBuffer): ArrayBuffer {
  const output = new ArrayBuffer(rendered.length * PARITY_AUDIO_CHANNELS * 2);
  const view = new DataView(output);
  const left = rendered.getChannelData(0);
  const right = rendered.numberOfChannels > 1 ? rendered.getChannelData(1) : left;
  let offset = 0;
  for (let frame = 0; frame < rendered.length; frame += 1) {
    for (const sample of [left[frame], right[frame]]) {
      const clamped = Math.max(-1, Math.min(1, sample));
      const signed = clamped < 0 ? Math.round(clamped * 32768) : Math.round(clamped * 32767);
      view.setInt16(offset, signed, true);
      offset += 2;
    }
  }
  return output;
}

/**
 * Render the authored preview mix independently in Chromium's Web Audio
 * engine. This deliberately excludes the UI-only preview master volume. Full-
 * program render_audio_processing remains an explicit limitation until the
 * editor can audition the same cached processed stem as export.
 */
export async function renderPreviewPCM(
  timeline: VideoTimelineDocument,
  assets: VideoAsset[],
  loadAsset: (asset: VideoAsset) => Promise<ArrayBuffer>,
): Promise<PreviewPCMResult> {
  const durationMs = Math.max(1, Math.round(timeline.duration_ms));
  const frameCount = Math.max(1, Math.round(durationMs * PARITY_AUDIO_SAMPLE_RATE / 1000));
  const context = new OfflineAudioContext(PARITY_AUDIO_CHANNELS, frameCount, PARITY_AUDIO_SAMPLE_RATE);
  const audible = collectAudibleTimelineClips(timeline, assets);
  const decoded = new Map<string, Promise<AudioBuffer>>();
  const limitations: string[] = [];
  if (timeline.metadata?.render_audio_processing) {
    limitations.push('render_audio_processing is not auditioned by the current browser preview');
  }

  for (const { clip, asset } of audible) {
    const rate = Math.max(0.25, Math.min(4, clip.playback_rate ?? 1));
    if (rate !== 1) limitations.push(`clip ${clip.id}: OfflineAudioContext playbackRate does not preserve pitch`);
    let bufferPromise = decoded.get(asset.id);
    if (!bufferPromise) {
      bufferPromise = loadAsset(asset).then((bytes) => context.decodeAudioData(bytes.slice(0)));
      decoded.set(asset.id, bufferPromise);
    }
    const buffer = await bufferPromise;
    const startSeconds = Math.max(0, clip.start_ms / 1000);
    if (startSeconds >= durationMs / 1000) continue;
    const offsetSeconds = Math.max(0, (clip.trim_in_ms ?? 0) / 1000);
    const outputDurationSeconds = Math.min(clip.duration_ms / 1000, durationMs / 1000 - startSeconds);
    const authoredSourceSeconds = outputDurationSeconds * rate;
    const trimWindowSeconds = clip.trim_out_ms > clip.trim_in_ms
      ? (clip.trim_out_ms - clip.trim_in_ms) / 1000
      : authoredSourceSeconds;
    const sourceDurationSeconds = Math.min(authoredSourceSeconds, trimWindowSeconds, Math.max(0, buffer.duration - offsetSeconds));
    if (sourceDurationSeconds <= 0 || outputDurationSeconds <= 0) continue;

    const source = context.createBufferSource();
    source.buffer = buffer;
    source.playbackRate.value = rate;
    const gain = context.createGain();
    gain.gain.setValueCurveAtTime(clipGainCurve(clip, outputDurationSeconds), startSeconds, outputDurationSeconds);
    source.connect(gain).connect(context.destination);
    source.start(startSeconds, offsetSeconds, sourceDurationSeconds);
  }

  const rendered = await context.startRendering();
  return {
    pcm: interleavePCM16(rendered),
    sampleRate: PARITY_AUDIO_SAMPLE_RATE,
    channels: PARITY_AUDIO_CHANNELS,
    sampleFormat: 's16le',
    frameCount,
    durationMs,
    audibleClipIDs: audible.map(({ clip }) => clip.id),
    limitations: [...new Set(limitations)],
  };
}
