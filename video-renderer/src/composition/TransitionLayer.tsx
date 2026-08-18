import React from 'react';
import { ContractTimelineTransition } from '../contract/timeline';

export interface TransitionLayerProps {
  transition: ContractTimelineTransition;
  progress: number; // 0 to 1
  children: React.ReactNode;
}

export const TransitionLayer: React.FC<TransitionLayerProps> = ({
  transition,
  progress,
  children,
}) => {
  const p = Math.max(0, Math.min(1, progress));

  switch (transition.type) {
    case 'fade':
    case 'crossfade':
      return <div style={{ opacity: p, width: '100%', height: '100%' }}>{children}</div>;

    case 'slide_left':
      return (
        <div
          style={{
            transform: `translateX(${(1 - p) * 100}%)`,
            width: '100%',
            height: '100%',
          }}
        >
          {children}
        </div>
      );

    case 'slide_right':
      return (
        <div
          style={{
            transform: `translateX(${-(1 - p) * 100}%)`,
            width: '100%',
            height: '100%',
          }}
        >
          {children}
        </div>
      );

    case 'wipe_left':
      return (
        <div
          style={{
            clipPath: `inset(0 ${(1 - p) * 100}% 0 0)`,
            width: '100%',
            height: '100%',
          }}
        >
          {children}
        </div>
      );

    case 'zoom_in':
      return (
        <div
          style={{
            transform: `scale(${p})`,
            opacity: p,
            width: '100%',
            height: '100%',
          }}
        >
          {children}
        </div>
      );

    default:
      return <>{children}</>;
  }
};
