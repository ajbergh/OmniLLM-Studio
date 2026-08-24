import { normalizeTimelineV2EvaluationInputs } from './renderContractNormalize';
import type {
  RenderManifestV1,
  RenderContractMetadata,
  TimelineV2Clip,
  TimelineV2MotionCurve,
  TimelineV2Track,
} from './renderContractTypes';

export const AUDIO_GRAPH_CONTRACT_V1 = 'audio-graph-v1' as const;
export const AUDIO_GRAPH_SAMPLE_RATE_V1 = 48_000 as const;
export const AUDIO_GRAPH_CHANNELS_V1 = 2 as const;

export interface AudioGraphV1 {
  contract_version: typeof AUDIO_GRAPH_CONTRACT_V1;
  sample_rate: typeof AUDIO_GRAPH_SAMPLE_RATE_V1;
  channels: typeof AUDIO_GRAPH_CHANNELS_V1;
  channel_layout: 'stereo';
  timeline_sample_count: number;
  range_start_sample: number;
  range_end_sample: number;
  range_sample_count: number;
  mix_policy: 'sum-no-normalize';
  program_stem_id: 'program-mix';
  sources: AudioGraphSourceV1[];
  program_processing: AudioGraphProgramProcessing;
}

export interface AudioGraphSourceV1 {
  node_id: string;
  stem_id: string;
  track_id: string;
  clip_id: string;
  asset_id: string;
  track_index: number;
  clip_index: number;
  enabled: boolean;
  suppression_reason: '' | 'track-muted' | 'clip-muted' | 'solo-suppressed' | 'outside-timeline';
  start_sample: number;
  end_sample: number;
  output_sample_count: number;
  source_start_ms: number;
  source_end_ms: number;
  playback_rate: number;
  pitch_policy: 'preserve';
  source_channels: number;
  channel_map: 'mono-to-stereo' | 'stereo-passthrough';
  base_gain: number;
  gain_mode: 'automation-overrides-base';
  gain_keyframes: AudioGraphGainKeyframe[];
  fade_in_samples: number;
  fade_out_samples: number;
  fade_curve: 'linear';
  fade_combine_policy: 'minimum';
}

export interface AudioGraphGainKeyframe {
  id: string;
  authored_order: number;
  time_ms: number;
  value: number;
  easing?: string;
  curve?: TimelineV2MotionCurve;
}

export interface AudioGraphProgramProcessing {
  mode: 'none' | 'processed-stem-required';
  stage: 'post-mix';
  input_stem_id: 'program-mix';
  output_stem_id: 'program-output';
  denoise: boolean;
  eq_preset: 'none' | 'voice' | 'warm' | 'bright';
  compressor: boolean;
  normalize: boolean;
  target_lufs: number;
  limiter: boolean;
  channel_mode: 'source' | 'mono' | 'stereo';
}

/**
 * Derive the canonical renderer-independent AudioGraph for one immutable render
 * manifest. This owns selection, timing, rate/pitch policy, channel mapping,
 * gain/fade semantics, mix order, program-processing placement, and exact
 * output sample counts. It performs no media I/O or DSP.
 */
