import Head from 'next/head';
import { useEffect, useState } from 'react';
import { useRouter } from 'next/router';
import { Activity, AlertTriangle, Download, RefreshCw } from 'lucide-react';
import { useI18n } from '../contexts/I18nContext';
import { buildWorkspacePageTitle, defaultWorkspaceSettings, readWorkspaceSettings, type WorkspaceSettings } from '../lib/workspaceSettings';
import { apiBase } from '../lib/runtimeApi';
import { Button } from '../components/ui/button';
import { Card, CardHeader } from '../components/ui/card';
import { Badge, type BadgeTone } from '../components/ui/badge';
import { Input, NativeSelect } from '../components/ui/input';
import { Skeleton } from '../components/ui/skeleton';
import { EmptyState } from '../components/ui/empty-state';
import { Table, TableBody, TableCell, TableContainer, TableHead, TableHeader, TableRow } from '../components/ui/table';

interface AuditItem {
  request_id: string;
  method: string;
  path: string;
  status: number;
  duration_ms: number;
  client_ip: string;
  user_agent: string;
  timestamp: string;
}

interface AuditPagination {
  page: number;
  page_size: number;
  total: number;
  total_pages: number;
}

function text(locale: string, en: string, zh: string) {
  return locale === 'zh-CN' ? zh : en;
}

function getAuditStatusTone(status: number): BadgeTone {
  if (status >= 500) {
    return 'danger';
  }
  if (status >= 400) {
    return 'warning';
  }
  if (status >= 300) {
    return 'info';
  }
  if (status >= 200) {
    return 'success';
  }
  return 'neutral';
}

function formatCount(value: number, locale: string) {
  return new Intl.NumberFormat(locale === 'zh-CN' ? 'zh-CN' : 'en-US').format(value);
}

