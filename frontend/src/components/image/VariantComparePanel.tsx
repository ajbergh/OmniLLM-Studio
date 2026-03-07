import { useState, useRef, useCallback, useEffect } from 'react';
import { X, Columns2, Layers, Check } from 'lucide-react';
import { clsx } from 'clsx';
import type { ImageNodeAsset } from '../../types';
import { attachmentUrl } from '../../api';

type CompareMode = 'side-by-side' | 'overlay';

interface VariantComparePanelProps {
  assets: ImageNodeAsset[];
  onSelect: (assetId: string) => void;
  onClose: () => void;
}

export function VariantComparePanel({ assets, onSelect, onClose }: VariantComparePanelProps) {
  const [mode, setMode] = useState<CompareMode>('side-by-side');
  const [sliderPos, setSliderPos] = useState(50);
  const [dragActive, setDragActive] = useState(false);
  const overlayRef = useRef<HTMLDivElement>(null);
  const [leftIdx, setLeftIdx] = useState(0);
  const [rightIdx, setRightIdx] = useState(Math.min(1, assets.length - 1));

  // Synchronized zoom/pan state
  const [zoom, setZoom] = useState(1);
  const [pan, setPan] = useState({ x: 0, y: 0 });
  const isPanning = useRef(false);
  const panStart = useRef({ x: 0, y: 0, panX: 0, panY: 0 });

  const handleWheel = useCallback((e: React.WheelEvent) => {
    e.preventDefault();
    const factor = e.deltaY < 0 ? 1.1 : 0.9;
    setZoom((z) => Math.min(Math.max(z * factor, 0.2), 8));
  }, []);

  const handlePointerDown = useCallback((e: React.PointerEvent) => {
    if (e.button === 1 || (e.button === 0 && e.altKey)) {
      isPanning.current = true;
      panStart.current = { x: e.clientX, y: e.clientY, panX: pan.x, panY: pan.y };
      (e.target as HTMLElement).setPointerCapture(e.pointerId);
    }
  }, [pan]);

  const handlePointerMove = useCallback((e: React.PointerEvent) => {
    if (isPanning.current) {
      setPan({
        x: panStart.current.panX + (e.clientX - panStart.current.x),
        y: panStart.current.panY + (e.clientY - panStart.current.y),
      });
    }
  }, []);

  const handlePointerUp = useCallback(() => {
    isPanning.current = false;
  }, []);

  // Overlay slider drag
  const handleSliderMove = useCallback((e: React.PointerEvent) => {
    if (!dragActive || !overlayRef.current) return;
    const rect = overlayRef.current.getBoundingClientRect();
    const pct = ((e.clientX - rect.left) / rect.width) * 100;
    setSliderPos(Math.min(Math.max(pct, 0), 100));
  }, [dragActive]);

  useEffect(() => {
    if (!dragActive) return;
    const up = () => setDragActive(false);
    window.addEventListener('pointerup', up);
    return () => window.removeEventListener('pointerup', up);
  }, [dragActive]);

  const imgStyle = {
    transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`,
    transformOrigin: 'center center',
  };

  if (assets.length < 2) return null;

  return (
    <div className="fixed inset-0 z-60 bg-surface/95 backdrop-blur-sm flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-border bg-surface-raised shrink-0">
        <div className="flex items-center gap-4">
          <span className="text-sm font-medium text-text">Compare Variants</span>
          <div className="flex rounded-lg bg-surface border border-border p-0.5">
            <button
              onClick={() => setMode('side-by-side')}
              className={clsx(
                'px-2.5 py-1 rounded-md text-xs flex items-center gap-1.5 transition-colors',
                mode === 'side-by-side'
                  ? 'bg-primary/20 text-primary'
                  : 'text-text-muted hover:text-text'
              )}
            >
              <Columns2 size={12} /> Side-by-side
            </button>
            <button
              onClick={() => setMode('overlay')}
              className={clsx(
                'px-2.5 py-1 rounded-md text-xs flex items-center gap-1.5 transition-colors',
                mode === 'overlay'
                  ? 'bg-primary/20 text-primary'
                  : 'text-text-muted hover:text-text'
              )}
            >
              <Layers size={12} /> Overlay
            </button>
          </div>
          <span className="text-[10px] text-text-muted">
            Alt+drag to pan · Scroll to zoom
          </span>
        </div>
        <button
          onClick={onClose}
          className="p-1.5 rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors"
        >
          <X size={18} />
        </button>
      </div>

      {/* Compare area */}
      <div
        className="flex-1 min-h-0 overflow-hidden"
        onWheel={handleWheel}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
      >
        {mode === 'side-by-side' ? (
          <div className="h-full flex gap-1 p-2">
            {assets.map((asset, idx) => (
              <div key={asset.id} className="flex-1 relative flex items-center justify-center bg-surface rounded-lg overflow-hidden">
                <img
                  src={attachmentUrl(asset.attachment_id)}
                  alt={`Variant ${idx + 1}`}
                  className="max-w-full max-h-full object-contain select-none"
                  style={imgStyle}
                  draggable={false}
                />
                <div className="absolute top-2 left-2 flex items-center gap-1.5">
                  <span className="text-[10px] bg-surface-glass backdrop-blur-sm rounded-md px-1.5 py-0.5 text-text-muted border border-border">
                    #{idx + 1}
                    {asset.is_selected && ' (Current)'}
                  </span>
                </div>
                <button
                  onClick={() => { onSelect(asset.id); onClose(); }}
                  className={clsx(
                    'absolute bottom-2 left-1/2 -translate-x-1/2 px-3 py-1.5 rounded-lg text-xs font-medium',
                    'flex items-center gap-1.5 transition-all',
                    asset.is_selected
                      ? 'bg-primary/20 text-primary border border-primary/30'
                      : 'bg-surface-glass backdrop-blur-sm text-text border border-border hover:border-primary/40'
                  )}
                >
                  <Check size={12} /> {asset.is_selected ? 'Current' : 'Set as Current'}
                </button>
              </div>
            ))}
          </div>
        ) : (
          /* Overlay mode with slider */
          <div
            ref={overlayRef}
            className="h-full relative flex items-center justify-center cursor-col-resize select-none"
            onPointerMove={handleSliderMove}
          >
            {/* Variant selector for overlay */}
            {assets.length > 2 && (
              <div className="absolute top-2 left-1/2 -translate-x-1/2 flex gap-2 z-30">
                <select
                  value={leftIdx}
                  onChange={(e) => setLeftIdx(Number(e.target.value))}
                  className="text-[10px] bg-surface-glass backdrop-blur-sm rounded-md px-1.5 py-0.5 text-text-muted border border-border"
                >
                  {assets.map((_, i) => (
                    <option key={i} value={i}>#{i + 1}</option>
                  ))}
                </select>
                <span className="text-[10px] text-text-muted self-center">vs</span>
                <select
                  value={rightIdx}
                  onChange={(e) => setRightIdx(Number(e.target.value))}
                  className="text-[10px] bg-surface-glass backdrop-blur-sm rounded-md px-1.5 py-0.5 text-text-muted border border-border"
                >
                  {assets.map((_, i) => (
                    <option key={i} value={i}>#{i + 1}</option>
                  ))}
                </select>
              </div>
            )}
            {/* Bottom layer — right variant */}
            <div className="absolute inset-0 flex items-center justify-center">
              <img
                src={attachmentUrl(assets[rightIdx].attachment_id)}
                alt={`Variant ${rightIdx + 1}`}
                className="max-w-full max-h-full object-contain"
                style={imgStyle}
                draggable={false}
              />
            </div>
            {/* Top layer — left variant, clipped */}
            <div
              className="absolute inset-0 flex items-center justify-center overflow-hidden"
              style={{ clipPath: `inset(0 ${100 - sliderPos}% 0 0)` }}
            >
              <img
                src={attachmentUrl(assets[leftIdx].attachment_id)}
                alt={`Variant ${leftIdx + 1}`}
                className="max-w-full max-h-full object-contain"
                style={imgStyle}
                draggable={false}
              />
            </div>
            {/* Slider line */}
            <div
              className="absolute top-0 bottom-0 w-0.5 bg-primary z-10 cursor-col-resize"
              style={{ left: `${sliderPos}%` }}
              onPointerDown={(e) => { e.stopPropagation(); setDragActive(true); }}
            >
              <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2
                              w-6 h-6 rounded-full bg-primary flex items-center justify-center shadow-glow">
                <Columns2 size={12} className="text-white" />
              </div>
            </div>
            {/* Labels */}
            <div className="absolute top-2 left-2 text-[10px] bg-surface-glass backdrop-blur-sm rounded-md px-1.5 py-0.5 text-text-muted border border-border z-20">
              #{leftIdx + 1} {assets[leftIdx].is_selected && '(Current)'}
            </div>
            <div className="absolute top-2 right-2 text-[10px] bg-surface-glass backdrop-blur-sm rounded-md px-1.5 py-0.5 text-text-muted border border-border z-20">
              #{rightIdx + 1} {assets[rightIdx].is_selected && '(Current)'}
            </div>
            {/* Select buttons below overlay */}
            <div className="absolute bottom-2 left-1/2 -translate-x-1/2 flex gap-2 z-20">
              {[leftIdx, rightIdx].map((idx) => {
                const asset = assets[idx];
                return (
                  <button
                    key={asset.id}
                    onClick={() => { onSelect(asset.id); onClose(); }}
                    className={clsx(
                      'px-3 py-1.5 rounded-lg text-xs font-medium flex items-center gap-1.5 transition-all',
                      asset.is_selected
                        ? 'bg-primary/20 text-primary border border-primary/30'
                        : 'bg-surface-glass backdrop-blur-sm text-text border border-border hover:border-primary/40'
                    )}
                  >
                    <Check size={12} /> #{idx + 1} {asset.is_selected ? '(Current)' : ''}
                  </button>
                );
              })}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
