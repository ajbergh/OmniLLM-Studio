import { useState } from 'react';
import { ChevronDown, ChevronUp, Shuffle } from 'lucide-react';
import { clsx } from 'clsx';

interface ImageAdvancedControlsProps {
  size: string;
  onSizeChange: (size: string) => void;
  seed: number | null;
  onSeedChange: (seed: number | null) => void;
  creativity: number;
  onCreativityChange: (value: number) => void;
  variants: number;
  onVariantsChange: (n: number) => void;
  supportsSeed?: boolean;
  supportsGuidance?: boolean;
  maxVariants?: number;
  supportedSizes?: string[];
}

const SIZES = [
  { value: '1024x1024', label: '1:1', desc: '1024×1024' },
  { value: '1792x1024', label: '16:9', desc: '1792×1024' },
  { value: '1024x1792', label: '9:16', desc: '1024×1792' },
];

function gcd(a: number, b: number): number {
  return b === 0 ? a : gcd(b, a % b);
}

function buildSizeOptions(supportedSizes?: string[]) {
  if (!supportedSizes || supportedSizes.length === 0) return SIZES;
  return supportedSizes.map((s) => {
    const known = SIZES.find((k) => k.value === s);
    if (known) return known;
    const [wStr, hStr] = s.split('x');
    const w = parseInt(wStr, 10);
    const h = parseInt(hStr, 10);
    const d = gcd(w, h);
    return { value: s, label: `${w / d}:${h / d}`, desc: `${w}×${h}` };
  });
}

export function ImageAdvancedControls({
  size,
  onSizeChange,
  seed,
  onSeedChange,
  creativity,
  onCreativityChange,
  variants,
  onVariantsChange,
  supportsSeed,
  supportsGuidance,
  maxVariants,
  supportedSizes,
}: ImageAdvancedControlsProps) {
  const [expanded, setExpanded] = useState(false);

  const showSeed = supportsSeed === true;
  const showCreativity = supportsGuidance === true;
  const variantCap = maxVariants ?? 4;
  const filteredSizes = buildSizeOptions(supportedSizes);

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
            <div className="flex gap-1.5">
              {filteredSizes.map((s) => (
                <button
                  key={s.value}
                  onClick={() => onSizeChange(s.value)}
                  className={clsx(
                    'flex-1 px-2 py-1.5 rounded-lg text-[10px] transition-colors',
                    size === s.value
                      ? 'bg-primary/20 text-primary border border-primary/30'
                      : 'bg-surface border border-border text-text-muted hover:text-text'
                  )}
                  title={s.desc}
                >
                  {s.label}
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

          {/* Creativity */}
          {showCreativity && (
          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <label className="text-[10px] text-text-muted uppercase tracking-wide">Creativity</label>
              <span className="text-[10px] text-text-muted font-mono">{Math.round(creativity * 100)}%</span>
            </div>
            <input
              type="range"
              min={0}
              max={100}
              value={Math.round(creativity * 100)}
              onChange={(e) => onCreativityChange(Number(e.target.value) / 100)}
              className="w-full accent-primary"
            />
            <div className="flex justify-between text-[9px] text-text-muted/50">
              <span>Precise</span>
              <span>Creative</span>
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
