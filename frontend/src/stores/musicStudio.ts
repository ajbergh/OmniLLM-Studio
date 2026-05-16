import { create } from 'zustand';
import { toast } from 'sonner';
import { musicApi } from '../api';
import type {
  MusicGenerationDetail,
  MusicGenerationProgress,
  MusicModel,
  MusicPromptForm,
  MusicProviderKey,
  MusicProvidersResponse,
  MusicSession,
} from '../types/music';

const DEFAULT_FORM: MusicPromptForm = {
  prompt: '',
  lyrics: '',
  instrumental: false,
  vocal_mode: 'auto',
  options: {
    genre: '',
    mood: '',
    era: '',
    instruments: [],
    scale: '',
    duration: '',
    structure: '',
    language: '',
    energy_curve: '',
    production_notes: '',
    negative_steer: '',
  },
};

const DEFAULT_MODELS: Record<MusicProviderKey, string> = {
  openrouter: 'google/lyria-3-clip-preview',
  gemini: 'lyria-3-clip-preview',
};

const PROVIDER_LABELS: Record<MusicProviderKey, string> = {
  openrouter: 'OpenRouter',
  gemini: 'Gemini',
};

function cloneForm(): MusicPromptForm {
  return {
    ...DEFAULT_FORM,
    options: { ...DEFAULT_FORM.options },
  };
}

function getNewMusicSessionTitle(): string {
  const now = new Date();
  return `Music Session - ${now.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })}`;
}

function hasAutoDateTimeTitleWithoutDescriptor(title: string): boolean {
  if (!title.startsWith('Music Session - ')) {
    return false;
  }
  const firstSeparator = title.indexOf(' - ');
  const secondSeparator = title.indexOf(' - ', firstSeparator + 3);
  return secondSeparator === -1;
}

