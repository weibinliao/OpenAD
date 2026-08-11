import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import HistoryPage from '../../pages/history';

const sessionRows = [
  {
    id: 'completed-session',
    root_path: 'C:\\Completed',
    status: 'completed',
    max_depth: 3,
    include_inherited: true,
    items_scanned: 10,
    permission_count: 4,
    started_at: '2026-08-08T08:00:00Z',
  },
  {
    id: 'failed-session',
    root_path: 'C:\\Failed',
    status: 'failed',
    max_depth: 1,
    include_inherited: false,
    items_scanned: 2,
    permission_count: 0,
    started_at: '2026-08-08T07:00:00Z',
  },
  {
    id: 'running-session',
    root_path: 'C:\\Running',
    status: 'running',
    max_depth: -1,
    include_inherited: true,
    items_scanned: 1,
    permission_count: 1,
    started_at: '2026-08-08T06:00:00Z',
  },
];

jest.mock('next/router', () => ({
  useRouter: () => ({ isReady: true, query: {}, push: jest.fn() }),
}));

jest.mock('../../contexts/I18nContext', () => ({
  useI18n: () => ({ locale: 'en', t: (key: string) => key }),
}));

describe('History page deletion controls', () => {
  beforeEach(() => {
    jest.restoreAllMocks();
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ items: sessionRows, pagination: { total: sessionRows.length } }),
    }) as jest.MockedFunction<typeof fetch>;
    jest.spyOn(window, 'confirm').mockReturnValue(true);
  });

  test('deletes selected terminal sessions and keeps running sessions protected', async () => {
    render(<HistoryPage />);

    await waitFor(() => expect(screen.getByText('C:\\Completed')).toBeInTheDocument());
    const runningCheckbox = screen.getByRole('checkbox', { name: 'Select C:\\Running' });
    expect(runningCheckbox).toBeDisabled();

    fireEvent.click(screen.getByRole('checkbox', { name: 'Select C:\\Completed' }));
    fireEvent.click(screen.getByRole('button', { name: 'Delete selected (1)' }));

    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining('/api/sessions'),
      expect.objectContaining({
        method: 'DELETE',
        body: JSON.stringify({ session_ids: ['completed-session'] }),
      }),
    ));
    await waitFor(() => expect(screen.queryByText('C:\\Completed')).not.toBeInTheDocument());
    expect(global.fetch).toHaveBeenCalledTimes(2);
  });

  test('offers a separate clear-finished-history action', async () => {
    render(<HistoryPage />);

    await waitFor(() => expect(screen.getByRole('button', { name: 'Clear finished history' })).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: 'Clear finished history' }));

    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining('/api/sessions'),
      expect.objectContaining({
        method: 'DELETE',
        body: JSON.stringify({ all_terminal: true }),
      }),
    ));
  });
});
