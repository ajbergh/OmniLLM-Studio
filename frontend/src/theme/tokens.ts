/**
 * Theme token types for OmniLLM-Studio.
 *
 * Each token maps 1:1 to a CSS custom property used by Tailwind v4 and global styles.
 * Token names use camelCase; the CSS variable mapping is in cssVars.ts.
 */

export type ThemeId = 'aurora' | 'ember' | 'light' | 'terminal';

/**
 * Complete set of design tokens defining a color theme.
 * Every field corresponds to a `--color-*` or `--shadow-*` CSS variable.
 */
export interface ThemeTokens {
  // ── Surfaces ──
  surface: string;        // --color-surface (app background)
  surfaceRaised: string;  // --color-surface-raised
  surfaceAlt: string;     // --color-surface-alt
  surfaceHover: string;   // --color-surface-hover
  surfaceGlass: string;   // --color-surface-glass (rgba)

  // ── Borders ──
  border: string;         // --color-border
  borderSubtle: string;   // --color-border-subtle
  borderFocus: string;    // --color-border-focus

  // ── Text ──
  text: string;           // --color-text
  textSecondary: string;  // --color-text-secondary
  textMuted: string;      // --color-text-muted

  // ── Brand ──
  primary: string;        // --color-primary
  primaryHover: string;   // --color-primary-hover
  primaryGlow: string;    // --color-primary-glow (rgba)
  primaryRgb: string;     // R,G,B triplet for composing rgba() at varying opacities
  accent: string;         // --color-accent
  accentGlow: string;     // --color-accent-glow (rgba)
  accentRgb: string;      // R,G,B triplet

  // ── Status colors ──
  danger: string;
  dangerSoft: string;
  success: string;
  successSoft: string;
  warning: string;
  warningSoft: string;

  // ── Shadows ──
  shadowSm: string;
  shadowMd: string;
  shadowLg: string;
  shadowGlow: string;
  shadowGlowLg: string;

  // ── Typography (optional) ──
  fontFamily?: string;   // --font-family (overrides body font)
}

/** A fully described theme with metadata. */
export interface ThemeDefinition {
  id: ThemeId;
  name: string;
  tokens: ThemeTokens;
  isDark: boolean;
}
