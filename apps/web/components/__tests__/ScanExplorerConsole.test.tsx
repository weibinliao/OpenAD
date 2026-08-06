import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';
import ScanExplorerConsole from '../ScanExplorerConsole';
import { ADConnectionProvider } from '../../contexts/ADConnectionContext';
import { I18nProvider } from '../../contexts/I18nContext';

jest.mock('next/router', () => ({
  useRouter: () => ({
    push: mockRouterPush,
    pathname: '/scan-workspace',
  }),
}));

const mockRouterPush = jest.fn();
const mockOnPathChange = jest.fn();
const mockOnScan = jest.fn();
const mockOnDepthChange = jest.fn();
const mockOnIncludeInheritedChange = jest.fn();
let mockProfiles: Array<Record<string, unknown>> = [];

function createFetchResponse(payload: unknown, ok = true, status = ok ? 200 : 500) {
  return {
    ok,
    status,
    json: async () => payload,
  } as Response;
}

function renderConsole(path = '', options: { adReady?: boolean } = {}) {
  return render(
    <I18nProvider>
      <ADConnectionProvider>
        <ScanExplorerConsole
          locale="en"
          path={path}
          depth={2}
          includeInherited={true}
          loading={false}
          adReady={options.adReady ?? false}
          itemsScanned={0}
          permissionCount={0}
          resultCount={0}
          skippedCount={0}
          sessionID=""
          progressStatus="idle"
          progressCurrentPath=""
          progressStartedAt={0}
          progressLastUpdatedAt={0}
          progressLiveConnected={false}
          progressTrackedPath=""
          message=""
          error=""
          onPathChange={mockOnPathChange}
          onDepthChange={mockOnDepthChange}
          onIncludeInheritedChange={mockOnIncludeInheritedChange}
          onScan={mockOnScan}
        />
      </ADConnectionProvider>
    </I18nProvider>
  );
}

