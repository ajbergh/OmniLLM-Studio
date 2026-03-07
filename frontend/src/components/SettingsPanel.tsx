import { useEffect, useState, useCallback } from 'react';
import { useProviderStore, useSettingsStore } from '../stores';
import { api, authApi, setAuthToken } from '../api';
import { X, Plus, Trash2, Eye, EyeOff, Save, Check, Shield, Zap, Globe, Server, Cloud, Cpu, ExternalLink, RefreshCw, Database, Wrench, DollarSign, UserPlus, Lock, Users, Palette } from 'lucide-react';
import { useTheme, THEMES } from '../theme';
import { clsx } from 'clsx';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import { getKnownChatModels, getKnownImageModels } from '../models';

const PROVIDER_TYPES = [
  { value: 'openai', label: 'OpenAI', icon: Zap, color: 'from-emerald-500/20 to-green-500/20', iconColor: 'text-emerald-400' },
  { value: 'anthropic', label: 'Anthropic', icon: Shield, color: 'from-orange-500/20 to-amber-500/20', iconColor: 'text-orange-400' },
  { value: 'gemini', label: 'Google Gemini', icon: Globe, color: 'from-blue-500/20 to-cyan-500/20', iconColor: 'text-blue-400' },
  { value: 'ollama', label: 'Ollama (Local)', icon: Server, color: 'from-purple-500/20 to-violet-500/20', iconColor: 'text-purple-400' },
  { value: 'openrouter', label: 'OpenRouter', icon: Cloud, color: 'from-pink-500/20 to-rose-500/20', iconColor: 'text-pink-400' },
  { value: 'groq', label: 'Groq', icon: Zap, color: 'from-yellow-500/20 to-amber-500/20', iconColor: 'text-yellow-400' },
  { value: 'together', label: 'Together AI', icon: Cpu, color: 'from-indigo-500/20 to-blue-500/20', iconColor: 'text-indigo-400' },
  { value: 'mistral', label: 'Mistral AI', icon: Globe, color: 'from-cyan-500/20 to-teal-500/20', iconColor: 'text-cyan-400' },
  { value: 'custom', label: 'Custom (OpenAI-compatible)', icon: Server, color: 'from-gray-500/20 to-slate-500/20', iconColor: 'text-gray-400' },
];

