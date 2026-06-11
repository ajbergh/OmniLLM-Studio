/**
 * Caption utilities shared by the caption panel and the store: SRT/WebVTT
 * parsing and serialization, plus the style presets (the backend mirrors the
 * serialization in backend/internal/video/captions.go for render-time
 * sidecar files — keep formats in sync).
 */
import type { VideoTimelineCanvas, VideoTimelineText } from '../../../types/video';

export interface CaptionCue {
  start_ms: number;
  end_ms: number;
  text: string;
}

/** Accepts SRT `HH:MM:SS,mmm`, VTT `HH:MM:SS.mmm`, and short VTT `MM:SS.mmm`. */
function parseTimestamp(value: string): number | null {
  const match = value.trim().match(/^(?:(\d{1,2}):)?(\d{1,2}):(\d{2})[.,](\d{1,3})$/);
  if (!match) return null;
  const hours = Number(match[1] || 0);
  const minutes = Number(match[2]);
  const seconds = Number(match[3]);
  const millis = Number(match[4].padEnd(3, '0'));
  return ((hours * 60 + minutes) * 60 + seconds) * 1000 + millis;
}

/** Parses SRT or WebVTT text into sorted cues. Unparseable blocks are skipped. */
export function parseCaptions(raw: string): CaptionCue[] {
  const text = raw.replace(/^\uFEFF/, '');
  const blocks = text.split(/\r?\n\r?\n+/);
  const cues: CaptionCue[] = [];
  for (const block of blocks) {
    const lines = block.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
    if (lines.length === 0) continue;
    if (/^(WEBVTT|NOTE|STYLE|REGION)\b/i.test(lines[0]) && !lines[0].includes('-->')) continue;
    const timeLineIndex = lines.findIndex((line) => line.includes('-->'));
    if (timeLineIndex === -1) continue;
    const [startRaw, endPart] = lines[timeLineIndex].split('-->');
    // VTT cue settings ("align:start position:0%") follow the end timestamp.
    const endRaw = (endPart || '').trim().split(/\s+/)[0];
    const start = parseTimestamp(startRaw);
    const end = parseTimestamp(endRaw);
    if (start === null || end === null || end <= start) continue;
    const textLines = lines.slice(timeLineIndex + 1);
    if (textLines.length === 0) continue;
    cues.push({ start_ms: start, end_ms: end, text: textLines.join('\n') });
  }
  return cues.sort((a, b) => a.start_ms - b.start_ms);
}

function pad(value: number, length = 2): string {
  return String(value).padStart(length, '0');
}

function formatTimestamp(ms: number, separator: ',' | '.'): string {
  const clamped = Math.max(0, Math.round(ms));
  const hours = Math.floor(clamped / 3_600_000);
  const minutes = Math.floor((clamped % 3_600_000) / 60_000);
  const seconds = Math.floor((clamped % 60_000) / 1000);
  const millis = clamped % 1000;
  return `${pad(hours)}:${pad(minutes)}:${pad(seconds)}${separator}${pad(millis, 3)}`;
}

export function serializeSrt(cues: CaptionCue[]): string {
  return (
    cues
      .map((cue, index) => `${index + 1}\n${formatTimestamp(cue.start_ms, ',')} --> ${formatTimestamp(cue.end_ms, ',')}\n${cue.text}`)
      .join('\n\n') + '\n'
  );
}

export function serializeVtt(cues: CaptionCue[]): string {
  return (
    'WEBVTT\n\n' +
    cues.map((cue) => `${formatTimestamp(cue.start_ms, '.')} --> ${formatTimestamp(cue.end_ms, '.')}\n${cue.text}`).join('\n\n') +
    '\n'
  );
}

export type CaptionPresetKey = 'subtitle' | 'bold_social' | 'lower_third' | 'training' | 'accessibility';

export interface CaptionPreset {
  key: CaptionPresetKey;
  label: string;
  /** Text-payload patch applied to every caption clip. */
  text: Partial<Omit<VideoTimelineText, 'text'>>;
  /** Canvas-px offset from center applied to every caption clip's transform. */
  position: (canvas: VideoTimelineCanvas) => { x: number; y: number };
}

export const CAPTION_PRESETS: CaptionPreset[] = [
  {
    key: 'subtitle',
    label: 'Subtitle',
    text: { font_size: 48, font_weight: '600', color: '#ffffff', background: '', stroke: '', shadow: true, text_align: 'center' },
    position: (canvas) => ({ x: 0, y: Math.round(canvas.height * 0.38) }),
  },
  {
    key: 'bold_social',
    label: 'Bold social',
    text: { font_size: 64, font_weight: '800', color: '#ffffff', background: '', stroke: '#000000', stroke_width: 4, shadow: false, text_align: 'center' },
    position: (canvas) => ({ x: 0, y: Math.round(canvas.height * 0.3) }),
  },
  {
    key: 'lower_third',
    label: 'Lower third',
    text: { font_size: 40, font_weight: '700', color: '#ffffff', background: '#111111', border_radius: 8, stroke: '', shadow: false, text_align: 'left' },
    position: (canvas) => ({ x: -Math.round(canvas.width * 0.22), y: Math.round(canvas.height * 0.36) }),
  },
  {
    key: 'training',
    label: 'Training burn-in',
    text: { font_size: 44, font_weight: '600', color: '#ffffff', background: '#000000cc', border_radius: 4, stroke: '', shadow: false, text_align: 'center' },
    position: (canvas) => ({ x: 0, y: Math.round(canvas.height * 0.4) }),
  },
  {
    key: 'accessibility',
    label: 'Accessibility',
    text: { font_size: 52, font_weight: '700', color: '#ffff00', background: '#000000', border_radius: 0, stroke: '', shadow: false, text_align: 'center' },
    position: (canvas) => ({ x: 0, y: Math.round(canvas.height * 0.38) }),
  },
];
