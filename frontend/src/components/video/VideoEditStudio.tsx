/**
 * Video Edit Studio shell: header (editor modes, templates, text, record),
 * project strip + project media/favorites bin on the left, preview canvas over
 * the timeline in the center, and a mode-gated right rail for properties,
 * effects, transitions, captions, audio, assistant, and export. The footer
 * owns the preview-only master volume; project export is unaffected by it.
 */
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { DragHandle, useResizablePanels } from '../ResizablePanels';
import {
  AudioLines,
  Bot,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronsLeftRight,
  Circle,
  Download,
  Film,
  Files,
  Filter,
  LayoutGrid,
  LayoutTemplate,
  List,
  Loader2,
  Music2,
  Pencil,
  Plus,
  Scissors,
  Settings,
  Sparkles,
  Square,
  Star,
  Subtitles,
  Trash2,
  Type,
  Upload,
  X,
} from 'lucide-react';
import { toast } from 'sonner';
import { api, videoApi } from '../../api';
import { useConversationStore, useSettingsStore } from '../../stores';
import { useVideoStudioStore } from '../../stores/videoStudio';
import { ContextMenu } from '../common/ContextMenu';
import type { ContextMenuEntry } from '../common/ContextMenu';
import { ConfirmDialog } from '../common/AppDialog';
import type { VideoAsset, VideoTimelineDocument } from '../../types/video';
import { RecordingModal } from './RecordingModal';
import { VideoCaptionPanel } from './VideoCaptionPanel';
import { VideoInspector } from './VideoInspector';
import { VideoPreviewCanvas } from './VideoPreviewCanvas';
import { VideoRenderPanel } from './VideoRenderPanel';
import { VideoTimeline } from './timeline/VideoTimeline';
import { TIMELINE_TEMPLATES } from './templates/timelineTemplates';
import { EDITOR_MODES, editorModeFeatures } from './editorModes';
import type { EditorModeKey } from './editorModes';
import type { LucideIcon } from 'lucide-react';

type InspectorRailTab = 'properties' | 'effects' | 'transitions' | 'captions' | 'audio' | 'assistant' | 'export';

