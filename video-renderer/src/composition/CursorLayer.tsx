import React from 'react';
import { ContractTimelineCursor } from '../contract/timeline';

export interface CursorLayerProps {
  cursor: ContractTimelineCursor;
  timeMs: number;
}

export const CursorLayer: React.FC<CursorLayerProps> = ({ cursor, timeMs }) => {
  const points = cursor.points || [];
  if (points.length === 0) return null;

  let currentPoint = points[0];
  let isClick = false;

  for (let i = 0; i < points.length; i++) {
    const pt = points[i];
    if (timeMs >= pt.timeMs) {
      currentPoint = pt;
      if (pt.click && Math.abs(timeMs - pt.timeMs) <= 300) {
        isClick = true;
      }
    }
  }

  const size = cursor.size || 24;
  const color = cursor.color || '#3b82f6';

  return (
    <div
      style={{
        position: 'absolute',
        left: `${currentPoint.x}px`,
        top: `${currentPoint.y}px`,
        transform: 'translate(-2px, -2px)',
        pointerEvents: 'none',
        zIndex: 9999,
      }}
    >
      <svg width={size} height={size} viewBox="0 0 24 24" fill={color}>
        <path d="M3 3l7 18 3-7 7-3L3 3z" stroke="#ffffff" strokeWidth="1.5" />
      </svg>
      {isClick && (
        <div
          style={{
            position: 'absolute',
            left: 0,
            top: 0,
            width: `${size * 1.5}px`,
            height: `${size * 1.5}px`,
            borderRadius: '50%',
            border: `2px solid ${color}`,
            transform: 'translate(-25%, -25%) scale(1.2)',
            opacity: 0.8,
            animation: 'ping 0.3s cubic-bezier(0, 0, 0.2, 1) forwards',
          }}
        />
      )}
    </div>
  );
};
