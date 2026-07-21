import { FileText, Filter, UsersRound } from 'lucide-react';
import { Badge, riskTone } from './ui/badge';
import { Button } from './ui/button';
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

interface PathSummary {
  path: string;
  label: string;
  rowCount: number;
  userCount: number;
  highRiskCount: number;
  explicitCount: number;
}

interface EnrichedPermissionLike {
  path: string;
  trustee: string;
  trustee_sid: string;
  rights: string;
  type: string;
  inherited: boolean;
  source?: string;
  applies_to?: string;
  account_type?: string;
  risk_level?: string;
  account_name?: string;
  originating_group?: string;
  group_inheritance_hierarchy?: string;
}

interface UserPermissionReportRow {
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
  group_inheritance_hierarchy: string;
  permissions: string;
  permission_count: number;
  risk_level: string;
  applies_to_summary: string;
  inheritance_summary: string;
  row_count: number;
  member_keys: string[];
}

interface PermissionReportWorkspaceProps {
  locale: string;
  filteredResultsCount: number;
  resultPathSummaries: PathSummary[];
  selectedResultPath: string;
  selectedPathSummary: PathSummary | null;
  selectedPathTokens: string[];
  adReady: boolean;
  compactTables: boolean;
  selectedPermission: EnrichedPermissionLike | null;
  selectedUserReportRow: UserPermissionReportRow | null;
  userReportRows: UserPermissionReportRow[];
  onClearFilters: () => void;
  onSelectResultPath: (path: string) => void;
  onSelectPermissionKey: (value: string) => void;
  onOpenDetail: () => void;
  riskBadgeClass: (level: string) => string;
}

function text(locale: string, en: string, zh: string) {
  return locale === 'zh-CN' ? zh : en;
}

function formatNumber(value: number) {
  return new Intl.NumberFormat().format(value);
}

