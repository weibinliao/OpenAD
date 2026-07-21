import Head from 'next/head';
import { useEffect, useState, type ReactNode } from 'react';
import { useRouter } from 'next/router';
import { Activity, AlertTriangle, ArrowLeft, RefreshCw } from 'lucide-react';
import { useI18n } from '../../contexts/I18nContext';
import { buildWorkspacePageTitle, defaultWorkspaceSettings, readWorkspaceSettings, type WorkspaceSettings } from '../../lib/workspaceSettings';
import { apiBase } from '../../lib/runtimeApi';
import { Button } from '../../components/ui/button';
import { Card, CardHeader } from '../../components/ui/card';
import { Badge, type BadgeTone } from '../../components/ui/badge';
import { NativeSelect } from '../../components/ui/input';
import { Skeleton } from '../../components/ui/skeleton';
import { EmptyState } from '../../components/ui/empty-state';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../../components/ui/table';

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

interface AuditDetailResponse {
  request_id: string;
  count: number;
  total_count: number;
  latest?: AuditItem;
  entries: AuditItem[];
  pagination: AuditPagination;
}

function text(locale: string, en: string, zh: string) {
  return locale === 'zh-CN' ? zh : en;
}

function statusTone(status: number): BadgeTone {
  if (status >= 500) return 'danger';
  if (status >= 400) return 'warning';
  if (status >= 300) return 'info';
  if (status >= 200) return 'success';
  return 'neutral';
}

