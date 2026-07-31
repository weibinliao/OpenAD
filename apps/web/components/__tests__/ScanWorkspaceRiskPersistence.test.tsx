import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import ScanWorkspacePage from '../../pages/scan-workspace';

const mockUpsertRiskFindings = jest.fn();

class StubWebSocket {
  onopen: (() => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: (() => void) | null = null;
  onclose: (() => void) | null = null;

  close() {}
}

jest.mock('next/router', () => ({
  useRouter: () => ({ query: {}, push: jest.fn() }),
}));

jest.mock('../../contexts/I18nContext', () => ({
  useI18n: () => ({ locale: 'en', t: (key: string) => key }),
}));

jest.mock('../../contexts/ADConnectionContext', () => ({
  useADConnection: () => ({
    activeProfile: null,
    config: { adServer: '', baseDN: '', username: '', password: '' },
    connection: { connected: false },
  }),
}));

jest.mock('../../lib/riskFindings', () => ({
  upsertRiskFindingsFromScan: (...args: unknown[]) => mockUpsertRiskFindings(...args),
}));

jest.mock('../../components/ScanExplorerConsole', () => function MockScanExplorerConsole(props: {
  progressStatus: string;
  message: string;
  error: string;
  onScan: (path: string) => void;
}) {
  return (
    <div>
      <span data-testid="scan-status">{props.progressStatus}</span>
      <span data-testid="scan-message">{props.message}</span>
      <span data-testid="scan-error">{props.error}</span>
      <button type="button" onClick={() => props.onScan('C:\\Data')}>Start test scan</button>
    </div>
  );
});

jest.mock('../../components/ScanCompletionSummary', () => function MockScanCompletionSummary() {
  return <div data-testid="completion-summary">Completed scan summary</div>;
});

describe('Scan Workspace risk persistence boundary', () => {
  beforeEach(() => {
    global.WebSocket = StubWebSocket as unknown as typeof WebSocket;
    mockUpsertRiskFindings.mockReset();
    mockUpsertRiskFindings.mockImplementation(() => {
      throw new DOMException(
        "Failed to execute 'setItem' on 'Storage': Setting the value exceeded the quota.",
        'QuotaExceededError',
      );
    });
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        session_id: 'session-1',
        root_path: 'C:\\Data',
        status: 'completed',
        items_scanned: 1,
        permission_count: 1,
        permissions: [{
          path: 'C:\\Data',
          trustee: 'Everyone',
          trustee_sid: 'S-1-1-0',
          rights: 'Modify',
          type: 'Allow',
          inherited: false,
          source: 'Explicit',
        }],
        skipped: [],
      }),
    }) as jest.MockedFunction<typeof fetch>;
  });

  test('keeps a completed scan completed when risk persistence fails', async () => {
    render(<ScanWorkspacePage />);

    fireEvent.click(screen.getByRole('button', { name: 'Start test scan' }));

    await waitFor(() => expect(screen.getByTestId('scan-status')).toHaveTextContent('completed'));
    expect(screen.getByTestId('completion-summary')).toBeInTheDocument();
    expect(screen.getByTestId('scan-error')).toBeEmptyDOMElement();
    expect(screen.getByTestId('scan-message')).toHaveTextContent('Scan completed, but risk findings could not be saved.');
    expect(mockUpsertRiskFindings).toHaveBeenCalledWith(expect.objectContaining({
      sessionID: 'session-1',
    }));
  });
});