function getGenerationDescriptor(generation: MusicGenerationDetail): string {
  const source = (generation.title || generation.prompt || '').replace(/\s+/g, ' ').trim();
  if (!source) {
    return 'Generated Track';
  }
  const cleaned = source.replace(/[\\/:*?"<>|]/g, '').trim();
  if (!cleaned) {
    return 'Generated Track';
  }
  return cleaned.length > 48 ? `${cleaned.slice(0, 48).trimEnd()}...` : cleaned;
}

function enabledProviders(providers: MusicProvidersResponse): MusicProviderKey[] {
  const out: MusicProviderKey[] = [];
  if (providers.openrouter) out.push('openrouter');
  if (providers.gemini) out.push('gemini');
  return out;
}

function upsertGeneration(items: MusicGenerationDetail[], next: MusicGenerationDetail): MusicGenerationDetail[] {
  const idx = items.findIndex((item) => item.id === next.id);
  if (idx === -1) {
    return [...items, next];
  }
  const copy = items.slice();
  copy[idx] = next;
  return copy;
}

function getSessionPromptForm(
  generations: MusicGenerationDetail[],
  activeGenerationId?: string | null,
): MusicPromptForm {
  const preferred = activeGenerationId
    ? generations.find((generation) => generation.id === activeGenerationId)
    : undefined;
  const latest = generations[generations.length - 1];
  const source = preferred || latest;
  if (!source) {
    return cloneForm();
  }
  return {
    ...cloneForm(),
    prompt: source.prompt || '',
    lyrics: source.lyrics || '',
    options: {
      ...cloneForm().options,
      structure: source.structure || '',
    },
  };
}

function getPromptFormFromGeneration(generation: MusicGenerationDetail | null | undefined): MusicPromptForm {
  if (!generation) {
    return cloneForm();
  }
  return {
    ...cloneForm(),
    prompt: generation.prompt || '',
    lyrics: generation.lyrics || '',
    options: {
      ...cloneForm().options,
      structure: generation.structure || '',
    },
  };
}

interface MusicStudioState {
  sessions: MusicSession[];
  activeSessionId: string | null;
  activeGenerationId: string | null;
  generations: MusicGenerationDetail[];
  providers: MusicProvidersResponse;
  selectedProvider: MusicProviderKey | null;
  modelsByProvider: Record<MusicProviderKey, MusicModel[]>;
  selectedModel: string | null;
  promptForm: MusicPromptForm;
  isLoading: boolean;
  isGenerating: boolean;
  generationProgress: MusicGenerationProgress | null;
  error: string | null;
  abortGeneration: (() => void) | null;

  loadProviders: () => Promise<void>;
  loadModels: (provider: MusicProviderKey, refresh?: boolean) => Promise<void>;
  loadSessions: () => Promise<void>;
  createSession: (title?: string) => Promise<MusicSession | null>;
  selectSession: (sessionId: string) => Promise<void>;
  setProvider: (provider: MusicProviderKey) => Promise<void>;
  setModel: (model: string) => void;
  setActiveGeneration: (generationId: string) => void;
  setPromptField: <K extends keyof MusicPromptForm>(key: K, value: MusicPromptForm[K]) => void;
  setOption: <K extends keyof MusicPromptForm['options']>(key: K, value: MusicPromptForm['options'][K]) => void;
  clearPrompt: () => void;
  generate: (parentId?: string) => void;
  branchFromGeneration: (generationId: string) => Promise<void>;
  regenerateFromGeneration: (generationId: string) => void;
  deleteSession: (sessionId: string) => Promise<void>;
  stopGeneration: () => void;
}

export const useMusicStudioStore = create<MusicStudioState>((set, get) => ({
  sessions: [],
  activeSessionId: null,
  activeGenerationId: null,
  generations: [],
  providers: { openrouter: false, gemini: false },
  selectedProvider: null,
  modelsByProvider: { openrouter: [], gemini: [] },
  selectedModel: null,
  promptForm: cloneForm(),
  isLoading: false,
  isGenerating: false,
  generationProgress: null,
  error: null,
  abortGeneration: null,

  loadProviders: async () => {
    try {
      const providers = await musicApi.providers();
      const available = enabledProviders(providers);
      const current = get().selectedProvider;
      const selectedProvider = current && available.includes(current) ? current : available[0] ?? null;
      set({ providers, selectedProvider });
      if (selectedProvider) {
        await get().loadModels(selectedProvider);
      } else {
        set({ selectedModel: null });
      }
    } catch (err) {
      set({ error: (err as Error).message });
    }
  },

  loadModels: async (provider, refresh = false) => {
    try {
      const models = refresh ? await musicApi.refreshModels(provider) : await musicApi.listModels(provider);
      const currentModel = get().selectedProvider === provider ? get().selectedModel : null;
      const selectedModel = currentModel && models.some((model) => model.id === currentModel)
        ? currentModel
        : models.find((model) => model.id === DEFAULT_MODELS[provider])?.id || models[0]?.id || null;
      set((state) => ({
        modelsByProvider: { ...state.modelsByProvider, [provider]: models },
        selectedProvider: provider,
        selectedModel,
      }));
    } catch (err) {
      set({ error: (err as Error).message });
      toast.error(`Could not load ${PROVIDER_LABELS[provider]} music models`);
    }
  },

  loadSessions: async () => {
    set({ isLoading: true, error: null });
    try {
      const sessions = await musicApi.listSessions();
      set({ sessions, isLoading: false });
      if (!get().activeSessionId && sessions.length > 0) {
        await get().selectSession(sessions[0].id);
      } else if (sessions.length === 0) {
        set({
          activeSessionId: null,
          activeGenerationId: null,
          generations: [],
          promptForm: cloneForm(),
        });
      }
    } catch (err) {
      set({ isLoading: false, error: (err as Error).message });
    }
  },

  createSession: async (title) => {
    const { selectedProvider, selectedModel } = get();
    try {
      const session = await musicApi.createSession({
        title: title || getNewMusicSessionTitle(),
        provider: selectedProvider || undefined,
        model: selectedModel || undefined,
      });
      set((state) => ({
        sessions: [session, ...state.sessions],
        activeSessionId: session.id,
        activeGenerationId: session.active_generation_id || null,
        generations: [],
        promptForm: cloneForm(),
      }));
      return session;
    } catch (err) {
      set({ error: (err as Error).message });
      toast.error('Could not create music session');
      return null;
    }
  },

  selectSession: async (sessionId) => {
    set({ isLoading: true, error: null });
    try {
      const detail = await musicApi.getSession(sessionId);
      const nextActiveGenerationId = detail.session.active_generation_id || detail.generations[detail.generations.length - 1]?.id || null;
      set((state) => ({
        sessions: upsertSession(state.sessions, detail.session),
        activeSessionId: detail.session.id,
        activeGenerationId: nextActiveGenerationId,
        generations: detail.generations,
        selectedProvider: detail.session.default_provider || state.selectedProvider,
        selectedModel: detail.session.default_model || state.selectedModel,
        promptForm: getSessionPromptForm(detail.generations, nextActiveGenerationId),
        isLoading: false,
      }));
      const provider = detail.session.default_provider;
      if (provider) {
        await get().loadModels(provider);
      }
    } catch (err) {
      set({ isLoading: false, error: (err as Error).message });
    }
  },

  setProvider: async (provider) => {
    set({ selectedProvider: provider, selectedModel: null });
    await get().loadModels(provider);
  },

  setModel: (model) => set({ selectedModel: model }),

  setActiveGeneration: (generationId) => set((state) => {
    const generation = state.generations.find((item) => item.id === generationId);
    return {
      activeGenerationId: generationId,
      promptForm: getPromptFormFromGeneration(generation),
    };
  }),

  setPromptField: (key, value) => set((state) => ({
    promptForm: { ...state.promptForm, [key]: value },
  })),

  setOption: (key, value) => set((state) => ({
    promptForm: {
      ...state.promptForm,
      options: { ...state.promptForm.options, [key]: value },
    },
  })),

  clearPrompt: () => set({ promptForm: cloneForm(), error: null, generationProgress: null }),

  generate: (parentId) => {
    const { selectedProvider, selectedModel, promptForm, activeSessionId } = get();
    if (!selectedProvider || !selectedModel) {
      toast.error('Choose a music provider and Lyria model');
      return;
    }
    if (!promptForm.prompt.trim()) {
      toast.error('Enter a music prompt');
      return;
    }
    set({
      isGenerating: true,
      generationProgress: { stage: 'queued', message: 'Preparing music generation' },
      error: null,
    });
    const stream = musicApi.generate({
      provider: selectedProvider,
      model: selectedModel,
      prompt: promptForm.prompt.trim(),
      lyrics: promptForm.lyrics.trim() || undefined,
      instrumental: promptForm.instrumental || promptForm.vocal_mode === 'instrumental',
      vocal_mode: promptForm.vocal_mode,
      options: promptForm.options,
      session_id: activeSessionId || undefined,
      parent_id: parentId,
    }, {
      onStarted: (progress) => set({ generationProgress: progress }),
      onProgress: (progress) => set({ generationProgress: progress }),
      onDone: (payload) => {
        const stateBefore = get();
        const currentSession = stateBefore.sessions.find((session) => session.id === payload.session.id);
        const currentSessionTitle = currentSession?.title || payload.session.title;
        const shouldAutoRename = stateBefore.generations.length === 0 && hasAutoDateTimeTitleWithoutDescriptor(currentSessionTitle);

        set((state) => ({
          sessions: upsertSession(state.sessions, payload.session),
          activeSessionId: payload.session.id,
          activeGenerationId: payload.generation.id,
          generations: upsertGeneration(state.generations, payload.generation),
          isGenerating: false,
          generationProgress: { stage: 'done', message: 'Music generation complete' },
          abortGeneration: null,
        }));

        if (shouldAutoRename) {
          const nextTitle = `${currentSessionTitle} - ${getGenerationDescriptor(payload.generation)}`;
          void musicApi.updateSession(payload.session.id, { title: nextTitle })
            .then((updatedSession) => {
              set((state) => ({ sessions: upsertSession(state.sessions, updatedSession) }));
            })
            .catch(() => {
              // Non-fatal: keep generation success even if auto-rename fails.
            });
        }

        toast.success('Track generated');
      },
      onError: (payload) => {
        set({
          isGenerating: false,
          generationProgress: null,
          error: payload.error,
          abortGeneration: null,
        });
        toast.error(payload.error || 'Music generation failed');
      },
    });
    set({ abortGeneration: stream.abort });
  },

  branchFromGeneration: async (generationId) => {
    try {
      const branch = await musicApi.branchGeneration(generationId);
      set({
        activeSessionId: branch.session_id,
        activeGenerationId: branch.parent_id,
        selectedProvider: branch.provider,
        selectedModel: branch.model,
        promptForm: {
          ...cloneForm(),
          prompt: branch.prompt || branch.assembled_prompt,
        },
      });
      toast.success('Prompt branched');
    } catch (err) {
      toast.error((err as Error).message);
    }
  },

  regenerateFromGeneration: (generationId) => {
    const generation = get().generations.find((item) => item.id === generationId);
    if (!generation) return;
    set({
      activeGenerationId: generation.id,
      selectedProvider: generation.provider,
      selectedModel: generation.model,
      promptForm: {
        ...cloneForm(),
        prompt: generation.prompt,
      },
    });
    get().generate(generation.id);
  },

  deleteSession: async (sessionId) => {
    try {
      await musicApi.deleteSession(sessionId);
      set((state) => {
        const sessions = state.sessions.filter((session) => session.id !== sessionId);
        const nextActive = state.activeSessionId === sessionId ? sessions[0]?.id || null : state.activeSessionId;
        return {
          sessions,
          activeSessionId: nextActive,
          activeGenerationId: state.activeSessionId === sessionId ? null : state.activeGenerationId,
          generations: state.activeSessionId === sessionId ? [] : state.generations,
          promptForm: state.activeSessionId === sessionId ? cloneForm() : state.promptForm,
        };
      });
      toast.success('Music session deleted');
      const next = get().activeSessionId;
      if (next) {
        await get().selectSession(next);
      }
    } catch (err) {
      toast.error((err as Error).message);
    }
  },

  stopGeneration: () => {
    get().abortGeneration?.();
    set({
      isGenerating: false,
      generationProgress: null,
      abortGeneration: null,
    });
  },
}));

function upsertSession(items: MusicSession[], next: MusicSession): MusicSession[] {
  const idx = items.findIndex((item) => item.id === next.id);
  if (idx === -1) {
    return [next, ...items];
  }
  const copy = items.slice();
  copy[idx] = next;
  return copy;
}
