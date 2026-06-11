/**
 * Zustand store for Video Studio (AI generation) and Video Edit Studio
 * (timeline editing). All timeline mutations follow the same shape: clone the
 * document, mutate the clone, push one undo snapshot via withTimelineHistory,
 * set state, then autosave. Drag interactions keep live values in component
 * state and call a store action once on pointer-up so each user gesture is a
 * single undo entry and a single save.
 *
 * Sequence counters guard async races: _saveSeq ignores out-of-order save
 * responses, and renderedSaveSeq (vs. _saveSeq) drives the "timeline changed
 * since last render" indicator.
 */
import { create } from 'zustand';
import { toast } from 'sonner';
import { videoApi } from '../api';
import { CAPTION_PRESETS, parseCaptions, serializeSrt, serializeVtt } from '../components/video/captions/captionUtils';
import type { CaptionCue, CaptionPresetKey } from '../components/video/captions/captionUtils';
import { annotationDefaults, ANNOTATION_PRESETS } from '../components/video/effects/annotationRegistry';
import { motionPreset } from '../components/video/effects/motionPresets';
import { TIMELINE_TEMPLATES } from '../components/video/templates/timelineTemplates';
import type { EditorModeKey } from '../components/video/editorModes';

const EDITOR_MODE_STORAGE_KEY = 'omnillm-video-editor-mode';

