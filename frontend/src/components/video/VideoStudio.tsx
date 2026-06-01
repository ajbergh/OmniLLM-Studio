import { useEffect, useMemo } from 'react';
import type { ReactNode } from 'react';
import {
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
import { videoApi } from '../../api';
import { useSettingsStore } from '../../stores';
import { useVideoStudioStore } from '../../stores/videoStudio';
import type { VideoAsset, VideoGenerationDetail, VideoProviderKey } from '../../types/video';

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
  const stopGeneration = useVideoStudioStore((state) => state.stopGeneration);
  const selectAsset = useVideoStudioStore((state) => state.selectAsset);
  const setAppMode = useSettingsStore((state) => state.setAppMode);

  useEffect(() => {
    loadProviders();
    loadProjects();
  }, [loadProviders, loadProjects]);

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

      <div className="grid min-h-0 flex-1 grid-cols-1 xl:grid-cols-[360px_minmax(0,1fr)_320px]">
        <aside className="min-h-0 overflow-y-auto border-b border-border bg-surface xl:border-b-0 xl:border-r">
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

                <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-1 2xl:grid-cols-2">
                  <ControlLabel label="Camera">
                    <input
                      value={promptForm.camera_motion}
                      onChange={(event) => setPromptField('camera_motion', event.target.value)}
                      className="min-h-10 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text"
                    />
                  </ControlLabel>
                  <ControlLabel label="Shot">
                    <input
                      value={promptForm.shot_type}
                      onChange={(event) => setPromptField('shot_type', event.target.value)}
                      className="min-h-10 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text"
                    />
                  </ControlLabel>
                </div>

                <ControlLabel label="Production notes">
                  <textarea
                    value={promptForm.production_notes}
                    onChange={(event) => setPromptField('production_notes', event.target.value)}
                    rows={3}
                    className="w-full resize-y rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text focus:border-primary/50 focus:outline-none"
                    placeholder="Lighting, continuity, style constraints"
                  />
                </ControlLabel>

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
                    onClick={isGenerating ? stopGeneration : clearPrompt}
                    className="min-h-10 rounded-lg border border-border bg-surface px-3 text-xs text-text-secondary hover:bg-surface-hover hover:text-text inline-flex items-center justify-center gap-1.5"
                  >
                    {isGenerating ? <Square size={13} /> : <RefreshCw size={13} />}
                    {isGenerating ? 'Stop' : 'Reset'}
                  </button>
                </div>
              </div>
            </section>
          </div>
        </aside>

        <main className="min-h-0 min-w-0 overflow-y-auto bg-surface p-3">
          <ResultPreview asset={activeAsset} generation={activeGeneration} onEdit={() => setAppMode('video-edit')} />
        </main>

        <aside className="min-h-0 overflow-y-auto border-t border-border bg-surface-raised xl:border-l xl:border-t-0">
          <HistoryPanel
            generations={generations}
            activeGenerationId={activeGeneration?.id || null}
            onSelect={(generation) => useVideoStudioStore.setState({ activeGenerationId: generation.id, selectedAssetId: generation.output_asset_id || selectedAssetId })}
            onBranch={(generationId) => { void branchFromGeneration(generationId); }}
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
}: {
  generations: VideoGenerationDetail[];
  activeGenerationId: string | null;
  onSelect: (generation: VideoGenerationDetail) => void;
  onBranch: (generationId: string) => void;
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

function ControlLabel({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-[11px] font-medium text-text-muted">{label}</span>
      {children}
    </label>
  );
}
