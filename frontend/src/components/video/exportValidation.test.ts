import { describe, expect, it } from 'vitest';
import type { VideoExportSettings, VideoRendererCapabilities, VideoTimelineDocument } from '../../types/video';
import { validateExport } from './exportValidation';

const capabilities: VideoRendererCapabilities = {
  renderer: 'ffmpeg',
  formats: ['mp4'],
  features: [
    { feature: 'transitions', label: 'Transitions', supported: true, partial: true, notes: 'Sampled export' },
    { feature: 'cursor_effects', label: 'Cursor', supported: true, partial: true, notes: 'Click audio is not synthesized' },
  ],
};

const timeline = {
  version: 1,
  canvas: { width: 1920, height: 1080, fps: 30, background: '#000000' },
  duration_ms: 1000,
  markers: [],
  metadata: {},
  tracks: [{
    id: 'track-1', type: 'layer', name: 'Layer 1', locked: false, muted: false, visible: true,
    clips: [{
      id: 'clip-1', start_ms: 0, duration_ms: 1000, trim_in_ms: 0, trim_out_ms: 1000,
      transform: { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1 },
      effects: [], keyframes: [],
      transitions: [{ id: 'transition-1', type: 'wipe', duration_ms: 250, direction: 'left' }],
      cursor: { events: [{ time_ms: 100, x: 10, y: 20, click: true }] },
    }],
  }],
} as VideoTimelineDocument;

const settings: VideoExportSettings = {
  format: 'mp4', codec: 'h264', resolution: 'project', quality: 'standard', include_audio: true,
};

describe('validateExport parity checks', () => {
  it('reports runtime-supported approximations with exact timeline paths', () => {
    const result = validateExport(timeline, [], settings, capabilities);
    expect(result.errors).toEqual([]);
    expect(result.warnings.some((warning) => warning.includes('tracks[0].clips[0].transitions[0]') && warning.includes('wipe'))).toBe(true);
    expect(result.warnings.some((warning) => warning.includes('tracks[0].clips[0].cursor') && warning.includes('sampled/partial'))).toBe(true);
    expect(result.warnings.some((warning) => warning.includes('preview-only'))).toBe(false);
  });

  it('promotes known fidelity differences to blocking errors in Strict Parity mode', () => {
    const result = validateExport(timeline, [], { ...settings, strict_parity: true }, capabilities);
    expect(result.warnings).toEqual([]);
    expect(result.errors.some((error) => error.includes('tracks[0].clips[0].transitions[0]'))).toBe(true);
    expect(result.errors.some((error) => error.includes('tracks[0].clips[0].cursor'))).toBe(true);
  });

  it('blocks Strict Parity when capability data is unavailable', () => {
    const result = validateExport(timeline, [], { ...settings, strict_parity: true }, null);
    expect(result.errors).toContain('Renderer capability data is unavailable — parity cannot be verified');
  });
});
