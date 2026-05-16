import { RefreshCw, Sparkles, Trash2 } from 'lucide-react';
import { clsx } from 'clsx';
import type { MusicModel, MusicPromptForm, MusicProviderKey, MusicProvidersResponse } from '../../types/music';

const PROVIDER_LABELS: Record<MusicProviderKey, string> = {
  openrouter: 'OpenRouter',
  gemini: 'Gemini',
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
  const availableProviders = (['openrouter', 'gemini'] as MusicProviderKey[]).filter((provider) => providers[provider]);
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
            aria-label="Refresh Lyria models"
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
            <label className="mb-1.5 block text-xs font-medium text-text-muted">Lyria Model</label>
            <select
              aria-label="Lyria model"
              value={selectedModel || ''}
              onChange={(event) => onModelChange(event.target.value)}
              disabled={!selectedProvider || models.length === 0}
              className="w-full min-h-10 rounded-xl border border-border bg-surface-alt px-3 text-sm text-text focus:outline-none focus:border-primary/50 disabled:opacity-50"
            >
              <option value="">{models.length === 0 ? 'No Lyria models available' : 'Select model'}</option>
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
          Add an enabled OpenRouter or Gemini provider profile in Settings before generating music.
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
          <TextInput label="Genre" value={promptForm.options.genre || ''} onChange={(value) => onOption('genre', value)} />
          <TextInput label="Mood" value={promptForm.options.mood || ''} onChange={(value) => onOption('mood', value)} />
          <TextInput label="Era" value={promptForm.options.era || ''} onChange={(value) => onOption('era', value)} />
          <TextInput label="Key / scale" value={promptForm.options.scale || ''} onChange={(value) => onOption('scale', value)} />
          <TextInput label="BPM" value={promptForm.options.bpm ? String(promptForm.options.bpm) : ''} type="number" onChange={(value) => onOption('bpm', value ? Number(value) : undefined)} />
          <TextInput label="Duration" value={promptForm.options.duration || ''} onChange={(value) => onOption('duration', value)} />
        </div>

        <TextInput
          label="Instruments"
          value={(promptForm.options.instruments || []).join(', ')}
          onChange={(value) => onOption('instruments', value.split(',').map((item) => item.trim()).filter(Boolean))}
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
