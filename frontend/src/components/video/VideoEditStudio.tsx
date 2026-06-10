import { useEffect, useMemo, useState } from 'react';
import { DragHandle, useResizablePanels } from '../ResizablePanels';
import { Check, Download, Film, LayoutGrid, List, Loader2, Music2, Pencil, Plus, Scissors, Trash2, X } from 'lucide-react';
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
  const renameAsset = useVideoStudioStore((state) => state.renameAsset);
  const deleteAsset = useVideoStudioStore((state) => state.deleteAsset);
  const loadRendererCapabilities = useVideoStudioStore((state) => state.loadRendererCapabilities);
  const setAppMode = useSettingsStore((state) => state.setAppMode);

  useEffect(() => {
    loadProviders();
    loadProjects();
    void loadRendererCapabilities();
  }, [loadProviders, loadProjects, loadRendererCapabilities]);

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
              onRename={(assetId, name) => { void renameAsset(assetId, name); }}
              onDelete={(assetId) => { void deleteAsset(assetId); }}
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

const ASSET_FILTERS = [
  { key: 'all', label: 'All' },
  { key: 'video', label: 'Video' },
  { key: 'image', label: 'Image' },
  { key: 'audio', label: 'Audio' },
  { key: 'export', label: 'Exports' },
] as const;

type AssetFilterKey = (typeof ASSET_FILTERS)[number]['key'];

function assetMatchesFilter(asset: VideoAsset, filter: AssetFilterKey): boolean {
  if (filter === 'all') return true;
  if (filter === 'audio') return asset.kind === 'audio' || asset.kind === 'music';
  return asset.kind === filter;
}

function AssetThumbnail({ asset, className }: { asset: VideoAsset; className: string }) {
  const url = videoApi.downloadUrl(asset.id);
  if (asset.kind === 'image') {
    return <img src={url} alt={asset.file_name} loading="lazy" className={`${className} object-cover`} />;
  }
  if (asset.kind === 'video' || asset.kind === 'export') {
    return <video src={url} muted preload="metadata" className={`${className} object-cover`} />;
  }
  return (
    <div className={`${className} flex items-center justify-center text-text-muted`}>
      <Music2 size={16} />
    </div>
  );
}

