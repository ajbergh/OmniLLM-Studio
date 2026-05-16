import { Plus, Trash2 } from 'lucide-react';
import { clsx } from 'clsx';
import type { MusicSession } from '../../types/music';

interface MusicSidebarProps {
  sessions: MusicSession[];
  activeSessionId: string | null;
  onNew: () => void;
  onSelect: (sessionId: string) => void;
  onDelete: (sessionId: string) => void;
}

export function MusicSidebar({ sessions, activeSessionId, onNew, onSelect, onDelete }: MusicSidebarProps) {
  return (
    <section className="rounded-xl border border-border bg-surface-raised p-3">
      <div className="mb-2 flex items-center justify-between gap-2">
        <h2 className="text-xs font-semibold uppercase tracking-wide text-text-muted">Sessions</h2>
        <button
          onClick={onNew}
          className="min-h-8 min-w-8 inline-flex items-center justify-center rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors"
          aria-label="New music session"
          title="New session"
        >
          <Plus size={14} />
        </button>
      </div>

      {sessions.length === 0 ? (
        <button
          onClick={onNew}
          className="w-full rounded-xl border border-dashed border-border bg-surface-alt px-3 py-4 text-center text-xs text-text-muted hover:text-text hover:border-primary/40 transition-colors"
        >
          New track session
        </button>
      ) : (
        <div className="max-h-44 space-y-1 overflow-y-auto pr-1">
          {sessions.map((session) => (
            <div
              key={session.id}
              className={clsx(
                'group flex items-center gap-2 rounded-xl border px-2 py-2 transition-colors',
                session.id === activeSessionId
                  ? 'border-primary/30 bg-primary/10 text-text'
                  : 'border-transparent text-text-secondary hover:bg-surface-hover hover:text-text'
              )}
            >
              <button
                onClick={() => onSelect(session.id)}
                className="min-w-0 flex-1 text-left"
              >
                <span className="block truncate text-sm">{session.title}</span>
                <span className="mt-0.5 block truncate text-[10px] text-text-muted">
                  {session.default_provider || 'provider'} · {session.default_model || 'model'}
                </span>
              </button>
              <button
                onClick={() => onDelete(session.id)}
                className="min-h-8 min-w-8 rounded-lg text-text-muted opacity-100 transition-colors hover:bg-danger-soft hover:text-danger md:opacity-0 md:group-hover:opacity-100"
                aria-label={`Delete ${session.title}`}
                title="Delete session"
              >
                <Trash2 size={13} className="mx-auto" />
              </button>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
