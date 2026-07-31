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
const mockOnOpenTemplates = jest.fn();

function createFetchResponse(payload: unknown) {
  return {
    ok: true,
    json: async () => payload,
  } as Response;
}

function renderConsole(path = '') {
  return render(
    <I18nProvider>
      <ADConnectionProvider>
        <ScanExplorerConsole
          locale="en"
          path={path}
          loading={false}
          adReady={false}
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
          onScan={mockOnScan}
          onOpenTemplates={mockOnOpenTemplates}
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
    mockOnOpenTemplates.mockClear();
    mockRouterPush.mockClear();

    global.fetch = jest.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes('/api/fs/directories?path=')) {
        return Promise.resolve(
          createFetchResponse({
            path: '\\\\server\\share',
            parent: '\\\\server',
            items: [],
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

  test('still starts a local scan on Enter', async () => {
    renderConsole();

    await waitFor(() => expect(global.fetch).toHaveBeenCalled());

    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: 'C:\\Data' },
    });

    await act(async () => {
      fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter', code: 'Enter' });
    });

    await waitFor(() => expect(mockOnScan).toHaveBeenCalledWith('C:\\Data'));
  });

  test('routes the AD prerequisite action to system settings', async () => {
    renderConsole('\\\\server\\share');

    fireEvent.click(await screen.findByRole('button', { name: 'Connect AD' }));

    expect(mockRouterPush).toHaveBeenCalledWith('/settings');
  });
});
