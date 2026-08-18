import React from 'react';
import { FrameState, EvaluatedClipState } from '../core/evaluateFrame';

export interface VideoCompositionProps {
  frameState: FrameState;
  assetUrlMap: Record<string, string>;
}

export const VideoComposition: React.FC<VideoCompositionProps> = ({
  frameState,
  assetUrlMap,
}) => {
  return (
    <div
      style={{
        position: 'relative',
        width: `${frameState.width}px`,
        height: `${frameState.height}px`,
        backgroundColor: frameState.backgroundColor,
        overflow: 'hidden',
      }}
    >
      {frameState.activeClips.map((clip) => {
        const assetUrl = clip.assetId ? assetUrlMap[clip.assetId] : undefined;
        return (
          <div
            key={clip.id}
            style={{
              position: 'absolute',
              left: `${clip.transform.x + frameState.width / 2}px`,
              top: `${clip.transform.y + frameState.height / 2}px`,
              transform: `translate(-50%, -50%) translate(${clip.transform.anchorX}px, ${clip.transform.anchorY}px) scale(${clip.transform.scaleX}, ${clip.transform.scaleY}) rotateZ(${clip.transform.rotationZ}deg)`,
              opacity: clip.transform.opacity,
              zIndex: clip.zIndex,
              pointerEvents: 'none',
            }}
          >
            {clip.text && (
              <div
                style={{
                  color: clip.text.color || '#ffffff',
                  fontSize: `${clip.text.fontSize || 32}px`,
                  fontFamily: clip.text.fontFamily || 'sans-serif',
                  fontWeight: clip.text.fontWeight || 'normal',
                  textAlign: clip.text.textAlign || 'center',
                  background: clip.text.background || 'transparent',
                  padding: clip.text.padding ? `${clip.text.padding}px` : undefined,
                  borderRadius: clip.text.borderRadius ? `${clip.text.borderRadius}px` : undefined,
                  whiteSpace: 'pre-wrap',
                }}
              >
                {clip.text.text}
              </div>
            )}
            {clip.shape && (
              <div
                style={{
                  width: '100px',
                  height: '100px',
                  backgroundColor: clip.shape.fill || '#3b82f6',
                  borderRadius: clip.shape.cornerRadius ? `${clip.shape.cornerRadius}px` : undefined,
                }}
              />
            )}
            {assetUrl && (
              <img
                src={assetUrl}
                alt=""
                style={{
                  maxWidth: '100%',
                  maxHeight: '100%',
                  objectFit: clip.fitMode,
                }}
              />
            )}
          </div>
        );
      })}
    </div>
  );
};
