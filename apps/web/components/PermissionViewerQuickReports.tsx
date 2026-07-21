import { ArrowRight, ClipboardList, FolderSearch, History, TableProperties, UsersRound } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { cn } from '../lib/cn';
import { Badge, riskTone } from './ui/badge';
import { Button } from './ui/button';
import { EmptyState } from './ui/empty-state';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './ui/table';

interface PermissionLike {
  path: string;
  trustee: string;
  trustee_sid: string;
  rights: string;
  type: string;
  inherited: boolean;
  risk_level?: string;
  account_name?: string;
  department?: string;
  division?: string;
  domain?: string;
  email?: string;
  originating_group?: string;
  group_inheritance_hierarchy?: string;
}

export type QuickReportMode = 'user' | 'folder' | 'owner';

interface PermissionViewerQuickReportsProps {
  locale: string;
  permissions: PermissionLike[];
  embedded?: boolean;
  adReady: boolean;
  exactPathCount: number;
  exportReady: boolean;
  initialMode?: QuickReportMode;
  mode?: QuickReportMode;
  onModeChange?: (mode: QuickReportMode) => void;
  onApplyTrusteeFilter: (value: string) => void;
  onApplyPathFilter: (value: string) => void;
  onClearFilters: () => void;
  onOpenReportControls: () => void;
  onOpenHistory: () => void;
}

interface TrusteeSummary {
  trustee: string;
  sid: string;
  rowCount: number;
  pathCount: number;
  highRiskCount: number;
  explicitCount: number;
  originatingGroups: string[];
  rows: PermissionLike[];
}

interface FolderSummary {
  path: string;
  rowCount: number;
  trusteeCount: number;
  highRiskCount: number;
  explicitCount: number;
  inheritedCount: number;
}

interface OwnerSummary {
  owner: string;
  rowCount: number;
  trusteeCount: number;
  pathCount: number;
  highRiskCount: number;
  explicitCount: number;
  topPath: string;
  rows: PermissionLike[];
}

function text(locale: string, en: string, zh: string) {
  return locale === 'zh-CN' ? zh : en;
}

function formatNumber(value: number) {
  return new Intl.NumberFormat().format(value);
}

function identityFor(item: PermissionLike) {
  return [item.account_name, item.trustee, item.trustee_sid]
    .map((value) => (value || '').trim())
    .find(Boolean) || 'Unknown';
}

