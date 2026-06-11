import { useEffect, useMemo, useRef, useState } from 'react';
import { DragHandle, useResizablePanels } from '../ResizablePanels';
import type { ReactNode } from 'react';
import {
  ChevronDown,
  Clapperboard,
  Download,
  Film,
  GitBranch,
  Library,
  ListPlus,
  Loader2,
  MessageSquare,
  Plus,
  RefreshCw,
  Scissors,
  Sparkles,
  Square,
  WandSparkles,
} from 'lucide-react';
import { toast } from 'sonner';
import { api, videoApi, videoAssetUrl } from '../../api';
import { useConversationStore, useCrossoverStore, useMessageStore, useSettingsStore } from '../../stores';
import { useVideoStudioStore } from '../../stores/videoStudio';
import type {
  InputAsset,
  VideoAsset,
  VideoCapability,
  VideoGenerationDetail,
  VideoGenerationValidationIssue,
  VideoModel,
  VideoPromptForm,
  VideoProviderKey,
} from '../../types/video';

// ── Cinematic option lists ────────────────────────────────────────────────────
const STYLE_OPTIONS = [
  'cinematic',
  'documentary',
  'animation',
  'hyperrealistic',
  'vintage film',
  'sci-fi',
  'horror',
  'fantasy',
  'noir',
  'nature documentary',
  'vlog',
  'commercial',
  'abstract',
];

const CAMERA_MOTION_OPTIONS = [
  'static locked-off',
  'slow push-in',
  'slow pull-out',
  'dolly forward',
  'dolly backward',
  'dolly zoom (vertigo)',
  'pan left',
  'pan right',
  'tilt up',
  'tilt down',
  'orbit / arc',
  'tracking follow',
  'crane up',
  'crane down',
  'handheld shake',
  'whip pan',
  'dutch tilt',
  'drone aerial',
];

const SHOT_TYPE_OPTIONS = [
  'extreme close-up',
  'close-up',
  'medium close-up',
  'medium shot',
  'full shot',
  'wide shot',
  'extreme wide / establishing',
  'over-the-shoulder',
  'point of view (POV)',
  'high angle',
  'low angle',
  'bird\'s-eye overhead',
  'worm\'s-eye',
];

const COMPOSITION_OPTIONS = [
  'rule of thirds',
  'centered / symmetrical',
  'leading lines',
  'golden ratio',
  'silhouette',
  'depth layers (foreground + background)',
  'overhead flat lay',
  'dutch angle',
  'frame within a frame',
];

const LENS_OPTIONS = [
  'shallow depth of field',
  'deep focus',
  'bokeh',
  'wide angle (barrel distortion)',
  'telephoto compression',
  'macro / extreme close',
  'fisheye',
  'tilt-shift miniature',
  'anamorphic lens flare',
  'soft focus / dreamy',
];

const LIGHTING_OPTIONS = [
  'golden hour warm',
  'blue hour twilight',
  'overcast soft diffused',
  'harsh midday sunlight',
  'studio three-point',
  'neon-lit night',
  'candlelight / warm practical',
  'foggy / desaturated',
  'backlit silhouette',
  'hard-contrast noir',
  'underwater caustics',
  'volumetric god rays',
];

const SOUND_EFFECTS_OPTIONS = [
  'ambient city traffic',
  'forest ambience',
  'ocean waves',
  'footsteps on gravel',
  'footsteps on wood',
  'rain on glass',
  'thunder',
  'wind',
  'door creak',
  'crowd murmur',
  'machinery hum',
  'silence',
];

const AMBIENT_NOISE_OPTIONS = [
  'quiet room tone',
  'city traffic',
  'forest birds',
  'ocean waves',
  'rain',
  'crowd murmur',
  'wind',
  'cafe buzz',
  'night crickets',
];

const AUTO_VALUE = '__auto__';
const CUSTOM_VALUE = '__custom__';

type PromptVariantKind = 'cinematic' | 'social' | 'product' | 'explainer' | 'documentary';

const PROMPT_VARIANTS: Array<{ kind: PromptVariantKind; label: string; aspectRatio?: string }> = [
  { kind: 'cinematic', label: 'Cinematic' },
  { kind: 'social', label: 'Social', aspectRatio: '9:16' },
  { kind: 'product', label: 'Product' },
  { kind: 'explainer', label: 'Explainer' },
  { kind: 'documentary', label: 'Documentary' },
];

function buildClientGenerationValidation(
  provider: VideoProviderKey,
  model: VideoModel | undefined,
  form: VideoPromptForm,
): {
  errors: VideoGenerationValidationIssue[];
  warnings: VideoGenerationValidationIssue[];
  normalizations: VideoGenerationValidationIssue[];
} {
  const errors: VideoGenerationValidationIssue[] = [];
  const warnings: VideoGenerationValidationIssue[] = [];
  const normalizations: VideoGenerationValidationIssue[] = [];
  if (!model) return { errors, warnings, normalizations };

  const caps = model.capabilities || [];
  const hasCap = (capability: VideoCapability) => caps.includes(capability);
  const addError = (field: string, code: string, message: string) => {
    errors.push({ field, code, message, severity: 'error' });
  };
  const addWarning = (field: string, code: string, message: string) => {
    warnings.push({ field, code, message, severity: 'warning' });
  };
  const addNormalization = (field: string, code: string, message: string, original: unknown, normalized: unknown) => {
    normalizations.push({ field, code, message, severity: 'normalization', original, normalized });
  };

  const referenceIds = (form.reference_asset_ids ?? []).filter(Boolean);
  const hasStartImage = Boolean(form.start_image_asset_id);
  const hasLastFrame = Boolean(form.last_frame_asset_id);
  const hasSourceVideo = Boolean(form.source_video_asset_id);
  const hasReferenceImages = referenceIds.length > 0;

  if (hasLastFrame && !hasStartImage) {
    addError('last_frame_asset_id', 'last_frame_requires_start_frame', 'Choose a start frame before choosing a last frame.');
  }
  if (hasSourceVideo && (hasStartImage || hasLastFrame || hasReferenceImages)) {
    addError('source_video_asset_id', 'source_video_exclusive', 'Source-video extension cannot be combined with start frame, last frame, or reference images.');
  }
  if (hasStartImage && !hasCap('image_to_video')) {
    addError('start_image_asset_id', 'image_to_video_unsupported', `${model.name} does not support start-frame image generation.`);
  }
  if (hasLastFrame && !hasCap('first_last_frame')) {
    addError('last_frame_asset_id', 'first_last_frame_unsupported', `${model.name} does not support first/last-frame interpolation.`);
  }
  if (hasSourceVideo && !hasCap('extend_video')) {
    addError('source_video_asset_id', 'source_video_unsupported', `${model.name} does not support source-video extension.`);
  }
  if (hasReferenceImages && !hasCap('reference_images')) {
    addError('reference_asset_ids', 'reference_images_unsupported', `${model.name} does not support reference images.`);
  }
  if (referenceIds.length > 3) {
    addError('reference_asset_ids', 'too_many_reference_images', 'Use no more than 3 reference images for this provider.');
  }
  if (form.negative_prompt?.trim() && !hasCap('negative_prompt')) {
    addError('negative_prompt', 'negative_prompt_unsupported', `${model.name} does not support negative prompts.`);
  }
  if (form.seed !== undefined && !hasCap('seed')) {
    addError('seed', 'seed_unsupported', `${model.name} does not expose deterministic seed control.`);
  }
  if (form.person_generation && !hasCap('person_generation')) {
    addError('person_generation', 'person_generation_unsupported', `${model.name} does not support person-generation policy controls.`);
  }

  const hasAudioCues = Boolean(form.dialogue?.trim() || form.sound_effects?.trim() || form.ambient_noise?.trim());
  if (hasAudioCues && provider === 'gemini') {
    addWarning('dialogue', 'gemini_audio_prompt_only', 'Gemini Veo dialogue and sound cues are added to the prompt; there is no separate native audio toggle.');
  } else if (hasAudioCues && !hasCap('audio_generation')) {
    addError('dialogue', 'audio_controls_unsupported', `${model.name} does not support audio or dialogue generation controls.`);
  }

  const aspectRatio = form.aspect_ratio || '16:9';
  if (model.aspect_ratios?.length && !model.aspect_ratios.some((value) => value.toLowerCase() === aspectRatio.toLowerCase())) {
    addError('aspect_ratio', 'aspect_ratio_unsupported', `${model.name} does not support aspect ratio ${aspectRatio}.`);
  }

  let resolution = form.resolution || (model.resolutions?.includes('720p') ? '720p' : model.resolutions?.[0] || '720p');
  if (hasSourceVideo && provider === 'gemini' && resolution.toLowerCase() !== '720p') {
    addNormalization('resolution', 'source_video_resolution_normalized', 'Gemini Veo source-video extension exports at 720p.', resolution, '720p');
    resolution = '720p';
  }
  if (model.resolutions?.length && !model.resolutions.some((value) => value.toLowerCase() === resolution.toLowerCase())) {
    addError('resolution', 'resolution_unsupported', `${model.name} does not support ${resolution} output.`);
  }

  let duration = form.duration_seconds || 8;
  if (model.duration_min_seconds && duration < model.duration_min_seconds) {
    addNormalization('duration_seconds', 'duration_min_normalized', `Duration will be raised to ${model.duration_min_seconds} seconds.`, duration, model.duration_min_seconds);
    duration = model.duration_min_seconds;
  }
  if (model.duration_max_seconds && duration > model.duration_max_seconds) {
    addNormalization('duration_seconds', 'duration_max_normalized', `Duration will be capped at ${model.duration_max_seconds} seconds.`, duration, model.duration_max_seconds);
    duration = model.duration_max_seconds;
  }
  if (provider === 'gemini') {
    const needsEightSeconds = hasSourceVideo || hasLastFrame || hasReferenceImages || resolution.toLowerCase() === '1080p' || resolution.toLowerCase() === '4k';
    if (needsEightSeconds && duration !== 8) {
      addNormalization('duration_seconds', 'gemini_duration_normalized', 'Gemini Veo will use 8 seconds for this mode and resolution.', duration, 8);
    }
  }

  if (model.fps_options?.length && form.fps && !model.fps_options.includes(form.fps)) {
    addNormalization('fps', 'fps_normalized', `${model.name} exports at ${model.fps_options[0]} fps.`, form.fps, model.fps_options[0]);
  }

  return { errors, warnings, normalizations };
}

