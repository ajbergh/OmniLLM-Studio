import { useState, useCallback, useEffect, useRef } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import { Search, X, MessageSquare, SlidersHorizontal, Clock, Filter, RefreshCw } from 'lucide-react';
import { searchApi } from '../api';
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
    try {
      const data = await searchApi.search(query, mode, 50);
      const nextResults = data.results || [];
      setResults(nextResults);
      setActiveResultIndex(nextResults.length > 0 ? 0 : -1);
    } catch (err) {
      toast.error(`Search failed: ${(err as Error).message}`);
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
              >
                <SlidersHorizontal size={14} />
              </motion.button>
              <motion.button
                whileHover={{ scale: 1.05 }}
                whileTap={{ scale: 0.95 }}
                onClick={onClose}
                className="p-1.5 rounded-lg text-text-muted hover:text-text"
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
                        {r.type === 'chunk' ? 'RAG Chunk' : (r.role || 'message')}
                      </span>
                      {r.role && (
                        <span className="text-xs text-text-muted capitalize ml-auto shrink-0">{r.role}</span>
                      )}
                    </div>
                    <div className="text-xs text-text-muted/80 line-clamp-2 pl-5">
                      {r.content}
                    </div>
                    <div className="flex items-center gap-3 pl-5 mt-1.5">
                      {r.timestamp && (
                        <span className="flex items-center gap-1 text-[10px] text-text-muted/50">
                          <Clock size={10} /> {new Date(r.timestamp).toLocaleDateString()}
                        </span>
                      )}
                      {r.score > 0 && (
                        <span className="text-[10px] text-text-muted/50">
                          Score: {(r.score * 100).toFixed(0)}
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
