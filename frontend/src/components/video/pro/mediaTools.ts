import { toast } from 'sonner';
import { useVideoStudioStore } from '../../../stores/videoStudio';
import type { VideoAsset, VideoTimelineDocument } from '../../../types/video';
import { commitTimelineCommand, referencedAssetIds } from './timelineCommandEngine';

export interface ProjectMediaReference {
  asset_id: string;
  clip_count: number;
  missing: boolean;
  asset?: VideoAsset;
}

export function projectMediaReferences(document: VideoTimelineDocument | null, assets: VideoAsset[]): ProjectMediaReference[] {
  if (!document) return [];
  const counts = new Map<string, number>();
  for (const track of document.tracks) {
    for (const clip of track.clips) {
      if (clip.asset_id) counts.set(clip.asset_id, (counts.get(clip.asset_id) || 0) + 1);
    }
  }
  const assetsById = new Map(assets.map((asset) => [asset.id, asset]));
  return Array.from(counts, ([asset_id, clip_count]) => ({
    asset_id,
    clip_count,
    missing: !assetsById.has(asset_id),
    asset: assetsById.get(asset_id),
  })).sort((a, b) => Number(b.missing) - Number(a.missing) || (a.asset?.file_name || a.asset_id).localeCompare(b.asset?.file_name || b.asset_id));
}

export async function replaceTimelineAsset(oldAssetId: string, newAssetId: string): Promise<boolean> {
  if (!oldAssetId || !newAssetId || oldAssetId === newAssetId) {
    toast.error('Choose different source and replacement assets');
    return false;
  }
  return commitTimelineCommand('Replace timeline media', (document, state) => {
    const replacement = state.assets.find((asset) => asset.id === newAssetId);
    if (!replacement) return { changed: false, message: 'Replacement asset is not available in this project' };
    let changed = 0;
    let firstClipId: string | null = null;
    let firstTrackId: string | null = null;
    for (const track of document.tracks) {
      if (track.locked) continue;
      for (const clip of track.clips) {
        if (clip.asset_id !== oldAssetId) continue;
        clip.asset_id = newAssetId;
        firstClipId ||= clip.id;
        firstTrackId ||= track.id;
        changed += 1;
      }
    }
    return {
      changed: changed > 0,
      message: changed > 0 ? `Replaced media in ${changed} clip${changed === 1 ? '' : 's'}` : 'No unlocked clips reference that media',
      selectedClipId: firstClipId,
      selectedClipIds: firstClipId ? [firstClipId] : [],
      selectedTrackId: firstTrackId,
    };
  });
}

export function createProjectManifest(): Blob | null {
  const state = useVideoStudioStore.getState();
  const project = state.projects.find((item) => item.id === state.activeProjectId);
  if (!project || !state.timeline) {
    toast.error('Create or select a video project first');
    return null;
  }
  const used = referencedAssetIds(state.timeline);
  const manifest = {
    schema: 'omnillm-video-project-manifest-v1',
    exported_at: new Date().toISOString(),
    project,
    timeline: state.timeline,
    assets: state.assets.map((asset) => ({
      ...asset,
      used_in_timeline: used.has(asset.id),
    })),
    renderer_capabilities: state.rendererCapabilities,
    note: 'This manifest contains project/timeline metadata, not media file bytes. Use the original project storage or File Library to retain media.',
  };
  return new Blob([JSON.stringify(manifest, null, 2)], { type: 'application/json' });
}

export function downloadProjectManifest(): void {
  const blob = createProjectManifest();
  if (!blob) return;
  const state = useVideoStudioStore.getState();
  const project = state.projects.find((item) => item.id === state.activeProjectId);
  const safeName = (project?.title || 'video-project').replace(/[^a-z0-9_-]+/gi, '-').replace(/^-+|-+$/g, '').toLowerCase();
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = `${safeName || 'video-project'}-manifest.json`;
  anchor.click();
  URL.revokeObjectURL(url);
  toast.success('Project manifest downloaded');
}
