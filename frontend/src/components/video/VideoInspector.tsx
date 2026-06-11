/**
 * Right-rail inspector. Two sections selectable via the `section` prop (the
 * studio's rail tabs render them separately): "assistant" — instruction box,
 * recipe library, plan preview with per-operation checkboxes and before→after
 * diffs, storyboard, and variant comparison cards; "properties" — timing,
 * transform + numeric crop, audio/fades, text styling, annotation controls
 * and presets, effect/transition browsers, motion presets, and keyframe rows.
 * Editor-mode feature gates hide whole groups.
 */
import { ArrowDown, ArrowUp, Sparkles, Trash2, Type } from 'lucide-react';
import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import { useVideoStudioStore } from '../../stores/videoStudio';
import { ContextMenu } from '../common/ContextMenu';
import type { ContextMenuEntry } from '../common/ContextMenu';
import { editorModeFeatures } from './editorModes';
import { effectDefinition, numberParam } from './effects/effectRegistry';
import { KEYFRAME_EASINGS, KEYFRAME_PROPERTIES } from './effects/keyframeUtils';
import { transitionDefinition } from './effects/transitionRegistry';
import { ANNOTATION_PRESETS, annotationDefinition } from './effects/annotationRegistry';
import { MOTION_PRESETS } from './effects/motionPresets';
import { AnnotationBrowser, EffectBrowser, TransitionBrowser } from './EffectBrowser';
import { describeOperationDiff } from './planDiff';
import type { VideoTimelineClip, VideoTimelineKeyframe } from '../../types/video';

function selectedClip(): VideoTimelineClip | null {
  const { timeline, selectedClipId } = useVideoStudioStore.getState();
  if (!timeline || !selectedClipId) return null;
  for (const track of timeline.tracks) {
    const clip = track.clips.find((item) => item.id === selectedClipId);
    if (clip) return clip;
  }
  return null;
}

/** HH:MM:SS.cc — hours omitted when zero. */
function formatTimecode(ms: number): string {
  const total = Math.max(0, Math.round(ms));
  const hours = Math.floor(total / 3_600_000);
  const minutes = Math.floor((total % 3_600_000) / 60_000);
  const seconds = Math.floor((total % 60_000) / 1000);
  const centis = Math.floor((total % 1000) / 10);
  const base = `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}.${String(centis).padStart(2, '0')}`;
  return hours > 0 ? `${hours}:${base}` : base;
}

/** Accepts plain seconds ("12.5") or timecode ("1:02", "00:01:02.15"). */
function parseTimecode(input: string): number | null {
  const trimmed = input.trim();
  if (!trimmed) return null;
  const parts = trimmed.split(':').map((part) => part.trim());
  if (parts.length > 3 || parts.some((part) => part === '' || Number.isNaN(Number(part)))) return null;
  let seconds = 0;
  for (const part of parts) {
    seconds = seconds * 60 + Number(part);
  }
  return Math.max(0, Math.round(seconds * 1000));
}

const TEXT_PRESETS: Array<{
  key: string;
  label: string;
  text: Partial<NonNullable<VideoTimelineClip['text']>>;
  position?: (canvas: { width: number; height: number }) => { x: number; y: number };
}> = [
  {
    key: 'title_card',
    label: 'Title card',
    text: { font_size: 96, font_weight: '800', color: '#ffffff', background: undefined, shadow: true, text_align: 'center' },
    position: () => ({ x: 0, y: 0 }),
  },
  {
    key: 'lower_third',
    label: 'Lower third',
    text: { font_size: 40, font_weight: '700', color: '#ffffff', background: '#111111', shadow: false, text_align: 'left' },
    position: (canvas) => ({ x: -Math.round(canvas.width * 0.22), y: Math.round(canvas.height * 0.35) }),
  },
  {
    key: 'subtitle',
    label: 'Subtitle',
    text: { font_size: 48, font_weight: '600', color: '#ffffff', background: undefined, shadow: true, text_align: 'center' },
    position: (canvas) => ({ x: 0, y: Math.round(canvas.height * 0.38) }),
  },
];

// Assistant recipes. Instruction recipes request a validated plan from the
// backend; local recipes run deterministic store actions directly.
const QUICK_WORKFLOWS: Array<
  | { kind: 'instruction'; label: string; instruction: string }
  | { kind: 'local'; label: string; description: string; run: () => void }
> = [
  { kind: 'instruction', label: '30s social cut', instruction: 'Create a 30-second social cut' },
  { kind: 'instruction', label: '15s teaser', instruction: 'Create a 15-second teaser' },
  { kind: 'instruction', label: 'Vertical 9:16', instruction: 'Convert the timeline to vertical 9:16' },
  { kind: 'instruction', label: 'Square 1:1', instruction: 'Convert the timeline to square 1:1' },
  { kind: 'instruction', label: 'Title card', instruction: 'Add a title card at the start' },
  { kind: 'instruction', label: 'Lower third', instruction: 'Add a lower third caption at the playhead' },
  { kind: 'instruction', label: 'Captions', instruction: 'Add captions from the prompt text' },
  { kind: 'instruction', label: 'Tighten pacing', instruction: 'Tighten pacing and remove trailing dead space' },
  { kind: 'instruction', label: 'Remove dead space', instruction: 'Remove dead space between clips and close the gaps' },
  { kind: 'instruction', label: 'Add callouts', instruction: 'Add callouts highlighting the selected clips' },
  { kind: 'instruction', label: 'Normalize audio', instruction: 'Normalize all clip volumes to 100%' },
  { kind: 'instruction', label: 'Prep for YouTube', instruction: 'Prepare the timeline for YouTube: 16:9 canvas, title card, end padding trimmed' },
  { kind: 'instruction', label: 'Prep for Reels', instruction: 'Prepare the timeline for Reels/TikTok: vertical 9:16, max 60 seconds, bold captions' },
  {
    kind: 'local',
    label: 'Duck music',
    description: 'Generates deterministic volume keyframes so music ducks under narration',
    run: () => { void useVideoStudioStore.getState().duckMusicUnderNarration(); },
  },
  {
    kind: 'local',
    label: 'Pan/zoom motion',
    description: 'Applies a Ken Burns motion preset to the selected clip',
    run: () => {
      const { selectedClipId, applyMotionPreset } = useVideoStudioStore.getState();
      if (selectedClipId) void applyMotionPreset(selectedClipId, 'ken_burns');
    },
  },
];

