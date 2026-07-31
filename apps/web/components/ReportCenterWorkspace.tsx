import { Download, Eye, FileCode2, FileSpreadsheet, FileText, ListTree, RefreshCw, Settings2, SlidersHorizontal, Wand2 } from 'lucide-react';
import { useRouter } from 'next/router';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useADConnection } from '../contexts/ADConnectionContext';
import { useI18n } from '../contexts/I18nContext';
import {
  buildPermissionReportPayload,
  buildPermissionRowKey,
  buildUserReportRows,
  type IndexedPermissionRow,
  type PermissionReportItem,
} from '../lib/reportPayload';
import { apiBase } from '../lib/runtimeApi';
import {
  createDefaultReportDefaults,
  readReportDefaults,
  writeReportDefaults,
  type ReportDefaults,
} from '../lib/workspaceSettings';
import ExportReportPreviewV2 from './ExportReportPreviewV2';
import PermissionReportWorkspace from './PermissionReportWorkspace';
import PermissionViewerQuickReports, { type QuickReportMode } from './PermissionViewerQuickReports';
import { Badge } from './ui/badge';
import { Button } from './ui/button';
import { Card } from './ui/card';
import { Input, NativeSelect, Textarea } from './ui/input';
import { cn } from '../lib/cn';

type ExportFormat = 'csv' | 'excel' | 'html';
type ConfigView = 'template' | 'fields' | 'delivery';
type WorkspaceView = 'results' | 'detail' | 'configure' | 'preview';

interface ScanSession {
  id: string;
  root_path: string;
  status: string;
  items_scanned: number;
  permission_count: number;
  started_at: string;
  finished_at?: string;
}

interface IdentityResolutionSummary {
  mode: string;
  resolved_principal_count?: number;
  unresolved_principal_count?: number;
}

interface SummaryTemplate {
  id: string;
  name: string;
  description: string;
  default_title: string;
  available_sections: string[];
  default_sections: string[];
}

const FALLBACK_TEMPLATES: SummaryTemplate[] = [
  {
    id: 'management',
    name: 'Management Summary',
    description: 'Operational overview with permission totals, risks and leading identities.',
    default_title: 'OpenAD - Management Summary',
    available_sections: ['metadata', 'kpis', 'top_trustees', 'top_paths', 'risk_summary'],
    default_sections: ['metadata', 'kpis', 'top_trustees', 'top_paths'],
  },
  {
    id: 'user-access',
    name: 'User Access Detail',
    description: 'Identity-first report for every reachable path and permission.',
    default_title: 'User Permission Report',
    available_sections: ['metadata', 'user_permissions', 'group_sources', 'risk_summary'],
    default_sections: ['metadata', 'user_permissions', 'group_sources'],
  },
  {
    id: 'audit-evidence',
    name: 'Audit Evidence Pack',
    description: 'ACL evidence with inheritance and risk context.',
    default_title: 'Permission Audit Evidence',
    available_sections: ['metadata', 'kpis', 'acl_evidence', 'risk_summary'],
    default_sections: ['metadata', 'kpis', 'acl_evidence', 'risk_summary'],
  },
];

const AD_FIELDS = [
  ['sam', 'Account name', '账户名'],
  ['given', 'First name', '名'],
  ['sn', 'Last name', '姓'],
  ['mail', 'Email', '邮箱'],
  ['department', 'Department', '部门'],
  ['division', 'Division', '事业部'],
  ['domain', 'Domain', '域'],
  ['originating-group', 'Originating group', '来源组'],
  ['rights', 'Permissions', '权限'],
] as const;

const FILE_FIELDS = [
  ['path', 'Path', '路径'],
  ['trustee', 'Trustee', '主体'],
  ['sid', 'SID', 'SID'],
  ['rights', 'Rights', '权限'],
  ['type', 'Allow / Deny', '允许/拒绝'],
  ['inherited', 'Inheritance', '继承'],
  ['source', 'Source', '来源'],
  ['applies-to', 'Applies to', '应用范围'],
  ['account-type', 'Account type', '账户类型'],
  ['risk-level', 'Risk level', '风险等级'],
] as const;

