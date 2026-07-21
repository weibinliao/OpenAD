import { Eye, FolderOpen } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import type { IndexedPermissionRow } from '../lib/reportPayload';
import { cn } from '../lib/cn';
import PermissionDetail, { type PermissionDetailItem } from './PermissionDetail';
import { Badge, riskTone } from './ui/badge';
import { Button } from './ui/button';
import { EmptyState } from './ui/empty-state';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './ui/table';

interface ExactPathPermissionTableProps {
  locale: string;
  selectedPath: string;
  entries: readonly IndexedPermissionRow<PermissionDetailItem>[];
  totalScanRowCount: number;
  variant?: 'standalone' | 'embedded';
}

function text(locale: string, en: string, zh: string) {
  return locale === 'zh-CN' ? zh : en;
}

function formatNumber(value: number) {
  return new Intl.NumberFormat().format(value);
}

function displayIdentity(permission: PermissionDetailItem) {
  return permission.account_name || permission.trustee || permission.trustee_sid || '-';
}

function fieldValue(value?: string | null) {
  return (value || '').trim() || '-';
}

function secondaryIdentity(permission: PermissionDetailItem) {
  const primary = displayIdentity(permission);
  if (permission.trustee && permission.trustee !== primary) {
    return permission.trustee;
  }
  if (permission.trustee_sid && permission.trustee_sid !== primary) {
    return permission.trustee_sid;
  }
  return permission.trustee_sid || '';
}

function accountDisplay(permission: PermissionDetailItem) {
  return (permission.account_name || '').trim() || displayIdentity(permission);
}

function inferRisk(permission: Pick<PermissionDetailItem, 'rights' | 'risk_level'>) {
  const provided = (permission.risk_level || '').trim().toLowerCase();
  if (provided === 'high' || provided === 'medium' || provided === 'low') {
    return provided;
  }

  const rights = permission.rights.trim().toLowerCase();
  if (!rights) {
    return 'unknown';
  }
  if (['full control', 'write', 'delete', 'take ownership', 'change permissions', 'modify'].some((token) => rights.includes(token))) {
    return 'high';
  }
  if (rights.includes('execute')) {
    return 'medium';
  }
  return 'low';
}

function inheritanceLabel(locale: string, inherited: boolean) {
  return inherited ? text(locale, 'Inherited', '继承') : text(locale, 'Explicit', '显式');
}

