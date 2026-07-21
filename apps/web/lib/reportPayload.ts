export interface PermissionReportItem {
  path: string;
  trustee: string;
  trustee_sid: string;
  rights: string;
  type: string;
  inherited: boolean;
  source?: string;
  applies_to?: string;
  account_type?: string;
  access_mask?: string;
  risk_level?: string;
  parent_delta?: string;
  account_name?: string;
  first_name?: string;
  last_name?: string;
  email?: string;
  department?: string;
  division?: string;
  domain?: string;
  originating_group?: string;
  group_inheritance_hierarchy?: string;
}

export interface IndexedPermissionRow<T extends PermissionReportItem = PermissionReportItem> {
  rowKey: string;
  item: T;
}

export interface UserPermissionReportRow {
  id: string;
  path: string;
  trustee: string;
  trustee_sid: string;
  account_name: string;
  first_name: string;
  last_name: string;
  email: string;
  department: string;
  division: string;
  domain: string;
  originating_group: string;
  group_inheritance_hierarchy: string;
  permissions: string;
  permission_count: number;
  risk_level: string;
  applies_to_summary: string;
  inheritance_summary: string;
  row_count: number;
  member_keys: string[];
}

export interface PermissionTranslationMapping {
  raw: string;
  translatedEn: string;
  translatedZh: string;
}

export interface PermissionReportPayload {
  permissions: PermissionReportItem[];
  user_rows: UserPermissionReportRow[];
  export_mode: 'management-summary' | 'scan-results';
  title: string;
  template: string;
  sections: string[];
  organization: string;
  prepared_by: string;
  report_period: string;
  focus_areas: string[];
  ad_fields: string[];
  file_columns: string[];
}

function inferRiskLevel(rights: string, provided?: string) {
  const normalized = (provided || '').trim().toLowerCase();
  if (normalized === 'high' || normalized === 'medium' || normalized === 'low') {
    return normalized;
  }

  const value = rights.trim().toLowerCase();
  if (!value) {
    return 'unknown';
  }
  if (['full control', 'write', 'delete', 'take ownership', 'change permissions', 'modify'].some((token) => value.includes(token))) {
    return 'high';
  }
  if (value.includes('execute')) {
    return 'medium';
  }
  return 'low';
}

function firstNonEmptyString(...values: Array<string | undefined | null>) {
  for (const value of values) {
    const trimmed = (value || '').trim();
    if (trimmed) {
      return trimmed;
    }
  }
  return '';
}

function searchTermFromTrustee(trustee: string) {
  const trimmed = trustee.trim();
  if (!trimmed || trimmed.toUpperCase().startsWith('S-1-')) {
    return '';
  }
  if (trimmed.includes('\\')) {
    return trimmed.split('\\').pop()?.trim() || '';
  }
  if (trimmed.includes('@')) {
    return trimmed.split('@')[0]?.trim() || '';
  }
  return trimmed;
}

function deriveDomain(identity: string) {
  const trimmed = identity.trim();
  if (!trimmed) {
    return '';
  }
  if (trimmed.includes('\\')) {
    return trimmed.split('\\')[0]?.trim().toUpperCase() || '';
  }
  if (trimmed.includes('@')) {
    return trimmed.split('@')[1]?.trim().toUpperCase() || '';
  }
  return '';
}

function joinDistinct(values: Array<string | undefined>, separator = ' · ') {
  return Array.from(
    new Set(
      values
        .map((value) => (value || '').trim())
        .filter(Boolean)
    )
  ).join(separator);
}

function riskPriority(level: string) {
  switch (level) {
    case 'high':
      return 3;
    case 'medium':
      return 2;
    case 'low':
      return 1;
    default:
      return 0;
  }
}

function profileRichness(item: PermissionReportItem) {
  return [
    item.account_name,
    item.first_name,
    item.last_name,
    item.email,
    item.department,
    item.division,
    item.domain,
    item.originating_group,
  ].filter(Boolean).length;
}

function translatePermissionTypeForExport(type: string, locale: string) {
  const normalized = type.trim().toLowerCase();
  if (locale === 'zh-CN') {
    switch (normalized) {
      case 'allow':
        return '允许';
      case 'deny':
        return '拒绝';
      default:
        return type.trim();
    }
  }

  switch (normalized) {
    case 'allow':
      return 'Allow';
    case 'deny':
      return 'Deny';
    default:
      return type.trim();
  }
}

function translatePermissionRightsForExport(rights: string, locale: string) {
  const normalized = rights.trim()
    .replaceAll('ReadAndExecute', 'Read and Execute')
    .replaceAll('FullControl', 'Full Control');

  if (locale !== 'zh-CN') {
    return normalized;
  }

  return normalized
    .replaceAll('Read and Execute', '读取和执行')
    .replaceAll('Full Control', '完全控制')
    .replaceAll('Modify', '修改')
    .replaceAll('Read', '读取')
    .replaceAll('Write', '写入')
    .replaceAll('Execute', '执行')
    .replaceAll('Delete', '删除')
    .replaceAll('Take Ownership', '取得所有权')
    .replaceAll('Change Permissions', '更改权限')
    .replaceAll('Synchronize', '同步');
}

