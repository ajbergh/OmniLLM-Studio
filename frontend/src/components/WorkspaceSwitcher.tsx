import { useState, useEffect, useCallback } from 'react';
import { workspaceApi } from '../api';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import { FolderOpen, Plus, Check, ChevronDown, Trash2, X, Pencil, Users } from 'lucide-react';
import type { Workspace, WorkspaceStats } from '../types';
import { WorkspaceMembersPanel } from './WorkspaceMembersPanel';

interface WorkspaceSwitcherProps {
  activeWorkspaceId: string | null;
  onSelectWorkspace: (id: string | null) => void;
}

export function WorkspaceSwitcher({ activeWorkspaceId, onSelectWorkspace }: WorkspaceSwitcherProps) {
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [open, setOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState('');
  const [stats, setStats] = useState<Record<string, WorkspaceStats>>({});
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState('');
  const [membersWorkspaceId, setMembersWorkspaceId] = useState<string | null>(null);

  const fetchWorkspaces = useCallback(async () => {
    try {
      const data = await workspaceApi.list();
      setWorkspaces(data || []);
    } catch {
      // silent
    }
  }, []);

  useEffect(() => {
    fetchWorkspaces();
  }, [fetchWorkspaces]);

  const handleCreate = async () => {
    if (!newName.trim()) return;
    try {
      const ws = await workspaceApi.create({ name: newName });
      setWorkspaces((prev) => [...prev, ws]);
      setNewName('');
      setCreating(false);
      onSelectWorkspace(ws.id);
      toast.success('Workspace created');
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  const handleDelete = (id: string, name: string) => {
    toast(`Delete workspace "${name}"?`, {
      action: {
        label: 'Delete',
        onClick: async () => {
          try {
            await workspaceApi.delete(id);
            setWorkspaces((prev) => prev.filter((w) => w.id !== id));
            if (activeWorkspaceId === id) onSelectWorkspace(null);
            toast.success('Workspace deleted');
          } catch (err) {
            toast.error((err as Error).message);
          }
        },
      },
      cancel: { label: 'Cancel', onClick: () => {} },
      duration: 6000,
    });
  };

  const handleRename = async (id: string) => {
    if (!editName.trim()) { setEditingId(null); return; }
    try {
      await workspaceApi.update(id, { name: editName.trim() });
      setWorkspaces((prev) => prev.map((w) => w.id === id ? { ...w, name: editName.trim() } : w));
      toast.success('Workspace renamed');
    } catch (err) {
      toast.error((err as Error).message);
    } finally {
      setEditingId(null);
    }
  };

  const loadStats = async (id: string) => {
    try {
      const s = await workspaceApi.getStats(id);
      setStats((prev) => ({ ...prev, [id]: s }));
    } catch {
      // silent
    }
  };

  const activeWorkspace = workspaces.find((w) => w.id === activeWorkspaceId);

  return (
    <div className="relative">
      <motion.button
        whileHover={{ scale: 1.02 }}
        whileTap={{ scale: 0.98 }}
        onClick={() => setOpen(!open)}
        className="flex items-center gap-2 px-3 py-1.5 rounded-xl glass text-sm text-text hover:bg-surface-light/50 transition-colors w-full"
      >
        <FolderOpen size={14} className="text-primary shrink-0" />
        <span className="truncate">{activeWorkspace?.name || 'All Conversations'}</span>
        <ChevronDown size={12} className="text-text-muted ml-auto shrink-0" />
      </motion.button>

      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, y: 4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 4 }}
            className="absolute top-full mt-1 left-0 right-0 glass-strong rounded-xl shadow-xl overflow-hidden z-50"
          >
            {/* All conversations */}
            <button
              onClick={() => { onSelectWorkspace(null); setOpen(false); }}
              className={`w-full text-left px-3 py-2 text-sm flex items-center gap-2 hover:bg-surface-light/50 transition-colors ${
                !activeWorkspaceId ? 'text-primary' : 'text-text'
              }`}
            >
              <FolderOpen size={12} /> All Conversations
              {!activeWorkspaceId && <Check size={12} className="ml-auto" />}
            </button>

            {/* Workspace list */}
            {workspaces.map((ws) => (
              <div key={ws.id} className="flex items-center group hover:bg-surface-light/50 transition-colors">
                {editingId === ws.id ? (
                  <div className="flex-1 flex items-center gap-1 px-3 py-1.5">
                    <input
                      type="text"
                      value={editName}
                      onChange={(e) => setEditName(e.target.value)}
                      className="flex-1 px-2 py-0.5 rounded bg-surface-light border border-border text-xs text-text focus:outline-none"
                      autoFocus
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') handleRename(ws.id);
                        if (e.key === 'Escape') setEditingId(null);
                      }}
                      onBlur={() => handleRename(ws.id)}
                    />
                  </div>
                ) : (
                  <button
                    onClick={() => { onSelectWorkspace(ws.id); setOpen(false); }}
                    onMouseEnter={() => loadStats(ws.id)}
                    className={`flex-1 text-left px-3 py-2 text-sm flex items-center gap-2 ${
                      activeWorkspaceId === ws.id ? 'text-primary' : 'text-text'
                    }`}
                  >
                    <FolderOpen size={12} />
                    <span className="truncate">{ws.name}</span>
                    {activeWorkspaceId === ws.id && <Check size={12} className="ml-auto" />}
                  </button>
                )}
                {stats[ws.id] && !editingId && (
                  <span className="text-xs text-text-muted mr-2 shrink-0">
                    {stats[ws.id].conversation_count} chats
                  </span>
                )}
                {editingId !== ws.id && (
                  <>
                    <button
                      onClick={() => { setMembersWorkspaceId(ws.id); }}
                      className="p-1.5 text-text-muted hover:text-primary opacity-0 group-hover:opacity-100 transition-all"
                      title="Manage members"
                    >
                      <Users size={10} />
                    </button>
                    <button
                      onClick={() => { setEditingId(ws.id); setEditName(ws.name); }}
                      className="p-1.5 text-text-muted hover:text-primary opacity-0 group-hover:opacity-100 transition-all"
                    >
                      <Pencil size={10} />
                    </button>
                    <button
                      onClick={() => handleDelete(ws.id, ws.name)}
                      className="p-1.5 text-text-muted hover:text-red-400 opacity-0 group-hover:opacity-100 transition-all mr-1"
                    >
                      <Trash2 size={10} />
                    </button>
                  </>
                )}
              </div>
            ))}

            {/* Create new */}
            <div className="border-t border-border">
              {creating ? (
                <div className="flex items-center gap-1 px-3 py-2">
                  <input
                    type="text"
                    value={newName}
                    onChange={(e) => setNewName(e.target.value)}
                    placeholder="Workspace name"
                    className="flex-1 px-2 py-0.5 rounded bg-surface-light border border-border text-xs text-text focus:outline-none"
                    autoFocus
                    onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
                  />
                  <button onClick={handleCreate} className="text-emerald-400"><Check size={12} /></button>
                  <button onClick={() => setCreating(false)} className="text-text-muted"><X size={12} /></button>
                </div>
              ) : (
                <button
                  onClick={() => setCreating(true)}
                  className="w-full text-left px-3 py-2 text-xs text-text-muted hover:text-text flex items-center gap-1.5 hover:bg-surface-light/50 transition-colors"
                >
                  <Plus size={12} /> New workspace
                </button>
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Workspace Members Panel */}
      {membersWorkspaceId && (
        <WorkspaceMembersPanel
          workspaceId={membersWorkspaceId}
          open={!!membersWorkspaceId}
          onClose={() => setMembersWorkspaceId(null)}
        />
      )}
    </div>
  );
}
