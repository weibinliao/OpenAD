import { type LucideIcon, ChevronRight, FileText, FolderOpen, ShieldAlert, UsersRound } from 'lucide-react';
import { useMemo } from 'react';
import { Badge, type BadgeTone } from './ui/badge';
import {
  TableContainer,
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from './ui/table';
import { cn } from '../lib/cn';

interface PreviewLedgerRow {
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
  permissions: string;
  group_inheritance_hierarchy: string;
  permission_count?: number;
  risk_level?: string;
  applies_to_summary?: string;
  inheritance_summary?: string;
  row_count?: number;
}

interface PreviewAclRow {
  path: string;
  trustee: string;
  rights: string;
  type: string;
  inherited: boolean;
  applies_to?: string;
  account_type?: string;
  parent_delta?: string;
}

interface StatsSummary {
  total: number;
  visible: number;
  highRisk: number;
  skipped: number;
}

interface ExportReportPreviewProps {
  locale: string;
  reportTitle: string;
  templateTitle: string;
  templateSummary: string;
  previewGeneratedAt: string;
  normalizedPath: string;
  modeSummary: string;
  sharePresetLabel: string;
  stats: StatsSummary;
  previewUserRows: PreviewLedgerRow[];
  previewAclRows: PreviewAclRow[];
  previewNarrativeLead: string;
  previewNarrativeBullets: string[];
}

interface PathSummary {
  path: string;
  label: string;
  rowCount: number;
}

interface ScopeBreadcrumb {
  value: string;
  label: string;
}

interface CompactMetaCardProps {
  label: string;
  value: string;
  detail: string;
}

interface KpiCardProps {
  label: string;
  value: string;
  detail: string;
  icon: LucideIcon;
  iconClassName: string;
}

function text(locale: string, en: string, zh: string) {
  return locale === 'zh-CN' ? zh : en;
}

function formatNumber(value: number) {
  return new Intl.NumberFormat().format(value);
}

function displayName(row: PreviewLedgerRow) {
  const fullName = [row.first_name, row.last_name].filter(Boolean).join(' ').trim();
  return fullName || '-';
}

function compactPathLabel(path: string) {
  const normalized = path.trim().replace(/\//g, '\\');
  if (!normalized) {
    return '-';
  }

  const parts = normalized.split('\\').filter(Boolean);
  return parts[parts.length - 1] || normalized;
}

function scopeBreadcrumbs(path: string): ScopeBreadcrumb[] {
  const normalized = path.trim().replace(/\//g, '\\');
  if (!normalized) {
    return [];
  }

  if (normalized.startsWith('\\\\')) {
    const parts = normalized.replace(/^\\\\+/, '').split('\\').filter(Boolean);
    if (parts.length === 0) {
      return [];
    }

    const root = parts.length >= 2 ? `\\\\${parts[0]}\\${parts[1]}` : `\\\\${parts[0]}`;
    const segments = [root];
    let current = root;
    for (const part of parts.slice(2)) {
      current = `${current}\\${part}`;
      segments.push(current);
    }

    return segments.map((segment) => ({
      value: segment,
      label: segment.startsWith('\\\\') && segment.replace(/^\\\\+/, '').split('\\').filter(Boolean).length <= 2 ? segment : compactPathLabel(segment),
    }));
  }

  const driveMatch = normalized.match(/^[A-Za-z]:\\/);
  if (!driveMatch) {
    return [{ value: normalized, label: compactPathLabel(normalized) }];
  }

  const root = driveMatch[0];
  const remainder = normalized.slice(root.length).split('\\').filter(Boolean);
  const segments = [root];
  let current = root.endsWith('\\') ? root.slice(0, -1) : root;
  for (const part of remainder) {
    current = `${current}\\${part}`;
    segments.push(current);
  }

  return segments.map((segment, index) => ({
    value: segment,
    label: index === 0 ? segment.replace(/\\$/, '') : compactPathLabel(segment),
  }));
}

function inferRiskLevel(row: PreviewLedgerRow) {
  const provided = (row.risk_level || '').trim().toLowerCase();
  if (provided === 'high' || provided === 'medium' || provided === 'low') {
    return provided;
  }

  const value = (row.permissions || '').trim().toLowerCase();
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

function accountDisplay(row: PreviewLedgerRow) {
  return row.account_name || row.trustee || row.trustee_sid || '-';
}

function secondaryIdentity(row: PreviewLedgerRow) {
  const primary = accountDisplay(row);
  if (row.trustee && row.trustee !== primary) {
    return row.trustee;
  }
  if (row.trustee_sid && row.trustee_sid !== primary) {
    return row.trustee_sid;
  }
  return '';
}

function riskNote(locale: string, level: string) {
  switch (level) {
    case 'high':
      return text(locale, 'High risk', '高风险');
    case 'medium':
      return text(locale, 'Needs review', '建议复核');
    default:
      return '';
  }
}

function riskNoteTone(level: string): BadgeTone {
  switch (level) {
    case 'high':
      return 'danger';
    case 'medium':
      return 'warning';
    default:
      return 'neutral';
  }
}

function CompactMetaCard({ label, value, detail }: CompactMetaCardProps) {
  return (
    <div className="bg-surface-base px-3 py-2.5">
      <dt className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{label}</dt>
      <dd className="mt-1 break-words text-[13px] font-medium leading-5 text-fg">{value || '-'}</dd>
      <div className="mt-0.5 text-2xs leading-4 text-fg-muted">{detail}</div>
    </div>
  );
}

function KpiCard({ label, value, detail, icon: Icon, iconClassName }: KpiCardProps) {
  return (
    <div className="bg-surface-base px-3 py-2.5">
      <div className="flex items-start justify-between gap-2.5">
        <div className="min-w-0">
          <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{label}</div>
          <div className="mt-1 text-[1.08rem] font-semibold leading-none tabular-nums text-fg">{value}</div>
        </div>
        <div className={cn('flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-surface-sunken', iconClassName)}>
          <Icon className="h-3.5 w-3.5" />
        </div>
      </div>
      <div className="mt-1 text-2xs leading-4 text-fg-muted">{detail}</div>
    </div>
  );
}

export default function ExportReportPreviewV2({
  locale,
  reportTitle,
  templateTitle,
  templateSummary,
  previewGeneratedAt,
  normalizedPath,
  modeSummary,
  sharePresetLabel,
  stats,
  previewUserRows,
  previewAclRows,
  previewNarrativeLead,
  previewNarrativeBullets,
}: ExportReportPreviewProps) {
  const uniquePaths = useMemo(
    () =>
      Array.from(
        new Set(
          previewUserRows
            .map((row) => row.path.trim())
            .filter(Boolean)
        )
      ),
    [previewUserRows]
  );

  const pathSummaries = useMemo<PathSummary[]>(() => {
    const counts = new Map<string, number>();
    for (const row of previewUserRows) {
      const path = row.path.trim();
      if (!path) {
        continue;
      }
      counts.set(path, (counts.get(path) || 0) + 1);
    }

    return Array.from(counts.entries())
      .map(([path, rowCount]) => ({
        path,
        label: compactPathLabel(path),
        rowCount,
      }))
      .sort((left, right) => {
        if (right.rowCount !== left.rowCount) {
          return right.rowCount - left.rowCount;
        }
        return left.path.localeCompare(right.path, undefined, { sensitivity: 'base' });
      });
  }, [previewUserRows]);

  const hasMultiplePaths = uniquePaths.length > 1;
  const scopePath = normalizedPath || uniquePaths[0] || text(locale, 'Awaiting scope path', '等待路径');
  const highRiskRows = useMemo(
    () => previewUserRows.filter((row) => inferRiskLevel(row) === 'high').length,
    [previewUserRows]
  );
  const rawRecordCount = previewAclRows.length || stats.visible || previewUserRows.length;
  const scopeTrail = useMemo(() => scopeBreadcrumbs(scopePath), [scopePath]);
  const deskNotes = previewNarrativeBullets.filter(Boolean).slice(0, 2);
  const deskSummary =
    previewNarrativeLead ||
    templateSummary ||
    text(
      locale,
      'Review summarized permission exposure by path, account, inheritance, and originating group in one continuous ledger.',
        '在一张连续台账中集中复核路径、账户、继承方式与来源组对应的权限暴露情况。'
      );
  const deskSupportStrip = [deskSummary, ...deskNotes].filter(Boolean).join(' · ');
  const pathSummaryLine = useMemo(() => {
    if (pathSummaries.length === 0) {
      return '';
    }

    const primary = pathSummaries
      .slice(0, 3)
      .map((item) => `${item.label} (${formatNumber(item.rowCount)})`)
      .join(' · ');

    if (pathSummaries.length <= 3) {
      return primary;
    }

    return locale === 'zh-CN'
      ? `${primary} · 另含 ${formatNumber(pathSummaries.length - 3)} 个路径`
      : `${primary} + ${formatNumber(pathSummaries.length - 3)} more paths`;
  }, [locale, pathSummaries]);

  const coverageLabel =
    stats.total > 0
      ? `${formatNumber(stats.visible || rawRecordCount)} / ${formatNumber(stats.total)}`
      : formatNumber(rawRecordCount);
  const reviewModeLabel = hasMultiplePaths
    ? text(locale, 'Cross-path permission review', '跨路径权限复核')
    : text(locale, 'Single-path permission review', '单路径权限复核');
  const ledgerSummary = hasMultiplePaths
    ? text(
        locale,
        `Included paths: ${pathSummaryLine}`,
        `已包含路径：${pathSummaryLine}`
      )
    : modeSummary;

  return (
    <article className="overflow-hidden rounded-lg border border-line bg-surface-base text-fg shadow-token-sm">
      <header>
        <div className="border-b border-line bg-surface-raised px-4 py-3 sm:px-5">
          <div className="flex flex-col gap-2.5 xl:flex-row xl:items-start xl:justify-between">
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center gap-1.5 text-2xs font-medium uppercase tracking-wide text-fg-muted">
                <Badge tone="neutral">
                  {text(locale, 'Permission Desk', '权限工作台')}
                </Badge>
                {scopeTrail.length > 0
                  ? scopeTrail.map((item, index) => (
                      <span key={item.value} className="inline-flex items-center gap-1.5 text-fg-muted">
                        {index > 0 ? <ChevronRight className="h-3 w-3 text-fg-faint" /> : null}
                        <span className="max-w-[240px] truncate text-fg-secondary">{item.label}</span>
                      </span>
                    ))
                  : (
                      <span className="text-fg-secondary">{text(locale, 'Awaiting scope path', '等待路径')}</span>
                    )}
              </div>
              <div className="mt-2 font-mono text-2xs text-fg-muted">{scopePath}</div>
            </div>

            <div className="flex flex-wrap gap-1.5">
              <Badge tone="neutral">
                {sharePresetLabel}
              </Badge>
              {stats.skipped > 0 ? (
                <Badge tone="warning">
                  {text(locale, `${formatNumber(stats.skipped)} skipped`, `${formatNumber(stats.skipped)} 条已跳过`)}
                </Badge>
              ) : null}
            </div>
          </div>
        </div>

        <div className="border-b border-line bg-surface-base px-4 py-3 sm:px-5">
          <div className="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
            <div className="min-w-0 max-w-4xl flex-1">
              <div className="flex flex-wrap items-center gap-2 text-2xs font-medium uppercase tracking-wide text-fg-muted">
                {text(locale, 'Authoritative Preview Ledger', '权威预览台账')}
                {highRiskRows > 0 ? (
                  <Badge tone="danger" className="uppercase tracking-wide">
                    <ShieldAlert className="h-3 w-3" />
                    {text(locale, `${formatNumber(highRiskRows)} high-risk summaries`, `${formatNumber(highRiskRows)} 条高风险汇总`)}
                  </Badge>
                ) : null}
              </div>
              <div className="mt-1.5">
                <h3 className="text-[1.45rem] font-semibold tracking-tight text-fg sm:text-[1.65rem]">{reportTitle}</h3>
              </div>
              <div className="mt-2 rounded-lg border border-line bg-surface-sunken px-3 py-2 text-[11px] leading-5 text-fg-muted">
                {deskSupportStrip}
              </div>
            </div>

            <dl className="grid gap-px overflow-hidden rounded-lg border border-line bg-line sm:grid-cols-2 xl:w-[430px] xl:min-w-[430px]">
              <CompactMetaCard
                label={text(locale, 'Template', '模板')}
                value={templateTitle || '-'}
                detail={text(locale, 'Report lens', '报告视角')}
              />
              <CompactMetaCard
                label={text(locale, 'Generated', '生成时间')}
                value={previewGeneratedAt || '-'}
                detail={text(locale, 'Preview snapshot', '预览快照')}
              />
              <CompactMetaCard
                label={text(locale, 'Review Mode', '复核模式')}
                value={reviewModeLabel}
                detail={modeSummary || text(locale, 'Current selection context', '当前选择上下文')}
              />
              <CompactMetaCard
                label={text(locale, 'Coverage', '覆盖率')}
                value={coverageLabel}
                detail={
                  stats.total > 0
                    ? text(locale, 'Visible / total source records', '可见 / 总源记录')
                    : text(locale, 'Visible source records', '可见源记录')
                }
              />
            </dl>
          </div>
        </div>

        <section className="border-b border-line bg-surface-raised px-4 py-2.5 sm:px-5">
          <div className="grid gap-px overflow-hidden rounded-lg border border-line bg-line md:grid-cols-2 xl:grid-cols-4">
            <KpiCard
              label={text(locale, 'Summarized Rows', '汇总行')}
              value={formatNumber(previewUserRows.length)}
              detail={text(locale, 'Path × account review rows', '路径 × 账户复核行')}
              icon={UsersRound}
              iconClassName="text-info"
            />
            <KpiCard
              label={text(locale, 'Paths in Scope', '路径范围')}
              value={formatNumber(uniquePaths.length)}
              detail={hasMultiplePaths ? pathSummaryLine : compactPathLabel(scopePath)}
              icon={FolderOpen}
              iconClassName="text-warning"
            />
            <KpiCard
              label={text(locale, 'Source Records', '源记录')}
              value={formatNumber(rawRecordCount)}
              detail={text(locale, 'Visible ACL entries in preview', '预览中的可见 ACL 条目')}
              icon={FileText}
              iconClassName="text-success"
            />
            <KpiCard
              label={text(locale, 'Elevated Risk', '高风险')}
              value={formatNumber(highRiskRows)}
              detail={
                stats.highRisk > highRiskRows
                  ? text(locale, `${formatNumber(stats.highRisk)} flagged source records`, `${formatNumber(stats.highRisk)} 条源记录已标记`)
                  : text(locale, 'Summaries requiring priority review', '需要优先复核的汇总行')
              }
              icon={ShieldAlert}
              iconClassName="text-danger"
            />
          </div>
        </section>
      </header>

      <section>
        <div className="flex flex-col gap-2.5 border-b border-line bg-surface-raised px-4 py-2.5 sm:px-5 xl:flex-row xl:items-center xl:justify-between">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2 text-sm font-semibold text-fg">
              <FileText className="h-4 w-4 text-accent-fg" />
              {text(locale, 'Permission Ledger', '权限台账')}
              <Badge tone="neutral">
                {text(locale, `${formatNumber(previewUserRows.length)} summarized rows`, `${formatNumber(previewUserRows.length)} 条汇总行`)}
              </Badge>
            </div>
            <div className="mt-1 text-[11px] leading-4 text-fg-muted">{ledgerSummary}</div>
          </div>

          <div className="flex flex-wrap gap-1.5">
            <Badge tone="neutral">
              {text(locale, 'Grouping: Path × Account', '分组：路径 × 账户')}
            </Badge>
            <Badge tone="neutral">
              {text(locale, 'Identity: Account / Name / E-Mail', '身份：账户 / 姓名 / 邮箱')}
            </Badge>
            <Badge tone="neutral">
              {text(locale, 'Traceability: Group / Permission / Inheritance', '可追溯性：来源组 / 权限 / 继承')}
            </Badge>
          </div>
        </div>

        <TableContainer className="max-h-[74vh] rounded-none border-0">
          <Table className="min-w-[1540px]">
          <caption className="sr-only">
            {text(
              locale,
              'Permission preview ledger aligned to the detailed review with columns for path, account, identity, permissions, and inheritance.',
              '与详细复核对齐的权限预览台账，包含路径、账户、身份、权限和继承等列。'
            )}
          </caption>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
                <TableHead className="py-2.5">{text(locale, 'Path', '路径')}</TableHead>
                <TableHead className="py-2.5">{text(locale, 'Account Name', '账户名')}</TableHead>
                <TableHead className="py-2.5">{text(locale, 'Name', '姓名')}</TableHead>
                <TableHead className="py-2.5">{text(locale, 'E-Mail', '邮箱')}</TableHead>
                <TableHead className="py-2.5">{text(locale, 'Department', '部门')}</TableHead>
                <TableHead className="py-2.5">{text(locale, 'Domain', '域')}</TableHead>
                <TableHead className="py-2.5">{text(locale, 'Originating Group', '来源组')}</TableHead>
                <TableHead className="py-2.5">{text(locale, 'Permissions', '权限')}</TableHead>
                <TableHead className="py-2.5">{text(locale, 'Inheritance', '继承')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {previewUserRows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={9} className="px-3 py-12 text-center text-sm text-fg-muted">
                  {text(locale, 'No report rows are available yet.', '当前还没有可显示的报告结果。')}
                </TableCell>
              </TableRow>
            ) : (
              previewUserRows.map((row) => {
                const riskLevel = inferRiskLevel(row);
                const helperIdentity = secondaryIdentity(row);
                const riskText = riskNote(locale, riskLevel);
                const primaryInheritance = row.inheritance_summary || row.group_inheritance_hierarchy || '-';
                const appliesSummary = row.applies_to_summary?.trim();
                const inheritanceChain = row.group_inheritance_hierarchy?.trim();
                const showInheritanceChain = Boolean(
                  inheritanceChain && inheritanceChain !== primaryInheritance && inheritanceChain !== appliesSummary
                );

                return (
                  <TableRow key={row.id} className="align-top">
                    <TableCell className="max-w-[320px] py-2.5 align-top">
                      <div className="flex flex-wrap items-center gap-2">
                        <div className="font-medium text-fg">{compactPathLabel(row.path)}</div>
                        {row.row_count ? (
                          <Badge tone="neutral">
                            {text(locale, `${formatNumber(row.row_count)} source rows`, `${formatNumber(row.row_count)} 条源记录`)}
                          </Badge>
                        ) : null}
                      </div>
                        <div className="mt-0.5 font-mono text-2xs text-fg-muted">{row.path || '-'}</div>
                    </TableCell>

                    <TableCell className="py-2.5 align-top">
                      <div className="font-medium text-fg">{accountDisplay(row)}</div>
                      {helperIdentity ? <div className="mt-0.5 break-all text-[11px] leading-4 text-fg-muted">{helperIdentity}</div> : null}
                    </TableCell>

                    <TableCell className="py-2.5 align-top">
                      <div className="text-fg">{displayName(row)}</div>
                    </TableCell>

                    <TableCell className="max-w-[220px] py-2.5 align-top">
                      <div className={`break-all leading-5 ${row.email ? 'text-fg' : 'text-fg-muted'}`}>{row.email || '-'}</div>
                    </TableCell>

                    <TableCell className="py-2.5 align-top">
                      <div className="text-fg">{row.department || '-'}</div>
                      {row.division ? (
                        <div className="mt-0.5 text-[11px] leading-4 text-fg-muted">
                          {text(locale, 'Division', '事业部')}: {row.division}
                        </div>
                      ) : null}
                    </TableCell>

                    <TableCell className="py-2.5 align-top text-fg-secondary">
                      {row.domain || '-'}
                    </TableCell>

                    <TableCell className="max-w-[220px] py-2.5 align-top text-fg-secondary">
                      <div className="break-words leading-5">{row.originating_group || '-'}</div>
                    </TableCell>

                    <TableCell className="max-w-[320px] py-2.5 align-top">
                      <div className="break-words leading-5 text-fg">{row.permissions || '-'}</div>
                      {riskText || row.permission_count ? (
                        <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
                          {riskText ? (
                            <Badge tone={riskNoteTone(riskLevel)} className="uppercase tracking-wide">
                              {riskText}
                            </Badge>
                          ) : null}
                          {row.permission_count ? (
                            <Badge tone="neutral">
                              {text(locale, `${formatNumber(row.permission_count)} permission types`, `${formatNumber(row.permission_count)} 种权限类型`)}
                            </Badge>
                          ) : null}
                        </div>
                      ) : null}
                    </TableCell>

                    <TableCell className="max-w-[260px] py-2.5 align-top">
                      <div className="text-fg">{primaryInheritance}</div>
                      {appliesSummary ? (
                        <div className="mt-0.5 break-words text-[11px] leading-4 text-fg-muted">
                          {text(locale, 'Applies to', '适用范围')}: {appliesSummary}
                        </div>
                      ) : null}
                      {showInheritanceChain ? (
                        <div className="mt-0.5 break-words text-[11px] leading-4 text-fg-muted">
                          {text(locale, 'Chain', '继承链')}: {inheritanceChain}
                        </div>
                      ) : null}
                    </TableCell>
                  </TableRow>
                );
              })
            )}
          </TableBody>
          </Table>
        </TableContainer>
      </section>

      <footer className="border-t border-line bg-surface-raised px-4 py-2.5 sm:px-5">
        <div className="flex flex-col gap-1.5 text-2xs leading-4 text-fg-muted lg:flex-row lg:items-center lg:justify-between">
          <div className="min-w-0">
            {hasMultiplePaths && pathSummaryLine ? `${text(locale, 'Included paths', '已包含路径')}: ${pathSummaryLine}` : modeSummary}
          </div>
          <div className="min-w-0 lg:text-right">
            {text(locale, 'Path × account semantics stay aligned with the current detailed review workflow.', '“路径 × 账户”的台账语义与当前详细复核工作流保持一致。')}
          </div>
        </div>
      </footer>
    </article>
  );
}
