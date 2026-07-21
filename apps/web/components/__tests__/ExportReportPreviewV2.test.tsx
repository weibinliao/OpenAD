import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import ExportReportPreviewV2 from '../ExportReportPreviewV2';

const baseProps = {
  locale: 'en',
  reportTitle: 'Finance Access Review',
  templateTitle: 'Operations Review',
  templateSummary: 'Compact enterprise review surface for summarized permissions.',
  previewGeneratedAt: '2026-03-31 09:45',
  normalizedPath: '\\\\server\\share',
  modeSummary: 'Focused on the current selected scope.',
  sharePresetLabel: 'Internal reviewers',
  stats: {
    total: 9,
    visible: 6,
    highRisk: 3,
    skipped: 1,
  },
  previewUserRows: [
    {
      id: 'row-1',
      path: '\\\\server\\share\\Finance',
      trustee: 'DOMAIN\\Alice',
      trustee_sid: 'S-1-5-21-1000',
      account_name: 'alice',
      first_name: 'Alice',
      last_name: 'Ng',
      email: 'alice@example.com',
      department: 'Finance',
      division: 'Operations',
      domain: 'DOMAIN',
      originating_group: 'DOMAIN\\Finance-Writers',
      permissions: 'Allow: Write',
      group_inheritance_hierarchy: 'DOMAIN\\Finance-Writers',
      permission_count: 2,
      risk_level: 'high',
      applies_to_summary: 'This Folder Only',
      inheritance_summary: 'Explicit',
      row_count: 2,
    },
    {
      id: 'row-2',
      path: '\\\\server\\share\\Payroll',
      trustee: 'DOMAIN\\Bob',
      trustee_sid: 'S-1-5-21-1001',
      account_name: 'bob',
      first_name: 'Bob',
      last_name: 'Li',
      email: 'bob@example.com',
      department: 'Payroll',
      division: 'Operations',
      domain: 'DOMAIN',
      originating_group: 'DOMAIN\\Payroll-Readers',
      permissions: 'Allow: Read',
      group_inheritance_hierarchy: 'Inherited from share root',
      permission_count: 1,
      risk_level: 'low',
      applies_to_summary: 'This Folder and Files',
      inheritance_summary: 'Inherited',
      row_count: 1,
    },
  ],
  previewAclRows: [
    {
      path: '\\\\server\\share\\Finance',
      trustee: 'DOMAIN\\Alice',
      rights: 'Write',
      type: 'Allow',
      inherited: false,
      applies_to: 'This Folder Only',
      account_type: 'User',
      parent_delta: 'Explicit on Current Item',
    },
    {
      path: '\\\\server\\share\\Payroll',
      trustee: 'DOMAIN\\Bob',
      rights: 'Read',
      type: 'Allow',
      inherited: true,
      applies_to: 'This Folder and Files',
      account_type: 'User',
      parent_delta: 'Inherited',
    },
  ],
  previewNarrativeLead: 'Summarized path-to-account exposure for the selected export scope.',
  previewNarrativeBullets: ['Prioritize write-capable identities.', 'Compare explicit and inherited chains inline.'],
};

describe('ExportReportPreviewV2', () => {
  test('renders the dense permission ledger with preserved columns and row context', () => {
    render(<ExportReportPreviewV2 {...baseProps} />);

    expect(screen.getByText('Finance Access Review')).toBeInTheDocument();
    expect(screen.getByText('Internal reviewers')).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Path' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Account Name' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Name' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'E-Mail' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Department' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Domain' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Originating Group' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Permissions' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Inheritance' })).toBeInTheDocument();

    expect(screen.getByText('alice')).toBeInTheDocument();
    expect(screen.getByText('Alice Ng')).toBeInTheDocument();
    expect(screen.getByText('alice@example.com')).toBeInTheDocument();
    expect(screen.getByText('DOMAIN\\Finance-Writers')).toBeInTheDocument();
    expect(screen.getByText('Allow: Write')).toBeInTheDocument();
    expect(screen.getByText('High risk')).toBeInTheDocument();
    expect(screen.getByText('Explicit')).toBeInTheDocument();
    expect(screen.getByText('2 source rows')).toBeInTheDocument();
  });

  test('shows the empty state without dropping the ledger structure', () => {
    render(
      <ExportReportPreviewV2
        {...baseProps}
        previewUserRows={[]}
        previewAclRows={[]}
        stats={{
          total: 0,
          visible: 0,
          highRisk: 0,
          skipped: 0,
        }}
      />
    );

    expect(screen.getByRole('columnheader', { name: 'Path' })).toBeInTheDocument();
    expect(screen.getByText('No report rows are available yet.')).toBeInTheDocument();
  });
});