function parseGenerationJSON<T>(value?: string): T | null {
  if (!value) return null;
  try {
    const parsed = JSON.parse(value) as T;
    return parsed ?? null;
  } catch {
    return null;
  }
}

function getGenerationSettings(generation: VideoGenerationDetail): Partial<VideoPromptForm> {
  const parsed = parseGenerationJSON<Partial<VideoPromptForm>>(generation.settings_json);
  return parsed && typeof parsed === 'object' ? parsed : {};
}

function getGenerationInputAssets(generation: VideoGenerationDetail): InputAsset[] {
  const parsed = parseGenerationJSON<InputAsset[]>(generation.input_assets_json);
  if (Array.isArray(parsed)) return parsed.filter((asset) => Boolean(asset?.asset_id && asset.role));
  const ids = parseGenerationJSON<string[]>(generation.input_asset_ids_json);
  if (Array.isArray(ids)) {
    return ids.filter(Boolean).map((asset_id) => ({ asset_id, role: 'start_frame' as const }));
  }
  return [];
}

function getGenerationInputMode(inputAssets: InputAsset[]): string {
  const roles = new Set(inputAssets.map((asset) => asset.role));
  if (roles.has('source_video')) return 'Extend';
  if (roles.has('start_frame') && roles.has('last_frame')) return 'First/last';
  if (roles.has('start_frame')) return 'Image-to-video';
  if (roles.has('reference_image')) return 'References';
  return 'Text-to-video';
}

function formatSettingSummary(settings: Partial<VideoPromptForm>): string {
  const parts = [
    settings.aspect_ratio,
    settings.duration_seconds ? `${settings.duration_seconds}s` : '',
    settings.resolution,
    settings.fps ? `${settings.fps} fps` : '',
    settings.seed !== undefined ? `seed ${settings.seed}` : '',
  ].filter(Boolean);
  return parts.length > 0 ? parts.join(' · ') : 'Default settings';
}

function formatCost(cost?: number): string | null {
  if (typeof cost !== 'number') return null;
  return `$${cost.toFixed(cost > 0 && cost < 1 ? 4 : 2)}`;
}

function compactID(value?: string): string | null {
  if (!value) return null;
  return value.length > 18 ? `${value.slice(0, 8)}...${value.slice(-6)}` : value;
}

function formatInputAssetSummary(inputAssets: InputAsset[]): string | null {
  if (inputAssets.length === 0) return null;
  const labels: Record<InputAsset['role'], string> = {
    start_frame: 'start',
    last_frame: 'last',
    reference_image: 'ref',
    source_video: 'source',
  };
  const counts = inputAssets.reduce<Record<string, number>>((acc, asset) => {
    acc[asset.role] = (acc[asset.role] || 0) + 1;
    return acc;
  }, {});
  return Object.entries(counts).map(([role, count]) => `${labels[role as InputAsset['role']] || role}${count > 1 ? ` x${count}` : ''}`).join(' · ');
}

function formatUsageSummary(usageJson?: string): string | null {
  const usage = parseGenerationJSON<Record<string, unknown>>(usageJson);
  if (!usage || typeof usage !== 'object') return null;
  const candidates = ['total_tokens', 'input_tokens', 'output_tokens', 'prompt_tokens', 'completion_tokens'];
  const parts = candidates
    .filter((key) => typeof usage[key] === 'number' || typeof usage[key] === 'string')
    .slice(0, 3)
    .map((key) => `${key.replace(/_/g, ' ')} ${String(usage[key])}`);
  return parts.length > 0 ? parts.join(' · ') : 'usage captured';
}

function buildPromptVariant(prompt: string, kind: PromptVariantKind): string {
  const seed = prompt.trim();
  const suffixes: Record<PromptVariantKind, string> = {
    cinematic: 'Reframe it as a polished cinematic sequence with deliberate camera movement, layered lighting, atmospheric detail, and a clear visual beginning, middle, and end.',
    social: 'Reframe it as a vertical short-form hook with immediate motion in the first second, bold readable subject framing, and a loopable ending.',
    product: 'Reframe it as a premium product spot with the object clearly visible, controlled reflections, tactile detail, and a confident final hero shot.',
    explainer: 'Reframe it as a clean explainer sequence with simple visual beats, clear cause-and-effect motion, and room for concise on-screen labels.',
    documentary: 'Reframe it as a natural documentary moment with observational camera language, grounded lighting, realistic pacing, and environmental context.',
  };
  return `${seed}\n\n${suffixes[kind]}`;
}