function AssetPanel({
  assets,
  activeAssetId,
  onSelect,
  onAdd,
  onRename,
  onDelete,
}: {
  assets: VideoAsset[];
  activeAssetId: string | null;
  onSelect: (assetId: string) => void;
  onAdd: (assetId: string) => void;
  onRename: (assetId: string, fileName: string) => void;
  onDelete: (assetId: string) => void;
}) {
  const [view, setView] = useState<'list' | 'grid'>('list');
  const [filter, setFilter] = useState<AssetFilterKey>('all');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingName, setEditingName] = useState('');

  const filtered = assets.filter((asset) => assetMatchesFilter(asset, filter));

  const commitRename = (assetId: string) => {
    if (editingName.trim()) onRename(assetId, editingName);
    setEditingId(null);
  };

  const actionButtons = (asset: VideoAsset) => (
    <>
      <button
        onClick={(event) => {
          event.stopPropagation();
          onAdd(asset.id);
        }}
        className="min-h-7 min-w-7 rounded-md border border-border bg-surface-alt text-text-muted hover:text-text inline-flex items-center justify-center"
        aria-label={`Add ${asset.file_name} to timeline`}
        title="Add to timeline"
      >
        <Plus size={12} />
      </button>
      <button
        onClick={(event) => {
          event.stopPropagation();
          setEditingId(asset.id);
          setEditingName(asset.file_name);
        }}
        className="min-h-7 min-w-7 rounded-md border border-border bg-surface-alt text-text-muted hover:text-text inline-flex items-center justify-center"
        aria-label={`Rename ${asset.file_name}`}
        title="Rename"
      >
        <Pencil size={12} />
      </button>
      <button
        onClick={(event) => {
          event.stopPropagation();
          window.open(videoApi.downloadUrl(asset.id), '_blank', 'noopener,noreferrer');
          toast.success('Opening asset download');
        }}
        className="min-h-7 min-w-7 rounded-md border border-border bg-surface-alt text-text-muted hover:text-text inline-flex items-center justify-center"
        aria-label={`Download ${asset.file_name}`}
        title="Download"
      >
        <Download size={12} />
      </button>
      <button
        onClick={(event) => {
          event.stopPropagation();
          if (window.confirm(`Delete "${asset.file_name}"? Clips using it will stop rendering.`)) {
            onDelete(asset.id);
          }
        }}
        className="min-h-7 min-w-7 rounded-md border border-border bg-surface-alt text-text-muted hover:text-red-400 inline-flex items-center justify-center"
        aria-label={`Delete ${asset.file_name}`}
        title="Delete"
      >
        <Trash2 size={12} />
      </button>
    </>
  );

  const nameOrEditor = (asset: VideoAsset) =>
    editingId === asset.id ? (
      <span className="flex items-center gap-1" onClick={(event) => event.stopPropagation()}>
        <input
          value={editingName}
          autoFocus
          onChange={(event) => setEditingName(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') commitRename(asset.id);
            if (event.key === 'Escape') setEditingId(null);
          }}
          className="min-h-7 w-full min-w-0 rounded-md border border-border bg-surface px-2 text-xs text-text"
          aria-label="Asset name"
        />
        <button onClick={() => commitRename(asset.id)} className="rounded p-1 text-text-muted hover:text-text" title="Save name" aria-label="Save name">
          <Check size={12} />
        </button>
        <button onClick={() => setEditingId(null)} className="rounded p-1 text-text-muted hover:text-text" title="Cancel rename" aria-label="Cancel rename">
          <X size={12} />
        </button>
      </span>
    ) : (
      <p className="truncate text-xs font-medium text-text">{asset.file_name}</p>
    );

  const sourceLine = (asset: VideoAsset) => {
    const bits = [asset.kind, asset.source_type];
    if (asset.source_studio) bits.push(`from ${asset.source_studio}`);
    if (asset.duration_ms) bits.push(`${Math.round(asset.duration_ms / 100) / 10}s`);
    bits.push(`${Math.max(1, Math.round(asset.size_bytes / 1024))} KB`);
    return bits.filter(Boolean).join(' · ');
  };

  return (
    <section className="rounded-lg border border-border bg-surface-alt p-3">
      <div className="mb-2 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-text">Media Bin</h2>
        <div className="flex items-center gap-1">
          <span className="text-[11px] text-text-muted">{filtered.length}</span>
          <button
            onClick={() => setView(view === 'list' ? 'grid' : 'list')}
            className="rounded-md border border-border bg-surface p-1.5 text-text-muted hover:text-text"
            title={view === 'list' ? 'Grid view' : 'List view'}
            aria-label={view === 'list' ? 'Switch to grid view' : 'Switch to list view'}
          >
            {view === 'list' ? <LayoutGrid size={12} /> : <List size={12} />}
          </button>
        </div>
      </div>
      <div className="mb-3 flex flex-wrap gap-1">
        {ASSET_FILTERS.map((item) => (
          <button
            key={item.key}
            onClick={() => setFilter(item.key)}
            className={`rounded-md border px-2 py-0.5 text-[10px] transition-colors ${
              filter === item.key
                ? 'border-primary/40 bg-primary/10 text-primary'
                : 'border-border bg-surface text-text-muted hover:text-text'
            }`}
          >
            {item.label}
          </button>
        ))}
      </div>
      {filtered.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border bg-surface px-3 py-8 text-center text-xs text-text-muted">
          {assets.length === 0 ? 'No media yet.' : 'No media matches this filter.'}
        </div>
      ) : view === 'grid' ? (
        <div className="grid grid-cols-2 gap-2">
          {filtered.map((asset) => (
            <div
              key={asset.id}
              role="button"
              tabIndex={0}
              draggable
              onDragStart={(event) => {
                event.dataTransfer.setData('application/x-video-asset-id', asset.id);
                event.dataTransfer.effectAllowed = 'copy';
              }}
              onClick={() => onSelect(asset.id)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  onSelect(asset.id);
                }
              }}
              className={`cursor-grab overflow-hidden rounded-lg border active:cursor-grabbing ${
                asset.id === activeAssetId ? 'border-primary/30 bg-primary/10' : 'border-border bg-surface'
              }`}
            >
              <AssetThumbnail asset={asset} className="h-16 w-full bg-black/40" />
              <div className="p-2">
                {nameOrEditor(asset)}
                <p className="mt-1 truncate text-[10px] text-text-muted">{sourceLine(asset)}</p>
                <div className="mt-1.5 flex gap-1">{actionButtons(asset)}</div>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="space-y-2">
          {filtered.map((asset) => (
            <div
              key={asset.id}
              role="button"
              tabIndex={0}
              draggable
              onDragStart={(event) => {
                event.dataTransfer.setData('application/x-video-asset-id', asset.id);
                event.dataTransfer.effectAllowed = 'copy';
              }}
              onClick={() => onSelect(asset.id)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  onSelect(asset.id);
                }
              }}
              className={`flex cursor-grab items-start gap-2 rounded-lg border p-2 active:cursor-grabbing ${
                asset.id === activeAssetId ? 'border-primary/30 bg-primary/10' : 'border-border bg-surface'
              }`}
            >
              <AssetThumbnail asset={asset} className="h-12 w-16 shrink-0 rounded-md bg-black/40" />
              <div className="min-w-0 flex-1">
                {nameOrEditor(asset)}
                <p className="mt-1 truncate text-[10px] text-text-muted">{sourceLine(asset)}</p>
                <div className="mt-1.5 flex gap-1">{actionButtons(asset)}</div>
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