export function evaluateAudioGraphV1(manifest: RenderManifestV1): AudioGraphV1 {
  if (manifest.settings.audio_sample_rate !== AUDIO_GRAPH_SAMPLE_RATE_V1) {
    throw new Error(`audio-graph-v1 requires audio_sample_rate=${AUDIO_GRAPH_SAMPLE_RATE_V1}`);
  }
  if (manifest.settings.audio_channels !== AUDIO_GRAPH_CHANNELS_V1) {
    throw new Error(`audio-graph-v1 requires audio_channels=${AUDIO_GRAPH_CHANNELS_V1}`);
  }

  const timeline = normalizeTimelineV2EvaluationInputs(manifest.timeline);
  if (!Number.isInteger(manifest.settings.range_start_frame)
      || !Number.isInteger(manifest.settings.range_end_frame)
      || manifest.settings.range_start_frame < 0
      || manifest.settings.range_end_frame < manifest.settings.range_start_frame) {
    throw new Error(
      `audio-graph-v1 has invalid render frame range [${manifest.settings.range_start_frame},${manifest.settings.range_end_frame})`,
    );
  }

  const timelineSampleCount = millisecondsToSamplesCeil(timeline.duration_ms, AUDIO_GRAPH_SAMPLE_RATE_V1);
  const rangeStartSample = Math.min(
    timelineSampleCount,
    frameToSamplesFloor(manifest.settings.range_start_frame, timeline.canvas.fps, AUDIO_GRAPH_SAMPLE_RATE_V1),
  );
  const rangeEndSample = Math.max(
    rangeStartSample,
    Math.min(
      timelineSampleCount,
      frameToSamplesCeil(manifest.settings.range_end_frame, timeline.canvas.fps, AUDIO_GRAPH_SAMPLE_RATE_V1),
    ),
  );

  const assets = new Map(manifest.assets.map((asset, index) => {
    const assetID = asset.asset_id.trim();
    if (!assetID) throw new Error(`audio-graph-v1 asset at order ${index} has empty asset_id`);
    return [assetID, asset] as const;
  }));
  if (assets.size !== manifest.assets.length) throw new Error('audio-graph-v1 has duplicate asset_id');

  const anySolo = timeline.tracks.some((track) => Boolean(track.solo));
  const sources: AudioGraphSourceV1[] = [];
  timeline.tracks.forEach((track, trackIndex) => {
    track.clips.forEach((clip, clipIndex) => {
      const assetID = clip.asset_id?.trim() ?? '';
      if (!assetID) {
        if (track.type === 'audio' || track.type === 'music') {
          throw new Error(`audio-graph-v1 audio clip ${JSON.stringify(clip.id)} has no asset_id`);
        }
        return;
      }
      const asset = assets.get(assetID);
      if (!asset) throw new Error(`audio-graph-v1 clip ${JSON.stringify(clip.id)} references missing manifest asset ${JSON.stringify(assetID)}`);

      const sourceChannels = asset.media?.channels ?? 0;
      const audioKind = asset.kind === 'audio' || asset.kind === 'music';
      if (!Number.isInteger(sourceChannels) || sourceChannels <= 0) {
        if (audioKind) throw new Error(`audio-graph-v1 audio asset ${JSON.stringify(assetID)} has no probed audio channel count`);
        return;
      }
      const channelMap = canonicalAudioChannelMap(sourceChannels);
      const baseGain = clip.volume ?? 1;
      if (!Number.isFinite(baseGain) || baseGain < 0 || baseGain > 2) {
        throw new Error(`audio-graph-v1 clip ${JSON.stringify(clip.id)} volume must be finite and between 0 and 2`);
      }
      if ((clip.fade_in_ms ?? 0) < 0 || (clip.fade_out_ms ?? 0) < 0) {
        throw new Error(`audio-graph-v1 clip ${JSON.stringify(clip.id)} fade durations cannot be negative`);
      }

      const startSample = Math.min(
        timelineSampleCount,
        millisecondsToSamplesFloor(clip.start_ms, AUDIO_GRAPH_SAMPLE_RATE_V1),
      );
      const endSample = Math.max(
        startSample,
        Math.min(
          timelineSampleCount,
          millisecondsToSamplesCeil(clip.start_ms + clip.duration_ms, AUDIO_GRAPH_SAMPLE_RATE_V1),
        ),
      );
      const [enabled, suppressionReason] = audioSourceEnabled(track, clip, anySolo, startSample, endSample);

      sources.push({
        node_id: `source:${clip.id}`,
        stem_id: `clip:${clip.id}`,
        track_id: track.id,
        clip_id: clip.id,
        asset_id: assetID,
        track_index: trackIndex,
        clip_index: clipIndex,
        enabled,
        suppression_reason: suppressionReason,
        start_sample: startSample,
        end_sample: endSample,
        output_sample_count: endSample - startSample,
        source_start_ms: clip.trim_in_ms,
        source_end_ms: clip.trim_out_ms,
        playback_rate: clip.playback_rate ?? 1,
        pitch_policy: 'preserve',
        source_channels: sourceChannels,
        channel_map: channelMap,
        base_gain: baseGain,
        gain_mode: 'automation-overrides-base',
        gain_keyframes: canonicalAudioGainKeyframes(clip),
        fade_in_samples: millisecondsToSamplesCeil(clip.fade_in_ms ?? 0, AUDIO_GRAPH_SAMPLE_RATE_V1),
        fade_out_samples: millisecondsToSamplesCeil(clip.fade_out_ms ?? 0, AUDIO_GRAPH_SAMPLE_RATE_V1),
        fade_curve: 'linear',
        fade_combine_policy: 'minimum',
      });
    });
  });

  return {
    contract_version: AUDIO_GRAPH_CONTRACT_V1,
    sample_rate: AUDIO_GRAPH_SAMPLE_RATE_V1,
    channels: AUDIO_GRAPH_CHANNELS_V1,
    channel_layout: 'stereo',
    timeline_sample_count: timelineSampleCount,
    range_start_sample: rangeStartSample,
    range_end_sample: rangeEndSample,
    range_sample_count: rangeEndSample - rangeStartSample,
    mix_policy: 'sum-no-normalize',
    program_stem_id: 'program-mix',
    sources,
    program_processing: canonicalAudioProgramProcessing(timeline.metadata),
  };
}

