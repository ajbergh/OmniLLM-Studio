import React from 'react';
import { ContractTimelineEffect, buildCssFilterString } from '../core/filters';

export interface EffectStackProps {
  effects: ContractTimelineEffect[];
  children: React.ReactNode;
}

export { buildCssFilterString };

export const EffectStack: React.FC<EffectStackProps> = ({ effects, children }) => {
  const filterString = buildCssFilterString(effects);
  if (!filterString) return <>{children}</>;

  return <div style={{ filter: filterString, width: '100%', height: '100%' }}>{children}</div>;
};