function compactPath(path: string) {
  const normalized = path.replace(/\//g, '\\');
  const parts = normalized.split('\\').filter(Boolean);
  if (parts.length <= 2) {
    return normalized || '-';
  }
  return `${parts.slice(-2).join('\\')}`;
}

function ownerFor(item: PermissionLike) {
  return [item.department, item.division, item.originating_group, item.domain, item.email]
    .map((value) => (value || '').trim())
    .find(Boolean) || 'Unassigned';
}

function riskLevel(item: PermissionLike) {
  const provided = (item.risk_level || '').trim().toLowerCase();
  if (provided === 'high' || provided === 'medium' || provided === 'low') {
    return provided;
  }
  const rights = (item.rights || '').toLowerCase();
  if (['full control', 'fullcontrol', 'write', 'modify', 'delete', 'take ownership', 'change permissions'].some((token) => rights.includes(token))) {
    return 'high';
  }
  if (rights.includes('execute')) {
    return 'medium';
  }
  return 'low';
}

function riskLabel(locale: string, level: string) {
  if (locale !== 'zh-CN') return level;
  if (level === 'high') return '高';
  if (level === 'medium') return '中';
  return '低';
}

export default function PermissionViewerQuickReports({
  locale,
  permissions,
  embedded = false,
  initialMode = 'user',
  mode: controlledMode,
  onModeChange,
  onApplyTrusteeFilter,
  onApplyPathFilter,
  onClearFilters,
  onOpenReportControls,
  onOpenHistory,
}: PermissionViewerQuickReportsProps) {
  const [internalMode, setInternalMode] = useState<QuickReportMode>(initialMode);
  const mode = controlledMode ?? internalMode;
  const [selectedTrustee, setSelectedTrustee] = useState('');
  const [selectedOwner, setSelectedOwner] = useState('');

  useEffect(() => {
    if (controlledMode === undefined) {
      setInternalMode(initialMode);
    }
  }, [controlledMode, initialMode]);

  const selectMode = (nextMode: QuickReportMode) => {
    if (controlledMode === undefined) {
      setInternalMode(nextMode);
    }
    onModeChange?.(nextMode);
  };

  const trusteeSummaries = useMemo<TrusteeSummary[]>(() => {
    const grouped = new Map<string, TrusteeSummary>();

    for (const item of permissions) {
      const trustee = identityFor(item);
      const current = grouped.get(trustee) || {
        trustee,
        sid: item.trustee_sid || '',
        rowCount: 0,
        pathCount: 0,
        highRiskCount: 0,
        explicitCount: 0,
        originatingGroups: [],
        rows: [],
      };

      current.rowCount += 1;
      current.highRiskCount += riskLevel(item) === 'high' ? 1 : 0;
      current.explicitCount += item.inherited ? 0 : 1;
      current.rows.push(item);
      if (item.originating_group && !current.originatingGroups.includes(item.originating_group)) {
        current.originatingGroups.push(item.originating_group);
      }
      grouped.set(trustee, current);
    }

    return Array.from(grouped.values())
      .map((item) => ({
        ...item,
        pathCount: new Set(item.rows.map((row) => row.path)).size,
        rows: [...item.rows].sort((a, b) => {
          const riskDelta = (riskLevel(b) === 'high' ? 1 : 0) - (riskLevel(a) === 'high' ? 1 : 0);
          return riskDelta || a.path.localeCompare(b.path, undefined, { sensitivity: 'base' });
        }),
      }))
      .sort((a, b) => b.highRiskCount - a.highRiskCount || b.rowCount - a.rowCount || a.trustee.localeCompare(b.trustee, undefined, { sensitivity: 'base' }))
      .slice(0, 12);
  }, [permissions]);

  const folderSummaries = useMemo<FolderSummary[]>(() => {
    const grouped = new Map<string, PermissionLike[]>();
    for (const item of permissions) {
      const current = grouped.get(item.path) || [];
      current.push(item);
      grouped.set(item.path, current);
    }

    return Array.from(grouped.entries())
      .map(([path, rows]) => ({
        path,
        rowCount: rows.length,
        trusteeCount: new Set(rows.map((row) => identityFor(row).toLowerCase())).size,
        highRiskCount: rows.filter((row) => riskLevel(row) === 'high').length,
        explicitCount: rows.filter((row) => !row.inherited).length,
        inheritedCount: rows.filter((row) => row.inherited).length,
      }))
      .sort((a, b) => b.highRiskCount - a.highRiskCount || b.rowCount - a.rowCount || a.path.localeCompare(b.path, undefined, { sensitivity: 'base' }))
      .slice(0, 10);
  }, [permissions]);

  const ownerSummaries = useMemo<OwnerSummary[]>(() => {
    const grouped = new Map<string, PermissionLike[]>();
    for (const item of permissions) {
      const owner = ownerFor(item);
      const current = grouped.get(owner) || [];
      current.push(item);
      grouped.set(owner, current);
    }

    return Array.from(grouped.entries())
      .map(([owner, rows]) => {
        const pathCounts = new Map<string, number>();
        rows.forEach((row) => pathCounts.set(row.path, (pathCounts.get(row.path) || 0) + 1));
        const topPath = Array.from(pathCounts.entries()).sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0], undefined, { sensitivity: 'base' }))[0]?.[0] || '';

        return {
          owner,
          rowCount: rows.length,
          trusteeCount: new Set(rows.map((row) => identityFor(row).toLowerCase())).size,
          pathCount: new Set(rows.map((row) => row.path)).size,
          highRiskCount: rows.filter((row) => riskLevel(row) === 'high').length,
          explicitCount: rows.filter((row) => !row.inherited).length,
          topPath,
          rows: [...rows].sort((a, b) => {
            const riskDelta = (riskLevel(b) === 'high' ? 1 : 0) - (riskLevel(a) === 'high' ? 1 : 0);
            return riskDelta || a.path.localeCompare(b.path, undefined, { sensitivity: 'base' });
          }),
        };
      })
      .sort((a, b) => b.highRiskCount - a.highRiskCount || b.explicitCount - a.explicitCount || b.rowCount - a.rowCount || a.owner.localeCompare(b.owner, undefined, { sensitivity: 'base' }))
      .slice(0, 10);
  }, [permissions]);

  const activeTrustee = trusteeSummaries.find((item) => item.trustee === selectedTrustee) || trusteeSummaries[0];
  const activeRows = activeTrustee?.rows.slice(0, 8) || [];
  const activeOwner = ownerSummaries.find((item) => item.owner === selectedOwner) || ownerSummaries[0];
  const activeOwnerRows = activeOwner?.rows.slice(0, 6) || [];

  const tabClass = (active: boolean) =>
    cn(
      'inline-flex items-center gap-1.5 whitespace-nowrap rounded-md px-3 py-1 text-xs font-medium transition-colors',
      'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-line-focus',
      active ? 'bg-surface-base text-fg shadow-token-sm' : 'text-fg-muted hover:text-fg-secondary',
    );
  const reportActions = (
    <div className="flex flex-wrap items-center gap-2">
      <Button type="button" size="sm" variant="primary" onClick={onOpenReportControls}>
        <TableProperties className="h-3.5 w-3.5" />
        {text(locale, 'Configure exports', '配置导出')}
      </Button>
      <Button type="button" size="sm" variant="secondary" onClick={onOpenHistory}>
        <History className="h-3.5 w-3.5" />
        {text(locale, 'Compare saved runs', '对比历史运行')}
      </Button>
    </div>
  );

  return (
    <section
      className={cn(
        'flex flex-col gap-4',
        embedded
          ? 'permission-quick-reports-embedded'
          : 'rounded-lg border border-line bg-surface-base p-4 shadow-token-sm sm:p-5',
      )}
      aria-labelledby={embedded ? undefined : 'permissionviewer-reports-title'}
      aria-label={embedded ? text(locale, 'User, folder, and owner reports', '用户、文件夹与责任人报告') : undefined}
    >
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        {!embedded ? <div className="min-w-0">
          <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Report workspace', '报告工作台')}</div>
          <h3 id="permissionviewer-reports-title" className="mt-1 text-sm font-semibold text-fg">
            {text(locale, 'User, folder, and owner reports', '用户、文件夹与责任人报告')}
          </h3>
          <p className="mt-1 text-xs text-fg-muted">{text(locale, 'Review the selected scan by user, folder, or owner before configuring the export.', '按用户、文件夹或责任人查看所选扫描，再配置导出。')}</p>
        </div> : null}
        <div className="inline-flex h-9 shrink-0 items-center gap-1 self-start rounded-lg bg-surface-sunken p-1" role="tablist" aria-label={text(locale, 'Quick report mode', '快速报告模式')}>
          <button type="button" role="tab" aria-selected={mode === 'user'} onClick={() => selectMode('user')} className={tabClass(mode === 'user')}>
            <UsersRound className="h-4 w-4" />
            {text(locale, 'User Access', '用户访问')}
          </button>
          <button type="button" role="tab" aria-selected={mode === 'folder'} onClick={() => selectMode('folder')} className={tabClass(mode === 'folder')}>
            <FolderSearch className="h-4 w-4" />
            {text(locale, 'Folder Report', '文件夹报告')}
          </button>
          <button type="button" role="tab" aria-selected={mode === 'owner'} onClick={() => selectMode('owner')} className={tabClass(mode === 'owner')}>
            <ClipboardList className="h-4 w-4" />
            {text(locale, 'Owner Review', '责任人复核')}
          </button>
        </div>
        {embedded ? reportActions : null}
      </div>

      {!embedded ? reportActions : null}

      {permissions.length === 0 ? (
        <EmptyState
          icon={TableProperties}
          title={text(locale, 'Run or import a scan to populate reports.', '执行或导入一次扫描后，这里会生成报告视图。')}
          className="py-8"
        />
      ) : mode === 'user' ? (
        <div className="grid gap-3 md:grid-cols-[240px_minmax(0,1fr)] xl:grid-cols-[280px_minmax(0,1fr)]">
          <aside className="flex min-w-0 flex-col gap-2">
            {trusteeSummaries.map((item) => {
              const active = item.trustee === activeTrustee?.trustee;
              return (
                <button
                  key={item.trustee}
                  type="button"
                  onClick={() => setSelectedTrustee(item.trustee)}
                  className={cn(
                    'flex flex-col gap-1 rounded-md border px-3 py-2.5 text-left transition-colors',
                    active ? 'border-accent/50 bg-accent-soft' : 'border-line bg-surface-base hover:bg-surface-sunken',
                  )}
                >
                  <span className="truncate text-xs font-semibold text-fg">{item.trustee}</span>
                  <span className="text-2xs text-fg-muted">
                    {formatNumber(item.pathCount)} {text(locale, 'paths', '路径')} · {formatNumber(item.rowCount)} {text(locale, 'rows', '行')}
                  </span>
                  <span className="flex flex-wrap items-center gap-1.5">
                    <Badge tone={item.highRiskCount > 0 ? 'danger' : 'success'}>{text(locale, 'Risk', '风险')} {formatNumber(item.highRiskCount)}</Badge>
                    <Badge tone="neutral">{text(locale, 'Explicit', '显式')} {formatNumber(item.explicitCount)}</Badge>
                  </span>
                </button>
              );
            })}
          </aside>

          <div className="min-w-0 overflow-hidden rounded-lg border border-line bg-surface-base">
            <div className="flex flex-wrap items-start justify-between gap-2 px-3 py-2.5 sm:px-4">
              <div className="min-w-0">
                <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'User permission report', '用户权限报告')}</div>
                <div className="mt-0.5 truncate text-sm font-semibold text-fg">{activeTrustee?.trustee || '-'}</div>
              </div>
              <div className="flex shrink-0 items-center gap-1.5">
                <Button type="button" size="sm" variant="secondary" onClick={() => activeTrustee && onApplyTrusteeFilter(activeTrustee.trustee)} disabled={!activeTrustee}>
                  {text(locale, 'Filter user', '筛选用户')}
                </Button>
                <Button type="button" size="sm" variant="secondary" onClick={onClearFilters}>
                  {text(locale, 'Clear filters', '清空过滤')}
                </Button>
              </div>
            </div>
            <div className="overflow-x-auto border-t border-line">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead>{text(locale, 'Path', '路径')}</TableHead>
                    <TableHead>{text(locale, 'Permissions', '权限')}</TableHead>
                    <TableHead>{text(locale, 'Via group', '来源组')}</TableHead>
                    <TableHead>{text(locale, 'Risk', '风险')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {activeRows.map((row, index) => {
                    const level = riskLevel(row);
                    return (
                      <TableRow key={`${row.path}-${row.trustee_sid}-${row.rights}-${index}`} className="align-top">
                        <TableCell className="py-2 align-top"><span className="break-all font-mono text-2xs">{row.path}</span></TableCell>
                        <TableCell className="py-2 align-top">{row.rights || '-'}</TableCell>
                        <TableCell className="py-2 align-top">{row.originating_group || row.group_inheritance_hierarchy || '-'}</TableCell>
                        <TableCell className="py-2 align-top"><Badge tone={riskTone(level)}>{riskLabel(locale, level)}</Badge></TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          </div>
        </div>
      ) : mode === 'folder' ? (
        <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
          {folderSummaries.map((item) => (
            <button
              key={item.path}
              type="button"
              onClick={() => onApplyPathFilter(item.path)}
              className="rounded-lg border border-line bg-surface-base p-3 text-left transition-colors hover:border-accent/40 hover:bg-surface-sunken"
            >
              <div className="flex items-start justify-between gap-2">
                <span className="min-w-0">
                  <strong className="block truncate text-xs font-semibold text-fg">{compactPath(item.path)}</strong>
                  <small className="block break-all font-mono text-2xs text-fg-muted">{item.path}</small>
                </span>
                <ArrowRight className="h-4 w-4 shrink-0 text-fg-faint" />
              </div>
              <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-2xs text-fg-muted">
                <span>{text(locale, 'Users', '用户')} <strong className="text-fg">{formatNumber(item.trusteeCount)}</strong></span>
                <span>{text(locale, 'Rows', '行')} <strong className="text-fg">{formatNumber(item.rowCount)}</strong></span>
                <span>{text(locale, 'Risk', '风险')} <strong className="text-fg">{formatNumber(item.highRiskCount)}</strong></span>
                <span>{text(locale, 'Explicit', '显式')} <strong className="text-fg">{formatNumber(item.explicitCount)}</strong></span>
              </div>
            </button>
          ))}
        </div>
      ) : (
        <div className="grid gap-3 md:grid-cols-[240px_minmax(0,1fr)] xl:grid-cols-[280px_minmax(0,1fr)]">
          <aside className="flex min-w-0 flex-col gap-2">
            {ownerSummaries.map((item) => {
              const active = item.owner === activeOwner?.owner;
              return (
                <button
                  key={item.owner}
                  type="button"
                  onClick={() => setSelectedOwner(item.owner)}
                  className={cn(
                    'flex flex-col gap-1 rounded-md border px-3 py-2.5 text-left transition-colors',
                    active ? 'border-accent/50 bg-accent-soft' : 'border-line bg-surface-base hover:bg-surface-sunken',
                  )}
                >
                  <span className="truncate text-xs font-semibold text-fg">{item.owner}</span>
                  <span className="text-2xs text-fg-muted">
                    {formatNumber(item.trusteeCount)} {text(locale, 'users', '用户')} · {formatNumber(item.pathCount)} {text(locale, 'paths', '路径')}
                  </span>
                  <span className="flex flex-wrap items-center gap-1.5">
                    <Badge tone={item.highRiskCount > 0 ? 'danger' : 'success'}>{text(locale, 'Risk', '风险')} {formatNumber(item.highRiskCount)}</Badge>
                    <Badge tone="neutral">{text(locale, 'Explicit', '显式')} {formatNumber(item.explicitCount)}</Badge>
                  </span>
                </button>
              );
            })}
          </aside>

          <div className="min-w-0 overflow-hidden rounded-lg border border-line bg-surface-base">
            <div className="flex flex-wrap items-start justify-between gap-2 px-3 py-2.5 sm:px-4">
              <div className="min-w-0">
                <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Remediation queue', '整改队列')}</div>
                <div className="mt-0.5 truncate text-sm font-semibold text-fg">{activeOwner?.owner || '-'}</div>
                <p className="mt-0.5 text-2xs text-fg-muted">
                  {activeOwner?.topPath ? text(locale, `Most frequent path: ${activeOwner.topPath}`, `最高频路径：${activeOwner.topPath}`) : text(locale, 'No owner queue yet.', '暂无责任人队列。')}
                </p>
              </div>
              <div className="flex shrink-0 items-center gap-1.5">
                <Button type="button" size="sm" variant="secondary" onClick={() => activeOwner?.topPath && onApplyPathFilter(activeOwner.topPath)} disabled={!activeOwner?.topPath}>
                  {text(locale, 'Filter top path', '筛选最高频路径')}
                </Button>
                <Button type="button" size="sm" variant="secondary" onClick={onClearFilters}>
                  {text(locale, 'Clear filters', '清空过滤')}
                </Button>
              </div>
            </div>
            <div className="flex flex-col gap-2 border-t border-line p-3 sm:p-4">
              {activeOwnerRows.map((row, index) => (
                <article key={`${row.path}-${row.trustee_sid}-${row.rights}-${index}`} className="rounded-md border border-line bg-surface-sunken px-3 py-2.5">
                  <div className="min-w-0">
                    <strong className="block truncate text-xs font-semibold text-fg">{identityFor(row)}</strong>
                    <span className="block break-all font-mono text-2xs text-fg-muted">{row.path}</span>
                  </div>
                  <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
                    <Badge tone={riskTone(riskLevel(row))}>{riskLabel(locale, riskLevel(row))}</Badge>
                    <Badge tone="neutral">{row.inherited ? text(locale, 'Inherited', '继承') : text(locale, 'Explicit', '显式')}</Badge>
                  </div>
                  <p className="mt-1 text-xs text-fg-secondary">{row.rights || '-'}</p>
                </article>
              ))}
            </div>
          </div>
        </div>
      )}
    </section>
  );
}
