import { useEffect, useMemo, useRef, useState } from 'react';
import { ChevronLeft, ChevronRight, Pause, Play, Scissors, X } from 'lucide-react';
import { toast } from 'sonner';
import { videoApi } from '../../../api';
import { useVideoStudioStore } from '../../../stores/videoStudio';
import type { VideoTimelineClip } from '../../../types/video';

function formatTime(ms: number): string {
  const total = Math.max(0, ms);
  const minutes = Math.floor(total / 60_000);
  const seconds = Math.floor((total % 60_000) / 1000);
  const millis = Math.floor(total % 1000);
  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}.${String(millis).padStart(3, '0')}`;
}

function timelineClipIds(): Set<string> {
  const timeline = useVideoStudioStore.getState().timeline;
  return new Set(timeline?.tracks.flatMap((track) => track.clips.map((clip) => clip.id)) || []);
}

function newestAddedClip(before: Set<string>, assetId: string): { clip: VideoTimelineClip; trackId: string } | null {
  const timeline = useVideoStudioStore.getState().timeline;
  if (!timeline) return null;
  const candidates = timeline.tracks.flatMap((track) => track.clips
    .filter((clip) => clip.asset_id === assetId && !before.has(clip.id))
    .map((clip) => ({ clip, trackId: track.id })));
  return candidates.sort((a, b) => b.clip.start_ms - a.clip.start_ms)[0] || null;
}

export function SourceMonitorLab({ open, onClose }: { open: boolean; onClose: () => void }) {
  const assets = useVideoStudioStore((state) => state.assets);
  const selectedAssetId = useVideoStudioStore((state) => state.selectedAssetId);
  const selectedTrackId = useVideoStudioStore((state) => state.selectedTrackId);
  const playheadMs = useVideoStudioStore((state) => state.playheadMs);
  const addAssetToTimeline = useVideoStudioStore((state) => state.addAssetToTimeline);
  const overwriteClipAt = useVideoStudioStore((state) => state.overwriteClipAt);
  const trimClip = useVideoStudioStore((state) => state.trimClip);
  const selectAsset = useVideoStudioStore((state) => state.selectAsset);
  const timeline = useVideoStudioStore((state) => state.timeline);

  const [sourceInMs, setSourceInMs] = useState(0);
  const [sourceOutMs, setSourceOutMs] = useState(0);
  const [currentMs, setCurrentMs] = useState(0);
  const [playing, setPlaying] = useState(false);
  const [busy, setBusy] = useState(false);
  const mediaRef = useRef<HTMLMediaElement | null>(null);

  const mediaAssets = useMemo(() => assets.filter((asset) => asset.mime_type.startsWith('video/') || asset.mime_type.startsWith('audio/') || asset.mime_type.startsWith('image/')), [assets]);
  const asset = mediaAssets.find((item) => item.id === selectedAssetId) || mediaAssets[0] || null;
  const durationMs = Number(asset?.duration_ms || 0);
  const sourceDurationMs = Math.max(100, (sourceOutMs || durationMs || 3_000) - sourceInMs);
  const unlockedTracks = timeline?.tracks.filter((track) => !track.locked) || [];
  const targetTrackId = unlockedTracks.some((track) => track.id === selectedTrackId) ? selectedTrackId as string : unlockedTracks[unlockedTracks.length - 1]?.id || '';

  useEffect(() => {
    setSourceInMs(0);
    setCurrentMs(0);
    setSourceOutMs(Number(asset?.duration_ms || 0));
    setPlaying(false);
  }, [asset?.id, asset?.duration_ms]);

  useEffect(() => {
    if (!open) return;
    const media = mediaRef.current;
    if (!media) return;
    const update = () => {
      const next = media.currentTime * 1000;
      setCurrentMs(next);
      if (sourceOutMs > sourceInMs && next >= sourceOutMs) {
        media.pause();
        media.currentTime = sourceInMs / 1000;
        setPlaying(false);
      }
    };
    const stopped = () => setPlaying(false);
    media.addEventListener('timeupdate', update);
    media.addEventListener('pause', stopped);
    media.addEventListener('ended', stopped);
    return () => {
      media.removeEventListener('timeupdate', update);
      media.removeEventListener('pause', stopped);
      media.removeEventListener('ended', stopped);
    };
  }, [open, sourceInMs, sourceOutMs]);

  if (!open) return null;

  const seek = (timeMs: number) => {
    const next = Math.max(0, Math.min(durationMs || Number.MAX_SAFE_INTEGER, timeMs));
    setCurrentMs(next);
    if (mediaRef.current) mediaRef.current.currentTime = next / 1000;
  };

  const toggle = () => {
    const media = mediaRef.current;
    if (!media) return;
    if (media.paused) {
      if (media.currentTime * 1000 < sourceInMs || (sourceOutMs > sourceInMs && media.currentTime * 1000 >= sourceOutMs)) media.currentTime = sourceInMs / 1000;
      void media.play().then(() => setPlaying(true)).catch(() => undefined);
    } else {
      media.pause();
      setPlaying(false);
    }
  };

  const place = async (mode: 'insert' | 'overwrite') => {
    if (!asset || !targetTrackId) {
      toast.error('Choose a source asset and an unlocked target layer');
      return;
    }
    setBusy(true);
    try {
      const before = timelineClipIds();
      if (mode === 'insert') {
        await addAssetToTimeline(asset.id, {
          track_id: targetTrackId,
          start_ms: playheadMs,
          duration_ms: sourceDurationMs,
        });
      } else {
        await overwriteClipAt(asset.id, targetTrackId, playheadMs);
      }
      const added = newestAddedClip(before, asset.id);
      if (added) {
        await trimClip(added.clip.id, {
          start_ms: playheadMs,
          duration_ms: sourceDurationMs,
          trim_in_ms: sourceInMs,
          trim_out_ms: sourceInMs + sourceDurationMs,
        });
        useVideoStudioStore.getState().selectClip(added.clip.id, added.trackId);
      }
      toast.success(`${mode === 'insert' ? 'Inserted' : 'Overwrote with'} source range ${formatTime(sourceInMs)} → ${formatTime(sourceInMs + sourceDurationMs)}`);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="fixed inset-0 z-[120] flex items-center justify-center bg-black/70 p-4" role="dialog" aria-modal="true" aria-label="Source monitor">
      <div className="flex max-h-[calc(100vh-2rem)] w-[min(58rem,100%)] flex-col overflow-hidden rounded-xl border border-border bg-surface-raised shadow-2xl">
        <div className="flex min-h-12 items-center gap-2 border-b border-border px-3">
          <Scissors size={15} className="text-primary" />
          <div className="flex-1 text-sm font-semibold text-text">Source Monitor</div>
          <button type="button" onClick={onClose} className="rounded p-1.5 text-text-muted hover:bg-surface-alt hover:text-text" aria-label="Close source monitor"><X size={16} /></button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-4">
          <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_18rem]">
            <div className="space-y-3">
              <div className="flex min-h-[20rem] items-center justify-center overflow-hidden rounded-lg border border-border bg-black">
                {!asset ? (
                  <div className="text-sm text-white/50">Import or generate media to use the source monitor.</div>
                ) : asset.mime_type.startsWith('video/') ? (
                  <video ref={(node) => { mediaRef.current = node; }} src={videoApi.downloadUrl(asset.id)} preload="metadata" className="max-h-[60vh] w-full object-contain" />
                ) : asset.mime_type.startsWith('audio/') ? (
                  <div className="w-full p-6"><audio ref={(node) => { mediaRef.current = node; }} src={videoApi.downloadUrl(asset.id)} preload="metadata" controls className="w-full" /></div>
                ) : (
                  <img src={videoApi.downloadUrl(asset.id)} alt={asset.file_name} className="max-h-[60vh] w-full object-contain" />
                )}
              </div>
              {asset && !asset.mime_type.startsWith('image/') && (
                <div className="flex flex-wrap items-center gap-2 rounded-lg border border-border bg-surface p-2">
                  <button type="button" onClick={() => seek(currentMs - 1_000 / Math.max(1, timeline?.canvas.fps || 30))} className="rounded border border-border bg-surface-alt p-2 text-text-muted hover:text-text" aria-label="Previous source frame"><ChevronLeft size={14} /></button>
                  <button type="button" onClick={toggle} className="inline-flex min-h-9 items-center gap-1 rounded border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:text-text">{playing ? <Pause size={13} /> : <Play size={13} />}{playing ? 'Pause' : 'Play'}</button>
                  <button type="button" onClick={() => seek(currentMs + 1_000 / Math.max(1, timeline?.canvas.fps || 30))} className="rounded border border-border bg-surface-alt p-2 text-text-muted hover:text-text" aria-label="Next source frame"><ChevronRight size={14} /></button>
                  <span className="ml-auto text-xs tabular-nums text-text-muted">{formatTime(currentMs)} / {formatTime(durationMs)}</span>
                </div>
              )}
            </div>

            <div className="space-y-3">
              <label className="block text-[10px] text-text-muted">Source asset
                <select value={asset?.id || ''} onChange={(event) => selectAsset(event.target.value)} className="mt-1 min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text">
                  {mediaAssets.length === 0 && <option value="">No media</option>}
                  {mediaAssets.map((item) => <option key={item.id} value={item.id}>{item.file_name}</option>)}
                </select>
              </label>
              <div className="grid grid-cols-2 gap-2">
                <label className="text-[10px] text-text-muted">Source In (s)
                  <input type="number" min={0} step={0.01} value={sourceInMs / 1000} onChange={(event) => setSourceInMs(Math.max(0, Math.min(sourceOutMs || durationMs, Number(event.target.value) * 1000 || 0)))} className="mt-1 min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text" />
                </label>
                <label className="text-[10px] text-text-muted">Source Out (s)
                  <input type="number" min={0.1} step={0.01} value={(sourceOutMs || durationMs || 3_000) / 1000} onChange={(event) => setSourceOutMs(Math.max(sourceInMs + 100, Math.min(durationMs || Number.MAX_SAFE_INTEGER, Number(event.target.value) * 1000 || sourceInMs + 100)))} className="mt-1 min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text" />
                </label>
              </div>
              <div className="grid grid-cols-2 gap-2">
                <button type="button" onClick={() => setSourceInMs(Math.round(currentMs))} disabled={!asset} className="min-h-9 rounded-md border border-border bg-surface-alt text-xs text-text-secondary disabled:opacity-40">Mark In</button>
                <button type="button" onClick={() => setSourceOutMs(Math.max(sourceInMs + 100, Math.round(currentMs)))} disabled={!asset} className="min-h-9 rounded-md border border-border bg-surface-alt text-xs text-text-secondary disabled:opacity-40">Mark Out</button>
              </div>
              <div className="rounded-md border border-border bg-surface px-2 py-2 text-[10px] text-text-muted">
                Range {formatTime(sourceInMs)} → {formatTime(sourceInMs + sourceDurationMs)} · {formatTime(sourceDurationMs)}
              </div>
              <label className="block text-[10px] text-text-muted">Target layer
                <select value={targetTrackId} disabled={!targetTrackId} onChange={(event) => useVideoStudioStore.setState({ selectedTrackId: event.target.value })} className="mt-1 min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text">
                  {unlockedTracks.map((track) => <option key={track.id} value={track.id}>{track.name}</option>)}
                </select>
              </label>
              <div className="grid grid-cols-2 gap-2">
                <button type="button" onClick={() => { void place('insert'); }} disabled={busy || !asset || !targetTrackId} className="min-h-10 rounded-md bg-primary text-xs font-semibold text-black disabled:opacity-40">Insert at playhead</button>
                <button type="button" onClick={() => { void place('overwrite'); }} disabled={busy || !asset || !targetTrackId} className="min-h-10 rounded-md border border-primary/40 bg-primary/10 text-xs font-semibold text-primary disabled:opacity-40">Overwrite</button>
              </div>
              <p className="text-[10px] leading-relaxed text-text-muted">Insert preserves existing media placement. Overwrite removes conflicting material on the target layer according to the existing timeline overwrite semantics. Source trim is applied after placement.</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
