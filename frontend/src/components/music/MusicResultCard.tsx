import { useMemo, useState } from 'react';
import { Copy, GitBranch, MessageSquare, Music2, Image, Video } from 'lucide-react';
import { toast } from 'sonner';
import { clsx } from 'clsx';
import { musicAssetUrl } from '../../api';
import type { MusicGenerationDetail } from '../../types/music';
import { MusicPlayer } from './MusicPlayer';
import { WaveformViewer } from './WaveformViewer';

type ResultTab = 'lyrics' | 'prompt' | 'metadata' | 'cost';

interface MusicResultCardProps {
  generation?: MusicGenerationDetail;
  isGenerating: boolean;
  progressMessage?: string;
  onBranch: (generationId: string) => void;
  onRegenerate: (generationId: string) => void;
  onSendToChat: (generation: MusicGenerationDetail) => void;
  onGenerateAlbumArt?: (generation: MusicGenerationDetail) => void;
  onSendToVideo?: (generation: MusicGenerationDetail) => void;
}

const TABS: Array<{ key: ResultTab; label: string }> = [
  { key: 'lyrics', label: 'Lyrics / structure' },
  { key: 'prompt', label: 'Prompt' },
  { key: 'metadata', label: 'Metadata' },
  { key: 'cost', label: 'Cost / usage' },
];

export function MusicResultCard({
  generation,
  isGenerating,
  progressMessage,
  onBranch,
  onRegenerate,
  onSendToChat,
  onGenerateAlbumArt,
  onSendToVideo,
}: MusicResultCardProps) {
  const [tab, setTab] = useState<ResultTab>('lyrics');
  const assetUrl = generation?.asset_id ? musicAssetUrl(generation.asset_id) : undefined;
  const canUseAsset = generation?.status === 'completed' && Boolean(assetUrl);
  const displayTitle = generation?.title || 'Untitled Track';

  const metadataRows = useMemo(() => {
    if (!generation) return [];
    return [
      ['Generation ID', generation.id],
      ['Provider', generation.provider],
      ['Model', generation.model],
      ['MIME type', generation.mime_type || 'unknown'],
      ['Output bytes', generation.output_bytes ? generation.output_bytes.toLocaleString() : '0'],
      ['Created', new Date(generation.created_at).toLocaleString()],
      ['Completed', generation.completed_at ? new Date(generation.completed_at).toLocaleString() : 'pending'],
    ];
  }, [generation]);

  if (!generation) {
    return (
      <div className="flex h-full min-h-[420px] flex-col items-center justify-center text-center text-text-muted/60">
        <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-2xl border border-border bg-surface-alt">
          <Music2 size={28} className="opacity-50" />
        </div>
        <p className="text-sm">Describe a song and generate to start.</p>
        {isGenerating && progressMessage && (
          <p className="mt-3 rounded-lg bg-primary/10 px-3 py-1 text-xs text-primary">{progressMessage}</p>
        )}
      </div>
    );
  }

  return (
    <article className="flex h-full min-h-0 flex-col">
      <header className="border-b border-border p-4">
        <div className="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
          <div className="min-w-0">
            <h1 className="truncate text-lg font-semibold text-text">{displayTitle}</h1>
            <div className="mt-2 flex flex-wrap gap-1.5">
              <span className="rounded-md border border-primary/20 bg-primary/10 px-2 py-0.5 text-[11px] text-primary">
                {generation.provider} · {generation.model}
              </span>
              <span className={clsx(
                'rounded-md border px-2 py-0.5 text-[11px]',
                generation.status === 'completed'
                  ? 'border-success/30 bg-success/10 text-success'
                  : generation.status === 'failed'
                    ? 'border-danger/30 bg-danger-soft text-danger'
                    : 'border-primary/30 bg-primary/10 text-primary'
              )}>
                {generation.status}
              </span>
            </div>
          </div>

          <div className="flex flex-wrap gap-2">
            <button
              onClick={() => navigator.clipboard.writeText(generation.assembled_prompt || generation.prompt).then(() => toast.success('Prompt copied'))}
              className="min-h-10 inline-flex items-center gap-2 rounded-lg border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:text-text hover:bg-surface-hover transition-colors"
            >
              <Copy size={14} />
              Copy prompt
            </button>
            <button
              onClick={() => onBranch(generation.id)}
              className="min-h-10 inline-flex items-center gap-2 rounded-lg border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:text-text hover:bg-surface-hover transition-colors"
            >
              <GitBranch size={14} />
              Branch
            </button>
            <button
              onClick={() => onRegenerate(generation.id)}
              className="min-h-10 inline-flex items-center gap-2 rounded-lg border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:text-text hover:bg-surface-hover transition-colors"
            >
              Regenerate
            </button>
            <button
              onClick={() => onSendToChat(generation)}
              className="min-h-10 inline-flex items-center gap-2 rounded-lg border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:text-text hover:bg-surface-hover transition-colors"
            >
              <MessageSquare size={14} />
              Send to Chat
            </button>
            {onGenerateAlbumArt && generation.status === 'completed' && (
              <button
                onClick={() => onGenerateAlbumArt(generation)}
                className="min-h-10 inline-flex items-center gap-2 rounded-lg border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:text-text hover:bg-surface-hover transition-colors"
                title="Generate album art for this track"
              >
                <Image size={14} />
                Album Art
              </button>
            )}
            {onSendToVideo && generation.status === 'completed' && (
              <button
                onClick={() => onSendToVideo(generation)}
                className="min-h-10 inline-flex items-center gap-2 rounded-lg border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:text-text hover:bg-surface-hover transition-colors"
                title="Translate music prompt to video and open Video Studio"
              >
                <Video size={14} />
                Make Video
              </button>
            )}
          </div>
        </div>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto p-4">
        {generation.status === 'failed' && generation.error ? (
          <div className="mb-4 rounded-xl border border-danger/30 bg-danger-soft p-3 text-sm text-danger">
            {generation.error}
          </div>
        ) : null}

        <div className="space-y-4">
          <MusicPlayer src={canUseAsset ? assetUrl : undefined} fileName={`${displayTitle}.mp3`} />
          <WaveformViewer src={canUseAsset ? assetUrl : undefined} active={isGenerating} />

          <div className="flex flex-wrap gap-1 rounded-xl border border-border bg-surface-alt p-1">
            {TABS.map((item) => (
              <button
                key={item.key}
                onClick={() => setTab(item.key)}
                className={clsx(
                  'min-h-9 rounded-lg px-2.5 text-xs font-medium transition-colors',
                  tab === item.key ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text hover:bg-surface-hover'
                )}
              >
                {item.label}
              </button>
            ))}
          </div>

          <TabBody tab={tab} generation={generation} rows={metadataRows} />
        </div>
      </div>
    </article>
  );
}