function normalizeTranslationValue(value: string) {
  return value.trim().replace(/\s+/g, ' ').toLowerCase();
}

function buildRawPermissionSignature(item: Pick<PermissionReportItem, 'type' | 'rights' | 'applies_to'>) {
  const typeLabel = translatePermissionTypeForExport(item.type || '', 'en-US');
  const rightsLabel = translatePermissionRightsForExport(item.rights || '', 'en-US');
  const appliesLabel = translateAppliesToForExport(item.applies_to || '', 'en-US');
  const parts: string[] = [];

  if (typeLabel && rightsLabel) {
    parts.push(`${typeLabel}: ${rightsLabel}`);
  } else if (rightsLabel) {
    parts.push(rightsLabel);
  }

  if (appliesLabel) {
    parts.push(appliesLabel);
  }

  return parts.join(', ');
}

function findPermissionTranslation(
  item: Pick<PermissionReportItem, 'type' | 'rights' | 'applies_to'>,
  rightMappings: readonly PermissionTranslationMapping[] = []
) {
  if (rightMappings.length === 0) {
    return undefined;
  }

  const rawSignature = normalizeTranslationValue(buildRawPermissionSignature(item));
  const rawRights = normalizeTranslationValue(translatePermissionRightsForExport(item.rights || '', 'en-US'));

  return rightMappings.find((mapping) => {
    const candidate = normalizeTranslationValue(mapping.raw || '');
    return candidate !== '' && (candidate === rawSignature || candidate === rawRights);
  });
}

function translateAppliesToForExport(appliesTo: string, locale: string) {
  const normalized = appliesTo.trim();
  if (locale !== 'zh-CN') {
    return normalized;
  }

  return normalized
    .replaceAll('This Folder, Subfolders and Files', '此文件夹、子文件夹和文件')
    .replaceAll('This Folder and Subfolders', '此文件夹和子文件夹')
    .replaceAll('This Folder and Files', '此文件夹和文件')
    .replaceAll('This Folder Only', '仅此文件夹')
    .replaceAll('Subfolders and Files Only', '仅子文件夹和文件')
    .replaceAll('Subfolders Only', '仅子文件夹')
    .replaceAll('Files Only', '仅文件')
    .replaceAll('(No Propagate)', '（不传播）');
}

function formatPermissionDisplayForExport(
  item: Pick<PermissionReportItem, 'type' | 'rights' | 'applies_to'>,
  locale: string,
  rightMappings: readonly PermissionTranslationMapping[] = []
) {
  const translation = findPermissionTranslation(item, rightMappings);
  if (translation) {
    return locale === 'zh-CN' ? translation.translatedZh : translation.translatedEn;
  }

  const typeLabel = translatePermissionTypeForExport(item.type || '', locale);
  const rightsLabel = translatePermissionRightsForExport(item.rights || '', locale);
  const appliesLabel = translateAppliesToForExport(item.applies_to || '', locale);
  const parts: string[] = [];

  if (typeLabel && rightsLabel) {
    parts.push(`${typeLabel}: ${rightsLabel}`);
  } else if (rightsLabel) {
    parts.push(rightsLabel);
  }

  if (appliesLabel) {
    parts.push(appliesLabel);
  }

  return parts.join(', ');
}

export function buildPermissionRowKey(item: Pick<PermissionReportItem, 'path' | 'trustee' | 'trustee_sid' | 'source'>, index: number) {
  return `${item.path}::${item.trustee}::${item.trustee_sid}::${item.source || ''}::${index}`;
}

export function toExportPermissions<T extends PermissionReportItem>(
  items: readonly T[],
  source: string,
  locale: string,
  rightMappings: readonly PermissionTranslationMapping[] = []
): PermissionReportItem[] {
  return items.map((item) => {
    const translation = findPermissionTranslation(item, rightMappings);
    return {
    path: item.path,
    trustee: item.trustee,
    trustee_sid: item.trustee_sid,
    rights: translation
      ? (locale === 'zh-CN' ? translation.translatedZh : translation.translatedEn) || item.rights
      : item.rights,
    type: item.type,
    inherited: item.inherited,
    source: item.source || source,
    applies_to: item.applies_to || '',
    account_type: item.account_type || '',
    access_mask: item.access_mask || '',
    risk_level: item.risk_level || '',
    parent_delta: item.parent_delta || '',
    account_name: firstNonEmptyString(item.account_name, item.trustee, item.trustee_sid),
    first_name: item.first_name || '',
    last_name: item.last_name || '',
    email: item.email || '',
    department: item.department || '',
    division: item.division || '',
    domain: item.domain || '',
    originating_group: item.originating_group || '',
    group_inheritance_hierarchy: item.group_inheritance_hierarchy || '',
    };
  });
}

