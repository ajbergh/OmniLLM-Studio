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
  slot?: string;
}): VideoTimelineClip {
  return {
    id: id('clip'),
    template_slot: options.slot,
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

function assetSlotClip(slot: string, startMs: number, durationMs: number, options: { x?: number; y?: number; scale?: number; z?: number } = {}): VideoTimelineClip {
  return {
    id: id('clip'), template_slot: slot, start_ms: startMs, duration_ms: durationMs, trim_in_ms: 0, trim_out_ms: durationMs,
    transform: { x: options.x || 0, y: options.y || 0, z: options.z || 0, scale: options.scale || 1, rotation: 0, opacity: 1 },
    effects: [], transitions: [], metadata: { required_asset_kind: 'visual' },
    keyframes: [
      { id: id('keyframe'), property: 'scale', time_ms: 0, value: (options.scale || 1) * 0.96, easing: 'ease-out' },
      { id: id('keyframe'), property: 'scale', time_ms: durationMs, value: (options.scale || 1) * 1.04, easing: 'ease-in-out' },
    ],
  };
}

function baseDocument(width: number, height: number, durationMs: number, tracks: VideoTimelineTrack[]): VideoTimelineDocument {
  const document: VideoTimelineDocument = {
    version: 1,
    canvas: { width, height, fps: 30, background: '#000000' },
    duration_ms: durationMs,
    tracks,
    markers: [],
    metadata: {},
  };
  const slots = tracks.flatMap((item) => item.clips).map((clip) => clip.template_slot).filter((slot): slot is string => Boolean(slot));
  if (slots.length > 0) document.metadata.template_slots = slots;
  return document;
}

export interface TimelineTemplate {
  key: string;
  label: string;
  description: string;
  build: () => VideoTimelineDocument;
}

export const TIMELINE_TEMPLATES: TimelineTemplate[] = [
  {
    key: 'app_demo', label: 'App Demo', description: 'Animated product walkthrough with screenshot slots.',
    build: () => baseDocument(1920, 1080, 16000, [track('layer', 'Screens', [assetSlotClip('Screenshot 1', 0, 5500, { x: -360, scale: 0.62, z: 120 }), assetSlotClip('Screenshot 2', 5250, 5500, { x: 320, scale: 0.62, z: 40 }), assetSlotClip('Screenshot 3', 10500, 5500, { scale: 0.72, z: 180 })]), track('text', 'Copy', [textClip({ text: 'A better way to work', slot: 'Headline', startMs: 0, durationMs: 4000, y: -400 })])]),
  },
  {
    key: 'product_hero', label: 'Product Hero', description: 'Cinematic hero image, logo, headline, and CTA.',
    build: () => { const doc = baseDocument(1920, 1080, 10000, [track('layer', 'Hero', [assetSlotClip('Hero Image', 0, 10000)]), track('layer', 'Logo', [assetSlotClip('Logo', 400, 9200, { x: -650, y: -390, scale: 0.22, z: 160 })]), track('text', 'Copy', [textClip({ text: 'Meet the future', slot: 'Headline', startMs: 700, durationMs: 8000, fontSize: 104 }), textClip({ text: 'Get started', slot: 'CTA', startMs: 2500, durationMs: 6500, fontSize: 36, background: '#ffffff', y: 300 })])]); doc.scenes = [{ id: id('scene'), name: 'Hero', start_ms: 0, duration_ms: 10000, camera: { field_of_view: 48, keyframes: [{ id: id('camera-keyframe'), property: 'z', time_ms: 0, value: 0, easing: 'ease-in-out' }, { id: id('camera-keyframe'), property: 'z', time_ms: 10000, value: 180, easing: 'ease-in-out' }] }, effects: [{ id: id('effect'), type: 'film_grain', enabled: true, params: { amount: 5 } }] }]; return doc; },
  },
  {
    key: 'floating_screens', label: 'Floating Screens', description: 'Three depth-separated screenshot cards with camera parallax.',
    build: () => { const doc = baseDocument(1920, 1080, 12000, [track('layer', 'Back', [assetSlotClip('Screenshot 1', 0, 12000, { x: -430, scale: 0.5, z: -180 })]), track('layer', 'Middle', [assetSlotClip('Screenshot 2', 0, 12000, { x: 0, scale: 0.58, z: 20 })]), track('layer', 'Front', [assetSlotClip('Screenshot 3', 0, 12000, { x: 430, scale: 0.5, z: 180 })])]); doc.scenes = [{ id: id('scene'), name: 'Float', start_ms: 0, duration_ms: 12000, camera: { field_of_view: 55, keyframes: [{ id: id('camera-keyframe'), property: 'x', time_ms: 0, value: -120, easing: 'ease-in-out' }, { id: id('camera-keyframe'), property: 'x', time_ms: 12000, value: 120, easing: 'ease-in-out' }] } }]; return doc; },
  },
  { key: 'feature_carousel', label: 'Feature Carousel', description: 'Three timed feature screenshots with editable copy.', build: () => baseDocument(1920, 1080, 15000, [track('layer', 'Features', [assetSlotClip('Screenshot 1', 0, 5000), assetSlotClip('Screenshot 2', 5000, 5000), assetSlotClip('Screenshot 3', 10000, 5000)]), track('text', 'Copy', [textClip({ text: 'Feature one', slot: 'Headline', startMs: 0, durationMs: 5000, y: -380 }), textClip({ text: 'Feature two', startMs: 5000, durationMs: 5000, y: -380 }), textClip({ text: 'Feature three', startMs: 10000, durationMs: 5000, y: -380 })])]) },
  { key: 'logo_reveal', label: 'Logo Reveal', description: 'Short polished logo reveal.', build: () => { const doc = baseDocument(1920, 1080, 5000, [track('layer', 'Logo', [assetSlotClip('Logo', 0, 5000, { scale: 0.38, z: 120 })])]); const logo = doc.tracks[0].clips[0]; logo.keyframes = [{ id: id('keyframe'), property: 'scale', time_ms: 0, value: 0.5, easing: 'ease-out' }, { id: id('keyframe'), property: 'scale', time_ms: 1800, value: 1.08, easing: 'ease-out', curve: { type: 'spring', stiffness: 210, damping: 18, mass: 1 } }, { id: id('keyframe'), property: 'scale', time_ms: 5000, value: 1, easing: 'ease-in-out' }]; return doc; } },
  { key: 'social_quote', label: 'Social Quote', description: 'Square quote card with portrait and attribution slots.', build: () => baseDocument(1080, 1080, 8000, [track('layer', 'Portrait', [assetSlotClip('Hero Image', 0, 8000, { x: -280, scale: 0.55 })]), track('text', 'Quote', [textClip({ text: '“Your quote here.”', slot: 'Body Copy', startMs: 0, durationMs: 8000, fontSize: 60, x: 230 }), textClip({ text: 'Name · Role', slot: 'Headline', startMs: 1000, durationMs: 7000, fontSize: 30, x: 230, y: 250 })])]) },
  { key: 'podcast_clip', label: 'Podcast Clip', description: 'Vertical podcast layout with speaker art, title, and captions.', build: () => baseDocument(1080, 1920, 30000, [track('layer', 'Speaker', [assetSlotClip('Hero Image', 0, 30000, { y: -250, scale: 0.82 })]), track('audio', 'Audio', [assetSlotClip('Music', 0, 30000)]), track('text', 'Title', [textClip({ text: 'Podcast title', slot: 'Headline', startMs: 0, durationMs: 30000, fontSize: 64, y: 650 })]), track('caption', 'Captions')]) },
  { key: 'vertical_product_ad', label: 'Vertical Product Ad', description: 'Fast vertical product ad with hero, headline, and CTA.', build: () => baseDocument(1080, 1920, 15000, [track('layer', 'Product', [assetSlotClip('Hero Image', 0, 15000, { scale: 0.9 })]), track('layer', 'Logo', [assetSlotClip('Logo', 0, 15000, { y: -700, scale: 0.24, z: 120 })]), track('text', 'Copy', [textClip({ text: 'Built for you', slot: 'Headline', startMs: 500, durationMs: 12000, fontSize: 84, y: 520 }), textClip({ text: 'Shop now', slot: 'CTA', startMs: 6000, durationMs: 8500, fontSize: 42, background: '#ffffff', y: 720 })])]) },
  { key: 'lower_third_pack', label: 'Lower Third Pack', description: 'Reusable lower thirds with replaceable name and role.', build: () => baseDocument(1920, 1080, 12000, [track('text', 'Lower thirds', [textClip({ text: 'Name', slot: 'Headline', startMs: 1000, durationMs: 5000, fontSize: 48, background: '#111111', align: 'left', x: -520, y: 360 }), textClip({ text: 'Role / company', slot: 'Body Copy', startMs: 1000, durationMs: 5000, fontSize: 28, align: 'left', x: -520, y: 420 })])]) },
  { key: 'before_after', label: 'Before / After', description: 'Side-by-side comparison with two replaceable visuals.', build: () => baseDocument(1920, 1080, 10000, [track('layer', 'Before', [assetSlotClip('Before', 0, 10000, { x: -480, scale: 0.49 })]), track('layer', 'After', [assetSlotClip('After', 0, 10000, { x: 480, scale: 0.49 })]), track('text', 'Labels', [textClip({ text: 'BEFORE', startMs: 0, durationMs: 10000, x: -480, y: -430, fontSize: 36 }), textClip({ text: 'AFTER', startMs: 0, durationMs: 10000, x: 480, y: -430, fontSize: 36 })])]) },
  { key: 'cinematic_title', label: 'Cinematic Title', description: 'Wide title treatment with grain and slow camera push.', build: () => { const doc = baseDocument(1920, 1080, 9000, [track('layer', 'Background', [assetSlotClip('Hero Image', 0, 9000)]), track('text', 'Title', [textClip({ text: 'CINEMATIC TITLE', slot: 'Headline', startMs: 500, durationMs: 8000, fontSize: 110 })])]); doc.scenes = [{ id: id('scene'), name: 'Title', start_ms: 0, duration_ms: 9000, camera: { field_of_view: 45, keyframes: [{ id: id('camera-keyframe'), property: 'z', time_ms: 0, value: 0, easing: 'ease-in-out' }, { id: id('camera-keyframe'), property: 'z', time_ms: 9000, value: 140, easing: 'ease-in-out' }] }, effects: [{ id: id('effect'), type: 'film_grain', enabled: true, params: { amount: 7 } }, { id: id('effect'), type: 'edge_fade', enabled: true, params: { amount: 0.35 } }] }]; return doc; } },
  { key: 'photo_slideshow_motion', label: 'Photo Slideshow', description: 'Five replaceable photos with slow zoom motion.', build: () => baseDocument(1920, 1080, 20000, [track('layer', 'Photos', [0, 1, 2, 3, 4].map((index) => assetSlotClip(`Photo ${index + 1}`, index * 4000, 4000))), track('music', 'Music')]) },
  {
    key: 'blank_16_9',
    label: 'Blank 16:9',
    description: 'Standard widescreen project with four generic layers.',
    build: () =>
      baseDocument(1920, 1080, 30000, [
        track('layer', 'Layer 1'),
        track('layer', 'Layer 2'),
        track('layer', 'Layer 3'),
        track('layer', 'Layer 4'),
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
