import { useEffect, useState, useCallback, useMemo } from 'react';
import { useConversationStore, useSettingsStore, useMessageStore } from '../stores';
import { useImageEditorStore } from '../stores/imageEditor';
import {
  Plus,
  Search,
  Pin,
  Pencil,
  Trash2,
  Archive,
  ArchiveRestore,
  PanelLeftClose,
  PanelLeftOpen,
  MessageSquare,
  MoreHorizontal,
  Sparkles,
  LogOut,
  User,
  ImageIcon,
} from 'lucide-react';
import { AppIcon } from './AppIcon';
import { clsx } from 'clsx';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import type { Conversation, ImageSession } from '../types';
import { WorkspaceSwitcher } from './WorkspaceSwitcher';
import { api, authApi, setAuthToken, imageSessionApi } from '../api';

function groupConversationsByDate(conversations: Conversation[]) {
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const yesterday = new Date(today.getTime() - 86400000);
  const weekAgo = new Date(today.getTime() - 7 * 86400000);
  const monthAgo = new Date(today.getTime() - 30 * 86400000);

  const groups: { label: string; conversations: Conversation[] }[] = [
    { label: 'Pinned', conversations: [] },
    { label: 'Today', conversations: [] },
    { label: 'Yesterday', conversations: [] },
    { label: 'This Week', conversations: [] },
    { label: 'This Month', conversations: [] },
    { label: 'Older', conversations: [] },
    { label: 'Archived', conversations: [] },
  ];

  for (const convo of conversations) {
    if (convo.archived) {
      groups[6].conversations.push(convo);
      continue;
    }
    if (convo.pinned) {
      groups[0].conversations.push(convo);
      continue;
    }
    const date = new Date(convo.updated_at || convo.created_at);
    if (date >= today) groups[1].conversations.push(convo);
    else if (date >= yesterday) groups[2].conversations.push(convo);
    else if (date >= weekAgo) groups[3].conversations.push(convo);
    else if (date >= monthAgo) groups[4].conversations.push(convo);
    else groups[5].conversations.push(convo);
  }

  return groups.filter((g) => g.conversations.length > 0);
}

function groupSessionsByDate(sessions: ImageSession[]) {
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const yesterday = new Date(today.getTime() - 86400000);
  const weekAgo = new Date(today.getTime() - 7 * 86400000);
  const monthAgo = new Date(today.getTime() - 30 * 86400000);

  const groups: { label: string; sessions: ImageSession[] }[] = [
    { label: 'Today', sessions: [] },
    { label: 'Yesterday', sessions: [] },
    { label: 'This Week', sessions: [] },
    { label: 'This Month', sessions: [] },
    { label: 'Older', sessions: [] },
  ];

  for (const session of sessions) {
    const date = new Date(session.updated_at || session.created_at);
    if (date >= today) groups[0].sessions.push(session);
    else if (date >= yesterday) groups[1].sessions.push(session);
    else if (date >= weekAgo) groups[2].sessions.push(session);
    else if (date >= monthAgo) groups[3].sessions.push(session);
    else groups[4].sessions.push(session);
  }

  return groups.filter((g) => g.sessions.length > 0);
}

