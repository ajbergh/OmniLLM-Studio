import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from 'react';
import type { ThemeId, ThemeDefinition } from './tokens';
import { THEMES, THEME_MAP, DEFAULT_THEME_ID } from './themes';
import { applyThemeTokens, setThemeAttr, updateMetaThemeColor } from './cssVars';

const STORAGE_KEY = 'omnillm.theme';

interface ThemeContextValue {
  currentThemeId: ThemeId;
  currentTheme: ThemeDefinition;
  setTheme: (id: ThemeId) => void;
  availableThemes: ThemeDefinition[];
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

function readPersistedThemeId(): ThemeId {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw && raw in THEME_MAP) return raw as ThemeId;
  } catch {
    // localStorage unavailable
  }
  return DEFAULT_THEME_ID;
}

function persistThemeId(id: ThemeId): void {
  try {
    localStorage.setItem(STORAGE_KEY, id);
  } catch {
    // localStorage unavailable
  }
}

function applyTheme(theme: ThemeDefinition): void {
  applyThemeTokens(theme.tokens);
  setThemeAttr(theme.id);
  updateMetaThemeColor(theme.tokens.surface);
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [themeId, setThemeId] = useState<ThemeId>(readPersistedThemeId);

  const currentTheme = THEME_MAP[themeId];

  // Apply theme tokens on mount and when themeId changes
  useEffect(() => {
    applyTheme(THEME_MAP[themeId]);
  }, [themeId]);

  const setTheme = useCallback((id: ThemeId) => {
    if (!(id in THEME_MAP)) return;
    setThemeId(id);
    persistThemeId(id);
  }, []);

  return (
    <ThemeContext.Provider
      value={{
        currentThemeId: themeId,
        currentTheme,
        setTheme,
        availableThemes: THEMES,
      }}
    >
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error('useTheme must be used within ThemeProvider');
  return ctx;
}
