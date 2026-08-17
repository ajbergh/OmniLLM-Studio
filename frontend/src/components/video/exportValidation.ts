import type { VideoAsset, VideoExportSettings, VideoRendererCapabilities, VideoTimelineDocument } from '../../types/video';
import { EFFECT_DEFINITIONS } from './effects/effectRegistry';
import { TRANSITION_DEFINITIONS } from './effects/transitionRegistry';
import { ANNOTATION_DEFINITIONS } from './effects/annotationRegistry';

export interface ExportValidationResult {
  /** Real problems — rendering is blocked until they are fixed. */
  errors: string[];
  /** Fidelity or sanity concerns — the user may render anyway. */
  warnings: string[];
}

/**
 * Pre-render checklist. Errors block the render; warnings are informational.
 * Mirrors backend constraints (service.validateExportSettings) where possible
 * so failures surface before the job is queued.
 */
export function validateExport(
  timeline: VideoTimelineDocument | null,
  assets: VideoAsset[],
  settings: VideoExportSettings,
  capabilities: VideoRendererCapabilities | null,
): ExportValidationResult {
  const errors: string[] = [];
  const warnings: string[] = [];
  const fidelityIssues: string[] = [];
  if (!timeline) {
    return { errors: ['No timeline loaded'], warnings };
  }

  const allClips = timeline.tracks.flatMap((track) => track.clips.map((clip) => ({ track, clip })));
  if (allClips.length === 0) {
    errors.push('The timeline is empty — add at least one clip');
  }
  if (timeline.duration_ms <= 0) {
    errors.push('Timeline duration is zero');
  }
  if (timeline.duration_ms > 60 * 60 * 1000) {
    warnings.push('Timeline is over an hour long — the render may take a very long time');
  } else if (timeline.duration_ms > 20 * 60 * 1000) {
    warnings.push('Long timeline (20+ minutes) — expect a slow render');
  }

  if (settings.resolution === 'custom' && (!settings.width || !settings.height)) {
    errors.push('Custom export size needs both width and height');
  }
  const width = settings.width || 0;
  const height = settings.height || 0;
  if ((width !== 0 || height !== 0) && (width < 16 || height < 16 || width > 7680 || height > 7680)) {
    errors.push('Export width/height must be between 16 and 7680 pixels');
  }
  if ((settings.fps || 0) > 120) {
    errors.push('FPS must be 120 or lower');
  }
  if (width * height >= 3840 * 2160 && (settings.fps || 30) >= 60) {
    warnings.push('4K at 60fps is a very large render');
  }
  if (settings.codec === 'h265') {
    if (settings.format !== 'mp4') {
      errors.push('H.265 requires the MP4 format');
    } else {
      warnings.push('H.265 needs an FFmpeg build with libx265 — if the render fails, check the job diagnostics');
    }
  }
  if (
    settings.range_end_ms !== undefined && settings.range_end_ms > 0 &&
    settings.range_start_ms !== undefined && settings.range_end_ms <= settings.range_start_ms
  ) {
    errors.push('Export range end must be after its start');
  }

  // Missing assets break clips at render time.
  const assetIds = new Set(assets.map((asset) => asset.id));
  const missing = allClips.filter(({ clip }) => clip.asset_id && !assetIds.has(clip.asset_id));
  if (missing.length > 0) {
    errors.push(`${missing.length} clip${missing.length === 1 ? ' references' : 's reference'} a missing asset`);
  }

  const hiddenWithClips = timeline.tracks.filter((track) => !track.visible && track.clips.length > 0);
  if (hiddenWithClips.length > 0) {
    warnings.push(`${hiddenWithClips.length} hidden layer${hiddenWithClips.length === 1 ? '' : 's'} will not appear in the export`);
  }
  const lockedWithClips = timeline.tracks.filter((track) => track.locked && track.clips.length > 0);
  if (lockedWithClips.length > 0) {
    warnings.push(`${lockedWithClips.length} locked layer${lockedWithClips.length === 1 ? ' renders' : 's render'} normally (lock only blocks editing)`);
  }

  // Report only authored limitations, with stable document paths. Runtime
  // capability data is authoritative; registries describe feature-specific
  // preview/export behavior within the broader capability.
  const featureSupport = new Map((capabilities?.features || []).map((feature) => [feature.feature, feature]));
  timeline.tracks.forEach((track, trackIndex) => {
    track.clips.forEach((clip, clipIndex) => {
      const path = `tracks[${trackIndex}].clips[${clipIndex}]`;
      (clip.effects || []).forEach((effect, effectIndex) => {
        if (!effect.enabled) return;
        const definition = EFFECT_DEFINITIONS.find((item) => item.type === effect.type);
        const support = featureSupport.get(definition?.exportFeature || 'effects');
        if (!definition?.exportSupported || support?.supported === false) {
          fidelityIssues.push(`${path}.effects[${effectIndex}]: ${effect.type} is not supported by the active exporter`);
        } else if (support?.partial) {
          fidelityIssues.push(`${path}.effects[${effectIndex}]: ${effect.type} is partial${support.notes ? ` — ${support.notes}` : ''}`);
        } else if (definition.previewFilter(effect.params || {}) === null) {
          fidelityIssues.push(`${path}.effects[${effectIndex}]: ${effect.type} is not represented exactly in the editor preview`);
        }
      });
      (clip.transitions || []).forEach((transition, transitionIndex) => {
        const definition = TRANSITION_DEFINITIONS.find((item) => item.type === transition.type);
        const support = featureSupport.get('transitions');
        if (!definition?.exportSupported || support?.supported === false) {
          fidelityIssues.push(`${path}.transitions[${transitionIndex}]: ${transition.type} is not supported by the active exporter`);
        } else if (support?.partial || definition.exportNote) {
          fidelityIssues.push(`${path}.transitions[${transitionIndex}]: ${transition.type} is approximate${definition.exportNote ? ` — ${definition.exportNote}` : support?.notes ? ` — ${support.notes}` : ''}`);
        }
      });
      if (clip.shape) {
        const definition = ANNOTATION_DEFINITIONS.find((item) => item.kind === clip.shape?.kind);
        const support = featureSupport.get('annotations');
        if (!definition || definition.exportSupport !== 'full' || support?.partial || support?.supported === false) {
          fidelityIssues.push(`${path}.shape: ${clip.shape.kind} is ${definition?.exportSupport === 'preview' ? 'not exported' : 'approximate'}${definition?.exportNote ? ` — ${definition.exportNote}` : ''}`);
        }
      }
      if (clip.cursor?.events?.length) {
        const support = featureSupport.get('cursor_effects');
        fidelityIssues.push(`${path}.cursor: cursor overlays are ${support?.supported === false ? 'unsupported' : 'sampled/partial'}${support?.notes ? ` — ${support.notes}` : ''}`);
      }
      if (clip.text) {
        const support = featureSupport.get('text_overlays');
        if (support?.partial || support?.supported === false) {
          fidelityIssues.push(`${path}.text: text layout is ${support.supported === false ? 'unsupported' : 'partial'}${support.notes ? ` — ${support.notes}` : ''}`);
        }
      }
      if ((clip.keyframes || []).length > 0) {
        const support = featureSupport.get('keyframes');
        fidelityIssues.push(`${path}.keyframes: animation uses ${support?.supported === false ? 'unsupported' : 'sampled/partial'} export evaluation${support?.notes ? ` — ${support.notes}` : ''}`);
      }
      for (const key of ['rotation_x', 'rotation_y', 'anchor_x', 'anchor_y', 'perspective', 'crop'] as const) {
        const value = clip.transform?.[key];
        const authored = typeof value === 'object' ? Object.values(value || {}).some((entry) => Number(entry) !== 0) : Number(value || 0) !== 0;
        if (authored) fidelityIssues.push(`${path}.transform.${key}: CSS and FFmpeg transform semantics differ`);
      }
    });
  });
  (timeline.scenes || []).forEach((scene, sceneIndex) => {
    if (scene.camera) fidelityIssues.push(`scenes[${sceneIndex}].camera: camera projection is sampled/partial`);
    (scene.effects || []).forEach((effect, effectIndex) => {
      if (effect.enabled) fidelityIssues.push(`scenes[${sceneIndex}].effects[${effectIndex}]: ${effect.type} does not share one preview/export implementation`);
    });
  });

  // Captions falling outside the timeline are silently cut off.
  const strayCaptions = timeline.tracks
    .filter((track) => track.type === 'caption')
    .flatMap((track) => track.clips)
    .filter((clip) => clip.start_ms + clip.duration_ms > timeline.duration_ms);
  if (strayCaptions.length > 0) {
    warnings.push(`${strayCaptions.length} caption${strayCaptions.length === 1 ? ' extends' : 's extend'} past the end of the timeline`);
  }

  // Audio sanity.
  if (!settings.include_audio) {
    const audible = allClips.some(({ track, clip }) => {
      if (track.muted || clip.muted) return false;
      const asset = clip.asset_id ? assets.find((item) => item.id === clip.asset_id) : undefined;
      return Boolean(asset && (asset.kind === 'audio' || asset.kind === 'music' || asset.mime_type.startsWith('video/')));
    });
    if (audible) {
      warnings.push('Audio is disabled but the timeline contains audible clips');
    }
  }

  if (!capabilities) {
    const message = 'Renderer capability data is unavailable — parity cannot be verified';
    if (settings.strict_parity) errors.push(message);
    else warnings.push(message);
  }

  if (settings.strict_parity) errors.push(...fidelityIssues);
  else warnings.push(...fidelityIssues);

  return { errors, warnings };
}
