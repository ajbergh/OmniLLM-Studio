/**
 * Export panel: format/preset/FPS/quality plus an advanced section (codec
 * incl. H.265, audio bitrate, caption burn-in and SRT/VTT sidecar, export
 * range). Render runs through the exportValidation checklist first — errors
 * block, warnings can be acknowledged ("Render anyway"). Shows the full job
 * history and a "timeline changed since last render" banner driven by the
 * store's save/render sequence counters.
 */
import { useState } from 'react';
import { AlertTriangle, ChevronDown, ChevronRight, Clapperboard, Loader2 } from 'lucide-react';
import { useVideoStudioStore } from '../../stores/videoStudio';
import { RenderJobStatus } from './RenderJobStatus';
import { validateExport } from './exportValidation';
import type { ExportValidationResult } from './exportValidation';

const EXPORT_PRESETS = [
  { key: 'project', label: 'Project size' },
  { key: '720p', label: '720p (training/LMS)' },
  { key: '1080p', label: '1080p' },
  { key: 'youtube_16_9', label: 'YouTube 16:9 (1920×1080)', width: 1920, height: 1080 },
  { key: 'youtube_4k', label: 'YouTube 4K (3840×2160)', width: 3840, height: 2160 },
  { key: 'shorts_9_16', label: 'Shorts/Reels/TikTok 9:16 (1080×1920)', width: 1080, height: 1920 },
  { key: 'square_1_1', label: 'Square 1:1 (1080×1080)', width: 1080, height: 1080 },
  { key: 'linkedin_16_9', label: 'LinkedIn 16:9 (1920×1080)', width: 1920, height: 1080 },
  { key: 'custom', label: 'Custom…' },
] as const;

