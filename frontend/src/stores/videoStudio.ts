import { create } from 'zustand';
import { toast } from 'sonner';
import { videoApi } from '../api';
import type {
  VideoAsset,
  VideoGenerationDetail,
  VideoGenerationProgress,
  VideoModel,
  VideoProject,
  VideoPromptForm,
  VideoProviderInfo,
  VideoProviderKey,
} from '../types/video';

const DEFAULT_FORM: VideoPromptForm = {
  prompt: '',
  negative_prompt: '',
  aspect_ratio: '16:9',
  duration_seconds: 6,
  resolution: '1080p',
  fps: 30,
  camera_motion: 'slow push-in',
  shot_type: 'medium shot',
  style_preset: 'cinematic',
  production_notes: '',
  enhance: true,
  place_on_timeline: false,
};

const DEFAULT_MODELS: Record<VideoProviderKey, string> = {
  mock: 'mock-video-v1',
  openrouter: '',
  gemini: '',
  openai: '',
  custom: '',
};

function cloneForm(): VideoPromptForm {
  return { ...DEFAULT_FORM };
}

function getNewProjectTitle(): string {
  const now = new Date();
  return `Video Project - ${now.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })}`;
}

function upsertProject(items: VideoProject[], next: VideoProject): VideoProject[] {
  const idx = items.findIndex((item) => item.id === next.id);
  if (idx === -1) return [next, ...items];
  const copy = items.slice();
  copy[idx] = next;
  return copy;
}

function upsertGeneration(items: VideoGenerationDetail[], next: VideoGenerationDetail): VideoGenerationDetail[] {
  const idx = items.findIndex((item) => item.id === next.id);
  if (idx === -1) return [...items, next];
  const copy = items.slice();
  copy[idx] = next;
  return copy;
}

function upsertAsset(items: VideoAsset[], next: VideoAsset): VideoAsset[] {
  const idx = items.findIndex((item) => item.id === next.id);
  if (idx === -1) return [next, ...items];
  const copy = items.slice();
  copy[idx] = next;
  return copy;
}

