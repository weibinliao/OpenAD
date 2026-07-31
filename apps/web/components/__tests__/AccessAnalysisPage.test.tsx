import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';

import AccessAnalysisPage from '../../pages/access';
import { I18nProvider } from '../../contexts/I18nContext';

jest.mock('next/router', () => ({
  useRouter: () => ({
    isReady: true,
    query: { path: '\\\\fs01\\Finance' },
  }),
}));

const response = {
  path_prefix: '\\\\fs01\\Finance',
  session: { id: 'session-1', root_path: '\\\\fs01\\Finance', status: 'completed' },
  sync_run_id: 'run-1',
  counts: { aces: 4, principals: 4, groups: 1, users: 1, via_groups: 1, unresolved: 1 },
  principals: [
    {
      sid: 'S-1-5-21-1-2001',
      name: 'Finance',
      source: 'group',
      group_name: 'Finance',
      group_sid: 'S-1-5-21-1-2001',
      member_count: 1,
      rights: ['Modify'],
      types: ['allow'],
      paths: ['\\\\fs01\\Finance'],
    },
    {
      sid: 'S-1-5-21-1-1001',
      name: 'Alice Adams',
      source: 'group-member',
      group_name: 'Finance',
      group_sid: 'S-1-5-21-1-2001',
      via_chain: 'Finance',
      rights: ['Modify'],
      types: ['allow'],
      paths: ['\\\\fs01\\Finance'],
      enabled: true,
    },
    {
      sid: 'S-1-5-21-1-1002',
      name: 'Bob Brown',
      source: 'user',
      rights: ['Read'],
      types: ['allow'],
      paths: ['\\\\fs01\\Finance'],
      enabled: true,
    },
    {
      sid: 'S-1-1-0',
      name: 'Everyone',
      source: 'unresolved',
      rights: ['Read'],
      types: ['allow'],
      paths: ['\\\\fs01\\Finance'],
    },
  ],
  aces: [],
};

function expectBefore(earlier: Element | null, later: Element | null) {
  expect(earlier).not.toBeNull();
  expect(later).not.toBeNull();
  expect(earlier!.compareDocumentPosition(later!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
}

describe('AccessAnalysisPage resource group tree', () => {
  beforeEach(() => {
    window.localStorage.setItem('fsa.locale', 'en');
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => response,
    } as Response) as jest.MockedFunction<typeof fetch>;
  });

  afterEach(() => jest.clearAllMocks());

  test('shows AD groups before their members and allows each group to collapse', async () => {
    render(
      <I18nProvider>
        <AccessAnalysisPage />
      </I18nProvider>,
    );

    const collapse = await screen.findByRole('button', { name: 'Collapse Finance' });
    expect(screen.getByText('AD Group')).toBeInTheDocument();

    const financeRow = collapse.closest('tr');
    const aliceRow = screen.getByText('Alice Adams').closest('tr');
    const bobRow = screen.getByText('Bob Brown').closest('tr');
    const everyoneRow = screen.getByText('Everyone').closest('tr');
    expectBefore(financeRow, aliceRow);
    expectBefore(aliceRow, bobRow);
    expectBefore(bobRow, everyoneRow);

    fireEvent.click(collapse);
    await waitFor(() => expect(screen.queryByText('Alice Adams')).not.toBeInTheDocument());
    expect(screen.getByText('Finance')).toBeInTheDocument();
    expect(screen.getByText('Bob Brown')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Expand Finance' })).toBeInTheDocument();
  });
});
