import { useState, useEffect, useCallback } from 'react';
import { workspaceMemberApi } from '../api';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import { Users, UserPlus, Shield, Eye, X, Trash2, ChevronDown } from 'lucide-react';
import type { WorkspaceMember } from '../types';

interface WorkspaceMembersPanelProps {
  workspaceId: string;
  open: boolean;
  onClose: () => void;
}

const ROLE_LABELS: Record<string, { label: string; color: string }> = {
  owner: { label: 'Owner', color: 'text-yellow-400' },
  admin: { label: 'Admin', color: 'text-purple-400' },
  member: { label: 'Member', color: 'text-blue-400' },
  viewer: { label: 'Viewer', color: 'text-text-muted' },
};

const ASSIGNABLE_ROLES = ['admin', 'member', 'viewer'] as const;

export function WorkspaceMembersPanel({ workspaceId, open, onClose }: WorkspaceMembersPanelProps) {
  const [members, setMembers] = useState<WorkspaceMember[]>([]);
  const [loading, setLoading] = useState(false);
  const [addUserId, setAddUserId] = useState('');
  const [addRole, setAddRole] = useState<string>('member');
  const [adding, setAdding] = useState(false);
  const [editingUserId, setEditingUserId] = useState<string | null>(null);

  const fetchMembers = useCallback(async () => {
    setLoading(true);
    try {
      const data = await workspaceMemberApi.list(workspaceId);
      setMembers(data || []);
    } catch {
      // silent — may 404 if workspace has no members yet
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    if (open) fetchMembers();
  }, [open, fetchMembers]);

  const handleAdd = async () => {
    if (!addUserId.trim()) return;
    setAdding(true);
    try {
      const member = await workspaceMemberApi.add(workspaceId, {
        user_id: addUserId.trim(),
        role: addRole,
      });
      setMembers((prev) => [...prev, member]);
      setAddUserId('');
      toast.success('Member added');
    } catch (err) {
      toast.error((err as Error).message);
    } finally {
      setAdding(false);
    }
  };

  const handleUpdateRole = async (userId: string, role: string) => {
    try {
      await workspaceMemberApi.updateRole(workspaceId, userId, { role });
      setMembers((prev) =>
        prev.map((m) => (m.user_id === userId ? { ...m, role: role as WorkspaceMember['role'] } : m))
      );
      toast.success('Role updated');
    } catch (err) {
      toast.error((err as Error).message);
    } finally {
      setEditingUserId(null);
    }
  };

  const handleRemove = async (userId: string) => {
    try {
      await workspaceMemberApi.remove(workspaceId, userId);
      setMembers((prev) => prev.filter((m) => m.user_id !== userId));
      toast.success('Member removed');
    } catch (err) {
      toast.error((err as Error).message);
    }
  };

  if (!open) return null;

  return (
    <AnimatePresence>
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center"
        onClick={(e) => e.target === e.currentTarget && onClose()}
      >
        <motion.div
          initial={{ opacity: 0, scale: 0.95 }}
          animate={{ opacity: 1, scale: 1 }}
          exit={{ opacity: 0, scale: 0.95 }}
          className="glass-strong rounded-2xl shadow-2xl w-full max-w-md p-6 mx-4"
        >
          {/* Header */}
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <Users size={18} className="text-primary" />
              <h3 className="text-lg font-semibold text-text">Workspace Members</h3>
            </div>
            <button onClick={onClose} className="p-1 text-text-muted hover:text-text transition-colors">
              <X size={18} />
            </button>
          </div>

          {/* Add member form */}
          <div className="flex items-center gap-2 mb-4">
            <input
              type="text"
              value={addUserId}
              onChange={(e) => setAddUserId(e.target.value)}
              placeholder="User ID"
              className="flex-1 px-3 py-1.5 rounded-lg bg-surface-light border border-border text-sm text-text placeholder:text-text-muted focus:outline-none focus:ring-1 focus:ring-primary"
              onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
            />
            <select
              value={addRole}
              onChange={(e) => setAddRole(e.target.value)}
              className="px-2 py-1.5 rounded-lg bg-surface-light border border-border text-sm text-text focus:outline-none"
            >
              {ASSIGNABLE_ROLES.map((r) => (
                <option key={r} value={r}>
                  {ROLE_LABELS[r].label}
                </option>
              ))}
            </select>
            <button
              onClick={handleAdd}
              disabled={adding || !addUserId.trim()}
              className="flex items-center gap-1 px-3 py-1.5 rounded-lg bg-primary/20 text-primary text-sm hover:bg-primary/30 disabled:opacity-40 transition-colors"
            >
              <UserPlus size={14} />
              Add
            </button>
          </div>

          {/* Members list */}
          <div className="space-y-1 max-h-64 overflow-y-auto">
            {loading && <p className="text-xs text-text-muted py-2 text-center">Loading…</p>}
            {!loading && members.length === 0 && (
              <p className="text-xs text-text-muted py-4 text-center">No members yet</p>
            )}
            {members.map((m) => {
              const roleInfo = ROLE_LABELS[m.role] || ROLE_LABELS.member;
              const isOwner = m.role === 'owner';
              return (
                <div
                  key={m.user_id}
                  className="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-surface-light/50 transition-colors group"
                >
                  {/* Avatar placeholder */}
                  <div className="w-7 h-7 rounded-full bg-surface-light flex items-center justify-center text-xs font-medium text-text-muted shrink-0">
                    {(m.display_name || m.username || m.user_id).charAt(0).toUpperCase()}
                  </div>

                  {/* Name / ID */}
                  <div className="flex-1 min-w-0">
                    <p className="text-sm text-text truncate">
                      {m.display_name || m.username || m.user_id}
                    </p>
                    {(m.display_name || m.username) && (
                      <p className="text-xs text-text-muted truncate">{m.user_id}</p>
                    )}
                  </div>

                  {/* Role badge / editor */}
                  {editingUserId === m.user_id ? (
                    <div className="relative">
                      <select
                        defaultValue={m.role}
                        onChange={(e) => handleUpdateRole(m.user_id, e.target.value)}
                        onBlur={() => setEditingUserId(null)}
                        autoFocus
                        className="px-2 py-0.5 rounded bg-surface-light border border-border text-xs text-text focus:outline-none"
                      >
                        {ASSIGNABLE_ROLES.map((r) => (
                          <option key={r} value={r}>
                            {ROLE_LABELS[r].label}
                          </option>
                        ))}
                      </select>
                    </div>
                  ) : (
                    <button
                      onClick={() => !isOwner && setEditingUserId(m.user_id)}
                      disabled={isOwner}
                      className={`flex items-center gap-1 text-xs px-2 py-0.5 rounded ${roleInfo.color} ${
                        isOwner ? 'cursor-default' : 'hover:bg-surface-light/70 cursor-pointer'
                      }`}
                      title={isOwner ? 'Cannot change owner role' : 'Click to change role'}
                    >
                      {m.role === 'owner' || m.role === 'admin' ? (
                        <Shield size={10} />
                      ) : m.role === 'viewer' ? (
                        <Eye size={10} />
                      ) : null}
                      {roleInfo.label}
                      {!isOwner && <ChevronDown size={8} className="opacity-0 group-hover:opacity-100" />}
                    </button>
                  )}

                  {/* Remove button */}
                  {!isOwner && (
                    <button
                      onClick={() => handleRemove(m.user_id)}
                      className="p-1 text-text-muted hover:text-red-400 opacity-0 group-hover:opacity-100 transition-all"
                      title="Remove member"
                    >
                      <Trash2 size={12} />
                    </button>
                  )}
                </div>
              );
            })}
          </div>
        </motion.div>
      </motion.div>
    </AnimatePresence>
  );
}
