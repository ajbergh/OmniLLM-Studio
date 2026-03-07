import { useState, useEffect, useCallback } from 'react';
import { branchApi } from '../api';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import { GitBranch, Plus, Trash2, Edit3, Check, X } from 'lucide-react';
import type { Branch } from '../types';

interface BranchSwitcherProps {
  conversationId: string;
  activeBranchId?: string | null;
  lastMessageId?: string;
  onSwitchBranch: (branchId: string | null) => void;
}

export function BranchSwitcher({ conversationId, activeBranchId, lastMessageId, onSwitchBranch }: BranchSwitcherProps) {
  const [branches, setBranches] = useState<Branch[]>([]);
  const [open, setOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState('');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState('');

  const fetchBranches = useCallback(async () => {
    try {
      const data = await branchApi.list(conversationId);
      setBranches(data || []);
    } catch {
      // silent
    }
  }, [conversationId]);

  useEffect(() => {
    fetchBranches();
  }, [fetchBranches]);

  const handleCreate = async () => {
    if (!newName.trim()) return;
    if (!lastMessageId) {
      toast.error('No messages to branch from. Send a message first.');
      return;
    }
    try {
      const branch = await branchApi.create(conversationId, { name: newName, fork_message_id: lastMessageId });
      setBranches((prev) => [...prev, branch]);
      setNewName('');
      setCreating(false);
      toast.success('Branch created');
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  const handleRename = async (branchId: string) => {
    if (!editName.trim()) return;
    try {
      await branchApi.rename(conversationId, branchId, editName);
      setBranches((prev) => prev.map((b) => (b.id === branchId ? { ...b, name: editName } : b)));
      setEditingId(null);
      toast.success('Branch renamed');
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  const handleDelete = async (branchId: string) => {
    try {
      await branchApi.delete(conversationId, branchId);
      setBranches((prev) => prev.filter((b) => b.id !== branchId));
      if (activeBranchId === branchId) onSwitchBranch(null);
      toast.success('Branch deleted');
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  if (branches.length === 0 && !open) {
    return (
      <motion.button
        whileHover={{ scale: 1.05 }}
        whileTap={{ scale: 0.95 }}
        onClick={() => { setOpen(true); setCreating(true); }}
        className="flex items-center gap-1.5 px-2 py-1 rounded-lg text-text-muted hover:text-text hover:bg-surface-light transition-colors text-xs"
        title="Create branch"
      >
        <GitBranch size={12} />
      </motion.button>
    );
  }

  return (
    <div className="relative">
      <motion.button
        whileHover={{ scale: 1.05 }}
        whileTap={{ scale: 0.95 }}
        onClick={() => setOpen(!open)}
        className="flex items-center gap-1.5 px-2 py-1 rounded-lg text-text-muted hover:text-text hover:bg-surface-light transition-colors text-xs"
      >
        <GitBranch size={12} />
        <span>{activeBranchId ? branches.find((b) => b.id === activeBranchId)?.name || 'Branch' : 'main'}</span>
        <span className="text-text-muted/50">({branches.length})</span>
      </motion.button>

      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, y: 4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 4 }}
            className="absolute top-full mt-1 left-0 w-56 glass-strong rounded-xl shadow-xl overflow-hidden z-50"
          >
            {/* Main branch */}
            <button
              onClick={() => { onSwitchBranch(null); setOpen(false); }}
              className={`w-full text-left px-3 py-2 text-sm flex items-center gap-2 hover:bg-surface-light/50 transition-colors ${
                !activeBranchId ? 'text-primary' : 'text-text'
              }`}
            >
              <GitBranch size={12} /> main
              {!activeBranchId && <Check size={12} className="ml-auto" />}
            </button>

            {/* Branches */}
            {branches.map((branch) => (
              <div key={branch.id} className="flex items-center hover:bg-surface-light/50 transition-colors">
                {editingId === branch.id ? (
                  <div className="flex-1 flex items-center gap-1 px-3 py-1.5">
                    <input
                      type="text"
                      value={editName}
                      onChange={(e) => setEditName(e.target.value)}
                      className="flex-1 px-2 py-0.5 rounded bg-surface-light border border-border text-xs text-text focus:outline-none"
                      autoFocus
                      onKeyDown={(e) => e.key === 'Enter' && handleRename(branch.id)}
                    />
                    <button onClick={() => handleRename(branch.id)} className="text-emerald-400"><Check size={12} /></button>
                    <button onClick={() => setEditingId(null)} className="text-text-muted"><X size={12} /></button>
                  </div>
                ) : (
                  <>
                    <button
                      onClick={() => { onSwitchBranch(branch.id); setOpen(false); }}
                      className={`flex-1 text-left px-3 py-2 text-sm flex items-center gap-2 ${
                        activeBranchId === branch.id ? 'text-primary' : 'text-text'
                      }`}
                    >
                      <GitBranch size={12} /> {branch.name}
                      {activeBranchId === branch.id && <Check size={12} className="ml-auto" />}
                    </button>
                    <button
                      onClick={() => { setEditingId(branch.id); setEditName(branch.name); }}
                      className="p-1.5 text-text-muted hover:text-text"
                    >
                      <Edit3 size={10} />
                    </button>
                    <button
                      onClick={() => handleDelete(branch.id)}
                      className="p-1.5 text-text-muted hover:text-red-400"
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
                    placeholder="Branch name"
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
                  <Plus size={12} /> New branch
                </button>
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