export function buildUserReportRows<T extends PermissionReportItem>(
  entries: readonly IndexedPermissionRow<T>[],
  locale: string,
  rightMappings: readonly PermissionTranslationMapping[] = []
): UserPermissionReportRow[] {
  const grouped = new Map<
    string,
    {
      representative: T;
      items: T[];
      memberKeys: string[];
    }
  >();

  for (const entry of entries) {
    const identity = firstNonEmptyString(entry.item.account_name, searchTermFromTrustee(entry.item.trustee), entry.item.trustee_sid).trim().toLowerCase();
    const domain = firstNonEmptyString(entry.item.domain, deriveDomain(entry.item.trustee || entry.item.account_name || '')).trim().toLowerCase();
    const key = `${entry.item.path}::${domain}::${identity}`;
    const current = grouped.get(key);

    if (!current) {
      grouped.set(key, {
        representative: entry.item,
        items: [entry.item],
        memberKeys: [entry.rowKey],
      });
      continue;
    }

    current.items.push(entry.item);
    current.memberKeys.push(entry.rowKey);
    if (profileRichness(entry.item) > profileRichness(current.representative)) {
      current.representative = entry.item;
    }
  }

  return Array.from(grouped.entries())
    .map<UserPermissionReportRow>(([id, group]) => {
      const riskLevel = group.items.reduce((worst, item) => {
        const next = inferRiskLevel(item.rights, item.risk_level);
        return riskPriority(next) > riskPriority(worst) ? next : worst;
      }, 'unknown');
      const explicitCount = group.items.filter((item) => !item.inherited).length;
      const inheritedCount = group.items.length - explicitCount;
      const inheritanceSummary =
        explicitCount > 0 && inheritedCount > 0
          ? 'Mixed'
          : explicitCount > 0
            ? 'Explicit'
            : inheritedCount > 0
              ? 'Inherited'
              : '-';

      return {
        id,
        path: group.representative.path,
        trustee: group.representative.trustee,
        trustee_sid: group.representative.trustee_sid,
        account_name: firstNonEmptyString(group.representative.account_name, searchTermFromTrustee(group.representative.trustee), group.representative.trustee_sid),
        first_name: group.representative.first_name || '',
        last_name: group.representative.last_name || '',
        email: group.representative.email || '',
        department: group.representative.department || '',
        division: group.representative.division || '',
        domain: firstNonEmptyString(group.representative.domain, deriveDomain(group.representative.trustee || group.representative.account_name || '')),
        originating_group: joinDistinct(group.items.map((item) => item.originating_group), ' / '),
        group_inheritance_hierarchy: joinDistinct(
          group.items.map((item) => item.group_inheritance_hierarchy || item.originating_group),
          ' / '
        ),
        permissions: joinDistinct(group.items.map((item) => formatPermissionDisplayForExport(item, locale, rightMappings)), ' / '),
        permission_count: new Set(group.items.map((item) => formatPermissionDisplayForExport(item, locale, rightMappings).trim()).filter(Boolean)).size,
        risk_level: riskLevel,
        applies_to_summary: joinDistinct(group.items.map((item) => item.applies_to)),
        inheritance_summary: inheritanceSummary,
        row_count: group.items.length,
        member_keys: [...group.memberKeys],
      };
    })
    .sort((a, b) => {
      const left = a.account_name.trim().toLowerCase();
      const right = b.account_name.trim().toLowerCase();
      if (left !== right) {
        return left.localeCompare(right);
      }
      return a.originating_group.trim().toLowerCase().localeCompare(b.originating_group.trim().toLowerCase());
    });
}

export function buildPermissionReportPayload<T extends PermissionReportItem>(params: {
  entries: readonly IndexedPermissionRow<T>[];
  source: string;
  locale: string;
  exportMode: 'management-summary' | 'scan-results';
  title: string;
  template: string;
  sections: readonly string[];
  organization: string;
  preparedBy: string;
  reportPeriod: string;
  focusAreas: readonly string[];
  adFields: readonly string[];
  fileColumns: readonly string[];
  rightMappings?: readonly PermissionTranslationMapping[];
}): PermissionReportPayload {
  const permissions = toExportPermissions(params.entries.map((entry) => entry.item), params.source, params.locale, params.rightMappings || []);

  return {
    permissions,
    user_rows: buildUserReportRows(params.entries, params.locale, params.rightMappings || []),
    export_mode: params.exportMode,
    title: params.title,
    template: params.template,
    sections: [...params.sections],
    organization: params.organization,
    prepared_by: params.preparedBy,
    report_period: params.reportPeriod,
    focus_areas: [...params.focusAreas],
    ad_fields: [...params.adFields],
    file_columns: [...params.fileColumns],
  };
}
