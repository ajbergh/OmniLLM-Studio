import { ArrowDown, ArrowUp, Sparkles, Trash2, Type } from 'lucide-react';
import type { ReactNode } from 'react';
import { useVideoStudioStore } from '../../stores/videoStudio';
import { editorModeFeatures } from './editorModes';
import { EFFECT_DEFINITIONS, defaultEffectParams, effectDefinition, numberParam } from './effects/effectRegistry';
import { KEYFRAME_EASINGS, KEYFRAME_PROPERTIES } from './effects/keyframeUtils';
import { TRANSITION_DEFINITIONS, transitionDefinition } from './effects/transitionRegistry';
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

const QUICK_WORKFLOWS = [
  { label: '30s social cut', instruction: 'Create a 30-second social cut' },
  { label: '15s teaser', instruction: 'Create a 15-second teaser' },
  { label: 'Vertical 9:16', instruction: 'Convert the timeline to vertical 9:16' },
  { label: 'Square 1:1', instruction: 'Convert the timeline to square 1:1' },
  { label: 'Title card', instruction: 'Add a title card at the start' },
  { label: 'Lower third', instruction: 'Add a lower third caption at the playhead' },
  { label: 'Captions', instruction: 'Add captions from the prompt text' },
  { label: 'Tighten pacing', instruction: 'Tighten pacing and remove trailing dead space' },
];