describe('ScanExplorerConsole', () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    mockOnPathChange.mockClear();
    mockOnScan.mockClear();
    mockOnDepthChange.mockClear();
    mockOnIncludeInheritedChange.mockClear();
    mockRouterPush.mockClear();
    mockProfiles = [];
    window.localStorage.clear();

    global.fetch = jest.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes('/api/ad/connections')) {
        return Promise.resolve(
          createFetchResponse({
            items: mockProfiles,
            count: mockProfiles.length,
          })
        );
      }

      if (url.includes('/api/fs/directories')) {
        const parsed = new URL(url);
        const requestedPath = parsed.searchParams.get('path') || '';

        if (requestedPath === '\\\\server') {
          return Promise.resolve(
            createFetchResponse({
              path: '\\\\server',
              parent: '',
              items: [
                { name: 'IT_dept', path: '\\\\server\\IT_dept' },
                { name: 'software', path: '\\\\server\\software' },
              ],
            })
          );
        }

        return Promise.resolve(
          createFetchResponse({
            path: '\\\\server\\share',
            parent: '\\\\server',
            items: [{ name: 'Team', path: '\\\\server\\share\\Team' }],
          })
        );
      }

      if (url.includes('/api/system/runtime-identity')) {
        return Promise.resolve(
          createFetchResponse({
            account_name: 'EXAMPLE\\operator',
            username: 'operator',
            domain: 'EXAMPLE',
            host: 'OPENAD-HOST',
            goos: 'windows',
          })
        );
      }

      return Promise.resolve(
        createFetchResponse({
          path: '',
          items: [{ name: 'C:', path: 'C:\\' }],
        })
      );
    }) as jest.MockedFunction<typeof fetch>;
  });

  afterAll(() => {
    global.fetch = originalFetch;
  });

  test('keeps directory browsing and runtime state without duplicate mission cards', async () => {
    renderConsole();

    await waitFor(() => expect(global.fetch).toHaveBeenCalled());

    expect(screen.getByText('Directory Browser')).toBeInTheDocument();
    expect(screen.getByText('Runtime')).toBeInTheDocument();
    expect(screen.queryByText('Current mission')).not.toBeInTheDocument();
    expect(screen.queryByText('Execution target')).not.toBeInTheDocument();
  });

  test('reveals a typed UNC path on Enter without starting the scan', async () => {
    renderConsole();

    await waitFor(() => expect(global.fetch).toHaveBeenCalled());

    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: '\\\\server\\share' },
    });

    await act(async () => {
      fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter', code: 'Enter' });
    });

    await waitFor(() => expect(mockOnPathChange).toHaveBeenCalledWith('\\\\server\\share'));
    expect(mockOnScan).not.toHaveBeenCalled();
  });

  test('shows the Windows credential boundary as soon as a UNC path is typed', async () => {
    renderConsole();

    await waitFor(() => expect(global.fetch).toHaveBeenCalled());

    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: '\\\\server\\share' },
    });

    const notice = await screen.findByRole('status', { name: 'UNC credential boundary' });
    expect(notice).toHaveTextContent('UNC access requires local Windows network credentials');
    expect(notice).toHaveTextContent('EXAMPLE\\operator');
    expect(notice).toHaveTextContent('Please make sure this Windows account can open the share in File Explorer');
    expect(global.fetch).toHaveBeenCalledWith(expect.stringContaining('/api/system/runtime-identity'));
  });

  test('discovers shares from a bare UNC server root without starting a scan', async () => {
    renderConsole();

    await waitFor(() => expect(global.fetch).toHaveBeenCalled());

    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: '\\\\server' },
    });

    expect(await screen.findByRole('button', { name: 'Discover Shares' })).toBeInTheDocument();

    await act(async () => {
      fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter', code: 'Enter' });
    });

    await waitFor(() => expect(mockOnPathChange).toHaveBeenCalledWith('\\\\server'));
    expect(mockOnScan).not.toHaveBeenCalled();
    expect(await screen.findByText('IT_dept')).toBeInTheDocument();
    expect(await screen.findByText('software')).toBeInTheDocument();
  });

  test('starts a UNC scan with the current typed path when Start Scan is clicked', async () => {
    renderConsole();

    await waitFor(() => expect(global.fetch).toHaveBeenCalled());

    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: '\\\\server\\share' },
    });

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Start Scan' }));
    });

    await waitFor(() => expect(mockOnPathChange).toHaveBeenCalledWith('\\\\server\\share'));
    expect(mockOnScan).toHaveBeenCalledWith('\\\\server\\share');
  });

  test('keeps UNC browsing and scan launch on local Windows credentials even when AD is configured', async () => {
    mockProfiles = [{
      id: 'profile-123',
      name: 'Primary DC',
      server: 'ldap://dc01.example.com',
      base_dn: 'DC=example,DC=com',
      bind_user: 'EXAMPLE\\scanner',
      is_default: true,
      last_test_ok: true,
      last_tested_at: '2026-08-06T01:00:00Z',
      created_at: '2026-08-06T01:00:00Z',
      updated_at: '2026-08-06T01:00:00Z',
    }];
    renderConsole('', { adReady: true });

    await waitFor(() => expect(global.fetch).toHaveBeenCalled());
    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: '\\\\server' },
    });

    await act(async () => {
      fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter', code: 'Enter' });
    });

    await waitFor(() => expect(mockOnPathChange).toHaveBeenCalledWith('\\\\server'));
    const directoryCalls = (global.fetch as jest.MockedFunction<typeof fetch>).mock.calls
      .map(([input]) => String(input))
      .filter((url) => url.includes('/api/fs/directories'));
    const serverDiscoveryURL = directoryCalls.find((url) => {
      const parsed = new URL(url);
      return parsed.searchParams.get('path') === '\\\\server';
    });
    expect(serverDiscoveryURL).toBeTruthy();
    expect(new URL(serverDiscoveryURL as string).searchParams.has('unc_access_mode')).toBe(false);
    expect(new URL(serverDiscoveryURL as string).searchParams.has('unc_access_connection_id')).toBe(false);
    expect(screen.queryByLabelText('Use saved AD connection for UNC access')).not.toBeInTheDocument();

    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: '\\\\server\\share' },
    });

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Start Scan' }));
    });

    const lastScanCall = mockOnScan.mock.calls[mockOnScan.mock.calls.length - 1];
    expect(lastScanCall).toEqual(['\\\\server\\share']);
  });

  test('reveals a local path on Enter without starting the scan', async () => {
    renderConsole();

    await waitFor(() => expect(global.fetch).toHaveBeenCalled());

    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: 'C:\\Data' },
    });

    await act(async () => {
      fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter', code: 'Enter' });
    });

    await waitFor(() => expect(mockOnPathChange).toHaveBeenCalledWith('C:\\Data'));
    expect(mockOnScan).not.toHaveBeenCalled();
  });

  test('routes the AD prerequisite action to system settings', async () => {
    renderConsole('\\\\server\\share');

    fireEvent.click(await screen.findByRole('button', { name: 'Connect AD' }));

    expect(mockRouterPush).toHaveBeenCalledWith('/settings');
  });
});
