import { useEffect, useCallback, useRef } from 'react';
import { useImageEditorStore } from '../../stores/imageEditor';

interface UseImageEditorShortcutsOptions {
  enabled: boolean;
  onDownload: () => void;
  onZoomChange: (zoom: number) => void;
  onFitToViewport?: () => void;
}

/**
 * Keyboard shortcuts for the image editor.
 * Only active when `enabled` is true, and ignores events
 * originating from text inputs / textareas.
 */
export function useImageEditorShortcuts({
  enabled,
  onDownload,
  onZoomChange,
  onFitToViewport,
}: UseImageEditorShortcutsOptions) {
  const store = useImageEditorStore;
  const downloadRef = useRef(onDownload);
  const zoomRef = useRef(onZoomChange);
  const fitRef = useRef(onFitToViewport);
  downloadRef.current = onDownload;
  zoomRef.current = onZoomChange;
  fitRef.current = onFitToViewport;

  const handler = useCallback(
    (e: KeyboardEvent) => {
      // Ignore when typing in inputs
      const tag = (e.target as HTMLElement)?.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
      if ((e.target as HTMLElement)?.isContentEditable) return;

      const state = store.getState();
      const { editMode } = state;
      const isEdit = editMode === 'edit';

      // Ctrl/Cmd modifiers
      const ctrlKey = e.ctrlKey || e.metaKey;

      // Ctrl+Z / Ctrl+Shift+Z — mask undo/redo
      if (ctrlKey && e.key === 'z' && !e.shiftKey && isEdit) {
        e.preventDefault();
        state.undoMaskStroke();
        return;
      }
      if (ctrlKey && e.key === 'z' && e.shiftKey && isEdit) {
        e.preventDefault();
        state.redoMaskStroke();
        return;
      }
      if (ctrlKey && e.key === 'y' && isEdit) {
        e.preventDefault();
        state.redoMaskStroke();
        return;
      }

      // Ctrl+S — download
      if (ctrlKey && e.key === 's') {
        e.preventDefault();
        downloadRef.current();
        return;
      }

      // No-modifier shortcuts below
      if (ctrlKey || e.altKey) return;

      switch (e.key.toLowerCase()) {
        case 'b':
          if (isEdit) state.setTool('brush');
          break;
        case 'e':
          if (isEdit) state.setTool('eraser');
          break;
        case 'v':
          state.setTool('pan');
          break;
        case 'm':
          if (isEdit) state.toggleMask();
          break;
        case '[':
          if (isEdit) state.setBrushSize(Math.max(1, state.brushSize - 5));
          break;
        case ']':
          if (isEdit) state.setBrushSize(Math.min(100, state.brushSize + 5));
          break;
        case '=':
        case '+':
          e.preventDefault();
          zoomRef.current(Math.min(state.zoom * 1.25, 10));
          break;
        case '-':
          e.preventDefault();
          zoomRef.current(Math.max(state.zoom / 1.25, 0.1));
          break;
        case 'f':
          if (fitRef.current) {
            fitRef.current();
          } else {
            zoomRef.current(1);
          }
          break;
        case '1':
          zoomRef.current(1);
          break;
        case '2':
          zoomRef.current(2);
          break;
        default:
          break;
      }
    },
    [store]
  );

  useEffect(() => {
    if (!enabled) return;
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [enabled, handler]);
}
