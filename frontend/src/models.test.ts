import { describe, it, expect } from 'vitest';
import { getKnownChatModels } from './models';

describe('getKnownChatModels', () => {
  it('returns default catalog for openrouter when no custom models given', () => {
    const models = getKnownChatModels('openrouter');
    expect(models).toContain('openai/gpt-5.5');
    expect(models).toContain('anthropic/claude-sonnet-4.6');
  });

  it('includes and dedupes custom models for openrouter', () => {
    const custom = ['qwen/qwen3.8-27b', 'dots-studio/dots3-note-preview:free', 'openai/gpt-5.5'];
    const models = getKnownChatModels('openrouter', custom);
    expect(models).toContain('qwen/qwen3.8-27b');
    expect(models).toContain('dots-studio/dots3-note-preview:free');
    expect(models).toContain('openai/gpt-5.5');
    // Ensure no duplicates
    const gpt55Count = models.filter((m) => m === 'openai/gpt-5.5').length;
    expect(gpt55Count).toBe(1);
  });

  it('handles empty or undefined customModels array gracefully', () => {
    const models1 = getKnownChatModels('openrouter', []);
    const models2 = getKnownChatModels('openrouter', undefined);
    expect(models1.length).toBeGreaterThan(0);
    expect(models1).toEqual(models2);
  });
});
