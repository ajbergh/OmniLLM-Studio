import { describe, expect, it } from 'vitest';
import { EDITOR_MODES, editorModeFeatures } from './editorModes';

describe('video editor modes', () => {
  it('defines explicit feature gates for all five modes', () => {
    expect(EDITOR_MODES.map((mode) => mode.key)).toEqual(['full', 'simple_trim', 'captions_only', 'social_clip', 'motion_design']);
    for (const mode of EDITOR_MODES) expect(Object.keys(mode.features)).toHaveLength(13);
    expect(editorModeFeatures('motion_design')).toMatchObject({ designAnimateTabs: true, animationBlocks: true, spatialControls: true, cameraControls: true });
  });
});
