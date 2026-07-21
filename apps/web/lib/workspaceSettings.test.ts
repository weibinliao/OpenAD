import {
  WORKSPACE_SETTINGS_KEY,
  OPERATION_LOG_KEY,
  REPORT_DEFAULTS_KEY,
  WORKSPACE_SETTINGS_UPDATED_EVENT,
  appendOperationLog,
  buildWorkspacePageTitle,
  createDefaultReportDefaults,
  defaultWorkspaceSettings,
  readReportDefaults,
  readWorkspaceSettings,
  resolveWorkspaceBranding,
  type ReportDefaults,
  type WorkspaceSettings,
  writeReportDefaults,
  writeWorkspaceSettings,
} from './workspaceSettings';

const fallbackReportDefaults: ReportDefaults = createDefaultReportDefaults('2026-04-02');

describe('workspaceSettings workspace defaults', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  test('persists and restores workspace settings with identity fields', () => {
    const saved: WorkspaceSettings = {
      ...defaultWorkspaceSettings,
      auditLogging: false,
      workspaceLabel: 'Security Review Room',
      workspaceTagline: 'NTFS and share operations',
      workspaceBadgeStyle: 'audit',
      workspaceLogoDataUrl: 'data:image/png;base64,ZmFrZS1sb2dv',
      workspaceLogoMimeType: 'image/png',
      workspaceLogoFileName: 'security-room.png',
    };

    writeWorkspaceSettings(saved);

    expect(readWorkspaceSettings()).toEqual(saved);
  });

  test('dispatches update event when workspace settings change', () => {
    const listener = jest.fn();
    window.addEventListener(WORKSPACE_SETTINGS_UPDATED_EVENT, listener);

    writeWorkspaceSettings(defaultWorkspaceSettings);

    expect(listener).toHaveBeenCalledTimes(1);
    window.removeEventListener(WORKSPACE_SETTINGS_UPDATED_EVENT, listener);
  });

  test('merges legacy workspace settings with new identity defaults', () => {
    window.localStorage.setItem(
      WORKSPACE_SETTINGS_KEY,
      JSON.stringify({
        rememberFilters: false,
        auditLogging: true,
        verboseADLogs: true,
        autoLoadRootTree: false,
        compactTables: false,
      })
    );

    expect(readWorkspaceSettings()).toEqual({
      ...defaultWorkspaceSettings,
      rememberFilters: false,
      auditLogging: true,
      verboseADLogs: true,
      autoLoadRootTree: false,
      compactTables: false,
    });
  });

  test('migrates the former Permission Protection default identity to OpenAD', () => {
    window.localStorage.setItem(
      WORKSPACE_SETTINGS_KEY,
      JSON.stringify({
        ...defaultWorkspaceSettings,
        workspaceLabel: 'Permission Protection Workspace',
        workspaceTagline: 'NTFS and AD permission protection',
        workspaceBadgeStyle: 'shield',
      }),
    );

    expect(readWorkspaceSettings()).toMatchObject({
      workspaceLabel: 'OpenAD',
      workspaceTagline: 'Active Directory and NTFS permission governance',
      workspaceBadgeStyle: 'graph',
    });
  });
});

describe('workspaceSettings report defaults', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  test('persists and restores report defaults', () => {
    const saved: ReportDefaults = {
      ...fallbackReportDefaults,
      exportMode: 'scan-results',
      selectedTemplateID: 'operations',
      selectedSections: ['metadata', 'action_plan'],
      organization: 'Operations',
      reportPeriod: '2026-04',
      sharePresetID: 'audit',
      shareNote: 'Weekly review package',
    };

    writeReportDefaults(saved);

    expect(readReportDefaults(fallbackReportDefaults)).toEqual(saved);
  });

  test('falls back when stored report defaults are invalid', () => {
    window.localStorage.setItem(REPORT_DEFAULTS_KEY, '{bad json');

    expect(readReportDefaults(fallbackReportDefaults)).toEqual(fallbackReportDefaults);
  });

  test('merges partial report defaults with fallback values', () => {
    window.localStorage.setItem(
      REPORT_DEFAULTS_KEY,
      JSON.stringify({
        organization: 'Operations',
        preparedBy: 'Control Desk',
        sharePresetID: 'audit',
      })
    );

    expect(readReportDefaults(fallbackReportDefaults)).toEqual({
      ...fallbackReportDefaults,
      organization: 'Operations',
      preparedBy: 'Control Desk',
      sharePresetID: 'audit',
    });
  });
});

describe('workspaceSettings branding helpers', () => {
  test('builds branded page titles with workspace identity and product fallback', () => {
    expect(
      buildWorkspacePageTitle('Scan Workspace', {
        workspaceLabel: 'Security Review Room',
      }, 'OpenAD')
    ).toBe('Scan Workspace · Security Review Room · OpenAD');
  });

  test('prefers a valid uploaded raster logo over the product mark', () => {
    const branding = resolveWorkspaceBranding({
      workspaceBadgeStyle: 'audit',
      workspaceLogoDataUrl: 'data:image/png;base64,ZmFrZS1sb2dv',
      workspaceLogoMimeType: 'image/png',
      workspaceLogoFileName: 'company-mark.png',
    }, 'OpenAD');

    expect(branding).toMatchObject({
      workspaceBadgeStyle: 'audit',
      workspaceLogoSrc: 'data:image/png;base64,ZmFrZS1sb2dv',
      workspaceLogoType: 'image/png',
      workspaceLogoFileName: 'company-mark.png',
      usingUploadedLogo: true,
    });
  });

  test('falls back to the OpenAD product mark when uploaded branding is invalid', () => {
    const branding = resolveWorkspaceBranding({
      workspaceBadgeStyle: 'vault',
      workspaceLogoDataUrl: 'data:image/svg+xml;base64,PHN2Zy8+',
      workspaceLogoMimeType: 'image/png',
      workspaceLogoFileName: 'not-supported.svg',
    }, 'OpenAD');

    expect(branding).toMatchObject({
      workspaceBadgeStyle: 'vault',
      workspaceLogoSrc: '/brand/openad.png',
      workspaceLogoType: 'image/png',
      workspaceLogoFileName: '',
      usingUploadedLogo: false,
    });
  });
});

describe('workspaceSettings operation log', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  test('skips log persistence when disabled', () => {
    appendOperationLog(
      {
        scope: 'scan',
        action: 'run-scan',
        message: 'disabled',
      },
      false
    );

    expect(window.localStorage.getItem(OPERATION_LOG_KEY)).toBeNull();
  });
});