export function VideoStudio() {
  const projects = useVideoStudioStore((state) => state.projects);
  const activeProjectId = useVideoStudioStore((state) => state.activeProjectId);
  const activeGenerationId = useVideoStudioStore((state) => state.activeGenerationId);
  const selectedAssetId = useVideoStudioStore((state) => state.selectedAssetId);
  const generations = useVideoStudioStore((state) => state.generations);
  const assets = useVideoStudioStore((state) => state.assets);
  const providers = useVideoStudioStore((state) => state.providers);
  const selectedProvider = useVideoStudioStore((state) => state.selectedProvider);
  const selectedModel = useVideoStudioStore((state) => state.selectedModel);
  const modelsByProvider = useVideoStudioStore((state) => state.modelsByProvider);
  const promptForm = useVideoStudioStore((state) => state.promptForm);
  const isLoading = useVideoStudioStore((state) => state.isLoading);
  const isGenerating = useVideoStudioStore((state) => state.isGenerating);
  const isEnhancing = useVideoStudioStore((state) => state.isEnhancing);
  const generationProgress = useVideoStudioStore((state) => state.generationProgress);
  const generationValidation = useVideoStudioStore((state) => state.generationValidation);
  const error = useVideoStudioStore((state) => state.error);
  const loadProviders = useVideoStudioStore((state) => state.loadProviders);
  const loadProjects = useVideoStudioStore((state) => state.loadProjects);
  const createProject = useVideoStudioStore((state) => state.createProject);
  const selectProject = useVideoStudioStore((state) => state.selectProject);
  const setProvider = useVideoStudioStore((state) => state.setProvider);
  const setModel = useVideoStudioStore((state) => state.setModel);
  const setPromptField = useVideoStudioStore((state) => state.setPromptField);
  const clearPrompt = useVideoStudioStore((state) => state.clearPrompt);
  const enhancePrompt = useVideoStudioStore((state) => state.enhancePrompt);
  const generate = useVideoStudioStore((state) => state.generate);
  const branchFromGeneration = useVideoStudioStore((state) => state.branchFromGeneration);
  const regenerateFromGeneration = useVideoStudioStore((state) => state.regenerateFromGeneration);
  const cancelGeneration = useVideoStudioStore((state) => state.cancelGeneration);
  const selectAsset = useVideoStudioStore((state) => state.selectAsset);
  const setAppMode = useSettingsStore((state) => state.setAppMode);
  const createConversation = useConversationStore((state) => state.createConversation);
  const selectConversation = useConversationStore((state) => state.selectConversation);
  const clearMessages = useMessageStore((state) => state.clearMessages);
  const fetchMessages = useMessageStore((state) => state.fetchMessages);
  const { crossoverContext, clearCrossoverContext } = useCrossoverStore();

  // Pending image attachment to import once a project is ready
  const pendingAttachmentRef = useRef<string | null>(null);

  useEffect(() => {
    loadProviders();
    loadProjects();
  }, [loadProviders, loadProjects]);

  // Consume crossover context from Image Studio → Video Studio handoff
  useEffect(() => {
    if (!crossoverContext || crossoverContext.type !== 'to-video') return;
    const { prompt, attachmentId } = crossoverContext.data;
    clearCrossoverContext();
    setPromptField('prompt', prompt);
    if (attachmentId) {
      pendingAttachmentRef.current = attachmentId;
    }
  }, [crossoverContext, clearCrossoverContext, setPromptField]);

  // Once we have an active project, import any pending attachment as start frame
  useEffect(() => {
    const attachmentId = pendingAttachmentRef.current;
    if (!attachmentId || !activeProjectId) return;
    pendingAttachmentRef.current = null;
    videoApi.importExternalAsset(activeProjectId, {
      source_studio: 'attachment',
      source_id: attachmentId,
      kind: 'image',
    }).then((asset) => {
      setPromptField('start_image_asset_id', asset.id);
      void videoApi.getProject(activeProjectId).then((detail) => {
        useVideoStudioStore.setState({ assets: detail.assets ?? [] });
      });
      toast.success('Image loaded as start frame');
    }).catch((err: Error) => {
      toast.error(`Failed to import image: ${err.message}`);
    });
  }, [activeProjectId, setPromptField]);

  const activeProject = useMemo(
    () => projects.find((project) => project.id === activeProjectId) || null,
    [projects, activeProjectId],
  );
  const activeGeneration = useMemo(
    () => generations.find((generation) => generation.id === activeGenerationId) || generations[generations.length - 1] || null,
    [generations, activeGenerationId],
  );
  const activeAsset = useMemo(
    () => assets.find((asset) => asset.id === selectedAssetId) || assets.find((asset) => asset.id === activeGeneration?.output_asset_id) || null,
    [assets, activeGeneration, selectedAssetId],
  );
  const models = modelsByProvider[selectedProvider] || [];
  const selectedModelInfo = models.find((model) => model.id === selectedModel);
  const selectedProviderInfo = providers.find((provider) => provider.key === selectedProvider);
  const selectedProviderConfigured = selectedProviderInfo?.configured ?? false;
  const selectedModelCapabilities = selectedModelInfo?.capabilities || [];
  const progressPercent = Math.round((generationProgress?.progress || 0) * 100);
  const clientValidation = useMemo(
    () => buildClientGenerationValidation(selectedProvider, selectedModelInfo, promptForm),
    [selectedProvider, selectedModelInfo, promptForm],
  );
  const visibleValidation = generationValidation && generationValidation.provider === selectedProvider && generationValidation.model === selectedModel
    ? generationValidation
    : clientValidation;
  const validationErrors = visibleValidation.errors ?? [];
  const validationWarnings = visibleValidation.warnings ?? [];
  const validationNormalizations = visibleValidation.normalizations ?? [];

  const startGeneration = () => {
    setPromptField('place_on_timeline', false);
    generate();
  };

  const handleSendGenerationToTimeline = async (generation: VideoGenerationDetail) => {
    if (!generation.output_asset_id) {
      toast.error('No generated video asset is available');
      return;
    }
    try {
      const detail = await videoApi.sendGenerationToTimeline(generation.id);
      useVideoStudioStore.setState({
        timelineRecord: detail.timeline,
        timeline: detail.document,
        selectedAssetId: detail.asset_id || generation.output_asset_id,
      });
      setAppMode('video-edit');
      toast.success('Video sent to timeline');
    } catch (err) {
      toast.error(`Failed to send to timeline: ${(err as Error).message}`);
    }
  };

  const handleRegisterAssetInLibrary = async (generation: VideoGenerationDetail) => {
    if (!generation.output_asset_id) {
      toast.error('No generated video asset is available');
      return;
    }
    try {
      const file = await videoApi.registerAssetInLibrary(generation.output_asset_id);
      toast.success(`Registered ${file.display_name || 'video'} in File Library`);
    } catch (err) {
      toast.error(`Failed to register in File Library: ${(err as Error).message}`);
    }
  };

  const handleSendGenerationToChat = async (generation: VideoGenerationDetail) => {
    if (!generation.output_asset_id) {
      toast.error('No generated video asset is available');
      return;
    }
    try {
      const asset = assets.find((item) => item.id === generation.output_asset_id);
      const title = asset?.file_name || generation.model || 'Video output';
      const convo = await createConversation(`Video: ${title}`);
      const attachment = await videoApi.attachAssetToConversation(generation.output_asset_id, convo.id);
      const content = [
        generation.prompt && `Video prompt: ${generation.prompt}`,
        generation.enhanced_prompt && `Enhanced prompt:\n${generation.enhanced_prompt}`,
        `Video output\n/v1/attachments/${attachment.id}/download`,
      ].filter(Boolean).join('\n\n');
      await api.sendMessage(convo.id, { content, no_reply: true });
      selectConversation(convo.id);
      clearMessages();
      await fetchMessages(convo.id);
      setAppMode('chat');
      toast.success('Video sent to chat');
    } catch (err) {
      toast.error(`Failed to send to chat: ${(err as Error).message}`);
    }
  };

  const handleUseEnhancedPrompt = (generation: VideoGenerationDetail) => {
    if (!generation.enhanced_prompt?.trim()) return;
    const settings = getGenerationSettings(generation);
    setPromptField('prompt', generation.enhanced_prompt.trim());
    setPromptField('enhance', false);
    if (settings.aspect_ratio) setPromptField('aspect_ratio', settings.aspect_ratio);
    if (settings.duration_seconds) setPromptField('duration_seconds', settings.duration_seconds);
    if (settings.resolution) setPromptField('resolution', settings.resolution);
    if (settings.fps) setPromptField('fps', settings.fps);
    toast.success('Enhanced prompt loaded');
  };

  const handleUsePromptVariant = (generation: VideoGenerationDetail, kind: PromptVariantKind) => {
    const basePrompt = (generation.enhanced_prompt || generation.prompt).trim();
    if (!basePrompt) return;
    const settings = getGenerationSettings(generation);
    const variant = PROMPT_VARIANTS.find((item) => item.kind === kind);
    setPromptField('prompt', buildPromptVariant(basePrompt, kind));
    setPromptField('enhance', false);
    if (settings.duration_seconds) setPromptField('duration_seconds', settings.duration_seconds);
    if (settings.resolution) setPromptField('resolution', settings.resolution);
    if (settings.fps) setPromptField('fps', settings.fps);
    setPromptField('aspect_ratio', variant?.aspectRatio || settings.aspect_ratio || promptForm.aspect_ratio);
    toast.success(`${variant?.label || 'Prompt'} variant loaded`);
  };

  const { leftStyle, rightStyle, startLeft, startRight } = useResizablePanels({ defaultLeft: 360, defaultRight: 320 });

  return (
    <div className="flex min-h-0 flex-1 flex-col bg-surface">
      <div className="flex flex-col gap-2 border-b border-border bg-surface-raised px-3 py-2 sm:flex-row sm:items-center sm:justify-between sm:px-4">
        <div className="flex min-w-0 items-center gap-3">
          <Film size={18} className="text-primary" />
          <span className="shrink-0 text-sm font-medium text-text">Video Studio</span>
          {activeProject && (
            <span className="min-w-0 truncate text-xs text-text-muted">- {activeProject.title}</span>
          )}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <button
            onClick={() => { void createProject(); }}
            className="min-h-9 rounded-lg border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:bg-surface-hover hover:text-text transition-colors inline-flex items-center gap-1.5"
          >
            <Plus size={13} />
            Project
          </button>
          <button
            onClick={() => setAppMode('video-edit')}
            className="min-h-9 rounded-lg border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:bg-surface-hover hover:text-text transition-colors inline-flex items-center gap-1.5"
          >
            <Scissors size={13} />
            Edit Studio
          </button>
        </div>
      </div>

      <div className="flex min-h-0 flex-1 flex-col xl:flex-row">
        <aside className="min-h-0 overflow-y-auto border-b border-border bg-surface xl:border-b-0 xl:border-r" style={leftStyle}>
          <div className="space-y-3 p-3">
            <ProjectStrip
              projects={projects}
              activeProjectId={activeProjectId}
              isLoading={isLoading}
              onNew={() => { void createProject(); }}
              onSelect={(id) => { void selectProject(id); }}
            />

            <section className="rounded-lg border border-border bg-surface-alt p-3">
              <div className="mb-3 flex items-center justify-between gap-2">
                <div className="flex items-center gap-2">
                  <Sparkles size={15} className="text-primary" />
                  <h2 className="text-sm font-semibold text-text">Create Video</h2>
                </div>
                {selectedModelInfo?.duration_max_seconds ? (
                  <span className="rounded-md border border-border bg-surface px-2 py-1 text-[10px] uppercase tracking-wide text-text-muted">
                    {selectedModelInfo.duration_min_seconds || 1}-{selectedModelInfo.duration_max_seconds}s
                  </span>
                ) : null}
              </div>

              <div className="space-y-3">
                <ControlLabel label="Provider">
                  <select
                    value={selectedProvider}
                    onChange={(event) => { void setProvider(event.target.value as VideoProviderKey); }}
                    className="min-h-10 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text focus:border-primary/50 focus:outline-none"
                  >
                    {providers.map((provider) => (
                      <option key={provider.key} value={provider.key}>
                        {provider.display_name}{provider.configured ? '' : ' (not configured)'}
                      </option>
                    ))}
                  </select>
                </ControlLabel>

                {!selectedProviderConfigured && (
                  <p className="rounded-lg border border-amber-500/20 bg-amber-500/5 px-3 py-2 text-xs text-amber-300/80">
                    Configure a video provider (OpenRouter, Gemini, or Luma) in Settings before generating.
                  </p>
                )}

                <ControlLabel label="Model">
                  <div className="flex gap-2">
                    <select
                      value={selectedModel || ''}
                      onChange={(event) => setModel(event.target.value)}
                      className="min-h-10 min-w-0 flex-1 rounded-lg border border-border bg-surface px-3 text-sm text-text focus:border-primary/50 focus:outline-none"
                    >
                      {models.map((model) => (
                        <option key={model.id} value={model.id}>
                          {model.name}
                        </option>
                      ))}
                    </select>
                    <button
                      onClick={() => { void useVideoStudioStore.getState().loadModels(selectedProvider, true); }}
                      disabled={!selectedProvider}
                      className="min-h-10 min-w-10 rounded-lg border border-border bg-surface text-text-muted hover:bg-surface-hover hover:text-text inline-flex items-center justify-center"
                      aria-label="Refresh video models"
                      title="Refresh video models"
                    >
                      <RefreshCw size={14} />
                    </button>
                  </div>
                </ControlLabel>

                {/* ── Prompt ── */}
                <CollapsibleSection label="Prompt" defaultOpen>
                  <ControlLabel label="Prompt">
                    <textarea
                      value={promptForm.prompt}
                      onChange={(event) => setPromptField('prompt', event.target.value)}
                      rows={8}
                      maxLength={selectedModelInfo?.max_prompt_chars || undefined}
                      placeholder="Describe a single video: subject, scene, action, camera, lighting, and style."
                      className="w-full resize-y rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-muted/50 focus:border-primary/50 focus:outline-none"
                    />
                  </ControlLabel>
                  {selectedModelCapabilities.includes('negative_prompt') && (
                    <ControlLabel label="Negative prompt">
                      <input
                        value={promptForm.negative_prompt}
                        onChange={(event) => setPromptField('negative_prompt', event.target.value)}
                        className="min-h-10 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text focus:border-primary/50 focus:outline-none"
                        placeholder="Artifacts to avoid"
                      />
                    </ControlLabel>
                  )}
                </CollapsibleSection>

                {/* ── Start / Last Frame ── */}
                {(selectedModelCapabilities.includes('image_to_video') || selectedModelCapabilities.includes('first_last_frame') || selectedModelCapabilities.includes('extend_video')) && (
                  <CollapsibleSection label="Start / Last Frame" defaultOpen>
                    {(selectedModelCapabilities.includes('image_to_video') || selectedModelCapabilities.includes('first_last_frame')) && (
                      <ControlLabel label="Start frame image">
                        <AssetPicker
                          value={promptForm.start_image_asset_id}
                          onChange={(id) => setPromptField('start_image_asset_id', id)}
                          assets={assets}
                          projectId={activeProjectId}
                          onAssetsChange={(updated) => useVideoStudioStore.setState({ assets: updated })}
                          accept="image"
                          placeholder="None — select an image asset"
                        />
                        <p className="mt-1 text-[10px] text-text-muted">Image asset from this project to use as the first frame.</p>
                      </ControlLabel>
                    )}
                    {selectedModelCapabilities.includes('first_last_frame') && (
                      <ControlLabel label="Last frame image">
                        <AssetPicker
                          value={promptForm.last_frame_asset_id}
                          onChange={(id) => setPromptField('last_frame_asset_id', id)}
                          assets={assets}
                          projectId={activeProjectId}
                          onAssetsChange={(updated) => useVideoStudioStore.setState({ assets: updated })}
                          accept="image"
                          placeholder="None — requires a start frame"
                        />
                        <p className="mt-1 text-[10px] text-text-muted">Veo 3.1 interpolates between start and last frame. Requires a start frame.</p>
                      </ControlLabel>
                    )}
                    {selectedModelCapabilities.includes('extend_video') && (
                      <ControlLabel label="Source video to extend">
                        <AssetPicker
                          value={promptForm.source_video_asset_id}
                          onChange={(id) => setPromptField('source_video_asset_id', id)}
                          assets={assets}
                          projectId={activeProjectId}
                          onAssetsChange={(updated) => useVideoStudioStore.setState({ assets: updated })}
                          accept="video"
                          placeholder="None — select a video asset to extend"
                        />
                        <p className="mt-1 text-[10px] text-text-muted">Continues from the final frame. Forced to 720p, 8s.</p>
                      </ControlLabel>
                    )}
                  </CollapsibleSection>
                )}

                {/* ── Reference Images ── */}
                {selectedModelCapabilities.includes('reference_images') && (
                  <CollapsibleSection label={`Reference Images (${(promptForm.reference_asset_ids ?? []).filter(Boolean).length}/3)`}>
                    <ControlLabel label="">
                      <div className="space-y-1.5">
                        {[0, 1, 2].map((idx) => (
                          <AssetPicker
                            key={idx}
                            value={(promptForm.reference_asset_ids ?? [])[idx]}
                            onChange={(id) => {
                              const current = [...(promptForm.reference_asset_ids ?? [])];
                              if (id) {
                                current[idx] = id;
                              } else {
                                current.splice(idx, 1);
                              }
                              const filtered = current.filter(Boolean);
                              setPromptField('reference_asset_ids', filtered.length > 0 ? filtered : undefined);
                            }}
                            assets={assets}
                            projectId={activeProjectId}
                            onAssetsChange={(updated) => useVideoStudioStore.setState({ assets: updated })}
                            accept="image"
                            placeholder={`Reference image ${idx + 1}`}
                          />
                        ))}
                      </div>
                      <p className="mt-1 text-[10px] text-text-muted">Up to 3 images. Requires 8s duration.</p>
                    </ControlLabel>
                  </CollapsibleSection>
                )}

                {/* ── Output Format ── */}
                <CollapsibleSection label="Output Format" defaultOpen>
                  <div className="grid grid-cols-2 gap-2">
                    <ControlLabel label="Aspect">
                      <select
                        value={promptForm.aspect_ratio}
                        onChange={(event) => setPromptField('aspect_ratio', event.target.value)}
                        className="min-h-10 w-full rounded-lg border border-border bg-surface px-2 text-sm text-text"
                      >
                        {(selectedModelInfo?.aspect_ratios || ['16:9', '9:16', '1:1']).map((value) => (
                          <option key={value} value={value}>{value}</option>
                        ))}
                      </select>
                    </ControlLabel>
                    <ControlLabel label="Duration">
                      <input
                        type="number"
                        min={selectedModelInfo?.duration_min_seconds || 1}
                        max={selectedModelInfo?.duration_max_seconds || 30}
                        value={promptForm.duration_seconds}
                        onChange={(event) => setPromptField('duration_seconds', Math.max(1, Number(event.target.value) || 1))}
                        className="min-h-10 w-full rounded-lg border border-border bg-surface px-2 text-sm text-text"
                      />
                    </ControlLabel>
                    <ControlLabel label="Resolution">
                      <select
                        value={promptForm.resolution}
                        onChange={(event) => setPromptField('resolution', event.target.value)}
                        className="min-h-10 w-full rounded-lg border border-border bg-surface px-2 text-sm text-text"
                      >
                        {(selectedModelInfo?.resolutions || ['720p', '1080p']).map((value) => (
                          <option key={value} value={value}>{value}</option>
                        ))}
                      </select>
                    </ControlLabel>
                    <ControlLabel label="FPS">
                      <select
                        value={promptForm.fps}
                        onChange={(event) => setPromptField('fps', Number(event.target.value))}
                        className="min-h-10 w-full rounded-lg border border-border bg-surface px-2 text-sm text-text"
                      >
                        {(selectedModelInfo?.fps_options || [24, 30]).map((value) => (
                          <option key={value} value={value}>{value}</option>
                        ))}
                      </select>
                    </ControlLabel>
                  </div>
                </CollapsibleSection>

                {/* ── Advanced ── */}
                {(selectedModelCapabilities.includes('person_generation') || selectedModelCapabilities.includes('seed')) && (
                  <CollapsibleSection label="Advanced">
                    <div className="space-y-2">
                      {selectedModelCapabilities.includes('person_generation') && (
                        <ControlLabel label="Person generation">
                          <select
                            value={promptForm.person_generation || 'allow'}
                            onChange={(event) => setPromptField('person_generation', event.target.value as 'allow' | 'dont_allow')}
                            className="min-h-10 w-full rounded-lg border border-border bg-surface px-2 text-sm text-text"
                          >
                            <option value="allow">Allow</option>
                            <option value="dont_allow">Don&apos;t allow</option>
                          </select>
                        </ControlLabel>
                      )}
                      {selectedModelCapabilities.includes('seed') && (
                        <ControlLabel label="Seed">
                          <input
                            type="number"
                            value={promptForm.seed ?? ''}
                            onChange={(event) => {
                              const value = event.target.value.trim();
                              setPromptField('seed', value === '' ? undefined : Number(value));
                            }}
                            placeholder="Optional deterministic seed"
                            className="min-h-10 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text focus:border-primary/50 focus:outline-none"
                          />
                        </ControlLabel>
                      )}
                    </div>
                  </CollapsibleSection>
                )}

                <CollapsibleSection label="Cinematic Controls">
                  <div className="space-y-1">
                    <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-1 2xl:grid-cols-2">
                      <SelectWithCustom
                        label="Style"
                        value={promptForm.style_preset}
                        choices={STYLE_OPTIONS}
                        customPlaceholder="e.g. impressionist, retro anime"
                        onChange={(value) => setPromptField('style_preset', value)}
                      />
                      <SelectWithCustom
                        label="Camera motion"
                        value={promptForm.camera_motion}
                        choices={CAMERA_MOTION_OPTIONS}
                        customPlaceholder="e.g. rotating crane"
                        onChange={(value) => setPromptField('camera_motion', value)}
                      />
                      <SelectWithCustom
                        label="Shot type"
                        value={promptForm.shot_type}
                        choices={SHOT_TYPE_OPTIONS}
                        customPlaceholder="e.g. two-shot"
                        onChange={(value) => setPromptField('shot_type', value)}
                      />
                      <SelectWithCustom
                        label="Composition"
                        value={promptForm.composition ?? ''}
                        choices={COMPOSITION_OPTIONS}
                        customPlaceholder="e.g. dynamic diagonal"
                        onChange={(value) => setPromptField('composition', value)}
                      />
                      <SelectWithCustom
                        label="Lens / focus"
                        value={promptForm.lens_effect ?? ''}
                        choices={LENS_OPTIONS}
                        customPlaceholder="e.g. anamorphic streak"
                        onChange={(value) => setPromptField('lens_effect', value)}
                      />
                      <SelectWithCustom
                        label="Lighting / ambiance"
                        value={promptForm.lighting ?? ''}
                        choices={LIGHTING_OPTIONS}
                        customPlaceholder="e.g. hard rim backlight"
                        onChange={(value) => setPromptField('lighting', value)}
                      />
                    </div>

                    {selectedModelCapabilities.includes('audio_generation') && (
                      <div className="space-y-2 border-t border-border pt-2">
                        <p className="text-[10px] uppercase tracking-wide text-text-muted">Audio cues</p>
                        <ControlLabel label='Dialogue (in quotes)'>
                          <input
                            value={promptForm.dialogue ?? ''}
                            onChange={(event) => setPromptField('dialogue', event.target.value)}
                            placeholder='"Hello," she whispered.'
                            className="min-h-10 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text focus:border-primary/50 focus:outline-none"
                          />
                        </ControlLabel>
                        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-1 2xl:grid-cols-2">
                          <SelectWithCustom
                            label="Sound effects"
                            value={promptForm.sound_effects ?? ''}
                            choices={SOUND_EFFECTS_OPTIONS}
                            customPlaceholder="e.g. thunder crack"
                            onChange={(value) => setPromptField('sound_effects', value)}
                          />
                          <SelectWithCustom
                            label="Ambient noise"
                            value={promptForm.ambient_noise ?? ''}
                            choices={AMBIENT_NOISE_OPTIONS}
                            customPlaceholder="e.g. distant traffic hum"
                            onChange={(value) => setPromptField('ambient_noise', value)}
                          />
                        </div>
                      </div>
                    )}

                    {(selectedModelCapabilities.includes('image_to_video') || selectedModelCapabilities.includes('first_last_frame') || selectedModelCapabilities.includes('extend_video')) && (
                      <div className="border-t border-border pt-2">
                        <ControlLabel label="Continuity notes">
                          <textarea
                            value={promptForm.continuity_notes ?? ''}
                            onChange={(event) => setPromptField('continuity_notes', event.target.value)}
                            rows={2}
                            className="w-full resize-y rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text focus:border-primary/50 focus:outline-none"
                            placeholder="Maintain character outfit, match exit direction, seamless loop…"
                          />
                        </ControlLabel>
                      </div>
                    )}

                    <div className="border-t border-border pt-2">
                      <ControlLabel label="Production notes">
                        <textarea
                          value={promptForm.production_notes}
                          onChange={(event) => setPromptField('production_notes', event.target.value)}
                          rows={2}
                          className="w-full resize-y rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text focus:border-primary/50 focus:outline-none"
                          placeholder="Any additional directives"
                        />
                      </ControlLabel>
                    </div>
                  </div>
                </CollapsibleSection>

                {generationProgress && (
                  <div className="rounded-lg border border-primary/20 bg-primary/5 p-2">
                    <div className="mb-1 flex items-center justify-between gap-2 text-[11px] text-text-muted">
                      <span className="truncate">{generationProgress.message}</span>
                      <span>{progressPercent > 0 ? `${progressPercent}%` : generationProgress.stage}</span>
                    </div>
                    <div className="h-1.5 overflow-hidden rounded-full bg-surface">
                      <div className="h-full bg-primary transition-all" style={{ width: `${Math.max(8, progressPercent)}%` }} />
                    </div>
                  </div>
                )}

                <ValidationMessages
                  errors={validationErrors}
                  warnings={validationWarnings}
                  normalizations={validationNormalizations}
                />

                {error && <p className="rounded-lg border border-danger/20 bg-danger-soft px-3 py-2 text-xs text-danger">{error}</p>}

                <div className="flex flex-wrap gap-2">
                  <button
                    onClick={() => { void enhancePrompt(); }}
                    disabled={isEnhancing || isGenerating || !promptForm.prompt.trim()}
                    className="min-h-10 flex-1 rounded-lg border border-border bg-surface px-3 text-xs font-medium text-text-secondary hover:bg-surface-hover hover:text-text disabled:cursor-not-allowed disabled:opacity-45 inline-flex items-center justify-center gap-1.5"
                  >
                    {isEnhancing ? <Loader2 size={14} className="animate-spin" /> : <WandSparkles size={14} />}
                    Enhance
                  </button>
                  <button
                    onClick={startGeneration}
                    disabled={isGenerating || !selectedProviderConfigured || !selectedModel || !promptForm.prompt.trim() || validationErrors.length > 0}
                    className="btn-primary min-h-10 flex-[2] rounded-lg px-3 text-xs font-medium disabled:cursor-not-allowed disabled:opacity-50 inline-flex items-center justify-center gap-1.5"
                  >
                    {isGenerating ? <Loader2 size={14} className="animate-spin" /> : <Clapperboard size={14} />}
                    Generate
                  </button>
                  <button
                    onClick={isGenerating ? () => { void cancelGeneration(); } : clearPrompt}
                    className="min-h-10 rounded-lg border border-border bg-surface px-3 text-xs text-text-secondary hover:bg-surface-hover hover:text-text inline-flex items-center justify-center gap-1.5"
                  >
                    {isGenerating ? <Square size={13} /> : <RefreshCw size={13} />}
                    {isGenerating ? 'Cancel' : 'Reset'}
                  </button>
                </div>
              </div>
            </section>
          </div>
        </aside>

        <DragHandle onMouseDown={startLeft} />

        <main className="min-h-0 min-w-0 flex-1 overflow-y-auto bg-surface p-3">
          <ResultPreview
            asset={activeAsset}
            generation={activeGeneration}
            onEdit={() => setAppMode('video-edit')}
            onUseEnhancedPrompt={handleUseEnhancedPrompt}
            onUsePromptVariant={handleUsePromptVariant}
          />
        </main>

        <DragHandle onMouseDown={startRight} />

        <aside className="min-h-0 overflow-y-auto border-t border-border bg-surface-raised xl:border-l xl:border-t-0" style={rightStyle}>
          <HistoryPanel
            generations={generations}
            activeGenerationId={activeGeneration?.id || null}
            isGenerating={isGenerating}
            onSelect={(generation) => useVideoStudioStore.setState({ activeGenerationId: generation.id, selectedAssetId: generation.output_asset_id || selectedAssetId })}
            onBranch={(generationId) => { void branchFromGeneration(generationId); }}
            onRegenerate={(generationId) => { void regenerateFromGeneration(generationId); }}
            onUseEnhancedPrompt={handleUseEnhancedPrompt}
            onExtend={(assetId) => {
              setPromptField('source_video_asset_id', assetId);
              toast.success('Source video set — choose a model with extension capability and generate');
            }}
            onSendToTimeline={(generation) => { void handleSendGenerationToTimeline(generation); }}
            onSendToChat={(generation) => { void handleSendGenerationToChat(generation); }}
            onRegisterInLibrary={(generation) => { void handleRegisterAssetInLibrary(generation); }}
          />
          <OutputPanel assets={assets} activeAssetId={activeAsset?.id || null} onSelect={selectAsset} />
        </aside>
      </div>
    </div>
  );
}

