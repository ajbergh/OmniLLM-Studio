import { useEffect, useState, useCallback } from 'react';
import type { ReactNode } from 'react';
import { useProviderStore, useSettingsStore, useFeatureFlagStore } from '../stores';
import { api, authApi, browserApi, mcpApi, musicApi, videoApi, setAuthToken } from '../api';
import { X, Plus, Trash2, Eye, EyeOff, Save, Check, Shield, Zap, Globe, Server, Cloud, Cpu, ExternalLink, RefreshCw, Database, Wrench, DollarSign, UserPlus, Lock, Users, Palette, ChevronDown, RotateCcw, Plug, Terminal, Play, Square, Pencil, AlertTriangle, CheckCircle2, ClipboardList, Trophy, Github, Calculator, Link2, Search, Music2, Film, Route as RouteIcon } from 'lucide-react';
import type { BrowserSession, BrowserStatus, CreateMCPServerRequest, MCPAuditEvent, MCPServer, MCPTool, MCPTransport, OpenRouterMetadata, ToolPolicy, UpdateMCPServerRequest } from '../types';
import type { MusicModel, MusicProviderKey } from '../types/music';
import type { VideoModel, VideoProviderKey } from '../types/video';
import { useTheme, THEMES } from '../theme';
import { clsx } from 'clsx';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import { formatModelOptionLabel, getKnownChatModels, getKnownImageModels, isFreeModel } from '../models';

const PROVIDER_TYPES = [
  { value: 'openai', label: 'OpenAI', icon: Zap, color: 'from-emerald-500/20 to-green-500/20', iconColor: 'text-emerald-400' },
  { value: 'anthropic', label: 'Anthropic', icon: Shield, color: 'from-orange-500/20 to-amber-500/20', iconColor: 'text-orange-400' },
  { value: 'gemini', label: 'Google Gemini', icon: Globe, color: 'from-blue-500/20 to-cyan-500/20', iconColor: 'text-blue-400' },
  { value: 'ollama', label: 'Ollama (Local)', icon: Server, color: 'from-purple-500/20 to-violet-500/20', iconColor: 'text-purple-400' },
  { value: 'openrouter', label: 'OpenRouter', icon: Cloud, color: 'from-pink-500/20 to-rose-500/20', iconColor: 'text-pink-400' },
  { value: 'groq', label: 'Groq', icon: Zap, color: 'from-yellow-500/20 to-amber-500/20', iconColor: 'text-yellow-400' },
  { value: 'together', label: 'Together AI', icon: Cpu, color: 'from-indigo-500/20 to-blue-500/20', iconColor: 'text-indigo-400' },
  { value: 'mistral', label: 'Mistral AI', icon: Globe, color: 'from-cyan-500/20 to-teal-500/20', iconColor: 'text-cyan-400' },
  { value: 'elevenlabs', label: 'ElevenLabs', icon: Music2, color: 'from-violet-500/20 to-purple-500/20', iconColor: 'text-violet-400' },
  { value: 'custom', label: 'Custom (OpenAI-compatible)', icon: Server, color: 'from-gray-500/20 to-slate-500/20', iconColor: 'text-gray-400' },
];

function getProviderMeta(type: string) {
  return PROVIDER_TYPES.find((t) => t.value === type) || PROVIDER_TYPES[PROVIDER_TYPES.length - 1];
}

function FreeModelBadge() {
  return (
    <span className="shrink-0 rounded-md border border-success/30 bg-success/10 px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-wide text-success">
      FREE
    </span>
  );
}

type SettingsTab = 'providers' | 'general' | 'appearance' | 'rag' | 'routing' | 'music' | 'video' | 'tools' | 'mcp' | 'pricing' | 'auth';

const SETTINGS_TABS: Array<{ key: SettingsTab; label: string }> = [
  { key: 'providers', label: 'Providers' },
  { key: 'general', label: 'General' },
  { key: 'appearance', label: 'Appearance' },
  { key: 'rag', label: 'RAG' },
  { key: 'routing', label: 'Routing' },
  { key: 'music', label: 'Music' },
  { key: 'video', label: 'Video' },
  { key: 'tools', label: 'Tools' },
  { key: 'mcp', label: 'MCP' },
  { key: 'pricing', label: 'Pricing' },
  { key: 'auth', label: 'Auth' },
];

const panelVariants = {
  hidden: { x: '100%', opacity: 0 },
  visible: { x: 0, opacity: 1, transition: { type: 'spring' as const, damping: 30, stiffness: 300 } },
  exit: { x: '100%', opacity: 0, transition: { duration: 0.2 } },
};

const backdropVariants = {
  hidden: { opacity: 0 },
  visible: { opacity: 1 },
  exit: { opacity: 0 },
};

export function SettingsPanel() {
  const { settingsOpen, toggleSettings, fetchSettings } = useSettingsStore();
  const { providers, fetchProviders, createProvider, updateProvider, deleteProvider } =
    useProviderStore();
  const { fetchFeatures } = useFeatureFlagStore();
  const [tab, setTab] = useState<SettingsTab>('providers');

  useEffect(() => {
    if (settingsOpen) {
      fetchProviders();
      fetchSettings();
      fetchFeatures();
    }
  }, [settingsOpen, fetchProviders, fetchSettings, fetchFeatures]);

  return (
    <AnimatePresence>
      {settingsOpen && (
        <>
          {/* Backdrop */}
          <motion.div
            variants={backdropVariants}
            initial="hidden"
            animate="visible"
            exit="exit"
            className="fixed inset-0 bg-black/60 backdrop-blur-sm z-40"
            onClick={toggleSettings}
          />

          {/* Panel */}
          <motion.div
            role="dialog"
            aria-modal="true"
            aria-label="Settings"
            variants={panelVariants}
            initial="hidden"
            animate="visible"
            exit="exit"
            className="fixed right-0 top-0 bottom-0 w-full sm:max-w-xl bg-surface-raised border-l border-border z-50 flex flex-col shadow-2xl"
          >
            {/* Header */}
            <div className="p-5 flex items-center justify-between">
              <div>
                <h2 className="text-lg font-bold gradient-text">Settings</h2>
                <p className="text-xs text-text-muted mt-0.5">Manage providers and preferences</p>
              </div>
              <motion.button
                whileHover={{ scale: 1.1, rotate: 90 }}
                whileTap={{ scale: 0.9 }}
                onClick={toggleSettings}
                className="p-2 rounded-xl hover:bg-surface-hover text-text-muted hover:text-text transition-colors"
                aria-label="Close Settings"
                title="Close Settings"
              >
                <X size={18} />
              </motion.button>
            </div>

            {/* Tabs */}
            <div className="mx-5 sm:hidden">
              <label className="sr-only" htmlFor="settings-tab-select">Settings section</label>
              <select
                id="settings-tab-select"
                value={tab}
                onChange={(event) => setTab(event.target.value as SettingsTab)}
                className="w-full min-h-11 rounded-xl border border-border bg-surface-alt px-3 text-sm text-text focus:outline-none focus:border-primary/50"
              >
                {SETTINGS_TABS.map((t) => (
                  <option key={t.key} value={t.key}>{t.label}</option>
                ))}
              </select>
            </div>
            <div className="hidden sm:flex mx-5 bg-surface-alt rounded-xl p-1 gap-1 flex-wrap">
              {SETTINGS_TABS.map((t) => (
                <button
                  key={t.key}
                  onClick={() => setTab(t.key)}
                  className={clsx(
                    'shrink-0 min-w-[84px] px-2.5 py-2 text-sm font-medium rounded-lg transition-all duration-200',
                    tab === t.key
                      ? 'bg-surface-hover text-text shadow-sm'
                      : 'text-text-muted hover:text-text'
                  )}
                >
                  {t.label}
                </button>
              ))}
            </div>

            {/* Content */}
            <div className="flex-1 overflow-y-auto p-5">
              <AnimatePresence mode="wait">
                {tab === 'providers' ? (
                  <motion.div
                    key="providers"
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -10 }}
                    transition={{ duration: 0.2 }}
                  >
                    <ProvidersTab
                      providers={providers}
                      onAdd={async (data) => {
                        await createProvider(data);
                        toast.success('Provider added');
                      }}
                      onUpdate={updateProvider}
                      onDelete={(id) => {
                        toast('Delete this provider?', {
                          action: {
                            label: 'Delete',
                            onClick: async () => {
                              await deleteProvider(id);
                              toast.success('Provider removed');
                            },
                          },
                          cancel: { label: 'Cancel', onClick: () => {} },
                          duration: 5000,
                        });
                      }}
                    />
                  </motion.div>
                ) : tab === 'general' ? (
                  <motion.div
                    key="general"
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -10 }}
                    transition={{ duration: 0.2 }}
                  >
                    <GeneralTab />
                  </motion.div>
                ) : tab === 'appearance' ? (
                  <motion.div
                    key="appearance"
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -10 }}
                    transition={{ duration: 0.2 }}
                  >
                    <AppearanceTab />
                  </motion.div>
                ) : tab === 'rag' ? (
                  <motion.div
                    key="rag"
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -10 }}
                    transition={{ duration: 0.2 }}
                  >
                    <RAGTab />
                  </motion.div>
                ) : tab === 'routing' ? (
                  <motion.div
                    key="routing"
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -10 }}
                    transition={{ duration: 0.2 }}
                  >
                    <RoutingTab />
                  </motion.div>
                ) : tab === 'music' ? (
                  <motion.div
                    key="music"
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -10 }}
                    transition={{ duration: 0.2 }}
                  >
                    <MusicTab />
                  </motion.div>
                ) : tab === 'video' ? (
                  <motion.div
                    key="video"
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -10 }}
                    transition={{ duration: 0.2 }}
                  >
                    <VideoTab />
                  </motion.div>
                ) : tab === 'tools' ? (
                  <motion.div
                    key="tools"
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -10 }}
                    transition={{ duration: 0.2 }}
                  >
                    <ToolsTab />
                  </motion.div>
                ) : tab === 'mcp' ? (
                  <motion.div
                    key="mcp"
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -10 }}
                    transition={{ duration: 0.2 }}
                  >
                    <MCPServersTab />
                  </motion.div>
                ) : tab === 'pricing' ? (
                  <motion.div
                    key="pricing"
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -10 }}
                    transition={{ duration: 0.2 }}
                  >
                    <PricingTab />
                  </motion.div>
                ) : (
                  <motion.div
                    key="auth"
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -10 }}
                    transition={{ duration: 0.2 }}
                  >
                    <AuthTab />
                  </motion.div>
                )}
              </AnimatePresence>
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
}

function ProvidersTab({
  providers,
  onAdd,
  onUpdate,
  onDelete,
}: {
  providers: ReturnType<typeof useProviderStore.getState>['providers'];
  onAdd: (data: { name: string; type: string; api_key?: string; base_url?: string; default_model?: string; default_image_model?: string }) => Promise<void>;
  onUpdate: (id: string, data: Record<string, unknown>) => Promise<void>;
  onDelete: (id: string) => void;
}) {
  const [adding, setAdding] = useState(false);
  const [newProvider, setNewProvider] = useState({
    name: '',
    type: 'openai',
    api_key: '',
    base_url: '',
    default_model: '',
    default_image_model: '',
  });
  const [ollamaModels, setOllamaModels] = useState<string[]>([]);
  const [ollamaLoading, setOllamaLoading] = useState(false);

  const fetchOllama = useCallback(async (baseUrl?: string) => {
    setOllamaLoading(true);
    const models = await api.fetchOllamaModels(baseUrl);
    setOllamaModels(models);
    setOllamaLoading(false);
  }, []);

  // Auto-fetch Ollama models when type changes to ollama
  useEffect(() => {
    if (newProvider.type === 'ollama' && adding) {
      const timer = window.setTimeout(() => {
        fetchOllama(newProvider.base_url || undefined);
      }, 300);
      return () => window.clearTimeout(timer);
    }
  }, [newProvider.type, adding, fetchOllama, newProvider.base_url]);

  const handleAdd = async () => {
    if (!newProvider.name) return;
    await onAdd({
      name: newProvider.name,
      type: newProvider.type,
      api_key: newProvider.api_key || undefined,
      base_url: newProvider.base_url || undefined,
      default_model: newProvider.default_model || undefined,
      default_image_model: newProvider.default_image_model || undefined,
    });
    setNewProvider({ name: '', type: 'openai', api_key: '', base_url: '', default_model: '', default_image_model: '' });
    setAdding(false);
  };

  const selectedMeta = getProviderMeta(newProvider.type);
  const newProviderChatModels = newProvider.type === 'ollama' ? ollamaModels : getKnownChatModels(newProvider.type);
  const newProviderImageModels = getKnownImageModels(newProvider.type);

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-sm text-text-muted">Configure your AI provider connections.</p>
        <motion.button
          whileHover={{ scale: 1.03 }}
          whileTap={{ scale: 0.97 }}
          onClick={() => setAdding(true)}
          className="btn-primary min-h-11 flex items-center justify-center gap-1.5 px-4 text-xs rounded-xl font-medium"
        >
          <Plus size={14} /> Add Provider
        </motion.button>
      </div>

      {/* Add provider form */}
      <AnimatePresence>
        {adding && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            transition={{ duration: 0.3 }}
            className="overflow-hidden"
          >
            <div className="p-5 rounded-2xl border border-primary/20 bg-gradient-to-br from-primary-glow to-transparent space-y-4">
              <h3 className="text-sm font-semibold gradient-text">New Provider</h3>

              {/* Provider type selection - visual grid */}
              <div>
                <label className="block text-xs text-text-muted mb-2 font-medium">Provider Type</label>
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                  {PROVIDER_TYPES.map((t) => (
                    <button
                      key={t.value}
                      onClick={() => setNewProvider((s) => ({
                        ...s,
                        type: t.value,
                        default_model: '',
                        default_image_model: '',
                      }))}
                      className={clsx(
                        'min-h-14 p-3 rounded-xl text-left transition-all duration-200 border',
                        newProvider.type === t.value
                          ? `bg-gradient-to-br ${t.color} border-primary/30 shadow-sm`
                          : 'bg-surface-alt border-border hover:border-border-subtle hover:bg-surface-hover'
                      )}
                    >
                      <t.icon size={16} className={clsx(
                        'mb-1.5',
                        newProvider.type === t.value ? t.iconColor : 'text-text-muted'
                      )} />
                      <span className="text-xs font-medium block truncate">{t.label}</span>
                    </button>
                  ))}
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <InputField
                  label="Display Name"
                  value={newProvider.name}
                  onChange={(v) => setNewProvider((s) => ({ ...s, name: v }))}
                  placeholder={`My ${selectedMeta.label}`}
                />
                <div className="space-y-3">
                  {newProvider.type === 'ollama' ? (
                    <div>
                      <label className="block text-xs text-text-muted mb-1.5 font-medium">
                        {newProviderImageModels.length > 0 ? 'Default Chat Model' : 'Default Model'}
                      </label>
                      <div className="flex gap-2">
                        <select
                          value={newProvider.default_model}
                          onChange={(e) => setNewProvider((s) => ({ ...s, default_model: e.target.value }))}
                          className="min-w-0 flex-1 px-3 py-2.5 text-sm bg-surface border border-border rounded-xl
                                     text-text focus:outline-none focus:border-primary transition-all input-glow"
                        >
                          <option value="">{ollamaLoading ? 'Loading...' : ollamaModels.length ? 'Select a model...' : 'No models found'}</option>
                          {ollamaModels.map((m) => (
                            <option key={m} value={m}>{m}</option>
                          ))}
                        </select>
                        <motion.button
                          whileHover={{ scale: 1.05 }}
                          whileTap={{ scale: 0.95 }}
                          onClick={() => fetchOllama(newProvider.base_url || undefined)}
                          disabled={ollamaLoading}
                          className="min-h-11 min-w-11 inline-flex items-center justify-center rounded-xl border border-border hover:bg-surface-hover
                                     text-text-muted hover:text-text transition-all disabled:opacity-40"
                          aria-label="Refresh models from Ollama"
                        >
                          <RefreshCw size={14} className={ollamaLoading ? 'animate-spin' : ''} />
                        </motion.button>
                      </div>
                      {!ollamaLoading && ollamaModels.length === 0 && (
                        <p className="text-[11px] text-warning mt-1.5">Could not connect to Ollama. Make sure it&apos;s running.</p>
                      )}
                    </div>
                  ) : newProviderChatModels.length > 0 ? (
                    <div>
                      <label className="block text-xs text-text-muted mb-1.5 font-medium">
                        {newProviderImageModels.length > 0 ? 'Default Chat Model' : 'Default Model'}
                      </label>
                      <select
                        value={newProvider.default_model}
                        onChange={(e) => setNewProvider((s) => ({ ...s, default_model: e.target.value }))}
                        className="w-full min-w-0 px-3 py-2.5 text-sm bg-surface border border-border rounded-xl
                                   text-text focus:outline-none focus:border-primary transition-all input-glow"
                      >
                        <option value="">Select a model...</option>
                        {newProviderChatModels.map((m) => (
                          <option key={m} value={m}>{formatModelOptionLabel(newProvider.type, m)}</option>
                        ))}
                      </select>
                    </div>
                  ) : (
                    <InputField
                      label={newProviderImageModels.length > 0 ? 'Default Chat Model' : 'Default Model'}
                      value={newProvider.default_model}
                      onChange={(v) => setNewProvider((s) => ({ ...s, default_model: v }))}
                      placeholder="model-name"
                    />
                  )}

                  {newProviderImageModels.length > 0 && (
                    <div>
                      <label className="block text-xs text-text-muted mb-1.5 font-medium">Default Image Model</label>
                      <select
                        value={newProvider.default_image_model}
                        onChange={(e) => setNewProvider((s) => ({ ...s, default_image_model: e.target.value }))}
                        className="w-full min-w-0 px-3 py-2.5 text-sm bg-surface border border-border rounded-xl
                                   text-text focus:outline-none focus:border-primary transition-all input-glow"
                      >
                        <option value="">Use built-in default</option>
                        {newProviderImageModels.map((m) => (
                          <option key={m} value={m}>{m}</option>
                        ))}
                      </select>
                    </div>
                  )}
                </div>
              </div>

              <InputField
                label="API Key"
                value={newProvider.api_key}
                onChange={(v) => setNewProvider((s) => ({ ...s, api_key: v }))}
                placeholder="sk-..."
                secret
              />
              <InputField
                label="Base URL (optional)"
                value={newProvider.base_url}
                onChange={(v) => setNewProvider((s) => ({ ...s, base_url: v }))}
                placeholder="https://api.openai.com/v1"
              />

              <div className="flex flex-col justify-end gap-2 pt-1 sm:flex-row">
                <button
                  onClick={() => setAdding(false)}
                  className="min-h-11 px-4 text-sm rounded-xl border border-border hover:bg-surface-hover
                             text-text-secondary transition-all"
                >
                  Cancel
                </button>
                <motion.button
                  whileHover={{ scale: 1.02 }}
                  whileTap={{ scale: 0.98 }}
                  onClick={handleAdd}
                  disabled={!newProvider.name}
                  className="btn-primary min-h-11 px-4 text-sm rounded-xl font-medium
                             disabled:opacity-40 disabled:cursor-not-allowed flex items-center gap-1.5"
                >
                  <Save size={14} /> Save Provider
                </motion.button>
              </div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Existing providers */}
      <div className="space-y-3">
        {providers.map((provider, index) => (
          <motion.div
            key={provider.id}
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: index * 0.05 }}
          >
            <ProviderCard
              provider={provider}
              onUpdate={onUpdate}
              onDelete={onDelete}
            />
          </motion.div>
        ))}
      </div>

      {providers.length === 0 && !adding && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          className="text-center py-12"
        >
          <div className="w-14 h-14 rounded-2xl bg-gradient-to-br from-primary/10 to-accent/10 border border-border
                          flex items-center justify-center mx-auto mb-4">
            <Cloud size={24} className="text-primary/60" />
          </div>
          <p className="text-text-muted text-sm mb-1">No providers configured yet</p>
          <p className="text-text-muted/60 text-xs">Add a provider to start chatting with AI models</p>
        </motion.div>
      )}
    </div>
  );
}

