import { useEffect, useCallback, useState } from 'react';
import { Sidebar } from './components/Sidebar';
import { ChatView } from './components/ChatView';
import { ImageEditStudio } from './components/image/ImageEditStudio';
import { SettingsPanel } from './components/SettingsPanel';
import { KeyboardShortcuts } from './components/KeyboardShortcuts';
import { LoginScreen } from './components/LoginScreen';
import { UsageDashboard } from './components/UsageDashboard';
import { TemplateManager } from './components/TemplateManager';
import { PluginManager } from './components/PluginManager';
import { EvalDashboard } from './components/EvalDashboard';
import { SearchPanel } from './components/SearchPanel';
import { ImportExportPanel } from './components/ImportExportPanel';
import { useSettingsStore, useConversationStore, useMessageStore } from './stores';
import { useImageEditorStore } from './stores/imageEditor';
import { authApi } from './api';
import { matchesShortcut } from './shortcuts';
import {
  Settings, Keyboard, BarChart3, Layout, Puzzle, FlaskConical,
  Search, FileArchive,
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

type OverlayPanel = 'shortcuts' | 'usage' | 'templates' | 'plugins' | 'eval' | 'search' | 'importExport';

function getNewImageSessionTitle() {
  const now = new Date();
  return now.toLocaleString(undefined, {
    month: 'short', day: 'numeric', year: 'numeric',
    hour: 'numeric', minute: '2-digit',
  });
}

function App() {
  const { toggleSettings, settingsOpen, sidebarOpen, toggleSidebar, appMode, setAppMode } = useSettingsStore();
  const { createConversation, selectConversation } = useConversationStore();
  const { clearMessages, fetchMessages } = useMessageStore();
  const { createSession: createImageSession, loadAllSessions, loadSession: loadImageSession } = useImageEditorStore();
  const [activePanel, setActivePanel] = useState<OverlayPanel | null>(null);
  const [authenticated, setAuthenticated] = useState(true); // Default true (solo mode)
  const [authChecked, setAuthChecked] = useState(false);
  const isMobile = useIsMobile();

  const shortcutsOpen = activePanel === 'shortcuts';
  const usageOpen = activePanel === 'usage';
  const templatesOpen = activePanel === 'templates';
  const pluginsOpen = activePanel === 'plugins';
  const evalOpen = activePanel === 'eval';
  const searchOpen = activePanel === 'search';
  const importExportOpen = activePanel === 'importExport';

  const openPanel = useCallback((panel: OverlayPanel) => {
    setActivePanel(panel);
  }, []);

  const togglePanel = useCallback((panel: OverlayPanel) => {
    setActivePanel((prev) => (prev === panel ? null : panel));
  }, []);

  const closePanels = useCallback(() => {
    setActivePanel(null);
  }, []);

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
        createConversation().then((convo) => {
          clearMessages();
          selectConversation(convo.id);
          toast.success('New conversation created');
        });
      }

      if (matchesShortcut(e, 'openSettings')) {
        e.preventDefault();
        closePanels();
        toggleSettings();
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
      clearMessages,
      loadAllSessions,
      loadImageSession,
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
                  <Sidebar />
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
          {/* Top bar */}
          <div className="absolute top-3 right-3 z-30 flex items-center gap-1">
            <motion.button
              whileHover={{ scale: 1.05 }}
              whileTap={{ scale: 0.95 }}
              onClick={() => openPanel('search')}
              className="inline-flex items-center gap-1.5 px-2.5 py-2.5 rounded-xl glass text-text-muted hover:text-text
                         transition-colors duration-200"
              aria-label="Search (Ctrl+/)"
            >
              <Search size={16} />
              <span className="hidden xl:inline text-[11px]">Search</span>
            </motion.button>
            <motion.button
              whileHover={{ scale: 1.05 }}
              whileTap={{ scale: 0.95 }}
              onClick={() => openPanel('usage')}
              className="inline-flex items-center gap-1.5 px-2.5 py-2.5 rounded-xl glass text-text-muted hover:text-text
                         transition-colors duration-200"
              aria-label="Usage Dashboard"
            >
              <BarChart3 size={16} />
              <span className="hidden xl:inline text-[11px]">Usage</span>
            </motion.button>
            <motion.button
              whileHover={{ scale: 1.05 }}
              whileTap={{ scale: 0.95 }}
              onClick={() => openPanel('templates')}
              className="inline-flex items-center gap-1.5 px-2.5 py-2.5 rounded-xl glass text-text-muted hover:text-text
                         transition-colors duration-200"
              aria-label="Prompt Templates"
            >
              <Layout size={16} />
              <span className="hidden xl:inline text-[11px]">Templates</span>
            </motion.button>
            <motion.button
              whileHover={{ scale: 1.05 }}
              whileTap={{ scale: 0.95 }}
              onClick={() => openPanel('plugins')}
              className="inline-flex items-center gap-1.5 px-2.5 py-2.5 rounded-xl glass text-text-muted hover:text-text
                         transition-colors duration-200"
              aria-label="Plugins"
            >
              <Puzzle size={16} />
              <span className="hidden xl:inline text-[11px]">Plugins</span>
            </motion.button>
            <motion.button
              whileHover={{ scale: 1.05 }}
              whileTap={{ scale: 0.95 }}
              onClick={() => openPanel('eval')}
              className="inline-flex items-center gap-1.5 px-2.5 py-2.5 rounded-xl glass text-text-muted hover:text-text
                         transition-colors duration-200"
              aria-label="Evaluation Harness"
            >
              <FlaskConical size={16} />
              <span className="hidden xl:inline text-[11px]">Eval</span>
            </motion.button>
            <motion.button
              whileHover={{ scale: 1.05 }}
              whileTap={{ scale: 0.95 }}
              onClick={() => openPanel('importExport')}
              className="inline-flex items-center gap-1.5 px-2.5 py-2.5 rounded-xl glass text-text-muted hover:text-text
                         transition-colors duration-200"
              aria-label="Import/Export"
            >
              <FileArchive size={16} />
              <span className="hidden xl:inline text-[11px]">Import/Export</span>
            </motion.button>
            <motion.button
              whileHover={{ scale: 1.05 }}
              whileTap={{ scale: 0.95 }}
              onClick={() => openPanel('shortcuts')}
              className="inline-flex items-center gap-1.5 px-2.5 py-2.5 rounded-xl glass text-text-muted hover:text-text
                         transition-colors duration-200"
              aria-label="Keyboard shortcuts (Ctrl+K)"
            >
              <Keyboard size={16} />
              <span className="hidden xl:inline text-[11px]">Shortcuts</span>
            </motion.button>
            <motion.button
              whileHover={{ scale: 1.05 }}
              whileTap={{ scale: 0.95 }}
              onClick={() => {
                closePanels();
                toggleSettings();
              }}
              className="inline-flex items-center gap-1.5 px-2.5 py-2.5 rounded-xl glass text-text-muted hover:text-text
                         transition-colors duration-200"
              aria-label="Settings (Ctrl+,)"
            >
              <Settings size={16} />
              <span className="hidden xl:inline text-[11px]">Settings</span>
            </motion.button>
          </div>

          {appMode === 'chat' && <ChatView />}
          {appMode === 'image' && <ImageEditStudio />}
        </main>

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
        <ImportExportPanel open={importExportOpen} onClose={closePanels} />
      </div>
      )}
    </>
  );
}

export default App;
