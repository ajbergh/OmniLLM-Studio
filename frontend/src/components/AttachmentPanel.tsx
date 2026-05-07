import { useState, useEffect, useCallback } from 'react';
import { api } from '../api';
import { motion } from 'framer-motion';
import { toast } from 'sonner';
import { Paperclip, Download, Trash2, FileText, Image, File } from 'lucide-react';
import { DialogShell } from './DialogShell';
import type { Attachment } from '../types';

interface AttachmentPanelProps {
  conversationId: string;
  open: boolean;
  onClose: () => void;
}

export function AttachmentPanel({ conversationId, open, onClose }: AttachmentPanelProps) {
  const [attachments, setAttachments] = useState<Attachment[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState('');

  const fetchAttachments = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      const data = await api.listAttachments(conversationId);
      setAttachments(data || []);
    } catch (err) {
      const message = (err as Error).message;
      setLoadError(message);
      toast.error(`Failed to load attachments: ${message}`);
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
    <DialogShell
      open={open}
      onClose={onClose}
      title={`Attachments (${attachments.length})`}
      icon={<Paperclip size={16} />}
      maxWidth="max-w-lg"
      maxHeight="max-h-[70vh]"
      bodyClassName="p-4"
    >
      {loading ? (
        <div className="py-12 text-center">
          <div className="w-5 h-5 border-2 border-primary/30 border-t-primary rounded-full animate-spin mx-auto" />
          <p className="text-xs text-text-muted mt-2">Loading attachments...</p>
        </div>
      ) : loadError ? (
        <div className="py-12 text-center">
          <p className="text-sm text-danger">Failed to load attachments</p>
          <p className="text-xs text-text-muted mt-1 break-words">{loadError}</p>
          <button
            onClick={fetchAttachments}
            className="mt-4 min-h-10 px-4 rounded-xl glass text-sm text-text hover:bg-surface-hover transition-colors"
          >
            Retry
          </button>
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
            const name = att.storage_path.split('/').pop() || 'Attachment';
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
                    {name}
                  </p>
                  <p className="text-[11px] text-text-muted break-words">
                    {att.mime_type} · {formatBytes(att.bytes)}
                    {att.width && att.height && ` · ${att.width}×${att.height}`}
                  </p>
                </div>
                <div className="flex items-center gap-1 opacity-100 sm:opacity-0 sm:group-hover:opacity-100 transition-opacity">
                  <motion.button
                    whileTap={{ scale: 0.9 }}
                    onClick={() => handleDownload(att.id)}
                    className="min-h-10 min-w-10 inline-flex items-center justify-center rounded-lg text-text-muted hover:text-primary hover:bg-primary/10 transition-colors"
                    aria-label={`Download ${name}`}
                    title="Download"
                  >
                    <Download size={14} />
                  </motion.button>
                  <motion.button
                    whileTap={{ scale: 0.9 }}
                    onClick={() => handleDelete(att.id)}
                    className="min-h-10 min-w-10 inline-flex items-center justify-center rounded-lg text-text-muted hover:text-red-400 hover:bg-red-400/10 transition-colors"
                    aria-label={`Delete ${name}`}
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
    </DialogShell>
  );
}
