import { useEffect, useMemo, useState } from 'react';
import { RefreshCw, Sparkles, Trash2 } from 'lucide-react';
import { clsx } from 'clsx';
import type { MusicModel, MusicPromptForm, MusicProviderKey, MusicProvidersResponse } from '../../types/music';

const AUTO_VALUE = '__auto__';
const CUSTOM_VALUE = '__custom__';

const GENRE_OPTIONS = [
  'Pop',
  'Rock',
  'Hip-Hop',
  'R&B',
  'EDM',
  'House',
  'Techno',
  'Ambient',
  'Cinematic',
  'Jazz',
  'Classical',
  'Lo-fi',
  'Country',
  'Reggaeton',
];

const MOOD_OPTIONS = [
  'Uplifting',
  'Energetic',
  'Melancholic',
  'Dark',
  'Dreamy',
  'Romantic',
  'Aggressive',
  'Epic',
  'Calm',
  'Hopeful',
  'Nostalgic',
  'Groovy',
];

const ERA_OPTIONS = [
  '1970s',
  '1980s',
  '1990s',
  '2000s',
  '2010s',
  '2020s',
  'Retro',
  'Modern',
  'Futuristic',
  'Timeless',
];

const SCALE_OPTIONS = [
  'C Major',
  'G Major',
  'D Major',
  'A Minor',
  'E Minor',
  'D Minor',
  'Pentatonic Major',
  'Pentatonic Minor',
  'Dorian',
  'Mixolydian',
  'Phrygian',
  'Lydian',
];

const BPM_OPTIONS = ['70', '85', '100', '115', '128', '140', '160'];

const DURATION_OPTIONS = [
  '15s',
  '30s',
  '45s',
  '60s',
  '90s',
  '120s',
  '180s',
  'Loop-friendly',
];

const INSTRUMENT_OPTIONS = [
  'Piano',
  'Electric Guitar',
  'Acoustic Guitar',
  'Synth Bass',
  '808 Drums',
  'Strings',
  'Brass',
  'Woodwinds',
  'Pad Synths',
  'Arpeggiator',
  'Choir',
  'Percussion',
];

const PROVIDER_LABELS: Record<MusicProviderKey, string> = {
  openrouter: 'OpenRouter',
  gemini: 'Gemini',
  elevenlabs: 'ElevenLabs',
};

interface MusicPromptBuilderProps {
  providers: MusicProvidersResponse;
  selectedProvider: MusicProviderKey | null;
  selectedModel: string | null;
  models: MusicModel[];
  promptForm: MusicPromptForm;
  isGenerating: boolean;
  progressMessage?: string;
  error?: string | null;
  onProviderChange: (provider: MusicProviderKey) => void;
  onModelChange: (model: string) => void;
  onRefreshModels: () => void;
  onPromptField: <K extends keyof MusicPromptForm>(key: K, value: MusicPromptForm[K]) => void;
  onOption: <K extends keyof MusicPromptForm['options']>(key: K, value: MusicPromptForm['options'][K]) => void;
  onGenerate: () => void;
  onClear: () => void;
  onStop: () => void;
}

