import Head from 'next/head';
import { useRouter } from 'next/router';
import { useEffect, useMemo, useState } from 'react';
import { Activity, AlertTriangle, CheckCircle2, Copy, Eye, KeyRound, Network, Pencil, RefreshCw, Trash2 } from 'lucide-react';
import { useADConnection } from '../contexts/ADConnectionContext';
import { useI18n } from '../contexts/I18nContext';
import { apiBase } from '../lib/runtimeApi';
import { buildWorkspacePageTitle, defaultWorkspaceSettings, readWorkspaceSettings, type WorkspaceSettings } from '../lib/workspaceSettings';
import { Button } from '../components/ui/button';
import { Card, CardHeader, CardContent } from '../components/ui/card';
import { Badge, type BadgeTone } from '../components/ui/badge';
import { Input, NativeSelect } from '../components/ui/input';
import { Skeleton } from '../components/ui/skeleton';
import { EmptyState } from '../components/ui/empty-state';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table';

interface FileActivityEvent {
  event_id: number;
  action: string;
  user: string;
  raw_user?: string;
  user_sid?: string;
  user_display?: string;
  user_dn?: string;
  user_type?: string;
  resolution: string;
  domain: string;
  path: string;
  object_type: string;
  access_mask: string;
  access_list: string;
  process_name: string;
  client_ip?: string;
  computer: string;
  timestamp: string;
  source: string;
}

interface FileActivitySummary {
  total: number;
  read: number;
  write: number;
  delete: number;
  permission_changes: number;
  share_access: number;
  other: number;
}

interface FileActivitySource {
  provider: string;
  scope: string;
  hours: number;
  limit: number;
  content_scanning: boolean;
  requires: string;
  remote_note: string;
  ad_resolution: string;
  ad_resolved: number;
  ad_unresolved: number;
  ad_warning?: string;
}

interface FileActivityReadinessCheck {
  id: string;
  label: string;
  status: string;
  detail: string;
  remediation?: string;
}

interface FileActivitySetupCommand {
  id: string;
  title: string;
  description: string;
  command: string;
  requires: string;
}

interface FileActivityReadiness {
  status: string;
  host_os: string;
  current_user: string;
  target_path?: string;
  generated_at: string;
  checks: FileActivityReadinessCheck[];
  commands: FileActivitySetupCommand[];
  next_steps: string[];
}

const emptySummary: FileActivitySummary = {
  total: 0,
  read: 0,
  write: 0,
  delete: 0,
  permission_changes: 0,
  share_access: 0,
  other: 0,
};

function text(locale: string, en: string, zh: string) {
  return locale === 'zh-CN' ? zh : en;
}

function formatNumber(value: number, locale: string) {
  return new Intl.NumberFormat(locale === 'zh-CN' ? 'zh-CN' : 'en-US').format(value);
}

function formatDateTime(value: string, locale: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value || '-';
  }
  return new Intl.DateTimeFormat(locale === 'zh-CN' ? 'zh-CN' : 'en-US', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date);
}

function actionLabel(action: string, locale: string) {
  switch (action) {
    case 'read':
      return text(locale, 'Read', '读取');
    case 'write':
      return text(locale, 'Write / Modify', '写入/修改');
    case 'delete':
      return text(locale, 'Delete', '删除');
    case 'permission-change':
      return text(locale, 'Permission Change', '权限变更');
    case 'share-access':
      return text(locale, 'Share Access', '共享访问');
    default:
      return text(locale, 'Other', '其他');
  }
}

function actionTone(action: string): BadgeTone {
  switch (action) {
    case 'write':
    case 'delete':
    case 'permission-change':
      return 'warning';
    case 'read':
      return 'info';
    case 'share-access':
      return 'success';
    default:
      return 'neutral';
  }
}

