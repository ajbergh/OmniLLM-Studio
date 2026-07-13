import { useState, useCallback, useEffect, useRef } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import { Search, X, MessageSquare, SlidersHorizontal, Clock, Filter, RefreshCw } from 'lucide-react';
import { api, searchApi, workspaceApi } from '../api';
import type { SearchResult, SearchMode } from '../types';

interface SearchPanelProps {
  open: boolean;
  onClose: () => void;
  onSelectResult?: (conversationId: string, messageId?: string) => void;
}

export function SearchPanel({ open, onClose, onSelectResult }: SearchPanelProps) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [activeResultIndex, setActiveResultIndex] = useState(-1);
  const [loading, setLoading] = useState(false);
  const [searchError, setSearchError] = useState('');
  const [mode, setMode] = useState<SearchMode>('hybrid');
  const [showFilters, setShowFilters] = useState(false);
  const [searched, setSearched] = useState(false);
  const [reindexing, setReindexing] = useState(false);
  const dialogRef = useRef<HTMLDivElement>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);

  const selectResult = useCallback((result: SearchResult) => {
    onSelectResult?.(result.conversation_id, result.message_id);
    onClose();
  }, [onClose, onSelectResult]);

  useEffect(() => {
    if (!open) return;
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    const timer = window.setTimeout(() => searchInputRef.current?.focus(), 0);
    return () => {
      window.clearTimeout(timer);
      document.body.style.overflow = prevOverflow;
    };
  }, [open]);

  const handleSearch = useCallback(async () => {
    if (!query.trim()) return;
    setLoading(true);
    setSearched(true);
    setSearchError('');
    try {
      const data = await searchApi.search(query, mode, 50);
      const nextResults = data.results || [];
      const conversationIds = [...new Set(nextResults.map((result) => result.conversation_id).filter(Boolean))];
      const [conversations, workspaces] = await Promise.all([
        Promise.all(conversationIds.map((id) => api.getConversation(id).catch(() => null))),
        workspaceApi.list().catch(() => []),
      ]);
      const conversationMap = new Map(conversations.filter(Boolean).map((conversation) => [conversation!.id, conversation!]));
      const workspaceMap = new Map(workspaces.map((workspace) => [workspace.id, workspace.name]));
      setResults(nextResults.map((result) => {
        const conversation = conversationMap.get(result.conversation_id);
        return {
          ...result,
          conversation_title: conversation?.title || 'Conversation',
          project_name: conversation?.workspace_id ? workspaceMap.get(conversation.workspace_id) : undefined,
        };
      }));
      setActiveResultIndex(nextResults.length > 0 ? 0 : -1);
    } catch (err) {
      const message = (err as Error).message;
      setSearchError(message);
      toast.error(`Search failed: ${message}`);
      setResults([]);
      setActiveResultIndex(-1);
    } finally {
      setLoading(false);
    }
  }, [query, mode]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      e.preventDefault();
      onClose();
      return;
    }

    if (e.key === 'ArrowDown' && results.length > 0) {
      e.preventDefault();
      setActiveResultIndex((prev) => Math.min(prev + 1, results.length - 1));
      return;
    }

    if (e.key === 'ArrowUp' && results.length > 0) {
      e.preventDefault();
      setActiveResultIndex((prev) => Math.max(prev - 1, 0));
      return;
    }

    if (e.key === 'Enter') {
      e.preventDefault();
      if (activeResultIndex >= 0 && results[activeResultIndex]) {
        selectResult(results[activeResultIndex]);
        return;
      }
      handleSearch();
    }
  };

  const handleDialogKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (e.key === 'Escape') {
      e.preventDefault();
      onClose();
      return;
    }

    if (e.key !== 'Tab') return;
    const container = dialogRef.current;
    if (!container) return;

    const focusable = container.querySelectorAll<HTMLElement>(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
    );
    if (focusable.length === 0) return;

    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  };

  if (!open) return null;

  return (
    <AnimatePresence>
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        className="fixed inset-0 bg-black/50 backdrop-blur-sm z-50 flex items-start justify-center pt-[10vh]"
        onClick={onClose}
      >
        <motion.div
          ref={dialogRef}
          role="dialog"
          aria-modal="true"
          aria-labelledby="search-panel-title"
          tabIndex={-1}
          initial={{ y: -20, opacity: 0 }}
          animate={{ y: 0, opacity: 1 }}
          exit={{ y: -20, opacity: 0 }}
          onKeyDown={handleDialogKeyDown}
          onClick={(e) => e.stopPropagation()}
          className="glass-strong rounded-2xl w-full max-w-2xl max-h-[70vh] overflow-hidden mx-4 flex flex-col"
        >
          {/* Search input */}
          <div className="px-4 py-3 border-b border-border flex items-center gap-2">
            <h2 id="search-panel-title" className="sr-only">Search conversations</h2>
            <Search size={16} className="text-text-muted shrink-0" />
            <input
              ref={searchInputRef}
              type="text"
              autoFocus
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Search conversations..."
              className="flex-1 bg-transparent text-text text-sm placeholder:text-text-muted/60
                         focus:outline-none"
            />
            <div className="flex items-center gap-1">
              <motion.button
                whileHover={{ scale: 1.05 }}
                whileTap={{ scale: 0.95 }}
                onClick={() => setShowFilters(!showFilters)}
                className={`p-1.5 rounded-lg transition-colors ${
                  showFilters ? 'text-primary bg-primary/10' : 'text-text-muted hover:text-text'
                }`}
                aria-label={showFilters ? 'Hide search filters' : 'Show search filters'}
                title={showFilters ? 'Hide filters' : 'Show filters'}
              >
                <SlidersHorizontal size={14} />
              </motion.button>
              <motion.button
                whileHover={{ scale: 1.05 }}
                whileTap={{ scale: 0.95 }}
                onClick={onClose}
                className="p-1.5 rounded-lg text-text-muted hover:text-text"
                aria-label="Close search"
                title="Close search"
              >
                <X size={14} />
              </motion.button>
            </div>
          </div>

          {/* Filters */}
          <AnimatePresence>
            {showFilters && (
              <motion.div
                initial={{ height: 0, opacity: 0 }}
                animate={{ height: 'auto', opacity: 1 }}
                exit={{ height: 0, opacity: 0 }}
                className="px-4 py-2 border-b border-border overflow-hidden"
              >
                <div className="flex items-center gap-2">
                  <Filter size={12} className="text-text-muted" />
                  <span className="text-xs text-text-muted mr-2">Mode:</span>
                  {(['hybrid', 'semantic', 'keyword'] as const).map((m) => (
                    <button
                      key={m}
                      onClick={() => setMode(m)}
                      className={`px-2.5 py-1 rounded-lg text-xs font-medium transition-colors ${
                        mode === m ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text'
                      }`}
                    >
                      {m.charAt(0).toUpperCase() + m.slice(1)}
                    </button>
                  ))}
                  <div className="ml-auto">
                    <button
                      onClick={async () => {
                        setReindexing(true);
                        try {
                          await searchApi.reindex();
                          toast.success('Search index rebuilt');
                        } catch (err) {
                          toast.error(`Reindex failed: ${(err as Error).message}`);
                        } finally {
                          setReindexing(false);
                        }
                      }}
                      disabled={reindexing}
                      className="flex items-center gap-1 px-2 py-1 rounded-lg text-xs text-text-muted hover:text-primary transition-colors disabled:opacity-50"
                      aria-label="Rebuild search index"
                    >
                      <RefreshCw size={11} className={reindexing ? 'animate-spin' : ''} />
                      {reindexing ? 'Reindexing...' : 'Reindex'}
                    </button>
                  </div>
                </div>
              </motion.div>
            )}
          </AnimatePresence>

          {/* Results */}
          <div className="flex-1 overflow-y-auto">
            {loading ? (
              <div className="py-16 text-center">
                <div className="w-5 h-5 border-2 border-primary/30 border-t-primary rounded-full animate-spin mx-auto" />
                <p className="text-xs text-text-muted mt-2">Searching...</p>
              </div>
            ) : searchError ? (
              <div className="py-16 px-6 text-center">
                <p className="text-sm text-danger">Search failed</p>
                <p className="text-xs text-text-muted mt-1 break-words">{searchError}</p>
                <button
                  onClick={handleSearch}
                  className="mt-4 min-h-10 px-4 rounded-xl glass text-sm text-text hover:bg-surface-hover transition-colors"
                >
                  Retry
                </button>
              </div>
            ) : !searched ? (
              <div className="py-16 text-center text-text-muted text-sm">
                <Search size={28} className="mx-auto mb-3 opacity-30" />
                <p>Search across all conversations</p>
                <p className="text-xs mt-1">Supports semantic, keyword, and hybrid search</p>
              </div>
            ) : results.length === 0 ? (
              <div className="py-16 text-center text-text-muted text-sm">
                No results found for "{query}"
              </div>
            ) : (
              <div className="p-2 space-y-1" role="listbox" aria-label="Search results">
                {results.map((r, i) => (
                  <motion.button
                    key={`${r.message_id || r.chunk_id}-${i}`}
                    initial={{ opacity: 0, y: 4 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: i * 0.02 }}
                    role="option"
                    aria-selected={i === activeResultIndex}
                    onMouseEnter={() => setActiveResultIndex(i)}
                    onClick={() => selectResult(r)}
                    className={`w-full text-left p-3 rounded-xl transition-colors group ${
                      i === activeResultIndex
                        ? 'bg-white/10 ring-1 ring-primary/30'
                        : 'hover:bg-white/5'
                    }`}
                  >
                    <div className="flex items-center gap-2 mb-1">
                      <MessageSquare size={12} className="text-primary shrink-0" />
                      <span className="text-xs font-medium text-text truncate">
                        {r.conversation_title || (r.type === 'chunk' ? 'RAG Chunk' : 'Conversation')}
                      </span>
                      {r.role && (
                        <span className="text-xs text-text-muted capitalize ml-auto shrink-0">{r.role}</span>
                      )}
                    </div>
                    {r.project_name && <div className="mb-1 pl-5 text-[10px] font-medium text-primary/80">Project · {r.project_name}</div>}
                    <div className="text-xs text-text-muted/80 line-clamp-3 pl-5">
                      <HighlightedExcerpt content={r.content} query={query} />
                    </div>
                    <div className="flex items-center gap-3 pl-5 mt-1.5">
                      {r.timestamp && (
                        <span className="flex items-center gap-1 text-[10px] text-text-muted/50">
                          <Clock size={10} /> {new Date(r.timestamp).toLocaleDateString()}
                        </span>
                      )}
                    </div>
                  </motion.button>
                ))}
              </div>
            )}
          </div>

          {/* Footer hint */}
          <div className="px-4 py-2 border-t border-border flex items-center justify-between">
            <span className="text-[10px] text-text-muted/50">
              Enter to search/open • Up/Down to navigate • Esc to close
            </span>
            {searched && results.length > 0 && (
              <span className="text-[10px] text-text-muted/50">{results.length} results</span>
            )}
          </div>
        </motion.div>
      </motion.div>
    </AnimatePresence>
  );
}

function HighlightedExcerpt({ content, query }: { content: string; query: string }) {
  const needle = query.trim();
  if (!needle) return content;
  const index = content.toLocaleLowerCase().indexOf(needle.toLocaleLowerCase());
  if (index < 0) return content;
  const start = Math.max(0, index - 90);
  const end = Math.min(content.length, index + needle.length + 140);
  return (
    <>
      {start > 0 ? '…' : ''}{content.slice(start, index)}
      <mark className="rounded bg-primary/25 px-0.5 text-text">{content.slice(index, index + needle.length)}</mark>
      {content.slice(index + needle.length, end)}{end < content.length ? '…' : ''}
    </>
  );
}
