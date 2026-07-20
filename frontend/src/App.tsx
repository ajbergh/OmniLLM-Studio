// App.tsx is the root component of the OmniLLM-Studio frontend.
// It manages the main layout, sidebar navigation, and conditional rendering
// of the various modals and UI panels (Chat, Settings, Image Studio, etc).

import { useEffect, useCallback, useState } from 'react';
import { Sidebar } from './components/Sidebar';
import { ChatView } from './components/ChatView';
import { ImageEditStudio } from './components/image/ImageEditStudio';
import { MusicStudio } from './components/music/MusicStudio';
import { VideoStudio } from './components/video/VideoStudio';
import { VideoEditStudioEnhanced } from './components/video/VideoEditStudioEnhanced';
import { SettingsPanel } from './components/SettingsPanel';
import { KeyboardShortcuts } from './components/KeyboardShortcuts';
import { LoginScreen } from './components/LoginScreen';
import { UsageDashboard } from './components/UsageDashboard';
import { TemplateManager } from './components/TemplateManager';
import { PluginManager } from './components/PluginManager';
import { EvalDashboard } from './components/EvalDashboard';
import { SearchPanel } from './components/SearchPanel';
import { ImportExportPanel } from './components/ImportExportPanel';
import { FileLibraryPanel } from './components/FileLibraryPanel';
import { DialogShell } from './components/DialogShell';
import { useSettingsStore, useConversationStore, useMessageStore, useProviderStore } from './stores';
import { useImageEditorStore } from './stores/imageEditor';
import { useMusicStudioStore } from './stores/musicStudio';
import { useVideoStudioStore } from './stores/videoStudio';
import { authApi } from './api';
import { matchesShortcut } from './shortcuts';
import {
  Settings, Keyboard, BarChart3, Layout, Puzzle, FlaskConical,
  Search, FileArchive, SlidersHorizontal, Files,
  type LucideIcon,
} from 'lucide-react';
import { Toaster, toast } from 'sonner';
import { AnimatePresence, motion } from 'framer-motion';

function useIsMobile(breakpoint = 768) {
  const [isMobile, setIsMobile] = useState(
    typeof window !== 'undefined' ? window.innerWidth < breakpoint : false
  );
  useEffect(() => {
    const mq = window.matchMedia(`(max-width: ${breakpoint - 1}px)`);
    const handler = (e: MediaQueryListEvent) => setIsMobile(e.matches);
    setIsMobile(mq.matches);
    mq.addEventListener('change', handler);
    return () => mq.removeEventListener('change', handler);
  }, [breakpoint]);
  return isMobile;
}

type OverlayPanel = 'shortcuts' | 'usage' | 'templates' | 'plugins' | 'eval' | 'search' | 'importExport' | 'fileLibrary' | 'tools';
type ToolPanel = Exclude<OverlayPanel, 'tools'>;

const GLOBAL_TOOL_ACTIONS: Array<{
  panel: ToolPanel;
  label: string;
  ariaLabel: string;
  Icon: LucideIcon;
}> = [
  { panel: 'search', label: 'Search', ariaLabel: 'Search conversations', Icon: Search },
  { panel: 'usage', label: 'Usage', ariaLabel: 'Usage Dashboard', Icon: BarChart3 },
  { panel: 'templates', label: 'Templates', ariaLabel: 'Prompt Templates', Icon: Layout },
  { panel: 'plugins', label: 'Plugins', ariaLabel: 'Plugins', Icon: Puzzle },
  { panel: 'eval', label: 'Eval', ariaLabel: 'Evaluation Harness', Icon: FlaskConical },
  { panel: 'fileLibrary', label: 'File Library', ariaLabel: 'File Library', Icon: Files },
  { panel: 'importExport', label: 'Import/Export', ariaLabel: 'Import and export data', Icon: FileArchive },
  { panel: 'shortcuts', label: 'Shortcuts', ariaLabel: 'Keyboard shortcuts', Icon: Keyboard },
];

function getNewImageSessionTitle() {
  const now = new Date();
  return now.toLocaleString(undefined, {
    month: 'short', day: 'numeric', year: 'numeric',
    hour: 'numeric', minute: '2-digit',
  });
}

