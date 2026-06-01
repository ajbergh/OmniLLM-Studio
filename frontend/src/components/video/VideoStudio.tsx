import { useEffect, useMemo, useRef, useState } from 'react';
import { DragHandle, useResizablePanels } from '../ResizablePanels';
import type { ReactNode } from 'react';
import {
  ChevronDown,
  Clapperboard,
  Download,
  Film,
  GitBranch,
  Loader2,
  Plus,
  RefreshCw,
  Scissors,
  Sparkles,
  Square,
  WandSparkles,
} from 'lucide-react';
import { toast } from 'sonner';
import { videoApi } from '../../api';
import { videoAssetUrl } from '../../api';
import { useCrossoverStore, useSettingsStore } from '../../stores';
import { useVideoStudioStore } from '../../stores/videoStudio';
import type { VideoAsset, VideoGenerationDetail, VideoProviderKey } from '../../types/video';

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
  const cancelGeneration = useVideoStudioStore((state) => state.cancelGeneration);
  const selectAsset = useVideoStudioStore((state) => state.selectAsset);
  const setAppMode = useSettingsStore((state) => state.setAppMode);
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

  const startGeneration = () => {
    setPromptField('place_on_timeline', false);
    generate();
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
                    Configure an OpenRouter or Gemini video provider before generating.
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
                {selectedModelCapabilities.includes('person_generation') && (
                  <CollapsibleSection label="Advanced">
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
                    disabled={isGenerating || !selectedProviderConfigured || !selectedModel || !promptForm.prompt.trim()}
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
          <ResultPreview asset={activeAsset} generation={activeGeneration} onEdit={() => setAppMode('video-edit')} />
        </main>

        <DragHandle onMouseDown={startRight} />

        <aside className="min-h-0 overflow-y-auto border-t border-border bg-surface-raised xl:border-l xl:border-t-0" style={rightStyle}>
          <HistoryPanel
            generations={generations}
            activeGenerationId={activeGeneration?.id || null}
            onSelect={(generation) => useVideoStudioStore.setState({ activeGenerationId: generation.id, selectedAssetId: generation.output_asset_id || selectedAssetId })}
            onBranch={(generationId) => { void branchFromGeneration(generationId); }}
            onExtend={(assetId) => {
              setPromptField('source_video_asset_id', assetId);
              toast.success('Source video set — choose a model with extension capability and generate');
            }}
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
}: {
  asset: VideoAsset | null;
  generation: VideoGenerationDetail | null;
  onEdit: () => void;
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
        <div className="border-t border-border p-3">
          <p className="line-clamp-3 text-xs leading-relaxed text-text-muted">{generation.prompt}</p>
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
  onSelect,
  onBranch,
  onExtend,
}: {
  generations: VideoGenerationDetail[];
  activeGenerationId: string | null;
  onSelect: (generation: VideoGenerationDetail) => void;
  onBranch: (generationId: string) => void;
  onExtend: (assetId: string) => void;
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
          {generations.slice().reverse().map((generation) => (
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
              <p className="mt-2 line-clamp-3 text-[11px] leading-relaxed text-text-muted">{generation.prompt}</p>
              <div className="mt-2 flex items-center justify-between gap-2">
                <span className="text-[10px] text-text-muted">{new Date(generation.created_at).toLocaleTimeString()}</span>
                <div className="flex items-center gap-1">
                  {generation.status === 'completed' && generation.output_asset_id && (
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
                  )}
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
          ))}
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
