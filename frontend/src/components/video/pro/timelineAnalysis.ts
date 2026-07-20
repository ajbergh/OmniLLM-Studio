import type {
  VideoAsset,
  VideoRendererCapabilities,
  VideoTimelineClip,
  VideoTimelineDocument,
  VideoTimelineTrack,
} from '../../../types/video';

export type TimelineIssueSeverity = 'info' | 'warning' | 'error';
export type TimelineIssueCategory = 'performance' | 'timeline' | 'audio' | 'captions' | 'media' | 'export';

export interface TimelineIssue {
  id: string;
  severity: TimelineIssueSeverity;
  category: TimelineIssueCategory;
  title: string;
  detail: string;
  clip_id?: string;
  track_id?: string;
  time_ms?: number;
  fix?: 'remove_track_gaps' | 'normalize_volume' | 'format_captions' | 'select_clip' | 'create_proxy';
}

export interface TimelineMetrics {
  tracks: number;
  clips: number;
  media_clips: number;
  caption_clips: number;
  keyframes: number;
  effects: number;
  transitions: number;
  cursor_events: number;
  duration_ms: number;
  max_visual_overlap: number;
  max_audio_overlap: number;
  unused_assets: number;
  estimated_document_bytes: number;
  estimated_undo_bytes: number;
  complexity_score: number;
}

export interface TimelineAnalysis {
  generated_at: string;
  metrics: TimelineMetrics;
  issues: TimelineIssue[];
  health: 'excellent' | 'good' | 'needs_attention' | 'high_complexity';
}

function issueId(prefix: string, index: number, suffix = ''): string {
  return `${prefix}-${index}${suffix ? `-${suffix}` : ''}`;
}

function isVisualClip(clip: VideoTimelineClip, asset?: VideoAsset): boolean {
  if (clip.audio_only) return false;
  if (clip.text || clip.shape) return true;
  return !asset || !asset.mime_type.startsWith('audio/');
}

function isAudioClip(clip: VideoTimelineClip, asset?: VideoAsset): boolean {
  if (!asset || clip.muted) return false;
  return asset.mime_type.startsWith('audio/') || asset.mime_type.startsWith('video/');
}

function maxOverlap(entries: Array<{ start: number; end: number }>): number {
  const events = entries.flatMap((entry) => [
    { at: entry.start, delta: 1 },
    { at: entry.end, delta: -1 },
  ]).sort((a, b) => a.at - b.at || a.delta - b.delta);
  let active = 0;
  let maximum = 0;
  for (const event of events) {
    active += event.delta;
    maximum = Math.max(maximum, active);
  }
  return maximum;
}

function rendererFeature(capabilities: VideoRendererCapabilities | null | undefined, feature: string) {
  return capabilities?.features.find((item) => item.feature === feature);
}

function trackGaps(track: VideoTimelineTrack): Array<{ start: number; end: number }> {
  const clips = [...track.clips].sort((a, b) => a.start_ms - b.start_ms);
  const gaps: Array<{ start: number; end: number }> = [];
  let cursor = clips[0]?.start_ms || 0;
  for (const clip of clips) {
    if (clip.start_ms > cursor) gaps.push({ start: cursor, end: clip.start_ms });
    cursor = Math.max(cursor, clip.start_ms + clip.duration_ms);
  }
  return gaps;
}

function trackOverlaps(track: VideoTimelineTrack): Array<{ left: VideoTimelineClip; right: VideoTimelineClip; duration: number }> {
  const clips = [...track.clips].sort((a, b) => a.start_ms - b.start_ms);
  const overlaps: Array<{ left: VideoTimelineClip; right: VideoTimelineClip; duration: number }> = [];
  for (let index = 1; index < clips.length; index += 1) {
    const left = clips[index - 1];
    const right = clips[index];
    const duration = left.start_ms + left.duration_ms - right.start_ms;
    if (duration > 0) overlaps.push({ left, right, duration });
  }
  return overlaps;
}

function captionCharactersPerSecond(clip: VideoTimelineClip): number {
  const text = clip.text?.text?.replace(/\s+/g, ' ').trim() || '';
  return text.length / Math.max(0.1, clip.duration_ms / 1000);
}

