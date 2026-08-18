import { describe, it, expect } from 'vitest';
import { buildCssFilterString, ContractTimelineEffect } from '../core/filters';

describe('EffectStack filter construction', () => {
  it('constructs CSS filter string matching effects', () => {
    const effects: ContractTimelineEffect[] = [
      { id: 'e1', type: 'blur', amount: 0.5, enabled: true },
      { id: 'e2', type: 'brightness', amount: 1.2, enabled: true },
      { id: 'e3', type: 'contrast', amount: 0.8, enabled: false }, // disabled
      { id: 'e4', type: 'grayscale', amount: 1, enabled: true },
    ];

    const filterString = buildCssFilterString(effects);
    expect(filterString).toBe('blur(5px) brightness(1.2) grayscale(1)');
  });

  it('handles empty or disabled effects', () => {
    expect(buildCssFilterString([])).toBe('');
    expect(buildCssFilterString([{ id: 'e', type: 'sepia', amount: 1, enabled: false }])).toBe('');
  });
});
