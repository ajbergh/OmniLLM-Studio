import { Clapperboard, Loader2 } from 'lucide-react';
import { useVideoStudioStore } from '../../stores/videoStudio';
import { RenderJobStatus } from './RenderJobStatus';

export function VideoRenderPanel() {
  const exportSettings = useVideoStudioStore((state) => state.exportSettings);
  const renderJobs = useVideoStudioStore((state) => state.renderJobs);
  const isRendering = useVideoStudioStore((state) => state.isRendering);
  const setExportSetting = useVideoStudioStore((state) => state.setExportSetting);
  const renderTimeline = useVideoStudioStore((state) => state.renderTimeline);
  const cancelRenderJob = useVideoStudioStore((state) => state.cancelRenderJob);
  const downloadRender = useVideoStudioStore((state) => state.downloadRender);

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
          <span className="mb-1 block text-[11px] font-medium text-text-muted">Resolution</span>
          <select
            value={exportSettings.resolution}
            onChange={(event) => setExportSetting('resolution', event.target.value as 'project' | '720p' | '1080p')}
            className="min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
          >
            <option value="project">Project</option>
            <option value="720p">720p</option>
            <option value="1080p">1080p</option>
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
