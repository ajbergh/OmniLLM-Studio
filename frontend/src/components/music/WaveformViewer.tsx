import { useEffect, useRef, useState } from 'react';
import { Activity } from 'lucide-react';

interface WaveformViewerProps {
  src?: string;
  active?: boolean;
}

export function WaveformViewer({ src, active = false }: WaveformViewerProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    const canvas = canvasRef.current;
    const ctx = canvas?.getContext('2d');
    if (!canvas || !ctx || !src) return;

    let cancelled = false;
    const currentCanvas = canvas;
    const currentCtx = ctx;
    const currentSrc = src;
    setError(false);

    async function renderWaveform() {
      try {
        const response = await fetch(currentSrc);
        const arrayBuffer = await response.arrayBuffer();
        const audioContext = new AudioContext();
        const buffer = await audioContext.decodeAudioData(arrayBuffer.slice(0));
        await audioContext.close();
        if (cancelled) return;

        const width = currentCanvas.width;
        const height = currentCanvas.height;
        const data = buffer.getChannelData(0);
        const step = Math.max(1, Math.floor(data.length / width));
        const amp = height / 2;
        currentCtx.clearRect(0, 0, width, height);
        currentCtx.fillStyle = 'rgba(255,255,255,0.04)';
        currentCtx.fillRect(0, 0, width, height);
        currentCtx.strokeStyle = active ? '#a78bfa' : '#818cf8';
        currentCtx.lineWidth = 1;
        currentCtx.beginPath();
        for (let x = 0; x < width; x += 1) {
          let min = 1;
          let max = -1;
          const offset = x * step;
          for (let i = 0; i < step && offset + i < data.length; i += 1) {
            const value = data[offset + i];
            if (value < min) min = value;
            if (value > max) max = value;
          }
          currentCtx.moveTo(x, (1 + min) * amp);
          currentCtx.lineTo(x, (1 + max) * amp);
        }
        currentCtx.stroke();
      } catch {
        if (!cancelled) setError(true);
      }
    }

    renderWaveform();
    return () => {
      cancelled = true;
    };
  }, [src, active]);

  if (!src || error) {
    return (
      <div className="h-24 rounded-xl border border-border bg-surface-alt flex items-center justify-center text-text-muted">
        <Activity size={18} className="opacity-50" />
      </div>
    );
  }

  return (
    <canvas
      ref={canvasRef}
      width={960}
      height={160}
      className="h-24 w-full rounded-xl border border-border bg-surface-alt"
      aria-label="Audio waveform"
    />
  );
}
