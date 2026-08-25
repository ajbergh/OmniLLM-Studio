import { useState } from 'react';
import { Search } from 'lucide-react';
import { EFFECT_CATEGORIES, EFFECT_DEFINITIONS, defaultEffectParams } from './effects/effectRegistry';
import type { EffectCategory, EffectDefinition } from './effects/effectRegistry';
import { TRANSITION_DEFINITIONS } from './effects/transitionRegistry';
import { clampTransitionDurationToPeer, transitionPlacementSupported } from './effects/transitionAuthoring';
import type { TransitionPeerOption } from './effects/transitionAuthoring';
import { ANNOTATION_DEFINITIONS } from './effects/annotationRegistry';
import type { VideoTimelineEffect, VideoTimelineShapeKind, VideoTimelineTransition } from '../../types/video';
import { useVideoStudioStore } from '../../stores/videoStudio';

function SupportBadge({ supported, note }: { supported: boolean; note?: string }) {
  if (supported) {
    return note ? (
      <span className="rounded bg-sky-400/15 px-1 py-0.5 text-[8px] font-semibold uppercase tracking-wide text-sky-300" title={note}>
        ≈ export
      </span>
    ) : null;
  }
  return (
    <span className="rounded bg-amber-400/15 px-1 py-0.5 text-[8px] font-semibold uppercase tracking-wide text-amber-300" title="Shows in the preview but is not applied at export">
      preview only
    </span>
  );
}

/** Searchable, categorized effect cards. Click to apply to the selected clip. */
export function EffectBrowser({ onApply, disabled }: {
  onApply: (effect: Omit<VideoTimelineEffect, 'id'>) => void;
  disabled?: boolean;
}) {
  const [query, setQuery] = useState('');
  const [category, setCategory] = useState<EffectCategory | 'all'>('all');
  const rendererCapabilities = useVideoStudioStore((state) => state.rendererCapabilities);

  const matches = (definition: EffectDefinition) =>
    (category === 'all' || definition.category === category) &&
    (!query.trim() || definition.label.toLowerCase().includes(query.trim().toLowerCase()));

  return (
    <div className="rounded-md border border-border bg-surface-alt/50 p-2">
      <div className="mb-1.5 flex items-center gap-1.5">
        <div className="relative min-w-0 flex-1">
          <Search size={10} className="pointer-events-none absolute left-1.5 top-1/2 -translate-y-1/2 text-text-muted" />
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search effects…"
            className="min-h-7 w-full rounded border border-border bg-surface px-1.5 pl-5 text-[11px] text-text focus:border-primary/50 focus:outline-none"
            aria-label="Search effects"
          />
        </div>
        <select
          value={category}
          onChange={(event) => setCategory(event.target.value as EffectCategory | 'all')}
          className="min-h-7 rounded border border-border bg-surface px-1 text-[10px] text-text-secondary"
          aria-label="Effect category"
        >
          <option value="all">All</option>
          {EFFECT_CATEGORIES.map((item) => (
            <option key={item.key} value={item.key}>{item.label}</option>
          ))}
        </select>
      </div>
      <div className="grid grid-cols-2 gap-1">
        {EFFECT_DEFINITIONS.filter(matches).map((definition) => {
          const runtimeSupport = definition.exportFeature
            ? rendererCapabilities?.features.find((feature) => feature.feature === definition.exportFeature)
            : undefined;
          return (
          <button
            key={definition.type}
            disabled={disabled}
            onClick={() => onApply({ type: definition.type, enabled: true, params: defaultEffectParams(definition) })}
            className="flex items-center justify-between gap-1 rounded border border-border bg-surface px-1.5 py-1.5 text-left text-[11px] text-text-secondary hover:border-primary/40 hover:text-text disabled:cursor-not-allowed disabled:opacity-45"
            title={`Apply ${definition.label} to the selected clip`}
          >
            <span className="min-w-0 truncate">{definition.label}</span>
            <SupportBadge supported={runtimeSupport?.supported ?? definition.exportSupported} note={runtimeSupport?.partial ? runtimeSupport.notes : undefined} />
          </button>
          );
        })}
        {EFFECT_DEFINITIONS.filter(matches).length === 0 && (
          <p className="col-span-2 px-1 py-2 text-center text-[10px] text-text-muted">No effects match.</p>
        )}
      </div>
    </div>
  );
}

/**
 * Annotation palette: every shape/annotation kind as a card with its
 * export-support badge. Click to create one on the timeline at the playhead.
 */