export default function AuditPage() {
  const { t, locale } = useI18n();
  const router = useRouter();
  const [workspaceSettings, setWorkspaceSettings] = useState<WorkspaceSettings>(defaultWorkspaceSettings);
  const [items, setItems] = useState<AuditItem[]>([]);
  const [pagination, setPagination] = useState<AuditPagination>({ page: 1, page_size: 50, total: 0, total_pages: 1 });
  const [pageSize, setPageSize] = useState(50);
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState(false);
  const [error, setError] = useState('');
  const [methodFilter, setMethodFilter] = useState('');
  const [pathFilter, setPathFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const activeFilterCount = [methodFilter.trim(), pathFilter.trim(), statusFilter.trim()].filter(Boolean).length;

  const fetchAudits = async (page = 1, nextPageSize = pageSize) => {
    setLoading(true);
    setError('');
    try {
      const search = new URLSearchParams();
      search.set('page', String(page));
      search.set('page_size', String(nextPageSize));
      if (methodFilter.trim()) search.set('method', methodFilter.trim());
      if (pathFilter.trim()) search.set('path', pathFilter.trim());
      if (statusFilter.trim()) search.set('status', statusFilter.trim());

      const response = await fetch(`${apiBase()}/api/audit/requests?${search.toString()}`);
      const data = await response.json();
      if (!response.ok) {
        throw new Error(data.error || t('loadFailed'));
      }

      const nextPagination = data.pagination || { page: 1, page_size: nextPageSize, total: 0, total_pages: 1 };
      setItems(data.items || []);
      setPagination(nextPagination);
      setPageSize(nextPagination.page_size || nextPageSize);
    } catch (requestError) {
      setItems([]);
      setError(requestError instanceof Error ? requestError.message : t('loadFailed'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void fetchAudits(1);
  }, []);

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }

    setWorkspaceSettings(readWorkspaceSettings());
  }, []);

  const handleExportSummary = async () => {
    if (exporting) {
      return;
    }

    setExporting(true);
    setError('');
    try {
      const response = await fetch(`${apiBase()}/api/audit/requests/export/summary`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          method: methodFilter.trim(),
          path: pathFilter.trim(),
          status: statusFilter.trim() ? Number(statusFilter.trim()) : 0,
          title: t('auditSummaryTitle'),
        }),
      });
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.error || t('exportFailed'));
      }

      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = 'audit-summary.md';
      link.click();
      window.URL.revokeObjectURL(url);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : t('exportFailed'));
    } finally {
      setExporting(false);
    }
  };

  return (
    <>
      <Head>
        <title>{buildWorkspacePageTitle(t('audit'), workspaceSettings, t('appTitle'))}</title>
      </Head>

      <div className="app-v2 mx-auto max-w-6xl space-y-5">
        {/* Page header */}
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h1 className="text-lg font-semibold text-fg">{t('audit')}</h1>
            <p className="mt-0.5 text-sm text-fg-muted">
              {text(locale, 'API request trails, durations, and status codes for operational review.', '记录 API 请求轨迹、耗时和状态码，便于后续追溯。')}
            </p>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <Button variant="secondary" size="sm" onClick={() => void fetchAudits(1)} disabled={loading}>
              <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
              {t('refresh')}
            </Button>
            <Button size="sm" onClick={() => void handleExportSummary()} disabled={exporting}>
              {exporting ? <RefreshCw className="h-3.5 w-3.5 animate-spin" /> : <Download className="h-3.5 w-3.5" />}
              {t('exportAuditSummary')}
            </Button>
          </div>
        </div>

        {error && (
          <div className="flex items-start gap-2 rounded-lg border border-danger/40 bg-danger-soft px-4 py-2.5 text-sm text-danger">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
            <div>
              <p className="font-medium">{text(locale, 'Audit records are currently unavailable', '审计记录当前不可用')}</p>
              <p className="mt-0.5 text-xs">{error}</p>
            </div>
          </div>
        )}

        <Card>
          <CardHeader
            title={text(locale, 'Audit Records', '审计记录')}
            description={text(locale, 'Filter the ledger, then drill into a single request for the full trail.', '通过过滤器收紧台账，需要完整证据链时进入单条请求详情。')}
          />

          {/* Filters */}
          <div className="flex flex-wrap items-center gap-2 border-b border-line px-4 pb-3">
            <NativeSelect
              aria-label={t('method')}
              value={methodFilter}
              onChange={(event) => setMethodFilter(event.target.value)}
              className="w-32"
            >
              <option value="">{t('anyMethod')}</option>
              <option value="GET">GET</option>
              <option value="POST">POST</option>
              <option value="PUT">PUT</option>
              <option value="PATCH">PATCH</option>
              <option value="DELETE">DELETE</option>
            </NativeSelect>
            <Input
              value={pathFilter}
              onChange={(event) => setPathFilter(event.target.value)}
              placeholder={text(locale, 'Path contains...', '路径包含...')}
              className="w-56 flex-1 sm:flex-none"
            />
            <Input
              value={statusFilter}
              onChange={(event) => setStatusFilter(event.target.value)}
              placeholder={t('statusFilterHint')}
              className="w-36"
            />
            <NativeSelect
              aria-label={text(locale, 'Rows per page', '每页行数')}
              value={pageSize}
              onChange={(event) => {
                const value = Number.parseInt(event.target.value, 10);
                setPageSize(value);
                void fetchAudits(1, value);
              }}
              className="w-24"
            >
              <option value={20}>20</option>
              <option value={50}>50</option>
              <option value={100}>100</option>
              <option value={200}>200</option>
            </NativeSelect>
            <Button variant="secondary" size="sm" onClick={() => void fetchAudits(1)} disabled={loading}>
              {text(locale, 'Apply', '应用')}
            </Button>
            <span className="ml-auto text-xs text-fg-muted">
              {activeFilterCount === 0
                ? text(locale, `${formatCount(pagination.total, locale)} records`, `共 ${formatCount(pagination.total, locale)} 条记录`)
                : text(locale, `${activeFilterCount} filters · ${formatCount(pagination.total, locale)} records`, `${activeFilterCount} 个过滤器 · 共 ${formatCount(pagination.total, locale)} 条记录`)}
            </span>
          </div>

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
              title={t('noAuditEntries')}
              description={text(
                locale,
                'No request trace matches the current filter set yet. Trigger new activity in the app and refresh.',
                '当前过滤条件下还没有匹配的请求轨迹。先在系统里执行新的操作，再刷新查看最新审计轨迹。'
              )}
              action={
                <Button variant="secondary" size="sm" onClick={() => void fetchAudits(1)} disabled={loading}>
                  <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
                  {t('refresh')}
                </Button>
              }
            />
          ) : (
            <TableContainer className="max-h-[56vh] rounded-none border-x-0 border-b-0">
              <Table className="min-w-[980px]">
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('requestId')}</TableHead>
                    <TableHead>{t('method')}</TableHead>
                    <TableHead>{t('path')}</TableHead>
                    <TableHead>{t('status')}</TableHead>
                    <TableHead className="text-right">{t('durationMs')}</TableHead>
                    <TableHead>{t('clientIp')}</TableHead>
                    <TableHead>{t('detectedAt')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {items.map((item) => (
                    <TableRow key={`${item.request_id}-${item.timestamp}`}>
                      <TableCell>
                        <button
                          type="button"
                          className="font-mono text-2xs text-accent-fg hover:underline"
                          onClick={() => void router.push(`/audit/${encodeURIComponent(item.request_id)}`)}
                        >
                          {item.request_id}
                        </button>
                      </TableCell>
                      <TableCell className="font-medium text-fg">{item.method}</TableCell>
                      <TableCell className="max-w-[320px]">
                        <span className="block truncate font-mono text-2xs" title={item.path}>{item.path}</span>
                      </TableCell>
                      <TableCell>
                        <Badge tone={getAuditStatusTone(item.status)}>{item.status}</Badge>
                      </TableCell>
                      <TableCell className="text-right tabular-nums">{item.duration_ms}</TableCell>
                      <TableCell className="font-mono text-2xs">{item.client_ip}</TableCell>
                      <TableCell className="whitespace-nowrap text-fg-muted">{new Date(item.timestamp).toLocaleString()}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          )}

          {/* Pagination */}
          <div className="flex items-center justify-between gap-2 border-t border-line px-4 py-2.5">
            <span className="text-xs text-fg-muted">
              {text(locale, `${formatCount(pageSize, locale)} rows per page`, `每页 ${formatCount(pageSize, locale)} 行`)}
            </span>
            <div className="flex items-center gap-2">
              <Button
                variant="secondary"
                size="sm"
                disabled={pagination.page <= 1 || loading}
                onClick={() => void fetchAudits(Math.max(1, pagination.page - 1))}
              >
                {t('prevPage')}
              </Button>
              <span className="text-xs text-fg-muted">
                {t('pageInfo', { current: pagination.page, total: Math.max(1, pagination.total_pages) })}
              </span>
              <Button
                variant="secondary"
                size="sm"
                disabled={pagination.page >= Math.max(1, pagination.total_pages) || loading}
                onClick={() => void fetchAudits(Math.min(Math.max(1, pagination.total_pages), pagination.page + 1))}
              >
                {t('nextPage')}
              </Button>
            </div>
          </div>
        </Card>
      </div>
    </>
  );
}
