export interface ScanDefaults {
  defaultDepth: number;
  includeInherited: boolean;
}

export type WorkspaceBadgeStyle = 'shield' | 'layers' | 'terminal' | 'audit' | 'compass' | 'beacon' | 'vault' | 'graph';
export type WorkspaceLogoMimeType = 'image/png' | 'image/jpeg';

export const WORKSPACE_LOGO_MAX_BYTES = 512 * 1024;
export const WORKSPACE_LOGO_ACCEPT = '.png,.jpg,.jpeg';
export const DEFAULT_WORKSPACE_LOGO_SRC = '/brand/openad.png';

export interface WorkspaceSettings {
  rememberFilters: boolean;
  auditLogging: boolean;
  verboseADLogs: boolean;
  autoLoadRootTree: boolean;
  compactTables: boolean;
  workspaceLabel: string;
  workspaceTagline: string;
  workspaceBadgeStyle: WorkspaceBadgeStyle;
  workspaceLogoDataUrl: string;
  workspaceLogoMimeType: WorkspaceLogoMimeType | '';
  workspaceLogoFileName: string;
}

export interface WorkspaceBranding {
  workspaceLabel: string;
  workspaceTagline: string;
  workspaceBadgeStyle: WorkspaceBadgeStyle;
  workspaceLogoSrc: string;
  workspaceLogoType: string;
  workspaceLogoFileName: string;
  usingUploadedLogo: boolean;
  productTitle: string;
  siteName: string;
}

export interface ReportRightMapping {
  id: string;
  raw: string;
  translatedEn: string;
  translatedZh: string;
  noteEn: string;
  noteZh: string;
}

export interface ReportDefaults {
  exportMode: 'management-summary' | 'scan-results';
  selectedTemplateID: string;
  selectedSections: string[];
  reportTitle: string;
  organization: string;
  preparedBy: string;
  reportPeriod: string;
  adFieldIDs: string[];
  fileColumnIDs: string[];
  rightMappings: ReportRightMapping[];
  sharePresetID: string;
  shareNote: string;
}

export interface OperationLogEntry {
  id: string;
  at: string;
  scope: 'ad' | 'scan' | 'system';
  action: string;
  message: string;
  context?: Record<string, string | number | boolean | null | undefined>;
}

export const SCAN_DEFAULTS_KEY = 'fsa.scanDefaults';
export const WORKSPACE_SETTINGS_KEY = 'fsa.workspaceSettings';
export const OPERATION_LOG_KEY = 'fsa.operationLog';
export const REPORT_DEFAULTS_KEY = 'fsa.reportDefaults';
export const SCAN_FILTERS_KEY = 'fsa.scanFilters';
export const WORKSPACE_SETTINGS_UPDATED_EVENT = 'fsa:workspace-settings-updated';

export const defaultScanDefaults: ScanDefaults = {
  defaultDepth: -1,
  includeInherited: true,
};

export const defaultWorkspaceSettings: WorkspaceSettings = {
  rememberFilters: true,
  auditLogging: true,
  verboseADLogs: false,
  autoLoadRootTree: true,
  compactTables: true,
  workspaceLabel: 'OpenAD',
  workspaceTagline: 'Active Directory and NTFS permission governance',
  workspaceBadgeStyle: 'graph',
  workspaceLogoDataUrl: '',
  workspaceLogoMimeType: '',
  workspaceLogoFileName: '',
};

function isWorkspaceBadgeStyle(value: string | undefined): value is WorkspaceBadgeStyle {
  return value === 'shield'
    || value === 'layers'
    || value === 'terminal'
    || value === 'audit'
    || value === 'compass'
    || value === 'beacon'
    || value === 'vault'
    || value === 'graph';
}

export function isWorkspaceLogoMimeType(value: string | undefined): value is WorkspaceLogoMimeType {
  return value === 'image/png' || value === 'image/jpeg';
}

function resolveUploadedWorkspaceLogo(value: Partial<WorkspaceSettings> | null | undefined) {
  const workspaceLogoDataUrl = value?.workspaceLogoDataUrl?.trim() || '';
  const workspaceLogoMimeType = isWorkspaceLogoMimeType(value?.workspaceLogoMimeType)
    ? value.workspaceLogoMimeType
    : null;

  if (!workspaceLogoDataUrl || !workspaceLogoMimeType) {
    return null;
  }

  if (!workspaceLogoDataUrl.toLowerCase().startsWith(`data:${workspaceLogoMimeType};base64,`)) {
    return null;
  }

  return {
    src: workspaceLogoDataUrl,
    type: workspaceLogoMimeType,
    fileName: value?.workspaceLogoFileName?.trim() || '',
  };
}

