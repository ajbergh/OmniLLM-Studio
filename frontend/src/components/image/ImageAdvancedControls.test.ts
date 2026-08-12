import { describe, expect, it } from 'vitest';
import {
  buildSizeOptions,
  getSourcePreservingEditSize,
} from './ImageAdvancedControls';

describe('ImageAdvancedControls size helpers', () => {
  it('handles provider auto size capabilities without treating them as dimensions', () => {
    expect(buildSizeOptions(['auto', '1024x1024'])).toEqual([
      { value: 'auto', label: 'Auto', desc: 'Provider-selected output size' },
      { value: '1024x1024', label: '1:1', desc: '1024×1024' },
    ]);
  });

  it('keeps unknown future capability values selectable instead of failing ratio parsing', () => {
    expect(buildSizeOptions(['native'])).toEqual([
      { value: 'native', label: 'native', desc: 'native' },
    ]);
  });

  it('derives aspect labels for provider-specific dimensions', () => {
    expect(buildSizeOptions(['1536x1024'])).toEqual([
      { value: '1536x1024', label: '3:2', desc: '1536×1024' },
    ]);
  });

  it('uses auto when a provider exposes it and omission otherwise for source-preserving edits', () => {
    expect(getSourcePreservingEditSize(['auto', '1024x1024'])).toBe('auto');
    expect(getSourcePreservingEditSize(['1:1', '16:9'])).toBe('');
    expect(getSourcePreservingEditSize(undefined)).toBe('');
  });
});
