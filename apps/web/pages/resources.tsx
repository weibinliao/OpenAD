import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import Link from 'next/link';
import {
  FolderSearch,
  FolderTree,
  GitFork,
  Loader2,
  RefreshCcw,
  ScanSearch,
  Square,
  Users,
  Wrench,
  AlertTriangle,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react';
import { apiBase, websocketBase } from '../lib/runtimeApi';
import { useDict } from '../lib/i18n';
import { useI18n } from '../contexts/I18nContext';
import { useADConnection } from '../contexts/ADConnectionContext';
import { cn } from '../lib/cn';
import { Card, CardContent, CardHeader } from '../components/ui/card';
import { Badge, riskLabel, riskTone } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Skeleton } from '../components/ui/skeleton';
import { EmptyState } from '../components/ui/empty-state';
import PermissionDetail, { type PermissionDetailItem } from '../components/PermissionDetail';
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
  items_scanned: number;
  permission_count: number;
  started_at: string;
  finished_at?: string;
}

interface ProgressEvent {
  items_scanned: number;
  permission_count: number;
  current_path?: string;
  status: string;
  error?: string;
}

type ScanPhase = 'idle' | 'running' | 'done' | 'failed';

function interpolate(template: string, params: Record<string, string | number>) {
  return template.replace(/\{(\w+)\}/g, (_, key) => String(params[key] ?? `{${key}}`));
}

