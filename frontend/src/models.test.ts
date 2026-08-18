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

  it('sorts alphabetically when custom models are added', () => {
    const custom = ['0-test-model', 'z-test-model'];
    const models = getKnownChatModels('openrouter', custom);
    expect(models[0]).toBe('0-test-model');
    expect(models[models.length - 1]).toBe('z-test-model');
  });

  it('works for gemini provider with custom models', () => {
    const custom = ['gemini-2.0-flash-exp', 'gemini-1.5-flash'];
    const models = getKnownChatModels('gemini', custom);
    expect(models).toContain('gemini-2.0-flash-exp');
    expect(models).toContain('gemini-1.5-flash');
    const flashCount = models.filter((m) => m === 'gemini-1.5-flash').length;
    expect(flashCount).toBe(1);
  });
});
