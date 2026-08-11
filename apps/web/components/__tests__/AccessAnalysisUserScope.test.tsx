import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';
import AccessAnalysisPage from '../../pages/access';
import { I18nProvider } from '../../contexts/I18nContext';

jest.mock('next/router', () => ({
  useRouter: () => ({
    isReady: true,
    query: {},
  }),
}));

const response = {
  path_prefix: '\\\\filesrv',
  user: {
    sid: 'S-1-5-21-1-1001',
    sam_account_name: 'alice',
    upn: 'alice@example.test',
    display_name: 'Alice Adams',
    email: '',
    department: '',
    enabled: true,
  },
  group_count: 0,
  groups: [],
  sessions: [{ id: 'session-1', root_path: '\\\\filesrv\\Finance', status: 'completed' }],
  entries: [],
  by_root_path: [],
  counts: { total: 0, direct: 0, via_group: 0, allow: 0, deny: 0 },
};

describe('AccessAnalysisPage by-user server scope', () => {
  beforeEach(() => {
    window.localStorage.setItem('fsa.locale', 'en');
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => response,
    } as Response) as jest.MockedFunction<typeof fetch>;
  });

  afterEach(() => jest.clearAllMocks());

  test('sends an optional UNC server scope and displays it with the consulted share snapshots', async () => {
    render(
      <I18nProvider>
        <AccessAnalysisPage />
      </I18nProvider>,
    );

    fireEvent.change(screen.getByLabelText('SID, account (sAMAccountName) or UPN…'), {
      target: { value: 'alice' },
    });
    fireEvent.change(screen.getByLabelText('Analysis scope'), {
      target: { value: '\\\\filesrv' },
    });
    expect(screen.getByPlaceholderText('Optional scope, e.g. \\\\fs01 or \\\\fs01\\\\Finance')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Analyze' }));

    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining('/api/access/by-user'),
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ principal: 'alice', path_prefix: '\\\\filesrv' }),
      }),
    ));
    expect(await screen.findByText('Scope: \\\\filesrv')).toBeInTheDocument();
    expect(screen.getByText('\\\\filesrv\\Finance')).toBeInTheDocument();
  });
});
