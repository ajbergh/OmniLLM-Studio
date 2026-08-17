import type { VideoAsset, VideoTimelineDocument, VideoTimelineTrack } from '../../../types/video';

export interface LargeVideoFixture {
  document: VideoTimelineDocument;
  assets: VideoAsset[];
}

/** Deterministic stress fixture: 40 layers, 2,000 clips, 16k keyframes, scenes,
 * effects, captions, audio, animation provenance, and a camera-enabled scene. */
export function createLargeVideoFixture(): LargeVideoFixture {
  const assets: VideoAsset[] = Array.from({ length: 80 }, (_, index) => ({
    id: `asset-${index}`, project_id: 'performance-project', source_type: 'fixture', kind: index % 5 === 0 ? 'audio' : 'video',
    file_name: `fixture-${index}.mp4`, mime_type: index % 5 === 0 ? 'audio/mpeg' : 'video/mp4', file_path: `fixture-${index}.mp4`, size_bytes: 1024, created_at: '2026-08-17T00:00:00Z',
  }));
  const tracks: VideoTimelineTrack[] = Array.from({ length: 40 }, (_, trackIndex) => ({
    id: `track-${trackIndex}`, type: trackIndex === 1 ? 'caption' : trackIndex % 8 === 0 ? 'audio' : 'layer', name: `Layer ${trackIndex + 1}`, locked: false, muted: false, visible: true,
    clips: Array.from({ length: 50 }, (_, clipIndex) => {
      const id = `clip-${trackIndex}-${clipIndex}`;
      const start = clipIndex * 2200 + (trackIndex % 5) * 120;
      const keyframes = Array.from({ length: 8 }, (_, point) => ({ id: `${id}-key-${point}`, property: (point % 2 === 0 ? 'x' : 'scale') as 'x' | 'scale', time_ms: point * 250, value: point % 2 === 0 ? point * 4 : 1 + point * 0.01, easing: 'ease-in-out' as const }));
      return {
        id, asset_id: tracksAssetId(trackIndex, clipIndex), start_ms: start, duration_ms: 2000, trim_in_ms: 0, trim_out_ms: 2000,
        transform: { x: 0, y: 0, z: trackIndex * 12, scale: 1, rotation: 0, opacity: 1 },
        effects: [{ id: `${id}-effect`, type: 'film_grain' as const, enabled: trackIndex % 6 === 0, params: { amount: 5 } }], transitions: [], keyframes,
        animation_blocks: [{ id: `${id}-block`, block_key: 'drift', family: 'during' as const, start_ms: 0, duration_ms: 2000, params: {}, generated_keyframe_ids: keyframes.slice(0, 2).map((keyframe) => keyframe.id) }],
        ...(trackIndex === 1 ? { text: { text: `Caption ${clipIndex}`, font_size: 44, color: '#fff' } } : {}),
      };
    }),
  }));
  return {
    assets,
    document: {
      version: 1, canvas: { width: 1920, height: 1080, fps: 30, background: '#000000' }, duration_ms: 120_000, tracks, markers: [], metadata: { fixture: 'large-motion-project' },
      scenes: [{ id: 'scene-performance', name: 'Camera scene', start_ms: 0, duration_ms: 120_000, camera: { field_of_view: 50, keyframes: [{ id: 'camera-start', property: 'x', time_ms: 0, value: -100, easing: 'ease-in-out' }, { id: 'camera-end', property: 'x', time_ms: 120_000, value: 100, easing: 'ease-in-out' }] }, effects: [{ id: 'scene-grain', type: 'film_grain', enabled: true, params: { amount: 4 } }] }],
    },
  };
}

function tracksAssetId(trackIndex: number, clipIndex: number): string {
  return `asset-${(trackIndex * 50 + clipIndex) % 80}`;
}
