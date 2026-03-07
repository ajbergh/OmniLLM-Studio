import { useState, useEffect, useCallback } from 'react';
import { api } from '../api';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import { FileText, ChevronDown, ChevronUp, RefreshCw, Database } from 'lucide-react';
import type { DocumentChunk } from '../types';

interface RAGSourcePanelProps {
  conversationId: string;
  attachmentId?: string;
}

export function RAGSourcePanel({ conversationId, attachmentId }: RAGSourcePanelProps) {
  const [chunks, setChunks] = useState<DocumentChunk[]>([]);
  const [loading, setLoading] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const [reindexing, setReindexing] = useState(false);

  const fetchChunks = useCallback(async () => {
    setLoading(true);
    try {
      const data = attachmentId
        ? await api.listAttachmentChunks(attachmentId)
        : await api.listChunks(conversationId);
      setChunks(data || []);
    } catch (err) {
      toast.error(`Failed to load chunks: ${(err as Error).message}`);
    } finally {
      setLoading(false);
    }
  }, [conversationId, attachmentId]);

  useEffect(() => {
    if (expanded) fetchChunks();
  }, [expanded, fetchChunks]);

  const handleReindex = async () => {
    setReindexing(true);
    try {
      if (attachmentId) {
        await api.indexAttachment(attachmentId);
        toast.success('Attachment indexed');
      } else {
        await api.reindexConversation(conversationId);
        toast.success('Conversation reindexed');
      }
      fetchChunks();
    } catch (err) {
      toast.error(`Reindex failed: ${(err as Error).message}`);
    } finally {
      setReindexing(false);
    }
  };

  return (
    <div className="glass rounded-xl overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between px-4 py-3 text-sm hover:bg-surface-light/50 transition-colors"
      >
        <div className="flex items-center gap-2 text-text-muted">
          <Database size={14} />
          <span className="font-medium">RAG Sources</span>
          {chunks.length > 0 && (
            <span className="text-xs px-1.5 py-0.5 rounded-full bg-primary/20 text-primary">
              {chunks.length}
            </span>
          )}
        </div>
        {expanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
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
              <div className="flex items-center justify-end">
                <motion.button
                  whileHover={{ scale: 1.05 }}
                  whileTap={{ scale: 0.95 }}
                  onClick={handleReindex}
                  disabled={reindexing}
                  className="flex items-center gap-1.5 text-xs text-primary hover:text-primary/80 transition-colors disabled:opacity-50"
                >
                  <RefreshCw size={12} className={reindexing ? 'animate-spin' : ''} />
                  Reindex
                </motion.button>
              </div>

              {loading ? (
                <div className="py-4 text-center text-text-muted text-xs">Loading chunks...</div>
              ) : chunks.length === 0 ? (
                <div className="py-4 text-center text-text-muted text-xs">No indexed chunks yet</div>
              ) : (
                <div className="max-h-60 overflow-y-auto space-y-1.5">
                  {chunks.map((chunk) => (
                    <div key={chunk.id} className="p-2.5 rounded-lg bg-surface-light/50 text-xs">
                      <div className="flex items-center gap-1.5 text-text-muted mb-1">
                        <FileText size={10} />
                        <span>Chunk #{chunk.chunk_index + 1}</span>
                        {chunk.token_count && (
                          <span className="ml-auto">{chunk.token_count} tokens</span>
                        )}
                      </div>
                      <p className="text-text line-clamp-3">{chunk.content}</p>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