function formatStatusTime(ms: number, fps = 30): string {
  const total = Math.max(0, Math.round(ms));
  const hours = Math.floor(total / 3_600_000);
  const minutes = Math.floor((total % 3_600_000) / 60_000);
  const seconds = Math.floor((total % 60_000) / 1000);
  const frames = Math.floor(((total % 1000) / 1000) * Math.max(1, fps));
  return `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}:${String(frames).padStart(2, '0')}`;
}

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
  const addShapeClip = useVideoStudioStore((state) => state.addShapeClip);
  const renameAsset = useVideoStudioStore((state) => state.renameAsset);
  const deleteAsset = useVideoStudioStore((state) => state.deleteAsset);
  const uploadAsset = useVideoStudioStore((state) => state.uploadAsset);
  const createProjectFromTemplate = useVideoStudioStore((state) => state.createProjectFromTemplate);
  const loadRendererCapabilities = useVideoStudioStore((state) => state.loadRendererCapabilities);
  const undoTimeline = useVideoStudioStore((state) => state.undoTimeline);
  const redoTimeline = useVideoStudioStore((state) => state.redoTimeline);
  const canUndo = useVideoStudioStore((state) => state.timelineUndoStack.length > 0);
  const canRedo = useVideoStudioStore((state) => state.timelineRedoStack.length > 0);
  const snappingEnabled = useVideoStudioStore((state) => state.snappingEnabled);
  const editorMode = useVideoStudioStore((state) => state.editorMode);
  const setEditorMode = useVideoStudioStore((state) => state.setEditorMode);
  const setAppMode = useSettingsStore((state) => state.setAppMode);
  const toggleSettings = useSettingsStore((state) => state.toggleSettings);
  const modeFeatures = editorModeFeatures(editorMode);
  const [templatesOpen, setTemplatesOpen] = useState(false);
  const [recordOpen, setRecordOpen] = useState(false);
  const [railTab, setRailTab] = useState<InspectorRailTab>('properties');
  const templatesRef = useRef<HTMLDivElement | null>(null);

  // The right rail tabs follow the editor mode's feature gates.
  const railTabs = [
    { key: 'properties' as const, label: 'Properties', enabled: true },
    { key: 'effects' as const, label: 'Effects', enabled: modeFeatures.effectControls },
    { key: 'transitions' as const, label: 'Transitions', enabled: modeFeatures.effectControls },
    { key: 'captions' as const, label: 'Captions', enabled: modeFeatures.captionsPanel },
    { key: 'audio' as const, label: 'Audio', enabled: true },
    { key: 'assistant' as const, label: 'AI Assistant', enabled: modeFeatures.assistant },
    { key: 'export' as const, label: 'Export', enabled: true },
  ].filter((tab) => tab.enabled);
  const activeRailTab = railTabs.some((tab) => tab.key === railTab) ? railTab : 'properties';

  const openFileLibrary = () => {
    window.dispatchEvent(new CustomEvent('omnillm:open-file-library', { detail: { preferredScope: 'all' } }));
  };

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

  const { leftStyle, rightStyle, startLeft, startRight, resizeLeft, resizeRight } = useResizablePanels({
    defaultLeft: 320,
    defaultRight: 340,
    minWidth: 260,
    storageKey: 'video-edit-studio-panels-v3',
  });
  const [collapsedLeft, setCollapsedLeft] = useState(false);
  const [collapsedRight, setCollapsedRight] = useState(false);
  const [mobilePanel, setMobilePanel] = useState<'preview' | 'timeline' | 'media' | 'inspector'>('preview');
  const isSavingTimeline = useVideoStudioStore((state) => state.isSavingTimeline);
  const saveStatus = useVideoStudioStore((state) => state.saveStatus);
  const saveError = useVideoStudioStore((state) => state.saveError);
  const saveTimeline = useVideoStudioStore((state) => state.saveTimeline);

  return (
    <div className="video-edit-shell flex min-h-0 flex-1 flex-col bg-surface text-text">
      <div className="border-b border-border bg-surface-raised/95 px-3 py-2 shadow-[0_1px_0_rgba(255,255,255,0.03)]">
        <div className="flex flex-col gap-2 xl:flex-row xl:items-center xl:justify-between">
          <div className="flex min-w-0 items-center gap-2">
            <Scissors size={17} className="text-primary" />
            <span className="shrink-0 text-sm font-semibold text-text">Video Edit Studio</span>
            {activeProject && (
              <>
                <span className="text-xs text-text-muted">·</span>
                <span className="min-w-0 truncate text-xs text-text-muted">{activeProject.title}</span>
                <span className="hidden text-xs text-text-muted sm:inline">· {new Date(activeProject.updated_at).toLocaleString()}</span>
              </>
            )}
            <span className="text-xs text-text-muted">·</span>
            <button
              type="button"
              onClick={saveStatus === 'error' ? () => { void saveTimeline(); } : undefined}
              disabled={saveStatus !== 'error'}
              className={`shrink-0 rounded px-1.5 py-0.5 text-[10px] transition-opacity ${
                isSavingTimeline
                  ? 'bg-primary/15 text-primary opacity-100'
                  : saveStatus === 'error'
                    ? 'cursor-pointer bg-red-400/10 text-red-300 hover:bg-red-400/15'
                    : 'text-text-muted opacity-80'
              }`}
              title={isSavingTimeline
                ? 'Saving timeline changes'
                : saveStatus === 'error'
                  ? `${saveError || 'Timeline save failed'} — click to retry`
                  : saveStatus === 'idle'
                    ? 'Timeline has not been saved yet'
                    : 'All timeline changes are saved'}
            >
              {isSavingTimeline ? 'Saving…' : saveStatus === 'error' ? 'Save failed · Retry' : saveStatus === 'idle' ? 'Not saved' : 'Saved'}
            </button>
          </div>
          <div className="hidden flex-wrap items-center gap-1.5 xl:flex">
            <button
              onClick={() => setRailTab('export')}
              disabled={!timeline}
              className="inline-flex min-h-8 items-center gap-1.5 rounded-md bg-primary px-3 text-xs font-semibold text-black shadow-sm hover:bg-primary-hover disabled:cursor-not-allowed disabled:opacity-45"
              title="Open export settings"
            >
              <Download size={13} />
              Export
              <ChevronDown size={12} />
            </button>
            <HeaderIconButton label="Undo" disabled={!canUndo} onClick={() => { void undoTimeline(); }}>
              <ChevronLeft size={13} />
            </HeaderIconButton>
            <HeaderIconButton label="Redo" disabled={!canRedo} onClick={() => { void redoTimeline(); }}>
              <ChevronRight size={13} />
            </HeaderIconButton>
            <select
              value={editorMode}
              onChange={(event) => setEditorMode(event.target.value as EditorModeKey)}
              className="min-h-8 rounded-md border border-border bg-surface-alt px-2 text-xs text-text-secondary hover:text-text"
              aria-label="Editor mode"
              title={EDITOR_MODES.find((mode) => mode.key === editorMode)?.description}
            >
              {EDITOR_MODES.map((mode) => (
                <option key={mode.key} value={mode.key}>{mode.label}</option>
              ))}
            </select>
            <HeaderActionButton onClick={() => setAppMode('video')} icon={Film} label="Create" />
            <HeaderActionButton onClick={() => { void createProject(); }} icon={Plus} label="Project" />
            {modeFeatures.templates && (
            <div ref={templatesRef} className="relative">
              <HeaderActionButton
                onClick={() => setTemplatesOpen((open) => !open)}
                icon={LayoutTemplate}
                label="Templates"
                title="Create a new project from a starter template"
              />
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
              <HeaderActionButton onClick={() => { void addTextClip(); }} icon={Type} label="Text" disabled={!timeline} />
            )}
            <button
              onClick={() => setRecordOpen(true)}
              disabled={!activeProjectId}
              className="inline-flex min-h-8 items-center gap-1.5 rounded-md border border-border bg-surface-alt px-2.5 text-xs text-text-secondary transition-colors hover:bg-surface-hover hover:text-text disabled:cursor-not-allowed disabled:opacity-45"
              title={activeProjectId ? 'Record screen, camera, or voiceover into this project' : 'Select or create a project first'}
            >
              <Circle size={10} className="text-red-400" fill="currentColor" />
              Record
            </button>
          </div>
        </div>
        <div className="mt-2 grid grid-cols-4 gap-1 xl:hidden" aria-label="Video editor quick actions">
          <button type="button" onClick={() => setAppMode('video')} className="min-h-11 rounded-lg border border-border bg-surface-alt text-xs text-text-secondary">Create</button>
          <button type="button" onClick={() => { void undoTimeline(); }} disabled={!canUndo} className="min-h-11 rounded-lg border border-border bg-surface-alt text-xs text-text-secondary disabled:opacity-40">Undo</button>
          <button type="button" onClick={() => { void redoTimeline(); }} disabled={!canRedo} className="min-h-11 rounded-lg border border-border bg-surface-alt text-xs text-text-secondary disabled:opacity-40">Redo</button>
          <button type="button" onClick={() => { setRailTab('export'); setCollapsedRight(false); setMobilePanel('inspector'); }} disabled={!timeline} className="min-h-11 rounded-lg bg-primary text-xs font-semibold text-black disabled:opacity-40">Export</button>
        </div>
      </div>
      {recordOpen && <RecordingModal onClose={() => setRecordOpen(false)} />}

      <div className="grid grid-cols-4 border-b border-border bg-surface-raised p-1 xl:hidden" role="tablist" aria-label="Video Edit Studio workspace">
        {(['preview', 'timeline', 'media', 'inspector'] as const).map((panel) => (
          <button
            key={panel}
            type="button"
            role="tab"
            aria-selected={mobilePanel === panel}
            onClick={() => {
              if (panel === 'media') setCollapsedLeft(false);
              if (panel === 'inspector') setCollapsedRight(false);
              setMobilePanel(panel);
            }}
            className={`min-h-11 rounded-lg px-1 text-[11px] font-medium capitalize transition-colors ${mobilePanel === panel ? 'bg-primary/15 text-primary' : 'text-text-muted hover:bg-surface-hover hover:text-text'}`}
          >
            {panel.charAt(0).toUpperCase() + panel.slice(1)}
          </button>
        ))}
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
          <aside className={`video-edit-panel relative min-h-0 w-full flex-1 border-b border-border bg-surface xl:w-auto xl:flex-none xl:border-b-0 xl:border-r ${mobilePanel === 'media' ? 'flex' : 'hidden xl:flex'}`} style={leftStyle}>
            <button
              onClick={() => setCollapsedLeft(true)}
              className="absolute right-1 top-1 z-10 hidden rounded p-1 text-text-muted hover:text-text xl:block"
              title="Collapse media panel"
              aria-label="Collapse media panel"
            >
              <ChevronLeft size={13} />
            </button>
            <div className="hidden xl:block">
            <StudioToolRail
              activeTab={activeRailTab}
              templatesEnabled={modeFeatures.templates}
              textEnabled={modeFeatures.addTextClip && Boolean(timeline)}
              effectsEnabled={modeFeatures.effectControls}
              captionsEnabled={modeFeatures.captionsPanel}
              assistantEnabled={modeFeatures.assistant}
              onMedia={() => setCollapsedLeft(false)}
              onLibrary={openFileLibrary}
              onTemplates={() => setTemplatesOpen((open) => !open)}
              onText={() => { void addTextClip(); }}
              onShapes={() => { void addShapeClip('rectangle'); setRailTab('properties'); }}
              onEffects={() => setRailTab('effects')}
              onTransitions={() => setRailTab('transitions')}
              onCaptions={() => setRailTab('captions')}
              onAudio={() => setRailTab('audio')}
              onAssistant={() => setRailTab('assistant')}
              onSettings={toggleSettings}
            />
            </div>
            <div className="min-w-0 flex-1 overflow-y-auto">
              <div className="space-y-3 p-3">
              <AssetPanel
                assets={assets}
                activeAssetId={activeAsset?.id || null}
                onSelect={selectAsset}
                onAdd={(assetId) => { void addAssetToTimeline(assetId); }}
                onRename={(assetId, name) => { void renameAsset(assetId, name); }}
                onDelete={(assetId) => { void deleteAsset(assetId); }}
                onUpload={uploadAsset}
                onOpenLibrary={openFileLibrary}
              />
              <ProjectStrip
                projects={projects}
                activeProjectId={activeProjectId}
                isLoading={isLoading}
                onNew={() => { void createProject(); }}
                onSelect={(id) => { void selectProject(id); }}
              />
              </div>
            </div>
          </aside>
        )}

        {!collapsedLeft && <DragHandle onMouseDown={startLeft} onKeyboardResize={resizeLeft} />}

        <main className={`${mobilePanel === 'preview' || mobilePanel === 'timeline' ? 'flex' : 'hidden xl:flex'} min-h-0 min-w-0 flex-1 flex-col bg-surface xl:min-h-[620px]`}>
          <section className={`${mobilePanel === 'preview' ? 'block' : 'hidden'} min-h-0 flex-1 border-b border-border p-3 xl:block`}>
            <VideoPreviewCanvas />
          </section>
          <section className={`${mobilePanel === 'timeline' ? 'block' : 'hidden'} min-h-0 flex-1 border-b border-border bg-surface-raised p-2 xl:block xl:h-[22rem] xl:flex-none`}>
            <VideoTimeline />
          </section>
        </main>

        {!collapsedRight && <DragHandle onMouseDown={startRight} onKeyboardResize={resizeRight} />}

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
          <aside className={`video-edit-panel relative min-h-0 w-full flex-1 flex-col border-t border-border bg-surface-raised xl:w-auto xl:flex-none xl:border-l xl:border-t-0 ${mobilePanel === 'inspector' ? 'flex' : 'hidden xl:flex'}`} style={rightStyle}>
            <div className="flex items-center gap-0.5 overflow-x-auto border-b border-border px-1 pt-1.5" role="tablist" aria-label="Inspector panels">
              <button
                onClick={() => setCollapsedRight(true)}
                className="hidden shrink-0 rounded p-0.5 text-text-muted hover:text-text xl:block"
                title="Collapse inspector panel"
                aria-label="Collapse inspector panel"
              >
                <ChevronRight size={12} />
              </button>
              {railTabs.map((tab) => (
                <button
                  key={tab.key}
                  role="tab"
                  aria-selected={activeRailTab === tab.key}
                  onClick={() => setRailTab(tab.key)}
                  className={`shrink-0 rounded-t-md border-b-2 px-1.5 py-1.5 text-[10px] transition-colors ${
                    activeRailTab === tab.key
                      ? 'border-primary font-medium text-text'
                      : 'border-transparent text-text-muted hover:text-text'
                  }`}
                >
                  {tab.label}
                </button>
              ))}
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto">
              {activeRailTab === 'properties' && <VideoInspector section="properties" focus="properties" />}
              {activeRailTab === 'effects' && modeFeatures.effectControls && <VideoInspector section="properties" focus="effects" />}
              {activeRailTab === 'transitions' && modeFeatures.effectControls && <VideoInspector section="properties" focus="transitions" />}
              {activeRailTab === 'audio' && <VideoInspector section="properties" focus="audio" />}
              {activeRailTab === 'assistant' && <VideoInspector section="assistant" />}
              {activeRailTab === 'captions' && modeFeatures.captionsPanel && (
                <div className="p-3">
                  <VideoCaptionPanel />
                </div>
              )}
              {activeRailTab === 'export' && (
                <div className="p-3">
                  <VideoRenderPanel />
                </div>
              )}
            </div>
          </aside>
        )}
      </div>
      <div className="hidden xl:block">
        <StudioStatusBar timeline={timeline} snappingEnabled={snappingEnabled} />
      </div>
    </div>
  );
}

