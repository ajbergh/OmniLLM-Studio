import { useCallback, useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { CheckCircle2, FileText, RefreshCw, Trash2, Upload } from 'lucide-react';
import { api, fileLibraryApi, workspaceApi } from '../api';
import { DialogShell } from './DialogShell';
import type { Attachment, Conversation, LibraryFile, Workspace } from '../types';

interface FileLibraryPanelProps {
  open: boolean;
  onClose: () => void;
  preferredScope?: 'workspace' | 'conversation' | 'global' | 'all';
  preferredWorkspaceId?: string | null;
}

interface ProjectAttachment {
  attachment: Attachment;
  conversationId: string;
  conversationTitle: string;
}

function attachmentOriginalName(att: Attachment): string {
  if (att.metadata_json) {
    try {
      const parsed = JSON.parse(att.metadata_json) as { original_name?: string };
      if (parsed.original_name && parsed.original_name.trim()) {
        return parsed.original_name.trim();
      }
    } catch {
      // ignore malformed metadata
    }
  }
  return att.storage_path;
}

function formatKB(sizeBytes: number): string {
  return `${(sizeBytes / 1024).toFixed(1)} KB`;
}

export function FileLibraryPanel({ open, onClose, preferredWorkspaceId = null }: FileLibraryPanelProps) {
  const [projects, setProjects] = useState<Workspace[]>([]);
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null);
  const [projectWorkspaceName, setProjectWorkspaceName] = useState('');
  const [projectConversations, setProjectConversations] = useState<Conversation[]>([]);
  const [projectAttachments, setProjectAttachments] = useState<ProjectAttachment[]>([]);

  const [query, setQuery] = useState('');
  const [files, setFiles] = useState<LibraryFile[]>([]);
  const [selectedAttachmentId, setSelectedAttachmentId] = useState('');
  const [loading, setLoading] = useState(false);
  const [runningAction, setRunningAction] = useState<string | null>(null);

  const loadProjects = useCallback(async () => {
    try {
      const list = await workspaceApi.list();
      setProjects(list);
      return list;
    } catch {
      setProjects([]);
      return [] as Workspace[];
    }
  }, []);

  const refreshProjectContext = useCallback(async (workspaceID: string) => {
    const conversationIDs = new Set<string>();

    try {
      const [workspace, convoList] = await Promise.all([
        workspaceApi.get(workspaceID),
        api.listConversations(false, workspaceID),
      ]);

      setProjectWorkspaceName(workspace.name || 'Selected Project');
      setProjectConversations(convoList);
      convoList.forEach((c) => conversationIDs.add(c.id));

      const attachmentLists = await Promise.all(
        convoList.map(async (convo) => {
          try {
            const list = await api.listAttachments(convo.id);
            return list
              .filter((a) => a.type === 'file')
              .map((attachment) => ({
                attachment,
                conversationId: convo.id,
                conversationTitle: convo.title || 'Untitled chat',
              }));
          } catch {
            return [] as ProjectAttachment[];
          }
        })
      );

      const flattened = attachmentLists
        .flat()
        .sort((a, b) => new Date(b.attachment.created_at).getTime() - new Date(a.attachment.created_at).getTime());
      setProjectAttachments(flattened);
    } catch {
      setProjectWorkspaceName('Selected Project');
      setProjectConversations([]);
      setProjectAttachments([]);
    }

    return { conversationIDs };
  }, []);

  const refreshProjectFiles = useCallback(async (workspaceID: string, conversationIDs: Set<string>) => {
    setLoading(true);
    try {
      const [workspaceFiles, conversationFiles] = await Promise.all([
        fileLibraryApi.list('workspace', query.trim() || undefined),
        fileLibraryApi.list('conversation', query.trim() || undefined),
      ]);

      const projectWorkspaceFiles = workspaceFiles.filter((f) => f.workspace_id === workspaceID);
      const projectConversationFiles = conversationFiles.filter(
        (f) => !!f.conversation_id && conversationIDs.has(f.conversation_id)
      );

      const deduped = new Map<string, LibraryFile>();
      [...projectWorkspaceFiles, ...projectConversationFiles].forEach((f) => deduped.set(f.id, f));
      setFiles(Array.from(deduped.values()));
    } catch (err) {
      toast.error(`Failed to load project files: ${(err as Error).message}`);
      setFiles([]);
    } finally {
      setLoading(false);
    }
  }, [query]);

  const refreshAll = useCallback(async () => {
    if (!selectedProjectId) {
      setProjectWorkspaceName('');
      setProjectConversations([]);
      setProjectAttachments([]);
      setFiles([]);
      return;
    }

    const context = await refreshProjectContext(selectedProjectId);
    await refreshProjectFiles(selectedProjectId, context.conversationIDs);
  }, [selectedProjectId, refreshProjectContext, refreshProjectFiles]);

  useEffect(() => {
    if (!open) return;

    loadProjects().then((list) => {
      let initialProjectID = preferredWorkspaceId;
      if (!initialProjectID && list.length > 0) {
        initialProjectID = list[0].id;
      }
      setSelectedProjectId(initialProjectID || null);
    });
  }, [open, preferredWorkspaceId, loadProjects]);

  useEffect(() => {
    if (!open || !selectedProjectId) {
      return;
    }

    const contextPromise = refreshProjectContext(selectedProjectId);
    contextPromise.then((context) => refreshProjectFiles(selectedProjectId, context.conversationIDs));
  }, [open, selectedProjectId, query, refreshProjectContext, refreshProjectFiles]);

  const selectedProjectAttachment = useMemo(
    () => projectAttachments.find((entry) => entry.attachment.id === selectedAttachmentId),
    [projectAttachments, selectedAttachmentId]
  );

  const selectableAttachments = useMemo(
    () => projectAttachments.map((entry) => ({
      id: entry.attachment.id,
      label: `${attachmentOriginalName(entry.attachment)} - ${entry.conversationTitle}`,
    })),
    [projectAttachments]
  );

  useEffect(() => {
    if (selectableAttachments.some((a) => a.id === selectedAttachmentId)) {
      return;
    }
    setSelectedAttachmentId(selectableAttachments[0]?.id || '');
  }, [selectableAttachments, selectedAttachmentId]);

  const indexedAttachmentIDs = useMemo(() => {
    const out = new Set<string>();
    files.forEach((f) => {
      if (f.attachment_id) {
        out.add(f.attachment_id);
      }
    });
    return out;
  }, [files]);

  const attachmentNameByID = useMemo(() => {
    const map = new Map<string, string>();
    projectAttachments.forEach((entry) => map.set(entry.attachment.id, attachmentOriginalName(entry.attachment)));
    return map;
  }, [projectAttachments]);

  const handleIngest = async () => {
    if (!selectedProjectId) {
      toast.error('Select a project first');
      return;
    }
    if (!selectedAttachmentId) {
      toast.error('Select an attachment first');
      return;
    }

    setRunningAction('ingest');
    try {
      await fileLibraryApi.ingest({
        attachment_id: selectedAttachmentId,
        scope: 'workspace',
        conversation_id: selectedProjectAttachment?.conversationId,
        workspace_id: selectedProjectId,
      });
      toast.success('File indexed to project');
      await refreshAll();
    } catch (err) {
      toast.error(`Indexing failed: ${(err as Error).message}`);
    } finally {
      setRunningAction(null);
    }
  };

  const handleDelete = async (fileId: string) => {
    setRunningAction(`delete:${fileId}`);
    try {
      await fileLibraryApi.delete(fileId, false);
      await refreshAll();
      toast.success('File removed from project files');
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
      await refreshAll();
      toast.success('File re-indexed');
    } catch (err) {
      toast.error(`Re-index failed: ${(err as Error).message}`);
    } finally {
      setRunningAction(null);
    }
  };

  const handleRevalidateProject = async () => {
    if (!selectedProjectId) {
      toast.error('Select a project first');
      return;
    }
    if (files.length === 0) {
      toast.error('No indexed files found for this project');
      return;
    }

    setRunningAction('revalidate_project');
    try {
      for (const file of files) {
        await fileLibraryApi.reindex(file.id);
      }
      toast.success(`Re-validated ${files.length} file${files.length !== 1 ? 's' : ''}`);
      await refreshAll();
    } catch (err) {
      toast.error(`Re-validation failed: ${(err as Error).message}`);
    } finally {
      setRunningAction(null);
    }
  };

  return (
    <DialogShell
      open={open}
      onClose={onClose}
      title="Project Files"
      icon={<FileText size={18} />}
      maxWidth="max-w-4xl"
      maxHeight="max-h-[82vh]"
      bodyClassName="p-4 sm:p-5"
    >
      <div className="space-y-3">
        <div className="rounded-xl border border-primary/20 bg-primary/5 p-3">
          {selectedProjectId ? (
            <>
              <p className="text-xs font-medium text-primary">Project: {projectWorkspaceName || 'Selected Project'}</p>
              <p className="mt-1 text-xs text-text-muted">
                All files attached in this project are listed here, with indexing status for RAG.
              </p>
              <p className="mt-1 text-[11px] text-text-muted">
                {projectConversations.length} chat{projectConversations.length !== 1 ? 's' : ''} · {projectAttachments.length} attached file{projectAttachments.length !== 1 ? 's' : ''} · {files.length} indexed file{files.length !== 1 ? 's' : ''}
              </p>
            </>
          ) : (
            <>
              <p className="text-xs font-medium text-primary">No project selected</p>
              <p className="mt-1 text-xs text-text-muted">Create a project, then select it here to manage Project Files.</p>
            </>
          )}
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <select
            value={selectedProjectId || ''}
            onChange={(e) => setSelectedProjectId(e.target.value || null)}
            className="h-10 min-w-[220px] rounded-xl border border-border bg-surface-alt px-3 text-sm text-text"
          >
            {projects.length === 0 ? (
              <option value="">No projects created</option>
            ) : (
              projects.map((project) => (
                <option key={project.id} value={project.id}>{project.name}</option>
              ))
            )}
          </select>
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search file name or metadata"
            className="h-10 min-w-[220px] flex-1 rounded-xl border border-border bg-surface-alt px-3 text-sm text-text"
          />
          <button
            onClick={refreshAll}
            className="h-10 rounded-xl border border-border px-3 text-sm text-text-muted hover:text-text inline-flex items-center gap-2"
          >
            <RefreshCw size={14} />
            Refresh
          </button>
        </div>

        <div className="rounded-xl border border-border bg-surface-alt/40 p-3">
          <p className="mb-2 text-xs font-medium text-text">Choose a project attachment to index</p>
          <div className="flex flex-wrap items-center gap-2">
            <select
              value={selectedAttachmentId}
              onChange={(e) => setSelectedAttachmentId(e.target.value)}
              className="h-9 min-w-[220px] flex-1 rounded-lg border border-border bg-surface px-3 text-sm text-text"
            >
              {selectableAttachments.length === 0 && <option value="">No file attachments found in this project</option>}
              {selectableAttachments.map((a) => (
                <option key={a.id} value={a.id}>{a.label}</option>
              ))}
            </select>
            <button
              onClick={handleIngest}
              disabled={!selectedProjectId || !selectedAttachmentId || runningAction === 'ingest'}
              className="h-9 rounded-lg bg-primary px-3 text-sm text-white hover:bg-primary-hover disabled:opacity-50 inline-flex items-center gap-2"
            >
              <Upload size={14} />
              {runningAction === 'ingest' ? 'Indexing...' : 'Index Attachment'}
            </button>
          </div>
        </div>

        <div className="rounded-xl border border-border bg-surface-alt/30 p-3">
          <div className="mb-2 flex items-center justify-between">
            <p className="text-xs font-medium text-text">Files Attached In This Project</p>
            <span className="text-[11px] text-text-muted">{projectAttachments.length}</span>
          </div>
          <div className="max-h-36 overflow-auto space-y-1.5">
            {projectAttachments.length === 0 ? (
              <p className="text-xs text-text-muted">No file attachments found in project chats yet.</p>
            ) : (
              projectAttachments.map((entry) => {
                const original = attachmentOriginalName(entry.attachment);
                const indexed = indexedAttachmentIDs.has(entry.attachment.id);
                return (
                  <div key={entry.attachment.id} className="rounded-lg border border-border/70 bg-surface/40 px-2 py-1.5 flex items-center gap-2">
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-xs text-text">{original}</p>
                      <p className="truncate text-[11px] text-text-muted">{entry.conversationTitle}</p>
                    </div>
                    <span className={indexed ? 'text-[10px] rounded-full px-2 py-0.5 bg-emerald-500/15 text-emerald-300' : 'text-[10px] rounded-full px-2 py-0.5 bg-amber-500/15 text-amber-300'}>
                      {indexed ? 'Indexed' : 'Not Indexed'}
                    </span>
                  </div>
                );
              })
            )}
          </div>
        </div>

        <div className="max-h-[34vh] overflow-auto rounded-xl border border-border divide-y divide-border/60">
          {loading ? (
            <div className="p-6 text-sm text-text-muted">Loading files...</div>
          ) : files.length === 0 ? (
            <div className="p-6 text-sm text-text-muted">No indexed files found.</div>
          ) : (
            files.map((file) => (
              <div key={file.id} className="p-3">
                <div className="flex items-start gap-3">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center justify-between gap-2">
                      <p className="truncate text-sm font-medium text-text">
                        {file.original_filename || (file.attachment_id ? attachmentNameByID.get(file.attachment_id) : '') || file.display_name}
                      </p>
                      <span className="text-[10px] uppercase tracking-[0.08em] text-text-muted">{file.scope}</span>
                    </div>
                    <p className="mt-0.5 text-xs text-text-muted">
                      {file.display_name !== (file.original_filename || '') && file.display_name ? `${file.display_name} • ` : ''}
                      {file.status} • {formatKB(file.size_bytes)}
                    </p>
                  </div>
                  <div className="flex items-center gap-1">
                    <button
                      onClick={() => handleReindex(file.id)}
                      disabled={runningAction === `reindex:${file.id}`}
                      className="p-1.5 rounded-lg border border-border text-text-muted hover:text-text"
                      title="Re-index file"
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
            ))
          )}
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <button
            onClick={handleRevalidateProject}
            disabled={!selectedProjectId || files.length === 0 || runningAction === 'revalidate_project'}
            className="h-9 rounded-lg border border-border bg-surface-alt px-3 text-sm text-text inline-flex items-center gap-2 disabled:opacity-50"
          >
            <CheckCircle2 size={14} />
            {runningAction === 'revalidate_project' ? 'Re-validating...' : 'Re-Validate Indexing For Project'}
          </button>
        </div>
      </div>
    </DialogShell>
  );
}
