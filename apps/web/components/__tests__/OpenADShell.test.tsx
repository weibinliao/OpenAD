import React from 'react';
import { render, screen } from '@testing-library/react';
import AppShellV2 from '../shell/AppShellV2';

jest.mock('next/router', () => ({
  useRouter: () => ({ pathname: '/identity', push: jest.fn() }),
}));

jest.mock('../../contexts/I18nContext', () => ({
  useI18n: () => ({ locale: 'en', setLocale: jest.fn(), t: (key: string) => key }),
}));

jest.mock('../../contexts/ThemeContext', () => ({
  useThemeContext: () => ({ theme: 'dark', resolvedTheme: 'dark', setTheme: jest.fn() }),
}));

jest.mock('../../contexts/ADConnectionContext', () => ({
  useADConnection: () => ({ connection: { connected: true }, activeProfile: null }),
}));

jest.mock('../../hooks/useRuntimeHealth', () => ({
  useRuntimeHealth: () => 'healthy',
}));

jest.mock('../../lib/i18n', () => ({
  useDict: () => ({
    common: { appName: 'OpenAD' },
    nav: {},
    shell: {
      adConnected: 'AD connected',
      adDisconnected: 'AD disconnected',
      themeLight: 'Light',
      themeDark: 'Dark',
      themeSystem: 'System',
      language: 'Language',
      pinSidebar: 'Pin sidebar open',
      unpinSidebar: 'Return sidebar to hover mode',
      runtimeHealthy: 'Local services healthy',
      runtimeChecking: 'Checking local services',
      runtimeOffline: 'Local API unavailable',
      runtimeTitle: 'Desktop local services',
      runtimeAutomatic: 'The desktop app starts both services automatically.',
      runtimeLocalOnly: 'Bound to this computer only through 127.0.0.1.',
      runtimeApi: 'API service',
      runtimeWeb: 'Desktop web',
      runtimeHelp: 'Close the program using port 18080 or 43110, then retry.',
    },
  }),
}));

describe('OpenAD application shell', () => {
  it('presents modules by operator responsibility', () => {
    render(
      <AppShellV2>
        <div>Directory content</div>
      </AppShellV2>,
    );

    expect(screen.getByText('OpenAD')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Overview:/ })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Directory Explorer/ })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Resource Inventory/ })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Scan Center/ })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Access Explorer/ })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /^Dashboard$/ })).not.toBeInTheDocument();
    expect(screen.getAllByText('Users, groups, OUs and sync')).toHaveLength(2);
  });
});
