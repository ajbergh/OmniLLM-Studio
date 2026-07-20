import { commitTimelineCommand } from './timelineCommandEngine';
import type { VideoAsset, VideoTimelineClip } from '../../../types/video';

function isTimeBasedAudio(clip: VideoTimelineClip, assets: Map<string, VideoAsset>): boolean {
  if (!clip.asset_id) return false;
  const asset = assets.get(clip.asset_id);
  return Boolean(asset && (asset.mime_type.startsWith('audio/') || asset.mime_type.startsWith('video/')));
}

export async function normalizeProjectAudio(target = 1): Promise<boolean> {
  return commitTimelineCommand('Normalize project audio', (document, state) => {
    const assets = new Map(state.assets.map((asset) => [asset.id, asset]));
    let changed = 0;
    for (const track of document.tracks) {
      if (track.locked) continue;
      for (const clip of track.clips) {
        if (!isTimeBasedAudio(clip, assets) || clip.muted) continue;
        if ((clip.volume ?? 1) !== target) {
          clip.volume = target;
          changed += 1;
        }
      }
    }
    return {
      changed: changed > 0,
      message: changed > 0 ? `Normalized ${changed} audio clip${changed === 1 ? '' : 's'} to ${Math.round(target * 100)}%` : 'Project audio is already normalized',
    };
  });
}

export async function applyProjectAudioFades(fadeMs = 250): Promise<boolean> {
  return commitTimelineCommand('Apply project audio fades', (document, state) => {
    const assets = new Map(state.assets.map((asset) => [asset.id, asset]));
    let changed = 0;
    for (const track of document.tracks) {
      if (track.locked) continue;
      for (const clip of track.clips) {
        if (!isTimeBasedAudio(clip, assets) || clip.muted) continue;
        const safeFade = Math.min(Math.max(0, Math.round(fadeMs)), Math.floor(clip.duration_ms / 2));
        if (clip.fade_in_ms !== safeFade || clip.fade_out_ms !== safeFade) {
          clip.fade_in_ms = safeFade;
          clip.fade_out_ms = safeFade;
          changed += 1;
        }
      }
    }
    return {
      changed: changed > 0,
      message: changed > 0 ? `Applied ${fadeMs}ms fades to ${changed} audio clip${changed === 1 ? '' : 's'}` : 'Audio fades already match',
    };
  });
}

export async function limitProjectGain(maximum = 1): Promise<boolean> {
  return commitTimelineCommand('Limit project clip gain', (document, state) => {
    const assets = new Map(state.assets.map((asset) => [asset.id, asset]));
    let changed = 0;
    for (const track of document.tracks) {
      if (track.locked) continue;
      for (const clip of track.clips) {
        if (!isTimeBasedAudio(clip, assets)) continue;
        if ((clip.volume ?? 1) > maximum) {
          clip.volume = maximum;
          changed += 1;
        }
        for (const keyframe of clip.keyframes || []) {
          if (keyframe.property === 'volume' && keyframe.value > maximum) {
            keyframe.value = maximum;
            changed += 1;
          }
        }
      }
    }
    return {
      changed: changed > 0,
      message: changed > 0 ? `Limited ${changed} gain value${changed === 1 ? '' : 's'} to ${Math.round(maximum * 100)}%` : 'No gain values exceed the selected ceiling',
    };
  });
}