function ResultPreview({
  asset,
  generation,
  onEdit,
  onUseEnhancedPrompt,
  onUsePromptVariant,
}: {
  asset: VideoAsset | null;
  generation: VideoGenerationDetail | null;
  onEdit: () => void;
  onUseEnhancedPrompt: (generation: VideoGenerationDetail) => void;
  onUsePromptVariant: (generation: VideoGenerationDetail, kind: PromptVariantKind) => void;
}) {
  if (!asset) {
    return (
      <section className="flex h-full min-h-[560px] items-center justify-center rounded-lg border border-dashed border-border bg-surface-alt">
        <div className="max-w-sm px-6 text-center">
          <Film size={34} className="mx-auto mb-4 text-primary" />
          <h2 className="text-base font-semibold text-text">Create a single video</h2>
          <p className="mt-2 text-sm leading-relaxed text-text-muted">No output selected.</p>
        </div>
      </section>
    );
  }

  const url = videoApi.downloadUrl(asset.id);
  const isVideo = asset.mime_type.startsWith('video/');
  const isImage = asset.mime_type.startsWith('image/');
  const isAudio = asset.mime_type.startsWith('audio/');

  return (
    <section className="flex min-h-[560px] flex-col rounded-lg border border-border bg-surface-alt">
      <div className="flex flex-col gap-2 border-b border-border p-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <h2 className="truncate text-sm font-semibold text-text">{asset.file_name}</h2>
          <p className="mt-1 text-xs text-text-muted">{asset.kind} · {asset.mime_type} · {Math.max(1, Math.round(asset.size_bytes / 1024))} KB</p>
        </div>
        <div className="flex shrink-0 flex-wrap items-center gap-2">
          <button
            onClick={() => window.open(url, '_blank', 'noopener,noreferrer')}
            className="min-h-9 rounded-lg border border-border bg-surface px-3 text-xs text-text-secondary hover:bg-surface-hover hover:text-text inline-flex items-center gap-1.5"
          >
            <Download size={13} />
            Download
          </button>
          <button
            onClick={onEdit}
            className="btn-primary min-h-9 rounded-lg px-3 text-xs font-medium inline-flex items-center gap-1.5"
          >
            <Scissors size={13} />
            Edit
          </button>
        </div>
      </div>

      <div className="flex min-h-0 flex-1 items-center justify-center bg-black p-3">
        {isVideo ? (
          <video src={url} controls className="max-h-[68vh] max-w-full rounded-md bg-black" />
        ) : isImage ? (
          <img src={url} alt={asset.file_name} className="max-h-[68vh] max-w-full rounded-md object-contain" />
        ) : isAudio ? (
          <div className="w-full max-w-xl rounded-lg border border-border bg-surface p-4">
            <audio src={url} controls className="w-full" />
          </div>
        ) : (
          <div className="rounded-lg border border-border bg-surface px-4 py-3 text-sm text-text-muted">
            Preview is unavailable for this output type.
          </div>
        )}
      </div>

      {generation && (
        <div className="space-y-2 border-t border-border p-3">
          <div>
            <div className="mb-1 text-[10px] uppercase tracking-wide text-text-muted">Original prompt</div>
            <p className="line-clamp-3 text-xs leading-relaxed text-text-muted">{generation.prompt}</p>
          </div>
          {generation.enhanced_prompt && generation.enhanced_prompt !== generation.prompt && (
            <div className="rounded-md border border-primary/20 bg-primary/5 p-2">
              <div className="mb-1 flex items-center justify-between gap-2">
                <span className="text-[10px] uppercase tracking-wide text-primary">Enhanced prompt</span>
                <button
                  onClick={() => onUseEnhancedPrompt(generation)}
                  className="inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-[10px] text-primary hover:bg-primary/10"
                >
                  <WandSparkles size={11} />
                  Use
                </button>
              </div>
              <p className="line-clamp-3 text-xs leading-relaxed text-text-secondary">{generation.enhanced_prompt}</p>
            </div>
          )}
          <div className="flex flex-wrap gap-1.5">
            {PROMPT_VARIANTS.map((variant) => (
              <button
                key={variant.kind}
                onClick={() => onUsePromptVariant(generation, variant.kind)}
                className="inline-flex items-center gap-1 rounded-md border border-border bg-surface px-2 py-1 text-[10px] text-text-secondary hover:bg-surface-hover hover:text-text"
              >
                <Sparkles size={10} />
                {variant.label}
              </button>
            ))}
          </div>
        </div>
      )}
    </section>
  );
}

