import React, { createContext, useContext, type ReactNode } from 'react';
import { useTheme, type AppliedTheme, type ThemeMode } from '../hooks/useTheme';

interface ThemeContextType {
  theme: ThemeMode;
  resolvedTheme: AppliedTheme;
  setTheme: (theme: ThemeMode) => void;
}

const ThemeContext = createContext<ThemeContextType | undefined>(undefined);

export function ThemeProvider({ children }: { children: ReactNode }) {
  const themeState = useTheme();

  return <ThemeContext.Provider value={themeState}>{children}</ThemeContext.Provider>;
}

export function useThemeContext() {
  const context = useContext(ThemeContext);
  if (context === undefined) {
    throw new Error('useThemeContext must be used within a ThemeProvider');
  }
  return context;
}
