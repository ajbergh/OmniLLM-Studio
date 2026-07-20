import { toast } from 'sonner';
import { useVideoStudioStore } from '../../../stores/videoStudio';
import type {
  VideoAsset,
  VideoExportSettings,
  VideoTimelineClip,
  VideoTimelineDocument,
  VideoTimelineTrack,
} from '../../../types/video';

const HISTORY_LIMIT = 50;
const MIN_CLIP_DURATION_MS = 100;

export interface TimelineBranchRecord {
  id: string;
  name: string;
  created_at: string;
  updated_at: string;
  document: VideoTimelineDocument;
}

export interface TimelineCommandResult {
  changed: boolean;
  message?: string;
  selectedClipId?: string | null;
  selectedClipIds?: string[];
  selectedTrackId?: string | null;
  playheadMs?: number;
}

type VideoStudioSnapshot = ReturnType<typeof useVideoStudioStore.getState>;
type TimelineMutator = (document: VideoTimelineDocument, state: VideoStudioSnapshot) => TimelineCommandResult;

function cloneDocument(document: VideoTimelineDocument): VideoTimelineDocument {
  return JSON.parse(JSON.stringify(document)) as VideoTimelineDocument;
}

function id(prefix: string): string {
  return `${prefix}-${globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(16).slice(2)}`}`;
}

function allClips(document: VideoTimelineDocument): Array<{ track: VideoTimelineTrack; clip: VideoTimelineClip }> {
  return document.tracks.flatMap((track) => track.clips.map((clip) => ({ track, clip })));
}

function findClip(document: VideoTimelineDocument, clipId: string): { track: VideoTimelineTrack; clip: VideoTimelineClip; index: number } | null {
  for (const track of document.tracks) {
    const index = track.clips.findIndex((clip) => clip.id === clipId);
    if (index >= 0) return { track, clip: track.clips[index], index };
  }
  return null;
}

function assetById(assets: VideoAsset[]): Map<string, VideoAsset> {
  return new Map(assets.map((asset) => [asset.id, asset]));
}

function recomputeDuration(document: VideoTimelineDocument): void {
  let end = 1_000;
  for (const { clip } of allClips(document)) {
    end = Math.max(end, clip.start_ms + clip.duration_ms);
  }
  document.duration_ms = Math.max(end, document.duration_ms || 0);
}

function stripBranches(document: VideoTimelineDocument): VideoTimelineDocument {
  const copy = cloneDocument(document);
  const metadata = { ...(copy.metadata || {}) };
  delete metadata.timeline_branches;
  delete metadata.active_timeline_branch_id;
  copy.metadata = metadata;
  return copy;
}

function branchesFrom(document: VideoTimelineDocument): TimelineBranchRecord[] {
  const value = document.metadata?.timeline_branches;
  if (!Array.isArray(value)) return [];
  return value.filter((entry): entry is TimelineBranchRecord => {
    if (!entry || typeof entry !== 'object') return false;
    const candidate = entry as Partial<TimelineBranchRecord>;
    return typeof candidate.id === 'string'
      && typeof candidate.name === 'string'
      && Boolean(candidate.document)
      && typeof candidate.document === 'object';
  });
}

export function getTimelineBranches(document: VideoTimelineDocument | null): TimelineBranchRecord[] {
  return document ? branchesFrom(document) : [];
}

export async function commitTimelineCommand(label: string, mutator: TimelineMutator): Promise<boolean> {
  const state = useVideoStudioStore.getState();
  const current = state.timeline;
  if (!current) {
    toast.error('Create or select a video project first');
    return false;
  }

  const previous = cloneDocument(current);
  const next = cloneDocument(current);
  const result = mutator(next, state);
  if (!result.changed) {
    if (result.message) toast.info(result.message);
    return false;
  }

  recomputeDuration(next);
  next.metadata = {
    ...(next.metadata || {}),
    last_editor_command: {
      label,
      at: new Date().toISOString(),
    },
  };

  useVideoStudioStore.setState((latest) => ({
    timeline: next,
    timelineUndoStack: [...latest.timelineUndoStack, previous].slice(-HISTORY_LIMIT),
    timelineRedoStack: [],
    selectedClipId: result.selectedClipId === undefined ? latest.selectedClipId : result.selectedClipId,
    selectedClipIds: result.selectedClipIds === undefined ? latest.selectedClipIds : result.selectedClipIds,
    selectedTrackId: result.selectedTrackId === undefined ? latest.selectedTrackId : result.selectedTrackId,
    playheadMs: result.playheadMs === undefined ? latest.playheadMs : result.playheadMs,
  }));

  await useVideoStudioStore.getState().saveTimeline(next);
  toast.success(result.message || label);
  return true;
}

export async function setTimelineInPoint(timeMs?: number): Promise<boolean> {
  return commitTimelineCommand('Set timeline in point', (document, state) => {
    const next = Math.max(0, Math.round(timeMs ?? state.playheadMs));
    const currentOut = Number(document.metadata?.edit_out_ms || 0);
    document.metadata = {
      ...(document.metadata || {}),
      edit_in_ms: next,
      ...(currentOut > next ? { edit_out_ms: currentOut } : {}),
    };
    if (currentOut > 0 && currentOut <= next) delete document.metadata.edit_out_ms;
    return { changed: true, message: `In point set at ${(next / 1000).toFixed(2)}s` };
  });
}

export async function setTimelineOutPoint(timeMs?: number): Promise<boolean> {
  return commitTimelineCommand('Set timeline out point', (document, state) => {
    const next = Math.min(document.duration_ms, Math.max(0, Math.round(timeMs ?? state.playheadMs)));
    const currentIn = Number(document.metadata?.edit_in_ms || 0);
    if (next <= currentIn) return { changed: false, message: 'Out point must be after the in point' };
    document.metadata = { ...(document.metadata || {}), edit_out_ms: next };
    return { changed: true, message: `Out point set at ${(next / 1000).toFixed(2)}s` };
  });
}

export async function clearTimelineRange(): Promise<boolean> {
  return commitTimelineCommand('Clear timeline range', (document) => {
    const metadata = { ...(document.metadata || {}) };
    const hadRange = metadata.edit_in_ms !== undefined || metadata.edit_out_ms !== undefined;
    delete metadata.edit_in_ms;
    delete metadata.edit_out_ms;
    document.metadata = metadata;
    return { changed: hadRange, message: hadRange ? 'Timeline range cleared' : 'No timeline range is set' };
  });
}

export function timelineRange(document: VideoTimelineDocument | null): { startMs: number; endMs: number } | null {
  if (!document) return null;
  const startMs = Number(document.metadata?.edit_in_ms || 0);
  const endMs = Number(document.metadata?.edit_out_ms || 0);
  return endMs > startMs ? { startMs, endMs } : null;
}

export async function applyTimelineRangeToExport(): Promise<void> {
  const state = useVideoStudioStore.getState();
  const range = timelineRange(state.timeline);
  if (!range) {
    toast.error('Set timeline in and out points first');
    return;
  }
  state.setExportSetting('range_start_ms', range.startMs);
  state.setExportSetting('range_end_ms', range.endMs);
  toast.success('Export range updated from timeline in/out points');
}

export async function slipSelectedClip(deltaMs: number): Promise<boolean> {
  return commitTimelineCommand('Slip selected clip', (document, state) => {
    const clipId = state.selectedClipId;
    if (!clipId) return { changed: false, message: 'Select one media clip first' };
    const location = findClip(document, clipId);
    if (!location || location.track.locked || !location.clip.asset_id) {
      return { changed: false, message: 'The selected clip cannot be slipped' };
    }
    const asset = assetById(state.assets).get(location.clip.asset_id);
    const assetDuration = Number(asset?.duration_ms || 0);
    const sourceSpan = Math.max(MIN_CLIP_DURATION_MS, location.clip.trim_out_ms - location.clip.trim_in_ms);
    const maxIn = assetDuration > 0 ? Math.max(0, assetDuration - sourceSpan) : Number.MAX_SAFE_INTEGER;
    const nextIn = Math.min(maxIn, Math.max(0, Math.round(location.clip.trim_in_ms + deltaMs)));
    const applied = nextIn - location.clip.trim_in_ms;
    if (applied === 0) return { changed: false, message: 'Slip is already at the source boundary' };
    location.clip.trim_in_ms = nextIn;
    location.clip.trim_out_ms = nextIn + sourceSpan;
    return { changed: true, message: `Slipped ${applied > 0 ? 'forward' : 'back'} ${Math.abs(applied) / 1000}s` };
  });
}

export async function slideSelectedClip(deltaMs: number): Promise<boolean> {
  return commitTimelineCommand('Slide selected clip', (document, state) => {
    const clipId = state.selectedClipId;
    if (!clipId) return { changed: false, message: 'Select one clip first' };
    const location = findClip(document, clipId);
    if (!location || location.track.locked) return { changed: false, message: 'The selected layer is locked' };

    const clips = [...location.track.clips].sort((a, b) => a.start_ms - b.start_ms);
    const selectedIndex = clips.findIndex((clip) => clip.id === clipId);
    if (selectedIndex <= 0 || selectedIndex >= clips.length - 1) {
      return { changed: false, message: 'Slide requires clips immediately before and after the selection' };
    }
    const previous = clips[selectedIndex - 1];
    const selected = clips[selectedIndex];
    const following = clips[selectedIndex + 1];
    const contiguousBefore = previous.start_ms + previous.duration_ms === selected.start_ms;
    const contiguousAfter = selected.start_ms + selected.duration_ms === following.start_ms;
    if (!contiguousBefore || !contiguousAfter) {
      return { changed: false, message: 'Slide requires contiguous neighboring clips' };
    }

    const minDelta = -(previous.duration_ms - MIN_CLIP_DURATION_MS);
    const maxDelta = following.duration_ms - MIN_CLIP_DURATION_MS;
    const applied = Math.max(minDelta, Math.min(maxDelta, Math.round(deltaMs)));
    if (applied === 0) return { changed: false, message: 'Slide is already at the neighboring source boundary' };

    previous.duration_ms += applied;
    previous.trim_out_ms += applied;
    selected.start_ms += applied;
    following.start_ms += applied;
    following.duration_ms -= applied;
    following.trim_in_ms += applied;
    return { changed: true, message: `Slid clip ${applied > 0 ? 'right' : 'left'} ${Math.abs(applied) / 1000}s` };
  });
}

export async function rollSelectedBoundary(deltaMs: number, boundary: 'out' | 'in' = 'out'): Promise<boolean> {
  return commitTimelineCommand('Roll edit point', (document, state) => {
    const clipId = state.selectedClipId;
    if (!clipId) return { changed: false, message: 'Select one clip first' };
    const location = findClip(document, clipId);
    if (!location || location.track.locked) return { changed: false, message: 'The selected layer is locked' };

    const clips = [...location.track.clips].sort((a, b) => a.start_ms - b.start_ms);
    const selectedIndex = clips.findIndex((clip) => clip.id === clipId);
    const left = boundary === 'out' ? clips[selectedIndex] : clips[selectedIndex - 1];
    const right = boundary === 'out' ? clips[selectedIndex + 1] : clips[selectedIndex];
    if (!left || !right || left.start_ms + left.duration_ms !== right.start_ms) {
      return { changed: false, message: 'Roll requires a contiguous edit point' };
    }

    const minDelta = -(left.duration_ms - MIN_CLIP_DURATION_MS);
    const maxDelta = right.duration_ms - MIN_CLIP_DURATION_MS;
    const applied = Math.max(minDelta, Math.min(maxDelta, Math.round(deltaMs)));
    if (applied === 0) return { changed: false, message: 'Roll is already at the source boundary' };

    left.duration_ms += applied;
    left.trim_out_ms += applied;
    right.start_ms += applied;
    right.duration_ms -= applied;
    right.trim_in_ms += applied;
    return { changed: true, message: `Rolled edit point ${applied > 0 ? 'right' : 'left'} ${Math.abs(applied) / 1000}s` };
  });
}

export async function liftSelectedClips(): Promise<boolean> {
  return commitTimelineCommand('Lift selected clips', (document, state) => {
    const selected = new Set(state.selectedClipIds.length > 0 ? state.selectedClipIds : state.selectedClipId ? [state.selectedClipId] : []);
    if (selected.size === 0) return { changed: false, message: 'Select one or more clips first' };
    let removed = 0;
    for (const track of document.tracks) {
      if (track.locked) continue;
      const before = track.clips.length;
      track.clips = track.clips.filter((clip) => !selected.has(clip.id));
      removed += before - track.clips.length;
    }
    return {
      changed: removed > 0,
      message: removed > 0 ? `Lifted ${removed} clip${removed === 1 ? '' : 's'}` : 'No unlocked selected clips were found',
      selectedClipId: null,
      selectedClipIds: [],
    };
  });
}

export async function extractSelectedClips(): Promise<boolean> {
  return commitTimelineCommand('Extract selected clips', (document, state) => {
    const selected = new Set(state.selectedClipIds.length > 0 ? state.selectedClipIds : state.selectedClipId ? [state.selectedClipId] : []);
    if (selected.size === 0) return { changed: false, message: 'Select one or more clips first' };
    let removed = 0;
    for (const track of document.tracks) {
      if (track.locked) continue;
      const spans = track.clips
        .filter((clip) => selected.has(clip.id))
        .map((clip) => ({ start: clip.start_ms, end: clip.start_ms + clip.duration_ms }))
        .sort((a, b) => a.start - b.start);
      if (spans.length === 0) continue;
      const remaining = track.clips.filter((clip) => !selected.has(clip.id));
      removed += track.clips.length - remaining.length;
      for (const clip of remaining) {
        let shift = 0;
        for (const span of spans) {
          if (clip.start_ms >= span.end) shift += span.end - span.start;
        }
        clip.start_ms = Math.max(0, clip.start_ms - shift);
      }
      track.clips = remaining.sort((a, b) => a.start_ms - b.start_ms);
    }
    return {
      changed: removed > 0,
      message: removed > 0 ? `Extracted ${removed} clip${removed === 1 ? '' : 's'} and closed the gaps` : 'No unlocked selected clips were found',
      selectedClipId: null,
      selectedClipIds: [],
    };
  });
}

export async function normalizeSelectedVolume(target = 1): Promise<boolean> {
  return commitTimelineCommand('Normalize selected volume', (document, state) => {
    const selected = new Set(state.selectedClipIds.length > 0 ? state.selectedClipIds : state.selectedClipId ? [state.selectedClipId] : []);
    if (selected.size === 0) return { changed: false, message: 'Select one or more clips first' };
    let changed = 0;
    for (const { track, clip } of allClips(document)) {
      if (!track.locked && selected.has(clip.id) && clip.volume !== target) {
        clip.volume = target;
        changed += 1;
      }
    }
    return { changed: changed > 0, message: changed > 0 ? `Normalized ${changed} selected clip${changed === 1 ? '' : 's'}` : 'Selected clips are already normalized' };
  });
}

function breakCaption(text: string, maxChars: number): string {
  const paragraphs = text.split(/\n+/).map((part) => part.trim()).filter(Boolean);
  const lines: string[] = [];
  for (const paragraph of paragraphs) {
    const words = paragraph.split(/\s+/);
    let current = '';
    for (const word of words) {
      const candidate = current ? `${current} ${word}` : word;
      if (candidate.length > maxChars && current) {
        lines.push(current);
        current = word;
      } else {
        current = candidate;
      }
    }
    if (current) lines.push(current);
  }
  return lines.join('\n');
}

export async function formatCaptionLines(maxChars = 42): Promise<boolean> {
  return commitTimelineCommand('Format caption lines', (document) => {
    let changed = 0;
    for (const track of document.tracks) {
      if (track.type !== 'caption' || track.locked) continue;
      for (const clip of track.clips) {
        if (!clip.text?.text) continue;
        const formatted = breakCaption(clip.text.text, Math.max(16, maxChars));
        if (formatted !== clip.text.text) {
          clip.text.text = formatted;
          changed += 1;
        }
      }
    }
    return { changed: changed > 0, message: changed > 0 ? `Formatted ${changed} caption${changed === 1 ? '' : 's'}` : 'Caption lines already fit the selected limit' };
  });
}

export async function createTimelineBranch(name: string): Promise<boolean> {
  return commitTimelineCommand('Create timeline branch', (document) => {
    const now = new Date().toISOString();
    const branch: TimelineBranchRecord = {
      id: id('branch'),
      name: name.trim() || `Version ${branchesFrom(document).length + 1}`,
      created_at: now,
      updated_at: now,
      document: stripBranches(document),
    };
    const branches = [...branchesFrom(document), branch];
    document.metadata = {
      ...(document.metadata || {}),
      timeline_branches: branches,
      active_timeline_branch_id: branch.id,
    };
    return { changed: true, message: `Created timeline branch “${branch.name}”` };
  });
}

export async function saveActiveTimelineBranch(): Promise<boolean> {
  return commitTimelineCommand('Save active timeline branch', (document) => {
    const activeId = typeof document.metadata?.active_timeline_branch_id === 'string'
      ? document.metadata.active_timeline_branch_id
      : '';
    if (!activeId) return { changed: false, message: 'No active timeline branch is selected' };
    const now = new Date().toISOString();
    let found = false;
    const branches = branchesFrom(document).map((branch) => {
      if (branch.id !== activeId) return branch;
      found = true;
      return { ...branch, updated_at: now, document: stripBranches(document) };
    });
    if (!found) return { changed: false, message: 'The active timeline branch no longer exists' };
    document.metadata = { ...(document.metadata || {}), timeline_branches: branches };
    return { changed: true, message: 'Saved the active timeline branch' };
  });
}

export async function switchTimelineBranch(branchId: string): Promise<boolean> {
  const currentState = useVideoStudioStore.getState();
  const current = currentState.timeline;
  if (!current) return false;
  const branch = branchesFrom(current).find((item) => item.id === branchId);
  if (!branch) {
    toast.error('Timeline branch not found');
    return false;
  }
  const previous = cloneDocument(current);
  const next = cloneDocument(branch.document);
  next.metadata = {
    ...(next.metadata || {}),
    timeline_branches: branchesFrom(current),
    active_timeline_branch_id: branch.id,
    last_editor_command: { label: `Switch to ${branch.name}`, at: new Date().toISOString() },
  };
  useVideoStudioStore.setState((latest) => ({
    timeline: next,
    timelineUndoStack: [...latest.timelineUndoStack, previous].slice(-HISTORY_LIMIT),
    timelineRedoStack: [],
    selectedClipId: null,
    selectedClipIds: [],
    selectedTrackId: null,
    playheadMs: 0,
  }));
  await useVideoStudioStore.getState().saveTimeline(next);
  toast.success(`Switched to “${branch.name}”`);
  return true;
}

export async function deleteTimelineBranch(branchId: string): Promise<boolean> {
  return commitTimelineCommand('Delete timeline branch', (document) => {
    const branches = branchesFrom(document);
    const nextBranches = branches.filter((branch) => branch.id !== branchId);
    if (nextBranches.length === branches.length) return { changed: false, message: 'Timeline branch not found' };
    document.metadata = { ...(document.metadata || {}), timeline_branches: nextBranches };
    if (document.metadata.active_timeline_branch_id === branchId) delete document.metadata.active_timeline_branch_id;
    return { changed: true, message: 'Timeline branch deleted' };
  });
}

export async function renderDraftProxy(): Promise<void> {
  const state = useVideoStudioStore.getState();
  if (!state.timeline) {
    toast.error('Create or select a project first');
    return;
  }
  if (state.isRendering) {
    toast.error('Wait for the current render to finish');
    return;
  }
  const previous: VideoExportSettings = { ...state.exportSettings };
  const range = timelineRange(state.timeline);
  state.setExportSetting('format', 'mp4');
  state.setExportSetting('codec', 'h264');
  state.setExportSetting('resolution', '720p');
  state.setExportSetting('preset', undefined);
  state.setExportSetting('quality', 'draft');
  state.setExportSetting('fps', Math.min(30, state.timeline.canvas.fps || 30));
  state.setExportSetting('range_start_ms', range?.startMs || 0);
  state.setExportSetting('range_end_ms', range?.endMs || 0);
  try {
    await state.renderTimeline();
    toast.success(range ? 'Draft proxy render queued for the marked range' : 'Draft proxy render queued');
  } finally {
    for (const [key, value] of Object.entries(previous) as Array<[keyof VideoExportSettings, VideoExportSettings[keyof VideoExportSettings]]>) {
      state.setExportSetting(key, value as never);
    }
  }
}

export function referencedAssetIds(document: VideoTimelineDocument | null): Set<string> {
  const ids = new Set<string>();
  if (!document) return ids;
  for (const { clip } of allClips(document)) {
    if (clip.asset_id) ids.add(clip.asset_id);
  }
  return ids;
}

export async function deleteUnusedProjectAssets(): Promise<number> {
  const state = useVideoStudioStore.getState();
  const used = referencedAssetIds(state.timeline);
  const unused = state.assets.filter((asset) => !used.has(asset.id) && asset.source_type !== 'generation');
  let deleted = 0;
  for (const asset of unused) {
    try {
      await useVideoStudioStore.getState().deleteAsset(asset.id);
      deleted += 1;
    } catch {
      // deleteAsset already reports the concrete error; continue the cleanup pass.
    }
  }
  if (deleted === 0) toast.info('No removable unused uploaded/imported assets were found');
  else toast.success(`Removed ${deleted} unused asset${deleted === 1 ? '' : 's'}`);
  return deleted;
}