export function MusicPromptBuilder({
  providers,
  selectedProvider,
  selectedModel,
  models,
  promptForm,
  isGenerating,
  progressMessage,
  error,
  onProviderChange,
  onModelChange,
  onRefreshModels,
  onPromptField,
  onOption,
  onGenerate,
  onClear,
  onStop,
}: MusicPromptBuilderProps) {
  const availableProviders = (['openrouter', 'gemini', 'elevenlabs'] as MusicProviderKey[]).filter((provider) => providers[provider]);
  const selectedModelMeta = models.find((model) => model.id === selectedModel);
  const canGenerate = Boolean(selectedProvider && selectedModel && promptForm.prompt.trim() && !isGenerating);

  return (
    <div className="space-y-4">
      <section className="rounded-xl border border-border bg-surface-raised p-3">
        <div className="flex items-center justify-between gap-2">
          <h2 className="text-xs font-semibold uppercase tracking-wide text-text-muted">Generate</h2>
          <button
            onClick={onRefreshModels}
            disabled={!selectedProvider || isGenerating}
            className="min-h-8 min-w-8 inline-flex items-center justify-center rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors disabled:opacity-40"
            aria-label="Refresh music models"
            title="Refresh models"
          >
            <RefreshCw size={13} />
          </button>
        </div>

        <div className="mt-3 space-y-3">
          <div>
            <label className="mb-1.5 block text-xs font-medium text-text-muted">Provider</label>
            {availableProviders.length <= 1 ? (
              <div className="min-h-10 rounded-xl border border-border bg-surface-alt px-3 text-sm text-text flex items-center">
                {availableProviders[0] ? PROVIDER_LABELS[availableProviders[0]] : 'No music provider configured'}
              </div>
            ) : (
              <select
                aria-label="Music provider"
                value={selectedProvider || ''}
                onChange={(event) => onProviderChange(event.target.value as MusicProviderKey)}
                className="w-full min-h-10 rounded-xl border border-border bg-surface-alt px-3 text-sm text-text focus:outline-none focus:border-primary/50"
              >
                {availableProviders.map((provider) => (
                  <option key={provider} value={provider}>{PROVIDER_LABELS[provider]}</option>
                ))}
              </select>
            )}
          </div>

          <div>
            <label className="mb-1.5 block text-xs font-medium text-text-muted">Model</label>
            <select
              aria-label="Music model"
              value={selectedModel || ''}
              onChange={(event) => onModelChange(event.target.value)}
              disabled={!selectedProvider || models.length === 0}
              className="w-full min-h-10 rounded-xl border border-border bg-surface-alt px-3 text-sm text-text focus:outline-none focus:border-primary/50 disabled:opacity-50"
            >
              <option value="">{models.length === 0 ? 'No music models available' : 'Select model'}</option>
              {models.map((model) => (
                <option key={model.id} value={model.id}>{model.name || model.id}</option>
              ))}
            </select>
            {selectedModelMeta && (
              <div className="mt-2 flex flex-wrap gap-1.5">
                {(selectedModelMeta.output_modalities || ['audio']).map((modality) => (
                  <span key={modality} className="rounded-md border border-primary/20 bg-primary/10 px-1.5 py-0.5 text-[10px] text-primary">
                    {modality}
                  </span>
                ))}
                <span className="rounded-md border border-border bg-surface px-1.5 py-0.5 text-[10px] text-text-muted">
                  {selectedModelMeta.supports_streaming ? 'streaming' : 'sync'}
                </span>
              </div>
            )}
          </div>
        </div>
      </section>

      {availableProviders.length === 0 && (
        <div className="rounded-xl border border-warning/30 bg-warning/10 px-3 py-2 text-xs text-warning">
          Add an enabled OpenRouter, Gemini, or ElevenLabs provider profile in Settings before generating music.
        </div>
      )}

      <section className="rounded-xl border border-border bg-surface-raised p-3 space-y-3">
        <div>
          <label className="mb-1.5 block text-xs font-medium text-text-muted">Prompt</label>
          <textarea
            value={promptForm.prompt}
            onChange={(event) => onPromptField('prompt', event.target.value)}
            placeholder="Describe the track..."
            rows={5}
            className="w-full resize-none rounded-xl border border-border bg-surface-alt px-3 py-2 text-sm text-text placeholder:text-text-muted/50 focus:outline-none focus:border-primary/50"
          />
        </div>

        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-1 2xl:grid-cols-2">
          <SelectWithCustom
            label="Genre"
            value={promptForm.options.genre || ''}
            choices={GENRE_OPTIONS}
            customPlaceholder="e.g. hyperpop, afrobeat"
            onChange={(value) => onOption('genre', value || undefined)}
          />
          <SelectWithCustom
            label="Mood"
            value={promptForm.options.mood || ''}
            choices={MOOD_OPTIONS}
            customPlaceholder="e.g. bittersweet, triumphant"
            onChange={(value) => onOption('mood', value || undefined)}
          />
          <SelectWithCustom
            label="Era"
            value={promptForm.options.era || ''}
            choices={ERA_OPTIONS}
            customPlaceholder="e.g. late 60s psych"
            onChange={(value) => onOption('era', value || undefined)}
          />
          <SelectWithCustom
            label="Key / scale"
            value={promptForm.options.scale || ''}
            choices={SCALE_OPTIONS}
            customPlaceholder="e.g. B harmonic minor"
            onChange={(value) => onOption('scale', value || undefined)}
          />
          <SelectWithCustom
            label="BPM"
            value={promptForm.options.bpm ? String(promptForm.options.bpm) : ''}
            choices={BPM_OPTIONS}
            customPlaceholder="e.g. 132"
            customInputType="number"
            onChange={(value) => {
              const parsed = value ? Number(value) : undefined;
              onOption('bpm', Number.isFinite(parsed) ? parsed : undefined);
            }}
          />
          <SelectWithCustom
            label="Duration"
            value={promptForm.options.duration || ''}
            choices={DURATION_OPTIONS}
            customPlaceholder="e.g. 75s"
            onChange={(value) => onOption('duration', value || undefined)}
          />
        </div>

        <SelectWithCustom
          label="Instruments"
          value={(promptForm.options.instruments || []).join(', ')}
          choices={INSTRUMENT_OPTIONS}
          customPlaceholder="Comma separated, e.g. sitar, tabla, flute"
          onChange={(value) => {
            const list = value
              .split(',')
              .map((item) => item.trim())
              .filter(Boolean);
            onOption('instruments', list.length > 0 ? list : undefined);
          }}
        />

        <div>
          <label className="mb-1.5 block text-xs font-medium text-text-muted">Vocals</label>
          <select
            value={promptForm.vocal_mode}
            onChange={(event) => {
              const value = event.target.value as MusicPromptForm['vocal_mode'];
              onPromptField('vocal_mode', value);
              onPromptField('instrumental', value === 'instrumental');
            }}
            className="w-full min-h-10 rounded-xl border border-border bg-surface-alt px-3 text-sm text-text focus:outline-none focus:border-primary/50"
          >
            <option value="auto">Auto</option>
            <option value="instrumental">Instrumental only</option>
            <option value="lyrics">Generate lyrics</option>
            <option value="custom">Use my lyrics</option>
          </select>
        </div>

        {promptForm.vocal_mode === 'custom' && (
          <div>
            <label className="mb-1.5 block text-xs font-medium text-text-muted">Lyrics</label>
            <textarea
              value={promptForm.lyrics}
              onChange={(event) => onPromptField('lyrics', event.target.value)}
              placeholder="[Intro]\n[Verse]\n[Chorus]"
              rows={5}
              className="w-full resize-none rounded-xl border border-border bg-surface-alt px-3 py-2 text-sm text-text placeholder:text-text-muted/50 focus:outline-none focus:border-primary/50"
            />
          </div>
        )}

        <TextArea label="Structure / section plan" value={promptForm.options.structure || ''} onChange={(value) => onOption('structure', value)} />
        <TextArea label="Production notes" value={promptForm.options.production_notes || ''} onChange={(value) => onOption('production_notes', value)} />
        <TextInput label="Negative steer" value={promptForm.options.negative_steer || ''} onChange={(value) => onOption('negative_steer', value)} />

        <details className="rounded-xl border border-border bg-surface-alt px-3 py-2">
          <summary className="cursor-pointer text-xs font-medium text-text-muted">Advanced</summary>
          <div className="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-1 2xl:grid-cols-2">
            <TextInput label="Seed" value={promptForm.options.seed ? String(promptForm.options.seed) : ''} type="number" onChange={(value) => onOption('seed', value ? Number(value) : undefined)} />
            <TextInput label="Temperature" value={promptForm.options.temperature ? String(promptForm.options.temperature) : ''} type="number" step="0.1" onChange={(value) => onOption('temperature', value ? Number(value) : undefined)} />
            <TextInput label="Language" value={promptForm.options.language || ''} onChange={(value) => onOption('language', value)} />
            <TextInput label="Energy curve" value={promptForm.options.energy_curve || ''} onChange={(value) => onOption('energy_curve', value)} />
          </div>
        </details>
      </section>

      <div className="sticky bottom-0 z-10 -mx-1 rounded-xl border border-border bg-surface-raised/95 p-2 backdrop-blur">
        {progressMessage && (
          <p className="mb-2 rounded-lg bg-primary/10 px-2 py-1 text-[11px] text-primary">{progressMessage}</p>
        )}
        {error && (
          <p className="mb-2 rounded-lg bg-danger-soft px-2 py-1 text-[11px] text-danger">{error}</p>
        )}
        <div className="flex gap-2">
          <button
            onClick={isGenerating ? onStop : onGenerate}
            disabled={!canGenerate && !isGenerating}
            className={clsx(
              'min-h-11 flex-1 rounded-xl text-sm font-medium transition-colors inline-flex items-center justify-center gap-2',
              canGenerate || isGenerating
                ? 'bg-primary text-white hover:bg-primary-hover shadow-glow'
                : 'bg-surface-hover text-text-muted/40 cursor-not-allowed'
            )}
          >
            <Sparkles size={15} />
            {isGenerating ? 'Stop' : 'Generate Track'}
          </button>
          <button
            onClick={onClear}
            disabled={isGenerating}
            className="min-h-11 min-w-11 inline-flex items-center justify-center rounded-xl border border-border bg-surface-alt text-text-muted hover:text-text hover:bg-surface-hover transition-colors disabled:opacity-40"
            aria-label="Clear prompt"
            title="Clear"
          >
            <Trash2 size={15} />
          </button>
        </div>
      </div>
    </div>
  );
}

