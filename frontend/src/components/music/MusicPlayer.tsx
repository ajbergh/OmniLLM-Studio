import { useRef, useState } from 'react';
import { Download, Pause, Play, Repeat, Volume2 } from 'lucide-react';
import { clsx } from 'clsx';

interface MusicPlayerProps {
  src?: string;
  fileName?: string;
}

export function MusicPlayer({ src, fileName = 'track.mp3' }: MusicPlayerProps) {
  const audioRef = useRef<HTMLAudioElement>(null);
  const [playing, setPlaying] = useState(false);
  const [loop, setLoop] = useState(false);
  const [volume, setVolume] = useState(0.85);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);

  const formatTime = (seconds: number) => {
    if (!Number.isFinite(seconds)) return '0:00';
    const minutes = Math.floor(seconds / 60);
    return `${minutes}:${String(Math.floor(seconds % 60)).padStart(2, '0')}`;
  };

  const togglePlay = async () => {
    const audio = audioRef.current;
    if (!audio || !src) return;
    if (audio.paused) {
      await audio.play();
    } else {
      audio.pause();
    }
  };

  return (
    <div className="rounded-xl border border-border bg-surface-alt p-3">
      <audio
        ref={audioRef}
        src={src}
        loop={loop}
        onPlay={() => setPlaying(true)}
        onPause={() => setPlaying(false)}
        onEnded={() => setPlaying(false)}
        onLoadedMetadata={(event) => setDuration(event.currentTarget.duration || 0)}
        onTimeUpdate={(event) => setCurrentTime(event.currentTarget.currentTime)}
        preload="metadata"
        className="sr-only"
      />

      <div className="flex flex-wrap items-center gap-2">
        <button
          onClick={togglePlay}
          disabled={!src}
          className={clsx(
            'min-h-10 min-w-10 inline-flex items-center justify-center rounded-lg transition-colors',
            src ? 'bg-primary text-white hover:bg-primary-hover shadow-glow' : 'bg-surface-hover text-text-muted/40 cursor-not-allowed'
          )}
          aria-label={playing ? 'Pause track' : 'Play track'}
          title={playing ? 'Pause' : 'Play'}
        >
          {playing ? <Pause size={16} /> : <Play size={16} />}
        </button>

        <label className="flex min-h-10 min-w-48 flex-[2] items-center gap-2 rounded-lg border border-border bg-surface px-3 text-xs text-text-muted">
          <span className="w-8 font-mono">{formatTime(currentTime)}</span>
          <input
            type="range"
            min={0}
            max={duration || 0}
            step={0.1}
            value={Math.min(currentTime, duration || 0)}
            disabled={!src || !duration}
            onChange={(event) => {
              const next = Number(event.target.value);
              setCurrentTime(next);
              if (audioRef.current) audioRef.current.currentTime = next;
            }}
            className="min-w-24 flex-1 accent-primary"
            aria-label="Track position"
          />
          <span className="w-8 text-right font-mono">{formatTime(duration)}</span>
        </label>

        <button
          onClick={() => setLoop((value) => !value)}
          disabled={!src}
          className={clsx(
            'min-h-10 min-w-10 inline-flex items-center justify-center rounded-lg transition-colors',
            loop ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text hover:bg-surface-hover',
            !src && 'opacity-40 cursor-not-allowed'
          )}
          aria-label={loop ? 'Disable loop' : 'Enable loop'}
          title="Loop"
        >
          <Repeat size={15} />
        </button>

        <label className="flex min-h-10 min-w-40 flex-1 items-center gap-2 rounded-lg border border-border bg-surface px-3 text-xs text-text-muted">
          <Volume2 size={14} />
          <input
            type="range"
            min={0}
            max={1}
            step={0.01}
            value={volume}
            onChange={(event) => {
              const next = Number(event.target.value);
              setVolume(next);
              if (audioRef.current) audioRef.current.volume = next;
            }}
            className="w-full accent-primary"
            aria-label="Volume"
          />
        </label>

        {src && (
          <a
            href={src}
            download={fileName}
            className="min-h-10 inline-flex items-center gap-2 rounded-lg border border-border bg-surface px-3 text-xs font-medium text-text-secondary hover:text-text hover:bg-surface-hover transition-colors"
          >
            <Download size={14} />
            Download
          </a>
        )}
      </div>
    </div>
  );
}
