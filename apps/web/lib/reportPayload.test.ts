import {
  buildPermissionReportPayload,
  buildUserReportRows,
  type IndexedPermissionRow,
  type PermissionReportItem,
} from './reportPayload';

function createEntry(overrides: Partial<PermissionReportItem>, rowKey: string): IndexedPermissionRow {
  return {
    rowKey,
    item: {
      path: `C:\\Finance`,
      trustee: `DOMAIN\\Alice`,
      trustee_sid: 'S-1-5-21-1000',
      rights: 'Read',
      type: 'Allow',
      inherited: false,
      source: 'LOCAL',
      applies_to: 'This Folder Only',
      account_type: 'User',
      access_mask: '0x1',
      risk_level: 'low',
      parent_delta: 'Explicit on Current Item',
      account_name: 'alice',
      first_name: 'Alice',
      last_name: 'Ng',
      email: 'alice@example.com',
      department: 'Finance',
      division: 'Operations',
      domain: 'DOMAIN',
      originating_group: `DOMAIN\\Finance-Readers`,
      group_inheritance_hierarchy: `DOMAIN\\Finance-Readers`,
      ...overrides,
    },
  };
}

describe('report payload semantics', () => {
  test('groups rows by path and account while preserving member keys', () => {
    const entries = [
      createEntry({}, 'finance-read'),
      createEntry(
        {
          rights: 'Write',
          inherited: true,
          applies_to: 'This Folder and Files',
          risk_level: 'high',
          originating_group: `DOMAIN\\Finance-Writers`,
          group_inheritance_hierarchy: `DOMAIN\\Finance-Writers`,
        },
        'finance-write'
      ),
      createEntry(
        {
          path: `C:\\Finance\\Payroll`,
          originating_group: `DOMAIN\\Payroll-Readers`,
          group_inheritance_hierarchy: `DOMAIN\\Payroll-Readers`,
        },
        'payroll-read'
      ),
    ];

    const rows = buildUserReportRows(entries, 'en');

    expect(rows).toHaveLength(2);
    expect(rows.map((row) => row.path)).toEqual(expect.arrayContaining([`C:\\Finance`, `C:\\Finance\\Payroll`]));

    const financeRow = rows.find((row) => row.path === `C:\\Finance`);
    expect(financeRow).toMatchObject({
      account_name: 'alice',
      permission_count: 2,
      risk_level: 'high',
      inheritance_summary: 'Mixed',
      row_count: 2,
      member_keys: ['finance-read', 'finance-write'],
    });
    expect(financeRow?.permissions).toBe('Allow: Read, This Folder Only / Allow: Write, This Folder and Files');
  });

  test('builds one shared payload for summary and download requests', () => {
    const entries = [
      createEntry({}, 'finance-read'),
      createEntry(
        {
          rights: 'Write',
          inherited: true,
          applies_to: 'This Folder and Files',
          risk_level: 'high',
          originating_group: `DOMAIN\\Finance-Writers`,
          group_inheritance_hierarchy: `DOMAIN\\Finance-Writers`,
        },
        'finance-write'
      ),
    ];

    const payload = buildPermissionReportPayload({
      entries,
      source: 'LOCAL',
      locale: 'en',
      exportMode: 'management-summary',
      title: 'Finance Access Review',
      template: 'management',
      sections: ['metadata', 'kpis'],
      organization: 'Your Organization',
      preparedBy: 'OpenAD',
      reportPeriod: '2026-03-31',
      focusAreas: ['High-risk rights'],
      adFields: ['sam', 'mail'],
      fileColumns: ['path', 'rights'],
    });

    expect(payload.permissions).toHaveLength(2);
    expect(payload.user_rows).toEqual(buildUserReportRows(entries, 'en'));
    expect(payload).toMatchObject({
      export_mode: 'management-summary',
      title: 'Finance Access Review',
      template: 'management',
      sections: ['metadata', 'kpis'],
      organization: 'Your Organization',
      prepared_by: 'OpenAD',
      report_period: '2026-03-31',
      focus_areas: ['High-risk rights'],
      ad_fields: ['sam', 'mail'],
      file_columns: ['path', 'rights'],
    });
  });

  test('allows scan-results mode to share the same payload shape', () => {
    const entries = [createEntry({}, 'finance-read')];

    const payload = buildPermissionReportPayload({
      entries,
      source: 'LOCAL',
      locale: 'en',
      exportMode: 'scan-results',
      title: 'Finance Raw Export',
      template: 'management',
      sections: ['metadata'],
      organization: 'Your Organization',
      preparedBy: 'OpenAD',
      reportPeriod: '2026-04-01',
      focusAreas: ['Exact path ACL rows'],
      adFields: ['sam'],
      fileColumns: ['path', 'rights'],
    });

    expect(payload.export_mode).toBe('scan-results');
    expect(payload.permissions).toHaveLength(1);
    expect(payload.user_rows).toEqual(buildUserReportRows(entries, 'en'));
  });

  test('applies configured permission translations to exported payload rows', () => {
    const entries = [
      createEntry(
        {
          rights: 'Modify, Synchronize',
          applies_to: 'This Folder, Subfolders and Files',
          risk_level: 'high',
        },
        'finance-modify'
      ),
    ];

    const payload = buildPermissionReportPayload({
      entries,
      source: 'LOCAL',
      locale: 'en',
      exportMode: 'management-summary',
      title: 'Finance Access Review',
      template: 'management',
      sections: ['metadata'],
      organization: 'Your Organization',
      preparedBy: 'OpenAD',
      reportPeriod: '2026-04-02',
      focusAreas: ['Translated permissions'],
      adFields: ['sam'],
      fileColumns: ['path', 'rights'],
      rightMappings: [
        {
          raw: 'Allow: Modify, Synchronize, This Folder, Subfolders and Files',
          translatedEn: 'Change',
          translatedZh: '修改',
        },
      ],
    });

    expect(payload.permissions[0].rights).toBe('Change');
    expect(payload.user_rows[0].permissions).toBe('Change');
    expect(payload.user_rows[0].permission_count).toBe(1);
    expect(payload.user_rows[0].risk_level).toBe('high');
  });
});
