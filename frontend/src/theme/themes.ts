import type { ThemeDefinition, ThemeId } from './tokens';

/**
 * OLED Black — true pure black (#000000) theme optimized for OLED displays with crisp contrast.
 */
const oled: ThemeDefinition = {
  id: 'oled',
  name: 'OLED Black',
  isDark: true,
  tokens: {
    // Surfaces — true pitch blacks and ultra-dark neutral layers
    surface: '#000000',
    background: '#000000',
    surfaceRaised: '#080808',
    surfaceAlt: '#0f0f0f',
    surfaceLight: '#171717',
    surfaceHover: '#171717',
    surfaceGlass: 'rgba(0, 0, 0, 0.85)',

    // Borders — subtle dark neutrals with high-contrast focus
    border: '#1f1f1f',
    borderSubtle: '#141414',
    borderFocus: '#6366f1',

    // Text — clean high-contrast neutrals
    text: '#f3f4f6',
    textSecondary: '#9ca3af',
    textMuted: '#6b7280',

    // Brand — vibrant indigo/purple accents tailored for deep black background
    primary: '#6366f1',
    primaryHover: '#818cf8',
    primaryGlow: 'rgba(99, 102, 241, 0.20)',
    primaryRgb: '99, 102, 241',
    accent: '#a855f7',
    accentGlow: 'rgba(168, 85, 247, 0.20)',
    accentRgb: '168, 85, 247',

    // Status
    danger: '#ef4444',
    dangerSoft: 'rgba(239, 68, 68, 0.15)',
    success: '#10b981',
    successSoft: 'rgba(16, 185, 129, 0.15)',
    warning: '#f59e0b',
    warningSoft: 'rgba(245, 158, 11, 0.15)',

    // Shadows — deeper contrast on true black
    shadowSm: '0 1px 2px rgba(0, 0, 0, 0.8)',
    shadowMd: '0 4px 12px rgba(0, 0, 0, 0.85)',
    shadowLg: '0 8px 32px rgba(0, 0, 0, 0.9)',
    shadowGlow: '0 0 20px rgba(99, 102, 241, 0.18)',
    shadowGlowLg: '0 0 60px rgba(99, 102, 241, 0.12), 0 0 20px rgba(168, 85, 247, 0.10)',
  },
};

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
    background: '#0c0c16',
    surfaceRaised: '#111122',
    surfaceAlt: '#161628',
    surfaceLight: '#1e1e38',
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
    background: '#120e0a',
    surfaceRaised: '#1a1510',
    surfaceAlt: '#1e1812',
    surfaceLight: '#2a221a',
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
 * Synthwave — 80s cyberpunk retro-futuristic dark theme with neon cyan & hot magenta.
 */
const synthwave: ThemeDefinition = {
  id: 'synthwave',
  name: 'Synthwave',
  isDark: true,
  tokens: {
    // Surfaces — deep neon midnight violet
    surface: '#18122B',
    background: '#18122B',
    surfaceRaised: '#22183d',
    surfaceAlt: '#2c1e4e',
    surfaceLight: '#392864',
    surfaceHover: '#392864',
    surfaceGlass: 'rgba(34, 24, 61, 0.85)',

    // Borders — vivid electric magenta/violet borders
    border: '#493077',
    borderSubtle: '#332057',
    borderFocus: '#00f2fe',

    // Text — soft lavender-white with pastel contrast
    text: '#fdf2ff',
    textSecondary: '#d8b4fe',
    textMuted: '#9d7bb0',

    // Brand — vibrant neon cyan and hot magenta
    primary: '#00f2fe',
    primaryHover: '#38f9d7',
    primaryGlow: 'rgba(0, 242, 254, 0.22)',
    primaryRgb: '0, 242, 254',
    accent: '#ff007f',
    accentGlow: 'rgba(255, 0, 127, 0.22)',
    accentRgb: '255, 0, 127',

    // Status
    danger: '#ff3366',
    dangerSoft: 'rgba(255, 51, 102, 0.18)',
    success: '#05ffa1',
    successSoft: 'rgba(5, 255, 161, 0.18)',
    warning: '#ffe600',
    warningSoft: 'rgba(255, 230, 0, 0.18)',

    // Shadows — electric neon glow
    shadowSm: '0 1px 3px rgba(10, 5, 20, 0.6)',
    shadowMd: '0 4px 14px rgba(10, 5, 20, 0.7)',
    shadowLg: '0 8px 32px rgba(10, 5, 20, 0.85)',
    shadowGlow: '0 0 20px rgba(0, 242, 254, 0.2)',
    shadowGlowLg: '0 0 60px rgba(0, 242, 254, 0.14), 0 0 24px rgba(255, 0, 127, 0.14)',
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
    background: '#f8f9fc',
    surfaceRaised: '#ffffff',
    surfaceAlt: '#eef0f5',
    surfaceLight: '#e2e5ed',
    surfaceHover: '#e2e5ed',
    surfaceGlass: 'rgba(255, 255, 255, 0.8)',

    // Borders
    border: '#d1d5e0',
    borderSubtle: '#e2e5ed',
    borderFocus: '#4f46e5',

    // Text
    text: '#1a1a2e',
    textSecondary: '#4a4a6a',
    textMuted: '#62627c',

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
    background: '#0a0a0a',
    surfaceRaised: '#111111',
    surfaceAlt: '#141414',
    surfaceLight: '#1c1c1c',
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
export const THEMES: ThemeDefinition[] = [oled, aurora, ember, synthwave, light, terminal];

/** O(1) lookup by theme ID. */
export const THEME_MAP: Record<ThemeId, ThemeDefinition> = {
  oled,
  aurora,
  ember,
  synthwave,
  light,
  terminal,
};

/** Default theme ID. */
export const DEFAULT_THEME_ID: ThemeId = 'oled';
