import { describe, it, expect } from 'vitest';
import { THEMES, THEME_MAP, DEFAULT_THEME_ID } from './themes';

describe('Theme System', () => {
  it('has OLED Black as the default theme', () => {
    expect(DEFAULT_THEME_ID).toBe('oled');
    expect(THEME_MAP['oled']).toBeDefined();
    expect(THEME_MAP['oled'].name).toBe('OLED Black');
    expect(THEME_MAP['oled'].isDark).toBe(true);
    expect(THEME_MAP['oled'].tokens.surface).toBe('#000000');
    expect(THEME_MAP['oled'].tokens.background).toBe('#000000');
  });

  it('includes all expected themes in THEMES and THEME_MAP', () => {
    const expectedIds = ['oled', 'aurora', 'ember', 'synthwave', 'light', 'terminal'];
    expect(THEMES.map((t) => t.id)).toEqual(expectedIds);
    for (const id of expectedIds) {
      expect(THEME_MAP[id as keyof typeof THEME_MAP]).toBeDefined();
      expect(THEME_MAP[id as keyof typeof THEME_MAP].tokens.surface).toBeDefined();
      expect(THEME_MAP[id as keyof typeof THEME_MAP].tokens.primary).toBeDefined();
      expect(THEME_MAP[id as keyof typeof THEME_MAP].tokens.accent).toBeDefined();
    }
  });

  it('validates color tokens format for OLED Black', () => {
    const oled = THEME_MAP['oled'];
    expect(oled.tokens.surface).toMatch(/^#[0-9a-fA-F]{6}$/);
    expect(oled.tokens.primary).toMatch(/^#[0-9a-fA-F]{6}$/);
    expect(oled.tokens.accent).toMatch(/^#[0-9a-fA-F]{6}$/);
    expect(oled.tokens.surfaceGlass).toContain('rgba');
  });
});

