import { useMemo, useState } from 'react';
import {
  Activity,
  AlertTriangle,
  AudioLines,
  CheckCircle2,
  ChevronDown,
  ChevronUp,
  Files,
  GitBranch,
  Play,
  Scissors,
  Sparkles,
  Subtitles,
  Trash2,
  X,
} from 'lucide-react';
import { toast } from 'sonner';
import { useVideoStudioStore } from '../../stores/videoStudio';
import { VideoEditStudio } from './VideoEditStudio';
import {
  applyTimelineRangeToExport,
  clearTimelineRange,
  createTimelineBranch,
  deleteTimelineBranch,
  deleteUnusedProjectAssets,
  extractSelectedClips,
  formatCaptionLines,
  getTimelineBranches,
  liftSelectedClips,
  normalizeSelectedVolume,
  renderDraftProxy,
  rollSelectedBoundary,
  saveActiveTimelineBranch,
  setTimelineInPoint,
  setTimelineOutPoint,
  slideSelectedClip,
  slipSelectedClip,
  switchTimelineBranch,
  timelineRange,
} from './pro/timelineCommandEngine';
import { analyzeTimeline, formatBytes, type TimelineIssue } from './pro/timelineAnalysis';
import { applyProjectAudioFades, limitProjectGain, normalizeProjectAudio } from './pro/audioTools';
import { cleanCaptionFillers, createCaptionsFromTranscript, replaceCaptionText } from './pro/transcriptTools';

type ProTab = 'edit' | 'analysis' | 'audio' | 'captions' | 'media' | 'versions' | 'performance';

const TABS: Array<{ key: ProTab; label: string; icon: typeof Scissors }> = [
  { key: 'edit', label: 'Edit', icon: Scissors },
  { key: 'analysis', label: 'Analyze', icon: Sparkles },
  { key: 'audio', label: 'Audio', icon: AudioLines },
  { key: 'captions', label: 'Transcript', icon: Subtitles },
  { key: 'media', label: 'Media', icon: Files },
  { key: 'versions', label: 'Versions', icon: GitBranch },
  { key: 'performance', label: 'Performance', icon: Activity },
];

function seconds(ms: number): string {
  return `${(ms / 1000).toFixed(2)}s`;
}

function healthLabel(health: ReturnType<typeof analyzeTimeline>['health']): string {
  if (health === 'high_complexity') return 'High complexity';
  if (health === 'needs_attention') return 'Needs attention';
  return health === 'excellent' ? 'Excellent' : 'Good';
}

function PanelButton({
  children,
  onClick,
  disabled,
  title,
  danger,
}: {
  children: React.ReactNode;
  onClick: () => void;
  disabled?: boolean;
  title?: string;
  danger?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      title={title}
      className={`min-h-9 rounded-md border px-2.5 text-xs font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-40 ${
        danger
          ? 'border-red-400/30 bg-red-400/10 text-red-300 hover:bg-red-400/15'
          : 'border-border bg-surface-alt text-text-secondary hover:border-primary/40 hover:text-text'
      }`}
    >
      {children}
    </button>
  );
}

function Metric({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="rounded-md border border-border bg-surface-alt/70 px-2 py-1.5">
      <div className="text-[9px] font-semibold uppercase tracking-wide text-text-muted">{label}</div>
      <div className="mt-0.5 text-xs font-medium text-text">{value}</div>
    </div>
  );
}

function IssueRow({ issue, onFix }: { issue: TimelineIssue; onFix: (issue: TimelineIssue) => void }) {
  const Icon = issue.severity === 'error' ? X : issue.severity === 'warning' ? AlertTriangle : CheckCircle2;
  return (
    <div className="rounded-md border border-border bg-surface-alt/70 p-2">
      <div className="flex items-start gap-2">
        <Icon
          size={13}
          className={issue.severity === 'error' ? 'mt-0.5 shrink-0 text-red-300' : issue.severity === 'warning' ? 'mt-0.5 shrink-0 text-amber-300' : 'mt-0.5 shrink-0 text-sky-300'}
        />
        <div className="min-w-0 flex-1">
          <div className="text-xs font-medium text-text">{issue.title}</div>
          <div className="mt-0.5 text-[10px] leading-relaxed text-text-muted">{issue.detail}</div>
        </div>
        {(issue.fix || issue.clip_id || issue.time_ms !== undefined) && (
          <button
            type="button"
            onClick={() => onFix(issue)}
            className="shrink-0 rounded border border-border bg-surface px-1.5 py-1 text-[9px] font-semibold uppercase tracking-wide text-text-muted hover:text-text"
          >
            {issue.fix && issue.fix !== 'select_clip' ? 'Fix' : 'Show'}
          </button>
        )}
      </div>
    </div>
  );
}