const DELIVERY_PRESETS = [
  { id: 'manager', en: 'Manager review', zh: '主管复核' },
  { id: 'audit', en: 'Audit evidence', zh: '审计取证' },
  { id: 'remediation', en: 'Remediation handoff', zh: '整改交接' },
] as const;

function text(locale: string, en: string, zh: string) {
  return locale === 'zh-CN' ? zh : en;
}

function firstQuery(value: string | string[] | undefined) {
  return Array.isArray(value) ? value[0] || '' : value || '';
}

function queryMode(value: string | string[] | undefined): QuickReportMode {
  const mode = firstQuery(value);
  return mode === 'folder' || mode === 'owner' ? mode : 'user';
}

function riskLevel(item: PermissionReportItem) {
  const provided = (item.risk_level || '').toLowerCase();
  if (provided === 'high' || provided === 'medium' || provided === 'low') return provided;
  const rights = (item.rights || '').toLowerCase();
  return ['full control', 'write', 'modify', 'delete', 'take ownership', 'change permissions']
    .some((token) => rights.includes(token)) ? 'high' : rights.includes('execute') ? 'medium' : 'low';
}

function compactPath(path: string) {
  const parts = path.replaceAll('/', '\\').split('\\').filter(Boolean);
  return parts.slice(-2).join('\\') || path || '-';
}

function normalizePermission(item: PermissionReportItem): PermissionReportItem {
  return {
    ...item,
    path: item.path || '',
    trustee: item.trustee || item.account_name || item.trustee_sid || '',
    trustee_sid: item.trustee_sid || '',
    rights: item.rights || '',
    type: item.type || '',
    inherited: Boolean(item.inherited),
  };
}

function identityResolutionLabel(locale: string, resolution: IdentityResolutionSummary) {
  const labels: Record<string, [string, string]> = {
    snapshot: ['Snapshot resolved', '快照解析'],
    'snapshot+ldap': ['Snapshot + LDAP', '快照 + LDAP'],
    ldap: ['LDAP resolved', 'LDAP 解析'],
    'raw-fallback': ['Raw ACL fallback', '原始 ACL 回退'],
  };
  const label = labels[resolution.mode];
  return label ? text(locale, label[0], label[1]) : resolution.mode;
}