function formatWhen(value: string | undefined, locale: string) {
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

export default function ResourcesPage() {
  const d = useDict();
  const { locale } = useI18n();
  const { activeProfile } = useADConnection();

  // ------------------------------------------------------------- command bar
  const [path, setPath] = useState('');
  const [depth, setDepth] = useState(-1);
  const [resolveAd, setResolveAd] = useState(true);
  const [phase, setPhase] = useState<ScanPhase>('idle');
  const [progress, setProgress] = useState<ProgressEvent | null>(null);
  const [scanError, setScanError] = useState('');
  const scanIdRef = useRef('');
  const socketRef = useRef<WebSocket | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);

  // ---------------------------------------------------------------- sessions
  const [sessions, setSessions] = useState<ScanSession[]>([]);
  const [sessionsLoading, setSessionsLoading] = useState(true);
  const [sessionsError, setSessionsError] = useState('');
  const [selected, setSelected] = useState<ScanSession | null>(null);

  const loadSessions = useCallback(async () => {
    setSessionsLoading(true);
    setSessionsError('');
    try {
      const response = await fetch(`${apiBase()}/api/sessions?status=completed&page=1&page_size=100`);
      const data = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(data.error || 'unavailable');
      setSessions(Array.isArray(data.items) ? data.items : []);
    } catch (requestError) {
      setSessions([]);
      setSessionsError(requestError instanceof Error ? requestError.message : d.resources.listUnavailable);
    } finally {
      setSessionsLoading(false);
    }
  }, [d.resources.listUnavailable]);

  useEffect(() => {
    loadSessions();
    return () => socketRef.current?.close();
  }, [loadSessions]);

  // Latest completed session per distinct root path.
  const roots = useMemo(() => {
    const byRoot = new Map<string, ScanSession>();
    for (const session of sessions) {
      const key = session.root_path.toLowerCase();
      if (!byRoot.has(key)) byRoot.set(key, session);
    }
    return Array.from(byRoot.values());
  }, [sessions]);

  const startScan = useCallback(
    async (targetPath?: string) => {
      const scanPath = (targetPath ?? path).trim();
      if (!scanPath || phase === 'running') return;
      if (targetPath) setPath(targetPath);

      const scanId = `scan-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
      scanIdRef.current = scanId;
      setPhase('running');
      setScanError('');
      setProgress({ items_scanned: 0, permission_count: 0, status: 'running' });

      // Live progress over WebSocket; the POST below blocks until completion.
      try {
        const socket = new WebSocket(`${websocketBase()}/api/scan/ws?scan_id=${encodeURIComponent(scanId)}`);
        socket.onmessage = (message) => {
          try {
            const event = JSON.parse(message.data) as ProgressEvent;
            setProgress(event);
          } catch {
            /* ignore malformed frames */
          }
        };
        socketRef.current = socket;
      } catch {
        /* progress is best-effort; the scan itself still runs */
      }

      try {
        const body: Record<string, unknown> = {
          path: scanPath,
          depth,
          include_inherited: true,
          scan_id: scanId,
        };
        if (resolveAd && activeProfile) {
          body.effective_permissions = { enabled: true, connection_id: activeProfile.id };
        }
        const response = await fetch(`${apiBase()}/api/scan`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
        });
        const data = await response.json().catch(() => ({}));
        if (!response.ok) throw new Error(data.error || d.resources.scanFailed);
        if (data.status === 'cancelled') {
          setPhase('idle');
        } else {
          setPhase('done');
        }
        await loadSessions();
      } catch (err) {
        setPhase('failed');
        setScanError(err instanceof Error ? err.message : d.resources.scanFailed);
      } finally {
        socketRef.current?.close();
        socketRef.current = null;
      }
    },
    [activeProfile, d, depth, loadSessions, path, phase, resolveAd],
  );

  const cancelScan = useCallback(async () => {
    if (!scanIdRef.current) return;
    await fetch(`${apiBase()}/api/scan/${encodeURIComponent(scanIdRef.current)}/cancel`, { method: 'POST' }).catch(
      () => undefined,
    );
  }, []);

  const depthOptions = [
    { value: 1, label: d.resources.depthRootOnly },
    { value: 2, label: d.resources.depthTwoLevels },
    { value: -1, label: d.resources.depthFull },
  ];

  return (
    <div className="app-v2 mx-auto max-w-6xl space-y-5">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 className="text-lg font-semibold text-fg">{d.resources.title}</h1>
          <p className="mt-0.5 text-sm text-fg-muted">{d.resources.subtitle}</p>
        </div>
        <div className="flex flex-wrap items-center gap-2 sm:shrink-0">
          <Link href="/history" className="text-xs font-medium text-accent-fg hover:underline">
            {d.resources.history}
          </Link>
          <span className="text-fg-faint">·</span>
          <Link href="/scan-workspace" className="inline-flex items-center gap-1 text-xs font-medium text-accent-fg hover:underline">
            <Wrench className="h-3 w-3" /> {d.resources.advancedWorkbench}
          </Link>
        </div>
      </div>

      {/* Command bar — the heart of the page */}
      <Card className="overflow-hidden">
        <CardContent className="space-y-3 pt-4">
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
            <div className="relative w-full min-w-0 flex-1">
              <FolderSearch className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-fg-faint" />
              <Input
                ref={inputRef}
                value={path}
                onChange={(event) => setPath(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') startScan();
                }}
                placeholder={d.resources.commandPlaceholder}
                className="h-11 pl-9 font-mono text-sm"
                disabled={phase === 'running'}
                aria-label={d.resources.commandPlaceholder}
              />
            </div>
            {phase === 'running' ? (
              <Button variant="danger" size="lg" onClick={cancelScan} className="w-full sm:w-auto">
                <Square className="h-3.5 w-3.5" /> {d.resources.cancelScan}
              </Button>
            ) : (
              <Button size="lg" onClick={() => startScan()} disabled={!path.trim()} className="w-full sm:w-auto">
                <ScanSearch className="h-4 w-4" /> {d.resources.scanNow}
              </Button>
            )}
          </div>

          {/* Options row */}
          <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-xs">
            <span className="flex items-center gap-1.5">
              <span className="text-fg-muted">{d.resources.depthLabel}:</span>
              <span className="flex items-center gap-1 rounded-lg bg-surface-sunken p-0.5">
                {depthOptions.map((option) => (
                  <button
                    key={option.value}
                    type="button"
                    onClick={() => setDepth(option.value)}
                    disabled={phase === 'running'}
                    className={cn(
                      'rounded-md px-2.5 py-1 font-medium transition-colors',
                      depth === option.value ? 'bg-surface-base text-fg shadow-token-sm' : 'text-fg-muted hover:text-fg',
                    )}
                  >
                    {option.label}
                  </button>
                ))}
              </span>
            </span>
            <label
              className={cn('flex cursor-pointer items-center gap-1.5', !activeProfile && 'cursor-not-allowed opacity-60')}
              title={activeProfile ? d.resources.resolveAdHint : d.resources.resolveAdNeedsConnection}
            >
              <input
                type="checkbox"
                checked={resolveAd && Boolean(activeProfile)}
                onChange={(event) => setResolveAd(event.target.checked)}
                disabled={!activeProfile || phase === 'running'}
                className="h-3.5 w-3.5"
              />
              <Users className="h-3.5 w-3.5 text-fg-muted" />
              <span className="text-fg-secondary">{d.resources.resolveAd}</span>
            </label>
            {!activeProfile ? <span className="text-warning">{d.resources.connectAdHint}</span> : null}
          </div>

          {/* Progress / result strip */}
          {phase === 'running' && progress ? (
            <div className="rounded-md border border-info/30 bg-info-soft px-3 py-2.5">
              <div className="flex items-center gap-2 text-xs text-info">
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                <span className="font-medium">{d.resources.scanning}</span>
                <span className="tabular-nums">
                  {progress.items_scanned.toLocaleString()} {d.resources.progressItems}
                </span>
                <span className="tabular-nums">
                  {progress.permission_count.toLocaleString()} {d.resources.progressPermissions}
                </span>
              </div>
              {progress.current_path ? (
                <p className="mt-1 truncate font-mono text-2xs text-info/80" title={progress.current_path}>
                  {progress.current_path}
                </p>
              ) : null}
              <div className="mt-2 h-1 overflow-hidden rounded-full bg-info/20">
                <div className="h-full w-1/3 animate-pulse rounded-full bg-info" />
              </div>
            </div>
          ) : phase === 'done' ? (
            <p className="flex items-center gap-1.5 rounded-md border border-success/40 bg-success-soft px-3 py-2 text-xs text-success">
              <CheckCircle2 className="h-3.5 w-3.5" /> {d.resources.scanDone}
              {progress ? (
                <span className="tabular-nums">
                  · {progress.items_scanned.toLocaleString()} {d.resources.progressItems} ·{' '}
                  {progress.permission_count.toLocaleString()} {d.resources.progressPermissions}
                </span>
              ) : null}
            </p>
          ) : phase === 'failed' && scanError ? (
            <p className="flex items-start gap-1.5 rounded-md border border-danger/40 bg-danger-soft px-3 py-2 text-xs text-danger">
              <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
              <span className="min-w-0 break-all">{scanError}</span>
            </p>
          ) : null}
        </CardContent>
      </Card>

      {/* Scanned roots grid */}
      <div>
        <div className="mb-2 flex items-baseline justify-between">
          <div>
            <h2 className="text-sm font-semibold text-fg">{d.resources.myShares}</h2>
            <p className="text-xs text-fg-muted">{d.resources.mySharesDesc}</p>
          </div>
          <Button variant="ghost" size="sm" onClick={loadSessions}>
            <RefreshCcw className="h-3 w-3" /> {d.common.refresh}
          </Button>
        </div>
        {sessionsLoading ? (
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            <Skeleton className="h-32" />
            <Skeleton className="h-32" />
            <Skeleton className="h-32" />
          </div>
        ) : sessionsError ? (
          <Card>
            <EmptyState
              icon={AlertTriangle}
              title={d.resources.listUnavailable}
              description={`${d.resources.listUnavailableHint} ${sessionsError}`}
              action={
                <Button aria-label={d.resources.retryResources} size="sm" onClick={() => void loadSessions()}>
                  <RefreshCcw className="h-3.5 w-3.5" /> {d.common.retry}
                </Button>
              }
            />
          </Card>
        ) : roots.length === 0 ? (
          <Card>
            <EmptyState
              icon={FolderTree}
              title={d.resources.noShares}
              action={<Button size="sm" onClick={() => inputRef.current?.focus()}>{d.resources.newScan}</Button>}
            />
          </Card>
        ) : (
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {roots.map((session) => (
              <RootCard
                key={session.id}
                session={session}
                locale={locale}
                selected={selected?.id === session.id}
                onBrowse={() => setSelected(selected?.id === session.id ? null : session)}
                onRescan={() => startScan(session.root_path)}
              />
            ))}
          </div>
        )}
      </div>

      {/* Inline permission browser */}
      {selected ? <PermissionBrowser key={selected.id} session={selected} /> : null}
    </div>
  );
}

/* ------------------------------- Root card -------------------------------- */

function RootCard({
  session,
  locale,
  selected,
  onBrowse,
  onRescan,
}: {
  session: ScanSession;
  locale: string;
  selected: boolean;
  onBrowse: () => void;
  onRescan: () => void;
}) {
  const d = useDict();
  // A tiny visual signature: how permission-dense this tree is.
  const density = session.items_scanned > 0 ? Math.min(1, session.permission_count / (session.items_scanned * 4)) : 0;

  return (
    <Card className={cn('flex flex-col transition-colors', selected && 'border-accent ring-1 ring-accent/40')}>
      <CardContent className="flex flex-1 flex-col gap-2.5 pt-4">
        <div className="flex items-start gap-2">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-accent-soft text-accent-fg">
            <FolderTree className="h-4 w-4" />
          </div>
          <div className="min-w-0">
            <p className="break-all font-mono text-xs font-medium leading-4 text-fg" title={session.root_path}>
              {session.root_path}
            </p>
            <p className="mt-0.5 text-2xs text-fg-muted">
              {d.resources.lastScanned} {formatWhen(session.finished_at, locale)}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-3 text-2xs text-fg-secondary">
          <span className="tabular-nums">
            {session.items_scanned.toLocaleString()} {d.resources.itemsCount}
          </span>
          <span className="tabular-nums">
            {session.permission_count.toLocaleString()} {d.resources.permissionsCount}
          </span>
        </div>

        <div className="flex items-center gap-2" title={d.resources.density}>
          <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-surface-sunken">
            <div
              className={cn('h-full rounded-full', density > 0.66 ? 'bg-risk-high' : density > 0.33 ? 'bg-risk-medium' : 'bg-risk-low')}
              style={{ width: `${Math.max(6, Math.round(density * 100))}%` }}
            />
          </div>
        </div>

        <div className="mt-auto flex flex-wrap items-center gap-1.5 pt-1">
          <Button variant={selected ? 'primary' : 'outline'} size="sm" onClick={onBrowse} className="min-w-[8rem] flex-1">
            {d.resources.viewPermissions}
          </Button>
          <Link href={`/access?path=${encodeURIComponent(session.root_path)}`} className="min-w-[8rem] flex-1">
            <Button variant="outline" size="sm" className="w-full">
              <GitFork className="h-3 w-3" /> {d.resources.whoCanAccess}
            </Button>
          </Link>
          <Button variant="ghost" size="icon" onClick={onRescan} aria-label={d.resources.rescan} title={d.resources.rescan}>
            <RefreshCcw className="h-3.5 w-3.5" />
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

/* --------------------------- Permission browser --------------------------- */

interface PermissionRow extends PermissionDetailItem {
  id: string;
}

function PermissionBrowser({ session }: { session: ScanSession }) {
  const d = useDict();
  const { locale } = useI18n();
  const [rows, setRows] = useState<PermissionRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [trusteeFilter, setTrusteeFilter] = useState('');
  const [pathFilter, setPathFilter] = useState('');
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [detail, setDetail] = useState<PermissionRow | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      const params = new URLSearchParams({ page: String(page), page_size: '50' });
      if (trusteeFilter.trim()) params.set('trustee', trusteeFilter.trim());
      if (pathFilter.trim()) params.set('path', pathFilter.trim());
      const response = await fetch(`${apiBase()}/api/sessions/${session.id}/permissions?${params}`);
      const data = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(data.error || 'unavailable');
      setRows(Array.isArray(data.items) ? data.items : []);
      setTotalPages(data.pagination?.total_pages || 1);
    } catch (requestError) {
      setRows([]);
      setTotalPages(1);
      setLoadError(requestError instanceof Error ? requestError.message : d.resources.permissionsUnavailable);
    } finally {
      setLoading(false);
    }
  }, [d.resources.permissionsUnavailable, page, pathFilter, session.id, trusteeFilter]);

  useEffect(() => {
    load();
  }, [load]);

  // Debounced filter → reset to page 1.
  const applyFilter = (setter: (value: string) => void) => (value: string) => {
    setter(value);
    setPage(1);
  };

  return (
    <Card>
      <CardHeader
        title={interpolate(d.resources.browserTitle, { path: session.root_path })}
        actions={
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
            <Input
              value={trusteeFilter}
              onChange={(event) => applyFilter(setTrusteeFilter)(event.target.value)}
              placeholder={d.resources.filterTrustee}
              className="h-8 w-full text-xs sm:w-48"
            />
            <Input
              value={pathFilter}
              onChange={(event) => applyFilter(setPathFilter)(event.target.value)}
              placeholder={d.resources.filterPath}
              className="h-8 w-full font-mono text-xs sm:w-48"
            />
          </div>
        }
      />
      <CardContent>
        {loading ? (
          <div className="space-y-2">
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
          </div>
        ) : loadError ? (
          <EmptyState
            icon={AlertTriangle}
            title={d.resources.permissionsUnavailable}
            description={`${d.resources.permissionsUnavailableHint} ${loadError}`}
            action={
              <Button aria-label={d.resources.retryPermissions} size="sm" onClick={() => void load()}>
                <RefreshCcw className="h-3.5 w-3.5" /> {d.common.retry}
              </Button>
            }
          />
        ) : rows.length === 0 ? (
          <EmptyState
            icon={FolderSearch}
            title={d.resources.noPermissions}
            description={d.resources.noPermissionsHint}
          />
        ) : (
          <>
            <TableContainer className="max-h-[480px] border-0">
              <Table className="min-w-[860px]">
                <TableHeader>
                  <TableRow>
                    <TableHead>{d.resources.colTrustee}</TableHead>
                    <TableHead>{d.resources.colRights}</TableHead>
                    <TableHead>{d.resources.colType}</TableHead>
                    <TableHead>{d.resources.colInherited}</TableHead>
                    <TableHead>{d.resources.colRisk}</TableHead>
                    <TableHead>{d.resources.colPath}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {rows.map((row) => (
                    <TableRow key={row.id} className="cursor-pointer" onClick={() => setDetail(row)}>
                      <TableCell className="max-w-[220px]">
                        <span className="block truncate text-xs font-medium text-fg" title={row.account_name || row.trustee}>
                          {row.account_name || row.trustee}
                        </span>
                        {row.originating_group ? (
                          <span className="block truncate text-2xs text-fg-muted" title={row.originating_group}>
                            ← {row.originating_group}
                          </span>
                        ) : null}
                      </TableCell>
                      <TableCell className="max-w-[160px]">
                        <span className="block truncate text-2xs" title={row.rights}>
                          {row.rights}
                        </span>
                      </TableCell>
                      <TableCell>
                        <Badge tone={row.type?.toLowerCase() === 'deny' ? 'danger' : 'success'}>
                          {row.type?.toLowerCase() === 'deny' ? d.access.deny : d.access.allow}
                        </Badge>
                      </TableCell>
                      <TableCell>{row.inherited ? d.common.yes : d.common.no}</TableCell>
                      <TableCell>
                        {row.risk_level ? <Badge tone={riskTone(row.risk_level)}>{riskLabel(row.risk_level, locale)}</Badge> : '—'}
                      </TableCell>
                      <TableCell className="max-w-[260px]">
                        <span className="block truncate font-mono text-2xs text-fg-muted" title={row.path}>
                          {row.path}
                        </span>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
            <div className="mt-3 flex items-center justify-between text-xs text-fg-muted">
              <span className="tabular-nums">{interpolate(d.resources.pageInfo, { page, total: totalPages })}</span>
              <div className="flex gap-1.5">
                <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
                  <ChevronLeft className="h-3 w-3" /> {d.resources.pagePrev}
                </Button>
                <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>
                  {d.resources.pageNext} <ChevronRight className="h-3 w-3" />
                </Button>
              </div>
            </div>
          </>
        )}
      </CardContent>
      <PermissionDetail permission={detail} isOpen={Boolean(detail)} onClose={() => setDetail(null)} />
    </Card>
  );
}