export function resolveWorkspaceBranding(
  value: Partial<WorkspaceSettings> | null | undefined,
  productTitle: string
): WorkspaceBranding {
  const workspaceLabel = value?.workspaceLabel?.trim() || defaultWorkspaceSettings.workspaceLabel;
  const workspaceTagline = value?.workspaceTagline?.trim() || defaultWorkspaceSettings.workspaceTagline;
  const workspaceBadgeStyle = isWorkspaceBadgeStyle(value?.workspaceBadgeStyle)
    ? value.workspaceBadgeStyle
    : defaultWorkspaceSettings.workspaceBadgeStyle;
  const uploadedWorkspaceLogo = resolveUploadedWorkspaceLogo(value);
  const workspaceLogoSrc = uploadedWorkspaceLogo?.src || DEFAULT_WORKSPACE_LOGO_SRC;
  const workspaceLogoType = uploadedWorkspaceLogo?.type || 'image/png';
  const normalizedProductTitle = productTitle.trim() || 'OpenAD';
  const siteName = workspaceLabel === normalizedProductTitle
    ? normalizedProductTitle
    : `${workspaceLabel} · ${normalizedProductTitle}`;

  return {
    workspaceLabel,
    workspaceTagline,
    workspaceBadgeStyle,
    workspaceLogoSrc,
    workspaceLogoType,
    workspaceLogoFileName: uploadedWorkspaceLogo?.fileName || '',
    usingUploadedLogo: Boolean(uploadedWorkspaceLogo),
    productTitle: normalizedProductTitle,
    siteName,
  };
}

export function workspaceBadgeAssetPath(style: WorkspaceBadgeStyle) {
  return `/brand/${style}.svg`;
}

export function buildWorkspacePageTitle(
  pageTitle: string | null | undefined,
  value: Partial<WorkspaceSettings> | null | undefined,
  productTitle: string
) {
  const branding = resolveWorkspaceBranding(value, productTitle);
  const normalizedPageTitle = (pageTitle || '').trim();

  if (!normalizedPageTitle) {
    return branding.siteName;
  }

  return `${normalizedPageTitle} · ${branding.siteName}`;
}

export function createDefaultReportDefaults(reportPeriod = new Date().toISOString().slice(0, 10)): ReportDefaults {
  return {
    exportMode: 'management-summary',
    selectedTemplateID: 'management',
    selectedSections: ['metadata', 'kpis', 'top_trustees', 'top_paths'],
    reportTitle: 'OpenAD - Management Summary',
    organization: 'Your Organization',
    preparedBy: 'OpenAD',
    reportPeriod,
    adFieldIDs: ['sam', 'given', 'sn', 'mail', 'department', 'division', 'domain', 'originating-group', 'rights'],
    fileColumnIDs: ['path', 'trustee', 'sid', 'rights', 'type', 'inherited', 'source', 'applies-to', 'account-type', 'risk-level', 'access-mask', 'parent-delta'],
    rightMappings: [
      {
        id: 'modify',
        raw: 'Allow: Modify, Synchronize, This Folder, Subfolders and Files',
        translatedEn: 'Change',
        translatedZh: '修改',
        noteEn: 'General change capability.',
        noteZh: '通用修改权限。',
      },
      {
        id: 'read-execute',
        raw: 'Allow: ReadAndExecute, Synchronize, This Folder, Subfolders and Files',
        translatedEn: 'Read',
        translatedZh: '读取',
        noteEn: 'Common read-only access.',
        noteZh: '常见只读权限。',
      },
      {
        id: 'full-control',
        raw: 'Allow: FullControl, This Folder, Subfolders and Files',
        translatedEn: 'Full Control',
        translatedZh: '完全控制',
        noteEn: 'Highest-risk allow entry.',
        noteZh: '高风险完全控制权限。',
      },
      {
        id: 'write',
        raw: 'Allow: Write, Synchronize, This Folder, Subfolders and Files',
        translatedEn: 'Single Permission: Write',
        translatedZh: '单项权限: 写入',
        noteEn: 'Pure write capability.',
        noteZh: '纯写入能力。',
      },
      {
        id: 'deny-change',
        raw: 'Deny: Modify, This Folder, Subfolders and Files',
        translatedEn: 'Denied Change',
        translatedZh: '拒绝修改',
        noteEn: 'Explicit deny for change.',
        noteZh: '显式拒绝修改。',
      },
      {
        id: 'deny-read',
        raw: 'Deny: ReadAndExecute, This Folder, Subfolders and Files',
        translatedEn: 'Denied Read',
        translatedZh: '拒绝读取',
        noteEn: 'Explicit deny for read.',
        noteZh: '显式拒绝读取。',
      },
      {
        id: 'access-denied',
        raw: 'Deny: FullControl, This Folder, Subfolders and Files',
        translatedEn: 'Access Denied',
        translatedZh: '拒绝访问',
        noteEn: 'Hard deny posture.',
        noteZh: '强拒绝访问。',
      },
      {
        id: 'deny-write',
        raw: 'Deny: Write, Synchronize, This Folder, Subfolders and Files',
        translatedEn: 'Denied Single Permission: Write',
        translatedZh: '拒绝单项权限: 写入',
        noteEn: 'Explicit deny for write.',
        noteZh: '显式拒绝写入。',
      },
    ],
    sharePresetID: 'manager',
    shareNote: '',
  };
}

