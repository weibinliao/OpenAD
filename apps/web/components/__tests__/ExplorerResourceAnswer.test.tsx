import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';

import Explorer from '../explorer/Explorer';
import { I18nProvider } from '../../contexts/I18nContext';

jest.mock('../../contexts/ADConnectionContext', () => ({
  useADConnection: () => ({ activeProfile: null }),
}));

const resourcePath = '\\\\fs01\\Finance';

const resourceResult = {
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
      paths: [resourcePath],
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
      paths: [resourcePath],
    },
    {
      sid: 'S-1-5-21-1-1002',
      name: 'Bob Brown',
      source: 'user',
      rights: ['Read'],
      types: ['allow'],
      paths: [resourcePath],
    },
  ],
};

function expectBefore(earlier: Element | null, later: Element | null) {
  expect(earlier).not.toBeNull();
  expect(later).not.toBeNull();
  expect(earlier!.compareDocumentPosition(later!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
}

describe('Explorer resource answer group tree', () => {
  beforeEach(() => {
    window.localStorage.setItem('fsa.locale', 'en');
    global.fetch = jest.fn(async (input) => {
      const url = String(input);
      if (url.includes('/api/sessions?')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({
            items: [{ id: 'session-1', root_path: resourcePath, permission_count: 4 }],
          }),
        } as Response;
      }
      if (url.includes('/api/access/by-resource')) {
        return { ok: true, status: 200, json: async () => resourceResult } as Response;
      }
      if (url.includes('/api/sessions/session-1/permissions')) {
        return { ok: true, status: 200, json: async () => ({ items: [] }) } as Response;
      }
      throw new Error(`Unexpected request: ${url}`);
    }) as jest.MockedFunction<typeof fetch>;
  });

  afterEach(() => jest.clearAllMocks());

  test('shows group parents before members and collapses the selected group', async () => {
    render(
      <I18nProvider>
        <Explorer />
      </I18nProvider>,
    );

    fireEvent.click(await screen.findByRole('treeitem', { name: /Finance/ }));

    const collapse = await screen.findByRole('button', { name: 'Collapse Finance' });
    const financeRow = collapse.closest('li');
    const aliceRow = screen.getByText('Alice Adams').closest('li');
    const bobRow = screen.getByText('Bob Brown').closest('li');
    expectBefore(financeRow, aliceRow);
    expectBefore(aliceRow, bobRow);

    fireEvent.click(collapse);
    await waitFor(() => expect(screen.queryByText('Alice Adams')).not.toBeInTheDocument());
    expect(screen.getByText('Bob Brown')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Expand Finance' })).toBeInTheDocument();
  });
});
