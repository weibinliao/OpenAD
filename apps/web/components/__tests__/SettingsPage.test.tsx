import type { ReactNode } from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';
import SettingsPage from '../../pages/settings';
import {
  WORKSPACE_SETTINGS_KEY,
  defaultWorkspaceSettings,
} from '../../lib/workspaceSettings';
import type { ConnectionProfile } from '../../contexts/ADConnectionContext';

jest.mock('next/head', () => ({
  __esModule: true,
  default: function MockHead({ children }: { children: ReactNode }) {
    return <>{children}</>;
  },
}));

jest.mock('next/router', () => ({
  useRouter: () => ({
    push: jest.fn(),
    pathname: '/settings',
  }),
}));

const setLocaleMock = jest.fn();
jest.mock('../../contexts/I18nContext', () => ({
  useI18n: () => ({
    locale: 'en',
    setLocale: setLocaleMock,
    t: (key: string) => {
      const labels: Record<string, string> = {
        appTitle: 'OpenAD',
      };

      return labels[key] || key;
    },
  }),
}));

const setThemeMock = jest.fn();
jest.mock('../../contexts/ThemeContext', () => ({
  useThemeContext: () => ({
    theme: 'dark',
    resolvedTheme: 'dark',
    setTheme: setThemeMock,
  }),
}));

const storedProfiles: ConnectionProfile[] = [
  {
    id: 'profile-1',
    name: 'Lab DC',
    server: 'ldap://dc01.lab.local:389',
    base_dn: 'DC=lab,DC=local',
    bind_user: 'LAB\\svc-scan',
    is_default: true,
    last_tested_at: '2026-07-01T10:00:00Z',
    last_test_ok: true,
    created_at: '2026-06-01T00:00:00Z',
    updated_at: '2026-07-01T10:00:00Z',
  },
  {
    id: 'profile-2',
    name: 'Staging DC',
    server: 'ldap://dc02.lab.local:389',
    base_dn: 'DC=staging,DC=local',
    bind_user: 'LAB\\svc-stage',
    is_default: false,
    last_tested_at: null,
    last_test_ok: null,
    created_at: '2026-06-15T00:00:00Z',
    updated_at: '2026-06-15T00:00:00Z',
  },
];

const refreshProfilesMock = jest.fn().mockResolvedValue(undefined);
const setActiveProfileIdMock = jest.fn();
const adConnectionState = {
  profiles: storedProfiles,
  profilesLoading: false,
  profilesOffline: false,
};

jest.mock('../../contexts/ADConnectionContext', () => ({
  useADConnection: () => ({
    config: { adServer: '', baseDN: '', username: '', password: '' },
    connection: { connected: false, message: '', testedAt: null },
    saveConnection: jest.fn(),
    clearConnection: jest.fn(),
    profiles: adConnectionState.profiles,
    profilesLoading: adConnectionState.profilesLoading,
    profilesOffline: adConnectionState.profilesOffline,
    refreshProfiles: refreshProfilesMock,
    activeProfileId: 'profile-1',
    setActiveProfileId: setActiveProfileIdMock,
    activeProfile: adConnectionState.profiles[0] || null,
  }),
}));

