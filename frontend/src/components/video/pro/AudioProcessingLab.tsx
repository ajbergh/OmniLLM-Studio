import { useEffect, useState } from 'react';
import { RotateCcw, X } from 'lucide-react';
import { toast } from 'sonner';
import { useVideoStudioStore } from '../../../stores/videoStudio';

type EQPreset = 'none' | 'voice' | 'warm' | 'bright';
type ChannelMode = 'source' | 'mono' | 'stereo';

interface AudioProcessingSettings {
  normalize: boolean;
  target_lufs: number;
  denoise: boolean;
  eq_preset: EQPreset;
  compressor: boolean;
  limiter: boolean;
  channels: ChannelMode;
}

const defaults: AudioProcessingSettings = {
  normalize: true,
  target_lufs: -16,
  denoise: false,
  eq_preset: 'voice',
  compressor: true,
  limiter: true,
  channels: 'stereo',
};

function normalizeSettings(value: unknown): AudioProcessingSettings {
  if (!value || typeof value !== 'object') return defaults;
  const input = value as Partial<AudioProcessingSettings>;
  const eq = ['none', 'voice', 'warm', 'bright'].includes(input.eq_preset || '')
    ? input.eq_preset as EQPreset
    : defaults.eq_preset;
  const channels = ['source', 'mono', 'stereo'].includes(input.channels || '')
    ? input.channels as ChannelMode
    : defaults.channels;
  return {
    normalize: input.normalize ?? defaults.normalize,
    target_lufs: Math.max(-30, Math.min(-5, Number(input.target_lufs ?? defaults.target_lufs))),
    denoise: input.denoise ?? defaults.denoise,
    eq_preset: eq,
    compressor: input.compressor ?? defaults.compressor,
    limiter: input.limiter ?? defaults.limiter,
    channels,
  };
}

export function AudioProcessingLab({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const timeline = useVideoStudioStore((state) => state.timeline);
  const saveTimeline = useVideoStudioStore((state) => state.saveTimeline);
  const [settings, setSettings] = useState<AudioProcessingSettings>(defaults);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setSettings(normalizeSettings(timeline?.metadata?.render_audio_processing));
    setError(null);
  }, [open, timeline]);

  if (!open) return null;

  const commit = async () => {
    if (!timeline || saving) return;
    const target = Number(settings.target_lufs);
    if (!Number.isFinite(target) || target < -30 || target > -5) {
      setError('Target loudness must be between -30 and -5 LUFS.');
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const next = structuredClone(timeline);
      next.metadata = {
        ...(next.metadata || {}),
        render_audio_processing: { ...settings, target_lufs: target },
      };
      await saveTimeline(next);
      toast.success('Audio processing chain saved');
      onClose();
    } catch (reason) {
      const message = reason instanceof Error ? reason.message : 'Could not save audio processing';
      setError(message);
      toast.error(message);
    } finally {
      setSaving(false);
    }
  };

  const clear = async () => {
    if (!timeline || saving) return;
    setSaving(true);
    setError(null);
    try {
      const next = structuredClone(timeline);
      next.metadata = { ...(next.metadata || {}) };
      delete next.metadata.render_audio_processing;
      await saveTimeline(next);
      setSettings(defaults);
      toast.success('Audio processing disabled');
      onClose();
    } catch (reason) {
      const message = reason instanceof Error ? reason.message : 'Could not disable audio processing';
      setError(message);
      toast.error(message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-[80] flex items-center justify-center bg-black/60 p-4">
      <div className="w-full max-w-2xl rounded-xl border border-border bg-surface p-4 shadow-2xl">
        <div className="flex items-start justify-between gap-3">
          <div>
            <h2 className="font-semibold text-text">Audio Processing</h2>
            <p className="mt-1 text-xs text-text-muted">
              Applies a deterministic FFmpeg processing chain to the final mixed audio during export.
            </p>
          </div>
          <button type="button" onClick={onClose} disabled={saving} aria-label="Close audio processing" className="rounded p-1.5 hover:bg-surface-alt">
            <X size={18} />
          </button>
        </div>

        <div className="mt-4 grid gap-3 sm:grid-cols-2">
          {([
            ['denoise', 'Noise reduction', 'High-pass filtering and FFT denoise for steady background noise.'],
            ['compressor', 'Compressor', 'Reduces large level differences before loudness normalization.'],
            ['normalize', 'LUFS normalization', 'Targets consistent perceived loudness across the export.'],
            ['limiter', 'Peak limiter', 'Prevents final peaks from clipping after gain processing.'],
          ] as const).map(([key, label, description]) => (
            <label key={key} className="flex items-start gap-2 rounded border border-border p-3 text-sm">
              <input type="checkbox" checked={settings[key]} onChange={(event) => setSettings((current) => ({ ...current, [key]: event.target.checked }))} />
              <span>
                <strong className="block text-text">{label}</strong>
                <span className="mt-0.5 block text-xs text-text-muted">{description}</span>
              </span>
            </label>
          ))}

          <label className="text-xs text-text-muted">
            Target loudness (LUFS)
            <input type="number" min={-30} max={-5} step={0.5} disabled={!settings.normalize} className="mt-1 w-full rounded border border-border bg-surface-alt p-2 text-text disabled:opacity-50" value={settings.target_lufs} onChange={(event) => setSettings((current) => ({ ...current, target_lufs: Number(event.target.value) }))} />
            <span className="mt-1 block">Typical targets: −16 LUFS for web video, −14 LUFS for louder social delivery.</span>
          </label>

          <label className="text-xs text-text-muted">
            EQ preset
            <select className="mt-1 w-full rounded border border-border bg-surface-alt p-2 text-text" value={settings.eq_preset} onChange={(event) => setSettings((current) => ({ ...current, eq_preset: event.target.value as EQPreset }))}>
              <option value="none">None</option>
              <option value="voice">Voice clarity</option>
              <option value="warm">Warm</option>
              <option value="bright">Bright</option>
            </select>
          </label>

          <label className="text-xs text-text-muted">
            Output channels
            <select className="mt-1 w-full rounded border border-border bg-surface-alt p-2 text-text" value={settings.channels} onChange={(event) => setSettings((current) => ({ ...current, channels: event.target.value as ChannelMode }))}>
              <option value="source">Preserve source layout</option>
              <option value="mono">Mono</option>
              <option value="stereo">Stereo</option>
            </select>
          </label>
        </div>

        <p className="mt-4 rounded border border-border bg-surface-alt p-2 text-xs text-text-muted">
          Processing is applied after clip volume, fades, keyframes, and the multi-track mix. Results depend on the FFmpeg filters available in the installed build.
        </p>
        {error && <p role="alert" className="mt-2 text-xs text-red-400">{error}</p>}

        <div className="mt-4 flex flex-wrap justify-between gap-2">
          <button type="button" onClick={() => void clear()} disabled={saving || !timeline?.metadata?.render_audio_processing} className="inline-flex items-center gap-1.5 rounded border border-border px-3 py-2 text-xs text-text-secondary disabled:opacity-40">
            <RotateCcw size={13} />Disable processing
          </button>
          <button type="button" onClick={() => void commit()} disabled={saving || !timeline} className="rounded bg-primary px-3 py-2 text-xs font-semibold text-black disabled:opacity-40">
            {saving ? 'Saving…' : 'Save processing chain'}
          </button>
        </div>
      </div>
    </div>
  );
}
