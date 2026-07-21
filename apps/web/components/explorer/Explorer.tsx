import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import Link from 'next/link';
import {
  Boxes,
  Building2,
  ChevronDown,
  ChevronRight,
  FileCog,
  FolderClosed,
  FolderTree,
  GitFork,
  Globe,
  HardDrive,
  Loader2,
  RefreshCcw,
  ScanSearch,
  Server,
  ShieldQuestion,
  UserRound,
  Users,
  Wrench,
  Zap,
} from 'lucide-react';
import { apiBase } from '../../lib/runtimeApi';
import { useDict } from '../../lib/i18n';
import { useADConnection } from '../../contexts/ADConnectionContext';
import { cn } from '../../lib/cn';
import { Badge, riskTone } from '../ui/badge';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Skeleton } from '../ui/skeleton';
import { QuickConnectCard } from '../QuickConnectCard';

/* ----------------------------------- model -------------------------------- */

export type NodeKind = 'ad' | 'share' | 'folder';

export interface ExplorerNode {
  id: string; // dn for ad nodes, path for share/folder nodes
  kind: NodeKind;
  label: string;
  adType?: string; // domain | ou | container | policy | group | user
  path?: string;
  sessionId?: string;
  hasChildren: boolean;
  permissionCount?: number;
}

interface BranchState {
  children: ExplorerNode[];
  expanded: boolean;
  loading: boolean;
  page: number;
  totalPages: number;
  error?: string;
}

function interpolate(template: string, params: Record<string, string | number>) {
  return template.replace(/\{(\w+)\}/g, (_, key) => String(params[key] ?? `{${key}}`));
}

function adIcon(adType: string | undefined) {
  switch (adType) {
    case 'domain':
      return Globe;
    case 'ou':
      return Building2;
    case 'container':
      return Boxes;
    case 'policy':
      return FileCog;
    case 'group':
      return Users;
    case 'user':
      return UserRound;
    default:
      return Boxes;
  }
}

/* --------------------------------- explorer -------------------------------- */

