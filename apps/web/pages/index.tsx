import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/router';
import {
  Activity,
  AlertTriangle,
  ArrowRight,
  CheckCircle2,
  Gauge,
  Clock3,
  FileSearch,
  FolderSearch,
  GitFork,
  HardDrive,
  RefreshCw,
  Server,
  ShieldAlert,
  ShieldCheck,
  Contact,
  Users,
} from 'lucide-react';
import { apiBase } from '../lib/runtimeApi';
import { useADConnection } from '../contexts/ADConnectionContext';
import { useI18n } from '../contexts/I18nContext';
import { useRuntimeHealth } from '../hooks/useRuntimeHealth';
import {
  RISK_FINDINGS_UPDATED_EVENT,
  readRiskFindings,
  sortRiskFindingsByPriority,
  summarizeRiskFindings,
  type RiskFinding,
} from '../lib/riskFindings';
import type { DashboardSessionSummary } from '../lib/dashboardSummary';
import { QuickConnectCard } from '../components/QuickConnectCard';
import { Button } from '../components/ui/button';
import { EmptyState } from '../components/ui/empty-state';
import { Skeleton } from '../components/ui/skeleton';
import { riskLabel, scanStatusLabel } from '../components/ui/badge';

interface SessionsResponse {
  items?: DashboardSessionSummary[];
  pagination?: { total?: number };
}

type RuntimeHealth = ReturnType<typeof useRuntimeHealth>;

interface OverviewRisk {
  id: string;
  severity: RiskFinding['severity'];
  title: string;
  path: string;
  trustee: string;
  rights: string;
}

