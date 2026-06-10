import { Clapperboard, Loader2 } from 'lucide-react';
import { useVideoStudioStore } from '../../stores/videoStudio';
import { RenderJobStatus } from './RenderJobStatus';

const EXPORT_PRESETS = [
  { key: 'project', label: 'Project size' },
  { key: '720p', label: '720p' },
  { key: '1080p', label: '1080p' },
  { key: 'youtube_16_9', label: 'YouTube 16:9 (1920×1080)', width: 1920, height: 1080 },
  { key: 'shorts_9_16', label: 'Shorts/Reels 9:16 (1080×1920)', width: 1080, height: 1920 },
  { key: 'square_1_1', label: 'Square 1:1 (1080×1080)', width: 1080, height: 1080 },
  { key: 'custom', label: 'Custom…' },
] as const;

export function VideoRenderPanel() {
  const exportSettings = useVideoStudioStore((state) => state.exportSettings);
  const renderJobs = useVideoStudioStore((state) => state.renderJobs);
  const isRendering = useVideoStudioStore((state) => state.isRendering);
  const rendererCapabilities = useVideoStudioStore((state) => state.rendererCapabilities);
  const setExportSetting = useVideoStudioStore((state) => state.setExportSetting);
  const renderTimeline = useVideoStudioStore((state) => state.renderTimeline);
  const cancelRenderJob = useVideoStudioStore((state) => state.cancelRenderJob);
  const downloadRender = useVideoStudioStore((state) => state.downloadRender);

  const presetKey = exportSettings.preset
    || (exportSettings.resolution === 'custom' ? 'custom' : exportSettings.resolution);

  const applyPreset = (key: string) => {
    const preset = EXPORT_PRESETS.find((item) => item.key === key);
    if (!preset) return;
    if (key === 'project' || key === '720p' || key === '1080p') {
      setExportSetting('resolution', key as 'project' | '720p' | '1080p');
      setExportSetting('preset', undefined);
      setExportSetting('width', undefined);
      setExportSetting('height', undefined);
      return;
    }
    setExportSetting('resolution', 'custom');
    setExportSetting('preset', key);
    if ('width' in preset && preset.width) {
      setExportSetting('width', preset.width);
      setExportSetting('height', preset.height);
    } else {
      setExportSetting('width', exportSettings.width || 1920);
      setExportSetting('height', exportSettings.height || 1080);
    }
  };

  const partialFeatures = (rendererCapabilities?.features || []).filter((f) => !f.supported || f.partial);

  return (
    <section className="rounded-lg border border-border bg-surface p-3">
      <div className="mb-3 flex items-center gap-2">
        <Clapperboard size={14} className="text-primary" />
        <h2 className="text-sm font-semibold text-text">Export</h2>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <label className="block">
          <span className="mb-1 block text-[11px] font-medium text-text-muted">Format</span>
          <select
            value={exportSettings.format}
            onChange={(event) => setExportSetting('format', event.target.value as 'mp4' | 'webm')}
            className="min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
          >
            <option value="mp4">MP4</option>
            <option value="webm">WebM</option>
          </select>
        </label>
        <label className="block">
          <span className="mb-1 block text-[11px] font-medium text-text-muted">Preset</span>
          <select
            value={presetKey}
            onChange={(event) => applyPreset(event.target.value)}
            className="min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
          >
            {EXPORT_PRESETS.map((preset) => (
              <option key={preset.key} value={preset.key}>{preset.label}</option>
            ))}
          </select>
        </label>
        {exportSettings.resolution === 'custom' && exportSettings.preset === 'custom' && (
          <>
            <label className="block">
              <span className="mb-1 block text-[11px] font-medium text-text-muted">Width</span>
              <input
                type="number"
                min={16}
                max={7680}
                value={exportSettings.width || 1920}
                onChange={(event) => setExportSetting('width', Math.max(16, Number(event.target.value) || 16))}
                className="min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
              />
            </label>
            <label className="block">
              <span className="mb-1 block text-[11px] font-medium text-text-muted">Height</span>
              <input
                type="number"
                min={16}
                max={7680}
                value={exportSettings.height || 1080}
                onChange={(event) => setExportSetting('height', Math.max(16, Number(event.target.value) || 16))}
                className="min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
              />
            </label>
          </>
        )}
        <label className="block">
          <span className="mb-1 block text-[11px] font-medium text-text-muted">FPS</span>
          <select
            value={exportSettings.fps || 30}
            onChange={(event) => setExportSetting('fps', Number(event.target.value))}
            className="min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
          >
            <option value={24}>24</option>
            <option value={30}>30</option>
            <option value={60}>60</option>
          </select>
        </label>
        <label className="block">
          <span className="mb-1 block text-[11px] font-medium text-text-muted">Quality</span>
          <select
            value={exportSettings.quality}
            onChange={(event) => setExportSetting('quality', event.target.value as 'draft' | 'standard' | 'high')}
            className="min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
          >
            <option value="draft">Draft</option>
            <option value="standard">Standard</option>
            <option value="high">High</option>
          </select>
        </label>
        <label className="flex items-end gap-2 pb-2 text-xs text-text-secondary">
          <input
            type="checkbox"
            checked={exportSettings.include_audio}
            onChange={(event) => setExportSetting('include_audio', event.target.checked)}
          />
          Audio
        </label>
      </div>
      {partialFeatures.length > 0 && (
        <p
          className="mt-2 rounded-md border border-amber-500/20 bg-amber-500/5 px-2 py-1.5 text-[10px] text-amber-400/70"
          title={partialFeatures.map((f) => `${f.label}${f.notes ? ` — ${f.notes}` : ''}`).join('\n')}
        >
          ⚠ Limited at export: {partialFeatures.map((f) => f.label).join(', ')}
        </p>
      )}
      <button
        onClick={() => { void renderTimeline(); }}
        disabled={isRendering}
        className="btn-primary mt-3 min-h-9 w-full rounded-lg px-3 text-xs font-medium inline-flex items-center justify-center gap-1.5 disabled:opacity-50"
      >
        {isRendering ? <Loader2 size={14} className="animate-spin" /> : <Clapperboard size={14} />}
        Render
      </button>
      <div className="mt-3 space-y-2">
        {renderJobs.slice(0, 3).map((job) => (
          <RenderJobStatus
            key={job.id}
            job={job}
            onCancel={(jobId) => { void cancelRenderJob(jobId); }}
            onDownload={downloadRender}
          />
        ))}
      </div>
    </section>
  );
}
