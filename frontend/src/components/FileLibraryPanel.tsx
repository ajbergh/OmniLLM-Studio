import { useCallback, useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { motion, AnimatePresence } from 'framer-motion';
import { FileText, RefreshCw, Trash2, Upload, X, Sparkles, Columns3 } from 'lucide-react';
import { api, fileLibraryApi } from '../api';
import { DialogShell } from './DialogShell';
import { useConversationStore } from '../stores';
import type { Attachment, LibraryFile } from '../types';

interface FileLibraryPanelProps {
  open: boolean;
  onClose: () => void;
  preferredScope?: 'workspace' | 'conversation' | 'global' | 'all';
}

export function FileLibraryPanel({ open, onClose, preferredScope = 'all' }: FileLibraryPanelProps) {
  const activeConversationId = useConversationStore((s) => s.activeId);
  const conversations = useConversationStore((s) => s.conversations);
  const activeConversation = conversations.find((c) => c.id === activeConversationId);
  const [scope, setScope] = useState<'all' | 'conversation' | 'workspace' | 'global'>('all');
  const [query, setQuery] = useState('');
  const [files, setFiles] = useState<LibraryFile[]>([]);
  const [attachments, setAttachments] = useState<Attachment[]>([]);
  const [selectedAttachmentId, setSelectedAttachmentId] = useState('');
  const [selectedFileIds, setSelectedFileIds] = useState<string[]>([]);
  const [summary, setSummary] = useState('');
  const [comparison, setComparison] = useState('');
  const [loading, setLoading] = useState(false);
  const [runningAction, setRunningAction] = useState<string | null>(null);

  const refreshFiles = useCallback(async () => {
    setLoading(true);
    try {
      const list = await fileLibraryApi.list(scope, query.trim() || undefined);
      setFiles(list);
    } catch (err) {
      toast.error(`Failed to load file library: ${(err as Error).message}`);
    } finally {
      setLoading(false);
    }
  }, [scope, query]);

  const refreshAttachments = useCallback(async () => {
    if (!activeConversationId) {
      setAttachments([]);
      setSelectedAttachmentId('');
      return;
    }
    try {
      const list = await api.listAttachments(activeConversationId);
      const fileAttachments = list.filter((a) => a.type === 'file');
      setAttachments(fileAttachments);
      if (!fileAttachments.some((a) => a.id === selectedAttachmentId)) {
        setSelectedAttachmentId(fileAttachments[0]?.id || '');
      }
    } catch {
      setAttachments([]);
      setSelectedAttachmentId('');
    }
  }, [activeConversationId, selectedAttachmentId]);

  useEffect(() => {
    if (!open) return;
    setScope(preferredScope);
    refreshFiles();
    refreshAttachments();
  }, [open, preferredScope, refreshFiles, refreshAttachments]);

  const attachmentNameById = useMemo(() => {
    const map = new Map<string, string>();
    attachments.forEach((a) => map.set(a.id, a.storage_path));
    return map;
  }, [attachments]);

  const toggleFileSelection = (id: string) => {
    setSelectedFileIds((prev) => (prev.includes(id) ? prev.filter((v) => v !== id) : [...prev, id]));
  };

  const handleIngest = async () => {
    if (!selectedAttachmentId) {
      toast.error('Select an attachment first');
      return;
    }
    setRunningAction('ingest');
    try {
      await fileLibraryApi.ingest({
        attachment_id: selectedAttachmentId,
        scope: scope === 'all' ? 'conversation' : scope,
        conversation_id: activeConversationId || undefined,
        workspace_id: scope === 'workspace' ? activeConversation?.workspace_id : undefined,
      });
      toast.success('Attachment indexed into file library');
      await refreshFiles();
    } catch (err) {
      toast.error(`Ingest failed: ${(err as Error).message}`);
    } finally {
      setRunningAction(null);
    }
  };

  const handleDelete = async (fileId: string) => {
    setRunningAction(`delete:${fileId}`);
    try {
      await fileLibraryApi.delete(fileId, false);
      setSelectedFileIds((prev) => prev.filter((id) => id !== fileId));
      await refreshFiles();
      toast.success('File removed from library');
    } catch (err) {
      toast.error(`Delete failed: ${(err as Error).message}`);
    } finally {
      setRunningAction(null);
    }
  };

  const handleReindex = async (fileId: string) => {
    setRunningAction(`reindex:${fileId}`);
    try {
      await fileLibraryApi.reindex(fileId);
      await refreshFiles();
      toast.success('File reindexed');
    } catch (err) {
      toast.error(`Reindex failed: ${(err as Error).message}`);
    } finally {
      setRunningAction(null);
    }
  };

  const handleSummarize = async () => {
    if (selectedFileIds.length === 0) {
      toast.error('Select at least one file');
      return;
    }
    setRunningAction('summarize');
    try {
      const resp = await fileLibraryApi.summarize({
        library_file_ids: selectedFileIds,
        summary_style: 'detailed',
        conversation_id: activeConversationId || undefined,
      });
      setSummary(resp.summary || '');
      setComparison('');
    } catch (err) {
      toast.error(`Summarize failed: ${(err as Error).message}`);
    } finally {
      setRunningAction(null);
    }
  };

  const handleCompare = async () => {
    if (selectedFileIds.length < 2) {
      toast.error('Select at least two files');
      return;
    }
    setRunningAction('compare');
    try {
      const resp = await fileLibraryApi.compare({
        library_file_ids: selectedFileIds,
        output_format: 'markdown',
        conversation_id: activeConversationId || undefined,
      });
      setComparison(resp.comparison || '');
      setSummary('');
    } catch (err) {
      toast.error(`Compare failed: ${(err as Error).message}`);
    } finally {
      setRunningAction(null);
    }
  };

  return (
    <DialogShell
      open={open}
      onClose={onClose}
      title="File Library"
      icon={<FileText size={18} />}
      maxWidth="max-w-4xl"
      maxHeight="max-h-[82vh]"
      bodyClassName="p-4 sm:p-5"
    >
      <div className="space-y-3">
        <div className="rounded-xl border border-primary/20 bg-primary/5 p-3">
          <p className="text-xs font-medium text-primary">Project Files Workflow</p>
          <p className="mt-1 text-xs text-text-muted">
            Upload files once to workspace scope, then every chat in that workspace can use them automatically.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <select
            value={scope}
            onChange={(e) => setScope(e.target.value as 'all' | 'conversation' | 'workspace' | 'global')}
            className="h-10 rounded-xl border border-border bg-surface-alt px-3 text-sm text-text"
          >
            <option value="all">All scopes</option>
            <option value="conversation">Conversation</option>
            <option value="workspace">Workspace</option>
            <option value="global">Global</option>
          </select>
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search file name or metadata"
            className="h-10 min-w-[220px] flex-1 rounded-xl border border-border bg-surface-alt px-3 text-sm text-text"
          />
          <button
            onClick={refreshFiles}
            className="h-10 rounded-xl border border-border px-3 text-sm text-text-muted hover:text-text inline-flex items-center gap-2"
          >
            <RefreshCw size={14} />
            Refresh
          </button>
        </div>

        <div className="rounded-xl border border-border bg-surface-alt/40 p-3">
          <div className="flex flex-wrap items-center gap-2">
            <select
              value={selectedAttachmentId}
              onChange={(e) => setSelectedAttachmentId(e.target.value)}
              className="h-9 min-w-[220px] flex-1 rounded-lg border border-border bg-surface px-3 text-sm text-text"
            >
              {attachments.length === 0 && <option value="">No file attachments in active conversation</option>}
              {attachments.map((a) => (
                <option key={a.id} value={a.id}>
                  {attachmentNameById.get(a.id) || a.id}
                </option>
              ))}
            </select>
            <button
              onClick={handleIngest}
              disabled={!selectedAttachmentId || runningAction === 'ingest'}
              className="h-9 rounded-lg bg-primary px-3 text-sm text-white hover:bg-primary-hover disabled:opacity-50 inline-flex items-center gap-2"
            >
              <Upload size={14} />
              {runningAction === 'ingest' ? 'Indexing...' : 'Ingest Attachment'}
            </button>
          </div>
        </div>

        <div className="max-h-[34vh] overflow-auto rounded-xl border border-border divide-y divide-border/60">
          {loading ? (
            <div className="p-6 text-sm text-text-muted">Loading files...</div>
          ) : files.length === 0 ? (
            <div className="p-6 text-sm text-text-muted">No indexed files found.</div>
          ) : (
            files.map((file) => {
              const selected = selectedFileIds.includes(file.id);
              return (
                <div key={file.id} className="p-3">
                  <div className="flex items-start gap-3">
                    <input
                      type="checkbox"
                      className="mt-1"
                      checked={selected}
                      onChange={() => toggleFileSelection(file.id)}
                    />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center justify-between gap-2">
                        <p className="truncate text-sm font-medium text-text">{file.display_name}</p>
                        <span className="text-[10px] uppercase tracking-[0.08em] text-text-muted">{file.scope}</span>
                      </div>
                      <p className="mt-0.5 text-xs text-text-muted">
                        {file.status} • {(file.size_bytes / 1024).toFixed(1)} KB
                      </p>
                    </div>
                    <div className="flex items-center gap-1">
                      <button
                        onClick={() => handleReindex(file.id)}
                        disabled={runningAction === `reindex:${file.id}`}
                        className="p-1.5 rounded-lg border border-border text-text-muted hover:text-text"
                        title="Reindex file"
                      >
                        <RefreshCw size={13} />
                      </button>
                      <button
                        onClick={() => handleDelete(file.id)}
                        disabled={runningAction === `delete:${file.id}`}
                        className="p-1.5 rounded-lg border border-border text-text-muted hover:text-red-400"
                        title="Delete file"
                      >
                        <Trash2 size={13} />
                      </button>
                    </div>
                  </div>
                </div>
              );
            })
          )}
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <button
            onClick={handleSummarize}
            disabled={runningAction === 'summarize'}
            className="h-9 rounded-lg border border-border bg-surface-alt px-3 text-sm text-text inline-flex items-center gap-2"
          >
            <Sparkles size={14} />
            Summarize Selected
          </button>
          <button
            onClick={handleCompare}
            disabled={runningAction === 'compare'}
            className="h-9 rounded-lg border border-border bg-surface-alt px-3 text-sm text-text inline-flex items-center gap-2"
          >
            <Columns3 size={14} />
            Compare Selected
          </button>
          {selectedFileIds.length > 0 && (
            <button
              onClick={() => setSelectedFileIds([])}
              className="h-9 rounded-lg border border-border px-3 text-sm text-text-muted hover:text-text inline-flex items-center gap-2"
            >
              <X size={14} />
              Clear Selection
            </button>
          )}
        </div>

        <AnimatePresence>
          {(summary || comparison) && (
            <motion.div
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -8 }}
              className="rounded-xl border border-border bg-surface-alt/40 p-3"
            >
              <p className="mb-2 text-xs uppercase tracking-[0.08em] text-text-muted">
                {summary ? 'Summary' : 'Comparison'}
              </p>
              <div className="max-h-[220px] overflow-auto whitespace-pre-wrap text-sm text-text">
                {summary || comparison}
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </DialogShell>
  );
}
