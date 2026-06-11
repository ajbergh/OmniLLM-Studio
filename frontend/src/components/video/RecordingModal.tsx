/**
 * Capture modal for screen (getDisplayMedia), camera, and voiceover
 * (getUserMedia) recordings via MediaRecorder: device pickers, optional mic
 * mix-in for screen shares, countdown, level meter, pause/resume, and a
 * review step that uploads the take to the media bin and optionally places it
 * on the timeline at the playhead. Environments without capture APIs get an
 * honest unsupported message — nothing is faked. Browser APIs expose no
 * cursor coordinates, so cursor metadata (see the timeline schema) is not
 * collected here yet.
 */
import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Camera, Circle, Mic, Monitor, Pause, Play, Square, X } from 'lucide-react';
import { toast } from 'sonner';
import { useVideoStudioStore } from '../../stores/videoStudio';

type RecordingSource = 'voiceover' | 'camera' | 'screen';
type RecordingStage = 'setup' | 'countdown' | 'recording' | 'review';

const SOURCE_OPTIONS: Array<{ key: RecordingSource; label: string; icon: typeof Mic; description: string }> = [
  { key: 'voiceover', label: 'Voiceover', icon: Mic, description: 'Record narration from your microphone' },
  { key: 'camera', label: 'Camera', icon: Camera, description: 'Record your webcam with audio' },
  { key: 'screen', label: 'Screen', icon: Monitor, description: 'Record a screen, window, or tab' },
];

function supportFor(source: RecordingSource): { supported: boolean; reason?: string } {
  if (typeof window === 'undefined' || typeof window.MediaRecorder === 'undefined') {
    return { supported: false, reason: 'MediaRecorder is not available in this browser' };
  }
  if (source === 'screen') {
    if (!navigator.mediaDevices?.getDisplayMedia) {
      return { supported: false, reason: 'Screen capture (getDisplayMedia) is not available in this environment' };
    }
    return { supported: true };
  }
  if (!navigator.mediaDevices?.getUserMedia) {
    return { supported: false, reason: 'Microphone/camera capture (getUserMedia) is not available in this environment' };
  }
  return { supported: true };
}

function pickMimeType(video: boolean): string | undefined {
  const candidates = video
    ? ['video/webm;codecs=vp9,opus', 'video/webm;codecs=vp8,opus', 'video/webm', 'video/mp4']
    : ['audio/webm;codecs=opus', 'audio/webm', 'audio/mp4'];
  return candidates.find((candidate) => MediaRecorder.isTypeSupported(candidate));
}

function formatElapsed(ms: number): string {
  const total = Math.floor(ms / 1000);
  const minutes = Math.floor(total / 60);
  const seconds = total % 60;
  return `${minutes}:${String(seconds).padStart(2, '0')}`;
}

