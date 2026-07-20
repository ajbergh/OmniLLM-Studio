import { useEffect, useRef, useState } from 'react';
import { Camera, Circle, Loader2, Mic, Monitor, Pause, Play, Square, X } from 'lucide-react';
import { toast } from 'sonner';
import { useVideoStudioStore } from '../../../stores/videoStudio';
import { createCaptionsFromTranscript } from './transcriptTools';

type CaptureMode = 'screen_camera' | 'screen' | 'camera' | 'voiceover';
type CaptureStage = 'setup' | 'recording' | 'review';
type CameraCorner = 'top-left' | 'top-right' | 'bottom-left' | 'bottom-right';

interface BrowserSpeechRecognitionEvent extends Event {
  results: ArrayLike<{ 0: { transcript: string }; isFinal: boolean }>;
}

interface BrowserSpeechRecognition {
  continuous: boolean;
  interimResults: boolean;
  lang: string;
  start(): void;
  stop(): void;
  abort(): void;
  onresult: ((event: BrowserSpeechRecognitionEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
}

type SpeechRecognitionConstructor = new () => BrowserSpeechRecognition;

function speechRecognitionConstructor(): SpeechRecognitionConstructor | undefined {
  const candidate = window as unknown as {
    SpeechRecognition?: SpeechRecognitionConstructor;
    webkitSpeechRecognition?: SpeechRecognitionConstructor;
  };
  return candidate.SpeechRecognition || candidate.webkitSpeechRecognition;
}

function pickMimeType(video: boolean): string | undefined {
  const choices = video
    ? ['video/webm;codecs=vp9,opus', 'video/webm;codecs=vp8,opus', 'video/webm', 'video/mp4']
    : ['audio/webm;codecs=opus', 'audio/webm', 'audio/mp4'];
  return choices.find((choice) => MediaRecorder.isTypeSupported(choice));
}

function elapsedLabel(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  return `${Math.floor(total / 60)}:${String(total % 60).padStart(2, '0')}`;
}

function stopStream(stream: MediaStream | null): void {
  stream?.getTracks().forEach((track) => track.stop());
}

function canvasSize(resolution: '720p' | '1080p', portrait: boolean): { width: number; height: number } {
  const landscape = resolution === '1080p' ? { width: 1920, height: 1080 } : { width: 1280, height: 720 };
  return portrait ? { width: landscape.height, height: landscape.width } : landscape;
}

export function RecordingLab({ open, onClose }: { open: boolean; onClose: () => void }) {
  const uploadAsset = useVideoStudioStore((state) => state.uploadAsset);
  const addAssetToTimeline = useVideoStudioStore((state) => state.addAssetToTimeline);

  const [mode, setMode] = useState<CaptureMode>('screen_camera');
  const [stage, setStage] = useState<CaptureStage>('setup');
  const [resolution, setResolution] = useState<'720p' | '1080p'>('1080p');
  const [fps, setFps] = useState<30 | 60>(30);
  const [portrait, setPortrait] = useState(false);
  const [cameraCorner, setCameraCorner] = useState<CameraCorner>('bottom-right');
  const [includeMic, setIncludeMic] = useState(true);
  const [captureTranscript, setCaptureTranscript] = useState(false);
  const [createCaptions, setCreateCaptions] = useState(false);
  const [addToTimeline, setAddToTimeline] = useState(true);
  const [paused, setPaused] = useState(false);
  const [elapsedMs, setElapsedMs] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [transcript, setTranscript] = useState('');
  const [interimTranscript, setInterimTranscript] = useState('');
  const [result, setResult] = useState<{ blob: Blob; url: string; isVideo: boolean } | null>(null);
  const [saving, setSaving] = useState(false);

  const previewRef = useRef<HTMLVideoElement | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const recorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const sourceStreamsRef = useRef<MediaStream[]>([]);
  const finalStreamRef = useRef<MediaStream | null>(null);
  const renderFrameRef = useRef<number | null>(null);
  const audioContextRef = useRef<AudioContext | null>(null);
  const recognitionRef = useRef<BrowserSpeechRecognition | null>(null);
  const startedAtRef = useRef(0);
  const pauseStartedRef = useRef(0);
  const pausedTotalRef = useRef(0);

  const cleanup = () => {
    if (renderFrameRef.current !== null) cancelAnimationFrame(renderFrameRef.current);
    renderFrameRef.current = null;
    sourceStreamsRef.current.forEach(stopStream);
    sourceStreamsRef.current = [];
    stopStream(finalStreamRef.current);
    finalStreamRef.current = null;
    recognitionRef.current?.abort();
    recognitionRef.current = null;
    void audioContextRef.current?.close().catch(() => undefined);
    audioContextRef.current = null;
    recorderRef.current = null;
  };

  useEffect(() => () => {
    cleanup();
    if (result?.url) URL.revokeObjectURL(result.url);
    // result is intentionally excluded: cleanup runs once and the current URL
    // is also revoked when a take is discarded or saved.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!open) return;
    if (stage === 'recording' && previewRef.current && finalStreamRef.current) {
      previewRef.current.srcObject = finalStreamRef.current;
      void previewRef.current.play().catch(() => undefined);
    }
  }, [open, stage]);

  useEffect(() => {
    if (stage !== 'recording' || paused) return;
    const timer = window.setInterval(() => {
      setElapsedMs(performance.now() - startedAtRef.current - pausedTotalRef.current);
    }, 250);
    return () => window.clearInterval(timer);
  }, [paused, stage]);

  if (!open) return null;

  const acquire = async (): Promise<{ stream: MediaStream; isVideo: boolean }> => {
    if (!navigator.mediaDevices || typeof MediaRecorder === 'undefined') {
      throw new Error('Media capture is not available in this environment');
    }

    const wantsScreen = mode === 'screen' || mode === 'screen_camera';
    const wantsCamera = mode === 'camera' || mode === 'screen_camera';
    const wantsVideo = mode !== 'voiceover';
    const sources: MediaStream[] = [];
    let screen: MediaStream | null = null;
    let camera: MediaStream | null = null;
    let microphone: MediaStream | null = null;

    if (wantsScreen) {
      screen = await navigator.mediaDevices.getDisplayMedia({
        video: { frameRate: { ideal: fps, max: fps } },
        audio: true,
      });
      sources.push(screen);
    }
    if (wantsCamera) {
      camera = await navigator.mediaDevices.getUserMedia({
        video: { width: { ideal: 1920 }, height: { ideal: 1080 }, frameRate: { ideal: fps, max: fps } },
        audio: includeMic,
      });
      sources.push(camera);
    } else if (includeMic || mode === 'voiceover') {
      microphone = await navigator.mediaDevices.getUserMedia({ audio: true });
      sources.push(microphone);
    }
    sourceStreamsRef.current = sources;

    const mixedAudioTracks: MediaStreamTrack[] = [];
    const audioStreams = [screen, camera, microphone].filter((stream): stream is MediaStream => Boolean(stream?.getAudioTracks().length));
    if (audioStreams.length > 0) {
      const context = new AudioContext();
      const destination = context.createMediaStreamDestination();
      audioStreams.forEach((stream) => context.createMediaStreamSource(stream).connect(destination));
      audioContextRef.current = context;
      mixedAudioTracks.push(...destination.stream.getAudioTracks());
    }

    if (!wantsVideo) {
      const stream = new MediaStream(mixedAudioTracks);
      finalStreamRef.current = stream;
      return { stream, isVideo: false };
    }

    if (mode !== 'screen_camera') {
      const source = screen || camera;
      if (!source?.getVideoTracks().length) throw new Error('No video track was captured');
      const stream = new MediaStream([...source.getVideoTracks(), ...mixedAudioTracks]);
      finalStreamRef.current = stream;
      return { stream, isVideo: true };
    }

    const canvas = canvasRef.current || document.createElement('canvas');
    canvasRef.current = canvas;
    const size = canvasSize(resolution, portrait);
    canvas.width = size.width;
    canvas.height = size.height;
    const context = canvas.getContext('2d', { alpha: false });
    if (!context || !screen || !camera) throw new Error('Could not initialize combined screen and camera capture');

    const screenVideo = document.createElement('video');
    screenVideo.srcObject = screen;
    screenVideo.muted = true;
    screenVideo.playsInline = true;
    const cameraVideo = document.createElement('video');
    cameraVideo.srcObject = camera;
    cameraVideo.muted = true;
    cameraVideo.playsInline = true;
    await Promise.all([screenVideo.play(), cameraVideo.play()]);

    const drawContain = (video: HTMLVideoElement, x: number, y: number, width: number, height: number, cover = false) => {
      const sourceWidth = video.videoWidth || width;
      const sourceHeight = video.videoHeight || height;
      const scale = cover ? Math.max(width / sourceWidth, height / sourceHeight) : Math.min(width / sourceWidth, height / sourceHeight);
      const drawWidth = sourceWidth * scale;
      const drawHeight = sourceHeight * scale;
      context.drawImage(video, x + (width - drawWidth) / 2, y + (height - drawHeight) / 2, drawWidth, drawHeight);
    };

    const draw = () => {
      context.fillStyle = '#000000';
      context.fillRect(0, 0, canvas.width, canvas.height);
      drawContain(screenVideo, 0, 0, canvas.width, canvas.height, false);
      const pipWidth = Math.round(canvas.width * 0.22);
      const pipHeight = Math.round(pipWidth * 9 / 16);
      const margin = Math.round(canvas.width * 0.025);
      const left = cameraCorner.endsWith('right') ? canvas.width - pipWidth - margin : margin;
      const top = cameraCorner.startsWith('bottom') ? canvas.height - pipHeight - margin : margin;
      context.save();
      context.shadowColor = 'rgba(0,0,0,0.65)';
      context.shadowBlur = Math.max(8, Math.round(canvas.width * 0.008));
      context.fillStyle = '#000000';
      context.fillRect(left - 3, top - 3, pipWidth + 6, pipHeight + 6);
      context.restore();
      context.save();
      context.beginPath();
      context.roundRect(left, top, pipWidth, pipHeight, Math.round(pipWidth * 0.05));
      context.clip();
      drawContain(cameraVideo, left, top, pipWidth, pipHeight, true);
      context.restore();
      renderFrameRef.current = requestAnimationFrame(draw);
    };
    draw();

    const canvasStream = canvas.captureStream(fps);
    const stream = new MediaStream([...canvasStream.getVideoTracks(), ...mixedAudioTracks]);
    finalStreamRef.current = stream;
    return { stream, isVideo: true };
  };

  const startSpeechRecognition = () => {
    if (!captureTranscript) return;
    const Recognition = speechRecognitionConstructor();
    if (!Recognition) {
      toast.info('Live browser transcription is not available; the recording will continue without a transcript');
      return;
    }
    const recognition = new Recognition();
    recognition.continuous = true;
    recognition.interimResults = true;
    recognition.lang = navigator.language || 'en-US';
    recognition.onresult = (event) => {
      let finalText = '';
      let interim = '';
      for (let index = 0; index < event.results.length; index += 1) {
        const resultItem = event.results[index];
        if (resultItem.isFinal) finalText += `${resultItem[0].transcript.trim()} `;
        else interim += `${resultItem[0].transcript.trim()} `;
      }
      if (finalText) setTranscript((current) => `${current}${finalText}`.trimStart());
      setInterimTranscript(interim.trim());
    };
    recognition.onerror = () => setInterimTranscript('');
    recognitionRef.current = recognition;
    recognition.start();
  };

  const startRecording = async () => {
    setError(null);
    setTranscript('');
    setInterimTranscript('');
    try {
      const { stream, isVideo } = await acquire();
      const mimeType = pickMimeType(isVideo);
      const recorder = new MediaRecorder(stream, mimeType ? { mimeType, videoBitsPerSecond: isVideo ? (resolution === '1080p' ? 8_000_000 : 4_000_000) : undefined } : undefined);
      chunksRef.current = [];
      recorder.ondataavailable = (event) => {
        if (event.data.size > 0) chunksRef.current.push(event.data);
      };
      recorder.onerror = () => setError('The browser recorder reported an error');
      recorder.onstop = () => {
        recognitionRef.current?.stop();
        recognitionRef.current = null;
        const blob = new Blob(chunksRef.current, { type: recorder.mimeType || (isVideo ? 'video/webm' : 'audio/webm') });
        cleanup();
        if (!blob.size) {
          setError('Recording produced no data');
          setStage('setup');
          return;
        }
        setResult({ blob, url: URL.createObjectURL(blob), isVideo });
        setStage('review');
      };
      recorderRef.current = recorder;
      recorder.start(1_000);
      startedAtRef.current = performance.now();
      pausedTotalRef.current = 0;
      setElapsedMs(0);
      setPaused(false);
      setStage('recording');
      startSpeechRecognition();
      stream.getVideoTracks()[0]?.addEventListener('ended', () => {
        if (recorder.state !== 'inactive') recorder.stop();
      });
    } catch (captureError) {
      cleanup();
      const domError = captureError as DOMException;
      setError(domError.name === 'NotAllowedError' ? 'Capture permission was denied' : (captureError as Error).message);
      setStage('setup');
    }
  };

  const stopRecording = () => {
    const recorder = recorderRef.current;
    if (recorder && recorder.state !== 'inactive') recorder.stop();
  };

  const togglePause = () => {
    const recorder = recorderRef.current;
    if (!recorder) return;
    if (recorder.state === 'recording') {
      recorder.pause();
      pauseStartedRef.current = performance.now();
      setPaused(true);
    } else if (recorder.state === 'paused') {
      recorder.resume();
      pausedTotalRef.current += performance.now() - pauseStartedRef.current;
      setPaused(false);
    }
  };

  const discard = () => {
    cleanup();
    if (result?.url) URL.revokeObjectURL(result.url);
    setResult(null);
    setTranscript('');
    setInterimTranscript('');
    setStage('setup');
  };

  const save = async () => {
    if (!result) return;
    setSaving(true);
    try {
      const extension = result.blob.type.includes('mp4') ? 'mp4' : 'webm';
      const kind = mode === 'voiceover' ? 'voiceover' : mode.replace('_', '-');
      const stamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
      const asset = await uploadAsset(new File([result.blob], `${kind}-${stamp}.${extension}`, { type: result.blob.type }));
      if (addToTimeline) await addAssetToTimeline(asset.id);
      if (createCaptions && transcript.trim()) await createCaptionsFromTranscript(transcript.trim(), false);
      URL.revokeObjectURL(result.url);
      setResult(null);
      setStage('setup');
      onClose();
      toast.success('Recording saved to project media');
    } catch (saveError) {
      toast.error(`Could not save recording: ${(saveError as Error).message}`);
    } finally {
      setSaving(false);
    }
  };

  const modeOptions: Array<{ key: CaptureMode; label: string; icon: typeof Monitor }> = [
    { key: 'screen_camera', label: 'Screen + camera', icon: Monitor },
    { key: 'screen', label: 'Screen', icon: Monitor },
    { key: 'camera', label: 'Camera', icon: Camera },
    { key: 'voiceover', label: 'Voiceover', icon: Mic },
  ];

  return (
    <div className="fixed inset-0 z-[120] flex items-center justify-center bg-black/70 p-4" role="dialog" aria-modal="true" aria-label="Recording lab">
      <div className="flex max-h-[calc(100vh-2rem)] w-[min(52rem,100%)] flex-col overflow-hidden rounded-xl border border-border bg-surface-raised shadow-2xl">
        <div className="flex min-h-12 items-center gap-2 border-b border-border px-3">
          <Circle size={13} className="text-red-400" fill="currentColor" />
          <div className="flex-1 text-sm font-semibold text-text">Recording Lab</div>
          <button type="button" onClick={() => { discard(); onClose(); }} disabled={stage === 'recording'} className="rounded p-1.5 text-text-muted hover:bg-surface-alt hover:text-text disabled:opacity-40" aria-label="Close recording lab"><X size={16} /></button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto p-4">
          {stage === 'setup' && (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                {modeOptions.map(({ key, label, icon: Icon }) => (
                  <button key={key} type="button" onClick={() => setMode(key)} className={`min-h-16 rounded-lg border p-2 text-left ${mode === key ? 'border-primary/50 bg-primary/10 text-primary' : 'border-border bg-surface-alt text-text-secondary hover:text-text'}`}>
                    <Icon size={16} />
                    <div className="mt-1 text-xs font-medium">{label}</div>
                  </button>
                ))}
              </div>

              {mode !== 'voiceover' && (
                <div className="grid grid-cols-2 gap-3 rounded-lg border border-border bg-surface p-3 sm:grid-cols-4">
                  <label className="text-[10px] text-text-muted">Resolution
                    <select value={resolution} onChange={(event) => setResolution(event.target.value as '720p' | '1080p')} className="mt-1 min-h-9 w-full rounded border border-border bg-surface-alt px-2 text-xs text-text">
                      <option value="720p">720p</option><option value="1080p">1080p</option>
                    </select>
                  </label>
                  <label className="text-[10px] text-text-muted">Frame rate
                    <select value={fps} onChange={(event) => setFps(Number(event.target.value) as 30 | 60)} className="mt-1 min-h-9 w-full rounded border border-border bg-surface-alt px-2 text-xs text-text">
                      <option value={30}>30 FPS</option><option value={60}>60 FPS</option>
                    </select>
                  </label>
                  <label className="text-[10px] text-text-muted">Orientation
                    <select value={portrait ? 'portrait' : 'landscape'} onChange={(event) => setPortrait(event.target.value === 'portrait')} className="mt-1 min-h-9 w-full rounded border border-border bg-surface-alt px-2 text-xs text-text">
                      <option value="landscape">Landscape</option><option value="portrait">Portrait</option>
                    </select>
                  </label>
                  {mode === 'screen_camera' && <label className="text-[10px] text-text-muted">Camera position
                    <select value={cameraCorner} onChange={(event) => setCameraCorner(event.target.value as CameraCorner)} className="mt-1 min-h-9 w-full rounded border border-border bg-surface-alt px-2 text-xs text-text">
                      <option value="top-left">Top left</option><option value="top-right">Top right</option><option value="bottom-left">Bottom left</option><option value="bottom-right">Bottom right</option>
                    </select>
                  </label>}
                </div>
              )}

              <div className="grid gap-2 rounded-lg border border-border bg-surface p-3 sm:grid-cols-2">
                <label className="flex items-center gap-2 text-xs text-text-secondary"><input type="checkbox" checked={includeMic || mode === 'voiceover'} disabled={mode === 'voiceover'} onChange={(event) => setIncludeMic(event.target.checked)} /> Include microphone</label>
                <label className="flex items-center gap-2 text-xs text-text-secondary"><input type="checkbox" checked={captureTranscript} onChange={(event) => setCaptureTranscript(event.target.checked)} /> Capture live browser transcript</label>
                <label className="flex items-center gap-2 text-xs text-text-secondary"><input type="checkbox" checked={addToTimeline} onChange={(event) => setAddToTimeline(event.target.checked)} /> Add recording at playhead</label>
                <label className="flex items-center gap-2 text-xs text-text-secondary"><input type="checkbox" checked={createCaptions} disabled={!captureTranscript} onChange={(event) => setCreateCaptions(event.target.checked)} /> Convert transcript to captions</label>
              </div>

              {error && <div role="alert" className="rounded-md border border-red-400/30 bg-red-400/10 px-3 py-2 text-xs text-red-300">{error}</div>}
              <button type="button" onClick={() => { void startRecording(); }} className="inline-flex min-h-10 items-center gap-2 rounded-md bg-primary px-4 text-xs font-semibold text-black hover:bg-primary-hover"><Circle size={12} fill="currentColor" /> Start recording</button>
            </div>
          )}

          {stage === 'recording' && (
            <div className="space-y-3">
              {mode !== 'voiceover' ? (
                <video ref={previewRef} muted playsInline className="max-h-[55vh] w-full rounded-lg bg-black object-contain" />
              ) : (
                <div className="flex min-h-48 items-center justify-center rounded-lg border border-border bg-surface text-text-muted"><Mic size={40} /></div>
              )}
              <div className="flex items-center gap-3 rounded-lg border border-border bg-surface p-3">
                <span className="inline-flex items-center gap-2 text-sm font-semibold text-red-300"><span className="h-2.5 w-2.5 animate-pulse rounded-full bg-red-400" /> {elapsedLabel(elapsedMs)}</span>
                <button type="button" onClick={togglePause} className="inline-flex min-h-9 items-center gap-1 rounded border border-border bg-surface-alt px-3 text-xs text-text-secondary hover:text-text">{paused ? <Play size={13} /> : <Pause size={13} />}{paused ? 'Resume' : 'Pause'}</button>
                <button type="button" onClick={stopRecording} className="inline-flex min-h-9 items-center gap-1 rounded bg-red-500 px-3 text-xs font-semibold text-white hover:bg-red-400"><Square size={12} fill="currentColor" /> Stop</button>
              </div>
              {(transcript || interimTranscript) && <div className="rounded-lg border border-border bg-surface p-3 text-xs leading-relaxed text-text-secondary"><span className="text-text">{transcript}</span> <span className="text-text-muted">{interimTranscript}</span></div>}
            </div>
          )}

          {stage === 'review' && result && (
            <div className="space-y-3">
              {result.isVideo ? <video src={result.url} controls className="max-h-[55vh] w-full rounded-lg bg-black object-contain" /> : <audio src={result.url} controls className="w-full" />}
              {transcript.trim() && (
                <label className="block text-[10px] text-text-muted">Captured transcript
                  <textarea value={transcript} onChange={(event) => setTranscript(event.target.value)} rows={5} className="mt-1 w-full rounded-md border border-border bg-surface-alt px-2 py-2 text-xs text-text" />
                </label>
              )}
              <div className="flex flex-wrap gap-2">
                <button type="button" onClick={() => { void save(); }} disabled={saving} className="inline-flex min-h-10 items-center gap-2 rounded-md bg-primary px-4 text-xs font-semibold text-black disabled:opacity-50">{saving && <Loader2 size={13} className="animate-spin" />} Save recording</button>
                <button type="button" onClick={discard} disabled={saving} className="min-h-10 rounded-md border border-border bg-surface-alt px-4 text-xs text-text-secondary hover:text-text">Discard and retry</button>
              </div>
            </div>
          )}
        </div>
      </div>
      <canvas ref={canvasRef} className="hidden" aria-hidden="true" />
    </div>
  );
}