export function Sidebar() {
  const {
    conversations,
    activeId,
    searchQuery,
    showArchived,
    fetchConversations,
    createConversation,
    selectConversation,
    updateConversation,
    deleteConversation,
    setSearchQuery,
    searchConversations,
    setShowArchived,
  } = useConversationStore();
  const { sidebarOpen, toggleSidebar, appMode, setAppMode } = useSettingsStore();
  const { fetchMessages, clearMessages } = useMessageStore();
  const {
    allSessions: imageSessions,
    activeSessionId,
    loadAllSessions,
    loadSession,
    createSession,
    deleteSession: deleteImageSession,
    renameSession,
  } = useImageEditorStore();

  const [editingId, setEditingId] = useState<string | null>(null);
  const [editTitle, setEditTitle] = useState('');
  const [contextMenuId, setContextMenuId] = useState<string | null>(null);
  const [searchFocused, setSearchFocused] = useState(false);
  const [activeWorkspaceId, setActiveWorkspaceId] = useState<string | null>(null);
  const [authEnabled, setAuthEnabled] = useState(false);
  const [currentUser, setCurrentUser] = useState<string | null>(null);

  useEffect(() => {
    fetchConversations();
  }, [fetchConversations]);

  // Load all image sessions when in image mode
  useEffect(() => {
    if (appMode === 'image') {
      loadAllSessions();
    }
  }, [appMode, loadAllSessions]);

  // Check if auth is enabled and get current user
  useEffect(() => {
    authApi.status().then((s) => {
      if (s.auth_enabled && s.has_users) {
        setAuthEnabled(true);
        authApi.me().then((u) => setCurrentUser(u.display_name || u.username)).catch(() => {});
      }
    }).catch(() => {});
  }, []);

  const handleLogout = async () => {
    try {
      await authApi.logout();
    } catch {
      // Server logout can fail if session already expired
    }
    setAuthToken(null);
    window.location.reload();
  };

  // Close context menu on outside click
  useEffect(() => {
    if (!contextMenuId) return;
    const handler = () => setContextMenuId(null);
    window.addEventListener('click', handler);
    return () => window.removeEventListener('click', handler);
  }, [contextMenuId]);

  const handleSelect = useCallback(
    (id: string) => {
      selectConversation(id);
      fetchMessages(id);
      setContextMenuId(null);
      // Auto-close sidebar on mobile
      if (window.innerWidth < 768 && sidebarOpen) {
        toggleSidebar();
      }
    },
    [selectConversation, fetchMessages, sidebarOpen, toggleSidebar]
  );

  const handleNew = async () => {
    const convo = await createConversation();
    clearMessages();
    selectConversation(convo.id);
    toast.success('New conversation created');
  };

  const handleRename = (id: string, currentTitle: string) => {
    setEditingId(id);
    setEditTitle(currentTitle);
    setContextMenuId(null);
  };

  const commitRename = async () => {
    if (editingId && editTitle.trim()) {
      await updateConversation(editingId, { title: editTitle.trim() });
    }
    setEditingId(null);
    setEditTitle('');
  };

  const handlePin = async (id: string, pinned: boolean) => {
    await updateConversation(id, { pinned: !pinned });
    toast.success(pinned ? 'Unpinned' : 'Pinned');
    setContextMenuId(null);
  };

  const handleArchive = async (id: string) => {
    await updateConversation(id, { archived: true });
    toast.success('Archived');
    setContextMenuId(null);
  };

  const handleUnarchive = async (id: string) => {
    await updateConversation(id, { archived: false });
    toast.success('Unarchived');
    setContextMenuId(null);
  };

  const handleDelete = (id: string) => {
    setContextMenuId(null);
    toast('Delete this conversation?', {
      action: {
        label: 'Delete',
        onClick: async () => {
          await deleteConversation(id);
          toast.success('Deleted');
        },
      },
      cancel: { label: 'Cancel', onClick: () => {} },
      duration: 5000,
    });
  };

  const handleSearch = (e: React.ChangeEvent<HTMLInputElement>) => {
    const q = e.target.value;
    setSearchQuery(q);
    searchConversations(q);
  };

  const groups = useMemo(() => groupConversationsByDate(conversations), [conversations]);
  const sessionGroups = useMemo(() => groupSessionsByDate(imageSessions), [imageSessions]);

  const handleNewSession = async () => {
    const now = new Date();
    const title = now.toLocaleString(undefined, {
      month: 'short', day: 'numeric', year: 'numeric',
      hour: 'numeric', minute: '2-digit',
    });
    const session = await createSession(title);
    if (session) {
      await loadAllSessions();
      await loadSession(session.conversation_id, session.id);
      toast.success('New image session created');
    }
  };

  const handleSelectSession = useCallback(
    async (session: ImageSession) => {
      await loadSession(session.conversation_id, session.id);
      setContextMenuId(null);
      if (window.innerWidth < 768 && sidebarOpen) {
        toggleSidebar();
      }
    },
    [loadSession, sidebarOpen, toggleSidebar]
  );

  const handleDeleteSession = (session: ImageSession) => {
    setContextMenuId(null);
    toast('Delete this session?', {
      action: {
        label: 'Delete',
        onClick: async () => {
          await deleteImageSession(session.conversation_id, session.id);
          // Clean up the backing conversation if no remaining sessions
          try {
            const remaining = await imageSessionApi.list(session.conversation_id);
            if (remaining.length === 0) {
              await api.deleteConversation(session.conversation_id);
            }
          } catch { /* ignore cleanup errors */ }
          await loadAllSessions();
          toast.success('Session deleted');
        },
      },
      cancel: { label: 'Cancel', onClick: () => {} },
      duration: 5000,
    });
  };

  const handleRenameSession = (id: string, currentTitle: string) => {
    setEditingId(id);
    setEditTitle(currentTitle);
    setContextMenuId(null);
  };

  const commitSessionRename = async () => {
    if (editingId && editTitle.trim()) {
      const session = imageSessions.find((s) => s.id === editingId);
      if (session) {
        await renameSession(session.conversation_id, session.id, editTitle.trim());
      }
    }
    setEditingId(null);
    setEditTitle('');
  };

  if (!sidebarOpen) {
    return (
      <motion.button
        initial={{ opacity: 0, x: -10 }}
        animate={{ opacity: 1, x: 0 }}
        onClick={toggleSidebar}
        className="fixed top-3 left-3 z-50 p-2.5 rounded-xl glass text-text-muted hover:text-text transition-colors"
        aria-label="Open sidebar"
      >
        <PanelLeftOpen size={18} />
      </motion.button>
    );
  }

  return (
    <aside className="w-72 h-full bg-surface-raised border-r border-border flex flex-col">
      {/* Header */}
      <div className="p-4 flex items-center justify-between">
        <div className="flex items-center gap-2.5">
          <AppIcon size={32} />
          <h1 className="text-sm font-bold gradient-text">OmniLLM-Studio</h1>
        </div>
        <div className="flex items-center gap-0.5">
          <motion.button
            whileHover={{ scale: 1.1 }}
            whileTap={{ scale: 0.9 }}
            onClick={appMode === 'image' ? handleNewSession : handleNew}
            className="p-2 rounded-xl hover:bg-surface-hover text-text-muted hover:text-primary transition-colors"
            aria-label={appMode === 'image' ? 'New session' : 'New conversation (Ctrl+N)'}
          >
            <Plus size={16} />
          </motion.button>
          <motion.button
            whileHover={{ scale: 1.1 }}
            whileTap={{ scale: 0.9 }}
            onClick={toggleSidebar}
            className="p-2 rounded-xl hover:bg-surface-hover text-text-muted hover:text-text transition-colors"
            aria-label="Close sidebar"
          >
            <PanelLeftClose size={16} />
          </motion.button>
        </div>
      </div>

      {/* Mode Switcher */}
      <div className="px-3 pb-2">
        <div className="flex rounded-xl bg-surface border border-border p-1">
          <button
            onClick={() => setAppMode('chat')}
            className={clsx(
              'flex-1 px-3 py-2 rounded-lg text-xs font-medium transition-all duration-200 flex items-center justify-center gap-2',
              appMode === 'chat'
                ? 'bg-primary/20 text-primary shadow-sm'
                : 'text-text-muted hover:text-text hover:bg-surface-hover'
            )}
          >
            <MessageSquare size={14} />
            Chat Studio
          </button>
          <button
            onClick={() => setAppMode('image')}
            className={clsx(
              'flex-1 px-3 py-2 rounded-lg text-xs font-medium transition-all duration-200 flex items-center justify-center gap-2',
              appMode === 'image'
                ? 'bg-primary/20 text-primary shadow-sm'
                : 'text-text-muted hover:text-text hover:bg-surface-hover'
            )}
          >
            <ImageIcon size={14} />
            Image Studio
          </button>
        </div>
      </div>

      {/* Workspace Switcher (chat mode only) */}
      {appMode === 'chat' && (
        <div className="px-3 pb-2">
          <WorkspaceSwitcher
            activeWorkspaceId={activeWorkspaceId}
            onSelectWorkspace={(id) => {
              setActiveWorkspaceId(id);
              fetchConversations(undefined, id);
            }}
          />
        </div>
      )}

      {/* Search (chat mode only) */}
      {appMode === 'chat' && (
        <div className="px-3 pb-3">
          <div className={clsx(
            'relative rounded-xl transition-all duration-300',
            searchFocused && 'ring-1 ring-primary/30 shadow-glow'
          )}>
            <Search size={14} className={clsx(
              'absolute left-3 top-1/2 -translate-y-1/2 transition-colors',
              searchFocused ? 'text-primary' : 'text-text-muted'
            )} />
            <input
              type="text"
              placeholder="Search conversations..."
              value={searchQuery}
              onChange={handleSearch}
              onFocus={() => setSearchFocused(true)}
              onBlur={() => setSearchFocused(false)}
              className="w-full pl-9 pr-3 py-2.5 text-sm bg-surface-alt border border-border rounded-xl
                         text-text placeholder-text-muted focus:outline-none
                         transition-all duration-300"
            />
          </div>
        </div>
      )}

      {/* List area */}
      <div className="flex-1 overflow-y-auto px-2 pb-2">
        {appMode === 'image' ? (
          // ── Image Sessions List ──
          imageSessions.length === 0 ? (
            <motion.div
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              className="text-center py-12 px-4"
            >
              <div className="w-12 h-12 rounded-2xl bg-gradient-to-br from-primary/20 to-accent/20 flex items-center justify-center mx-auto mb-4">
                <ImageIcon size={20} className="text-primary" />
              </div>
              <p className="text-text-muted text-sm mb-3">No image sessions yet</p>
              <button
                onClick={handleNewSession}
                className="btn-primary px-4 py-2 rounded-xl text-xs font-medium inline-flex items-center gap-1.5"
              >
                <Plus size={13} /> Start your first session
              </button>
            </motion.div>
          ) : (
            sessionGroups.map((group) => (
              <div key={group.label} className="mb-2">
                <div className="px-3 py-1.5 text-[10px] font-semibold uppercase tracking-widest text-text-muted/60">
                  {group.label}
                </div>
                {group.sessions.map((session, i) => (
                  <SessionItem
                    key={session.id}
                    session={session}
                    isActive={session.id === activeSessionId}
                    editingId={editingId}
                    editTitle={editTitle}
                    setEditTitle={setEditTitle}
                    commitRename={commitSessionRename}
                    setEditingId={setEditingId}
                    contextMenuId={contextMenuId}
                    setContextMenuId={setContextMenuId}
                    onSelect={handleSelectSession}
                    onRename={handleRenameSession}
                    onDelete={handleDeleteSession}
                    index={i}
                  />
                ))}
              </div>
            ))
          )
        ) : (
          // ── Conversation List ──
          conversations.length === 0 ? (
            <motion.div
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              className="text-center py-12 px-4"
            >
              <div className="w-12 h-12 rounded-2xl bg-gradient-to-br from-primary/20 to-accent/20 flex items-center justify-center mx-auto mb-4">
                <Sparkles size={20} className="text-primary" />
              </div>
              <p className="text-text-muted text-sm mb-3">No conversations yet</p>
              <button
                onClick={handleNew}
                className="btn-primary px-4 py-2 rounded-xl text-xs font-medium inline-flex items-center gap-1.5"
              >
                <Plus size={13} /> Start your first chat
              </button>
            </motion.div>
          ) : searchQuery ? (
            conversations.map((convo, i) => (
              <ConversationItem
                key={convo.id}
                convo={convo}
                isActive={convo.id === activeId}
                editingId={editingId}
                editTitle={editTitle}
                setEditTitle={setEditTitle}
                commitRename={commitRename}
                setEditingId={setEditingId}
                contextMenuId={contextMenuId}
                setContextMenuId={setContextMenuId}
                onSelect={handleSelect}
                onRename={handleRename}
                onPin={handlePin}
                onArchive={handleArchive}
                onUnarchive={handleUnarchive}
                onDelete={handleDelete}
                index={i}
              />
            ))
          ) : (
            groups.map((group) => (
              <div key={group.label} className="mb-2">
                <div className="px-3 py-1.5 text-[10px] font-semibold uppercase tracking-widest text-text-muted/60">
                  {group.label}
                </div>
                {group.conversations.map((convo, i) => (
                  <ConversationItem
                    key={convo.id}
                    convo={convo}
                    isActive={convo.id === activeId}
                    editingId={editingId}
                    editTitle={editTitle}
                    setEditTitle={setEditTitle}
                    commitRename={commitRename}
                    setEditingId={setEditingId}
                    contextMenuId={contextMenuId}
                    setContextMenuId={setContextMenuId}
                    onSelect={handleSelect}
                    onRename={handleRename}
                    onPin={handlePin}
                    onArchive={handleArchive}
                    onUnarchive={handleUnarchive}
                    onDelete={handleDelete}
                    index={i}
                  />
                ))}
              </div>
            ))
          )
        )}
      </div>

      {/* Footer */}
      <div className="p-3 border-t border-border">
        {appMode === 'chat' && (
          <button
            onClick={() => setShowArchived(!showArchived)}
            className={clsx(
              'w-full flex items-center gap-2 px-3 py-2 rounded-xl text-xs transition-colors mb-2',
              showArchived
                ? 'bg-primary/10 text-primary'
                : 'text-text-muted hover:bg-surface-hover hover:text-text'
            )}
          >
            <Archive size={13} />
            <span>{showArchived ? 'Hide Archived' : 'Show Archived'}</span>
          </button>
        )}

        {authEnabled && (
          <div className="flex items-center justify-between px-2 py-1.5 mb-1">
            <div className="flex items-center gap-2 text-[11px] text-text-muted min-w-0">
              <User size={12} className="shrink-0" />
              <span className="truncate">{currentUser || 'User'}</span>
            </div>
            <motion.button
              whileHover={{ scale: 1.1 }}
              whileTap={{ scale: 0.9 }}
              onClick={handleLogout}
              className="p-1.5 rounded-lg hover:bg-surface-hover text-text-muted hover:text-red-400 transition-colors"
              aria-label="Sign out"
              title="Sign out"
            >
              <LogOut size={13} />
            </motion.button>
          </div>
        )}

        <div className="flex items-center gap-2 px-2 py-1.5 text-[11px] text-text-muted">
          <div className="w-2 h-2 rounded-full bg-success animate-pulse" />
          <span>OmniLLM-Studio</span>
        </div>
      </div>
    </aside>
  );
}

