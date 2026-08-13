import { beforeEach, describe, expect, it } from 'vitest';
import { useImageEditorStore } from './imageEditor';

describe('image editor reference state', () => {
  beforeEach(() => {
    useImageEditorStore.getState().reset();
  });

  it('does not impose a store-level reference count limit', () => {
    const store = useImageEditorStore.getState();
    store.addContentReference('content-1');
    store.addContentReference('content-2');
    store.addContentReference('content-3');
    store.addStyleReference('style-1');

    const state = useImageEditorStore.getState();
    expect(state.contentReferenceIds).toEqual(['content-1', 'content-2', 'content-3']);
    expect(state.styleReferenceIds).toEqual(['style-1']);
  });

  it('deduplicates attachment ids without discarding other references', () => {
    const store = useImageEditorStore.getState();
    store.addContentReference('content-1');
    store.addContentReference('content-1');
    store.addStyleReference('style-1');
    store.addStyleReference('style-1');

    const state = useImageEditorStore.getState();
    expect(state.contentReferenceIds).toEqual(['content-1']);
    expect(state.styleReferenceIds).toEqual(['style-1']);
  });
});