export function VideoInspector() {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const playheadMs = useVideoStudioStore((state) => state.playheadMs);
  const selectedClipId = useVideoStudioStore((state) => state.selectedClipId);
  const rendererCapabilities = useVideoStudioStore((state) => state.rendererCapabilities);
  const assistantInstruction = useVideoStudioStore((state) => state.assistantInstruction);
  const assistantPlan = useVideoStudioStore((state) => state.assistantPlan);
  const storyboard = useVideoStudioStore((state) => state.storyboard);
  const socialVariants = useVideoStudioStore((state) => state.socialVariants);
  const setAssistantInstruction = useVideoStudioStore((state) => state.setAssistantInstruction);
  const updateClipTransform = useVideoStudioStore((state) => state.updateClipTransform);
  const updateClipVolume = useVideoStudioStore((state) => state.updateClipVolume);
  const updateClipFade = useVideoStudioStore((state) => state.updateClipFade);
  const updateClipText = useVideoStudioStore((state) => state.updateClipText);
  const addClipEffect = useVideoStudioStore((state) => state.addClipEffect);
  const toggleClipEffect = useVideoStudioStore((state) => state.toggleClipEffect);
  const removeClipEffect = useVideoStudioStore((state) => state.removeClipEffect);
  const updateClipEffect = useVideoStudioStore((state) => state.updateClipEffect);
  const reorderClipEffect = useVideoStudioStore((state) => state.reorderClipEffect);
  const addClipTransition = useVideoStudioStore((state) => state.addClipTransition);
  const updateClipTransition = useVideoStudioStore((state) => state.updateClipTransition);
  const removeClipTransition = useVideoStudioStore((state) => state.removeClipTransition);
  const addKeyframe = useVideoStudioStore((state) => state.addKeyframe);
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

  return (
    <div className="min-h-0 overflow-y-auto p-3">
      {modeFeatures.assistant && (
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
                setAssistantInstruction(workflow.instruction);
                void requestEditPlan();
              }}
              className="rounded-md border border-border bg-surface-alt px-2 py-1 text-[10px] text-text-muted hover:text-text"
              title={workflow.instruction}
            >
              {workflow.label}
            </button>
          ))}
        </div>
        {assistantPlan && (
          <div className="mt-3 rounded-lg border border-primary/25 bg-primary/10 p-2">
            <p className="text-xs font-medium text-text">{assistantPlan.summary}</p>
            <p className="mt-1 text-[11px] text-text-muted">
              {(assistantPlan.preview?.length ?? assistantPlan.operations.length)} valid operation{(assistantPlan.preview?.length ?? assistantPlan.operations.length) === 1 ? '' : 's'}
            </p>
            {assistantPlan.preview && assistantPlan.preview.length > 0 && (
              <ul className="mt-1.5 space-y-0.5">
                {assistantPlan.preview.map((line, index) => (
                  <li key={index} className="text-[10px] text-text-secondary">• {line}</li>
                ))}
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
              disabled={(assistantPlan.preview?.length ?? assistantPlan.operations.length) === 0}
              onClick={() => { void applyAssistantPlan(); }}
            >
              Apply
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
        {socialVariants.length > 0 && (
          <div className="mt-3 flex flex-wrap gap-1">
            {socialVariants.map((variant) => (
              <span key={variant.name} className="rounded-md border border-border bg-surface-alt px-2 py-1 text-[10px] text-text-muted">
                {variant.name}
              </span>
            ))}
          </div>
        )}
      </section>
      )}

      <section className="mt-3 rounded-lg border border-border bg-surface p-3">
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
            <Field label="Start">
              <span className="text-xs text-text-secondary">{Math.round(clip.start_ms / 100) / 10}s</span>
            </Field>
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
                <Slider label="Scale" min={0.25} max={3} step={0.05} value={clip.transform.scale} onChange={(value) => { void updateClipTransform(selectedClipId as string, { scale: value }); }} />
                <Slider label="Opacity" min={0} max={1} step={0.05} value={clip.transform.opacity} onChange={(value) => { void updateClipTransform(selectedClipId as string, { opacity: value }); }} />
                <Slider label="Rotation" min={-180} max={180} step={1} value={clip.transform.rotation} onChange={(value) => { void updateClipTransform(selectedClipId as string, { rotation: value }); }} />
              </>
            )}
            {clip.volume !== undefined && (
              <Slider label="Volume" min={0} max={2} step={0.05} value={clip.volume} onChange={(value) => { void updateClipVolume(selectedClipId as string, value); }} />
            )}
            <Slider label="Fade in" min={0} max={5000} step={100} value={clip.fade_in_ms || 0} onChange={(value) => { void updateClipFade(selectedClipId as string, { fade_in_ms: value }); }} />
            <Slider label="Fade out" min={0} max={5000} step={100} value={clip.fade_out_ms || 0} onChange={(value) => { void updateClipFade(selectedClipId as string, { fade_out_ms: value }); }} />
            {clip.text && (
              <Field label="Text">
                <input
                  value={clip.text.text}
                  onChange={(event) => { void updateClipText(selectedClipId as string, { text: event.target.value }); }}
                  className="min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
                />
              </Field>
            )}
            {modeFeatures.effectControls && (
            <div className="grid grid-cols-1 gap-2">
              <select
                value=""
                onChange={(event) => {
                  const definition = effectDefinition(event.target.value);
                  if (!definition) return;
                  void addClipEffect(selectedClipId as string, { type: definition.type, enabled: true, params: defaultEffectParams(definition) });
                }}
                className="min-h-8 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text-secondary"
                aria-label="Add effect"
              >
                <option value="">+ Add effect…</option>
                {EFFECT_DEFINITIONS.map((definition) => (
                  <option key={definition.type} value={definition.type}>
                    {definition.label}{definition.exportSupported ? '' : ' (preview only)'}
                  </option>
                ))}
              </select>
              <select
                value=""
                onChange={(event) => {
                  const definition = transitionDefinition(event.target.value);
                  if (!definition) return;
                  void addClipTransition(selectedClipId as string, {
                    type: definition.type,
                    duration_ms: definition.defaultDurationMs,
                    ...(definition.supportsDirection ? { direction: 'left' as const } : {}),
                  });
                }}
                className="min-h-8 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text-secondary"
                aria-label="Add transition"
              >
                <option value="">+ Add transition…</option>
                {TRANSITION_DEFINITIONS.map((definition) => (
                  <option key={definition.type} value={definition.type}>
                    {definition.label}{definition.exportSupported ? '' : ' (preview only)'}
                  </option>
                ))}
              </select>
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
                  <div key={effect.id} className="rounded-md border border-border bg-surface-alt px-2 py-1.5 text-[11px] text-text-muted">
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
                    {definition?.params.map((param) => {
                      const value = numberParam(effect.params, param.key, param.defaultValue);
                      return (
                        <label key={param.key} className="mt-1 block">
                          <span className="block text-[10px]">{param.label}: {Math.round(value * 100) / 100}</span>
                          <input
                            type="range"
                            min={param.min}
                            max={param.max}
                            step={param.step}
                            value={value}
                            onChange={(event) => {
                              void updateClipEffect(selectedClipId as string, effect.id, { params: { [param.key]: Number(event.target.value) } });
                            }}
                            className="w-full"
                            aria-label={`${definition.label} ${param.label}`}
                          />
                        </label>
                      );
                    })}
                  </div>
                );
              })}
              {(clip.transitions || []).map((transition) => {
                const definition = transitionDefinition(transition.type);
                return (
                  <div key={transition.id} className="rounded-md border border-border bg-surface-alt px-2 py-1.5 text-[11px] text-text-muted">
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
                <div key={keyframe.id} className="rounded-md border border-border bg-surface-alt px-2 py-1.5 text-[11px] text-text-muted">
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
    </div>
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
  return (
    <Field label={`${label}: ${Math.round(value * 100) / 100}`}>
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(event) => onChange(Number(event.target.value))}
        className="w-full"
      />
    </Field>
  );
}
