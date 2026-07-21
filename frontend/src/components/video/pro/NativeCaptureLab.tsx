import { useEffect, useMemo, useState } from 'react';
import { MonitorUp, Square, X } from 'lucide-react';
import { toast } from 'sonner';
import { useVideoStudioStore } from '../../../stores/videoStudio';

interface NativeCaptureCapabilities {
  supported: boolean;
  ffmpeg_available: boolean;
  audio_devices: string[];
  video_devices: string[];
  system_audio_devices: string[];
  reconnect_supported: boolean;
  reason?: string;
}

interface NativeCaptureStartResult {
  session_id: string;
  status: string;
}

interface NativeCaptureBridge {
  NativeCaptureCapabilities?: () => Promise<NativeCaptureCapabilities>;
  StartNativeCapture?: (request: {
    project_id: string;
    fps: number;
    audio_device: string;
    capture_cursor: boolean;
    capture_keystrokes: boolean;
    reconnect: boolean;
  }) => Promise<NativeCaptureStartResult>;
  StopNativeCapture?: (sessionId: string) => Promise<unknown>;
  ImportNativeCapture?: (sessionId: string, projectId: string) => Promise<unknown>;
}

function desktopBridge(): NativeCaptureBridge | undefined {
  return (window as unknown as {
    go?: { main?: { App?: NativeCaptureBridge } };
  }).go?.main?.App;
}

