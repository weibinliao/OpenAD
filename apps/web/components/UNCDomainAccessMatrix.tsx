import { Network, Search, UsersRound } from 'lucide-react';
import { useMemo, useState } from 'react';
import { Badge } from './ui/badge';
import { Button } from './ui/button';
import { EmptyState } from './ui/empty-state';
import { Input } from './ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './ui/table';

interface DomainAccessEntry {
  path: string;
  trustee: string;
  trustee_sid: string;
  rights: string;
  type: string;
  inherited: boolean;
  source?: string;
  applies_to?: string;
  account_type?: string;
  account_name?: string;
  email?: string;
  domain?: string;
  originating_group?: string;
  group_inheritance_hierarchy?: string;
}

interface UNCDomainAccessMatrixProps {
  locale: string;
  entries: DomainAccessEntry[];
  adReady: boolean;
  isUNC: boolean;
  onApplyTrusteeFilter: (value: string) => void;
  onApplyPathFilter: (value: string) => void;
}

interface MatrixRow {
  id: string;
  principal: string;
  domain: string;
  accountType: string;
  path: string;
  rights: string;
  type: string;
  inherited: boolean;
  appliesTo: string;
  accessSource: string;
  viaGroup: string;
  unresolved: boolean;
}

interface PrincipalSummary {
  principal: string;
  domain: string;
  accountType: string;
  pathCount: number;
  rowCount: number;
  viaGroupCount: number;
  explicitCount: number;
  rights: string[];
}

function text(locale: string, en: string, zh: string) {
  return locale === 'zh-CN' ? zh : en;
}

function formatNumber(value: number) {
  return new Intl.NumberFormat().format(value);
}

function normalize(value: string | undefined | null) {
  return (value || '').trim();
}

function isSID(value: string) {
  return value.toUpperCase().startsWith('S-1-');
}

function principalLabel(item: DomainAccessEntry) {
  return [item.account_name, item.trustee, item.trustee_sid]
    .map(normalize)
    .find(Boolean) || 'Unknown';
}

function deriveDomain(item: DomainAccessEntry, principal: string) {
  const explicit = normalize(item.domain);
  if (explicit) {
    return explicit.toUpperCase();
  }
  if (principal.includes('\\')) {
    return principal.split('\\')[0]?.trim().toUpperCase() || '';
  }
  const email = normalize(item.email);
  if (email.includes('@')) {
    return email.split('@')[1]?.trim().toUpperCase() || '';
  }
  return '';
}

function normalizeRights(rights: string) {
  return normalize(rights)
    .replaceAll('FullControl', 'Full Control')
    .replaceAll('ReadAndExecute', 'Read and Execute');
}

function sourceLabel(item: DomainAccessEntry, locale: string) {
  const viaGroup = normalize(item.originating_group) || normalize(item.group_inheritance_hierarchy);
  if (viaGroup) {
    return text(locale, 'Via AD group', '通过 AD 组');
  }
  if (item.inherited) {
    return text(locale, 'Inherited ACL', '继承 ACL');
  }
  return text(locale, 'Direct ACE', '直接 ACE');
}

function buildRows(entries: DomainAccessEntry[], locale: string): MatrixRow[] {
  return entries.map((item, index) => {
    const principal = principalLabel(item);
    const viaGroup = normalize(item.originating_group) || normalize(item.group_inheritance_hierarchy);
    return {
      id: `${item.path}|${principal}|${item.rights}|${item.type}|${index}`,
      principal,
      domain: deriveDomain(item, principal) || '-',
      accountType: normalize(item.account_type) || '-',
      path: normalize(item.path),
      rights: normalizeRights(item.rights) || '-',
      type: normalize(item.type) || '-',
      inherited: Boolean(item.inherited),
      appliesTo: normalize(item.applies_to) || '-',
      accessSource: sourceLabel(item, locale),
      viaGroup: viaGroup || '-',
      unresolved: isSID(principal) || isSID(normalize(item.trustee_sid)),
    };
  });
}

function buildPrincipalSummaries(rows: MatrixRow[]) {
  const grouped = new Map<string, MatrixRow[]>();
  for (const row of rows) {
    const key = `${row.domain}|${row.principal}`.toLowerCase();
    const current = grouped.get(key) || [];
    current.push(row);
    grouped.set(key, current);
  }

  return Array.from(grouped.values()).map<PrincipalSummary>((items) => ({
    principal: items[0].principal,
    domain: items[0].domain,
    accountType: items[0].accountType,
    pathCount: new Set(items.map((item) => item.path)).size,
    rowCount: items.length,
    viaGroupCount: items.filter((item) => item.viaGroup !== '-').length,
    explicitCount: items.filter((item) => !item.inherited).length,
    rights: Array.from(new Set(items.map((item) => item.rights).filter(Boolean))).slice(0, 3),
  })).sort((a, b) => b.pathCount - a.pathCount || b.rowCount - a.rowCount || a.principal.localeCompare(b.principal, undefined, { sensitivity: 'base' }));
}