function canonicalAudioGainKeyframes(clip: TimelineV2Clip): AudioGraphGainKeyframe[] {
  return clip.keyframes
    .map((keyframe, authoredOrder) => ({ keyframe, authoredOrder }))
    .filter(({ keyframe }) => keyframe.property.trim().toLowerCase() === 'volume')
    .map(({ keyframe, authoredOrder }) => {
      const id = keyframe.id.trim();
      if (!id) throw new Error(`audio-graph-v1 clip ${JSON.stringify(clip.id)} has a volume keyframe with empty id`);
      if (!Number.isInteger(keyframe.time_ms)) {
        throw new Error(`audio-graph-v1 clip ${JSON.stringify(clip.id)} volume keyframe ${JSON.stringify(id)} time_ms must be an integer`);
      }
      if (!Number.isFinite(keyframe.value) || keyframe.value < 0 || keyframe.value > 2) {
        throw new Error(`audio-graph-v1 clip ${JSON.stringify(clip.id)} volume keyframe ${JSON.stringify(id)} value must be finite and between 0 and 2`);
      }
      const curve = keyframe.curve
        ? { ...keyframe.curve, type: keyframe.curve.type.trim().toLowerCase() as TimelineV2MotionCurve['type'] } as TimelineV2MotionCurve
        : undefined;
      return {
        id,
        authored_order: authoredOrder,
        time_ms: keyframe.time_ms,
        value: keyframe.value,
        ...(keyframe.easing ? { easing: keyframe.easing.trim().toLowerCase() } : {}),
        ...(curve ? { curve } : {}),
      };
    })
    .sort((left, right) => left.time_ms - right.time_ms || left.authored_order - right.authored_order);
}

function canonicalAudioChannelMap(sourceChannels: number): AudioGraphSourceV1['channel_map'] {
  if (sourceChannels === 1) return 'mono-to-stereo';
  if (sourceChannels === 2) return 'stereo-passthrough';
  throw new Error(`source channel count ${sourceChannels} has no canonical v1 channel mapping`);
}

function audioSourceEnabled(
  track: TimelineV2Track,
  clip: TimelineV2Clip,
  anySolo: boolean,
  startSample: number,
  endSample: number,
): [boolean, AudioGraphSourceV1['suppression_reason']] {
  if (track.muted) return [false, 'track-muted'];
  if (clip.muted) return [false, 'clip-muted'];
  if (anySolo && !track.solo) return [false, 'solo-suppressed'];
  if (endSample <= startSample) return [false, 'outside-timeline'];
  return [true, ''];
}

