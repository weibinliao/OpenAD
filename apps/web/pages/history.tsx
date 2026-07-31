import Head from 'next/head';
import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/router';
import { AlertTriangle, ArrowRightLeft, CheckCircle2, History, RefreshCw } from 'lucide-react';
import { useI18n } from '../contexts/I18nContext';
import { buildWorkspacePageTitle, defaultWorkspaceSettings, readWorkspaceSettings, type WorkspaceSettings } from '../lib/workspaceSettings';
import { apiBase } from '../lib/runtimeApi';
import { Card, CardContent, CardHeader } from '../components/ui/card';
import { Badge, scanStatusLabel, type BadgeTone } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { NativeSelect } from '../components/ui/input';
import { Skeleton } from '../components/ui/skeleton';
import { EmptyState } from '../components/ui/empty-state';
import {
  TableContainer,
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '../components/ui/table';

interface ScanSession {
  id: string;
  root_path: string;
  status: string;
  max_depth: number;
  include_inherited: boolean;
  items_scanned: number;
  permission_count: number;
  started_at: string;
  finished_at?: string;
  error_message?: string;
}

interface CompareResponse {
  baseline_id: string;
  current_id: string;
  changes_count: number;
}

function openSessionInStudio(router: ReturnType<typeof useRouter>, sessionID: string) {
  void router.push(`/reports?session=${encodeURIComponent(sessionID)}`);
}

function text(locale: string, en: string, zh: string) {
  return locale === 'zh-CN' ? zh : en;
}

function formatDateTime(value: string | undefined, locale: string) {
  if (!value) {
    return '-';
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return new Intl.DateTimeFormat(locale === 'zh-CN' ? 'zh-CN' : 'en-US', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date);
}

function formatCount(value: number, locale: string) {
  return new Intl.NumberFormat(locale === 'zh-CN' ? 'zh-CN' : 'en-US').format(value);
}

export default function HistoryPage() {
  const { t, locale } = useI18n();
  const [workspaceSettings, setWorkspaceSettings] = useState<WorkspaceSettings>(defaultWorkspaceSettings);
  const [sessions, setSessions] = useState<ScanSession[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [baselineSessionID, setBaselineSessionID] = useState('');
  const [currentSessionID, setCurrentSessionID] = useState('');
  const [compareError, setCompareError] = useState<string | null>(null);
  const [compareResult, setCompareResult] = useState<CompareResponse | null>(null);
  const [comparing, setComparing] = useState(false);
  const router = useRouter();

  const fetchSessions = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch(`${apiBase()}/api/sessions`);
      const data = await response.json().catch(() => ({}));
      if (!response.ok) {
        throw new Error(data.error || t('failedFetchSessions'));
      }
      setSessions(data.items || []);
    } catch (err) {
      setSessions([]);
      setError(err instanceof Error ? err.message : t('loadFailed'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void fetchSessions();
  }, []);

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }

    setWorkspaceSettings(readWorkspaceSettings());
  }, []);

  const formatDate = (dateStr: string) => new Date(dateStr).toLocaleString();

  const completedSessions = useMemo(
    () => sessions.filter((session) => session.status === 'completed'),
    [sessions]
  );

  const latestCompletedAt = useMemo(() => {
    let latest: string | null = null;

    completedSessions.forEach((session) => {
      const candidate = session.finished_at || session.started_at;
      if (!candidate) {
        return;
      }

      if (!latest || new Date(candidate).getTime() > new Date(latest).getTime()) {
        latest = candidate;
      }
    });

    return latest;
  }, [completedSessions]);

  useEffect(() => {
    if (completedSessions.length < 2) {
      return;
    }
    if (!currentSessionID) {
      setCurrentSessionID(completedSessions[0].id);
    }
    if (!baselineSessionID) {
      setBaselineSessionID(completedSessions[1].id);
    }
  }, [baselineSessionID, completedSessions, currentSessionID]);

  const handleRunCompare = async () => {
    if (!baselineSessionID || !currentSessionID) {
      setCompareError(t('compareNeedBoth'));
      return;
    }
    if (baselineSessionID === currentSessionID) {
      setCompareError(t('compareMustDiff'));
      return;
    }

    setComparing(true);
    setCompareError(null);
    setCompareResult(null);

    try {
      const response = await fetch(`${apiBase()}/api/compare`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          baseline_session_id: baselineSessionID,
          current_session_id: currentSessionID,
        }),
      });
      const data = await response.json();
      if (!response.ok) {
        throw new Error(data.error || t('failedCompare'));
      }
      setCompareResult(data as CompareResponse);
    } catch (compareErr) {
      setCompareError(compareErr instanceof Error ? compareErr.message : t('failedCompare'));
    } finally {
      setComparing(false);
    }
  };

  const getStatusTone = (status: string): BadgeTone => {
    switch (status) {
      case 'completed':
        return 'success';
      case 'failed':
        return 'danger';
      case 'running':
        return 'info';
      default:
        return 'neutral';
    }
  };

  return (
    <>
      <Head>
        <title>{buildWorkspacePageTitle(t('scanHistory'), workspaceSettings, t('appTitle'))}</title>
      </Head>

      <div className="app-v2 mx-auto max-w-6xl space-y-5">
        {/* Page header */}
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h1 className="text-lg font-semibold text-fg">{t('scanHistory')}</h1>
            <p className="mt-0.5 text-sm text-fg-muted">
              {text(locale, 'Review previous scan sessions, compare completed runs, and open any completed session in Report Center.', '回看以往扫描会话、比较已完成运行，并可在报告中心打开任一已完成会话。')}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="secondary" size="sm" onClick={() => void router.push('/scan-workspace')}>
              {text(locale, 'Open Scan Workspace', '打开扫描工作区')}
            </Button>
            <Button variant="outline" size="sm" onClick={() => void fetchSessions()} disabled={loading}>
              <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
              {t('refresh')}
            </Button>
          </div>
        </div>

        {/* Summary stats */}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3" aria-label={text(locale, 'History status summary', '历史状态摘要')}>
          <Card className="px-4 py-3.5">
            <p className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Completed runs', '已完成运行')}</p>
            <p className="mt-1 text-lg font-semibold leading-6 text-fg">{formatCount(completedSessions.length, locale)}</p>
            <p className="mt-0.5 text-2xs text-fg-muted">{completedSessions.length >= 2 ? text(locale, 'Ready for baseline comparison.', '已可进行基线对比。') : text(locale, 'Need two completed runs to compare.', '需要两次已完成运行才能对比。')}</p>
          </Card>
          <Card className="px-4 py-3.5">
            <p className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Visible sessions', '可见会话')}</p>
            <p className="mt-1 text-lg font-semibold leading-6 text-fg">{formatCount(sessions.length, locale)}</p>
            <p className="mt-0.5 text-2xs text-fg-muted">{loading ? t('loadingHistory') : error ? text(locale, 'History endpoint needs attention.', '历史接口需要关注。') : text(locale, 'Ledger is synced with the backend.', '台账已与后端同步。')}</p>
          </Card>
          <Card className="px-4 py-3.5">
            <p className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Latest baseline', '最近基线')}</p>
            <p className="mt-1 text-lg font-semibold leading-6 text-fg">{latestCompletedAt ? formatDateTime(latestCompletedAt, locale) : '-'}</p>
            <p className="mt-0.5 text-2xs text-fg-muted">{text(locale, 'Open a completed session in Report Center when report evidence is needed.', '需要报告证据时，可在报告中心打开已完成会话。')}</p>
          </Card>
        </div>

        {/* Compare */}
        <Card>
          <CardHeader
            title={text(locale, 'Compare completed runs', '比较已完成运行')}
            description={text(locale, 'Pick a baseline and a current session, then run the comparison.', '选择基线会话与当前会话，然后执行比较。')}
          />
          <CardContent className="space-y-3">
            <div className="grid gap-2 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
              <NativeSelect value={baselineSessionID} onChange={(event) => setBaselineSessionID(event.target.value)}>
                <option value="">{t('selectBaseline')}</option>
                {completedSessions.map((session) => (
                  <option key={session.id} value={session.id}>{new Date(session.started_at).toLocaleString()} | {session.root_path}</option>
                ))}
              </NativeSelect>
              <NativeSelect value={currentSessionID} onChange={(event) => setCurrentSessionID(event.target.value)}>
                <option value="">{t('selectCurrent')}</option>
                {completedSessions.map((session) => (
                  <option key={session.id} value={session.id}>{new Date(session.started_at).toLocaleString()} | {session.root_path}</option>
                ))}
              </NativeSelect>
              <Button onClick={handleRunCompare} disabled={comparing || completedSessions.length < 2} loading={comparing}>
                {!comparing ? <ArrowRightLeft className="h-4 w-4" /> : null}
                {comparing ? t('comparing') : t('runCompare')}
              </Button>
            </div>

            {compareError ? (
              <div className="flex items-start gap-2 rounded-lg border border-danger/40 bg-danger-soft px-4 py-2.5 text-sm text-danger">
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                <div>
                  <p className="font-medium">{text(locale, 'Compare could not start', '无法开始比较')}</p>
                  <p className="mt-0.5 text-xs">{compareError}</p>
                </div>
              </div>
            ) : null}
            {compareResult ? (
              <div className="flex items-start gap-2 rounded-lg border border-success/40 bg-success-soft px-4 py-2.5 text-sm text-success">
                <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0" />
                <div>
                  <p className="font-medium">{t('compareDone')}: <strong>{compareResult.changes_count}</strong></p>
                  <button
                    type="button"
                    onClick={() => openSessionInStudio(router, compareResult.current_id)}
                    className="mt-0.5 text-xs font-medium underline underline-offset-2 hover:opacity-80"
                  >
                    {text(locale, 'Load current session', '导入当前会话')}
                  </button>
                </div>
              </div>
            ) : null}
          </CardContent>
        </Card>

        {/* Session records */}
        <Card>
          <CardHeader
            title={text(locale, 'Session Records', '会话记录')}
            description={text(locale, 'Completed, running, and failed sessions from the history endpoint.', '来自历史接口的已完成、运行中和失败会话。')}
            actions={
              latestCompletedAt ? (
                <span className="text-2xs text-fg-muted">
                  {text(locale, `Latest ${formatDateTime(latestCompletedAt, locale)}`, `最近 ${formatDateTime(latestCompletedAt, locale)}`)}
                </span>
              ) : null
            }
          />
          <CardContent className="space-y-3">
            {error ? (
              <div className="flex items-start gap-2 rounded-lg border border-danger/40 bg-danger-soft px-4 py-2.5 text-sm text-danger">
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                <div>
                  <p className="font-medium">{text(locale, 'History records are currently unavailable', '历史记录当前不可用')}</p>
                  <p className="mt-0.5 text-xs">
                    {error}
                    <br />
                    {text(
                      locale,
                      'This usually means the local history database has not been initialized yet, or the backend cannot read it right now. You can still run live scans and come back after the backend storage is ready.',
                      '这通常表示本地历史数据库尚未初始化，或后端当前无法读取它。你仍然可以先执行实时扫描，等后端存储就绪后再回到这里查看历史。'
                    )}
                  </p>
                </div>
              </div>
            ) : null}

            {loading ? (
              <div className="space-y-2">
                <Skeleton className="h-8 w-full" />
                <Skeleton className="h-8 w-full" />
                <Skeleton className="h-8 w-full" />
                <Skeleton className="h-8 w-full" />
              </div>
            ) : sessions.length === 0 ? (
              <EmptyState
                icon={History}
                title={t('noHistory')}
                description={text(locale, 'No completed or saved scan sessions are available yet. Start a scan in Scan Workspace first, then return here to compare, reload, or review historical runs.', '当前还没有可用的已完成或已保存扫描会话。请先在扫描工作区执行扫描，之后再回到这里进行对比、重载或查看历史运行。')}
                action={
                  <Button size="sm" onClick={() => void router.push('/scan-workspace')}>
                    {text(locale, 'Open Scan Workspace', '前往扫描工作区')}
                  </Button>
                }
              />
            ) : (
              <TableContainer>
                <Table className="min-w-[920px]">
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('path')}</TableHead>
                      <TableHead>{t('status')}</TableHead>
                      <TableHead className="text-right">{t('items')}</TableHead>
                      <TableHead className="text-right">{t('permissions')}</TableHead>
                      <TableHead>{t('started')}</TableHead>
                      <TableHead>{t('actions')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {sessions.map((session) => (
                      <TableRow key={session.id}>
                        <TableCell className="max-w-[320px]">
                          <span className="block truncate font-medium text-fg" title={session.root_path}>{session.root_path}</span>
                          <span className="mt-0.5 block text-2xs text-fg-muted">{t('depthLabel')}: {session.max_depth}, {t('inheritedLabel')}: {session.include_inherited ? t('yes') : t('no')}</span>
                        </TableCell>
                        <TableCell>
                          <Badge tone={getStatusTone(session.status)}>{scanStatusLabel(session.status, locale)}</Badge>
                        </TableCell>
                        <TableCell className="text-right tabular-nums">{session.items_scanned}</TableCell>
                        <TableCell className="text-right tabular-nums">{session.permission_count}</TableCell>
                        <TableCell className="whitespace-nowrap text-fg-muted">{formatDate(session.started_at)}</TableCell>
                        <TableCell>
                          <div className="flex flex-col items-start gap-1">
                            <Button variant="outline" size="sm" onClick={() => openSessionInStudio(router, session.id)}>
                              {text(locale, 'Open report', '打开报告')}
                            </Button>
                            <span className="font-mono text-2xs text-fg-faint">{session.id}</span>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            )}
          </CardContent>
        </Card>
      </div>
    </>
  );
}
