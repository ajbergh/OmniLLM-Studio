import { Plus, Sparkles, Type } from 'lucide-react';
import type { ReactNode } from 'react';
import { useVideoStudioStore } from '../../stores/videoStudio';
import type { VideoTimelineClip } from '../../types/video';

function selectedClip(): VideoTimelineClip | null {
  const { timeline, selectedClipId } = useVideoStudioStore.getState();
  if (!timeline || !selectedClipId) return null;
  for (const track of timeline.tracks) {
    const clip = track.clips.find((item) => item.id === selectedClipId);
    if (clip) return clip;
  }
  return null;
}

export function VideoInspector() {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const selectedClipId = useVideoStudioStore((state) => state.selectedClipId);
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
  const addClipTransition = useVideoStudioStore((state) => state.addClipTransition);
  const addKeyframe = useVideoStudioStore((state) => state.addKeyframe);
  const addTextClip = useVideoStudioStore((state) => state.addTextClip);
  const requestStoryboard = useVideoStudioStore((state) => state.requestStoryboard);
  const requestEditPlan = useVideoStudioStore((state) => state.requestEditPlan);
  const requestTimelinePlan = useVideoStudioStore((state) => state.requestTimelinePlan);
  const applyAssistantPlan = useVideoStudioStore((state) => state.applyAssistantPlan);
  const requestSocialVariants = useVideoStudioStore((state) => state.requestSocialVariants);
  const clip = selectedClip();

  return (
    <div className="min-h-0 overflow-y-auto p-3">
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
        {assistantPlan && (
          <div className="mt-3 rounded-lg border border-primary/25 bg-primary/10 p-2">
            <p className="text-xs font-medium text-text">{assistantPlan.summary}</p>
            <p className="mt-1 text-[11px] text-text-muted">{assistantPlan.operations.length} validated operation{assistantPlan.operations.length === 1 ? '' : 's'}</p>
            <button className="mt-2 min-h-8 rounded-md bg-primary px-2 text-xs font-medium text-white" onClick={() => { void applyAssistantPlan(); }}>
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
          <div className="rounded-lg border border-dashed border-border bg-surface-alt p-4 text-center text-xs text-text-muted">
            {timeline ? 'Select a clip to edit.' : 'No timeline loaded.'}
          </div>
        ) : (
          <div className="space-y-3">
            <Field label="Start">
              <span className="text-xs text-text-secondary">{Math.round(clip.start_ms / 100) / 10}s</span>
            </Field>
            {clip.transform && (
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
            <div className="grid grid-cols-2 gap-2">
              <button
                className="inline-flex min-h-8 items-center justify-center gap-1 rounded-md border border-border bg-surface-alt px-2 text-xs text-text-secondary hover:text-text"
                onClick={() => { void addClipEffect(selectedClipId as string, { type: 'brightness', enabled: true, params: { amount: 1.1 } }); }}
              >
                <Plus size={12} />
                Effect
              </button>
              <button
                className="inline-flex min-h-8 items-center justify-center gap-1 rounded-md border border-border bg-surface-alt px-2 text-xs text-text-secondary hover:text-text"
                onClick={() => { void addClipTransition(selectedClipId as string, { type: 'fade', duration_ms: 500 }); }}
              >
                <Plus size={12} />
                Transition
              </button>
              <button
                className="col-span-2 inline-flex min-h-8 items-center justify-center gap-1 rounded-md border border-border bg-surface-alt px-2 text-xs text-text-secondary hover:text-text"
                onClick={() => { void addKeyframe(selectedClipId as string, { property: 'opacity', time_ms: Math.max(0, clip.start_ms), value: clip.transform?.opacity ?? 1, easing: 'ease-in-out' }); }}
              >
                <Plus size={12} />
                Keyframe
              </button>
            </div>
            <div className="space-y-1">
              {clip.effects.map((effect) => (
                <div key={effect.id} className="rounded-md border border-border bg-surface-alt px-2 py-1 text-[11px] text-text-muted">
                  {effect.type} · {effect.enabled ? 'on' : 'off'}
                </div>
              ))}
              {(clip.transitions || []).map((transition) => (
                <div key={transition.id} className="rounded-md border border-border bg-surface-alt px-2 py-1 text-[11px] text-text-muted">
                  {transition.type} · {transition.duration_ms}ms
                </div>
              ))}
            </div>
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