export function AnnotationBrowser({ onAdd, disabled }: {
  onAdd: (kind: VideoTimelineShapeKind) => void;
  disabled?: boolean;
}) {
  return (
    <div className="grid grid-cols-2 gap-1 rounded-md border border-border bg-surface-alt/50 p-2">
      {ANNOTATION_DEFINITIONS.map((definition) => (
        <button
          key={definition.kind}
          disabled={disabled}
          onClick={() => onAdd(definition.kind)}
          className="flex items-center justify-between gap-1 rounded border border-border bg-surface px-1.5 py-1.5 text-left text-[11px] text-text-secondary hover:border-primary/40 hover:text-text disabled:cursor-not-allowed disabled:opacity-45"
          title={definition.exportNote || `Add a ${definition.label.toLowerCase()} at the playhead`}
        >
          <span className="min-w-0 truncate">{definition.label}</span>
          <SupportBadge supported={definition.exportSupport !== 'preview'} note={definition.exportSupport === 'partial' ? definition.exportNote : undefined} />
        </button>
      ))}
    </div>
  );
}

/** Transition cards with explicit canonical placement/peer authoring. */
export function TransitionBrowser({ onApply, peerOptions = [], disabled }: {
  onApply: (transition: Omit<VideoTimelineTransition, 'id'>) => void;
  peerOptions?: TransitionPeerOption[];
  disabled?: boolean;
}) {
  const [placement, setPlacement] = useState<NonNullable<VideoTimelineTransition['placement']>>('in');
  const [peerClipId, setPeerClipId] = useState('');
  const effectivePeer = peerOptions.find((peer) => peer.id === peerClipId) || peerOptions[0];
  return (
    <div className="rounded-md border border-border bg-surface-alt/50 p-2">
      <div className="mb-1.5 flex items-center gap-1">
        <select
          value={placement}
          onChange={(event) => setPlacement(event.target.value as NonNullable<VideoTimelineTransition['placement']>)}
          className="min-h-7 flex-1 rounded border border-border bg-surface px-1 text-[10px] text-text-secondary"
          aria-label="Transition placement"
        >
          <option value="in">In · clip start</option>
          <option value="out">Out · clip end</option>
          <option value="between" disabled={peerOptions.length === 0}>Between · overlapping peer</option>
        </select>
        {placement === 'between' && (
          <select
            value={effectivePeer?.id || ''}
            onChange={(event) => setPeerClipId(event.target.value)}
            className="min-h-7 min-w-0 flex-1 rounded border border-border bg-surface px-1 text-[10px] text-text-secondary"
            aria-label="Transition peer clip"
          >
            {peerOptions.map((peer) => <option key={peer.id} value={peer.id}>{peer.label}</option>)}
          </select>
        )}
      </div>
      <div className="grid grid-cols-2 gap-1">
        {TRANSITION_DEFINITIONS.map((definition) => {
          const durationMs = placement === 'between' && effectivePeer
            ? clampTransitionDurationToPeer(definition.defaultDurationMs, effectivePeer)
            : definition.defaultDurationMs;
          const placementSupported = transitionPlacementSupported(definition.type, placement);
          const unavailable = disabled || !placementSupported || (placement === 'between' && !effectivePeer);
          return (
            <button
              key={definition.type}
              disabled={unavailable}
              onClick={() => onApply({
                type: definition.type,
                duration_ms: durationMs,
                placement,
                ...(placement === 'between' && effectivePeer ? { peer_clip_id: effectivePeer.id } : {}),
                ...(definition.supportsDirection ? { direction: 'left' as const } : {}),
              })}
              className="flex items-center justify-between gap-1 rounded border border-border bg-surface px-1.5 py-1.5 text-left text-[11px] text-text-secondary hover:border-primary/40 hover:text-text disabled:cursor-not-allowed disabled:opacity-45"
              title={!placementSupported
                ? `${definition.label} does not support ${placement} placement`
                : (definition.exportNote || `Add a ${definition.label} transition to the selected clip`)}
            >
              <span className="min-w-0 truncate">{definition.label}</span>
              <SupportBadge supported={definition.exportSupported} note={definition.exportNote} />
            </button>
          );
        })}
      </div>
      {placement === 'between' && peerOptions.length === 0 && (
        <p className="mt-1 text-[9px] text-amber-300">Between transitions require at least 100ms of real overlap with another visible visual clip.</p>
      )}
    </div>
  );
}
