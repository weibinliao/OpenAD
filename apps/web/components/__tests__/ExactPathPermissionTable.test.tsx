import { fireEvent, render, screen } from '@testing-library/react';
import ExactPathPermissionTable from '../ExactPathPermissionTable';
import { I18nProvider } from '../../contexts/I18nContext';

const baseEntries = [
  {
    rowKey: 'acl-1',
    item: {
      path: '\\\\server\\share\\Finance',
      trustee: 'DOMAIN\\Alice',
      trustee_sid: 'S-1-5-21-1000',
      rights: 'Write',
      type: 'Allow',
      inherited: false,
      source: 'UNC',
      applies_to: 'This Folder Only',
      account_type: 'User',
      risk_level: 'high',
      parent_delta: 'Explicit on Current Item',
      account_name: 'alice',
      first_name: 'Alice',
      last_name: 'Ng',
      email: 'alice@example.com',
      department: 'Finance',
      division: 'Operations',
      domain: 'DOMAIN',
      originating_group: 'DOMAIN\\Finance-Writers',
      group_inheritance_hierarchy: 'DOMAIN\\Finance-Writers',
    },
  },
  {
    rowKey: 'acl-2',
    item: {
      path: '\\\\server\\share\\Finance',
      trustee: 'DOMAIN\\Alice',
      trustee_sid: 'S-1-5-21-1000',
      rights: 'Delete',
      type: 'Allow',
      inherited: false,
      source: 'UNC',
      applies_to: 'This Folder and Files',
      account_type: 'User',
      risk_level: 'high',
      parent_delta: 'Explicit on Current Item',
      account_name: 'alice',
      first_name: 'Alice',
      last_name: 'Ng',
      email: 'alice@example.com',
      department: 'Finance',
      division: 'Operations',
      domain: 'DOMAIN',
      originating_group: 'DOMAIN\\Finance-Owners',
      group_inheritance_hierarchy: 'DOMAIN\\Finance-Owners',
    },
  },
];

function renderTable() {
  return render(
    <I18nProvider>
      <ExactPathPermissionTable
        locale="en"
        selectedPath="\\\\server\\share\\Finance"
        entries={baseEntries}
        totalScanRowCount={6}
      />
    </I18nProvider>
  );
}

describe('ExactPathPermissionTable', () => {
  test('renders one table row per raw ACL entry for the exact selected folder', () => {
    renderTable();

    expect(screen.getByText('Exact-path ACL evidence')).toBeInTheDocument();
    expect(screen.getByText((content) => content.includes('server') && content.includes('share') && content.includes('Finance'))).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Account Name' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Originating Group' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Permissions' })).toBeInTheDocument();
    expect(screen.getByText('Write')).toBeInTheDocument();
    expect(screen.getByText('Delete')).toBeInTheDocument();
    expect(screen.getByText('DOMAIN\\Finance-Writers')).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: /Inspect ACL row for alice/i })).toHaveLength(2);
  });

  test('opens the inspector for the selected raw ACL row', () => {
    renderTable();

    const inspectButtons = screen.getAllByRole('button', { name: /Inspect ACL row for alice/i });
    fireEvent.click(inspectButtons[1]);

    expect(screen.getByText('Inspector')).toBeInTheDocument();
    expect(screen.getByText('Permission Summary')).toBeInTheDocument();
    expect(screen.getAllByText('Delete')).toHaveLength(2);
    expect(screen.getAllByText('This Folder and Files')).toHaveLength(2);
  });
});
