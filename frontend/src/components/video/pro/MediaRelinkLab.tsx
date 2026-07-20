import { useMemo, useState } from 'react';
import { Download, Files, Link2, X } from 'lucide-react';
import { useVideoStudioStore } from '../../../stores/videoStudio';
import { downloadProjectManifest, projectMediaReferences, replaceTimelineAsset } from './mediaTools';

export function MediaRelinkLab({ open, onClose }: { open: boolean; onClose: () => void }) {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const assets = useVideoStudioStore((state) => state.assets);
  const [sourceAssetId, setSourceAssetId] = useState('');
  const [replacementAssetId, setReplacementAssetId] = useState('');
  const references = useMemo(() => projectMediaReferences(timeline, assets), [assets, timeline]);
  const missing = references.filter((reference) => reference.missing);

  if (!open) return null;

  const replace = async () => {
    const changed = await replaceTimelineAsset(sourceAssetId, replacementAssetId);
    if (changed) {
      setSourceAssetId('');
      setReplacementAssetId('');
    }
  };

  return (
    <div className="fixed inset-0 z-[120] flex items-center justify-center bg-black/70 p-4" role="dialog" aria-modal="true" aria-label="Media relink lab">
      <div className="flex max-h-[calc(100vh-2rem)] w-[min(46rem,100%)] flex-col overflow-hidden rounded-xl border border-border bg-surface-raised shadow-2xl">
        <div className="flex min-h-12 items-center gap-2 border-b border-border px-3">
          <Files size={15} className="text-primary" />
          <div className="flex-1 text-sm font-semibold text-text">Media Relink & Project Manifest</div>
          <button type="button" onClick={onClose} className="rounded p-1.5 text-text-muted hover:bg-surface-alt hover:text-text" aria-label="Close media relink lab"><X size={16} /></button>
        </div>
        <div className="min-h-0 flex-1 space-y-4 overflow-y-auto p-4">
          <section className="rounded-lg border border-border bg-surface p-3">
            <div className="flex items-start gap-2">
              <Link2 size={14} className="mt-0.5 text-primary" />
              <div className="min-w-0 flex-1">
                <h3 className="text-xs font-semibold text-text">Replace media while preserving edits</h3>
                <p className="mt-1 text-[10px] leading-relaxed text-text-muted">
                  Every unlocked clip referencing the source asset is switched to the replacement asset. Timeline timing, trim, transforms, effects, transitions, captions, and keyframes remain unchanged.
                </p>
              </div>
            </div>
            <div className="mt-3 grid gap-2 sm:grid-cols-[1fr_1fr_auto]">
              <label className="text-[10px] text-text-muted">Source reference
                <select
                  value={sourceAssetId}
                  onChange={(event) => setSourceAssetId(event.target.value)}
                  className="mt-1 min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
                  aria-label="Source timeline media"
                >
                  <option value="">Choose source…</option>
                  {references.map((reference) => (
                    <option key={reference.asset_id} value={reference.asset_id}>
                      {reference.missing ? '[Missing] ' : ''}{reference.asset?.file_name || reference.asset_id} · {reference.clip_count} clip{reference.clip_count === 1 ? '' : 's'}
                    </option>
                  ))}
                </select>
              </label>
              <label className="text-[10px] text-text-muted">Replacement project asset
                <select
                  value={replacementAssetId}
                  onChange={(event) => setReplacementAssetId(event.target.value)}
                  className="mt-1 min-h-9 w-full rounded-md border border-border bg-surface-alt px-2 text-xs text-text"
                  aria-label="Replacement project media"
                >
                  <option value="">Choose replacement…</option>
                  {assets.filter((asset) => asset.id !== sourceAssetId).map((asset) => (
                    <option key={asset.id} value={asset.id}>{asset.file_name}</option>
                  ))}
                </select>
              </label>
              <button
                type="button"
                onClick={() => { void replace(); }}
                disabled={!sourceAssetId || !replacementAssetId}
                className="min-h-9 self-end rounded-md bg-primary px-3 text-xs font-semibold text-black disabled:cursor-not-allowed disabled:opacity-40"
              >
                Replace
              </button>
            </div>
          </section>

          <section className="rounded-lg border border-border bg-surface p-3">
            <div className="flex items-center justify-between gap-2">
              <div>
                <h3 className="text-xs font-semibold text-text">Reference health</h3>
                <p className="mt-1 text-[10px] text-text-muted">{references.length} referenced asset{references.length === 1 ? '' : 's'} · {missing.length} missing</p>
              </div>
              <span className={`rounded-full px-2 py-1 text-[9px] font-semibold uppercase tracking-wide ${missing.length ? 'bg-red-400/10 text-red-300' : 'bg-emerald-400/10 text-emerald-300'}`}>
                {missing.length ? 'Relink required' : 'All linked'}
              </span>
            </div>
            <div className="mt-2 max-h-56 space-y-1 overflow-y-auto">
              {references.map((reference) => (
                <div key={reference.asset_id} className="flex items-center gap-2 rounded-md border border-border bg-surface-alt/70 px-2 py-1.5">
                  <span className={`h-2 w-2 rounded-full ${reference.missing ? 'bg-red-400' : 'bg-emerald-400'}`} />
                  <span className="min-w-0 flex-1 truncate text-[10px] text-text-secondary">{reference.asset?.file_name || reference.asset_id}</span>
                  <span className="text-[9px] text-text-muted">{reference.clip_count} clip{reference.clip_count === 1 ? '' : 's'}</span>
                  <button type="button" onClick={() => setSourceAssetId(reference.asset_id)} className="rounded border border-border bg-surface px-1.5 py-1 text-[9px] text-text-muted hover:text-text">Select</button>
                </div>
              ))}
              {references.length === 0 && <div className="rounded-md border border-dashed border-border p-3 text-center text-[10px] text-text-muted">No media references in this timeline.</div>}
            </div>
          </section>

          <section className="rounded-lg border border-border bg-surface p-3">
            <h3 className="text-xs font-semibold text-text">Portable project manifest</h3>
            <p className="mt-1 text-[10px] leading-relaxed text-text-muted">
              Download project, timeline, renderer-capability, and asset metadata for diagnostics or migration planning. The manifest intentionally does not embed media file bytes.
            </p>
            <button type="button" onClick={downloadProjectManifest} className="mt-2 inline-flex min-h-9 items-center gap-2 rounded-md border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:text-text">
              <Download size={12} /> Download manifest
            </button>
          </section>
        </div>
      </div>
    </div>
  );
}