export function VideoInspector({ section = 'all' }: { section?: 'all' | 'properties' | 'assistant' } = {}) {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const playheadMs = useVideoStudioStore((state) => state.playheadMs);
  const selectedClipId = useVideoStudioStore((state) => state.selectedClipId);
  const rendererCapabilities = useVideoStudioStore((state) => state.rendererCapabilities);
  const assistantInstruction = useVideoStudioStore((state) => state.assistantInstruction);
  const assistantPlan = useVideoStudioStore((state) => state.assistantPlan);
  const storyboard = useVideoStudioStore((state) => state.storyboard);
  const socialVariants = useVideoStudioStore((state) => state.socialVariants);
  const setAssistantInstruction = useVideoStudioStore((state) => state.setAssistantInstruction);
  const assets = useVideoStudioStore((state) => state.assets);
  const trimClip = useVideoStudioStore((state) => state.trimClip);
  const updateClipTransform = useVideoStudioStore((state) => state.updateClipTransform);
  const updateClipVolume = useVideoStudioStore((state) => state.updateClipVolume);
  const toggleClipMute = useVideoStudioStore((state) => state.toggleClipMute);
  const updateClipFade = useVideoStudioStore((state) => state.updateClipFade);
  const updateClipText = useVideoStudioStore((state) => state.updateClipText);
  const updateClipShape = useVideoStudioStore((state) => state.updateClipShape);
  const addShapeClip = useVideoStudioStore((state) => state.addShapeClip);
  const applyAnnotationPreset = useVideoStudioStore((state) => state.applyAnnotationPreset);
  const addClipEffect = useVideoStudioStore((state) => state.addClipEffect);
  const toggleClipEffect = useVideoStudioStore((state) => state.toggleClipEffect);
  const removeClipEffect = useVideoStudioStore((state) => state.removeClipEffect);
  const updateClipEffect = useVideoStudioStore((state) => state.updateClipEffect);
  const reorderClipEffect = useVideoStudioStore((state) => state.reorderClipEffect);
  const addClipTransition = useVideoStudioStore((state) => state.addClipTransition);
  const updateClipTransition = useVideoStudioStore((state) => state.updateClipTransition);
  const removeClipTransition = useVideoStudioStore((state) => state.removeClipTransition);
  const addKeyframe = useVideoStudioStore((state) => state.addKeyframe);
  const applyMotionPreset = useVideoStudioStore((state) => state.applyMotionPreset);
  const updateKeyframe = useVideoStudioStore((state) => state.updateKeyframe);
  const removeKeyframe = useVideoStudioStore((state) => state.removeKeyframe);
  const bringClipForward = useVideoStudioStore((state) => state.bringClipForward);
  const sendClipBackward = useVideoStudioStore((state) => state.sendClipBackward);
  const setCanvas = useVideoStudioStore((state) => state.setCanvas);
  const addTextClip = useVideoStudioStore((state) => state.addTextClip);
  const requestStoryboard = useVideoStudioStore((state) => state.requestStoryboard);
  const requestEditPlan = useVideoStudioStore((state) => state.requestEditPlan);
  const requestTimelinePlan = useVideoStudioStore((state) => state.requestTimelinePlan);
  const applyAssistantPlan = useVideoStudioStore((state) => state.applyAssistantPlan);
  const requestSocialVariants = useVideoStudioStore((state) => state.requestSocialVariants);
  const editorMode = useVideoStudioStore((state) => state.editorMode);
  const modeFeatures = editorModeFeatures(editorMode);
  const clip = selectedClip();
  const [rowMenu, setRowMenu] = useState<{ kind: 'effect' | 'transition' | 'keyframe'; id: string; x: number; y: number } | null>(null);
  const [planMenu, setPlanMenu] = useState<{ x: number; y: number } | null>(null);
  const [variantMenu, setVariantMenu] = useState<{ name: string; x: number; y: number } | null>(null);
  const createProjectFromVariant = useVideoStudioStore((state) => state.createProjectFromVariant);
  // Unchecked plan operations are skipped on apply; a fresh plan starts with
  // everything included.
  const [planExcluded, setPlanExcluded] = useState<Set<number>>(new Set());
  useEffect(() => {
    setPlanExcluded(new Set());
  }, [assistantPlan]);
  const planTotal = assistantPlan?.preview?.length ?? assistantPlan?.operations.length ?? 0;
  const planSelectedCount = planTotal - planExcluded.size;
  const applySelectedPlan = () => {
    if (!assistantPlan) return;
    const selected = Array.from({ length: planTotal }, (_, index) => index).filter((index) => !planExcluded.has(index));
    void applyAssistantPlan(selected.length === planTotal ? undefined : selected);
  };

  const showAssistant = section === 'all' || section === 'assistant';
  const showProperties = section === 'all' || section === 'properties';

  return (
    <div className="min-h-0 overflow-y-auto p-3">
      {showAssistant && modeFeatures.assistant && (
      <section className="rounded-lg border border-border bg-surface p-3">
        <div className="mb-3 flex items-center gap-2">
          <Sparkles size={14} className="text-primary" />
          <h2 className="text-sm font-semibold text-text">Assistant</h2>
        </div>
        <textarea
          value={assistantInstruction}
          onChange={(event) => setAssistantInstruction(event.target.value)}
          rows={3}
          className="w-full resize-y rounded-lg border border-border bg-surface-alt px-3 py-2 text-xs text-text focus:outline-none focus:border-primary/50"
          placeholder="Edit request"
        />
        <div className="mt-2 grid grid-cols-2 gap-2">
          <button className="inline-flex min-h-8 items-center justify-center rounded-md border border-border bg-surface-alt px-2 text-xs text-text-secondary hover:text-text" onClick={() => { void requestStoryboard(); }}>Storyboard</button>
          <button className="inline-flex min-h-8 items-center justify-center rounded-md border border-border bg-surface-alt px-2 text-xs text-text-secondary hover:text-text" onClick={() => { void requestTimelinePlan(); }}>Plan</button>
          <button className="inline-flex min-h-8 items-center justify-center rounded-md border border-border bg-surface-alt px-2 text-xs text-text-secondary hover:text-text" onClick={() => { void requestEditPlan(); }}>Edit plan</button>
          <button className="inline-flex min-h-8 items-center justify-center rounded-md border border-border bg-surface-alt px-2 text-xs text-text-secondary hover:text-text" onClick={() => { void requestSocialVariants(); }}>Variants</button>
        </div>
        <div className="mt-2 flex flex-wrap gap-1">
          {QUICK_WORKFLOWS.map((workflow) => (
            <button
              key={workflow.label}
              onClick={() => {
                if (workflow.kind === 'local') {
                  workflow.run();
                } else {
                  setAssistantInstruction(workflow.instruction);
                  void requestEditPlan();
                }
              }}
              className={`rounded-md border px-2 py-1 text-[10px] hover:text-text ${workflow.kind === 'local' ? 'border-emerald-400/30 bg-emerald-400/5 text-emerald-300/80' : 'border-border bg-surface-alt text-text-muted'}`}
              title={workflow.kind === 'local' ? `${workflow.description} (runs instantly, no AI)` : workflow.instruction}
            >
              {workflow.label}
            </button>
          ))}
        </div>
        {assistantPlan && (
          <div
            className="mt-3 rounded-lg border border-primary/25 bg-primary/10 p-2"
            onContextMenu={(event) => {
              event.preventDefault();
              setPlanMenu({ x: event.clientX, y: event.clientY });
            }}
          >
            <p className="text-xs font-medium text-text">{assistantPlan.summary}</p>
            <p className="mt-1 text-[11px] text-text-muted">
              {planSelectedCount} of {planTotal} operation{planTotal === 1 ? '' : 's'} selected
            </p>
            {assistantPlan.preview && assistantPlan.preview.length > 0 && (
              <ul className="mt-1.5 space-y-0.5">
                {assistantPlan.preview.map((line, index) => {
                  const operation = assistantPlan.operations[index];
                  const diff = operation ? describeOperationDiff(operation, timeline, assets) : {};
                  return (
                    <li key={index} className="flex items-start gap-1.5 text-[10px] text-text-secondary">
                      <input
                        type="checkbox"
                        checked={!planExcluded.has(index)}
                        onChange={() => {
                          setPlanExcluded((excluded) => {
                            const next = new Set(excluded);
                            if (next.has(index)) next.delete(index);
                            else next.add(index);
                            return next;
                          });
                        }}
                        className="mt-0.5 shrink-0"
                        aria-label={`Include operation: ${line}`}
                      />
                      <span className={`min-w-0 ${planExcluded.has(index) ? 'opacity-50 line-through' : ''}`}>
                        {line}
                        {(diff.target || diff.before || diff.after) && (
                          <span className="block text-[9px] text-text-muted">
                            {diff.target && <span>{diff.target}</span>}
                            {(diff.before || diff.after) && (
                              <span>{diff.target ? ' · ' : ''}{diff.before ?? '—'} → {diff.after ?? '—'}</span>
                            )}
                          </span>
                        )}
                      </span>
                    </li>
                  );
                })}
              </ul>
            )}
            {assistantPlan.issues && assistantPlan.issues.length > 0 && (
              <ul className="mt-1.5 space-y-0.5">
                {assistantPlan.issues.map((line, index) => (
                  <li key={index} className="text-[10px] text-amber-400/80">⚠ {line} (will be skipped)</li>
                ))}
              </ul>
            )}
            <button
              className="mt-2 min-h-8 rounded-md bg-primary px-2 text-xs font-medium text-white disabled:opacity-50"
              disabled={planSelectedCount === 0}
              onClick={applySelectedPlan}
            >
              {planSelectedCount < planTotal ? `Apply ${planSelectedCount} selected` : 'Apply'}
            </button>
          </div>
        )}
        {storyboard && (
          <div className="mt-3 space-y-2">
            <p className="text-xs font-medium text-text">{storyboard.title}</p>
            {storyboard.scenes.slice(0, 3).map((scene) => (
              <div key={scene.id} className="rounded-md border border-border bg-surface-alt p-2">
                <p className="text-[11px] font-medium text-text">{scene.title}</p>
                <p className="mt-1 line-clamp-2 text-[10px] text-text-muted">{scene.description}</p>
              </div>
            ))}
          </div>
        )}
        {!assistantPlan && !storyboard && socialVariants.length === 0 && (
          <p className="mt-2 rounded-md border border-dashed border-border bg-surface-alt px-2 py-2 text-[11px] text-text-muted">
            Describe an edit above, or pick a recipe — the assistant proposes a reviewable plan; nothing changes until you apply it.
          </p>
        )}
        {socialVariants.length > 0 && (
          <div className="mt-3 space-y-1.5">
            <p className="text-[10px] font-medium uppercase tracking-wide text-text-muted">Variants — compare, then create a project (non-destructive)</p>
            {socialVariants.map((variant) => (
              <button
                key={variant.name}
                onClick={(event) => {
                  event.preventDefault();
                  setVariantMenu({ name: variant.name, x: event.clientX, y: event.clientY });
                }}
                onContextMenu={(event) => {
                  event.preventDefault();
                  setVariantMenu({ name: variant.name, x: event.clientX, y: event.clientY });
                }}
                className="block w-full rounded-md border border-border bg-surface-alt px-2 py-1.5 text-left hover:border-primary/40"
                title="Click for variant actions — applying creates a new project; this timeline is untouched"
              >
                <span className="flex items-center justify-between gap-2">
                  <span className="text-[11px] font-medium text-text">{variant.name}</span>
                  <span className="shrink-0 text-[9px] text-text-muted">{variant.aspect_ratio} · {variant.width}×{variant.height}</span>
                </span>
                <span className="mt-0.5 block truncate text-[10px] text-text-muted">
                  {variant.plan.summary} · {variant.plan.operations.length} operation{variant.plan.operations.length === 1 ? '' : 's'}
                </span>
              </button>
            ))}
          </div>
        )}
      </section>
      )}

      {showProperties && (
      <section className={`${showAssistant ? 'mt-3 ' : ''}rounded-lg border border-border bg-surface p-3`}>
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-sm font-semibold text-text">Inspector</h2>
          <button
            onClick={() => { void addTextClip(); }}
            className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-border bg-surface-alt text-text-muted hover:text-text"
            title="Add text clip"
            aria-label="Add text clip"
          >
            <Type size={14} />
          </button>
        </div>
        {/* Annotation palette — creation works with or without a selection. */}
        {modeFeatures.addTextClip && timeline && (
          <details className="mb-3 rounded-md border border-border bg-surface-alt/40">
            <summary className="cursor-pointer select-none px-2 py-1.5 text-[11px] font-medium text-text-secondary hover:text-text">
              Annotations — add at playhead
            </summary>
            <div className="p-1.5">
              <AnnotationBrowser onAdd={(kind) => { void addShapeClip(kind); }} />
            </div>
          </details>
        )}
        {!clip ? (
          !timeline ? (
            <div className="rounded-lg border border-dashed border-border bg-surface-alt p-4 text-center text-xs text-text-muted">
              No timeline loaded.
            </div>
          ) : !modeFeatures.canvasControls ? (
            <div className="rounded-lg border border-dashed border-border bg-surface-alt p-4 text-center text-xs text-text-muted">
              Select a clip to edit.
            </div>
          ) : (
            <div className="space-y-3">
              <p className="text-[11px] text-text-muted">Select a clip to edit it, or adjust the project canvas.</p>
              <Field label="Canvas preset">
                <div className="flex flex-wrap gap-1">
                  {[
                    { label: '16:9', width: 1920, height: 1080 },
                    { label: '9:16', width: 1080, height: 1920 },
                    { label: '1:1', width: 1080, height: 1080 },
                  ].map((preset) => (
                    <button
                      key={preset.label}
                      className={`rounded-md border px-2 py-1 text-[10px] ${
                        timeline.canvas.width === preset.width && timeline.canvas.height === preset.height
                          ? 'border-primary/40 bg-primary/10 text-primary'
                          : 'border-border bg-surface-alt text-text-muted hover:text-text'
                      }`}
                      onClick={() => { void setCanvas({ width: preset.width, height: preset.height }); }}
                    >
                      {preset.label}
                    </button>
                  ))}
                </div>
              </Field>
              {/* Commit on blur so typing doesn't save + push undo per keystroke. */}
              <div className="grid grid-cols-2 gap-2">
                <Field label="Width">
                  <input
                    type="number"
                    min={16}
                    key={`w-${timeline.canvas.width}`}
                    defaultValue={timeline.canvas.width}
                    onBlur={(event) => {
                      const width = Number(event.target.value);
                      if (Number.isFinite(width) && width !== timeline.canvas.width) void setCanvas({ width });
                    }}
                    onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }}
                    className="min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
                  />
                </Field>
                <Field label="Height">
                  <input
                    type="number"
                    min={16}
                    key={`h-${timeline.canvas.height}`}
                    defaultValue={timeline.canvas.height}
                    onBlur={(event) => {
                      const height = Number(event.target.value);
                      if (Number.isFinite(height) && height !== timeline.canvas.height) void setCanvas({ height });
                    }}
                    onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }}
                    className="min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
                  />
                </Field>
                <Field label="FPS">
                  <input
                    type="number"
                    min={1}
                    max={120}
                    key={`fps-${timeline.canvas.fps}`}
                    defaultValue={timeline.canvas.fps}
                    onBlur={(event) => {
                      const fps = Number(event.target.value);
                      if (Number.isFinite(fps) && fps !== timeline.canvas.fps) void setCanvas({ fps });
                    }}
                    onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }}
                    className="min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
                  />
                </Field>
                <Field label="Background">
                  <input
                    type="color"
                    key={`bg-${timeline.canvas.background}`}
                    defaultValue={/^#[0-9a-fA-F]{6}$/.test(timeline.canvas.background) ? timeline.canvas.background : '#000000'}
                    onBlur={(event) => {
                      if (event.target.value !== timeline.canvas.background) void setCanvas({ background: event.target.value });
                    }}
                    className="h-9 w-full cursor-pointer rounded-md border border-border bg-surface-alt"
                  />
                </Field>
              </div>
            </div>
          )
        ) : (
          <div className="space-y-3">
            {/* Timing accepts timecode (MM:SS.cc / H:MM:SS.cc) or plain seconds. */}
            <div className="grid grid-cols-2 gap-2">
              <TimecodeField
                label="Start"
                valueMs={clip.start_ms}
                onCommit={(ms) => { void trimClip(clip.id, { start_ms: ms }); }}
              />
              <TimecodeField
                label="End"
                valueMs={clip.start_ms + clip.duration_ms}
                onCommit={(ms) => { void trimClip(clip.id, { duration_ms: Math.max(100, ms - clip.start_ms) }); }}
              />
              <TimecodeField
                label="Duration"
                valueMs={clip.duration_ms}
                onCommit={(ms) => { void trimClip(clip.id, { duration_ms: Math.max(100, ms) }); }}
              />
              <TimecodeField
                label="Trim in"
                valueMs={clip.trim_in_ms}
                onCommit={(ms) => { void trimClip(clip.id, { trim_in_ms: ms }); }}
              />
            </div>
            {modeFeatures.transformControls && (
            <Field label={`Layer order: ${clip.z_index ?? 0}`}>
              <div className="flex gap-2">
                <button
                  className="inline-flex min-h-8 flex-1 items-center justify-center gap-1 rounded-md border border-border bg-surface-alt px-2 text-xs text-text-secondary hover:text-text"
                  onClick={() => { void bringClipForward(selectedClipId as string); }}
                >
                  <ArrowUp size={12} />
                  Forward
                </button>
                <button
                  className="inline-flex min-h-8 flex-1 items-center justify-center gap-1 rounded-md border border-border bg-surface-alt px-2 text-xs text-text-secondary hover:text-text"
                  onClick={() => { void sendClipBackward(selectedClipId as string); }}
                >
                  <ArrowDown size={12} />
                  Backward
                </button>
              </div>
            </Field>
            )}
            {modeFeatures.transformControls && clip.transform && (
              <>
                <div className="grid grid-cols-2 gap-2">
                  <Field label="X">
                    <input
                      type="number"
                      step={1}
                      key={`x-${clip.id}-${clip.transform.x}`}
                      defaultValue={Math.round(clip.transform.x)}
                      onBlur={(event) => {
                        const x = Number(event.target.value);
                        if (Number.isFinite(x) && x !== clip.transform?.x) void updateClipTransform(clip.id, { x });
                      }}
                      onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }}
                      className="min-h-8 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
                    />
                  </Field>
                  <Field label="Y">
                    <input
                      type="number"
                      step={1}
                      key={`y-${clip.id}-${clip.transform.y}`}
                      defaultValue={Math.round(clip.transform.y)}
                      onBlur={(event) => {
                        const y = Number(event.target.value);
                        if (Number.isFinite(y) && y !== clip.transform?.y) void updateClipTransform(clip.id, { y });
                      }}
                      onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }}
                      className="min-h-8 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
                    />
                  </Field>
                </div>
                <Slider label="Scale" min={0.25} max={3} step={0.05} value={clip.transform.scale} onChange={(value) => { void updateClipTransform(selectedClipId as string, { scale: value }); }} />
                <Slider label="Opacity" min={0} max={1} step={0.05} value={clip.transform.opacity} onChange={(value) => { void updateClipTransform(selectedClipId as string, { opacity: value }); }} />
                <Slider label="Rotation" min={-180} max={180} step={1} value={clip.transform.rotation} onChange={(value) => { void updateClipTransform(selectedClipId as string, { rotation: value }); }} />
                <div className="flex flex-wrap gap-1">
                  {(() => {
                    const asset = clip.asset_id ? assets.find((item) => item.id === clip.asset_id) : undefined;
                    const fillScale = asset?.width && asset?.height && timeline
                      ? (asset.width / asset.height >= timeline.canvas.width / timeline.canvas.height
                        ? (asset.width / asset.height) / (timeline.canvas.width / timeline.canvas.height)
                        : (timeline.canvas.width / timeline.canvas.height) / (asset.width / asset.height))
                      : 1;
                    return [
                      { label: 'Fit', action: () => { void updateClipTransform(clip.id, { x: 0, y: 0, scale: 1 }); } },
                      { label: 'Fill', action: () => { void updateClipTransform(clip.id, { x: 0, y: 0, scale: fillScale }); } },
                      { label: 'Center', action: () => { void updateClipTransform(clip.id, { x: 0, y: 0 }); } },
                      { label: 'Reset', action: () => { void updateClipTransform(clip.id, { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1, crop: undefined }); } },
                    ].map((button) => (
                      <button
                        key={button.label}
                        onClick={button.action}
                        className="rounded-md border border-border bg-surface-alt px-2 py-1 text-[10px] text-text-muted hover:text-text"
                      >
                        {button.label}
                      </button>
                    ));
                  })()}
                </div>
                {/* Numeric crop — percentages of the source frame per side. */}
                {clip.asset_id && (
                  <Field label="Crop % (top / right / bottom / left)">
                    <div className="grid grid-cols-4 gap-1">
                      {(['top', 'right', 'bottom', 'left'] as const).map((side) => {
                        const crop = clip.transform?.crop;
                        const current = Math.round(((crop?.[side] ?? 0) as number) * 100);
                        return (
                          <input
                            key={side}
                            type="number"
                            min={0}
                            max={95}
                            aria-label={`Crop ${side} percent`}
                            title={`Crop ${side} (%)`}
                            defaultValue={current}
                            // Re-key on external crop changes so canvas edits show up.
                            data-side={side}
                            onBlur={(event) => {
                              const value = Math.max(0, Math.min(95, Math.round(Number(event.target.value))));
                              if (!Number.isFinite(value) || value === current) return;
                              const nextCrop = { top: 0, right: 0, bottom: 0, left: 0, ...(clip.transform?.crop || {}), [side]: value / 100 };
                              const isEmpty = nextCrop.top === 0 && nextCrop.right === 0 && nextCrop.bottom === 0 && nextCrop.left === 0;
                              void updateClipTransform(clip.id, { crop: isEmpty ? undefined : nextCrop });
                            }}
                            onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }}
                            className="min-h-8 w-full rounded-md border border-border bg-surface-alt px-1 text-center text-xs text-text"
                          />
                        );
                      })}
                    </div>
                  </Field>
                )}
              </>
            )}
            {clip.volume !== undefined && (
              <>
                <Slider label="Volume" min={0} max={2} step={0.05} value={clip.volume} onChange={(value) => { void updateClipVolume(selectedClipId as string, value); }} />
                <button
                  onClick={() => { void toggleClipMute(clip.id); }}
                  className={`min-h-8 w-full rounded-md border px-2 text-xs ${clip.muted ? 'border-amber-400/40 bg-amber-400/10 text-amber-300' : 'border-border bg-surface-alt text-text-muted hover:text-text'}`}
                  title="Silence this clip without changing its volume"
                >
                  {clip.muted ? 'Clip muted — click to unmute' : 'Mute clip'}
                </button>
              </>
            )}
            <Slider label="Fade in" min={0} max={5000} step={100} value={clip.fade_in_ms || 0} onChange={(value) => { void updateClipFade(selectedClipId as string, { fade_in_ms: value }); }} />
            <Slider label="Fade out" min={0} max={5000} step={100} value={clip.fade_out_ms || 0} onChange={(value) => { void updateClipFade(selectedClipId as string, { fade_out_ms: value }); }} />
            {clip.text && (
              <>
                <Field label="Text">
                  <input
                    value={clip.text.text}
                    onChange={(event) => { void updateClipText(selectedClipId as string, { text: event.target.value }); }}
                    className="min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
                  />
                </Field>
                <div className="grid grid-cols-2 gap-2">
                  <Field label="Font size">
                    <input
                      type="number"
                      min={8}
                      max={240}
                      key={`fs-${clip.id}-${clip.text.font_size ?? ''}`}
                      defaultValue={clip.text.font_size || 48}
                      onBlur={(event) => {
                        const fontSize = Math.max(8, Math.min(240, Math.round(Number(event.target.value))));
                        if (Number.isFinite(fontSize) && fontSize !== clip.text?.font_size) void updateClipText(clip.id, { font_size: fontSize });
                      }}
                      onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }}
                      className="min-h-8 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
                    />
                  </Field>
                  <Field label="Weight">
                    <select
                      value={clip.text.font_weight || '700'}
                      onChange={(event) => { void updateClipText(clip.id, { font_weight: event.target.value }); }}
                      className="min-h-8 w-full rounded-md border border-border bg-surface-alt px-1 text-xs text-text"
                    >
                      {['400', '500', '600', '700', '800'].map((weight) => (
                        <option key={weight} value={weight}>{weight}</option>
                      ))}
                    </select>
                  </Field>
                  <Field label="Color">
                    <input
                      type="color"
                      value={/^#[0-9a-fA-F]{6}$/.test(clip.text.color || '') ? (clip.text.color as string) : '#ffffff'}
                      onChange={(event) => { void updateClipText(clip.id, { color: event.target.value }); }}
                      className="h-8 w-full cursor-pointer rounded-md border border-border bg-surface-alt"
                    />
                  </Field>
                  <Field label="Background">
                    <div className="flex items-center gap-1">
                      <input
                        type="color"
                        value={/^#[0-9a-fA-F]{6}$/.test(clip.text.background || '') ? (clip.text.background as string) : '#111111'}
                        onChange={(event) => { void updateClipText(clip.id, { background: event.target.value }); }}
                        className="h-8 min-w-0 flex-1 cursor-pointer rounded-md border border-border bg-surface-alt"
                      />
                      {clip.text.background && (
                        <button
                          onClick={() => { void updateClipText(clip.id, { background: undefined }); }}
                          className="rounded border border-border px-1.5 py-1 text-[10px] text-text-muted hover:text-text"
                          title="Remove background"
                        >
                          ×
                        </button>
                      )}
                    </div>
                  </Field>
                  <Field label="Align">
                    <select
                      value={clip.text.text_align || 'center'}
                      onChange={(event) => { void updateClipText(clip.id, { text_align: event.target.value as 'left' | 'center' | 'right' }); }}
                      className="min-h-8 w-full rounded-md border border-border bg-surface-alt px-1 text-xs text-text"
                    >
                      {(['left', 'center', 'right'] as const).map((align) => (
                        <option key={align} value={align}>{align}</option>
                      ))}
                    </select>
                  </Field>
                  <Field label="Shadow">
                    <button
                      onClick={() => { void updateClipText(clip.id, { shadow: !clip.text?.shadow }); }}
                      className={`min-h-8 w-full rounded-md border px-2 text-xs ${clip.text.shadow ? 'border-primary/40 bg-primary/10 text-primary' : 'border-border bg-surface-alt text-text-muted hover:text-text'}`}
                    >
                      {clip.text.shadow ? 'On' : 'Off'}
                    </button>
                  </Field>
                </div>
                <Field label="Text preset">
                  <div className="flex flex-wrap gap-1">
                    {TEXT_PRESETS.map((preset) => (
                      <button
                        key={preset.key}
                        onClick={() => {
                          void updateClipText(clip.id, preset.text);
                          if (preset.position && timeline) {
                            void updateClipTransform(clip.id, preset.position(timeline.canvas));
                          }
                        }}
                        className="rounded-md border border-border bg-surface-alt px-2 py-1 text-[10px] text-text-muted hover:text-text"
                      >
                        {preset.label}
                      </button>
                    ))}
                  </div>
                </Field>
              </>
            )}
            {clip.shape && (() => {
              const shape = clip.shape;
              const definition = annotationDefinition(shape.kind);
              const fillKinds = ['highlight', 'label', 'speech_bubble', 'step_marker', 'spotlight', 'rounded_rectangle', 'ellipse'];
              const strokeKinds = ['rectangle', 'rounded_rectangle', 'ellipse', 'arrow', 'line', 'checkmark', 'x_mark', 'speech_bubble', 'label'];
              const blurKinds = ['blur', 'pixelate'];
              const cornerKinds = ['rounded_rectangle', 'speech_bubble', 'label'];
              return (
                <>
                  <div className="flex items-center justify-between">
                    <span className="text-[11px] font-medium text-text-secondary">{definition?.label || shape.kind}</span>
                    {definition && definition.exportSupport !== 'full' && (
                      <span
                        className={`rounded px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wide ${definition.exportSupport === 'preview' ? 'bg-amber-400/15 text-amber-300' : 'bg-sky-400/15 text-sky-300'}`}
                        title={definition.exportNote || (definition.exportSupport === 'preview' ? 'This annotation shows in the preview but is not drawn at export yet' : 'Exports with reduced fidelity')}
                      >
                        {definition.exportSupport === 'preview' ? 'preview only' : 'partial export'}
                      </span>
                    )}
                  </div>
                  <div className="grid grid-cols-2 gap-2">
                    <Field label="Shape width">
                      <input
                        type="number"
                        min={2}
                        key={`sw-${clip.id}-${shape.width ?? ''}`}
                        defaultValue={shape.width || 320}
                        onBlur={(event) => {
                          const width = Math.max(2, Math.round(Number(event.target.value)));
                          if (Number.isFinite(width) && width !== shape.width) void updateClipShape(clip.id, { width });
                        }}
                        onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }}
                        className="min-h-8 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
                      />
                    </Field>
                    <Field label="Shape height">
                      <input
                        type="number"
                        min={2}
                        key={`sh-${clip.id}-${shape.height ?? ''}`}
                        defaultValue={shape.height || 180}
                        onBlur={(event) => {
                          const height = Math.max(2, Math.round(Number(event.target.value)));
                          if (Number.isFinite(height) && height !== shape.height) void updateClipShape(clip.id, { height });
                        }}
                        onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }}
                        className="min-h-8 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
                      />
                    </Field>
                    {fillKinds.includes(shape.kind) && (
                      <Field label="Fill">
                        <input
                          type="color"
                          value={/^#[0-9a-fA-F]{6}$/.test(shape.fill || '') ? (shape.fill as string) : '#facc15'}
                          onChange={(event) => { void updateClipShape(clip.id, { fill: event.target.value }); }}
                          className="h-8 w-full cursor-pointer rounded-md border border-border bg-surface-alt"
                        />
                      </Field>
                    )}
                    {blurKinds.includes(shape.kind) && (
                      <Field label={shape.kind === 'pixelate' ? 'Block size' : 'Blur radius'}>
                        <input
                          type="number"
                          min={1}
                          max={50}
                          key={`br-${clip.id}-${shape.blur_radius ?? ''}`}
                          defaultValue={shape.blur_radius || 12}
                          onBlur={(event) => {
                            const blurRadius = Math.max(1, Math.min(50, Math.round(Number(event.target.value))));
                            if (Number.isFinite(blurRadius) && blurRadius !== shape.blur_radius) void updateClipShape(clip.id, { blur_radius: blurRadius });
                          }}
                          onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }}
                          className="min-h-8 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
                        />
                      </Field>
                    )}
                    {strokeKinds.includes(shape.kind) && (
                      <>
                        <Field label="Stroke">
                          <input
                            type="color"
                            value={/^#[0-9a-fA-F]{6}$/.test(shape.stroke || '') ? (shape.stroke as string) : '#f59e0b'}
                            onChange={(event) => { void updateClipShape(clip.id, { stroke: event.target.value }); }}
                            className="h-8 w-full cursor-pointer rounded-md border border-border bg-surface-alt"
                          />
                        </Field>
                        <Field label="Stroke width">
                          <input
                            type="number"
                            min={1}
                            max={100}
                            key={`stw-${clip.id}-${shape.stroke_width ?? ''}`}
                            defaultValue={shape.stroke_width || 6}
                            onBlur={(event) => {
                              const strokeWidth = Math.max(1, Math.min(100, Math.round(Number(event.target.value))));
                              if (Number.isFinite(strokeWidth) && strokeWidth !== shape.stroke_width) void updateClipShape(clip.id, { stroke_width: strokeWidth });
                            }}
                            onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }}
                            className="min-h-8 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
                          />
                        </Field>
                      </>
                    )}
                    {cornerKinds.includes(shape.kind) && (
                      <Field label="Corner radius">
                        <input
                          type="number"
                          min={0}
                          max={200}
                          key={`cr-${clip.id}-${shape.corner_radius ?? ''}`}
                          defaultValue={shape.corner_radius || 0}
                          onBlur={(event) => {
                            const cornerRadius = Math.max(0, Math.min(200, Math.round(Number(event.target.value))));
                            if (Number.isFinite(cornerRadius) && cornerRadius !== shape.corner_radius) void updateClipShape(clip.id, { corner_radius: cornerRadius });
                          }}
                          onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }}
                          className="min-h-8 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
                        />
                      </Field>
                    )}
                  </div>
                  <Field label="Annotation preset">
                    <div className="flex flex-wrap gap-1">
                      {ANNOTATION_PRESETS.map((preset) => (
                        <button
                          key={preset.key}
                          onClick={() => { void applyAnnotationPreset(clip.id, preset.key); }}
                          className="rounded-md border border-border bg-surface-alt px-2 py-1 text-[10px] text-text-muted hover:text-text"
                          title={`Apply the "${preset.label}" style to this annotation`}
                        >
                          {preset.label}
                        </button>
                      ))}
                    </div>
                  </Field>
                </>
              );
            })()}
            {modeFeatures.effectControls && (
            <div className="grid grid-cols-1 gap-2">
              <Field label="Effects">
                <EffectBrowser onApply={(effect) => { void addClipEffect(selectedClipId as string, effect); }} />
              </Field>
              <Field label="Transitions">
                <TransitionBrowser onApply={(transition) => { void addClipTransition(selectedClipId as string, transition); }} />
              </Field>
              <Field label="Pan & zoom motion">
                <div className="flex flex-wrap gap-1">
                  {MOTION_PRESETS.map((preset) => (
                    <button
                      key={preset.key}
                      onClick={() => { void applyMotionPreset(selectedClipId as string, preset.key); }}
                      className="rounded-md border border-border bg-surface-alt px-2 py-1 text-[10px] text-text-muted hover:text-text"
                      title={`${preset.description} — generates editable keyframes (scale animation is preview-only at export)`}
                    >
                      {preset.label}
                    </button>
                  ))}
                </div>
              </Field>
              <select
                value=""
                onChange={(event) => {
                  const property = event.target.value as VideoTimelineKeyframe['property'];
                  if (!KEYFRAME_PROPERTIES.includes(property)) return;
                  const currentValue =
                    property === 'volume'
                      ? clip.volume ?? 1
                      : clip.transform?.[property] ?? (property === 'scale' || property === 'opacity' ? 1 : 0);
                  // Keyframe times are clip-relative (measured from clip start).
                  const timeMs = Math.max(0, Math.min(clip.duration_ms, Math.round(playheadMs - clip.start_ms)));
                  void addKeyframe(selectedClipId as string, { property, time_ms: timeMs, value: currentValue, easing: 'ease-in-out' });
                }}
                className="min-h-8 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text-secondary"
                aria-label="Add keyframe at playhead"
              >
                <option value="">+ Add keyframe at playhead…</option>
                {KEYFRAME_PROPERTIES.map((property) => (
                  <option key={property} value={property}>{property}</option>
                ))}
              </select>
            </div>
            )}
            {(() => {
              const limited = (rendererCapabilities?.features || []).filter((f) => !f.supported || f.partial);
              if (rendererCapabilities && limited.length === 0) return null;
              const text = limited.length > 0
                ? `Limited at export: ${limited.map((f) => f.label).join(', ')}`
                : 'Some effects & transitions may not be rendered in export';
              const tooltip = limited.length > 0
                ? limited.map((f) => `${f.label}${f.notes ? ` — ${f.notes}` : ''}`).join('\n')
                : 'Renderer capability information is unavailable.';
              return (
                <p className="rounded-md border border-amber-500/20 bg-amber-500/5 px-2 py-1.5 text-[10px] text-amber-400/70" title={tooltip}>
                  ⚠ {text}
                </p>
              );
            })()}
            {modeFeatures.effectControls && (
            <div className="space-y-1">
              {clip.effects.map((effect, effectIndex) => {
                const definition = effectDefinition(effect.type);
                return (
                  <div
                    key={effect.id}
                    className="rounded-md border border-border bg-surface-alt px-2 py-1.5 text-[11px] text-text-muted"
                    onContextMenu={(event) => {
                      event.preventDefault();
                      setRowMenu({ kind: 'effect', id: effect.id, x: event.clientX, y: event.clientY });
                    }}
                  >
                    <div className="flex items-center gap-1">
                      <button
                        className="min-w-0 flex-1 truncate text-left hover:text-text"
                        title={effect.enabled ? 'Disable effect' : 'Enable effect'}
                        onClick={() => { void toggleClipEffect(selectedClipId as string, effect.id); }}
                      >
                        {definition?.label || effect.type} · {effect.enabled ? 'on' : 'off'}
                      </button>
                      {definition && !definition.exportSupported && (
                        <span className="rounded bg-amber-500/15 px-1 py-0.5 text-[9px] text-amber-400/90" title="Not applied by the FFmpeg renderer at export yet">
                          preview only
                        </span>
                      )}
                      <button
                        className="rounded p-0.5 hover:text-text disabled:cursor-not-allowed disabled:opacity-30"
                        disabled={effectIndex === 0}
                        title="Apply earlier"
                        aria-label={`Move ${effect.type} effect up`}
                        onClick={() => { void reorderClipEffect(selectedClipId as string, effect.id, -1); }}
                      >
                        <ArrowUp size={11} />
                      </button>
                      <button
                        className="rounded p-0.5 hover:text-text disabled:cursor-not-allowed disabled:opacity-30"
                        disabled={effectIndex >= clip.effects.length - 1}
                        title="Apply later"
                        aria-label={`Move ${effect.type} effect down`}
                        onClick={() => { void reorderClipEffect(selectedClipId as string, effect.id, 1); }}
                      >
                        <ArrowDown size={11} />
                      </button>
                      <button
                        className="rounded p-0.5 hover:text-text"
                        title="Remove effect"
                        aria-label={`Remove ${effect.type} effect`}
                        onClick={() => { void removeClipEffect(selectedClipId as string, effect.id); }}
                      >
                        <Trash2 size={11} />
                      </button>
                    </div>
                    {definition?.params.map((param) => (
                      <div key={param.key} className="mt-1">
                        <Slider
                          label={param.label}
                          min={param.min}
                          max={param.max}
                          step={param.step}
                          value={numberParam(effect.params, param.key, param.defaultValue)}
                          onChange={(value) => {
                            void updateClipEffect(selectedClipId as string, effect.id, { params: { [param.key]: value } });
                          }}
                        />
                      </div>
                    ))}
                  </div>
                );
              })}
              {(clip.transitions || []).map((transition) => {
                const definition = transitionDefinition(transition.type);
                return (
                  <div
                    key={transition.id}
                    className="rounded-md border border-border bg-surface-alt px-2 py-1.5 text-[11px] text-text-muted"
                    onContextMenu={(event) => {
                      event.preventDefault();
                      setRowMenu({ kind: 'transition', id: transition.id, x: event.clientX, y: event.clientY });
                    }}
                  >
                    <div className="flex items-center gap-1">
                      <span className="min-w-0 flex-1 truncate">{definition?.label || transition.type}</span>
                      {definition && !definition.exportSupported && (
                        <span className="rounded bg-amber-500/15 px-1 py-0.5 text-[9px] text-amber-400/90" title="Not applied by the FFmpeg renderer at export yet">
                          preview only
                        </span>
                      )}
                      {definition?.exportNote && definition.exportSupported && (
                        <span className="rounded bg-surface px-1 py-0.5 text-[9px]" title={definition.exportNote}>≈ export</span>
                      )}
                      <button
                        className="rounded p-0.5 hover:text-text"
                        title="Remove transition"
                        aria-label={`Remove ${transition.type} transition`}
                        onClick={() => { void removeClipTransition(selectedClipId as string, transition.id); }}
                      >
                        <Trash2 size={11} />
                      </button>
                    </div>
                    <div className="mt-1 flex items-center gap-2">
                      <label className="flex items-center gap-1">
                        <span className="text-[10px]">Duration</span>
                        <input
                          type="number"
                          min={100}
                          step={100}
                          key={`${transition.id}-${transition.duration_ms}`}
                          defaultValue={transition.duration_ms}
                          onBlur={(event) => {
                            const duration = Math.max(100, Math.round(Number(event.target.value)));
                            if (Number.isFinite(duration) && duration !== transition.duration_ms) {
                              void updateClipTransition(selectedClipId as string, transition.id, { duration_ms: duration });
                            }
                          }}
                          onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }}
                          className="w-16 rounded border border-border bg-surface px-1 py-0.5 text-[10px]"
                          aria-label={`${transition.type} duration ms`}
                        />
                        <span className="text-[10px]">ms</span>
                      </label>
                      {definition?.supportsDirection && (
                        <select
                          value={transition.direction || 'left'}
                          onChange={(event) => {
                            void updateClipTransition(selectedClipId as string, transition.id, { direction: event.target.value as 'left' | 'right' | 'up' | 'down' });
                          }}
                          className="rounded border border-border bg-surface px-1 py-0.5 text-[10px]"
                          aria-label={`${transition.type} direction`}
                        >
                          {(['left', 'right', 'up', 'down'] as const).map((direction) => (
                            <option key={direction} value={direction}>{direction}</option>
                          ))}
                        </select>
                      )}
                    </div>
                  </div>
                );
              })}
              {clip.keyframes.map((keyframe) => (
                <div
                  key={keyframe.id}
                  className="rounded-md border border-border bg-surface-alt px-2 py-1.5 text-[11px] text-text-muted"
                  onContextMenu={(event) => {
                    event.preventDefault();
                    setRowMenu({ kind: 'keyframe', id: keyframe.id, x: event.clientX, y: event.clientY });
                  }}
                >
                  <div className="flex items-center gap-1">
                    <span className="min-w-0 flex-1 truncate">{keyframe.property} keyframe</span>
                    <button
                      className="rounded p-0.5 hover:text-text"
                      title="Remove keyframe"
                      aria-label={`Remove ${keyframe.property} keyframe`}
                      onClick={() => { void removeKeyframe(selectedClipId as string, keyframe.id); }}
                    >
                      <Trash2 size={11} />
                    </button>
                  </div>
                  <div className="mt-1 flex items-center gap-1">
                    <input
                      type="number"
                      min={0}
                      step={0.1}
                      key={`t-${keyframe.id}-${keyframe.time_ms}`}
                      defaultValue={Math.round(keyframe.time_ms / 100) / 10}
                      onBlur={(event) => {
                        const timeMs = Math.max(0, Math.round(Number(event.target.value) * 1000));
                        if (Number.isFinite(timeMs) && timeMs !== keyframe.time_ms) {
                          void updateKeyframe(selectedClipId as string, keyframe.id, { time_ms: timeMs });
                        }
                      }}
                      onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }}
                      className="w-14 rounded border border-border bg-surface px-1 py-0.5 text-[10px]"
                      aria-label={`${keyframe.property} keyframe time seconds (from clip start)`}
                      title="Time in seconds from clip start"
                    />
                    <span className="text-[10px]">s =</span>
                    <input
                      type="number"
                      step={0.05}
                      key={`v-${keyframe.id}-${keyframe.value}`}
                      defaultValue={keyframe.value}
                      onBlur={(event) => {
                        const value = Number(event.target.value);
                        if (Number.isFinite(value) && value !== keyframe.value) {
                          void updateKeyframe(selectedClipId as string, keyframe.id, { value });
                        }
                      }}
                      onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }}
                      className="w-16 rounded border border-border bg-surface px-1 py-0.5 text-[10px]"
                      aria-label={`${keyframe.property} keyframe value`}
                    />
                    <select
                      value={keyframe.easing || 'linear'}
                      onChange={(event) => {
                        void updateKeyframe(selectedClipId as string, keyframe.id, { easing: event.target.value as VideoTimelineKeyframe['easing'] });
                      }}
                      className="flex-1 rounded border border-border bg-surface px-1 py-0.5 text-[10px]"
                      aria-label={`${keyframe.property} keyframe easing`}
                    >
                      {KEYFRAME_EASINGS.map((easing) => (
                        <option key={easing} value={easing}>{easing}</option>
                      ))}
                    </select>
                  </div>
                </div>
              ))}
            </div>
            )}
          </div>
        )}
      </section>
      )}
      {planMenu && assistantPlan && (() => {
        const items: ContextMenuEntry[] = [
          {
            label: planSelectedCount < planTotal ? `Apply ${planSelectedCount} selected` : 'Apply plan',
            disabled: planSelectedCount === 0,
            action: applySelectedPlan,
          },
          'divider',
          { label: 'Copy plan JSON', action: () => { void navigator.clipboard.writeText(JSON.stringify(assistantPlan, null, 2)); } },
          { label: 'Copy summary', action: () => { void navigator.clipboard.writeText([assistantPlan.summary, ...(assistantPlan.preview || [])].join('\n')); } },
          'divider',
          { label: 'Dismiss plan', danger: true, action: () => useVideoStudioStore.setState({ assistantPlan: null }) },
        ];
        return <ContextMenu position={planMenu} items={items} onClose={() => setPlanMenu(null)} />;
      })()}
      {variantMenu && (() => {
        const variant = socialVariants.find((item) => item.name === variantMenu.name);
        if (!variant) return null;
        const items: ContextMenuEntry[] = [
          {
            label: 'Create project with this variant',
            action: () => { void createProjectFromVariant(variant); },
          },
          { label: 'Copy plan JSON', action: () => { void navigator.clipboard.writeText(JSON.stringify(variant.plan, null, 2)); } },
        ];
        return <ContextMenu position={{ x: variantMenu.x, y: variantMenu.y }} items={items} onClose={() => setVariantMenu(null)} />;
      })()}
      {rowMenu && clip && (() => {
        let items: ContextMenuEntry[] = [];
        if (rowMenu.kind === 'effect') {
          const effect = clip.effects.find((item) => item.id === rowMenu.id);
          if (!effect) return null;
          const index = clip.effects.findIndex((item) => item.id === rowMenu.id);
          items = [
            { label: effect.enabled ? 'Disable' : 'Enable', action: () => { void toggleClipEffect(clip.id, effect.id); } },
            { label: 'Duplicate effect', action: () => { void addClipEffect(clip.id, { type: effect.type, enabled: effect.enabled, params: { ...effect.params } }); } },
            { label: 'Move up', disabled: index === 0, action: () => { void reorderClipEffect(clip.id, effect.id, -1); } },
            { label: 'Move down', disabled: index >= clip.effects.length - 1, action: () => { void reorderClipEffect(clip.id, effect.id, 1); } },
            'divider',
            { label: 'Remove effect', danger: true, action: () => { void removeClipEffect(clip.id, effect.id); } },
          ];
        } else if (rowMenu.kind === 'transition') {
          const transition = (clip.transitions || []).find((item) => item.id === rowMenu.id);
          if (!transition) return null;
          items = [
            { label: 'Remove transition', danger: true, action: () => { void removeClipTransition(clip.id, transition.id); } },
          ];
        } else {
          const keyframe = clip.keyframes.find((item) => item.id === rowMenu.id);
          if (!keyframe) return null;
          items = [
            { label: 'Duplicate keyframe', action: () => { void addKeyframe(clip.id, { property: keyframe.property, time_ms: keyframe.time_ms + 250, value: keyframe.value, easing: keyframe.easing }); } },
            'divider',
            { label: 'Delete keyframe', danger: true, action: () => { void removeKeyframe(clip.id, keyframe.id); } },
          ];
        }
        return <ContextMenu position={{ x: rowMenu.x, y: rowMenu.y }} items={items} onClose={() => setRowMenu(null)} />;
      })()}
    </div>
  );
}

