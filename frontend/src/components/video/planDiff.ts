/**
 * Frontend-derived diffs for assistant plan operations: resolves each
 * operation's target against the live timeline and computes a before→after
 * summary so users can judge a plan before applying it. Purely informational
 * — validation and application stay backend-side.
 */
import type { VideoAsset, VideoEditOperation, VideoTimelineDocument } from '../../types/video';

export interface OperationDiff {
  /** Resolved clip/layer/asset name the operation targets. */
  target?: string;
  /** Current value → proposed value, when derivable from the timeline. */
  before?: string;
  after?: string;
}

function formatSeconds(ms: number | undefined): string {
  return ms === undefined ? '—' : `${(ms / 1000).toFixed(1)}s`;
}

function clipLabel(doc: VideoTimelineDocument, clipId: string | undefined, assets: VideoAsset[]): { label: string; trackName: string } | null {
  if (!clipId) return null;
  for (const track of doc.tracks) {
    const clip = track.clips.find((item) => item.id === clipId);
    if (!clip) continue;
    const asset = clip.asset_id ? assets.find((item) => item.id === clip.asset_id) : undefined;
    const label = clip.text?.text || asset?.file_name || clip.shape?.kind || clipId.slice(0, 8);
    return { label, trackName: track.name };
  }
  return null;
}

/**
 * Derives a before→after summary for an assistant plan operation against the
 * current timeline so users can judge each change before applying it.
 */
export function describeOperationDiff(
  operation: VideoEditOperation,
  doc: VideoTimelineDocument | null,
  assets: VideoAsset[],
): OperationDiff {
  if (!doc) return {};
  const located = clipLabel(doc, operation.clip_id, assets);
  const target = located ? `${located.label} (on ${located.trackName})` : undefined;
  const clip = operation.clip_id
    ? doc.tracks.flatMap((track) => track.clips).find((item) => item.id === operation.clip_id)
    : undefined;

  switch (operation.type) {
    case 'set_canvas':
      return {
        before: `${doc.canvas.width}×${doc.canvas.height} @ ${doc.canvas.fps}fps`,
        after: `${operation.width ?? doc.canvas.width}×${operation.height ?? doc.canvas.height} @ ${operation.fps ?? doc.canvas.fps}fps`,
      };
    case 'set_duration':
      return { before: formatSeconds(doc.duration_ms), after: formatSeconds(operation.duration_ms) };
    case 'move_clip':
      return clip
        ? { target, before: `starts ${formatSeconds(clip.start_ms)}`, after: `starts ${formatSeconds(operation.start_ms)}` }
        : { target };
    case 'trim_clip':
      return clip
        ? { target, before: formatSeconds(clip.duration_ms), after: formatSeconds(operation.duration_ms) }
        : { target };
    case 'set_volume':
      return clip
        ? { target, before: `${Math.round((clip.volume ?? 1) * 100)}%`, after: operation.volume !== undefined ? `${Math.round(operation.volume * 100)}%` : '—' }
        : { target };
    case 'set_transform': {
      if (!clip) return { target };
      const transform = clip.transform || { x: 0, y: 0, scale: 1, opacity: 1 };
      const beforeBits: string[] = [];
      const afterBits: string[] = [];
      if (operation.x !== undefined || operation.y !== undefined) {
        beforeBits.push(`x ${Math.round(transform.x ?? 0)}, y ${Math.round(transform.y ?? 0)}`);
        afterBits.push(`x ${Math.round(operation.x ?? transform.x ?? 0)}, y ${Math.round(operation.y ?? transform.y ?? 0)}`);
      }
      if (operation.scale !== undefined) {
        beforeBits.push(`scale ${Math.round((transform.scale ?? 1) * 100)}%`);
        afterBits.push(`scale ${Math.round(operation.scale * 100)}%`);
      }
      if (operation.opacity !== undefined) {
        beforeBits.push(`opacity ${Math.round((transform.opacity ?? 1) * 100)}%`);
        afterBits.push(`opacity ${Math.round(operation.opacity * 100)}%`);
      }
      return { target, before: beforeBits.join(' · ') || undefined, after: afterBits.join(' · ') || undefined };
    }
    case 'delete_clip':
      return clip ? { target, before: `${formatSeconds(clip.start_ms)} → ${formatSeconds(clip.start_ms + clip.duration_ms)}`, after: 'removed' } : { target };
    case 'add_text_clip':
      return { target: operation.track_id ? doc.tracks.find((track) => track.id === operation.track_id)?.name : undefined, after: `“${operation.text || ''}” at ${formatSeconds(operation.start_ms)}` };
    case 'add_asset_clip': {
      const asset = operation.asset_id ? assets.find((item) => item.id === operation.asset_id) : undefined;
      return { target: asset?.file_name, after: `placed at ${formatSeconds(operation.start_ms)}` };
    }
    case 'add_marker':
      return { after: `marker at ${formatSeconds(operation.start_ms)}` };
    default:
      return { target };
  }
}
