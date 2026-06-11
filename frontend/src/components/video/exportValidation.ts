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

  // Preview-only/partial features actually used by this timeline.
  const previewOnlyEffects = new Set(EFFECT_DEFINITIONS.filter((definition) => !definition.exportSupported).map((definition) => definition.type as string));
  const usedPreviewEffects = new Set<string>();
  const previewOnlyTransitions = new Set(TRANSITION_DEFINITIONS.filter((definition) => !definition.exportSupported).map((definition) => definition.type as string));
  const usedPreviewTransitions = new Set<string>();
  const previewOnlyAnnotations = new Set(ANNOTATION_DEFINITIONS.filter((definition) => definition.exportSupport === 'preview').map((definition) => definition.kind as string));
  const usedPreviewAnnotations = new Set<string>();
  let cursorClips = 0;
  for (const { clip } of allClips) {
    for (const effect of clip.effects || []) {
      if (effect.enabled && previewOnlyEffects.has(effect.type)) usedPreviewEffects.add(effect.type);
    }
    for (const transition of clip.transitions || []) {
      if (previewOnlyTransitions.has(transition.type)) usedPreviewTransitions.add(transition.type);
    }
    if (clip.shape && previewOnlyAnnotations.has(clip.shape.kind)) usedPreviewAnnotations.add(clip.shape.kind);
    if (clip.cursor?.events?.length) cursorClips += 1;
  }
  if (usedPreviewEffects.size > 0) {
    warnings.push(`Preview-only effects will not export: ${Array.from(usedPreviewEffects).join(', ')}`);
  }
  if (usedPreviewTransitions.size > 0) {
    warnings.push(`Preview-only transitions will not export: ${Array.from(usedPreviewTransitions).join(', ')}`);
  }
  if (usedPreviewAnnotations.size > 0) {
    warnings.push(`Preview-only annotations will not export: ${Array.from(usedPreviewAnnotations).join(', ')}`);
  }
  if (cursorClips > 0) {
    warnings.push('Cursor effects are preview-only and will not draw in the export');
  }

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

  // Track-solo never exports — surface it when active capability data agrees.
  const soloFeature = capabilities?.features.find((feature) => feature.feature === 'track_solo');
  if (soloFeature && !soloFeature.supported) {
    // Informational only when the user actually relies on solo is unknowable
    // here (solo is ephemeral), so leave it to the capability footer.
  }

  return { errors, warnings };
}