function hasWindow() {
  return typeof window !== 'undefined';
}

function safeRead<T>(key: string, fallback: T): T {
  if (!hasWindow()) {
    return fallback;
  }

  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) {
      return fallback;
    }
    return { ...fallback, ...(JSON.parse(raw) as Partial<T>) };
  } catch {
    return fallback;
  }
}

export function readScanDefaults() {
  return safeRead(SCAN_DEFAULTS_KEY, defaultScanDefaults);
}

export function writeScanDefaults(value: ScanDefaults) {
  if (!hasWindow()) {
    return;
  }
  window.localStorage.setItem(SCAN_DEFAULTS_KEY, JSON.stringify(value));
}

export function readWorkspaceSettings() {
  const settings = safeRead(WORKSPACE_SETTINGS_KEY, defaultWorkspaceSettings);
  const usesLegacyIdentity = settings.workspaceLabel === 'Permission Protection Workspace';
  if (!usesLegacyIdentity) {
    return settings;
  }

  return {
    ...settings,
    workspaceLabel: defaultWorkspaceSettings.workspaceLabel,
    workspaceTagline: settings.workspaceTagline === 'NTFS and AD permission protection'
      ? defaultWorkspaceSettings.workspaceTagline
      : settings.workspaceTagline,
    workspaceBadgeStyle: settings.workspaceBadgeStyle === 'shield'
      ? defaultWorkspaceSettings.workspaceBadgeStyle
      : settings.workspaceBadgeStyle,
  };
}

export function writeWorkspaceSettings(value: WorkspaceSettings) {
  if (!hasWindow()) {
    return;
  }
  window.localStorage.setItem(WORKSPACE_SETTINGS_KEY, JSON.stringify(value));
  window.dispatchEvent(new CustomEvent(WORKSPACE_SETTINGS_UPDATED_EVENT, { detail: value }));
}

export function readReportDefaults<T extends ReportDefaults>(fallback: T) {
  return safeRead(REPORT_DEFAULTS_KEY, fallback);
}

export function writeReportDefaults(value: ReportDefaults) {
  if (!hasWindow()) {
    return;
  }
  window.localStorage.setItem(REPORT_DEFAULTS_KEY, JSON.stringify(value));
}

export function appendOperationLog(entry: Omit<OperationLogEntry, 'id' | 'at'>, enabled = true) {
  if (!hasWindow() || !enabled) {
    return;
  }

  try {
    const raw = window.localStorage.getItem(OPERATION_LOG_KEY);
    const existing = raw ? ((JSON.parse(raw) as OperationLogEntry[]) || []) : [];
    const next: OperationLogEntry[] = [
      {
        id: typeof window.crypto?.randomUUID === 'function' ? window.crypto.randomUUID() : `log-${Date.now()}`,
        at: new Date().toISOString(),
        ...entry,
      },
      ...existing,
    ].slice(0, 200);

    window.localStorage.setItem(OPERATION_LOG_KEY, JSON.stringify(next));
  } catch {
    // Ignore local log persistence errors.
  }
}
