import { useEffect, useState } from 'react';
import { Circle, Files, Scissors } from 'lucide-react';
import { useVideoStudioStore } from '../../stores/videoStudio';
import type { VideoTimelineDocument } from '../../types/video';
import { VideoEditStudioEnhanced } from './VideoEditStudioEnhanced';
import { MediaRelinkLab } from './pro/MediaRelinkLab';
import { RecordingLab } from './pro/RecordingLab';
import { SourceMonitorLab } from './pro/SourceMonitorLab';
import { createTimelineBranch } from './pro/timelineCommandEngine';

function documentBytes(document: VideoTimelineDocument): number {
  return new TextEncoder().encode(JSON.stringify(document)).byteLength;
}

export function fitHistoryToBudget(history: VideoTimelineDocument[], budgetBytes: number): VideoTimelineDocument[] {
  let total = 0;
  const retained: VideoTimelineDocument[] = [];
  for (let index = history.length - 1; index >= 0; index -= 1) {
    const document = history[index];
    const bytes = documentBytes(document);
    if (retained.length > 0 && total + bytes > budgetBytes) break;
    retained.push(document);
    total += bytes;
  }
  return retained.reverse();
}

/**
 * Bound snapshot-based undo/redo memory while the store remains compatible
 * with the current timeline-history contract. The newest snapshot is always
 * retained, even when a single very large timeline exceeds the configured
 * budget. A future timeline schema can replace snapshots with command patches
 * without requiring this compatibility guard.
 */
function useTimelineHistoryBudget() {
  useEffect(() => {
    let previousUndo = useVideoStudioStore.getState().timelineUndoStack;
    let previousRedo = useVideoStudioStore.getState().timelineRedoStack;
    let compacting = false;
    return useVideoStudioStore.subscribe((state) => {
      if (compacting || (state.timelineUndoStack === previousUndo && state.timelineRedoStack === previousRedo)) return;
      previousUndo = state.timelineUndoStack;
      previousRedo = state.timelineRedoStack;
      const configuredMb = Number(window.localStorage.getItem('omnillm-video-history-budget-mb') || 32);
      const budget = Math.max(8, Math.min(256, Number.isFinite(configuredMb) ? configuredMb : 32)) * 1024 * 1024;
      const undo = fitHistoryToBudget(state.timelineUndoStack, Math.round(budget * 0.8));
      const redo = fitHistoryToBudget(state.timelineRedoStack, Math.round(budget * 0.2));
      if (undo.length === state.timelineUndoStack.length && redo.length === state.timelineRedoStack.length) return;
      compacting = true;
      useVideoStudioStore.setState({ timelineUndoStack: undo, timelineRedoStack: redo });
      previousUndo = undo;
      previousRedo = redo;
      compacting = false;
    });
  }, []);
}

/**
 * Coalesce high-frequency rAF playhead writes while media elements continue
 * playing natively. This reduces full editor-store invalidations during
 * playback without throttling explicit seeks, frame steps, pause, or end-of-
 * timeline resets. The original store action is restored on unmount.
 */
function usePlaybackUpdateCoalescing() {
  useEffect(() => {
    const original = useVideoStudioStore.getState().setPlayhead;
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
 * Every assistant apply is preceded by a named timeline version. The existing
 * validated plan application remains authoritative; this wrapper only adds a
 * durable, user-visible rollback point and restores the original action when
 * the editor unmounts.
 */
function useAssistantEditCheckpoints() {
  useEffect(() => {
    const original = useVideoStudioStore.getState().applyAssistantPlan;
    const checkpointed: typeof original = async (operationIndexes) => {
      const state = useVideoStudioStore.getState();
      if (state.timeline && state.assistantPlan) {
        const instruction = state.assistantInstruction.trim();
        const label = instruction ? `Before AI: ${instruction.slice(0, 64)}` : `Before AI edit ${new Date().toLocaleTimeString()}`;
        await createTimelineBranch(label);
      }
      await original(operationIndexes);
    };
    useVideoStudioStore.setState({ applyAssistantPlan: checkpointed });
    return () => {
      if (useVideoStudioStore.getState().applyAssistantPlan === checkpointed) {
        useVideoStudioStore.setState({ applyAssistantPlan: original });
      }
    };
  }, []);
}

/**
 * Top-level Video Edit Studio runtime. It preserves the accepted editor shell,
 * adds the advanced workflow drawer, installs bounded playback UI updates and
 * timeline-history memory, checkpoints assistant edits, and exposes source,
 * recording, and media-relink labs.
 */
export function VideoEditStudioUltimate() {
  usePlaybackUpdateCoalescing();
  useTimelineHistoryBudget();
  useAssistantEditCheckpoints();
  const activeProjectId = useVideoStudioStore((state) => state.activeProjectId);
  const [recordingOpen, setRecordingOpen] = useState(false);
  const [mediaOpen, setMediaOpen] = useState(false);
  const [sourceOpen, setSourceOpen] = useState(false);

  const launcherClass = 'inline-flex min-h-10 min-w-10 items-center justify-center gap-2 rounded-full border bg-surface-raised px-2.5 text-xs font-semibold text-text shadow-xl disabled:cursor-not-allowed disabled:opacity-40 sm:px-3';

  return (
    <div className="relative flex min-h-0 flex-1 flex-col">
      <VideoEditStudioEnhanced />
      <div className="fixed bottom-28 right-4 z-[64] flex items-center gap-2 sm:bottom-14 sm:right-44">
        <button
          type="button"
          onClick={() => setSourceOpen(true)}
          disabled={!activeProjectId}
          className={`${launcherClass} border-primary/30 hover:border-primary/60`}
          aria-label="Open source monitor"
          title={activeProjectId ? 'Mark source in/out and insert or overwrite at the playhead' : 'Create or select a video project first'}
        >
          <Scissors size={12} className="text-primary" />
          <span className="hidden sm:inline">Source</span>
        </button>
        <button
          type="button"
          onClick={() => setMediaOpen(true)}
          disabled={!activeProjectId}
          className={`${launcherClass} border-primary/30 hover:border-primary/60`}
          aria-label="Open media relink lab"
          title={activeProjectId ? 'Inspect, relink, and export project media metadata' : 'Create or select a video project first'}
        >
          <Files size={12} className="text-primary" />
          <span className="hidden sm:inline">Media Lab</span>
        </button>
        <button
          type="button"
          onClick={() => setRecordingOpen(true)}
          disabled={!activeProjectId}
          className={`${launcherClass} border-red-400/30 hover:border-red-400/60`}
          aria-label="Open recording lab"
          title={activeProjectId ? 'Record screen and camera, screen, camera, or voiceover' : 'Create or select a video project first'}
        >
          <Circle size={11} className="text-red-400" fill="currentColor" />
          <span className="hidden sm:inline">Recording Lab</span>
        </button>
      </div>
      <SourceMonitorLab open={sourceOpen} onClose={() => setSourceOpen(false)} />
      <MediaRelinkLab open={mediaOpen} onClose={() => setMediaOpen(false)} />
      <RecordingLab open={recordingOpen} onClose={() => setRecordingOpen(false)} />
    </div>
  );
}
