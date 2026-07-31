import Head from 'next/head';
import { useRouter } from 'next/router';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { ArrowRight, CheckCircle2, ListChecks, ShieldAlert, ShieldCheck, Sparkles, Target, UsersRound } from 'lucide-react';
import { useI18n } from '../contexts/I18nContext';
import {
  RISK_FINDINGS_UPDATED_EVENT,
  loadRiskFindings,
  summarizeRiskFindings,
  updateRiskFindingStatus,
  type RiskFinding,
  type RiskFindingSeverity,
  type RiskFindingStatus,
} from '../lib/riskFindings';
import {
  buildWorkspacePageTitle,
  defaultWorkspaceSettings,
  readWorkspaceSettings,
  type WorkspaceSettings,
} from '../lib/workspaceSettings';
import { Card, CardContent, CardHeader } from '../components/ui/card';
import { Badge, riskTone } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { NativeSelect } from '../components/ui/input';
import { EmptyState } from '../components/ui/empty-state';
import { cn } from '../lib/cn';

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

function statusLabel(status: RiskFindingStatus, locale: string) {
  switch (status) {
    case 'accepted':
      return text(locale, 'Accepted', '已接受');
    case 'resolved':
      return text(locale, 'Resolved', '已解决');
    default:
      return text(locale, 'Open', '开放');
  }
}

function severityLabel(severity: RiskFindingSeverity | 'all', locale: string) {
  switch (severity) {
    case 'critical':
      return text(locale, 'Critical', '严重');
    case 'high':
      return text(locale, 'High', '高');
    case 'medium':
      return text(locale, 'Medium', '中');
    case 'low':
      return text(locale, 'Low', '低');
    default:
      return text(locale, 'All severities', '全部等级');
  }
}

function effortLabel(value: string | undefined, locale: string) {
  switch (value) {
    case 'quick-win':
      return text(locale, 'Quick win', '快速整改');
    case 'owner-review':
      return text(locale, 'Owner review', '负责人复核');
    case 'planned-change':
      return text(locale, 'Planned change', '计划变更');
    default:
      return text(locale, 'Review', '复核');
  }
}

function categoryLabel(value: string | undefined, locale: string) {
  switch (value) {
    case 'overexposure':
      return text(locale, 'Overexposure', '过度暴露');
    case 'privilege':
      return text(locale, 'Privilege', '特权');
    case 'hygiene':
      return text(locale, 'Hygiene', '卫生度');
    case 'governance':
      return text(locale, 'Governance', '治理');
    case 'operational-friction':
      return text(locale, 'Operational friction', '运维摩擦');
    case 'sensitive-data':
      return text(locale, 'Sensitive data', '敏感数据');
    default:
      return text(locale, 'Uncategorized', '未分类');
  }
}

