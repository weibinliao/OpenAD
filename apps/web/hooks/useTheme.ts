import { useEffect, useState } from 'react';

export type ThemeMode = 'light' | 'dark' | 'midnight-slate' | 'cyberpunk' | 'neon-aurora' | 'system';
export type AppliedTheme = 'light' | 'dark' | 'midnight-slate' | 'cyberpunk' | 'neon-aurora';

const STORAGE_KEY = 'fsa.theme';

function resolveTheme(theme: ThemeMode): AppliedTheme {
  if (theme === 'system') {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }
  // Redesign supports light/dark only; retired decorative themes resolve to dark.
  if (theme === 'midnight-slate' || theme === 'cyberpunk' || theme === 'neon-aurora') {
    return 'dark';
  }
  return theme;
}

export function useTheme() {
  const [theme, setTheme] = useState<ThemeMode>('dark');
  const [resolvedTheme, setResolvedTheme] = useState<AppliedTheme>('dark');

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }

    const savedTheme = window.localStorage.getItem(STORAGE_KEY) as ThemeMode | null;
    if (savedTheme && ['light', 'dark', 'midnight-slate', 'cyberpunk', 'neon-aurora', 'system'].includes(savedTheme)) {
      setTheme(savedTheme);
      return;
    }

    // Dark-first: the product's default face is the dark ops console.
    setTheme('dark');
  }, []);

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }

    const applyTheme = () => {
      const nextResolvedTheme = resolveTheme(theme);
      setResolvedTheme(nextResolvedTheme);
      document.documentElement.dataset.theme = nextResolvedTheme;
      document.documentElement.classList.toggle('dark', nextResolvedTheme === 'dark');
    };

    applyTheme();

    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
    const handleChange = () => {
      if (theme === 'system') {
        applyTheme();
      }
    };

    mediaQuery.addEventListener('change', handleChange);
    return () => mediaQuery.removeEventListener('change', handleChange);
  }, [theme]);

  const setThemeAndSave = (nextTheme: ThemeMode) => {
    setTheme(nextTheme);
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(STORAGE_KEY, nextTheme);
    }
  };

  return {
    theme,
    resolvedTheme,
    setTheme: setThemeAndSave,
  };
}
