import type { ThemeDefinition, ThemeId } from './tokens';

/**
 * Aurora Ink — default dark theme with indigo/purple tones.
 * Values extracted verbatim from the existing @theme block in index.css.
 */
const aurora: ThemeDefinition = {
  id: 'aurora',
  name: 'Aurora Ink',
  isDark: true,
  tokens: {
    // Surfaces
    surface: '#0c0c16',
    surfaceRaised: '#111122',
    surfaceAlt: '#161628',
    surfaceHover: '#1e1e38',
    surfaceGlass: 'rgba(22, 22, 40, 0.8)',

    // Borders
    border: '#232340',
    borderSubtle: '#1a1a30',
    borderFocus: '#6366f1',

    // Text
    text: '#eaeaf0',
    textSecondary: '#b8b8d0',
    textMuted: '#8a8aa8',

    // Brand
    primary: '#6366f1',
    primaryHover: '#818cf8',
    primaryGlow: 'rgba(99, 102, 241, 0.15)',
    primaryRgb: '99, 102, 241',
    accent: '#a855f7',
    accentGlow: 'rgba(168, 85, 247, 0.15)',
    accentRgb: '168, 85, 247',

    // Status
    danger: '#ef4444',
    dangerSoft: 'rgba(239, 68, 68, 0.15)',
    success: '#10b981',
    successSoft: 'rgba(16, 185, 129, 0.15)',
    warning: '#f59e0b',
    warningSoft: 'rgba(245, 158, 11, 0.15)',

    // Shadows
    shadowSm: '0 1px 2px rgba(0, 0, 0, 0.3)',
    shadowMd: '0 4px 12px rgba(0, 0, 0, 0.4)',
    shadowLg: '0 8px 32px rgba(0, 0, 0, 0.5)',
    shadowGlow: '0 0 20px rgba(99, 102, 241, 0.15)',
    shadowGlowLg: '0 0 60px rgba(99, 102, 241, 0.1), 0 0 20px rgba(168, 85, 247, 0.08)',
  },
};

/**
 * Ember — warm dark theme with amber/copper tones.
 */
const ember: ThemeDefinition = {
  id: 'ember',
  name: 'Ember',
  isDark: true,
  tokens: {
    // Surfaces — warm dark tones
    surface: '#120e0a',
    surfaceRaised: '#1a1510',
    surfaceAlt: '#1e1812',
    surfaceHover: '#2a221a',
    surfaceGlass: 'rgba(30, 24, 18, 0.8)',

    // Borders
    border: '#3d3020',
    borderSubtle: '#2a221a',
    borderFocus: '#f59e0b',

    // Text — warm cream
    text: '#f5ede4',
    textSecondary: '#d4c4b0',
    textMuted: '#a08d78',

    // Brand — amber/copper
    primary: '#f59e0b',
    primaryHover: '#fbbf24',
    primaryGlow: 'rgba(245, 158, 11, 0.15)',
    primaryRgb: '245, 158, 11',
    accent: '#ea580c',
    accentGlow: 'rgba(234, 88, 12, 0.15)',
    accentRgb: '234, 88, 12',

    // Status
    danger: '#ef4444',
    dangerSoft: 'rgba(239, 68, 68, 0.15)',
    success: '#10b981',
    successSoft: 'rgba(16, 185, 129, 0.15)',
    warning: '#fbbf24',
    warningSoft: 'rgba(251, 191, 36, 0.15)',

    // Shadows
    shadowSm: '0 1px 2px rgba(0, 0, 0, 0.3)',
    shadowMd: '0 4px 12px rgba(0, 0, 0, 0.4)',
    shadowLg: '0 8px 32px rgba(0, 0, 0, 0.5)',
    shadowGlow: '0 0 20px rgba(245, 158, 11, 0.12)',
    shadowGlowLg: '0 0 60px rgba(245, 158, 11, 0.08), 0 0 20px rgba(234, 88, 12, 0.06)',
  },
};

/**
 * Cloud — a clean light theme for daytime use.
 */