export default function Explorer() {
  const d = useDict();
  const { activeProfile } = useADConnection();
  const connectionId = activeProfile?.id ?? null;

  const [selection, setSelection] = useState<ExplorerNode | null>(null);
  const [adRootOpen, setAdRootOpen] = useState(true);
  const [sharesOpen, setSharesOpen] = useState(true);
  const [branches, setBranches] = useState<Record<string, BranchState>>({});
  const [adRootBranch, setAdRootBranch] = useState<BranchState | null>(null);
  const [shares, setShares] = useState<ExplorerNode[]>([]);
  const [sharesLoading, setSharesLoading] = useState(true);

  /* ------------------------------ AD tree loading ----------------------------- */

  const fetchAdChildren = useCallback(
    async (parentDN: string, page: number) => {
      const response = await fetch(`${apiBase()}/api/ad/tree`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ connection_id: connectionId, parent_dn: parentDN, page, page_size: 80 }),
      });
      const data = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(data.error || d.common.error);
      // The tree endpoint returns `nodes`, not `items`.
      const rawNodes = Array.isArray(data.nodes) ? data.nodes : [];
      const nodes: ExplorerNode[] = rawNodes.map(
        (item: { dn: string; name: string; node_type: string; has_children: boolean }) => ({
          id: item.dn,
          kind: 'ad' as const,
          label: item.name,
          adType: item.node_type,
          hasChildren: item.has_children,
        }),
      );
      return { nodes, totalPages: data.pagination?.total_pages ?? 1 };
    },
    [connectionId, d],
  );

  const loadAdRoot = useCallback(async () => {
    if (!connectionId) return;
    setAdRootBranch({ children: [], expanded: true, loading: true, page: 1, totalPages: 1 });
    try {
      const { nodes, totalPages } = await fetchAdChildren('', 1);
      setAdRootBranch({ children: nodes, expanded: true, loading: false, page: 1, totalPages });
    } catch (error) {
      setAdRootBranch({
        children: [],
        expanded: true,
        loading: false,
        page: 1,
        totalPages: 1,
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }, [connectionId, fetchAdChildren]);

  /* ----------------------------- shares loading ------------------------------ */

  const loadShares = useCallback(async () => {
    setSharesLoading(true);
    try {
      const response = await fetch(`${apiBase()}/api/sessions?status=completed&page=1&page_size=100`);
      const data = await response.json().catch(() => ({}));
      const items: Array<{ id: string; root_path: string; permission_count: number }> = Array.isArray(data.items)
        ? data.items
        : [];
      const byRoot = new Map<string, ExplorerNode>();
      for (const session of items) {
        const key = session.root_path.toLowerCase();
        if (!byRoot.has(key)) {
          byRoot.set(key, {
            id: session.root_path,
            kind: 'share',
            label: session.root_path,
            path: session.root_path,
            sessionId: session.id,
            hasChildren: true,
            permissionCount: session.permission_count,
          });
        }
      }
      setShares(Array.from(byRoot.values()));
    } catch {
      setShares([]);
    } finally {
      setSharesLoading(false);
    }
  }, []);

  const fetchFolderChildren = useCallback(async (node: ExplorerNode) => {
    const params = new URLSearchParams();
    if (node.kind === 'folder' && node.path) params.set('parent', node.path);
    const response = await fetch(`${apiBase()}/api/sessions/${node.sessionId}/folders?${params}`);
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.error || 'unavailable');
    const nodes: ExplorerNode[] = (Array.isArray(data.items) ? data.items : []).map(
      (item: { path: string; name: string; has_children: boolean; permission_count: number }) => ({
        id: item.path,
        kind: 'folder' as const,
        label: item.name,
        path: item.path,
        sessionId: node.sessionId,
        hasChildren: item.has_children,
        permissionCount: item.permission_count,
      }),
    );
    return { nodes, totalPages: 1 };
  }, []);

  useEffect(() => {
    loadShares();
  }, [loadShares]);

  useEffect(() => {
    if (connectionId) loadAdRoot();
  }, [connectionId, loadAdRoot]);

  /* ------------------------------ expand/collapse ----------------------------- */

  const toggleNode = useCallback(
    async (node: ExplorerNode) => {
      const current = branches[node.id];
      if (current) {
        setBranches((prev) => ({ ...prev, [node.id]: { ...current, expanded: !current.expanded } }));
        return;
      }
      setBranches((prev) => ({
        ...prev,
        [node.id]: { children: [], expanded: true, loading: true, page: 1, totalPages: 1 },
      }));
      try {
        const { nodes, totalPages } =
          node.kind === 'ad' ? await fetchAdChildren(node.id, 1) : await fetchFolderChildren(node);
        setBranches((prev) => ({
          ...prev,
          [node.id]: { children: nodes, expanded: true, loading: false, page: 1, totalPages },
        }));
      } catch (error) {
        setBranches((prev) => ({
          ...prev,
          [node.id]: {
            children: [],
            expanded: true,
            loading: false,
            page: 1,
            totalPages: 1,
            error: error instanceof Error ? error.message : String(error),
          },
        }));
      }
    },
    [branches, fetchAdChildren, fetchFolderChildren],
  );

  const loadMoreAd = useCallback(
    async (nodeId: string) => {
      const isRoot = nodeId === '';
      const current = isRoot ? adRootBranch : branches[nodeId];
      if (!current || current.loading || current.page >= current.totalPages) return;
      const nextPage = current.page + 1;
      const update = (state: BranchState) => {
        if (isRoot) setAdRootBranch(state);
        else setBranches((prev) => ({ ...prev, [nodeId]: state }));
      };
      update({ ...current, loading: true });
      try {
        const { nodes, totalPages } = await fetchAdChildren(nodeId, nextPage);
        update({ ...current, children: [...current.children, ...nodes], loading: false, page: nextPage, totalPages });
      } catch (error) {
        update({ ...current, loading: false, error: error instanceof Error ? error.message : String(error) });
      }
    },
    [adRootBranch, branches, fetchAdChildren],
  );

  /* ---------------------------------- render --------------------------------- */

  if (!connectionId && shares.length === 0 && !sharesLoading) {
    return (
      <div className="app-v2 mx-auto max-w-6xl space-y-5">
        <ExplorerHeader />
        <div className="py-6">
          <QuickConnectCard />
        </div>
      </div>
    );
  }

  return (
    <div className="app-v2 flex min-h-0 flex-col gap-3 xl:h-[calc(100vh-7rem)] xl:min-h-[480px]">
      <ExplorerHeader />
      <div className="grid min-h-0 flex-1 grid-cols-1 gap-2 xl:grid-cols-[290px_minmax(0,1fr)_310px]">
        {/* -------- Left: the unified tree -------- */}
        <div className="flex min-h-[320px] min-w-0 flex-col overflow-hidden rounded-lg border border-line bg-surface-base shadow-token-sm xl:min-h-0">
          <PaneHeader
            label={activeProfile ? activeProfile.name : d.explorer.title}
            icon={Zap}
          />
          <div className="flex-1 overflow-y-auto py-1.5">
            {/* AD branch */}
            <TreeRootRow
              icon={Zap}
              label={d.explorer.adRoot}
              open={adRootOpen}
              onToggle={() => setAdRootOpen((value) => !value)}
              trailing={
                connectionId ? (
                  <button
                    type="button"
                    onClick={(event) => {
                      event.stopPropagation();
                      loadAdRoot();
                    }}
                    className="rounded p-0.5 text-fg-faint hover:text-fg"
                    aria-label={d.common.refresh}
                  >
                    <RefreshCcw className="h-3 w-3" />
                  </button>
                ) : null
              }
            />
            {adRootOpen ? (
              !connectionId ? (
                <p className="px-4 py-2 text-2xs text-fg-muted">{d.explorer.connectFirst}</p>
              ) : adRootBranch?.loading && adRootBranch.children.length === 0 ? (
                <TreeSkeleton />
              ) : adRootBranch?.error ? (
                <p className="break-all px-4 py-2 text-2xs text-danger">{adRootBranch.error}</p>
              ) : (
                <>
                  {adRootBranch?.children.map((node) => (
                    <TreeNodeRow
                      key={node.id}
                      node={node}
                      depth={1}
                      branches={branches}
                      selection={selection}
                      onSelect={setSelection}
                      onToggle={toggleNode}
                      onLoadMore={loadMoreAd}
                      d={d}
                    />
                  ))}
                  {adRootBranch && adRootBranch.page < adRootBranch.totalPages ? (
                    <LoadMoreRow depth={1} onClick={() => loadMoreAd('')} label={d.explorer.loadMore} />
                  ) : null}
                </>
              )
            ) : null}

            {/* Shares branch */}
            <TreeRootRow
              icon={HardDrive}
              label={d.explorer.sharesRoot}
              open={sharesOpen}
              onToggle={() => setSharesOpen((value) => !value)}
              trailing={
                <button
                  type="button"
                  onClick={(event) => {
                    event.stopPropagation();
                    loadShares();
                  }}
                  className="rounded p-0.5 text-fg-faint hover:text-fg"
                  aria-label={d.common.refresh}
                >
                  <RefreshCcw className="h-3 w-3" />
                </button>
              }
            />
            {sharesOpen ? (
              sharesLoading ? (
                <TreeSkeleton />
              ) : shares.length === 0 ? (
                <p className="px-4 py-2 text-2xs text-fg-muted">{d.explorer.scanPrompt}</p>
              ) : (
                shares.map((node) => (
                  <TreeNodeRow
                    key={node.id}
                    node={node}
                    depth={1}
                    branches={branches}
                    selection={selection}
                    onSelect={setSelection}
                    onToggle={toggleNode}
                    onLoadMore={loadMoreAd}
                    d={d}
                  />
                ))
              )
            ) : null}
          </div>

          {/* Scan strip at the bottom of the tree — grow the shares branch in place */}
          <ScanStrip onScanned={loadShares} connectionId={connectionId} />
        </div>

        {/* -------- Middle: the answer pane -------- */}
        <div className="flex min-h-[320px] min-w-0 flex-col overflow-hidden rounded-lg border border-line bg-surface-base shadow-token-sm xl:min-h-0">
          <PaneHeader
            label={
              selection
                ? selection.kind === 'ad' && selection.adType === 'user'
                  ? d.explorer.whatCanReach
                  : selection.kind === 'share' || selection.kind === 'folder'
                    ? d.explorer.whoCanAccess
                    : selection.label
                : d.explorer.empty
            }
            icon={GitFork}
          />
          <div className="min-h-0 flex-1 overflow-y-auto">
            <AnswerPane
              selection={selection}
              connectionId={connectionId}
              onSelect={setSelection}
              shares={shares}
              domainName={adRootBranch?.children[0]?.label || activeProfile?.name || null}
            />
          </div>
        </div>

        {/* -------- Right: inspector -------- */}
        <div className="flex min-h-[320px] min-w-0 flex-col overflow-hidden rounded-lg border border-line bg-surface-base shadow-token-sm xl:min-h-0">
          <PaneHeader label={d.explorer.inspector} icon={Wrench} />
          <div className="min-h-0 flex-1 overflow-y-auto">
            <InspectorPane selection={selection} connectionId={connectionId} />
          </div>
        </div>
      </div>
    </div>
  );
}

function PaneHeader({ label, icon: Icon }: { label: string; icon: React.ComponentType<{ className?: string }> }) {
  return (
    <div className="flex h-8 shrink-0 items-center gap-1.5 border-b border-line bg-surface-raised px-3">
      <Icon className="h-3 w-3 text-accent-fg" />
      <span className="truncate text-2xs font-semibold uppercase tracking-wider text-fg-muted">{label}</span>
    </div>
  );
}

function ExplorerHeader() {
  const d = useDict();
  return (
    <div className="flex flex-col gap-2 sm:flex-row sm:items-baseline sm:justify-between">
      <div className="flex items-baseline gap-3">
        <h1 className="text-lg font-semibold text-fg">{d.explorer.title}</h1>
        <p className="hidden text-sm text-fg-muted md:block">{d.explorer.subtitle}</p>
      </div>
      <div className="flex flex-wrap items-center gap-2 text-xs">
        <Link href="/access" className="inline-flex items-center gap-1 font-medium text-accent-fg hover:underline">
          <GitFork className="h-3 w-3" /> {d.explorer.openAccess}
        </Link>
        <span className="text-fg-faint">·</span>
        <Link href="/scan-workspace" className="inline-flex items-center gap-1 font-medium text-accent-fg hover:underline">
          <Wrench className="h-3 w-3" /> {d.explorer.openWorkbench}
        </Link>
      </div>
    </div>
  );
}

/* --------------------------------- tree rows -------------------------------- */

function TreeRootRow({
  icon: Icon,
  label,
  open,
  onToggle,
  trailing,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  open: boolean;
  onToggle: () => void;
  trailing?: React.ReactNode;
}) {
  // Note: trailing actions must stay siblings of the toggle button — nesting
  // buttons breaks HTML parsing and crashes React hydration.
  return (
    <div className="group flex w-full items-center gap-1.5 px-2 py-1.5 hover:bg-surface-sunken">
      <button
        type="button"
        onClick={onToggle}
        className="flex min-w-0 flex-1 items-center gap-1.5 text-left text-xs font-semibold uppercase tracking-wide text-fg-secondary"
      >
        {open ? <ChevronDown className="h-3 w-3 text-fg-faint" /> : <ChevronRight className="h-3 w-3 text-fg-faint" />}
        <Icon className="h-3.5 w-3.5 text-accent-fg" />
        <span className="flex-1 truncate">{label}</span>
      </button>
      {trailing}
    </div>
  );
}

function TreeNodeRow({
  node,
  depth,
  branches,
  selection,
  onSelect,
  onToggle,
  onLoadMore,
  d,
}: {
  node: ExplorerNode;
  depth: number;
  branches: Record<string, BranchState>;
  selection: ExplorerNode | null;
  onSelect: (node: ExplorerNode) => void;
  onToggle: (node: ExplorerNode) => void;
  onLoadMore: (nodeId: string) => void;
  d: ReturnType<typeof useDict>;
}) {
  const branch = branches[node.id];
  const selected = selection?.id === node.id;
  const Icon = node.kind === 'ad' ? adIcon(node.adType) : node.kind === 'share' ? Server : FolderClosed;

  // Shares read best as "leaf name + dim full path" on two lines.
  const shareLeaf =
    node.kind === 'share' ? node.label.replace(/[\\/]+$/, '').split(/[\\/]/).filter(Boolean).pop() || node.label : null;

  return (
    <div>
      <div
        className={cn(
          'group flex w-full cursor-pointer items-center gap-1.5 border-l-2 py-1 pr-2 text-left transition-colors',
          selected
            ? 'border-accent bg-accent-soft text-accent-fg'
            : 'border-transparent hover:bg-surface-sunken',
        )}
        style={{ paddingLeft: `${depth * 14 + 4}px` }}
        onClick={() => onSelect(node)}
        title={node.id}
        role="treeitem"
        aria-selected={selected}
      >
        {node.hasChildren ? (
          <button
            type="button"
            onClick={(event) => {
              event.stopPropagation();
              onToggle(node);
            }}
            className="shrink-0 rounded p-0.5 text-fg-faint hover:text-fg"
            aria-label={branch?.expanded ? 'collapse' : 'expand'}
          >
            {branch?.loading ? (
              <Loader2 className="h-3 w-3 animate-spin" />
            ) : branch?.expanded ? (
              <ChevronDown className="h-3 w-3" />
            ) : (
              <ChevronRight className="h-3 w-3" />
            )}
          </button>
        ) : (
          <span className="w-4 shrink-0" />
        )}
        <Icon
          className={cn(
            'h-3.5 w-3.5 shrink-0',
            selected
              ? 'text-accent-fg'
              : node.kind === 'share'
                ? 'text-accent-fg/70'
                : node.adType === 'group'
                  ? 'text-info'
                  : 'text-fg-muted',
          )}
        />
        {shareLeaf ? (
          <span className="min-w-0 flex-1">
            <span className={cn('block truncate text-xs font-medium', selected ? 'text-accent-fg' : 'text-fg')}>
              {shareLeaf}
            </span>
            <span className="block truncate font-mono text-2xs leading-3 text-fg-faint">{node.label}</span>
          </span>
        ) : (
          <span
            className={cn(
              'min-w-0 flex-1 truncate text-xs',
              node.kind === 'folder' && 'font-mono text-2xs',
              selected ? 'text-accent-fg' : 'text-fg-secondary',
            )}
          >
            {node.label}
          </span>
        )}
        {typeof node.permissionCount === 'number' && node.permissionCount > 0 ? (
          <span className="shrink-0 font-mono text-2xs tabular-nums text-fg-faint">{node.permissionCount}</span>
        ) : null}
      </div>
      {branch?.expanded ? (
        <div role="group" className="relative">
          {/* Indent guide */}
          <span
            aria-hidden
            className="pointer-events-none absolute bottom-0 top-0 w-px bg-line/70"
            style={{ left: `${depth * 14 + 11}px` }}
          />
          {branch.error ? (
            <p className="break-all py-1 text-2xs text-danger" style={{ paddingLeft: `${(depth + 1) * 14 + 6}px` }}>
              {branch.error}
            </p>
          ) : null}
          {branch.children.map((child) => (
            <TreeNodeRow
              key={child.id}
              node={child}
              depth={depth + 1}
              branches={branches}
              selection={selection}
              onSelect={onSelect}
              onToggle={onToggle}
              onLoadMore={onLoadMore}
              d={d}
            />
          ))}
          {node.kind === 'ad' && branch.page < branch.totalPages ? (
            <LoadMoreRow depth={depth + 1} onClick={() => onLoadMore(node.id)} label={d.explorer.loadMore} />
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function LoadMoreRow({ depth, onClick, label }: { depth: number; onClick: () => void; label: string }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="py-1 text-2xs font-medium text-accent-fg hover:underline"
      style={{ paddingLeft: `${depth * 14 + 24}px` }}
    >
      {label}
    </button>
  );
}

function TreeSkeleton() {
  return (
    <div className="space-y-1.5 px-4 py-2">
      <Skeleton className="h-4 w-3/4" />
      <Skeleton className="h-4 w-2/3" />
      <Skeleton className="h-4 w-4/5" />
    </div>
  );
}

/* --------------------------------- scan strip ------------------------------- */

function ScanStrip({ onScanned, connectionId }: { onScanned: () => void; connectionId: string | null }) {
  const d = useDict();
  const [path, setPath] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const scan = useCallback(async () => {
    const target = path.trim();
    if (!target || busy) return;
    setBusy(true);
    setError('');
    try {
      const body: Record<string, unknown> = {
        path: target,
        depth: -1,
        include_inherited: true,
        scan_id: `explorer-${Date.now().toString(36)}`,
      };
      if (connectionId) body.effective_permissions = { enabled: true, connection_id: connectionId };
      const response = await fetch(`${apiBase()}/api/scan`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      const data = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(data.error || d.resources.scanFailed);
      setPath('');
      onScanned();
    } catch (err) {
      setError(err instanceof Error ? err.message : d.resources.scanFailed);
    } finally {
      setBusy(false);
    }
  }, [busy, connectionId, d, onScanned, path]);

  return (
    <div className="border-t border-line p-2">
      <div className="flex flex-col gap-1.5 sm:flex-row sm:items-center">
        <Input
          value={path}
          onChange={(event) => setPath(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') scan();
          }}
          placeholder={d.explorer.scanPlaceholder}
          className="h-7 min-w-0 flex-1 font-mono text-2xs"
          disabled={busy}
          aria-label={d.explorer.scanPrompt}
        />
        <Button size="sm" onClick={scan} loading={busy} disabled={!path.trim()} className="w-full sm:w-auto">
          <ScanSearch className="h-3 w-3" /> {d.explorer.scanAction}
        </Button>
      </div>
      {error ? <p className="mt-1 break-all text-2xs text-danger">{error}</p> : null}
    </div>
  );
}

/* -------------------------------- answer pane ------------------------------- */

function AnswerPane({
  selection,
  connectionId,
  onSelect,
  shares,
  domainName,
}: {
  selection: ExplorerNode | null;
  connectionId: string | null;
  onSelect: (node: ExplorerNode) => void;
  shares: ExplorerNode[];
  domainName: string | null;
}) {
  const d = useDict();

  if (!selection) {
    return <QueryPrompt shares={shares} domainName={domainName} onSelect={onSelect} />;
  }

  if (selection.kind === 'ad' && selection.adType === 'user') {
    return <UserAnswer key={selection.id} node={selection} connectionId={connectionId} />;
  }
  if (selection.kind === 'ad' && selection.adType === 'group') {
    return <GroupAnswer key={selection.id} node={selection} connectionId={connectionId} />;
  }
  if (selection.kind === 'share' || selection.kind === 'folder') {
    return <ResourceAnswer key={selection.id} node={selection} />;
  }
  // OU / container / domain — structural node
  return (
    <div className="flex h-full flex-col items-center justify-center gap-2 px-6 text-center">
      <Building2 className="h-8 w-8 text-fg-faint" />
      <p className="break-all font-mono text-2xs text-fg-muted">{selection.id}</p>
      <p className="text-xs text-fg-muted">{d.explorer.emptyHint}</p>
    </div>
  );
}

/** The empty state IS the brand moment: a terminal-style prompt that turns
 *  the operator's real data into clickable questions. */
function QueryPrompt({
  shares,
  domainName,
  onSelect,
}: {
  shares: ExplorerNode[];
  domainName: string | null;
  onSelect: (node: ExplorerNode) => void;
}) {
  const d = useDict();
  const suggestions = shares.slice(0, 2);

  return (
    <div className="flex h-full min-w-0 flex-col items-center justify-center gap-6 overflow-hidden px-4 py-6 sm:gap-8 sm:px-8">
      {/* The prompt */}
      <div className="w-full max-w-full font-mono sm:max-w-lg">
        <p className="text-2xs uppercase tracking-[0.18em] text-fg-faint sm:tracking-[0.25em]">OpenAD</p>
        <p className="mt-3 flex items-baseline gap-2 text-xl font-semibold leading-7 text-fg sm:gap-3 sm:text-2xl sm:leading-8">
          <span className="select-none text-accent">▸</span>
          <span>
            {d.explorer.promptWho} <span className="text-accent-fg">\\…</span>
            <span className="pp-cursor ml-1 inline-block h-6 w-[10px] translate-y-1 bg-accent" aria-hidden />
          </span>
        </p>
        <p className="mt-2 flex items-baseline gap-2 text-xl font-semibold leading-7 text-fg-faint sm:gap-3 sm:text-2xl sm:leading-8">
          <span className="select-none">▸</span>
          <span>{d.explorer.promptWhat}</span>
        </p>
        <p className="mt-5 text-sm text-fg-muted">{d.explorer.promptTagline}</p>
      </div>

      {/* Real-data suggestions */}
      <div className="w-full max-w-full space-y-2 sm:max-w-lg">
        <p className="break-words text-2xs uppercase leading-4 tracking-wider text-fg-faint">{d.explorer.promptHint}</p>
        {suggestions.map((share) => (
          <button
            key={share.id}
            type="button"
            onClick={() => onSelect(share)}
            className="group flex w-full min-w-0 items-center gap-3 rounded-lg border border-line bg-surface-raised px-4 py-3 text-left transition-colors hover:border-accent/60 hover:bg-accent-soft/40"
          >
            <Server className="h-4 w-4 shrink-0 text-fg-muted transition-colors group-hover:text-accent-fg" />
            <span className="min-w-0 flex-1 truncate font-mono text-xs text-fg-secondary group-hover:text-fg">
              {interpolate(d.explorer.suggestWho, { path: share.label })}
            </span>
            {typeof share.permissionCount === 'number' ? (
              <span className="shrink-0 font-mono text-2xs tabular-nums text-fg-faint">
                {share.permissionCount.toLocaleString()}
              </span>
            ) : null}
            <ChevronRight className="h-3.5 w-3.5 shrink-0 text-fg-faint transition-transform group-hover:translate-x-0.5 group-hover:text-accent-fg" />
          </button>
        ))}
        {domainName ? (
          <div className="flex w-full min-w-0 items-center gap-3 rounded-lg border border-dashed border-line px-4 py-3">
            <Globe className="h-4 w-4 shrink-0 text-fg-muted" />
            <span className="min-w-0 flex-1 truncate text-xs text-fg-muted">
              {interpolate(d.explorer.suggestExpand, { domain: domainName })}
            </span>
          </div>
        ) : null}
      </div>
    </div>
  );
}

/** Big-number stat band that anchors every answer view. */
function StatBand({ stats }: { stats: Array<{ value: number; label: string; tone?: 'danger' | 'warning' }> }) {
  return (
    <div className="flex divide-x divide-line overflow-hidden rounded-lg border border-line bg-surface-raised">
      {stats.map((stat) => (
        <div key={stat.label} className="flex-1 px-4 py-2.5">
          <p
            className={cn(
              'font-mono text-xl font-semibold leading-6 tabular-nums',
              stat.tone === 'danger' ? 'text-danger' : stat.tone === 'warning' ? 'text-warning' : 'text-fg',
            )}
          >
            {stat.value.toLocaleString()}
          </p>
          <p className="mt-0.5 truncate text-2xs uppercase tracking-wide text-fg-faint">{stat.label}</p>
        </div>
      ))}
    </div>
  );
}

interface ResolvedPrincipal {
  sid?: string;
  sam_account_name?: string;
  name?: string;
  email?: string;
  department?: string;
}

function useResolvedPrincipal(dn: string, connectionId: string | null) {
  const [principal, setPrincipal] = useState<ResolvedPrincipal | null>(null);
  const [error, setError] = useState('');
  useEffect(() => {
    let cancelled = false;
    if (!connectionId) return;
    (async () => {
      try {
        const response = await fetch(`${apiBase()}/api/ad/principals/expand`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ connection_id: connectionId, identifiers: [dn], include_nested: false }),
        });
        const data = await response.json().catch(() => ({}));
        if (!response.ok) throw new Error(data.error || 'resolve failed');
        const resolved = Array.isArray(data.results) ? data.results[0]?.principal : data.principal;
        const principalData = resolved || (Array.isArray(data.principals) ? data.principals[0] : null);
        if (!cancelled) setPrincipal(principalData || null);
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [connectionId, dn]);
  return { principal, error };
}

function UserAnswer({ node, connectionId }: { node: ExplorerNode; connectionId: string | null }) {
  const d = useDict();
  const { principal } = useResolvedPrincipal(node.id, connectionId);
  const [result, setResult] = useState<{
    groups: Array<{ group_name: string; direct: boolean; via_chain?: string }>;
    by_root_path: Array<{ root_path: string; entries: Array<AccessEntryLite> }>;
    counts: { total: number };
  } | null>(null);
  const [status, setStatus] = useState<'loading' | 'ready' | 'need-sync' | 'need-scan' | 'error'>('loading');
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    const identifier = principal?.sid || principal?.sam_account_name;
    if (!identifier) return;
    (async () => {
      try {
        const response = await fetch(`${apiBase()}/api/access/by-user`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ principal: identifier }),
        });
        const data = await response.json().catch(() => ({}));
        if (cancelled) return;
        if (response.status === 409) {
          setStatus(String(data.error || '').includes('scan') ? 'need-scan' : 'need-sync');
          setError(data.error || '');
          return;
        }
        if (!response.ok) throw new Error(data.error || d.common.error);
        setResult(data);
        setStatus('ready');
      } catch (err) {
        if (!cancelled) {
          setStatus('error');
          setError(err instanceof Error ? err.message : String(err));
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [d, principal]);

  return (
    <div className="space-y-4 p-4">
      <div>
        <p className="flex items-center gap-2 text-sm font-semibold text-fg">
          <UserRound className="h-4 w-4 text-accent-fg" /> {node.label}
        </p>
        {principal?.sam_account_name ? (
          <p className="mt-0.5 font-mono text-2xs text-fg-muted">{principal.sam_account_name}</p>
        ) : null}
      </div>

      {status === 'loading' ? (
        <div className="space-y-2">
          <Skeleton className="h-6 w-2/3" />
          <Skeleton className="h-24 w-full" />
        </div>
      ) : status === 'need-sync' ? (
        <SyncNudge connectionId={connectionId} detail={error} />
      ) : status === 'need-scan' ? (
        <p className="rounded-md border border-warning/40 bg-warning-soft px-3 py-2 text-xs text-warning">
          {d.explorer.needScan}
        </p>
      ) : status === 'error' ? (
        <p className="break-all rounded-md border border-danger/40 bg-danger-soft px-3 py-2 text-xs text-danger">{error}</p>
      ) : result ? (
        <>
          <StatBand
            stats={[
              { value: result.counts.total, label: d.explorer.statPaths },
              { value: result.groups.length, label: d.explorer.statGroups },
              {
                value: (result as { counts: { deny?: number } }).counts.deny ?? 0,
                label: d.explorer.statDeny,
                tone: ((result as { counts: { deny?: number } }).counts.deny ?? 0) > 0 ? 'danger' : undefined,
              },
            ]}
          />
          {result.by_root_path.length === 0 ? (
            <p className="text-xs text-fg-muted">{d.common.empty}</p>
          ) : (
            result.by_root_path.map((group) => (
              <div key={group.root_path} className="overflow-hidden rounded-md border border-line">
                <p className="border-b border-line bg-surface-raised px-3 py-1.5 font-mono text-2xs font-medium text-fg">
                  {group.root_path}
                </p>
                <ul className="divide-y divide-line/60">
                  {group.entries.slice(0, 30).map((entry, index) => (
                    <AccessEntryRow key={`${entry.path}-${index}`} entry={entry} d={d} />
                  ))}
                </ul>
              </div>
            ))
          )}
        </>
      ) : null}
    </div>
  );
}

interface AccessEntryLite {
  path: string;
  rights: string;
  type: string;
  risk_level?: string;
  why: { kind: string; group_name?: string; via_chain?: string };
}

function AccessEntryRow({ entry, d }: { entry: AccessEntryLite; d: ReturnType<typeof useDict> }) {
  return (
    <li className="flex items-center gap-2 px-3 py-1.5">
      <span className="min-w-0 flex-1 truncate font-mono text-2xs text-fg-secondary" title={entry.path}>
        {entry.path}
      </span>
      <span className="max-w-[120px] shrink-0 truncate text-2xs text-fg-muted" title={entry.rights}>
        {entry.rights}
      </span>
      {entry.type?.toLowerCase() === 'deny' ? <Badge tone="danger">{d.explorer.deny}</Badge> : null}
      {entry.risk_level ? <Badge tone={riskTone(entry.risk_level)}>{entry.risk_level}</Badge> : null}
      {entry.why.kind === 'direct' ? (
        <Badge tone="accent">{d.explorer.directBadge}</Badge>
      ) : (
        <Badge tone="info" className="max-w-[140px]">
          <span className="truncate" title={entry.why.via_chain || entry.why.group_name}>
            {interpolate(d.explorer.viaBadge, { group: entry.why.group_name || '' })}
          </span>
        </Badge>
      )}
    </li>
  );
}

function GroupAnswer({ node, connectionId }: { node: ExplorerNode; connectionId: string | null }) {
  const d = useDict();
  const [members, setMembers] = useState<Array<{ dn: string; name?: string; sam_account_name?: string; type: string; depth: number }> | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    if (!connectionId) return;
    (async () => {
      try {
        const response = await fetch(`${apiBase()}/api/ad/groups/members`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ connection_id: connectionId, group_dn: node.id, include_nested: true, max_depth: 10 }),
        });
        const data = await response.json().catch(() => ({}));
        if (!response.ok) throw new Error(data.error || d.common.error);
        if (!cancelled) setMembers(data.resolution?.members || data.group?.members?.map((m: never) => ({ ...(m as object), depth: 0 })) || []);
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [connectionId, d, node.id]);

  return (
    <div className="space-y-3 p-4">
      <p className="flex items-center gap-2 text-sm font-semibold text-fg">
        <Users className="h-4 w-4 text-info" /> {node.label}
      </p>
      {error ? (
        <p className="break-all rounded-md border border-danger/40 bg-danger-soft px-3 py-2 text-xs text-danger">{error}</p>
      ) : members === null ? (
        <div className="space-y-2">
          <Skeleton className="h-5 w-full" />
          <Skeleton className="h-5 w-4/5" />
        </div>
      ) : (
        <>
          <p className="text-2xs uppercase tracking-wide text-fg-muted">
            {d.explorer.members} · <b className="tabular-nums">{members.length}</b>
          </p>
          <ul className="divide-y divide-line/60 overflow-hidden rounded-md border border-line">
            {members.slice(0, 100).map((member) => (
              <li key={member.dn} className="flex items-center gap-2 px-3 py-1.5">
                {member.type === 'group' ? (
                  <Users className="h-3 w-3 shrink-0 text-info" />
                ) : (
                  <UserRound className="h-3 w-3 shrink-0 text-fg-muted" />
                )}
                <span className="min-w-0 flex-1 truncate text-xs text-fg-secondary">
                  {member.name || member.sam_account_name || member.dn}
                </span>
                {member.depth > 0 ? <Badge tone="neutral">L{member.depth}</Badge> : <Badge tone="accent">{d.explorer.directBadge}</Badge>}
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}

interface ResourcePrincipalLite {
  sid: string;
  name: string;
  source: string;
  rights: string[];
  types: string[];
  risk_level?: string;
  group_name?: string;
  via_chain?: string;
}

function ResourceAnswer({ node }: { node: ExplorerNode }) {
  const d = useDict();
  const { activeProfile } = useADConnection();
  const [principals, setPrincipals] = useState<ResourcePrincipalLite[] | null>(null);
  const [aces, setAces] = useState<Array<{ trustee: string; rights: string; type: string; inherited: boolean; risk_level?: string }> | null>(null);
  const [needSync, setNeedSync] = useState(false);
  const [filter, setFilter] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    (async () => {
      // Resolved principals (needs an AD snapshot); raw ACEs always work.
      const byResource = fetch(`${apiBase()}/api/access/by-resource`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path_prefix: node.path }),
      })
        .then(async (response) => {
          const data = await response.json().catch(() => ({}));
          if (response.status === 409) {
            if (!cancelled) setNeedSync(true);
            return;
          }
          if (!response.ok) throw new Error(data.error || '');
          if (!cancelled) setPrincipals(data.principals || []);
        })
        .catch(() => undefined);

      const rawAces = fetch(
        `${apiBase()}/api/sessions/${node.sessionId}/permissions?page=1&page_size=100&path=${encodeURIComponent(node.path || '')}`,
      )
        .then(async (response) => {
          const data = await response.json().catch(() => ({}));
          if (!response.ok) throw new Error(data.error || '');
          if (!cancelled) setAces(data.items || []);
        })
        .catch((err) => {
          if (!cancelled) setError(err instanceof Error ? err.message : String(err));
        });

      await Promise.all([byResource, rawAces]);
    })();
    return () => {
      cancelled = true;
    };
  }, [node.path, node.sessionId]);

  const filteredPrincipals = useMemo(() => {
    if (!principals) return null;
    const term = filter.trim().toLowerCase();
    if (!term) return principals;
    return principals.filter(
      (principal) =>
        principal.name.toLowerCase().includes(term) ||
        (principal.group_name || '').toLowerCase().includes(term),
    );
  }, [filter, principals]);

  return (
    <div className="space-y-3 p-4">
      <div className="flex items-center justify-between gap-2">
        <p className="flex min-w-0 items-center gap-2 text-sm font-semibold text-fg">
          <Server className="h-4 w-4 shrink-0 text-accent-fg" />
          <span className="truncate font-mono text-xs" title={node.path}>
            {node.path}
          </span>
        </p>
        {principals && principals.length > 4 ? (
          <Input
            value={filter}
            onChange={(event) => setFilter(event.target.value)}
            placeholder={d.explorer.filterQuick}
            className="h-7 w-36 text-2xs"
          />
        ) : null}
      </div>

      {needSync ? <SyncNudge connectionId={activeProfile?.id ?? null} detail="" /> : null}

      {aces !== null ? (
        <StatBand
          stats={[
            ...(principals !== null ? [{ value: principals.length, label: d.explorer.statPrincipals }] : []),
            { value: aces.length, label: d.explorer.statEntries },
            {
              value: aces.filter((ace) => ace.type?.toLowerCase() === 'deny').length,
              label: d.explorer.statDeny,
              tone: aces.some((ace) => ace.type?.toLowerCase() === 'deny') ? ('danger' as const) : undefined,
            },
            {
              value: aces.filter((ace) => (ace.risk_level || '').toLowerCase() === 'high').length,
              label: d.explorer.statHighRisk,
              tone: aces.some((ace) => (ace.risk_level || '').toLowerCase() === 'high') ? ('warning' as const) : undefined,
            },
          ]}
        />
      ) : null}

      {filteredPrincipals && filteredPrincipals.length > 0 ? (
        <div>
          <p className="mb-1.5 text-2xs uppercase tracking-wide text-fg-muted">
            {d.explorer.whoCanAccess} · <b className="tabular-nums">{filteredPrincipals.length}</b> {d.explorer.principals}
          </p>
          <ul className="divide-y divide-line/60 overflow-hidden rounded-md border border-line">
            {filteredPrincipals.slice(0, 100).map((principal, index) => (
              <li key={`${principal.sid}-${index}`} className="flex items-center gap-2 px-3 py-1.5">
                {principal.source === 'unresolved' ? (
                  <ShieldQuestion className="h-3 w-3 shrink-0 text-warning" />
                ) : principal.source === 'group-member' ? (
                  <Users className="h-3 w-3 shrink-0 text-info" />
                ) : (
                  <UserRound className="h-3 w-3 shrink-0 text-fg-muted" />
                )}
                <span className="min-w-0 flex-1 truncate text-xs text-fg-secondary" title={principal.sid}>
                  {principal.name}
                </span>
                {principal.source === 'group-member' && principal.group_name ? (
                  <Badge tone="info" className="max-w-[130px]">
                    <span className="truncate" title={principal.via_chain || principal.group_name}>
                      {interpolate(d.explorer.viaBadge, { group: principal.group_name })}
                    </span>
                  </Badge>
                ) : null}
                {principal.source === 'unresolved' ? <Badge tone="warning">{d.explorer.unresolved}</Badge> : null}
                <span className="max-w-[110px] shrink-0 truncate text-2xs text-fg-muted" title={principal.rights.join(', ')}>
                  {principal.rights.join(', ')}
                </span>
                {principal.types.includes('deny') ? <Badge tone="danger">{d.explorer.deny}</Badge> : null}
                {principal.risk_level ? <Badge tone={riskTone(principal.risk_level)}>{principal.risk_level}</Badge> : null}
              </li>
            ))}
          </ul>
        </div>
      ) : null}

      {/* Raw ACEs — always available, even before any directory sync */}
      {aces === null && !error ? (
        <div className="space-y-2">
          <Skeleton className="h-5 w-full" />
          <Skeleton className="h-5 w-4/5" />
        </div>
      ) : error ? (
        <p className="break-all rounded-md border border-danger/40 bg-danger-soft px-3 py-2 text-xs text-danger">{error}</p>
      ) : aces && aces.length > 0 ? (
        <div>
          <p className="mb-1.5 text-2xs uppercase tracking-wide text-fg-muted">{d.explorer.aceFallbackTitle}</p>
          <ul className="divide-y divide-line/60 overflow-hidden rounded-md border border-line">
            {aces.slice(0, 60).map((ace, index) => (
              <li key={index} className="flex items-center gap-2 px-3 py-1.5">
                <span className="min-w-0 flex-1 truncate text-xs text-fg-secondary" title={ace.trustee}>
                  {ace.trustee}
                </span>
                <span className="max-w-[130px] shrink-0 truncate text-2xs text-fg-muted" title={ace.rights}>
                  {ace.rights}
                </span>
                {ace.type?.toLowerCase() === 'deny' ? <Badge tone="danger">{d.explorer.deny}</Badge> : null}
                {ace.risk_level ? <Badge tone={riskTone(ace.risk_level)}>{ace.risk_level}</Badge> : null}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}

/* -------------------------------- sync nudge -------------------------------- */

function SyncNudge({ connectionId, detail }: { connectionId: string | null; detail: string }) {
  const d = useDict();
  const [running, setRunning] = useState(false);
  const [message, setMessage] = useState('');
  const timerRef = useRef<number | null>(null);

  useEffect(() => () => {
    if (timerRef.current) window.clearInterval(timerRef.current);
  }, []);

  const startSync = useCallback(async () => {
    if (!connectionId || running) return;
    setRunning(true);
    setMessage('');
    try {
      const response = await fetch(`${apiBase()}/api/ad/sync`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ connection_id: connectionId }),
      });
      const data = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(data.error || d.common.error);
      // Poll until the run finishes, then reload so the answer panes refresh.
      timerRef.current = window.setInterval(async () => {
        const runsResponse = await fetch(`${apiBase()}/api/ad/sync/runs?page=1&page_size=1`).catch(() => null);
        const runs = runsResponse ? await runsResponse.json().catch(() => ({})) : {};
        const run = Array.isArray(runs.items) ? runs.items[0] : null;
        if (run && run.status !== 'running') {
          if (timerRef.current) window.clearInterval(timerRef.current);
          window.location.reload();
        }
      }, 3000);
    } catch (err) {
      setRunning(false);
      setMessage(err instanceof Error ? err.message : String(err));
    }
  }, [connectionId, d, running]);

  return (
    <div className="rounded-md border border-warning/40 bg-warning-soft px-3 py-2.5">
      <p className="text-xs font-medium text-warning">{d.explorer.needSyncTitle}</p>
      <p className="mt-0.5 text-2xs text-warning/90">{detail || d.explorer.needSync}</p>
      {message ? <p className="mt-1 break-all text-2xs text-danger">{message}</p> : null}
      <Button size="sm" className="mt-2" onClick={startSync} loading={running} disabled={!connectionId}>
        {running ? d.explorer.syncRunning : d.explorer.syncNow}
      </Button>
    </div>
  );
}

/* --------------------------------- inspector -------------------------------- */

function InspectorPane({ selection, connectionId }: { selection: ExplorerNode | null; connectionId: string | null }) {
  const d = useDict();
  const { activeProfile } = useADConnection();

  if (!selection) {
    return (
      <div className="space-y-4 p-4">
        {activeProfile ? <ConnectionCard profile={activeProfile} /> : null}
      </div>
    );
  }

  return (
    <div className="space-y-4 p-4">
      {selection.kind === 'ad' && selection.adType === 'user' ? (
        <UserInspector node={selection} connectionId={connectionId} />
      ) : selection.kind === 'share' || selection.kind === 'folder' ? (
        <ResourceInspector node={selection} />
      ) : (
        <div>
          <p className="text-sm font-medium text-fg">{selection.label}</p>
          <p className="mt-1 break-all font-mono text-2xs text-fg-muted">{selection.id}</p>
          {selection.adType ? (
            <Badge tone="neutral" className="mt-2">
              {selection.adType}
            </Badge>
          ) : null}
        </div>
      )}
    </div>
  );
}

/** Idle inspector: the live connection + directory snapshot at a glance. */
function ConnectionCard({ profile }: { profile: { id: string; name: string; server: string; base_dn: string } }) {
  const d = useDict();
  const [run, setRun] = useState<{ status: string; user_count: number; group_count: number; finished_at?: string; started_at: string } | null>(null);
  const [starting, setStarting] = useState(false);

  const loadRun = useCallback(async () => {
    try {
      const response = await fetch(`${apiBase()}/api/ad/sync/runs?page=1&page_size=1`);
      const data = await response.json().catch(() => ({}));
      setRun(Array.isArray(data.items) && data.items.length > 0 ? data.items[0] : null);
    } catch {
      /* backend offline — leave as unknown */
    }
  }, []);

  useEffect(() => {
    loadRun();
  }, [loadRun]);

  useEffect(() => {
    if (run?.status !== 'running') return;
    const timer = window.setInterval(loadRun, 3000);
    return () => window.clearInterval(timer);
  }, [loadRun, run?.status]);

  const startSync = useCallback(async () => {
    setStarting(true);
    try {
      await fetch(`${apiBase()}/api/ad/sync`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ connection_id: profile.id }),
      });
      await loadRun();
    } finally {
      setStarting(false);
    }
  }, [loadRun, profile.id]);

  const running = run?.status === 'running';

  return (
    <div className="space-y-3">
      <div className="rounded-lg border border-line bg-surface-raised p-3">
        <p className="flex items-center gap-1.5 text-xs font-semibold text-fg">
          <Zap className="h-3.5 w-3.5 text-accent-fg" /> {profile.name}
        </p>
        <p className="mt-1.5 break-all font-mono text-2xs text-fg-muted">{profile.server}</p>
        <p className="break-all font-mono text-2xs text-fg-faint">{profile.base_dn}</p>
      </div>

      <div className="rounded-lg border border-line p-3">
        <p className="text-2xs font-semibold uppercase tracking-wide text-fg-muted">{d.identity.syncTitle}</p>
        {run ? (
          <div className="mt-2 space-y-1 text-2xs text-fg-secondary">
            <p className="flex items-center gap-1.5">
              <span
                className={cn(
                  'h-1.5 w-1.5 rounded-full',
                  run.status === 'completed' ? 'bg-success' : run.status === 'running' ? 'animate-pulse bg-info' : 'bg-danger',
                )}
              />
              {run.status}
              <span className="text-fg-faint">
                · {new Date(run.finished_at || run.started_at).toLocaleString()}
              </span>
            </p>
            <p className="font-mono tabular-nums text-fg-muted">
              {run.user_count.toLocaleString()} users · {run.group_count.toLocaleString()} groups
            </p>
          </div>
        ) : (
          <p className="mt-2 text-2xs text-fg-muted">{d.identity.syncNever}</p>
        )}
        <Button size="sm" variant="outline" className="mt-2.5 w-full" onClick={startSync} loading={starting || running}>
          <RefreshCcw className="h-3 w-3" /> {running ? d.identity.syncRunning : d.identity.syncNow}
        </Button>
      </div>
    </div>
  );
}

function UserInspector({ node, connectionId }: { node: ExplorerNode; connectionId: string | null }) {
  const d = useDict();
  const { principal } = useResolvedPrincipal(node.id, connectionId);

  return (
    <div className="space-y-3">
      <div>
        <p className="text-sm font-medium text-fg">{node.label}</p>
        <p className="mt-1 break-all font-mono text-2xs text-fg-muted">{node.id}</p>
      </div>
      {principal ? (
        <dl className="space-y-2 text-xs">
          {principal.sam_account_name ? <InspectorFact label={d.explorer.account} value={principal.sam_account_name} mono /> : null}
          {principal.email ? <InspectorFact label={d.explorer.email} value={principal.email} /> : null}
          {principal.department ? <InspectorFact label={d.explorer.department} value={principal.department} /> : null}
          {principal.sid ? <InspectorFact label={d.explorer.sid} value={principal.sid} mono /> : null}
        </dl>
      ) : (
        <Skeleton className="h-16 w-full" />
      )}
      {principal?.sam_account_name ? (
        <Link href={`/access?principal=${encodeURIComponent(principal.sam_account_name)}`}>
          <Button variant="outline" size="sm" className="w-full">
            <GitFork className="h-3 w-3" /> {d.explorer.openAccess}
          </Button>
        </Link>
      ) : null}
    </div>
  );
}

function ResourceInspector({ node }: { node: ExplorerNode }) {
  const d = useDict();
  const { activeProfile } = useADConnection();
  const [rescanning, setRescanning] = useState(false);

  const rescan = useCallback(async () => {
    if (!node.path || rescanning) return;
    setRescanning(true);
    try {
      const body: Record<string, unknown> = {
        path: node.path,
        depth: -1,
        include_inherited: true,
        scan_id: `explorer-rescan-${Date.now().toString(36)}`,
      };
      if (activeProfile) body.effective_permissions = { enabled: true, connection_id: activeProfile.id };
      await fetch(`${apiBase()}/api/scan`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      window.location.reload();
    } finally {
      setRescanning(false);
    }
  }, [activeProfile, node.path, rescanning]);

  return (
    <div className="space-y-3">
      <div>
        <p className="break-all font-mono text-xs font-medium text-fg">{node.path}</p>
        {typeof node.permissionCount === 'number' ? (
          <p className="mt-1 text-2xs text-fg-muted">
            <span className="tabular-nums">{node.permissionCount.toLocaleString()}</span> {d.explorer.permissions}
          </p>
        ) : null}
      </div>
      <div className="space-y-1.5">
        <Link href={`/access?path=${encodeURIComponent(node.path || '')}`}>
          <Button variant="outline" size="sm" className="w-full">
            <GitFork className="h-3 w-3" /> {d.explorer.openAccess}
          </Button>
        </Link>
        <Button variant="ghost" size="sm" className="w-full" onClick={rescan} loading={rescanning}>
          <RefreshCcw className="h-3 w-3" /> {d.explorer.rescan}
        </Button>
      </div>
    </div>
  );
}

function InspectorFact({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <dt className="text-2xs uppercase tracking-wide text-fg-faint">{label}</dt>
      <dd className={cn('mt-0.5 break-all text-fg-secondary', mono && 'font-mono text-2xs')}>{value}</dd>
    </div>
  );
}