function canonicalAudioProgramProcessing(metadata: RenderContractMetadata): AudioGraphProgramProcessing {
  const base: AudioGraphProgramProcessing = {
    mode: 'none',
    stage: 'post-mix',
    input_stem_id: 'program-mix',
    output_stem_id: 'program-output',
    denoise: false,
    eq_preset: 'none',
    compressor: false,
    normalize: false,
    target_lufs: -16,
    limiter: false,
    channel_mode: 'source',
  };
  const raw = metadata?.render_audio_processing;
  if (raw === undefined || raw === null) return base;
  if (typeof raw !== 'object' || Array.isArray(raw)) {
    throw new Error('audio-graph-v1 metadata.render_audio_processing must be an object');
  }
  const values = raw as Record<string, unknown>;
  const required = ['normalize', 'target_lufs', 'denoise', 'eq_preset', 'compressor', 'limiter', 'channels'] as const;
  const allowed = new Set<string>(required);
  for (const key of required) {
    if (!(key in values)) throw new Error(`audio-graph-v1 metadata.render_audio_processing.${key} is required`);
  }
  for (const key of Object.keys(values)) {
    if (!allowed.has(key)) throw new Error(`audio-graph-v1 metadata.render_audio_processing has unsupported field ${JSON.stringify(key)}`);
  }

  const bool = (key: 'normalize' | 'denoise' | 'compressor' | 'limiter'): boolean => {
    const value = values[key];
    if (typeof value !== 'boolean') throw new Error(`audio-graph-v1 metadata.render_audio_processing.${key} must be a boolean`);
    return value;
  };
  const target = values.target_lufs;
  if (typeof target !== 'number' || !Number.isFinite(target) || target < -30 || target > -5) {
    throw new Error('audio-graph-v1 metadata.render_audio_processing.target_lufs must be finite and between -30 and -5');
  }
  const eq = typeof values.eq_preset === 'string' ? values.eq_preset.trim().toLowerCase() : '';
  if (!['none', 'voice', 'warm', 'bright'].includes(eq)) {
    throw new Error(`audio-graph-v1 metadata.render_audio_processing.eq_preset ${JSON.stringify(eq)} is unsupported`);
  }
  const channels = typeof values.channels === 'string' ? values.channels.trim().toLowerCase() : '';
  if (!['source', 'mono', 'stereo'].includes(channels)) {
    throw new Error(`audio-graph-v1 metadata.render_audio_processing.channels ${JSON.stringify(channels)} is unsupported`);
  }

  return {
    ...base,
    mode: 'processed-stem-required',
    denoise: bool('denoise'),
    eq_preset: eq as AudioGraphProgramProcessing['eq_preset'],
    compressor: bool('compressor'),
    normalize: bool('normalize'),
    target_lufs: target,
    limiter: bool('limiter'),
    channel_mode: channels as AudioGraphProgramProcessing['channel_mode'],
  };
}

function millisecondsToSamplesFloor(ms: number, sampleRate: number): number {
  if (ms <= 0 || sampleRate <= 0) return 0;
  return Math.floor(ms * sampleRate / 1000);
}

function millisecondsToSamplesCeil(ms: number, sampleRate: number): number {
  if (ms <= 0 || sampleRate <= 0) return 0;
  return Math.ceil(ms * sampleRate / 1000);
}

function frameToSamplesFloor(frame: number, fps: number, sampleRate: number): number {
  if (frame <= 0 || fps <= 0 || sampleRate <= 0) return 0;
  return Math.floor(frame * sampleRate / fps);
}

function frameToSamplesCeil(frame: number, fps: number, sampleRate: number): number {
  if (frame <= 0 || fps <= 0 || sampleRate <= 0) return 0;
  return Math.ceil(frame * sampleRate / fps);
}
