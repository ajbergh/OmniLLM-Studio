import { useState } from 'react';
import type { PointerEvent as ReactPointerEvent } from 'react';
import { ChevronDown, ChevronRight, SkipBack, SkipForward } from 'lucide-react';
import { useVideoStudioStore } from '../../../stores/videoStudio';
import { ContextMenu } from '../../common/ContextMenu';
import type { ContextMenuEntry } from '../../common/ContextMenu';
import { InputDialog } from '../../common/AppDialog';
import { KEYFRAME_EASINGS } from '../effects/keyframeUtils';
import type { VideoTimelineKeyframe } from '../../../types/video';

/**
 * Collapsible keyframe lane for the selected clip: one row per animated
 * property, diamonds draggable in time, right-click for value/easing edits.
 * Renders inside the timeline scroll container so positions line up.
 */
export function KeyframeLane({ pxPerMs }: { pxPerMs: number }) {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const selectedClipId = useVideoStudioStore((state) => state.selectedClipId);
  const setPlayhead = useVideoStudioStore((state) => state.setPlayhead);
  const playheadMs = useVideoStudioStore((state) => state.playheadMs);
  const updateKeyframe = useVideoStudioStore((state) => state.updateKeyframe);
  const removeKeyframe = useVideoStudioStore((state) => state.removeKeyframe);
  const addKeyframe = useVideoStudioStore((state) => state.addKeyframe);

  const [collapsed, setCollapsed] = useState(false);
  const [menu, setMenu] = useState<{ keyframe: VideoTimelineKeyframe; x: number; y: number } | null>(null);
  const [valueDialog, setValueDialog] = useState<VideoTimelineKeyframe | null>(null);
  const [liveDrag, setLiveDrag] = useState<{ id: string; timeMs: number } | null>(null);

  const clip = timeline?.tracks.flatMap((track) => track.clips).find((item) => item.id === selectedClipId);
  if (!clip || (clip.keyframes || []).length === 0) return null;

  const keyframes = (clip.keyframes || []).map((keyframe) =>
    liveDrag && liveDrag.id === keyframe.id ? { ...keyframe, time_ms: liveDrag.timeMs } : keyframe);
  const properties = Array.from(new Set(keyframes.map((keyframe) => keyframe.property)));
  const sortedTimes = Array.from(new Set(keyframes.map((keyframe) => clip.start_ms + keyframe.time_ms))).sort((a, b) => a - b);

  const jump = (direction: -1 | 1) => {
    const target = direction === 1
      ? sortedTimes.find((time) => time > playheadMs + 1)
      : [...sortedTimes].reverse().find((time) => time < playheadMs - 1);
    if (target !== undefined) setPlayhead(target);
  };

  const beginDrag = (keyframe: VideoTimelineKeyframe, event: ReactPointerEvent<HTMLElement>) => {
    if (event.button !== 0) return;
    event.preventDefault();
    event.stopPropagation();
    const startX = event.clientX;
    let live = keyframe.time_ms;
    const onMove = (moveEvent: PointerEvent) => {
      const deltaMs = Math.round((moveEvent.clientX - startX) / pxPerMs);
      live = Math.max(0, Math.min(clip.duration_ms, keyframe.time_ms + deltaMs));
      setLiveDrag({ id: keyframe.id, timeMs: live });
    };
    const onUp = () => {
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
      setLiveDrag(null);
      if (live !== keyframe.time_ms) void updateKeyframe(clip.id, keyframe.id, { time_ms: live });
    };
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
  };

  return (
    <div className="border-t border-border bg-surface-raised/60">
      <div className="grid grid-cols-[116px_minmax(0,1fr)]">
        <div className="sticky left-0 z-20 flex items-center gap-1 border-r border-border bg-surface-alt px-2 py-1">
          <button
            onClick={() => setCollapsed((value) => !value)}
            className="rounded p-0.5 text-text-muted hover:text-text"
            title={collapsed ? 'Expand keyframe lane' : 'Collapse keyframe lane'}
            aria-label={collapsed ? 'Expand keyframe lane' : 'Collapse keyframe lane'}
          >
            {collapsed ? <ChevronRight size={11} /> : <ChevronDown size={11} />}
          </button>
          <span className="min-w-0 flex-1 truncate text-[10px] font-medium text-text-secondary">Keyframes</span>
          <button onClick={() => jump(-1)} className="rounded p-0.5 text-text-muted hover:text-text" title="Previous keyframe" aria-label="Previous keyframe">
            <SkipBack size={10} />
          </button>
          <button onClick={() => jump(1)} className="rounded p-0.5 text-text-muted hover:text-text" title="Next keyframe" aria-label="Next keyframe">
            <SkipForward size={10} />
          </button>
        </div>
        <div className="relative">
          {!collapsed && properties.map((property) => (
            <div key={property} className="relative h-6 border-b border-border/40 last:border-b-0">
              <span className="pointer-events-none absolute left-1 top-1/2 -translate-y-1/2 text-[9px] uppercase tracking-wide text-text-muted">
                {property}
              </span>
              {keyframes.filter((keyframe) => keyframe.property === property).map((keyframe) => (
                <button
                  key={keyframe.id}
                  type="button"
                  className="absolute top-1/2 h-2.5 w-2.5 -translate-y-1/2 rotate-45 cursor-ew-resize rounded-[1px] border border-white/70 bg-primary hover:bg-primary/80"
                  style={{ left: Math.max(0, (clip.start_ms + keyframe.time_ms) * pxPerMs - 5) }}
                  title={`${property} = ${keyframe.value} @ ${(keyframe.time_ms / 1000).toFixed(2)}s (${keyframe.easing || 'linear'}) — drag to move, right-click to edit`}
                  aria-label={`${property} keyframe at ${(keyframe.time_ms / 1000).toFixed(2)} seconds`}
                  onPointerDown={(event) => beginDrag(keyframe, event)}
                  onContextMenu={(event) => {
                    event.preventDefault();
                    event.stopPropagation();
                    setMenu({ keyframe, x: event.clientX, y: event.clientY });
                  }}
                />
              ))}
            </div>
          ))}
          {collapsed && (
            <div className="relative h-5">
              {keyframes.map((keyframe) => (
                <span
                  key={keyframe.id}
                  className="absolute top-1/2 h-2 w-2 -translate-y-1/2 rotate-45 rounded-[1px] bg-primary/70"
                  style={{ left: Math.max(0, (clip.start_ms + keyframe.time_ms) * pxPerMs - 4) }}
                  aria-hidden="true"
                />
              ))}
            </div>
          )}
        </div>
      </div>
      {menu && (() => {
        const { keyframe } = menu;
        const items: ContextMenuEntry[] = [
          { label: `Edit value (${keyframe.value})…`, action: () => setValueDialog(keyframe) },
          'divider',
          ...KEYFRAME_EASINGS.map((easing) => ({
            label: `${(keyframe.easing || 'linear') === easing ? '✓ ' : ''}Easing: ${easing}`,
            action: () => { void updateKeyframe(clip.id, keyframe.id, { easing }); },
          })),
          'divider',
          {
            label: 'Duplicate keyframe',
            action: () => {
              void addKeyframe(clip.id, {
                property: keyframe.property,
                time_ms: Math.min(clip.duration_ms, keyframe.time_ms + 250),
                value: keyframe.value,
                easing: keyframe.easing,
              });
            },
          },
          { label: 'Delete keyframe', danger: true, action: () => { void removeKeyframe(clip.id, keyframe.id); } },
        ];
        return <ContextMenu position={{ x: menu.x, y: menu.y }} items={items} onClose={() => setMenu(null)} />;
      })()}
      {valueDialog && (
        <InputDialog
          title={`Edit ${valueDialog.property} keyframe value`}
          initialValue={String(valueDialog.value)}
          inputType="number"
          submitLabel="Save"
          validate={(value) => (Number.isFinite(Number(value)) ? null : 'Enter a number')}
          onSubmit={(value) => {
            setValueDialog(null);
            void updateKeyframe(clip.id, valueDialog.id, { value: Number(value) });
          }}
          onCancel={() => setValueDialog(null)}
        />
      )}
    </div>
  );
}