function TextInput({
  label,
  value,
  onChange,
  type = 'text',
  step,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  type?: string;
  step?: string;
}) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-xs font-medium text-text-muted">{label}</span>
      <input
        type={type}
        step={step}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="w-full min-h-10 rounded-xl border border-border bg-surface-alt px-3 text-sm text-text placeholder:text-text-muted/50 focus:outline-none focus:border-primary/50"
      />
    </label>
  );
}

function TextArea({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-xs font-medium text-text-muted">{label}</span>
      <textarea
        value={value}
        onChange={(event) => onChange(event.target.value)}
        rows={3}
        className="w-full resize-none rounded-xl border border-border bg-surface-alt px-3 py-2 text-sm text-text placeholder:text-text-muted/50 focus:outline-none focus:border-primary/50"
      />
    </label>
  );
}

function SelectWithCustom({
  label,
  value,
  choices,
  onChange,
  customPlaceholder,
  customInputType = 'text',
}: {
  label: string;
  value: string;
  choices: string[];
  onChange: (value: string) => void;
  customPlaceholder?: string;
  customInputType?: 'text' | 'number';
}) {
  const normalized = value.trim();
  const normalizedChoices = useMemo(() => new Set(choices.map((choice) => choice.toLowerCase())), [choices]);
  const [mode, setMode] = useState<string>(AUTO_VALUE);

  useEffect(() => {
    if (!normalized) {
      setMode(AUTO_VALUE);
      return;
    }
    if (normalizedChoices.has(normalized.toLowerCase())) {
      setMode(normalized);
      return;
    }
    setMode(CUSTOM_VALUE);
  }, [normalized, normalizedChoices]);

  return (
    <div className="space-y-1.5">
      <label className="mb-0 block text-xs font-medium text-text-muted">{label}</label>
      <select
        value={mode}
        onChange={(event) => {
          const next = event.target.value;
          setMode(next);
          if (next === AUTO_VALUE) {
            onChange('');
            return;
          }
          if (next === CUSTOM_VALUE) {
            if (!normalizedChoices.has(normalized.toLowerCase())) {
              onChange(normalized);
            } else {
              onChange('');
            }
            return;
          }
          onChange(next);
        }}
        className="w-full min-h-10 rounded-xl border border-border bg-surface-alt px-3 text-sm text-text focus:outline-none focus:border-primary/50"
      >
        <option value={AUTO_VALUE}>Auto</option>
        {choices.map((choice) => (
          <option key={choice} value={choice}>{choice}</option>
        ))}
        <option value={CUSTOM_VALUE}>Custom...</option>
      </select>
      {mode === CUSTOM_VALUE && (
        <input
          type={customInputType}
          value={normalizedChoices.has(normalized.toLowerCase()) ? '' : value}
          onChange={(event) => onChange(event.target.value)}
          placeholder={customPlaceholder || 'Enter custom value'}
          className="w-full min-h-10 rounded-xl border border-border bg-surface-alt px-3 text-sm text-text placeholder:text-text-muted/50 focus:outline-none focus:border-primary/50"
        />
      )}
    </div>
  );
}