export default function ExactPathPermissionTable({
  locale,
  selectedPath,
  entries,
  totalScanRowCount,
  variant = 'standalone',
}: ExactPathPermissionTableProps) {
  const [selectedRowKey, setSelectedRowKey] = useState('');
  const embedded = variant === 'embedded';

  useEffect(() => {
    if (!selectedRowKey) {
      return;
    }

    if (!entries.some((entry) => entry.rowKey === selectedRowKey)) {
      setSelectedRowKey('');
    }
  }, [entries, selectedRowKey]);

  const selectedPermission = useMemo(
    () => entries.find((entry) => entry.rowKey === selectedRowKey)?.item || null,
    [entries, selectedRowKey]
  );
  const highRiskCount = useMemo(
    () => entries.filter((entry) => inferRisk(entry.item) === 'high').length,
    [entries]
  );
  const exactPathRecordCount = entries.length;
  const scopePath = selectedPath || text(locale, 'Awaiting selected folder path', '等待所选文件夹路径');
  const emptyStateTitle = selectedPath
    ? text(locale, 'No exact-path evidence rows', '当前没有精确路径证据行')
    : text(locale, 'Select a folder to inspect', '请选择一个文件夹进行检查');

  let emptyStateMessage = text(
    locale,
    'Select a folder and run a scan to view exact-path ACL rows.',
    '请选择文件夹并执行扫描，以查看精确路径 ACL 行。'
  );
  if (selectedPath && totalScanRowCount > 0) {
    emptyStateMessage = text(
      locale,
      'No raw ACL rows matched the exact selected folder. Child paths stay out of this table.',
      '没有原始 ACL 行与当前精确选中的文件夹匹配。这个表不会带入子路径结果。'
    );
  } else if (selectedPath) {
    emptyStateMessage = text(
      locale,
      'Run a scan to load raw ACL rows for the selected folder.',
      '请先执行扫描，再加载所选文件夹的原始 ACL 行。'
    );
  }

  return (
    <>
      <article className={cn('overflow-hidden rounded-lg border border-line bg-surface-base', !embedded && 'shadow-token-sm')}>
        <header className="px-4 py-4 sm:px-5">
          <div className="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
            <div className="min-w-0 flex-1">
              <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{embedded ? text(locale, 'Evidence grid', '证据表格') : text(locale, 'Primary Evidence Surface', '主证据视图')}</div>
              <div className="mt-1 flex flex-wrap items-center gap-2 text-sm font-semibold text-fg">
                <span>{text(locale, 'Exact-path ACL evidence', '精确路径 ACL 证据')}</span>
                <Badge tone="neutral">{text(locale, 'Selected folder only', '仅限当前选中文件夹')}</Badge>
                <Badge tone="neutral">{text(locale, 'One raw ACL row per record', '每条原始 ACL 各占一行')}</Badge>
              </div>
              <div className="mt-1.5 flex min-w-0 items-center gap-1.5 text-xs text-fg-muted">
                <FolderOpen className="h-3.5 w-3.5 shrink-0" />
                <span className="truncate font-mono text-2xs">{scopePath}</span>
                <span className="hidden text-fg-faint lg:inline">{text(locale, 'Child paths excluded', '排除子路径')}</span>
              </div>
            </div>

            <div className="grid shrink-0 grid-cols-3 gap-2">
              <div className="rounded-md border border-line bg-surface-sunken px-3 py-2">
                <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Exact Rows', '精确行')}</div>
                <div className="mt-1 text-lg font-semibold text-fg">{formatNumber(exactPathRecordCount)}</div>
              </div>
              <div className="rounded-md border border-line bg-surface-sunken px-3 py-2">
                <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'High Risk', '高风险')}</div>
                <div className="mt-1 text-lg font-semibold text-fg">{formatNumber(highRiskCount)}</div>
              </div>
              <div className="rounded-md border border-line bg-surface-sunken px-3 py-2">
                <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Scan Rows', '扫描行')}</div>
                <div className="mt-1 text-lg font-semibold text-fg">{formatNumber(totalScanRowCount)}</div>
              </div>
            </div>
          </div>
        </header>

        <div className="flex flex-wrap items-center justify-between gap-2 border-t border-line bg-surface-raised px-4 py-2 sm:px-5">
          <div className="flex flex-wrap items-center gap-2">
            <Badge tone="neutral">{text(locale, 'Primary evidence grid', '主证据表格')}</Badge>
            <Badge tone="neutral">{text(locale, 'Child paths excluded', '排除子路径')}</Badge>
            <Badge tone="neutral">{text(locale, 'Row inspector available', '可打开行检查面板')}</Badge>
          </div>
          <div className="text-2xs text-fg-muted">
            {text(locale, 'Use Inspect to review one raw ACL row without leaving the primary evidence grid.', '使用“查看”可单独审阅一条原始 ACL，而不离开主证据表格。')}
          </div>
        </div>

        <div className="max-h-[72vh] overflow-auto border-t border-line">
          <Table className="min-w-[1500px]">
            <caption className="sr-only">
              {text(
                locale,
                'Exact-path permission table with one raw ACL entry per row for the selected folder.',
                '精确路径权限表：每一行都是所选文件夹上的一条原始 ACL 记录。'
              )}
            </caption>
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
                <TableHead>{text(locale, 'Inspect', '查看')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {entries.length === 0 ? (
                <TableRow className="hover:bg-transparent">
                  <TableCell colSpan={10} className="px-4 py-8 sm:px-5 sm:py-10">
                    <EmptyState
                      icon={FolderOpen}
                      title={emptyStateTitle}
                      description={emptyStateMessage}
                      className="mx-auto max-w-2xl py-6"
                    />
                  </TableCell>
                </TableRow>
              ) : (
                entries.map((entry) => {
                  const permission = entry.item;
                  const selected = entry.rowKey === selectedRowKey;
                  const risk = inferRisk(permission);
                  const secondary = secondaryIdentity(permission);
                  const chainSummary = [permission.group_inheritance_hierarchy, permission.source]
                    .map((value) => (value || '').trim())
                    .filter((value, index, values) => Boolean(value) && values.indexOf(value) === index && value !== (permission.originating_group || '').trim())
                    .join(' · ');
                  const permissionMeta = [permission.parent_delta, permission.applies_to]
                    .map((value) => (value || '').trim())
                    .filter((value, index, values) => Boolean(value) && values.indexOf(value) === index)
                    .join(' · ');

                  return (
                    <TableRow
                      key={entry.rowKey}
                      data-state={selected ? 'selected' : undefined}
                      className="align-top"
                    >
                      <TableCell className="max-w-[220px] py-3 align-top">
                        <div className="font-medium text-fg">{accountDisplay(permission)}</div>
                        {secondary ? <div className="mt-0.5 break-all text-2xs leading-4 text-fg-muted">{secondary}</div> : null}
                        <div className="mt-0.5 font-mono text-2xs text-fg-muted">{fieldValue(permission.trustee_sid)}</div>
                      </TableCell>

                      <TableCell className="py-3 align-top text-fg">{fieldValue(permission.first_name)}</TableCell>

                      <TableCell className="py-3 align-top text-fg">{fieldValue(permission.last_name)}</TableCell>

                      <TableCell className="max-w-[220px] py-3 align-top">
                        <div className={cn('break-all leading-5', permission.email ? 'text-fg' : 'text-fg-muted')}>{fieldValue(permission.email)}</div>
                      </TableCell>

                      <TableCell className="py-3 align-top text-fg">{fieldValue(permission.department)}</TableCell>

                      <TableCell className="py-3 align-top text-fg">{fieldValue(permission.division)}</TableCell>

                      <TableCell className="py-3 align-top">
                        <div className="text-fg">{fieldValue(permission.domain)}</div>
                        {permission.account_type ? <div className="mt-0.5 text-2xs leading-4 text-fg-muted">{permission.account_type}</div> : null}
                      </TableCell>

                      <TableCell className="max-w-[240px] py-3 align-top">
                        <div className="break-words text-fg">{fieldValue(permission.originating_group)}</div>
                        {chainSummary ? <div className="mt-0.5 break-words text-2xs leading-4 text-fg-muted">{chainSummary}</div> : null}
                      </TableCell>

                      <TableCell className="max-w-[320px] py-3 align-top">
                        <div className="break-words leading-5 text-fg">{fieldValue(permission.rights)}</div>
                        <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
                          <Badge tone={riskTone(risk)} className="uppercase tracking-wide">
                            {risk}
                          </Badge>
                          <Badge tone="neutral">{fieldValue(permission.type)}</Badge>
                          <Badge tone="neutral">{inheritanceLabel(locale, permission.inherited)}</Badge>
                          {permission.applies_to ? <Badge tone="neutral">{permission.applies_to}</Badge> : null}
                        </div>
                        {permissionMeta ? <div className="mt-1 break-words text-2xs leading-4 text-fg-muted">{permissionMeta}</div> : null}
                      </TableCell>

                      <TableCell className="py-3 align-top">
                        <Button
                          type="button"
                          variant={selected ? 'secondary' : 'ghost'}
                          size="sm"
                          onClick={() => setSelectedRowKey(entry.rowKey)}
                          aria-pressed={selected}
                          aria-label={text(locale, `Inspect ACL row for ${displayIdentity(permission)}`, `查看 ${displayIdentity(permission)} 的 ACL 行`)}
                        >
                          <Eye className="h-3.5 w-3.5" />
                          {selected ? text(locale, 'Inspecting', '查看中') : text(locale, 'Inspect', '查看')}
                        </Button>
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </div>
      </article>

      <PermissionDetail permission={selectedPermission} isOpen={Boolean(selectedPermission)} onClose={() => setSelectedRowKey('')} />
    </>
  );
}
