import { Paintbrush, Eraser, Move, ZoomIn, ZoomOut, Maximize, Eye, EyeOff, Undo2, Redo2, Download } from 'lucide-react';
import { clsx } from 'clsx';
import { useImageEditorStore } from '../../stores/imageEditor';

interface CanvasToolbarProps {
  zoom: number;
  onZoomChange: (zoom: number) => void;
  onDownload: () => void;
  onFitToViewport?: () => void;
}

export function CanvasToolbar({ zoom, onZoomChange, onDownload, onFitToViewport }: CanvasToolbarProps) {
  const {
    tool,
    brushSize,
    maskVisible,
    editMode,
    setTool,
    setBrushSize,
    toggleMask,
    undoMaskStroke,
    redoMaskStroke,
  } = useImageEditorStore();

  const showMaskTools = editMode === 'edit';

  return (
    <div
      data-testid="image-canvas-toolbar"
      data-canvas-interactive="true"
      className="absolute top-3 left-1/2 -translate-x-1/2 flex items-center gap-0.5
                 bg-surface-glass backdrop-blur-sm rounded-xl border border-border p-1 shadow-lg z-10"
    >
      {/* Drawing tools (edit mode only) */}
      {showMaskTools && (
        <>
          <button
            data-testid="canvas-tool-brush"
            onClick={() => setTool('brush')}
            className={clsx(
              'p-1.5 rounded-lg transition-colors min-w-[32px] min-h-[32px] flex items-center justify-center',
              tool === 'brush' ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text hover:bg-surface-hover'
            )}
            title="Brush (B)"
          >
            <Paintbrush size={14} />
          </button>
          <button
            data-testid="canvas-tool-eraser"
            onClick={() => setTool('eraser')}
            className={clsx(
              'p-1.5 rounded-lg transition-colors min-w-[32px] min-h-[32px] flex items-center justify-center',
              tool === 'eraser' ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text hover:bg-surface-hover'
            )}
            title="Eraser (E)"
          >
            <Eraser size={14} />
          </button>
          <button
            data-testid="canvas-tool-pan"
            onClick={() => setTool('pan')}
            className={clsx(
              'p-1.5 rounded-lg transition-colors min-w-[32px] min-h-[32px] flex items-center justify-center',
              tool === 'pan' ? 'bg-primary/20 text-primary' : 'text-text-muted hover:text-text hover:bg-surface-hover'
            )}
            title="Pan (Space)"
          >
            <Move size={14} />
          </button>

          <div className="w-px h-5 bg-border mx-0.5" />

          {/* Brush size inline */}
          <div className="flex items-center gap-1 px-1">
            <input
              data-testid="canvas-brush-size"
              type="range"
              min={1}
              max={100}
              value={brushSize}
              onChange={(e) => setBrushSize(Number(e.target.value))}
              className="w-16 accent-primary"
              title={`Brush size: ${brushSize}`}
            />
            <span data-testid="canvas-brush-size-value" className="text-[9px] text-text-muted font-mono w-5 text-center">{brushSize}</span>
          </div>

          <div className="w-px h-5 bg-border mx-0.5" />

          <button data-testid="canvas-undo-stroke" onClick={undoMaskStroke} className="p-1.5 rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors min-w-[32px] min-h-[32px] flex items-center justify-center" title="Undo stroke (Ctrl+Z)">
            <Undo2 size={13} />
          </button>
          <button data-testid="canvas-redo-stroke" onClick={redoMaskStroke} className="p-1.5 rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors min-w-[32px] min-h-[32px] flex items-center justify-center" title="Redo stroke (Ctrl+Shift+Z)">
            <Redo2 size={13} />
          </button>

          <button
            data-testid="canvas-toggle-mask"
            onClick={toggleMask}
            className="p-1.5 rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors min-w-[32px] min-h-[32px] flex items-center justify-center"
            title="Toggle mask (M)"
          >
            {maskVisible ? <Eye size={13} /> : <EyeOff size={13} />}
          </button>

          <div className="w-px h-5 bg-border mx-0.5" />
        </>
      )}

      {/* Zoom controls */}
      <button
        data-testid="canvas-zoom-in"
        onClick={() => onZoomChange(Math.min(zoom * 1.25, 10))}
        className="p-1.5 rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors min-w-[32px] min-h-[32px] flex items-center justify-center"
        title="Zoom in (+)"
      >
        <ZoomIn size={14} />
      </button>
      <span data-testid="canvas-zoom-value" className="text-[9px] text-text-muted font-mono min-w-[2.5rem] text-center">
        {Math.round(zoom * 100)}%
      </span>
      <button
        data-testid="canvas-zoom-out"
        onClick={() => onZoomChange(Math.max(zoom / 1.25, 0.1))}
        className="p-1.5 rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors min-w-[32px] min-h-[32px] flex items-center justify-center"
        title="Zoom out (-)"
      >
        <ZoomOut size={14} />
      </button>
      <button
        data-testid="canvas-fit"
        onClick={() => onFitToViewport ? onFitToViewport() : onZoomChange(1)}
        className="p-1.5 rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors min-w-[32px] min-h-[32px] flex items-center justify-center"
        title="Fit (F)"
      >
        <Maximize size={14} />
      </button>

      <div className="w-px h-5 bg-border mx-0.5" />

      <button
        data-testid="canvas-download"
        onClick={onDownload}
        className="p-1.5 rounded-lg text-text-muted hover:text-text hover:bg-surface-hover transition-colors min-w-[32px] min-h-[32px] flex items-center justify-center"
        title="Download (Ctrl+S)"
      >
        <Download size={14} />
      </button>
    </div>
  );
}