function ProviderCard({
  provider,
  onUpdate,
  onDelete,
}: {
  provider: ReturnType<typeof useProviderStore.getState>['providers'][0];
  onUpdate: (id: string, data: Record<string, unknown>) => Promise<void>;
  onDelete: (id: string) => void;
}) {
  const [apiKey, setApiKey] = useState('');
  const [saved, setSaved] = useState(false);
  const [ollamaModels, setOllamaModels] = useState<string[]>([]);
  const [ollamaLoading, setOllamaLoading] = useState(false);
  const meta = getProviderMeta(provider.type);
  const isOllama = provider.type.toLowerCase() === 'ollama';
  const chatModelOptions = isOllama ? ollamaModels : getKnownChatModels(provider.type);
  const imageModelOptions = getKnownImageModels(provider.type);

  const fetchOllamaModelsForCard = useCallback(async () => {
    setOllamaLoading(true);
    const models = await api.fetchOllamaModels(provider.base_url || undefined);
    setOllamaModels(models);
    setOllamaLoading(false);
  }, [provider.base_url]);

  useEffect(() => {
    if (isOllama) {
      fetchOllamaModelsForCard();
    }
  }, [isOllama, fetchOllamaModelsForCard]);

  const handleSaveKey = async () => {
    if (!apiKey) return;
    await onUpdate(provider.id, { api_key: apiKey });
    setApiKey('');
    setSaved(true);
    toast.success('API key updated');
    setTimeout(() => setSaved(false), 2000);
  };

  return (
    <div className={clsx(
      'p-4 rounded-2xl border bg-surface-alt space-y-3 transition-all duration-200',
      provider.enabled ? 'border-border hover:border-primary/20' : 'border-border opacity-60'
    )}>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-center gap-3">
          <div className={clsx(
            'w-9 h-9 rounded-xl bg-gradient-to-br flex items-center justify-center',
            meta.color
          )}>
            <meta.icon size={16} className={meta.iconColor} />
          </div>
          <div className="min-w-0">
            <h3 className="text-sm font-semibold truncate">{provider.name}</h3>
            <span className="text-[11px] text-text-muted capitalize">{meta.label}</span>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <motion.button
            whileHover={{ scale: 1.05 }}
            whileTap={{ scale: 0.95 }}
            onClick={() => {
              onUpdate(provider.id, { enabled: !provider.enabled });
              toast.success(provider.enabled ? 'Provider disabled' : 'Provider enabled');
            }}
            className={clsx(
              'min-h-9 px-3 text-xs rounded-full font-medium transition-all border',
              provider.enabled
                ? 'bg-success-soft text-success border-success/20'
                : 'bg-surface-hover text-text-muted border-border'
            )}
          >
            {provider.enabled ? 'Active' : 'Inactive'}
          </motion.button>
          <motion.button
            whileHover={{ scale: 1.1 }}
            whileTap={{ scale: 0.9 }}
            onClick={() => onDelete(provider.id)}
            className="min-h-10 min-w-10 inline-flex items-center justify-center rounded-xl hover:bg-danger-soft text-text-muted hover:text-danger transition-all"
            aria-label={`Delete ${provider.name}`}
            title="Delete provider"
          >
            <Trash2 size={14} />
          </motion.button>
        </div>
      </div>

      {provider.base_url && (
        <div className="flex items-center gap-1.5 text-xs text-text-muted bg-surface rounded-lg px-3 py-1.5">
          <ExternalLink size={10} />
          <span className="truncate">{provider.base_url}</span>
        </div>
      )}

      {/* Model selection */}
      {isOllama ? (
        <div>
          <label className="block text-xs text-text-muted mb-1.5 font-medium">
            {imageModelOptions.length > 0 ? 'Default Chat Model' : 'Default Model'}
          </label>
          <div className="flex gap-2">
            <select
              value={provider.default_model || ''}
              onChange={(e) => {
                onUpdate(provider.id, { default_model: e.target.value });
                toast.success('Model updated');
              }}
              className="min-w-0 flex-1 px-3 py-2 text-sm bg-surface border border-border rounded-xl
                         text-text focus:outline-none focus:border-primary transition-all input-glow"
            >
              <option value="">{ollamaLoading ? 'Loading...' : ollamaModels.length ? 'Select a model...' : 'No models found'}</option>
              {ollamaModels.map((m) => (
                <option key={m} value={m}>{m}</option>
              ))}
              {provider.default_model && !ollamaModels.includes(provider.default_model) && (
                <option value={provider.default_model}>{provider.default_model} (not installed)</option>
              )}
            </select>
            <motion.button
              whileHover={{ scale: 1.05 }}
              whileTap={{ scale: 0.95 }}
              onClick={fetchOllamaModelsForCard}
              disabled={ollamaLoading}
              className="min-h-10 min-w-10 inline-flex items-center justify-center rounded-xl border border-border hover:bg-surface-hover
                         text-text-muted hover:text-text transition-all disabled:opacity-40"
              aria-label="Refresh models from Ollama"
            >
              <RefreshCw size={14} className={ollamaLoading ? 'animate-spin' : ''} />
            </motion.button>
          </div>
          {!ollamaLoading && ollamaModels.length === 0 && (
            <p className="text-[11px] text-warning mt-1.5">Could not connect to Ollama. Make sure it&apos;s running.</p>
          )}
        </div>
      ) : chatModelOptions.length > 0 ? (
        <div>
          <label className="block text-xs text-text-muted mb-1.5 font-medium">
            {imageModelOptions.length > 0 ? 'Default Chat Model' : 'Default Model'}
          </label>
          <select
            value={provider.default_model || ''}
            onChange={(e) => {
              onUpdate(provider.id, { default_model: e.target.value });
              toast.success('Model updated');
            }}
            className="w-full min-w-0 px-3 py-2 text-sm bg-surface border border-border rounded-xl
                       text-text focus:outline-none focus:border-primary transition-all input-glow"
          >
            <option value="">Select a model...</option>
            {chatModelOptions.map((m) => (
              <option key={m} value={m}>{formatModelOptionLabel(provider.type, m)}</option>
            ))}
            {provider.default_model && !chatModelOptions.includes(provider.default_model) && (
              <option value={provider.default_model}>{formatModelOptionLabel(provider.type, provider.default_model)} (custom)</option>
            )}
          </select>
        </div>
      ) : (
        provider.default_model && (
          <div className="flex items-center gap-1.5 text-xs text-text-muted">
            <Cpu size={10} />
            <span className="font-medium">Model:</span>
            <span className="truncate">{provider.default_model}</span>
            {isFreeModel(provider.type, provider.default_model) && <FreeModelBadge />}
          </div>
        )
      )}

      {imageModelOptions.length > 0 && (
        <div>
          <label className="block text-xs text-text-muted mb-1.5 font-medium">Default Image Model</label>
          <select
            value={provider.default_image_model || ''}
            onChange={(e) => {
              onUpdate(provider.id, { default_image_model: e.target.value });
              toast.success('Default image model updated');
            }}
            className="w-full min-w-0 px-3 py-2 text-sm bg-surface border border-border rounded-xl
                       text-text focus:outline-none focus:border-primary transition-all input-glow"
          >
            <option value="">Use built-in default</option>
            {imageModelOptions.map((m) => (
              <option key={m} value={m}>{m}</option>
            ))}
            {provider.default_image_model && !imageModelOptions.includes(provider.default_image_model) && (
              <option value={provider.default_image_model}>{provider.default_image_model} (custom)</option>
            )}
          </select>
        </div>
      )}

      {/* API Key update */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <input
          type="password"
          value={apiKey}
          onChange={(e) => setApiKey(e.target.value)}
          placeholder="Update API key..."
          className="min-w-0 flex-1 px-3 py-2 text-sm bg-surface border border-border rounded-xl
                     text-text placeholder-text-muted focus:outline-none
                     transition-all input-glow"
        />
        <motion.button
          whileHover={{ scale: 1.05 }}
          whileTap={{ scale: 0.95 }}
          onClick={handleSaveKey}
          disabled={!apiKey}
          className="min-h-11 min-w-11 inline-flex items-center justify-center rounded-xl btn-primary disabled:opacity-40 disabled:cursor-not-allowed"
          aria-label={`Save API key for ${provider.name}`}
          title="Save API key"
        >
          {saved ? <Check size={14} /> : <Save size={14} />}
        </motion.button>
      </div>

      {/* OpenRouter-specific settings */}
      {provider.type === 'openrouter' && <OpenRouterSettings provider={provider} onUpdate={onUpdate} />}

      {/* OpenRouter: Fetch models dynamically */}
      {provider.type === 'openrouter' && (
        <OpenRouterModelsButton provider={provider} />
      )}
    </div>
  );
}

function OpenRouterModelsButton({
  provider,
}: {
  provider: ReturnType<typeof useProviderStore.getState>['providers'][0];
}) {
  const [loading, setLoading] = useState(false);
  const [models, setModels] = useState<Array<{ id: string; name: string }>>([]);
  const [expanded, setExpanded] = useState(false);

  const fetchModels = async () => {
    setLoading(true);
    try {
      const result = await api.fetchOpenRouterModels(provider.id);
      setModels(result);
      setExpanded(true);
      toast.success(`Fetched ${result.length} models`);
    } catch {
      toast.error('Failed to fetch OpenRouter models');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="border-t border-border pt-3 mt-3">
      <button
        type="button"
        onClick={models.length === 0 ? fetchModels : () => setExpanded(!expanded)}
        disabled={loading}
        className="flex items-center gap-2 text-xs font-medium text-text-muted hover:text-text transition-colors"
      >
        <RefreshCw size={12} className={loading ? 'animate-spin' : ''} />
        <span>{loading ? 'Fetching...' : models.length > 0 ? 'OpenRouter Models' : 'Fetch OpenRouter Models'}</span>
        {models.length > 0 && (
          <ChevronDown
            size={12}
            className={`transition-transform ${expanded ? 'rotate-180' : ''}`}
          />
        )}
      </button>

      {expanded && models.length > 0 && (
        <div className="mt-2 max-h-40 overflow-y-auto space-y-1">
          {models.map((m) => (
            <div
              key={m.id}
              className="flex items-center justify-between text-xs px-2 py-1.5 rounded-lg bg-surface-hover/50"
            >
              <span className="truncate flex-1">{m.name || m.id}</span>
              <span className="text-text-muted/60 ml-2 font-mono text-[10px]">{m.id}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function OpenRouterSettings({
  provider,
  onUpdate,
}: {
  provider: ReturnType<typeof useProviderStore.getState>['providers'][0];
  onUpdate: (id: string, data: Record<string, unknown>) => Promise<void>;
}) {
  const [expanded, setExpanded] = useState(false);
  const initialMeta: OpenRouterMetadata = (() => {
    try {
      return JSON.parse(provider.metadata_json || '{}') as OpenRouterMetadata;
    } catch {
      return {};
    }
  })();

  const [showCost, setShowCost] = useState(initialMeta.show_cost ?? false);
  const [route, setRoute] = useState(initialMeta.route || '');
  const [allowFallbacks, setAllowFallbacks] = useState(initialMeta.provider_prefs?.allow_fallbacks ?? true);
  const [providerOrder, setProviderOrder] = useState((initialMeta.provider_prefs?.order || []).join(', '));
  const [providerOnly, setProviderOnly] = useState((initialMeta.provider_prefs?.only || []).join(', '));
  const [providerIgnore, setProviderIgnore] = useState((initialMeta.provider_prefs?.ignore || []).join(', '));
  const [modelFallbacks, setModelFallbacks] = useState((initialMeta.model_fallbacks || []).join(', '));
  const [plugins, setPlugins] = useState<Record<string, boolean>>(() => {
    const defaults: Record<string, boolean> = {
      web: false,
      'file-parser': false,
      'response-healing': false,
      'context-compression': false,
    };
    for (const p of initialMeta.plugins || []) {
      if (p.id in defaults) defaults[p.id] = p.enabled ?? true;
    }
    return defaults;
  });
  const [saving, setSaving] = useState(false);

  const parseCSV = (value: string): string[] =>
    value
      .split(',')
      .map((v) => v.trim())
      .filter(Boolean);

  const handleSave = async () => {
    const meta: OpenRouterMetadata = {
      show_cost: showCost,
      route: route || undefined,
      model_fallbacks: parseCSV(modelFallbacks),
      provider_prefs: {
        order: parseCSV(providerOrder),
        only: parseCSV(providerOnly),
        ignore: parseCSV(providerIgnore),
        allow_fallbacks: allowFallbacks,
      },
      plugins: Object.entries(plugins).map(([id, enabled]) => ({ id, enabled })),
    };

    setSaving(true);
    try {
      await onUpdate(provider.id, { metadata_json: JSON.stringify(meta) });
      toast.success('OpenRouter settings saved');
    } catch {
      toast.error('Failed to save OpenRouter settings');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="border-t border-border pt-3 mt-3">
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="flex items-center gap-2 text-xs font-medium text-text-muted hover:text-text transition-colors w-full"
      >
        <Cloud size={12} />
        <span>OpenRouter Settings</span>
        <ChevronDown
          size={12}
          className={`ml-auto transition-transform ${expanded ? 'rotate-180' : ''}`}
        />
      </button>

      {expanded && (
        <div className="mt-3 space-y-3">
          <div className="flex items-center justify-between py-2">
            <div>
              <label className="text-xs font-medium text-text-secondary block">Show Credit Cost</label>
              <p className="text-[10px] text-text-muted mt-0.5">
                Display OpenRouter credit cost in chat messages
              </p>
            </div>
            <button
              type="button"
              onClick={() => setShowCost((v) => !v)}
              className={`relative w-10 h-5 rounded-full transition-colors ${
                showCost ? 'bg-primary' : 'bg-border'
              }`}
            >
              <span
                className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform ${
                  showCost ? 'translate-x-5' : ''
                }`}
              />
            </button>
          </div>

          <div>
            <label className="text-xs font-medium text-text-secondary block mb-1">Routing Strategy</label>
            <select
              value={route}
              onChange={(e) => setRoute(e.target.value)}
              className="w-full px-3 py-2 text-xs bg-surface border border-border rounded-lg text-text"
            >
              <option value="">Default</option>
              <option value="fallback">Fallback</option>
            </select>
          </div>

          <div className="flex items-center justify-between py-1">
            <label className="text-xs font-medium text-text-secondary">Allow Provider Fallbacks</label>
            <button
              type="button"
              onClick={() => setAllowFallbacks((v) => !v)}
              className={`relative w-10 h-5 rounded-full transition-colors ${allowFallbacks ? 'bg-primary' : 'bg-border'}`}
            >
              <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform ${allowFallbacks ? 'translate-x-5' : ''}`} />
            </button>
          </div>

          <div>
            <label className="text-xs font-medium text-text-secondary block mb-1">Preferred Providers (order)</label>
            <input
              value={providerOrder}
              onChange={(e) => setProviderOrder(e.target.value)}
              placeholder="openai, anthropic"
              className="w-full px-3 py-2 text-xs bg-surface border border-border rounded-lg text-text"
            />
          </div>

          <div>
            <label className="text-xs font-medium text-text-secondary block mb-1">Only Providers</label>
            <input
              value={providerOnly}
              onChange={(e) => setProviderOnly(e.target.value)}
              placeholder="openai, mistral"
              className="w-full px-3 py-2 text-xs bg-surface border border-border rounded-lg text-text"
            />
          </div>

          <div>
            <label className="text-xs font-medium text-text-secondary block mb-1">Ignore Providers</label>
            <input
              value={providerIgnore}
              onChange={(e) => setProviderIgnore(e.target.value)}
              placeholder="google, together"
              className="w-full px-3 py-2 text-xs bg-surface border border-border rounded-lg text-text"
            />
          </div>

          <div>
            <label className="text-xs font-medium text-text-secondary block mb-1">Model Fallbacks</label>
            <input
              value={modelFallbacks}
              onChange={(e) => setModelFallbacks(e.target.value)}
              placeholder="openai/gpt-5.4-mini, anthropic/claude-sonnet-4.6"
              className="w-full px-3 py-2 text-xs bg-surface border border-border rounded-lg text-text"
            />
          </div>

          <div>
            <label className="text-xs font-medium text-text-secondary block mb-1">OpenRouter Plugins</label>
            <div className="grid grid-cols-2 gap-2">
              {(['web', 'file-parser', 'response-healing', 'context-compression'] as const).map((pluginId) => (
                <label key={pluginId} className="flex items-center gap-2 text-xs text-text-secondary">
                  <input
                    type="checkbox"
                    checked={plugins[pluginId]}
                    onChange={(e) => setPlugins((prev) => ({ ...prev, [pluginId]: e.target.checked }))}
                  />
                  <span>{pluginId}</span>
                </label>
              ))}
            </div>
          </div>

          <button
            type="button"
            onClick={handleSave}
            disabled={saving}
            className="w-full mt-1 px-3 py-2 text-xs rounded-lg bg-primary text-white disabled:opacity-50"
          >
            {saving ? 'Saving…' : 'Save OpenRouter Settings'}
          </button>
        </div>
      )}
    </div>
  );
}

function InputField({
  label,
  value,
  onChange,
  placeholder,
  secret,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  secret?: boolean;
}) {
  const [show, setShow] = useState(false);

  return (
    <div>
      <label className="block text-xs text-text-muted mb-1.5 font-medium">{label}</label>
      <div className="relative">
        <input
          type={secret && !show ? 'password' : 'text'}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          className="w-full min-w-0 px-3 py-2.5 text-sm bg-surface border border-border rounded-xl
                     text-text placeholder-text-muted focus:outline-none
                     transition-all pr-9 input-glow"
        />
        {secret && (
          <button
            type="button"
            onClick={() => setShow(!show)}
            className="absolute right-2.5 top-1/2 -translate-y-1/2 p-1 rounded-md
                       text-text-muted hover:text-text transition-colors"
            aria-label={show ? `Hide ${label}` : `Show ${label}`}
            title={show ? 'Hide value' : 'Show value'}
          >
            {show ? <EyeOff size={14} /> : <Eye size={14} />}
          </button>
        )}
      </div>
    </div>
  );
}

// ============================================
// Appearance Tab
// ============================================

function AppearanceTab() {
  const { currentThemeId, setTheme } = useTheme();

  return (
    <div className="space-y-6">
      <div className="p-5 rounded-2xl bg-surface-alt border border-border">
        <div className="flex items-center gap-3 mb-4">
          <div className="w-10 h-10 rounded-2xl bg-gradient-to-br from-violet-500/20 to-fuchsia-500/20 flex items-center justify-center shadow-md shadow-violet-500/10">
            <Palette size={18} className="text-violet-400" />
          </div>
          <div>
            <h3 className="text-sm font-bold">Color Theme</h3>
            <p className="text-[11px] text-text-muted">Choose a color theme — changes apply instantly</p>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          {THEMES.map((theme) => {
            const isActive = theme.id === currentThemeId;
            const t = theme.tokens;
            return (
              <button
                key={theme.id}
                onClick={() => setTheme(theme.id)}
                className={clsx(
                  'relative p-4 rounded-xl border-2 text-left transition-all duration-200',
                  isActive
                    ? 'border-primary ring-2 ring-primary/30 shadow-lg shadow-primary/10'
                    : 'border-border hover:border-border-focus/50 hover:shadow-md'
                )}
                style={{ background: t.surfaceAlt }}
              >
                {isActive && (
                  <div className="absolute top-2 right-2 w-5 h-5 rounded-full flex items-center justify-center" style={{ background: t.primary }}>
                    <Check size={12} className="text-white" />
                  </div>
                )}
                {/* Color swatches */}
                <div className="flex gap-1.5 mb-3">
                  <span className="w-5 h-5 rounded-full border border-white/10" style={{ background: t.surface }} />
                  <span className="w-5 h-5 rounded-full border border-white/10" style={{ background: t.primary }} />
                  <span className="w-5 h-5 rounded-full border border-white/10" style={{ background: t.accent }} />
                  <span className="w-5 h-5 rounded-full border border-white/10" style={{ background: t.text }} />
                </div>
                <div className="text-sm font-semibold" style={{ color: t.text }}>{theme.name}</div>
                <div className="text-[10px] mt-0.5" style={{ color: t.textMuted }}>
                  {theme.isDark ? 'Dark' : 'Light'} theme{t.fontFamily ? ' · Monospace' : ''}
                </div>
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}

// ============================================
// General Tab
// ============================================

function GeneralTab() {
  const { settings, updateSettings } = useSettingsStore();
  const [webSearchProvider, setWebSearchProvider] = useState(
    settings.web_search_provider || 'auto'
  );
  const [braveApiKey, setBraveApiKey] = useState(settings.brave_api_key || '');
  const [showBraveKey, setShowBraveKey] = useState(false);
  const [jinaApiKey, setJinaApiKey] = useState(settings.jina_api_key || '');
  const [showJinaKey, setShowJinaKey] = useState(false);
  const [savingSearch, setSavingSearch] = useState(false);
  const [jinaReaderEnabled, setJinaReaderEnabled] = useState(settings.jina_reader_enabled);
  const [jinaMaxLen, setJinaMaxLen] = useState(settings.jina_reader_max_len || 10000);
  const [appVersion, setAppVersion] = useState('...');

  useEffect(() => {
    setWebSearchProvider(settings.web_search_provider || 'auto');
    setBraveApiKey(settings.brave_api_key || '');
    setJinaApiKey(settings.jina_api_key || '');
    setJinaReaderEnabled(settings.jina_reader_enabled);
    setJinaMaxLen(settings.jina_reader_max_len || 10000);
  }, [settings]);

  // Fetch version from backend
  useEffect(() => {
    api.version()
      .then((data) => setAppVersion(data.version || '?'))
      .catch(() => setAppVersion('?'));
  }, []);

  const saveSearchSettings = async () => {
    setSavingSearch(true);
    try {
      await updateSettings({
        web_search_provider: webSearchProvider === 'auto' ? '' : webSearchProvider,
        brave_api_key: braveApiKey,
        jina_api_key: jinaApiKey,
        jina_reader_enabled: jinaReaderEnabled,
        jina_reader_max_len: jinaMaxLen,
      });
      toast.success('Web search settings saved.');
    } catch {
      toast.error('Failed to save search settings');
    } finally {
      setSavingSearch(false);
    }
  };

  return (
    <div className="space-y-6">
      {/* Web Search settings */}
      <div className="p-5 rounded-2xl bg-surface-alt border border-border">
        <div className="flex items-center gap-3 mb-4">
          <div className="w-10 h-10 rounded-2xl bg-gradient-to-br from-blue-500/20 to-cyan-500/20 flex items-center justify-center shadow-md shadow-blue-500/10">
            <Globe size={18} className="text-blue-400" />
          </div>
          <div>
            <h3 className="text-sm font-bold">Web Search</h3>
            <p className="text-[11px] text-text-muted">Configure how the app searches the web for current information</p>
          </div>
        </div>

        <div className="space-y-4">
          {/* Provider selection */}
          <div>
            <label className="text-xs font-medium text-text-secondary mb-1.5 block">Search Provider</label>
            <select
              value={webSearchProvider}
              onChange={(e) => setWebSearchProvider(e.target.value)}
              className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl
                         text-text focus:outline-none focus:border-primary/50 transition-colors"
            >
              <option value="auto">Auto (Brave if key set, else DuckDuckGo)</option>
              <option value="brave">Brave Search (API key required)</option>
              <option value="ddg">DuckDuckGo (no key needed)</option>
              <option value="none">Disabled</option>
            </select>
          </div>

          {/* Brave API key */}
          {(webSearchProvider === 'auto' || webSearchProvider === 'brave') && (
            <div>
              <label className="text-xs font-medium text-text-secondary mb-1.5 block">
                Brave Search API Key
                <a
                  href="https://brave.com/search/api/"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="ml-2 text-primary hover:underline inline-flex items-center gap-0.5"
                >
                  Get free key <ExternalLink size={9} />
                </a>
              </label>
              <div className="relative">
                <input
                  type={showBraveKey ? 'text' : 'password'}
                  value={braveApiKey}
                  onChange={(e) => setBraveApiKey(e.target.value)}
                  placeholder="BSA..."
                  className="w-full px-3 py-2 pr-10 text-sm bg-surface border border-border rounded-xl
                             text-text focus:outline-none focus:border-primary/50 transition-colors font-mono"
                />
                <button
                  type="button"
                  onClick={() => setShowBraveKey(!showBraveKey)}
                  className="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-text-muted hover:text-text transition-colors"
                >
                  {showBraveKey ? <EyeOff size={14} /> : <Eye size={14} />}
                </button>
              </div>
              <p className="text-[10px] text-text-muted mt-1">
                Free tier: 2,000 queries/month. Leave empty to use DuckDuckGo.
              </p>
            </div>
          )}

          {webSearchProvider === 'none' && (
            <p className="text-xs text-text-muted bg-surface rounded-xl px-3 py-2 border border-border/50">
              Web search is disabled. The app will not search for current information.
            </p>
          )}

          {webSearchProvider === 'ddg' && (
            <p className="text-xs text-text-muted bg-surface rounded-xl px-3 py-2 border border-border/50">
              DuckDuckGo requires no API key and works out of the box. For better news results, consider adding a Brave Search API key.
            </p>
          )}

          {/* Jina Reader toggle */}
          {webSearchProvider !== 'none' && (
            <>
              <div className="flex items-center justify-between py-2">
                <div>
                  <label className="text-xs font-medium text-text-secondary block">Jina Reader</label>
                  <p className="text-[10px] text-text-muted mt-0.5">
                    Fetches full page content for richer answers. Add an API key below to bypass anti-bot limits.
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => setJinaReaderEnabled(!jinaReaderEnabled)}
                  className={`relative w-10 h-5 rounded-full transition-colors ${
                    jinaReaderEnabled ? 'bg-primary' : 'bg-border'
                  }`}
                >
                  <span
                    className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform ${
                      jinaReaderEnabled ? 'translate-x-5' : ''
                    }`}
                  />
                </button>
              </div>
              {jinaReaderEnabled && (
                <div className="mt-2 space-y-3">
                  <div className="space-y-1">
                    <label className="text-[10px] font-medium text-text-muted uppercase tracking-wider block">
                      API Key (Optional, bypasses rate limits)
                    </label>
                    <div className="relative">
                      <input
                        type={showJinaKey ? 'text' : 'password'}
                        value={jinaApiKey}
                        onChange={(e) => setJinaApiKey(e.target.value)}
                        placeholder="jina_..."
                        className="w-full px-3 py-2 pr-10 text-sm bg-surface border border-border rounded-xl
                               text-text focus:outline-none focus:border-primary/50 transition-colors font-mono"
                      />
                      <button
                        type="button"
                        onClick={() => setShowJinaKey(!showJinaKey)}
                        className="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-text-muted hover:text-text transition-colors"
                      >
                        {showJinaKey ? <EyeOff size={14} /> : <Eye size={14} />}
                      </button>
                    </div>
                  </div>
                  <div className="space-y-1">
                    <label className="text-[10px] font-medium text-text-muted uppercase tracking-wider block">
                      Max chars per page
                    </label>
                    <input
                      type="number"
                      min={1000}
                      max={50000}
                      step={1000}
                      value={jinaMaxLen}
                      onChange={(e) => setJinaMaxLen(Number(e.target.value))}
                      className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl
                             text-text focus:outline-none focus:border-primary/50 transition-colors"
                    />
                    <p className="text-[10px] text-text-muted">
                      Characters of page content sent to the LLM per source. Higher = richer data but slower. Default: 10000.
                    </p>
                  </div>
                </div>
              )}
            </>
          )}

          <motion.button
            whileHover={{ scale: 1.02 }}
            whileTap={{ scale: 0.98 }}
            onClick={saveSearchSettings}
            disabled={savingSearch}
            className="flex items-center gap-2 px-4 py-2 rounded-xl btn-primary text-sm font-medium
                       disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {savingSearch ? <RefreshCw size={14} className="animate-spin" /> : <Save size={14} />}
            Save Search Settings
          </motion.button>
        </div>
      </div>

      {/* App info */}
      <div className="p-5 rounded-2xl bg-surface-alt border border-border">
        <div className="flex items-center gap-3 mb-4">
          <div className="w-10 h-10 rounded-2xl bg-gradient-to-br from-primary to-accent flex items-center justify-center shadow-md shadow-primary/20">
            <Zap size={18} className="text-white" />
          </div>
          <div>
            <h3 className="text-sm font-bold">OmniLLM-Studio</h3>
            <p className="text-[11px] text-text-muted">Your unified AI chat interface</p>
          </div>
        </div>
        <div className="space-y-2">
          <InfoRow label="Version" value={appVersion} />
          <InfoRow label="Backend" value="Go + SQLite" />
          <InfoRow label="Frontend" value="React + TypeScript" />
          <InfoRow label="Data Storage" value="100% Local" />
        </div>
      </div>

      {/* Keyboard shortcuts */}
      <div className="p-5 rounded-2xl bg-surface-alt border border-border">
        <h3 className="text-sm font-semibold mb-3">Keyboard Shortcuts</h3>
        <div className="space-y-2">
          <ShortcutRow keys={['Ctrl', 'N']} desc="New conversation" />
          <ShortcutRow keys={['Ctrl', ',']} desc="Open settings" />
          <ShortcutRow keys={['Enter']} desc="Send message" />
          <ShortcutRow keys={['Shift', 'Enter']} desc="New line in message" />
        </div>
      </div>

      <p className="text-xs text-text-muted/40 text-center">
        More settings coming soon
      </p>
    </div>
  );
}

// ============================================
// Routing / Intent Model Settings Tab
// ============================================

const ROUTER_DEFAULTS = {
  mode: 'sports_only',
  structuredOutputMode: 'auto',
  confidenceThreshold: 0.75,
  fallbackBehavior: 'local_detector',
  timeoutMS: 8000,
  maxTokens: 600,
  temperature: 0,
  cacheEnabled: true,
} as const;

const CHAT_CAPABLE_PROVIDER_TYPES = new Set(['openai', 'anthropic', 'ollama', 'openrouter', 'groq', 'together', 'mistral', 'gemini']);

function RoutingTab() {
  const { settings, updateSettings } = useSettingsStore();
  const { providers } = useProviderStore();
  const chatProviders = providers.filter((provider) => provider.enabled && CHAT_CAPABLE_PROVIDER_TYPES.has(provider.type));
  const [routerEnabled, setRouterEnabled] = useState(settings.router_enabled ?? false);
  const [mode, setMode] = useState(settings.router_mode || ROUTER_DEFAULTS.mode);
  const [provider, setProvider] = useState(settings.router_provider || '');
  const [model, setModel] = useState(settings.router_model || '');
  const [structuredMode, setStructuredMode] = useState(settings.router_structured_output_mode || ROUTER_DEFAULTS.structuredOutputMode);
  const [confidence, setConfidence] = useState(settings.router_confidence_threshold ?? ROUTER_DEFAULTS.confidenceThreshold);
  const [fallback, setFallback] = useState(settings.router_fallback_behavior || ROUTER_DEFAULTS.fallbackBehavior);
  const [timeoutMS, setTimeoutMS] = useState(settings.router_timeout_ms ?? ROUTER_DEFAULTS.timeoutMS);
  const [maxTokens, setMaxTokens] = useState(settings.router_max_tokens ?? ROUTER_DEFAULTS.maxTokens);
  const [temperature, setTemperature] = useState(settings.router_temperature ?? ROUTER_DEFAULTS.temperature);
  const [showTrace, setShowTrace] = useState(settings.router_show_trace ?? false);
  const [cacheEnabled, setCacheEnabled] = useState(settings.router_cache_enabled ?? ROUTER_DEFAULTS.cacheEnabled);
  const [suggestions, setSuggestions] = useState<import('../types').RouterModelSuggestion[]>([]);
  const [notes, setNotes] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setRouterEnabled(settings.router_enabled ?? false);
    setMode(settings.router_mode || ROUTER_DEFAULTS.mode);
    setProvider(settings.router_provider || '');
    setModel(settings.router_model || '');
    setStructuredMode(settings.router_structured_output_mode || ROUTER_DEFAULTS.structuredOutputMode);
    setConfidence(settings.router_confidence_threshold ?? ROUTER_DEFAULTS.confidenceThreshold);
    setFallback(settings.router_fallback_behavior || ROUTER_DEFAULTS.fallbackBehavior);
    setTimeoutMS(settings.router_timeout_ms ?? ROUTER_DEFAULTS.timeoutMS);
    setMaxTokens(settings.router_max_tokens ?? ROUTER_DEFAULTS.maxTokens);
    setTemperature(settings.router_temperature ?? ROUTER_DEFAULTS.temperature);
    setShowTrace(settings.router_show_trace ?? false);
    setCacheEnabled(settings.router_cache_enabled ?? ROUTER_DEFAULTS.cacheEnabled);
  }, [settings]);

  useEffect(() => {
    if (!provider) {
      setSuggestions([]);
      setNotes([]);
      return;
    }
    api.getRouterSuggestions(provider)
      .then((resp) => {
        setSuggestions(resp.suggestions || []);
        setNotes(resp.notes || []);
      })
      .catch(() => {
        setSuggestions([]);
        setNotes(['No curated suggestions available for this provider. Enter a chat-capable model manually.']);
      });
  }, [provider]);

  const selectedProvider = chatProviders.find((p) => p.id === provider || p.name === provider || p.type === provider);

  const saveRoutingSettings = async () => {
    setSaving(true);
    try {
      await updateSettings({
        router_enabled: routerEnabled,
        router_mode: mode,
        router_provider: provider,
        router_model: model.trim(),
        router_structured_output_mode: structuredMode,
        router_confidence_threshold: confidence,
        router_fallback_behavior: fallback,
        router_timeout_ms: timeoutMS,
        router_max_tokens: maxTokens,
        router_temperature: temperature,
        router_show_trace: showTrace,
        router_cache_enabled: cacheEnabled,
      });
      toast.success('Routing settings saved');
    } catch {
      toast.error('Failed to save routing settings');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="p-5 rounded-2xl bg-surface-alt border border-border">
        <div className="flex items-center gap-3 mb-4">
          <div className="w-10 h-10 rounded-2xl bg-gradient-to-br from-sky-500/20 to-emerald-500/20 flex items-center justify-center shadow-md shadow-sky-500/10">
            <RouteIcon size={18} className="text-sky-300" />
          </div>
          <div>
            <h3 className="text-sm font-bold">Routing / Intent Model</h3>
            <p className="text-[11px] text-text-muted">Classify requests and route tool lookups before the main model runs</p>
          </div>
        </div>

        <div className="space-y-4">
          <div className="flex items-center justify-between gap-4 py-2">
            <div>
              <label className="text-xs font-medium text-text-secondary block">Enable Router Model</label>
              <p className="text-[10px] text-text-muted mt-0.5">Uses a small, fast model for structured routing. Existing sports detection remains as fallback.</p>
            </div>
            <button type="button" onClick={() => setRouterEnabled(!routerEnabled)} className={`relative w-10 h-5 rounded-full transition-colors ${routerEnabled ? 'bg-primary' : 'bg-border'}`} aria-label="Enable router model">
              <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform ${routerEnabled ? 'translate-x-5' : ''}`} />
            </button>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <label className="block">
              <span className="text-xs font-medium text-text-secondary mb-1.5 block">Router Mode</span>
              <select value={mode} onChange={(e) => setMode(e.target.value)} className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text focus:outline-none focus:border-primary/50">
                <option value="off">Off</option>
                <option value="sports_only">Sports only</option>
                <option value="tools_only">Tools only</option>
                <option value="all_preflight">All preflight routes</option>
              </select>
            </label>
            <label className="block">
              <span className="text-xs font-medium text-text-secondary mb-1.5 block">Fallback Behavior</span>
              <select value={fallback} onChange={(e) => setFallback(e.target.value)} className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text focus:outline-none focus:border-primary/50">
                <option value="local_detector">Local detector</option>
                <option value="normal_llm">Normal LLM</option>
                <option value="main_model">Main model</option>
                <option value="clarify">Clarify</option>
                <option value="fail_closed">Fail closed</option>
              </select>
            </label>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <label className="block">
              <span className="text-xs font-medium text-text-secondary mb-1.5 block">Router Provider</span>
              <select value={provider} onChange={(e) => setProvider(e.target.value)} className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text focus:outline-none focus:border-primary/50">
                <option value="">Choose provider</option>
                {chatProviders.map((p) => (
                  <option key={p.id} value={p.id}>{p.name} ({p.type})</option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="text-xs font-medium text-text-secondary mb-1.5 block">Router Model</span>
              <input value={model} onChange={(e) => setModel(e.target.value)} placeholder="gpt-4o-mini" className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text placeholder:text-text-muted focus:outline-none focus:border-primary/50" />
            </label>
          </div>

          {provider && (
            <div className="rounded-xl border border-border bg-surface/50 p-3">
              <p className="text-xs font-semibold text-text mb-2">Recommended routing models{selectedProvider ? ` for ${selectedProvider.name}` : ''}</p>
              {suggestions.length > 0 ? (
                <div className="flex flex-wrap gap-2">
                  {suggestions.map((suggestion) => (
                    <button key={suggestion.model} type="button" onClick={() => setModel(suggestion.model)} className="rounded-lg border border-border bg-surface px-2.5 py-1.5 text-left text-[11px] text-text-secondary hover:text-text hover:border-primary/40 transition-colors" title={suggestion.reason}>
                      {suggestion.model}
                    </button>
                  ))}
                </div>
              ) : (
                <p className="text-[11px] text-text-muted">No static suggestions for this provider type.</p>
              )}
              {notes.length > 0 && <p className="mt-2 text-[10px] text-text-muted leading-relaxed">{notes.join(' ')}</p>}
            </div>
          )}

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <label className="block">
              <span className="text-xs font-medium text-text-secondary mb-1.5 block">Structured Output</span>
              <select value={structuredMode} onChange={(e) => setStructuredMode(e.target.value)} className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text focus:outline-none focus:border-primary/50">
                <option value="auto">Auto</option>
                <option value="json_schema">JSON schema</option>
                <option value="json_object">JSON object</option>
                <option value="prompted_json">Prompted JSON</option>
              </select>
            </label>
            <label className="block">
              <span className="text-xs font-medium text-text-secondary mb-1.5 block">Confidence Threshold ({confidence.toFixed(2)})</span>
              <input type="range" min={0.5} max={0.99} step={0.01} value={confidence} onChange={(e) => setConfidence(Number(e.target.value))} className="w-full accent-primary" />
            </label>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <label className="block">
              <span className="text-xs font-medium text-text-secondary mb-1.5 block">Timeout (ms)</span>
              <input type="number" min={1000} step={500} value={timeoutMS} onChange={(e) => setTimeoutMS(Number(e.target.value))} className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text focus:outline-none focus:border-primary/50" />
            </label>
            <label className="block">
              <span className="text-xs font-medium text-text-secondary mb-1.5 block">Max Tokens</span>
              <input type="number" min={100} step={50} value={maxTokens} onChange={(e) => setMaxTokens(Number(e.target.value))} className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text focus:outline-none focus:border-primary/50" />
            </label>
            <label className="block">
              <span className="text-xs font-medium text-text-secondary mb-1.5 block">Temperature</span>
              <input type="number" min={0} max={1} step={0.1} value={temperature} onChange={(e) => setTemperature(Number(e.target.value))} className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text focus:outline-none focus:border-primary/50" />
            </label>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <RoutingToggleRow label="Show router trace" description="Emits compact router SSE metadata while streaming" enabled={showTrace} onToggle={() => setShowTrace(!showTrace)} />
            <RoutingToggleRow label="Router cache" description="Reserved for future short-lived decision caching" enabled={cacheEnabled} onToggle={() => setCacheEnabled(!cacheEnabled)} />
          </div>

          <motion.button whileHover={{ scale: 1.02 }} whileTap={{ scale: 0.98 }} onClick={saveRoutingSettings} disabled={saving} className="flex items-center gap-2 px-4 py-2 rounded-xl btn-primary text-sm font-medium disabled:opacity-50 disabled:cursor-not-allowed">
            {saving ? <RefreshCw size={14} className="animate-spin" /> : <Save size={14} />}
            Save Routing Settings
          </motion.button>
        </div>
      </div>
    </div>
  );
}

function RoutingToggleRow({ label, description, enabled, onToggle }: { label: string; description: string; enabled: boolean; onToggle: () => void }) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-xl border border-border bg-surface/50 p-3">
      <div>
        <p className="text-xs font-medium text-text-secondary">{label}</p>
        <p className="text-[10px] text-text-muted mt-0.5">{description}</p>
      </div>
      <button type="button" onClick={onToggle} className={`relative w-10 h-5 rounded-full transition-colors ${enabled ? 'bg-primary' : 'bg-border'}`}>
        <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform ${enabled ? 'translate-x-5' : ''}`} />
      </button>
    </div>
  );
}

// ============================================
// RAG Settings Tab
// ============================================

// RAG default values — kept in sync with backend `models.DefaultAppSettings`.
// "" embedding model means "Auto" (backend resolver picks canonical model per provider).
const RAG_DEFAULTS = {
  embeddingModel: '',
  chunkSize: 1000,
  chunkOverlap: 200,
  topK: 5,
} as const;

function RAGTab() {
  const { settings, updateSettings } = useSettingsStore();
  const [ragEnabled, setRagEnabled] = useState(settings.rag_enabled ?? false);
  const [embeddingModel, setEmbeddingModel] = useState(settings.rag_embedding_model ?? RAG_DEFAULTS.embeddingModel);
  const [chunkSize, setChunkSize] = useState(settings.rag_chunk_size ?? RAG_DEFAULTS.chunkSize);
  const [chunkOverlap, setChunkOverlap] = useState(settings.rag_chunk_overlap ?? RAG_DEFAULTS.chunkOverlap);
  const [topK, setTopK] = useState(settings.rag_top_k ?? RAG_DEFAULTS.topK);
  const [saving, setSaving] = useState(false);
  const [reindexing, setReindexing] = useState(false);
  const [advancedOpen, setAdvancedOpen] = useState(false);

  useEffect(() => {
    setRagEnabled(settings.rag_enabled ?? false);
    setEmbeddingModel(settings.rag_embedding_model ?? RAG_DEFAULTS.embeddingModel);
    setChunkSize(settings.rag_chunk_size ?? RAG_DEFAULTS.chunkSize);
    setChunkOverlap(settings.rag_chunk_overlap ?? RAG_DEFAULTS.chunkOverlap);
    setTopK(settings.rag_top_k ?? RAG_DEFAULTS.topK);
  }, [settings]);

  const isCustomized =
    embeddingModel !== RAG_DEFAULTS.embeddingModel ||
    chunkSize !== RAG_DEFAULTS.chunkSize ||
    chunkOverlap !== RAG_DEFAULTS.chunkOverlap ||
    topK !== RAG_DEFAULTS.topK;

  const resetAdvanced = () => {
    setEmbeddingModel(RAG_DEFAULTS.embeddingModel);
    setChunkSize(RAG_DEFAULTS.chunkSize);
    setChunkOverlap(RAG_DEFAULTS.chunkOverlap);
    setTopK(RAG_DEFAULTS.topK);
  };

  const saveRAGSettings = async () => {
    setSaving(true);
    try {
      await updateSettings({
        rag_enabled: ragEnabled,
        rag_embedding_model: embeddingModel,
        rag_chunk_size: chunkSize,
        rag_chunk_overlap: chunkOverlap,
        rag_top_k: topK,
      });
      toast.success('RAG settings saved');
    } catch {
      toast.error('Failed to save RAG settings');
    } finally {
      setSaving(false);
    }
  };

  const handleReindexAll = async () => {
    if (!window.confirm(
      'Drop every conversation\'s vector index? Each conversation will lazy-rebuild from existing chunk text on its next query. This is safe but may make the first retrieval per conversation a bit slower.'
    )) {
      return;
    }
    setReindexing(true);
    try {
      const result = await api.reindexAll();
      toast.success(`Reset ${result.conversations_dropped} conversation${result.conversations_dropped === 1 ? '' : 's'}`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to reindex');
    } finally {
      setReindexing(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="p-5 rounded-2xl bg-surface-alt border border-border">
        <div className="flex items-center gap-3 mb-4">
          <div className="w-10 h-10 rounded-2xl bg-gradient-to-br from-emerald-500/20 to-green-500/20 flex items-center justify-center shadow-md shadow-emerald-500/10">
            <Database size={18} className="text-emerald-400" />
          </div>
          <div>
            <h3 className="text-sm font-bold">Retrieval-Augmented Generation</h3>
            <p className="text-[11px] text-text-muted">Enhance responses with context from your uploaded documents</p>
          </div>
        </div>

        <div className="space-y-4">
          {/* RAG toggle */}
          <div className="flex items-center justify-between py-2">
            <div>
              <label className="text-xs font-medium text-text-secondary block">Enable RAG</label>
              <p className="text-[10px] text-text-muted mt-0.5">Automatically index attachments and use them as context</p>
            </div>
            <button
              type="button"
              onClick={() => setRagEnabled(!ragEnabled)}
              className={`relative w-10 h-5 rounded-full transition-colors ${
                ragEnabled ? 'bg-primary' : 'bg-border'
              }`}
            >
              <span
                className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform ${
                  ragEnabled ? 'translate-x-5' : ''
                }`}
              />
            </button>
          </div>

          {ragEnabled && (
            <div className="rounded-xl border border-border bg-surface/50 overflow-hidden">
              <button
                type="button"
                onClick={() => setAdvancedOpen((v) => !v)}
                className="w-full flex items-center justify-between px-3 py-2.5 text-xs font-medium
                           text-text-secondary hover:text-text hover:bg-surface transition-colors"
              >
                <span className="flex items-center gap-2">
                  Advanced
                  {isCustomized && (
                    <span className="text-[9px] uppercase tracking-wide px-1.5 py-0.5 rounded
                                     bg-primary/20 text-primary font-semibold">
                      Customized
                    </span>
                  )}
                </span>
                <ChevronDown
                  size={14}
                  className={`transition-transform ${advancedOpen ? 'rotate-180' : ''}`}
                />
              </button>

              {advancedOpen && (
                <div className="px-3 pb-4 pt-1 space-y-4 border-t border-border">
                  <p className="text-[11px] text-text-muted leading-relaxed pt-2">
                    Sensible defaults work for most users. These knobs are useful if you want to
                    tune retrieval quality vs. token cost or pin a specific embedding model.
                  </p>

                  {/* Embedding model */}
                  <div>
                    <label className="text-xs font-medium text-text-secondary mb-1.5 block">Embedding Model</label>
                    <select
                      value={embeddingModel}
                      onChange={(e) => setEmbeddingModel(e.target.value)}
                      className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl
                                 text-text focus:outline-none focus:border-primary/50 transition-colors"
                    >
                      <option value="">Auto (recommended) — picks the canonical model per provider</option>
                      <optgroup label="OpenAI">
                        <option value="text-embedding-3-small">text-embedding-3-small</option>
                        <option value="text-embedding-3-large">text-embedding-3-large</option>
                        <option value="text-embedding-ada-002">text-embedding-ada-002</option>
                      </optgroup>
                      <optgroup label="Mistral">
                        <option value="mistral-embed">mistral-embed</option>
                      </optgroup>
                      <optgroup label="Together">
                        <option value="togethercomputer/m2-bert-80M-8k-base">m2-bert-80M-8k-base</option>
                        <option value="WhereIsAI/UAE-Large-V1">UAE-Large-V1</option>
                      </optgroup>
                      <optgroup label="Gemini">
                        <option value="text-embedding-004">text-embedding-004</option>
                      </optgroup>
                      <optgroup label="Ollama (Local)">
                        <option value="nomic-embed-text">nomic-embed-text</option>
                        <option value="mxbai-embed-large">mxbai-embed-large</option>
                        <option value="bge-m3">bge-m3</option>
                        <option value="all-minilm">all-minilm</option>
                      </optgroup>
                      <optgroup label="OpenRouter">
                        <option value="openai/text-embedding-3-small">openai/text-embedding-3-small</option>
                        <option value="openai/text-embedding-3-large">openai/text-embedding-3-large</option>
                      </optgroup>
                    </select>
                    <p className="text-[10px] text-text-muted mt-1.5">
                      Embeddings auto-route to the first enabled provider that supports them
                      (OpenAI → Mistral → Together → Ollama → Gemini). Pin a specific model only
                      if you have a reason to — changing it after indexing requires a reindex.
                    </p>
                  </div>

                  {/* Chunk size */}
                  <div>
                    <label className="text-xs font-medium text-text-secondary mb-1.5 block">
                      Chunk Size <span className="text-text-muted font-normal">({chunkSize} chars)</span>
                    </label>
                    <input
                      type="range"
                      min={128}
                      max={2048}
                      step={64}
                      value={chunkSize}
                      onChange={(e) => setChunkSize(Number(e.target.value))}
                      className="w-full accent-primary"
                    />
                    <div className="flex justify-between text-[10px] text-text-muted mt-0.5">
                      <span>128</span>
                      <span>2048</span>
                    </div>
                    <p className="text-[10px] text-text-muted mt-1">
                      Smaller for Q&amp;A; larger for code or structured docs. Changing this requires a reindex.
                    </p>
                  </div>

                  {/* Chunk overlap */}
                  <div>
                    <label className="text-xs font-medium text-text-secondary mb-1.5 block">
                      Chunk Overlap <span className="text-text-muted font-normal">({chunkOverlap} chars)</span>
                    </label>
                    <input
                      type="range"
                      min={0}
                      max={500}
                      step={20}
                      value={chunkOverlap}
                      onChange={(e) => setChunkOverlap(Number(e.target.value))}
                      className="w-full accent-primary"
                    />
                    <div className="flex justify-between text-[10px] text-text-muted mt-0.5">
                      <span>0</span>
                      <span>500</span>
                    </div>
                  </div>

                  {/* Top-K */}
                  <div>
                    <label className="text-xs font-medium text-text-secondary mb-1.5 block">
                      Top-K Results <span className="text-text-muted font-normal">({topK})</span>
                    </label>
                    <input
                      type="range"
                      min={1}
                      max={20}
                      step={1}
                      value={topK}
                      onChange={(e) => setTopK(Number(e.target.value))}
                      className="w-full accent-primary"
                    />
                    <div className="flex justify-between text-[10px] text-text-muted mt-0.5">
                      <span>1</span>
                      <span>20</span>
                    </div>
                    <p className="text-[10px] text-text-muted mt-1">
                      How many chunks to inject per query. More context = more tokens. No reindex required.
                    </p>
                  </div>

                  {isCustomized && (
                    <button
                      type="button"
                      onClick={resetAdvanced}
                      className="flex items-center gap-1.5 text-[11px] text-text-muted hover:text-text transition-colors"
                    >
                      <RotateCcw size={12} />
                      Reset to defaults
                    </button>
                  )}
                </div>
              )}
            </div>
          )}

          <motion.button
            whileHover={{ scale: 1.02 }}
            whileTap={{ scale: 0.98 }}
            onClick={saveRAGSettings}
            disabled={saving}
            className="flex items-center gap-2 px-4 py-2 rounded-xl btn-primary text-sm font-medium
                       disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {saving ? <RefreshCw size={14} className="animate-spin" /> : <Save size={14} />}
            Save RAG Settings
          </motion.button>
        </div>
      </div>

      {/* Maintenance — admin only */}
      <div className="p-5 rounded-2xl bg-surface-alt border border-border">
        <div className="flex items-center gap-3 mb-3">
          <div className="w-10 h-10 rounded-2xl bg-gradient-to-br from-amber-500/20 to-orange-500/20 flex items-center justify-center shadow-md shadow-amber-500/10">
            <RefreshCw size={18} className="text-amber-400" />
          </div>
          <div>
            <h3 className="text-sm font-bold">Maintenance</h3>
            <p className="text-[11px] text-text-muted">Vector store cleanup &amp; migration</p>
          </div>
        </div>

        <p className="text-[11px] text-text-muted mb-3 leading-relaxed">
          Reset every conversation's vector index. Each conversation lazy-rebuilds from existing
          chunks on its next query — no re-embedding network call required for documents that
          were indexed before the chromem-go upgrade.
        </p>

        <motion.button
          whileHover={{ scale: 1.02 }}
          whileTap={{ scale: 0.98 }}
          onClick={handleReindexAll}
          disabled={reindexing}
          className="flex items-center gap-2 px-4 py-2 rounded-xl bg-amber-500/15 hover:bg-amber-500/25
                     border border-amber-500/30 text-amber-300 text-sm font-medium
                     disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {reindexing ? <RefreshCw size={14} className="animate-spin" /> : <RefreshCw size={14} />}
          Reindex All Documents
        </motion.button>
      </div>
    </div>
  );
}

// ============================================
// Music Studio Settings Tab
// ============================================

function MusicTab() {
  const { settings, updateSettings } = useSettingsStore();
  const { isEnabled, updateFeature } = useFeatureFlagStore();
  const [enabled, setEnabled] = useState(isEnabled('music_studio'));
  const [providers, setProviders] = useState({ openrouter: false, gemini: false, elevenlabs: false });
  const [defaultProvider, setDefaultProvider] = useState<MusicProviderKey>(
    (settings.default_music_provider as MusicProviderKey) || 'openrouter'
  );
  const [openRouterModel, setOpenRouterModel] = useState(settings.default_music_model_openrouter || 'google/lyria-3-clip-preview');
  const [geminiModel, setGeminiModel] = useState(settings.default_music_model_gemini || 'lyria-3-clip-preview');
  const [elevenlabsModel, setElevenlabsModel] = useState(settings.default_music_model_elevenlabs || 'music_v1');
  const [customGeminiModel, setCustomGeminiModel] = useState(settings.custom_gemini_lyria_model || '');
  const [autoEnhance, setAutoEnhance] = useState(settings.auto_enhance_music_prompts ?? false);
  const [saveMetadata, setSaveMetadata] = useState(settings.save_music_generation_metadata ?? true);
  const [models, setModels] = useState<Record<MusicProviderKey, MusicModel[]>>({ openrouter: [], gemini: [], elevenlabs: [] });
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  const loadMusicSettings = useCallback(async () => {
    setLoading(true);
    try {
      const providerStatus = await musicApi.providers();
      const [openrouterModels, geminiModels, elevenlabsModels] = await Promise.all([
        musicApi.listModels('openrouter').catch(() => []),
        musicApi.listModels('gemini').catch(() => []),
        musicApi.listModels('elevenlabs').catch(() => []),
      ]);
      setProviders(providerStatus);
      setModels({ openrouter: openrouterModels, gemini: geminiModels, elevenlabs: elevenlabsModels });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    setEnabled(isEnabled('music_studio'));
  }, [isEnabled]);

  useEffect(() => {
    setDefaultProvider((settings.default_music_provider as MusicProviderKey) || 'openrouter');
    setOpenRouterModel(settings.default_music_model_openrouter || 'google/lyria-3-clip-preview');
    setGeminiModel(settings.default_music_model_gemini || 'lyria-3-clip-preview');
    setElevenlabsModel(settings.default_music_model_elevenlabs || 'music_v1');
    setCustomGeminiModel(settings.custom_gemini_lyria_model || '');
    setAutoEnhance(settings.auto_enhance_music_prompts ?? false);
    setSaveMetadata(settings.save_music_generation_metadata ?? true);
  }, [settings]);

  useEffect(() => {
    loadMusicSettings();
  }, [loadMusicSettings]);

  const refreshModels = async () => {
    setLoading(true);
    try {
      const [openrouterModels, geminiModels, elevenlabsModels] = await Promise.all([
        musicApi.refreshModels('openrouter').catch(() => []),
        musicApi.refreshModels('gemini').catch(() => []),
        musicApi.refreshModels('elevenlabs').catch(() => []),
      ]);
      setModels({ openrouter: openrouterModels, gemini: geminiModels, elevenlabs: elevenlabsModels });
      toast.success('Music model lists refreshed');
    } finally {
      setLoading(false);
    }
  };

  const saveMusicSettings = async () => {
    if (customGeminiModel.trim() && !customGeminiModel.trim().startsWith('lyria-')) {
      toast.error('Custom Gemini Lyria model must start with lyria-');
      return;
    }
    setSaving(true);
    try {
      await updateFeature('music_studio', enabled);
      await updateSettings({
        default_music_provider: defaultProvider,
        default_music_model_openrouter: openRouterModel,
        default_music_model_gemini: geminiModel,
        default_music_model_elevenlabs: elevenlabsModel,
        custom_gemini_lyria_model: customGeminiModel.trim(),
        auto_enhance_music_prompts: autoEnhance,
        save_music_generation_metadata: saveMetadata,
      });
      toast.success('Music Studio settings saved');
    } catch {
      toast.error('Failed to save Music Studio settings');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="p-5 rounded-2xl bg-surface-alt border border-border">
        <div className="mb-4 flex items-center gap-3">
          <div className="w-10 h-10 rounded-2xl bg-gradient-to-br from-fuchsia-500/20 to-indigo-500/20 flex items-center justify-center shadow-md shadow-fuchsia-500/10">
            <Music2 size={18} className="text-fuchsia-300" />
          </div>
          <div>
            <h3 className="text-sm font-bold">Music Studio</h3>
            <p className="text-[11px] text-text-muted">Configure music generation across OpenRouter, Gemini, and ElevenLabs</p>
          </div>
        </div>

        <div className="space-y-4">
          <div className="flex items-center justify-between py-2">
            <div>
              <label className="text-xs font-medium text-text-secondary block">Enable Music Studio</label>
              <p className="text-[10px] text-text-muted mt-0.5">Shows Music Studio in the app sidebar</p>
            </div>
            <button
              type="button"
              onClick={() => setEnabled((value) => !value)}
              className={`relative w-10 h-5 rounded-full transition-colors ${enabled ? 'bg-primary' : 'bg-border'}`}
            >
              <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform ${enabled ? 'translate-x-5' : ''}`} />
            </button>
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div>
              <label className="text-xs font-medium text-text-secondary mb-1.5 block">Default provider</label>
              <select
                value={defaultProvider}
                onChange={(event) => setDefaultProvider(event.target.value as MusicProviderKey)}
                className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text focus:outline-none focus:border-primary/50"
              >
                <option value="openrouter">OpenRouter {providers.openrouter ? '' : '(not configured)'}</option>
                <option value="gemini">Gemini {providers.gemini ? '' : '(not configured)'}</option>
                <option value="elevenlabs">ElevenLabs {providers.elevenlabs ? '' : '(not configured)'}</option>
              </select>
            </div>
            <div>
              <label className="text-xs font-medium text-text-secondary mb-1.5 block">Output directory</label>
              <div className="min-h-10 rounded-xl border border-border bg-surface px-3 py-2 text-xs text-text-muted">
                {settings.music_output_directory || 'OMNILLM_ATTACHMENTS_DIR/music'}
              </div>
            </div>
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <ModelSelect
              label="OpenRouter Lyria model"
              value={openRouterModel}
              models={models.openrouter}
              fallback="google/lyria-3-clip-preview"
              onChange={setOpenRouterModel}
            />
            <ModelSelect
              label="Gemini Lyria model"
              value={geminiModel}
              models={models.gemini}
              fallback="lyria-3-clip-preview"
              onChange={setGeminiModel}
            />
            <ModelSelect
              label="ElevenLabs music model"
              value={elevenlabsModel}
              models={models.elevenlabs}
              fallback="music_v1"
              onChange={setElevenlabsModel}
            />
          </div>

          <div>
            <label className="text-xs font-medium text-text-secondary mb-1.5 block">Custom Gemini Lyria model override</label>
            <input
              value={customGeminiModel}
              onChange={(event) => setCustomGeminiModel(event.target.value)}
              placeholder="lyria-..."
              className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text placeholder:text-text-muted/50 focus:outline-none focus:border-primary/50"
            />
          </div>

          <div className="space-y-2 rounded-xl border border-border bg-surface p-3">
            <ToggleRow label="Auto-enhance simple prompts before generation" checked={autoEnhance} onChange={setAutoEnhance} />
            <ToggleRow label="Save prompt and response metadata with assets" checked={saveMetadata} onChange={setSaveMetadata} />
          </div>

          <div className="flex flex-col gap-2 sm:flex-row">
            <motion.button
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
              onClick={refreshModels}
              disabled={loading}
              className="min-h-10 flex items-center justify-center gap-2 px-4 rounded-xl border border-border bg-surface text-sm text-text-secondary hover:text-text hover:bg-surface-hover disabled:opacity-50"
            >
              <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
              Refresh Music Models
            </motion.button>
            <motion.button
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
              onClick={saveMusicSettings}
              disabled={saving}
              className="min-h-10 flex items-center justify-center gap-2 px-4 rounded-xl btn-primary text-sm font-medium disabled:opacity-50"
            >
              {saving ? <RefreshCw size={14} className="animate-spin" /> : <Save size={14} />}
              Save Music Settings
            </motion.button>
          </div>
        </div>
      </div>
    </div>
  );
}

// ============================================
// Video Studio Settings Tab
// ============================================

function VideoTab() {
  const { isEnabled, updateFeature } = useFeatureFlagStore();
  const [enabled, setEnabled] = useState(isEnabled('video_studio'));
  const [providers, setProviders] = useState<Array<{ key: VideoProviderKey; display_name: string; configured: boolean; mock: boolean }>>([]);
  const [provider, setProvider] = useState<VideoProviderKey>('mock');
  const [models, setModels] = useState<VideoModel[]>([]);
  const [model, setModel] = useState('mock-video-v1');
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  const loadVideoSettings = useCallback(async () => {
    setLoading(true);
    try {
      const providerStatus = await videoApi.providers();
      setProviders(providerStatus);
      const configured = providerStatus.filter((item) => item.configured);
      const preferredRealProvider = configured.find((item) => !item.mock)?.key;
      const currentIsUsable = configured.some((item) => item.key === provider) && (provider !== 'mock' || !preferredRealProvider);
      const preferredProvider = currentIsUsable ? provider : preferredRealProvider || configured[0]?.key || 'mock';
      setProvider(preferredProvider);
      const loadedModels = await videoApi.listModels(preferredProvider).catch(() => []);
      setModels(loadedModels);
      setModel((current) => loadedModels.some((item) => item.id === current) ? current : loadedModels[0]?.id || 'mock-video-v1');
    } finally {
      setLoading(false);
    }
  }, [provider]);

  useEffect(() => {
    setEnabled(isEnabled('video_studio'));
  }, [isEnabled]);

  useEffect(() => {
    loadVideoSettings();
  }, [loadVideoSettings]);

  const handleProviderChange = async (nextProvider: VideoProviderKey) => {
    setProvider(nextProvider);
    setLoading(true);
    try {
      const loadedModels = await videoApi.listModels(nextProvider).catch(() => []);
      setModels(loadedModels);
      setModel(loadedModels[0]?.id || '');
    } finally {
      setLoading(false);
    }
  };

  const refreshModels = async () => {
    setLoading(true);
    try {
      const loadedModels = await videoApi.refreshModels(provider).catch(() => []);
      setModels(loadedModels);
      setModel((current) => loadedModels.some((item) => item.id === current) ? current : loadedModels[0]?.id || '');
      toast.success('Video model list refreshed');
    } finally {
      setLoading(false);
    }
  };

  const saveVideoSettings = async () => {
    setSaving(true);
    try {
      await updateFeature('video_studio', enabled);
      toast.success('Video Studio settings saved');
    } catch {
      toast.error('Failed to save Video Studio settings');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="p-5 rounded-2xl bg-surface-alt border border-border">
        <div className="mb-4 flex items-center gap-3">
          <div className="w-10 h-10 rounded-2xl bg-gradient-to-br from-cyan-500/20 to-emerald-500/20 flex items-center justify-center shadow-md shadow-cyan-500/10">
            <Film size={18} className="text-cyan-300" />
          </div>
          <div>
            <h3 className="text-sm font-bold">Video Studio</h3>
            <p className="text-[11px] text-text-muted">Configure AI video generation and timeline editing availability</p>
          </div>
        </div>

        <div className="space-y-4">
          <div className="flex items-center justify-between py-2">
            <div>
              <label className="text-xs font-medium text-text-secondary block">Enable Video Studio</label>
              <p className="text-[10px] text-text-muted mt-0.5">Shows Video Studio in the app sidebar</p>
            </div>
            <button
              type="button"
              onClick={() => setEnabled((value) => !value)}
              className={`relative w-10 h-5 rounded-full transition-colors ${enabled ? 'bg-primary' : 'bg-border'}`}
            >
              <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform ${enabled ? 'translate-x-5' : ''}`} />
            </button>
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div>
              <label className="text-xs font-medium text-text-secondary mb-1.5 block">Default provider</label>
              <select
                value={provider}
                onChange={(event) => { void handleProviderChange(event.target.value as VideoProviderKey); }}
                className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text focus:outline-none focus:border-primary/50"
              >
                {providers.map((item) => (
                  <option key={item.key} value={item.key}>
                    {item.display_name} {item.configured ? '' : '(not configured)'}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="text-xs font-medium text-text-secondary mb-1.5 block">Output directory</label>
              <div className="min-h-10 rounded-xl border border-border bg-surface px-3 py-2 text-xs text-text-muted">
                OMNILLM_ATTACHMENTS_DIR/video
              </div>
            </div>
          </div>

          <VideoModelSelect
            value={model}
            models={models}
            fallback="mock-video-v1"
            onChange={setModel}
          />

          <div className="rounded-xl border border-border bg-surface p-3">
            <p className="text-xs text-text-secondary">
              OpenRouter Video and direct Gemini Veo use encrypted provider profiles when API keys are configured. The mock provider stays available for local placeholder assets.
            </p>
          </div>

          <div className="flex flex-col gap-2 sm:flex-row">
            <motion.button
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
              onClick={refreshModels}
              disabled={loading}
              className="min-h-10 flex items-center justify-center gap-2 px-4 rounded-xl border border-border bg-surface text-sm text-text-secondary hover:text-text hover:bg-surface-hover disabled:opacity-50"
            >
              <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
              Refresh Video Models
            </motion.button>
            <motion.button
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
              onClick={saveVideoSettings}
              disabled={saving}
              className="min-h-10 flex items-center justify-center gap-2 px-4 rounded-xl btn-primary text-sm font-medium disabled:opacity-50"
            >
              {saving ? <RefreshCw size={14} className="animate-spin" /> : <Save size={14} />}
              Save Video Settings
            </motion.button>
          </div>
        </div>
      </div>
    </div>
  );
}

function VideoModelSelect({
  value,
  models,
  fallback,
  onChange,
}: {
  value: string;
  models: VideoModel[];
  fallback: string;
  onChange: (value: string) => void;
}) {
  const options = models.length > 0 ? models : [{ id: fallback, name: fallback } as VideoModel];
  return (
    <div>
      <label className="text-xs font-medium text-text-secondary mb-1.5 block">Default video model</label>
      <select
        value={value || fallback}
        onChange={(event) => onChange(event.target.value)}
        className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text focus:outline-none focus:border-primary/50"
      >
        {options.map((item) => (
          <option key={item.id} value={item.id}>{item.name || item.id}</option>
        ))}
      </select>
      {options[0]?.notes && <p className="mt-1.5 text-[10px] text-text-muted">{options[0].notes}</p>}
    </div>
  );
}

function ModelSelect({
  label,
  value,
  models,
  fallback,
  onChange,
}: {
  label: string;
  value: string;
  models: MusicModel[];
  fallback: string;
  onChange: (value: string) => void;
}) {
  const options = models.length > 0 ? models : [{ id: fallback, name: fallback } as MusicModel];
  return (
    <div>
      <label className="text-xs font-medium text-text-secondary mb-1.5 block">{label}</label>
      <select
        value={value || fallback}
        onChange={(event) => onChange(event.target.value)}
        className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text focus:outline-none focus:border-primary/50"
      >
        {options.map((model) => (
          <option key={model.id} value={model.id}>{model.name || model.id}</option>
        ))}
      </select>
    </div>
  );
}

function ToggleRow({ label, checked, onChange }: { label: string; checked: boolean; onChange: (checked: boolean) => void }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <span className="text-xs text-text-secondary">{label}</span>
      <button
        type="button"
        onClick={() => onChange(!checked)}
        className={`relative h-5 w-10 rounded-full transition-colors ${checked ? 'bg-primary' : 'bg-border'}`}
      >
        <span className={`absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform ${checked ? 'translate-x-5' : ''}`} />
      </button>
    </div>
  );
}

// ============================================
// Tools Settings Tab
// ============================================

function BrowserSettingsCard({
  toolsLoaded,
  browserToolsRegistered,
}: {
  toolsLoaded: boolean;
  browserToolsRegistered: boolean;
}) {
  const { isEnabled, updateFeature } = useFeatureFlagStore();
  const enabled = isEnabled('headless_browser');
  const [status, setStatus] = useState<BrowserStatus | null>(null);
  const [sessions, setSessions] = useState<BrowserSession[]>([]);
  const [loading, setLoading] = useState(true);
  const [closingId, setClosingId] = useState<string | null>(null);

  const loadBrowserState = useCallback(async () => {
    if (!enabled || !toolsLoaded || !browserToolsRegistered) {
      setStatus(null);
      setSessions([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const [nextStatus, nextSessions] = await Promise.all([
        browserApi.status(),
        browserApi.listSessions(),
      ]);
      setStatus(nextStatus);
      setSessions(nextSessions || []);
    } catch {
      toast.error('Failed to load browser status');
    } finally {
      setLoading(false);
    }
  }, [browserToolsRegistered, enabled, toolsLoaded]);

  useEffect(() => {
    loadBrowserState();
  }, [loadBrowserState]);

  const toggleFeature = async () => {
    const nextEnabled = !enabled;
    await updateFeature('headless_browser', nextEnabled);
    if (!nextEnabled) {
      setStatus(null);
      setSessions([]);
    }
  };

  const closeSession = async (id: string) => {
    setClosingId(id);
    try {
      await browserApi.closeSession(id);
      setSessions((prev) => prev.filter((session) => session.id !== id));
      setStatus((prev) => prev ? { ...prev, active_sessions: Math.max(0, prev.active_sessions - 1) } : prev);
      toast.success('Browser session closed');
    } catch {
      toast.error('Failed to close browser session');
    } finally {
      setClosingId(null);
    }
  };

  const runtimeReady = status?.enabled === true;
  const activeCount = status?.active_sessions ?? sessions.length;
  const browserUnavailable = enabled && toolsLoaded && !browserToolsRegistered;

  return (
    <div className="p-5 rounded-2xl bg-surface-alt border border-border">
      <div className="flex items-center gap-3 mb-4">
        <div className="w-10 h-10 rounded-2xl bg-gradient-to-br from-cyan-500/20 to-blue-500/20 flex items-center justify-center shadow-md shadow-cyan-500/10">
          <Globe size={18} className="text-cyan-400" />
        </div>
        <div className="min-w-0">
          <h3 className="text-sm font-bold">Headless Browser</h3>
          <p className="text-[11px] text-text-muted">JavaScript page reading, screenshots, and stateful browser sessions</p>
        </div>
      </div>

      <div className="space-y-4">
        <div className="flex items-center justify-between py-2">
          <div className="mr-4">
            <label className="text-xs font-medium text-text-secondary block">Browser Tools Enabled</label>
            <p className="text-[10px] text-text-muted mt-0.5">
              Tool visibility is controlled here; the runtime also requires OMNILLM_BROWSER_ENABLED on the backend.
            </p>
          </div>
          <button
            type="button"
            onClick={toggleFeature}
            className={`shrink-0 relative w-10 h-5 rounded-full transition-colors ${
              enabled ? 'bg-primary' : 'bg-border'
            }`}
          >
            <span
              className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform ${
                enabled ? 'translate-x-5' : ''
              }`}
            />
          </button>
        </div>

        {enabled && (
          <div className="flex items-center gap-3 py-1">
            {browserUnavailable ? (
              <>
                <span className="w-2 h-2 rounded-full bg-amber-400 shrink-0" />
                <div className="min-w-0">
                  <span className="text-xs text-amber-300">Tools not registered</span>
                  <p className="text-[10px] text-text-muted mt-0.5">Restart the backend to activate the browser implementation.</p>
                </div>
              </>
            ) : loading ? (
              <>
                <span className="w-2 h-2 rounded-full bg-text-muted/40 shrink-0 animate-pulse" />
                <span className="text-xs text-text-muted">Checking Chromium status…</span>
              </>
            ) : runtimeReady ? (
              <>
                <span className="w-2 h-2 rounded-full bg-emerald-400 shrink-0" />
                <div className="min-w-0 flex-1">
                  <span className="text-xs text-emerald-300">Chromium ready</span>
                  {status?.cache_dir && (
                    <p className="text-[10px] text-text-muted font-mono truncate mt-0.5">{status.cache_dir}</p>
                  )}
                </div>
                {activeCount > 0 && (
                  <span className="ml-auto shrink-0 text-[10px] font-medium px-2 py-0.5 rounded-full bg-cyan-500/15 text-cyan-400">
                    {activeCount} session{activeCount !== 1 ? 's' : ''}
                  </span>
                )}
              </>
            ) : (
              <>
                <span className="w-2 h-2 rounded-full bg-amber-400 shrink-0" />
                <div className="min-w-0">
                  <span className="text-xs text-amber-300">Backend not configured</span>
                  <p className="text-[10px] text-text-muted mt-0.5">Set <span className="font-mono">OMNILLM_BROWSER_ENABLED=true</span> and restart to enable Chromium.</p>
                </div>
              </>
            )}
          </div>
        )}

        {sessions.length > 0 && (
          <div className="rounded-xl border border-border bg-surface/50 overflow-hidden">
            <div className="px-3 py-2 text-[10px] font-semibold uppercase tracking-wider text-text-muted border-b border-border">
              Active sessions
            </div>
            <div className="divide-y divide-border/70">
              {sessions.map((session) => (
                <div key={session.id} className="flex items-center justify-between gap-3 px-3 py-2">
                  <div className="min-w-0">
                    <div className="text-xs font-mono text-text truncate">{session.id.slice(0, 12)}</div>
                    <div className="text-[10px] text-text-muted truncate">{session.current_url || 'No page loaded'}</div>
                    <div className="text-[10px] text-text-muted/70">
                      Last used {new Date(session.last_used_at).toLocaleString()}
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={() => closeSession(session.id)}
                    disabled={closingId === session.id}
                    className="shrink-0 p-1.5 rounded-lg text-text-muted hover:text-red-400 hover:bg-red-500/10 transition-colors disabled:opacity-50"
                    aria-label="Close browser session"
                    title="Close browser session"
                  >
                    {closingId === session.id ? <RefreshCw size={13} className="animate-spin" /> : <X size={13} />}
                  </button>
                </div>
              ))}
            </div>
          </div>
        )}

        <button
          type="button"
          onClick={loadBrowserState}
          disabled={loading}
          className="flex items-center gap-2 text-[11px] text-text-muted hover:text-text transition-colors disabled:opacity-50"
        >
          <RefreshCw size={12} className={loading ? 'animate-spin' : ''} />
          Refresh browser status
        </button>
      </div>
    </div>
  );
}

function ToolsTab() {
  const [tools, setTools] = useState<import('../types').ToolDefinition[]>([]);
  const [loading, setLoading] = useState(true);
  const browserToolsRegistered = tools.some((tool) => tool.name.startsWith('browser_'));

  useEffect(() => {
    api.listTools()
      .then((t) => setTools(t || []))
      .catch(() => toast.error('Failed to load tools'))
      .finally(() => setLoading(false));
  }, []);

  const updatePermission = async (toolName: string, policy: string) => {
    try {
      await api.updateToolPermission(toolName, policy);
      setTools((prev) =>
        prev.map((t) => (t.name === toolName ? { ...t, policy } : t))
      );
      toast.success(`${toolName} set to ${policy}`);
    } catch {
      toast.error('Failed to update tool permission');
    }
  };

  return (
    <div className="space-y-6">
      <BrowserSettingsCard toolsLoaded={!loading} browserToolsRegistered={browserToolsRegistered} />

      {loading ? (
        <div className="flex items-center justify-center py-8">
          <RefreshCw size={16} className="animate-spin text-text-muted" />
        </div>
      ) : tools.length === 0 ? (
        <div className="text-center py-8">
          <p className="text-sm text-text-muted">No tools registered</p>
          <p className="text-xs text-text-muted/60 mt-1">Tools will appear here when plugins provide them</p>
        </div>
      ) : (
        <>
          {tools.map((tool) => {
            const enabled = tool.policy === 'allow' || tool.policy === 'ask';
            
            const toggle = () => {
              updatePermission(tool.name, enabled ? 'deny' : 'allow');
            };

            let icon = <Wrench size={18} className="text-orange-400" />;
            let gradient = "from-orange-500/20 to-amber-500/20";
            let shadow = "shadow-orange-500/10";
            
            if (tool.name === 'sports_lookup') {
              icon = <Trophy size={18} className="text-emerald-400" />;
              gradient = "from-emerald-500/20 to-teal-500/20";
              shadow = "shadow-emerald-500/10";
            } else if (tool.name.includes('github')) {
              icon = <Github size={18} className="text-slate-400" />;
              gradient = "from-slate-500/20 to-gray-500/20";
              shadow = "shadow-slate-500/10";
            } else if (tool.name === 'web_search') {
              icon = <Search size={18} className="text-blue-400" />;
              gradient = "from-blue-500/20 to-indigo-500/20";
              shadow = "shadow-blue-500/10";
            } else if (tool.name === 'calculator') {
              icon = <Calculator size={18} className="text-pink-400" />;
              gradient = "from-pink-500/20 to-rose-500/20";
              shadow = "shadow-pink-500/10";
            } else if (tool.name.includes('url')) {
              icon = <Link2 size={18} className="text-cyan-400" />;
              gradient = "from-cyan-500/20 to-blue-500/20";
              shadow = "shadow-cyan-500/10";
            } else if (tool.name.startsWith('mcp_')) {
              icon = <Plug size={18} className="text-violet-400" />;
              gradient = "from-violet-500/20 to-purple-500/20";
              shadow = "shadow-violet-500/10";
            }

            const shortDesc = tool.description ? tool.description.split('.')[0] + '.' : 'Registered Tool capability';

            return (
              <div key={tool.name} className="p-5 rounded-2xl bg-surface-alt border border-border">
                <div className="flex items-center gap-3 mb-4">
                  <div className={`w-10 h-10 rounded-2xl bg-gradient-to-br ${gradient} flex items-center justify-center shadow-md ${shadow} shrink-0`}>
                    {icon}
                  </div>
                  <div>
                    <h3 className="text-sm font-bold">{tool.name}</h3>
                    <p className="text-[11px] text-text-muted">{shortDesc}</p>
                  </div>
                </div>

                <div className="flex items-center justify-between py-2">
                  <div className="mr-4">
                    <label className="text-xs font-medium text-text-secondary block">Enable {tool.name}</label>
                    <p className="text-[10px] text-text-muted mt-0.5">
                      {tool.description || 'Allow the AI to use this tool during conversations.'}
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={toggle}
                    className={`shrink-0 relative w-10 h-5 rounded-full transition-colors ${
                      enabled ? 'bg-primary' : 'bg-border'
                    }`}
                  >
                    <span
                      className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform ${
                        enabled ? 'translate-x-5' : ''
                      }`}
                    />
                  </button>
                </div>
              </div>
            );
          })}
        </>
      )}

    </div>
  );
}

// ============================================
// MCP Servers Settings Tab
// ============================================

type MCPFormState = {
  name: string;
  transport: MCPTransport;
  command: string;
  argsText: string;
  url: string;
  envText: string;
  headersText: string;
  enabled: boolean;
};

const emptyMCPForm: MCPFormState = {
  name: '',
  transport: 'stdio',
  command: '',
  argsText: '',
  url: '',
  envText: '',
  headersText: '',
  enabled: false,
};

const filesystemMCPTemplate: MCPFormState = {
  name: 'filesystem',
  transport: 'stdio',
  command: 'npx.cmd',
  argsText: [
    '-y',
    '@modelcontextprotocol/server-filesystem@2025.8.21',
    'C:\\Users\\you\\Documents',
  ].join('\n'),
  url: '',
  envText: '',
  headersText: '',
  enabled: false,
};

function parseMCPArgs(value: string): string[] {
  return value
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean);
}

function parseMCPEnv(value: string, label: string): Record<string, string> {
  const env: Record<string, string> = {};
  for (const rawLine of value.split('\n')) {
    const line = rawLine.trim();
    if (!line) continue;
    const equals = line.indexOf('=');
    if (equals <= 0) {
      throw new Error(`${label} must use KEY=value lines`);
    }
    const key = line.slice(0, equals).trim();
    const envValue = line.slice(equals + 1).trim();
    if (!key) {
      throw new Error(`${label} keys cannot be empty`);
    }
    env[key] = envValue;
  }
  return env;
}

function createMCPRequest(form: MCPFormState): CreateMCPServerRequest {
  const name = form.name.trim();
  const transport = form.transport;
  if (!name) throw new Error('Server name is required');
  
  const request: CreateMCPServerRequest = {
    name,
    transport,
    enabled: form.enabled,
  };

  if (transport === 'stdio') {
    const command = form.command.trim();
    if (!command) throw new Error('Command is required for stdio transport');
    request.command = command;
    request.args = parseMCPArgs(form.argsText);
    if (form.envText.trim()) {
      request.env = parseMCPEnv(form.envText, 'Environment variables');
    }
  } else if (transport === 'http') {
    const url = form.url.trim();
    if (!url) throw new Error('URL is required for http transport');
    request.url = url;
    if (form.headersText.trim()) {
      request.headers = parseMCPEnv(form.headersText, 'Headers');
    }
  }

  return request;
}

function updateMCPRequest(form: MCPFormState): UpdateMCPServerRequest {
  const request = createMCPRequest(form) as UpdateMCPServerRequest;
  if (form.transport === 'stdio' && !form.envText.trim()) {
    delete request.env;
  }
  if (form.transport === 'http' && !form.headersText.trim()) {
    delete request.headers;
  }
  return request;
}

function formFromMCPServer(server: MCPServer): MCPFormState {
  return {
    name: server.name,
    transport: server.transport || 'stdio',
    command: server.command || '',
    argsText: (server.args || []).join('\n'),
    url: server.url || '',
    envText: '',
    headersText: '',
    enabled: server.enabled,
  };
}

function statusBadgeClass(status: MCPServer['status']): string {
  if (status === 'connected') return 'border-success/30 bg-success/10 text-success';
  if (status === 'connecting') return 'border-primary/30 bg-primary/10 text-primary';
  if (status === 'error') return 'border-danger/30 bg-danger-soft text-danger';
  if (status === 'disabled') return 'border-border bg-surface text-text-muted';
  return 'border-amber-500/30 bg-amber-500/10 text-amber-300';
}

function MCPServersTab() {
  const [servers, setServers] = useState<MCPServer[]>([]);
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<MCPFormState>(emptyMCPForm);
  const [saving, setSaving] = useState(false);
  const [busyById, setBusyById] = useState<Record<string, string>>({});
  const [testTools, setTestTools] = useState<Record<string, MCPTool[]>>({});
  const [auditEvents, setAuditEvents] = useState<MCPAuditEvent[]>([]);

  const fetchServers = useCallback(async () => {
    try {
      setLoading(true);
      const [serverData, auditData] = await Promise.all([
        mcpApi.listServers(),
        mcpApi.listAudit({ limit: 50 }),
      ]);
      setServers(serverData);
      setAuditEvents(auditData);
    } catch {
      toast.error('Failed to load MCP servers');
    } finally {
      setLoading(false);
    }
  }, []);

  const refreshAudit = useCallback(async () => {
    try {
      setAuditEvents(await mcpApi.listAudit({ limit: 50 }));
    } catch {
      // Leave stale audit rows visible if the refresh fails after an action.
    }
  }, []);

  useEffect(() => {
    fetchServers();
  }, [fetchServers]);

  const resetForm = () => {
    setAdding(false);
    setEditingId(null);
    setForm(emptyMCPForm);
  };

  const applyServer = (server: MCPServer) => {
    setServers((prev) => {
      const next = prev.some((item) => item.id === server.id)
        ? prev.map((item) => (item.id === server.id ? server : item))
        : [...prev, server];
      return [...next].sort((a: MCPServer, b: MCPServer) => a.name.localeCompare(b.name));
    });
  };

  const markBusy = (serverId: string, label?: string) => {
    setBusyById((prev) => {
      const next = { ...prev };
      if (label) next[serverId] = label;
      else delete next[serverId];
      return next;
    });
  };

  const handleSave = async () => {
    try {
      setSaving(true);
      if (editingId) {
        const updated = await mcpApi.updateServer(editingId, updateMCPRequest(form));
        applyServer(updated);
        toast.success('MCP server updated');
      } else {
        const created = await mcpApi.createServer(createMCPRequest(form));
        applyServer(created);
        toast.success('MCP server added');
      }
      refreshAudit();
      resetForm();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to save MCP server');
    } finally {
      setSaving(false);
    }
  };

  const handleLifecycle = async (
    serverId: string,
    label: string,
    action: () => Promise<MCPServer>,
    success: string,
  ) => {
    markBusy(serverId, label);
    try {
      applyServer(await action());
      refreshAudit();
      toast.success(success);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'MCP server action failed');
    } finally {
      markBusy(serverId);
    }
  };

  const handleRefreshTools = async (server: MCPServer) => {
    markBusy(server.id, 'Refreshing');
    try {
      const tools = await mcpApi.refreshTools(server.id);
      applyServer({ ...server, status: 'connected', tools });
      refreshAudit();
      toast.success(`${tools.length} MCP tool${tools.length === 1 ? '' : 's'} refreshed`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to refresh MCP tools');
    } finally {
      markBusy(server.id);
    }
  };

  const handleTest = async (serverId: string) => {
    markBusy(serverId, 'Testing');
    try {
      const result = await mcpApi.testServer(serverId);
      const tools = result.tools || [];
      setTestTools((prev) => ({ ...prev, [serverId]: tools }));
      refreshAudit();
      toast.success(`MCP test passed with ${tools.length} tool${tools.length === 1 ? '' : 's'}`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'MCP test failed');
    } finally {
      markBusy(serverId);
    }
  };

  const handlePolicy = async (serverId: string, internalName: string, policy: ToolPolicy) => {
    try {
      await mcpApi.updateToolPolicy(serverId, internalName, policy);
      setServers((prev) =>
        prev.map((server) => {
          if (server.id !== serverId) return server;
          return {
            ...server,
            tools: (server.tools || []).map((tool) =>
              tool.internal_name === internalName ? { ...tool, policy } : tool,
            ),
          };
        }),
      );
      refreshAudit();
      toast.success(`${internalName} set to ${policy}`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update MCP tool policy');
    }
  };

  const startEditing = (server: MCPServer) => {
    setAdding(false);
    setEditingId(server.id);
    setForm(formFromMCPServer(server));
  };

  const deleteServer = (server: MCPServer) => {
    toast(`Delete ${server.name}?`, {
      action: {
        label: 'Delete',
        onClick: async () => {
          try {
            await mcpApi.deleteServer(server.id);
            setServers((prev) => prev.filter((item) => item.id !== server.id));
            refreshAudit();
            toast.success('MCP server deleted');
          } catch (err) {
            toast.error(err instanceof Error ? err.message : 'Failed to delete MCP server');
          }
        },
      },
      cancel: { label: 'Cancel', onClick: () => {} },
      duration: 5000,
    });
  };

  return (
    <div className="space-y-6">
      <div className="p-5 rounded-2xl bg-surface-alt border border-border">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-gradient-to-br from-cyan-500/20 to-emerald-500/20 flex items-center justify-center shadow-md shadow-cyan-500/10">
              <Plug size={18} className="text-cyan-300" />
            </div>
            <div>
              <h3 className="text-sm font-bold">MCP Servers</h3>
              <p className="text-[11px] text-text-muted">Connect stdio and HTTP Model Context Protocol servers</p>
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={fetchServers}
              className="min-h-10 inline-flex items-center justify-center gap-1.5 px-3 rounded-xl border border-border hover:bg-surface-hover text-xs text-text transition-colors"
            >
              <RefreshCw size={13} />
              Refresh
            </button>
            <button
              type="button"
              onClick={() => {
                setEditingId(null);
                setAdding(true);
                setForm(emptyMCPForm);
              }}
              className="btn-primary min-h-10 inline-flex items-center justify-center gap-1.5 px-3 text-xs rounded-xl font-medium"
            >
              <Plus size={13} />
              Add Server
            </button>
          </div>
        </div>

        <div className="mt-4 rounded-xl border border-amber-500/20 bg-amber-500/5 p-3">
          <div className="flex gap-2">
            <AlertTriangle size={15} className="mt-0.5 shrink-0 text-amber-300" />
            <p className="text-[11px] leading-relaxed text-text-muted">
              Stdio MCP launches local commands. Use absolute paths when the desktop app cannot find
              `npx`, `uvx`, Node, or Python from its inherited PATH.
            </p>
          </div>
        </div>
      </div>

      <AnimatePresence>
        {(adding || editingId) && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            className="overflow-hidden"
          >
            <MCPServerForm
              form={form}
              editing={Boolean(editingId)}
              saving={saving}
              onChange={(patch) => setForm((prev) => ({ ...prev, ...patch }))}
              onCancel={resetForm}
              onSave={handleSave}
              onUseFilesystemTemplate={() => setForm(filesystemMCPTemplate)}
            />
          </motion.div>
        )}
      </AnimatePresence>

      {loading ? (
        <div className="flex items-center justify-center py-10">
          <RefreshCw size={16} className="animate-spin text-text-muted" />
        </div>
      ) : servers.length === 0 ? (
        <div className="p-8 rounded-2xl bg-surface-alt border border-border text-center">
          <Terminal size={22} className="mx-auto mb-3 text-text-muted" />
          <p className="text-sm font-medium text-text">No MCP servers configured</p>
          <p className="mt-1 text-xs text-text-muted">Add a stdio or HTTP server to discover tools.</p>
        </div>
      ) : (
        <div className="space-y-4">
          {servers.map((server) => (
            <MCPServerCard
              key={server.id}
              server={server}
              busyLabel={busyById[server.id]}
              testTools={testTools[server.id] || []}
              onEdit={() => startEditing(server)}
              onDelete={() => deleteServer(server)}
              onTest={() => handleTest(server.id)}
              onStart={() => handleLifecycle(server.id, 'Starting', () => mcpApi.startServer(server.id), 'MCP server started')}
              onStop={() => handleLifecycle(server.id, 'Stopping', () => mcpApi.stopServer(server.id), 'MCP server stopped')}
              onRestart={() => handleLifecycle(server.id, 'Restarting', () => mcpApi.restartServer(server.id), 'MCP server restarted')}
              onRefresh={() => handleRefreshTools(server)}
              onPolicy={(toolName, policy) => handlePolicy(server.id, toolName, policy)}
            />
          ))}
        </div>
      )}

      <MCPAuditPanel events={auditEvents} servers={servers} onRefresh={refreshAudit} />
    </div>
  );
}

function MCPAuditPanel({
  events,
  servers,
  onRefresh,
}: {
  events: MCPAuditEvent[];
  servers: MCPServer[];
  onRefresh: () => void;
}) {
  const serverNameById = new Map(servers.map((server) => [server.id, server.name]));

  return (
    <div className="p-5 rounded-2xl bg-surface-alt border border-border">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-2xl bg-gradient-to-br from-emerald-500/20 to-teal-500/20 flex items-center justify-center shadow-md shadow-emerald-500/10">
            <ClipboardList size={18} className="text-emerald-300" />
          </div>
          <div>
            <h3 className="text-sm font-bold">MCP Activity</h3>
            <p className="text-[11px] text-text-muted">Recent server and tool events</p>
          </div>
        </div>
        <button
          type="button"
          onClick={onRefresh}
          className="min-h-10 inline-flex items-center justify-center gap-1.5 rounded-xl border border-border px-3 text-xs text-text hover:bg-surface-hover transition-colors"
        >
          <RefreshCw size={13} />
          Refresh
        </button>
      </div>

      {events.length === 0 ? (
        <p className="rounded-xl border border-border bg-surface p-3 text-xs text-text-muted">
          No MCP activity recorded yet.
        </p>
      ) : (
        <div className="space-y-2">
          {events.map((event) => (
            <div key={event.id} className="rounded-xl border border-border bg-surface p-3">
              <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-xs font-semibold text-text">{event.event_type}</span>
                    <span className="text-[11px] text-text-muted">
                      {serverNameById.get(event.server_id) || event.server_id}
                    </span>
                    {event.tool_name && (
                      <span className="rounded-md border border-border bg-surface-alt px-1.5 py-0.5 text-[10px] text-text-muted">
                        {event.tool_name}
                      </span>
                    )}
                  </div>
                  {event.error_msg && (
                    <p className="mt-1 break-words text-[11px] text-danger">{event.error_msg}</p>
                  )}
                </div>
                <div className="shrink-0 text-[11px] text-text-muted">
                  {new Date(event.created_at).toLocaleString()}
                  {typeof event.duration_ms === 'number' && (
                    <span className="ml-2">{event.duration_ms} ms</span>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function MCPServerForm({
  form,
  editing,
  saving,
  onChange,
  onCancel,
  onSave,
  onUseFilesystemTemplate,
}: {
  form: MCPFormState;
  editing: boolean;
  saving: boolean;
  onChange: (patch: Partial<MCPFormState>) => void;
  onCancel: () => void;
  onSave: () => void;
  onUseFilesystemTemplate: () => void;
}) {
  return (
    <div className="p-5 rounded-2xl bg-surface-alt border border-border">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-4">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-2xl bg-gradient-to-br from-primary/20 to-cyan-500/20 flex items-center justify-center shadow-md shadow-primary/10">
            <Terminal size={18} className="text-primary" />
          </div>
          <div>
            <h3 className="text-sm font-bold">{editing ? 'Edit MCP Server' : 'Add MCP Server'}</h3>
            <p className="text-[11px] text-text-muted">Configure a local stdio or remote HTTP server</p>
          </div>
        </div>
        {!editing && (
          <button
            type="button"
            onClick={onUseFilesystemTemplate}
            className="min-h-10 inline-flex items-center justify-center gap-1.5 px-3 rounded-xl border border-border hover:bg-surface-hover text-xs text-text transition-colors"
          >
            <ClipboardList size={13} />
            Filesystem Template
          </button>
        )}
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 mb-3">
        <div>
          <label className="block text-xs text-text-muted mb-1 font-medium">Name</label>
          <input
            value={form.name}
            onChange={(event) => onChange({ name: event.target.value })}
            placeholder="my-server"
            className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text focus:outline-none focus:border-primary transition-all input-glow"
          />
        </div>
        <div>
          <label className="block text-xs text-text-muted mb-1 font-medium">Transport</label>
          <select
            value={form.transport}
            onChange={(event) => onChange({ transport: event.target.value as MCPTransport })}
            className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text focus:outline-none focus:border-primary transition-all input-glow appearance-none"
            style={{
              backgroundImage: 'url("data:image/svg+xml;charset=US-ASCII,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20width%3D%2212%22%20height%3D%2212%22%20viewBox%3D%220%200%2012%2012%22%3E%3Cpath%20fill%3D%22%239ca3af%22%20d%3D%22M2.22%204.22a.75.75%200%200%201%201.06%200L6%206.94l2.72-2.72a.75.75%200%201%201%201.06%201.06l-3.25%203.25a.75.75%200%200%201-1.06%200L2.22%205.28a.75.75%200%200%201%200-1.06Z%22%2F%3E%3C%2Fsvg%3E")',
              backgroundRepeat: 'no-repeat',
              backgroundPosition: 'right 12px center',
              paddingRight: '32px'
            }}
          >
            <option value="stdio">stdio (Local Command)</option>
            <option value="http">http (Streamable Remote)</option>
          </select>
        </div>
      </div>

      {form.transport === 'stdio' ? (
        <>
          <div className="mb-3">
            <label className="block text-xs text-text-muted mb-1 font-medium">Command</label>
            <input
              value={form.command}
              onChange={(event) => onChange({ command: event.target.value })}
              placeholder="npx.cmd or python"
              className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text focus:outline-none focus:border-primary transition-all input-glow"
            />
          </div>

          <div className="mb-3">
            <label className="block text-xs text-text-muted mb-1 font-medium">Arguments (one per line)</label>
            <textarea
              value={form.argsText}
              onChange={(event) => onChange({ argsText: event.target.value })}
              rows={4}
              placeholder={"-y\n@modelcontextprotocol/server-filesystem@2025.8.21\nC:\\Users\\you\\Documents"}
              className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text focus:outline-none focus:border-primary transition-all input-glow font-mono resize-y"
            />
          </div>

          <div className="mb-3">
            <label className="block text-xs text-text-muted mb-1 font-medium">Environment (KEY=value)</label>
            <textarea
              value={form.envText}
              onChange={(event) => onChange({ envText: event.target.value })}
              rows={3}
              placeholder={editing ? 'Leave blank to keep existing secret values' : 'GITHUB_TOKEN=...'}
              className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text focus:outline-none focus:border-primary transition-all input-glow font-mono resize-y"
            />
            <p className="mt-1 text-[11px] text-text-muted">
              Environment values are encrypted at rest. Saving env lines replaces stored values.
            </p>
          </div>
        </>
      ) : (
        <>
          <div className="mb-3">
            <label className="block text-xs text-text-muted mb-1 font-medium">Server URL</label>
            <input
              value={form.url}
              onChange={(event) => onChange({ url: event.target.value })}
              placeholder="http://localhost:8000/mcp"
              className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text focus:outline-none focus:border-primary transition-all input-glow"
            />
          </div>

          <div className="mb-3">
            <label className="block text-xs text-text-muted mb-1 font-medium">Custom Headers (KEY=value)</label>
            <textarea
              value={form.headersText}
              onChange={(event) => onChange({ headersText: event.target.value })}
              rows={3}
              placeholder={editing ? 'Leave blank to keep existing secret headers' : 'Authorization=Bearer xyz...'}
              className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl text-text focus:outline-none focus:border-primary transition-all input-glow font-mono resize-y"
            />
            <p className="mt-1 text-[11px] text-text-muted">
              Headers are encrypted at rest. Use for authentication (e.g. Authorization, X-API-Key).
            </p>
          </div>
        </>
      )}

      <label className="mt-4 flex items-center gap-2 text-xs text-text">
        <input
          type="checkbox"
          checked={form.enabled}
          onChange={(event) => onChange({ enabled: event.target.checked })}
          className="accent-primary"
        />
        Start this server automatically
      </label>

      <div className="mt-4 flex flex-wrap justify-end gap-2">
        <button
          type="button"
          onClick={onCancel}
          className="min-h-10 px-4 text-sm rounded-xl border border-border hover:bg-surface-hover text-text transition-colors"
        >
          Cancel
        </button>
        <button
          type="button"
          onClick={onSave}
          disabled={saving}
          className="btn-primary min-h-10 inline-flex items-center justify-center gap-1.5 px-4 text-sm rounded-xl font-medium disabled:opacity-50"
        >
          {saving ? <RefreshCw size={14} className="animate-spin" /> : <Save size={14} />}
          Save
        </button>
      </div>
    </div>
  );
}

function MCPServerCard({
  server,
  busyLabel,
  testTools,
  onEdit,
  onDelete,
  onTest,
  onStart,
  onStop,
  onRestart,
  onRefresh,
  onPolicy,
}: {
  server: MCPServer;
  busyLabel?: string;
  testTools: MCPTool[];
  onEdit: () => void;
  onDelete: () => void;
  onTest: () => void;
  onStart: () => void;
  onStop: () => void;
  onRestart: () => void;
  onRefresh: () => void;
  onPolicy: (toolName: string, policy: ToolPolicy) => void;
}) {
  const tools = server.tools || [];
  const commandLine = server.transport === 'http' 
    ? server.url || 'No URL configured'
    : [server.command, ...(server.args || [])].filter(Boolean).join(' ');

  return (
    <div className="p-5 rounded-2xl bg-surface-alt border border-border">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-sm font-bold text-text">{server.name}</h3>
            <span className={clsx('rounded-md border px-2 py-0.5 text-[10px] font-semibold uppercase', statusBadgeClass(server.status))}>
              {server.status}
            </span>
            <span className="rounded-md border border-border bg-surface px-2 py-0.5 text-[10px] font-semibold uppercase text-text-muted">
              {server.transport}
            </span>
          </div>
          <p className="mt-1 truncate font-mono text-[11px] text-text-muted">{commandLine || 'No command configured'}</p>
          {server.env_keys && server.env_keys.length > 0 && (
            <p className="mt-1 text-[11px] text-text-muted">
              Env: {server.env_keys.join(', ')}
            </p>
          )}
          {server.header_keys && server.header_keys.length > 0 && (
            <p className="mt-1 text-[11px] text-text-muted">
              Headers: {server.header_keys.join(', ')}
            </p>
          )}
          {server.last_error && (
            <div className="mt-2 flex gap-2 rounded-xl border border-danger/20 bg-danger-soft/40 p-2 text-[11px] text-danger">
              <AlertTriangle size={13} className="mt-0.5 shrink-0" />
              <span className="min-w-0 break-words">{server.last_error}</span>
            </div>
          )}
        </div>

        <div className="flex flex-wrap gap-2 sm:justify-end">
          <MCPActionButton label="Test" title="Test connection" busy={busyLabel === 'Testing'} onClick={onTest}>
            <CheckCircle2 size={13} />
          </MCPActionButton>
          {server.status === 'connected' ? (
            <MCPActionButton label="Stop" title="Stop server" busy={busyLabel === 'Stopping'} onClick={onStop}>
              <Square size={13} />
            </MCPActionButton>
          ) : (
            <MCPActionButton label="Start" title="Start server" busy={busyLabel === 'Starting'} onClick={onStart}>
              <Play size={13} />
            </MCPActionButton>
          )}
          <MCPActionButton label="Restart" title="Restart server" busy={busyLabel === 'Restarting'} onClick={onRestart}>
            <RefreshCw size={13} />
          </MCPActionButton>
          <MCPActionButton label="Tools" title="Refresh tools" busy={busyLabel === 'Refreshing'} onClick={onRefresh}>
            <Wrench size={13} />
          </MCPActionButton>
          <MCPActionButton label="Edit" title="Edit server" onClick={onEdit}>
            <Pencil size={13} />
          </MCPActionButton>
          <button
            type="button"
            onClick={onDelete}
            title="Delete server"
            className="min-h-10 min-w-10 inline-flex items-center justify-center rounded-xl hover:bg-danger-soft text-text-muted hover:text-danger transition-colors"
          >
            <Trash2 size={14} />
          </button>
        </div>
      </div>

      {busyLabel && (
        <div className="mt-3 flex items-center gap-2 text-xs text-text-muted">
          <RefreshCw size={13} className="animate-spin" />
          {busyLabel}
        </div>
      )}

      {testTools.length > 0 && (
        <div className="mt-3 rounded-xl border border-success/20 bg-success/5 p-3">
          <p className="text-[11px] font-semibold uppercase text-success">Last test discovered {testTools.length} tool{testTools.length === 1 ? '' : 's'}</p>
          <p className="mt-1 truncate text-[11px] text-text-muted">{testTools.map((tool) => tool.name).join(', ')}</p>
        </div>
      )}

      <div className="mt-4">
        <div className="mb-2 flex items-center justify-between">
          <p className="text-xs font-semibold text-text">Discovered Tools</p>
          <span className="text-[11px] text-text-muted">{tools.length}</span>
        </div>
        {tools.length === 0 ? (
          <p className="rounded-xl border border-border bg-surface p-3 text-xs text-text-muted">
            Start or refresh this server to load tools.
          </p>
        ) : (
          <div className="space-y-2">
            {tools.map((tool) => (
              <MCPToolRow
                key={tool.internal_name}
                tool={tool}
                onPolicy={(policy) => onPolicy(tool.internal_name, policy)}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function MCPActionButton({
  label,
  title,
  busy = false,
  onClick,
  children,
}: {
  label: string;
  title: string;
  busy?: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      title={title}
      disabled={busy}
      className="min-h-10 inline-flex items-center justify-center gap-1.5 rounded-xl border border-border px-3 text-xs text-text hover:bg-surface-hover disabled:opacity-50 transition-colors"
    >
      {busy ? <RefreshCw size={13} className="animate-spin" /> : children}
      {label}
    </button>
  );
}

function MCPToolRow({ tool, onPolicy }: { tool: MCPTool; onPolicy: (policy: ToolPolicy) => void }) {
  return (
    <div className="flex flex-col gap-3 rounded-xl border border-border bg-surface p-3 sm:flex-row sm:items-center sm:justify-between">
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-sm font-medium text-text">{tool.internal_name}</span>
          {tool.title && (
            <span className="rounded-md border border-border bg-surface-alt px-1.5 py-0.5 text-[10px] text-text-muted">
              {tool.title}
            </span>
          )}
        </div>
        {tool.description && (
          <p className="mt-1 line-clamp-2 text-[11px] text-text-muted">{tool.description}</p>
        )}
        <p className="mt-1 text-[11px] text-text-muted">MCP name: {tool.name}</p>
      </div>
      <select
        value={tool.policy}
        onChange={(event) => onPolicy(event.target.value as ToolPolicy)}
        className="min-h-10 px-2 py-1.5 text-xs bg-surface-alt border border-border rounded-lg text-text focus:outline-none focus:border-primary/50 transition-colors"
      >
        <option value="allow">Allow</option>
        <option value="ask">Ask</option>
        <option value="deny">Deny</option>
      </select>
    </div>
  );
}

// ============================================
// Pricing Settings Tab
// ============================================

function PricingTab() {
  const [rules, setRules] = useState<import('../types').PricingRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState(false);
  const [newRule, setNewRule] = useState({
    provider_type: '',
    model_pattern: '',
    input_cost_per_mtok: 0,
    output_cost_per_mtok: 0,
    currency: 'USD',
  });

  const fetchRules = useCallback(async () => {
    try {
      const data = await api.listPricing();
      setRules(data || []);
    } catch {
      toast.error('Failed to load pricing rules');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchRules();
  }, [fetchRules]);

  const handleAdd = async () => {
    if (!newRule.provider_type || !newRule.model_pattern) {
      toast.error('Provider and model are required');
      return;
    }
    try {
      const created = await api.upsertPricing({
        id: crypto.randomUUID(),
        provider_type: newRule.provider_type,
        model_pattern: newRule.model_pattern,
        input_cost_per_mtok: newRule.input_cost_per_mtok,
        output_cost_per_mtok: newRule.output_cost_per_mtok,
        currency: newRule.currency,
      });
      setRules((prev) => [...prev.filter((r) => r.id !== created.id), created]);
      setNewRule({ provider_type: '', model_pattern: '', input_cost_per_mtok: 0, output_cost_per_mtok: 0, currency: 'USD' });
      setAdding(false);
      toast.success('Pricing rule saved');
    } catch {
      toast.error('Failed to save pricing rule');
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await api.deletePricing(id);
      setRules((prev) => prev.filter((r) => r.id !== id));
      toast.success('Pricing rule deleted');
    } catch {
      toast.error('Failed to delete pricing rule');
    }
  };

  return (
    <div className="space-y-6">
      <div className="p-5 rounded-2xl bg-surface-alt border border-border">
        <div className="flex items-center gap-3 mb-4">
          <div className="w-10 h-10 rounded-2xl bg-gradient-to-br from-green-500/20 to-emerald-500/20 flex items-center justify-center shadow-md shadow-green-500/10">
            <DollarSign size={18} className="text-green-400" />
          </div>
          <div>
            <h3 className="text-sm font-bold">Pricing Rules</h3>
            <p className="text-[11px] text-text-muted">Define cost per million tokens for accurate usage tracking</p>
          </div>
        </div>

        <div className="flex flex-col gap-2 mb-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-xs text-text-muted">{rules.length} rule{rules.length !== 1 ? 's' : ''} configured</p>
          <motion.button
            whileHover={{ scale: 1.03 }}
            whileTap={{ scale: 0.97 }}
            onClick={() => setAdding(true)}
            className="btn-primary min-h-10 flex items-center justify-center gap-1.5 px-3 text-xs rounded-xl font-medium"
          >
            <Plus size={12} /> Add Rule
          </motion.button>
        </div>

        {/* Add rule form */}
        <AnimatePresence>
          {adding && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              className="overflow-hidden mb-3"
            >
              <div className="p-4 rounded-xl border border-primary/20 bg-gradient-to-br from-primary-glow to-transparent space-y-3">
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div>
                    <label className="block text-xs text-text-muted mb-1 font-medium">Provider</label>
                    <input
                      value={newRule.provider_type}
                      onChange={(e) => setNewRule((s) => ({ ...s, provider_type: e.target.value }))}
                      placeholder="openai"
                      className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl
                                 text-text focus:outline-none focus:border-primary transition-all"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-text-muted mb-1 font-medium">Model</label>
                    <input
                      value={newRule.model_pattern}
                      onChange={(e) => setNewRule((s) => ({ ...s, model_pattern: e.target.value }))}
                      placeholder="gpt-4o"
                      className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl
                                 text-text focus:outline-none focus:border-primary transition-all"
                    />
                  </div>
                </div>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div>
                    <label className="block text-xs text-text-muted mb-1 font-medium">Input $/M tokens</label>
                    <input
                      type="number"
                      step="0.01"
                      min="0"
                      value={newRule.input_cost_per_mtok}
                      onChange={(e) => setNewRule((s) => ({ ...s, input_cost_per_mtok: Math.max(0, parseFloat(e.target.value) || 0) }))}
                      className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl
                                 text-text focus:outline-none focus:border-primary transition-all"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-text-muted mb-1 font-medium">Output $/M tokens</label>
                    <input
                      type="number"
                      step="0.01"
                      min="0"
                      value={newRule.output_cost_per_mtok}
                      onChange={(e) => setNewRule((s) => ({ ...s, output_cost_per_mtok: Math.max(0, parseFloat(e.target.value) || 0) }))}
                      className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl
                                 text-text focus:outline-none focus:border-primary transition-all"
                    />
                  </div>
                </div>
                <div className="flex flex-col justify-end gap-2 sm:flex-row">
                  <button onClick={() => setAdding(false)}
                    className="min-h-10 px-3 text-xs rounded-lg border border-border hover:bg-surface-hover text-text-secondary"
                  >Cancel</button>
                  <motion.button whileHover={{ scale: 1.02 }} whileTap={{ scale: 0.98 }}
                    onClick={handleAdd}
                    className="btn-primary min-h-10 px-3 text-xs rounded-lg font-medium flex items-center justify-center gap-1"
                  ><Save size={12} /> Save</motion.button>
                </div>
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        {loading ? (
          <div className="flex items-center justify-center py-8">
            <RefreshCw size={16} className="animate-spin text-text-muted" />
          </div>
        ) : rules.length === 0 ? (
          <div className="text-center py-8">
            <p className="text-sm text-text-muted">No pricing rules defined</p>
            <p className="text-xs text-text-muted/60 mt-1">Add rules to track costs per model</p>
          </div>
        ) : (
          <div className="space-y-2">
            {rules.map((rule) => (
              <div
                key={rule.id}
                className="flex flex-col gap-3 p-3 rounded-xl bg-surface border border-border sm:flex-row sm:items-center sm:justify-between"
              >
                <div className="flex-1 min-w-0">
                  <span className="text-sm font-medium text-text break-words">{rule.provider_type}/{rule.model_pattern}</span>
                  <div className="flex flex-wrap gap-3 mt-0.5">
                    <span className="text-[11px] text-text-muted">In: ${rule.input_cost_per_mtok}/M</span>
                    <span className="text-[11px] text-text-muted">Out: ${rule.output_cost_per_mtok}/M</span>
                  </div>
                </div>
                <motion.button
                  whileHover={{ scale: 1.1 }}
                  whileTap={{ scale: 0.9 }}
                  onClick={() => handleDelete(rule.id)}
                  className="min-h-10 min-w-10 inline-flex items-center justify-center rounded-xl hover:bg-danger-soft text-text-muted hover:text-danger transition-all self-start sm:self-center"
                  aria-label={`Delete pricing rule for ${rule.provider_type}/${rule.model_pattern}`}
                  title="Delete pricing rule"
                >
                  <Trash2 size={14} />
                </motion.button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between text-sm">
      <span className="text-text-muted">{label}</span>
      <span className="text-text font-medium">{value}</span>
    </div>
  );
}

function ShortcutRow({ keys, desc }: { keys: string[]; desc: string }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-sm text-text-muted">{desc}</span>
      <div className="flex items-center gap-1">
        {keys.map((key, i) => (
          <span key={i}>
            <kbd className="px-2 py-1 text-[11px] font-mono bg-surface border border-border rounded-md text-text-secondary">
              {key}
            </kbd>
            {i < keys.length - 1 && <span className="text-text-muted mx-0.5">+</span>}
          </span>
        ))}
      </div>
    </div>
  );
}

// ── Auth Tab ──────────────────────────────────────────────────────────────

function AuthTab() {
  const [authEnabled, setAuthEnabled] = useState(false);
  const [, setHasUsers] = useState(false);
  const [loading, setLoading] = useState(true);
  const [registering, setRegistering] = useState(false);

  // Registration form
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [showPassword, setShowPassword] = useState(false);

  // Current user info
  const [currentUser, setCurrentUser] = useState<{ username: string; display_name: string; role: string } | null>(null);

  const fetchStatus = useCallback(async () => {
    try {
      const status = await authApi.status();
      setAuthEnabled(status.auth_enabled);
      setHasUsers(status.has_users);
      if (status.auth_enabled && status.has_users) {
        try {
          const user = await authApi.me();
          setCurrentUser({ username: user.username, display_name: user.display_name, role: user.role });
        } catch {
          // Token may be invalid
        }
      }
    } catch {
      // Auth endpoint unavailable
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchStatus();
  }, [fetchStatus]);

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!username.trim() || !password.trim()) return;
    if (password.length < 8) {
      toast.error('Password must be at least 8 characters');
      return;
    }

    setRegistering(true);
    try {
      const res = await authApi.register({
        username: username.trim(),
        password,
        display_name: displayName.trim() || undefined,
      });
      setAuthToken(res.token);
      toast.success('Admin account created! Multi-user mode is now active.');
      setUsername('');
      setPassword('');
      setDisplayName('');
      await fetchStatus();
    } catch (err) {
      toast.error((err as Error).message);
    } finally {
      setRegistering(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="w-6 h-6 border-2 border-primary/30 border-t-primary rounded-full animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Status card */}
      <div className="glass rounded-2xl p-5">
        <div className="flex items-center gap-3 mb-4">
          {authEnabled ? (
            <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-emerald-500/20 to-green-500/20 flex items-center justify-center">
              <Users size={18} className="text-emerald-400" />
            </div>
          ) : (
            <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-amber-500/20 to-yellow-500/20 flex items-center justify-center">
              <Lock size={18} className="text-amber-400" />
            </div>
          )}
          <div>
            <h3 className="text-sm font-bold text-text">
              {authEnabled ? 'Multi-User Mode Active' : 'Solo Mode (No Authentication)'}
            </h3>
            <p className="text-xs text-text-muted mt-0.5">
              {authEnabled
                ? 'Users registered'
                : 'Anyone with access to this URL can use OmniLLM-Studio'}
            </p>
          </div>
        </div>

        {authEnabled && currentUser && (
          <div className="border-t border-border pt-3 mt-3 space-y-2">
            <InfoRow label="Signed in as" value={currentUser.display_name || currentUser.username} />
            <InfoRow label="Username" value={currentUser.username} />
            <InfoRow label="Role" value={currentUser.role} />
          </div>
        )}
      </div>

      {/* Enable multi-user mode */}
      {!authEnabled && (
        <div className="glass rounded-2xl p-5">
          <div className="flex items-center gap-3 mb-4">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-primary/20 to-accent/20 flex items-center justify-center">
              <UserPlus size={18} className="text-primary" />
            </div>
            <div>
              <h3 className="text-sm font-bold">Enable Multi-User Mode</h3>
              <p className="text-xs text-text-muted mt-0.5">
                Register the first admin account to activate authentication
              </p>
            </div>
          </div>

          <div className="bg-amber-500/5 border border-amber-500/20 rounded-xl p-3 mb-4">
            <p className="text-xs text-amber-300/80">
              Once enabled, all users will need to sign in. The first account gets admin privileges.
              This cannot be undone from the UI.
            </p>
          </div>

          <form onSubmit={handleRegister} className="space-y-3">
            <div>
              <label className="block text-xs text-text-muted mb-1 font-medium">Username</label>
              <input
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="admin"
                className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl
                           text-text placeholder:text-text-muted/40 focus:outline-none focus:border-primary transition-all"
              />
            </div>
            <div>
              <label className="block text-xs text-text-muted mb-1 font-medium">Display Name (optional)</label>
              <input
                type="text"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                placeholder="Administrator"
                className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl
                           text-text placeholder:text-text-muted/40 focus:outline-none focus:border-primary transition-all"
              />
            </div>
            <div>
              <label className="block text-xs text-text-muted mb-1 font-medium">Password</label>
              <div className="relative">
                <input
                  type={showPassword ? 'text' : 'password'}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Min 8 characters"
                  className="w-full px-3 py-2 pr-10 text-sm bg-surface border border-border rounded-xl
                             text-text placeholder:text-text-muted/40 focus:outline-none focus:border-primary transition-all"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-text-muted hover:text-text"
                >
                  {showPassword ? <EyeOff size={14} /> : <Eye size={14} />}
                </button>
              </div>
            </div>

            <motion.button
              whileHover={{ scale: 1.01 }}
              whileTap={{ scale: 0.99 }}
              type="submit"
              disabled={registering || !username.trim() || password.length < 8}
              className="w-full py-2.5 rounded-xl btn-primary text-sm font-medium flex items-center justify-center gap-2
                         disabled:opacity-50 disabled:cursor-not-allowed mt-4"
            >
              {registering ? (
                <div className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
              ) : (
                <>
                  <UserPlus size={14} /> Create Admin &amp; Enable Auth
                </>
              )}
            </motion.button>
          </form>
        </div>
      )}

      {/* Info when enabled */}
      {authEnabled && (
        <div className="glass rounded-2xl p-5">
          <h3 className="text-sm font-bold text-text mb-3">Registration</h3>
          <p className="text-xs text-text-muted mb-3">
            New users can register at the login screen. The first user is always an admin.
            Subsequent users get the member role.
          </p>
          <div className="space-y-2">
            <ShortcutRow keys={['POST']} desc="/v1/auth/register — register new user" />
            <ShortcutRow keys={['POST']} desc="/v1/auth/logout — sign out" />
          </div>
        </div>
      )}
    </div>
  );
}