function TimecodeField({ label, valueMs, onCommit }: { label: string; valueMs: number; onCommit: (ms: number) => void }) {
  return (
    <Field label={label}>
      <input
        key={`${label}-${valueMs}`}
        defaultValue={formatTimecode(valueMs)}
        onBlur={(event) => {
          const ms = parseTimecode(event.target.value);
          if (ms !== null && ms !== valueMs) onCommit(ms);
        }}
        onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }}
        className="min-h-8 w-full rounded-md border border-border bg-surface-alt px-2 font-mono text-xs text-text"
        title="Timecode (MM:SS.cc) or seconds"
      />
    </Field>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 block text-[11px] font-medium text-text-muted">{label}</span>
      {children}
    </label>
  );
}

function Slider({
  label,
  min,
  max,
  step,
  value,
  onChange,
}: {
  label: string;
  min: number;
  max: number;
  step: number;
  value: number;
  onChange: (value: number) => void;
}) {
  // Track the value locally while dragging and commit once on release —
  // committing per input event floods the undo history and fires a save
  // request for every pixel of slider travel.
  const [draft, setDraft] = useState<number | null>(null);
  const display = draft ?? value;
  const commit = () => {
    if (draft !== null && draft !== value) onChange(draft);
    setDraft(null);
  };
  return (
    <Field label={`${label}: ${Math.round(display * 100) / 100}`}>
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={display}
        onChange={(event) => setDraft(Number(event.target.value))}
        onPointerUp={commit}
        onKeyUp={commit}
        onBlur={commit}
        className="w-full"
      />
    </Field>
  );
}
