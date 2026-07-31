import React from 'react';
import { existsSync } from 'fs';
import { resolve } from 'path';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';

const push = jest.fn();
const replace = jest.fn();

jest.mock('next/router', () => ({
  useRouter: () => ({
    query: {
      mode: 'owner',
      session: 'session-1',
      path: '\\\\fs01\\Finance',
    },
    isReady: true,
    pathname: '/reports',
    push,
    replace,
  }),
}));

jest.mock('../../contexts/I18nContext', () => ({
  useI18n: () => ({ locale: 'en', t: (key: string) => key }),
}));

jest.mock('../../contexts/ADConnectionContext', () => ({
  useADConnection: () => ({
    connection: { connected: true },
    activeProfile: { id: 'profile-1', name: 'Production AD' },
    config: { adServer: '', baseDN: '', username: '', password: '' },
  }),
}));

describe('ReportCenterWorkspace', () => {
  const createObjectURL = jest.fn(() => 'blob:report');
  const revokeObjectURL = jest.fn();

  beforeEach(() => {
    push.mockReset();
    replace.mockReset();
    createObjectURL.mockClear();
    revokeObjectURL.mockClear();
    window.localStorage.clear();
    Object.defineProperty(window.URL, 'createObjectURL', {
      configurable: true,
      value: createObjectURL,
    });
    Object.defineProperty(window.URL, 'revokeObjectURL', {
      configurable: true,
      value: revokeObjectURL,
    });
    global.fetch = jest.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes('/api/sessions?')) {
        return {
          ok: true,
          json: async () => ({
            items: [{
              id: 'session-1',
              root_path: '\\\\fs01\\Finance',
              status: 'completed',
              items_scanned: 18,
              permission_count: 1,
              started_at: '2026-07-11T01:00:00.000Z',
              finished_at: '2026-07-11T01:01:00.000Z',
            }],
          }),
        } as Response;
      }
      if (url.includes('/api/sessions/session-1/bundle')) {
        return {
          ok: true,
          json: async () => ({
            session: {
              id: 'session-1',
              root_path: '\\\\fs01\\Finance',
              status: 'completed',
              items_scanned: 18,
              permission_count: 1,
              started_at: '2026-07-11T01:00:00.000Z',
            },
            permissions: [{
              path: '\\\\fs01\\Finance',
              trustee: 'EXAMPLE\\alice',
              trustee_sid: 'S-1-5-21-1',
              rights: 'Modify',
              type: 'Allow',
              inherited: false,
              risk_level: 'high',
              account_name: 'alice',
              department: 'Finance',
            }],
            identity_resolution: {
              mode: 'snapshot',
              resolved_principal_count: 1,
              unresolved_principal_count: 0,
            },
          }),
        } as Response;
      }
      if (url.includes('/api/export/summary/templates')) {
        return { ok: true, json: async () => ({ items: [] }) } as Response;
      }
      if (url.includes('/api/export/summary')) {
        return {
          ok: true,
          json: async () => ({
            title: 'Finance Permission Report',
            markdown: '# Finance Permission Report',
            sections: ['metadata', 'kpis'],
            template: 'management',
            template_name: 'Management Summary',
          }),
        } as Response;
      }
      if (url.includes('/api/export/download')) {
        return {
          ok: true,
          blob: async () => new Blob(['report']),
        } as Response;
      }
      throw new Error('Unexpected request: ' + url);
    }) as jest.Mock;
  });

  it('loads a selected scan session and keeps report tasks inside a compact tabbed workbench', async () => {
    const workspacePath = resolve(process.cwd(), 'components/ReportCenterWorkspace.tsx');
    expect(existsSync(workspacePath)).toBe(true);
    if (!existsSync(workspacePath)) return;
    const ReportCenterWorkspace = require('../ReportCenterWorkspace').default;

    render(<ReportCenterWorkspace />);

    expect(screen.getByRole('heading', { name: 'Report Center' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('combobox', { name: 'Scan data source' })).toHaveValue('session-1'));
    expect(screen.getByText('AD context ready')).toBeInTheDocument();
    expect(screen.getByText('Snapshot resolved')).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Owner Review' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', { name: 'Report results' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.queryByText('Report workspace')).not.toBeInTheDocument();
    expect(screen.queryByText('Active session')).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Report configuration' })).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Preview & export' })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Configure exports' }));
    expect(screen.getByRole('tab', { name: 'Configure report' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('heading', { name: 'Report configuration' })).toBeInTheDocument();
    expect(screen.getByText('Report template')).toBeInTheDocument();
    expect(screen.getByText('Organization')).toBeInTheDocument();
    expect(screen.getByText('Prepared by')).toBeInTheDocument();
    expect(screen.getByText('Report period')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Delivery' }));
    expect(screen.getByText('Operator notes')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('tab', { name: 'Preview & export' }));
    expect(screen.getByRole('tab', { name: 'Preview & export' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('heading', { name: 'Preview & export' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Export CSV' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Export Excel' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Export HTML' })).toBeEnabled();
    expect(screen.queryByRole('button', { name: 'Export PDF' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Export DOCX' })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('tab', { name: 'Configure report' }));
    fireEvent.click(screen.getByRole('button', { name: 'Generate preview' }));
    await waitFor(() => expect(screen.getAllByText('Preview ready').length).toBeGreaterThan(0));
    expect(screen.getByRole('tab', { name: 'Preview & export' })).toHaveAttribute('aria-selected', 'true');
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining('/api/export/summary'),
      expect.objectContaining({ method: 'POST' }),
    );

    fireEvent.click(screen.getByRole('button', { name: 'Export CSV' }));
    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining('/api/export/download'),
      expect.objectContaining({
        method: 'POST',
        body: expect.stringContaining('"format":"csv"'),
      }),
    ));
    expect(createObjectURL).toHaveBeenCalled();
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:report');
  });
});
