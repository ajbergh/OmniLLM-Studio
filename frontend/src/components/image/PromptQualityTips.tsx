import { useEffect, useState } from 'react';
import { Check, AlertTriangle } from 'lucide-react';
import { clsx } from 'clsx';

interface Tip {
  id: string;
  label: string;
  ok: boolean;
}

function analyzePrompt(text: string): Tip[] {
  const trimmed = text.trim();
  const lower = trimmed.toLowerCase();
  const tips: Tip[] = [];

  // Subject specificity
  tips.push({
    id: 'subject',
    label: 'Subject described',
    ok: trimmed.length >= 8,
  });

  // Style descriptors
  const styleWords = [
    'realistic', 'cartoon', 'anime', 'watercolor', 'oil painting', 'digital art',
    'photograph', 'illustration', 'sketch', '3d render', 'pixel art', 'cinematic',
    'photorealistic', 'abstract', 'surreal', 'minimalist', 'vintage', 'retro',
    'flat', 'cyberpunk', 'steampunk', 'fantasy', 'sci-fi', 'impressionist',
    'pop art', 'concept art', 'isometric', 'low poly',
  ];
  tips.push({
    id: 'style',
    label: 'Style descriptor',
    ok: styleWords.some((w) => lower.includes(w)),
  });

  // Lighting / atmosphere
  const lightWords = [
    'lighting', 'light', 'shadow', 'glow', 'bright', 'dark', 'ambient',
    'dramatic', 'golden hour', 'sunset', 'sunrise', 'neon', 'backlit',
    'rim light', 'soft light', 'harsh', 'moody', 'warm', 'cool', 'fog',
    'mist', 'haze', 'overcast', 'sunny', 'cloudy', 'moonlight',
  ];
  tips.push({
    id: 'lighting',
    label: 'Lighting or atmosphere',
    ok: lightWords.some((w) => lower.includes(w)),
  });

  // Composition
  const compWords = [
    'close-up', 'closeup', 'wide shot', 'overhead', 'aerial', 'bird',
    'eye level', 'low angle', 'high angle', 'macro', 'portrait',
    'landscape', 'panoramic', 'centered', 'rule of thirds', 'symmetr',
    'full body', 'half body', 'headshot', 'medium shot', 'establishing',
  ];
  tips.push({
    id: 'composition',
    label: 'Composition hint',
    ok: compWords.some((w) => lower.includes(w)),
  });

  // Length check
  const wordCount = trimmed.split(/\s+/).length;
  tips.push({
    id: 'length',
    label: wordCount < 5 ? 'Add more detail (very short)' : 'Sufficient detail',
    ok: wordCount >= 5,
  });

  return tips;
}

interface PromptQualityTipsProps {
  prompt: string;
}

export function PromptQualityTips({ prompt }: PromptQualityTipsProps) {
  const [tips, setTips] = useState<Tip[]>([]);

  useEffect(() => {
    if (!prompt.trim()) {
      setTips([]);
      return;
    }
    const timer = setTimeout(() => {
      setTips(analyzePrompt(prompt));
    }, 500);
    return () => clearTimeout(timer);
  }, [prompt]);

  if (tips.length === 0) return null;

  const score = tips.filter((t) => t.ok).length;
  const total = tips.length;

  return (
    <div className="space-y-1">
      <div className="flex items-center gap-1.5">
        <div className="flex-1 h-1 rounded-full bg-surface overflow-hidden">
          <div
            className={clsx(
              'h-full rounded-full transition-all duration-300',
              score === total ? 'bg-emerald-500' : score >= 3 ? 'bg-amber-400' : 'bg-red-400'
            )}
            style={{ width: `${(score / total) * 100}%` }}
          />
        </div>
        <span className="text-[9px] text-text-muted font-mono">
          {score}/{total}
        </span>
      </div>
      <div className="flex flex-wrap gap-x-2 gap-y-0.5">
        {tips.map((t) => (
          <span
            key={t.id}
            className={clsx(
              'inline-flex items-center gap-0.5 text-[9px]',
              t.ok ? 'text-emerald-400' : 'text-amber-400/70'
            )}
          >
            {t.ok ? <Check size={8} /> : <AlertTriangle size={8} />}
            {t.label}
          </span>
        ))}
      </div>
    </div>
  );
}