export default function PermissionReportWorkspace({
  locale,
  filteredResultsCount,
  resultPathSummaries,
  selectedResultPath,
  selectedPathSummary,
  selectedPathTokens,
  adReady,
  compactTables,
  selectedPermission,
  selectedUserReportRow,
  userReportRows,
  onClearFilters,
  onSelectResultPath,
  onSelectPermissionKey,
  onOpenDetail,
  riskBadgeClass,
}: PermissionReportWorkspaceProps) {
  const currentReportUsers = selectedPathSummary?.userCount ?? userReportRows.length;
  const currentReportRecords = selectedPathSummary?.rowCount ?? userReportRows.length;

  return (
    <>
      <div className="mt-3 flex flex-wrap items-center gap-2">
        <Button type="button" variant="outline" size="sm" onClick={onClearFilters}>
          <Filter className="h-3.5 w-3.5" />
          {text(locale, 'Clear Filters', '清空过滤')}
        </Button>
        <Badge tone="neutral">
          {text(locale, 'Current Report Users', '当前报告用户数')}: {formatNumber(currentReportUsers)}
        </Badge>
        <Badge tone="neutral">
          {text(locale, 'Current Report Records', '当前报告记录数')}: {formatNumber(currentReportRecords)}
        </Badge>
        <Badge tone="neutral">
          {text(locale, 'Filtered Paths', '过滤后路径数')}: {formatNumber(resultPathSummaries.length)}
        </Badge>
        <Badge tone="neutral">
          {text(locale, 'Filtered Dataset Records', '过滤后总记录数')}: {formatNumber(filteredResultsCount)}
        </Badge>
        {selectedPathSummary && (
          <Badge tone="neutral">
            {text(locale, 'Current Path', '当前路径')}: {selectedPathSummary.label}
          </Badge>
        )}
      </div>
      <div className="hidden">
        <Button type="button" variant="outline" size="sm" onClick={onClearFilters}>
          <Filter className="h-3.5 w-3.5" />
          {text(locale, 'Clear Filters', '清空过滤')}
        </Button>
        <Badge tone="neutral">{text(locale, 'Visible Rows', '当前可见行')}: {formatNumber(filteredResultsCount)}</Badge>
        <Badge tone="neutral">{text(locale, 'Report Paths', '报告路径')}: {formatNumber(resultPathSummaries.length)}</Badge>
        <Badge tone="neutral">{text(locale, 'Active View', '当前视图')}: {text(locale, 'User Report', '用户报告')}</Badge>
        {selectedPathSummary && <Badge tone="neutral">{text(locale, 'Current Path', '当前路径')}: {selectedPathSummary.label}</Badge>}
      </div>

      <div className="mt-4 grid gap-4 xl:grid-cols-[260px_minmax(0,1fr)]">
        <aside className="rounded-lg border border-line bg-surface-base p-4 shadow-token-sm">
          <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Report Navigator', '报告导航')}</div>
          <div className="mt-2 text-sm text-fg-muted">
            {text(locale, 'Pick one scanned path and review its user report in place.', '选择一个已扫描路径，在右侧直接查看该路径的用户权限报告。')}
          </div>
          <div className="mt-4 space-y-2">
            {resultPathSummaries.length === 0 ? (
              <div className="rounded-lg border border-dashed border-line px-3 py-5 text-sm text-fg-muted">
                {text(locale, 'Run a scan or adjust filters to populate the report list.', '先执行扫描，或调整过滤条件后再查看报告列表。')}
              </div>
            ) : (
              resultPathSummaries.map((item) => {
                const active = item.path === selectedResultPath;
                return (
                  <button
                    key={item.path}
                    type="button"
                    onClick={() => onSelectResultPath(item.path)}
                    className={cn(
                      'w-full rounded-lg border px-3 py-3 text-left transition-colors',
                      active
                        ? 'border-accent/40 bg-accent-soft text-fg shadow-token-sm'
                        : 'border-line bg-surface-base text-fg-secondary hover:border-accent/30 hover:bg-surface-sunken hover:text-fg'
                    )}
                  >
                    <div className="flex items-center justify-between gap-3">
                      <div className="min-w-0">
                        <div className="truncate text-sm font-medium">{item.label}</div>
                        <div className="mt-1 truncate font-mono text-2xs text-fg-muted">{item.path}</div>
                      </div>
                      <Badge tone="neutral">{formatNumber(item.userCount)}</Badge>
                    </div>
                    <div className="mt-2 flex flex-wrap gap-2 text-[11px] text-fg-muted">
                      <span>{text(locale, 'Users', '用户')} {formatNumber(item.userCount)}</span>
                      <span>{text(locale, 'Rows', '行')} {formatNumber(item.rowCount)}</span>
                      <span>{text(locale, 'High Risk', '高风险')} {formatNumber(item.highRiskCount)}</span>
                    </div>
                  </button>
                );
              })
            )}
          </div>
        </aside>

        <div className="overflow-hidden rounded-lg border border-line bg-surface-base shadow-token-sm">
          <div className="border-b border-line px-4 py-4">
            <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
              <div>
                <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Active Permission Report', '当前权限报告')}</div>
                <div className="mt-2 text-xl font-semibold text-fg">
                  {selectedPathSummary ? selectedPathSummary.label : text(locale, 'Awaiting result path', '等待结果路径')}
                </div>
                {selectedPathTokens.length > 0 && (
                  <div className="mt-3 flex flex-wrap gap-2">
                    {selectedPathTokens.map((token, index) => (
                      <span key={`${token}-${index}`} className="rounded-full border border-line bg-surface-sunken px-3 py-1 text-xs text-fg-secondary">
                        {token}
                      </span>
                    ))}
                  </div>
                )}
                <p className="mt-3 text-sm text-fg-muted">
                  {selectedPathSummary
                    ? text(
                        locale,
                        `Found ${selectedPathSummary.userCount} principals across ${selectedPathSummary.rowCount} permission records on the selected path.`,
                        `当前路径共有 ${selectedPathSummary.userCount} 个主体、${selectedPathSummary.rowCount} 条权限记录。`
                      )
                    : text(locale, 'No path is selected yet.', '当前还没有选中任何结果路径。')}
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <span className="inline-flex items-center gap-2 rounded-full bg-accent px-3 py-1.5 text-sm text-fg-on-accent shadow-token-sm">
                  <UsersRound className="h-4 w-4" />
                  {text(locale, 'User Permission View', '用户权限视图')}
                </span>
              </div>
            </div>

            {selectedPathSummary && (
              <div className="mt-4 grid gap-3 md:grid-cols-4">
                <div className="rounded-lg border border-line bg-surface-sunken px-3 py-3">
                  <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Users', '用户')}</div>
                  <div className="mt-2 text-xl font-semibold tabular-nums text-fg">{formatNumber(selectedPathSummary.userCount)}</div>
                </div>
                <div className="rounded-lg border border-line bg-surface-sunken px-3 py-3">
                  <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Permission Records', '权限记录')}</div>
                  <div className="mt-2 text-xl font-semibold tabular-nums text-fg">{formatNumber(selectedPathSummary.rowCount)}</div>
                </div>
                <div className="rounded-lg border border-line bg-surface-sunken px-3 py-3">
                  <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Explicit', '显式')}</div>
                  <div className="mt-2 text-xl font-semibold tabular-nums text-fg">{formatNumber(selectedPathSummary.explicitCount)}</div>
                </div>
                <div className="rounded-lg border border-line bg-surface-sunken px-3 py-3">
                  <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'High Risk', '高风险')}</div>
                  <div className="mt-2 text-xl font-semibold tabular-nums text-fg">{formatNumber(selectedPathSummary.highRiskCount)}</div>
                </div>
              </div>
            )}

            {selectedResultPath && (
              <div
                className={cn(
                  'mt-4 flex items-start gap-2 rounded-lg border px-3 py-2.5 text-sm',
                  adReady ? 'border-info/40 bg-info-soft text-info' : 'border-warning/40 bg-warning-soft text-warning'
                )}
              >
                {adReady
                  ? text(locale, 'This report is using the verified domain connection, so effective group members can be unfolded into user rows.', '当前报告使用了已验证的域连接，组成员会尽量展开成用户权限结果。')
                  : text(locale, 'This report is still in SID fallback mode. UNC paths can be scanned, but unresolved identities may remain at the SID level.', '当前报告仍处于 SID 回退模式。UNC 路径可以扫描，但未解析成功的主体会保留为 SID 视角。')}
              </div>
            )}
          </div>

          <div className="px-4 py-4">
            <TableContainer>
              <Table className={cn('min-w-[1340px]', compactTables && '[&_td]:py-1')}>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead>{text(locale, 'Account Name', '账户名')}</TableHead>
                    <TableHead>{text(locale, 'First Name', '名字')}</TableHead>
                    <TableHead>{text(locale, 'Last Name', '姓氏')}</TableHead>
                    <TableHead>{text(locale, 'E-Mail', '邮箱')}</TableHead>
                    <TableHead>{text(locale, 'Department', '部门')}</TableHead>
                    <TableHead>{text(locale, 'Division', '事业部')}</TableHead>
                    <TableHead>{text(locale, 'Domain', '域')}</TableHead>
                    <TableHead>{text(locale, 'Originating Group', '来源组')}</TableHead>
                    <TableHead>{text(locale, 'Permissions', '权限')}</TableHead>
                    <TableHead>{text(locale, 'Group Inheritance', '组继承链')}</TableHead>
                    <TableHead>{text(locale, 'Inspect', '查看')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {userReportRows.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={11} className="px-3 py-8 text-center text-sm text-fg-muted">{text(locale, 'No matching report rows on the selected path.', '当前路径下没有匹配的用户报告结果。')}</TableCell>
                    </TableRow>
                  ) : (
                    userReportRows.map((item) => {
                      const active = selectedUserReportRow?.id === item.id;
                      return (
                        <TableRow
                          key={item.id}
                          className={cn('cursor-pointer', active && 'bg-accent-soft hover:bg-accent-soft')}
                          onClick={() => onSelectPermissionKey(item.member_keys[0] || '')}
                        >
                          <TableCell className="py-2 align-top">
                            <div className="font-medium text-fg">{item.account_name || item.trustee}</div>
                            <div className="mt-1 break-all text-2xs text-fg-muted">{item.trustee}</div>
                          </TableCell>
                          <TableCell className="py-2 align-top">{item.first_name || '-'}</TableCell>
                          <TableCell className="py-2 align-top">{item.last_name || '-'}</TableCell>
                          <TableCell className="py-2 align-top">{item.email || '-'}</TableCell>
                          <TableCell className="py-2 align-top">{item.department || '-'}</TableCell>
                          <TableCell className="py-2 align-top">{item.division || '-'}</TableCell>
                          <TableCell className="py-2 align-top">{item.domain || '-'}</TableCell>
                          <TableCell className="max-w-[200px] break-words py-2 align-top text-fg-muted">{item.originating_group || '-'}</TableCell>
                          <TableCell className="max-w-[260px] break-words py-2 align-top">
                            <div className="text-fg">{item.permissions || '-'}</div>
                            <div className="mt-1 flex flex-wrap items-center gap-2">
                              <Badge tone={riskTone(item.risk_level)} className={riskBadgeClass(item.risk_level)}>
                                {item.risk_level.toUpperCase()}
                              </Badge>
                              <span className="text-[11px] text-fg-muted">
                                {text(locale, `${item.permission_count} permission types`, `${item.permission_count} 种权限`)}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell className="max-w-[220px] break-words py-2 align-top text-fg-muted">
                            {item.group_inheritance_hierarchy || item.inheritance_summary || '-'}
                          </TableCell>
                          <TableCell className="py-2 align-top">
                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              onClick={(event) => {
                                event.stopPropagation();
                                onSelectPermissionKey(item.member_keys[0] || '');
                                onOpenDetail();
                              }}
                            >
                              <FileText className="h-3.5 w-3.5" />
                              {text(locale, 'Inspect', '查看')}
                            </Button>
                          </TableCell>
                        </TableRow>
                      );
                    })
                  )}
                </TableBody>
              </Table>
            </TableContainer>

            <div className="mt-4 rounded-lg border border-line bg-surface-sunken p-4">
              <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Focused Record', '当前聚焦记录')}</div>
              {selectedUserReportRow ? (
                <>
                  <div className="mt-3 flex flex-wrap items-start justify-between gap-4">
                    <div>
                      <div className="text-lg font-semibold text-fg">{selectedUserReportRow.account_name || selectedUserReportRow.trustee}</div>
                      <div className="mt-1 break-all text-sm text-fg-muted">{selectedUserReportRow.trustee}</div>
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <Badge tone={riskTone(selectedUserReportRow.risk_level)} className={riskBadgeClass(selectedUserReportRow.risk_level)}>
                        {selectedUserReportRow.risk_level.toUpperCase()}
                      </Badge>
                      <Badge tone="neutral">{text(locale, 'Rows', '行')}: {formatNumber(selectedUserReportRow.row_count)}</Badge>
                      <Badge tone="neutral">{text(locale, 'Inheritance', '继承')}: {selectedUserReportRow.inheritance_summary}</Badge>
                    </div>
                  </div>
                  <div className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                    <div className="rounded-lg border border-line bg-surface-base px-3 py-3">
                      <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Permissions', '权限')}</div>
                      <div className="mt-1 text-sm text-fg">{selectedUserReportRow.permissions || '-'}</div>
                    </div>
                    <div className="rounded-lg border border-line bg-surface-base px-3 py-3">
                      <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Originating Group', '来源组')}</div>
                      <div className="mt-1 text-sm text-fg">{selectedUserReportRow.originating_group || '-'}</div>
                    </div>
                    <div className="rounded-lg border border-line bg-surface-base px-3 py-3">
                      <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Identity Context', '身份上下文')}</div>
                      <div className="mt-1 text-sm text-fg">{[selectedUserReportRow.department, selectedUserReportRow.division, selectedUserReportRow.domain].filter(Boolean).join(' / ') || selectedUserReportRow.email || '-'}</div>
                    </div>
                    <div className="rounded-lg border border-line bg-surface-base px-3 py-3">
                      <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Path', '路径')}</div>
                      <div className="mt-1 break-all font-mono text-2xs text-fg">{selectedUserReportRow.path}</div>
                    </div>
                  </div>
                </>
              ) : (
                <p className="mt-3 text-sm text-fg-muted">{text(locale, 'Pick a path and a row to inspect its context.', '先选路径，再点一条结果查看上下文。')}</p>
              )}

              {selectedPermission && (
                <Button type="button" variant="secondary" className="mt-4" onClick={onOpenDetail}>
                  <FileText className="h-4 w-4" />
                  {text(locale, 'Open Full Inspector', '打开完整检查面板')}
                </Button>
              )}
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