function getProviderMeta(type: string) {
  return PROVIDER_TYPES.find((t) => t.value === type) || PROVIDER_TYPES[PROVIDER_TYPES.length - 1];
}

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
  const [tab, setTab] = useState<'providers' | 'general' | 'appearance' | 'rag' | 'tools' | 'pricing' | 'auth'>('providers');

  useEffect(() => {
    if (settingsOpen) {
      fetchProviders();
      fetchSettings();
    }
  }, [settingsOpen, fetchProviders, fetchSettings]);

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
              >
                <X size={18} />
              </motion.button>
            </div>

            {/* Tabs */}
            <div className="flex mx-5 bg-surface-alt rounded-xl p-1 gap-1 overflow-x-auto">
              {([
                { key: 'providers', label: 'Providers' },
                { key: 'general', label: 'General' },
                { key: 'appearance', label: 'Appearance' },
                { key: 'rag', label: 'RAG' },
                { key: 'tools', label: 'Tools' },
                { key: 'pricing', label: 'Pricing' },
                { key: 'auth', label: 'Auth' },
              ] as const).map((t) => (
                <button
                  key={t.key}
                  onClick={() => setTab(t.key)}
                  className={clsx(
                    'shrink-0 min-w-[92px] px-3 py-2 text-sm font-medium rounded-lg transition-all duration-200',
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
      fetchOllama(newProvider.base_url || undefined);
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
      <div className="flex items-center justify-between">
        <p className="text-sm text-text-muted">Configure your AI provider connections.</p>
        <motion.button
          whileHover={{ scale: 1.03 }}
          whileTap={{ scale: 0.97 }}
          onClick={() => setAdding(true)}
          className="btn-primary flex items-center gap-1.5 px-4 py-2 text-xs rounded-xl font-medium"
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
                <div className="grid grid-cols-3 gap-2">
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
                        'p-3 rounded-xl text-left transition-all duration-200 border',
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

              <div className="grid grid-cols-2 gap-3">
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
                          className="flex-1 px-3 py-2.5 text-sm bg-surface border border-border rounded-xl
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
                          className="p-2.5 rounded-xl border border-border hover:bg-surface-hover
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
                        className="w-full px-3 py-2.5 text-sm bg-surface border border-border rounded-xl
                                   text-text focus:outline-none focus:border-primary transition-all input-glow"
                      >
                        <option value="">Select a model...</option>
                        {newProviderChatModels.map((m) => (
                          <option key={m} value={m}>{m}</option>
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
                        className="w-full px-3 py-2.5 text-sm bg-surface border border-border rounded-xl
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

              <div className="flex justify-end gap-2 pt-1">
                <button
                  onClick={() => setAdding(false)}
                  className="px-4 py-2 text-sm rounded-xl border border-border hover:bg-surface-hover
                             text-text-secondary transition-all"
                >
                  Cancel
                </button>
                <motion.button
                  whileHover={{ scale: 1.02 }}
                  whileTap={{ scale: 0.98 }}
                  onClick={handleAdd}
                  disabled={!newProvider.name}
                  className="btn-primary px-4 py-2 text-sm rounded-xl font-medium
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
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className={clsx(
            'w-9 h-9 rounded-xl bg-gradient-to-br flex items-center justify-center',
            meta.color
          )}>
            <meta.icon size={16} className={meta.iconColor} />
          </div>
          <div>
            <h3 className="text-sm font-semibold">{provider.name}</h3>
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
              'px-3 py-1 text-xs rounded-full font-medium transition-all border',
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
            className="p-2 rounded-xl hover:bg-danger-soft text-text-muted hover:text-danger transition-all"
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
              className="flex-1 px-3 py-2 text-sm bg-surface border border-border rounded-xl
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
              className="p-2 rounded-xl border border-border hover:bg-surface-hover
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
            className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl
                       text-text focus:outline-none focus:border-primary transition-all input-glow"
          >
            <option value="">Select a model...</option>
            {chatModelOptions.map((m) => (
              <option key={m} value={m}>{m}</option>
            ))}
            {provider.default_model && !chatModelOptions.includes(provider.default_model) && (
              <option value={provider.default_model}>{provider.default_model} (custom)</option>
            )}
          </select>
        </div>
      ) : (
        provider.default_model && (
          <div className="flex items-center gap-1.5 text-xs text-text-muted">
            <Cpu size={10} />
            <span className="font-medium">Model:</span> {provider.default_model}
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
            className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl
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
      <div className="flex items-center gap-2">
        <input
          type="password"
          value={apiKey}
          onChange={(e) => setApiKey(e.target.value)}
          placeholder="Update API key..."
          className="flex-1 px-3 py-2 text-sm bg-surface border border-border rounded-xl
                     text-text placeholder-text-muted focus:outline-none focus:border-primary
                     transition-all input-glow"
        />
        <motion.button
          whileHover={{ scale: 1.05 }}
          whileTap={{ scale: 0.95 }}
          onClick={handleSaveKey}
          disabled={!apiKey}
          className="p-2.5 rounded-xl btn-primary disabled:opacity-40 disabled:cursor-not-allowed"
        >
          {saved ? <Check size={14} /> : <Save size={14} />}
        </motion.button>
      </div>
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
          className="w-full px-3 py-2.5 text-sm bg-surface border border-border rounded-xl
                     text-text placeholder-text-muted focus:outline-none
                     transition-all pr-9 input-glow"
        />
        {secret && (
          <button
            type="button"
            onClick={() => setShow(!show)}
            className="absolute right-2.5 top-1/2 -translate-y-1/2 p-1 rounded-md
                       text-text-muted hover:text-text transition-colors"
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
  const [savingSearch, setSavingSearch] = useState(false);
  const [jinaReaderEnabled, setJinaReaderEnabled] = useState(settings.jina_reader_enabled);
  const [appVersion, setAppVersion] = useState('...');

  useEffect(() => {
    setWebSearchProvider(settings.web_search_provider || 'auto');
    setBraveApiKey(settings.brave_api_key || '');
    setJinaReaderEnabled(settings.jina_reader_enabled);
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
        jina_reader_enabled: jinaReaderEnabled,
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
            <div className="flex items-center justify-between py-2">
              <div>
                <label className="text-xs font-medium text-text-secondary block">Jina Reader</label>
                <p className="text-[10px] text-text-muted mt-0.5">
                  Fetches full page content for richer answers (free, no API key)
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
// RAG Settings Tab
// ============================================

function RAGTab() {
  const { settings, updateSettings } = useSettingsStore();
  const [ragEnabled, setRagEnabled] = useState(settings.rag_enabled ?? false);
  const [embeddingModel, setEmbeddingModel] = useState(settings.rag_embedding_model || 'text-embedding-3-small');
  const [chunkSize, setChunkSize] = useState(settings.rag_chunk_size ?? 512);
  const [chunkOverlap, setChunkOverlap] = useState(settings.rag_chunk_overlap ?? 64);
  const [topK, setTopK] = useState(settings.rag_top_k ?? 5);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setRagEnabled(settings.rag_enabled ?? false);
    setEmbeddingModel(settings.rag_embedding_model || 'text-embedding-3-small');
    setChunkSize(settings.rag_chunk_size ?? 512);
    setChunkOverlap(settings.rag_chunk_overlap ?? 64);
    setTopK(settings.rag_top_k ?? 5);
  }, [settings]);

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
            <>
              {/* Embedding model */}
              <div>
                <label className="text-xs font-medium text-text-secondary mb-1.5 block">Embedding Model</label>
                <select
                  value={embeddingModel}
                  onChange={(e) => setEmbeddingModel(e.target.value)}
                  className="w-full px-3 py-2 text-sm bg-surface border border-border rounded-xl
                             text-text focus:outline-none focus:border-primary/50 transition-colors"
                >
                  <option value="text-embedding-3-small">text-embedding-3-small (OpenAI)</option>
                  <option value="text-embedding-3-large">text-embedding-3-large (OpenAI)</option>
                  <option value="text-embedding-ada-002">text-embedding-ada-002 (OpenAI)</option>
                  <option value="nomic-embed-text">nomic-embed-text (Ollama)</option>
                  <option value="all-minilm">all-minilm (Ollama)</option>
                </select>
              </div>

              {/* Chunk size */}
              <div>
                <label className="text-xs font-medium text-text-secondary mb-1.5 block">
                  Chunk Size <span className="text-text-muted font-normal">({chunkSize} tokens)</span>
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
              </div>

              {/* Chunk overlap */}
              <div>
                <label className="text-xs font-medium text-text-secondary mb-1.5 block">
                  Chunk Overlap <span className="text-text-muted font-normal">({chunkOverlap} tokens)</span>
                </label>
                <input
                  type="range"
                  min={0}
                  max={256}
                  step={16}
                  value={chunkOverlap}
                  onChange={(e) => setChunkOverlap(Number(e.target.value))}
                  className="w-full accent-primary"
                />
                <div className="flex justify-between text-[10px] text-text-muted mt-0.5">
                  <span>0</span>
                  <span>256</span>
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
              </div>
            </>
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
    </div>
  );
}

// ============================================
// Tools Settings Tab
// ============================================

function ToolsTab() {
  const [tools, setTools] = useState<import('../types').ToolDefinition[]>([]);
  const [loading, setLoading] = useState(true);

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
      <div className="p-5 rounded-2xl bg-surface-alt border border-border">
        <div className="flex items-center gap-3 mb-4">
          <div className="w-10 h-10 rounded-2xl bg-gradient-to-br from-orange-500/20 to-amber-500/20 flex items-center justify-center shadow-md shadow-orange-500/10">
            <Wrench size={18} className="text-orange-400" />
          </div>
          <div>
            <h3 className="text-sm font-bold">Tool Permissions</h3>
            <p className="text-[11px] text-text-muted">Control which tools the AI can use during conversations</p>
          </div>
        </div>

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
          <div className="space-y-2">
            {tools.map((tool) => (
              <div
                key={tool.name}
                className="flex items-center justify-between p-3 rounded-xl bg-surface border border-border"
              >
                <div className="flex-1 min-w-0 mr-3">
                  <span className="text-sm font-medium text-text block truncate">{tool.name}</span>
                  {tool.description && (
                    <span className="text-[11px] text-text-muted block truncate">{tool.description}</span>
                  )}
                </div>
                <select
                  value={tool.policy || 'allow'}
                  onChange={(e) => updatePermission(tool.name, e.target.value)}
                  className="px-2 py-1.5 text-xs bg-surface-alt border border-border rounded-lg
                             text-text focus:outline-none focus:border-primary/50 transition-colors"
                >
                  <option value="allow">Allow</option>
                  <option value="deny">Deny</option>
                  <option value="ask">Ask</option>
                </select>
              </div>
            ))}
          </div>
        )}
      </div>
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

        <div className="flex items-center justify-between mb-3">
          <p className="text-xs text-text-muted">{rules.length} rule{rules.length !== 1 ? 's' : ''} configured</p>
          <motion.button
            whileHover={{ scale: 1.03 }}
            whileTap={{ scale: 0.97 }}
            onClick={() => setAdding(true)}
            className="btn-primary flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-xl font-medium"
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
                <div className="grid grid-cols-2 gap-3">
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
                <div className="grid grid-cols-2 gap-3">
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
                <div className="flex justify-end gap-2">
                  <button onClick={() => setAdding(false)}
                    className="px-3 py-1.5 text-xs rounded-lg border border-border hover:bg-surface-hover text-text-secondary"
                  >Cancel</button>
                  <motion.button whileHover={{ scale: 1.02 }} whileTap={{ scale: 0.98 }}
                    onClick={handleAdd}
                    className="btn-primary px-3 py-1.5 text-xs rounded-lg font-medium flex items-center gap-1"
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
                className="flex items-center justify-between p-3 rounded-xl bg-surface border border-border"
              >
                <div className="flex-1 min-w-0">
                  <span className="text-sm font-medium text-text">{rule.provider_type}/{rule.model_pattern}</span>
                  <div className="flex gap-3 mt-0.5">
                    <span className="text-[11px] text-text-muted">In: ${rule.input_cost_per_mtok}/M</span>
                    <span className="text-[11px] text-text-muted">Out: ${rule.output_cost_per_mtok}/M</span>
                  </div>
                </div>
                <motion.button
                  whileHover={{ scale: 1.1 }}
                  whileTap={{ scale: 0.9 }}
                  onClick={() => handleDelete(rule.id)}
                  className="p-2 rounded-xl hover:bg-danger-soft text-text-muted hover:text-danger transition-all"
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
              <h3 className="text-sm font-bold text-text">Enable Multi-User Mode</h3>
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
