import { useEffect, useState, useRef } from 'react';
import { useProviderStore, useConversationStore } from '../stores';
import { ChevronDown, Check, Cpu, Search, Plus, Loader2 } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';
import { clsx } from 'clsx';
import { toast } from 'sonner';
import { api } from '../api';
import { KNOWN_MODELS, getKnownChatModels, isFreeModel, getModelToolCallingSupport, getProviderToolCallingSupport } from '../models';

interface Props {
  conversationId: string;
}

function FreeModelBadge() {
  return (
    <span className="shrink-0 rounded-md border border-success/30 bg-success/10 px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-wide text-success">
      FREE
    </span>
  );
}

function NoToolsBadge({ title }: { title?: string }) {
  return (
    <span
      title={title ?? 'This model does not support function calling — tools like web search and browser are unavailable'}
      className="shrink-0 rounded-md border border-amber-500/30 bg-amber-500/10 px-1.5 py-0.5 text-[9px] font-medium uppercase tracking-wide text-amber-500/70"
    >
      no tools
    </span>
  );
}

export function ModelSelector({ conversationId }: Props) {
  const { providers, fetchProviders } = useProviderStore();
  const { conversations, updateConversation } = useConversationStore();
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [customModel, setCustomModel] = useState('');
  const [showCustomInput, setShowCustomInput] = useState(false);
  const [ollamaModels, setOllamaModels] = useState<string[]>([]);
  const [loadingOllama, setLoadingOllama] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    fetchProviders();
  }, [fetchProviders]);

  // Fetch Ollama models when dropdown opens and there's an Ollama provider
  useEffect(() => {
    if (!open) return;
    const ollamaProvider = providers.find(
      (p) => p.enabled && p.type.toLowerCase() === 'ollama'
    );
    if (ollamaProvider) {
      setLoadingOllama(true);
      api
        .fetchOllamaModels(ollamaProvider.base_url)
        .then((models) => {
          setOllamaModels(models);
          setLoadingOllama(false);
        })
        .catch(() => setLoadingOllama(false));
    }
  }, [open, providers]);

  // Focus search when dropdown opens
  useEffect(() => {
    if (open) {
      setTimeout(() => searchRef.current?.focus(), 100);
    } else {
      setSearch('');
      setShowCustomInput(false);
      setCustomModel('');
    }
  }, [open]);

  const convo = conversations.find((c) => c.id === conversationId);
  const currentProvider = convo?.default_provider;
  const currentModel = convo?.default_model;
  const enabledProviders = providers.filter((p) => p.enabled);

  // Derive the effective model to display: prefer the conversation's stored model,
  // then fall back to the first enabled provider's default_model.
  const fallbackProvider = enabledProviders[0];
  const effectiveModel = currentModel || fallbackProvider?.default_model || fallbackProvider?.name;
  const effectiveProvider = currentProvider || fallbackProvider?.id;
  const effectiveProviderProfile = enabledProviders.find(
    (p) => p.id === effectiveProvider || p.name === effectiveProvider
  ) || fallbackProvider;
  const effectiveModelIsFree = isFreeModel(effectiveProviderProfile?.type || '', effectiveModel);

  const handleSelect = async (providerId: string, model: string) => {
    await updateConversation(conversationId, {
      default_provider: providerId,
      default_model: model,
    });
    setOpen(false);
    toast.success(`Switched to ${model}`);
  };

  const handleCustomSubmit = async (providerId: string) => {
    if (!customModel.trim()) return;
    await handleSelect(providerId, customModel.trim());
    setCustomModel('');
    setShowCustomInput(false);
  };

  const displayLabel = effectiveModel || effectiveProvider || 'Select model';

  // Get models for a provider, using dynamic Ollama models when available
  const getModels = (providerType: string): string[] => {
    const type = providerType.toLowerCase();
    if (type === 'ollama') {
      return ollamaModels.length > 0
        ? ollamaModels
        : KNOWN_MODELS.ollama;
    }
    return getKnownChatModels(type);
  };

  // Filter models by search query
  const filterModels = (models: string[]): string[] => {
    if (!search.trim()) return models;
    const q = search.toLowerCase();
    return models.filter((m) => m.toLowerCase().includes(q));
  };

  return (
    <div className="relative">
      <motion.button
        whileHover={{ scale: 1.02 }}
        whileTap={{ scale: 0.98 }}
        onClick={() => setOpen(!open)}
        className={clsx(
          'flex items-center gap-2 px-3.5 py-2 text-xs rounded-xl',
          'border transition-all duration-200',
          open
            ? 'bg-primary/10 border-primary/30 text-text'
            : 'bg-surface-alt border-border hover:border-primary/20 text-text-secondary hover:text-text'
        )}
      >
        <Cpu size={13} className={open ? 'text-primary' : 'text-text-muted'} />
        <span className="truncate max-w-[180px] font-medium">{displayLabel}</span>
        {effectiveModelIsFree && <FreeModelBadge />}
        <ChevronDown
          size={13}
          className={clsx(
            'transition-transform duration-200',
            open && 'rotate-180'
          )}
        />
      </motion.button>

      <AnimatePresence>
        {open && (
          <>
            <div className="fixed inset-0 z-40" onClick={() => setOpen(false)} />
            <motion.div
              ref={dropdownRef}
              initial={{ opacity: 0, y: -8, scale: 0.96 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, y: -8, scale: 0.96 }}
              transition={{ duration: 0.15, ease: 'easeOut' }}
              className="absolute right-0 top-full mt-2 z-50 rounded-2xl shadow-lg
                          min-w-[280px] max-w-[340px] bg-surface-raised border border-border
                          flex flex-col"
              style={{ maxHeight: 'min(420px, calc(100vh - 120px))' }}
            >
              {/* Search bar */}
              <div className="p-2 border-b border-border shrink-0">
                <div className="relative">
                  <Search size={12} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-text-muted" />
                  <input
                    ref={searchRef}
                    type="text"
                    placeholder="Search models..."
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    className="w-full pl-7 pr-3 py-1.5 text-xs bg-surface-alt border border-border rounded-lg
                               text-text placeholder-text-muted focus:outline-none focus:border-primary/30
                               transition-colors"
                  />
                </div>
              </div>

              {/* Scrollable model list */}
              <div className="overflow-y-auto overscroll-contain flex-1 py-1">
                {enabledProviders.length === 0 ? (
                  <div className="px-4 py-6 text-center">
                    <Cpu size={20} className="mx-auto mb-2 text-text-muted/40" />
                    <p className="text-xs text-text-muted">No providers configured.</p>
                    <p className="text-[10px] text-text-muted/60 mt-1">Add one in Settings</p>
                  </div>
                ) : (
                  enabledProviders.map((provider) => {
                    const models = getModels(provider.type);
                    const filtered = filterModels(models);
                    const isOllama = provider.type.toLowerCase() === 'ollama';

                    // Skip provider section if all models filtered out and no search match
                    if (search && filtered.length === 0 && !provider.type.toLowerCase().includes(search.toLowerCase())) {
                      return null;
                    }

                    return (
                      <div key={provider.id} className="mb-1">
                        <div className="px-3 py-1.5 text-[10px] font-bold uppercase tracking-widest text-text-muted/50 flex items-center gap-2 sticky top-0 bg-surface-raised z-10">
                          <div className="w-1.5 h-1.5 rounded-full bg-success/60" />
                          {provider.name}
                          {isOllama && loadingOllama && (
                            <Loader2 size={10} className="animate-spin text-text-muted/40" />
                          )}
                          {!getProviderToolCallingSupport(provider.type) && (
                            <NoToolsBadge title={`${provider.name} models do not currently support function calling — web search, browser tools, and other tool use are unavailable`} />
                          )}
                        </div>

                        {/* Dynamic/known models */}
                        {filtered.map((model) => {
                          const isActive = effectiveModel === model && effectiveProvider === provider.id;
                          const modelIsFree = isFreeModel(provider.type, model);
                          return (
                            <button
                              key={model}
                              onClick={() => handleSelect(provider.id, model)}
                              className={clsx(
                                'w-full text-left px-3 py-1.5 text-[13px] transition-all flex items-center justify-between gap-2 rounded-lg mx-1',
                                isActive
                                  ? 'text-primary font-medium bg-primary/5'
                                  : 'text-text-secondary hover:text-text hover:bg-surface-hover'
                              )}
                              style={{ width: 'calc(100% - 8px)' }}
                            >
                              <span className="min-w-0 flex flex-1 items-center gap-1.5">
                                <span className="truncate">{model}</span>
                                {modelIsFree && <FreeModelBadge />}
                                {getProviderToolCallingSupport(provider.type) && !getModelToolCallingSupport(provider.type, model) && (
                                  <NoToolsBadge />
                                )}
                              </span>
                              {isActive && <Check size={12} className="text-primary shrink-0" />}
                            </button>
                          );
                        })}

                        {/* Show provider's default_model if not in known list */}
                        {provider.default_model &&
                          !models.includes(provider.default_model) &&
                          (!search || provider.default_model.toLowerCase().includes(search.toLowerCase())) && (
                            <button
                              onClick={() => handleSelect(provider.id, provider.default_model!)}
                              className={clsx(
                                'w-full text-left px-3 py-1.5 text-[13px] hover:bg-surface-hover text-text-secondary',
                                'hover:text-text transition-all mx-1 rounded-lg flex items-center justify-between gap-2',
                                effectiveModel === provider.default_model && effectiveProvider === provider.id
                                  ? 'text-primary font-medium bg-primary/5'
                                  : ''
                            )}
                            style={{ width: 'calc(100% - 8px)' }}
                          >
                              <span className="min-w-0 flex flex-1 items-center gap-1.5">
                                <span className="truncate">{provider.default_model}</span>
                                {isFreeModel(provider.type, provider.default_model) && <FreeModelBadge />}
                                {getProviderToolCallingSupport(provider.type) && !getModelToolCallingSupport(provider.type, provider.default_model!) && (
                                  <NoToolsBadge />
                                )}
                              </span>
                              {effectiveModel === provider.default_model && effectiveProvider === provider.id && (
                                <Check size={12} className="text-primary shrink-0" />
                              )}
                            </button>
                          )}

                        {/* Custom model input */}
                        {showCustomInput && (
                          <div className="px-2 py-1 mx-1">
                            <div className="flex items-center gap-1">
                              <input
                                type="text"
                                placeholder="Model name..."
                                value={customModel}
                                onChange={(e) => setCustomModel(e.target.value)}
                                onKeyDown={(e) => {
                                  if (e.key === 'Enter') handleCustomSubmit(provider.id);
                                  if (e.key === 'Escape') { setShowCustomInput(false); setCustomModel(''); }
                                }}
                                className="flex-1 px-2 py-1 text-xs bg-surface-alt border border-border rounded-md
                                           text-text placeholder-text-muted focus:outline-none focus:border-primary/30"
                                autoFocus
                              />
                              <button
                                onClick={() => handleCustomSubmit(provider.id)}
                                className="px-2 py-1 text-[10px] rounded-md bg-primary/10 text-primary
                                           hover:bg-primary/20 transition-colors font-medium"
                              >
                                Use
                              </button>
                            </div>
                          </div>
                        )}
                      </div>
                    );
                  })
                )}
              </div>

              {/* Footer with custom model option */}
              <div className="p-2 border-t border-border shrink-0">
                <button
                  onClick={() => setShowCustomInput(!showCustomInput)}
                  className="w-full flex items-center gap-2 px-2 py-1.5 text-[11px] text-text-muted
                             hover:text-text hover:bg-surface-hover rounded-lg transition-colors"
                >
                  <Plus size={12} />
                  Use custom model
                </button>
              </div>
            </motion.div>
          </>
        )}
      </AnimatePresence>
    </div>
  );
}