function ConversationItem({
  convo,
  isActive,
  editingId,
  editTitle,
  setEditTitle,
  commitRename,
  setEditingId,
  contextMenuId,
  setContextMenuId,
  onSelect,
  onRename,
  onPin,
  onArchive,
  onUnarchive,
  onDelete,
  index,
}: {
  convo: Conversation;
  isActive: boolean;
  editingId: string | null;
  editTitle: string;
  setEditTitle: (t: string) => void;
  commitRename: () => void;
  setEditingId: (id: string | null) => void;
  contextMenuId: string | null;
  setContextMenuId: (id: string | null) => void;
  onSelect: (id: string) => void;
  onRename: (id: string, title: string) => void;
  onPin: (id: string, pinned: boolean) => void;
  onArchive: (id: string) => void;
  onUnarchive: (id: string) => void;
  onDelete: (id: string) => void;
  index: number;
}) {
  return (
    <motion.div
      initial={{ opacity: 0, x: -10 }}
      animate={{ opacity: 1, x: 0 }}
      transition={{ delay: index * 0.03, duration: 0.3 }}
      onClick={() => onSelect(convo.id)}
      className={clsx(
        'group flex items-center gap-2.5 px-3 py-2.5 rounded-xl cursor-pointer text-sm mb-0.5',
        'transition-all duration-200 relative',
        isActive
          ? 'bg-primary/10 text-text border border-primary/20'
          : 'text-text-secondary hover:bg-surface-hover hover:text-text border border-transparent'
      )}
    >
      {/* Active indicator */}
      {isActive && (
        <motion.div
          layoutId="active-indicator"
          className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-5 rounded-full bg-gradient-to-b from-primary to-accent"
          transition={{ type: 'spring', stiffness: 400, damping: 30 }}
        />
      )}

      <MessageSquare size={14} className={clsx(
        'shrink-0 transition-colors',
        isActive ? 'text-primary' : 'opacity-40'
      )} />

      {editingId === convo.id ? (
        <input
          autoFocus
          value={editTitle}
          onChange={(e) => setEditTitle(e.target.value)}
          onBlur={commitRename}
          onKeyDown={(e) => {
            if (e.key === 'Enter') commitRename();
            if (e.key === 'Escape') setEditingId(null);
          }}
          className="flex-1 bg-transparent border-b border-primary outline-none text-sm text-text"
          onClick={(e) => e.stopPropagation()}
        />
      ) : (
        <span className={clsx('flex-1 truncate', convo.archived && 'opacity-60')}>{convo.title}</span>
      )}

      {convo.archived && (
        <Archive size={11} className="shrink-0 text-text-muted opacity-50" />
      )}
      {convo.pinned && !convo.archived && (
        <Pin size={11} className="shrink-0 text-primary opacity-60" />
      )}

      {/* Context menu trigger */}
      <button
        onClick={(e) => {
          e.stopPropagation();
          setContextMenuId(contextMenuId === convo.id ? null : convo.id);
        }}
        className={clsx(
          'p-1 rounded-lg hover:bg-surface-alt text-text-muted transition-all',
          contextMenuId === convo.id ? 'opacity-100' : 'opacity-100 md:opacity-0 md:group-hover:opacity-100'
        )}
      >
        <MoreHorizontal size={14} />
      </button>

      {/* Context menu */}
      <AnimatePresence>
        {contextMenuId === convo.id && (
          <motion.div
            initial={{ opacity: 0, scale: 0.95, y: -4 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.95, y: -4 }}
            transition={{ duration: 0.15 }}
            className="absolute right-0 top-full mt-1 z-50 glass-strong rounded-xl shadow-lg py-1.5 min-w-[150px]"
            onClick={(e) => e.stopPropagation()}
          >
            <ContextMenuItem onClick={() => onRename(convo.id, convo.title)}>
              Rename
            </ContextMenuItem>
            <ContextMenuItem onClick={() => onPin(convo.id, convo.pinned)}>
              {convo.pinned ? 'Unpin' : 'Pin'}
            </ContextMenuItem>
            {convo.archived ? (
              <ContextMenuItem onClick={() => onUnarchive(convo.id)} icon={<ArchiveRestore size={12} />}>
                Unarchive
              </ContextMenuItem>
            ) : (
              <ContextMenuItem onClick={() => onArchive(convo.id)} icon={<Archive size={12} />}>
                Archive
              </ContextMenuItem>
            )}
            <div className="my-1 mx-2 border-t border-border" />
            <ContextMenuItem onClick={() => onDelete(convo.id)} icon={<Trash2 size={12} />} danger>
              Delete
            </ContextMenuItem>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  );
}

function SessionItem({
  session,
  isActive,
  editingId,
  editTitle,
  setEditTitle,
  commitRename,
  setEditingId,
  contextMenuId,
  setContextMenuId,
  onSelect,
  onRename,
  onDelete,
  index,
}: {
  session: ImageSession;
  isActive: boolean;
  editingId: string | null;
  editTitle: string;
  setEditTitle: (t: string) => void;
  commitRename: () => void;
  setEditingId: (id: string | null) => void;
  contextMenuId: string | null;
  setContextMenuId: (id: string | null) => void;
  onSelect: (session: ImageSession) => void;
  onRename: (id: string, title: string) => void;
  onDelete: (session: ImageSession) => void;
  index: number;
}) {
  return (
    <motion.div
      initial={{ opacity: 0, x: -10 }}
      animate={{ opacity: 1, x: 0 }}
      transition={{ delay: index * 0.03, duration: 0.3 }}
      onClick={() => onSelect(session)}
      className={clsx(
        'group flex items-center gap-2.5 px-3 py-2.5 rounded-xl cursor-pointer text-sm mb-0.5',
        'transition-all duration-200 relative',
        isActive
          ? 'bg-primary/10 text-text border border-primary/20'
          : 'text-text-secondary hover:bg-surface-hover hover:text-text border border-transparent'
      )}
    >
      {isActive && (
        <motion.div
          layoutId="active-session-indicator"
          className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-5 rounded-full bg-gradient-to-b from-primary to-accent"
          transition={{ type: 'spring', stiffness: 400, damping: 30 }}
        />
      )}

      <ImageIcon size={14} className={clsx(
        'shrink-0 transition-colors',
        isActive ? 'text-primary' : 'opacity-40'
      )} />

      {editingId === session.id ? (
        <input
          autoFocus
          value={editTitle}
          onChange={(e) => setEditTitle(e.target.value)}
          onBlur={commitRename}
          onKeyDown={(e) => {
            if (e.key === 'Enter') commitRename();
            if (e.key === 'Escape') setEditingId(null);
          }}
          className="flex-1 bg-transparent border-b border-primary outline-none text-sm text-text"
          onClick={(e) => e.stopPropagation()}
        />
      ) : (
        <span className="flex-1 truncate">{session.title}</span>
      )}

      <button
        onClick={(e) => {
          e.stopPropagation();
          setContextMenuId(contextMenuId === session.id ? null : session.id);
        }}
        className={clsx(
          'p-1 rounded-lg hover:bg-surface-alt text-text-muted transition-all',
          contextMenuId === session.id ? 'opacity-100' : 'opacity-100 md:opacity-0 md:group-hover:opacity-100'
        )}
      >
        <MoreHorizontal size={14} />
      </button>

      <AnimatePresence>
        {contextMenuId === session.id && (
          <motion.div
            initial={{ opacity: 0, scale: 0.95, y: -4 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.95, y: -4 }}
            transition={{ duration: 0.15 }}
            className="absolute right-0 top-full mt-1 z-50 min-w-[160px] py-1.5 bg-surface-raised border border-border rounded-xl shadow-lg"
            onClick={(e) => e.stopPropagation()}
          >
            <ContextMenuItem
              icon={<Pencil size={13} />}
              onClick={() => onRename(session.id, session.title)}
            >
              Rename
            </ContextMenuItem>
            <ContextMenuItem
              icon={<Trash2 size={13} />}
              onClick={() => onDelete(session)}
              danger
            >
              Delete
            </ContextMenuItem>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  );
}

function ContextMenuItem({
  children,
  onClick,
  icon,
  danger,
}: {
  children: React.ReactNode;
  onClick: () => void;
  icon?: React.ReactNode;
  danger?: boolean;
}) {
  return (
    <button
      onClick={onClick}
      className={clsx(
        'w-full text-left px-3 py-2 text-sm hover:bg-surface-hover transition-colors flex items-center gap-2 rounded-lg mx-0.5',
        danger ? 'text-danger hover:text-danger' : 'text-text-secondary hover:text-text'
      )}
      style={{ width: 'calc(100% - 4px)' }}
    >
      {icon}
      {children}
    </button>
  );
}
