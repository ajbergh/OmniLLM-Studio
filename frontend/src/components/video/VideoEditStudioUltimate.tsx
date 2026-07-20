import { useEffect, useState } from 'react';
import { AudioLines, Captions, Circle, Files, MonitorUp, Scissors } from 'lucide-react';
import { useVideoStudioStore } from '../../stores/videoStudio';
import type { VideoTimelineDocument } from '../../types/video';
import { VideoEditStudioEnhanced } from './VideoEditStudioEnhanced';
import { MediaRelinkLab } from './pro/MediaRelinkLab';
import { RecordingLab } from './pro/RecordingLab';
import { AudioProcessingLab } from './pro/AudioProcessingLab';
import { NativeCaptureLab } from './pro/NativeCaptureLab';
import { TranscriptionLab } from './pro/TranscriptionLab';
import { PatchHistory, createTimelinePatch, revertTimelinePatch, applyTimelinePatch } from './pro/patchHistory';
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
 * Replace unbounded document snapshots with compact reversible JSON patches.
 * The existing store actions remain API-compatible: their temporary snapshot
 * push is converted immediately, while one sentinel document keeps existing
 * canUndo/canRedo selectors working.
 */
function usePatchTimelineHistory() {
  useEffect(() => {
    const configuredMb = Number(window.localStorage.getItem('omnillm-video-history-budget-mb') || 32);
    const history = new PatchHistory(Math.max(8, Math.min(256, Number.isFinite(configuredMb) ? configuredMb : 32)) * 1024 * 1024);
    const originalUndo = useVideoStudioStore.getState().undoTimeline;
    const originalRedo = useVideoStudioStore.getState().redoTimeline;
    let undoRef = useVideoStudioStore.getState().timelineUndoStack;
    let redoRef = useVideoStudioStore.getState().timelineRedoStack;
    let applying = false;
    const unsubscribe = useVideoStudioStore.subscribe((state) => {
      if (applying) return;
      if (state.timelineUndoStack === undoRef && state.timelineRedoStack === redoRef) return;
      if (!state.timeline || state.timelineUndoStack.length === 0) {
        if (state.timelineUndoStack.length === 0 && state.timelineRedoStack.length === 0) history.reset();
        undoRef = state.timelineUndoStack; redoRef = state.timelineRedoStack; return;
      }
      const previous = state.timelineUndoStack[state.timelineUndoStack.length - 1];
      const patch = createTimelinePatch(previous, state.timeline);
      history.record(patch);
      applying = true;
      const undoSentinel = history.undo.length ? [previous] : [];
      useVideoStudioStore.setState({ timelineUndoStack: undoSentinel, timelineRedoStack: [] });
      undoRef = undoSentinel; redoRef = [];
      applying = false;
    });
    const undo: typeof originalUndo = async () => {
      const state = useVideoStudioStore.getState(); const patch = history.popUndo();
      if (!state.timeline || !patch) return;
      const next = revertTimelinePatch(state.timeline, patch);
      applying = true; useVideoStudioStore.setState({ timeline: next, timelineUndoStack: history.undo.length ? [next] : [], timelineRedoStack: history.redo.length ? [state.timeline] : [] }); applying = false;
      await useVideoStudioStore.getState().saveTimeline(next);
    };
    const redo: typeof originalRedo = async () => {
      const state = useVideoStudioStore.getState(); const patch = history.popRedo();
      if (!state.timeline || !patch) return;
      const next = applyTimelinePatch(state.timeline, patch);
      applying = true; useVideoStudioStore.setState({ timeline: next, timelineUndoStack: history.undo.length ? [state.timeline] : [], timelineRedoStack: history.redo.length ? [next] : [] }); applying = false;
      await useVideoStudioStore.getState().saveTimeline(next);
    };
    useVideoStudioStore.setState({ undoTimeline: undo, redoTimeline: redo });
    return () => { unsubscribe(); if (useVideoStudioStore.getState().undoTimeline === undo) useVideoStudioStore.setState({ undoTimeline: originalUndo, redoTimeline: originalRedo }); };
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
  usePatchTimelineHistory();
  useAssistantEditCheckpoints();
  const activeProjectId = useVideoStudioStore((state) => state.activeProjectId);
  const [recordingOpen, setRecordingOpen] = useState(false);
  const [mediaOpen, setMediaOpen] = useState(false);
  const [sourceOpen, setSourceOpen] = useState(false);
  const [transcriptionOpen, setTranscriptionOpen] = useState(false);
  const [audioOpen, setAudioOpen] = useState(false);
  const [nativeCaptureOpen, setNativeCaptureOpen] = useState(false);

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
        <button type="button" onClick={() => setTranscriptionOpen(true)} disabled={!activeProjectId} className={`${launcherClass} border-sky-400/30 hover:border-sky-400/60`} aria-label="Open transcription lab" title="Provider-backed transcription and reusable captions"><Captions size={12} className="text-sky-300"/><span className="hidden xl:inline">Transcribe</span></button>
        <button type="button" onClick={() => setAudioOpen(true)} disabled={!activeProjectId} className={`${launcherClass} border-emerald-400/30 hover:border-emerald-400/60`} aria-label="Open audio processing" title="Denoise, EQ, compression, LUFS normalization, and limiting"><AudioLines size={12} className="text-emerald-300"/><span className="hidden xl:inline">Audio</span></button>
        <button type="button" onClick={() => setNativeCaptureOpen(true)} disabled={!activeProjectId} className={`${launcherClass} border-violet-400/30 hover:border-violet-400/60`} aria-label="Open Windows native capture" title="Native Windows screen/audio capture"><MonitorUp size={12} className="text-violet-300"/><span className="hidden xl:inline">Native</span></button>
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
      <TranscriptionLab open={transcriptionOpen} onClose={() => setTranscriptionOpen(false)} />
      <AudioProcessingLab open={audioOpen} onClose={() => setAudioOpen(false)} />
      <NativeCaptureLab open={nativeCaptureOpen} onClose={() => setNativeCaptureOpen(false)} />
    </div>
  );
}