function VideoProDrawer() {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const assets = useVideoStudioStore((state) => state.assets);
  const selectedClipId = useVideoStudioStore((state) => state.selectedClipId);
  const selectedClipIds = useVideoStudioStore((state) => state.selectedClipIds);
  const selectedTrackId = useVideoStudioStore((state) => state.selectedTrackId);
  const playheadMs = useVideoStudioStore((state) => state.playheadMs);
  const rendererCapabilities = useVideoStudioStore((state) => state.rendererCapabilities);
  const undoDepth = useVideoStudioStore((state) => state.timelineUndoStack.length);
  const isRendering = useVideoStudioStore((state) => state.isRendering);
  const selectClip = useVideoStudioStore((state) => state.selectClip);
  const setPlayhead = useVideoStudioStore((state) => state.setPlayhead);
  const removeAllGaps = useVideoStudioStore((state) => state.removeAllGaps);
  const duckMusicUnderNarration = useVideoStudioStore((state) => state.duckMusicUnderNarration);

  const [open, setOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(false);
  const [tab, setTab] = useState<ProTab>('edit');
  const [deltaSeconds, setDeltaSeconds] = useState(0.25);
  const [branchName, setBranchName] = useState('');
  const [transcript, setTranscript] = useState('');
  const [replaceExistingCaptions, setReplaceExistingCaptions] = useState(false);
  const [captionFind, setCaptionFind] = useState('');
  const [captionReplace, setCaptionReplace] = useState('');
  const [issueFilter, setIssueFilter] = useState<'all' | TimelineIssue['category']>('all');

  const analysis = useMemo(
    () => analyzeTimeline(timeline, assets, rendererCapabilities, undoDepth),
    [assets, rendererCapabilities, timeline, undoDepth],
  );
  const range = timelineRange(timeline);
  const branches = getTimelineBranches(timeline);
  const selectedCount = selectedClipIds.length || (selectedClipId ? 1 : 0);
  const deltaMs = Math.max(10, Math.round(deltaSeconds * 1000));

  const handleIssue = async (issue: TimelineIssue) => {
    if (issue.clip_id) {
      selectClip(issue.clip_id, issue.track_id || null);
    }
    if (issue.time_ms !== undefined) setPlayhead(issue.time_ms);
    if (issue.fix === 'remove_track_gaps' && issue.track_id) await removeAllGaps(issue.track_id);
    if (issue.fix === 'normalize_volume') await normalizeSelectedVolume(1);
    if (issue.fix === 'format_captions') await formatCaptionLines(42);
    if (issue.fix === 'create_proxy') await renderDraftProxy();
  };

  const applySafeAnalysisFixes = async () => {
    const name = `Before auto-fix ${new Date().toLocaleTimeString()}`;
    await createTimelineBranch(name);
    const gapTracks = Array.from(new Set(analysis.issues.filter((issue) => issue.fix === 'remove_track_gaps').map((issue) => issue.track_id).filter(Boolean))) as string[];
    for (const trackId of gapTracks) await useVideoStudioStore.getState().removeAllGaps(trackId);
    await limitProjectGain(1);
    await formatCaptionLines(42);
    toast.success('Safe timeline fixes applied; the previous state is stored as a version');
  };

  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="fixed bottom-14 right-4 z-[65] inline-flex min-h-10 items-center gap-2 rounded-full border border-primary/30 bg-surface-raised px-3 text-xs font-semibold text-text shadow-xl hover:border-primary/60"
        title="Open advanced timeline tools"
        aria-label="Open advanced timeline tools"
      >
        <Sparkles size={14} className="text-primary" />
        Advanced tools
        {analysis.issues.filter((issue) => issue.severity !== 'info').length > 0 && (
          <span className="rounded-full bg-amber-400/15 px-1.5 py-0.5 text-[9px] text-amber-300">
            {analysis.issues.filter((issue) => issue.severity !== 'info').length}
          </span>
        )}
      </button>
    );
  }

  return (
    <aside
      className={`fixed bottom-3 right-3 z-[70] flex w-[min(44rem,calc(100vw-1.5rem))] flex-col overflow-hidden rounded-xl border border-border bg-surface-raised shadow-2xl ${collapsed ? 'max-h-14' : 'max-h-[min(44rem,calc(100vh-6rem))]'}`}
      aria-label="Advanced video editing tools"
    >
      <div className="flex min-h-12 items-center gap-2 border-b border-border px-3">
        <Sparkles size={15} className="text-primary" />
        <div className="min-w-0 flex-1">
          <div className="text-xs font-semibold text-text">Advanced Video Tools</div>
          <div className="truncate text-[9px] text-text-muted">
            {selectedCount > 0 ? `${selectedCount} clip${selectedCount === 1 ? '' : 's'} selected` : 'No clip selected'}
            {selectedTrackId ? ` · layer ${selectedTrackId}` : ''}
          </div>
        </div>
        <button type="button" onClick={() => setCollapsed((value) => !value)} className="rounded p-1.5 text-text-muted hover:bg-surface-alt hover:text-text" aria-label={collapsed ? 'Expand advanced tools' : 'Collapse advanced tools'}>
          {collapsed ? <ChevronUp size={15} /> : <ChevronDown size={15} />}
        </button>
        <button type="button" onClick={() => setOpen(false)} className="rounded p-1.5 text-text-muted hover:bg-surface-alt hover:text-text" aria-label="Close advanced tools">
          <X size={15} />
        </button>
      </div>

      {!collapsed && (
        <>
          <div className="flex shrink-0 gap-1 overflow-x-auto border-b border-border bg-surface px-2 py-1.5">
            {TABS.map(({ key, label, icon: Icon }) => (
              <button
                key={key}
                type="button"
                onClick={() => setTab(key)}
                className={`inline-flex min-h-8 shrink-0 items-center gap-1 rounded-md px-2 text-[10px] font-medium ${tab === key ? 'bg-primary/15 text-primary' : 'text-text-muted hover:bg-surface-alt hover:text-text'}`}
              >
                <Icon size={11} />
                {label}
              </button>
            ))}
          </div>

          <div className="min-h-0 flex-1 overflow-y-auto p-3">
            {tab === 'edit' && (
              <div className="space-y-3">
                <section className="rounded-lg border border-border bg-surface p-3">
                  <div className="mb-2 flex items-center justify-between">
                    <h3 className="text-xs font-semibold text-text">Source-aware edits</h3>
                    <label className="flex items-center gap-1 text-[10px] text-text-muted">
                      Step
                      <input
                        type="number"
                        min={0.01}
                        step={0.05}
                        value={deltaSeconds}
                        onChange={(event) => setDeltaSeconds(Math.max(0.01, Number(event.target.value) || 0.01))}
                        className="h-7 w-20 rounded border border-border bg-surface-alt px-1.5 text-right text-[10px] text-text"
                        aria-label="Professional edit step seconds"
                      />
                      s
                    </label>
                  </div>
                  <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                    <PanelButton onClick={() => { void slipSelectedClip(-deltaMs); }} disabled={!selectedClipId}>Slip −</PanelButton>
                    <PanelButton onClick={() => { void slipSelectedClip(deltaMs); }} disabled={!selectedClipId}>Slip +</PanelButton>
                    <PanelButton onClick={() => { void slideSelectedClip(-deltaMs); }} disabled={!selectedClipId}>Slide −</PanelButton>
                    <PanelButton onClick={() => { void slideSelectedClip(deltaMs); }} disabled={!selectedClipId}>Slide +</PanelButton>
                    <PanelButton onClick={() => { void rollSelectedBoundary(-deltaMs); }} disabled={!selectedClipId}>Roll −</PanelButton>
                    <PanelButton onClick={() => { void rollSelectedBoundary(deltaMs); }} disabled={!selectedClipId}>Roll +</PanelButton>
                    <PanelButton onClick={() => { void liftSelectedClips(); }} disabled={selectedCount === 0}>Lift</PanelButton>
                    <PanelButton onClick={() => { void extractSelectedClips(); }} disabled={selectedCount === 0}>Extract</PanelButton>
                  </div>
                  <p className="mt-2 text-[10px] leading-relaxed text-text-muted">
                    Slip changes the source window without moving the clip. Slide preserves the selected duration while trimming contiguous neighbors. Roll moves a contiguous edit point. Extract closes removed time per unlocked layer.
                  </p>
                </section>

                <section className="rounded-lg border border-border bg-surface p-3">
                  <div className="mb-2 flex items-center justify-between">
                    <h3 className="text-xs font-semibold text-text">Timeline in/out</h3>
                    <span className="text-[10px] text-text-muted">Playhead {seconds(playheadMs)}</span>
                  </div>
                  {range ? (
                    <div className="mb-2 rounded-md border border-primary/20 bg-primary/5 px-2 py-1.5 text-[10px] text-text-secondary">
                      {seconds(range.startMs)} → {seconds(range.endMs)} · {seconds(range.endMs - range.startMs)}
                    </div>
                  ) : (
                    <div className="mb-2 rounded-md border border-dashed border-border px-2 py-1.5 text-[10px] text-text-muted">No marked range</div>
                  )}
                  <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                    <PanelButton onClick={() => { void setTimelineInPoint(); }}>Set In</PanelButton>
                    <PanelButton onClick={() => { void setTimelineOutPoint(); }}>Set Out</PanelButton>
                    <PanelButton onClick={() => { void applyTimelineRangeToExport(); }} disabled={!range}>Use for export</PanelButton>
                    <PanelButton onClick={() => { void clearTimelineRange(); }} disabled={!range}>Clear</PanelButton>
                  </div>
                </section>
              </div>
            )}

            {tab === 'analysis' && (
              <div className="space-y-3">
                <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                  <Metric label="Health" value={healthLabel(analysis.health)} />
                  <Metric label="Issues" value={analysis.issues.length} />
                  <Metric label="Complexity" value={analysis.metrics.complexity_score} />
                  <Metric label="Max video layers" value={analysis.metrics.max_visual_overlap} />
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <select
                    value={issueFilter}
                    onChange={(event) => setIssueFilter(event.target.value as typeof issueFilter)}
                    className="min-h-9 rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
                    aria-label="Timeline issue category"
                  >
                    <option value="all">All findings</option>
                    <option value="performance">Performance</option>
                    <option value="timeline">Timeline</option>
                    <option value="audio">Audio</option>
                    <option value="captions">Captions</option>
                    <option value="media">Media</option>
                    <option value="export">Export fidelity</option>
                  </select>
                  <PanelButton onClick={() => { void applySafeAnalysisFixes(); }} disabled={!timeline || analysis.issues.length === 0}>Create version + safe fixes</PanelButton>
                </div>
                <div className="space-y-2">
                  {analysis.issues.filter((issue) => issueFilter === 'all' || issue.category === issueFilter).map((issue) => (
                    <IssueRow key={issue.id} issue={issue} onFix={(item) => { void handleIssue(item); }} />
                  ))}
                  {analysis.issues.filter((issue) => issueFilter === 'all' || issue.category === issueFilter).length === 0 && (
                    <div className="rounded-md border border-dashed border-border p-4 text-center text-xs text-text-muted">No findings in this category.</div>
                  )}
                </div>
              </div>
            )}

            {tab === 'audio' && (
              <div className="space-y-3">
                <section className="rounded-lg border border-border bg-surface p-3">
                  <h3 className="mb-2 text-xs font-semibold text-text">Project audio finish</h3>
                  <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
                    <PanelButton onClick={() => { void normalizeProjectAudio(1); }}>Normalize clips</PanelButton>
                    <PanelButton onClick={() => { void limitProjectGain(1); }}>Limit gain to 100%</PanelButton>
                    <PanelButton onClick={() => { void applyProjectAudioFades(250); }}>250ms edge fades</PanelButton>
                    <PanelButton onClick={() => { void duckMusicUnderNarration(); }}>Duck music</PanelButton>
                    <PanelButton onClick={() => { void normalizeSelectedVolume(1); }} disabled={selectedCount === 0}>Normalize selection</PanelButton>
                  </div>
                  <p className="mt-2 text-[10px] leading-relaxed text-text-muted">
                    These operations are non-destructive timeline edits and each creates one undo entry. Loudness-unit normalization and noise reduction still require an analysis/render pass; the gain ceiling prevents obvious over-unity values now.
                  </p>
                </section>
                <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                  <Metric label="Audio overlap" value={analysis.metrics.max_audio_overlap} />
                  <Metric label="Gain warnings" value={analysis.issues.filter((issue) => issue.category === 'audio').length} />
                  <Metric label="Selected" value={selectedCount} />
                  <Metric label="Preview only" value="Master gain" />
                </div>
              </div>
            )}

            {tab === 'captions' && (
              <div className="space-y-3">
                <section className="rounded-lg border border-border bg-surface p-3">
                  <h3 className="text-xs font-semibold text-text">Create captions from a transcript</h3>
                  <p className="mt-1 text-[10px] leading-relaxed text-text-muted">
                    Paste a transcript with one caption per line, or prose that can be split into sentences. Segments are distributed across the marked in/out range or the full timeline. This is transcript-to-caption conversion, not speech recognition.
                  </p>
                  <textarea
                    value={transcript}
                    onChange={(event) => setTranscript(event.target.value)}
                    rows={6}
                    placeholder="Paste transcript text…"
                    className="mt-2 w-full resize-y rounded-md border border-border bg-surface-alt px-2 py-2 text-xs text-text focus:border-primary/50 focus:outline-none"
                  />
                  <div className="mt-2 flex flex-wrap items-center gap-2">
                    <label className="flex items-center gap-2 text-[10px] text-text-muted">
                      <input type="checkbox" checked={replaceExistingCaptions} onChange={(event) => setReplaceExistingCaptions(event.target.checked)} />
                      Replace existing captions
                    </label>
                    <PanelButton onClick={() => { void createCaptionsFromTranscript(transcript, replaceExistingCaptions); }} disabled={!transcript.trim()}>Create captions</PanelButton>
                    <PanelButton onClick={() => { void formatCaptionLines(42); }}>Format to 42 chars</PanelButton>
                    <PanelButton onClick={() => { void cleanCaptionFillers(); }}>Clean filler words</PanelButton>
                  </div>
                </section>
                <section className="rounded-lg border border-border bg-surface p-3">
                  <h3 className="mb-2 text-xs font-semibold text-text">Caption find and replace</h3>
                  <div className="grid grid-cols-1 gap-2 sm:grid-cols-[1fr_1fr_auto]">
                    <input value={captionFind} onChange={(event) => setCaptionFind(event.target.value)} placeholder="Find" className="min-h-9 rounded-md border border-border bg-surface-alt px-2 text-xs text-text" />
                    <input value={captionReplace} onChange={(event) => setCaptionReplace(event.target.value)} placeholder="Replace with" className="min-h-9 rounded-md border border-border bg-surface-alt px-2 text-xs text-text" />
                    <PanelButton onClick={() => { void replaceCaptionText(captionFind, captionReplace); }} disabled={!captionFind.trim()}>Replace all</PanelButton>
                  </div>
                </section>
              </div>
            )}

            {tab === 'media' && (
              <div className="space-y-3">
                <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                  <Metric label="Project assets" value={assets.length} />
                  <Metric label="Used in timeline" value={assets.length - analysis.metrics.unused_assets} />
                  <Metric label="Unused" value={analysis.metrics.unused_assets} />
                  <Metric label="Media clips" value={analysis.metrics.media_clips} />
                </div>
                <section className="rounded-lg border border-border bg-surface p-3">
                  <h3 className="mb-1 text-xs font-semibold text-text">Proxy and cleanup</h3>
                  <p className="mb-2 text-[10px] leading-relaxed text-text-muted">
                    Draft proxy uses the existing durable render queue at 720p/H.264 and honors the marked range. Cleanup removes only unused uploads/imports; generated outputs are retained.
                  </p>
                  <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                    <PanelButton onClick={() => { void renderDraftProxy(); }} disabled={isRendering || !timeline}><span className="inline-flex items-center gap-1"><Play size={11} /> Render draft proxy</span></PanelButton>
                    <PanelButton onClick={() => { void deleteUnusedProjectAssets(); }} disabled={analysis.metrics.unused_assets === 0}><span className="inline-flex items-center gap-1"><Trash2 size={11} /> Remove unused imports</span></PanelButton>
                  </div>
                </section>
              </div>
            )}

            {tab === 'versions' && (
              <div className="space-y-3">
                <section className="rounded-lg border border-border bg-surface p-3">
                  <h3 className="mb-2 text-xs font-semibold text-text">Non-destructive timeline versions</h3>
                  <div className="flex gap-2">
                    <input
                      value={branchName}
                      onChange={(event) => setBranchName(event.target.value)}
                      placeholder={`Version ${branches.length + 1}`}
                      className="min-h-9 min-w-0 flex-1 rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
                    />
                    <PanelButton onClick={() => { void createTimelineBranch(branchName).then((created) => { if (created) setBranchName(''); }); }}>Create version</PanelButton>
                    <PanelButton onClick={() => { void saveActiveTimelineBranch(); }} disabled={!timeline?.metadata?.active_timeline_branch_id}>Save active</PanelButton>
                  </div>
                </section>
                <div className="space-y-2">
                  {branches.map((branch) => {
                    const active = timeline?.metadata?.active_timeline_branch_id === branch.id;
                    return (
                      <div key={branch.id} className={`flex items-center gap-2 rounded-md border p-2 ${active ? 'border-primary/40 bg-primary/5' : 'border-border bg-surface-alt/70'}`}>
                        <GitBranch size={13} className={active ? 'text-primary' : 'text-text-muted'} />
                        <div className="min-w-0 flex-1">
                          <div className="truncate text-xs font-medium text-text">{branch.name}</div>
                          <div className="text-[9px] text-text-muted">Updated {new Date(branch.updated_at).toLocaleString()}</div>
                        </div>
                        {!active && <PanelButton onClick={() => { void switchTimelineBranch(branch.id); }}>Open</PanelButton>}
                        <button type="button" onClick={() => { void deleteTimelineBranch(branch.id); }} className="rounded p-1.5 text-text-muted hover:bg-red-400/10 hover:text-red-300" aria-label={`Delete ${branch.name}`}><Trash2 size={12} /></button>
                      </div>
                    );
                  })}
                  {branches.length === 0 && <div className="rounded-md border border-dashed border-border p-4 text-center text-xs text-text-muted">No saved timeline versions yet.</div>}
                </div>
              </div>
            )}

            {tab === 'performance' && (
              <div className="space-y-3">
                <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                  <Metric label="Tracks" value={analysis.metrics.tracks} />
                  <Metric label="Clips" value={analysis.metrics.clips} />
                  <Metric label="Keyframes" value={analysis.metrics.keyframes} />
                  <Metric label="Effects" value={analysis.metrics.effects} />
                  <Metric label="Transitions" value={analysis.metrics.transitions} />
                  <Metric label="Cursor samples" value={analysis.metrics.cursor_events} />
                  <Metric label="Document" value={formatBytes(analysis.metrics.estimated_document_bytes)} />
                  <Metric label="Undo estimate" value={formatBytes(analysis.metrics.estimated_undo_bytes)} />
                </div>
                <section className="rounded-lg border border-border bg-surface p-3 text-[10px] leading-relaxed text-text-muted">
                  <h3 className="mb-1 text-xs font-semibold text-text">Performance guidance</h3>
                  <p>
                    Complexity score: <span className="font-semibold text-text">{analysis.metrics.complexity_score}</span>. Maximum simultaneous visual layers: <span className="font-semibold text-text">{analysis.metrics.max_visual_overlap}</span>. Maximum simultaneous audio sources: <span className="font-semibold text-text">{analysis.metrics.max_audio_overlap}</span>.
                  </p>
                  <p className="mt-2">
                    Use a draft proxy when layered video decoding becomes uneven. Keep undo depth intentional on very large documents; the current editor still stores document snapshots for compatibility. The metrics here provide a deterministic baseline for the command-history and playback-clock migration.
                  </p>
                </section>
              </div>
            )}
          </div>
        </>
      )}
    </aside>
  );
}

/**
 * Progressive enhancement wrapper for the established Video Edit Studio. The
 * original workspace remains authoritative; this adds professional edit
 * commands, deterministic diagnostics, transcript tools, proxy workflows, and
 * non-destructive timeline versions without replacing the accepted layout.
 */
export function VideoEditStudioEnhanced() {
  return (
    <div className="relative flex min-h-0 flex-1 flex-col">
      <VideoEditStudio />
      <VideoProDrawer />
    </div>
  );
}