export default function OpenADOverview() {
  const router = useRouter();
  const { locale } = useI18n();
  const { activeProfile, connection } = useADConnection();
  const runtimeHealth = useRuntimeHealth();
  const [sessions, setSessions] = useState<DashboardSessionSummary[]>([]);
  const [sessionsTotal, setSessionsTotal] = useState(0);
  const [backendOnline, setBackendOnline] = useState<boolean | null>(null);
  const [sessionsLoading, setSessionsLoading] = useState(true);
  const [sessionsError, setSessionsError] = useState('');
  const [risks, setRisks] = useState<RiskFinding[]>([]);
  const isChinese = locale === 'zh-CN';
  const text = (zh: string, en: string) => (isChinese ? zh : en);

  const loadSessions = useCallback(async () => {
    setSessionsLoading(true);
    setSessionsError('');
    try {
      const response = await fetch(`${apiBase()}/api/sessions?status=completed&page=1&page_size=6`);
      const data = (await response.json().catch(() => ({}))) as SessionsResponse;
      if (!response.ok) throw new Error('sessions unavailable');
      const items = Array.isArray(data.items) ? data.items : [];
      setSessions(items);
      setSessionsTotal(data.pagination?.total ?? items.length);
      setBackendOnline(true);
    } catch (requestError) {
      setSessions([]);
      setSessionsTotal(0);
      setBackendOnline(false);
      setSessionsError(requestError instanceof Error ? requestError.message : 'sessions unavailable');
    } finally {
      setSessionsLoading(false);
    }
  }, []);

  const loadRisks = useCallback(() => setRisks(readRiskFindings()), []);

  useEffect(() => {
    void loadSessions();
    loadRisks();
    window.addEventListener(RISK_FINDINGS_UPDATED_EVENT, loadRisks);
    return () => window.removeEventListener(RISK_FINDINGS_UPDATED_EVENT, loadRisks);
  }, [loadRisks, loadSessions]);

  const riskSummary = useMemo(() => summarizeRiskFindings(risks), [risks]);
  const displayRisks = useMemo<OverviewRisk[]>(() => {
    return sortRiskFindingsByPriority(risks.filter((risk) => risk.status === 'open'))
      .slice(0, 3)
      .map((risk) => ({
        id: risk.id,
        severity: risk.severity,
        title: risk.title,
        path: risk.path,
        trustee: risk.trustee,
        rights: risk.rights,
      }));
  }, [risks]);
  const latestSession = sessions[0] || null;
  const highRiskCount = riskSummary.critical + riskSummary.high;
  const connected = Boolean(activeProfile || connection.connected);
  const riskBars = [
    { key: 'critical', label: text('严重', 'Critical'), value: riskSummary.critical },
    { key: 'high', label: text('高危', 'High'), value: riskSummary.high },
    { key: 'medium', label: text('中危', 'Medium'), value: riskSummary.medium },
    { key: 'low', label: text('低危', 'Low'), value: riskSummary.low },
  ] as const;
  const maxRiskCount = Math.max(1, ...riskBars.map((bar) => bar.value));

  const statusItems = [
    {
      key: 'ad',
      label: text('AD 连接', 'AD Connection'),
      value: connected ? activeProfile?.name || text('已连接', 'Connected') : text('未连接', 'Not connected'),
      detail: connected ? text('目录查询可用', 'Directory queries available') : text('前往系统设置', 'Open system settings'),
      icon: Users,
      tone: connected ? 'success' : 'warning',
      href: connected ? '/identity' : '/settings',
    },
    {
      key: 'directory',
      label: text('目录同步', 'Directory Sync'),
      value: connected ? text('可以同步', 'Ready to sync') : text('等待连接', 'Waiting for AD'),
      detail: text('用户、组与嵌套关系', 'Users, groups and nested membership'),
      icon: RefreshCw,
      tone: connected ? 'success' : 'warning',
      href: '/identity',
    },
    {
      key: 'scan',
      label: text('最近扫描', 'Latest Scan'),
      value: latestSession
        ? `${formatCompact(latestSession.items_scanned)} ${text('个对象', 'items')}`
        : text('暂无真实扫描', 'No live scan yet'),
      detail: latestSession
        ? `${formatNumber(latestSession.permission_count)} ${text('个权限项', 'ACL entries')} · ${text('完成于', 'Completed')} ${formatDateTime(latestSession.finished_at || latestSession.started_at, locale)}`
        : text('进入扫描中心创建任务', 'Create a task in Scan Center'),
      icon: HardDrive,
      tone: latestSession ? 'success' : 'neutral',
      href: '/scan-workspace',
    },
    {
      key: 'risk',
      label: text('高优先级风险', 'High-priority Risks'),
      value: String(highRiskCount),
      detail: text('需要运维人员复核', 'Require operator review'),
      icon: ShieldAlert,
      tone: highRiskCount > 0 ? 'danger' : 'success',
      href: '/findings',
    },
    {
      key: 'runtime',
      label: text('本地服务', 'Local Services'),
      value: runtimeValue(runtimeHealth, isChinese),
      detail: 'API 18080 · WEB 43110',
      icon: Server,
      tone: runtimeHealth === 'healthy' ? 'success' : runtimeHealth === 'offline' ? 'danger' : 'neutral',
      href: '/settings',
    },
  ] as const;

  const actions = [
    {
      label: text('开始权限扫描', 'Start permission scan'),
      description: text('选择文件夹或共享并采集 ACL', 'Select a folder or share and collect ACLs'),
      icon: FolderSearch,
      href: '/scan-workspace',
    },
    {
      label: text('同步目录快照', 'Sync directory snapshot'),
      description: text('更新用户、组与成员关系', 'Refresh users, groups and memberships'),
      icon: RefreshCw,
      href: '/identity',
    },
    {
      label: text('分析有效访问', 'Analyze effective access'),
      description: text('按用户或资源解释权限链路', 'Explain access by identity or resource'),
      icon: GitFork,
      href: '/access',
    },
    {
      label: text('查看资源清单', 'Review resource inventory'),
      description: text('整理共享、扫描目标与结果', 'Organize shares, targets and results'),
      icon: HardDrive,
      href: '/resources',
    },
  ];

  const reports = [
    {
      label: text('用户权限报告', 'User access report'),
      description: text('查看用户可访问的全部路径', 'All paths reachable by a user'),
      icon: Contact,
      mode: 'user',
    },
    {
      label: text('文件夹权限报告', 'Folder access report'),
      description: text('查看路径上的主体与权限', 'Trustees and rights on a path'),
      icon: FileSearch,
      mode: 'folder',
    },
    {
      label: text('所有者报告', 'Owner report'),
      description: text('按责任归属汇总权限风险', 'Summarize access risk by ownership'),
      icon: ShieldCheck,
      mode: 'owner',
    },
  ];

  const readiness = [
    { label: text('目录身份数据', 'Directory identity data'), ready: connected, warning: false },
    { label: text('权限扫描数据', 'Permission scan data'), ready: sessions.length > 0, warning: backendOnline === false },
    { label: text('有效权限分析', 'Effective access analysis'), ready: connected && sessions.length > 0, warning: false },
    { label: text('风险治理证据', 'Risk governance evidence'), ready: risks.length > 0, warning: false },
  ];

  return (
    <div className="openad-overview">
      <header className="openad-overview-header">
        <div className="openad-overview-heading">
          <h1>{text('OpenAD 运维总览', 'OpenAD Operations Overview')}</h1>
          <p>{text('从目录状态、权限采集到风险证据的每日工作入口', 'Daily entry point from directory state to permission evidence')}</p>
        </div>
        <div className="openad-overview-mode">
          <ShieldCheck className="h-3.5 w-3.5" aria-hidden />
          {text('只读审计模式', 'Read-only audit mode')}
        </div>
      </header>

      <div className="openad-panel-title">
        <Gauge className="h-3.5 w-3.5" aria-hidden />
        {text('环境状态', 'Environment Status')}
      </div>
      <section className="openad-overview-status" aria-label={text('环境状态', 'Environment Status')}>
        {statusItems.map((item) => (
          <StatusItem key={item.key} label={item.label} value={item.value} detail={item.detail} icon={item.icon} tone={item.tone} onClick={() => void router.push(item.href)} />
        ))}
      </section>

      {!connected ? (
        <section className="openad-overview-quick-connect" aria-label={text('快速连接 Active Directory', 'Quick Active Directory connection')}>
          <QuickConnectCard className="openad-quick-connect-card" />
        </section>
      ) : null}

      <div className="openad-operations-grid">
        <section className="openad-tool-panel">
          <div className="openad-panel-header">
            <div className="openad-panel-title">
              <Activity className="h-3.5 w-3.5" aria-hidden />
              {text('今日操作', "Today's Operations")}
            </div>
            <span className="openad-panel-meta">{text('按正常运维顺序排列', 'Ordered by normal operations flow')}</span>
          </div>
          <div className="openad-action-list">
            {actions.map((action) => (
              <button key={action.href} className="openad-action" type="button" onClick={() => void router.push(action.href)}>
                <span className="openad-action-icon"><action.icon className="h-4 w-4" aria-hidden /></span>
                <span className="openad-action-copy">
                  <strong>{action.label}</strong>
                  <span>{action.description}</span>
                </span>
              </button>
            ))}
          </div>
        </section>

        <section className="openad-tool-panel">
          <div className="openad-panel-header">
            <div className="openad-panel-title">
              <AlertTriangle className="h-3.5 w-3.5" aria-hidden />
              {text('需要关注', 'Attention Required')}
            </div>
            <button className="openad-panel-link" type="button" onClick={() => void router.push('/findings')}>
              {text('查看全部', 'View all')} <ArrowRight className="h-3 w-3" aria-hidden />
            </button>
          </div>
          <div className="openad-risk-distribution" aria-label={text('风险级别分布', 'Risk distribution')}>
            <div className="openad-risk-distribution-title">{text('风险级别分布', 'Risk distribution')}</div>
            <div className="openad-risk-distribution-grid">
              {riskBars.map((bar) => (
                <div className="openad-risk-bar" key={bar.key}>
                  <span>{bar.label}</span>
                  <strong>{bar.value}</strong>
                  <span className="openad-risk-track" aria-hidden>
                    <span
                      className={`openad-risk-fill is-${bar.key}`}
                      data-testid={`risk-bar-${bar.key}`}
                      style={{ width: `${Math.round((bar.value / maxRiskCount) * 100)}%` }}
                    />
                  </span>
                </div>
              ))}
            </div>
          </div>
          <div className="openad-attention-list">
            {displayRisks.length === 0 ? (
              <EmptyState
                icon={ShieldCheck}
                title={text('当前没有待复核风险', 'No risks require review')}
                description={text('完成权限扫描后，风险发现会在此列出需要处理的证据。', 'Complete a permission scan to populate this queue with actionable evidence.')}
                action={<Button size="sm" onClick={() => void router.push('/scan-workspace')}>{text('前往扫描中心', 'Open Scan Center')}</Button>}
                className="py-8"
              />
            ) : displayRisks.map((risk) => (
              <button
                key={risk.id}
                className="openad-attention-row"
                type="button"
                onClick={() => void router.push(`/findings?focus=${encodeURIComponent(risk.id)}`)}
              >
                <span className="openad-risk-mark">{risk.severity.slice(0, 1).toUpperCase()}</span>
                <span className="openad-risk-copy">
                  <strong>{risk.title}</strong>
                  <span>{risk.trustee} · {risk.rights}</span>
                </span>
                <span className="openad-risk-right">{riskLabel(risk.severity, locale)}</span>
              </button>
            ))}
          </div>
        </section>
      </div>

      <div className="openad-evidence-grid">
        <section className="openad-tool-panel">
          <div className="openad-panel-header">
            <div className="openad-panel-title">
              <Clock3 className="h-3.5 w-3.5" aria-hidden />
              {text('最近活动', 'Recent Activity')}
            </div>
            <button className="openad-panel-link" type="button" onClick={() => void router.push('/history')}>
              {text('扫描历史', 'Scan history')} <ArrowRight className="h-3 w-3" aria-hidden />
            </button>
          </div>
          {sessionsLoading ? (
            <div role="status" aria-label={text('正在加载最近活动', 'Loading recent activity')} className="space-y-2 p-3">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          ) : sessionsError ? (
            <EmptyState
              icon={AlertTriangle}
              title={text('最近活动暂不可用', 'Recent activity unavailable')}
              description={text('本地历史接口当前不可用。请检查本地 API 后重试。', 'The local history endpoint is unavailable. Check the local API and retry.')}
              action={
                <Button aria-label={text('重试加载最近活动', 'Retry loading recent activity')} size="sm" onClick={() => void loadSessions()}>
                  <RefreshCw className="h-3.5 w-3.5" /> {text('重试', 'Retry')}
                </Button>
              }
              className="py-8"
            />
          ) : sessions.length === 0 ? (
            <EmptyState
              icon={Clock3}
              title={text('还没有已完成扫描', 'No completed scans yet')}
              description={text('先在扫描中心完成一次扫描，最近活动会在这里显示。', 'Complete a scan in Scan Center to populate recent activity.')}
              action={<Button size="sm" onClick={() => void router.push('/scan-workspace')}>{text('前往扫描中心', 'Open Scan Center')}</Button>}
              className="py-8"
            />
          ) : (
            <div className="openad-table-scroll">
              <table className="openad-activity-table">
                <thead>
                  <tr>
                    <th>{text('资源路径', 'Resource path')}</th>
                    <th>{text('状态', 'State')}</th>
                    <th>{text('权限项', 'ACL entries')}</th>
                    <th>{text('完成时间', 'Completed')}</th>
                  </tr>
                </thead>
                <tbody>
                  {sessions.slice(0, 4).map((session) => (
                    <tr key={session.id}>
                      <td><span className="openad-activity-path" title={session.root_path}>{session.root_path}</span></td>
                      <td><span className="openad-state-label">{scanStatusLabel(session.status, locale)}</span></td>
                      <td>{formatCompact(session.permission_count)}</td>
                      <td>{formatRelative(session.finished_at || session.started_at, isChinese)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <div className="flex min-w-0 flex-col gap-[14px]">
          <section className="openad-tool-panel">
            <div className="openad-panel-header">
              <div className="openad-panel-title">
                <FileSearch className="h-3.5 w-3.5" aria-hidden />
                {text('权限报告入口', 'Permission Reports')}
              </div>
            </div>
            <div className="openad-reports-grid">
              {reports.map((report) => (
                <button
                  key={report.mode}
                  className="openad-report-button"
                  type="button"
                  onClick={() => void router.push(`/reports?mode=${report.mode}`)}
                >
                  <report.icon className="h-3.5 w-3.5" aria-hidden />
                  <strong>{report.label}</strong>
                  <span>{report.description}</span>
                </button>
              ))}
            </div>
          </section>

          <section className="openad-tool-panel">
            <div className="openad-panel-header">
              <div className="openad-panel-title">
                <CheckCircle2 className="h-3.5 w-3.5" aria-hidden />
                {text('模块就绪度', 'Module Readiness')}
              </div>
              <span className="openad-panel-meta">{sessionsTotal || sessions.length} {text('次扫描', 'scans')}</span>
            </div>
            <div className="openad-readiness-list">
              {readiness.map((item) => (
                <div className="openad-readiness-row" key={item.label}>
                  <span>{item.label}</span>
                  <span className="openad-readiness-state">
                    <span className={`openad-readiness-dot ${item.ready ? 'is-ready' : item.warning ? 'is-warning' : ''}`} aria-hidden />
                    {item.ready ? text('可用', 'Ready') : item.warning ? text('样例数据', 'Sample data') : text('待准备', 'Not ready')}
                  </span>
                </div>
              ))}
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}

function StatusItem({
  label,
  value,
  detail,
  icon: Icon,
  tone,
  onClick,
}: {
  label: string;
  value: string;
  detail: string;
  icon: React.ComponentType<{ className?: string }>;
  tone: 'success' | 'warning' | 'danger' | 'neutral';
  onClick: () => void;
}) {
  return (
    <button className="openad-status-item" type="button" onClick={onClick}>
      <span className={`openad-status-icon ${tone === 'neutral' ? '' : `is-${tone}`}`}>
        <Icon className="h-4 w-4" aria-hidden />
      </span>
      <span className="openad-status-copy">
        <span>{label}</span>
        <strong>{value}</strong>
        <small title={detail}>{detail}</small>
      </span>
    </button>
  );
}

function runtimeValue(health: RuntimeHealth, isChinese: boolean) {
  if (health === 'healthy') return isChinese ? '运行正常' : 'Healthy';
  if (health === 'offline') return isChinese ? '服务离线' : 'Offline';
  return isChinese ? '正在检查' : 'Checking';
}

function formatCompact(value: number) {
  return new Intl.NumberFormat('en-US', {
    notation: value >= 1000 ? 'compact' : 'standard',
    maximumFractionDigits: 1,
  }).format(value);
}

function formatNumber(value: number) {
  return new Intl.NumberFormat('en-US').format(value);
}

function formatDateTime(value: string | undefined, locale: string) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';
  return date.toLocaleString(locale === 'zh-CN' ? 'zh-CN' : 'en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function formatRelative(value: string | undefined, isChinese: boolean) {
  if (!value) return '-';
  const timestamp = new Date(value).getTime();
  if (Number.isNaN(timestamp)) return '-';
  const minutes = Math.max(1, Math.round((Date.now() - timestamp) / 60000));
  if (minutes < 60) return isChinese ? `${minutes} 分钟前` : `${minutes} min ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return isChinese ? `${hours} 小时前` : `${hours} hr ago`;
  const days = Math.round(hours / 24);
  return isChinese ? `${days} 天前` : `${days} d ago`;
}