function ProjectStrip({
  projects,
  activeProjectId,
  isLoading,
  onNew,
  onSelect,
}: {
  projects: Array<{ id: string; title: string; updated_at: string }>;
  activeProjectId: string | null;
  isLoading: boolean;
  onNew: () => void;
  onSelect: (id: string) => void;
}) {
  return (
    <section className="rounded-lg border border-border bg-surface-alt p-3">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-text">Projects</h2>
        <button
          onClick={onNew}
          className="min-h-8 rounded-lg border border-border bg-surface px-2 text-xs text-text-muted hover:text-text inline-flex items-center gap-1"
        >
          <Plus size={12} />
          New
        </button>
      </div>
      <div className="max-h-40 space-y-1 overflow-y-auto">
        {isLoading && projects.length === 0 ? (
          <div className="flex items-center gap-2 rounded-lg bg-surface px-3 py-2 text-xs text-text-muted">
            <Loader2 size={13} className="animate-spin" />
            Loading projects
          </div>
        ) : projects.length === 0 ? (
          <button
            onClick={onNew}
            className="w-full rounded-lg border border-dashed border-border bg-surface px-3 py-4 text-center text-xs text-text-muted hover:text-text"
          >
            Create your first video project
          </button>
        ) : (
          projects.map((project) => (
            <button
              key={project.id}
              onClick={() => onSelect(project.id)}
              className={`w-full rounded-lg px-3 py-2 text-left text-xs transition-colors ${
                project.id === activeProjectId
                  ? 'border border-primary/30 bg-primary/10 text-primary'
                  : 'border border-transparent bg-surface text-text-secondary hover:bg-surface-hover hover:text-text'
              }`}
            >
              <span className="block truncate font-medium">{project.title}</span>
              <span className="mt-0.5 block text-[10px] text-text-muted">{new Date(project.updated_at).toLocaleDateString()}</span>
            </button>
          ))
        )}
      </div>
    </section>
  );
}

