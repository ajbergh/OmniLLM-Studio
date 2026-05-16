import { FileAudio } from 'lucide-react';
import type { MusicGenerationDetail } from '../../types/music';

interface MusicAssetDetailsProps {
  generation?: MusicGenerationDetail;
}

export function MusicAssetDetails({ generation }: MusicAssetDetailsProps) {
  if (!generation) {
    return (
      <section className="border-t border-border p-3 text-sm text-text-muted">
        Select a generation to inspect asset details.
      </section>
    );
  }

  return (
    <section className="border-t border-border p-3">
      <div className="mb-3 flex items-center gap-2">
        <FileAudio size={15} className="text-primary" />
        <h2 className="text-xs font-semibold uppercase tracking-wide text-text-muted">Asset</h2>
      </div>
      <div className="space-y-2 text-xs">
        <DetailRow label="Generation" value={generation.id.slice(0, 8)} />
        <DetailRow label="Provider" value={generation.provider} />
        <DetailRow label="Model" value={generation.model} />
        <DetailRow label="MIME" value={generation.mime_type || 'pending'} />
        <DetailRow label="Bytes" value={generation.output_bytes ? generation.output_bytes.toLocaleString() : '0'} />
        <DetailRow label="Created" value={new Date(generation.created_at).toLocaleString()} />
      </div>
    </section>
  );
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <span className="text-text-muted">{label}</span>
      <span className="min-w-0 truncate text-right text-text">{value}</span>
    </div>
  );
}
