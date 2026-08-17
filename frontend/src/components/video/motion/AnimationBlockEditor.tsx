import { Trash2 } from 'lucide-react';
import type { VideoAnimationBlock } from '../../../types/video';
import { animationBlock } from './animationBlockRegistry';

export function AnimationBlockEditor({ block, onUpdate, onRemove }: {
  block: VideoAnimationBlock;
  onUpdate: (patch: { durationMs?: number; delayMs?: number }) => void;
  onRemove: () => void;
}) {
  const definition = animationBlock(block.block_key);
  return (
    <div className="rounded-md border border-border bg-surface-alt p-2 text-[11px] text-text-muted" data-animation-block={block.block_key}>
      <div className="flex items-center gap-1">
        <span className="min-w-0 flex-1 truncate font-medium text-text">{definition?.label || block.block_key}</span>
        <span className="rounded bg-surface px-1 text-[9px] uppercase">{block.family}</span>
        <button type="button" onClick={onRemove} className="rounded p-1 hover:text-red-400" aria-label={`Remove ${definition?.label || block.block_key}`}><Trash2 size={11} /></button>
      </div>
      <div className="mt-1 grid grid-cols-2 gap-2">
        <label className="text-[9px]">Duration ms
          <input type="number" min={1} defaultValue={block.duration_ms} onBlur={(event) => onUpdate({ durationMs: Math.max(1, Math.round(Number(event.target.value))) })} className="mt-0.5 w-full rounded border border-border bg-surface px-1 py-0.5 text-[10px]" />
        </label>
        <label className="text-[9px]">Delay ms
          <input type="number" min={0} defaultValue={block.delay_ms || 0} onBlur={(event) => onUpdate({ delayMs: Math.max(0, Math.round(Number(event.target.value))) })} className="mt-0.5 w-full rounded border border-border bg-surface px-1 py-0.5 text-[10px]" />
        </label>
      </div>
      <p className="mt-1 text-[9px]">{block.generated_keyframe_ids.length} editable keyframe{block.generated_keyframe_ids.length === 1 ? '' : 's'}</p>
    </div>
  );
}
