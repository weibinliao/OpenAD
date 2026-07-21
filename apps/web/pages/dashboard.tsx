import React, { useCallback, useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import {
  FolderSearch,
  Users,
  GitFork,
  FileBarChart,
  ShieldAlert,
  Layers,
  ScanSearch,
  Clock,
  AlertTriangle,
  ArrowRight,
  PlugZap,
  CheckCircle2,
} from 'lucide-react';
import { apiBase } from '../lib/runtimeApi';
import { useDict } from '../lib/i18n';
import { useI18n } from '../contexts/I18nContext';
import { useADConnection } from '../contexts/ADConnectionContext';
import { QuickConnectCard } from '../components/QuickConnectCard';
import {
  readRiskFindings,
  summarizeRiskFindings,
  RISK_FINDINGS_UPDATED_EVENT,
  type RiskFinding,
  type RiskFindingSummary,
} from '../lib/riskFindings';
import type { DashboardSessionSummary } from '../lib/dashboardSummary';
import { Card, CardHeader, CardContent } from '../components/ui/card';
import { Badge, riskTone } from '../components/ui/badge';
import { Button } from '../components/ui/button';
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

interface SessionsResponse {
  items?: DashboardSessionSummary[];
  pagination?: { total?: number };
  error?: string;
}

function formatDateTime(value: string | undefined | null, locale: string) {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  return date.toLocaleString(locale === 'zh-CN' ? 'zh-CN' : 'en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function statusTone(status: string): Parameters<typeof Badge>[0]['tone'] {
  switch (status) {
    case 'completed':
      return 'success';
    case 'running':
      return 'info';
    case 'failed':
      return 'danger';
    default:
      return 'neutral';
  }
}

export default function DashboardPage() {
  const d = useDict();
  const { locale } = useI18n();
  const { activeProfile } = useADConnection();

  const [backendOnline, setBackendOnline] = useState<boolean | null>(null);
  const [sessions, setSessions] = useState<DashboardSessionSummary[]>([]);
  const [sessionsTotal, setSessionsTotal] = useState(0);
  const [sessionsLoading, setSessionsLoading] = useState(true);
  const [findings, setFindings] = useState<RiskFinding[]>([]);
  const [summary, setSummary] = useState<RiskFindingSummary | null>(null);

  const loadSessions = useCallback(async () => {
    setSessionsLoading(true);
    try {
      const response = await fetch(`${apiBase()}/api/sessions?status=completed&page=1&page_size=6`);
      const data = (await response.json().catch(() => ({}))) as SessionsResponse;
      if (!response.ok) throw new Error(data.error || 'sessions unavailable');
      const items = Array.isArray(data.items) ? data.items : [];
      setSessions(items);
      setSessionsTotal(data.pagination?.total ?? items.length);
      setBackendOnline(true);
    } catch {
      setSessions([]);
      setSessionsTotal(0);
      setBackendOnline(false);
    } finally {
      setSessionsLoading(false);
    }
  }, []);

  const loadFindings = useCallback(() => {
    const items = readRiskFindings();
    setFindings(items);
    setSummary(summarizeRiskFindings(items));
  }, []);

  useEffect(() => {
    loadSessions();
    loadFindings();
    if (typeof window === 'undefined') return;
    const onFindingsUpdate = () => loadFindings();
    window.addEventListener(RISK_FINDINGS_UPDATED_EVENT, onFindingsUpdate);
    return () => window.removeEventListener(RISK_FINDINGS_UPDATED_EVENT, onFindingsUpdate);
  }, [loadFindings, loadSessions]);

  const latestSession = sessions[0] || null;
  const highRiskOpen = summary ? summary.critical + summary.high : 0;

  const topRisks = useMemo(
    () =>
      findings
        .filter((f) => f.status === 'open')
        .sort((a, b) => (b.priorityScore ?? severityRank(b.severity)) - (a.priorityScore ?? severityRank(a.severity)))
        .slice(0, 5),
    [findings],
  );

  const riskBars = useMemo(() => {
    if (!summary) return [];
    const entries = [
      { key: 'critical', label: 'Critical', value: summary.critical, cls: 'bg-risk-critical' },
      { key: 'high', label: 'High', value: summary.high, cls: 'bg-risk-high' },
      { key: 'medium', label: 'Medium', value: summary.medium, cls: 'bg-risk-medium' },
      { key: 'low', label: 'Low', value: summary.low, cls: 'bg-risk-low' },
    ];
    const max = Math.max(1, ...entries.map((e) => e.value));
    return entries.map((e) => ({ ...e, pct: Math.round((e.value / max) * 100) }));
  }, [summary]);

  return (
    <div className="app-v2 mx-auto max-w-6xl space-y-5">
      {/* Page header */}
      <div>
        <h1 className="text-lg font-semibold text-fg">{d.dashboard.title}</h1>
        <p className="mt-0.5 text-sm text-fg-muted">{d.dashboard.subtitle}</p>
      </div>

      {backendOnline === false ? (
        <div className="flex items-center gap-2 rounded-lg border border-warning/40 bg-warning-soft px-4 py-2.5 text-sm text-warning">
          <AlertTriangle className="h-4 w-4 shrink-0" />
          {d.common.backendOffline}
        </div>
      ) : null}

      {/* Onboarding: show until the operator has both a connection and at least one scan */}
      {backendOnline !== false && (!activeProfile || (!sessionsLoading && sessionsTotal === 0)) ? (
        <GettingStarted
          connected={Boolean(activeProfile)}
          hasScans={sessionsTotal > 0}
        />
      ) : null}

      {/* Stat cards */}
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          icon={ScanSearch}
          label={d.dashboard.statScans}
          value={sessionsLoading ? null : String(sessionsTotal)}
        />
        <StatCard
          icon={Layers}
          label={d.dashboard.statPermissions}
          value={sessionsLoading ? null : latestSession ? latestSession.permission_count.toLocaleString() : '—'}
        />
        <StatCard
          icon={ShieldAlert}
          label={d.dashboard.statHighRisk}
          value={summary ? String(highRiskOpen) : '—'}
          tone={highRiskOpen > 0 ? 'danger' : 'default'}
        />
        <StatCard
          icon={Clock}
          label={d.dashboard.statLastScan}
          value={sessionsLoading ? null : formatDateTime(latestSession?.finished_at, locale)}
        />
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-3">
        {/* Recent scans */}
        <Card className="xl:col-span-2">
          <CardHeader
            title={d.dashboard.recentScans}
            actions={
              <Link href="/history" className="inline-flex items-center gap-1 text-xs font-medium text-accent-fg hover:underline">
                {d.common.viewAll} <ArrowRight className="h-3 w-3" />
              </Link>
            }
          />
          <CardContent>
            {sessionsLoading ? (
              <div className="space-y-2">
                <Skeleton className="h-8 w-full" />
                <Skeleton className="h-8 w-full" />
                <Skeleton className="h-8 w-full" />
              </div>
            ) : sessions.length === 0 ? (
              <EmptyState
                icon={FolderSearch}
                title={d.common.empty}
                description={d.dashboard.noScans}
                action={
                  <Link href="/resources">
                    <Button size="sm">{d.dashboard.actionNewScan}</Button>
                  </Link>
                }
              />
            ) : (
              <TableContainer className="border-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{d.dashboard.scanColumns.path}</TableHead>
                      <TableHead>{d.dashboard.scanColumns.status}</TableHead>
                      <TableHead className="text-right">{d.dashboard.scanColumns.items}</TableHead>
                      <TableHead className="text-right">{d.dashboard.scanColumns.permissions}</TableHead>
                      <TableHead>{d.dashboard.scanColumns.finished}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {sessions.map((session) => (
                      <TableRow key={session.id}>
                        <TableCell className="max-w-[260px]">
                          <span className="block truncate font-mono text-2xs text-fg" title={session.root_path}>
                            {session.root_path}
                          </span>
                        </TableCell>
                        <TableCell>
                          <Badge tone={statusTone(session.status)}>{session.status}</Badge>
                        </TableCell>
                        <TableCell className="text-right tabular-nums">{session.items_scanned.toLocaleString()}</TableCell>
                        <TableCell className="text-right tabular-nums">{session.permission_count.toLocaleString()}</TableCell>
                        <TableCell className="whitespace-nowrap text-fg-muted">
                          {formatDateTime(session.finished_at, locale)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            )}
          </CardContent>
        </Card>

        {/* Right rail */}
        <div className="space-y-4">
          {/* Quick actions */}
          <Card>
            <CardHeader title={d.dashboard.quickActions} />
            <CardContent className="grid grid-cols-1 gap-2 sm:grid-cols-2">
              <QuickAction href="/resources" icon={FolderSearch} label={d.dashboard.actionNewScan} />
              <QuickAction href="/" icon={Users} label={d.dashboard.actionBrowseAd} />
              <QuickAction href="/findings" icon={ShieldAlert} label={d.nav.risks} />
              <QuickAction href="/access" icon={GitFork} label={d.dashboard.actionAnalyzeAccess} />
            </CardContent>
          </Card>

          {/* Risk distribution */}
          <Card>
            <CardHeader title={d.dashboard.riskDistribution} />
            <CardContent className="space-y-2">
              {riskBars.length === 0 || !summary || summary.total === 0 ? (
                <p className="py-2 text-xs text-fg-muted">{d.common.empty}</p>
              ) : (
                riskBars.map((bar) => (
                  <div key={bar.key} className="flex items-center gap-2">
                    <span className="w-14 shrink-0 text-2xs text-fg-muted">{bar.label}</span>
                    <div className="h-2 flex-1 overflow-hidden rounded-full bg-surface-sunken">
                      <div className={`h-full rounded-full ${bar.cls}`} style={{ width: `${bar.pct}%` }} />
                    </div>
                    <span className="w-8 shrink-0 text-right text-2xs tabular-nums text-fg-secondary">{bar.value}</span>
                  </div>
                ))
              )}
            </CardContent>
          </Card>

          {/* Top risks */}
          <Card>
            <CardHeader
              title={d.dashboard.topRisks}
              actions={
                <Link href="/findings" className="inline-flex items-center gap-1 text-xs font-medium text-accent-fg hover:underline">
                  {d.common.viewAll} <ArrowRight className="h-3 w-3" />
                </Link>
              }
            />
            <CardContent>
              {topRisks.length === 0 ? (
                <p className="py-2 text-xs text-fg-muted">{d.common.empty}</p>
              ) : (
                <ul className="space-y-2.5">
                  {topRisks.map((risk) => (
                    <li key={risk.id} className="flex items-start gap-2">
                      <Badge tone={riskTone(risk.severity)} className="mt-0.5 shrink-0 capitalize">
                        {risk.severity}
                      </Badge>
                      <div className="min-w-0">
                        <p className="truncate text-xs font-medium text-fg" title={risk.title}>
                          {risk.title}
                        </p>
                        <p className="truncate font-mono text-2xs text-fg-muted" title={risk.path}>
                          {risk.path}
                        </p>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}

function GettingStarted({ connected, hasScans }: { connected: boolean; hasScans: boolean }) {
  const d = useDict();
  const steps = [
    {
      key: 'connect',
      title: d.dashboard.step1Title,
      desc: d.dashboard.step1Desc,
      done: connected,
      href: '/settings',
      icon: PlugZap,
    },
    {
      key: 'scan',
      title: d.dashboard.step2Title,
      desc: d.dashboard.step2Desc,
      done: hasScans,
      href: '/resources',
      icon: FolderSearch,
    },
    {
      key: 'analyze',
      title: d.dashboard.step3Title,
      desc: d.dashboard.step3Desc,
      done: false,
      href: '/access',
      icon: GitFork,
    },
  ];

  return (
    <Card>
      <CardHeader title={d.dashboard.getStarted} />
      <CardContent className="space-y-4">
        <ol className="grid gap-3 md:grid-cols-3">
          {steps.map((step, index) => (
            <li
              key={step.key}
              className={
                step.done
                  ? 'rounded-lg border border-success/40 bg-success-soft/40 p-3'
                  : 'rounded-lg border border-line p-3'
              }
            >
              <div className="mb-1.5 flex items-center gap-2">
                <span
                  className={
                    step.done
                      ? 'flex h-6 w-6 items-center justify-center rounded-full bg-success text-fg-on-accent'
                      : 'flex h-6 w-6 items-center justify-center rounded-full bg-surface-sunken text-fg-secondary text-xs font-semibold'
                  }
                >
                  {step.done ? <CheckCircle2 className="h-3.5 w-3.5" /> : index + 1}
                </span>
                <span className="text-sm font-medium text-fg">{step.title}</span>
              </div>
              <p className="mb-2.5 text-xs text-fg-muted">{step.desc}</p>
              <Link href={step.href}>
                <Button variant={step.done ? 'ghost' : 'outline'} size="sm" className="w-full">
                  <step.icon className="h-3.5 w-3.5" />
                  {step.done ? d.dashboard.stepDone : d.dashboard.stepStart}
                </Button>
              </Link>
            </li>
          ))}
        </ol>
        {!connected ? <QuickConnectCard /> : null}
      </CardContent>
    </Card>
  );
}

function severityRank(severity: RiskFinding['severity']): number {
  switch (severity) {
    case 'critical':
      return 400;
    case 'high':
      return 300;
    case 'medium':
      return 200;
    default:
      return 100;
  }
}

function StatCard({
  icon: Icon,
  label,
  value,
  tone = 'default',
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value: string | null;
  tone?: 'default' | 'danger';
}) {
  return (
    <Card className="px-4 py-3.5">
      <div className="flex items-center gap-3">
        <div
          className={
            tone === 'danger'
              ? 'flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-danger-soft text-danger'
              : 'flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-accent-soft text-accent-fg'
          }
        >
          <Icon className="h-4.5 h-[18px] w-[18px]" />
        </div>
        <div className="min-w-0">
          <p className="truncate text-2xs font-medium uppercase tracking-wide text-fg-muted">{label}</p>
          {value === null ? (
            <Skeleton className="mt-1 h-5 w-16" />
          ) : (
            <p className={`truncate text-lg font-semibold leading-6 ${tone === 'danger' ? 'text-danger' : 'text-fg'}`}>{value}</p>
          )}
        </div>
      </div>
    </Card>
  );
}

function QuickAction({
  href,
  icon: Icon,
  label,
  disabled = false,
}: {
  href?: string;
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  disabled?: boolean;
}) {
  const inner = (
    <span
      className={
        disabled
          ? 'flex h-full flex-col items-center justify-center gap-1.5 rounded-md border border-dashed border-line px-2 py-3 text-center opacity-50'
          : 'flex h-full flex-col items-center justify-center gap-1.5 rounded-md border border-line px-2 py-3 text-center transition-colors hover:border-accent/50 hover:bg-accent-soft/50'
      }
    >
      <Icon className="h-4 w-4 text-fg-muted" />
      <span className="text-2xs font-medium leading-3.5 text-fg-secondary">{label}</span>
    </span>
  );
  if (disabled || !href) return inner;
  return <Link href={href}>{inner}</Link>;
}