const light: ThemeDefinition = {
  id: 'light',
  name: 'Cloud',
  isDark: false,
  tokens: {
    // Surfaces
    surface: '#f8f9fc',
    surfaceRaised: '#ffffff',
    surfaceAlt: '#eef0f5',
    surfaceHover: '#e2e5ed',
    surfaceGlass: 'rgba(255, 255, 255, 0.8)',

    // Borders
    border: '#d1d5e0',
    borderSubtle: '#e2e5ed',
    borderFocus: '#4f46e5',

    // Text
    text: '#1a1a2e',
    textSecondary: '#4a4a6a',
    textMuted: '#8888a8',

    // Brand
    primary: '#4f46e5',
    primaryHover: '#6366f1',
    primaryGlow: 'rgba(79, 70, 229, 0.12)',
    primaryRgb: '79, 70, 229',
    accent: '#7c3aed',
    accentGlow: 'rgba(124, 58, 237, 0.10)',
    accentRgb: '124, 58, 237',

    // Status
    danger: '#dc2626',
    dangerSoft: 'rgba(220, 38, 38, 0.10)',
    success: '#059669',
    successSoft: 'rgba(5, 150, 105, 0.10)',
    warning: '#d97706',
    warningSoft: 'rgba(217, 119, 6, 0.10)',

    // Shadows (lighter for light theme)
    shadowSm: '0 1px 2px rgba(0, 0, 0, 0.06)',
    shadowMd: '0 4px 12px rgba(0, 0, 0, 0.08)',
    shadowLg: '0 8px 32px rgba(0, 0, 0, 0.10)',
    shadowGlow: '0 0 20px rgba(79, 70, 229, 0.10)',
    shadowGlowLg: '0 0 60px rgba(79, 70, 229, 0.06), 0 0 20px rgba(124, 58, 237, 0.04)',
  },
};

/**
 * Phosphor — retro green-on-black terminal look with monospace font.
 */
const terminal: ThemeDefinition = {
  id: 'terminal',
  name: 'Terminal',
  isDark: true,
  tokens: {
    // Surfaces — pure blacks
    surface: '#0a0a0a',
    surfaceRaised: '#111111',
    surfaceAlt: '#141414',
    surfaceHover: '#1c1c1c',
    surfaceGlass: 'rgba(10, 10, 10, 0.9)',

    // Borders — dim green
    border: '#1a3a1a',
    borderSubtle: '#142814',
    borderFocus: '#00ff41',

    // Text — phosphor green
    text: '#00ff41',
    textSecondary: '#00cc33',
    textMuted: '#338833',

    // Brand — bright phosphor green
    primary: '#00ff41',
    primaryHover: '#33ff66',
    primaryGlow: 'rgba(0, 255, 65, 0.15)',
    primaryRgb: '0, 255, 65',
    accent: '#00cc33',
    accentGlow: 'rgba(0, 204, 51, 0.12)',
    accentRgb: '0, 204, 51',

    // Status
    danger: '#ff3333',
    dangerSoft: 'rgba(255, 51, 51, 0.15)',
    success: '#00ff41',
    successSoft: 'rgba(0, 255, 65, 0.15)',
    warning: '#ffcc00',
    warningSoft: 'rgba(255, 204, 0, 0.15)',

    // Shadows — green glow
    shadowSm: '0 1px 2px rgba(0, 0, 0, 0.5)',
    shadowMd: '0 4px 12px rgba(0, 0, 0, 0.6)',
    shadowLg: '0 8px 32px rgba(0, 0, 0, 0.7)',
    shadowGlow: '0 0 20px rgba(0, 255, 65, 0.12)',
    shadowGlowLg: '0 0 60px rgba(0, 255, 65, 0.08), 0 0 20px rgba(0, 204, 51, 0.06)',

    // Terminal monospace font
    fontFamily: "'Courier New', 'Lucida Console', 'Consolas', monospace",
  },
};

/** All available themes. */
export const THEMES: ThemeDefinition[] = [aurora, ember, light, terminal];

/** O(1) lookup by theme ID. */
export const THEME_MAP: Record<ThemeId, ThemeDefinition> = {
  aurora,
  ember,
  light,
  terminal,
};

/** Default theme ID. */
export const DEFAULT_THEME_ID: ThemeId = 'aurora';
