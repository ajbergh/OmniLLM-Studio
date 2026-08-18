import React from 'react';
import { ContractTimelineShape } from '../contract/timeline';

export interface ShapeLayerProps {
  shape: ContractTimelineShape;
  width?: number;
  height?: number;
}

export const ShapeLayer: React.FC<ShapeLayerProps> = ({
  shape,
  width = 120,
  height = 120,
}) => {
  const fill = shape.fill || '#3b82f6';
  const stroke = shape.stroke || 'transparent';
  const strokeWidth = shape.strokeWidth || 0;
  const cornerRadius = shape.cornerRadius || 0;
  const opacity = shape.opacity ?? 1;

  switch (shape.kind) {
    case 'ellipse':
    case 'circle':
      return (
        <svg width={width} height={height} style={{ opacity, overflow: 'visible' }}>
          <ellipse
            cx={width / 2}
            cy={height / 2}
            rx={width / 2}
            ry={height / 2}
            fill={fill}
            stroke={stroke}
            strokeWidth={strokeWidth}
          />
        </svg>
      );
    case 'arrow':
      return (
        <svg width={width} height={height} viewBox="0 0 100 100" style={{ opacity, overflow: 'visible' }}>
          <path
            d="M 10 50 L 60 50 L 60 30 L 90 50 L 60 70 L 60 50 Z"
            fill={fill}
            stroke={stroke}
            strokeWidth={strokeWidth}
          />
        </svg>
      );
    case 'speech_bubble':
      return (
        <svg width={width} height={height} viewBox="0 0 100 100" style={{ opacity, overflow: 'visible' }}>
          <path
            d="M 10 20 Q 10 10 20 10 L 80 10 Q 90 10 90 20 L 90 60 Q 90 70 80 70 L 40 70 L 20 90 L 25 70 L 20 70 Q 10 70 10 60 Z"
            fill={fill}
            stroke={stroke}
            strokeWidth={strokeWidth}
          />
        </svg>
      );
    case 'rectangle':
    default:
      return (
        <div
          style={{
            width: `${width}px`,
            height: `${height}px`,
            backgroundColor: fill,
            border: strokeWidth > 0 ? `${strokeWidth}px solid ${stroke}` : undefined,
            borderRadius: cornerRadius > 0 ? `${cornerRadius}px` : undefined,
            opacity,
            filter: shape.blurRadius ? `blur(${shape.blurRadius}px)` : undefined,
          }}
        />
      );
  }
};
