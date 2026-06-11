import { useRef, useState } from 'react';
import { Download, Loader2, Plus, Scissors, Sparkles, Subtitles, Trash2, Upload } from 'lucide-react';
import { useVideoStudioStore } from '../../stores/videoStudio';
import { CAPTION_PRESETS } from './captions/captionUtils';

function toSeconds(ms: number): string {
  return (Math.round(ms / 100) / 10).toFixed(1);
}

export function VideoCaptionPanel() {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const playheadMs = useVideoStudioStore((state) => state.playheadMs);
  const selectedClipId = useVideoStudioStore((state) => state.selectedClipId);
  const selectClip = useVideoStudioStore((state) => state.selectClip);
  const setPlayhead = useVideoStudioStore((state) => state.setPlayhead);
  const addCaptionSegment = useVideoStudioStore((state) => state.addCaptionSegment);
  const importCaptions = useVideoStudioStore((state) => state.importCaptions);
  const exportCaptions = useVideoStudioStore((state) => state.exportCaptions);
  const mergeCaptionClipWithNext = useVideoStudioStore((state) => state.mergeCaptionClipWithNext);
  const applyCaptionPreset = useVideoStudioStore((state) => state.applyCaptionPreset);
  const trimClip = useVideoStudioStore((state) => state.trimClip);
  const updateClipText = useVideoStudioStore((state) => state.updateClipText);
  const deleteClip = useVideoStudioStore((state) => state.deleteClip);
  const splitClipAt = useVideoStudioStore((state) => state.splitClipAt);

  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [importing, setImporting] = useState(false);

  const segments = (timeline?.tracks || [])
    .filter((track) => track.type === 'caption')
    .flatMap((track) => track.clips.map((clip) => ({ clip, trackId: track.id })))
    .sort((a, b) => a.clip.start_ms - b.clip.start_ms);

  const handleImportFile = async (list: FileList | null) => {
    if (!list || list.length === 0) return;
    setImporting(true);
    try {
      const raw = await list[0].text();
      await importCaptions(raw);
    } finally {
      setImporting(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const downloadCaptions = (format: 'srt' | 'vtt') => {
    const content = exportCaptions(format);
    if (!content) return;
    const blob = new Blob([content], { type: format === 'srt' ? 'application/x-subrip' : 'text/vtt' });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = `captions.${format}`;
    anchor.click();
    URL.revokeObjectURL(url);
  };

  if (!timeline) return null;

  return (
    <section className="rounded-lg border border-border bg-surface p-3">
      <div className="mb-2 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Subtitles size={14} className="text-primary" />
          <h2 className="text-sm font-semibold text-text">Captions</h2>
        </div>
        <span className="text-[11px] text-text-muted">{segments.length}</span>
      </div>
      <div className="mb-2 grid grid-cols-2 gap-2">
        <button
          className="inline-flex min-h-8 items-center justify-center gap-1 rounded-md border border-border bg-surface-alt px-2 text-xs text-text-secondary hover:text-text"
          onClick={() => { void addCaptionSegment(); }}
          title="Add a caption segment at the playhead"
        >
          <Plus size={12} />
          Add at playhead
        </button>
        <button
          className="inline-flex min-h-8 items-center justify-center gap-1 rounded-md border border-border bg-surface-alt px-2 text-xs text-text-secondary hover:text-text disabled:opacity-45"
          onClick={() => fileInputRef.current?.click()}
          disabled={importing}
          title="Import an .srt or .vtt file"
        >
          {importing ? <Loader2 size={12} className="animate-spin" /> : <Upload size={12} />}
          Import SRT/VTT
        </button>
        <input
          ref={fileInputRef}
          type="file"
          accept=".srt,.vtt,text/vtt,application/x-subrip"
          className="hidden"
          onChange={(event) => { void handleImportFile(event.target.files); }}
        />
        <button
          className="inline-flex min-h-8 items-center justify-center gap-1 rounded-md border border-border bg-surface-alt px-2 text-xs text-text-secondary hover:text-text"
          onClick={() => downloadCaptions('srt')}
          title="Download captions as SRT"
        >
          <Download size={12} />
          Export SRT
        </button>
        <button
          className="inline-flex min-h-8 items-center justify-center gap-1 rounded-md border border-border bg-surface-alt px-2 text-xs text-text-secondary hover:text-text"
          onClick={() => downloadCaptions('vtt')}
          title="Download captions as WebVTT"
        >
          <Download size={12} />
          Export VTT
        </button>
      </div>
      <div className="mb-2 flex flex-wrap items-center gap-1">
        <span className="text-[10px] text-text-muted">Style:</span>
        {CAPTION_PRESETS.map((preset) => (
          <button
            key={preset.key}
            className="rounded-md border border-border bg-surface-alt px-2 py-1 text-[10px] text-text-muted hover:text-text"
            onClick={() => { void applyCaptionPreset(preset.key); }}
            title={`Apply the ${preset.label} style to all caption clips`}
          >
            {preset.label}
          </button>
        ))}
        <button
          className="inline-flex items-center gap-1 rounded-md border border-border bg-surface-alt px-2 py-1 text-[10px] text-text-muted opacity-50"
          disabled
          title="AI caption generation requires a transcription provider — coming soon"
        >
          <Sparkles size={10} />
          AI captions — soon
        </button>
      </div>
      {segments.length === 0 ? (
        <p className="rounded-md border border-dashed border-border bg-surface-alt px-2 py-2 text-[11px] text-text-muted">
          No captions yet. Add a segment at the playhead or import an SRT/VTT file.
        </p>
      ) : (
        <div className="max-h-72 space-y-1.5 overflow-y-auto pr-0.5">
          {segments.map(({ clip, trackId }, index) => {
            const endMs = clip.start_ms + clip.duration_ms;
            const playheadInside = playheadMs > clip.start_ms && playheadMs < endMs;
            const selected = clip.id === selectedClipId;
            return (
              <div
                key={clip.id}
                className={`rounded-md border px-2 py-1.5 ${selected ? 'border-primary/50 bg-primary/5' : 'border-border bg-surface-alt'}`}
                onClick={() => {
                  selectClip(clip.id, trackId);
                  setPlayhead(clip.start_ms);
                }}
              >
                <textarea
                  key={`text-${clip.id}-${clip.text?.text || ''}`}
                  rows={Math.min(3, (clip.text?.text || '').split('\n').length)}
                  defaultValue={clip.text?.text || ''}
                  onClick={(event) => event.stopPropagation()}
                  onBlur={(event) => {
                    const value = event.target.value;
                    if (value !== (clip.text?.text || '')) void updateClipText(clip.id, { text: value });
                  }}
                  className="w-full resize-none rounded border border-transparent bg-transparent text-xs text-text focus:border-border focus:bg-surface focus:outline-none"
                  aria-label={`Caption ${index + 1} text`}
                />
                <div className="mt-1 flex items-center gap-1" onClick={(event) => event.stopPropagation()}>
                  <input
                    key={`start-${clip.id}-${clip.start_ms}`}
                    type="number"
                    min={0}
                    step={0.1}
                    defaultValue={toSeconds(clip.start_ms)}
                    onBlur={(event) => {
                      const startMs = Math.max(0, Math.round(Number(event.target.value) * 1000));
                      if (Number.isFinite(startMs) && startMs !== clip.start_ms) {
                        void trimClip(clip.id, { start_ms: startMs });
                      }
                    }}
                    onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }}
                    className="w-14 rounded border border-border bg-surface px-1 py-0.5 text-[10px] text-text-secondary"
                    aria-label={`Caption ${index + 1} start seconds`}
                    title="Start (seconds)"
                  />
                  <span className="text-[10px] text-text-muted">→</span>
                  <input
                    key={`end-${clip.id}-${endMs}`}
                    type="number"
                    min={0}
                    step={0.1}
                    defaultValue={toSeconds(endMs)}
                    onBlur={(event) => {
                      const newEndMs = Math.round(Number(event.target.value) * 1000);
                      const duration = newEndMs - clip.start_ms;
                      if (Number.isFinite(duration) && duration >= 100 && newEndMs !== endMs) {
                        void trimClip(clip.id, { duration_ms: duration, trim_out_ms: clip.trim_in_ms + duration });
                      }
                    }}
                    onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }}
                    className="w-14 rounded border border-border bg-surface px-1 py-0.5 text-[10px] text-text-secondary"
                    aria-label={`Caption ${index + 1} end seconds`}
                    title="End (seconds)"
                  />
                  <span className="flex-1" />
                  <button
                    className="rounded p-1 text-text-muted hover:text-text disabled:cursor-not-allowed disabled:opacity-35"
                    disabled={!playheadInside}
                    onClick={() => { void splitClipAt(clip.id, playheadMs); }}
                    title={playheadInside ? 'Split at playhead' : 'Move the playhead inside this caption to split'}
                    aria-label={`Split caption ${index + 1} at playhead`}
                  >
                    <Scissors size={11} />
                  </button>
                  <button
                    className="rounded p-1 text-[10px] text-text-muted hover:text-text disabled:cursor-not-allowed disabled:opacity-35"
                    disabled={index >= segments.length - 1}
                    onClick={() => { void mergeCaptionClipWithNext(clip.id); }}
                    title="Merge with the next caption"
                    aria-label={`Merge caption ${index + 1} with next`}
                  >
                    ⇣⇡
                  </button>
                  <button
                    className="rounded p-1 text-text-muted hover:text-red-400"
                    onClick={() => { void deleteClip(clip.id); }}
                    title="Delete caption"
                    aria-label={`Delete caption ${index + 1}`}
                  >
                    <Trash2 size={11} />
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
}
