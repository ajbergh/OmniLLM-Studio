import { useCallback, useEffect, useRef, useState } from 'react';
import { videoApi } from '../../api';
import type { CanonicalFrameLayerState } from '../../video/renderContractFrameState';
import { composeWeightedTransitionPairRgba } from '../../video/renderContractTransitionPairPixelKernel';
import type { PreviewTransitionPairSlot } from './previewFrameTransitionPairs';
import {
  paintPreviewWeightedPairCanvasLayer,
  resolvePreviewWeightedPairCanvasLayerPlan,
  type PreviewWeightedPairCanvasLayer,
} from './previewFrameWeightedPairCanvas';

export interface PreviewWeightedPairCanvasRuntimeLayer extends PreviewWeightedPairCanvasLayer {
  asset?: {
    id: string;
    file_name: string;
    mime_type: string;
  };
  canonicalState?: CanonicalFrameLayerState;
}

interface PreviewWeightedPairCanvasProps<T extends PreviewWeightedPairCanvasRuntimeLayer> {
  slot: PreviewTransitionPairSlot<T>;
  canvasWidth: number;
  canvasHeight: number;
  stageWidth: number;
  stageHeight: number;
  registerVideo: (clipId: string, node: HTMLVideoElement | null) => void;
}

type RasterSource = HTMLImageElement | HTMLVideoElement;

/**
 * Deterministic weighted pair surface. The hidden media elements are the raw
 * decoded sources and remain registered with VideoPreviewCanvas' existing
 * canonical source-time synchronizer. This component owns only readiness,
 * isolated 2D rasterization, and the exact #277 weighted pixel kernel.
 */