function HeaderActionButton({
  icon: Icon,
  label,
  title,
  disabled = false,
  onClick,
}: {
  icon: LucideIcon;
  label: string;
  title?: string;
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className="inline-flex min-h-8 items-center gap-1.5 rounded-md border border-border bg-surface-alt px-2.5 text-xs text-text-secondary transition-colors hover:bg-surface-hover hover:text-text disabled:cursor-not-allowed disabled:opacity-45"
      title={title || label}
    >
      <Icon size={13} />
      {label}
    </button>
  );
}

function HeaderIconButton({
  label,
  disabled = false,
  onClick,
  children,
}: {
  label: string;
  disabled?: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-border bg-surface-alt text-text-muted transition-colors hover:bg-surface-hover hover:text-text disabled:cursor-not-allowed disabled:opacity-35"
      title={label}
      aria-label={label}
    >
      {children}
    </button>
  );
}

function StudioToolRail({
  activeTab,
  templatesEnabled,
  textEnabled,
  effectsEnabled,
  captionsEnabled,
  assistantEnabled,
  onMedia,
  onLibrary,
  onTemplates,
  onText,
  onShapes,
  onEffects,
  onTransitions,
  onCaptions,
  onAudio,
  onAssistant,
  onSettings,
}: {
  activeTab: InspectorRailTab;
  templatesEnabled: boolean;
  textEnabled: boolean;
  effectsEnabled: boolean;
  captionsEnabled: boolean;
  assistantEnabled: boolean;
  onMedia: () => void;
  onLibrary: () => void;
  onTemplates: () => void;
  onText: () => void;
  onShapes: () => void;
  onEffects: () => void;
  onTransitions: () => void;
  onCaptions: () => void;
  onAudio: () => void;
  onAssistant: () => void;
  onSettings: () => void;
}) {
  const topItems: Array<{
    key: string;
    label: string;
    Icon: LucideIcon;
    active?: boolean;
    disabled?: boolean;
    onClick: () => void;
  }> = [
    { key: 'media', label: 'Media', Icon: Film, active: activeTab === 'properties', onClick: onMedia },
    { key: 'library', label: 'Library', Icon: Files, onClick: onLibrary },
    { key: 'templates', label: 'Templates', Icon: LayoutTemplate, disabled: !templatesEnabled, onClick: onTemplates },
    { key: 'text', label: 'Text', Icon: Type, disabled: !textEnabled, onClick: onText },
    { key: 'shapes', label: 'Shapes', Icon: Square, disabled: !textEnabled, onClick: onShapes },
    { key: 'effects', label: 'Effects', Icon: Sparkles, active: activeTab === 'effects', disabled: !effectsEnabled, onClick: onEffects },
    { key: 'transitions', label: 'Transitions', Icon: ChevronsLeftRight, active: activeTab === 'transitions', disabled: !effectsEnabled, onClick: onTransitions },
    { key: 'captions', label: 'Captions', Icon: Subtitles, active: activeTab === 'captions', disabled: !captionsEnabled, onClick: onCaptions },
    { key: 'audio', label: 'Audio', Icon: AudioLines, active: activeTab === 'audio', onClick: onAudio },
    { key: 'assistant', label: 'AI Assistant', Icon: Bot, active: activeTab === 'assistant', disabled: !assistantEnabled, onClick: onAssistant },
  ];

  return (
    <nav className="flex w-16 shrink-0 flex-col border-r border-border bg-surface/80 py-2" aria-label="Video Edit Studio tools">
      <div className="flex flex-1 flex-col items-stretch gap-1 px-1.5">
        {topItems.map(({ key, label, Icon, active = false, disabled = false, onClick }) => (
          <button
            key={key}
            type="button"
            onClick={onClick}
            disabled={disabled}
            className={`flex min-h-11 flex-col items-center justify-center gap-0.5 rounded-md border text-[9px] leading-tight transition-colors disabled:cursor-not-allowed disabled:opacity-35 ${
              active
                ? 'border-primary/45 bg-primary/12 text-primary'
                : 'border-transparent text-text-muted hover:border-border hover:bg-surface-alt hover:text-text'
            }`}
            title={label}
            aria-label={key === 'text' ? 'Add text from tool rail' : label}
          >
            <Icon size={15} />
            <span className="max-w-full truncate">{label}</span>
          </button>
        ))}
      </div>
      <div className="mt-2 border-t border-border px-1.5 pt-2">
        <button
          type="button"
          onClick={onSettings}
          className="flex min-h-11 w-full flex-col items-center justify-center gap-0.5 rounded-md border border-transparent text-[9px] leading-tight text-text-muted transition-colors hover:border-border hover:bg-surface-alt hover:text-text"
          title="Settings"
          aria-label="Settings"
        >
          <Settings size={15} />
          <span>Settings</span>
        </button>
      </div>
    </nav>
  );
}

