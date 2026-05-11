import { useState, useEffect, useCallback } from 'react';
import { createPortal } from 'react-dom';
import { workspaceApi, fileLibraryApi } from '../api';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import { Plus, Check, Trash2, X, Settings2, Palette, Files, Users } from 'lucide-react';
import type { Workspace, LibraryFile } from '../types';
import { WorkspaceMembersPanel } from './WorkspaceMembersPanel';

interface WorkspaceSwitcherProps {
  activeWorkspaceId: string | null;
  onSelectWorkspace: (id: string | null) => void;
}

export function WorkspaceSwitcher({ activeWorkspaceId, onSelectWorkspace }: WorkspaceSwitcherProps) {
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [workspaceFileCounts, setWorkspaceFileCounts] = useState<Record<string, number>>({});

  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState('');
  const [newDescription, setNewDescription] = useState('');
  const [newColor, setNewColor] = useState('#6366f1');
  const [newIcon, setNewIcon] = useState('folder');
  const [newMemoryMode, setNewMemoryMode] = useState<'default' | 'project_only'>('default');

  const [settingsWorkspaceId, setSettingsWorkspaceId] = useState<string | null>(null);
  const [settingsTab, setSettingsTab] = useState<'general' | 'ai' | 'files'>('general');
  const [settingsFiles, setSettingsFiles] = useState<LibraryFile[]>([]);
  const [revalidating, setRevalidating] = useState(false);
  const [membersWorkspaceId, setMembersWorkspaceId] = useState<string | null>(null);
  const [settingsDraft, setSettingsDraft] = useState({
    name: '',
    description: '',
    color: '#6366f1',
    icon: 'folder',
    project_instructions: '',
    memory_mode: 'default' as 'default' | 'project_only',
  });

  const fetchWorkspaces = useCallback(async () => {
    try {
      const data = await workspaceApi.list();
      setWorkspaces(data || []);
    } catch {
      // silent
    }
  }, []);

  const fetchWorkspaceFileCounts = useCallback(async () => {
    try {
      const files = await fileLibraryApi.list('workspace');
      const counts: Record<string, number> = {};
      for (const f of files) {
        if (!f.workspace_id) continue;
        counts[f.workspace_id] = (counts[f.workspace_id] || 0) + 1;
      }
      setWorkspaceFileCounts(counts);
    } catch {
      // silent
    }
  }, []);

  useEffect(() => {
    fetchWorkspaces();
    fetchWorkspaceFileCounts();
  }, [fetchWorkspaces, fetchWorkspaceFileCounts]);

  const iconLabel = (icon: string) => {
    switch (icon) {
      case 'briefcase':
        return '💼';
      case 'flask':
        return '🧪';
      case 'book':
        return '📚';
      case 'pen':
        return '✍️';
      case 'rocket':
        return '🚀';
      case 'calendar':
        return '📅';
      case 'folder':
      default:
        return '📁';
    }
  };

  const handleCreate = async () => {
    if (!newName.trim()) return;
    try {
      const ws = await workspaceApi.create({
        name: newName.trim(),
        description: newDescription.trim(),
        color: newColor,
        icon: newIcon,
        memory_mode: newMemoryMode,
      });
      setWorkspaces((prev) => [...prev, ws]);
      setNewName('');
      setNewDescription('');
      setNewColor('#6366f1');
      setNewIcon('folder');
      setNewMemoryMode('default');
      setCreating(false);
      onSelectWorkspace(ws.id);
      toast.success('Project created');
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  const handleDelete = (id: string, name: string) => {
    toast(`Delete project "${name}"?`, {
      action: {
        label: 'Delete',
        onClick: async () => {
          try {
            await workspaceApi.delete(id);
            setWorkspaces((prev) => prev.filter((w) => w.id !== id));
            setWorkspaceFileCounts((prev) => {
              const copy = { ...prev };
              delete copy[id];
              return copy;
            });
            if (activeWorkspaceId === id) onSelectWorkspace(null);
            toast.success('Project deleted');
          } catch (err) {
            toast.error((err as Error).message);
          }
        },
      },
      cancel: { label: 'Cancel', onClick: () => {} },
      duration: 6000,
    });
  };

  const loadSettingsFiles = useCallback(async (wsId: string) => {
    try {
      const files = await fileLibraryApi.list('workspace');
      setSettingsFiles(files.filter((f) => f.workspace_id === wsId));
    } catch {
      // silent
    }
  }, []);

  const openSettings = (ws: Workspace) => {
    setSettingsWorkspaceId(ws.id);
    setSettingsTab('general');
    setSettingsDraft({
      name: ws.name || '',
      description: ws.description || '',
      color: ws.color || '#6366f1',
      icon: ws.icon || 'folder',
      project_instructions: ws.project_instructions || '',
      memory_mode: ws.memory_mode || 'default',
    });
    loadSettingsFiles(ws.id);
  };

  const handleRevalidateSettings = async () => {
    if (!settingsWorkspaceId) return;
    setRevalidating(true);
    const unindexed = settingsFiles.filter((f) => f.status !== 'indexed');
    try {
      for (const f of unindexed) {
        await fileLibraryApi.reindex(f.id);
      }
      await loadSettingsFiles(settingsWorkspaceId);
      toast.success('Re-indexing complete');
    } catch (err) {
      toast.error((err as Error).message);
    } finally {
      setRevalidating(false);
    }
  };

  const saveSettings = async () => {
    if (!settingsWorkspaceId) return;
    try {
      const updated = await workspaceApi.update(settingsWorkspaceId, settingsDraft);
      setWorkspaces((prev) => prev.map((w) => (w.id === settingsWorkspaceId ? updated : w)));
      toast.success('Project settings saved');
      setSettingsWorkspaceId(null);
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  return (
    <div className="rounded-xl border border-border bg-surface/60">
      <div className="px-3 py-2 border-b border-border/70 flex items-center justify-between">
        <p className="text-[11px] font-semibold uppercase tracking-wider text-text-muted">Projects</p>
        <button
          onClick={() => setCreating((v) => !v)}
          className="inline-flex items-center gap-1 text-[11px] text-text-muted hover:text-text"
        >
          <Plus size={12} /> New
        </button>
      </div>

      {creating && (
        <div className="px-3 py-2 space-y-2 border-b border-border/70">
          <input
            type="text"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            placeholder="Project name"
            className="w-full px-2 py-1 rounded bg-surface-light border border-border text-xs text-text focus:outline-none"
            autoFocus
            onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
          />
          <input
            type="text"
            value={newDescription}
            onChange={(e) => setNewDescription(e.target.value)}
            placeholder="Description (optional)"
            className="w-full px-2 py-1 rounded bg-surface-light border border-border text-xs text-text focus:outline-none"
          />
          <div className="flex items-center gap-2">
            <label className="inline-flex items-center gap-1 text-[11px] text-text-muted">
              <Palette size={11} />
              <input
                type="color"
                value={newColor}
                onChange={(e) => setNewColor(e.target.value)}
                className="h-6 w-8 rounded border border-border bg-surface"
              />
            </label>
            <select
              value={newIcon}
              onChange={(e) => setNewIcon(e.target.value)}
              className="flex-1 px-2 py-1 rounded bg-surface-light border border-border text-xs text-text focus:outline-none"
            >
              <option value="folder">Folder</option>
              <option value="briefcase">Briefcase</option>
              <option value="flask">Flask</option>
              <option value="book">Book</option>
              <option value="pen">Pen</option>
              <option value="rocket">Rocket</option>
              <option value="calendar">Calendar</option>
            </select>
          </div>
          <select
            value={newMemoryMode}
            onChange={(e) => setNewMemoryMode(e.target.value as 'default' | 'project_only')}
            className="w-full px-2 py-1 rounded bg-surface-light border border-border text-xs text-text focus:outline-none"
          >
            <option value="default">Default project memory</option>
            <option value="project_only">Project-only memory</option>
          </select>
          <div className="flex items-center justify-end gap-2">
            <button onClick={handleCreate} className="text-emerald-400"><Check size={12} /></button>
            <button onClick={() => setCreating(false)} className="text-text-muted"><X size={12} /></button>
          </div>
        </div>
      )}

      <div className="max-h-72 overflow-y-auto p-1.5">
        {workspaces.map((ws) => {
          const isActive = activeWorkspaceId === ws.id;
          const fileCount = workspaceFileCounts[ws.id] || 0;
          return (
            <div
              key={ws.id}
              className={isActive ? 'group flex items-center gap-2 rounded-lg px-2 py-1.5 bg-primary/10 border border-primary/20 mb-1' : 'group flex items-center gap-2 rounded-lg px-2 py-1.5 hover:bg-surface-hover border border-transparent mb-1'}
            >
              <button
                onClick={() => onSelectWorkspace(ws.id)}
                className="flex-1 min-w-0 text-left flex items-center gap-2"
              >
                <span className="inline-flex items-center justify-center w-5 h-5 text-[11px] rounded" style={{ backgroundColor: `${ws.color || '#6366f1'}22` }}>
                  {iconLabel(ws.icon || 'folder')}
                </span>
                <span className={isActive ? 'text-sm text-text truncate' : 'text-sm text-text-secondary truncate'}>{ws.name}</span>
              </button>
              <span className="inline-flex items-center gap-1 text-[10px] text-text-muted shrink-0">
                <Files size={10} /> {fileCount}
              </span>
              <button
                onClick={() => openSettings(ws)}
                className="p-1 text-text-muted hover:text-primary opacity-0 group-hover:opacity-100"
                title="Project settings"
              >
                <Settings2 size={11} />
              </button>
              <button
                onClick={() => setMembersWorkspaceId(ws.id)}
                className="p-1 text-text-muted hover:text-primary opacity-0 group-hover:opacity-100"
                title="Manage members"
              >
                <Users size={11} />
              </button>
              <button
                onClick={() => handleDelete(ws.id, ws.name)}
                className="p-1 text-text-muted hover:text-red-400 opacity-0 group-hover:opacity-100"
                title="Delete project"
              >
                <Trash2 size={11} />
              </button>
            </div>
          );
        })}

        {workspaces.length === 0 && (
          <p className="px-2 py-2 text-xs text-text-muted">No projects yet</p>
        )}
      </div>

      {createPortal(
        <AnimatePresence>
          {settingsWorkspaceId && (
            <motion.div
              className="fixed inset-0 z-[9999] bg-black/55 flex items-center justify-center p-4"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              onClick={() => setSettingsWorkspaceId(null)}
            >
              <motion.div
                className="w-full max-w-xl rounded-2xl glass-strong border border-border flex flex-col"
                style={{ maxHeight: '85vh' }}
                initial={{ scale: 0.98, opacity: 0 }}
                animate={{ scale: 1, opacity: 1 }}
                exit={{ scale: 0.98, opacity: 0 }}
                onClick={(e) => e.stopPropagation()}
              >
              {/* Header */}
              <div className="flex items-center justify-between px-5 py-4 border-b border-border shrink-0">
                <h3 className="text-sm font-semibold text-text">Project Settings</h3>
                <button onClick={() => setSettingsWorkspaceId(null)} className="text-text-muted hover:text-text">
                  <X size={14} />
                </button>
              </div>

              {/* Tabs */}
              <div className="flex border-b border-border px-5 shrink-0">
                {(['general', 'ai', 'files'] as const).map((tab) => (
                  <button
                    key={tab}
                    onClick={() => setSettingsTab(tab)}
                    className={`px-3 py-2.5 text-xs font-medium border-b-2 -mb-px transition-colors ${
                      settingsTab === tab
                        ? 'border-primary text-primary'
                        : 'border-transparent text-text-muted hover:text-text'
                    }`}
                  >
                    {tab === 'general' ? 'General' : tab === 'ai' ? 'AI Behavior' : 'Files & Indexing'}
                  </button>
                ))}
              </div>

              {/* Tab content */}
              <div className="flex-1 overflow-y-auto p-5 space-y-4 min-h-0">
                {settingsTab === 'general' && (
                  <>
                    <input
                      type="text"
                      value={settingsDraft.name}
                      onChange={(e) => setSettingsDraft((prev) => ({ ...prev, name: e.target.value }))}
                      placeholder="Project name"
                      className="w-full px-3 py-2 rounded-xl bg-surface-alt border border-border text-sm text-text focus:outline-none"
                    />
                    <input
                      type="text"
                      value={settingsDraft.description}
                      onChange={(e) => setSettingsDraft((prev) => ({ ...prev, description: e.target.value }))}
                      placeholder="Project description"
                      className="w-full px-3 py-2 rounded-xl bg-surface-alt border border-border text-sm text-text focus:outline-none"
                    />
                    <div className="grid grid-cols-2 gap-3">
                      <label className="text-xs text-text-muted">
                        Color
                        <input
                          type="color"
                          value={settingsDraft.color}
                          onChange={(e) => setSettingsDraft((prev) => ({ ...prev, color: e.target.value }))}
                          className="mt-1 h-9 w-full rounded-xl border border-border bg-surface cursor-pointer"
                        />
                      </label>
                      <label className="text-xs text-text-muted">
                        Icon
                        <select
                          value={settingsDraft.icon}
                          onChange={(e) => setSettingsDraft((prev) => ({ ...prev, icon: e.target.value }))}
                          className="mt-1 w-full px-3 py-2 rounded-xl bg-surface-alt border border-border text-sm text-text focus:outline-none"
                        >
                          <option value="folder">📁 Folder</option>
                          <option value="briefcase">💼 Briefcase</option>
                          <option value="flask">🧪 Flask</option>
                          <option value="book">📚 Book</option>
                          <option value="pen">✍️ Pen</option>
                          <option value="rocket">🚀 Rocket</option>
                          <option value="calendar">📅 Calendar</option>
                        </select>
                      </label>
                    </div>
                  </>
                )}

                {settingsTab === 'ai' && (
                  <>
                    <label className="block text-xs text-text-muted">
                      Memory mode
                      <select
                        value={settingsDraft.memory_mode}
                        onChange={(e) => setSettingsDraft((prev) => ({ ...prev, memory_mode: e.target.value as 'default' | 'project_only' }))}
                        className="mt-1 w-full px-3 py-2 rounded-xl bg-surface-alt border border-border text-sm text-text focus:outline-none"
                      >
                        <option value="default">Default — include all conversation history</option>
                        <option value="project_only">Project-only — restrict to project files &amp; instructions</option>
                      </select>
                    </label>
                    <label className="block text-xs text-text-muted">
                      Project instructions
                      <p className="mt-0.5 mb-1.5 text-[11px] text-text-muted/70">Tell the AI how to behave in all chats within this project.</p>
                      <textarea
                        rows={9}
                        value={settingsDraft.project_instructions}
                        onChange={(e) => setSettingsDraft((prev) => ({ ...prev, project_instructions: e.target.value }))}
                        placeholder="Act like my marketing mentor, ask clarifying questions, use bullet points..."
                        className="mt-1 w-full px-3 py-2 rounded-xl bg-surface-alt border border-border text-sm text-text resize-y focus:outline-none"
                      />
                    </label>
                  </>
                )}

                {settingsTab === 'files' && (
                  <>
                    <div className="flex items-center justify-between">
                      <div>
                        <p className="text-sm font-medium text-text">
                          {settingsFiles.length} file{settingsFiles.length !== 1 ? 's' : ''} in this project
                        </p>
                        <p className="text-xs text-text-muted mt-0.5">
                          {settingsFiles.filter((f) => f.status === 'indexed').length} indexed ·{' '}
                          {settingsFiles.filter((f) => f.status !== 'indexed').length} not indexed
                        </p>
                      </div>
                      <button
                        onClick={handleRevalidateSettings}
                        disabled={revalidating}
                        className="px-3 py-1.5 rounded-xl text-xs font-medium text-white bg-primary hover:bg-primary-hover disabled:opacity-50 transition-opacity"
                      >
                        {revalidating ? 'Re-indexing…' : 'Re-Validate Indexing'}
                      </button>
                    </div>
                    <div className="space-y-1.5">
                      {settingsFiles.length === 0 && (
                        <p className="text-xs text-text-muted py-6 text-center">
                          No files attached to this project yet.<br />
                          Upload files in a project chat to add them here.
                        </p>
                      )}
                      {settingsFiles.map((f) => (
                        <div key={f.id} className="flex items-center gap-2 px-3 py-2 rounded-xl bg-surface-alt border border-border">
                          <span className="flex-1 min-w-0 text-xs text-text truncate">
                            {f.display_name || f.original_filename || f.id}
                          </span>
                          <span
                            className={`shrink-0 text-[10px] px-1.5 py-0.5 rounded-full font-medium ${
                              f.status === 'indexed'
                                ? 'bg-emerald-500/15 text-emerald-400'
                                : 'bg-amber-500/15 text-amber-400'
                            }`}
                          >
                            {f.status === 'indexed' ? 'Indexed' : 'Not indexed'}
                          </span>
                        </div>
                      ))}
                    </div>
                  </>
                )}
              </div>

              {/* Footer */}
              <div className="flex items-center justify-between px-5 py-4 border-t border-border shrink-0">
                <button
                  onClick={() => {
                    const wsId = settingsWorkspaceId;
                    setSettingsWorkspaceId(null);
                    const ws = workspaces.find((w) => w.id === wsId);
                    if (ws) handleDelete(ws.id, ws.name);
                  }}
                  className="px-3 py-2 rounded-xl text-xs text-red-400 hover:bg-red-500/10 transition-colors"
                >
                  Delete project
                </button>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setSettingsWorkspaceId(null)}
                    className="px-3 py-2 rounded-xl text-xs text-text-muted hover:text-text hover:bg-surface-hover"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={saveSettings}
                    className="px-3 py-2 rounded-xl text-xs font-semibold text-white bg-primary hover:bg-primary-hover"
                  >
                    Save settings
                  </button>
                </div>
              </div>
            </motion.div>
          </motion.div>
          )}
        </AnimatePresence>,
        document.body
      )}

      {membersWorkspaceId && createPortal(
        <WorkspaceMembersPanel
          workspaceId={membersWorkspaceId}
          open={!!membersWorkspaceId}
          onClose={() => setMembersWorkspaceId(null)}
        />,
        document.body
      )}
    </div>
  );
}