export function PreviewWeightedPairCanvas<T extends PreviewWeightedPairCanvasRuntimeLayer>({
  slot,
  canvasWidth,
  canvasHeight,
  stageWidth,
  stageHeight,
  registerVideo,
}: PreviewWeightedPairCanvasProps<T>) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const sourceRefs = useRef(new Map<string, RasterSource>());
  const [sourceRevision, setSourceRevision] = useState(0);
  const [ready, setReady] = useState(false);
  const bumpSourceRevision = useCallback(() => setSourceRevision((value) => value + 1), []);

  const setSourceRef = useCallback((clipId: string, node: RasterSource | null) => {
    if (node) sourceRefs.current.set(clipId, node);
    else sourceRefs.current.delete(clipId);
  }, []);

  const draw = useCallback((): boolean => {
    const canvas = canvasRef.current;
    if (!canvas || canvasWidth <= 0 || canvasHeight <= 0) return false;
    if (slot.execution !== 'weighted-canvas-deferred' || !slot.weightedRasterSource?.supported) return false;
    const pairLayers = [slot.lower, slot.upper] as const;
    const isolated = new Map<string, ImageData>();

    for (const layer of pairLayers) {
      const source = sourceRefs.current.get(layer.clip.id);
      if (!source || !sourceReady(source)) return false;
      const [intrinsicWidth, intrinsicHeight] = intrinsicSize(source);
      const layerPlan = resolvePreviewWeightedPairCanvasLayerPlan(
        slot,
        layer,
        intrinsicWidth,
        intrinsicHeight,
      );
      const surface = document.createElement('canvas');
      surface.width = canvasWidth;
      surface.height = canvasHeight;
      const context = surface.getContext('2d', { willReadFrequently: true });
      if (!context) throw new Error('weighted pair Canvas could not create isolated 2D context');
      context.clearRect(0, 0, canvasWidth, canvasHeight);
      paintPreviewWeightedPairCanvasLayer(context, source, layerPlan);
      isolated.set(layer.clip.id, context.getImageData(0, 0, canvasWidth, canvasHeight));
    }

    const outgoing = isolated.get(slot.pixel.outgoing_clip_id);
    const incoming = isolated.get(slot.pixel.incoming_clip_id);
    if (!outgoing || !incoming) {
      throw new Error(`weighted pair ${JSON.stringify(slot.surface.transition_id)} raster inputs do not match canonical outgoing/incoming ids`);
    }
    const output = composeWeightedTransitionPairRgba(slot.pixel, outgoing.data, incoming.data);
    const context = canvas.getContext('2d');
    if (!context) throw new Error('weighted pair Canvas could not create output 2D context');
    context.clearRect(0, 0, canvasWidth, canvasHeight);
    context.putImageData(new ImageData(output, canvasWidth, canvasHeight), 0, 0);
    return true;
  }, [canvasHeight, canvasWidth, slot, sourceRevision]);

  useEffect(() => {
    setReady(false);
    setReady(draw());
  }, [draw]);

  const renderSource = (layer: T) => {
    const asset = layer.asset;
    if (!asset) throw new Error(`weighted pair source ${JSON.stringify(layer.clip.id)} is missing its media asset`);
    const commonStyle = {
      position: 'absolute' as const,
      left: 0,
      top: 0,
      width: 1,
      height: 1,
      opacity: 0,
      pointerEvents: 'none' as const,
    };
    if (asset.mime_type.startsWith('video/')) {
      return (
        <video
          key={`weighted-source-${layer.clip.id}`}
          ref={(node) => {
            setSourceRef(layer.clip.id, node);
            registerVideo(layer.clip.id, node);
          }}
          data-video-preview-media="true"
          data-preview-weighted-pair-source={layer.clip.id}
          src={videoApi.downloadUrl(asset.id)}
          preload="auto"
          playsInline
          muted
          autoPlay={false}
          aria-hidden="true"
          tabIndex={-1}
          style={commonStyle}
          onLoadedMetadata={bumpSourceRevision}
          onLoadedData={bumpSourceRevision}
          onSeeked={bumpSourceRevision}
          onError={bumpSourceRevision}
        />
      );
    }
    if (asset.mime_type.startsWith('image/')) {
      return (
        <img
          key={`weighted-source-${layer.clip.id}`}
          ref={(node) => setSourceRef(layer.clip.id, node)}
          data-preview-weighted-pair-source={layer.clip.id}
          src={videoApi.downloadUrl(asset.id)}
          alt=""
          aria-hidden="true"
          style={commonStyle}
          onLoad={bumpSourceRevision}
          onError={bumpSourceRevision}
        />
      );
    }
    throw new Error(`weighted pair source ${JSON.stringify(layer.clip.id)} has unsupported media ${JSON.stringify(asset.mime_type)}`);
  };

  return (
    <div
      key={`weighted-transition-pair-${slot.surface.transition_id}`}
      data-preview-transition-pair-id={slot.surface.transition_id}
      data-preview-transition-pair-execution="weighted-canvas"
      data-preview-transition-pair-lower-clip={slot.surface.lower_clip_id}
      data-preview-transition-pair-upper-clip={slot.surface.upper_clip_id}
      data-preview-transition-pair-ready={ready ? 'true' : 'false'}
      className="pointer-events-none absolute inset-0"
    >
      {renderSource(slot.lower)}
      {renderSource(slot.upper)}
      <canvas
        ref={canvasRef}
        width={canvasWidth}
        height={canvasHeight}
        className="absolute inset-0"
        style={{ width: stageWidth, height: stageHeight }}
        aria-hidden="true"
      />
    </div>
  );
}

function sourceReady(source: RasterSource): boolean {
  if (source instanceof HTMLVideoElement) {
    return source.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA
      && !source.seeking
      && source.videoWidth > 0
      && source.videoHeight > 0;
  }
  return source.complete && source.naturalWidth > 0 && source.naturalHeight > 0;
}

function intrinsicSize(source: RasterSource): [number, number] {
  return source instanceof HTMLVideoElement
    ? [source.videoWidth, source.videoHeight]
    : [source.naturalWidth, source.naturalHeight];
}
