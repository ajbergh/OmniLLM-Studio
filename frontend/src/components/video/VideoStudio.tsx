import { useEffect, useMemo } from 'react';
import type { ReactNode } from 'react';
import {
  BookMarked,
  Clapperboard,
  Download,
  Film,
  GitBranch,
  Loader2,
  MessageSquare,
  Plus,
  RefreshCw,
  Sparkles,
  Square,
  WandSparkles,
} from 'lucide-react';
import { toast } from 'sonner';
import { videoApi } from '../../api';
import { useVideoStudioStore } from '../../stores/videoStudio';
import { useConversationStore, useCrossoverStore } from '../../stores';
import type { VideoGenerationDetail, VideoProviderKey } from '../../types/video';
import { VideoInspector } from './VideoInspector';
import { VideoPreviewCanvas } from './VideoPreviewCanvas';
import { VideoRenderPanel } from './VideoRenderPanel';
import { VideoTimeline } from './timeline/VideoTimeline';

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
  const addAssetToTimeline = useVideoStudioStore((state) => state.addAssetToTimeline);

  const { crossoverContext, clearCrossoverContext } = useCrossoverStore();

  useEffect(() => {
    loadProviders();
    loadProjects();
  }, [loadProviders, loadProjects]);

  // Receive crossover context from Image Studio or Music Studio.
  useEffect(() => {
    if (!crossoverContext || crossoverContext.type !== 'to-video') return;
    const { prompt } = crossoverContext.data;
    clearCrossoverContext();
    if (prompt) {
      setPromptField('prompt', prompt);
      toast.success('Prompt pre-filled from Image/Music Studio');
    }
  }, [crossoverContext, clearCrossoverContext, setPromptField]);

  const activeProject = useMemo(
    () => projects.find((project) => project.id === activeProjectId) || null,
    [projects, activeProjectId],
  );
  const activeGeneration = useMemo(
    () => generations.find((generation) => generation.id === activeGenerationId) || generations[generations.length - 1] || null,
    [generations, activeGenerationId],
  );
  const activeAsset = useMemo(
    () => assets.find((asset) => asset.id === selectedAssetId) || assets.find((asset) => asset.id === activeGeneration?.output_asset_id) || assets[0] || null,
    [assets, activeGeneration, selectedAssetId],
  );
  const models = modelsByProvider[selectedProvider] || [];
  const selectedModelCapabilities = models.find((model) => model.id === selectedModel)?.capabilities || [];
  const progressPercent = Math.round((generationProgress?.progress || 0) * 100);

  const handleDownload = () => {
    if (!activeAsset) return;
    window.open(videoApi.downloadUrl(activeAsset.id), '_blank', 'noopener,noreferrer');
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
            className="min-h-9 rounded-lg border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:text-text hover:bg-surface-hover transition-colors inline-flex items-center gap-1.5"
          >
            <Plus size={13} />
            Project
          </button>
          <button
            onClick={handleDownload}
            disabled={!activeAsset}
            className="min-h-9 rounded-lg border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:text-text hover:bg-surface-hover disabled:opacity-45 disabled:cursor-not-allowed transition-colors inline-flex items-center gap-1.5"
          >
            <Download size={13} />
            Download
          </button>
        </div>
      </div>

      <div className="grid min-h-0 flex-1 grid-cols-1 xl:grid-cols-[340px_minmax(0,1fr)_340px]">
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
                  <h2 className="text-sm font-semibold text-text">Generate</h2>
                </div>
                {providers.some((provider) => provider.mock) && (
                  <span className="rounded-md border border-border bg-surface px-2 py-1 text-[10px] uppercase tracking-wide text-text-muted">
                    Mock ready
                  </span>
                )}
              </div>

              <div className="space-y-3">
                <ControlLabel label="Provider">
                  <select
                    value={selectedProvider}
                    onChange={(event) => { void setProvider(event.target.value as VideoProviderKey); }}
                    className="min-h-10 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text focus:outline-none focus:border-primary/50"
                  >
                    {providers.map((provider) => (
                      <option key={provider.key} value={provider.key}>
                        {provider.display_name}
                      </option>
                    ))}
                  </select>
                </ControlLabel>

                <ControlLabel label="Model">
                  <div className="flex gap-2">
                    <select
                      value={selectedModel || ''}
                      onChange={(event) => setModel(event.target.value)}
                      className="min-h-10 min-w-0 flex-1 rounded-lg border border-border bg-surface px-3 text-sm text-text focus:outline-none focus:border-primary/50"
                    >
                      {models.map((model) => (
                        <option key={model.id} value={model.id}>
                          {model.name}
                        </option>
                      ))}
                    </select>
                    <button
                      onClick={() => { void useVideoStudioStore.getState().loadModels(selectedProvider, true); }}
                      className="min-h-10 min-w-10 rounded-lg border border-border bg-surface text-text-muted hover:text-text hover:bg-surface-hover inline-flex items-center justify-center"
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
                    rows={7}
                    maxLength={models.find((model) => model.id === selectedModel)?.max_prompt_chars || undefined}
                    placeholder="Describe the scene, subject, movement, camera, and style."
                    className="w-full resize-y rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-muted/50 focus:outline-none focus:border-primary/50"
                  />
                </ControlLabel>

                {selectedModelCapabilities.includes('negative_prompt') && (
                  <ControlLabel label="Negative prompt">
                    <input
                      value={promptForm.negative_prompt}
                      onChange={(event) => setPromptField('negative_prompt', event.target.value)}
                      className="min-h-10 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text focus:outline-none focus:border-primary/50"
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
                      {(models.find((model) => model.id === selectedModel)?.aspect_ratios || ['16:9', '9:16', '1:1']).map((value) => (
                        <option key={value} value={value}>{value}</option>
                      ))}
                    </select>
                  </ControlLabel>
                  <ControlLabel label="Duration">
                    <input
                      type="number"
                      min={2}
                      max={30}
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
                      {(models.find((model) => model.id === selectedModel)?.resolutions || ['720p', '1080p']).map((value) => (
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
                      {(models.find((model) => model.id === selectedModel)?.fps_options || [24, 30]).map((value) => (
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
                    className="w-full resize-y rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text focus:outline-none focus:border-primary/50"
                    placeholder="Lighting, mood, continuity, production details"
                  />
                </ControlLabel>

                {generationProgress && (
                  <div className="rounded-lg border border-primary/20 bg-primary/5 p-2">
                    <div className="mb-1 flex items-center justify-between gap-2 text-[11px] text-text-muted">
                      <span className="truncate">{generationProgress.message}</span>
                      <span>{progressPercent > 0 ? `${progressPercent}%` : generationProgress.stage}</span>
                    </div>
                    <div className="h-1.5 overflow-hidden rounded-full bg-surface">
                      <div
                        className="h-full bg-primary transition-all"
                        style={{ width: `${Math.max(8, progressPercent)}%` }}
                      />
                    </div>
                  </div>
                )}

                {error && <p className="rounded-lg border border-danger/20 bg-danger-soft px-3 py-2 text-xs text-danger">{error}</p>}

                <div className="flex flex-wrap gap-2">
                  <button
                    onClick={() => { void enhancePrompt(); }}
                    disabled={isEnhancing || isGenerating || !promptForm.prompt.trim()}
                    className="min-h-10 flex-1 rounded-lg border border-border bg-surface px-3 text-xs font-medium text-text-secondary hover:text-text hover:bg-surface-hover disabled:opacity-45 disabled:cursor-not-allowed inline-flex items-center justify-center gap-1.5"
                  >
                    {isEnhancing ? <Loader2 size={14} className="animate-spin" /> : <WandSparkles size={14} />}
                    Enhance
                  </button>
                  <button
                    onClick={() => generate()}
                    disabled={isGenerating || !selectedModel || !promptForm.prompt.trim()}
                    className="btn-primary min-h-10 flex-[2] rounded-lg px-3 text-xs font-medium inline-flex items-center justify-center gap-1.5 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {isGenerating ? <Loader2 size={14} className="animate-spin" /> : <Clapperboard size={14} />}
                    Generate
                  </button>
                  <button
                    onClick={isGenerating ? stopGeneration : clearPrompt}
                    className="min-h-10 rounded-lg border border-border bg-surface px-3 text-xs text-text-secondary hover:text-text hover:bg-surface-hover inline-flex items-center justify-center gap-1.5"
                  >
                    {isGenerating ? <Square size={13} /> : <RefreshCw size={13} />}
                    {isGenerating ? 'Stop' : 'Reset'}
                  </button>
                </div>
              </div>
            </section>
          </div>
        </aside>

        <main className="flex min-h-[620px] min-w-0 flex-col bg-surface">
          <section className="min-h-0 flex-1 border-b border-border p-3">
            <VideoPreviewCanvas />
          </section>

          <section className="h-72 shrink-0 border-b border-border bg-surface-raised p-3">
            <VideoTimeline />
          </section>
        </main>

        <aside className="min-h-0 border-t border-border bg-surface-raised xl:border-l xl:border-t-0">
          <div className="flex h-full min-h-[520px] flex-col overflow-y-auto">
            <VideoInspector />
            <div className="px-3 pb-3">
              <VideoRenderPanel />
            </div>
            <HistoryPanel
              generations={generations}
              activeGenerationId={activeGeneration?.id || null}
              onSelect={(generation) => useVideoStudioStore.setState({ activeGenerationId: generation.id })}
              onBranch={(generationId) => { void branchFromGeneration(generationId); }}
            />
            <AssetPanel
              assets={assets}
              activeAssetId={activeAsset?.id || null}
              onSelect={selectAsset}
              onAdd={(assetId) => { void addAssetToTimeline(assetId); }}
            />
          </div>
        </aside>
      </div>
    </div>
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
    <section className="min-h-0 flex-1 overflow-y-auto border-b border-border p-3">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-text">History</h2>
        <span className="text-[11px] text-text-muted">{generations.length} run{generations.length === 1 ? '' : 's'}</span>
      </div>
      {generations.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border bg-surface-alt px-3 py-8 text-center text-xs text-text-muted">
          Generated clips and branches appear here.
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
              className={`w-full rounded-lg border p-3 text-left transition-colors ${
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

function AssetPanel({
  assets,
  activeAssetId,
  onSelect,
  onAdd,
}: {
  assets: Array<{ id: string; file_name: string; kind: string; size_bytes: number; mime_type: string }>;
  activeAssetId: string | null;
  onSelect: (assetId: string) => void;
  onAdd: (assetId: string) => void;
}) {
  const createConversation = useConversationStore((state) => state.createConversation);
  const selectConversation = useConversationStore((state) => state.selectConversation);

  const handleSendToChat = async (asset: { id: string; file_name: string }) => {
    try {
      const convo = await createConversation(`Video: ${asset.file_name}`);
      await videoApi.attachAssetToConversation(asset.id, convo.id);
      selectConversation(convo.id);
      toast.success('Video asset sent to chat');
    } catch (err) {
      toast.error(`Failed to send to chat: ${err instanceof Error ? err.message : String(err)}`);
    }
  };

  const handleRegisterInLibrary = async (asset: { id: string; file_name: string }) => {
    try {
      await videoApi.registerAssetInLibrary(asset.id);
      toast.success(`"${asset.file_name}" added to File Library`);
    } catch (err) {
      toast.error(`Failed to register in library: ${err instanceof Error ? err.message : String(err)}`);
    }
  };
  return (
    <section className="min-h-0 border-t border-border p-3">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-text">Assets</h2>
        <span className="text-[11px] text-text-muted">{assets.length} item{assets.length === 1 ? '' : 's'}</span>
      </div>
      {assets.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border bg-surface-alt px-3 py-8 text-center text-xs text-text-muted">
          Project media bin is empty.
        </div>
      ) : (
        <div className="space-y-2">
          {assets.map((asset) => (
            <div
              key={asset.id}
              role="button"
              tabIndex={0}
              onClick={() => onSelect(asset.id)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  onSelect(asset.id);
                }
              }}
              className={`rounded-lg border p-3 ${
                asset.id === activeAssetId ? 'border-primary/30 bg-primary/10' : 'border-border bg-surface-alt'
              }`}
            >
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <p className="truncate text-xs font-medium text-text">{asset.file_name}</p>
                  <p className="mt-1 text-[10px] text-text-muted">{asset.kind} · {asset.mime_type}</p>
                </div>
                <button
                  onClick={(event) => {
                    event.stopPropagation();
                    onAdd(asset.id);
                  }}
                  className="min-h-8 min-w-8 rounded-lg border border-border bg-surface text-text-muted hover:text-text inline-flex items-center justify-center"
                  aria-label={`Add ${asset.file_name} to timeline`}
                  title="Add to timeline"
                >
                  <Plus size={13} />
                </button>
                <button
                  onClick={(event) => {
                    event.stopPropagation();
                    window.open(videoApi.downloadUrl(asset.id), '_blank', 'noopener,noreferrer');
                    toast.success('Opening asset download');
                  }}
                  className="min-h-8 min-w-8 rounded-lg border border-border bg-surface text-text-muted hover:text-text inline-flex items-center justify-center"
                  aria-label={`Download ${asset.file_name}`}
                  title="Download"
                >
                  <Download size={13} />
                </button>
                <button
                  onClick={(event) => {
                    event.stopPropagation();
                    void handleSendToChat(asset);
                  }}
                  className="min-h-8 min-w-8 rounded-lg border border-border bg-surface text-text-muted hover:text-text inline-flex items-center justify-center"
                  aria-label={`Send ${asset.file_name} to chat`}
                  title="Send to Chat"
                >
                  <MessageSquare size={13} />
                </button>
                <button
                  onClick={(event) => {
                    event.stopPropagation();
                    void handleRegisterInLibrary(asset);
                  }}
                  className="min-h-8 min-w-8 rounded-lg border border-border bg-surface text-text-muted hover:text-text inline-flex items-center justify-center"
                  aria-label={`Register ${asset.file_name} in File Library`}
                  title="Register in File Library"
                >
                  <BookMarked size={13} />
                </button>
              </div>
              <p className="mt-2 text-[10px] text-text-muted">{Math.max(1, Math.round(asset.size_bytes / 1024))} KB</p>
            </div>
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
