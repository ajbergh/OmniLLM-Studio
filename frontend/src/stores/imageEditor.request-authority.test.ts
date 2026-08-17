import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ImageEditEditRequest, ImageEditGenerateRequest } from '../types';

const apiMocks = vi.hoisted(() => ({
  generate: vi.fn(),
  edit: vi.fn(),
  listAll: vi.fn(),
}));

vi.mock('../api', () => ({
  imageSessionApi: apiMocks,
}));

import { useImageEditorStore } from './imageEditor';

describe('image editor request authority', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    apiMocks.listAll.mockResolvedValue([]);
    useImageEditorStore.getState().reset();
    useImageEditorStore.setState({
      activeSessionId: 'session-1',
      contentReferenceIds: ['stale-content-reference'],
      styleReferenceIds: ['stale-style-reference'],
    });
  });

  it('does not reattach stored references when a generate request omits them', async () => {
    apiMocks.generate.mockResolvedValue({
      node: { id: 'generated-node' },
      assets: [],
    });
    const request: ImageEditGenerateRequest = {
      prompt: 'Generate without references',
      override: { provider: 'openai', model: 'gpt-image-1' },
    };

    await useImageEditorStore.getState().generate('conversation-1', request);

    expect(apiMocks.generate).toHaveBeenCalledWith('conversation-1', 'session-1', request);
    const forwardedRequest = apiMocks.generate.mock.calls[0][2];
    expect(forwardedRequest).not.toHaveProperty('reference_image_ids');
    expect(forwardedRequest).not.toHaveProperty('style_reference_ids');
  });

  it('does not reattach stored references when an edit request omits them', async () => {
    apiMocks.edit.mockResolvedValue({
      node: { id: 'edited-node' },
      assets: [],
    });
    const request: ImageEditEditRequest = {
      instruction: 'Edit without references',
      base_image_attachment_id: 'base-image',
      override: { provider: 'openai', model: 'gpt-image-1' },
    };

    await useImageEditorStore.getState().edit('conversation-1', request);

    expect(apiMocks.edit).toHaveBeenCalledWith('conversation-1', 'session-1', request);
    const forwardedRequest = apiMocks.edit.mock.calls[0][2];
    expect(forwardedRequest).not.toHaveProperty('reference_image_ids');
    expect(forwardedRequest).not.toHaveProperty('style_reference_ids');
  });
});