describe('SettingsPage', () => {
  const savedWorkspaceSettings = {
    ...defaultWorkspaceSettings,
    workspaceLabel: 'Saved Workspace',
    workspaceTagline: 'Saved tagline',
    workspaceBadgeStyle: 'shield' as const,
  };

  beforeEach(() => {
    window.localStorage.clear();
    window.localStorage.setItem(WORKSPACE_SETTINGS_KEY, JSON.stringify(savedWorkspaceSettings));
    adConnectionState.profiles = storedProfiles;
    adConnectionState.profilesOffline = false;

    global.fetch = jest.fn(() =>
      Promise.resolve({
        ok: true,
        json: async () => ({}),
      } as Response)
    ) as jest.Mock;
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('renders stored AD connection profiles with default, active and test-status badges', async () => {
    render(<SettingsPage />);

    expect(await screen.findByText('Lab DC')).toBeInTheDocument();
    expect(screen.getByText('Staging DC')).toBeInTheDocument();
    expect(screen.getByText('Default')).toBeInTheDocument();
    expect(screen.getByText('OK')).toBeInTheDocument();
    expect(screen.getByText('Never tested')).toBeInTheDocument();
    // Active profile shows the "In use" badge on its row.
    expect(screen.getAllByText('In use').length).toBeGreaterThan(0);
  });

  test('selects a profile as active via the Use action', async () => {
    render(<SettingsPage />);

    const useButton = await screen.findByRole('button', { name: 'Use' });
    fireEvent.click(useButton);

    expect(setActiveProfileIdMock).toHaveBeenCalledWith('profile-2');
  });

  test('runs a connection test and shows the inline result', async () => {
    (global.fetch as jest.Mock).mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith('/api/ad/connections/profile-1/test') && init?.method === 'POST') {
        return Promise.resolve({
          ok: true,
          json: async () => ({ ok: true, message: 'Active Directory connection successful' }),
        } as Response);
      }
      return Promise.resolve({ ok: true, json: async () => ({}) } as Response);
    });

    render(<SettingsPage />);

    const testButtons = await screen.findAllByRole('button', { name: 'Test' });
    fireEvent.click(testButtons[0]);

    expect(await screen.findByText('Active Directory connection successful')).toBeInTheDocument();
    expect(refreshProfilesMock).toHaveBeenCalled();
  });

  test('shows the offline warning banner when the connection store is unreachable', async () => {
    adConnectionState.profiles = [];
    adConnectionState.profilesOffline = true;

    render(<SettingsPage />);

    expect(
      await screen.findByText(
        'The backend connection store is unreachable. Stored AD connections are unavailable until the API is back online.'
      )
    ).toBeInTheDocument();
  });

  test('opens the add-connection dialog and validates required fields', async () => {
    render(<SettingsPage />);

    fireEvent.click(await screen.findByRole('button', { name: /Add connection/ }));

    expect(await screen.findByLabelText('Domain account')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    expect(await screen.findByText('Server address and domain account are required.')).toBeInTheDocument();
  });

  test('keeps scan and report configuration out of system settings', async () => {
    render(<SettingsPage />);

    expect(await screen.findByText('Workspace identity')).toBeInTheDocument();
    expect(screen.getByRole('img', { name: 'Workspace logo preview' })).toHaveAttribute(
      'src',
      '/brand/openad.png',
    );
    expect(screen.getByText('Workspace behavior')).toBeInTheDocument();
    expect(screen.queryByText('Scan defaults')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Default depth')).not.toBeInTheDocument();
    expect(screen.queryByText('Report defaults')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Organization')).not.toBeInTheDocument();
  });

  test('keeps workspace identity edits in draft state until save', async () => {
    render(<SettingsPage />);

    const labelInput = await screen.findByDisplayValue('Saved Workspace');
    const saveButton = screen.getByRole('button', { name: 'Save identity' });

    expect(saveButton).toBeDisabled();

    fireEvent.change(labelInput, { target: { value: 'Draft Workspace' } });

    expect(screen.getByDisplayValue('Draft Workspace')).toBeInTheDocument();
    expect(screen.getByText('Unsaved draft')).toBeInTheDocument();
    expect(saveButton).toBeEnabled();
    expect(JSON.parse(window.localStorage.getItem(WORKSPACE_SETTINGS_KEY) || '{}')).toMatchObject({
      workspaceLabel: 'Saved Workspace',
      workspaceTagline: 'Saved tagline',
    });

    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(JSON.parse(window.localStorage.getItem(WORKSPACE_SETTINGS_KEY) || '{}')).toMatchObject({
        workspaceLabel: 'Draft Workspace',
        workspaceTagline: 'Saved tagline',
      });
    });

    expect(screen.getByText('Workspace identity saved')).toBeInTheDocument();
    expect(screen.getByText('Identity synced')).toBeInTheDocument();
  });

  test('reverts unsaved workspace identity edits without changing persisted settings', async () => {
    render(<SettingsPage />);

    const taglineInput = await screen.findByDisplayValue('Saved tagline');

    fireEvent.change(taglineInput, { target: { value: 'Draft tagline' } });
    fireEvent.click(screen.getByRole('button', { name: 'Revert' }));

    await waitFor(() => {
      expect(screen.getByDisplayValue('Saved tagline')).toBeInTheDocument();
    });

    expect(JSON.parse(window.localStorage.getItem(WORKSPACE_SETTINGS_KEY) || '{}')).toMatchObject({
      workspaceLabel: 'Saved Workspace',
      workspaceTagline: 'Saved tagline',
    });
    expect(screen.getByText('Draft reverted')).toBeInTheDocument();
  });
});