export default function FindingsPage() {
  const router = useRouter();
  const { t, locale } = useI18n();
  const [workspaceSettings, setWorkspaceSettings] = useState<WorkspaceSettings>(defaultWorkspaceSettings);
  const [findings, setFindings] = useState<RiskFinding[]>([]);
  const [statusFilter, setStatusFilter] = useState<RiskFindingStatus | 'all'>('open');
  const [severityFilter, setSeverityFilter] = useState<RiskFindingSeverity | 'all'>('all');
  const [categoryFilter, setCategoryFilter] = useState('all');
  const [findingsLoading, setFindingsLoading] = useState(true);
  const [findingsError, setFindingsError] = useState('');
  const [statusPendingID, setStatusPendingID] = useState('');

  const reloadFindings = useCallback(async (showLoading = true) => {
    if (showLoading) {
      setFindingsLoading(true);
    }
    setFindingsError('');
    try {
      setFindings(await loadRiskFindings());
    } catch (requestError) {
      setFindingsError(requestError instanceof Error ? requestError.message : 'Risk findings are unavailable.');
    } finally {
      if (showLoading) {
        setFindingsLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    setWorkspaceSettings(readWorkspaceSettings());
    void reloadFindings();

    const onUpdate = () => void reloadFindings(false);
    window.addEventListener(RISK_FINDINGS_UPDATED_EVENT, onUpdate);
    window.addEventListener('storage', onUpdate);
    return () => {
      window.removeEventListener(RISK_FINDINGS_UPDATED_EVENT, onUpdate);
      window.removeEventListener('storage', onUpdate);
    };
  }, [reloadFindings]);

  const summary = useMemo(() => summarizeRiskFindings(findings), [findings]);
  const categoryOptions = useMemo(
    () => Array.from(new Set(findings.map((finding) => finding.category).filter((value): value is string => Boolean(value)))).sort(),
    [findings]
  );
  const visibleFindings = useMemo(
    () => findings
      .filter((finding) => statusFilter === 'all' || finding.status === statusFilter)
      .filter((finding) => severityFilter === 'all' || finding.severity === severityFilter)
      .filter((finding) => categoryFilter === 'all' || finding.category === categoryFilter)
      .sort((a, b) => (b.priorityScore || 0) - (a.priorityScore || 0) || b.lastSeenAt.localeCompare(a.lastSeenAt)),
    [categoryFilter, findings, severityFilter, statusFilter]
  );

  const setStatus = async (finding: RiskFinding, status: RiskFindingStatus) => {
    setStatusPendingID(finding.id);
    setFindingsError('');
    try {
      const updated = await updateRiskFindingStatus(finding.id, status);
      setFindings((current) => current.map((item) => item.id === updated.id ? updated : item));
    } catch (requestError) {
      setFindingsError(requestError instanceof Error ? requestError.message : 'Risk finding update failed.');
    } finally {
      setStatusPendingID('');
    }
  };

  const openFindingInReport = (finding: RiskFinding) => {
    const params = new URLSearchParams();
    params.set('mode', 'folder');
    params.set('path', finding.path);
    if (finding.lastSessionID) {
      params.set('session', finding.lastSessionID);
    }
    void router.push(`/reports?${params.toString()}`);
  };

  const statusTabs: Array<{ id: RiskFindingStatus | 'all'; label: string; count: number }> = [
    { id: 'open', label: text(locale, 'Open', '开放'), count: summary.open },
    { id: 'accepted', label: text(locale, 'Accepted', '已接受'), count: summary.accepted },
    { id: 'resolved', label: text(locale, 'Resolved', '已解决'), count: summary.resolved },
    { id: 'all', label: text(locale, 'All', '全部'), count: summary.total },
  ];

  return (
    <>
      <Head>
        <title>{buildWorkspacePageTitle(text(locale, 'Exposure Center', '暴露面中心'), workspaceSettings, t('appTitle'))}</title>
      </Head>

      <div className="app-v2 mx-auto max-w-6xl space-y-5">
        {/* Page header */}
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h1 className="text-lg font-semibold text-fg">{text(locale, 'Exposure Center', '暴露面中心')}</h1>
            <p className="mt-0.5 text-sm text-fg-muted">
              {text(locale, 'Actionable permission findings with score, impact, evidence, and remediation guidance.', '可行动的权限风险，附带评分、影响、证据和整改建议。')}
            </p>
            <div className="mt-2 flex flex-wrap items-center gap-1.5">
              <Badge tone="danger">{text(locale, 'Critical', '严重')}: {formatNumber(summary.critical, locale)}</Badge>
              <Badge tone="warning">{text(locale, 'High', '高')}: {formatNumber(summary.high, locale)}</Badge>
              <Badge tone="info">{text(locale, 'Exposure score', '暴露评分')}: {formatNumber(summary.exposureScore, locale)}</Badge>
            </div>
          </div>
          <Button size="sm" onClick={() => void router.push('/scan-workspace')}>
            {text(locale, 'Run Scan', '运行扫描')}
            <ArrowRight className="h-4 w-4" />
          </Button>
        </div>

        {/* Summary stats */}
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-5">
          <SummaryCard
            label={text(locale, 'Exposure score', '暴露评分')}
            value={formatNumber(summary.exposureScore, locale)}
            hint={text(locale, 'Average of the top open priority scores. Higher means faster remediation is needed.', '开放风险中最高优先级的平均值，越高越需要快速整改。')}
            tone="accent"
          />
          <SummaryCard
            label={text(locale, 'Open high impact', '开放高影响')}
            value={formatNumber(summary.critical + summary.high, locale)}
            hint={text(locale, 'Critical and high findings that should not stay undocumented.', '不应长期无记录保留的严重和高风险项。')}
          />
          <SummaryCard
            label={text(locale, 'Blast paths', '风险路径')}
            value={formatNumber(summary.riskyPaths, locale)}
            hint={text(locale, 'Unique paths currently carrying open findings.', '当前存在开放风险的唯一文件路径数量。')}
          />
          <SummaryCard
            label={text(locale, 'Sensitive areas', '敏感区域')}
            value={formatNumber(summary.sensitivePaths, locale)}
            hint={text(locale, 'Sensitive business paths with open exposure findings.', '当前存在开放暴露风险的敏感业务路径。')}
            tone="danger"
          />
          <SummaryCard
            label={text(locale, 'Quick wins', '快速整改')}
            value={formatNumber(summary.quickWins, locale)}
            hint={text(locale, 'Likely cleanup items such as orphaned identities or deny-entry friction.', '通常可快速处理的孤立身份或拒绝项摩擦。')}
          />
        </div>

        {/* Prioritized queue */}
        <Card>
          <CardHeader
            title={text(locale, 'Prioritized Queue', '优先级队列')}
            description={text(locale, 'Findings are generated from broad access, sensitive path exposure, ownership-grade rights, explicit exceptions, privileged groups, unresolved identities, nested groups, inherited spread, and deny-entry friction.', '风险来自宽泛访问、敏感路径暴露、所有权级权限、显式例外、特权组、未解析身份、嵌套组、继承扩散和拒绝项摩擦。')}
            actions={
              <div
                className="inline-flex h-9 items-center gap-1 rounded-lg bg-surface-sunken p-1"
                role="tablist"
                aria-label={text(locale, 'Finding status', '风险状态')}
              >
                {statusTabs.map((tab) => (
                  <button
                    key={tab.id}
                    type="button"
                    role="tab"
                    onClick={() => setStatusFilter(tab.id)}
                    aria-selected={statusFilter === tab.id}
                    className={cn(
                      'inline-flex items-center gap-1.5 whitespace-nowrap rounded-md px-3 py-1 text-xs font-medium transition-colors',
                      'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-line-focus',
                      statusFilter === tab.id
                        ? 'bg-surface-base text-fg shadow-token-sm'
                        : 'text-fg-muted hover:text-fg-secondary'
                    )}
                  >
                    {tab.label}
                    <span className="tabular-nums text-2xs text-fg-muted">{formatNumber(tab.count, locale)}</span>
                  </button>
                ))}
              </div>
            }
          />
          <CardContent className="space-y-4">
            <div className="flex flex-wrap items-end gap-3" aria-label={text(locale, 'Finding filters', '风险筛选器')}>
              <label className="w-full sm:w-48">
                <span className="mb-1 block text-xs font-medium text-fg-secondary">{text(locale, 'Severity', '等级')}</span>
                <NativeSelect value={severityFilter} onChange={(event) => setSeverityFilter(event.target.value as RiskFindingSeverity | 'all')}>
                  <option value="all">{severityLabel('all', locale)}</option>
                  <option value="critical">{severityLabel('critical', locale)}</option>
                  <option value="high">{severityLabel('high', locale)}</option>
                  <option value="medium">{severityLabel('medium', locale)}</option>
                  <option value="low">{severityLabel('low', locale)}</option>
                </NativeSelect>
              </label>
              <label className="w-full sm:w-48">
                <span className="mb-1 block text-xs font-medium text-fg-secondary">{text(locale, 'Category', '类别')}</span>
                <NativeSelect value={categoryFilter} onChange={(event) => setCategoryFilter(event.target.value)}>
                  <option value="all">{text(locale, 'All categories', '全部类别')}</option>
                  {categoryOptions.map((category) => (
                    <option key={category} value={category}>{categoryLabel(category, locale)}</option>
                  ))}
                </NativeSelect>
              </label>
            </div>

            {findingsLoading ? (
              <div className="px-6 py-12 text-center text-sm text-fg-muted" role="status">
                {text(locale, 'Loading findings...', '正在加载风险项...')}
              </div>
            ) : findingsError && findings.length === 0 ? (
              <EmptyState
                icon={ShieldAlert}
                title={text(locale, 'Findings could not be loaded', '风险项加载失败')}
                description={findingsError}
                action={<Button variant="outline" size="sm" onClick={() => void reloadFindings()}>{text(locale, 'Retry', '重试')}</Button>}
              />
            ) : visibleFindings.length === 0 ? (
              <EmptyState
                icon={ShieldCheck}
                title={text(locale, 'No findings in this queue', '当前队列没有风险项')}
                description={text(locale, 'Run or import a scan to generate permission exposure findings, or change the filters above.', '执行或导入扫描生成权限暴露风险，或调整上方筛选器。')}
              />
            ) : (
              <div className="space-y-3">
                {findingsError ? (
                  <div className="rounded-md border border-danger/30 bg-danger-soft px-3 py-2 text-xs text-danger" role="alert">
                    {findingsError}
                  </div>
                ) : null}
                {visibleFindings.map((finding) => (
                  <article key={finding.id} className="rounded-lg border border-line bg-surface-base p-4 shadow-token-sm">
                    <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                      <div className="min-w-0 space-y-2">
                        <div className="flex flex-wrap items-center gap-1.5">
                          <Badge tone={riskTone(finding.severity)}>{severityLabel(finding.severity, locale)}</Badge>
                          <Badge tone="neutral">{statusLabel(finding.status, locale)}</Badge>
                          <Badge tone="info">{text(locale, 'Score', '评分')} {finding.priorityScore || '-'}</Badge>
                          <Badge tone="neutral">{categoryLabel(finding.category, locale)}</Badge>
                        </div>
                        <h3 className="text-sm font-semibold text-fg">{finding.title}</h3>
                        {finding.description ? <p className="text-xs text-fg-secondary">{finding.description}</p> : null}
                        {finding.businessQuestion ? (
                          <div className="flex items-start gap-1.5 rounded-md bg-accent-soft px-2.5 py-1.5 text-xs text-accent-fg">
                            <Sparkles className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                            {finding.businessQuestion}
                          </div>
                        ) : null}
                        {finding.sensitiveLabels?.length ? (
                          <div className="flex items-start gap-1.5 rounded-md bg-danger-soft px-2.5 py-1.5 text-xs text-danger">
                            <ShieldAlert className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                            <span>{text(locale, 'Sensitive area', '敏感区域')}: {finding.sensitiveLabels.join(', ')}</span>
                          </div>
                        ) : null}
                        <div className="break-all rounded-md bg-surface-sunken px-2.5 py-1.5 font-mono text-2xs text-fg-secondary">{finding.path}</div>
                        <div className="flex flex-wrap gap-x-4 gap-y-1 text-2xs text-fg-muted">
                          <span>{text(locale, 'Trustee', '主体')}: <strong className="font-medium text-fg-secondary">{finding.trustee}</strong></span>
                          <span>{text(locale, 'Rights', '权限')}: <strong className="font-medium text-fg-secondary">{finding.rights || '-'}</strong></span>
                          <span>{text(locale, 'Effort', '成本')}: <strong className="font-medium text-fg-secondary">{effortLabel(finding.remediationEffort, locale)}</strong></span>
                          <span>{text(locale, 'Seen', '出现')}: <strong className="font-medium text-fg-secondary">{formatNumber(finding.seenCount, locale)}</strong></span>
                          <span>{text(locale, 'Last seen', '最近出现')}: <strong className="font-medium text-fg-secondary">{formatDateTime(finding.lastSeenAt, locale)}</strong></span>
                        </div>
                        {finding.impact ? (
                          <div className="flex items-start gap-1.5 text-xs text-fg-secondary">
                            <Target className="mt-0.5 h-4 w-4 shrink-0 text-fg-muted" />
                            <span>{finding.impact}</span>
                          </div>
                        ) : null}
                        <p className="text-xs text-fg-secondary">{finding.suggestedAction}</p>
                        {finding.evidence?.length ? (
                          <div className="rounded-md border border-line bg-surface-sunken/60 px-3 py-2">
                            <div className="flex items-center gap-1.5 text-2xs font-semibold uppercase tracking-wide text-fg-muted">
                              <ListChecks className="h-3.5 w-3.5" /> {text(locale, 'Evidence', '证据')}
                            </div>
                            <ul className="mt-1 list-disc space-y-0.5 pl-4 text-2xs text-fg-secondary">
                              {finding.evidence.slice(0, 5).map((entry) => (
                                <li key={entry}>{entry}</li>
                              ))}
                            </ul>
                          </div>
                        ) : null}
                        {finding.controlMapping?.length ? (
                          <div className="flex flex-wrap gap-1.5">
                            {finding.controlMapping.map((item) => <Badge key={item} tone="neutral">{item}</Badge>)}
                          </div>
                        ) : null}
                        {finding.note ? <p className="text-2xs italic text-fg-muted">{finding.note}</p> : null}
                      </div>

                      <div className="flex shrink-0 flex-row flex-wrap gap-2 lg:w-40 lg:flex-col">
                        <Button variant="outline" size="sm" className="lg:justify-start" onClick={() => openFindingInReport(finding)}>
                          {text(locale, 'Open report scope', '打开报告范围')}
                        </Button>
                        {finding.status !== 'accepted' ? (
                          <Button variant="outline" size="sm" className="lg:justify-start" loading={statusPendingID === finding.id} onClick={() => void setStatus(finding, 'accepted')}>
                            <UsersRound className="h-3.5 w-3.5" />
                            {text(locale, 'Accept risk', '接受风险')}
                          </Button>
                        ) : null}
                        {finding.status !== 'resolved' ? (
                          <Button variant="outline" size="sm" className="lg:justify-start" loading={statusPendingID === finding.id} onClick={() => void setStatus(finding, 'resolved')}>
                            <CheckCircle2 className="h-3.5 w-3.5" />
                            {text(locale, 'Mark resolved', '标记解决')}
                          </Button>
                        ) : (
                          <Button variant="outline" size="sm" className="lg:justify-start" loading={statusPendingID === finding.id} onClick={() => void setStatus(finding, 'open')}>
                            {text(locale, 'Reopen', '重新打开')}
                          </Button>
                        )}
                      </div>
                    </div>
                  </article>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </>
  );
}

function SummaryCard({
  label,
  value,
  hint,
  tone = 'default',
}: {
  label: string;
  value: string;
  hint: string;
  tone?: 'default' | 'accent' | 'danger';
}) {
  return (
    <Card className="px-4 py-3.5">
      <p className={cn(
        'text-2xs font-medium uppercase tracking-wide',
        tone === 'accent' ? 'text-accent-fg' : tone === 'danger' ? 'text-danger' : 'text-fg-muted'
      )}>
        {label}
      </p>
      <p className={cn('mt-1 text-lg font-semibold leading-6', tone === 'danger' ? 'text-danger' : 'text-fg')}>{value}</p>
      <p className="mt-0.5 text-2xs text-fg-muted">{hint}</p>
    </Card>
  );
}
