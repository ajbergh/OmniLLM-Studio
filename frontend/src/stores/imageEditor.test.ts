import { beforeEach, describe, expect, it } from 'vitest';
import { useImageEditorStore, type MaskStroke } from './imageEditor';
import type { ImageNodeWithMask } from '../types';

const strokeA: MaskStroke = {
  points: [{ x: 10, y: 10 }, { x: 20, y: 20 }],
  brushSize: 30,
  tool: 'brush',
  feather: 0,
};

const strokeB: MaskStroke = {
  points: [{ x: 100, y: 100 }, { x: 120, y: 120 }],
  brushSize: 20,
  tool: 'eraser',
  feather: 0,
};

function node(id: string, stroke?: MaskStroke): ImageNodeWithMask {
  return {
    id,
    session_id: 'session-1',
    operation_type: 'edit',
    instruction: id,
    provider: 'openai',
    model: 'gpt-image-1',
    created_at: '2026-08-12T00:00:00Z',
    ...(stroke
      ? {
          mask: {
            id: `mask-${id}`,
            node_id: id,
            attachment_id: `attachment-${id}`,
            stroke_json: JSON.stringify([stroke]),
            created_at: '2026-08-12T00:00:00Z',
          },
        }
      : {}),
  };
}

describe('imageEditor mask lifecycle', () => {
  beforeEach(() => {
    useImageEditorStore.getState().reset();
  });

  it('restores the target node mask and clears stroke history on undo navigation', () => {
    useImageEditorStore.setState({
      nodes: [node('a', strokeA), node('b', strokeB)],
      activeNodeId: 'b',
      activeNodeAssets: [{
        id: 'asset-b',
        node_id: 'b',
        attachment_id: 'attachment-b',
        variant_index: 0,
        is_selected: true,
        created_at: '2026-08-12T00:00:00Z',
      }],
      nodeUndoStack: ['a'],
      maskStrokes: [strokeB],
      maskUndoStack: [[strokeA]],
      maskRedoStack: [[strokeB]],
    });

    useImageEditorStore.getState().undoNodeNavigation();

    const state = useImageEditorStore.getState();
    expect(state.activeNodeId).toBe('a');
    expect(state.activeNodeAssets).toEqual([]);
    expect(state.maskStrokes).toEqual([strokeA]);
    expect(state.maskUndoStack).toEqual([]);
    expect(state.maskRedoStack).toEqual([]);
  });

  it('does not carry an old mask onto a node that has no persisted mask', () => {
    useImageEditorStore.setState({
      nodes: [node('a', strokeA), node('b')],
      activeNodeId: 'a',
      maskStrokes: [strokeA],
    });

    useImageEditorStore.getState().branchFromNode('b');

    const state = useImageEditorStore.getState();
    expect(state.activeNodeId).toBe('b');
    expect(state.maskStrokes).toEqual([]);
    expect(state.maskUndoStack).toEqual([]);
    expect(state.maskRedoStack).toEqual([]);
  });

  it('restores a persisted mask when directly selecting a node', () => {
    useImageEditorStore.setState({
      nodes: [node('a'), node('b', strokeB)],
      activeNodeId: 'a',
      maskStrokes: [strokeA],
    });

    useImageEditorStore.getState().setActiveNode('b');

    expect(useImageEditorStore.getState().maskStrokes).toEqual([strokeB]);
  });
});