function TabBody({
  tab,
  generation,
  rows,
}: {
  tab: ResultTab;
  generation: MusicGenerationDetail;
  rows: string[][];
}) {
  if (tab === 'lyrics') {
    const text = [generation.lyrics, generation.structure].filter(Boolean).join('\n\n').trim();
    return <PreBlock>{text || 'No lyric or structure text returned yet.'}</PreBlock>;
  }
  if (tab === 'prompt') {
    return <PreBlock>{generation.assembled_prompt || generation.prompt}</PreBlock>;
  }
  if (tab === 'metadata') {
    return (
      <div className="rounded-xl border border-border bg-surface-alt p-3">
        {rows.map(([label, value]) => (
          <div key={label} className="flex items-center justify-between gap-3 border-b border-border/60 py-2 last:border-0">
            <span className="text-xs text-text-muted">{label}</span>
            <span className="min-w-0 truncate text-right text-xs text-text">{value}</span>
          </div>
        ))}
      </div>
    );
  }
  return (
    <div className="rounded-xl border border-border bg-surface-alt p-3 text-xs text-text-secondary">
      <div className="flex items-center justify-between gap-3">
        <span className="text-text-muted">Cost</span>
        <span>{generation.cost_usd != null ? `$${generation.cost_usd.toFixed(5)}` : 'Not reported'}</span>
      </div>
      <div className="mt-2 flex items-center justify-between gap-3">
        <span className="text-text-muted">Output bytes</span>
        <span>{generation.output_bytes.toLocaleString()}</span>
      </div>
    </div>
  );
}

function PreBlock({ children }: { children: string }) {
  return (
    <pre className="max-h-80 overflow-auto whitespace-pre-wrap rounded-xl border border-border bg-surface-alt p-3 text-sm leading-6 text-text-secondary">
      {children}
    </pre>
  );
}
