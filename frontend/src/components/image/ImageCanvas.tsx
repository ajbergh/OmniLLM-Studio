import { useRef, useEffect, useState, useCallback, forwardRef, useImperativeHandle } from 'react';
import { useImageEditorStore, type MaskStroke } from '../../stores/imageEditor';
import { CanvasToolbar } from './CanvasToolbar';
import { attachmentUrl } from '../../api';
import { drawMaskStroke, type ImageMaskingMode } from './maskRaster';

interface ImageCanvasProps {
  attachmentId: string;
  zoom: number;
  onZoomChange: (zoom: number) => void;
  maskingMode?: ImageMaskingMode;
}

export interface ImageCanvasHandle {
  exportMaskBlob: (mode?: ImageMaskingMode) => Blob | null;
  fitToViewport: () => void;
}

export const ImageCanvas = forwardRef<ImageCanvasHandle, ImageCanvasProps>(function ImageCanvas({ attachmentId, zoom, onZoomChange, maskingMode = 'none' }, ref) {
  const containerRef = useRef<HTMLDivElement>(null);
  const maskCanvasRef = useRef<HTMLCanvasElement>(null);
  const imgRef = useRef<HTMLImageElement>(null);
  const [pan, setPan] = useState({ x: 0, y: 0 });
  const [isPanning, setIsPanning] = useState(false);
  const [panStart, setPanStart] = useState({ x: 0, y: 0 });
  const [imageLoaded, setImageLoaded] = useState(false);
  const [spaceHeld, setSpaceHeld] = useState(false);
  const [isDrawing, setIsDrawing] = useState(false);
  const autoFitPendingRef = useRef(true);
  // Use a ref for current stroke points so renderMask always sees the latest data
  const currentStrokeRef = useRef<{ x: number; y: number }[]>([]);
  // Touch drawing uses a ref rather than React state so pointer moves that arrive
  // before the state update is flushed still append to the correct stroke.
  const touchDrawingPointerRef = useRef<number | null>(null);

  // Touch gesture state
  const touchStateRef = useRef<{
    pointers: Map<number, { x: number; y: number }>;
    initialDist: number | null;
    initialZoom: number;
    initialPan: { x: number; y: number };
    initialMidpoint: { x: number; y: number } | null;
  }>({
    pointers: new Map(),
    initialDist: null,
    initialZoom: 1,
    initialPan: { x: 0, y: 0 },
    initialMidpoint: null,
  });

  const {
    tool,
    brushSize,
    brushFeather,
    maskStrokes,
    maskVisible,
    maskOpacity,
    editMode,
    addMaskStroke,
  } = useImageEditorStore();

  const imageUrl = attachmentUrl(attachmentId);
  const isMaskMode = editMode === 'edit' && maskingMode !== 'none' && (tool === 'brush' || tool === 'eraser');

  // rAF mask rendering guard — prevents >60fps mask redraws
  const maskRafRef = useRef(0);

  const isInteractiveOverlayTarget = (target: EventTarget | null): boolean =>
    target instanceof Element && target.closest('[data-canvas-interactive="true"]') !== null;

  // Reset pan when image changes
  useEffect(() => {
    setPan({ x: 0, y: 0 });
    setImageLoaded(false);
    autoFitPendingRef.current = true;
  }, [attachmentId]);

  // Resize mask canvas to match image dimensions
  // Re-run when editMode changes so the canvas is sized if the image was already loaded
  useEffect(() => {
    const img = imgRef.current;
    const canvas = maskCanvasRef.current;
    if (!img || !canvas || !imageLoaded) return;
    canvas.width = img.naturalWidth;
    canvas.height = img.naturalHeight;
    renderMask();
  }, [imageLoaded, editMode]); // eslint-disable-line react-hooks/exhaustive-deps

  // Re-render mask whenever strokes or visibility changes (throttled to rAF)
  useEffect(() => {
    cancelAnimationFrame(maskRafRef.current);
    maskRafRef.current = requestAnimationFrame(() => renderMask());
  }, [maskStrokes, maskVisible, maskOpacity, maskingMode]); // eslint-disable-line react-hooks/exhaustive-deps

  function renderMask() {
    const canvas = maskCanvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    ctx.clearRect(0, 0, canvas.width, canvas.height);
    if (!maskVisible) return;

    const hasStrokes = maskStrokes.length > 0;
    const hasInProgress = currentStrokeRef.current.length > 0;
    if (!hasStrokes && !hasInProgress) return;

    // Draw all completed strokes
    for (const stroke of maskStrokes) {
      drawMaskStroke(ctx, stroke, { mode: 'preview', opacity: maskOpacity });
    }
    // Also draw current in-progress stroke
    if (hasInProgress) {
      drawMaskStroke(ctx, {
        points: currentStrokeRef.current,
        brushSize,
        tool,
        feather: brushFeather,
      } as MaskStroke, { mode: 'preview', opacity: maskOpacity });
    }
  }

  // Convert screen coordinates to image coordinates
  function screenToImage(clientX: number, clientY: number): { x: number; y: number } | null {
    const img = imgRef.current;
    const container = containerRef.current;
    if (!img || !container) return null;

    const containerRect = container.getBoundingClientRect();
    // Center of container
    const cx = containerRect.width / 2;
    const cy = containerRect.height / 2;

    // The image center in screen space
    const imgScreenX = containerRect.left + cx + pan.x;
    const imgScreenY = containerRect.top + cy + pan.y;

    // Image dimensions in screen space
    const imgW = img.naturalWidth * zoom;
    const imgH = img.naturalHeight * zoom;

    // Image top-left in screen space
    const imgLeft = imgScreenX - imgW / 2;
    const imgTop = imgScreenY - imgH / 2;

    // Convert to image pixel coordinates
    const x = (clientX - imgLeft) / zoom;
    const y = (clientY - imgTop) / zoom;

    if (x < 0 || y < 0 || x > img.naturalWidth || y > img.naturalHeight) return null;
    return { x, y };
  }

  // Mouse wheel zoom
  const handleWheel = useCallback(
    (e: WheelEvent) => {
      if (isInteractiveOverlayTarget(e.target)) return;
      e.preventDefault();
      const factor = e.deltaY < 0 ? 1.1 : 1 / 1.1;
      const newZoom = Math.max(0.1, Math.min(10, zoom * factor));
      onZoomChange(newZoom);
    },
    [zoom, onZoomChange]
  );

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    el.addEventListener('wheel', handleWheel, { passive: false });
    return () => el.removeEventListener('wheel', handleWheel);
  }, [handleWheel]);

  // Space key for temporary pan mode
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement)?.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
      if ((e.target as HTMLElement)?.isContentEditable) return;
      if (e.code === 'Space' && !e.repeat) {
        e.preventDefault();
        setSpaceHeld(true);
      }
    };
    const onKeyUp = (e: KeyboardEvent) => {
      if (e.code === 'Space') {
        setSpaceHeld(false);
        setIsPanning(false);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    window.addEventListener('keyup', onKeyUp);
    return () => {
      window.removeEventListener('keydown', onKeyDown);
      window.removeEventListener('keyup', onKeyUp);
    };
  }, []);

  const handleMouseDown = (e: React.MouseEvent) => {
    if (isInteractiveOverlayTarget(e.target)) return;

    if (e.button === 1 || spaceHeld || tool === 'pan') {
      // Middle click or space+click or pan tool → pan
      setIsPanning(true);
      setPanStart({ x: e.clientX - pan.x, y: e.clientY - pan.y });
      e.preventDefault();
      return;
    }

    // Mask drawing
    if (isMaskMode && e.button === 0) {
      const pt = screenToImage(e.clientX, e.clientY);
      if (pt) {
        setIsDrawing(true);
        currentStrokeRef.current = [pt];
        renderMask();
      }
      e.preventDefault();
    }
  };

  const handleMouseMove = (e: React.MouseEvent) => {
    if (isPanning) {
      setPan({ x: e.clientX - panStart.x, y: e.clientY - panStart.y });
      return;
    }

    if (isDrawing && isMaskMode) {
      const pt = screenToImage(e.clientX, e.clientY);
      if (pt) {
        currentStrokeRef.current = [...currentStrokeRef.current, pt];
        // Live render — directly call renderMask (ref is already updated)
        renderMask();
      }
    }
  };

  const handleMouseUp = () => {
    if (isDrawing && currentStrokeRef.current.length > 0) {
      addMaskStroke({
        points: currentStrokeRef.current,
        brushSize,
        tool: tool as 'brush' | 'eraser',
        feather: brushFeather,
      });
      currentStrokeRef.current = [];
      setIsDrawing(false);
    }
    setIsPanning(false);
  };

  // ── Touch gestures: 1 finger → draw, 2 fingers → pinch-zoom & pan ────

  const getTouchDist = (a: { x: number; y: number }, b: { x: number; y: number }) =>
    Math.hypot(a.x - b.x, a.y - b.y);

  const getTouchMid = (a: { x: number; y: number }, b: { x: number; y: number }) => ({
    x: (a.x + b.x) / 2,
    y: (a.y + b.y) / 2,
  });

  const resetPinchState = () => {
    const ts = touchStateRef.current;
    if (ts.pointers.size < 2) {
      ts.initialDist = null;
      ts.initialMidpoint = null;
    }
  };

  const handlePointerDown = (e: React.PointerEvent) => {
    // Mouse/pen continue through the existing mouse path. Restricting pointer
    // gesture state to touch avoids double-recording strokes in browsers that
    // dispatch both pointer and mouse events for a mouse/pen device.
    if (e.pointerType !== 'touch' || isInteractiveOverlayTarget(e.target)) return;
    e.preventDefault();

    const ts = touchStateRef.current;
    ts.pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);

    if (ts.pointers.size === 1 && isMaskMode) {
      const pt = screenToImage(e.clientX, e.clientY);
      if (pt) {
        touchDrawingPointerRef.current = e.pointerId;
        currentStrokeRef.current = [pt];
        setIsDrawing(true);
        renderMask();
      }
      return;
    }

    if (ts.pointers.size === 2) {
      // A second finger changes the gesture to pinch/pan. Discard the
      // in-progress one-finger stroke so an accidental line is never committed.
      touchDrawingPointerRef.current = null;
      currentStrokeRef.current = [];
      setIsDrawing(false);
      renderMask();

      const [a, b] = Array.from(ts.pointers.values());
      ts.initialDist = getTouchDist(a, b);
      ts.initialZoom = zoom;
      ts.initialPan = { ...pan };
      ts.initialMidpoint = getTouchMid(a, b);
    }
  };

  const handlePointerMove = (e: React.PointerEvent) => {
    if (e.pointerType !== 'touch') return;

    const ts = touchStateRef.current;
    if (!ts.pointers.has(e.pointerId)) return;
    e.preventDefault();
    ts.pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });

    if (ts.pointers.size === 2 && ts.initialDist != null && ts.initialMidpoint != null) {
      const [a, b] = Array.from(ts.pointers.values());
      const dist = getTouchDist(a, b);
      const scale = dist / ts.initialDist;
      const newZoom = Math.max(0.1, Math.min(10, ts.initialZoom * scale));
      onZoomChange(newZoom);

      // Pan while pinching
      const mid = getTouchMid(a, b);
      setPan({
        x: ts.initialPan.x + (mid.x - ts.initialMidpoint.x),
        y: ts.initialPan.y + (mid.y - ts.initialMidpoint.y),
      });
      return;
    }

    if (
      ts.pointers.size === 1 &&
      touchDrawingPointerRef.current === e.pointerId &&
      isMaskMode
    ) {
      const pt = screenToImage(e.clientX, e.clientY);
      if (pt) {
        currentStrokeRef.current = [...currentStrokeRef.current, pt];
        renderMask();
      }
    }
  };

  const handlePointerUp = (e: React.PointerEvent) => {
    if (e.pointerType !== 'touch') return;

    const ts = touchStateRef.current;
    const wasDrawingPointer = touchDrawingPointerRef.current === e.pointerId;
    ts.pointers.delete(e.pointerId);

    if (wasDrawingPointer) {
      if (currentStrokeRef.current.length > 0) {
        addMaskStroke({
          points: currentStrokeRef.current,
          brushSize,
          tool: tool as 'brush' | 'eraser',
          feather: brushFeather,
        });
      }
      touchDrawingPointerRef.current = null;
      currentStrokeRef.current = [];
      setIsDrawing(false);
    }

    resetPinchState();
  };

  const handlePointerCancel = (e: React.PointerEvent) => {
    if (e.pointerType !== 'touch') return;

    const ts = touchStateRef.current;
    ts.pointers.delete(e.pointerId);
    if (touchDrawingPointerRef.current === e.pointerId) {
      touchDrawingPointerRef.current = null;
      currentStrokeRef.current = [];
      setIsDrawing(false);
      renderMask();
    }
    resetPinchState();
  };

  const handleDownload = () => {
    const a = document.createElement('a');
    a.href = imageUrl;
    a.download = `image-${attachmentId.slice(0, 8)}.png`;
    a.click();
  };

  /** Export a provider-appropriate PNG selection guide. */
  const exportMaskBlob = useCallback((mode: ImageMaskingMode = maskingMode): Blob | null => {
    const canvas = maskCanvasRef.current;
    const img = imgRef.current;
    if (!canvas || !img || maskStrokes.length === 0) return null;

    // Create export canvas
    const exportCanvas = document.createElement('canvas');
    exportCanvas.width = img.naturalWidth;
    exportCanvas.height = img.naturalHeight;
    const ctx = exportCanvas.getContext('2d');
    if (!ctx) return null;

    // Start with fully opaque black (unmasked = keep)
    ctx.fillStyle = '#000000';
    ctx.fillRect(0, 0, exportCanvas.width, exportCanvas.height);

    // Pixel masks use transparent edit regions; semantic masks use visible
    // white-on-black guidance while sharing the same feather rasterization.
    for (const stroke of maskStrokes) {
      drawMaskStroke(ctx, stroke, { mode: mode === 'semantic' ? 'semantic' : 'pixel' });
    }

    // Convert to blob synchronously by converting to data URL
    const dataUrl = exportCanvas.toDataURL('image/png');
    const binary = atob(dataUrl.split(',')[1]);
    const array = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
      array[i] = binary.charCodeAt(i);
    }
    return new Blob([array], { type: 'image/png' });
  }, [maskStrokes, maskingMode]);

  const fitToViewport = useCallback(() => {
    const img = imgRef.current;
    const container = containerRef.current;
    if (!img || !container || !img.naturalWidth || !img.naturalHeight) {
      onZoomChange(1);
      return;
    }
    const cW = container.clientWidth;
    const cH = container.clientHeight;
    if (cW <= 0 || cH <= 0) return;
    const fitZoom = Math.min(cW / img.naturalWidth, cH / img.naturalHeight, 1);
    onZoomChange(fitZoom);
    setPan({ x: 0, y: 0 });
    autoFitPendingRef.current = false;
  }, [onZoomChange]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container || !imageLoaded) return;

    const fitWhenVisible = () => {
      if (autoFitPendingRef.current && container.clientWidth > 0 && container.clientHeight > 0) {
        fitToViewport();
      }
    };
    fitWhenVisible();
    const observer = new ResizeObserver(fitWhenVisible);
    observer.observe(container);
    return () => observer.disconnect();
  }, [imageLoaded, fitToViewport]);

  // Expose exportMaskBlob and fitToViewport to parent via ref
  useImperativeHandle(ref, () => ({
    exportMaskBlob,
    fitToViewport,
  }), [exportMaskBlob, fitToViewport]);

  const getCursor = () => {
    if (isPanning || spaceHeld || tool === 'pan') return 'grabbing';
    if (isMaskMode) return 'crosshair';
    return 'default';
  };

  return (
    <div
      ref={containerRef}
      className="w-full h-full relative overflow-hidden select-none"
      style={{ cursor: getCursor(), touchAction: 'none' }}
      onMouseDown={handleMouseDown}
      onMouseMove={handleMouseMove}
      onMouseUp={handleMouseUp}
      onMouseLeave={handleMouseUp}
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerUp}
      onPointerCancel={handlePointerCancel}
    >
      <CanvasToolbar zoom={zoom} onZoomChange={onZoomChange} onDownload={handleDownload} onFitToViewport={fitToViewport} />
      <div
        className="absolute inset-0 flex items-center justify-center"
        style={{
          transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`,
          transformOrigin: 'center',
          willChange: 'transform',
        }}
      >
        {/* Checkerboard background for transparency */}
        <div
          className="relative"
          style={{
            backgroundImage: `linear-gradient(45deg, #1a1a30 25%, transparent 25%),
                              linear-gradient(-45deg, #1a1a30 25%, transparent 25%),
                              linear-gradient(45deg, transparent 75%, #1a1a30 75%),
                              linear-gradient(-45deg, transparent 75%, #1a1a30 75%)`,
            backgroundSize: '20px 20px',
            backgroundPosition: '0 0, 0 10px, 10px -10px, -10px 0px',
          }}
        >
          <img
            ref={imgRef}
            src={imageUrl}
            alt="Generated image"
            className="max-w-none block"
            style={{ imageRendering: zoom > 2 ? 'pixelated' : 'auto' }}
            onLoad={() => {
              setImageLoaded(true);
              autoFitPendingRef.current = true;
            }}
            draggable={false}
          />
          {/* Mask overlay canvas */}
          {editMode === 'edit' && (
            <canvas
              data-testid="image-mask-canvas"
              ref={maskCanvasRef}
              className="absolute inset-0 w-full h-full pointer-events-none"
              style={{ opacity: maskVisible ? (maskingMode === 'none' ? 0.35 : 1) : 0 }}
            />
          )}
          {!imageLoaded && (
            <div className="absolute inset-0 flex items-center justify-center bg-surface/80">
              <div className="w-8 h-8 border-2 border-primary/30 border-t-primary rounded-full animate-spin" />
            </div>
          )}
        </div>
      </div>
    </div>
  );
});