function initialEditorMode(): EditorModeKey {
  if (typeof window === 'undefined') return 'full';
  const stored = window.localStorage.getItem(EDITOR_MODE_STORAGE_KEY);
  return stored === 'simple_trim' || stored === 'captions_only' || stored === 'social_clip' ? stored : 'full';
}
import type {
  InputAsset,
  VideoAsset,
  VideoEditPlan,
  VideoExportSettings,
  VideoGenerationDetail,
  VideoGenerationProgress,
  VideoGenerationValidationResult,
  VideoModel,
  VideoProject,
  VideoPromptForm,
  VideoProviderInfo,
  VideoProviderKey,
  VideoRendererCapabilities,
  VideoRenderJob,
  VideoSocialVariant,
  VideoStoryboardResponse,
  VideoTimelineClip,
  VideoTimelineDocument,
  VideoTimelineEffect,
  VideoTimelineKeyframe,
  VideoTimelineRecord,
  VideoTimelineShape,
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
  composition: '',
  lens_effect: '',
  lighting: '',
  dialogue: '',
  sound_effects: '',
  ambient_noise: '',
  continuity_notes: '',
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
  openrouter: '',
  gemini: '',
  luma: '',
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

const TIMELINE_HISTORY_LIMIT = 50;

function withTimelineHistory(
  state: { timelineUndoStack: VideoTimelineDocument[] },
  previous: VideoTimelineDocument,
): { timelineUndoStack: VideoTimelineDocument[]; timelineRedoStack: VideoTimelineDocument[] } {
  return {
    timelineUndoStack: [...state.timelineUndoStack, cloneTimeline(previous)].slice(-TIMELINE_HISTORY_LIMIT),
    timelineRedoStack: [],
  };
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
    // Generic layers, matching the backend's NewEmptyTimeline: index 0 is the
    // background; later layers stack on top.
    tracks: [
      { id: 'track-layer-1', type: 'layer', name: 'Layer 1', locked: false, muted: false, visible: true, clips: [] },
      { id: 'track-layer-2', type: 'layer', name: 'Layer 2', locked: false, muted: false, visible: true, clips: [] },
      { id: 'track-layer-3', type: 'layer', name: 'Layer 3', locked: false, muted: false, visible: true, clips: [] },
      { id: 'track-layer-4', type: 'layer', name: 'Layer 4', locked: false, muted: false, visible: true, clips: [] },
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

function parseGenerationInputAssets(inputAssetsJson?: string, inputAssetIdsJson?: string): Partial<VideoPromptForm> {
  const fields: Partial<VideoPromptForm> = {};
  if (inputAssetsJson) {
    try {
      const parsed = JSON.parse(inputAssetsJson) as InputAsset[];
      if (Array.isArray(parsed)) {
        const references: string[] = [];
        for (const item of parsed) {
          if (!item?.asset_id) continue;
          if (item.role === 'start_frame') fields.start_image_asset_id = item.asset_id;
          if (item.role === 'last_frame') fields.last_frame_asset_id = item.asset_id;
          if (item.role === 'source_video') fields.source_video_asset_id = item.asset_id;
          if (item.role === 'reference_image') references.push(item.asset_id);
        }
        if (references.length > 0) fields.reference_asset_ids = references;
        return fields;
      }
    } catch {
      // Fall back to the legacy flat asset ID list below.
    }
  }

  if (inputAssetIdsJson) {
    try {
      const ids = JSON.parse(inputAssetIdsJson) as string[];
      if (Array.isArray(ids) && ids[0]) {
        fields.start_image_asset_id = ids[0];
      }
    } catch {
      return fields;
    }
  }
  return fields;
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

function ensureCaptionTrack(doc: VideoTimelineDocument): VideoTimelineTrack {
  let track = doc.tracks.find((item) => item.type === 'caption');
  if (!track) {
    track = { id: newId('track'), type: 'caption', name: 'Captions 1', locked: false, muted: false, visible: true, clips: [] };
    doc.tracks.push(track);
  }
  return track;
}

function captionClipFromCue(cue: CaptionCue, canvas: VideoTimelineDocument['canvas'], presetKey?: CaptionPresetKey): VideoTimelineClip {
  const preset = (presetKey && CAPTION_PRESETS.find((item) => item.key === presetKey)) || CAPTION_PRESETS[0];
  const position = preset.position(canvas);
  const duration = Math.max(100, cue.end_ms - cue.start_ms);
  return {
    id: newId('clip'),
    start_ms: Math.max(0, cue.start_ms),
    duration_ms: duration,
    trim_in_ms: 0,
    trim_out_ms: duration,
    transform: { ...defaultTransform(), x: position.x, y: position.y },
    text: { text: cue.text, ...preset.text },
    effects: [],
    keyframes: [],
    transitions: [],
  };
}

/** Project-default caption style, recorded in the timeline metadata. */
function defaultCaptionPreset(doc: VideoTimelineDocument | null): CaptionPresetKey | undefined {
  const key = doc?.metadata?.default_caption_preset;
  return typeof key === 'string' && CAPTION_PRESETS.some((preset) => preset.key === key)
    ? (key as CaptionPresetKey)
    : undefined;
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
  timelineUndoStack: VideoTimelineDocument[];
  timelineRedoStack: VideoTimelineDocument[];
  selectedClipId: string | null;
  selectedClipIds: string[];
  selectedTrackId: string | null;
  // Ephemeral playback solo — only this track contributes preview audio.
  // Not persisted to the timeline document and ignored by export.
  soloTrackId: string | null;
  // Auto-scroll the timeline to keep the playhead in view during playback.
  followPlayhead: boolean;
  clipClipboard: Array<{ clip: VideoTimelineClip; trackId: string }> | null;
  attributeClipboard: Partial<Pick<VideoTimelineClip, 'transform' | 'volume' | 'fade_in_ms' | 'fade_out_ms' | 'effects' | 'transitions' | 'text'>> | null;
  playheadMs: number;
  zoom: number;
  isPlaying: boolean;
  snappingEnabled: boolean;
  // Ripple mode: deletes/trims/inserts shift later clips on the same layer to
  // keep the timeline gap-free.
  rippleEnabled: boolean;
  toolMode: 'select' | 'blade';
  editorMode: EditorModeKey;
  rendererCapabilities: VideoRendererCapabilities | null;
  renderJobs: VideoRenderJob[];
  activeRenderJobId: string | null;
  // Dirty-render tracking: the save sequence captured when the last completed
  // render started. 0 = nothing rendered yet this session.
  _renderStartedSaveSeq: number;
  renderedSaveSeq: number;
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
  generationValidation: VideoGenerationValidationResult | null;
  error: string | null;
  abortGeneration: (() => void) | null;
  _pollInterval: ReturnType<typeof setInterval> | null;
  _renderPollTimeout: number | null;
  _saveSeq: number;

  loadProviders: () => Promise<void>;
  loadModels: (provider: VideoProviderKey, refresh?: boolean) => Promise<void>;
  loadProjects: () => Promise<void>;
  createProject: (title?: string) => Promise<VideoProject | null>;
  createProjectFromTemplate: (templateKey: string) => Promise<void>;
  selectProject: (projectId: string) => Promise<void>;
  setProvider: (provider: VideoProviderKey) => Promise<void>;
  setModel: (model: string) => void;
  setPromptField: <K extends keyof VideoPromptForm>(key: K, value: VideoPromptForm[K]) => void;
  clearPrompt: () => void;
  enhancePrompt: () => Promise<void>;
  generate: (parentId?: string) => void;
  branchFromGeneration: (generationId: string) => Promise<void>;
  regenerateFromGeneration: (generationId: string) => Promise<void>;
  deleteProject: (projectId: string) => Promise<void>;
  stopGeneration: () => void;
  cancelGeneration: () => Promise<void>;
  selectAsset: (assetId: string | null) => void;

  loadTimeline: (projectId?: string) => Promise<void>;
  saveTimeline: (document?: VideoTimelineDocument) => Promise<void>;
  addAssetToTimeline: (assetId: string, options?: { track_id?: string; track_type?: VideoTimelineTrackType; start_ms?: number; duration_ms?: number }) => Promise<void>;
  moveClip: (clipId: string, trackId: string, startMs: number) => Promise<void>;
  trimClip: (clipId: string, updates: Partial<Pick<VideoTimelineClip, 'start_ms' | 'duration_ms' | 'trim_in_ms' | 'trim_out_ms'>>) => Promise<void>;
  splitClipAtPlayhead: () => Promise<void>;
  deleteClip: (clipId?: string) => Promise<void>;
  duplicateClip: (clipId?: string) => Promise<void>;
  selectClip: (clipId: string | null, trackId?: string | null, additive?: boolean) => void;
  renameAsset: (assetId: string, fileName: string) => Promise<void>;
  deleteAsset: (assetId: string) => Promise<void>;
  uploadAsset: (file: File) => Promise<void>;
  importMusicAsset: (musicAssetId: string, title?: string) => Promise<VideoAsset | null>;
  loadRendererCapabilities: () => Promise<void>;
  setPlayhead: (timeMs: number) => void;
  setZoom: (zoom: number) => void;
  zoomToFit: (containerWidth: number) => void;
  setPlaying: (playing: boolean) => void;
  toggleSnapping: () => void;
  toggleRipple: () => void;
  removeGap: (trackId: string, startMs: number, endMs: number) => Promise<void>;
  removeAllGaps: (trackId?: string) => Promise<void>;
  rippleDeleteClip: (clipId?: string) => Promise<void>;
  rippleTrimClip: (clipId: string, edge: 'start' | 'end', newTimeMs: number) => Promise<void>;
  insertClipAt: (assetId: string, trackId: string, timeMs: number, ripple?: boolean) => Promise<void>;
  overwriteClipAt: (assetId: string, trackId: string, timeMs: number) => Promise<void>;
  toggleTrackMute: (trackId: string) => Promise<void>;
  toggleTrackLock: (trackId: string) => Promise<void>;
  toggleTrackVisibility: (trackId: string) => Promise<void>;
  updateClipTransform: (clipId: string, transform: Partial<VideoTimelineTransform>) => Promise<void>;
  updateClipVolume: (clipId: string, volume: number) => Promise<void>;
  updateClipFade: (clipId: string, fade: { fade_in_ms?: number; fade_out_ms?: number }) => Promise<void>;
  /** Clears fades on the given clips (or the selection) as one undo step. */
  removeClipFades: (clipIds?: string[]) => Promise<void>;
  /** Generates volume keyframes on music clips so they duck under narration. */
  duckMusicUnderNarration: (duckTo?: number, rampMs?: number) => Promise<void>;
  addTextClip: (text?: string, options?: { trackId?: string; startMs?: number }) => Promise<void>;
  addShapeClip: (kind: VideoTimelineShape['kind'], options?: { trackId?: string; startMs?: number }) => Promise<void>;
  updateClipShape: (clipId: string, patch: Partial<VideoTimelineShape>) => Promise<void>;
  /** Shape resize commits dimensions and position together as one undo step. */
  resizeShapeClip: (clipId: string, patch: { width?: number; height?: number; x?: number; y?: number }) => Promise<void>;
  applyAnnotationPreset: (clipId: string, presetKey: string) => Promise<void>;
  updateClipText: (clipId: string, text: Partial<NonNullable<VideoTimelineClip['text']>>) => Promise<void>;
  addClipEffect: (clipId: string, effect: Omit<VideoTimelineEffect, 'id'>) => Promise<void>;
  toggleClipEffect: (clipId: string, effectId: string) => Promise<void>;
  removeClipEffect: (clipId: string, effectId: string) => Promise<void>;
  addClipTransition: (clipId: string, transition: Omit<VideoTimelineTransition, 'id'>) => Promise<void>;
  updateClipTransition: (clipId: string, transitionId: string, patch: Partial<Omit<VideoTimelineTransition, 'id'>>) => Promise<void>;
  removeClipTransition: (clipId: string, transitionId: string) => Promise<void>;
  updateClipEffect: (clipId: string, effectId: string, patch: Partial<Omit<VideoTimelineEffect, 'id'>>) => Promise<void>;
  reorderClipEffect: (clipId: string, effectId: string, direction: -1 | 1) => Promise<void>;
  addKeyframe: (clipId: string, keyframe: Omit<VideoTimelineKeyframe, 'id'>) => Promise<void>;
  /** Replaces pan/zoom (x/y/scale) keyframes with a generated motion preset. */
  applyMotionPreset: (clipId: string, presetKey: string) => Promise<void>;
  updateKeyframe: (clipId: string, keyframeId: string, patch: Partial<Omit<VideoTimelineKeyframe, 'id'>>) => Promise<void>;
  removeKeyframe: (clipId: string, keyframeId: string) => Promise<void>;
  addTrack: (type?: VideoTimelineTrackType, name?: string) => Promise<void>;
  removeTrack: (trackId: string) => Promise<void>;
  renameTrack: (trackId: string, name: string) => Promise<void>;
  reorderTrack: (trackId: string, targetIndex: number) => Promise<void>;
  setTrackHeight: (trackId: string, height: number) => Promise<void>;
  duplicateTrack: (trackId: string) => Promise<void>;
  insertTrackAdjacent: (trackId: string, where: 'above' | 'below') => Promise<void>;
  clearTrack: (trackId: string) => Promise<void>;
  toggleTrackSolo: (trackId: string) => void;
  moveTrackToEdge: (trackId: string, edge: 'top' | 'bottom') => Promise<void>;
  selectClipsOnTrack: (trackId: string) => void;
  setSelectedClips: (ids: string[]) => void;
  selectClipsRelativeToPlayhead: (which: 'before' | 'after') => void;
  selectAllClips: () => void;
  moveClipToAdjacentTrack: (clipId: string, direction: 'above' | 'below') => Promise<void>;
  copySelection: (clipId?: string) => void;
  cutSelection: (clipId?: string) => Promise<void>;
  pasteClips: (atMs?: number, trackId?: string) => Promise<void>;
  copyClipAttributes: (clipId?: string) => void;
  pasteClipAttributes: (clipId?: string) => Promise<void>;
  setTimelineDuration: (durationMs: number) => Promise<void>;
  splitAllAtPlayhead: () => Promise<void>;
  toggleClipMute: (clipId?: string) => Promise<void>;
  detachClipAudio: (clipId: string) => Promise<void>;
  addAssetAsMusicBed: (assetId: string) => Promise<void>;
  toggleFollowPlayhead: () => void;
  duplicateProject: (projectId?: string) => Promise<void>;
  createProjectFromVariant: (variant: VideoSocialVariant) => Promise<void>;
  addMarker: (timeMs?: number, label?: string) => Promise<void>;
  removeMarker: (markerId: string) => Promise<void>;
  updateClipZIndex: (clipId: string, zIndex: number) => Promise<void>;
  bringClipForward: (clipId?: string) => Promise<void>;
  sendClipBackward: (clipId?: string) => Promise<void>;
  nudgeSelection: (deltaMs: number) => Promise<void>;
  setCanvas: (patch: Partial<VideoTimelineDocument['canvas']>) => Promise<void>;
  splitClipAt: (clipId: string, timeMs: number) => Promise<void>;
  trimClipEdgeToPlayhead: (edge: 'start' | 'end') => Promise<void>;
  groupClips: (clipIds?: string[]) => Promise<void>;
  ungroupClips: (groupId?: string) => Promise<void>;
  alignSelection: (mode: 'start' | 'end' | 'distribute') => Promise<void>;
  setToolMode: (mode: 'select' | 'blade') => void;
  setEditorMode: (mode: EditorModeKey) => void;
  addCaptionSegment: (text?: string) => Promise<void>;
  importCaptions: (raw: string) => Promise<void>;
  exportCaptions: (format: 'srt' | 'vtt') => string | null;
  mergeCaptionClipWithNext: (clipId: string) => Promise<void>;
  applyCaptionPreset: (preset: CaptionPresetKey) => Promise<void>;
  /** Shifts all caption clips (or one) in time as a single undo step. */
  shiftCaptions: (deltaMs: number, clipId?: string) => Promise<void>;
  undoTimeline: () => Promise<void>;
  redoTimeline: () => Promise<void>;
  setExportSetting: <K extends keyof VideoExportSettings>(key: K, value: VideoExportSettings[K]) => void;
  renderTimeline: () => Promise<void>;
  pollRenderJob: (jobId: string) => Promise<void>;
  cancelRenderJob: (jobId?: string) => Promise<void>;
  downloadRender: (jobId?: string) => void;
  /** Re-renders using the settings stored on an earlier job. */
  retryRenderJob: (jobId: string) => Promise<void>;
  setAssistantInstruction: (instruction: string) => void;
  requestStoryboard: () => Promise<void>;
  requestEditPlan: () => Promise<void>;
  requestTimelinePlan: () => Promise<void>;
  applyAssistantPlan: (selectedIndices?: number[]) => Promise<void>;
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
  selectedProvider: 'openrouter',
  modelsByProvider: { openrouter: [], gemini: [], luma: [], openai: [], custom: [] },
  selectedModel: null,
  promptForm: cloneForm(),
  timelineRecord: null,
  timeline: null,
  timelineUndoStack: [],
  timelineRedoStack: [],
  selectedClipId: null,
  selectedClipIds: [],
  selectedTrackId: null,
  soloTrackId: null,
  followPlayhead: true,
  clipClipboard: null,
  attributeClipboard: null,
  playheadMs: 0,
  zoom: 1,
  isPlaying: false,
  snappingEnabled: true,
  rippleEnabled: false,
  toolMode: 'select',
  editorMode: initialEditorMode(),
  rendererCapabilities: null,
  renderJobs: [],
  activeRenderJobId: null,
  _renderStartedSaveSeq: 0,
  renderedSaveSeq: 0,
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
  generationValidation: null,
  error: null,
  abortGeneration: null,
  _pollInterval: null,
  _renderPollTimeout: null,
  _saveSeq: 0,

  loadProviders: async () => {
    try {
      const providers = await videoApi.providers();
      const available = providers.filter((provider) => provider.configured).map((provider) => provider.key);
      const current = get().selectedProvider;
      const preferredProvider = providers.find((provider) => provider.configured)?.key;
      const selectedProvider = available.includes(current)
        ? current
        : preferredProvider || providers.find((provider) => provider.key === current)?.key || providers[0]?.key || 'openrouter';
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
        generationValidation: null,
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
        timelineUndoStack: [],
        timelineRedoStack: [],
        generationValidation: null,
      }));
      await get().loadTimeline(project.id);
      return project;
    } catch (err) {
      set({ error: (err as Error).message });
      toast.error('Could not create video project');
      return null;
    }
  },

  createProjectFromTemplate: async (templateKey) => {
    const template = TIMELINE_TEMPLATES.find((item) => item.key === templateKey);
    if (!template) return;
    const project = await get().createProject(`${template.label} project`);
    if (!project) return;
    const document = template.build();
    set({ timeline: document, timelineUndoStack: [], timelineRedoStack: [], selectedClipId: null, selectedClipIds: [], selectedTrackId: null });
    await get().saveTimeline(document);
    toast.success(`Created project from "${template.label}" template`);
  },

  selectProject: async (projectId) => {
    // Stop generation/render polls from the previous project — left running
    // they would keep injecting the old project's data into the new one.
    const { _pollInterval, _renderPollTimeout } = get();
    if (_pollInterval) clearInterval(_pollInterval);
    if (_renderPollTimeout) clearTimeout(_renderPollTimeout);
    set({
      isLoading: true,
      error: null,
      _pollInterval: null,
      _renderPollTimeout: null,
      isGenerating: false,
      generationProgress: null,
      activeRenderJobId: null,
      renderJobs: [],
    });
    try {
      const detail = await videoApi.getProject(projectId);
      const gens = detail.generations ?? [];
      const assetList = detail.assets ?? [];
      const nextActiveGenerationId = gens[gens.length - 1]?.id || null;
      set((state) => ({
        projects: upsertProject(state.projects, detail.project),
        activeProjectId: detail.project.id,
        activeGenerationId: nextActiveGenerationId,
        selectedAssetId: assetList[0]?.id || null,
        generations: gens,
        assets: assetList,
        selectedProvider: detail.project.default_provider || state.selectedProvider,
        selectedModel: detail.project.default_model || state.selectedModel,
        timeline: defaultTimeline(detail.project),
        timelineRecord: null,
        selectedClipId: null,
        selectedClipIds: [],
        selectedTrackId: null,
        playheadMs: 0,
        timelineUndoStack: [],
        timelineRedoStack: [],
        generationValidation: null,
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
    set({ selectedProvider: provider, selectedModel: null, generationValidation: null });
    await get().loadModels(provider);
  },

  setModel: (model) => set({ selectedModel: model, generationValidation: null }),

  setPromptField: (key, value) => set((state) => ({
    promptForm: { ...state.promptForm, [key]: value },
    generationValidation: null,
  })),

  clearPrompt: () => set({ promptForm: cloneForm(), error: null, generationProgress: null, generationValidation: null }),

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
    const { selectedProvider, selectedModel, promptForm, activeProjectId, providers } = get();
    if (!selectedProvider || !selectedModel) {
      toast.error('Choose a video provider and model');
      return;
    }
    if (!providers.find((provider) => provider.key === selectedProvider)?.configured) {
      toast.error('Configure a video provider (OpenRouter, Gemini, or Luma) first');
      return;
    }
    if (!promptForm.prompt.trim()) {
      toast.error('Enter a video prompt');
      return;
    }
    void (async () => {
      const baseRequest = {
        ...promptForm,
        provider: selectedProvider,
        model: selectedModel,
        prompt: promptForm.prompt.trim(),
        project_id: activeProjectId || undefined,
        parent_id: parentId,
      };
      let generationRequest = baseRequest;
      try {
        const validation = await videoApi.validateGeneration(baseRequest);
        set({ generationValidation: validation });
        if (!validation.valid) {
          const message = validation.errors[0]?.message || 'Video generation settings are invalid';
          set({ error: message, generationProgress: null, isGenerating: false });
          toast.error(message);
          return;
        }
        generationRequest = {
          ...baseRequest,
          ...validation.normalized_request,
          project_id: activeProjectId || undefined,
          parent_id: parentId,
        };
        if (validation.normalizations.length > 0) {
          set((state) => ({
            promptForm: {
              ...state.promptForm,
              aspect_ratio: generationRequest.aspect_ratio,
              duration_seconds: generationRequest.duration_seconds,
              resolution: generationRequest.resolution,
              fps: generationRequest.fps,
            },
          }));
          toast.info('Video settings were normalized for the selected model');
        }
      } catch (err) {
        const message = (err as Error).message;
        set({ error: message, generationProgress: null, isGenerating: false });
        toast.error(message || 'Could not validate video settings');
        return;
      }

      set({
        isGenerating: true,
        generationProgress: { stage: 'queued', message: 'Preparing video generation' },
        error: null,
      });
      videoApi.generate(generationRequest).then((resp) => {
        set((state) => ({
          activeProjectId: resp.project_id,
          activeGenerationId: resp.generation_id,
          generations: upsertGeneration(state.generations, resp.generation),
          generationProgress: { stage: 'running', message: 'Video generation in progress' },
        }));
        // Start polling until terminal state
        const interval = setInterval(async () => {
          try {
            const gen = await videoApi.getGeneration(resp.generation_id);
            set((state) => ({ generations: upsertGeneration(state.generations, gen) }));
            if (gen.status === 'completed') {
              clearInterval(interval);
              set({
                isGenerating: false,
                _pollInterval: null,
                generationProgress: { stage: 'done', message: 'Video generation complete', progress: 1 },
                activeGenerationId: gen.id,
                selectedAssetId: gen.output_asset_id || get().selectedAssetId,
              });
              if (gen.output_asset_id) {
                // Reload project assets
                const proj = get().activeProjectId;
                if (proj) {
                  videoApi.getProject(proj).then((detail) => {
                    set({ assets: detail.assets ?? [] });
                    if (get().promptForm.place_on_timeline && gen.output_asset_id) {
                      void get().addAssetToTimeline(gen.output_asset_id);
                    }
                  }).catch(() => { /* non-fatal */ });
                }
              }
              toast.success('Video generation complete');
            } else if (gen.status === 'failed' || gen.status === 'cancelled') {
              clearInterval(interval);
              set({
                isGenerating: false,
                _pollInterval: null,
                generationProgress: null,
                error: gen.error || `Video generation ${gen.status}`,
              });
              toast.error(gen.error || `Video generation ${gen.status}`);
            } else {
              set({ generationProgress: { stage: gen.status, message: 'Video generation in progress' } });
            }
          } catch {
            // Polling error is transient — keep trying
          }
        }, 8000);
        set({ _pollInterval: interval, abortGeneration: null });
      }).catch((err: Error) => {
        set({
          isGenerating: false,
          generationProgress: null,
          error: err.message,
          abortGeneration: null,
        });
        toast.error(err.message || 'Video generation failed');
      });
    })();
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
      const inputAssets = parseGenerationInputAssets(branch.input_assets_json, branch.input_asset_ids_json);
      set((state) => ({
        activeProjectId: branch.project_id,
        activeGenerationId: branch.parent_id,
        selectedProvider: branch.provider,
        selectedModel: branch.model,
        promptForm: {
          ...state.promptForm,
          ...settings,
          start_image_asset_id: undefined,
          last_frame_asset_id: undefined,
          source_video_asset_id: undefined,
          reference_asset_ids: [],
          ...inputAssets,
          prompt: branch.enhanced_prompt || branch.prompt,
          negative_prompt: branch.negative_prompt || state.promptForm.negative_prompt,
        },
        generationValidation: null,
      }));
      toast.success('Video prompt branched');
    } catch (err) {
      toast.error((err as Error).message);
    }
  },

  regenerateFromGeneration: async (generationId) => {
    try {
      if (get().isGenerating) {
        toast.error('Wait for the current video generation to finish first');
        return;
      }
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
      const inputAssets = parseGenerationInputAssets(branch.input_assets_json, branch.input_asset_ids_json);
      set((state) => ({
        activeProjectId: branch.project_id,
        activeGenerationId: branch.parent_id,
        selectedProvider: branch.provider,
        selectedModel: branch.model,
        promptForm: {
          ...state.promptForm,
          ...settings,
          start_image_asset_id: undefined,
          last_frame_asset_id: undefined,
          source_video_asset_id: undefined,
          reference_asset_ids: [],
          ...inputAssets,
          prompt: branch.enhanced_prompt || branch.prompt,
          negative_prompt: branch.negative_prompt || '',
          enhance: false,
          place_on_timeline: false,
        },
        generationValidation: null,
        error: null,
      }));
      toast.info('Regenerating with the previous effective prompt and settings');
      get().generate(branch.parent_id);
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
          timelineUndoStack: activeDeleted ? [] : state.timelineUndoStack,
          timelineRedoStack: activeDeleted ? [] : state.timelineRedoStack,
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
    const { _pollInterval } = get();
    if (_pollInterval) clearInterval(_pollInterval);
    set({
      isGenerating: false,
      generationProgress: null,
      abortGeneration: null,
      _pollInterval: null,
    });
  },

  cancelGeneration: async () => {
    const { activeGenerationId, _pollInterval } = get();
    if (_pollInterval) clearInterval(_pollInterval);
    set({ isGenerating: false, generationProgress: null, _pollInterval: null });
    if (activeGenerationId) {
      try {
        await videoApi.cancelGeneration(activeGenerationId);
        set((state) => ({
          generations: upsertGeneration(state.generations, {
            ...state.generations.find((g) => g.id === activeGenerationId)!,
            status: 'cancelled',
          }),
        }));
        toast.success('Generation cancelled');
      } catch {
        // Non-fatal \u2014 already cleared local state
      }
    }
  },

  selectAsset: (assetId) => set({ selectedAssetId: assetId }),

  loadTimeline: async (projectId) => {
    const id = projectId || get().activeProjectId;
    if (!id) return;
    try {
      const detail = await videoApi.getTimeline(id);
      set({
        timelineRecord: detail.timeline,
        timeline: detail.document,
        timelineUndoStack: [],
        timelineRedoStack: [],
        selectedClipId: null,
        selectedClipIds: [],
        selectedTrackId: null,
      });
    } catch (err) {
      set({
        timeline: defaultTimeline(get().projects.find((project) => project.id === id)),
        timelineUndoStack: [],
        timelineRedoStack: [],
        error: (err as Error).message,
      });
    }
  },

  saveTimeline: async (document) => {
    const projectId = get().activeProjectId;
    const doc = document || get().timeline;
    if (!projectId || !doc) return;
    const seq = get()._saveSeq + 1;
    set({ isSavingTimeline: true, _saveSeq: seq });
    try {
      const detail = await videoApi.saveTimeline(projectId, doc);
      // Only the latest save's response may update local state — applying an
      // older response (rapid consecutive edits, out-of-order replies) would
      // clobber newer local edits.
      if (get()._saveSeq !== seq) return;
      if (get().activeProjectId === projectId) {
        set({ timelineRecord: detail.timeline, timeline: detail.document, isSavingTimeline: false });
      } else {
        set({ isSavingTimeline: false });
      }
    } catch (err) {
      if (get()._saveSeq === seq) {
        set({ isSavingTimeline: false, error: (err as Error).message });
      }
      toast.error('Could not save timeline');
    }
  },

  addAssetToTimeline: async (assetId, options = {}) => {
    const projectId = get().activeProjectId;
    if (!projectId) return;
    const asset = get().assets.find((item) => item.id === assetId);
    const previous = get().timeline ? cloneTimeline(get().timeline as VideoTimelineDocument) : null;
    // Without an explicit target, prefer the selected layer; the backend
    // otherwise picks the first unlocked track that accepts the asset.
    const selectedTrack = !options.track_id && get().selectedTrackId
      ? get().timeline?.tracks.find((track) => track.id === get().selectedTrackId && !track.locked)
      : undefined;
    try {
      const detail = await videoApi.importAssetToTimeline(projectId, {
        asset_id: assetId,
        track_id: options.track_id ?? selectedTrack?.id,
        track_type: options.track_type || assetTrackType(asset),
        start_ms: options.start_ms ?? get().playheadMs,
        duration_ms: options.duration_ms,
      });
      set((state) => ({
        timelineRecord: detail.timeline,
        timeline: detail.document,
        selectedAssetId: assetId,
        ...(previous ? withTimelineHistory(state, previous) : { timelineRedoStack: [] }),
      }));
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
    const newStart = Math.max(0, Math.round(startMs));
    const delta = newStart - loc.clip.start_ms;
    // When the dragged clip is part of the current selection (including a
    // group selection), the rest of the selection shifts by the same delta on
    // their own tracks.
    const selected = get().selectedClipIds;
    const companions = selected.includes(clipId) ? selected.filter((id) => id !== clipId) : [];
    const [clip] = loc.track.clips.splice(loc.clipIndex, 1);
    clip.start_ms = newStart;
    const target = next.tracks.find((track) => track.id === trackId) || loc.track;
    target.clips.push(clip);
    for (const companionId of companions) {
      const companion = findClip(next, companionId);
      if (!companion || companion.track.locked) continue;
      companion.clip.start_ms = Math.max(0, companion.clip.start_ms + delta);
    }
    for (const track of next.tracks) {
      track.clips.sort((a, b) => a.start_ms - b.start_ms);
    }
    recomputeDuration(next);
    set((state) => ({
      timeline: next,
      selectedClipId: clip.id,
      selectedClipIds: companions.length > 0 ? selected : [clip.id],
      selectedTrackId: target.id,
      ...withTimelineHistory(state, current),
    }));
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
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  splitClipAtPlayhead: async () => {
    const clipId = get().selectedClipId;
    if (!clipId) return;
    await get().splitClipAt(clipId, get().playheadMs);
  },

  splitClipAt: async (clipId, timeMs) => {
    const current = get().timeline;
    if (!current || !clipId) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    const offset = timeMs - loc.clip.start_ms;
    if (offset <= 0 || offset >= loc.clip.duration_ms) {
      toast.error('Split point must be inside the clip');
      return;
    }
    // Keyframes are clip-relative: the left half keeps keyframes up to the
    // split point; the right half keeps the rest rebased to its new start.
    // Fades split with their edge — fade-in stays left, fade-out stays right.
    const keyframes = loc.clip.keyframes || [];
    const left = {
      ...loc.clip,
      duration_ms: offset,
      trim_out_ms: loc.clip.trim_in_ms + offset,
      fade_out_ms: undefined,
      keyframes: keyframes.filter((keyframe) => keyframe.time_ms <= offset),
    };
    const right = {
      ...loc.clip,
      id: newId('clip'),
      start_ms: timeMs,
      duration_ms: loc.clip.duration_ms - offset,
      trim_in_ms: loc.clip.trim_in_ms + offset,
      trim_out_ms: loc.clip.trim_out_ms,
      fade_in_ms: undefined,
      keyframes: keyframes
        .filter((keyframe) => keyframe.time_ms >= offset)
        .map((keyframe) => ({ ...keyframe, id: newId('keyframe'), time_ms: keyframe.time_ms - offset })),
    };
    loc.track.clips.splice(loc.clipIndex, 1, left, right);
    set((state) => ({
      timeline: recomputeDuration(next),
      selectedClipId: right.id,
      selectedClipIds: [right.id],
      selectedTrackId: loc.track.id,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  trimClipEdgeToPlayhead: async (edge) => {
    const current = get().timeline;
    const clipId = get().selectedClipId;
    if (!current || !clipId) return;
    const loc = findClip(current, clipId);
    if (!loc) return;
    const playhead = get().playheadMs;
    const offset = playhead - loc.clip.start_ms;
    if (offset <= 0 || offset >= loc.clip.duration_ms) {
      toast.error('Move the playhead inside the selected clip');
      return;
    }
    if (edge === 'start') {
      await get().trimClip(clipId, {
        start_ms: playhead,
        duration_ms: loc.clip.duration_ms - offset,
        trim_in_ms: loc.clip.trim_in_ms + offset,
        trim_out_ms: loc.clip.trim_out_ms,
      });
    } else {
      await get().trimClip(clipId, {
        duration_ms: offset,
        trim_out_ms: loc.clip.trim_in_ms + offset,
      });
    }
  },

  groupClips: async (clipIds) => {
    const current = get().timeline;
    const ids = clipIds && clipIds.length > 0 ? clipIds : get().selectedClipIds;
    if (!current) return;
    if (ids.length < 2) {
      toast.error('Select at least two clips to group');
      return;
    }
    const next = cloneTimeline(current);
    const groupId = newId('group');
    for (const id of ids) {
      const loc = findClip(next, id);
      if (loc) loc.clip.group_id = groupId;
    }
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  ungroupClips: async (groupId) => {
    const current = get().timeline;
    if (!current) return;
    let target = groupId;
    if (!target) {
      const selectedId = get().selectedClipId;
      const loc = selectedId ? findClip(current, selectedId) : null;
      target = loc?.clip.group_id;
    }
    if (!target) return;
    const next = cloneTimeline(current);
    for (const track of next.tracks) {
      for (const clip of track.clips) {
        if (clip.group_id === target) delete clip.group_id;
      }
    }
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  alignSelection: async (mode) => {
    const current = get().timeline;
    const ids = get().selectedClipIds;
    if (!current) return;
    if (ids.length < 2) {
      toast.error('Select at least two clips');
      return;
    }
    const next = cloneTimeline(current);
    const locs = ids
      .map((id) => findClip(next, id))
      .filter((loc): loc is NonNullable<typeof loc> => Boolean(loc));
    if (locs.length < 2) return;
    if (mode === 'start') {
      const minStart = Math.min(...locs.map((loc) => loc.clip.start_ms));
      for (const loc of locs) loc.clip.start_ms = minStart;
    } else if (mode === 'end') {
      const maxEnd = Math.max(...locs.map((loc) => loc.clip.start_ms + loc.clip.duration_ms));
      for (const loc of locs) loc.clip.start_ms = Math.max(0, maxEnd - loc.clip.duration_ms);
    } else {
      const sorted = [...locs].sort((a, b) => a.clip.start_ms - b.clip.start_ms);
      const span = sorted[sorted.length - 1].clip.start_ms - sorted[0].clip.start_ms;
      if (sorted.length > 2 && span > 0) {
        const step = span / (sorted.length - 1);
        sorted.forEach((loc, index) => {
          loc.clip.start_ms = Math.round(sorted[0].clip.start_ms + step * index);
        });
      }
    }
    for (const track of next.tracks) {
      track.clips.sort((a, b) => a.start_ms - b.start_ms);
    }
    set((state) => ({
      timeline: recomputeDuration(next),
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  setToolMode: (mode) => set({ toolMode: mode }),

  setEditorMode: (mode) => {
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(EDITOR_MODE_STORAGE_KEY, mode);
    }
    set({ editorMode: mode });
  },

  addCaptionSegment: async (text = 'New caption') => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const track = ensureCaptionTrack(next);
    const start = Math.max(0, Math.round(get().playheadMs));
    const clip = captionClipFromCue({ start_ms: start, end_ms: start + 2000, text }, next.canvas, defaultCaptionPreset(next));
    track.clips.push(clip);
    track.clips.sort((a, b) => a.start_ms - b.start_ms);
    set((state) => ({
      timeline: recomputeDuration(next),
      selectedClipId: clip.id,
      selectedClipIds: [clip.id],
      selectedTrackId: track.id,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  importCaptions: async (raw) => {
    const current = get().timeline;
    if (!current) return;
    const cues = parseCaptions(raw);
    if (cues.length === 0) {
      toast.error('No caption cues found in the file');
      return;
    }
    const next = cloneTimeline(current);
    const track = ensureCaptionTrack(next);
    for (const cue of cues) {
      track.clips.push(captionClipFromCue(cue, next.canvas, defaultCaptionPreset(next)));
    }
    track.clips.sort((a, b) => a.start_ms - b.start_ms);
    set((state) => ({
      timeline: recomputeDuration(next),
      selectedTrackId: track.id,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
    toast.success(`Imported ${cues.length} caption${cues.length === 1 ? '' : 's'}`);
  },

  exportCaptions: (format) => {
    const timeline = get().timeline;
    if (!timeline) return null;
    const cues: CaptionCue[] = timeline.tracks
      .filter((track) => track.type === 'caption')
      .flatMap((track) => track.clips)
      .filter((clip) => clip.text?.text?.trim())
      .map((clip) => ({
        start_ms: clip.start_ms,
        end_ms: clip.start_ms + clip.duration_ms,
        text: clip.text?.text || '',
      }))
      .sort((a, b) => a.start_ms - b.start_ms);
    if (cues.length === 0) {
      toast.error('No caption clips to export');
      return null;
    }
    return format === 'srt' ? serializeSrt(cues) : serializeVtt(cues);
  },

  mergeCaptionClipWithNext: async (clipId) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    const following = loc.track.clips
      .filter((clip) => clip.id !== clipId && clip.start_ms >= loc.clip.start_ms)
      .sort((a, b) => a.start_ms - b.start_ms)[0];
    if (!following) {
      toast.error('No following caption to merge with');
      return;
    }
    const end = Math.max(loc.clip.start_ms + loc.clip.duration_ms, following.start_ms + following.duration_ms);
    loc.clip.duration_ms = end - loc.clip.start_ms;
    loc.clip.trim_out_ms = loc.clip.trim_in_ms + loc.clip.duration_ms;
    loc.clip.text = {
      ...(loc.clip.text || { text: '' }),
      text: [loc.clip.text?.text || '', following.text?.text || ''].filter(Boolean).join('\n'),
    };
    loc.track.clips = loc.track.clips.filter((clip) => clip.id !== following.id);
    set((state) => ({
      timeline: recomputeDuration(next),
      selectedClipId: loc.clip.id,
      selectedClipIds: [loc.clip.id],
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  applyCaptionPreset: async (presetKey) => {
    const current = get().timeline;
    if (!current) return;
    const preset = CAPTION_PRESETS.find((item) => item.key === presetKey);
    if (!preset) return;
    const hasCaptions = current.tracks.some((track) => track.type === 'caption' && track.clips.length > 0);
    if (!hasCaptions) {
      toast.error('No caption clips yet');
      return;
    }
    const next = cloneTimeline(current);
    const position = preset.position(next.canvas);
    for (const track of next.tracks) {
      if (track.type !== 'caption') continue;
      for (const clip of track.clips) {
        clip.text = { ...(clip.text || { text: '' }), ...preset.text };
        clip.transform = { ...defaultTransform(), ...(clip.transform || {}), x: position.x, y: position.y };
      }
    }
    // The applied style becomes the project default for new/imported captions.
    next.metadata = { ...(next.metadata || {}), default_caption_preset: preset.key };
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  shiftCaptions: async (deltaMs, clipId) => {
    const current = get().timeline;
    if (!current || !Number.isFinite(deltaMs) || deltaMs === 0) return;
    const next = cloneTimeline(current);
    let moved = false;
    for (const track of next.tracks) {
      if (track.type !== 'caption' || track.locked) continue;
      for (const clip of track.clips) {
        if (clipId && clip.id !== clipId) continue;
        const start = Math.max(0, clip.start_ms + Math.round(deltaMs));
        if (start !== clip.start_ms) {
          clip.start_ms = start;
          moved = true;
        }
      }
      track.clips.sort((a, b) => a.start_ms - b.start_ms);
    }
    if (!moved) return;
    set((state) => ({
      timeline: recomputeDuration(next),
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  deleteClip: async (clipId) => {
    const current = get().timeline;
    const ids = clipId
      ? [clipId]
      : get().selectedClipIds.length > 0
        ? get().selectedClipIds
        : get().selectedClipId
          ? [get().selectedClipId as string]
          : [];
    if (!current || ids.length === 0) return;
    const idSet = new Set(ids);
    const next = cloneTimeline(current);
    for (const track of next.tracks) {
      track.clips = track.clips.filter((clip) => !idSet.has(clip.id));
    }
    set((state) => ({
      timeline: recomputeDuration(next),
      selectedClipId: null,
      selectedClipIds: [],
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  duplicateClip: async (clipId) => {
    const current = get().timeline;
    const ids = clipId
      ? [clipId]
      : get().selectedClipIds.length > 0
        ? get().selectedClipIds
        : get().selectedClipId
          ? [get().selectedClipId as string]
          : [];
    if (!current || ids.length === 0) return;
    const next = cloneTimeline(current);
    const copies: { id: string; trackId: string }[] = [];
    for (const id of ids) {
      const loc = findClip(next, id);
      if (!loc) continue;
      const copy = { ...loc.clip, id: newId('clip'), start_ms: loc.clip.start_ms + loc.clip.duration_ms + 250 };
      loc.track.clips.splice(loc.clipIndex + 1, 0, copy);
      copies.push({ id: copy.id, trackId: loc.track.id });
    }
    if (copies.length === 0) return;
    set((state) => ({
      timeline: recomputeDuration(next),
      selectedClipId: copies[copies.length - 1].id,
      selectedClipIds: copies.map((copy) => copy.id),
      selectedTrackId: copies[copies.length - 1].trackId,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  selectClip: (clipId, trackId = null, additive = false) => set((state) => {
    if (!clipId) {
      return { selectedClipId: null, selectedClipIds: [], selectedTrackId: trackId };
    }
    if (additive) {
      const already = state.selectedClipIds.includes(clipId);
      const selectedClipIds = already
        ? state.selectedClipIds.filter((id) => id !== clipId)
        : [...state.selectedClipIds, clipId];
      return {
        selectedClipIds,
        selectedClipId: already ? selectedClipIds[selectedClipIds.length - 1] || null : clipId,
        selectedTrackId: trackId,
      };
    }
    // Clicking a grouped clip selects the whole group.
    let selectedClipIds = [clipId];
    if (state.timeline) {
      const loc = findClip(state.timeline, clipId);
      const groupId = loc?.clip.group_id;
      if (groupId) {
        selectedClipIds = [];
        for (const track of state.timeline.tracks) {
          for (const clip of track.clips) {
            if (clip.group_id === groupId) selectedClipIds.push(clip.id);
          }
        }
      }
    }
    return { selectedClipId: clipId, selectedClipIds, selectedTrackId: trackId };
  }),

  renameAsset: async (assetId, fileName) => {
    const name = fileName.trim();
    if (!name) return;
    try {
      const updated = await videoApi.updateAsset(assetId, { file_name: name });
      set((state) => ({
        assets: state.assets.map((asset) => (asset.id === assetId ? updated : asset)),
      }));
      toast.success('Asset renamed');
    } catch (err) {
      toast.error((err as Error).message);
    }
  },

  deleteAsset: async (assetId) => {
    try {
      await videoApi.deleteAsset(assetId);
      set((state) => ({
        assets: state.assets.filter((asset) => asset.id !== assetId),
        selectedAssetId: state.selectedAssetId === assetId ? null : state.selectedAssetId,
      }));
      toast.success('Asset deleted');
    } catch (err) {
      toast.error((err as Error).message);
    }
  },

  uploadAsset: async (file) => {
    const projectId = get().activeProjectId;
    if (!projectId) {
      toast.error('Select or create a project first');
      return;
    }
    try {
      const asset = await videoApi.uploadAsset(projectId, file);
      set((state) => ({
        assets: [asset, ...state.assets],
        selectedAssetId: asset.id,
      }));
      toast.success(`Uploaded ${asset.file_name}`);
    } catch (err) {
      toast.error((err as Error).message);
    }
  },

  importMusicAsset: async (musicAssetId, title) => {
    // Cross-studio handoff (e.g. Music Studio → media bin): import into the
    // active video project, creating one when none is selected yet.
    let projectId = get().activeProjectId;
    if (!projectId) {
      const project = await get().createProject(title ? `${title} video` : undefined);
      projectId = project?.id ?? null;
    }
    if (!projectId) return null;
    try {
      const asset = await videoApi.importExternalAsset(projectId, {
        source_studio: 'music',
        source_id: musicAssetId,
      });
      if (get().activeProjectId === projectId) {
        set((state) => ({
          assets: [asset, ...state.assets.filter((item) => item.id !== asset.id)],
          selectedAssetId: asset.id,
        }));
      }
      return asset;
    } catch (err) {
      toast.error((err as Error).message);
      return null;
    }
  },

  loadRendererCapabilities: async () => {
    if (get().rendererCapabilities) return;
    try {
      const rendererCapabilities = await videoApi.rendererCapabilities();
      set({ rendererCapabilities });
    } catch {
      // Non-fatal — the inspector falls back to a generic warning.
    }
  },

  setPlayhead: (timeMs) => set((state) => ({ playheadMs: Math.max(0, Math.min(timeMs, state.timeline?.duration_ms || timeMs)) })),
  setZoom: (zoom) => set({ zoom: Math.min(4, Math.max(0.35, zoom)) }),
  zoomToFit: (containerWidth) => {
    const duration = get().timeline?.duration_ms || 30000;
    if (containerWidth <= 0 || duration <= 0) return;
    // Base scale is 0.02 px/ms at zoom 1 (see VideoTimeline pxPerMs).
    get().setZoom(containerWidth / (duration * 0.02));
  },
  setPlaying: (playing) => set({ isPlaying: playing }),
  toggleSnapping: () => set((state) => ({ snappingEnabled: !state.snappingEnabled })),
  toggleRipple: () => set((state) => ({ rippleEnabled: !state.rippleEnabled })),

  removeGap: async (trackId, startMs, endMs) => {
    const current = get().timeline;
    if (!current) return;
    const gap = Math.round(endMs - startMs);
    if (gap <= 0) return;
    const next = cloneTimeline(current);
    const track = next.tracks.find((item) => item.id === trackId);
    if (!track || track.locked) return;
    const threshold = Math.round(endMs);
    let moved = false;
    for (const clip of track.clips) {
      if (clip.start_ms >= threshold) {
        clip.start_ms = Math.max(0, clip.start_ms - gap);
        moved = true;
      }
    }
    if (!moved) return;
    track.clips.sort((a, b) => a.start_ms - b.start_ms);
    set((state) => ({
      timeline: recomputeDuration(next),
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  removeAllGaps: async (trackId) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    let moved = false;
    for (const track of next.tracks) {
      if (track.locked) continue;
      if (trackId && track.id !== trackId) continue;
      const clips = [...track.clips].sort((a, b) => a.start_ms - b.start_ms);
      // Compact clip-to-clip; the first clip keeps its position so an
      // intentional delayed start (e.g. late music bed) survives.
      let cursor: number | null = null;
      for (const clip of clips) {
        if (cursor !== null && clip.start_ms > cursor) {
          clip.start_ms = cursor;
          moved = true;
        }
        cursor = Math.max(cursor ?? clip.start_ms, clip.start_ms) + clip.duration_ms;
      }
      track.clips = clips;
    }
    if (!moved) return;
    set((state) => ({
      timeline: recomputeDuration(next),
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  rippleDeleteClip: async (clipId) => {
    const current = get().timeline;
    const ids = clipId
      ? [clipId]
      : get().selectedClipIds.length > 0
        ? get().selectedClipIds
        : get().selectedClipId
          ? [get().selectedClipId as string]
          : [];
    if (!current || ids.length === 0) return;
    const idSet = new Set(ids);
    const next = cloneTimeline(current);
    let removedAny = false;
    for (const track of next.tracks) {
      if (track.locked) continue;
      const removed = track.clips.filter((clip) => idSet.has(clip.id));
      if (removed.length === 0) continue;
      track.clips = track.clips.filter((clip) => !idSet.has(clip.id));
      // Close each removed span right-to-left so earlier shifts don't move
      // later spans' reference points.
      for (const gone of [...removed].sort((a, b) => b.start_ms - a.start_ms)) {
        for (const clip of track.clips) {
          if (clip.start_ms > gone.start_ms) {
            clip.start_ms = Math.max(0, clip.start_ms - gone.duration_ms);
          }
        }
      }
      track.clips.sort((a, b) => a.start_ms - b.start_ms);
      removedAny = true;
    }
    if (!removedAny) return;
    set((state) => ({
      timeline: recomputeDuration(next),
      selectedClipId: null,
      selectedClipIds: [],
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  rippleTrimClip: async (clipId, edge, newTimeMs) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc || loc.track.locked) return;
    const clip = loc.clip;
    const oldEnd = clip.start_ms + clip.duration_ms;
    if (edge === 'start') {
      // Trim from the left, then pull the clip (and everything after it on
      // the layer) back so the clip still begins where it used to.
      const delta = Math.round(Math.min(Math.max(newTimeMs - clip.start_ms, 0), clip.duration_ms - 100));
      if (delta <= 0) return;
      clip.trim_in_ms = Math.max(0, clip.trim_in_ms + delta);
      clip.duration_ms -= delta;
      clip.trim_out_ms = Math.max(clip.trim_in_ms + 100, clip.trim_out_ms ?? clip.trim_in_ms + clip.duration_ms);
      for (const other of loc.track.clips) {
        if (other.id !== clip.id && other.start_ms >= oldEnd) {
          other.start_ms = Math.max(0, other.start_ms - delta);
        }
      }
    } else {
      const newDuration = Math.round(Math.max(100, newTimeMs - clip.start_ms));
      const delta = newDuration - clip.duration_ms;
      if (delta === 0) return;
      clip.duration_ms = newDuration;
      clip.trim_out_ms = clip.trim_in_ms + newDuration;
      for (const other of loc.track.clips) {
        if (other.id !== clip.id && other.start_ms >= oldEnd) {
          other.start_ms = Math.max(0, other.start_ms + delta);
        }
      }
    }
    loc.track.clips.sort((a, b) => a.start_ms - b.start_ms);
    set((state) => ({
      timeline: recomputeDuration(next),
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  insertClipAt: async (assetId, trackId, timeMs, ripple) => {
    const current = get().timeline;
    const asset = get().assets.find((item) => item.id === assetId);
    if (!current) return;
    if (!asset) {
      toast.error('Asset not found in this project');
      return;
    }
    const next = cloneTimeline(current);
    const track = next.tracks.find((item) => item.id === trackId);
    if (!track || track.locked) return;
    const at = Math.max(0, Math.round(timeMs));
    const duration = Math.max(100, Math.round(asset.duration_ms || 4000));
    if (ripple ?? get().rippleEnabled) {
      // Split any clip straddling the insert point; everything at/after the
      // point shifts right to make room.
      const result: VideoTimelineClip[] = [];
      for (const clip of track.clips) {
        const offset = at - clip.start_ms;
        if (offset > 0 && offset < clip.duration_ms) {
          const right = JSON.parse(JSON.stringify(clip)) as VideoTimelineClip;
          right.id = newId('clip');
          right.start_ms = at;
          right.duration_ms = clip.duration_ms - offset;
          right.trim_in_ms = clip.trim_in_ms + offset;
          right.trim_out_ms = clip.trim_out_ms;
          right.fade_in_ms = undefined;
          right.keyframes = (clip.keyframes || [])
            .filter((keyframe) => keyframe.time_ms >= offset)
            .map((keyframe) => ({ ...keyframe, id: newId('keyframe'), time_ms: keyframe.time_ms - offset }));
          clip.duration_ms = offset;
          clip.trim_out_ms = clip.trim_in_ms + offset;
          clip.fade_out_ms = undefined;
          clip.keyframes = (clip.keyframes || []).filter((keyframe) => keyframe.time_ms <= offset);
          result.push(clip, right);
        } else {
          result.push(clip);
        }
      }
      for (const clip of result) {
        if (clip.start_ms >= at) clip.start_ms += duration;
      }
      track.clips = result;
    }
    const inserted: VideoTimelineClip = {
      id: newId('clip'),
      asset_id: assetId,
      start_ms: at,
      duration_ms: duration,
      trim_in_ms: 0,
      trim_out_ms: duration,
      transform: defaultTransform(),
      effects: [],
      keyframes: [],
      transitions: [],
    };
    track.clips.push(inserted);
    track.clips.sort((a, b) => a.start_ms - b.start_ms);
    set((state) => ({
      timeline: recomputeDuration(next),
      selectedClipId: inserted.id,
      selectedClipIds: [inserted.id],
      selectedTrackId: track.id,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  overwriteClipAt: async (assetId, trackId, timeMs) => {
    const current = get().timeline;
    const asset = get().assets.find((item) => item.id === assetId);
    if (!current) return;
    if (!asset) {
      toast.error('Asset not found in this project');
      return;
    }
    const next = cloneTimeline(current);
    const track = next.tracks.find((item) => item.id === trackId);
    if (!track || track.locked) return;
    const at = Math.max(0, Math.round(timeMs));
    const duration = Math.max(100, Math.round(asset.duration_ms || 4000));
    const end = at + duration;
    // Carve out [at, end): clips inside vanish, straddlers get trimmed, and a
    // clip spanning the whole region splits around it.
    const result: VideoTimelineClip[] = [];
    for (const clip of track.clips) {
      const clipEnd = clip.start_ms + clip.duration_ms;
      if (clipEnd <= at || clip.start_ms >= end) {
        result.push(clip);
        continue;
      }
      const leftKeep = clip.start_ms < at;
      const rightKeep = clipEnd > end;
      if (leftKeep) {
        const left = rightKeep ? (JSON.parse(JSON.stringify(clip)) as VideoTimelineClip) : clip;
        const offset = at - clip.start_ms;
        left.duration_ms = offset;
        left.trim_out_ms = left.trim_in_ms + offset;
        left.fade_out_ms = undefined;
        left.keyframes = (left.keyframes || []).filter((keyframe) => keyframe.time_ms <= offset);
        result.push(left);
      }
      if (rightKeep) {
        const shift = end - clip.start_ms;
        const right = clip;
        const newRightId = leftKeep ? newId('clip') : right.id;
        right.id = newRightId;
        right.trim_in_ms = right.trim_in_ms + shift;
        right.duration_ms = clipEnd - end;
        right.start_ms = end;
        right.fade_in_ms = undefined;
        right.keyframes = (right.keyframes || [])
          .filter((keyframe) => keyframe.time_ms >= shift)
          .map((keyframe) => ({ ...keyframe, id: newId('keyframe'), time_ms: keyframe.time_ms - shift }));
        result.push(right);
      }
    }
    const inserted: VideoTimelineClip = {
      id: newId('clip'),
      asset_id: assetId,
      start_ms: at,
      duration_ms: duration,
      trim_in_ms: 0,
      trim_out_ms: duration,
      transform: defaultTransform(),
      effects: [],
      keyframes: [],
      transitions: [],
    };
    result.push(inserted);
    result.sort((a, b) => a.start_ms - b.start_ms);
    track.clips = result;
    set((state) => ({
      timeline: recomputeDuration(next),
      selectedClipId: inserted.id,
      selectedClipIds: [inserted.id],
      selectedTrackId: track.id,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  toggleTrackMute: async (trackId) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const track = next.tracks.find((item) => item.id === trackId);
    if (!track) return;
    track.muted = !track.muted;
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  toggleTrackLock: async (trackId) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const track = next.tracks.find((item) => item.id === trackId);
    if (!track) return;
    track.locked = !track.locked;
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  toggleTrackVisibility: async (trackId) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const track = next.tracks.find((item) => item.id === trackId);
    if (!track) return;
    track.visible = !track.visible;
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  updateClipTransform: async (clipId, transform) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.transform = { ...defaultTransform(), ...(loc.clip.transform || {}), ...transform };
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  updateClipVolume: async (clipId, volume) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.volume = Math.min(2, Math.max(0, volume));
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
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
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  removeClipFades: async (clipIds) => {
    const current = get().timeline;
    const ids = clipIds && clipIds.length > 0
      ? clipIds
      : get().selectedClipIds.length > 0
        ? get().selectedClipIds
        : get().selectedClipId
          ? [get().selectedClipId as string]
          : [];
    if (!current || ids.length === 0) return;
    const next = cloneTimeline(current);
    let changed = false;
    for (const id of ids) {
      const loc = findClip(next, id);
      if (!loc || loc.track.locked) continue;
      if (loc.clip.fade_in_ms || loc.clip.fade_out_ms) {
        loc.clip.fade_in_ms = undefined;
        loc.clip.fade_out_ms = undefined;
        changed = true;
      }
    }
    if (!changed) return;
    set((state) => ({ timeline: next, ...withTimelineHistory(state, current) }));
    await get().saveTimeline(next);
  },

  duckMusicUnderNarration: async (duckTo = 0.3, rampMs = 250) => {
    const current = get().timeline;
    if (!current) return;
    const assets = get().assets;
    const assetKind = (clip: VideoTimelineClip) => assets.find((item) => item.id === clip.asset_id)?.kind;
    // Narration = audio assets and detached-audio clips on audible tracks.
    const narration: Array<{ start: number; end: number }> = [];
    for (const track of current.tracks) {
      if (track.muted) continue;
      for (const clip of track.clips) {
        if (clip.muted) continue;
        if (assetKind(clip) === 'audio' || clip.audio_only) {
          narration.push({ start: clip.start_ms, end: clip.start_ms + clip.duration_ms });
        }
      }
    }
    if (narration.length === 0) {
      toast.info('No narration (audio clips) found to duck under');
      return;
    }
    narration.sort((a, b) => a.start - b.start);
    const merged: Array<{ start: number; end: number }> = [];
    for (const interval of narration) {
      const last = merged[merged.length - 1];
      if (last && interval.start <= last.end) {
        last.end = Math.max(last.end, interval.end);
      } else {
        merged.push({ ...interval });
      }
    }
    const next = cloneTimeline(current);
    let ducked = 0;
    for (const track of next.tracks) {
      if (track.locked) continue;
      for (const clip of track.clips) {
        if (assetKind(clip) !== 'music') continue;
        const base = clip.volume ?? 1;
        const duck = Math.round(base * Math.max(0, Math.min(1, duckTo)) * 100) / 100;
        const clipEnd = clip.start_ms + clip.duration_ms;
        const keyframes: VideoTimelineKeyframe[] = [];
        const addPoint = (timeMs: number, value: number) => {
          const clamped = Math.max(0, Math.min(clip.duration_ms, Math.round(timeMs)));
          if (keyframes.some((keyframe) => keyframe.time_ms === clamped)) return;
          keyframes.push({ id: newId('keyframe'), property: 'volume', time_ms: clamped, value, easing: 'linear' });
        };
        let overlapped = false;
        for (const interval of merged) {
          const start = Math.max(interval.start, clip.start_ms);
          const end = Math.min(interval.end, clipEnd);
          if (end <= start) continue;
          overlapped = true;
          addPoint(start - clip.start_ms - rampMs, base);
          addPoint(start - clip.start_ms, duck);
          addPoint(end - clip.start_ms, duck);
          addPoint(end - clip.start_ms + rampMs, base);
        }
        if (!overlapped) continue;
        // Deterministic: regenerate the volume envelope; other properties keep
        // their keyframes.
        clip.keyframes = [
          ...(clip.keyframes || []).filter((keyframe) => keyframe.property !== 'volume'),
          ...keyframes.sort((a, b) => a.time_ms - b.time_ms),
        ];
        ducked += 1;
      }
    }
    if (ducked === 0) {
      toast.info('No music clips overlap narration');
      return;
    }
    set((state) => ({ timeline: next, ...withTimelineHistory(state, current) }));
    await get().saveTimeline(next);
    toast.success(`Ducked ${ducked} music clip${ducked === 1 ? '' : 's'} under narration`);
  },

  addTextClip: async (text = 'Title card', options = {}) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    // Explicit target first, then the topmost (foreground) unlocked generic
    // layer, then a legacy text track, then a new layer on top.
    let track = (options.trackId ? next.tracks.find((item) => item.id === options.trackId && !item.locked) : undefined)
      ?? [...next.tracks].reverse().find((item) => item.type === 'layer' && !item.locked)
      ?? next.tracks.find((item) => item.type === 'text' && !item.locked);
    if (!track) {
      track = { id: newId('track'), type: 'layer', name: `Layer ${next.tracks.length + 1}`, locked: false, muted: false, visible: true, clips: [] };
      next.tracks.push(track);
    }
    const clip: VideoTimelineClip = {
      id: newId('clip'),
      start_ms: Math.max(0, Math.round(options.startMs ?? get().playheadMs)),
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
    set((state) => ({
      timeline: recomputeDuration(next),
      selectedClipId: clip.id,
      selectedTrackId: track.id,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  updateClipText: async (clipId, text) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.text = { ...(loc.clip.text || { text: '' }), ...text };
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  addShapeClip: async (kind, options = {}) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    let track = (options.trackId ? next.tracks.find((item) => item.id === options.trackId && !item.locked) : undefined)
      ?? [...next.tracks].reverse().find((item) => item.type === 'layer' && !item.locked);
    if (!track) {
      track = { id: newId('track'), type: 'layer', name: `Layer ${next.tracks.length + 1}`, locked: false, muted: false, visible: true, clips: [] };
      next.tracks.push(track);
    }
    // Per-kind creation defaults (colors, sizes, label text) live in the
    // annotation registry shared with the palette and inspector.
    const defaults = annotationDefaults(kind, next.canvas);
    const clip: VideoTimelineClip = {
      id: newId('clip'),
      start_ms: Math.max(0, Math.round(options.startMs ?? get().playheadMs)),
      duration_ms: 4000,
      trim_in_ms: 0,
      trim_out_ms: 4000,
      transform: { ...defaultTransform(), opacity: defaults.opacity },
      shape: defaults.shape,
      ...(defaults.text ? { text: { ...defaults.text } } : {}),
      effects: [],
      keyframes: [],
      transitions: [],
    };
    track.clips.push(clip);
    set((state) => ({
      timeline: recomputeDuration(next),
      selectedClipId: clip.id,
      selectedClipIds: [clip.id],
      selectedTrackId: track.id,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  updateClipShape: async (clipId, patch) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc || !loc.clip.shape) return;
    loc.clip.shape = { ...loc.clip.shape, ...patch };
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  applyAnnotationPreset: async (clipId, presetKey) => {
    const current = get().timeline;
    const preset = ANNOTATION_PRESETS.find((item) => item.key === presetKey);
    if (!current || !preset) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc || !loc.clip.shape) return;
    loc.clip.shape = { ...loc.clip.shape, ...preset.shape };
    if (preset.text) {
      // Presets restyle text but never replace what the user wrote.
      loc.clip.text = { ...(loc.clip.text || { text: '' }), ...preset.text, text: loc.clip.text?.text || '' };
    }
    if (preset.opacity !== undefined) {
      loc.clip.transform = { ...defaultTransform(), ...(loc.clip.transform || {}), opacity: preset.opacity };
    }
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  resizeShapeClip: async (clipId, patch) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc || !loc.clip.shape) return;
    if (patch.width !== undefined) loc.clip.shape.width = Math.max(8, Math.round(patch.width));
    if (patch.height !== undefined) loc.clip.shape.height = Math.max(8, Math.round(patch.height));
    if (patch.x !== undefined || patch.y !== undefined) {
      loc.clip.transform = {
        ...defaultTransform(),
        ...(loc.clip.transform || {}),
        ...(patch.x !== undefined ? { x: Math.round(patch.x) } : {}),
        ...(patch.y !== undefined ? { y: Math.round(patch.y) } : {}),
      };
    }
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  addClipEffect: async (clipId, effect) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.effects = [...(loc.clip.effects || []), { ...effect, id: newId('effect') }];
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  toggleClipEffect: async (clipId, effectId) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.effects = (loc.clip.effects || []).map((effect) => effect.id === effectId ? { ...effect, enabled: !effect.enabled } : effect);
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  removeClipEffect: async (clipId, effectId) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.effects = (loc.clip.effects || []).filter((effect) => effect.id !== effectId);
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  addClipTransition: async (clipId, transition) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.transitions = [...(loc.clip.transitions || []), { ...transition, id: newId('transition') }];
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  addKeyframe: async (clipId, keyframe) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.keyframes = [...(loc.clip.keyframes || []), { ...keyframe, id: newId('keyframe') }];
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  applyMotionPreset: async (clipId, presetKey) => {
    const current = get().timeline;
    const preset = motionPreset(presetKey);
    if (!current || !preset) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc || loc.track.locked) return;
    const generated = preset.build(loc.clip, next.canvas)
      .map((spec) => ({ ...spec, id: newId('keyframe') }));
    // Motion presets own the pan/zoom envelope; other properties keep theirs.
    loc.clip.keyframes = [
      ...(loc.clip.keyframes || []).filter((keyframe) => !['x', 'y', 'scale'].includes(keyframe.property)),
      ...generated,
    ];
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
    toast.success(generated.length > 0 ? `Applied "${preset.label}" motion` : 'Removed pan/zoom motion');
  },

  updateClipTransition: async (clipId, transitionId, patch) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.transitions = (loc.clip.transitions || []).map((transition) =>
      transition.id === transitionId ? { ...transition, ...patch } : transition,
    );
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  removeClipTransition: async (clipId, transitionId) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.transitions = (loc.clip.transitions || []).filter((transition) => transition.id !== transitionId);
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  updateClipEffect: async (clipId, effectId, patch) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.effects = (loc.clip.effects || []).map((effect) =>
      effect.id === effectId ? { ...effect, ...patch, params: { ...effect.params, ...(patch.params || {}) } } : effect,
    );
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  reorderClipEffect: async (clipId, effectId, direction) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    const effects = loc.clip.effects || [];
    const index = effects.findIndex((effect) => effect.id === effectId);
    const targetIndex = index + direction;
    if (index === -1 || targetIndex < 0 || targetIndex >= effects.length) return;
    [effects[index], effects[targetIndex]] = [effects[targetIndex], effects[index]];
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  updateKeyframe: async (clipId, keyframeId, patch) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.keyframes = (loc.clip.keyframes || []).map((keyframe) =>
      keyframe.id === keyframeId ? { ...keyframe, ...patch, time_ms: Math.max(0, patch.time_ms ?? keyframe.time_ms) } : keyframe,
    );
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  removeKeyframe: async (clipId, keyframeId) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.keyframes = (loc.clip.keyframes || []).filter((keyframe) => keyframe.id !== keyframeId);
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  addTrack: async (type = 'layer', name) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const count = next.tracks.filter((track) => track.type === type).length;
    const defaultName = type === 'layer' ? `Layer ${next.tracks.length + 1}` : `${type.charAt(0).toUpperCase()}${type.slice(1)} ${count + 1}`;
    const track: VideoTimelineTrack = {
      id: newId('track'),
      type,
      name: name?.trim() || defaultName,
      locked: false,
      muted: false,
      visible: true,
      clips: [],
    };
    next.tracks.push(track);
    set((state) => ({
      timeline: next,
      selectedTrackId: track.id,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  removeTrack: async (trackId) => {
    const current = get().timeline;
    if (!current) return;
    const track = current.tracks.find((item) => item.id === trackId);
    if (!track) return;
    const next = cloneTimeline(current);
    next.tracks = next.tracks.filter((item) => item.id !== trackId);
    const removedClipIds = new Set(track.clips.map((clip) => clip.id));
    set((state) => ({
      timeline: recomputeDuration(next),
      selectedTrackId: state.selectedTrackId === trackId ? null : state.selectedTrackId,
      selectedClipId: state.selectedClipId && removedClipIds.has(state.selectedClipId) ? null : state.selectedClipId,
      selectedClipIds: state.selectedClipIds.filter((id) => !removedClipIds.has(id)),
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  renameTrack: async (trackId, name) => {
    const trimmed = name.trim();
    const current = get().timeline;
    if (!current || !trimmed) return;
    const next = cloneTimeline(current);
    const track = next.tracks.find((item) => item.id === trackId);
    if (!track || track.name === trimmed) return;
    track.name = trimmed;
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  reorderTrack: async (trackId, targetIndex) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const fromIndex = next.tracks.findIndex((item) => item.id === trackId);
    if (fromIndex === -1) return;
    const toIndex = Math.max(0, Math.min(next.tracks.length - 1, targetIndex));
    if (toIndex === fromIndex) return;
    const [track] = next.tracks.splice(fromIndex, 1);
    next.tracks.splice(toIndex, 0, track);
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  setTrackHeight: async (trackId, height) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const track = next.tracks.find((item) => item.id === trackId);
    if (!track) return;
    track.height = Math.max(32, Math.min(160, Math.round(height)));
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  duplicateTrack: async (trackId) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const index = next.tracks.findIndex((item) => item.id === trackId);
    if (index === -1) return;
    const copy = JSON.parse(JSON.stringify(next.tracks[index])) as VideoTimelineTrack;
    copy.id = newId('track');
    copy.name = `${copy.name} copy`;
    for (const clip of copy.clips) {
      clip.id = newId('clip');
      delete clip.group_id;
      clip.effects = (clip.effects || []).map((effect) => ({ ...effect, id: newId('effect') }));
      clip.keyframes = (clip.keyframes || []).map((keyframe) => ({ ...keyframe, id: newId('keyframe') }));
      clip.transitions = (clip.transitions || []).map((transition) => ({ ...transition, id: newId('transition') }));
    }
    // Insert directly above the source — toward the foreground.
    next.tracks.splice(index + 1, 0, copy);
    set((state) => ({
      timeline: next,
      selectedTrackId: copy.id,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  insertTrackAdjacent: async (trackId, where) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const index = next.tracks.findIndex((item) => item.id === trackId);
    if (index === -1) return;
    const track: VideoTimelineTrack = {
      id: newId('track'),
      type: 'layer',
      name: `Layer ${next.tracks.length + 1}`,
      locked: false,
      muted: false,
      visible: true,
      clips: [],
    };
    // "above" = toward the foreground = higher array index.
    next.tracks.splice(where === 'above' ? index + 1 : index, 0, track);
    set((state) => ({
      timeline: next,
      selectedTrackId: track.id,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  clearTrack: async (trackId) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const target = next.tracks.find((item) => item.id === trackId);
    if (!target || target.clips.length === 0) return;
    const removed = new Set(target.clips.map((clip) => clip.id));
    target.clips = [];
    set((state) => ({
      timeline: recomputeDuration(next),
      selectedClipId: state.selectedClipId && removed.has(state.selectedClipId) ? null : state.selectedClipId,
      selectedClipIds: state.selectedClipIds.filter((id) => !removed.has(id)),
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  toggleTrackSolo: (trackId) => set((state) => ({ soloTrackId: state.soloTrackId === trackId ? null : trackId })),

  moveTrackToEdge: async (trackId, edge) => {
    const current = get().timeline;
    if (!current) return;
    await get().reorderTrack(trackId, edge === 'top' ? current.tracks.length - 1 : 0);
  },

  setSelectedClips: (ids) => set({
    selectedClipIds: ids,
    selectedClipId: ids[ids.length - 1] || null,
  }),

  selectClipsOnTrack: (trackId) => set((state) => {
    const track = state.timeline?.tracks.find((item) => item.id === trackId);
    if (!track || track.clips.length === 0) return {};
    const ids = track.clips.map((clip) => clip.id);
    return { selectedClipIds: ids, selectedClipId: ids[ids.length - 1], selectedTrackId: trackId };
  }),

  selectClipsRelativeToPlayhead: (which) => set((state) => {
    if (!state.timeline) return {};
    const playhead = state.playheadMs;
    const ids: string[] = [];
    for (const track of state.timeline.tracks) {
      for (const clip of track.clips) {
        const matches = which === 'before'
          ? clip.start_ms < playhead
          : clip.start_ms + clip.duration_ms > playhead;
        if (matches) ids.push(clip.id);
      }
    }
    return { selectedClipIds: ids, selectedClipId: ids[ids.length - 1] || null };
  }),

  selectAllClips: () => set((state) => {
    if (!state.timeline) return {};
    const ids = state.timeline.tracks.flatMap((track) => track.clips.map((clip) => clip.id));
    return { selectedClipIds: ids, selectedClipId: ids[ids.length - 1] || null };
  }),

  moveClipToAdjacentTrack: async (clipId, direction) => {
    const current = get().timeline;
    if (!current) return;
    const loc = findClip(current, clipId);
    if (!loc) return;
    // "above" = toward the foreground (higher array index; the UI shows the
    // track list reversed).
    const target = current.tracks[loc.trackIndex + (direction === 'above' ? 1 : -1)];
    if (!target || target.locked) return;
    await get().moveClip(clipId, target.id, loc.clip.start_ms);
  },

  copySelection: (clipId) => set((state) => {
    const ids = clipId
      ? [clipId]
      : state.selectedClipIds.length > 0
        ? state.selectedClipIds
        : state.selectedClipId
          ? [state.selectedClipId]
          : [];
    if (!state.timeline || ids.length === 0) return {};
    const entries: Array<{ clip: VideoTimelineClip; trackId: string }> = [];
    for (const id of ids) {
      const loc = findClip(state.timeline, id);
      if (loc) entries.push({ clip: JSON.parse(JSON.stringify(loc.clip)) as VideoTimelineClip, trackId: loc.track.id });
    }
    return entries.length > 0 ? { clipClipboard: entries } : {};
  }),

  cutSelection: async (clipId) => {
    get().copySelection(clipId);
    if (!get().clipClipboard?.length) return;
    await get().deleteClip(clipId);
  },

  pasteClips: async (atMs, trackId) => {
    const current = get().timeline;
    const clipboard = get().clipClipboard;
    if (!current || !clipboard || clipboard.length === 0) return;
    const next = cloneTimeline(current);
    const pasteAt = Math.max(0, Math.round(atMs ?? get().playheadMs));
    const earliest = Math.min(...clipboard.map((entry) => entry.clip.start_ms));
    const newIds: string[] = [];
    let lastTrackId: string | null = null;
    for (const entry of clipboard) {
      const clip = JSON.parse(JSON.stringify(entry.clip)) as VideoTimelineClip;
      clip.id = newId('clip');
      delete clip.group_id;
      clip.effects = (clip.effects || []).map((effect) => ({ ...effect, id: newId('effect') }));
      clip.keyframes = (clip.keyframes || []).map((keyframe) => ({ ...keyframe, id: newId('keyframe') }));
      clip.transitions = (clip.transitions || []).map((transition) => ({ ...transition, id: newId('transition') }));
      clip.start_ms = pasteAt + (entry.clip.start_ms - earliest);
      const target = (trackId ? next.tracks.find((track) => track.id === trackId && !track.locked) : undefined)
        ?? next.tracks.find((track) => track.id === entry.trackId && !track.locked)
        ?? next.tracks.find((track) => !track.locked);
      if (!target) continue;
      target.clips.push(clip);
      target.clips.sort((a, b) => a.start_ms - b.start_ms);
      newIds.push(clip.id);
      lastTrackId = target.id;
    }
    if (newIds.length === 0) return;
    set((state) => ({
      timeline: recomputeDuration(next),
      selectedClipIds: newIds,
      selectedClipId: newIds[newIds.length - 1],
      selectedTrackId: lastTrackId,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  copyClipAttributes: (clipId) => set((state) => {
    const id = clipId ?? state.selectedClipId;
    if (!state.timeline || !id) return {};
    const loc = findClip(state.timeline, id);
    if (!loc) return {};
    const { transform, volume, fade_in_ms, fade_out_ms, effects, transitions, text } =
      JSON.parse(JSON.stringify(loc.clip)) as VideoTimelineClip;
    return { attributeClipboard: { transform, volume, fade_in_ms, fade_out_ms, effects, transitions, text } };
  }),

  pasteClipAttributes: async (clipId) => {
    const attrs = get().attributeClipboard;
    const current = get().timeline;
    if (!attrs || !current) return;
    const ids = clipId
      ? [clipId]
      : get().selectedClipIds.length > 0
        ? get().selectedClipIds
        : get().selectedClipId
          ? [get().selectedClipId as string]
          : [];
    if (ids.length === 0) return;
    const next = cloneTimeline(current);
    let applied = false;
    for (const id of ids) {
      const loc = findClip(next, id);
      if (!loc) continue;
      if (attrs.transform) loc.clip.transform = JSON.parse(JSON.stringify(attrs.transform));
      if (attrs.volume !== undefined) loc.clip.volume = attrs.volume;
      if (attrs.fade_in_ms !== undefined) loc.clip.fade_in_ms = attrs.fade_in_ms;
      if (attrs.fade_out_ms !== undefined) loc.clip.fade_out_ms = attrs.fade_out_ms;
      if (attrs.effects) loc.clip.effects = attrs.effects.map((effect) => ({ ...(JSON.parse(JSON.stringify(effect)) as VideoTimelineEffect), id: newId('effect') }));
      if (attrs.transitions) loc.clip.transitions = attrs.transitions.map((transition) => ({ ...(JSON.parse(JSON.stringify(transition)) as VideoTimelineTransition), id: newId('transition') }));
      // Styling pastes onto clips that already have text; their content is kept.
      if (attrs.text && loc.clip.text) loc.clip.text = { ...(JSON.parse(JSON.stringify(attrs.text)) as NonNullable<VideoTimelineClip['text']>), text: loc.clip.text.text };
      applied = true;
    }
    if (!applied) return;
    set((state) => ({ timeline: next, ...withTimelineHistory(state, current) }));
    await get().saveTimeline(next);
  },

  setTimelineDuration: async (durationMs) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    next.duration_ms = Math.max(1000, Math.round(durationMs));
    // Never shrink below the last clip end.
    recomputeDuration(next);
    set((state) => ({ timeline: next, ...withTimelineHistory(state, current) }));
    await get().saveTimeline(next);
  },

  splitAllAtPlayhead: async () => {
    const current = get().timeline;
    if (!current) return;
    const playhead = get().playheadMs;
    const next = cloneTimeline(current);
    let split = false;
    for (const track of next.tracks) {
      if (track.locked) continue;
      const result: VideoTimelineClip[] = [];
      for (const clip of track.clips) {
        const offset = playhead - clip.start_ms;
        if (offset <= 0 || offset >= clip.duration_ms) {
          result.push(clip);
          continue;
        }
        const left = clip;
        const right = JSON.parse(JSON.stringify(clip)) as VideoTimelineClip;
        right.id = newId('clip');
        right.start_ms = playhead;
        right.duration_ms = clip.duration_ms - offset;
        right.trim_in_ms = left.trim_in_ms + offset;
        right.trim_out_ms = clip.trim_out_ms;
        left.duration_ms = offset;
        left.trim_out_ms = left.trim_in_ms + offset;
        result.push(left, right);
        split = true;
      }
      track.clips = result;
    }
    if (!split) return;
    set((state) => ({ timeline: next, ...withTimelineHistory(state, current) }));
    await get().saveTimeline(next);
  },

  toggleClipMute: async (clipId) => {
    const id = clipId ?? get().selectedClipId;
    const current = get().timeline;
    if (!current || !id) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, id);
    if (!loc) return;
    loc.clip.muted = !loc.clip.muted;
    set((state) => ({ timeline: next, ...withTimelineHistory(state, current) }));
    await get().saveTimeline(next);
  },

  detachClipAudio: async (clipId) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc || loc.clip.audio_only) return;
    // The original keeps its visuals but goes silent; an audio-only twin
    // lands on a new layer directly below so it can be edited independently.
    const twin = JSON.parse(JSON.stringify(loc.clip)) as VideoTimelineClip;
    twin.id = newId('clip');
    twin.audio_only = true;
    twin.muted = false;
    twin.transform = undefined;
    twin.text = undefined;
    twin.effects = [];
    twin.transitions = [];
    twin.keyframes = (loc.clip.keyframes || [])
      .filter((keyframe) => keyframe.property === 'volume')
      .map((keyframe) => ({ ...keyframe, id: newId('keyframe') }));
    delete twin.group_id;
    loc.clip.muted = true;
    const track: VideoTimelineTrack = {
      id: newId('track'),
      type: 'layer',
      name: 'Detached audio',
      locked: false,
      muted: false,
      visible: true,
      clips: [twin],
    };
    next.tracks.splice(loc.trackIndex, 0, track);
    set((state) => ({
      timeline: next,
      selectedClipId: twin.id,
      selectedClipIds: [twin.id],
      selectedTrackId: track.id,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
    toast.success('Audio detached to a new layer');
  },

  addAssetAsMusicBed: async (assetId) => {
    // Music beds run the full timeline on the bottom (background) layer.
    const timeline = get().timeline;
    const bottom = timeline?.tracks.find((track) => !track.locked);
    await get().addAssetToTimeline(assetId, {
      track_id: bottom?.id,
      start_ms: 0,
      duration_ms: timeline && timeline.duration_ms > 0 ? timeline.duration_ms : undefined,
    });
  },

  toggleFollowPlayhead: () => set((state) => ({ followPlayhead: !state.followPlayhead })),

  duplicateProject: async (projectId) => {
    const id = projectId ?? get().activeProjectId;
    if (!id) return;
    try {
      const copy = await videoApi.duplicateProject(id);
      await get().loadProjects();
      await get().selectProject(copy.id);
      toast.success('Project duplicated');
    } catch (err) {
      toast.error(`Failed to duplicate project: ${(err as Error).message}`);
    }
  },

  createProjectFromVariant: async (variant) => {
    const sourceId = get().activeProjectId;
    if (!sourceId) return;
    try {
      const copy = await videoApi.duplicateProject(sourceId);
      // Clip IDs survive duplication, so the variant's plan applies cleanly
      // to the copied timeline.
      await videoApi.assistant.applyEditPlan(copy.id, variant.plan);
      await get().loadProjects();
      await get().selectProject(copy.id);
      toast.success(`Created "${variant.name}" variant project`);
    } catch (err) {
      toast.error(`Failed to create variant project: ${(err as Error).message}`);
    }
  },

  addMarker: async (timeMs, label) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const time = Math.max(0, Math.round(timeMs ?? get().playheadMs));
    next.markers = [...(next.markers || []), { id: newId('marker'), time_ms: time, label: label?.trim() || `Marker ${(next.markers?.length || 0) + 1}` }];
    next.markers.sort((a, b) => a.time_ms - b.time_ms);
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  removeMarker: async (markerId) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    next.markers = (next.markers || []).filter((marker) => marker.id !== markerId);
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  updateClipZIndex: async (clipId, zIndex) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    const loc = findClip(next, clipId);
    if (!loc) return;
    loc.clip.z_index = Math.round(zIndex);
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  bringClipForward: async (clipId) => {
    const id = clipId || get().selectedClipId;
    if (!id) return;
    const clip = get().timeline ? findClip(get().timeline as VideoTimelineDocument, id) : null;
    if (!clip) return;
    await get().updateClipZIndex(id, (clip.clip.z_index ?? 0) + 1);
  },

  sendClipBackward: async (clipId) => {
    const id = clipId || get().selectedClipId;
    if (!id) return;
    const clip = get().timeline ? findClip(get().timeline as VideoTimelineDocument, id) : null;
    if (!clip) return;
    await get().updateClipZIndex(id, (clip.clip.z_index ?? 0) - 1);
  },

  nudgeSelection: async (deltaMs) => {
    const current = get().timeline;
    const ids = get().selectedClipIds.length > 0
      ? get().selectedClipIds
      : get().selectedClipId
        ? [get().selectedClipId as string]
        : [];
    if (!current || ids.length === 0 || !Number.isFinite(deltaMs) || deltaMs === 0) return;
    const next = cloneTimeline(current);
    let moved = false;
    for (const id of ids) {
      const loc = findClip(next, id);
      if (!loc || loc.track.locked) continue;
      const start = Math.max(0, loc.clip.start_ms + Math.round(deltaMs));
      if (start !== loc.clip.start_ms) {
        loc.clip.start_ms = start;
        moved = true;
      }
    }
    if (!moved) return;
    for (const track of next.tracks) {
      track.clips.sort((a, b) => a.start_ms - b.start_ms);
    }
    set((state) => ({
      timeline: recomputeDuration(next),
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  setCanvas: async (patch) => {
    const current = get().timeline;
    if (!current) return;
    const next = cloneTimeline(current);
    next.canvas = {
      ...next.canvas,
      ...patch,
      width: Math.max(16, Math.round(patch.width ?? next.canvas.width)),
      height: Math.max(16, Math.round(patch.height ?? next.canvas.height)),
      fps: Math.max(1, Math.min(120, Math.round(patch.fps ?? next.canvas.fps))),
    };
    set((state) => ({
      timeline: next,
      ...withTimelineHistory(state, current),
    }));
    await get().saveTimeline(next);
  },

  undoTimeline: async () => {
    const { timeline, timelineUndoStack, timelineRedoStack, selectedClipId } = get();
    if (!timeline || timelineUndoStack.length === 0) return;
    const previous = cloneTimeline(timelineUndoStack[timelineUndoStack.length - 1]);
    const selected = selectedClipId ? findClip(previous, selectedClipId) : null;
    set((state) => ({
      timeline: previous,
      timelineUndoStack: timelineUndoStack.slice(0, -1),
      timelineRedoStack: [...timelineRedoStack, cloneTimeline(timeline)].slice(-TIMELINE_HISTORY_LIMIT),
      selectedClipId: selected ? selectedClipId : null,
      selectedClipIds: state.selectedClipIds.filter((id) => findClip(previous, id)),
      selectedTrackId: selected ? selected.track.id : null,
    }));
    await get().saveTimeline(previous);
  },

  redoTimeline: async () => {
    const { timeline, timelineUndoStack, timelineRedoStack, selectedClipId } = get();
    if (!timeline || timelineRedoStack.length === 0) return;
    const next = cloneTimeline(timelineRedoStack[timelineRedoStack.length - 1]);
    const selected = selectedClipId ? findClip(next, selectedClipId) : null;
    set((state) => ({
      timeline: next,
      timelineUndoStack: [...timelineUndoStack, cloneTimeline(timeline)].slice(-TIMELINE_HISTORY_LIMIT),
      timelineRedoStack: timelineRedoStack.slice(0, -1),
      selectedClipId: selected ? selectedClipId : null,
      selectedClipIds: state.selectedClipIds.filter((id) => findClip(next, id)),
      selectedTrackId: selected ? selected.track.id : null,
    }));
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
      const startedSeq = get()._saveSeq;
      const job = await videoApi.renderTimeline(projectId, {
        ...get().exportSettings,
        estimated_duration_ms: get().timeline?.duration_ms,
      });
      set((state) => ({
        renderJobs: upsertRenderJob(state.renderJobs, job),
        activeRenderJobId: job.id,
        _renderStartedSaveSeq: startedSeq,
        isRendering: false,
      }));
      toast.success('Render started');
      void get().pollRenderJob(job.id);
    } catch (err) {
      set({ isRendering: false, error: (err as Error).message });
      toast.error('Could not start render');
    }
  },

  retryRenderJob: async (jobId) => {
    const job = get().renderJobs.find((item) => item.id === jobId);
    if (!job) return;
    try {
      const settings = JSON.parse(job.settings_json) as VideoExportSettings;
      if (settings && typeof settings === 'object') {
        set({ exportSettings: { ...DEFAULT_EXPORT, ...settings } });
      }
    } catch {
      // Unparseable settings — render with the current panel settings instead.
    }
    await get().renderTimeline();
  },

  pollRenderJob: async (jobId) => {
    try {
      const job = await videoApi.getRenderJob(jobId);
      // The user may have switched projects while this poll was in flight —
      // don't inject another project's render job into the current state.
      if (get().activeProjectId !== job.project_id) return;
      set((state) => ({ renderJobs: upsertRenderJob(state.renderJobs, job), activeRenderJobId: job.id }));
      if (job.status === 'completed') {
        // The render reflects the timeline as of when it started.
        set((state) => ({ renderedSaveSeq: state._renderStartedSaveSeq }));
        if (get().activeProjectId === job.project_id) {
          const assets = await videoApi.listAssets(job.project_id);
          if (get().activeProjectId === job.project_id) {
            set({ assets, selectedAssetId: job.output_asset_id || get().selectedAssetId });
          }
        }
        toast.success('Render complete');
        return;
      }
      if (job.status === 'failed' || job.status === 'cancelled') {
        if (job.error) toast.error(job.error);
        return;
      }
      const timeout = window.setTimeout(() => { void get().pollRenderJob(jobId); }, 1000);
      set({ _renderPollTimeout: timeout });
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
        selected_clip_id: get().selectedClipId || undefined,
        playhead_ms: get().playheadMs || undefined,
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
        selected_clip_id: get().selectedClipId || undefined,
        playhead_ms: get().playheadMs || undefined,
      });
      set({ assistantPlan });
    } catch (err) {
      toast.error((err as Error).message);
    }
  },

  applyAssistantPlan: async (selectedIndices) => {
    const projectId = get().activeProjectId;
    const plan = get().assistantPlan;
    if (!projectId || !plan) return;
    // The backend annotates plans so operations[i] ↔ preview[i]; a selection
    // applies just those operations.
    const operations = selectedIndices
      ? plan.operations.filter((_, index) => selectedIndices.includes(index))
      : plan.operations;
    if (operations.length === 0) return;
    const previous = get().timeline ? cloneTimeline(get().timeline as VideoTimelineDocument) : null;
    try {
      const detail = await videoApi.assistant.applyEditPlan(projectId, { ...plan, operations });
      set((state) => ({
        timelineRecord: detail.timeline,
        timeline: detail.document,
        assistantPlan: null,
        ...(previous ? withTimelineHistory(state, previous) : { timelineRedoStack: [] }),
      }));
      if (plan.issues && plan.issues.length > 0) {
        toast.info(`Edit plan applied — ${plan.issues.length} invalid operation${plan.issues.length === 1 ? '' : 's'} skipped`);
      } else {
        toast.success('Edit plan applied');
      }
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
        selected_clip_id: get().selectedClipId || undefined,
        playhead_ms: get().playheadMs || undefined,
      });
      set({ socialVariants });
    } catch (err) {
      toast.error((err as Error).message);
    }
  },
}));
