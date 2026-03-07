import type { ThemeTokens, ThemeId } from './tokens';

/**
 * Maps ThemeTokens fields to their CSS custom property names.
 * Order matches the @theme block in index.css for consistency.
 */
const TOKEN_TO_VAR: Record<keyof ThemeTokens, string> = {
  // Surfaces
  surface: '--color-surface',
  surfaceRaised: '--color-surface-raised',
  surfaceAlt: '--color-surface-alt',
  surfaceHover: '--color-surface-hover',
  surfaceGlass: '--color-surface-glass',

  // Borders
  border: '--color-border',
  borderSubtle: '--color-border-subtle',
  borderFocus: '--color-border-focus',

  // Text
  text: '--color-text',
  textSecondary: '--color-text-secondary',
  textMuted: '--color-text-muted',

  // Brand
  primary: '--color-primary',
  primaryHover: '--color-primary-hover',
  primaryGlow: '--color-primary-glow',
  primaryRgb: '--primary-rgb',
  accent: '--color-accent',
  accentGlow: '--color-accent-glow',
  accentRgb: '--accent-rgb',

  // Status
  danger: '--color-danger',
  dangerSoft: '--color-danger-soft',
  success: '--color-success',
  successSoft: '--color-success-soft',
  warning: '--color-warning',
  warningSoft: '--color-warning-soft',

  // Shadows
  shadowSm: '--shadow-sm',
  shadowMd: '--shadow-md',
  shadowLg: '--shadow-lg',
  shadowGlow: '--shadow-glow',
  shadowGlowLg: '--shadow-glow-lg',

  // Typography (optional)
  fontFamily: '--font-family',
};

/** Default body font — used when a theme doesn't specify fontFamily. */
const DEFAULT_FONT = "'Inter', system-ui, -apple-system, BlinkMacSystemFont, sans-serif";

/**
 * Apply all theme tokens as CSS custom properties on :root.
 * Tailwind v4 @theme block serves as the build-time default;
 * these runtime setProperty calls override them via CSS cascade.
 */
export function applyThemeTokens(tokens: ThemeTokens): void {
  const style = document.documentElement.style;
  for (const [key, varName] of Object.entries(TOKEN_TO_VAR)) {
    const value = tokens[key as keyof ThemeTokens];
    if (value != null) {
      style.setProperty(varName, value);
    }
  }
  // Reset font to default if not specified by the theme
  style.setProperty('--font-family', tokens.fontFamily || DEFAULT_FONT);
}

/**
 * Set the data-theme attribute on <html> for CSS selectors.
 */
export function setThemeAttr(themeId: ThemeId): void {
  document.documentElement.dataset.theme = themeId;
}

/**
 * Update <meta name="theme-color"> for browser chrome.
 */
export function updateMetaThemeColor(bgColor: string): void {
  const meta = document.querySelector('meta[name="theme-color"]');
  if (meta) {
    meta.setAttribute('content', bgColor);
  }
}
