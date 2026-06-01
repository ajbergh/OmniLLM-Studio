import { useEffect, useMemo } from 'react';
import { DragHandle, useResizablePanels } from '../ResizablePanels';
import { Download, Film, Loader2, Plus, Scissors } from 'lucide-react';
import { toast } from 'sonner';
import { videoApi } from '../../api';
import { useSettingsStore } from '../../stores';
import { useVideoStudioStore } from '../../stores/videoStudio';
import type { VideoAsset } from '../../types/video';
import { VideoInspector } from './VideoInspector';
import { VideoPreviewCanvas } from './VideoPreviewCanvas';
import { VideoRenderPanel } from './VideoRenderPanel';
import { VideoTimeline } from './timeline/VideoTimeline';

export function VideoEditStudio() {
  const projects = useVideoStudioStore((state) => state.projects);
  const activeProjectId = useVideoStudioStore((state) => state.activeProjectId);
  const selectedAssetId = useVideoStudioStore((state) => state.selectedAssetId);
  const assets = useVideoStudioStore((state) => state.assets);
  const timeline = useVideoStudioStore((state) => state.timeline);
  const isLoading = useVideoStudioStore((state) => state.isLoading);
  const loadProviders = useVideoStudioStore((state) => state.loadProviders);
  const loadProjects = useVideoStudioStore((state) => state.loadProjects);
  const createProject = useVideoStudioStore((state) => state.createProject);
  const selectProject = useVideoStudioStore((state) => state.selectProject);
  const selectAsset = useVideoStudioStore((state) => state.selectAsset);
  const addAssetToTimeline = useVideoStudioStore((state) => state.addAssetToTimeline);
  const addTextClip = useVideoStudioStore((state) => state.addTextClip);
  const setAppMode = useSettingsStore((state) => state.setAppMode);

  useEffect(() => {
    loadProviders();
    loadProjects();
  }, [loadProviders, loadProjects]);

  const activeProject = useMemo(
    () => projects.find((project) => project.id === activeProjectId) || null,
    [projects, activeProjectId],
  );
  const activeAsset = useMemo(
    () => assets.find((asset) => asset.id === selectedAssetId) || assets[0] || null,
    [assets, selectedAssetId],
  );

  const { leftStyle, rightStyle, startLeft, startRight } = useResizablePanels({ defaultLeft: 300, defaultRight: 340 });

  return (
    <div className="flex min-h-0 flex-1 flex-col bg-surface">
      <div className="flex flex-col gap-2 border-b border-border bg-surface-raised px-3 py-2 sm:flex-row sm:items-center sm:justify-between sm:px-4">
        <div className="flex min-w-0 items-center gap-3">
          <Scissors size={18} className="text-primary" />
          <span className="shrink-0 text-sm font-medium text-text">Video Edit Studio</span>
          {activeProject && (
            <span className="min-w-0 truncate text-xs text-text-muted">- {activeProject.title}</span>
          )}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <button
            onClick={() => setAppMode('video')}
            className="min-h-9 rounded-lg border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:bg-surface-hover hover:text-text transition-colors inline-flex items-center gap-1.5"
          >
            <Film size={13} />
            Create
          </button>
          <button
            onClick={() => { void createProject(); }}
            className="min-h-9 rounded-lg border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:bg-surface-hover hover:text-text transition-colors inline-flex items-center gap-1.5"
          >
            <Plus size={13} />
            Project
          </button>
          <button
            onClick={() => { void addTextClip(); }}
            disabled={!timeline}
            className="min-h-9 rounded-lg border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:bg-surface-hover hover:text-text disabled:cursor-not-allowed disabled:opacity-45 transition-colors"
          >
            Text
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
            <AssetPanel
              assets={assets}
              activeAssetId={activeAsset?.id || null}
              onSelect={selectAsset}
              onAdd={(assetId) => { void addAssetToTimeline(assetId); }}
            />
          </div>
        </aside>

        <DragHandle onMouseDown={startLeft} />

        <main className="flex min-h-[620px] min-w-0 flex-1 flex-col bg-surface">
          <section className="min-h-0 flex-1 border-b border-border p-3">
            <VideoPreviewCanvas />
          </section>
          <section className="h-72 shrink-0 border-b border-border bg-surface-raised p-3">
            <VideoTimeline />
          </section>
        </main>

        <DragHandle onMouseDown={startRight} />

        <aside className="min-h-0 overflow-y-auto border-t border-border bg-surface-raised xl:border-l xl:border-t-0" style={rightStyle}>
          <VideoInspector />
          <div className="px-3 pb-3">
            <VideoRenderPanel />
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
            Create a video project first
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

function AssetPanel({
  assets,
  activeAssetId,
  onSelect,
  onAdd,
}: {
  assets: VideoAsset[];
  activeAssetId: string | null;
  onSelect: (assetId: string) => void;
  onAdd: (assetId: string) => void;
}) {
  return (
    <section className="rounded-lg border border-border bg-surface-alt p-3">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-text">Media Bin</h2>
        <span className="text-[11px] text-text-muted">{assets.length}</span>
      </div>
      {assets.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border bg-surface px-3 py-8 text-center text-xs text-text-muted">
          No media yet.
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
                asset.id === activeAssetId ? 'border-primary/30 bg-primary/10' : 'border-border bg-surface'
              }`}
            >
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <p className="truncate text-xs font-medium text-text">{asset.file_name}</p>
                  <p className="mt-1 text-[10px] text-text-muted">{asset.kind} · {asset.mime_type}</p>
                </div>
                <div className="flex shrink-0 gap-1">
                  <button
                    onClick={(event) => {
                      event.stopPropagation();
                      onAdd(asset.id);
                    }}
                    className="min-h-8 min-w-8 rounded-lg border border-border bg-surface-alt text-text-muted hover:text-text inline-flex items-center justify-center"
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
                    className="min-h-8 min-w-8 rounded-lg border border-border bg-surface-alt text-text-muted hover:text-text inline-flex items-center justify-center"
                    aria-label={`Download ${asset.file_name}`}
                    title="Download"
                  >
                    <Download size={13} />
                  </button>
                </div>
              </div>
              <p className="mt-2 text-[10px] text-text-muted">{Math.max(1, Math.round(asset.size_bytes / 1024))} KB</p>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
