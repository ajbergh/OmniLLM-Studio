import { useEffect, useMemo } from 'react';
import { useVideoStudioStore } from '../../../stores/videoStudio';
import { TimelinePlayhead } from './TimelinePlayhead';
import { TimelineRuler } from './TimelineRuler';
import { TimelineToolbar } from './TimelineToolbar';
import { TimelineTrack } from './TimelineTrack';

export function VideoTimeline() {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const assets = useVideoStudioStore((state) => state.assets);
  const selectedClipId = useVideoStudioStore((state) => state.selectedClipId);
  const playheadMs = useVideoStudioStore((state) => state.playheadMs);
  const zoom = useVideoStudioStore((state) => state.zoom);
  const isPlaying = useVideoStudioStore((state) => state.isPlaying);
  const snappingEnabled = useVideoStudioStore((state) => state.snappingEnabled);
  const isSavingTimeline = useVideoStudioStore((state) => state.isSavingTimeline);
  const setPlayhead = useVideoStudioStore((state) => state.setPlayhead);
  const setZoom = useVideoStudioStore((state) => state.setZoom);
  const setPlaying = useVideoStudioStore((state) => state.setPlaying);
  const toggleSnapping = useVideoStudioStore((state) => state.toggleSnapping);
  const selectClip = useVideoStudioStore((state) => state.selectClip);
  const moveClip = useVideoStudioStore((state) => state.moveClip);
  const splitClipAtPlayhead = useVideoStudioStore((state) => state.splitClipAtPlayhead);
  const deleteClip = useVideoStudioStore((state) => state.deleteClip);
  const duplicateClip = useVideoStudioStore((state) => state.duplicateClip);
  const saveTimeline = useVideoStudioStore((state) => state.saveTimeline);
  const toggleTrackMute = useVideoStudioStore((state) => state.toggleTrackMute);
  const toggleTrackLock = useVideoStudioStore((state) => state.toggleTrackLock);
  const toggleTrackVisibility = useVideoStudioStore((state) => state.toggleTrackVisibility);

  const pxPerMs = useMemo(() => 0.02 * zoom, [zoom]);
  const width = Math.max(900, (timeline?.duration_ms || 30000) * pxPerMs);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (target?.tagName === 'INPUT' || target?.tagName === 'TEXTAREA' || target?.isContentEditable) return;
      if (event.key === ' ') {
        event.preventDefault();
        setPlaying(!useVideoStudioStore.getState().isPlaying);
      } else if (event.key === 'Delete' || event.key === 'Backspace') {
        void deleteClip();
      } else if (event.key.toLowerCase() === 's' && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        void saveTimeline();
      } else if (event.key.toLowerCase() === 's') {
        void splitClipAtPlayhead();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [deleteClip, saveTimeline, setPlaying, splitClipAtPlayhead]);

  if (!timeline) {
    return (
      <div className="flex h-full items-center justify-center text-xs text-text-muted">
        Timeline will load with the active project.
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col rounded-lg border border-border bg-surface">
      <TimelineToolbar
        isPlaying={isPlaying}
        snappingEnabled={snappingEnabled}
        zoom={zoom}
        isSaving={isSavingTimeline}
        onPlayPause={() => setPlaying(!isPlaying)}
        onSplit={() => { void splitClipAtPlayhead(); }}
        onDelete={() => { void deleteClip(); }}
        onDuplicate={() => { void duplicateClip(); }}
        onSave={() => { void saveTimeline(); }}
        onZoom={setZoom}
        onToggleSnap={toggleSnapping}
      />
      <div className="min-h-0 flex-1 overflow-auto">
        <div className="grid grid-cols-[116px_minmax(0,1fr)]">
          <div className="h-8 border-b border-r border-border bg-surface-alt" />
          <div className="relative">
            <TimelineRuler durationMs={timeline.duration_ms} pxPerMs={pxPerMs} onSeek={setPlayhead} />
            <TimelinePlayhead x={playheadMs * pxPerMs} />
          </div>
        </div>
        <div className="grid grid-cols-[116px_minmax(0,1fr)]">
          <div className="border-r border-border bg-surface-alt" />
          <div className="relative">
            <TimelinePlayhead x={playheadMs * pxPerMs} />
          </div>
        </div>
        {timeline.tracks.map((track) => (
          <TimelineTrack
            key={track.id}
            track={track}
            assets={assets}
            selectedClipId={selectedClipId}
            pxPerMs={pxPerMs}
            width={width}
            onMoveClip={(clipId, trackId, startMs) => { void moveClip(clipId, trackId, startMs); }}
            onSelectClip={(clipId, trackId) => selectClip(clipId || null, clipId ? trackId : null)}
            onToggleMute={(trackId) => { void toggleTrackMute(trackId); }}
            onToggleLock={(trackId) => { void toggleTrackLock(trackId); }}
            onToggleVisibility={(trackId) => { void toggleTrackVisibility(trackId); }}
          />
        ))}
      </div>
    </div>
  );
}
