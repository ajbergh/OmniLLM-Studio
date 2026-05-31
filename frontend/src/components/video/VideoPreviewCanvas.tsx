import { Pause, Play } from 'lucide-react';
import { videoApi } from '../../api';
import { useVideoStudioStore } from '../../stores/videoStudio';

function formatTime(ms: number): string {
  const seconds = Math.floor(ms / 1000);
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${mins}:${secs.toString().padStart(2, '0')}`;
}

export function VideoPreviewCanvas() {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const assets = useVideoStudioStore((state) => state.assets);
  const playheadMs = useVideoStudioStore((state) => state.playheadMs);
  const isPlaying = useVideoStudioStore((state) => state.isPlaying);
  const setPlaying = useVideoStudioStore((state) => state.setPlaying);

  const activeClips = (timeline?.tracks || [])
    .filter((track) => track.visible && !track.muted)
    .flatMap((track) => track.clips.map((clip) => ({ track, clip })))
    .filter(({ clip }) => playheadMs >= clip.start_ms && playheadMs < clip.start_ms + clip.duration_ms);
  const visual = [...activeClips].reverse().find(({ track }) => ['video', 'image', 'text', 'caption', 'callout'].includes(track.type));
  const asset = visual?.clip.asset_id ? assets.find((item) => item.id === visual.clip.asset_id) : undefined;

  return (
    <div className="flex h-full min-h-[320px] flex-col rounded-lg border border-border bg-black">
      <div className="flex items-center justify-between border-b border-white/10 px-3 py-2 text-xs text-white/70">
        <span>Preview</span>
        <span>{timeline ? `${timeline.canvas.width}x${timeline.canvas.height} · ${timeline.canvas.fps}fps` : 'No timeline'}</span>
      </div>
      <div className="relative flex min-h-0 flex-1 items-center justify-center overflow-hidden p-4">
        <div
          className="relative flex max-h-full max-w-full items-center justify-center overflow-hidden border border-white/10 bg-neutral-950"
          style={{
            aspectRatio: timeline ? `${timeline.canvas.width} / ${timeline.canvas.height}` : '16 / 9',
            width: '100%',
            background: timeline?.canvas.background || '#000000',
          }}
        >
          {asset?.mime_type.startsWith('image/') ? (
            <img src={videoApi.downloadUrl(asset.id)} alt={asset.file_name} className="h-full w-full object-contain" />
          ) : visual?.clip.text ? (
            <div className="px-8 text-center text-4xl font-bold text-white drop-shadow">
              {visual.clip.text.text}
            </div>
          ) : asset ? (
            <div className="max-w-md px-6 text-center">
              <div className="mx-auto mb-3 h-14 w-14 rounded-lg border border-white/15 bg-white/5" />
              <p className="truncate text-sm font-medium text-white">{asset.file_name}</p>
              <p className="mt-2 text-xs text-white/55">{asset.kind} · {asset.mime_type}</p>
            </div>
          ) : (
            <div className="text-center text-xs text-white/55">No active visual clip at playhead</div>
          )}
        </div>
      </div>
      <div className="flex items-center gap-3 border-t border-white/10 px-3 py-2">
        <button
          onClick={() => setPlaying(!isPlaying)}
          className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-white/15 bg-white/5 text-white/75 hover:text-white"
          title={isPlaying ? 'Pause preview' : 'Play preview'}
          aria-label={isPlaying ? 'Pause preview' : 'Play preview'}
        >
          {isPlaying ? <Pause size={14} /> : <Play size={14} />}
        </button>
        <span className="text-xs text-white/65">
          {formatTime(playheadMs)} / {formatTime(timeline?.duration_ms || 0)}
        </span>
        <div className="h-1 flex-1 rounded-full bg-white/10">
          <div
            className="h-full rounded-full bg-primary"
            style={{ width: `${timeline ? Math.min(100, (playheadMs / Math.max(1, timeline.duration_ms)) * 100) : 0}%` }}
          />
        </div>
      </div>
    </div>
  );
}
