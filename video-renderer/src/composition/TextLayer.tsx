import React from 'react';
import { ContractTimelineText } from '../contract/timeline';

export interface TextLayerProps {
  text: ContractTimelineText;
  opacity?: number;
}

export const TextLayer: React.FC<TextLayerProps> = ({ text, opacity = 1 }) => {
  const style: React.CSSProperties = {
    color: text.color || '#ffffff',
    fontSize: `${text.fontSize || 32}px`,
    fontFamily: text.fontFamily || 'Inter, sans-serif',
    fontWeight: text.fontWeight || 'normal',
    fontStyle: text.fontStyle || 'normal',
    textAlign: text.textAlign || 'center',
    background: text.background || 'transparent',
    lineHeight: text.lineHeight ?? 1.25,
    letterSpacing: text.letterSpacing ? `${text.letterSpacing}px` : undefined,
    borderRadius: text.borderRadius ? `${text.borderRadius}px` : undefined,
    padding: text.padding ? `${text.padding}px` : undefined,
    maxWidth: text.wrapWidth ? `${text.wrapWidth}px` : undefined,
    whiteSpace: 'pre-wrap',
    textShadow: text.shadow ? '0 2px 8px rgba(0, 0, 0, 0.75)' : undefined,
    WebkitTextStroke: text.stroke && text.strokeWidth ? `${text.strokeWidth}px ${text.stroke}` : undefined,
    opacity,
  };

  return <div style={style}>{text.text}</div>;
};
