import { create } from 'zustand';
import { toast } from 'sonner';
import { videoApi } from '../api';
import type {
  VideoAsset,
  VideoEditPlan,
  VideoExportSettings,
  VideoGenerationDetail,
  VideoGenerationProgress,
  VideoModel,
  VideoProject,
  VideoPromptForm,
  VideoProviderInfo,
  VideoProviderKey,
  VideoRenderJob,
  VideoSocialVariant,
  VideoStoryboardResponse,
  VideoTimelineClip,
  VideoTimelineDocument,
  VideoTimelineEffect,
  VideoTimelineKeyframe,
  VideoTimelineRecord,
  VideoTimelineTrack,
  VideoTimelineTrackType,
  VideoTimelineTransform,
  VideoTimelineTransition,
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

const DEFAULT_EXPORT: VideoExportSettings = {
  format: 'mp4',
  codec: 'h264',
  resolution: 'project',
  fps: 30,
  quality: 'standard',
  include_audio: true,
  register_in_file_library: false,
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

function newId(prefix: string): string {
  const id = globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  return `${prefix}-${id}`;
}

function cloneTimeline(doc: VideoTimelineDocument): VideoTimelineDocument {
  return JSON.parse(JSON.stringify(doc)) as VideoTimelineDocument;
}

function defaultTimeline(project?: VideoProject | null): VideoTimelineDocument {
  return {
    version: 1,
    canvas: {
      width: project?.width || 1920,
      height: project?.height || 1080,
      fps: project?.fps || 30,
      background: '#000000',
    },
    duration_ms: Math.max(project?.duration_ms || 0, 30000),
    tracks: [
      { id: 'track-video-1', type: 'video', name: 'Video 1', locked: false, muted: false, visible: true, clips: [] },
      { id: 'track-overlay-1', type: 'image', name: 'Overlay 1', locked: false, muted: false, visible: true, clips: [] },
      { id: 'track-audio-1', type: 'audio', name: 'Audio 1', locked: false, muted: false, visible: true, clips: [] },
      { id: 'track-text-1', type: 'text', name: 'Text 1', locked: false, muted: false, visible: true, clips: [] },
    ],
    markers: [],
    metadata: {},
  };
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

function upsertRenderJob(items: VideoRenderJob[], next: VideoRenderJob): VideoRenderJob[] {
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

function assetTrackType(asset?: VideoAsset): VideoTimelineTrackType {
  if (!asset) return 'video';
  if (asset.kind === 'music') return 'music';
  if (asset.kind === 'audio') return 'audio';
  if (asset.kind === 'image') return 'image';
  if (asset.kind === 'text') return 'text';
  if (asset.kind === 'caption') return 'caption';
  return 'video';
}

function defaultTransform(): VideoTimelineTransform {
  return { x: 0, y: 0, scale: 1, rotation: 0, opacity: 1 };
}

function recomputeDuration(doc: VideoTimelineDocument): VideoTimelineDocument {
  let end = 0;
  for (const track of doc.tracks) {
    for (const clip of track.clips) {
      end = Math.max(end, clip.start_ms + clip.duration_ms);
    }
  }
  doc.duration_ms = Math.max(1000, end, doc.duration_ms || 30000);
  return doc;
}

function findClip(doc: VideoTimelineDocument, clipId: string): { track: VideoTimelineTrack; clip: VideoTimelineClip; trackIndex: number; clipIndex: number } | null {
  for (let trackIndex = 0; trackIndex < doc.tracks.length; trackIndex += 1) {
    const track = doc.tracks[trackIndex];
    const clipIndex = track.clips.findIndex((clip) => clip.id === clipId);
    if (clipIndex !== -1) return { track, clip: track.clips[clipIndex], trackIndex, clipIndex };
  }
  return null;
}

interface VideoStudioState {
  projects: VideoProject[];
  activeProjectId: string | null;
  activeGenerationId: string | null;
  selectedAssetId: string | null;
  generations: VideoGenerationDetail[];
  assets: VideoAsset[];
  providers: VideoProviderInfo[];
  selectedProvider: VideoProviderKey;
  modelsByProvider: Record<VideoProviderKey, VideoModel[]>;
  selectedModel: string | null;
  promptForm: VideoPromptForm;
  timelineRecord: VideoTimelineRecord | null;
  timeline: VideoTimelineDocument | null;
  selectedClipId: string | null;
  selectedTrackId: string | null;
  playheadMs: number;
  zoom: number;
  isPlaying: boolean;
  snappingEnabled: boolean;
  renderJobs: VideoRenderJob[];
  activeRenderJobId: string | null;
  exportSettings: VideoExportSettings;
  assistantInstruction: string;
  assistantPlan: VideoEditPlan | null;
  storyboard: VideoStoryboardResponse | null;
  socialVariants: VideoSocialVariant[];
  isLoading: boolean;
  isGenerating: boolean;
  isEnhancing: boolean;
  isSavingTimeline: boolean;
  isRendering: boolean;
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
  selectAsset: (assetId: string | null) => void;

  loadTimeline: (projectId?: string) => Promise<void>;
  saveTimeline: (document?: VideoTimelineDocument) => Promise<void>;
  addAssetToTimeline: (assetId: string, options?: { track_id?: string; track_type?: VideoTimelineTrackType; start_ms?: number; duration_ms?: number }) => Promise<void>;
  moveClip: (clipId: string, trackId: string, startMs: number) => Promise<void>;
  trimClip: (clipId: string, updates: Partial<Pick<VideoTimelineClip, 'start_ms' | 'duration_ms' | 'trim_in_ms' | 'trim_out_ms'>>) => Promise<void>;
  splitClipAtPlayhead: () => Promise<void>;
  deleteClip: (clipId?: string) => Promise<void>;
  duplicateClip: (clipId?: string) => Promise<void>;
  selectClip: (clipId: string | null, trackId?: string | null) => void;
  setPlayhead: (timeMs: number) => void;
  setZoom: (zoom: number) => void;
  setPlaying: (playing: boolean) => void;
  toggleSnapping: () => void;
  toggleTrackMute: (trackId: string) => Promise<void>;
  toggleTrackLock: (trackId: string) => Promise<void>;
  toggleTrackVisibility: (trackId: string) => Promise<void>;
  updateClipTransform: (clipId: string, transform: Partial<VideoTimelineTransform>) => Promise<void>;
  updateClipVolume: (clipId: string, volume: number) => Promise<void>;
  updateClipFade: (clipId: string, fade: { fade_in_ms?: number; fade_out_ms?: number }) => Promise<void>;
  addTextClip: (text?: string) => Promise<void>;
  updateClipText: (clipId: string, text: Partial<NonNullable<VideoTimelineClip['text']>>) => Promise<void>;
  addClipEffect: (clipId: string, effect: Omit<VideoTimelineEffect, 'id'>) => Promise<void>;
  toggleClipEffect: (clipId: string, effectId: string) => Promise<void>;
  removeClipEffect: (clipId: string, effectId: string) => Promise<void>;
  addClipTransition: (clipId: string, transition: Omit<VideoTimelineTransition, 'id'>) => Promise<void>;
  addKeyframe: (clipId: string, keyframe: Omit<VideoTimelineKeyframe, 'id'>) => Promise<void>;
  setExportSetting: <K extends keyof VideoExportSettings>(key: K, value: VideoExportSettings[K]) => void;
  renderTimeline: () => Promise<void>;
  pollRenderJob: (jobId: string) => Promise<void>;
  cancelRenderJob: (jobId?: string) => Promise<void>;
  downloadRender: (jobId?: string) => void;
  setAssistantInstruction: (instruction: string) => void;
  requestStoryboard: () => Promise<void>;
  requestEditPlan: () => Promise<void>;
  requestTimelinePlan: () => Promise<void>;
  applyAssistantPlan: () => Promise<void>;
  requestSocialVariants: () => Promise<void>;
}

export const useVideoStudioStore = create<VideoStudioState>((set, get) => ({
  projects: [],
  activeProjectId: null,
  activeGenerationId: null,
  selectedAssetId: null,
  generations: [],
  assets: [],
  providers: [],
  selectedProvider: 'mock',
  modelsByProvider: { mock: [], openrouter: [], gemini: [], openai: [], custom: [] },
  selectedModel: 'mock-video-v1',
  promptForm: cloneForm(),
  timelineRecord: null,
  timeline: null,
  selectedClipId: null,
  selectedTrackId: null,
  playheadMs: 0,
  zoom: 1,
  isPlaying: false,
  snappingEnabled: true,
  renderJobs: [],
  activeRenderJobId: null,
  exportSettings: { ...DEFAULT_EXPORT },
  assistantInstruction: '',
  assistantPlan: null,
  storyboard: null,
  socialVariants: [],
  isLoading: false,
  isGenerating: false,
  isEnhancing: false,
  isSavingTimeline: false,
  isRendering: false,
  generationProgress: null,
  error: null,
  abortGeneration: null,

  loadProviders: async () => {
    try {
      const providers = await videoApi.providers();
      const available = providers.filter((provider) => provider.configured).map((provider) => provider.key);
      const current = get().selectedProvider;
      const preferredRealProvider = providers.find((provider) => provider.configured && !provider.mock)?.key;
      const selectedProvider = available.includes(current) && (current !== 'mock' || !preferredRealProvider)
        ? current
        : preferredRealProvider || available[0] || 'mock';
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
        set({ activeProjectId: null, activeGenerationId: null, selectedAssetId: null, generations: [], assets: [], timeline: null, timelineRecord: null });
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
        selectedAssetId: null,
        generations: [],
        assets: [],
        timeline: defaultTimeline(project),
        timelineRecord: null,
      }));
      await get().loadTimeline(project.id);
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
        selectedAssetId: detail.assets[0]?.id || null,
        generations: detail.generations,
        assets: detail.assets,
        selectedProvider: detail.project.default_provider || state.selectedProvider,
        selectedModel: detail.project.default_model || state.selectedModel,
        timeline: defaultTimeline(detail.project),
        timelineRecord: null,
        selectedClipId: null,
        selectedTrackId: null,
        playheadMs: 0,
        isLoading: false,
      }));
      if (detail.project.default_provider) {
        await get().loadModels(detail.project.default_provider);
      }
      await get().loadTimeline(detail.project.id);
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
          selectedAssetId: payload.asset?.id || state.selectedAssetId,
          generations: upsertGeneration(state.generations, payload.generation),
          assets: payload.asset ? upsertAsset(state.assets, payload.asset) : state.assets,
          isGenerating: false,
          generationProgress: { stage: 'done', message: 'Video generation complete', progress: 1 },
          abortGeneration: null,
        }));
        if (payload.asset && get().promptForm.place_on_timeline) {
          void get().addAssetToTimeline(payload.asset.id);
        }
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
          selectedAssetId: activeDeleted ? null : state.selectedAssetId,
          generations: activeDeleted ? [] : state.generations,
          assets: activeDeleted ? [] : state.assets,
          timeline: activeDeleted ? null : state.timeline,
          timelineRecord: activeDeleted ? null : state.timelineRecord,
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

  selectAsset: (assetId) => set({ selectedAssetId: assetId }),

  loadTimeline: async (projectId) => {
    const id = projectId || get().activeProjectId;
    if (!id) return;
    try {
      const detail = await videoApi.getTimeline(id);
      set({ timelineRecord: detail.timeline, timeline: detail.document, selectedClipId: null, selectedTrackId: null });
    } catch (err) {
      set({ timeline: defaultTimeline(get().projects.find((project) => project.id === id)), error: (err as Error).message });
    }
  },

  saveTimeline: async (document) => {
    const projectId = get().activeProjectId;
    const doc = document || get().timeline;
    if (!projectId || !doc) return;
    set({ isSavingTimeline: true });
    try {
      const detail = await videoApi.saveTimeline(projectId, doc);
      set({ timelineRecord: detail.timeline, timeline: detail.document, isSavingTimeline: false });
    } catch (err) {
      set({ isSavingTimeline: false, error: (err as Error).message });
      toast.error('Could not save timeline');
    }
  },

  addAssetToTimeline: async (assetId, options = {}) => {
    const projectId = get().activeProjectId;
    if (!projectId) return;
    const asset = get().assets.find((item) => item.id === assetId);
    try {
      const detail = await videoApi.importAssetToTimeline(projectId, {
        asset_id: assetId,
        track_id: options.track_id,
        track_type: options.track_type || assetTrackType(asset),
        start_ms: options.start_ms ?? get().playheadMs,
        duration_ms: options.duration_ms,
      });
      set({ timelineRecord: detail.timeline, timeline: detail.document, selectedAssetId: assetId });
      toast.success('Asset added to timeline');
    } catch (err) {
      toast.error((err as Error).message);
    }
  },

  moveClip: async (clipId, trackId, startMs) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    const [clip] = loc.track.clips.splice(loc.clipIndex, 1);
    clip.start_ms = Math.max(0, Math.round(startMs));
    const target = next.tracks.find((track) => track.id === trackId) || loc.track;
    target.clips.push(clip);
    target.clips.sort((a, b) => a.start_ms - b.start_ms);
    recomputeDuration(next);
    set({ timeline: next, selectedClipId: clip.id, selectedTrackId: target.id });
    await get().saveTimeline(next);
  },

  trimClip: async (clipId, updates) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.start_ms = Math.max(0, Math.round(updates.start_ms ?? loc.clip.start_ms));
    loc.clip.duration_ms = Math.max(100, Math.round(updates.duration_ms ?? loc.clip.duration_ms));
    loc.clip.trim_in_ms = Math.max(0, Math.round(updates.trim_in_ms ?? loc.clip.trim_in_ms));
    loc.clip.trim_out_ms = Math.max(loc.clip.trim_in_ms + 100, Math.round(updates.trim_out_ms ?? loc.clip.trim_out_ms ?? loc.clip.duration_ms));
    recomputeDuration(next);
    set({ timeline: next });
    await get().saveTimeline(next);
  },

  splitClipAtPlayhead: async () => {
    const current = get().timeline;
    const clipId = get().selectedClipId;
    if (!current || !clipId) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    const splitAt = get().playheadMs;
    const offset = splitAt - loc.clip.start_ms;
    if (offset <= 0 || offset >= loc.clip.duration_ms) {
      toast.error('Move the playhead inside the selected clip');
      return;
    }
    const left = { ...loc.clip, duration_ms: offset, trim_out_ms: loc.clip.trim_in_ms + offset };
    const right = {
      ...loc.clip,
      id: newId('clip'),
      start_ms: splitAt,
      duration_ms: loc.clip.duration_ms - offset,
      trim_in_ms: loc.clip.trim_in_ms + offset,
      trim_out_ms: loc.clip.trim_out_ms,
    };
    loc.track.clips.splice(loc.clipIndex, 1, left, right);
    set({ timeline: recomputeDuration(next), selectedClipId: right.id, selectedTrackId: loc.track.id });
    await get().saveTimeline(next);
  },

  deleteClip: async (clipId) => {
    const current = get().timeline;
    const id = clipId || get().selectedClipId;
    if (!current || !id) return;
    const next = cloneTimeline(current);
    for (const track of next.tracks) {
      track.clips = track.clips.filter((clip) => clip.id !== id);
    }
    set({ timeline: recomputeDuration(next), selectedClipId: null });
    await get().saveTimeline(next);
  },

  duplicateClip: async (clipId) => {
    const current = get().timeline;
    const id = clipId || get().selectedClipId;
    if (!current || !id) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, id);
    if (!loc) return;
    const copy = { ...loc.clip, id: newId('clip'), start_ms: loc.clip.start_ms + loc.clip.duration_ms + 250 };
    loc.track.clips.splice(loc.clipIndex + 1, 0, copy);
    set({ timeline: recomputeDuration(next), selectedClipId: copy.id, selectedTrackId: loc.track.id });
    await get().saveTimeline(next);
  },

  selectClip: (clipId, trackId = null) => set({ selectedClipId: clipId, selectedTrackId: trackId }),
  setPlayhead: (timeMs) => set((state) => ({ playheadMs: Math.max(0, Math.min(timeMs, state.timeline?.duration_ms || timeMs)) })),
  setZoom: (zoom) => set({ zoom: Math.min(4, Math.max(0.35, zoom)) }),
  setPlaying: (playing) => set({ isPlaying: playing }),
  toggleSnapping: () => set((state) => ({ snappingEnabled: !state.snappingEnabled })),

  toggleTrackMute: async (trackId) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const track = next.tracks.find((item) => item.id === trackId);
    if (!track) return;
    track.muted = !track.muted;
    set({ timeline: next });
    await get().saveTimeline(next);
  },

  toggleTrackLock: async (trackId) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const track = next.tracks.find((item) => item.id === trackId);
    if (!track) return;
    track.locked = !track.locked;
    set({ timeline: next });
    await get().saveTimeline(next);
  },

  toggleTrackVisibility: async (trackId) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const track = next.tracks.find((item) => item.id === trackId);
    if (!track) return;
    track.visible = !track.visible;
    set({ timeline: next });
    await get().saveTimeline(next);
  },

  updateClipTransform: async (clipId, transform) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.transform = { ...defaultTransform(), ...(loc.clip.transform || {}), ...transform };
    set({ timeline: next });
    await get().saveTimeline(next);
  },

  updateClipVolume: async (clipId, volume) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.volume = Math.min(2, Math.max(0, volume));
    set({ timeline: next });
    await get().saveTimeline(next);
  },

  updateClipFade: async (clipId, fade) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.fade_in_ms = Math.max(0, fade.fade_in_ms ?? loc.clip.fade_in_ms ?? 0);
    loc.clip.fade_out_ms = Math.max(0, fade.fade_out_ms ?? loc.clip.fade_out_ms ?? 0);
    set({ timeline: next });
    await get().saveTimeline(next);
  },

  addTextClip: async (text = 'Title card') => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    let track = next.tracks.find((item) => item.type === 'text');
    if (!track) {
      track = { id: 'track-text-1', type: 'text', name: 'Text 1', locked: false, muted: false, visible: true, clips: [] };
      next.tracks.push(track);
    }
    const clip: VideoTimelineClip = {
      id: newId('clip'),
      start_ms: get().playheadMs,
      duration_ms: 3000,
      trim_in_ms: 0,
      trim_out_ms: 3000,
      transform: defaultTransform(),
      text: { text, font_size: 48, font_weight: '700', color: '#ffffff', shadow: true },
      effects: [],
      keyframes: [],
      transitions: [],
    };
    track.clips.push(clip);
    set({ timeline: recomputeDuration(next), selectedClipId: clip.id, selectedTrackId: track.id });
    await get().saveTimeline(next);
  },

  updateClipText: async (clipId, text) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.text = { ...(loc.clip.text || { text: '' }), ...text };
    set({ timeline: next });
    await get().saveTimeline(next);
  },

  addClipEffect: async (clipId, effect) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.effects = [...(loc.clip.effects || []), { ...effect, id: newId('effect') }];
    set({ timeline: next });
    await get().saveTimeline(next);
  },

  toggleClipEffect: async (clipId, effectId) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.effects = (loc.clip.effects || []).map((effect) => effect.id === effectId ? { ...effect, enabled: !effect.enabled } : effect);
    set({ timeline: next });
    await get().saveTimeline(next);
  },

  removeClipEffect: async (clipId, effectId) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.effects = (loc.clip.effects || []).filter((effect) => effect.id !== effectId);
    set({ timeline: next });
    await get().saveTimeline(next);
  },

  addClipTransition: async (clipId, transition) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.transitions = [...(loc.clip.transitions || []), { ...transition, id: newId('transition') }];
    set({ timeline: next });
    await get().saveTimeline(next);
  },

  addKeyframe: async (clipId, keyframe) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.keyframes = [...(loc.clip.keyframes || []), { ...keyframe, id: newId('keyframe') }];
    set({ timeline: next });
    await get().saveTimeline(next);
  },

  setExportSetting: (key, value) => set((state) => ({
    exportSettings: { ...state.exportSettings, [key]: value },
  })),

  renderTimeline: async () => {
    const projectId = get().activeProjectId;
    if (!projectId || !get().timeline) return;
    set({ isRendering: true, error: null });
    try {
      await get().saveTimeline();
      const job = await videoApi.renderTimeline(projectId, get().exportSettings);
      set((state) => ({
        renderJobs: upsertRenderJob(state.renderJobs, job),
        activeRenderJobId: job.id,
        isRendering: false,
      }));
      toast.success('Render started');
      void get().pollRenderJob(job.id);
    } catch (err) {
      set({ isRendering: false, error: (err as Error).message });
      toast.error('Could not start render');
    }
  },

  pollRenderJob: async (jobId) => {
    try {
      const job = await videoApi.getRenderJob(jobId);
      set((state) => ({ renderJobs: upsertRenderJob(state.renderJobs, job), activeRenderJobId: job.id }));
      if (job.status === 'completed') {
        if (get().activeProjectId) {
          const assets = await videoApi.listAssets(get().activeProjectId as string);
          set({ assets, selectedAssetId: job.output_asset_id || get().selectedAssetId });
        }
        toast.success('Render complete');
        return;
      }
      if (job.status === 'failed' || job.status === 'cancelled') {
        if (job.error) toast.error(job.error);
        return;
      }
      window.setTimeout(() => { void get().pollRenderJob(jobId); }, 1000);
    } catch (err) {
      set({ error: (err as Error).message });
    }
  },

  cancelRenderJob: async (jobId) => {
    const id = jobId || get().activeRenderJobId;
    if (!id) return;
    try {
      const job = await videoApi.cancelRenderJob(id);
      set((state) => ({ renderJobs: upsertRenderJob(state.renderJobs, job) }));
      toast.success('Render cancelled');
    } catch (err) {
      toast.error((err as Error).message);
    }
  },

  downloadRender: (jobId) => {
    const id = jobId || get().activeRenderJobId;
    const job = get().renderJobs.find((item) => item.id === id);
    if (!job?.output_asset_id) return;
    window.open(videoApi.downloadUrl(job.output_asset_id), '_blank', 'noopener,noreferrer');
  },

  setAssistantInstruction: (instruction) => set({ assistantInstruction: instruction }),

  requestStoryboard: async () => {
    const projectId = get().activeProjectId;
    if (!projectId) return;
    try {
      const storyboard = await videoApi.assistant.storyboard(projectId, {
        prompt: get().promptForm.prompt,
        instruction: get().assistantInstruction,
        timeline: get().timeline || undefined,
      });
      set({ storyboard });
      toast.success('Storyboard created');
    } catch (err) {
      toast.error((err as Error).message);
    }
  },

  requestEditPlan: async () => {
    const projectId = get().activeProjectId;
    if (!projectId) return;
    try {
      const assistantPlan = await videoApi.assistant.editPlan(projectId, {
        prompt: get().promptForm.prompt,
        instruction: get().assistantInstruction,
        timeline: get().timeline || undefined,
      });
      set({ assistantPlan });
    } catch (err) {
      toast.error((err as Error).message);
    }
  },

  requestTimelinePlan: async () => {
    const projectId = get().activeProjectId;
    if (!projectId) return;
    try {
      const assistantPlan = await videoApi.assistant.timelinePlan(projectId, {
        prompt: get().promptForm.prompt,
        instruction: get().assistantInstruction,
        timeline: get().timeline || undefined,
      });
      set({ assistantPlan });
    } catch (err) {
      toast.error((err as Error).message);
    }
  },

  applyAssistantPlan: async () => {
    const projectId = get().activeProjectId;
    const plan = get().assistantPlan;
    if (!projectId || !plan) return;
    try {
      const detail = await videoApi.assistant.applyEditPlan(projectId, plan);
      set({ timelineRecord: detail.timeline, timeline: detail.document, assistantPlan: null });
      toast.success('Edit plan applied');
    } catch (err) {
      toast.error((err as Error).message);
    }
  },

  requestSocialVariants: async () => {
    const projectId = get().activeProjectId;
    if (!projectId) return;
    try {
      const socialVariants = await videoApi.assistant.socialVariants(projectId, {
        prompt: get().promptForm.prompt,
        instruction: get().assistantInstruction,
        timeline: get().timeline || undefined,
      });
      set({ socialVariants });
    } catch (err) {
      toast.error((err as Error).message);
    }
  },
}));
