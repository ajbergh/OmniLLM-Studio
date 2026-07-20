import { commitTimelineCommand, timelineRange } from './timelineCommandEngine';
import type { VideoTimelineClip, VideoTimelineDocument, VideoTimelineTrack } from '../../../types/video';

function id(prefix: string): string {
  return `${prefix}-${globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(16).slice(2)}`}`;
}

function splitTranscript(text: string): string[] {
  const normalized = text
    .replace(/\r/g, '')
    .replace(/\n{2,}/g, '\n')
    .trim();
  if (!normalized) return [];
  const explicit = normalized.split('\n').map((line) => line.trim()).filter(Boolean);
  if (explicit.length > 1) return explicit;
  return normalized
    .split(/(?<=[.!?])\s+/)
    .map((segment) => segment.trim())
    .filter(Boolean);
}

function captionTrack(document: VideoTimelineDocument): VideoTimelineTrack {
  const existing = document.tracks.find((track) => track.type === 'caption' && !track.locked);
  if (existing) return existing;
  const track: VideoTimelineTrack = {
    id: id('track'),
    type: 'caption',
    name: 'Captions',
    locked: false,
    muted: false,
    visible: true,
    clips: [],
  };
  document.tracks.push(track);
  return track;
}

function captionClip(text: string, startMs: number, durationMs: number, canvas: VideoTimelineDocument['canvas']): VideoTimelineClip {
  return {
    id: id('clip'),
    start_ms: startMs,
    duration_ms: Math.max(500, durationMs),
    trim_in_ms: 0,
    trim_out_ms: Math.max(500, durationMs),
    transform: {
      x: 0,
      y: Math.round(canvas.height * 0.38),
      scale: 1,
      rotation: 0,
      opacity: 1,
    },
    text: {
      text,
      font_size: Math.max(30, Math.round(canvas.height * 0.045)),
      font_weight: '700',
      color: '#ffffff',
      stroke: '#000000',
      stroke_width: 3,
      shadow: true,
      text_align: 'center',
      line_height: 1.15,
      params: { source: 'pasted_transcript' },
    },
    effects: [],
    keyframes: [],
    transitions: [],
  };
}

export async function createCaptionsFromTranscript(text: string, replaceExisting = false): Promise<boolean> {
  const segments = splitTranscript(text);
  return commitTimelineCommand('Create captions from transcript', (document) => {
    if (segments.length === 0) return { changed: false, message: 'Paste a transcript first' };
    const range = timelineRange(document) || { startMs: 0, endMs: document.duration_ms };
    const totalWeight = segments.reduce((sum, segment) => sum + Math.max(1, segment.split(/\s+/).length), 0);
    const totalDuration = Math.max(segments.length * 700, range.endMs - range.startMs);
    const track = captionTrack(document);
    if (replaceExisting) track.clips = [];
    let cursor = range.startMs;
    segments.forEach((segment, index) => {
      const words = Math.max(1, segment.split(/\s+/).length);
      const remaining = range.endMs - cursor;
      const weighted = index === segments.length - 1
        ? remaining
        : Math.max(700, Math.round(totalDuration * (words / totalWeight)));
      const duration = Math.max(500, Math.min(weighted, remaining));
      if (duration <= 0) return;
      track.clips.push(captionClip(segment, cursor, duration, document.canvas));
      cursor += duration;
    });
    track.clips.sort((a, b) => a.start_ms - b.start_ms);
    return {
      changed: true,
      message: `Created ${segments.length} caption segment${segments.length === 1 ? '' : 's'} from the pasted transcript`,
      selectedTrackId: track.id,
    };
  });
}

export async function replaceCaptionText(search: string, replacement: string): Promise<boolean> {
  const needle = search.trim();
  return commitTimelineCommand('Replace caption text', (document) => {
    if (!needle) return { changed: false, message: 'Enter caption text to find' };
    let changed = 0;
    for (const track of document.tracks) {
      if (track.type !== 'caption' || track.locked) continue;
      for (const clip of track.clips) {
        if (!clip.text?.text) continue;
        const next = clip.text.text.split(needle).join(replacement);
        if (next !== clip.text.text) {
          clip.text.text = next;
          changed += 1;
        }
      }
    }
    return {
      changed: changed > 0,
      message: changed > 0 ? `Updated ${changed} caption segment${changed === 1 ? '' : 's'}` : `No captions contain “${needle}”`,
    };
  });
}

const FILLER_PATTERN = /\b(?:um+|uh+|erm+|ah+|you know|kind of|sort of)\b[,.]?\s*/gi;

export async function cleanCaptionFillers(): Promise<boolean> {
  return commitTimelineCommand('Remove caption filler words', (document) => {
    let changed = 0;
    for (const track of document.tracks) {
      if (track.type !== 'caption' || track.locked) continue;
      for (const clip of track.clips) {
        if (!clip.text?.text) continue;
        const next = clip.text.text
          .replace(FILLER_PATTERN, '')
          .replace(/\s{2,}/g, ' ')
          .replace(/^\s+|\s+$/g, '');
        if (next !== clip.text.text) {
          clip.text.text = next;
          changed += 1;
        }
      }
    }
    return {
      changed: changed > 0,
      message: changed > 0 ? `Cleaned filler words from ${changed} caption segment${changed === 1 ? '' : 's'}` : 'No common filler words were found',
    };
  });
}
