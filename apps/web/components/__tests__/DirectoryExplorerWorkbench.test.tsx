import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import '@testing-library/jest-dom';
import DirectoryExplorerWorkbench from '../DirectoryExplorerWorkbench';
import { I18nProvider } from '../../contexts/I18nContext';

function response(payload: unknown) {
  return Promise.resolve({ ok: true, json: async () => payload } as Response);
}

describe('DirectoryExplorerWorkbench', () => {
  beforeEach(() => {
    window.localStorage.setItem('fsa.locale', 'en');
    global.fetch = jest.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith('/api/ad/tree')) {
        return response({
          nodes: [
            { dn: 'CN=Domain Admins,DC=lab,DC=local', name: 'Domain Admins', node_type: 'group', has_children: true },
            { dn: 'CN=Alice,DC=lab,DC=local', name: 'Alice', node_type: 'user', has_children: false },
          ],
          pagination: { total_pages: 1 },
        });
      }
      if (url.endsWith('/api/ad/users/query')) {
        return response({
          users: [{
            dn: 'CN=Alice,DC=lab,DC=local',
            username: 'azhang',
            display_name: 'Alice Zhang',
            email: 'alice@lab.local',
            department: 'IT',
            domain: 'LAB.LOCAL',
            groups: [
              'CN=Domain Admins,DC=lab,DC=local',
              'CN=File Operators,DC=lab,DC=local',
            ],
          }],
        });
      }
      if (url.endsWith('/api/ad/groups/query')) {
        return response({ groups: [{ dn: 'CN=File Operators,DC=lab,DC=local', name: 'File Operators' }] });
      }
      if (url.endsWith('/api/ad/groups/members')) {
        return response({
          resolution: {
            members: [{ dn: 'CN=Bob Chen,DC=lab,DC=local', name: 'Bob Chen', sam_account_name: 'bchen', type: 'user', depth: 0 }],
          },
        });
      }
      return response({});
    }) as jest.MockedFunction<typeof fetch>;
  });

  afterEach(() => jest.clearAllMocks());

  test('filters loaded tree nodes without another request', async () => {
    render(<I18nProvider><DirectoryExplorerWorkbench connectionId="profile-1" /></I18nProvider>);

    expect(await screen.findByText('Domain Admins')).toBeInTheDocument();
    const callsBeforeFilter = (global.fetch as jest.Mock).mock.calls.length;

    fireEvent.change(screen.getByLabelText('Filter expanded tree nodes'), { target: { value: 'alice' } });

    expect(screen.queryByText('Domain Admins')).not.toBeInTheDocument();
    expect(screen.getByText('Alice')).toBeInTheDocument();
    expect((global.fetch as jest.Mock).mock.calls).toHaveLength(callsBeforeFilter);
  });

  test('searches users and groups together', async () => {
    render(<I18nProvider><DirectoryExplorerWorkbench connectionId="profile-1" /></I18nProvider>);
    await screen.findByText('Domain Admins');

    fireEvent.change(screen.getByLabelText('Search account, name, email, or group'), { target: { value: 'alice' } });
    const searchButton = screen.getByRole('button', { name: 'Search entire directory' });
    expect(searchButton).toHaveClass('shrink-0');
    expect(searchButton).toHaveAttribute('title', 'Search entire directory');
    fireEvent.click(searchButton);

    expect(await screen.findByText('Alice Zhang')).toBeInTheDocument();
    expect(screen.getByText('CN=Alice,DC=lab,DC=local')).toBeInTheDocument();
    expect(screen.getByText('File Operators')).toBeInTheDocument();
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(expect.stringContaining('/api/ad/users/query'), expect.objectContaining({ method: 'POST' }));
      expect(global.fetch).toHaveBeenCalledWith(expect.stringContaining('/api/ad/groups/query'), expect.objectContaining({ method: 'POST' }));
    });
  });

  test('runs the full directory search for a query forwarded from global search', async () => {
    render(
      <I18nProvider>
        <DirectoryExplorerWorkbench connectionId="profile-1" initialQuery="alex" />
      </I18nProvider>,
    );

    expect(await screen.findByText('Alice Zhang')).toBeInTheDocument();
    expect(screen.getByLabelText('Search account, name, email, or group')).toHaveValue('alex');

    await waitFor(() => {
      const userSearchCall = (global.fetch as jest.Mock).mock.calls.find(
        ([input]) => String(input).endsWith('/api/ad/users/query'),
      );
      expect(userSearchCall).toBeDefined();
      expect(JSON.parse(String(userSearchCall?.[1]?.body))).toMatchObject({
        connection_id: 'profile-1',
        query: 'alex',
      });
    });
  });

  test('shows user details and direct groups when a user tree node is selected', async () => {
    render(<I18nProvider><DirectoryExplorerWorkbench connectionId="profile-1" /></I18nProvider>);

    fireEvent.click(await screen.findByText('Alice'));

    expect(await screen.findByRole('heading', { name: 'Alice Zhang' })).toBeInTheDocument();
    const inspector = screen.getByRole('region', { name: 'Object details' });
    expect(within(inspector).getByText('Direct AD groups')).toBeInTheDocument();
    expect(within(inspector).getByText('Domain Admins')).toBeInTheDocument();
    expect(within(inspector).getByText('File Operators')).toBeInTheDocument();
    expect(within(inspector).getByText('alice@lab.local')).toBeInTheDocument();
  });

  test('shows group details and members in the right inspector when selected', async () => {
    render(<I18nProvider><DirectoryExplorerWorkbench connectionId="profile-1" /></I18nProvider>);

    fireEvent.click(await screen.findByText('Domain Admins'));

    expect(await screen.findByRole('heading', { name: 'Domain Admins' })).toBeInTheDocument();
    expect(await screen.findByText('Bob Chen')).toBeInTheDocument();
    expect(global.fetch).toHaveBeenCalledWith(expect.stringContaining('/api/ad/groups/members'), expect.objectContaining({ method: 'POST' }));
  });

  test('automatically suggests directory objects while typing and selects a suggestion', async () => {
    render(<I18nProvider><DirectoryExplorerWorkbench connectionId="profile-1" /></I18nProvider>);
    await screen.findByText('Domain Admins');

    fireEvent.change(screen.getByRole('combobox', { name: 'Search account, name, email, or group' }), {
      target: { value: 'ali' },
    });

    const suggestion = await screen.findByRole('option', { name: /Alice Zhang/ });
    fireEvent.click(suggestion);

    expect(await screen.findByRole('heading', { name: 'Alice Zhang' })).toBeInTheDocument();
    expect(screen.getByText('2 direct groups')).toBeInTheDocument();
  });
});
