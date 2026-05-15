import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Link, ChevronDown, ChevronUp, ExternalLink, AlertTriangle, Database, Globe } from 'lucide-react';
import type { URLContextSourceRef } from '../types';

const KIND_LABELS: Record<string, string> = {
  github_repo: 'GitHub Repo',
  github_file: 'GitHub File',
  github_directory: 'GitHub Dir',
  github_raw: 'GitHub Raw',
  webpage: 'Web Page',
  pdf: 'PDF',
  unknown: 'URL',
};

interface URLContextSourcePanelProps {
  sources: URLContextSourceRef[];
  usedRag?: boolean;
  warnings?: string[];
}

export function URLContextSourcePanel({ sources, usedRag, warnings }: URLContextSourcePanelProps) {
  const [expanded, setExpanded] = useState(false);

  if (sources.length === 0) return null;

  return (
    <div className="glass rounded-xl overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between px-4 py-3 text-sm hover:bg-surface-light/50 transition-colors"
      >
        <div className="flex items-center gap-2 text-amber-400/80">
          <Link size={14} />
          <span className="font-medium text-text-muted">Sources Inspected</span>
          <span className="text-xs px-1.5 py-0.5 rounded-full bg-amber-400/15 text-amber-400">
            {sources.length}
          </span>
          {usedRag && (
            <span className="flex items-center gap-1 text-xs px-1.5 py-0.5 rounded-full bg-primary/15 text-primary">
              <Database size={10} />
              RAG
            </span>
          )}
        </div>
        {expanded ? (
          <ChevronUp size={14} className="text-text-muted" />
        ) : (
          <ChevronDown size={14} className="text-text-muted" />
        )}
      </button>

      <AnimatePresence>
        {expanded && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            className="overflow-hidden"
          >
            <div className="px-4 pb-3 space-y-2">
              {warnings && warnings.length > 0 && (
                <div className="flex items-start gap-2 p-2.5 rounded-lg bg-amber-400/10 text-xs text-amber-300">
                  <AlertTriangle size={12} className="mt-0.5 shrink-0" />
                  <ul className="space-y-0.5">
                    {warnings.map((w, i) => (
                      <li key={i}>{w}</li>
                    ))}
                  </ul>
                </div>
              )}

              <div className="space-y-1.5">
                {sources.map((src) => (
                  <div
                    key={src.id}
                    className="flex items-start gap-2.5 p-2.5 rounded-lg bg-surface-light/50 text-xs"
                  >
                    <Link size={12} className="text-amber-400 mt-0.5 shrink-0" />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-1.5 flex-wrap">
                        <span className="font-medium text-text truncate max-w-xs">
                          {src.title || src.url}
                        </span>
                        <span className="px-1 py-0.5 rounded bg-surface text-text-muted text-[10px] shrink-0">
                          {KIND_LABELS[src.kind] ?? src.kind}
                        </span>
                        {src.loaded_via_browser && (
                          <span className="inline-flex items-center gap-1 px-1 py-0.5 rounded bg-cyan-400/10 text-cyan-300 text-[10px] shrink-0">
                            <Globe size={9} />
                            via browser
                          </span>
                        )}
                      </div>
                      <div className="flex items-center gap-1 mt-0.5 text-text-muted">
                        <span className="truncate max-w-sm">{src.final_url || src.url}</span>
                        <a
                          href={src.final_url || src.url}
                          target="_blank"
                          rel="noopener noreferrer"
                          onClick={(e) => e.stopPropagation()}
                          className="shrink-0 hover:text-primary transition-colors"
                        >
                          <ExternalLink size={10} />
                        </a>
                      </div>
                      {src.fetched_at && (
                        <div className="text-text-muted mt-0.5">
                          {new Date(src.fetched_at).toLocaleString()}
                        </div>
                      )}
                    </div>
                  </div>
                ))}
              </div>

              {usedRag && (
                <p className="text-[11px] text-text-muted pt-1">
                  Content indexed into RAG — follow-up questions will retrieve relevant excerpts.
                </p>
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