function App() {
  const { toggleSettings, settingsOpen, sidebarOpen, toggleSidebar, appMode, setAppMode } = useSettingsStore();
  const { activeId, createConversation, selectConversation } = useConversationStore();
  const { clearMessages, fetchMessages } = useMessageStore();
  const providers = useProviderStore((s) => s.providers);
  const imageSessionId = useImageEditorStore((state) => state.activeSessionId);
  const { createSession: createImageSession, loadAllSessions, loadSession: loadImageSession } = useImageEditorStore();
  const musicSessionId = useMusicStudioStore((state) => state.activeSessionId);
  const { createSession: createMusicSession, loadSessions: loadMusicSessions, selectSession: selectMusicSession } = useMusicStudioStore();
  const videoProjectId = useVideoStudioStore((state) => state.activeProjectId);
  const { createProject: createVideoProject, loadProjects: loadVideoProjects, selectProject: selectVideoProject } = useVideoStudioStore();
  const [activePanel, setActivePanel] = useState<OverlayPanel | null>(null);
  const [fileLibraryPreferredScope, setFileLibraryPreferredScope] = useState<'workspace' | 'conversation' | 'global' | 'all'>('all');
  const [fileLibraryPreferredWorkspaceId, setFileLibraryPreferredWorkspaceId] = useState<string | null>(null);
  const [authenticated, setAuthenticated] = useState(true); // Default true (solo mode)
  const [authChecked, setAuthChecked] = useState(false);
  const isMobile = useIsMobile();

  useEffect(() => {
    const restoreRoute = async () => {
      const parts = window.location.pathname.split('/').filter(Boolean).map(decodeURIComponent);
      if (parts[0] === 'chat' && parts[1]) {
        selectConversation(parts[1]);
        await fetchMessages(parts[1]);
      } else if (parts[0] === 'image' && parts[1]) {
        await loadAllSessions();
        const session = useImageEditorStore.getState().allSessions.find((item) => item.id === parts[1]);
        if (session) await loadImageSession(session.conversation_id, session.id);
      } else if (parts[0] === 'music' && parts[1]) {
        await loadMusicSessions();
        await selectMusicSession(parts[1]);
      } else if (parts[0] === 'video' && parts[1]) {
        await loadVideoProjects();
        await selectVideoProject(parts[1]);
      } else if (activeId && appMode === 'chat') {
        await fetchMessages(activeId);
      }
    };
    void restoreRoute();
  // The route is intentionally restored only once; store actions are stable Zustand references.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    let path = '/';
    if (appMode === 'chat' && activeId) path = `/chat/${encodeURIComponent(activeId)}`;
    if (appMode === 'image' && imageSessionId) path = `/image/${encodeURIComponent(imageSessionId)}`;
    if (appMode === 'music' && musicSessionId) path = `/music/${encodeURIComponent(musicSessionId)}`;
    if (appMode === 'video' && videoProjectId) path = `/video/${encodeURIComponent(videoProjectId)}`;
    if (appMode === 'video-edit' && videoProjectId) path = `/video/${encodeURIComponent(videoProjectId)}/edit`;
    if (window.location.pathname !== path) window.history.replaceState({}, '', path);
  }, [activeId, appMode, imageSessionId, musicSessionId, videoProjectId]);

  // Studio data must load independently of the sidebar: the mobile drawer unmounts
  // immediately after a mode change.
  useEffect(() => {
    if (appMode === 'image') void loadAllSessions();
    if (appMode === 'music') void loadMusicSessions();
    if (appMode === 'video' || appMode === 'video-edit') void loadVideoProjects();
  }, [appMode, loadAllSessions, loadMusicSessions, loadVideoProjects]);

  const shortcutsOpen = activePanel === 'shortcuts';
  const usageOpen = activePanel === 'usage';
  const templatesOpen = activePanel === 'templates';
  const pluginsOpen = activePanel === 'plugins';
  const evalOpen = activePanel === 'eval';
  const searchOpen = activePanel === 'search';
  const fileLibraryOpen = activePanel === 'fileLibrary';
  const importExportOpen = activePanel === 'importExport';
  const toolsOpen = activePanel === 'tools';

  const dismissPopovers = useCallback(() => {
    window.dispatchEvent(new CustomEvent('omnillm:dismiss-popovers'));
  }, []);

  const openPanel = useCallback((panel: OverlayPanel) => {
    dismissPopovers();
    setActivePanel(panel);
  }, [dismissPopovers]);

  useEffect(() => {
    const handler = (evt: Event) => {
      const customEvt = evt as CustomEvent<{
        preferredScope?: 'workspace' | 'conversation' | 'global' | 'all';
        preferredWorkspaceId?: string | null;
      }>;
      if (customEvt.detail?.preferredScope) {
        setFileLibraryPreferredScope(customEvt.detail.preferredScope);
      }
      setFileLibraryPreferredWorkspaceId(customEvt.detail?.preferredWorkspaceId || null);
      setActivePanel('fileLibrary');
    };
    window.addEventListener('omnillm:open-file-library', handler as EventListener);
    return () => window.removeEventListener('omnillm:open-file-library', handler as EventListener);
  }, []);

  const togglePanel = useCallback((panel: OverlayPanel) => {
    dismissPopovers();
    setActivePanel((prev) => (prev === panel ? null : panel));
  }, [dismissPopovers]);

  const closePanels = useCallback(() => {
    setActivePanel(null);
  }, []);

  const openSettingsPanel = useCallback(() => {
    dismissPopovers();
    closePanels();
    toggleSettings();
  }, [closePanels, dismissPopovers, toggleSettings]);

  // Check auth status on mount — solo mode (no users) stays authenticated
  useEffect(() => {
    authApi.status()
      .then((status) => {
        if (status.auth_enabled && status.has_users) {
          // Multi-user mode: check if we have a valid token
          const token = localStorage.getItem('omnillm_auth_token');
          if (!token) {
            setAuthenticated(false);
          }
        }
        setAuthChecked(true);
      })
      .catch(() => {
        // If auth endpoint fails, assume solo mode
        setAuthChecked(true);
      });
  }, []);

  // Keyboard shortcuts
  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      // Skip shortcuts when not authenticated
      if (!authenticated) return;

      if (matchesShortcut(e, 'newConversation')) {
        e.preventDefault();
        if (appMode === 'image') {
          createImageSession(getNewImageSessionTitle()).then(async (session) => {
            if (!session) return;
            await loadAllSessions();
            await loadImageSession(session.conversation_id, session.id);
            toast.success('New image session created');
          });
          return;
        }
        if (appMode === 'music') {
          createMusicSession().then(async (session) => {
            if (!session) return;
            await loadMusicSessions();
            toast.success('New music session created');
          });
          return;
        }
        if (appMode === 'video' || appMode === 'video-edit') {
          createVideoProject().then(async (project) => {
            if (!project) return;
            await loadVideoProjects();
            toast.success('New video project created');
          });
          return;
        }
        const enabledProviders = providers.filter((p) => p.enabled);
        if (enabledProviders.length === 0) {
          toast.error('Configure and enable a provider before starting a conversation');
          openSettingsPanel();
          return;
        }
        const defaultProvider = enabledProviders[0];
        createConversation(undefined, {
          provider: defaultProvider?.id,
          model: defaultProvider?.default_model || undefined,
        }).then((convo) => {
          clearMessages(convo.id);
          selectConversation(convo.id);
          toast.success('New conversation created');
        });
      }

      if (matchesShortcut(e, 'openSettings')) {
        e.preventDefault();
        openSettingsPanel();
      }

      if (matchesShortcut(e, 'openShortcuts')) {
        e.preventDefault();
        togglePanel('shortcuts');
      }

      if (matchesShortcut(e, 'openSearch')) {
        e.preventDefault();
        togglePanel('search');
      }

      if (matchesShortcut(e, 'toggleSidebar')) {
        e.preventDefault();
        toggleSidebar();
      }

      if (e.key === 'Escape') {
        if (activePanel) {
          closePanels();
          return;
        }
        if (settingsOpen) {
          toggleSettings();
        }
      }
    },
    [
      activePanel,
      appMode,
      authenticated,
      closePanels,
      createConversation,
      createImageSession,
      createMusicSession,
      createVideoProject,
      clearMessages,
      loadAllSessions,
      loadImageSession,
      loadMusicSessions,
      loadVideoProjects,
      openSettingsPanel,
      providers,
      selectConversation,
      settingsOpen,
      togglePanel,
      toggleSettings,
      toggleSidebar,
    ]
  );

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleKeyDown]);

  return (
    <>
      <Toaster
        position="top-center"
        toastOptions={{
          style: {
            background: 'var(--color-surface-alt)',
            border: '1px solid var(--color-border)',
            color: 'var(--color-text)',
            fontSize: '0.875rem',
          },
        }}
        gap={8}
      />

      {/* Auth gate: show login screen when not authenticated */}
      {!authenticated && authChecked && (
        <LoginScreen onAuthenticated={() => setAuthenticated(true)} />
      )}

      {authenticated && (
      <div className="flex h-full relative overflow-hidden">
        {/* Ambient background effects */}
        <div className="fixed inset-0 pointer-events-none z-0 overflow-hidden">
          <div
            className="absolute rounded-full blur-[120px] opacity-[0.03]"
            style={{
              width: '600px',
              height: '600px',
              background: 'linear-gradient(135deg, var(--color-primary), var(--color-accent))',
              top: '-10%',
              right: '-5%',
            }}
          />
          <div
            className="absolute rounded-full blur-[120px] opacity-[0.02]"
            style={{
              width: '400px',
              height: '400px',
              background: 'linear-gradient(135deg, var(--color-accent), #ec4899)',
              bottom: '-5%',
              left: '10%',
            }}
          />
        </div>

        {/* Mobile sidebar: overlay with backdrop */}
        {isMobile && (
          <AnimatePresence>
            {sidebarOpen && (
              <>
                <motion.div
                  key="sidebar-backdrop"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  className="fixed inset-0 bg-black/50 backdrop-blur-sm z-30"
                  onClick={toggleSidebar}
                />
                <motion.div
                  key="sidebar-mobile"
                  initial={{ x: -288 }}
                  animate={{ x: 0 }}
                  exit={{ x: -288 }}
                  transition={{ duration: 0.3, ease: [0.4, 0, 0.2, 1] }}
                  className="fixed left-0 top-0 bottom-0 z-40 w-72"
                >
                  <Sidebar forceOpen />
                </motion.div>
              </>
            )}
          </AnimatePresence>
        )}

        {/* Desktop sidebar: inline with animation */}
        {!isMobile && (
          <>
            <AnimatePresence mode="wait">
              {sidebarOpen && (
                <motion.div
                  key="sidebar"
                  initial={{ width: 0, opacity: 0 }}
                  animate={{ width: 288, opacity: 1 }}
                  exit={{ width: 0, opacity: 0 }}
                  transition={{ duration: 0.3, ease: [0.4, 0, 0.2, 1] }}
                  className="shrink-0 overflow-hidden relative z-10"
                >
                  <Sidebar />
                </motion.div>
              )}
            </AnimatePresence>

            {/* Collapsed sidebar toggle button */}
            {!sidebarOpen && <Sidebar />}
          </>
        )}

        {/* Collapsed sidebar toggle on mobile */}
        {isMobile && !sidebarOpen && <Sidebar />}

        <main className="flex-1 flex flex-col relative z-10 min-w-0">
          {/* Global tools participate in layout so they cannot cover workspace controls. */}
          <div className="shrink-0 border-b border-border bg-surface-raised/85 backdrop-blur px-3 py-2 pl-14 md:pl-3">
            <div className="flex min-h-11 items-center justify-between gap-3">
              <div className="min-w-0">
                <p className="text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted">
                  {appMode === 'image' ? 'Image Studio' : appMode === 'music' ? 'Music Studio' : appMode === 'video' ? 'Video Studio' : appMode === 'video-edit' ? 'Edit Studio' : 'Chat Studio'}
                </p>
              </div>

              {isMobile ? (
                <div className="flex shrink-0 items-center gap-2">
                  <motion.button
                    whileTap={{ scale: 0.97 }}
                    onClick={openSettingsPanel}
                    className="min-h-11 inline-flex items-center gap-2 rounded-xl glass px-3 text-sm font-medium text-text-muted hover:text-text transition-colors"
                    aria-label="Settings"
                    title="Settings"
                  >
                    <Settings size={16} />
                    <span>Settings</span>
                  </motion.button>
                  <motion.button
                    whileTap={{ scale: 0.97 }}
                    onClick={() => openPanel('tools')}
                    className="min-h-11 inline-flex items-center gap-2 rounded-xl glass px-3 text-sm font-medium text-text-muted hover:text-text transition-colors"
                    aria-label="Open tools menu"
                  >
                    <SlidersHorizontal size={16} />
                    <span>Tools</span>
                  </motion.button>
                </div>
              ) : (
                <div className="flex flex-wrap items-center justify-end gap-1.5">
                  {GLOBAL_TOOL_ACTIONS.map(({ panel, label, ariaLabel, Icon }) => (
                    <motion.button
                      key={panel}
                      whileHover={{ scale: 1.03 }}
                      whileTap={{ scale: 0.97 }}
                      onClick={() => openPanel(panel)}
                      className="min-h-10 inline-flex items-center gap-1.5 rounded-xl glass px-2.5 text-text-muted hover:text-text transition-colors duration-200"
                      aria-label={ariaLabel}
                      title={ariaLabel}
                    >
                      <Icon size={16} />
                      <span className="hidden xl:inline text-[11px]">{label}</span>
                    </motion.button>
                  ))}
                  <motion.button
                    whileHover={{ scale: 1.03 }}
                    whileTap={{ scale: 0.97 }}
                    onClick={openSettingsPanel}
                    className="min-h-10 inline-flex items-center gap-1.5 rounded-xl glass px-2.5 text-text-muted hover:text-text transition-colors duration-200"
                    aria-label="Settings"
                    title="Settings"
                  >
                    <Settings size={16} />
                    <span className="hidden xl:inline text-[11px]">Settings</span>
                  </motion.button>
                </div>
              )}
            </div>
          </div>

          {appMode === 'chat' && <ChatView />}
          {appMode === 'image' && <ImageEditStudio />}
          {appMode === 'music' && <MusicStudio />}
          {appMode === 'video' && <VideoStudio />}
          {appMode === 'video-edit' && <VideoEditStudioEnhanced />}
        </main>

        <DialogShell
          open={toolsOpen}
          onClose={closePanels}
          title="Tools"
          icon={<SlidersHorizontal size={18} />}
          maxWidth="max-w-md"
          maxHeight="max-h-[70vh]"
          placement="bottom"
          bodyClassName="p-3 sm:p-4"
        >
          <div className="grid grid-cols-2 gap-2">
            {GLOBAL_TOOL_ACTIONS.map(({ panel, label, ariaLabel, Icon }) => (
              <button
                key={panel}
                onClick={() => openPanel(panel)}
                className="min-h-12 rounded-xl border border-border bg-surface-alt px-3 text-left text-sm text-text-secondary hover:text-text hover:border-primary/30 transition-colors inline-flex items-center gap-2"
                aria-label={ariaLabel}
              >
                <Icon size={16} className="shrink-0 text-primary" />
                <span className="truncate">{label}</span>
              </button>
            ))}
            <button
              onClick={openSettingsPanel}
              className="min-h-12 rounded-xl border border-border bg-surface-alt px-3 text-left text-sm text-text-secondary hover:text-text hover:border-primary/30 transition-colors inline-flex items-center gap-2"
              aria-label="Settings"
            >
              <Settings size={16} className="shrink-0 text-primary" />
              <span className="truncate">Settings</span>
            </button>
          </div>
        </DialogShell>

        <SettingsPanel />
        <KeyboardShortcuts open={shortcutsOpen} onClose={closePanels} />
        <UsageDashboard open={usageOpen} onClose={closePanels} />
        <TemplateManager open={templatesOpen} onClose={closePanels} />
        <PluginManager open={pluginsOpen} onClose={closePanels} />
        <EvalDashboard open={evalOpen} onClose={closePanels} />
        <SearchPanel
          open={searchOpen}
          onClose={closePanels}
          onSelectResult={(conversationId) => {
            setAppMode('chat');
            selectConversation(conversationId);
            fetchMessages(conversationId);
          }}
        />
        <FileLibraryPanel
          open={fileLibraryOpen}
          onClose={closePanels}
          preferredScope={fileLibraryPreferredScope}
          preferredWorkspaceId={fileLibraryPreferredWorkspaceId}
        />
        <ImportExportPanel open={importExportOpen} onClose={closePanels} />
      </div>
      )}
    </>
  );
}

export default App;