function StudioStatusBar({
  timeline,
  snappingEnabled,
}: {
  timeline: VideoTimelineDocument | null;
  snappingEnabled: boolean;
}) {
  const fps = timeline?.canvas.fps || 30;
  const markers = timeline?.markers?.length || 0;
  const previewVolume = useVideoStudioStore((state) => state.previewVolume);
  const setPreviewVolume = useVideoStudioStore((state) => state.setPreviewVolume);
  return (
    <div className="flex min-h-9 shrink-0 items-center gap-4 overflow-x-auto border-t border-border bg-surface-raised px-3 text-[11px] text-text-muted">
      <span>Project length: <span className="font-mono text-text-secondary">{formatStatusTime(timeline?.duration_ms || 0, fps)}</span></span>
      <span className="h-4 w-px bg-border" />
      <span>Frame rate: <span className="text-text-secondary">{fps}fps</span></span>
      <span className="h-4 w-px bg-border" />
      <span>Resolution: <span className="text-text-secondary">{timeline ? `${timeline.canvas.width}x${timeline.canvas.height}` : '—'}</span></span>
      <span className="h-4 w-px bg-border" />
      <span>Snap: <span className="text-text-secondary">{snappingEnabled ? 'On' : 'Off'}</span></span>
      <span className="h-4 w-px bg-border" />
      <span>Markers: <span className="text-text-secondary">{markers}</span></span>
      <div className="ml-auto hidden items-center gap-2 md:flex">
        <AudioLines size={13} />
        <span className="text-text-secondary">Preview</span>
        <input
          type="range"
          min={0}
          max={100}
          value={Math.round(previewVolume * 100)}
          onChange={(event) => setPreviewVolume(Number(event.target.value) / 100)}
          className="w-28"
          aria-label="Preview volume"
        />
        <span className="w-8 text-right font-mono tabular-nums text-text-secondary">{Math.round(previewVolume * 100)}%</span>
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
  const [deleteCandidate, setDeleteCandidate] = useState<{ id: string; title: string } | null>(null);
  const duplicateProject = useVideoStudioStore((state) => state.duplicateProject);
  const deleteProject = useVideoStudioStore((state) => state.deleteProject);
  return (
    <section className="rounded-lg border border-border bg-surface-alt p-3">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-text">Project</h2>
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
            action: () => setDeleteCandidate({ id: project.id, title: project.title }),
          },
        ];
        return <ContextMenu position={{ x: menu.x, y: menu.y }} items={items} onClose={() => setMenu(null)} />;
      })()}
      {deleteCandidate && (
        <ConfirmDialog
          title="Delete project"
          message={`Delete "${deleteCandidate.title}" and all of its assets? This cannot be undone.`}
          confirmLabel="Delete project"
          danger
          onConfirm={() => {
            setDeleteCandidate(null);
            void deleteProject(deleteCandidate.id);
          }}
          onCancel={() => setDeleteCandidate(null)}
        />
      )}
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
  onOpenLibrary,
}: {
  assets: VideoAsset[];
  activeAssetId: string | null;
  onSelect: (assetId: string) => void;
  onAdd: (assetId: string) => void;
  onRename: (assetId: string, fileName: string) => void;
  onDelete: (assetId: string) => void;
  onUpload: (file: File) => Promise<VideoAsset>;
  onOpenLibrary: () => void;
}) {
  const [view, setView] = useState<'list' | 'grid'>('list');
  const [filter, setFilter] = useState<AssetFilterKey>('all');
  const [sourceTab, setSourceTab] = useState<'project' | 'favorites'>('project');
  const [favoriteAssetIds, setFavoriteAssetIds] = useState<string[]>(() => {
    if (typeof window === 'undefined') return [];
    try {
      const value = JSON.parse(window.localStorage.getItem('omnillm-video-favorite-assets') || '[]');
      return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : [];
    } catch {
      return [];
    }
  });
  const [query, setQuery] = useState('');
  const [sortKey, setSortKey] = useState<'newest' | 'name' | 'duration' | 'size'>('newest');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingName, setEditingName] = useState('');
  const [uploading, setUploading] = useState(false);
  const [dragOver, setDragOver] = useState(false);
  const [menu, setMenu] = useState<{ assetId: string; x: number; y: number } | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<{ id: string; name: string; used: number } | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const timeline = useVideoStudioStore((state) => state.timeline);
  const addAssetToTimeline = useVideoStudioStore((state) => state.addAssetToTimeline);
  const addAssetAsMusicBed = useVideoStudioStore((state) => state.addAssetAsMusicBed);
  const addTrack = useVideoStudioStore((state) => state.addTrack);
  const setAppMode = useSettingsStore((state) => state.setAppMode);
  const createConversation = useConversationStore((state) => state.createConversation);

  useEffect(() => {
    window.localStorage.setItem('omnillm-video-favorite-assets', JSON.stringify(favoriteAssetIds));
  }, [favoriteAssetIds]);

  const toggleFavorite = (assetId: string) => {
    setFavoriteAssetIds((current) => current.includes(assetId)
      ? current.filter((id) => id !== assetId)
      : [...current, assetId]);
  };

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
        try {
          await onUpload(file);
        } catch {
          // The store already surfaced the per-file error; continue the batch.
        }
      }
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const filtered = assets
    .filter((asset) => sourceTab !== 'favorites' || favoriteAssetIds.includes(asset.id))
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
          toggleFavorite(asset.id);
        }}
        className={`min-h-7 min-w-7 rounded-md border bg-surface-alt inline-flex items-center justify-center ${
          favoriteAssetIds.includes(asset.id) ? 'border-amber-400/40 text-amber-300' : 'border-border text-text-muted hover:text-text'
        }`}
        aria-label={`${favoriteAssetIds.includes(asset.id) ? 'Remove' : 'Add'} ${asset.file_name} ${favoriteAssetIds.includes(asset.id) ? 'from' : 'to'} favorites`}
        title={favoriteAssetIds.includes(asset.id) ? 'Remove from favorites' : 'Add to favorites'}
      >
        <Star size={12} fill={favoriteAssetIds.includes(asset.id) ? 'currentColor' : 'none'} />
      </button>
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
          setDeleteCandidate({ id: asset.id, name: asset.file_name, used: usedCounts.get(asset.id) || 0 });
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
      <div className="mb-2 flex items-center justify-between gap-2">
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
      <div className="mb-2 flex border-b border-border text-[11px]" role="tablist" aria-label="Media sources">
        {[
          { key: 'project' as const, label: 'Project Media' },
          { key: 'favorites' as const, label: 'Favorites' },
        ].map((tab) => (
          <button
            key={tab.key}
            type="button"
            role="tab"
            aria-selected={sourceTab === tab.key}
            onClick={() => setSourceTab(tab.key)}
            className={`border-b-2 px-2 py-1.5 transition-colors ${
              sourceTab === tab.key ? 'border-primary text-text' : 'border-transparent text-text-muted hover:text-text'
            }`}
          >
            {tab.label}
          </button>
        ))}
        <button
          type="button"
          onClick={onOpenLibrary}
          className="ml-auto border-b-2 border-transparent px-2 py-1.5 text-text-muted transition-colors hover:text-text"
          title="Open the global File Library"
        >
          Library ↗
        </button>
      </div>
      <div className="mb-2 flex items-center gap-1.5">
        <div className="relative min-w-0 flex-1">
          <Filter size={12} className="pointer-events-none absolute left-2 top-1/2 -translate-y-1/2 text-text-muted" />
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search media..."
            className="min-h-8 w-full min-w-0 rounded-md border border-border bg-surface px-2 pl-7 text-xs text-text focus:border-primary/50 focus:outline-none"
            aria-label="Search media bin"
          />
        </div>
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
        assets.length === 0 ? (
          <div className="rounded-lg border border-dashed border-border bg-surface px-3 py-6 text-center">
            <p className="text-xs font-medium text-text-secondary">Your media bin is empty</p>
            <p className="mt-1 text-[11px] text-text-muted">
              Drop video, image, or audio files here — or generate a clip and use “Send to Timeline”.
            </p>
            <button
              onClick={() => fileInputRef.current?.click()}
              className="mt-3 inline-flex min-h-8 items-center gap-1.5 rounded-md border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:text-text"
            >
              <Upload size={12} />
              Upload media
            </button>
          </div>
        ) : (
          <div className="rounded-lg border border-dashed border-border bg-surface px-3 py-8 text-center text-xs text-text-muted">
            {sourceTab === 'favorites' ? 'No favorite media yet. Use the star on any asset to keep it here.' : 'No media matches this filter.'}
          </div>
        )
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
          {
            label: favoriteAssetIds.includes(asset.id) ? 'Remove from favorites' : 'Add to favorites',
            action: () => toggleFavorite(asset.id),
          },
          'divider',
          { label: 'Rename', action: () => { setEditingId(asset.id); setEditingName(asset.file_name); } },
          { label: 'Download', action: () => { window.open(videoApi.downloadUrl(asset.id), '_blank', 'noopener,noreferrer'); } },
          { label: 'Send to Chat', action: () => { void sendAssetToChat(asset); } },
          { label: 'Register in File Library', action: () => { void registerInLibrary(asset); } },
          'divider',
          {
            label: used > 0 ? `Delete (used by ${used} clip${used === 1 ? '' : 's'})` : 'Delete',
            danger: true,
            action: () => setDeleteCandidate({ id: asset.id, name: asset.file_name, used }),
          },
        ];
        return <ContextMenu position={{ x: menu.x, y: menu.y }} items={items} onClose={() => setMenu(null)} />;
      })()}
      {deleteCandidate && (
        <ConfirmDialog
          title="Delete asset"
          message={deleteCandidate.used > 0
            ? `Delete "${deleteCandidate.name}"? ${deleteCandidate.used} clip${deleteCandidate.used === 1 ? '' : 's'} in the timeline use${deleteCandidate.used === 1 ? 's' : ''} it and will stop rendering.`
            : `Delete "${deleteCandidate.name}"?`}
          confirmLabel="Delete asset"
          danger
          onConfirm={() => {
            setDeleteCandidate(null);
            onDelete(deleteCandidate.id);
          }}
          onCancel={() => setDeleteCandidate(null)}
        />
      )}
    </section>
  );
}
