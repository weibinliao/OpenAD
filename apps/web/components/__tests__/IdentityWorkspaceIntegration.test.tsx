import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';
import IdentityPage from '../../pages/identity';
import { I18nProvider } from '../../contexts/I18nContext';

jest.mock('next/router', () => ({
  useRouter: () => ({ query: { q: 'alex' } }),
}));

jest.mock('../../contexts/ADConnectionContext', () => ({
  useADConnection: () => ({
    activeProfile: {
      id: 'profile-1',
      name: 'Example DC',
      server: 'dc01.example.com',
      base_dn: 'DC=example,DC=com',
      bind_user: 'EXAMPLE\\operator',
      is_default: true,
      last_tested_at: '2026-07-11T00:00:00Z',
      last_test_ok: true,
      created_at: '2026-07-01T00:00:00Z',
      updated_at: '2026-07-11T00:00:00Z',
    },
  }),
}));

jest.mock('../DirectoryExplorerWorkbench', () => ({
  __esModule: true,
  default: ({ connectionId, initialQuery }: { connectionId: string; initialQuery?: string }) => (
    <div data-testid="directory-explorer-workbench">
      Unified directory workspace: {connectionId} · query: {initialQuery}
    </div>
  ),
}));

describe('IdentityPage desktop workflow', () => {
  beforeEach(() => {
    window.localStorage.setItem('fsa.locale', 'en');
    global.fetch = jest.fn(() => Promise.resolve({
      ok: true,
      json: async () => ({ items: [] }),
    } as Response)) as jest.MockedFunction<typeof fetch>;
  });

  afterEach(() => jest.clearAllMocks());

  test('renders one unified directory workbench without separate user, group, and tree tabs', async () => {
    render(<I18nProvider><IdentityPage /></I18nProvider>);

    const workbench = await screen.findByTestId('directory-explorer-workbench');
    const snapshot = screen.getByText('Directory snapshot');

    expect(workbench).toHaveTextContent('profile-1');
    expect(workbench).toHaveTextContent('query: alex');
    expect(workbench.compareDocumentPosition(snapshot) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(screen.queryAllByRole('tab')).toHaveLength(0);
  });

  test('shows a retry action when the directory snapshot status cannot be loaded', async () => {
    global.fetch = jest.fn().mockRejectedValue(new Error('offline')) as jest.MockedFunction<typeof fetch>;

    render(<I18nProvider><IdentityPage /></I18nProvider>);

    expect(await screen.findByText('Directory snapshot unavailable')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    await waitFor(() => expect(global.fetch).toHaveBeenCalledTimes(2));
  });
});
