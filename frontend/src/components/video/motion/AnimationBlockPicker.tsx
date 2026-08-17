import { ANIMATION_BLOCKS, type AnimationBlockFamily } from './animationBlockRegistry';

export function AnimationBlockPicker({ onApply }: { onApply: (key: string, mode: 'replace' | 'stack') => void }) {
  return (
    <div className="space-y-2" data-testid="animation-block-picker">
      {(['in', 'during', 'out'] as AnimationBlockFamily[]).map((family) => (
        <div key={family}>
          <p className="mb-1 text-[10px] font-semibold uppercase tracking-wide text-text-muted">{family}</p>
          <div className="flex flex-wrap gap-1">
            {ANIMATION_BLOCKS.filter((block) => block.family === family).map((block) => (
              <button
                key={block.key}
                type="button"
                className="rounded-md border border-border bg-surface-alt px-2 py-1 text-[10px] text-text-muted hover:border-primary/40 hover:text-text"
                title={`${block.description} Click to replace the current ${family} block; Shift-click to stack.`}
                onClick={(event) => onApply(block.key, event.shiftKey ? 'stack' : 'replace')}
              >
                {block.label}
              </button>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
