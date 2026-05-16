import { GitBranch, MoreHorizontal, Music2, RefreshCw } from 'lucide-react';
import { clsx } from 'clsx';
import type { MusicGenerationDetail } from '../../types/music';

interface MusicHistoryPanelProps {
  generations: MusicGenerationDetail[];
  activeGenerationId: string | null;
  onSelect: (generationId: string) => void;
  onBranch: (generationId: string) => void;
  onRegenerate: (generationId: string) => void;
}

export function MusicHistoryPanel({
  generations,
  activeGenerationId,
  onSelect,
  onBranch,
  onRegenerate,
}: MusicHistoryPanelProps) {
  return (
    <section className="flex h-full min-h-0 flex-col">
      <div className="border-b border-border p-3">
        <h2 className="text-xs font-semibold uppercase tracking-wide text-text-muted">History</h2>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto p-2">
        {generations.length === 0 ? (
          <div className="flex h-full min-h-48 flex-col items-center justify-center text-center text-text-muted/60">
            <Music2 size={28} className="mb-2 opacity-40" />
            <p className="text-sm">No generations yet</p>
          </div>
        ) : (
          generations.map((generation) => (
            <div
              key={generation.id}
              className={clsx(
                'group mb-1 rounded-xl border p-2 transition-colors',
                generation.id === activeGenerationId
                  ? 'border-primary/30 bg-primary/10'
                  : 'border-transparent hover:border-border hover:bg-surface-hover'
              )}
            >
              <button
                onClick={() => onSelect(generation.id)}
                className="w-full text-left"
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="min-w-0 truncate text-sm font-medium text-text">{generation.title || 'Untitled Track'}</span>
                  <StatusBadge status={generation.status} />
                </div>
                <p className="mt-1 line-clamp-2 text-xs text-text-muted">{generation.prompt}</p>
                <div className="mt-2 flex flex-wrap gap-1.5 text-[10px] text-text-muted">
                  <span className="rounded-md border border-border bg-surface-alt px-1.5 py-0.5">{generation.provider}</span>
                  <span className="rounded-md border border-border bg-surface-alt px-1.5 py-0.5">{generation.model}</span>
                  {generation.cost_usd != null && (
                    <span className="rounded-md border border-border bg-surface-alt px-1.5 py-0.5">${generation.cost_usd.toFixed(4)}</span>
                  )}
                </div>
              </button>
              <div className="mt-2 flex items-center gap-1 opacity-100 md:opacity-0 md:group-hover:opacity-100 transition-opacity">
                <button
                  onClick={() => onBranch(generation.id)}
                  className="min-h-8 inline-flex items-center gap-1 rounded-lg px-2 text-[11px] text-text-muted hover:text-text hover:bg-surface-alt transition-colors"
                >
                  <GitBranch size={12} />
                  Branch
                </button>
                <button
                  onClick={() => onRegenerate(generation.id)}
                  className="min-h-8 inline-flex items-center gap-1 rounded-lg px-2 text-[11px] text-text-muted hover:text-text hover:bg-surface-alt transition-colors"
                >
                  <RefreshCw size={12} />
                  Regenerate
                </button>
                <span className="ml-auto text-text-muted/50">
                  <MoreHorizontal size={13} />
                </span>
              </div>
            </div>
          ))
        )}
      </div>
    </section>
  );
}

function StatusBadge({ status }: { status: string }) {
  const color = status === 'completed'
    ? 'border-success/30 bg-success/10 text-success'
    : status === 'failed'
      ? 'border-danger/30 bg-danger-soft text-danger'
      : 'border-primary/30 bg-primary/10 text-primary';
  return (
    <span className={clsx('shrink-0 rounded-md border px-1.5 py-0.5 text-[9px] font-medium uppercase', color)}>
      {status}
    </span>
  );
}
