import { Camera, Copy, Plus, Trash2 } from 'lucide-react';
import { useVideoStudioStore } from '../../../stores/videoStudio';
import { EFFECT_DEFINITIONS, defaultEffectParams } from '../effects/effectRegistry';

const CAMERA_PRESETS = [
  { label: 'Push In', key: 'push_in' },
  { label: 'Pull Out', key: 'pull_out' },
  { label: 'Pan Left', key: 'pan_left' },
  { label: 'Pan Right', key: 'pan_right' },
  { label: 'Crane Up', key: 'crane_up' },
  { label: 'Crane Down', key: 'crane_down' },
  { label: 'Dolly Left', key: 'dolly_left' },
  { label: 'Dolly Right', key: 'dolly_right' },
  { label: 'Orbit', key: 'orbit' },
  { label: 'Handheld', key: 'handheld' },
  { label: 'Rack Focus', key: 'rack_focus' },
] as const;

export function SceneStrip() {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const selectedSceneId = useVideoStudioStore((state) => state.selectedSceneId);
  const selectScene = useVideoStudioStore((state) => state.selectScene);
  const addScene = useVideoStudioStore((state) => state.addScene);
  const duplicateScene = useVideoStudioStore((state) => state.duplicateScene);
  const deleteScene = useVideoStudioStore((state) => state.deleteScene);
  const reorderScene = useVideoStudioStore((state) => state.reorderScene);
  const applyCameraPreset = useVideoStudioStore((state) => state.applyCameraPreset);
  const addSceneEffect = useVideoStudioStore((state) => state.addSceneEffect);
  const removeSceneEffect = useVideoStudioStore((state) => state.removeSceneEffect);
  if (!timeline) return null;
  const scenes = timeline.scenes || [];
  const selected = scenes.find((scene) => scene.id === selectedSceneId);
  const cinematic = EFFECT_DEFINITIONS.filter((effect) => ['film_grain', 'bloom', 'color_grade', 'edge_fade', 'rgb_split', 'ghost_trail', 'motion_blur', 'depth_of_field', 'rack_focus'].includes(effect.type));
  return (
    <div className="border-b border-border bg-surface-alt/70 px-2 py-1" data-testid="scene-strip">
      <div className="flex items-center gap-1 overflow-x-auto">
        {scenes.length === 0 ? (
          <button type="button" onClick={() => { void addScene({ startMs: 0, durationMs: timeline.duration_ms }); }} className="rounded border border-dashed border-border px-2 py-1 text-[10px] text-text-muted hover:text-text">Implicit full-timeline scene · click to edit</button>
        ) : scenes.map((scene, index) => (
          <button
            type="button"
            key={scene.id}
            onClick={() => selectScene(scene.id)}
            onDoubleClick={() => { const next = index === 0 ? scenes.length - 1 : index - 1; void reorderScene(scene.id, next); }}
            className={`min-w-24 rounded border px-2 py-1 text-left text-[10px] ${selectedSceneId === scene.id ? 'border-primary/60 bg-primary/10 text-text' : 'border-border bg-surface text-text-muted'}`}
            title={`${(scene.start_ms / 1000).toFixed(1)}s–${((scene.start_ms + scene.duration_ms) / 1000).toFixed(1)}s · double-click to move earlier`}
          >
            <span className="block truncate font-medium">{scene.name}</span>
            <span className="text-[9px]">{(scene.duration_ms / 1000).toFixed(1)}s</span>
          </button>
        ))}
        <button type="button" onClick={() => { void addScene({ durationMs: 5000 }); }} className="rounded p-1 text-text-muted hover:text-text" aria-label="Add scene"><Plus size={12} /></button>
        {selected && <><button type="button" onClick={() => { void duplicateScene(selected.id); }} className="rounded p-1 text-text-muted hover:text-text" aria-label="Duplicate scene"><Copy size={12} /></button><button type="button" onClick={() => { void deleteScene(selected.id); }} className="rounded p-1 text-text-muted hover:text-red-400" aria-label="Delete scene"><Trash2 size={12} /></button></>}
      </div>
      {selected && (
        <div className="mt-1 flex items-center gap-1 overflow-x-auto" data-testid="camera-lane">
          <Camera size={11} className="shrink-0 text-primary" />
          <span className="shrink-0 text-[9px] uppercase text-text-muted">Camera</span>
          {CAMERA_PRESETS.map((preset) => <button key={preset.label} type="button" onClick={() => { void applyCameraPreset(selected.id, preset.key); }} className="shrink-0 rounded border border-border bg-surface px-1.5 py-0.5 text-[9px] text-text-muted hover:text-text">{preset.label}</button>)}
          <span className="mx-1 h-4 w-px shrink-0 bg-border" />
          {cinematic.map((definition) => <button key={definition.type} type="button" onClick={() => { void addSceneEffect(selected.id, { type: definition.type, enabled: true, params: defaultEffectParams(definition) }); }} className="shrink-0 rounded border border-border bg-surface px-1.5 py-0.5 text-[9px] text-text-muted hover:text-text">+ {definition.label}</button>)}
          {(selected.effects || []).map((effect) => <button key={effect.id} type="button" onClick={() => { void removeSceneEffect(selected.id, effect.id); }} className="shrink-0 rounded bg-primary/10 px-1.5 py-0.5 text-[9px] text-primary" title="Remove scene effect">{effect.type} ×</button>)}
        </div>
      )}
    </div>
  );
}