export function analyzeTimeline(
  document: VideoTimelineDocument | null,
  assets: VideoAsset[],
  capabilities?: VideoRendererCapabilities | null,
  undoDepth = 0,
): TimelineAnalysis {
  if (!document) {
    return {
      generated_at: new Date().toISOString(),
      metrics: {
        tracks: 0,
        clips: 0,
        media_clips: 0,
        caption_clips: 0,
        keyframes: 0,
        effects: 0,
        transitions: 0,
        cursor_events: 0,
        duration_ms: 0,
        max_visual_overlap: 0,
        max_audio_overlap: 0,
        unused_assets: assets.length,
        estimated_document_bytes: 0,
        estimated_undo_bytes: 0,
        complexity_score: 0,
      },
      issues: [],
      health: 'excellent',
    };
  }

  const assetMap = new Map(assets.map((asset) => [asset.id, asset]));
  const referencedAssets = new Set<string>();
  const issues: TimelineIssue[] = [];
  const clips = document.tracks.flatMap((track) => track.clips.map((clip) => ({ track, clip, asset: clip.asset_id ? assetMap.get(clip.asset_id) : undefined })));
  clips.forEach(({ clip }) => { if (clip.asset_id) referencedAssets.add(clip.asset_id); });

  const visualEntries = clips
    .filter(({ track, clip, asset }) => track.visible && isVisualClip(clip, asset))
    .map(({ clip }) => ({ start: clip.start_ms, end: clip.start_ms + clip.duration_ms }));
  const audioEntries = clips
    .filter(({ track, clip, asset }) => !track.muted && isAudioClip(clip, asset))
    .map(({ clip }) => ({ start: clip.start_ms, end: clip.start_ms + clip.duration_ms }));

  document.tracks.forEach((track, trackIndex) => {
    const gaps = trackGaps(track).filter((gap) => gap.end - gap.start >= 250);
    if (gaps.length >= 3) {
      issues.push({
        id: issueId('gaps', trackIndex),
        severity: 'info',
        category: 'timeline',
        title: `${gaps.length} gaps on ${track.name}`,
        detail: `This layer contains ${(gaps.reduce((sum, gap) => sum + gap.end - gap.start, 0) / 1000).toFixed(1)} seconds of empty space.`,
        track_id: track.id,
        time_ms: gaps[0].start,
        fix: 'remove_track_gaps',
      });
    }

    trackOverlaps(track).forEach((overlap, overlapIndex) => {
      issues.push({
        id: issueId('overlap', trackIndex, String(overlapIndex)),
        severity: track.type === 'caption' ? 'warning' : 'info',
        category: track.type === 'caption' ? 'captions' : 'timeline',
        title: track.type === 'caption' ? 'Caption overlap' : 'Overlapping clips on one layer',
        detail: `${overlap.left.id} and ${overlap.right.id} overlap by ${(overlap.duration / 1000).toFixed(2)} seconds.`,
        clip_id: overlap.right.id,
        track_id: track.id,
        time_ms: overlap.right.start_ms,
        fix: 'select_clip',
      });
    });
  });

  clips.forEach(({ track, clip, asset }, index) => {
    if (clip.start_ms < 0 || clip.duration_ms < 100 || clip.start_ms + clip.duration_ms > document.duration_ms + 1) {
      issues.push({
        id: issueId('bounds', index),
        severity: 'error',
        category: 'timeline',
        title: 'Clip timing is outside the timeline',
        detail: `Clip ${clip.id} starts at ${(clip.start_ms / 1000).toFixed(2)}s and lasts ${(clip.duration_ms / 1000).toFixed(2)}s.`,
        clip_id: clip.id,
        track_id: track.id,
        time_ms: Math.max(0, clip.start_ms),
        fix: 'select_clip',
      });
    }

    if (clip.asset_id && !asset) {
      issues.push({
        id: issueId('missing-asset', index),
        severity: 'error',
        category: 'media',
        title: 'Missing project media',
        detail: `Clip ${clip.id} references an asset that is no longer available.`,
        clip_id: clip.id,
        track_id: track.id,
        time_ms: clip.start_ms,
        fix: 'select_clip',
      });
    }

    if ((clip.volume ?? 1) > 1) {
      issues.push({
        id: issueId('gain', index),
        severity: 'warning',
        category: 'audio',
        title: 'Clip gain is above unity',
        detail: `Volume is ${Math.round((clip.volume || 1) * 100)}%; inspect the output for clipping.`,
        clip_id: clip.id,
        track_id: track.id,
        time_ms: clip.start_ms,
        fix: 'normalize_volume',
      });
    }

    if (track.type === 'caption' || asset?.kind === 'caption') {
      const text = clip.text?.text || '';
      const lines = text.split('\n');
      const cps = captionCharactersPerSecond(clip);
      if (lines.length > 2 || lines.some((line) => line.length > 42) || cps > 20) {
        issues.push({
          id: issueId('caption-readability', index),
          severity: 'warning',
          category: 'captions',
          title: 'Caption may be difficult to read',
          detail: `${lines.length} line${lines.length === 1 ? '' : 's'}, longest line ${Math.max(0, ...lines.map((line) => line.length))} characters, ${cps.toFixed(1)} characters/second.`,
          clip_id: clip.id,
          track_id: track.id,
          time_ms: clip.start_ms,
          fix: 'format_captions',
        });
      }
    }

    if ((clip.cursor?.events?.length || 0) > 0 && rendererFeature(capabilities, 'cursor_effects')?.supported === false) {
      issues.push({
        id: issueId('cursor-export', index),
        severity: 'warning',
        category: 'export',
        title: 'Cursor overlay will not export',
        detail: 'Cursor metadata appears in the editor preview, but the current renderer does not draw it into the exported video.',
        clip_id: clip.id,
        track_id: track.id,
        time_ms: clip.start_ms,
      });
    }

    const hasScaleOrOpacityKeys = (clip.keyframes || []).some((keyframe) => keyframe.property === 'scale' || keyframe.property === 'opacity');
    const keyframeSupport = rendererFeature(capabilities, 'keyframes');
    if (hasScaleOrOpacityKeys && keyframeSupport?.partial) {
      issues.push({
        id: issueId('keyframe-export', index),
        severity: 'warning',
        category: 'export',
        title: 'Some keyframes are preview-only',
        detail: keyframeSupport.notes || 'Scale or opacity animation may not match the editor preview at export.',
        clip_id: clip.id,
        track_id: track.id,
        time_ms: clip.start_ms,
      });
    }
  });

  const duplicateGroups = new Map<string, VideoAsset[]>();
  assets.forEach((asset) => {
    const key = `${asset.file_name.toLowerCase()}::${asset.size_bytes}`;
    duplicateGroups.set(key, [...(duplicateGroups.get(key) || []), asset]);
  });
  Array.from(duplicateGroups.values()).filter((group) => group.length > 1).forEach((group, index) => {
    issues.push({
      id: issueId('duplicate-media', index),
      severity: 'info',
      category: 'media',
      title: 'Possible duplicate project media',
      detail: `${group.length} assets share the name “${group[0].file_name}” and the same file size.`,
    });
  });

  const unusedAssets = assets.filter((asset) => !referencedAssets.has(asset.id));
  if (unusedAssets.length > 0) {
    issues.push({
      id: 'unused-media',
      severity: 'info',
      category: 'media',
      title: `${unusedAssets.length} unused project asset${unusedAssets.length === 1 ? '' : 's'}`,
      detail: 'Unused uploads and imports can be removed from the project media bin after review.',
    });
  }

  const estimatedDocumentBytes = new TextEncoder().encode(JSON.stringify(document)).byteLength;
  const metrics: TimelineMetrics = {
    tracks: document.tracks.length,
    clips: clips.length,
    media_clips: clips.filter(({ clip }) => Boolean(clip.asset_id)).length,
    caption_clips: document.tracks.filter((track) => track.type === 'caption').reduce((sum, track) => sum + track.clips.length, 0),
    keyframes: clips.reduce((sum, { clip }) => sum + (clip.keyframes?.length || 0), 0),
    effects: clips.reduce((sum, { clip }) => sum + (clip.effects?.length || 0), 0),
    transitions: clips.reduce((sum, { clip }) => sum + (clip.transitions?.length || 0), 0),
    cursor_events: clips.reduce((sum, { clip }) => sum + (clip.cursor?.events?.length || 0), 0),
    duration_ms: document.duration_ms,
    max_visual_overlap: maxOverlap(visualEntries),
    max_audio_overlap: maxOverlap(audioEntries),
    unused_assets: unusedAssets.length,
    estimated_document_bytes: estimatedDocumentBytes,
    estimated_undo_bytes: estimatedDocumentBytes * undoDepth,
    complexity_score: 0,
  };
  metrics.complexity_score = Math.round(
    metrics.clips
      + metrics.keyframes * 0.2
      + metrics.effects * 2
      + metrics.transitions * 1.5
      + metrics.cursor_events * 0.05
      + Math.max(0, metrics.max_visual_overlap - 2) * 10
      + Math.max(0, metrics.max_audio_overlap - 4) * 5,
  );

  if (metrics.clips > 250 || metrics.max_visual_overlap > 6 || estimatedDocumentBytes > 1_000_000) {
    issues.unshift({
      id: 'proxy-recommended',
      severity: 'warning',
      category: 'performance',
      title: 'Preview proxy recommended',
      detail: `This timeline has a complexity score of ${metrics.complexity_score}. A draft proxy can reduce decoder and compositing pressure while reviewing the edit.`,
      fix: 'create_proxy',
    });
  }

  const errors = issues.filter((issue) => issue.severity === 'error').length;
  const warnings = issues.filter((issue) => issue.severity === 'warning').length;
  const health: TimelineAnalysis['health'] = metrics.complexity_score > 400
    ? 'high_complexity'
    : errors > 0 || warnings > 8
      ? 'needs_attention'
      : warnings > 0 || issues.length > 5
        ? 'good'
        : 'excellent';

  return {
    generated_at: new Date().toISOString(),
    metrics,
    issues,
    health,
  };
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