function actionIcon(action: string) {
  switch (action) {
    case 'read':
      return Eye;
    case 'write':
      return Pencil;
    case 'delete':
      return Trash2;
    case 'permission-change':
      return KeyRound;
    case 'share-access':
      return Network;
    default:
      return Activity;
  }
}

function readinessTone(status: string): BadgeTone {
  switch (status) {
    case 'ok':
      return 'success';
    case 'warning':
      return 'warning';
    case 'fail':
      return 'danger';
    default:
      return 'info';
  }
}

function readinessLabel(status: string, locale: string) {
  switch (status) {
    case 'ok':
      return text(locale, 'Ready', '就绪');
    case 'warning':
      return text(locale, 'Needs attention', '需要关注');
    case 'fail':
      return text(locale, 'Blocked', '被阻塞');
    default:
      return text(locale, 'Check needed', '需要检查');
  }
}

export default function FileActivityPage() {
  const { t, locale } = useI18n();
  const router = useRouter();
  const { config, connection } = useADConnection();
  const [workspaceSettings, setWorkspaceSettings] = useState<WorkspaceSettings>(defaultWorkspaceSettings);
  const [items, setItems] = useState<FileActivityEvent[]>([]);
  const [summary, setSummary] = useState<FileActivitySummary>(emptySummary);
  const [source, setSource] = useState<FileActivitySource | null>(null);
  const [pathFilter, setPathFilter] = useState('');
  const [userFilter, setUserFilter] = useState('');
  const [actionFilter, setActionFilter] = useState('');
  const [hours, setHours] = useState(24);
  const [limit, setLimit] = useState(100);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [readiness, setReadiness] = useState<FileActivityReadiness | null>(null);
  const [readinessPath, setReadinessPath] = useState('');
  const [readinessLoading, setReadinessLoading] = useState(false);
  const [readinessError, setReadinessError] = useState('');
  const [copiedCommand, setCopiedCommand] = useState('');

  const activeFilterCount = useMemo(
    () => [pathFilter.trim(), userFilter.trim(), actionFilter.trim()].filter(Boolean).length,
    [actionFilter, pathFilter, userFilter]
  );
  const adResolutionReady = Boolean(connection.connected && config.adServer && config.baseDN && config.username && config.password);
  const adResolutionConfigured = Boolean(connection.connected && config.adServer && config.baseDN && config.username);

  const fetchReadiness = async (targetPath = readinessPath) => {
    setReadinessLoading(true);
    setReadinessError('');
    try {
      const search = new URLSearchParams();
      if (targetPath.trim()) {
        search.set('path', targetPath.trim());
      }
      const response = await fetch(`${apiBase()}/api/file-activity/readiness${search.toString() ? `?${search.toString()}` : ''}`);
      const data = await response.json();
      if (!response.ok) {
        throw new Error(data.error || t('loadFailed'));
      }
      setReadiness(data);
    } catch (requestError) {
      setReadinessError(requestError instanceof Error ? requestError.message : t('loadFailed'));
    } finally {
      setReadinessLoading(false);
    }
  };

  const copySetupCommand = async (command: FileActivitySetupCommand) => {
    try {
      await navigator.clipboard.writeText(command.command);
      setCopiedCommand(command.id);
      window.setTimeout(() => setCopiedCommand(''), 1600);
    } catch {
      setCopiedCommand('');
    }
  };

  const fetchActivity = async () => {
    setLoading(true);
    setError('');
    try {
      const response = await fetch(`${apiBase()}/api/file-activity/events/query`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          hours,
          limit,
          path: pathFilter.trim(),
          user: userFilter.trim(),
          action: actionFilter.trim(),
          ad_resolution: {
            enabled: adResolutionReady,
            server: config.adServer,
            base_dn: config.baseDN,
            username: config.username,
            password: adResolutionReady ? config.password : '',
          },
        }),
      });
      const data = await response.json();
      setSource(data.source || null);
      if (!response.ok) {
        throw new Error(data.error || t('loadFailed'));
      }
      setItems(data.items || []);
      setSummary(data.summary || emptySummary);
    } catch (requestError) {
      setItems([]);
      setSummary(emptySummary);
      setError(requestError instanceof Error ? requestError.message : t('loadFailed'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    setWorkspaceSettings(readWorkspaceSettings());
    void fetchReadiness('');
    void fetchActivity();
  }, []);

  return (
    <>
      <Head>
        <title>{buildWorkspacePageTitle(text(locale, 'File Activity', '文件访问活动'), workspaceSettings, t('appTitle'))}</title>
      </Head>

      <div className="app-v2 mx-auto max-w-6xl space-y-5">
        {/* Page header */}
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <h1 className="text-lg font-semibold text-fg">{text(locale, 'File Activity', '文件访问活动')}</h1>
            <p className="mt-0.5 text-sm text-fg-muted">
              {text(locale, 'Windows Security event metadata for file access. Records access behavior only, never file contents.', '查询 Windows Security 事件元数据中的文件访问记录，只审计访问行为，不读取文件内容。')}
            </p>
            <div className="mt-2 flex flex-wrap items-center gap-1.5">
              <Badge tone={readiness ? readinessTone(readiness.status) : 'info'}>
                {readiness ? readinessLabel(readiness.status, locale) : text(locale, 'Readiness unknown', '就绪状态未知')}
              </Badge>
              <Badge tone={adResolutionReady ? 'success' : adResolutionConfigured ? 'warning' : 'info'}>
                {adResolutionReady
                  ? text(locale, 'AD names on', 'AD 名称解析开启')
                  : adResolutionConfigured
                    ? text(locale, 'AD password needed', '需要 AD 密码')
                    : text(locale, 'AD not connected', 'AD 未连接')}
              </Badge>
              <Badge tone="neutral">{text(locale, 'No content read', '不读取内容')}</Badge>
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <Button variant="secondary" size="sm" onClick={() => void router.push('/ad-workspace')}>
              {text(locale, 'Connect AD', '连接 AD')}
            </Button>
            <Button size="sm" onClick={() => void fetchActivity()} disabled={loading}>
              <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
              {text(locale, 'Refresh Activity', '刷新访问活动')}
            </Button>
          </div>
        </div>

        {/* Setup check */}
        <Card>
          <CardHeader
            title={text(locale, 'Setup Check', '配置检查')}
            description={text(locale, 'Security log access, audit policy, and folder SACLs must be in place before file activity can show data.', '文件访问记录出现前必须满足：Security 日志权限、审计策略、目录 SACL。')}
            actions={
              <div className="flex items-center gap-2">
                <Input
                  value={readinessPath}
                  onChange={(event) => setReadinessPath(event.target.value)}
                  placeholder={text(locale, 'Optional target path, e.g. C:\\Shares\\Finance', '可选目标路径，例如 C:\\Shares\\Finance')}
                  className="hidden w-64 sm:block"
                />
                <Button variant="secondary" size="sm" onClick={() => void fetchReadiness()} disabled={readinessLoading}>
                  <RefreshCw className={`h-3.5 w-3.5 ${readinessLoading ? 'animate-spin' : ''}`} />
                  {text(locale, 'Check', '检查')}
                </Button>
              </div>
            }
          />
          <CardContent>
            {readinessError ? (
              <div className="mb-3 flex items-start gap-2 rounded-lg border border-danger/40 bg-danger-soft px-4 py-2.5 text-sm text-danger">
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                <div>
                  <p className="font-medium">{text(locale, 'Readiness check failed', '就绪检查失败')}</p>
                  <p className="mt-0.5 text-xs">{readinessError}</p>
                </div>
              </div>
            ) : null}

            <div className="grid gap-4 xl:grid-cols-[minmax(0,1.15fr)_minmax(20rem,0.85fr)]">
              {/* Current readiness */}
              <div className="overflow-hidden rounded-lg border border-line">
                <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line bg-surface-raised px-4 py-2.5">
                  <div className="min-w-0">
                    <p className="text-sm font-medium text-fg">{text(locale, 'Current readiness', '当前就绪状态')}</p>
                    <p className="mt-0.5 truncate text-xs text-fg-muted">
                      {readiness?.current_user || '-'} · {readiness?.target_path || text(locale, 'No target path provided', '未提供目标路径')}
                    </p>
                  </div>
                  <Badge tone={readiness ? readinessTone(readiness.status) : 'info'}>
                    {readiness ? readinessLabel(readiness.status, locale) : text(locale, 'Checking', '检查中')}
                  </Badge>
                </div>
                <div className="divide-y divide-line">
                  {(readiness?.checks || []).map((check) => (
                    <div key={check.id} className="grid gap-2 px-4 py-2.5 md:grid-cols-[minmax(11rem,0.35fr)_minmax(0,1fr)_auto] md:items-start">
                      <div className="text-xs font-medium text-fg">{check.label}</div>
                      <div className="text-2xs leading-4 text-fg-muted">
                        <div>{check.detail}</div>
                        {check.remediation ? <div className="mt-1 text-fg-secondary">{check.remediation}</div> : null}
                      </div>
                      <Badge tone={readinessTone(check.status)}>{readinessLabel(check.status, locale)}</Badge>
                    </div>
                  ))}
                </div>
              </div>

              {/* Setup commands */}
              <div className="rounded-lg border border-line px-4 py-3">
                <p className="text-sm font-medium text-fg">{text(locale, 'If a check fails', '如果检查失败')}</p>
                <p className="mt-0.5 text-xs leading-4 text-fg-muted">
                  {text(locale, 'Run the relevant command on the file server with administrator rights, then come back and check again.', '在文件服务器上用管理员权限执行对应命令，然后回来重新检查。')}
                </p>
                <div className="mt-3 space-y-2">
                  {(readiness?.commands || []).map((command) => (
                    <details key={command.id} className="rounded-md border border-line bg-surface-base px-3 py-2">
                      <summary className="cursor-pointer text-xs font-medium text-fg">{command.title}</summary>
                      <div className="mt-2 text-2xs leading-4 text-fg-muted">{command.description}</div>
                      <pre className="mt-2 max-h-44 overflow-auto whitespace-pre-wrap rounded-md border border-line bg-surface-sunken p-3 font-mono text-2xs leading-4 text-fg">{command.command}</pre>
                      <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-2xs text-fg-muted">
                        <span>{text(locale, 'Requires', '需要')}: {command.requires}</span>
                        <Button variant="ghost" size="sm" onClick={() => void copySetupCommand(command)}>
                          {copiedCommand === command.id ? <CheckCircle2 className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                          {copiedCommand === command.id ? text(locale, 'Copied', '已复制') : text(locale, 'Copy', '复制')}
                        </Button>
                      </div>
                    </details>
                  ))}
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Access ledger */}
        <Card>
          <CardHeader
            title={text(locale, 'Access Ledger', '访问台账')}
            description={text(locale, 'Filter by path, user, action, and time window.', '按路径、用户、行为和时间窗口过滤。')}
          />

          {/* Filters */}
          <div className="flex flex-wrap items-center gap-2 border-b border-line px-4 pb-3">
            <Input
              value={pathFilter}
              onChange={(event) => setPathFilter(event.target.value)}
              placeholder={text(locale, 'Path contains...', '路径包含...')}
              className="w-48 flex-1 sm:flex-none"
            />
            <Input
              value={userFilter}
              onChange={(event) => setUserFilter(event.target.value)}
              placeholder={text(locale, 'User contains...', '用户包含...')}
              className="w-44 flex-1 sm:flex-none"
            />
            <NativeSelect
              aria-label={text(locale, 'Action', '行为')}
              value={actionFilter}
              onChange={(event) => setActionFilter(event.target.value)}
              className="w-44"
            >
              <option value="">{text(locale, 'Any action', '任意行为')}</option>
              <option value="read">{actionLabel('read', locale)}</option>
              <option value="write">{actionLabel('write', locale)}</option>
              <option value="delete">{actionLabel('delete', locale)}</option>
              <option value="permission-change">{actionLabel('permission-change', locale)}</option>
              <option value="share-access">{actionLabel('share-access', locale)}</option>
            </NativeSelect>
            <NativeSelect
              aria-label={text(locale, 'Time window', '时间窗口')}
              value={hours}
              onChange={(event) => setHours(Number(event.target.value))}
              className="w-36"
            >
              <option value={1}>{text(locale, 'Last hour', '最近 1 小时')}</option>
              <option value={24}>{text(locale, 'Last 24h', '最近 24 小时')}</option>
              <option value={72}>{text(locale, 'Last 3d', '最近 3 天')}</option>
              <option value={168}>{text(locale, 'Last 7d', '最近 7 天')}</option>
            </NativeSelect>
            <NativeSelect
              aria-label={text(locale, 'Limit', '数量上限')}
              value={limit}
              onChange={(event) => setLimit(Number(event.target.value))}
              className="w-24"
            >
              <option value={50}>50</option>
              <option value={100}>100</option>
              <option value={200}>200</option>
              <option value={500}>500</option>
            </NativeSelect>
            <Button variant="secondary" size="sm" onClick={() => void fetchActivity()} disabled={loading}>
              <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
              {text(locale, 'Apply filters', '应用筛选')}
            </Button>
          </div>

          {/* Summary strip */}
          <div className="flex flex-wrap items-center gap-1.5 border-b border-line px-4 py-2.5">
            <Badge tone="neutral">{text(locale, 'Events', '事件')}: {formatNumber(summary.total, locale)}</Badge>
            <Badge tone="info">{text(locale, 'Reads', '读取')}: {formatNumber(summary.read, locale)}</Badge>
            <Badge tone="warning">
              {text(locale, 'Write/Delete', '写入/删除')}: {formatNumber(summary.write + summary.delete + summary.permission_changes, locale)}
            </Badge>
            <Badge tone="neutral">
              {activeFilterCount === 0
                ? text(locale, 'Open filter scope', '开放过滤范围')
                : text(locale, `${activeFilterCount} active filters`, `${activeFilterCount} 个活动过滤器`)}
            </Badge>
            <Badge tone="neutral">{text(locale, `${formatNumber(hours, locale)}h window`, `${formatNumber(hours, locale)} 小时时间窗`)}</Badge>
            <Badge tone={adResolutionReady ? 'success' : 'warning'}>
              {adResolutionReady ? text(locale, 'AD SID resolution active', 'AD SID 解析已启用') : text(locale, 'AD SID resolution not active', 'AD SID 解析未启用')}
            </Badge>
          </div>

          {error ? (
            <div className="px-4 pt-4">
              <div className="flex items-start gap-2 rounded-lg border border-danger/40 bg-danger-soft px-4 py-2.5 text-sm text-danger">
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                <div>
                  <p className="font-medium">{text(locale, 'File activity audit is not ready yet', '文件访问审计尚未就绪')}</p>
                  <p className="mt-0.5 text-xs">
                    {error}
                    <br />
                    {source?.requires || text(locale, 'Enable Windows Object Access auditing, configure SACLs on target folders, and run with Security log read permission.', '启用 Windows 对象访问审计，在目标目录配置 SACL，并用具备 Security 日志读取权限的身份运行。')}
                  </p>
                </div>
              </div>
            </div>
          ) : null}

          {!adResolutionReady ? (
            <div className="px-4 pt-4">
              <div className="flex flex-wrap items-start justify-between gap-2 rounded-lg border border-warning/40 bg-warning-soft px-4 py-2.5 text-sm text-warning">
                <div className="flex items-start gap-2">
                  <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                  <div>
                    <p className="font-medium">{text(locale, 'AD name resolution is not active', 'AD 名称解析未启用')}</p>
                    <p className="mt-0.5 text-xs">
                      {text(locale, 'File events may contain SIDs until AD is connected in the current browser session. Open AD Workspace, test the connection, then return here and refresh.', '在当前浏览器会话未连接 AD 前，文件事件可能显示 SID。请打开 AD 工作区完成连接测试，然后回到这里刷新。')}
                    </p>
                  </div>
                </div>
                <Button variant="outline" size="sm" onClick={() => void router.push('/ad-workspace')}>
                  {text(locale, 'Open AD Workspace', '打开 AD 工作区')}
                </Button>
              </div>
            </div>
          ) : null}

          {loading ? (
            <div className="space-y-2 p-4">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          ) : items.length === 0 ? (
            <EmptyState
              icon={Activity}
              title={text(locale, 'No file activity matched', '没有匹配的文件访问活动')}
              description={text(locale, 'If this should show activity, confirm the audit policy, SACL, time window, and Security log permissions.', '如果这里应该出现访问行为，请确认审计策略、SACL、时间窗口和 Security 日志读取权限。')}
            />
          ) : (
            <div className="overflow-x-auto">
              <Table className="min-w-[1180px]">
                <TableHeader>
                  <TableRow>
                    <TableHead>{text(locale, 'Action', '行为')}</TableHead>
                    <TableHead>{text(locale, 'User', '用户')}</TableHead>
                    <TableHead>{text(locale, 'Path', '路径')}</TableHead>
                    <TableHead>{text(locale, 'Event', '事件')}</TableHead>
                    <TableHead>{text(locale, 'Access', '访问')}</TableHead>
                    <TableHead>{text(locale, 'Process / Client', '进程/客户端')}</TableHead>
                    <TableHead>{text(locale, 'Time', '时间')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {items.map((item) => {
                    const Icon = actionIcon(item.action);
                    return (
                      <TableRow key={`${item.event_id}-${item.timestamp}-${item.user}-${item.path}`}>
                        <TableCell>
                          <Badge tone={actionTone(item.action)}>
                            <Icon className="h-3 w-3" />
                            {actionLabel(item.action, locale)}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <div className="font-mono text-2xs font-medium text-fg">{item.user || '-'}</div>
                          {item.user_sid && item.user_sid !== item.user ? (
                            <div className="mt-0.5 font-mono text-2xs text-fg-muted">SID: {item.user_sid}</div>
                          ) : null}
                          {item.resolution ? (
                            <div className="mt-0.5 text-2xs text-fg-muted">
                              {item.resolution === 'active-directory' ? text(locale, 'Resolved by AD', '已由 AD 解析') : text(locale, 'From event log', '来自事件日志')}
                            </div>
                          ) : null}
                        </TableCell>
                        <TableCell className="max-w-[300px]">
                          <span className="block truncate font-mono text-2xs" title={item.path || '-'}>{item.path || '-'}</span>
                        </TableCell>
                        <TableCell>
                          <Badge tone="neutral">{item.event_id}</Badge>
                        </TableCell>
                        <TableCell className="max-w-[200px]">
                          <span className="block truncate text-2xs text-fg-muted" title={item.access_mask || item.access_list || '-'}>
                            {item.access_mask || item.access_list || '-'}
                          </span>
                        </TableCell>
                        <TableCell className="max-w-[200px]">
                          <span className="block truncate text-2xs text-fg-muted" title={item.process_name || item.client_ip || item.computer || '-'}>
                            {item.process_name || item.client_ip || item.computer || '-'}
                          </span>
                        </TableCell>
                        <TableCell className="whitespace-nowrap text-fg-muted">{formatDateTime(item.timestamp, locale)}</TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          )}
        </Card>
      </div>
    </>
  );
}
