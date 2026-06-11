import { useState } from 'react';
import { Download, Loader2, X } from 'lucide-react';
import { toast } from 'sonner';
import { ContextMenu } from '../common/ContextMenu';
import type { ContextMenuEntry } from '../common/ContextMenu';
import type { VideoRenderJob } from '../../types/video';

export function RenderJobStatus({
  job,
  onCancel,
  onDownload,
  onRetry,
}: {
  job: VideoRenderJob;
  onCancel: (jobId: string) => void;
  onDownload: (jobId: string) => void;
  onRetry?: () => void;
}) {
  const progress = Math.round((job.progress || 0) * 100);
  const terminal = ['completed', 'failed', 'cancelled'].includes(job.status);
  const [menu, setMenu] = useState<{ x: number; y: number } | null>(null);

  const copyToClipboard = (text: string, what: string) => {
    void navigator.clipboard.writeText(text).then(
      () => toast.success(`${what} copied`),
      () => toast.error(`Could not copy ${what.toLowerCase()}`),
    );
  };

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
          <p className="text-[10px] text-text-muted">{progress}%</p>
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
          { label: 'Render again', disabled: !onRetry, action: () => onRetry?.() },
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
