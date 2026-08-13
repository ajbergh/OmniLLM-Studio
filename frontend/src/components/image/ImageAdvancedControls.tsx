import { useEffect, useMemo, useRef, useState } from 'react';
import { ChevronDown, ChevronUp, Shuffle } from 'lucide-react';
import { clsx } from 'clsx';
import { useImageEditorStore } from '../../stores/imageEditor';

interface ImageAdvancedControlsProps {
  size: string;
  onSizeChange: (size: string) => void;
  seed: number | null;
  onSeedChange: (seed: number | null) => void;
  guidance: number;
  onGuidanceChange: (value: number) => void;
  variants: number;
  onVariantsChange: (n: number) => void;
  supportsSeed?: boolean;
  supportsGuidance?: boolean;
  maxVariants?: number;
  supportedSizes?: string[];
}

interface SizeOption {
  value: string;
  label: string;
  desc: string;
}

const SIZES: SizeOption[] = [
  { value: '1024x1024', label: '1:1', desc: '1024×1024' },
  { value: '1792x1024', label: '16:9', desc: '1792×1024' },
  { value: '1024x1792', label: '9:16', desc: '1024×1792' },
];

function gcd(a: number, b: number): number {
  return b === 0 ? a : gcd(b, a % b);
}

export function buildSizeOptions(supportedSizes?: string[]): SizeOption[] {
  if (!supportedSizes || supportedSizes.length === 0) return SIZES;

  return supportedSizes.map((size) => {
    const known = SIZES.find((candidate) => candidate.value === size);
    if (known) return known;

    if (size === 'auto') {
      return { value: size, label: 'Auto', desc: 'Provider-selected output size' };
    }

    const match = /^(\d+)x(\d+)$/.exec(size);
    if (!match) {
      // Capability data can evolve independently of the client. Unknown values
      // must remain selectable instead of crashing while trying to derive a ratio.
      return { value: size, label: size, desc: size };
    }

    const w = Number(match[1]);
    const h = Number(match[2]);
    if (!Number.isFinite(w) || !Number.isFinite(h) || w <= 0 || h <= 0) {
      return { value: size, label: size, desc: size };
    }

    const divisor = gcd(w, h);
    return { value: size, label: `${w / divisor}:${h / divisor}`, desc: `${w}×${h}` };
  });
}

export function getSourcePreservingEditSize(supportedSizes?: string[]): string {
  // OpenAI exposes `auto`; its edit endpoint rejects a blank size in the
  // current backend adapter because the adapter otherwise falls back to 1:1.
  // Other image providers preserve their input geometry when no explicit size
  // is sent, so use an empty value for those providers.
  return supportedSizes?.includes('auto') ? 'auto' : '';
}

function getGenerationFallbackSize(supportedSizes?: string[]): string {
  const explicitSizes = supportedSizes?.filter((value) => value !== 'auto') ?? [];
  if (explicitSizes.includes('1024x1024')) return '1024x1024';
  return explicitSizes[0] ?? '1024x1024';
}

