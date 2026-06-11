import { useEffect, useMemo, useRef, useState } from 'react';
import { DragHandle, useResizablePanels } from '../ResizablePanels';
import { Check, ChevronLeft, ChevronRight, Download, Film, LayoutGrid, LayoutTemplate, List, Loader2, Music2, Pencil, Plus, Scissors, Trash2, Upload, X } from 'lucide-react';
import { toast } from 'sonner';
import { api, videoApi } from '../../api';
import { useConversationStore, useSettingsStore } from '../../stores';
import { useVideoStudioStore } from '../../stores/videoStudio';
import { ContextMenu } from '../common/ContextMenu';
import type { ContextMenuEntry } from '../common/ContextMenu';
import type { VideoAsset } from '../../types/video';
import { VideoCaptionPanel } from './VideoCaptionPanel';
import { VideoInspector } from './VideoInspector';
import { VideoPreviewCanvas } from './VideoPreviewCanvas';
import { VideoRenderPanel } from './VideoRenderPanel';
import { VideoTimeline } from './timeline/VideoTimeline';
import { TIMELINE_TEMPLATES } from './templates/timelineTemplates';
import { EDITOR_MODES, editorModeFeatures } from './editorModes';
import type { EditorModeKey } from './editorModes';

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
  const uploadAsset = useVideoStudioStore((state) => state.uploadAsset);
  const createProjectFromTemplate = useVideoStudioStore((state) => state.createProjectFromTemplate);
  const loadRendererCapabilities = useVideoStudioStore((state) => state.loadRendererCapabilities);
  const editorMode = useVideoStudioStore((state) => state.editorMode);
  const setEditorMode = useVideoStudioStore((state) => state.setEditorMode);
  const setAppMode = useSettingsStore((state) => state.setAppMode);
  const modeFeatures = editorModeFeatures(editorMode);
  const [templatesOpen, setTemplatesOpen] = useState(false);
  const templatesRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!templatesOpen) return;
    const onPointerDown = (event: PointerEvent) => {
      if (!templatesRef.current?.contains(event.target as Node)) setTemplatesOpen(false);
    };
    window.addEventListener('pointerdown', onPointerDown);
    return () => window.removeEventListener('pointerdown', onPointerDown);
  }, [templatesOpen]);

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

  const { leftStyle, rightStyle, startLeft, startRight } = useResizablePanels({
    defaultLeft: 300,
    defaultRight: 340,
    storageKey: 'video-edit-studio-panels',
  });
  const [collapsedLeft, setCollapsedLeft] = useState(false);
  const [collapsedRight, setCollapsedRight] = useState(false);
  const isSavingTimeline = useVideoStudioStore((state) => state.isSavingTimeline);

  return (
    <div className="flex min-h-0 flex-1 flex-col bg-surface">
      <div className="flex flex-col gap-2 border-b border-border bg-surface-raised px-3 py-2 sm:flex-row sm:items-center sm:justify-between sm:px-4">
        <div className="flex min-w-0 items-center gap-3">
          <Scissors size={18} className="text-primary" />
          <span className="shrink-0 text-sm font-medium text-text">Video Edit Studio</span>
          {activeProject && (
            <span className="min-w-0 truncate text-xs text-text-muted">- {activeProject.title}</span>
          )}
          <span
            className={`shrink-0 rounded px-1.5 py-0.5 text-[10px] transition-opacity ${
              isSavingTimeline ? 'bg-primary/15 text-primary opacity-100' : 'text-text-muted opacity-60'
            }`}
            title={isSavingTimeline ? 'Saving timeline changes' : 'All timeline changes are saved'}
          >
            {isSavingTimeline ? 'Saving…' : 'Saved'}
          </span>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <select
            value={editorMode}
            onChange={(event) => setEditorMode(event.target.value as EditorModeKey)}
            className="min-h-9 rounded-lg border border-border bg-surface-alt px-2 text-xs text-text-secondary hover:text-text"
            aria-label="Editor mode"
            title={EDITOR_MODES.find((mode) => mode.key === editorMode)?.description}
          >
            {EDITOR_MODES.map((mode) => (
              <option key={mode.key} value={mode.key}>{mode.label}</option>
            ))}
          </select>
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
          {modeFeatures.templates && (
          <div ref={templatesRef} className="relative">
            <button
              onClick={() => setTemplatesOpen((open) => !open)}
              className="min-h-9 rounded-lg border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:bg-surface-hover hover:text-text transition-colors inline-flex items-center gap-1.5"
              title="Create a new project from a starter template"
            >
              <LayoutTemplate size={13} />
              Templates
            </button>
            {templatesOpen && (
              <div className="absolute right-0 top-full z-40 mt-1 w-64 rounded-md border border-border bg-surface p-1 shadow-xl">
                {TIMELINE_TEMPLATES.map((template) => (
                  <button
                    key={template.key}
                    className="block w-full rounded px-2 py-1.5 text-left hover:bg-surface-alt"
                    onClick={() => {
                      setTemplatesOpen(false);
                      void createProjectFromTemplate(template.key);
                    }}
                  >
                    <span className="block text-xs font-medium text-text">{template.label}</span>
                    <span className="block text-[10px] text-text-muted">{template.description}</span>
                  </button>
                ))}
              </div>
            )}
          </div>
          )}
          {modeFeatures.addTextClip && (
          <button
            onClick={() => { void addTextClip(); }}
            disabled={!timeline}
            className="min-h-9 rounded-lg border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:bg-surface-hover hover:text-text disabled:cursor-not-allowed disabled:opacity-45 transition-colors"
          >
            Text
          </button>
          )}
        </div>
      </div>

      <div className="flex min-h-0 flex-1 flex-col xl:flex-row">
        {collapsedLeft ? (
          <button
            onClick={() => setCollapsedLeft(false)}
            className="hidden w-6 shrink-0 items-center justify-center border-r border-border bg-surface text-text-muted hover:text-text xl:flex"
            title="Expand media panel"
            aria-label="Expand media panel"
          >
            <ChevronRight size={14} />
          </button>
        ) : (
          <aside className="relative min-h-0 overflow-y-auto border-b border-border bg-surface xl:border-b-0 xl:border-r" style={leftStyle}>
            <button
              onClick={() => setCollapsedLeft(true)}
              className="absolute right-1 top-1 z-10 hidden rounded p-1 text-text-muted hover:text-text xl:block"
              title="Collapse media panel"
              aria-label="Collapse media panel"
            >
              <ChevronLeft size={13} />
            </button>
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
                onUpload={uploadAsset}
              />
            </div>
          </aside>
        )}

        {!collapsedLeft && <DragHandle onMouseDown={startLeft} />}

        <main className="flex min-h-[620px] min-w-0 flex-1 flex-col bg-surface">
          <section className="min-h-0 flex-1 border-b border-border p-3">
            <VideoPreviewCanvas />
          </section>
          <section className="h-72 shrink-0 border-b border-border bg-surface-raised p-3">
            <VideoTimeline />
          </section>
        </main>

        {!collapsedRight && <DragHandle onMouseDown={startRight} />}

        {collapsedRight ? (
          <button
            onClick={() => setCollapsedRight(false)}
            className="hidden w-6 shrink-0 items-center justify-center border-l border-border bg-surface-raised text-text-muted hover:text-text xl:flex"
            title="Expand inspector panel"
            aria-label="Expand inspector panel"
          >
            <ChevronLeft size={14} />
          </button>
        ) : (
          <aside className="relative min-h-0 overflow-y-auto border-t border-border bg-surface-raised xl:border-l xl:border-t-0" style={rightStyle}>
            <button
              onClick={() => setCollapsedRight(true)}
              className="absolute left-1 top-1 z-10 hidden rounded p-1 text-text-muted hover:text-text xl:block"
              title="Collapse inspector panel"
              aria-label="Collapse inspector panel"
            >
              <ChevronRight size={13} />
            </button>
            <VideoInspector />
            {modeFeatures.captionsPanel && (
              <div className="px-3 pb-3">
                <VideoCaptionPanel />
              </div>
            )}
            <div className="px-3 pb-3">
              <VideoRenderPanel />
            </div>
          </aside>
        )}
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
  const [menu, setMenu] = useState<{ projectId: string; x: number; y: number } | null>(null);
  const duplicateProject = useVideoStudioStore((state) => state.duplicateProject);
  const deleteProject = useVideoStudioStore((state) => state.deleteProject);
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
              onContextMenu={(event) => {
                event.preventDefault();
                setMenu({ projectId: project.id, x: event.clientX, y: event.clientY });
              }}
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
      {menu && (() => {
        const project = projects.find((item) => item.id === menu.projectId);
        if (!project) return null;
        const items: ContextMenuEntry[] = [
          { label: 'Open project', action: () => onSelect(project.id) },
          { label: 'Duplicate project (with assets)', action: () => { void duplicateProject(project.id); } },
          'divider',
          {
            label: 'Delete project',
            danger: true,
            action: () => {
              if (window.confirm(`Delete "${project.title}" and all of its assets?`)) void deleteProject(project.id);
            },
          },
        ];
        return <ContextMenu position={{ x: menu.x, y: menu.y }} items={items} onClose={() => setMenu(null)} />;
      })()}
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

function formatBytes(bytes: number): string {
  if (bytes >= 1 << 30) return `${(bytes / (1 << 30)).toFixed(1)} GB`;
  if (bytes >= 1 << 20) return `${(bytes / (1 << 20)).toFixed(1)} MB`;
  return `${Math.max(1, Math.round(bytes / 1024))} KB`;
}

function assetMatchesFilter(asset: VideoAsset, filter: AssetFilterKey): boolean {
  if (filter === 'all') return true;
  if (filter === 'audio') return asset.kind === 'audio' || asset.kind === 'music';
  return asset.kind === filter;
}

function AssetThumbnail({ asset, className }: { asset: VideoAsset; className: string }) {
  const url = videoApi.downloadUrl(asset.id);
  // Server-generated artifacts are cheap static images; fall back to loading
  // the real media when they haven't been generated (e.g. FFmpeg missing).
  if (asset.thumbnail_path) {
    return <img src={videoApi.artifactUrl(asset.id, 'thumbnail')} alt={asset.file_name} loading="lazy" className={`${className} object-cover`} />;
  }
  if (asset.waveform_path) {
    return <img src={videoApi.artifactUrl(asset.id, 'waveform')} alt={`${asset.file_name} waveform`} loading="lazy" className={`${className} object-cover`} />;
  }
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
  onUpload,
}: {
  assets: VideoAsset[];
  activeAssetId: string | null;
  onSelect: (assetId: string) => void;
  onAdd: (assetId: string) => void;
  onRename: (assetId: string, fileName: string) => void;
  onDelete: (assetId: string) => void;
  onUpload: (file: File) => Promise<void>;
}) {
  const [view, setView] = useState<'list' | 'grid'>('list');
  const [filter, setFilter] = useState<AssetFilterKey>('all');
  const [query, setQuery] = useState('');
  const [sortKey, setSortKey] = useState<'newest' | 'name' | 'duration' | 'size'>('newest');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingName, setEditingName] = useState('');
  const [uploading, setUploading] = useState(false);
  const [dragOver, setDragOver] = useState(false);
  const [menu, setMenu] = useState<{ assetId: string; x: number; y: number } | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const timeline = useVideoStudioStore((state) => state.timeline);
  const addAssetToTimeline = useVideoStudioStore((state) => state.addAssetToTimeline);
  const addAssetAsMusicBed = useVideoStudioStore((state) => state.addAssetAsMusicBed);
  const addTrack = useVideoStudioStore((state) => state.addTrack);
  const setAppMode = useSettingsStore((state) => state.setAppMode);
  const createConversation = useConversationStore((state) => state.createConversation);

  // Clip counts per asset id, for the in-use badge and delete warnings.
  const usedCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const track of timeline?.tracks || []) {
      for (const clip of track.clips) {
        if (clip.asset_id) counts.set(clip.asset_id, (counts.get(clip.asset_id) || 0) + 1);
      }
    }
    return counts;
  }, [timeline]);

  const sendAssetToChat = async (asset: VideoAsset) => {
    try {
      const convo = await createConversation(`Video asset: ${asset.file_name}`);
      const attachment = await videoApi.attachAssetToConversation(asset.id, convo.id);
      await api.sendMessage(convo.id, {
        content: `Video Edit Studio asset: ${asset.file_name}\n/v1/attachments/${attachment.id}/download`,
        no_reply: true,
      });
      setAppMode('chat');
      toast.success('Asset sent to chat');
    } catch (err) {
      toast.error(`Failed to send to chat: ${(err as Error).message}`);
    }
  };

  const registerInLibrary = async (asset: VideoAsset) => {
    try {
      const file = await videoApi.registerAssetInLibrary(asset.id);
      toast.success(`Registered ${file.display_name || asset.file_name} in File Library`);
    } catch (err) {
      toast.error(`Failed to register in File Library: ${(err as Error).message}`);
    }
  };

  const handleFiles = async (list: FileList | null) => {
    if (!list || list.length === 0) return;
    setUploading(true);
    try {
      for (const file of Array.from(list)) {
        await onUpload(file);
      }
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const filtered = assets
    .filter((asset) => assetMatchesFilter(asset, filter))
    .filter((asset) => !query.trim() || asset.file_name.toLowerCase().includes(query.trim().toLowerCase()))
    .sort((a, b) => {
      switch (sortKey) {
        case 'name':
          return a.file_name.localeCompare(b.file_name);
        case 'duration':
          return (b.duration_ms || 0) - (a.duration_ms || 0);
        case 'size':
          return b.size_bytes - a.size_bytes;
        default:
          return b.created_at.localeCompare(a.created_at);
      }
    });

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
    const used = usedCounts.get(asset.id);
    if (used) bits.push(`in use ×${used}`);
    return bits.filter(Boolean).join(' · ');
  };

  const openAssetMenu = (event: React.MouseEvent, asset: VideoAsset) => {
    event.preventDefault();
    event.stopPropagation();
    onSelect(asset.id);
    setMenu({ assetId: asset.id, x: event.clientX, y: event.clientY });
  };

  return (
    <section
      className={`rounded-lg border p-3 transition-colors ${dragOver ? 'border-primary/60 bg-primary/5' : 'border-border bg-surface-alt'}`}
      onDragOver={(event) => {
        if (event.dataTransfer.types.includes('Files')) {
          event.preventDefault();
          setDragOver(true);
        }
      }}
      onDragLeave={() => setDragOver(false)}
      onDrop={(event) => {
        if (event.dataTransfer.types.includes('Files')) {
          event.preventDefault();
          setDragOver(false);
          void handleFiles(event.dataTransfer.files);
        }
      }}
    >
      <div className="mb-2 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-text">Media Bin</h2>
        <div className="flex items-center gap-1">
          <span
            className="text-[11px] text-text-muted"
            title={`${assets.length} asset${assets.length === 1 ? '' : 's'} using ${formatBytes(assets.reduce((sum, asset) => sum + asset.size_bytes, 0))} of storage`}
          >
            {filtered.length} · {formatBytes(assets.reduce((sum, asset) => sum + asset.size_bytes, 0))}
          </span>
          <input
            ref={fileInputRef}
            type="file"
            multiple
            accept="video/*,image/*,audio/*"
            className="hidden"
            onChange={(event) => { void handleFiles(event.target.files); }}
          />
          <button
            onClick={() => fileInputRef.current?.click()}
            disabled={uploading}
            className="rounded-md border border-border bg-surface p-1.5 text-text-muted hover:text-text disabled:cursor-not-allowed disabled:opacity-45"
            title="Upload local media (video, image, audio)"
            aria-label="Upload local media"
          >
            {uploading ? <Loader2 size={12} className="animate-spin" /> : <Upload size={12} />}
          </button>
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
      <div className="mb-2 flex items-center gap-1.5">
        <input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Search media..."
          className="min-h-8 min-w-0 flex-1 rounded-md border border-border bg-surface px-2 text-xs text-text focus:border-primary/50 focus:outline-none"
          aria-label="Search media bin"
        />
        <select
          value={sortKey}
          onChange={(event) => setSortKey(event.target.value as typeof sortKey)}
          className="min-h-8 rounded-md border border-border bg-surface px-1.5 text-[11px] text-text-secondary"
          aria-label="Sort media bin"
          title="Sort media"
        >
          <option value="newest">Newest</option>
          <option value="name">Name</option>
          <option value="duration">Duration</option>
          <option value="size">Size</option>
        </select>
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
              onContextMenu={(event) => openAssetMenu(event, asset)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  onSelect(asset.id);
                } else if ((event.key === 'F10' && event.shiftKey) || event.key === 'ContextMenu') {
                  event.preventDefault();
                  const rect = event.currentTarget.getBoundingClientRect();
                  onSelect(asset.id);
                  setMenu({ assetId: asset.id, x: rect.left + rect.width / 2, y: rect.bottom });
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
              onContextMenu={(event) => openAssetMenu(event, asset)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  onSelect(asset.id);
                } else if ((event.key === 'F10' && event.shiftKey) || event.key === 'ContextMenu') {
                  event.preventDefault();
                  const rect = event.currentTarget.getBoundingClientRect();
                  onSelect(asset.id);
                  setMenu({ assetId: asset.id, x: rect.left + rect.width / 2, y: rect.bottom });
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
      {menu && (() => {
        const asset = assets.find((item) => item.id === menu.assetId);
        if (!asset) return null;
        const used = usedCounts.get(asset.id) || 0;
        const items: ContextMenuEntry[] = [
          { label: 'Add to timeline at playhead', action: () => onAdd(asset.id) },
          {
            label: 'Add to new layer',
            action: () => {
              void (async () => {
                await addTrack('layer');
                const trackId = useVideoStudioStore.getState().selectedTrackId;
                await addAssetToTimeline(asset.id, trackId ? { track_id: trackId } : undefined);
              })();
            },
          },
          ...(asset.kind === 'audio' || asset.kind === 'music'
            ? [{ label: 'Add as music bed (full length)', action: () => { void addAssetAsMusicBed(asset.id); } }]
            : []),
          'divider',
          { label: 'Rename', action: () => { setEditingId(asset.id); setEditingName(asset.file_name); } },
          { label: 'Download', action: () => { window.open(videoApi.downloadUrl(asset.id), '_blank', 'noopener,noreferrer'); } },
          { label: 'Send to Chat', action: () => { void sendAssetToChat(asset); } },
          { label: 'Register in File Library', action: () => { void registerInLibrary(asset); } },
          'divider',
          {
            label: used > 0 ? `Delete (used by ${used} clip${used === 1 ? '' : 's'})` : 'Delete',
            danger: true,
            action: () => {
              const warning = used > 0
                ? `Delete "${asset.file_name}"? ${used} clip${used === 1 ? '' : 's'} in the timeline use${used === 1 ? 's' : ''} it and will stop rendering.`
                : `Delete "${asset.file_name}"?`;
              if (window.confirm(warning)) onDelete(asset.id);
            },
          },
        ];
        return <ContextMenu position={{ x: menu.x, y: menu.y }} items={items} onClose={() => setMenu(null)} />;
      })()}
    </section>
  );
}
