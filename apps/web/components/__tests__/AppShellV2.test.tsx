import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import AppShellV2 from '../shell/AppShellV2';
import { defaultWorkspaceSettings } from '../../lib/workspaceSettings';

const push = jest.fn();
const postMessage = jest.fn();
let pathname = '/dashboard';

jest.mock('next/router', () => ({
  useRouter: () => ({ pathname, push }),
}));

jest.mock('../../contexts/I18nContext', () => ({
  useI18n: () => ({ locale: 'en', setLocale: jest.fn(), t: (key: string) => key }),
}));

jest.mock('../../contexts/ThemeContext', () => ({
  useThemeContext: () => ({ theme: 'dark', resolvedTheme: 'dark', setTheme: jest.fn() }),
}));

jest.mock('../../contexts/ADConnectionContext', () => ({
  useADConnection: () => ({
    connection: { connected: true },
    activeProfile: { id: 'profile-1', name: 'LAB', last_test_ok: true },
  }),
}));

jest.mock('../../hooks/useRuntimeHealth', () => ({
  useRuntimeHealth: () => 'healthy',
}));

jest.mock('../../lib/i18n', () => ({
  useDict: () => ({
    common: { appName: 'OpenAD' },
    explorer: { title: 'Explorer' },
    nav: {
      groupOverview: 'Overview',
      groupDirectory: 'Directory & Resources',
      groupInsight: 'Analysis',
      groupSystem: 'System',
      dashboard: 'Dashboard',
      access: 'Access Analysis',
      risks: 'Risks',
      audit: 'Audit',
      fileActivity: 'File Activity',
      settings: 'Settings',
    },
    shell: {
      adConnected: 'AD connected',
      adDisconnected: 'AD disconnected',
      themeLight: 'Light',
      themeDark: 'Dark',
      themeSystem: 'System',
      language: 'Language',
      expandSidebar: 'Expand',
      collapseSidebar: 'Collapse',
      pinSidebar: 'Pin sidebar open',
      unpinSidebar: 'Collapse sidebar',
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

describe('AppShellV2 desktop workspace', () => {
  beforeEach(() => {
    pathname = '/dashboard';
    window.localStorage.clear();
    push.mockReset();
    postMessage.mockReset();
    global.fetch = jest.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith('/api/ad/users/query')) {
        return Promise.resolve({
          ok: true,
          json: async () => ({
            users: [{
              dn: 'CN=Alice,DC=lab,DC=local',
              username: 'azhang',
              display_name: 'Alice Zhang',
              email: 'alice@lab.local',
              groups: ['CN=Domain Admins,DC=lab,DC=local'],
            }],
          }),
        } as Response);
      }
      if (url.endsWith('/api/ad/groups/query')) {
        return Promise.resolve({ ok: true, json: async () => ({ groups: [] }) } as Response);
      }
      return Promise.resolve({ ok: true, json: async () => ({}) } as Response);
    }) as jest.MockedFunction<typeof fetch>;
    Object.defineProperty(window, 'chrome', {
      configurable: true,
      value: { webview: { postMessage } },
    });
  });

  it('uses shared native chrome, a compact navigation rail, and a floating command surface', () => {
    const { container } = render(
      <AppShellV2>
        <div>Dashboard content</div>
      </AppShellV2>,
    );

    expect(container.querySelector('.titlebar')).not.toBeNull();
    expect(container.querySelector('.pp-nav-rail')).not.toBeNull();
    expect(container.querySelector('.pp-command-island')).not.toBeNull();
  });

  it('applies the saved table density to the entire workspace', () => {
    const { container, rerender } = render(
      <AppShellV2 workspaceSettings={{ ...defaultWorkspaceSettings, compactTables: true }}>
        <div>Dashboard content</div>
      </AppShellV2>,
    );

    expect(container.querySelector('.pp-workspace-content')).toHaveClass('is-compact-tables');

    rerender(
      <AppShellV2 workspaceSettings={{ ...defaultWorkspaceSettings, compactTables: false }}>
        <div>Dashboard content</div>
      </AppShellV2>,
    );

    expect(container.querySelector('.pp-workspace-content')).not.toHaveClass('is-compact-tables');
  });

  it('keeps product identity in the window title without reserving a redundant sidebar header', () => {
    const { container } = render(
      <AppShellV2>
        <div>Dashboard content</div>
      </AppShellV2>,
    );

    expect(container.querySelector('.win-title')).toHaveTextContent('OpenAD');
    expect(container.querySelector('.win-title')).not.toHaveTextContent('Overview');
    expect(container.querySelector('.pp-rail-header')).toBeNull();
    expect(container.querySelector('.pp-rail-brand')).toBeNull();
    expect(container.querySelector('.pp-rail-link[href="/"]')).toBeInTheDocument();
  });

  it('temporarily expands on hover and collapses after pointer leave', () => {
    const { container } = render(
      <AppShellV2>
        <div>Dashboard content</div>
      </AppShellV2>,
    );
    const slot = container.querySelector('.pp-nav-slot') as HTMLElement;
    const rail = container.querySelector('.pp-nav-rail') as HTMLElement;

    expect(slot).not.toHaveClass('is-expanded');
    expect(rail).not.toHaveClass('is-expanded');
    expect(screen.queryByText('API 18080 \u00b7 WEB 43110')).not.toBeInTheDocument();
    fireEvent.pointerEnter(slot);
    expect(slot).toHaveClass('is-expanded');
    expect(rail).toHaveClass('is-expanded');
    expect(screen.getByText('API 18080 \u00b7 WEB 43110')).toBeInTheDocument();
    fireEvent.pointerLeave(slot);
    expect(slot).not.toHaveClass('is-expanded');
    expect(rail).not.toHaveClass('is-expanded');
    expect(screen.queryByText('API 18080 \u00b7 WEB 43110')).not.toBeInTheDocument();
  });

  it('returns workspace content to the top when the active route changes', () => {
    const { container, rerender } = render(
      <AppShellV2>
        <div>Dashboard content</div>
      </AppShellV2>,
    );
    const content = container.querySelector('.pp-workspace-content') as HTMLElement;
    content.scrollTop = 236;

    pathname = '/identity';
    rerender(
      <AppShellV2>
        <div>Identity content</div>
      </AppShellV2>,
    );

    expect(content.scrollTop).toBe(0);
  });

  it('pins the rail and stores the preference', () => {
    const { container } = render(
      <AppShellV2>
        <div>Dashboard content</div>
      </AppShellV2>,
    );

    fireEvent.pointerEnter(container.querySelector('.pp-nav-slot') as HTMLElement);
    fireEvent.click(screen.getByRole('button', { name: 'Pin sidebar open' }));

    expect(container.querySelector('.pp-nav-slot')).toHaveClass('is-pinned');
    expect(container.querySelector('.pp-nav-slot')).toHaveClass('is-expanded');
    expect(screen.getByText('API 18080 \u00b7 WEB 43110')).toBeInTheDocument();
    expect(window.localStorage.getItem('permission-protector.desktop-nav-pinned')).toBe('true');

    fireEvent.click(screen.getByRole('button', { name: 'Collapse sidebar' }));
    fireEvent.pointerLeave(container.querySelector('.pp-nav-slot') as HTMLElement);
    expect(container.querySelector('.pp-nav-slot')).not.toHaveClass('is-expanded');
    expect(window.localStorage.getItem('permission-protector.desktop-nav-pinned')).toBe('false');
  });

  it('shows the automatic local-service details', async () => {
    render(
      <AppShellV2>
        <div>Dashboard content</div>
      </AppShellV2>,
    );

    fireEvent.keyDown(screen.getByRole('button', { name: 'Local services healthy' }), { key: 'Enter' });

    expect(await screen.findByText('http://127.0.0.1:18080')).toBeInTheDocument();
    expect(screen.getByText('http://127.0.0.1:43110')).toBeInTheDocument();
    expect(screen.getByText('The desktop app starts both services automatically.')).toBeInTheDocument();
  });

  it('runs the global search from the explicit search action', () => {
    render(
      <AppShellV2>
        <div>Dashboard content</div>
      </AppShellV2>,
    );

    fireEvent.change(screen.getByRole('combobox', { name: 'Global workspace search' }), {
      target: { value: 'alex' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Run global search' }));

    expect(push).toHaveBeenCalledWith('/identity?q=alex');
  });

  it('treats slash-prefixed module names as commands instead of resource paths', () => {
    render(
      <AppShellV2>
        <div>Dashboard content</div>
      </AppShellV2>,
    );

    fireEvent.change(screen.getByRole('combobox', { name: 'Global workspace search' }), {
      target: { value: '/reports' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Run global search' }));

    expect(push).toHaveBeenCalledWith('/reports');
  });

  it('keeps unknown module commands in the command menu instead of opening access analysis', () => {
    render(
      <AppShellV2>
        <div>Dashboard content</div>
      </AppShellV2>,
    );

    fireEvent.change(screen.getByRole('combobox', { name: 'Global workspace search' }), {
      target: { value: '/does-not-exist' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Run global search' }));

    expect(push).not.toHaveBeenCalled();
    expect(screen.getByText('Unknown module command')).toBeInTheDocument();
  });

  it('offers AD autocomplete from the global search and routes the selected user', async () => {
    render(
      <AppShellV2>
        <div>Dashboard content</div>
      </AppShellV2>,
    );

    fireEvent.change(screen.getByRole('combobox', { name: 'Global workspace search' }), {
      target: { value: 'ali' },
    });

    fireEvent.click(await screen.findByRole('option', { name: /Alice Zhang/ }));

    expect(push).toHaveBeenCalledWith('/identity?q=azhang');
  });

  it('uses the saved workspace logo in the shell and synchronizes it to the desktop host', async () => {
    const logoDataUrl = 'data:image/png;base64,ZmFrZS1sb2dv';
    const { container } = render(
      <AppShellV2
        workspaceSettings={{
          ...defaultWorkspaceSettings,
          workspaceLabel: 'Contoso Security',
          workspaceLogoDataUrl: logoDataUrl,
          workspaceLogoMimeType: 'image/png',
          workspaceLogoFileName: 'contoso.png',
        }}
      >
        <div>Dashboard content</div>
      </AppShellV2>,
    );

    expect(container.querySelector('.pp-rail-header')).toBeNull();
    expect(container.querySelector('.win-title img')).toHaveAttribute('src', logoDataUrl);
    expect(screen.queryByText('OA')).not.toBeInTheDocument();
    await waitFor(() => {
      expect(postMessage).toHaveBeenCalledWith({
        type: 'permission-protector-branding',
        logoDataUrl,
      });
    });
  });
});
