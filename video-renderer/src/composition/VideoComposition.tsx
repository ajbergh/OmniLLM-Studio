import React from 'react';
import { FrameState, EvaluatedClipState } from '../core/evaluateFrame';
import { TextLayer } from './TextLayer';
import { ShapeLayer } from './ShapeLayer';
import { CursorLayer } from './CursorLayer';
import { EffectStack } from './EffectStack';
import { TransitionLayer } from './TransitionLayer';

export interface VideoCompositionProps {
  frameState: FrameState;
  assetUrlMap: Record<string, string>;
}

export const VideoComposition: React.FC<VideoCompositionProps> = ({
  frameState,
  assetUrlMap,
}) => {
  const content = (
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

        // Base transform with center origin offset and anchor compensation
        const transformStyle: React.CSSProperties = {
          position: 'absolute',
          left: `${clip.transform.x + frameState.width / 2}px`,
          top: `${clip.transform.y + frameState.height / 2}px`,
          transform: `translate(-50%, -50%) translate(${clip.transform.anchorX}px, ${clip.transform.anchorY}px) scale3d(${clip.transform.scaleX}, ${clip.transform.scaleY}, ${clip.transform.scaleZ}) rotateX(${clip.transform.rotationX}deg) rotateY(${clip.transform.rotationY}deg) rotateZ(${clip.transform.rotationZ}deg)`,
          opacity: clip.transform.opacity,
          zIndex: clip.zIndex,
          pointerEvents: 'none',
        };

        const clipInner = (
          <EffectStack effects={clip.effects}>
            {clip.text && <TextLayer text={clip.text} />}
            {clip.shape && <ShapeLayer shape={clip.shape} />}
            {clip.cursor && <CursorLayer cursor={clip.cursor} timeMs={clip.clipTimeMs} />}
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
          </EffectStack>
        );

        return (
          <div key={clip.id} style={transformStyle}>
            {clipInner}
          </div>
        );
      })}
    </div>
  );

  // Wrap in scene effects if present
  if (frameState.activeScene && frameState.activeScene.effects.length > 0) {
    return <EffectStack effects={frameState.activeScene.effects}>{content}</EffectStack>;
  }

  return content;
};