function HistoryPanel({
  generations,
  activeGenerationId,
  isGenerating,
  onSelect,
  onBranch,
  onRegenerate,
  onUseEnhancedPrompt,
  onExtend,
  onSendToTimeline,
  onSendToChat,
  onRegisterInLibrary,
}: {
  generations: VideoGenerationDetail[];
  activeGenerationId: string | null;
  isGenerating: boolean;
  onSelect: (generation: VideoGenerationDetail) => void;
  onBranch: (generationId: string) => void;
  onRegenerate: (generationId: string) => void;
  onUseEnhancedPrompt: (generation: VideoGenerationDetail) => void;
  onExtend: (assetId: string) => void;
  onSendToTimeline: (generation: VideoGenerationDetail) => void;
  onSendToChat: (generation: VideoGenerationDetail) => void;
  onRegisterInLibrary: (generation: VideoGenerationDetail) => void;
}) {
  return (
    <section className="border-b border-border p-3">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-text">Generation History</h2>
        <span className="text-[11px] text-text-muted">{generations.length}</span>
      </div>
      {generations.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border bg-surface-alt px-3 py-8 text-center text-xs text-text-muted">
          No generations yet.
        </div>
      ) : (
        <div className="space-y-2">
          {generations.slice().reverse().map((generation) => {
            const settings = getGenerationSettings(generation);
            const inputAssets = getGenerationInputAssets(generation);
            const inputMode = getGenerationInputMode(inputAssets);
            const inputAssetSummary = formatInputAssetSummary(inputAssets);
            const cost = formatCost(generation.cost_usd);
            const upstream = compactID(generation.upstream_job_id || generation.upstream_request_id);
            const usage = formatUsageSummary(generation.usage_json);
            const metadataBadges = [generation.provider, inputMode, formatSettingSummary(settings), cost, upstream ? `job ${upstream}` : '', usage]
              .filter((item): item is string => Boolean(item));
            return (
              <div
                key={generation.id}
                role="button"
                tabIndex={0}
                onClick={() => onSelect(generation)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault();
                    onSelect(generation);
                  }
                }}
                className={`rounded-lg border p-3 text-left transition-colors ${
                  generation.id === activeGenerationId
                    ? 'border-primary/30 bg-primary/10'
                    : 'border-border bg-surface-alt hover:bg-surface-hover'
                }`}
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="truncate text-xs font-medium text-text">{generation.model}</span>
                  <span className="rounded-md border border-border bg-surface px-1.5 py-0.5 text-[10px] text-text-muted">
                    {generation.status}
                  </span>
                </div>
                <div className="mt-2 flex flex-wrap gap-1">
                  {metadataBadges.map((item) => (
                    <span key={item} className="rounded-md border border-border bg-surface px-1.5 py-0.5 text-[10px] text-text-muted">
                      {item}
                    </span>
                  ))}
                </div>
                {inputAssetSummary && (
                  <p className="mt-1 truncate text-[10px] text-text-muted">Inputs: {inputAssetSummary}</p>
                )}
                <p className="mt-2 line-clamp-3 text-[11px] leading-relaxed text-text-muted">{generation.prompt}</p>
                {generation.enhanced_prompt && generation.enhanced_prompt !== generation.prompt && (
                  <p className="mt-1 line-clamp-2 text-[10px] leading-relaxed text-text-secondary">
                    Enhanced: {generation.enhanced_prompt}
                  </p>
                )}
                {generation.error && (
                  <div className="mt-2 rounded-md border border-danger/20 bg-danger-soft px-2 py-1.5 text-[10px] leading-relaxed text-danger">
                    {generation.error}
                  </div>
                )}
                <div className="mt-2 flex items-center justify-between gap-2">
                  <span className="text-[10px] text-text-muted">{new Date(generation.created_at).toLocaleTimeString()}</span>
                  <div className="flex flex-wrap items-center justify-end gap-1">
                    {generation.status === 'completed' && generation.output_asset_id && (
                      <>
                        <button
                          onClick={(event) => {
                            event.stopPropagation();
                            onSendToTimeline(generation);
                          }}
                          className="inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-[10px] text-sky-300 hover:bg-sky-400/10"
                          title="Send to timeline"
                        >
                          <ListPlus size={11} />
                          Timeline
                        </button>
                        <button
                          onClick={(event) => {
                            event.stopPropagation();
                            onSendToChat(generation);
                          }}
                          className="inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-[10px] text-violet-300 hover:bg-violet-400/10"
                          title="Send to chat"
                        >
                          <MessageSquare size={11} />
                          Chat
                        </button>
                        <button
                          onClick={(event) => {
                            event.stopPropagation();
                            onRegisterInLibrary(generation);
                          }}
                          className="inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-[10px] text-amber-300 hover:bg-amber-400/10"
                          title="Register in File Library"
                        >
                          <Library size={11} />
                          Library
                        </button>
                        <button
                          onClick={(event) => {
                            event.stopPropagation();
                            onExtend(generation.output_asset_id!);
                          }}
                          className="inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-[10px] text-emerald-400 hover:bg-emerald-400/10"
                          title="Extend this video"
                        >
                          <Scissors size={11} />
                          Extend
                        </button>
                      </>
                    )}
                    {generation.enhanced_prompt && generation.enhanced_prompt !== generation.prompt && (
                      <button
                        onClick={(event) => {
                          event.stopPropagation();
                          onUseEnhancedPrompt(generation);
                        }}
                        className="inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-[10px] text-primary hover:bg-primary/10"
                        title="Use enhanced prompt"
                      >
                        <WandSparkles size={11} />
                        Use
                      </button>
                    )}
                    <button
                      onClick={(event) => {
                        event.stopPropagation();
                        onRegenerate(generation.id);
                      }}
                      disabled={isGenerating}
                      className="inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-[10px] text-text-secondary hover:bg-surface-hover hover:text-text disabled:cursor-not-allowed disabled:opacity-45"
                      title="Regenerate with same settings"
                    >
                      <RefreshCw size={11} />
                      Regen
                    </button>
                    <button
                      onClick={(event) => {
                        event.stopPropagation();
                        onBranch(generation.id);
                      }}
                      className="inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-[10px] text-primary hover:bg-primary/10"
                    >
                      <GitBranch size={11} />
                      Branch
                    </button>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
}

function OutputPanel({
  assets,
  activeAssetId,
  onSelect,
}: {
  assets: VideoAsset[];
  activeAssetId: string | null;
  onSelect: (assetId: string) => void;
}) {
  const outputAssets = assets.filter((asset) => asset.source_type === 'generation' || asset.kind === 'video');
  return (
    <section className="p-3">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-text">Outputs</h2>
        <span className="text-[11px] text-text-muted">{outputAssets.length}</span>
      </div>
      {outputAssets.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border bg-surface-alt px-3 py-8 text-center text-xs text-text-muted">
          No outputs yet.
        </div>
      ) : (
        <div className="space-y-2">
          {outputAssets.map((asset) => (
            <button
              key={asset.id}
              onClick={() => onSelect(asset.id)}
              className={`w-full rounded-lg border p-3 text-left ${
                asset.id === activeAssetId ? 'border-primary/30 bg-primary/10' : 'border-border bg-surface-alt hover:bg-surface-hover'
              }`}
            >
              <span className="block truncate text-xs font-medium text-text">{asset.file_name}</span>
              <span className="mt-1 block text-[10px] text-text-muted">{asset.kind} · {asset.mime_type}</span>
            </button>
          ))}
        </div>
      )}
    </section>
  );
}

function AssetPicker({
  value,
  onChange,
  assets,
  onAssetsChange,
  projectId,
  accept = 'any',
  placeholder = 'Select an asset',
}: {
  value: string | undefined;
  onChange: (id: string | undefined) => void;
  assets: VideoAsset[];
  onAssetsChange?: (updated: VideoAsset[]) => void;
  projectId?: string | null;
  accept?: 'image' | 'video' | 'any';
  placeholder?: string;
}) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const createProject = useVideoStudioStore((state) => state.createProject);

  const filtered = assets.filter((a) =>
    accept === 'image' ? a.kind === 'image' :
    accept === 'video' ? a.kind === 'video' :
    true,
  );
  const selectedAsset = value ? assets.find((a) => a.id === value) : undefined;
  const thumbUrl = selectedAsset ? videoAssetUrl(selectedAsset.id) : undefined;
  const isImage = selectedAsset?.kind === 'image' || selectedAsset?.mime_type?.startsWith('image/');
  const isVideo = selectedAsset?.kind === 'video' || selectedAsset?.mime_type?.startsWith('video/');

  const acceptAttr = accept === 'image' ? 'image/*' : accept === 'video' ? 'video/*' : 'image/*,video/*';

  async function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    try {
      // Auto-create a project if none is selected yet
      let pid = projectId;
      if (!pid) {
        const created = await createProject();
        pid = created?.id ?? null;
      }
      if (!pid) {
        toast.error('Could not create a project for upload');
        return;
      }
      const asset = await videoApi.uploadAsset(pid, file);
      if (onAssetsChange) {
        onAssetsChange([asset, ...assets]);
      }
      onChange(asset.id);
    } catch {
      toast.error('Upload failed');
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  }

  return (
    <div>
      <div className="flex gap-1.5">
        <select
          value={value ?? ''}
          onChange={(e) => onChange(e.target.value || undefined)}
          className="min-h-10 min-w-0 flex-1 rounded-lg border border-border bg-surface px-3 text-sm text-text focus:border-primary/50 focus:outline-none"
        >
          <option value="">{placeholder}</option>
          {filtered.map((asset) => (
            <option key={asset.id} value={asset.id}>
              {asset.file_name}
            </option>
          ))}
          {filtered.length === 0 && (
            <option disabled value="">No {accept === 'any' ? '' : accept + ' '}assets in this project</option>
          )}
        </select>
        <input
          ref={fileInputRef}
          type="file"
          accept={acceptAttr}
          className="hidden"
          onChange={(e) => { void handleFileChange(e); }}
        />
        <button
          type="button"
          onClick={() => fileInputRef.current?.click()}
          disabled={uploading}
          title="Upload local file"
          className="min-h-10 min-w-10 rounded-lg border border-border bg-surface text-text-muted hover:bg-surface-hover hover:text-text disabled:cursor-not-allowed disabled:opacity-45 inline-flex items-center justify-center shrink-0"
        >
          {uploading ? <Loader2 size={14} className="animate-spin" /> : <Plus size={14} />}
        </button>
      </div>
      {thumbUrl && isImage && (
        <div className="mt-1.5 overflow-hidden rounded-md border border-border bg-black">
          <img
            src={thumbUrl}
            alt={selectedAsset?.file_name}
            className="max-h-32 w-full object-contain"
          />
        </div>
      )}
      {thumbUrl && isVideo && (
        <div className="mt-1.5 overflow-hidden rounded-md border border-border bg-black">
          <video
            src={thumbUrl}
            className="max-h-20 w-full object-contain"
            muted
            preload="metadata"
          />
        </div>
      )}
    </div>
  );
}

function ControlLabel({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      {label && <span className="mb-1.5 block text-[11px] font-medium text-text-muted">{label}</span>}
      {children}
    </label>
  );
}

function ValidationMessages({
  errors,
  warnings,
  normalizations,
}: {
  errors: VideoGenerationValidationIssue[];
  warnings: VideoGenerationValidationIssue[];
  normalizations: VideoGenerationValidationIssue[];
}) {
  const issues = [...errors, ...normalizations, ...warnings];
  if (issues.length === 0) return null;
  return (
    <div className="space-y-1.5">
      {issues.map((issue) => {
        const tone = issue.severity === 'error'
          ? 'border-danger/20 bg-danger-soft text-danger'
          : issue.severity === 'normalization'
            ? 'border-sky-400/20 bg-sky-400/5 text-sky-200'
            : 'border-amber-500/20 bg-amber-500/5 text-amber-300/90';
        return (
          <p key={`${issue.severity}-${issue.code}-${issue.field ?? ''}`} className={`rounded-lg border px-3 py-2 text-xs ${tone}`}>
            {issue.message}
          </p>
        );
      })}
    </div>
  );
}

function CollapsibleSection({ label, children, defaultOpen = false }: { label: string; children: ReactNode; defaultOpen?: boolean }) {
  return (
    <details open={defaultOpen} className="group rounded-lg border border-border bg-surface-alt">
      <summary className="flex cursor-pointer select-none list-none items-center justify-between rounded-lg px-3 py-2 hover:bg-surface-hover transition-colors [&::-webkit-details-marker]:hidden">
        <span className="text-[11px] font-medium uppercase tracking-wide text-text-muted group-open:text-text transition-colors">{label}</span>
        <ChevronDown size={13} className="text-text-muted transition-transform group-open:rotate-180" />
      </summary>
      <div className="space-y-2 px-3 pb-3 pt-1">
        {children}
      </div>
    </details>
  );
}

function SelectWithCustom({
  label,
  value,
  choices,
  onChange,
  customPlaceholder,
}: {
  label: string;
  value: string;
  choices: string[];
  onChange: (value: string) => void;
  customPlaceholder?: string;
}) {
  const normalized = value.trim();
  const choiceSet = useMemo(() => new Set(choices.map((c) => c.toLowerCase())), [choices]);
  const isCustom = normalized !== '' && !choiceSet.has(normalized.toLowerCase());
  const [mode, setMode] = useState<string>(
    normalized === '' ? AUTO_VALUE : isCustom ? CUSTOM_VALUE : normalized,
  );

  // Keep mode in sync when the value is changed externally (e.g. clearPrompt)
  useEffect(() => {
    if (!normalized) {
      setMode(AUTO_VALUE);
    } else if (choiceSet.has(normalized.toLowerCase())) {
      setMode(normalized);
    } else {
      setMode(CUSTOM_VALUE);
    }
  }, [normalized, choiceSet]);

  return (
    <div className="space-y-1.5">
      <label className="block text-[11px] font-medium text-text-muted">{label}</label>
      <select
        value={mode}
        onChange={(event) => {
          const next = event.target.value;
          setMode(next);
          if (next === AUTO_VALUE) {
            onChange('');
          } else if (next === CUSTOM_VALUE) {
            // keep existing custom text, or clear if it was a known preset
            if (choiceSet.has(normalized.toLowerCase())) onChange('');
          } else {
            onChange(next);
          }
        }}
        className="w-full min-h-10 rounded-lg border border-border bg-surface px-3 text-sm text-text focus:border-primary/50 focus:outline-none"
      >
        <option value={AUTO_VALUE}>Auto</option>
        {choices.map((choice) => (
          <option key={choice} value={choice}>{choice}</option>
        ))}
        <option value={CUSTOM_VALUE}>Custom…</option>
      </select>
      {mode === CUSTOM_VALUE && (
        <input
          type="text"
          value={choiceSet.has(normalized.toLowerCase()) ? '' : value}
          onChange={(event) => onChange(event.target.value)}
          placeholder={customPlaceholder ?? 'Enter custom value'}
          className="w-full min-h-10 rounded-lg border border-border bg-surface px-3 text-sm text-text placeholder:text-text-muted/50 focus:border-primary/50 focus:outline-none"
        />
      )}
    </div>
  );
}