export default function AuditDetailPage() {
  const router = useRouter();
  const { t, locale } = useI18n();
  const [workspaceSettings, setWorkspaceSettings] = useState<WorkspaceSettings>(defaultWorkspaceSettings);
  const [detail, setDetail] = useState<AuditDetailResponse | null>(null);
  const [pageSize, setPageSize] = useState(50);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const requestId = Array.isArray(router.query.id) ? router.query.id[0] : router.query.id || '';

  useEffect(() => {
    if (typeof window === 'undefined') return;
    setWorkspaceSettings(readWorkspaceSettings());
  }, []);

  const fetchDetail = async (page = 1, nextPageSize = pageSize) => {
    if (!requestId) return;
    setLoading(true);
    setError('');
    try {
      const search = new URLSearchParams({ page: String(page), page_size: String(nextPageSize) });
      const response = await fetch(`${apiBase()}/api/audit/requests/${encodeURIComponent(requestId)}?${search.toString()}`);
      const data = await response.json().catch(() => ({}));
      if (!response.ok) {
        throw new Error(data.error || text(locale, 'Audit request was not found.', '未找到该审计请求。'));
      }
      setDetail(data);
      setPageSize(data.pagination?.page_size || nextPageSize);
    } catch (requestError) {
      setDetail(null);
      setError(requestError instanceof Error ? requestError.message : text(locale, 'Failed to load audit request.', '加载审计请求失败。'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!router.isReady || !requestId) return;
    void fetchDetail(1);
  }, [router.isReady, requestId]);

  const pagination = detail?.pagination || { page: 1, page_size: pageSize, total: 0, total_pages: 1 };
  const latest = detail?.latest;

  return (
    <>
      <Head>
        <title>{buildWorkspacePageTitle(text(locale, 'Audit Detail', '审计详情'), workspaceSettings, t('appTitle'))}</title>
      </Head>

      <div className="app-v2 mx-auto max-w-6xl space-y-5">
        {/* Page header */}
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <h1 className="text-lg font-semibold text-fg">{text(locale, 'Audit Request Detail', '审计请求详情')}</h1>
            <p className="mt-0.5 flex flex-wrap items-center gap-2 text-sm text-fg-muted">
              <span>{text(locale, 'All captured entries for a single request ID.', '同一个请求 ID 下捕获的全部审计条目。')}</span>
              <Badge tone="neutral" className="font-mono">
                {requestId || text(locale, 'No request selected', '未选择请求')}
              </Badge>
            </p>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <Button variant="secondary" size="sm" onClick={() => void router.push('/audit')}>
              <ArrowLeft className="h-3.5 w-3.5" />
              {text(locale, 'Back to audit', '返回审计')}
            </Button>
            <Button size="sm" onClick={() => void fetchDetail(pagination.page)} disabled={loading || !requestId}>
              <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
              {t('refresh')}
            </Button>
          </div>
        </div>

        {error && (
          <div className="flex items-start gap-2 rounded-lg border border-danger/40 bg-danger-soft px-4 py-2.5 text-sm text-danger">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
            <div>
              <p className="font-medium">{text(locale, 'Audit detail unavailable', '审计详情不可用')}</p>
              <p className="mt-0.5 text-xs">{error}</p>
            </div>
          </div>
        )}

        {/* Latest entry summary */}
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <SummaryCard
            label={text(locale, 'Method', '方法')}
            value={latest ? latest.method : '—'}
            hint={latest ? latest.path : text(locale, 'No latest entry available.', '暂无最近条目。')}
          />
          <SummaryCard
            label={text(locale, 'Latest status', '最近状态')}
            value={latest ? <Badge tone={statusTone(latest.status)}>{latest.status}</Badge> : '—'}
            hint={text(locale, 'Status from the latest captured entry.', '来自最近捕获条目的状态。')}
          />
          <SummaryCard
            label={text(locale, 'Duration', '耗时')}
            value={latest ? `${latest.duration_ms} ms` : '—'}
            hint={text(locale, 'Latest captured duration.', '最近捕获的耗时。')}
          />
          <SummaryCard
            label={text(locale, 'Entries', '条目')}
            value={String(pagination.total)}
            hint={latest ? `${text(locale, 'Client', '客户端')}: ${latest.client_ip || '-'}` : text(locale, 'Short-term runtime evidence.', '短期运行证据。')}
          />
        </div>

        <Card>
          <CardHeader
            title={text(locale, 'Request Evidence', '请求证据')}
            description={text(locale, 'Use this view when a request ID from the audit ledger needs deeper review.', '当审计台账中的某个请求 ID 需要深入排查时，使用此页面查看证据。')}
            actions={
              <NativeSelect
                aria-label={text(locale, 'Rows per page', '每页行数')}
                value={pageSize}
                onChange={(event) => {
                  const value = Number.parseInt(event.target.value, 10);
                  setPageSize(value);
                  void fetchDetail(1, value);
                }}
                className="w-24"
              >
                <option value={20}>20</option>
                <option value={50}>50</option>
                <option value={100}>100</option>
                <option value={200}>200</option>
              </NativeSelect>
            }
          />

          {loading ? (
            <div className="space-y-2 p-4">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          ) : detail && detail.entries.length > 0 ? (
            <div className="overflow-x-auto border-t border-line">
              <Table className="min-w-[980px]">
                <TableHeader>
                  <TableRow>
                    <TableHead>{text(locale, 'Time', '时间')}</TableHead>
                    <TableHead>{t('method')}</TableHead>
                    <TableHead>{t('path')}</TableHead>
                    <TableHead>{t('status')}</TableHead>
                    <TableHead className="text-right">{t('durationMs')}</TableHead>
                    <TableHead>{text(locale, 'User agent', '用户代理')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {detail.entries.map((entry) => (
                    <TableRow key={`${entry.timestamp}-${entry.method}-${entry.path}`}>
                      <TableCell className="whitespace-nowrap text-fg-muted">{new Date(entry.timestamp).toLocaleString()}</TableCell>
                      <TableCell className="font-medium text-fg">{entry.method}</TableCell>
                      <TableCell className="max-w-[320px]">
                        <span className="block truncate font-mono text-2xs" title={entry.path}>{entry.path}</span>
                      </TableCell>
                      <TableCell>
                        <Badge tone={statusTone(entry.status)}>{entry.status}</Badge>
                      </TableCell>
                      <TableCell className="text-right tabular-nums">{entry.duration_ms}</TableCell>
                      <TableCell className="max-w-[280px]">
                        <span className="block truncate text-fg-muted" title={entry.user_agent || '-'}>{entry.user_agent || '-'}</span>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          ) : (
            <EmptyState
              icon={Activity}
              title={text(locale, 'No audit entries found', '未找到审计条目')}
              description={text(locale, 'The API did not return entries for this request ID.', 'API 没有返回该请求 ID 的条目。')}
            />
          )}

          {/* Pagination */}
          <div className="flex items-center justify-between gap-2 border-t border-line px-4 py-2.5">
            <span className="text-xs text-fg-muted">
              {text(locale, `${pagination.total} entries`, `共 ${pagination.total} 条`)}
            </span>
            <div className="flex items-center gap-2">
              <Button
                variant="secondary"
                size="sm"
                disabled={pagination.page <= 1 || loading}
                onClick={() => void fetchDetail(Math.max(1, pagination.page - 1))}
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
                onClick={() => void fetchDetail(Math.min(Math.max(1, pagination.total_pages), pagination.page + 1))}
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

function SummaryCard({ label, value, hint }: { label: string; value: ReactNode; hint?: string }) {
  return (
    <Card className="px-4 py-3.5">
      <p className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{label}</p>
      <div className="mt-1 text-lg font-semibold leading-6 text-fg">{value}</div>
      {hint ? (
        <p className="mt-0.5 truncate text-2xs text-fg-muted" title={hint}>
          {hint}
        </p>
      ) : null}
    </Card>
  );
}