export function RecordingModal({ initialSource = 'voiceover', onClose }: { initialSource?: RecordingSource; onClose: () => void }) {
  const uploadAsset = useVideoStudioStore((state) => state.uploadAsset);
  const addAssetToTimeline = useVideoStudioStore((state) => state.addAssetToTimeline);

  const [source, setSource] = useState<RecordingSource>(initialSource);
  const [stage, setStage] = useState<RecordingStage>('setup');
  const [micDevices, setMicDevices] = useState<MediaDeviceInfo[]>([]);
  const [cameraDevices, setCameraDevices] = useState<MediaDeviceInfo[]>([]);
  const [micId, setMicId] = useState<string>('');
  const [cameraId, setCameraId] = useState<string>('');
  const [includeMic, setIncludeMic] = useState(true);
  const [useCountdown, setUseCountdown] = useState(true);
  const [placeOnTimeline, setPlaceOnTimeline] = useState(true);
  const [countdown, setCountdown] = useState(3);
  const [elapsedMs, setElapsedMs] = useState(0);
  const [paused, setPaused] = useState(false);
  const [level, setLevel] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<{ blob: Blob; url: string; isVideo: boolean } | null>(null);
  const [saving, setSaving] = useState(false);

  const streamRef = useRef<MediaStream | null>(null);
  const recorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const audioContextRef = useRef<AudioContext | null>(null);
  const meterRafRef = useRef<number | null>(null);
  const startedAtRef = useRef<number>(0);
  const pausedTotalRef = useRef<number>(0);
  const pausedAtRef = useRef<number>(0);
  const livePreviewRef = useRef<HTMLVideoElement | null>(null);

  const support = supportFor(source);

  // Device labels need permission; enumerate after the first getUserMedia and
  // on open so the pickers are useful when permission was already granted.
  useEffect(() => {
    if (!navigator.mediaDevices?.enumerateDevices) return;
    navigator.mediaDevices.enumerateDevices().then((devices) => {
      setMicDevices(devices.filter((device) => device.kind === 'audioinput'));
      setCameraDevices(devices.filter((device) => device.kind === 'videoinput'));
    }).catch(() => { /* non-fatal */ });
  }, [stage]);

  const cleanupStream = () => {
    recorderRef.current = null;
    streamRef.current?.getTracks().forEach((track) => track.stop());
    streamRef.current = null;
    if (meterRafRef.current !== null) cancelAnimationFrame(meterRafRef.current);
    meterRafRef.current = null;
    void audioContextRef.current?.close().catch(() => { /* already closed */ });
    audioContextRef.current = null;
  };

  useEffect(() => () => {
    cleanupStream();
    if (result?.url) URL.revokeObjectURL(result.url);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const startMeter = (stream: MediaStream) => {
    if (stream.getAudioTracks().length === 0) return;
    try {
      const AudioContextCtor = window.AudioContext || (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
      if (!AudioContextCtor) return;
      const context = new AudioContextCtor();
      const analyser = context.createAnalyser();
      analyser.fftSize = 256;
      context.createMediaStreamSource(stream).connect(analyser);
      audioContextRef.current = context;
      const data = new Uint8Array(analyser.frequencyBinCount);
      const tick = () => {
        analyser.getByteTimeDomainData(data);
        let peak = 0;
        for (const sample of data) peak = Math.max(peak, Math.abs(sample - 128) / 128);
        setLevel(peak);
        meterRafRef.current = requestAnimationFrame(tick);
      };
      meterRafRef.current = requestAnimationFrame(tick);
    } catch {
      // Meter is best-effort; recording continues without it.
    }
  };

  const acquireStream = async (): Promise<MediaStream> => {
    if (source === 'screen') {
      const display = await navigator.mediaDevices.getDisplayMedia({ video: true, audio: true });
      if (includeMic && navigator.mediaDevices?.getUserMedia) {
        try {
          const mic = await navigator.mediaDevices.getUserMedia({ audio: micId ? { deviceId: { exact: micId } } : true });
          mic.getAudioTracks().forEach((track) => display.addTrack(track));
        } catch {
          toast.info('Microphone unavailable — recording screen without mic audio');
        }
      }
      return display;
    }
    if (source === 'camera') {
      return navigator.mediaDevices.getUserMedia({
        video: cameraId ? { deviceId: { exact: cameraId } } : true,
        audio: micId ? { deviceId: { exact: micId } } : true,
      });
    }
    return navigator.mediaDevices.getUserMedia({ audio: micId ? { deviceId: { exact: micId } } : true });
  };

  const beginRecording = async () => {
    setError(null);
    let stream: MediaStream;
    try {
      stream = await acquireStream();
    } catch (err) {
      const name = (err as DOMException)?.name;
      setError(name === 'NotAllowedError'
        ? 'Permission denied. Allow access in the browser prompt and try again.'
        : `Could not start capture: ${(err as Error).message}`);
      return;
    }
    streamRef.current = stream;
    startMeter(stream);
    // The user can end a screen share from the browser UI — stop with it.
    stream.getVideoTracks()[0]?.addEventListener('ended', () => stopRecording());

    if (useCountdown) {
      setStage('countdown');
      for (let i = 3; i > 0; i -= 1) {
        setCountdown(i);
        await new Promise((resolve) => setTimeout(resolve, 1000));
        if (!streamRef.current) return; // cancelled mid-countdown
      }
    }

    const isVideo = source !== 'voiceover';
    const mimeType = pickMimeType(isVideo);
    let recorder: MediaRecorder;
    try {
      recorder = new MediaRecorder(stream, mimeType ? { mimeType } : undefined);
    } catch (err) {
      cleanupStream();
      setError(`Could not start the recorder: ${(err as Error).message}`);
      setStage('setup');
      return;
    }
    chunksRef.current = [];
    recorder.ondataavailable = (event) => {
      if (event.data.size > 0) chunksRef.current.push(event.data);
    };
    recorder.onstop = () => {
      const blob = new Blob(chunksRef.current, { type: recorder.mimeType || (isVideo ? 'video/webm' : 'audio/webm') });
      cleanupStream();
      if (blob.size === 0) {
        setError('Recording produced no data');
        setStage('setup');
        return;
      }
      setResult({ blob, url: URL.createObjectURL(blob), isVideo });
      setStage('review');
    };
    recorderRef.current = recorder;
    recorder.start(1000);
    startedAtRef.current = performance.now();
    pausedTotalRef.current = 0;
    setPaused(false);
    setElapsedMs(0);
    setStage('recording');
  };

  // Elapsed-time ticker while recording.
  useEffect(() => {
    if (stage !== 'recording' || paused) return;
    const interval = window.setInterval(() => {
      setElapsedMs(performance.now() - startedAtRef.current - pausedTotalRef.current);
    }, 250);
    return () => window.clearInterval(interval);
  }, [stage, paused]);

  // Live camera preview while recording video sources.
  useEffect(() => {
    if (stage === 'recording' && livePreviewRef.current && streamRef.current && source !== 'voiceover') {
      livePreviewRef.current.srcObject = streamRef.current;
      void livePreviewRef.current.play().catch(() => { /* autoplay policy */ });
    }
  }, [stage, source]);

  const stopRecording = () => {
    const recorder = recorderRef.current;
    if (recorder && recorder.state !== 'inactive') {
      recorder.stop();
    } else {
      cleanupStream();
      setStage('setup');
    }
  };

  const togglePause = () => {
    const recorder = recorderRef.current;
    if (!recorder) return;
    if (recorder.state === 'recording' && typeof recorder.pause === 'function') {
      recorder.pause();
      pausedAtRef.current = performance.now();
      setPaused(true);
    } else if (recorder.state === 'paused' && typeof recorder.resume === 'function') {
      recorder.resume();
      pausedTotalRef.current += performance.now() - pausedAtRef.current;
      setPaused(false);
    }
  };

  const cancelRecording = () => {
    const recorder = recorderRef.current;
    if (recorder && recorder.state !== 'inactive') {
      recorder.onstop = () => cleanupStream();
      recorder.stop();
    }
    cleanupStream();
    if (result?.url) URL.revokeObjectURL(result.url);
    setResult(null);
    setStage('setup');
  };

  const saveRecording = async (addToTimeline: boolean) => {
    if (!result) return;
    setSaving(true);
    try {
      const extension = result.blob.type.includes('mp4') ? 'mp4' : 'webm';
      const stamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
      const name = `${source === 'voiceover' ? 'voiceover' : source === 'camera' ? 'camera-recording' : 'screen-recording'}-${stamp}.${extension}`;
      await uploadAsset(new File([result.blob], name, { type: result.blob.type }));
      const assetId = useVideoStudioStore.getState().selectedAssetId;
      if (addToTimeline && assetId) {
        await addAssetToTimeline(assetId);
      }
      URL.revokeObjectURL(result.url);
      onClose();
    } catch (err) {
      toast.error(`Could not save the recording: ${(err as Error).message}`);
    } finally {
      setSaving(false);
    }
  };

  return createPortal(
    <div className="fixed inset-0 z-[110] flex items-center justify-center bg-black/60" onClick={stage === 'setup' || stage === 'review' ? onClose : undefined}>
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Record media"
        className="w-[26rem] max-w-[calc(100vw-2rem)] rounded-lg border border-border bg-surface p-4 shadow-xl"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="mb-3 flex items-center justify-between">
          <h3 className="text-sm font-semibold text-text">Record</h3>
          <button onClick={stage === 'recording' || stage === 'countdown' ? cancelRecording : onClose} className="rounded p-1 text-text-muted hover:text-text" aria-label="Close recorder">
            <X size={14} />
          </button>
        </div>

        {stage === 'setup' && (
          <div className="space-y-3">
            <div className="grid grid-cols-3 gap-2">
              {SOURCE_OPTIONS.map(({ key, label, icon: Icon, description }) => (
                <button
                  key={key}
                  onClick={() => setSource(key)}
                  className={`flex flex-col items-center gap-1 rounded-lg border p-2 text-xs ${source === key ? 'border-primary/50 bg-primary/10 text-primary' : 'border-border bg-surface-alt text-text-secondary hover:text-text'}`}
                  title={description}
                >
                  <Icon size={16} />
                  {label}
                </button>
              ))}
            </div>
            {!support.supported ? (
              <p className="rounded-md border border-amber-400/30 bg-amber-400/10 px-2 py-2 text-[11px] leading-relaxed text-amber-300">
                {support.reason}. Recording is unavailable here — upload media to the bin instead.
              </p>
            ) : (
              <>
                {(source === 'voiceover' || source === 'camera' || includeMic) && micDevices.length > 0 && (
                  <label className="block text-[11px] text-text-muted">
                    Microphone
                    <select
                      value={micId}
                      onChange={(event) => setMicId(event.target.value)}
                      className="mt-1 min-h-8 w-full rounded-md border border-border bg-surface-alt px-1 text-xs text-text"
                    >
                      <option value="">Default microphone</option>
                      {micDevices.map((device) => (
                        <option key={device.deviceId} value={device.deviceId}>{device.label || 'Microphone'}</option>
                      ))}
                    </select>
                  </label>
                )}
                {source === 'camera' && cameraDevices.length > 0 && (
                  <label className="block text-[11px] text-text-muted">
                    Camera
                    <select
                      value={cameraId}
                      onChange={(event) => setCameraId(event.target.value)}
                      className="mt-1 min-h-8 w-full rounded-md border border-border bg-surface-alt px-1 text-xs text-text"
                    >
                      <option value="">Default camera</option>
                      {cameraDevices.map((device) => (
                        <option key={device.deviceId} value={device.deviceId}>{device.label || 'Camera'}</option>
                      ))}
                    </select>
                  </label>
                )}
                {source === 'screen' && (
                  <label className="flex items-center gap-2 text-[11px] text-text-secondary">
                    <input type="checkbox" checked={includeMic} onChange={(event) => setIncludeMic(event.target.checked)} />
                    Include microphone narration
                  </label>
                )}
                {source === 'screen' && (
                  <p className="text-[10px] leading-relaxed text-text-muted">
                    System audio capture depends on what the browser offers in the share dialog (usually tab audio only). Cursor metadata is not captured by browser APIs yet.
                  </p>
                )}
                <label className="flex items-center gap-2 text-[11px] text-text-secondary">
                  <input type="checkbox" checked={useCountdown} onChange={(event) => setUseCountdown(event.target.checked)} />
                  3-second countdown before recording
                </label>
                <label className="flex items-center gap-2 text-[11px] text-text-secondary">
                  <input type="checkbox" checked={placeOnTimeline} onChange={(event) => setPlaceOnTimeline(event.target.checked)} />
                  Place on timeline at playhead when done
                </label>
                {error && <p className="text-[11px] text-red-400">{error}</p>}
                <button
                  onClick={() => { void beginRecording(); }}
                  className="inline-flex min-h-9 w-full items-center justify-center gap-2 rounded-md bg-red-500/90 px-3 text-xs font-medium text-white hover:bg-red-500"
                >
                  <Circle size={12} fill="currentColor" />
                  Start recording
                </button>
              </>
            )}
          </div>
        )}

        {stage === 'countdown' && (
          <div className="flex flex-col items-center gap-3 py-6">
            <span className="text-5xl font-bold tabular-nums text-text">{countdown}</span>
            <p className="text-xs text-text-muted">Recording starts in…</p>
            <button onClick={cancelRecording} className="rounded-md border border-border bg-surface-alt px-3 py-1.5 text-xs text-text-secondary hover:text-text">
              Cancel
            </button>
          </div>
        )}

        {stage === 'recording' && (
          <div className="space-y-3">
            {source !== 'voiceover' && (
              <video ref={livePreviewRef} muted playsInline className="aspect-video w-full rounded-md border border-border bg-black object-contain" />
            )}
            <div className="flex items-center gap-2">
              <span className="flex items-center gap-1.5 text-xs font-medium text-red-400">
                <Circle size={9} fill="currentColor" className={paused ? '' : 'animate-pulse'} />
                {paused ? 'Paused' : 'Recording'}
              </span>
              <span className="font-mono text-xs tabular-nums text-text">{formatElapsed(elapsedMs)}</span>
              <div className="ml-auto h-2 w-28 overflow-hidden rounded-full bg-surface-alt" title="Microphone level">
                <div className="h-full rounded-full bg-emerald-400 transition-[width]" style={{ width: `${Math.min(100, level * 130)}%` }} />
              </div>
            </div>
            <div className="flex gap-2">
              <button
                onClick={togglePause}
                className="inline-flex min-h-9 flex-1 items-center justify-center gap-1.5 rounded-md border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:text-text"
              >
                {paused ? <Play size={12} /> : <Pause size={12} />}
                {paused ? 'Resume' : 'Pause'}
              </button>
              <button
                onClick={stopRecording}
                className="inline-flex min-h-9 flex-1 items-center justify-center gap-1.5 rounded-md bg-red-500/90 px-3 text-xs font-medium text-white hover:bg-red-500"
              >
                <Square size={12} fill="currentColor" />
                Stop
              </button>
            </div>
          </div>
        )}

        {stage === 'review' && result && (
          <div className="space-y-3">
            {result.isVideo ? (
              <video src={result.url} controls className="aspect-video w-full rounded-md border border-border bg-black object-contain" />
            ) : (
              <audio src={result.url} controls className="w-full" />
            )}
            <div className="flex flex-col gap-2">
              <button
                onClick={() => { void saveRecording(placeOnTimeline); }}
                disabled={saving}
                className="min-h-9 rounded-md bg-primary px-3 text-xs font-medium text-white hover:bg-primary/90 disabled:opacity-50"
              >
                {saving ? 'Saving…' : placeOnTimeline ? 'Add to media bin + timeline' : 'Add to media bin'}
              </button>
              {placeOnTimeline && (
                <button
                  onClick={() => { void saveRecording(false); }}
                  disabled={saving}
                  className="min-h-9 rounded-md border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:text-text disabled:opacity-50"
                >
                  Add to media bin only
                </button>
              )}
              <button
                onClick={cancelRecording}
                disabled={saving}
                className="min-h-9 rounded-md border border-border bg-surface-alt px-3 text-xs text-red-400 hover:text-red-300 disabled:opacity-50"
              >
                Discard recording
              </button>
            </div>
          </div>
        )}
      </div>
    </div>,
    document.body,
  );
}