export default function ReportCenterWorkspace() {
  const router = useRouter();
  const { locale } = useI18n();
  const { activeProfile, config, connection } = useADConnection();
  const initialDefaults = useMemo(() => createDefaultReportDefaults(), []);
  const [sessions, setSessions] = useState<ScanSession[]>([]);
  const [sessionID, setSessionID] = useState('');
  const [activeSession, setActiveSession] = useState<ScanSession | null>(null);
  const [permissions, setPermissions] = useState<PermissionReportItem[]>([]);
  const [identityResolution, setIdentityResolution] = useState<IdentityResolutionSummary | null>(null);
  const [mode, setMode] = useState<QuickReportMode>('user');
  const [trusteeFilter, setTrusteeFilter] = useState('');
  const [pathFilter, setPathFilter] = useState('');
  const [selectedPath, setSelectedPath] = useState('');
  const [selectedKey, setSelectedKey] = useState('');
  const [templates, setTemplates] = useState<SummaryTemplate[]>(FALLBACK_TEMPLATES);
  const [configView, setConfigView] = useState<ConfigView>('template');
  const [workspaceView, setWorkspaceView] = useState<WorkspaceView>('results');
  const [defaults, setDefaults] = useState<ReportDefaults>(initialDefaults);
  const [previewMarkdown, setPreviewMarkdown] = useState('');
  const [message, setMessage] = useState('');
  const [sessionsLoading, setSessionsLoading] = useState(true);
  const [datasetLoading, setDatasetLoading] = useState(false);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [exporting, setExporting] = useState<ExportFormat | ''>('');
  const [error, setError] = useState('');
  const loadedSession = useRef('');
  const defaultsReady = useRef(false);

  const requestedSession = firstQuery(router.query.session);
  const requestedPath = firstQuery(router.query.path);
  const adReady = Boolean(
    activeProfile
    || (connection.connected && config.adServer && config.baseDN && config.username && config.password),
  );
  const template = templates.find((item) => item.id === defaults.selectedTemplateID) || templates[0];

  useEffect(() => {
    if (!router.isReady) return;
    setMode(queryMode(router.query.mode));
    if (requestedSession) setSessionID(requestedSession);
    if (requestedPath) {
      setPathFilter(requestedPath);
      setSelectedPath(requestedPath);
    }
  }, [requestedPath, requestedSession, router.isReady, router.query.mode]);

  useEffect(() => {
    setDefaults(readReportDefaults(initialDefaults));
    defaultsReady.current = true;
  }, [initialDefaults]);

  useEffect(() => {
    if (defaultsReady.current) writeReportDefaults(defaults);
  }, [defaults]);

  const loadSessions = useCallback(async () => {
    setSessionsLoading(true);
    setError('');
    try {
      const response = await fetch(`${apiBase()}/api/sessions?status=completed&page=1&page_size=50`);
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || 'Failed to load scan sessions');
      const items = Array.isArray(data.items) ? data.items as ScanSession[] : [];
      setSessions(items);
      setSessionID((current) => {
        if (current && items.some((item) => item.id === current)) return current;
        return items.find((item) => item.id === requestedSession)?.id
          || items.find((item) => item.root_path === requestedPath)?.id
          || items[0]?.id
          || '';
      });
    } catch (requestError) {
      setSessions([]);
      setError(requestError instanceof Error ? requestError.message : text(locale, 'Session list unavailable.', '会话列表不可用。'));
    } finally {
      setSessionsLoading(false);
    }
  }, [locale, requestedPath, requestedSession]);

  useEffect(() => {
    void loadSessions();
    const loadTemplates = async () => {
      try {
        const response = await fetch(`${apiBase()}/api/export/summary/templates`);
        const data = await response.json();
        if (response.ok && Array.isArray(data.items) && data.items.length) {
          setTemplates(data.items as SummaryTemplate[]);
        }
      } catch {
        // Fallback templates keep the report center usable.
      }
    };
    void loadTemplates();
  }, [loadSessions]);

  const loadBundle = useCallback(async (targetSession: string) => {
    if (!targetSession) return;
    setDatasetLoading(true);
    setIdentityResolution(null);
    setError('');
    setMessage('');
    try {
      const response = await fetch(`${apiBase()}/api/sessions/${encodeURIComponent(targetSession)}/bundle`);
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || 'Failed to load scan session');
      if (!data.session) throw new Error(text(locale, 'Session metadata is missing.', '会话元数据缺失。'));
      const rows = Array.isArray(data.permissions) ? data.permissions.map(normalizePermission) : [];
      loadedSession.current = targetSession;
      setActiveSession(data.session as ScanSession);
      setPermissions(rows);
      setIdentityResolution(data.identity_resolution || null);
      setSelectedKey('');
      setSelectedPath((current) => current || requestedPath || data.session.root_path || rows[0]?.path || '');
    } catch (requestError) {
      loadedSession.current = '';
      setActiveSession(null);
      setPermissions([]);
      setIdentityResolution(null);
      setError(requestError instanceof Error ? requestError.message : text(locale, 'Session data unavailable.', '会话数据不可用。'));
    } finally {
      setDatasetLoading(false);
    }
  }, [locale, requestedPath]);

  useEffect(() => {
    if (sessionID && loadedSession.current !== sessionID) void loadBundle(sessionID);
  }, [loadBundle, sessionID]);

  const filtered = useMemo(() => {
    const trustee = trusteeFilter.trim().toLowerCase();
    const path = pathFilter.trim().toLowerCase();
    return permissions.filter((item) => {
      const identity = `${item.account_name || ''} ${item.trustee} ${item.trustee_sid}`.toLowerCase();
      return (!trustee || identity.includes(trustee)) && (!path || item.path.toLowerCase().includes(path));
    });
  }, [pathFilter, permissions, trusteeFilter]);

  const entries = useMemo<IndexedPermissionRow<PermissionReportItem>[]>(
    () => filtered.map((item, index) => ({ item, rowKey: buildPermissionRowKey(item, index) })),
    [filtered],
  );
  const userRows = useMemo(
    () => buildUserReportRows(entries, locale, defaults.rightMappings),
    [defaults.rightMappings, entries, locale],
  );
  const pathSummaries = useMemo(() => {
    const groups = new Map<string, PermissionReportItem[]>();
    filtered.forEach((item) => groups.set(item.path, [...(groups.get(item.path) || []), item]));
    return Array.from(groups.entries()).map(([path, rows]) => ({
      path,
      label: compactPath(path),
      rowCount: rows.length,
      userCount: new Set(rows.map((item) => item.account_name || item.trustee || item.trustee_sid)).size,
      highRiskCount: rows.filter((item) => riskLevel(item) === 'high').length,
      explicitCount: rows.filter((item) => !item.inherited).length,
    }));
  }, [filtered]);

  useEffect(() => {
    if (!pathSummaries.length) return setSelectedPath('');
    setSelectedPath((current) => pathSummaries.some((item) => item.path === current) ? current : pathSummaries[0].path);
  }, [pathSummaries]);

  useEffect(() => {
    if (!entries.length) return setSelectedKey('');
    setSelectedKey((current) => entries.some((item) => item.rowKey === current) ? current : entries[0].rowKey);
  }, [entries]);

  const selectedPathSummary = pathSummaries.find((item) => item.path === selectedPath) || null;
  const selectedEntry = entries.find((item) => item.rowKey === selectedKey) || entries[0] || null;
  const selectedUserRow = userRows.find((item) => item.member_keys.includes(selectedKey)) || userRows[0] || null;
  const rootPath = activeSession?.root_path || requestedPath || permissions[0]?.path || '';
  const stats = {
    total: permissions.length,
    visible: filtered.length,
    highRisk: filtered.filter((item) => riskLevel(item) === 'high').length,
    skipped: 0,
  };
  const delivery = DELIVERY_PRESETS.find((item) => item.id === defaults.sharePresetID) || DELIVERY_PRESETS[0];

  const payload = () => buildPermissionReportPayload({
    entries,
    source: activeSession?.id || 'SESSION',
    locale,
    exportMode: defaults.exportMode,
    title: defaults.reportTitle || template?.default_title || 'Permission Report',
    template: template?.id || defaults.selectedTemplateID,
    sections: defaults.selectedSections,
    organization: defaults.organization,
    preparedBy: defaults.preparedBy,
    reportPeriod: defaults.reportPeriod,
    focusAreas: defaults.rightMappings.slice(0, 3).map((item) => locale === 'zh-CN' ? item.translatedZh : item.translatedEn),
    adFields: defaults.adFieldIDs,
    fileColumns: defaults.fileColumnIDs,
    rightMappings: defaults.rightMappings,
  });

  const generatePreview = async () => {
    if (!entries.length) return;
    setPreviewLoading(true);
    setMessage('');
    try {
      const response = await fetch(`${apiBase()}/api/export/summary`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload()),
      });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || 'Preview generation failed');
      setPreviewMarkdown(data.markdown || '');
      if (data.title) setDefaults((current) => ({ ...current, reportTitle: data.title }));
      setMessage(text(locale, 'Preview ready', '预览已生成'));
      setWorkspaceView('preview');
    } catch (requestError) {
      setMessage(requestError instanceof Error ? requestError.message : text(locale, 'Preview generation failed.', '预览生成失败。'));
    } finally {
      setPreviewLoading(false);
    }
  };

  const exportReport = async (format: ExportFormat) => {
    if (!entries.length) return;
    setExporting(format);
    setMessage('');
    try {
      const response = await fetch(`${apiBase()}/api/export/download`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...payload(), format, filename: `permission-report-${Date.now()}` }),
      });
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.error || 'Export failed');
      }
      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `permission-report-${Date.now()}.${format === 'excel' ? 'xlsx' : format}`;
      link.click();
      window.URL.revokeObjectURL(url);
      setMessage(text(locale, 'Export created', '导出已生成'));
    } catch (requestError) {
      setMessage(requestError instanceof Error ? requestError.message : text(locale, 'Export failed.', '导出失败。'));
    } finally {
      setExporting('');
    }
  };

  const changeMode = (next: QuickReportMode) => {
    setMode(next);
    const params = new URLSearchParams({ mode: next });
    if (sessionID) params.set('session', sessionID);
    if (rootPath) params.set('path', rootPath);
    void router.replace(`/reports?${params.toString()}`, undefined, { shallow: true });
  };

  const updateDefaults = <K extends keyof ReportDefaults>(key: K, value: ReportDefaults[K]) => {
    setDefaults((current) => ({ ...current, [key]: value }));
  };

  const toggleValue = (key: 'selectedSections' | 'adFieldIDs' | 'fileColumnIDs', value: string) => {
    const values = defaults[key];
    updateDefaults(key, values.includes(value) ? values.filter((item) => item !== value) : [...values, value]);
  };

  const chooseTemplate = (id: string) => {
    const next = templates.find((item) => item.id === id);
    setDefaults((current) => ({
      ...current,
      selectedTemplateID: id,
      reportTitle: next?.default_title || current.reportTitle,
      selectedSections: next?.default_sections || current.selectedSections,
    }));
  };

  const workspaceTabs = [
    ['results', FileText, text(locale, 'Report results', '报告结果')],
    ['detail', ListTree, text(locale, 'Permission detail', '权限明细')],
    ['configure', Settings2, text(locale, 'Configure report', '报告配置')],
    ['preview', Eye, text(locale, 'Preview & export', '预览导出')],
  ] as const;

  return (
    <div className="report-center-workbench app-v2">
      <header className="report-center-header">
        <div>
          <div className="text-2xs font-medium uppercase tracking-wide text-accent">{text(locale, 'Permission reporting', '权限报告')}</div>
          <h1 className="mt-1 text-lg font-semibold text-fg">{text(locale, 'Report Center', '报告中心')}</h1>
          <p className="mt-1 max-w-3xl text-sm text-fg-muted">
            {text(locale, 'Create user, folder and owner reports from the latest scan or any completed historical session.', '从最近扫描或任一已完成历史会话生成用户、文件夹和所有者报告。')}
          </p>
        </div>
        <div className="flex flex-wrap justify-end gap-2">
          <Badge tone={permissions.length ? 'success' : 'neutral'}>{permissions.length ? text(locale, `${permissions.length} permission rows`, `${permissions.length} 条权限记录`) : text(locale, 'Awaiting dataset', '等待数据集')}</Badge>
          <Badge tone={adReady ? 'success' : 'warning'}>{adReady ? text(locale, 'AD context ready', 'AD 上下文就绪') : text(locale, 'AD context optional', 'AD 上下文可选')}</Badge>
          {identityResolution
            ? <Badge tone={identityResolution.mode === 'raw-fallback' ? 'warning' : 'success'}>{identityResolutionLabel(locale, identityResolution)}</Badge>
            : null}
        </div>
      </header>

      <Card className="report-center-source-panel p-3">
        <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-end">
          <label className="min-w-0">
              <span className="mb-1 block text-xs font-medium text-fg-secondary">{text(locale, 'Scan data source', '扫描数据源')}</span>
              <NativeSelect
                aria-label={text(locale, 'Scan data source', '扫描数据源')}
                value={sessionID}
                disabled={sessionsLoading}
                onChange={(event) => {
                  loadedSession.current = '';
                  setIdentityResolution(null);
                  setSessionID(event.target.value);
                }}
              >
                <option value="">{text(locale, 'Choose a completed scan', '选择已完成扫描')}</option>
                {sessions.map((session) => <option key={session.id} value={session.id}>{session.root_path} · {new Date(session.finished_at || session.started_at).toLocaleString()}</option>)}
              </NativeSelect>
          </label>
          <Button type="button" variant="secondary" loading={sessionsLoading || datasetLoading} onClick={() => void loadSessions()}>
            <RefreshCw className="h-4 w-4" />{text(locale, 'Refresh sessions', '刷新会话')}
          </Button>
        </div>
        {error ? <p className="mt-3 text-xs text-danger">{error}</p> : null}
      </Card>

      <div className="report-center-workspace-tabs" role="tablist" aria-label={text(locale, 'Report workspace', '报告工作区')}>
        {workspaceTabs.map(([value, Icon, label]) => (
          <button
            key={value}
            type="button"
            role="tab"
            aria-selected={workspaceView === value}
            className={cn('report-center-workspace-tab', workspaceView === value && 'is-active')}
            onClick={() => setWorkspaceView(value)}
          >
            <Icon className="h-4 w-4" aria-hidden />
            <span>{label}</span>
          </button>
        ))}
      </div>

      <div className="report-center-stage">
        {workspaceView === 'results' ? (
          <div className="report-center-pane">
            <PermissionViewerQuickReports
              locale={locale}
              permissions={filtered}
              embedded
              adReady={adReady}
              exactPathCount={filtered.filter((item) => item.path === rootPath).length}
              exportReady={Boolean(permissions.length)}
              mode={mode}
              onModeChange={changeMode}
              onApplyTrusteeFilter={setTrusteeFilter}
              onApplyPathFilter={setPathFilter}
              onClearFilters={() => { setTrusteeFilter(''); setPathFilter(''); }}
              onOpenReportControls={() => setWorkspaceView('configure')}
              onOpenHistory={() => void router.push('/history')}
            />
          </div>
        ) : null}

        {workspaceView === 'detail' ? (
          <div className="report-center-pane">
            {permissions.length ? (
              <section aria-labelledby="report-user-detail">
                <h2 id="report-user-detail" className="flex items-center gap-2 text-sm font-semibold text-fg"><ListTree className="h-4 w-4 text-accent" />{text(locale, 'User permission detail', '用户权限明细')}</h2>
                <PermissionReportWorkspace
                  locale={locale}
                  filteredResultsCount={filtered.length}
                  resultPathSummaries={pathSummaries}
                  selectedResultPath={selectedPath}
                  selectedPathSummary={selectedPathSummary}
                  selectedPathTokens={selectedPath.replaceAll('/', '\\').split('\\').filter(Boolean)}
                  adReady={adReady}
                  compactTables
                  selectedPermission={selectedEntry?.item || null}
                  selectedUserReportRow={selectedUserRow}
                  userReportRows={userRows.filter((row) => !selectedPath || row.path === selectedPath)}
                  onClearFilters={() => { setTrusteeFilter(''); setPathFilter(''); }}
                  onSelectResultPath={setSelectedPath}
                  onSelectPermissionKey={setSelectedKey}
                  onOpenDetail={() => setWorkspaceView('preview')}
                  riskBadgeClass={() => ''}
                />
              </section>
            ) : (
              <div className="rounded-lg border border-dashed border-line px-5 py-10 text-center text-sm text-fg-muted">{text(locale, 'Choose a completed scan to inspect permission details.', '请选择已完成扫描以查看权限明细。')}</div>
            )}
          </div>
        ) : null}

        {workspaceView === 'configure' ? (
          <div className="report-center-pane">
            <Card id="report-configuration" className="overflow-hidden p-0">
        <div className="flex flex-wrap items-start justify-between gap-3 border-b border-line p-4 sm:px-5">
          <div>
            <h2 className="flex items-center gap-2 text-sm font-semibold text-fg"><Settings2 className="h-4 w-4 text-accent" />{text(locale, 'Report configuration', '报告配置')}</h2>
            <p className="mt-1 text-xs text-fg-muted">{text(locale, 'Choose the template, scope and fields before previewing.', '预览前选择模板、范围与字段。')}</p>
          </div>
          <div className="inline-flex rounded-md border border-line bg-surface-sunken p-1">
            {(['management-summary', 'scan-results'] as const).map((value) => (
              <button key={value} type="button" onClick={() => updateDefaults('exportMode', value)} className={cn('rounded px-2.5 py-1 text-xs', defaults.exportMode === value ? 'bg-surface-base text-fg shadow-token-sm' : 'text-fg-muted')}>
                {value === 'management-summary' ? text(locale, 'Summary', '摘要') : text(locale, 'Evidence rows', '证据明细')}
              </button>
            ))}
          </div>
        </div>
        <div className="flex border-b border-line bg-surface-sunken px-3">
          {([
            ['template', text(locale, 'Template & scope', '模板与范围')],
            ['fields', text(locale, 'Output fields', '输出字段')],
            ['delivery', text(locale, 'Delivery', '交付设置')],
          ] as const).map(([value, label]) => (
            <button key={value} type="button" onClick={() => setConfigView(value)} className={cn('border-b-2 px-3 py-2.5 text-xs font-medium', configView === value ? 'border-accent text-fg' : 'border-transparent text-fg-muted')}>{label}</button>
          ))}
        </div>

        <div className="p-4 sm:p-5">
          {configView === 'template' ? (
            <div className="grid gap-4 xl:grid-cols-[minmax(260px,0.85fr)_minmax(0,1.15fr)]">
              <div className="space-y-3">
                <label><span className="mb-1 block text-xs font-medium text-fg-secondary">{text(locale, 'Report template', '报告模板')}</span><NativeSelect value={defaults.selectedTemplateID} onChange={(event) => chooseTemplate(event.target.value)}>{templates.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</NativeSelect></label>
                <p className="text-xs leading-5 text-fg-muted">{template?.description}</p>
                <label><span className="mb-1 block text-xs font-medium text-fg-secondary">{text(locale, 'Report title', '报告标题')}</span><Input value={defaults.reportTitle} onChange={(event) => updateDefaults('reportTitle', event.target.value)} /></label>
                <div className="grid gap-3 sm:grid-cols-2">
                  <label><span className="mb-1 block text-xs font-medium text-fg-secondary">{text(locale, 'Organization', '组织')}</span><Input value={defaults.organization} onChange={(event) => updateDefaults('organization', event.target.value)} /></label>
                  <label><span className="mb-1 block text-xs font-medium text-fg-secondary">{text(locale, 'Prepared by', '制表人')}</span><Input value={defaults.preparedBy} onChange={(event) => updateDefaults('preparedBy', event.target.value)} /></label>
                </div>
                <label><span className="mb-1 block text-xs font-medium text-fg-secondary">{text(locale, 'Report period', '报告周期')}</span><Input value={defaults.reportPeriod} onChange={(event) => updateDefaults('reportPeriod', event.target.value)} /></label>
              </div>
              <div>
                <div className="text-xs font-medium text-fg-secondary">{text(locale, 'Sections & current scope', '章节与当前范围')}</div>
                <div className="mt-2 grid gap-2 sm:grid-cols-2">{(template?.available_sections || []).map((section) => <CheckOption key={section} label={section.replaceAll('_', ' ')} checked={defaults.selectedSections.includes(section)} onChange={() => toggleValue('selectedSections', section)} />)}</div>
                <div className="mt-4 rounded-md border border-line bg-surface-sunken p-3">
                  <div className="break-all font-mono text-xs text-fg">{rootPath || '-'}</div>
                  <div className="mt-2 text-2xs text-fg-muted">{text(locale, `${filtered.length} of ${permissions.length} rows selected`, `已选择 ${filtered.length}/${permissions.length} 条记录`)}</div>
                </div>
              </div>
            </div>
          ) : null}

          {configView === 'fields' ? (
            <div className="grid gap-5 xl:grid-cols-2">
              <FieldGroup title={text(locale, 'Identity fields', '身份字段')} locale={locale} options={AD_FIELDS} values={defaults.adFieldIDs} onToggle={(id) => toggleValue('adFieldIDs', id)} />
              <FieldGroup title={text(locale, 'File fields', '文件字段')} locale={locale} options={FILE_FIELDS} values={defaults.fileColumnIDs} onToggle={(id) => toggleValue('fileColumnIDs', id)} />
            </div>
          ) : null}

          {configView === 'delivery' ? (
            <div className="grid gap-4 lg:grid-cols-[260px_minmax(0,1fr)]">
              <div className="space-y-2">{DELIVERY_PRESETS.map((item) => <button key={item.id} type="button" onClick={() => updateDefaults('sharePresetID', item.id)} className={cn('w-full rounded-md border px-3 py-2.5 text-left text-xs', defaults.sharePresetID === item.id ? 'border-accent/50 bg-accent-soft text-fg' : 'border-line text-fg-secondary')}>{locale === 'zh-CN' ? item.zh : item.en}</button>)}</div>
              <label><span className="mb-1 block text-xs font-medium text-fg-secondary">{text(locale, 'Operator notes', '操作备注')}</span><Textarea className="min-h-[148px]" value={defaults.shareNote} onChange={(event) => updateDefaults('shareNote', event.target.value)} /></label>
            </div>
          ) : null}

          <div className="mt-5 flex flex-wrap items-center gap-2 border-t border-line pt-4">
            <Button type="button" loading={previewLoading} disabled={!entries.length} onClick={() => void generatePreview()}><Wand2 className="h-4 w-4" />{text(locale, 'Generate preview', '生成预览')}</Button>
            <span className="text-xs text-fg-muted">{message}</span>
          </div>
        </div>
            </Card>
          </div>
        ) : null}

        {workspaceView === 'preview' ? (
          <div className="report-center-pane">
            <section id="report-preview" aria-labelledby="report-preview-title">
        <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 id="report-preview-title" className="flex items-center gap-2 text-sm font-semibold text-fg"><SlidersHorizontal className="h-4 w-4 text-accent" />{text(locale, 'Preview & export', '预览与导出')}</h2>
            <p className="mt-1 text-xs text-fg-muted">{previewMarkdown ? text(locale, 'Preview ready', '预览已生成') : text(locale, 'Live preview uses the current dataset and configuration.', '实时预览使用当前数据集和配置。')}</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <ExportButton label={text(locale, 'Export CSV', '导出 CSV')} icon={Download} loading={exporting === 'csv'} disabled={!entries.length} onClick={() => void exportReport('csv')} />
            <ExportButton label={text(locale, 'Export Excel', '导出 Excel')} icon={FileSpreadsheet} loading={exporting === 'excel'} disabled={!entries.length} onClick={() => void exportReport('excel')} />
            <ExportButton label={text(locale, 'Export HTML', '导出 HTML')} icon={FileCode2} loading={exporting === 'html'} disabled={!entries.length} onClick={() => void exportReport('html')} />
          </div>
        </div>
        {entries.length ? (
          <ExportReportPreviewV2
            locale={locale}
            reportTitle={defaults.reportTitle}
            templateTitle={template?.name || defaults.selectedTemplateID}
            templateSummary={template?.description || ''}
            previewGeneratedAt={new Date().toLocaleString()}
            normalizedPath={rootPath}
            modeSummary={mode === 'user' ? text(locale, 'User permissions', '用户权限') : mode === 'folder' ? text(locale, 'Folder permissions', '文件夹权限') : text(locale, 'Owner review', '责任人复核')}
            sharePresetLabel={locale === 'zh-CN' ? delivery.zh : delivery.en}
            stats={stats}
            previewUserRows={userRows.slice(0, 12)}
            previewAclRows={filtered.slice(0, 20)}
            previewNarrativeLead={previewMarkdown.split('\n').find((line) => line.trim() && !line.startsWith('#')) || text(locale, 'Permission evidence is ready for review.', '权限证据已可复核。')}
            previewNarrativeBullets={[
              text(locale, `${stats.highRisk} high-risk rows in the current scope`, `当前范围有 ${stats.highRisk} 条高风险记录`),
              text(locale, `${pathSummaries.length} paths represented`, `覆盖 ${pathSummaries.length} 个路径`),
            ]}
          />
        ) : (
          <div className="rounded-lg border border-dashed border-line px-5 py-10 text-center text-sm text-fg-muted">{text(locale, 'Choose a completed scan to preview a report.', '请选择已完成扫描以预览报告。')}</div>
        )}
            </section>
          </div>
        ) : null}
      </div>
    </div>
  );
}

function CheckOption({ label, checked, onChange }: { label: string; checked: boolean; onChange: () => void }) {
  return <label className="flex items-center gap-2 rounded-md border border-line px-3 py-2 text-xs text-fg-secondary"><input type="checkbox" checked={checked} onChange={onChange} className="h-4 w-4 accent-accent" /><span>{label}</span></label>;
}

function FieldGroup({ title, locale, options, values, onToggle }: { title: string; locale: string; options: ReadonlyArray<readonly [string, string, string]>; values: string[]; onToggle: (id: string) => void }) {
  return <div><h3 className="text-xs font-medium text-fg-secondary">{title}</h3><div className="mt-2 grid gap-2 sm:grid-cols-2">{options.map(([id, en, zh]) => <CheckOption key={id} label={locale === 'zh-CN' ? zh : en} checked={values.includes(id)} onChange={() => onToggle(id)} />)}</div></div>;
}

function ExportButton({ label, icon: Icon, loading, disabled, onClick }: { label: string; icon: typeof FileText; loading: boolean; disabled: boolean; onClick: () => void }) {
  return <Button type="button" variant="secondary" size="sm" loading={loading} disabled={disabled} onClick={onClick}><Icon className="h-3.5 w-3.5" />{label}</Button>;
}
