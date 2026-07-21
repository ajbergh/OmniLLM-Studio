import { useEffect, useMemo, useState } from 'react';
import { RefreshCw, X } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '../../../api';
import type { ProviderProfile } from '../../../types';
import { useVideoStudioStore } from '../../../stores/videoStudio';
import { transcriptionApi, type VideoTranscript } from './transcriptionApi';

const TERMINAL_STATUSES = new Set(['completed', 'failed', 'cancelled']);

function parseJSON(value?: string): Record<string, unknown> {
  if (!value) return {};
  try {
    const parsed = JSON.parse(value) as unknown;
    return parsed && typeof parsed === 'object' ? parsed as Record<string, unknown> : {};
  } catch {
    return {};
  }
}

function formatCost(cost?: number): string {
  if (cost === undefined) return 'Provider did not report cost';
  return `$${cost.toFixed(cost < 0.01 ? 4 : 2)}`;
}

export function TranscriptionLab({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const projectId = useVideoStudioStore((state) => state.activeProjectId);
  const assets = useVideoStudioStore((state) => state.assets);
  const timeline = useVideoStudioStore((state) => state.timeline);
  const saveTimeline = useVideoStudioStore((state) => state.saveTimeline);
  const [items, setItems] = useState<VideoTranscript[]>([]);
  const [profiles, setProfiles] = useState<ProviderProfile[]>([]);
  const [assetId, setAssetId] = useState('');
  const [profileId, setProfileId] = useState('');
  const [model, setModel] = useState('');
  const [language, setLanguage] = useState('');
  const [translateToEnglish, setTranslateToEnglish] = useState(false);
  const [wordTimestamps, setWordTimestamps] = useState(true);
  const [diarization, setDiarization] = useState(false);
  const [retainProviderData, setRetainProviderData] = useState(false);
  const [replaceCaptions, setReplaceCaptions] = useState(false);
  const [consent, setConsent] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const eligibleProfiles = useMemo(
    () => profiles.filter((profile) => (
      profile.enabled && ['openai', 'custom'].includes(profile.type.toLowerCase())
    )),
    [profiles],
  );
  const selectedProfile = eligibleProfiles.find((profile) => profile.id === profileId);
  const mediaAssets = useMemo(
    () => assets.filter((asset) => (
      asset.mime_type.startsWith('audio/') || asset.mime_type.startsWith('video/')
    )),
    [assets],
  );

  useEffect(() => {
    if (!open || !projectId) return;
    let cancelled = false;
    setError('');
    setLoading(true);
    void Promise.all([transcriptionApi.list(projectId), api.listProviders()])
      .then(([transcripts, providerProfiles]) => {
        if (cancelled) return;
        setItems(transcripts);
        setProfiles(providerProfiles);
        const firstEligible = providerProfiles.find((profile) => (
          profile.enabled && ['openai', 'custom'].includes(profile.type.toLowerCase())
        ));
        if (firstEligible) {
          setProfileId((current) => current || firstEligible.id);
          setModel((current) => current || firstEligible.default_model || 'gpt-4o-mini-transcribe');
        }
        setAssetId((current) => current || mediaAssets[0]?.id || '');
      })
      .catch((reason: unknown) => {
        if (!cancelled) setError(reason instanceof Error ? reason.message : 'Could not load transcription data');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, projectId, mediaAssets]);

  useEffect(() => {
    if (!selectedProfile) return;
    setModel(selectedProfile.default_model || 'gpt-4o-mini-transcribe');
  }, [selectedProfile]);

  useEffect(() => {
    if (!open || items.every((item) => TERMINAL_STATUSES.has(item.status))) return;
    const interval = window.setInterval(() => {
      void Promise.all(items
        .filter((item) => !TERMINAL_STATUSES.has(item.status))
        .map((item) => transcriptionApi.get(item.id).catch(() => item)))
        .then((updates) => {
          setItems((current) => current.map((item) => (
            updates.find((update) => update.id === item.id) ?? item
          )));
        });
    }, 2000);
    return () => window.clearInterval(interval);
  }, [open, items]);

  if (!open) return null;

  const refresh = async () => {
    if (!projectId) return;
    setLoading(true);
    try {
      setItems(await transcriptionApi.list(projectId));
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not refresh transcripts');
    } finally {
      setLoading(false);
    }
  };

  const start = async () => {
    if (!projectId || !assetId || !profileId || !consent || loading) return;
    setLoading(true);
    setError('');
    try {
      const item = await transcriptionApi.start(projectId, {
        asset_id: assetId,
        provider_profile_id: profileId,
        model: model.trim() || undefined,
        language: language.trim() || undefined,
        translate_to: translateToEnglish ? 'en' : undefined,
        word_timestamps: wordTimestamps,
        diarization,
        allow_remote_processing: true,
        retain_provider_data: retainProviderData,
      });
      setItems((current) => [item, ...current.filter((existing) => existing.id !== item.id)]);
      setConsent(false);
      toast.success('Transcription queued');
    } catch (reason) {
      const message = reason instanceof Error ? reason.message : 'Could not start transcription';
      setError(message);
      toast.error(message);
    } finally {
      setLoading(false);
    }
  };

  const addCaptions = async (id: string) => {
    if (!timeline) return;
    setLoading(true);
    setError('');
    try {
      const response = await transcriptionApi.captions(id);
      const next = structuredClone(timeline);
      let track = next.tracks.find((candidate) => candidate.type === 'caption');
      if (!track) {
        track = {
          id: `track-caption-${Date.now()}`,
          type: 'caption',
          name: 'Captions',
          locked: false,
          muted: false,
          visible: true,
          clips: [],
        };
        next.tracks.push(track);
      }
      const incomingIds = new Set(response.clips.map((clip) => clip.id));
      if (replaceCaptions) {
        track.clips = track.clips.filter((clip) => !clip.id.startsWith('caption-'));
      } else {
        track.clips = track.clips.filter((clip) => !incomingIds.has(clip.id));
      }
      track.clips.push(...response.clips);
      track.clips.sort((left, right) => left.start_ms - right.start_ms);
      await saveTimeline(next);
      toast.success(`${response.clips.length} caption segment(s) added`);
    } catch (reason) {
      const message = reason instanceof Error ? reason.message : 'Could not regenerate captions';
      setError(message);
      toast.error(message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-[80] flex items-center justify-center bg-black/60 p-4">
      <div className="max-h-[88vh] w-full max-w-4xl overflow-auto rounded-xl border border-border bg-surface p-4 shadow-2xl">
        <div className="mb-4 flex items-start justify-between gap-3">
          <div>
            <h2 className="font-semibold text-text">Provider Transcription</h2>
            <p className="mt-1 text-xs text-text-muted">
              Durable remote speech-to-text with reusable timed segments. Media is sent only after explicit consent.
            </p>
          </div>
          <div className="flex gap-1">
            <button type="button" onClick={() => void refresh()} disabled={loading} aria-label="Refresh transcripts" className="rounded p-1.5 hover:bg-surface-alt">
              <RefreshCw size={16} className={loading ? 'animate-spin' : ''} />
            </button>
            <button type="button" onClick={onClose} aria-label="Close transcription lab" className="rounded p-1.5 hover:bg-surface-alt">
              <X size={18} />
            </button>
          </div>
        </div>

        <div className="grid gap-3 sm:grid-cols-2">
          <label className="text-xs text-text-muted">
            Media
            <select className="mt-1 w-full rounded border border-border bg-surface-alt p-2 text-text" value={assetId} onChange={(event) => setAssetId(event.target.value)}>
              <option value="">Select audio/video</option>
              {mediaAssets.map((asset) => <option key={asset.id} value={asset.id}>{asset.file_name}</option>)}
            </select>
          </label>
          <label className="text-xs text-text-muted">
            Provider profile
            <select className="mt-1 w-full rounded border border-border bg-surface-alt p-2 text-text" value={profileId} onChange={(event) => setProfileId(event.target.value)}>
              <option value="">Select configured OpenAI/custom provider</option>
              {eligibleProfiles.map((profile) => <option key={profile.id} value={profile.id}>{profile.name} ({profile.type})</option>)}
            </select>
          </label>
          <label className="text-xs text-text-muted">
            Model
            <input className="mt-1 w-full rounded border border-border bg-surface-alt p-2 text-text" value={model} onChange={(event) => setModel(event.target.value)} placeholder="gpt-4o-mini-transcribe" />
          </label>
          <label className="text-xs text-text-muted">
            Source language (optional)
            <input className="mt-1 w-full rounded border border-border bg-surface-alt p-2 text-text" value={language} onChange={(event) => setLanguage(event.target.value)} placeholder="Auto-detect, or en / es / de…" />
          </label>
        </div>

        {eligibleProfiles.length === 0 && (
          <p className="mt-3 rounded border border-amber-400/30 bg-amber-400/10 p-2 text-xs text-amber-200">
            Configure and enable an OpenAI or OpenAI-compatible custom provider before transcribing.
          </p>
        )}

        <div className="mt-3 grid gap-2 sm:grid-cols-2">
          <label className="flex items-center gap-2 rounded border border-border p-2 text-xs text-text-secondary"><input type="checkbox" checked={wordTimestamps} onChange={(event) => setWordTimestamps(event.target.checked)} />Request word and segment timestamps</label>
          <label className="flex items-center gap-2 rounded border border-border p-2 text-xs text-text-secondary"><input type="checkbox" checked={diarization} onChange={(event) => setDiarization(event.target.checked)} />Request speaker labels when supported</label>
          <label className="flex items-center gap-2 rounded border border-border p-2 text-xs text-text-secondary"><input type="checkbox" checked={translateToEnglish} onChange={(event) => setTranslateToEnglish(event.target.checked)} />Translate result to English</label>
          <label className="flex items-center gap-2 rounded border border-border p-2 text-xs text-text-secondary"><input type="checkbox" checked={retainProviderData} onChange={(event) => setRetainProviderData(event.target.checked)} />Provider may retain data under its policy</label>
        </div>

        <label className="mt-3 flex items-start gap-2 rounded border border-red-400/30 bg-red-400/5 p-3 text-xs text-text-secondary">
          <input type="checkbox" checked={consent} onChange={(event) => setConsent(event.target.checked)} />
          <span>I authorize sending the selected media to this remote provider. Provider privacy terms and usage charges apply; OmniLLM-Studio cannot guarantee diarization, pricing, or data-retention behavior not returned by that provider.</span>
        </label>
        <button className="mt-3 rounded bg-primary px-3 py-2 text-xs font-semibold text-black disabled:opacity-40" disabled={!projectId || !assetId || !profileId || !consent || loading} onClick={() => void start()}>
          Start transcription
        </button>
        {error && <p role="alert" className="mt-2 text-xs text-red-400">{error}</p>}

        <div className="mt-6 flex items-center justify-between gap-3">
          <h3 className="text-sm font-semibold text-text">Durable transcripts</h3>
          <label className="flex items-center gap-2 text-xs text-text-muted"><input type="checkbox" checked={replaceCaptions} onChange={(event) => setReplaceCaptions(event.target.checked)} />Replace existing generated captions</label>
        </div>
        <div className="mt-2 space-y-2">
          {items.length === 0 && <p className="rounded border border-dashed border-border p-4 text-center text-xs text-text-muted">No provider transcripts for this project yet.</p>}
          {items.map((item) => {
            const metadata = parseJSON(item.metadata_json);
            const privacy = parseJSON(item.privacy_json);
            const hasSpeakers = item.segments?.some((segment) => Boolean(segment.speaker));
            return (
              <div key={item.id} className="rounded border border-border bg-surface-alt p-3">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <div className="text-sm font-medium text-text">{item.status} · {item.model}</div>
                    <div className="text-[11px] text-text-muted">
                      {item.language || 'language pending'}{item.translated_language ? ` → ${item.translated_language}` : ''} · {item.segments?.length || 0} segments · {formatCost(item.cost_usd)}
                    </div>
                    <div className="mt-1 text-[10px] text-text-muted">
                      Speakers: {hasSpeakers ? 'returned' : metadata.requested_diarization ? 'requested, not returned' : 'not requested'} · Remote retention: {privacy.retain_provider_data ? 'allowed' : 'not requested'}
                    </div>
                  </div>
                  {item.status === 'completed' && (
                    <button className="rounded border border-border px-2 py-1 text-xs hover:bg-surface" disabled={loading} onClick={() => void addCaptions(item.id)}>
                      Regenerate captions
                    </button>
                  )}
                </div>
                {item.error && <p className="mt-1 text-xs text-red-400">{item.error}</p>}
                <p className="mt-2 line-clamp-4 whitespace-pre-wrap text-xs text-text-secondary">{item.text || (TERMINAL_STATUSES.has(item.status) ? 'No transcript text returned.' : 'Transcription is still processing…')}</p>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
