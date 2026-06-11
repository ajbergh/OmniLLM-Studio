import { useState } from 'react';
import { Download, Loader2, X } from 'lucide-react';
import { toast } from 'sonner';
import { videoApi } from '../../api';
import { ContextMenu } from '../common/ContextMenu';
import type { ContextMenuEntry } from '../common/ContextMenu';
import type { VideoExportSettings, VideoRenderJob } from '../../types/video';

export function RenderJobStatus({
  job,
  onCancel,
  onDownload,
  onRetry,
}: {
  job: VideoRenderJob;
  onCancel: (jobId: string) => void;
  onDownload: (jobId: string) => void;
  onRetry?: (jobId: string) => void;
}) {
  const progress = Math.round((job.progress || 0) * 100);
  const terminal = ['completed', 'failed', 'cancelled'].includes(job.status);
  const [menu, setMenu] = useState<{ x: number; y: number } | null>(null);
  const [detailsOpen, setDetailsOpen] = useState(false);

  const copyToClipboard = (text: string, what: string) => {
    void navigator.clipboard.writeText(text).then(
      () => toast.success(`${what} copied`),
      () => toast.error(`Could not copy ${what.toLowerCase()}`),
    );
  };

  const registerInLibrary = async () => {
    if (!job.output_asset_id) return;
    try {
      await videoApi.registerAssetInLibrary(job.output_asset_id);
      toast.success('Export registered in File Library');
    } catch (err) {
      toast.error(`Failed to register: ${(err as Error).message}`);
    }
  };

  const elapsed = (() => {
    const start = job.started_at ? Date.parse(job.started_at) : Date.parse(job.created_at);
    const end = job.completed_at ? Date.parse(job.completed_at) : Date.now();
    if (!Number.isFinite(start) || !Number.isFinite(end) || end < start) return null;
    return Math.round((end - start) / 1000);
  })();

  const settingsSummary = (() => {
    try {
      const settings = JSON.parse(job.settings_json) as VideoExportSettings;
      const bits = [settings.format?.toUpperCase(), settings.codec, settings.resolution];
      if (settings.width && settings.height) bits.push(`${settings.width}×${settings.height}`);
      if (settings.fps) bits.push(`${settings.fps}fps`);
      if (settings.quality) bits.push(settings.quality);
      if ((settings.range_end_ms || 0) > (settings.range_start_ms || 0)) {
        bits.push(`range ${((settings.range_start_ms || 0) / 1000).toFixed(1)}–${((settings.range_end_ms || 0) / 1000).toFixed(1)}s`);
      }
      if (settings.burn_in_captions === false) bits.push('no caption burn-in');
      if (settings.sidecar_captions) bits.push(`${settings.sidecar_captions.toUpperCase()} sidecar`);
      return bits.filter(Boolean).join(' · ');
    } catch {
      return null;
    }
  })();

  return (
    <div
      className="rounded-lg border border-border bg-surface-alt p-2"
      onContextMenu={(event) => {
        event.preventDefault();
        setMenu({ x: event.clientX, y: event.clientY });
      }}
    >
      <div className="flex items-center justify-between gap-2">
        <div className="min-w-0">
          <p className="truncate text-xs font-medium text-text">{job.status}</p>
          <p className="text-[10px] text-text-muted">
            {progress}%
            {elapsed !== null && ` · ${elapsed}s${terminal ? '' : ' elapsed'}`}
            {job.completed_at && ` · ${new Date(job.completed_at).toLocaleTimeString()}`}
          </p>
        </div>
        <div className="flex items-center gap-1">
          {job.status === 'completed' && job.output_asset_id && (
            <button
              onClick={() => onDownload(job.id)}
              className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-border bg-surface text-text-muted hover:text-text"
              title="Download render"
              aria-label="Download render"
            >
              <Download size={13} />
            </button>
          )}
          {!terminal && (
            <button
              onClick={() => onCancel(job.id)}
              className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-border bg-surface text-text-muted hover:text-text"
              title="Cancel render"
              aria-label="Cancel render"
            >
              {job.status === 'running' ? <Loader2 size={13} className="animate-spin" /> : <X size={13} />}
            </button>
          )}
        </div>
      </div>
      <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-surface">
        <div className="h-full bg-primary" style={{ width: `${Math.max(3, progress)}%` }} />
      </div>
      {job.error && <p className="mt-2 text-[10px] text-danger">{job.error}</p>}
      {settingsSummary && (
        <button
          className="mt-1.5 block w-full truncate text-left text-[10px] text-text-muted hover:text-text"
          onClick={() => setDetailsOpen((open) => !open)}
          title={settingsSummary}
        >
          {detailsOpen ? '▾' : '▸'} {settingsSummary}
        </button>
      )}
      {detailsOpen && job.metadata_json && job.metadata_json !== '{}' && (
        <pre className="mt-1 max-h-36 overflow-auto whitespace-pre-wrap break-all rounded-md bg-surface p-2 text-[9px] text-text-muted">
          {formatJobMetadata(job.metadata_json)}
        </pre>
      )}
      {job.status === 'failed' && job.metadata_json && job.metadata_json !== '{}' && (
        <details className="mt-1.5">
          <summary className="cursor-pointer text-[10px] text-text-muted hover:text-text">FFmpeg diagnostics</summary>
          <pre className="mt-1 max-h-36 overflow-auto whitespace-pre-wrap break-all rounded-md bg-surface p-2 text-[9px] text-text-muted">
            {formatDiagnostics(job.metadata_json)}
          </pre>
        </details>
      )}
      {menu && (() => {
        const items: ContextMenuEntry[] = [
          { label: 'Download output', disabled: job.status !== 'completed' || !job.output_asset_id, action: () => onDownload(job.id) },
          { label: 'Render again with these settings', disabled: !onRetry, action: () => onRetry?.(job.id) },
          { label: 'Register output in File Library', disabled: job.status !== 'completed' || !job.output_asset_id, action: () => { void registerInLibrary(); } },
          'divider',
          { label: 'Copy error', disabled: !job.error, action: () => copyToClipboard(job.error || '', 'Error') },
          {
            label: 'Copy FFmpeg diagnostics',
            disabled: !job.metadata_json || job.metadata_json === '{}',
            action: () => copyToClipboard(formatDiagnostics(job.metadata_json || ''), 'Diagnostics'),
          },
          { label: 'Copy job ID', action: () => copyToClipboard(job.id, 'Job ID') },
          'divider',
          { label: 'Cancel job', disabled: terminal, danger: true, action: () => onCancel(job.id) },
        ];
        return <ContextMenu position={menu} items={items} onClose={() => setMenu(null)} />;
      })()}
    </div>
  );
}

function formatJobMetadata(metadataJson: string): string {
  try {
    const meta = JSON.parse(metadataJson) as Record<string, unknown>;
    return Object.entries(meta)
      .filter(([key]) => key !== 'ffmpeg_command' && key !== 'ffmpeg_stderr')
      .map(([key, value]) => `${key}: ${String(value)}`)
      .join('\n') || metadataJson;
  } catch {
    return metadataJson;
  }
}

function formatDiagnostics(metadataJson: string): string {
  try {
    const meta = JSON.parse(metadataJson) as Record<string, unknown>;
    const lines: string[] = [];
    if (typeof meta.ffmpeg_command === 'string') lines.push(`$ ${meta.ffmpeg_command}`);
    if (typeof meta.ffmpeg_stderr === 'string' && meta.ffmpeg_stderr.trim()) lines.push(meta.ffmpeg_stderr.trim());
    return lines.join('\n\n') || metadataJson;
  } catch {
    return metadataJson;
  }
}