export function ImageAdvancedControls({
  size,
  onSizeChange,
  seed,
  onSeedChange,
  guidance,
  onGuidanceChange,
  variants,
  onVariantsChange,
  supportsSeed,
  supportsGuidance,
  maxVariants,
  supportedSizes,
}: ImageAdvancedControlsProps) {
  const [expanded, setExpanded] = useState(false);
  const editMode = useImageEditorStore((state) => state.editMode);
  const previousModeRef = useRef<typeof editMode | null>(null);

  const showSeed = supportsSeed === true;
  const showGuidance = supportsGuidance === true;
  const variantCap = maxVariants ?? 4;
  const filteredSizes = useMemo(() => buildSizeOptions(supportedSizes), [supportedSizes]);
  const sourcePreservingSize = useMemo(
    () => getSourcePreservingEditSize(supportedSizes),
    [supportedSizes],
  );

  useEffect(() => {
    if (variants > variantCap) onVariantsChange(Math.max(1, variantCap));
  }, [onVariantsChange, variantCap, variants]);

  // A generation size is not a safe default for an edit. Entering edit mode
  // must return to provider-native source preservation unless the user then
  // explicitly selects a new output ratio/size.
  useEffect(() => {
    if (previousModeRef.current === editMode) return;
    previousModeRef.current = editMode;

    if (editMode === 'edit') {
      if (size !== sourcePreservingSize) onSizeChange(sourcePreservingSize);
      return;
    }

    if (size === '' || size === 'auto') {
      onSizeChange(getGenerationFallbackSize(supportedSizes));
    }
  }, [editMode, onSizeChange, size, sourcePreservingSize, supportedSizes]);

  // Provider/model changes can invalidate a previously selected output size.
  // Prefer the source-preserving edit behavior rather than silently sending a
  // stale size from the previous model.
  useEffect(() => {
    if (!supportedSizes || supportedSizes.length === 0) return;

    if (editMode === 'edit') {
      if (size === '' || size === 'auto') {
        if (size !== sourcePreservingSize) onSizeChange(sourcePreservingSize);
        return;
      }
      if (!supportedSizes.includes(size)) onSizeChange(sourcePreservingSize);
      return;
    }

    if (!supportedSizes.includes(size)) {
      onSizeChange(getGenerationFallbackSize(supportedSizes));
    }
  }, [editMode, onSizeChange, size, sourcePreservingSize, supportedSizes]);

  const displayedSizes = useMemo(() => {
    if (editMode !== 'edit') return filteredSizes;

    const sourceOption: SizeOption = {
      value: sourcePreservingSize,
      label: 'Source',
      desc: sourcePreservingSize === 'auto'
        ? 'Do not force 1:1; let the provider choose edit output geometry'
        : 'Preserve the source image geometry unless you explicitly choose another size',
    };
    return [sourceOption, ...filteredSizes.filter((option) => option.value !== sourcePreservingSize)];
  }, [editMode, filteredSizes, sourcePreservingSize]);

  return (
    <div className="border border-border rounded-xl overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between px-3 py-2 text-xs font-medium text-text-muted
                   hover:text-text hover:bg-surface-hover transition-colors"
      >
        Advanced Controls
        {expanded ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
      </button>

      {expanded && (
        <div className="px-3 pb-3 space-y-3 border-t border-border">
          {/* Size / Aspect Ratio */}
          <div className="space-y-1.5 pt-2">
            <label className="text-[10px] text-text-muted uppercase tracking-wide">Size</label>
            <div className="flex flex-wrap gap-1.5">
              {displayedSizes.map((option) => (
                <button
                  key={option.value || 'source'}
                  onClick={() => onSizeChange(option.value)}
                  className={clsx(
                    'min-w-12 flex-1 px-2 py-1.5 rounded-lg text-[10px] transition-colors',
                    size === option.value
                      ? 'bg-primary/20 text-primary border border-primary/30'
                      : 'bg-surface border border-border text-text-muted hover:text-text'
                  )}
                  title={option.desc}
                >
                  {option.label}
                </button>
              ))}
            </div>
          </div>

          {/* Seed */}
          {showSeed && (
          <div className="space-y-1.5">
            <label className="text-[10px] text-text-muted uppercase tracking-wide">Seed</label>
            <div className="flex items-center gap-1.5">
              <input
                type="number"
                value={seed ?? ''}
                onChange={(e) => {
                  const v = e.target.value;
                  onSeedChange(v === '' ? null : parseInt(v, 10));
                }}
                placeholder="Random"
                className="flex-1 px-2 py-1 text-xs rounded-lg bg-surface border border-border
                           text-text placeholder:text-text-muted/50 focus:outline-none focus:border-primary/40
                           [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
              />
              <button
                onClick={() => onSeedChange(null)}
                className="p-1.5 rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors"
                title="Randomize"
              >
                <Shuffle size={12} />
              </button>
            </div>
          </div>
          )}

          {/* Guidance */}
          {showGuidance && (
          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <label className="text-[10px] text-text-muted uppercase tracking-wide">Guidance</label>
              <span className="text-[10px] text-text-muted font-mono">{guidance.toFixed(1)}</span>
            </div>
            <input
              type="range"
              min={1}
              max={20}
              step={0.5}
              value={guidance}
              onChange={(e) => onGuidanceChange(Number(e.target.value))}
              className="w-full accent-primary"
            />
            <div className="flex justify-between text-[9px] text-text-muted/50">
              <span>Lower</span>
              <span>Stronger</span>
            </div>
          </div>
          )}

          {/* Variants */}
          <div className="space-y-1.5">
            <label className="text-[10px] text-text-muted uppercase tracking-wide">Variants</label>
            <div className="flex gap-1.5">
              {[1, 2, 3, 4].filter((n) => n <= variantCap).map((n) => (
                <button
                  key={n}
                  onClick={() => onVariantsChange(n)}
                  className={clsx(
                    'flex-1 py-1 rounded-lg text-xs transition-colors',
                    variants === n
                      ? 'bg-primary/20 text-primary border border-primary/30'
                      : 'bg-surface border border-border text-text-muted hover:text-text'
                  )}
                >
                  {n}
                </button>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