export function NativeCaptureLab({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const projectId = useVideoStudioStore((state) => state.activeProjectId);
  const selectProject = useVideoStudioStore((state) => state.selectProject);
  const [capabilities, setCapabilities] = useState<NativeCaptureCapabilities | null>(null);
  const [sessionId, setSessionId] = useState('');
  const [status, setStatus] = useState<'idle' | 'starting' | 'recording' | 'stopping' | 'importing' | 'completed' | 'error'>('idle');
  const [audioDevice, setAudioDevice] = useState('');
  const [captureCursor, setCaptureCursor] = useState(true);
  const [captureKeystrokes, setCaptureKeystrokes] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setCapabilities(null);
    setError(null);
    const bridge = desktopBridge();
    if (!bridge?.NativeCaptureCapabilities) {
      setCapabilities({
        supported: false,
        ffmpeg_available: false,
        audio_devices: [],
        video_devices: [],
        system_audio_devices: [],
        reconnect_supported: false,
        reason: 'Native capture is available in the Windows Wails build.',
      });
      return;
    }
    void bridge.NativeCaptureCapabilities()
      .then((value) => {
        if (!cancelled) setCapabilities(value);
      })
      .catch((reason: unknown) => {
        if (cancelled) return;
        setCapabilities({
          supported: false,
          ffmpeg_available: false,
          audio_devices: [],
          video_devices: [],
          system_audio_devices: [],
          reconnect_supported: false,
          reason: reason instanceof Error ? reason.message : 'Could not inspect native capture capabilities.',
        });
      });
    return () => {
      cancelled = true;
    };
  }, [open]);

  const audioDevices = useMemo(
    () => Array.from(new Set([
      ...(capabilities?.system_audio_devices ?? []),
      ...(capabilities?.audio_devices ?? []),
    ])),
    [capabilities],
  );

  if (!open) return null;

  const start = async () => {
    const bridge = desktopBridge();
    if (!bridge?.StartNativeCapture || !projectId || status === 'recording') return;
    setStatus('starting');
    setError(null);
    try {
      const result = await bridge.StartNativeCapture({
        project_id: projectId,
        fps: 30,
        audio_device: audioDevice,
        capture_cursor: captureCursor,
        capture_keystrokes: captureKeystrokes,
        reconnect: false,
      });
      setSessionId(result.session_id);
      setStatus('recording');
      toast.success('Native desktop capture started');
    } catch (reason) {
      const message = reason instanceof Error ? reason.message : 'Could not start native capture';
      setError(message);
      setStatus('error');
      toast.error(message);
    }
  };

  const stop = async () => {
    const bridge = desktopBridge();
    if (!bridge?.StopNativeCapture || !sessionId || !projectId) return;
    setStatus('stopping');
    setError(null);
    try {
      await bridge.StopNativeCapture(sessionId);
      setStatus('importing');
      if (!bridge.ImportNativeCapture) throw new Error('Native capture import is unavailable');
      await bridge.ImportNativeCapture(sessionId, projectId);
      await selectProject(projectId);
      setSessionId('');
      setStatus('completed');
      toast.success('Native recording imported into the project');
    } catch (reason) {
      const message = reason instanceof Error ? reason.message : 'Could not finish native capture';
      setError(message);
      setStatus('error');
      toast.error(message);
    }
  };

  const recording = status === 'recording';
  const busy = status === 'starting' || status === 'stopping' || status === 'importing';

  return (
    <div className="fixed inset-0 z-[80] flex items-center justify-center bg-black/60 p-4">
      <div className="w-full max-w-xl rounded-xl border border-border bg-surface p-4 shadow-2xl">
        <div className="flex items-start justify-between gap-3">
          <div>
            <h2 className="font-semibold text-text">Windows Native Capture</h2>
            <p className="mt-1 text-xs text-text-muted">
              FFmpeg desktop capture with optional loopback audio, cursor telemetry, and explicit keystroke consent.
            </p>
          </div>
          <button type="button" onClick={onClose} disabled={recording || busy} aria-label="Close native capture">
            <X size={18} />
          </button>
        </div>

        {!capabilities ? (
          <p className="mt-4 rounded border border-border bg-surface-alt p-3 text-sm text-text-muted">
            Checking native capture capabilities…
          </p>
        ) : !capabilities.supported ? (
          <p className="mt-4 rounded border border-border bg-surface-alt p-3 text-sm text-text-muted">
            {capabilities.reason || 'Native capture is unavailable.'}
          </p>
        ) : (
          <div className="mt-4 space-y-4">
            <label className="block text-xs text-text-muted">
              Audio or loopback device
              <select
                className="mt-1 w-full rounded border border-border bg-surface-alt p-2 text-text"
                value={audioDevice}
                onChange={(event) => setAudioDevice(event.target.value)}
                disabled={recording || busy}
              >
                <option value="">No audio</option>
                {audioDevices.map((name) => (
                  <option key={name} value={name}>{name}</option>
                ))}
              </select>
            </label>

            <div className="grid gap-2 sm:grid-cols-2">
              <label className="flex items-start gap-2 rounded border border-border p-3 text-sm text-text-secondary">
                <input
                  type="checkbox"
                  checked={captureCursor}
                  onChange={(event) => setCaptureCursor(event.target.checked)}
                  disabled={recording || busy}
                />
                <span>
                  <strong className="block text-text">Capture cursor path</strong>
                  Stores pointer position and click timing with the recording.
                </span>
              </label>
              <label className="flex items-start gap-2 rounded border border-amber-400/30 bg-amber-400/5 p-3 text-sm text-text-secondary">
                <input
                  type="checkbox"
                  checked={captureKeystrokes}
                  onChange={(event) => setCaptureKeystrokes(event.target.checked)}
                  disabled={recording || busy}
                />
                <span>
                  <strong className="block text-text">Capture keystroke telemetry</strong>
                  Opt in only when no passwords or sensitive text will be entered. Virtual-key timing is stored locally; typed text is not reconstructed.
                </span>
              </label>
            </div>

            {!capabilities.reconnect_supported && (
              <p className="rounded border border-border bg-surface-alt p-2 text-xs text-text-muted">
                Device reconnect is not seamless. If an audio device disconnects, stop the recording, reconnect it, refresh the device list, and begin a new take.
              </p>
            )}

            {error && (
              <p role="alert" className="rounded border border-red-400/30 bg-red-400/10 p-2 text-xs text-red-200">
                {error}
              </p>
            )}

            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                disabled={recording || busy || !projectId}
                onClick={() => void start()}
                className="inline-flex items-center gap-2 rounded bg-red-500 px-3 py-2 text-xs font-semibold text-white disabled:opacity-40"
              >
                <MonitorUp size={14} />
                Start desktop capture
              </button>
              <button
                type="button"
                disabled={!recording || busy}
                onClick={() => void stop()}
                className="inline-flex items-center gap-2 rounded border border-border px-3 py-2 text-xs disabled:opacity-40"
              >
                <Square size={13} />
                Stop and import
              </button>
            </div>
            <p className="text-xs text-text-muted">Status: {status}</p>
          </div>
        )}
      </div>
    </div>
  );
}