export function VideoRenderPanel() {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const assets = useVideoStudioStore((state) => state.assets);
  const exportSettings = useVideoStudioStore((state) => state.exportSettings);
  const renderJobs = useVideoStudioStore((state) => state.renderJobs);
  const isRendering = useVideoStudioStore((state) => state.isRendering);
  const rendererCapabilities = useVideoStudioStore((state) => state.rendererCapabilities);
  const setExportSetting = useVideoStudioStore((state) => state.setExportSetting);
  const renderTimeline = useVideoStudioStore((state) => state.renderTimeline);
  const retryRenderJob = useVideoStudioStore((state) => state.retryRenderJob);
  const cancelRenderJob = useVideoStudioStore((state) => state.cancelRenderJob);
  const deleteRenderJob = useVideoStudioStore((state) => state.deleteRenderJob);
  const downloadRender = useVideoStudioStore((state) => state.downloadRender);
  const saveSeq = useVideoStudioStore((state) => state._saveSeq);
  const renderedSaveSeq = useVideoStudioStore((state) => state.renderedSaveSeq);
  const selectedClipIds = useVideoStudioStore((state) => state.selectedClipIds);

  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [checklist, setChecklist] = useState<ExportValidationResult | null>(null);

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
  const hasRendered = renderedSaveSeq > 0;
  const dirtySinceRender = hasRendered && saveSeq > renderedSaveSeq;
  const rangeActive = (exportSettings.range_end_ms || 0) > (exportSettings.range_start_ms || 0);

  const startRender = (skipChecklist = false) => {
    const result = validateExport(timeline, assets, exportSettings, rendererCapabilities);
    if (result.errors.length > 0 || (!skipChecklist && result.warnings.length > 0)) {
      setChecklist(result);
      return;
    }
    setChecklist(null);
    void renderTimeline();
  };

  const setRangeFromSelection = () => {
    if (!timeline || selectedClipIds.length === 0) return;
    const selected = timeline.tracks.flatMap((track) => track.clips).filter((clip) => selectedClipIds.includes(clip.id));
    if (selected.length === 0) return;
    setExportSetting('range_start_ms', Math.min(...selected.map((clip) => clip.start_ms)));
    setExportSetting('range_end_ms', Math.max(...selected.map((clip) => clip.start_ms + clip.duration_ms)));
  };

  return (
    <section className="rounded-lg border border-border bg-surface p-3">
      <div className="mb-3 flex items-center gap-2">
        <Clapperboard size={14} className="text-primary" />
        <h2 className="text-sm font-semibold text-text">Export</h2>
        {dirtySinceRender && (
          <span className="ml-auto rounded bg-amber-400/15 px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wide text-amber-300" title="The timeline has been edited since the last completed render">
            changed since render
          </span>
        )}
      </div>
      <div className="grid grid-cols-2 gap-2">
        <label className="block">
          <span className="mb-1 block text-[11px] font-medium text-text-muted">Format</span>
          <select
            value={exportSettings.format}
            onChange={(event) => {
              const format = event.target.value as 'mp4' | 'webm';
              setExportSetting('format', format);
              setExportSetting('codec', format === 'webm' ? 'vp9' : 'h264');
            }}
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

      <button
        className="mt-2 inline-flex items-center gap-1 text-[11px] text-text-muted hover:text-text"
        onClick={() => setAdvancedOpen((open) => !open)}
        aria-expanded={advancedOpen}
      >
        {advancedOpen ? <ChevronDown size={11} /> : <ChevronRight size={11} />}
        Advanced — codec, captions, range
      </button>
      {advancedOpen && (
        <div className="mt-2 grid grid-cols-2 gap-2 rounded-md border border-border bg-surface-alt/50 p-2">
          <label className="block">
            <span className="mb-1 block text-[11px] font-medium text-text-muted">Codec</span>
            <select
              value={exportSettings.codec || (exportSettings.format === 'webm' ? 'vp9' : 'h264')}
              onChange={(event) => setExportSetting('codec', event.target.value as 'h264' | 'h265' | 'vp9')}
              className="min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
              title="H.265 needs an FFmpeg build with libx265"
            >
              {exportSettings.format === 'webm' ? (
                <option value="vp9">VP9</option>
              ) : (
                <>
                  <option value="h264">H.264</option>
                  <option value="h265">H.265 (if FFmpeg supports it)</option>
                </>
              )}
            </select>
          </label>
          <label className="block">
            <span className="mb-1 block text-[11px] font-medium text-text-muted">Audio bitrate</span>
            <select
              value={exportSettings.audio_bitrate_kbps || 128}
              onChange={(event) => setExportSetting('audio_bitrate_kbps', Number(event.target.value))}
              className="min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
              disabled={!exportSettings.include_audio}
            >
              {[96, 128, 192, 256, 320].map((kbps) => (
                <option key={kbps} value={kbps}>{kbps} kbps</option>
              ))}
            </select>
          </label>
          <label className="col-span-2 flex items-center gap-2 text-xs text-text-secondary">
            <input
              type="checkbox"
              checked={exportSettings.burn_in_captions !== false}
              onChange={(event) => setExportSetting('burn_in_captions', event.target.checked)}
            />
            Burn captions into the video
          </label>
          <label className="block">
            <span className="mb-1 block text-[11px] font-medium text-text-muted">Caption sidecar file</span>
            <select
              value={exportSettings.sidecar_captions || ''}
              onChange={(event) => setExportSetting('sidecar_captions', event.target.value as '' | 'srt' | 'vtt')}
              className="min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
              title="Writes the captions as a separate asset next to the export"
            >
              <option value="">None</option>
              <option value="srt">SRT</option>
              <option value="vtt">VTT</option>
            </select>
          </label>
          <div className="col-span-2">
            <span className="mb-1 block text-[11px] font-medium text-text-muted">Export range (seconds)</span>
            <div className="flex items-center gap-1.5">
              <input
                type="number"
                min={0}
                step={0.1}
                value={(exportSettings.range_start_ms || 0) / 1000}
                onChange={(event) => setExportSetting('range_start_ms', Math.max(0, Math.round(Number(event.target.value) * 1000)))}
                className="min-h-8 w-20 rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
                aria-label="Export range start seconds"
              />
              <span className="text-[10px] text-text-muted">→</span>
              <input
                type="number"
                min={0}
                step={0.1}
                value={(exportSettings.range_end_ms || 0) / 1000}
                onChange={(event) => setExportSetting('range_end_ms', Math.max(0, Math.round(Number(event.target.value) * 1000)))}
                className="min-h-8 w-20 rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
                aria-label="Export range end seconds"
              />
              <button
                className="rounded-md border border-border bg-surface px-2 py-1 text-[10px] text-text-muted hover:text-text disabled:opacity-40"
                disabled={selectedClipIds.length === 0}
                onClick={setRangeFromSelection}
                title="Set the range from the selected clips"
              >
                From selection
              </button>
              <button
                className="rounded-md border border-border bg-surface px-2 py-1 text-[10px] text-text-muted hover:text-text"
                onClick={() => {
                  setExportSetting('range_start_ms', 0);
                  setExportSetting('range_end_ms', 0);
                }}
                title="Export the full timeline"
              >
                Full
              </button>
            </div>
            <p className="mt-1 text-[10px] text-text-muted">
              {rangeActive
                ? `Exporting ${(((exportSettings.range_end_ms || 0) - (exportSettings.range_start_ms || 0)) / 1000).toFixed(1)}s window`
                : 'Full timeline (set end > start to export a range)'}
            </p>
          </div>
        </div>
      )}

      {partialFeatures.length > 0 && (
        <p
          className="mt-2 rounded-md border border-amber-500/20 bg-amber-500/5 px-2 py-1.5 text-[10px] text-amber-400/70"
          title={partialFeatures.map((f) => `${f.label}${f.notes ? ` — ${f.notes}` : ''}`).join('\n')}
        >
          ⚠ Limited at export: {partialFeatures.map((f) => f.label).join(', ')}
        </p>
      )}
      <button
        onClick={() => startRender()}
        disabled={isRendering}
        className="btn-primary mt-3 min-h-9 w-full rounded-lg px-3 text-xs font-medium inline-flex items-center justify-center gap-1.5 disabled:opacity-50"
      >
        {isRendering ? <Loader2 size={14} className="animate-spin" /> : <Clapperboard size={14} />}
        Render
      </button>
      {dirtySinceRender && (
        <button
          onClick={() => startRender()}
          disabled={isRendering}
          className="mt-1.5 min-h-8 w-full rounded-lg border border-amber-400/30 bg-amber-400/10 px-3 text-[11px] text-amber-300 hover:bg-amber-400/15 disabled:opacity-50"
          title="The timeline changed after the last render finished"
        >
          Timeline changed — render again
        </button>
      )}
      <div className="mt-3 max-h-80 space-y-2 overflow-y-auto pr-0.5">
        {renderJobs.map((job) => (
          <RenderJobStatus
            key={job.id}
            job={job}
            onCancel={(jobId) => { void cancelRenderJob(jobId); }}
            onDownload={downloadRender}
            onRetry={(jobId) => { void retryRenderJob(jobId); }}
            onDelete={(jobId) => { void deleteRenderJob(jobId); }}
          />
        ))}
        {renderJobs.length === 0 && (
          <p className="rounded-md border border-dashed border-border bg-surface-alt px-2 py-2 text-[11px] text-text-muted">
            No renders yet. Configure the export above and press Render.
          </p>
        )}
      </div>

      {checklist && (
        <div className="fixed inset-0 z-[110] flex items-center justify-center bg-black/50" onClick={() => setChecklist(null)}>
          <div
            role="dialog"
            aria-modal="true"
            aria-label="Export checklist"
            className="w-96 max-w-[calc(100vw-2rem)] rounded-lg border border-border bg-surface p-4 shadow-xl"
            onClick={(event) => event.stopPropagation()}
          >
            <h3 className="text-sm font-semibold text-text">Export checklist</h3>
            {checklist.errors.length > 0 && (
              <ul className="mt-2 space-y-1">
                {checklist.errors.map((error) => (
                  <li key={error} className="flex items-start gap-1.5 text-[11px] text-red-400">
                    <AlertTriangle size={12} className="mt-0.5 shrink-0" />
                    {error}
                  </li>
                ))}
              </ul>
            )}
            {checklist.warnings.length > 0 && (
              <ul className="mt-2 space-y-1">
                {checklist.warnings.map((warning) => (
                  <li key={warning} className="flex items-start gap-1.5 text-[11px] text-amber-300">
                    <AlertTriangle size={12} className="mt-0.5 shrink-0" />
                    {warning}
                  </li>
                ))}
              </ul>
            )}
            <div className="mt-4 flex justify-end gap-2">
              <button
                className="rounded-md border border-border bg-surface-alt px-3 py-1.5 text-[12px] text-text-secondary hover:text-text"
                onClick={() => setChecklist(null)}
              >
                Go back
              </button>
              {checklist.errors.length === 0 && (
                <button
                  className="rounded-md bg-primary px-3 py-1.5 text-[12px] font-medium text-white hover:bg-primary/90"
                  onClick={() => {
                    setChecklist(null);
                    void renderTimeline();
                  }}
                >
                  Render anyway
                </button>
              )}
            </div>
          </div>
        </div>
      )}
    </section>
  );
}
