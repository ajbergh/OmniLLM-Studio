import { useState, useEffect, useCallback } from 'react';
import { api } from '../api';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import { Paperclip, Download, Trash2, X, FileText, Image, File } from 'lucide-react';
import type { Attachment } from '../types';

interface AttachmentPanelProps {
  conversationId: string;
  open: boolean;
  onClose: () => void;
}

export function AttachmentPanel({ conversationId, open, onClose }: AttachmentPanelProps) {
  const [attachments, setAttachments] = useState<Attachment[]>([]);
  const [loading, setLoading] = useState(false);

  const fetchAttachments = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.listAttachments(conversationId);
      setAttachments(data || []);
    } catch (err) {
      toast.error(`Failed to load attachments: ${(err as Error).message}`);
    } finally {
      setLoading(false);
    }
  }, [conversationId]);

  useEffect(() => {
    if (open) fetchAttachments();
  }, [open, fetchAttachments]);

  const handleDelete = async (id: string) => {
    try {
      await api.deleteAttachment(id);
      setAttachments((prev) => prev.filter((a) => a.id !== id));
      toast.success('Attachment deleted');
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  const handleDownload = (id: string) => {
    const url = api.downloadAttachmentUrl(id);
    window.open(url, '_blank');
  };

  const getIcon = (mimeType: string) => {
    if (mimeType.startsWith('image/')) return Image;
    if (mimeType.includes('pdf') || mimeType.includes('text')) return FileText;
    return File;
  };

  const formatBytes = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  if (!open) return null;

  return (
    <AnimatePresence>
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        className="fixed inset-0 bg-black/50 backdrop-blur-sm z-50 flex items-center justify-center"
        onClick={onClose}
      >
        <motion.div
          initial={{ scale: 0.95, opacity: 0 }}
          animate={{ scale: 1, opacity: 1 }}
          exit={{ scale: 0.95, opacity: 0 }}
          onClick={(e) => e.stopPropagation()}
          className="glass-strong rounded-2xl w-full max-w-lg mx-4 max-h-[60vh] flex flex-col overflow-hidden"
        >
          {/* Header */}
          <div className="px-5 py-4 border-b border-border flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Paperclip size={16} className="text-primary" />
              <h3 className="text-sm font-bold text-text">Attachments</h3>
              <span className="text-xs text-text-muted">({attachments.length})</span>
            </div>
            <motion.button
              whileHover={{ scale: 1.1 }}
              whileTap={{ scale: 0.9 }}
              onClick={onClose}
              className="p-1.5 rounded-lg hover:bg-surface-hover text-text-muted hover:text-text transition-colors"
            >
              <X size={16} />
            </motion.button>
          </div>

          {/* Content */}
          <div className="flex-1 overflow-y-auto p-4">
            {loading ? (
              <div className="py-12 text-center">
                <div className="w-5 h-5 border-2 border-primary/30 border-t-primary rounded-full animate-spin mx-auto" />
                <p className="text-xs text-text-muted mt-2">Loading attachments...</p>
              </div>
            ) : attachments.length === 0 ? (
              <div className="py-12 text-center text-text-muted">
                <Paperclip size={28} className="mx-auto mb-3 opacity-30" />
                <p className="text-sm">No attachments in this conversation</p>
              </div>
            ) : (
              <div className="space-y-2">
                {attachments.map((att) => {
                  const Icon = getIcon(att.mime_type);
                  return (
                    <motion.div
                      key={att.id}
                      initial={{ opacity: 0, y: 4 }}
                      animate={{ opacity: 1, y: 0 }}
                      className="flex items-center gap-3 p-3 rounded-xl bg-surface-light/50 group hover:bg-surface-light transition-colors"
                    >
                      <div className="w-9 h-9 rounded-lg bg-primary/10 flex items-center justify-center shrink-0">
                        <Icon size={16} className="text-primary" />
                      </div>
                      <div className="flex-1 min-w-0">
                        <p className="text-sm text-text font-medium truncate">
                          {att.storage_path.split('/').pop() || 'Attachment'}
                        </p>
                        <p className="text-[11px] text-text-muted">
                          {att.mime_type} · {formatBytes(att.bytes)}
                          {att.width && att.height && ` · ${att.width}×${att.height}`}
                        </p>
                      </div>
                      <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                        <motion.button
                          whileTap={{ scale: 0.9 }}
                          onClick={() => handleDownload(att.id)}
                          className="p-1.5 rounded-lg text-text-muted hover:text-primary hover:bg-primary/10 transition-colors"
                          title="Download"
                        >
                          <Download size={14} />
                        </motion.button>
                        <motion.button
                          whileTap={{ scale: 0.9 }}
                          onClick={() => handleDelete(att.id)}
                          className="p-1.5 rounded-lg text-text-muted hover:text-red-400 hover:bg-red-400/10 transition-colors"
                          title="Delete"
                        >
                          <Trash2 size={14} />
                        </motion.button>
                      </div>
                    </motion.div>
                  );
                })}
              </div>
            )}
          </div>
        </motion.div>
      </motion.div>
    </AnimatePresence>
  );
}
