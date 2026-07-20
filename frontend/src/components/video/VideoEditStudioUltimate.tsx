import { useEffect, useRef, useState } from 'react';
import { Circle } from 'lucide-react';
import { useVideoStudioStore } from '../../stores/videoStudio';
import { VideoEditStudioEnhanced } from './VideoEditStudioEnhanced';
import { RecordingLab } from './pro/RecordingLab';

/**
 * Coalesce high-frequency rAF playhead writes while media elements continue
 * playing natively. This reduces full editor-store invalidations during
 * playback without throttling explicit seeks, frame steps, pause, or end-of-
 * timeline resets. The original store action is restored on unmount.
 */
function usePlaybackUpdateCoalescing() {
  const originalRef = useRef<ReturnType<typeof useVideoStudioStore.getState>['setPlayhead'] | null>(null);

  useEffect(() => {
    const original = useVideoStudioStore.getState().setPlayhead;
    originalRef.current = original;
    let lastCommittedAt = 0;
    let pendingTime: number | null = null;
    let timer: number | null = null;

    const commitPending = () => {
      timer = null;
      if (pendingTime === null) return;
      const next = pendingTime;
      pendingTime = null;
      const state = useVideoStudioStore.getState();
      const duration = state.timeline?.duration_ms || Number.MAX_SAFE_INTEGER;
      useVideoStudioStore.setState({ playheadMs: Math.max(0, Math.min(duration, next)) });
      lastCommittedAt = performance.now();
    };

    const coalesced = (timeMs: number) => {
      const state = useVideoStudioStore.getState();
      const duration = state.timeline?.duration_ms || Number.MAX_SAFE_INTEGER;
      const clamped = Math.max(0, Math.min(duration, timeMs));
      if (!state.isPlaying || clamped === 0 || clamped >= duration) {
        if (timer !== null) window.clearTimeout(timer);
        timer = null;
        pendingTime = null;
        original(clamped);
        lastCommittedAt = performance.now();
        return;
      }

      const configured = Number(window.localStorage.getItem('omnillm-video-playback-ui-hz') || 30);
      const uiHz = Math.max(15, Math.min(60, Number.isFinite(configured) ? configured : 30));
      const interval = 1000 / uiHz;
      const now = performance.now();
      if (now - lastCommittedAt >= interval) {
        useVideoStudioStore.setState({ playheadMs: clamped });
        lastCommittedAt = now;
        pendingTime = null;
        if (timer !== null) window.clearTimeout(timer);
        timer = null;
        return;
      }
      pendingTime = clamped;
      if (timer === null) timer = window.setTimeout(commitPending, Math.max(0, interval - (now - lastCommittedAt)));
    };

    useVideoStudioStore.setState({ setPlayhead: coalesced });
    return () => {
      if (timer !== null) window.clearTimeout(timer);
      if (useVideoStudioStore.getState().setPlayhead === coalesced) {
        useVideoStudioStore.setState({ setPlayhead: original });
      }
    };
  }, []);
}

/**
 * Top-level Video Edit Studio runtime. It preserves the accepted editor shell,
 * adds the advanced workflow drawer, installs bounded playback UI updates, and
 * exposes a combined screen/camera/voiceover recording lab.
 */
export function VideoEditStudioUltimate() {
  usePlaybackUpdateCoalescing();
  const activeProjectId = useVideoStudioStore((state) => state.activeProjectId);
  const [recordingOpen, setRecordingOpen] = useState(false);

  return (
    <div className="relative flex min-h-0 flex-1 flex-col">
      <VideoEditStudioEnhanced />
      <button
        type="button"
        onClick={() => setRecordingOpen(true)}
        disabled={!activeProjectId}
        className="fixed bottom-14 right-44 z-[64] inline-flex min-h-10 items-center gap-2 rounded-full border border-red-400/30 bg-surface-raised px-3 text-xs font-semibold text-text shadow-xl hover:border-red-400/60 disabled:cursor-not-allowed disabled:opacity-40"
        aria-label="Open recording lab"
        title={activeProjectId ? 'Record screen and camera, screen, camera, or voiceover' : 'Create or select a video project first'}
      >
        <Circle size={11} className="text-red-400" fill="currentColor" />
        Recording Lab
      </button>
      <RecordingLab open={recordingOpen} onClose={() => setRecordingOpen(false)} />
    </div>
  );
}