export default function UNCDomainAccessMatrix({
  locale,
  entries,
  adReady,
  isUNC,
  onApplyTrusteeFilter,
  onApplyPathFilter,
}: UNCDomainAccessMatrixProps) {
  const [query, setQuery] = useState('');
  const rows = useMemo(() => buildRows(entries, locale), [entries, locale]);
  const visibleRows = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) {
      return rows;
    }
    return rows.filter((row) => [row.principal, row.domain, row.path, row.rights, row.viaGroup, row.accountType]
      .some((value) => value.toLowerCase().includes(needle)));
  }, [query, rows]);
  const summaries = useMemo(() => buildPrincipalSummaries(visibleRows).slice(0, 8), [visibleRows]);
  const domainPrincipalCount = useMemo(() => new Set(rows.map((row) => `${row.domain}|${row.principal}`.toLowerCase())).size, [rows]);
  const pathCount = useMemo(() => new Set(rows.map((row) => row.path.toLowerCase())).size, [rows]);
  const viaGroupCount = rows.filter((row) => row.viaGroup !== '-').length;
  const unresolvedCount = rows.filter((row) => row.unresolved).length;
  const tableRows = visibleRows.slice(0, 120);

  return (
    <section className="rounded-lg border border-line bg-surface-base shadow-token-sm" aria-labelledby="unc-domain-matrix-title">
      <div className="flex flex-col gap-3 px-4 pt-4 sm:px-5 md:flex-row md:items-start md:justify-between">
        <div className="min-w-0">
          <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'UNC access evidence', 'UNC 访问证据')}</div>
          <h3 id="unc-domain-matrix-title" className="mt-1 text-sm font-semibold text-fg">
            {text(locale, 'Who has which permissions?', '谁拥有哪些权限？')}
          </h3>
          <p className="mt-1 text-xs text-fg-muted">
            {text(
              locale,
              'A domain-focused matrix for UNC reviews: principal, path, rights, inheritance, and the AD group that granted access.',
              '面向 UNC 复核的域权限矩阵：主体、路径、权限、继承状态，以及授予访问的 AD 组。'
            )}
          </p>
        </div>
        <div className="flex shrink-0 flex-wrap items-center gap-2">
          <Badge tone={isUNC ? 'success' : 'info'}>{isUNC ? 'UNC' : text(locale, 'Local path', '本地路径')}</Badge>
          <Badge tone={adReady ? 'success' : 'warning'}>{adReady ? text(locale, 'AD context active', 'AD 上下文已启用') : text(locale, 'AD context limited', 'AD 上下文受限')}</Badge>
        </div>
      </div>

      <div className="mt-3 grid grid-cols-2 gap-2 px-4 sm:px-5 lg:grid-cols-4">
        <div className="rounded-md border border-line bg-surface-sunken px-3 py-2">
          <span className="block text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Domain principals', '域主体')}</span>
          <strong className="mt-0.5 block text-lg font-semibold text-fg">{formatNumber(domainPrincipalCount)}</strong>
        </div>
        <div className="rounded-md border border-line bg-surface-sunken px-3 py-2">
          <span className="block text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'UNC paths', 'UNC 路径')}</span>
          <strong className="mt-0.5 block text-lg font-semibold text-fg">{formatNumber(pathCount)}</strong>
        </div>
        <div className="rounded-md border border-line bg-surface-sunken px-3 py-2">
          <span className="block text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Via AD group', '通过 AD 组')}</span>
          <strong className="mt-0.5 block text-lg font-semibold text-fg">{formatNumber(viaGroupCount)}</strong>
        </div>
        <div className="rounded-md border border-line bg-surface-sunken px-3 py-2">
          <span className="block text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Unresolved SID', '未解析 SID')}</span>
          <strong className="mt-0.5 block text-lg font-semibold text-fg">{formatNumber(unresolvedCount)}</strong>
        </div>
      </div>

      <div className="mt-3 px-4 sm:px-5">
        <label className="relative block max-w-md">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-fg-faint" />
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={text(locale, 'Search user, group, path, rights, or source group', '搜索用户、组、路径、权限或来源组')}
            className="pl-9"
          />
        </label>
      </div>

      {rows.length === 0 ? (
        <EmptyState
          icon={Network}
          title={text(locale, 'Run or import a UNC scan to populate the domain access matrix.', '执行或导入 UNC 扫描后，这里会生成域访问矩阵。')}
          className="pb-6"
        />
      ) : (
        <div className="grid gap-3 px-4 pb-4 pt-3 sm:px-5 lg:grid-cols-[280px_minmax(0,1fr)]">
          <aside className="flex min-w-0 flex-col gap-2">
            <div className="text-2xs font-medium uppercase tracking-wide text-fg-muted">{text(locale, 'Access holders', '访问主体')}</div>
            {summaries.length === 0 ? (
              <div className="rounded-md border border-dashed border-line px-3 py-4 text-xs text-fg-muted">{text(locale, 'No matching principal.', '没有匹配主体。')}</div>
            ) : summaries.map((summary) => (
              <button
                key={`${summary.domain}-${summary.principal}`}
                type="button"
                onClick={() => onApplyTrusteeFilter(summary.principal)}
                className="flex items-start gap-2.5 rounded-md border border-line bg-surface-base px-3 py-2.5 text-left transition-colors hover:border-accent/40 hover:bg-surface-sunken"
              >
                <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-accent-soft text-accent-fg"><UsersRound className="h-4 w-4" /></span>
                <span className="min-w-0">
                  <strong className="block truncate text-xs font-semibold text-fg">{summary.principal}</strong>
                  <small className="block text-2xs text-fg-muted">{summary.domain} · {summary.accountType}</small>
                  <em className="block text-2xs not-italic text-fg-faint">{formatNumber(summary.pathCount)} {text(locale, 'paths', '路径')} · {formatNumber(summary.rowCount)} {text(locale, 'rows', '行')} · {formatNumber(summary.viaGroupCount)} {text(locale, 'via group', '经组')}</em>
                </span>
              </button>
            ))}
          </aside>

          <div className="min-w-0 overflow-hidden rounded-lg border border-line bg-surface-base">
            <div className="max-h-[480px] overflow-auto">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead>{text(locale, 'Principal', '主体')}</TableHead>
                    <TableHead>{text(locale, 'Domain', '域')}</TableHead>
                    <TableHead>{text(locale, 'UNC Path', 'UNC 路径')}</TableHead>
                    <TableHead>{text(locale, 'Rights', '权限')}</TableHead>
                    <TableHead>{text(locale, 'Access Source', '访问来源')}</TableHead>
                    <TableHead>{text(locale, 'Inheritance', '继承')}</TableHead>
                    <TableHead>{text(locale, 'Action', '动作')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {tableRows.map((row) => (
                    <TableRow key={row.id} className="align-top">
                      <TableCell className="py-2 align-top">
                        <span className="block text-xs font-medium text-fg">{row.principal}</span>
                        <small className="block text-2xs text-fg-muted">{row.accountType}</small>
                      </TableCell>
                      <TableCell className="py-2 align-top">{row.domain}</TableCell>
                      <TableCell className="py-2 align-top"><span className="break-all font-mono text-2xs">{row.path}</span></TableCell>
                      <TableCell className="py-2 align-top">{row.type} · {row.rights}</TableCell>
                      <TableCell className="py-2 align-top">
                        <Badge tone="neutral">{row.accessSource}</Badge>
                        <div className="mt-0.5 break-all text-2xs text-fg-muted">{row.viaGroup}</div>
                      </TableCell>
                      <TableCell className="py-2 align-top">{row.inherited ? text(locale, 'Inherited', '继承') : text(locale, 'Explicit', '显式')} · {row.appliesTo}</TableCell>
                      <TableCell className="py-2 align-top">
                        <div className="flex items-center gap-1.5">
                          <Button type="button" variant="outline" size="sm" onClick={() => onApplyTrusteeFilter(row.principal)}>{text(locale, 'User', '主体')}</Button>
                          <Button type="button" variant="outline" size="sm" onClick={() => onApplyPathFilter(row.path)}>{text(locale, 'Path', '路径')}</Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
            {visibleRows.length > tableRows.length ? (
              <div className="border-t border-line bg-surface-raised px-3 py-2 text-2xs text-fg-muted">
                {text(locale, 'Showing first 120 matching rows. Narrow the search to inspect a specific user, group, path, or right.', '当前显示前 120 条匹配行。可缩小搜索范围查看特定用户、组、路径或权限。')}
              </div>
            ) : null}
          </div>
        </div>
      )}
    </section>
  );
}