function parseGenerationSettings(generation: VideoGenerationDetail | null | undefined): Partial<VideoPromptForm> {
  if (!generation?.settings_json) return {};
  try {
    const parsed = JSON.parse(generation.settings_json) as Partial<VideoPromptForm>;
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

interface VideoStudioState {
  projects: VideoProject[];
  activeProjectId: string | null;
  activeGenerationId: string | null;
  generations: VideoGenerationDetail[];
  assets: VideoAsset[];
  providers: VideoProviderInfo[];
  selectedProvider: VideoProviderKey;
  modelsByProvider: Record<VideoProviderKey, VideoModel[]>;
  selectedModel: string | null;
  promptForm: VideoPromptForm;
  isLoading: boolean;
  isGenerating: boolean;
  isEnhancing: boolean;
  generationProgress: VideoGenerationProgress | null;
  error: string | null;
  abortGeneration: (() => void) | null;

  loadProviders: () => Promise<void>;
  loadModels: (provider: VideoProviderKey, refresh?: boolean) => Promise<void>;
  loadProjects: () => Promise<void>;
  createProject: (title?: string) => Promise<VideoProject | null>;
  selectProject: (projectId: string) => Promise<void>;
  setProvider: (provider: VideoProviderKey) => Promise<void>;
  setModel: (model: string) => void;
  setPromptField: <K extends keyof VideoPromptForm>(key: K, value: VideoPromptForm[K]) => void;
  clearPrompt: () => void;
  enhancePrompt: () => Promise<void>;
  generate: (parentId?: string) => void;
  branchFromGeneration: (generationId: string) => Promise<void>;
  deleteProject: (projectId: string) => Promise<void>;
  stopGeneration: () => void;
}

export const useVideoStudioStore = create<VideoStudioState>((set, get) => ({
  projects: [],
  activeProjectId: null,
  activeGenerationId: null,
  generations: [],
  assets: [],
  providers: [],
  selectedProvider: 'mock',
  modelsByProvider: { mock: [], openrouter: [], gemini: [], openai: [], custom: [] },
  selectedModel: 'mock-video-v1',
  promptForm: cloneForm(),
  isLoading: false,
  isGenerating: false,
  isEnhancing: false,
  generationProgress: null,
  error: null,
  abortGeneration: null,

  loadProviders: async () => {
    try {
      const providers = await videoApi.providers();
      const available = providers.filter((provider) => provider.configured).map((provider) => provider.key);
      const current = get().selectedProvider;
      const selectedProvider = available.includes(current) ? current : available[0] || 'mock';
      set({ providers, selectedProvider });
      await get().loadModels(selectedProvider);
    } catch (err) {
      set({ error: (err as Error).message });
      toast.error('Could not load video providers');
    }
  },

  loadModels: async (provider, refresh = false) => {
    try {
      const models = refresh ? await videoApi.refreshModels(provider) : await videoApi.listModels(provider);
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
      toast.error('Could not load video models');
    }
  },

  loadProjects: async () => {
    set({ isLoading: true, error: null });
    try {
      const projects = await videoApi.listProjects();
      set({ projects, isLoading: false });
      if (!get().activeProjectId && projects.length > 0) {
        await get().selectProject(projects[0].id);
      } else if (projects.length === 0) {
        set({ activeProjectId: null, activeGenerationId: null, generations: [], assets: [] });
      }
    } catch (err) {
      set({ isLoading: false, error: (err as Error).message });
    }
  },

  createProject: async (title) => {
    const { selectedProvider, selectedModel } = get();
    try {
      const project = await videoApi.createProject({
        title: title || getNewProjectTitle(),
        provider: selectedProvider,
        model: selectedModel || undefined,
        width: 1920,
        height: 1080,
        fps: 30,
        aspect_ratio: '16:9',
      });
      set((state) => ({
        projects: [project, ...state.projects],
        activeProjectId: project.id,
        activeGenerationId: null,
        generations: [],
        assets: [],
      }));
      return project;
    } catch (err) {
      set({ error: (err as Error).message });
      toast.error('Could not create video project');
      return null;
    }
  },

  selectProject: async (projectId) => {
    set({ isLoading: true, error: null });
    try {
      const detail = await videoApi.getProject(projectId);
      const nextActiveGenerationId = detail.generations[detail.generations.length - 1]?.id || null;
      set((state) => ({
        projects: upsertProject(state.projects, detail.project),
        activeProjectId: detail.project.id,
        activeGenerationId: nextActiveGenerationId,
        generations: detail.generations,
        assets: detail.assets,
        selectedProvider: detail.project.default_provider || state.selectedProvider,
        selectedModel: detail.project.default_model || state.selectedModel,
        isLoading: false,
      }));
      if (detail.project.default_provider) {
        await get().loadModels(detail.project.default_provider);
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

  setPromptField: (key, value) => set((state) => ({
    promptForm: { ...state.promptForm, [key]: value },
  })),

  clearPrompt: () => set({ promptForm: cloneForm(), error: null, generationProgress: null }),

  enhancePrompt: async () => {
    const { promptForm } = get();
    if (!promptForm.prompt.trim()) {
      toast.error('Enter a video prompt first');
      return;
    }
    set({ isEnhancing: true, error: null });
    try {
      const response = await videoApi.enhancePrompt({
        prompt: promptForm.prompt,
        aspect_ratio: promptForm.aspect_ratio,
        duration_seconds: promptForm.duration_seconds,
        negative_prompt: promptForm.negative_prompt || undefined,
      });
      set((state) => ({
        promptForm: { ...state.promptForm, prompt: response.prompt, enhance: false },
        isEnhancing: false,
      }));
      toast.success('Prompt enhanced');
    } catch (err) {
      set({ isEnhancing: false, error: (err as Error).message });
      toast.error('Could not enhance prompt');
    }
  },

  generate: (parentId) => {
    const { selectedProvider, selectedModel, promptForm, activeProjectId } = get();
    if (!selectedProvider || !selectedModel) {
      toast.error('Choose a video provider and model');
      return;
    }
    if (!promptForm.prompt.trim()) {
      toast.error('Enter a video prompt');
      return;
    }
    set({
      isGenerating: true,
      generationProgress: { stage: 'queued', message: 'Preparing video generation' },
      error: null,
    });
    const stream = videoApi.generate({
      ...promptForm,
      provider: selectedProvider,
      model: selectedModel,
      prompt: promptForm.prompt.trim(),
      project_id: activeProjectId || undefined,
      parent_id: parentId,
    }, {
      onStarted: (progress) => set({ generationProgress: progress }),
      onProgress: (progress) => set({ generationProgress: progress }),
      onDone: (payload) => {
        set((state) => ({
          projects: upsertProject(state.projects, payload.project),
          activeProjectId: payload.project.id,
          activeGenerationId: payload.generation.id,
          generations: upsertGeneration(state.generations, payload.generation),
          assets: payload.asset ? upsertAsset(state.assets, payload.asset) : state.assets,
          isGenerating: false,
          generationProgress: { stage: 'done', message: 'Video generation complete', progress: 1 },
          abortGeneration: null,
        }));
        toast.success('Video asset generated');
      },
      onError: (payload) => {
        set({
          isGenerating: false,
          generationProgress: null,
          error: payload.error,
          abortGeneration: null,
        });
        toast.error(payload.error || 'Video generation failed');
      },
    });
    set({ abortGeneration: stream.abort });
  },

  branchFromGeneration: async (generationId) => {
    try {
      const branch = await videoApi.branchGeneration(generationId);
      const settings = parseGenerationSettings({
        id: branch.parent_id,
        project_id: branch.project_id,
        status: 'completed',
        provider: branch.provider,
        model: branch.model,
        prompt: branch.prompt,
        settings_json: branch.settings_json,
        created_at: '',
      });
      set((state) => ({
        activeProjectId: branch.project_id,
        activeGenerationId: branch.parent_id,
        selectedProvider: branch.provider,
        selectedModel: branch.model,
        promptForm: {
          ...state.promptForm,
          ...settings,
          prompt: branch.enhanced_prompt || branch.prompt,
          negative_prompt: branch.negative_prompt || state.promptForm.negative_prompt,
        },
      }));
      toast.success('Video prompt branched');
    } catch (err) {
      toast.error((err as Error).message);
    }
  },

  deleteProject: async (projectId) => {
    try {
      await videoApi.deleteProject(projectId);
      set((state) => {
        const projects = state.projects.filter((project) => project.id !== projectId);
        const activeDeleted = state.activeProjectId === projectId;
        return {
          projects,
          activeProjectId: activeDeleted ? projects[0]?.id || null : state.activeProjectId,
          activeGenerationId: activeDeleted ? null : state.activeGenerationId,
          generations: activeDeleted ? [] : state.generations,
          assets: activeDeleted ? [] : state.assets,
        };
      });
      toast.success('Video project deleted');
      const next = get().activeProjectId;
      if (next) {
        await get().selectProject(next);
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
