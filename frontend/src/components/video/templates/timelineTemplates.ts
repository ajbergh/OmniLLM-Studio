import type {
  VideoTimelineClip,
  VideoTimelineDocument,
  VideoTimelineTrack,
  VideoTimelineTrackType,
} from '../../../types/video';

function id(prefix: string): string {
  const value = globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  return `${prefix}-${value}`;
}

function track(type: VideoTimelineTrackType, name: string, clips: VideoTimelineClip[] = []): VideoTimelineTrack {
  return { id: id('track'), type, name, locked: false, muted: false, visible: true, clips };
}

function textClip(options: {
  text: string;
  startMs: number;
  durationMs: number;
  fontSize?: number;
  fontWeight?: string;
  background?: string;
  align?: 'left' | 'center' | 'right';
  x?: number;
  y?: number;
}): VideoTimelineClip {
  return {
    id: id('clip'),
    start_ms: options.startMs,
    duration_ms: options.durationMs,
    trim_in_ms: 0,
    trim_out_ms: options.durationMs,
    transform: { x: options.x ?? 0, y: options.y ?? 0, scale: 1, rotation: 0, opacity: 1 },
    text: {
      text: options.text,
      font_size: options.fontSize ?? 64,
      font_weight: options.fontWeight ?? '700',
      color: '#ffffff',
      background: options.background,
      shadow: !options.background,
      text_align: options.align ?? 'center',
    },
    effects: [],
    keyframes: [],
    transitions: [],
  };
}

function baseDocument(width: number, height: number, durationMs: number, tracks: VideoTimelineTrack[]): VideoTimelineDocument {
  return {
    version: 1,
    canvas: { width, height, fps: 30, background: '#000000' },
    duration_ms: durationMs,
    tracks,
    markers: [],
    metadata: {},
  };
}

export interface TimelineTemplate {
  key: string;
  label: string;
  description: string;
  build: () => VideoTimelineDocument;
}

export const TIMELINE_TEMPLATES: TimelineTemplate[] = [
  {
    key: 'blank_16_9',
    label: 'Blank 16:9',
    description: 'Standard widescreen project with video, overlay, audio, and text tracks.',
    build: () =>
      baseDocument(1920, 1080, 30000, [
        track('video', 'Video 1'),
        track('image', 'Overlay 1'),
        track('audio', 'Audio 1'),
        track('text', 'Text 1'),
      ]),
  },
  {
    key: 'reel_9_16',
    label: '9:16 Short / Reel',
    description: 'Vertical short with a hook title and a caption track.',
    build: () =>
      baseDocument(1080, 1920, 30000, [
        track('video', 'Video 1'),
        track('image', 'Overlay 1'),
        track('music', 'Music 1'),
        track('text', 'Text 1', [
          textClip({ text: 'Your hook here', startMs: 0, durationMs: 2500, fontSize: 88, y: -480 }),
        ]),
        track('caption', 'Captions 1'),
      ]),
  },
  {
    key: 'square_1_1',
    label: '1:1 Social Square',
    description: 'Square feed post with video, overlay, music, and text tracks.',
    build: () =>
      baseDocument(1080, 1080, 30000, [
        track('video', 'Video 1'),
        track('image', 'Overlay 1'),
        track('music', 'Music 1'),
        track('text', 'Text 1'),
      ]),
  },
  {
    key: 'title_lower_third',
    label: 'Title + Lower Third',
    description: 'Opening title card followed by a styled lower third.',
    build: () =>
      baseDocument(1920, 1080, 30000, [
        track('video', 'Video 1'),
        track('text', 'Titles', [
          textClip({ text: 'Your Title', startMs: 0, durationMs: 3000, fontSize: 96 }),
          textClip({
            text: 'Name — Role',
            startMs: 3000,
            durationMs: 5000,
            fontSize: 40,
            background: '#111111',
            align: 'left',
            x: -420,
            y: 380,
          }),
        ]),
      ]),
  },
  {
    key: 'talking_head_captions',
    label: 'Captioned Talking Head',
    description: 'Single speaker layout with a pre-seeded caption track.',
    build: () =>
      baseDocument(1920, 1080, 30000, [
        track('video', 'Camera'),
        track('caption', 'Captions 1', [
          textClip({ text: 'First caption…', startMs: 0, durationMs: 2000, fontSize: 48, fontWeight: '600', y: 410 }),
          textClip({ text: 'Second caption…', startMs: 2000, durationMs: 2000, fontSize: 48, fontWeight: '600', y: 410 }),
          textClip({ text: 'Third caption…', startMs: 4000, durationMs: 2000, fontSize: 48, fontWeight: '600', y: 410 }),
        ]),
      ]),
  },
  {
    key: 'slideshow',
    label: 'Image Slideshow',
    description: 'Image track with markers every 4 seconds as slide slots, plus a title.',
    build: () => {
      const doc = baseDocument(1920, 1080, 20000, [
        track('image', 'Slides'),
        track('music', 'Music 1'),
        track('text', 'Text 1', [textClip({ text: 'Slideshow title', startMs: 0, durationMs: 3000, fontSize: 80 })]),
      ]);
      doc.markers = [0, 4000, 8000, 12000, 16000].map((timeMs, index) => ({
        id: id('marker'),
        time_ms: timeMs,
        label: `Slide ${index + 1}`,
      }));
      return doc;
    },
  },
];
