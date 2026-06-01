import { Eye, EyeOff, Lock, Unlock, Volume2, VolumeX } from 'lucide-react';
import type { VideoAsset, VideoTimelineClip, VideoTimelineTrack as Track, VideoTimelineTrackType } from '../../../types/video';
import { TimelineClip } from './TimelineClip';

export function TimelineTrack({
  track,
  assets,
  selectedClipId,
  pxPerMs,
  width,
  onMoveClip,
  onTrimClip,
  onAddAsset,
  onSelectClip,
  onToggleMute,
  onToggleLock,
  onToggleVisibility,
}: {
  track: Track;
  assets: VideoAsset[];
  selectedClipId: string | null;
  pxPerMs: number;
  width: number;
  onMoveClip: (clipId: string, trackId: string, startMs: number) => void;
  onTrimClip: (clipId: string, updates: Partial<Pick<VideoTimelineClip, 'start_ms' | 'duration_ms' | 'trim_in_ms' | 'trim_out_ms'>>) => void;
  onAddAsset: (assetId: string, trackId: string, trackType: VideoTimelineTrackType, startMs: number) => void;
  onSelectClip: (clipId: string, trackId: string) => void;
  onToggleMute: (trackId: string) => void;
  onToggleLock: (trackId: string) => void;
  onToggleVisibility: (trackId: string) => void;
}) {
  return (
    <div className="grid min-h-[52px] grid-cols-[116px_minmax(0,1fr)] border-b border-border last:border-b-0">
      <div className="flex min-w-0 items-center gap-1 border-r border-border bg-surface-alt px-2">
        <span className="min-w-0 flex-1 truncate text-[11px] font-medium text-text-secondary">{track.name}</span>
        <button
          onClick={() => onToggleMute(track.id)}
          className="rounded p-1 text-text-muted hover:text-text"
          title={track.muted ? 'Unmute track' : 'Mute track'}
          aria-label={track.muted ? 'Unmute track' : 'Mute track'}
        >
          {track.muted ? <VolumeX size={12} /> : <Volume2 size={12} />}
        </button>
        <button
          onClick={() => onToggleLock(track.id)}
          className="rounded p-1 text-text-muted hover:text-text"
          title={track.locked ? 'Unlock track' : 'Lock track'}
          aria-label={track.locked ? 'Unlock track' : 'Lock track'}
        >
          {track.locked ? <Lock size={12} /> : <Unlock size={12} />}
        </button>
        <button
          onClick={() => onToggleVisibility(track.id)}
          className="rounded p-1 text-text-muted hover:text-text"
          title={track.visible ? 'Hide track' : 'Show track'}
          aria-label={track.visible ? 'Hide track' : 'Show track'}
        >
          {track.visible ? <Eye size={12} /> : <EyeOff size={12} />}
        </button>
      </div>
      <div
        className="relative bg-surface"
        style={{ width }}
        onDragOver={(event) => {
          if (!track.locked && (event.dataTransfer.types.includes('application/x-video-clip-id') || event.dataTransfer.types.includes('application/x-video-asset-id'))) {
            event.preventDefault();
          }
        }}
        onDrop={(event) => {
          event.preventDefault();
          if (track.locked) return;
          const clipId = event.dataTransfer.getData('application/x-video-clip-id');
          const assetId = event.dataTransfer.getData('application/x-video-asset-id');
          const rect = event.currentTarget.getBoundingClientRect();
          const startMs = Math.max(0, Math.round((event.clientX - rect.left) / pxPerMs));
          if (clipId) {
            onMoveClip(clipId, track.id, startMs);
          } else if (assetId) {
            onAddAsset(assetId, track.id, track.type, startMs);
          }
        }}
        onClick={() => onSelectClip('', track.id)}
      >
        {track.clips.map((clip) => (
          <TimelineClip
            key={clip.id}
            clip={clip}
            asset={assets.find((asset) => asset.id === clip.asset_id)}
            selected={selectedClipId === clip.id}
            pxPerMs={pxPerMs}
            trackId={track.id}
            onSelect={onSelectClip}
            onTrim={onTrimClip}
          />
        ))}
      </div>
    </div>
  );
}
