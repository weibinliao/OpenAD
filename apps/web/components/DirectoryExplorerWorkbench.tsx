import React, { useCallback, useMemo, useRef, useState } from 'react';
import Link from 'next/link';
import {
  AlertTriangle,
  Boxes,
  Building2,
  ChevronDown,
  ChevronRight,
  Download,
  FileCog,
  FolderTree,
  GitFork,
  Globe,
  ListFilter,
  Loader2,
  RefreshCw,
  Search,
  UserRound,
  Users,
  X,
} from 'lucide-react';
import { apiBase } from '../lib/runtimeApi';
import { cn } from '../lib/cn';
import {
  directoryGroupName,
  groupNameFromDN,
  searchDirectoryObjects,
  type DirectoryGroup,
  type DirectoryMatch,
  type DirectoryUser,
} from '../lib/directorySearch';
import { useI18n } from '../contexts/I18nContext';
import { Badge } from './ui/badge';
import { Button } from './ui/button';
import { Card, CardContent, CardHeader } from './ui/card';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from './ui/dropdown-menu';
import { EmptyState } from './ui/empty-state';
import { Input } from './ui/input';
import { Skeleton } from './ui/skeleton';

interface TreeNode {
  dn: string;
  name: string;
  node_type: string;
  has_children: boolean;
}

interface TreeState {
  children: TreeNode[];
  expanded: boolean;
  loading: boolean;
  page: number;
  totalPages: number;
  error?: string;
}

interface Member {
  dn: string;
  name?: string;
  sam_account_name?: string;
  email?: string;
  type: string;
  depth: number;
  path?: string[];
}

interface GroupMembersData {
  group?: { members?: Member[] };
  resolution?: { members?: Member[] };
}

type SelectedObject =
  | { kind: 'user'; user: DirectoryUser }
  | { kind: 'group'; group: DirectoryGroup }
  | { kind: 'node'; node: TreeNode };

const PAGE_SIZE = 50;

const copy = {
  en: {
    treeTitle: 'AD directory tree',
    treeDescription: 'Select an object to inspect it. Use the arrow to expand containers and groups.',
    filterTree: 'Filter expanded tree nodes',
    refresh: 'Refresh',
    searchTitle: 'Directory lookup',
    searchDescription: 'Type to autocomplete users and groups from the current AD connection.',
    searchPlaceholder: 'Search account, name, email, or group',
    search: 'Search entire directory',
    users: 'User',
    groups: 'Group',
    noSearch: 'Enter at least two characters to see matching directory objects.',
    noResults: 'No directory objects matched this query.',
    loadMore: 'Load more',
    direct: 'Direct only',
    nested: 'Include nested',
    account: 'Account',
    email: 'Email',
    department: 'Department',
    division: 'Division',
    domain: 'Domain',
    type: 'Object type',
    dn: 'Distinguished name',
    depth: 'Depth',
    empty: 'No items found',
    loadedFilterHint: 'Only reorganizes the current tree view; it does not query Active Directory.',
    inspectorTitle: 'Object details',
    inspectorDescription: 'The selected tree node or search result appears here.',
    inspectorEmpty: 'Select a user, group, OU, or domain to inspect its directory context.',
    directGroups: 'Direct AD groups',
    directGroupCount: (count: number) => `${count} direct ${count === 1 ? 'group' : 'groups'}`,
    groupMembers: 'Group members',
    exportExcel: 'Export Excel',
    exportCSV: 'Export CSV',
    exportFormats: 'More export formats',
    memberCount: (count: number) => `${count} ${count === 1 ? 'member' : 'members'}`,
    analyzeAccess: 'Analyze access',
    structuralHint: 'This structural directory object organizes the tree. Expand it to inspect loaded children.',
    clearSelection: 'Clear selection',
    loadingDetails: 'Loading directory details',
  },
  'zh-CN': {
    treeTitle: 'AD 目录树',
    treeDescription: '单击对象查看详情；使用箭头展开容器和组。',
    filterTree: '筛选已展开节点',
    refresh: '刷新',
    searchTitle: '目录查找',
    searchDescription: '输入内容后，自动联想当前 AD 连接中的用户和组。',
    searchPlaceholder: '搜索账号、姓名、邮箱或组名',
    search: '搜索整个目录',
    users: '用户',
    groups: '组',
    noSearch: '请输入至少两个字符，自动显示匹配的目录对象。',
    noResults: '没有匹配的目录对象。',
    loadMore: '加载更多',
    direct: '仅直属成员',
    nested: '包含嵌套成员',
    account: '账号',
    email: '邮箱',
    department: '部门',
    division: '事业部',
    domain: '域',
    type: '对象类型',
    dn: '可分辨名称',
    depth: '深度',
    empty: '暂无数据',
    loadedFilterHint: '仅整理当前树视图，不会查询 Active Directory。',
    inspectorTitle: '对象详情',
    inspectorDescription: '这里显示树节点或搜索结果的完整目录信息。',
    inspectorEmpty: '请选择用户、组、OU 或域，查看它的目录上下文。',
    directGroups: '直属 AD 组',
    directGroupCount: (count: number) => `${count} 个直属组`,
    groupMembers: '组成员',
    memberCount: (count: number) => `${count} 个成员`,
    analyzeAccess: '分析访问权限',
    exportExcel: '导出 Excel',
    exportCSV: '导出 CSV',
    exportFormats: '更多导出格式',
    structuralHint: '这是用于组织目录树的结构对象。展开它可以继续查看已加载的子对象。',
    clearSelection: '清除选择',
    loadingDetails: '正在加载目录详情',
  },
} as const;

function iconFor(type: string) {
  if (type === 'domain') return Globe;
  if (type === 'ou') return Building2;
  if (type === 'policy') return FileCog;
  if (type === 'group') return Users;
  if (type === 'user') return UserRound;
  return Boxes;
}

function selectedDN(selected: SelectedObject | null) {
  if (!selected) return '';
  if (selected.kind === 'user') return selected.user.dn;
  if (selected.kind === 'group') return selected.group.dn;
  return selected.node.dn;
}

function exportFilename(response: Response, fallback: string) {
  const disposition = response.headers.get('Content-Disposition') || '';
  const encodedMatch = disposition.match(/filename\*=UTF-8''([^;]+)/i);
  if (encodedMatch) {
    try {
      return decodeURIComponent(encodedMatch[1].replace(/^"|"$/g, ''));
    } catch {
      // Fall through to the plain filename when the server value is malformed.
    }
  }
  const plainMatch = disposition.match(/filename="?([^";]+)"?/i);
  return plainMatch?.[1] || fallback;
}

type GroupExportFormat = 'excel' | 'csv';

export default function DirectoryExplorerWorkbench({
  connectionId,
  initialQuery = '',
}: {
  connectionId: string;
  initialQuery?: string;
}) {
  const { locale } = useI18n();
  const labels = copy[locale];
  const [root, setRoot] = useState<TreeState | null>(null);
  const [nodeStates, setNodeStates] = useState<Record<string, TreeState>>({});
  const [treeFilter, setTreeFilter] = useState('');
  const [query, setQuery] = useState('');
  const [searching, setSearching] = useState(false);
  const [searched, setSearched] = useState(false);
  const [matches, setMatches] = useState<DirectoryMatch[]>([]);
  const [searchError, setSearchError] = useState('');
  const [activeMatchIndex, setActiveMatchIndex] = useState(-1);
  const [selected, setSelected] = useState<SelectedObject | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState('');
  const [includeNested, setIncludeNested] = useState(true);
  const [groupData, setGroupData] = useState<GroupMembersData | null>(null);
  const [exporting, setExporting] = useState<GroupExportFormat | ''>('');
  const searchSequence = useRef(0);
  const detailSequence = useRef(0);
  const lastExternalSearch = useRef('');

  const fetchChildren = useCallback(async (parentDN: string, page: number) => {
    const response = await fetch(`${apiBase()}/api/ad/tree`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ connection_id: connectionId, parent_dn: parentDN, page, page_size: PAGE_SIZE }),
    });
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.error || 'Unable to load the AD tree.');
    return {
      nodes: Array.isArray(data.nodes) ? data.nodes as TreeNode[] : [],
      totalPages: data.pagination?.total_pages ?? 1,
    };
  }, [connectionId]);

  const loadRoot = useCallback(async () => {
    setRoot({ children: [], expanded: true, loading: true, page: 1, totalPages: 1 });
    setNodeStates({});
    try {
      const result = await fetchChildren('', 1);
      setRoot({ children: result.nodes, expanded: true, loading: false, page: 1, totalPages: result.totalPages });
    } catch (loadError) {
      setRoot({
        children: [],
        expanded: true,
        loading: false,
        page: 1,
        totalPages: 1,
        error: loadError instanceof Error ? loadError.message : String(loadError),
      });
    }
  }, [fetchChildren]);

  React.useEffect(() => {
    void loadRoot();
  }, [loadRoot]);

  const toggleNode = useCallback(async (node: TreeNode) => {
    const current = nodeStates[node.dn];
    if (current) {
      setNodeStates((previous) => ({
        ...previous,
        [node.dn]: { ...current, expanded: !current.expanded },
      }));
      return;
    }
    setNodeStates((previous) => ({
      ...previous,
      [node.dn]: { children: [], expanded: true, loading: true, page: 1, totalPages: 1 },
    }));
    try {
      const result = await fetchChildren(node.dn, 1);
      const children = result.nodes.filter(
        (candidate) => candidate.dn.toLowerCase() !== node.dn.toLowerCase(),
      );
      setNodeStates((previous) => ({
        ...previous,
        [node.dn]: {
          children,
          expanded: true,
          loading: false,
          page: 1,
          totalPages: result.totalPages,
        },
      }));
    } catch (loadError) {
      setNodeStates((previous) => ({
        ...previous,
        [node.dn]: {
          children: [],
          expanded: true,
          loading: false,
          page: 1,
          totalPages: 1,
          error: loadError instanceof Error ? loadError.message : String(loadError),
        },
      }));
    }
  }, [fetchChildren, nodeStates]);

  const loadMore = useCallback(async (parentDN: string) => {
    const current = parentDN ? nodeStates[parentDN] : root;
    if (!current || current.loading || current.page >= current.totalPages) return;
    const nextPage = current.page + 1;
    const result = await fetchChildren(parentDN, nextPage);
    const nextState = {
      ...current,
      children: [...current.children, ...result.nodes],
      page: nextPage,
      totalPages: result.totalPages,
      loading: false,
    };
    if (parentDN) {
      setNodeStates((previous) => ({ ...previous, [parentDN]: nextState }));
    } else {
      setRoot(nextState);
    }
  }, [fetchChildren, nodeStates, root]);

  const chooseMatch = useCallback((match: DirectoryMatch, updateQuery = true) => {
    if (match.kind === 'user') {
      setSelected({ kind: 'user', user: match.user });
    } else {
      setSelected({ kind: 'group', group: match.group });
      setIncludeNested(true);
    }
    if (updateQuery) setQuery(match.label);
    setActiveMatchIndex(-1);
    setDetailError('');
  }, []);

  const searchDirectory = useCallback(async (
    value: string,
    options: { limit?: number; chooseExact?: boolean } = {},
  ) => {
    const trimmed = value.trim();
    if (trimmed.length < 2) return;
    const sequence = ++searchSequence.current;
    setSearching(true);
    setSearchError('');
    try {
      const result = await searchDirectoryObjects({
        connectionId,
        query: trimmed,
        limit: options.limit ?? 12,
      });
      if (sequence !== searchSequence.current) return;
      setMatches(result.matches);
      setSearched(true);
      setActiveMatchIndex(result.matches.length > 0 ? 0 : -1);

      if (options.chooseExact) {
        const normalized = trimmed.toLowerCase();
        const exact = result.matches.find((match) => {
          if (match.dn.toLowerCase() === normalized || match.label.toLowerCase() === normalized) return true;
          return match.kind === 'user' && match.user.username.toLowerCase() === normalized;
        });
        if (exact) chooseMatch(exact, false);
      }
    } catch (error) {
      if (sequence !== searchSequence.current) return;
      setMatches([]);
      setSearched(true);
      setActiveMatchIndex(-1);
      setSearchError(error instanceof Error ? error.message : String(error));
    } finally {
      if (sequence === searchSequence.current) setSearching(false);
    }
  }, [chooseMatch, connectionId]);

  React.useEffect(() => {
    const trimmed = query.trim();
    if (trimmed.length < 2) {
      setMatches([]);
      setSearched(false);
      setActiveMatchIndex(-1);
      return;
    }
    const timer = window.setTimeout(() => {
      void searchDirectory(trimmed, { limit: 12 });
    }, 220);
    return () => window.clearTimeout(timer);
  }, [query, searchDirectory]);

  React.useEffect(() => {
    const trimmed = initialQuery.trim();
    const searchKey = `${connectionId}:${trimmed.toLowerCase()}`;
    if (trimmed.length < 2 || lastExternalSearch.current === searchKey) return;
    lastExternalSearch.current = searchKey;
    setQuery(trimmed);
    void searchDirectory(trimmed, { limit: 25, chooseExact: true });
  }, [connectionId, initialQuery, searchDirectory]);

  React.useEffect(() => {
    if (selected?.kind !== 'group') {
      setGroupData(null);
      return;
    }
    const sequence = ++detailSequence.current;
    setDetailLoading(true);
    setDetailError('');
    fetch(`${apiBase()}/api/ad/groups/members`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        connection_id: connectionId,
        group_dn: selected.group.dn,
        include_nested: includeNested,
        max_depth: 10,
      }),
    }).then(async (response) => {
      const data = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(data.error || 'Unable to load group members.');
      if (sequence === detailSequence.current) setGroupData(data);
    }).catch((error) => {
      if (sequence !== detailSequence.current) return;
      setGroupData(null);
      setDetailError(error instanceof Error ? error.message : String(error));
    }).finally(() => {
      if (sequence === detailSequence.current) setDetailLoading(false);
    });
  }, [connectionId, includeNested, selected]);

  const selectTreeNode = useCallback(async (node: TreeNode) => {
    setSelected({ kind: 'node', node });
    setDetailError('');
    setGroupData(null);
    if (node.node_type === 'group') {
      setSelected({ kind: 'group', group: { dn: node.dn, name: node.name } });
      setIncludeNested(true);
      return;
    }
    if (node.node_type !== 'user') return;

    const sequence = ++detailSequence.current;
    setDetailLoading(true);
    try {
      const result = await searchDirectoryObjects({
        connectionId,
        query: node.name,
        limit: 25,
      });
      if (sequence !== detailSequence.current) return;
      const user = result.users.find((candidate) => candidate.dn.toLowerCase() === node.dn.toLowerCase())
        || result.users[0];
      if (user) setSelected({ kind: 'user', user });
    } catch (error) {
      if (sequence !== detailSequence.current) return;
      setDetailError(error instanceof Error ? error.message : String(error));
    } finally {
      if (sequence === detailSequence.current) setDetailLoading(false);
    }
  }, [connectionId]);

  const normalizedFilter = treeFilter.trim().toLowerCase();
  const hasLoadedMatch = useCallback((node: TreeNode): boolean => {
    const ownMatch = `${node.name} ${node.dn} ${node.node_type}`.toLowerCase().includes(normalizedFilter);
    if (!normalizedFilter || ownMatch) return true;
    return (nodeStates[node.dn]?.children || []).some(hasLoadedMatch);
  }, [nodeStates, normalizedFilter]);

  const currentDN = selectedDN(selected);
  const renderNodes = (nodes: TreeNode[], depth: number): React.ReactNode => nodes
    .filter(hasLoadedMatch)
    .map((node) => {
      const state = nodeStates[node.dn];
      const Icon = iconFor(node.node_type);
      const isSelected = currentDN.toLowerCase() === node.dn.toLowerCase();
      return (
        <div key={node.dn}>
          <button
            type="button"
            onClick={() => {
              void selectTreeNode(node);
              if (node.has_children) void toggleNode(node);
            }}
            className={cn(
              'flex min-h-8 w-full items-center gap-1.5 px-2 py-1 text-left text-xs transition-colors hover:bg-surface-sunken',
              isSelected && 'bg-accent-soft text-fg',
            )}
            style={{ paddingLeft: `${depth * 16 + 8}px` }}
            title={node.dn}
            aria-pressed={isSelected}
          >
            {node.has_children
              ? state?.loading
                ? <Loader2 className="h-3 w-3 shrink-0 animate-spin text-fg-muted" />
                : state?.expanded
                  ? <ChevronDown className="h-3 w-3 shrink-0 text-fg-muted" />
                  : <ChevronRight className="h-3 w-3 shrink-0 text-fg-muted" />
              : <span className="w-3 shrink-0" />}
            <Icon className="h-3.5 w-3.5 shrink-0 text-fg-muted" />
            <span className="truncate text-fg-secondary">{node.name}</span>
            <Badge tone="neutral" className="ml-auto shrink-0">{node.node_type}</Badge>
          </button>
          {state?.expanded ? (
            <div>
              {state.error ? <p className="px-3 py-1 text-2xs text-danger">{state.error}</p> : null}
              {renderNodes(state.children, depth + 1)}
              {state.page < state.totalPages ? (
                <button
                  type="button"
                  className="px-3 py-1 text-2xs font-medium text-accent-fg"
                  onClick={() => loadMore(node.dn)}
                >
                  {labels.loadMore}
                </button>
              ) : null}
            </div>
          ) : null}
        </div>
      );
    });

  const handleSearchKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'ArrowDown' && matches.length > 0) {
      event.preventDefault();
      setActiveMatchIndex((current) => (current + 1) % matches.length);
      return;
    }
    if (event.key === 'ArrowUp' && matches.length > 0) {
      event.preventDefault();
      setActiveMatchIndex((current) => (current <= 0 ? matches.length - 1 : current - 1));
      return;
    }
    if (event.key === 'Escape') {
      setActiveMatchIndex(-1);
      return;
    }
    if (event.key === 'Enter') {
      event.preventDefault();
      const activeMatch = activeMatchIndex >= 0 ? matches[activeMatchIndex] : null;
      if (activeMatch) chooseMatch(activeMatch);
      else void searchDirectory(query, { limit: 50, chooseExact: true });
    }
  };

  const members = useMemo(() => {
    const source = includeNested ? groupData?.resolution?.members : groupData?.group?.members;
    return Array.isArray(source)
      ? source.map((member) => ({ ...member, depth: member.depth ?? 0 }))
      : [];
  }, [groupData, includeNested]);

  const exportGroupMembers = useCallback(async (format: GroupExportFormat) => {
    if (selected?.kind !== 'group' || exporting) return;
    setExporting(format);
    setDetailError('');
    try {
      const response = await fetch(`${apiBase()}/api/ad/groups/members/export`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          connection_id: connectionId,
          group_dn: selected.group.dn,
          include_nested: includeNested,
          max_depth: 10,
          format,
        }),
      });
      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.error || 'Unable to export group members.');
      }

      const blob = await response.blob();
      const objectURL = URL.createObjectURL(blob);
      try {
        const anchor = document.createElement('a');
        anchor.href = objectURL;
        anchor.download = exportFilename(
          response,
          `${directoryGroupName(selected.group)}-members.${format === 'csv' ? 'csv' : 'xlsx'}`,
        );
        document.body.appendChild(anchor);
        anchor.click();
        anchor.remove();
      } finally {
        URL.revokeObjectURL(objectURL);
      }
    } catch (exportError) {
      setDetailError(exportError instanceof Error ? exportError.message : String(exportError));
    } finally {
      setExporting('');
    }
  }, [connectionId, exporting, includeNested, selected]);

  return (
    <Card className="overflow-hidden">
      <div className="grid min-h-[600px] lg:grid-cols-[minmax(300px,0.9fr)_minmax(420px,1.1fr)] xl:grid-cols-[minmax(280px,0.78fr)_minmax(300px,0.86fr)_minmax(360px,1.08fr)]">
        <section className="border-b border-line lg:border-b-0 lg:border-r" aria-label={labels.treeTitle}>
          <CardHeader
            title={labels.treeTitle}
            description={labels.treeDescription}
            actions={(
              <Button variant="outline" size="icon" onClick={loadRoot} title={labels.refresh} aria-label={labels.refresh}>
                <RefreshCw className="h-3.5 w-3.5" />
              </Button>
            )}
          />
          <CardContent className="pt-0">
            <div className="relative">
              <ListFilter className="pointer-events-none absolute left-2.5 top-2.5 h-3.5 w-3.5 text-fg-muted" />
              <Input
                value={treeFilter}
                onChange={(event) => setTreeFilter(event.target.value)}
                aria-label={labels.filterTree}
                placeholder={labels.filterTree}
                className="pl-8"
              />
            </div>
            <p className="mt-1.5 text-2xs text-fg-muted">{labels.loadedFilterHint}</p>
            <div className="mt-3 max-h-[470px] overflow-auto rounded-md border border-line bg-surface-base py-1">
              {root?.loading && root.children.length === 0 ? (
                <div className="space-y-2 p-3">
                  <Skeleton className="h-6 w-2/3" />
                  <Skeleton className="h-6 w-1/2" />
                </div>
              ) : null}
              {root?.error ? (
                <p className="flex gap-2 p-3 text-xs text-danger">
                  <AlertTriangle className="h-4 w-4" />
                  {root.error}
                </p>
              ) : null}
              {root && !root.loading && root.children.length === 0 ? (
                <EmptyState icon={FolderTree} title={labels.empty} className="py-10" />
              ) : null}
              {root ? renderNodes(root.children, 0) : null}
              {root && root.page < root.totalPages ? (
                <button
                  type="button"
                  className="px-3 py-1 text-2xs font-medium text-accent-fg"
                  onClick={() => loadMore('')}
                >
                  {labels.loadMore}
                </button>
              ) : null}
            </div>
          </CardContent>
        </section>

        <section className="border-b border-line lg:border-b-0 xl:border-r" aria-label={labels.searchTitle}>
          <CardHeader title={labels.searchTitle} description={labels.searchDescription} />
          <CardContent className="pt-0">
            <div className="flex gap-2">
              <Input
                value={query}
                role="combobox"
                aria-expanded={matches.length > 0}
                aria-controls="directory-search-options"
                aria-autocomplete="list"
                aria-activedescendant={activeMatchIndex >= 0 ? `directory-search-option-${activeMatchIndex}` : undefined}
                onChange={(event) => {
                  setQuery(event.target.value);
                  setSelected(null);
                  setActiveMatchIndex(-1);
                }}
                onKeyDown={handleSearchKeyDown}
                aria-label={labels.searchPlaceholder}
                placeholder={labels.searchPlaceholder}
              />
              <Button
                className="shrink-0"
                size="icon"
                onClick={() => void searchDirectory(query, { limit: 50, chooseExact: true })}
                loading={searching}
                disabled={query.trim().length < 2}
                aria-label={labels.search}
                title={labels.search}
              >
                <Search className="h-3.5 w-3.5" />
              </Button>
            </div>

            {searchError ? (
              <p className="mt-3 flex gap-2 rounded-md border border-danger/30 bg-danger-soft p-3 text-xs text-danger">
                <AlertTriangle className="h-4 w-4" />
                {searchError}
              </p>
            ) : null}
            {searching && matches.length === 0 ? (
              <div className="mt-4 space-y-2">
                <Skeleton className="h-12 w-full" />
                <Skeleton className="h-12 w-full" />
              </div>
            ) : null}
            {!searched && !searching ? <EmptyState icon={Search} title={labels.noSearch} className="py-16" /> : null}
            {searched && matches.length === 0 && !searching ? (
              <EmptyState icon={Search} title={labels.noResults} className="py-16" />
            ) : null}
            {matches.length > 0 ? (
              <div
                id="directory-search-options"
                role="listbox"
                aria-label={labels.searchTitle}
                className="mt-4 max-h-[450px] space-y-1 overflow-auto"
              >
                {matches.map((match, index) => {
                  const MatchIcon = match.kind === 'user' ? UserRound : Users;
                  const groupCount = match.kind === 'user' ? match.user.groups?.length || 0 : null;
                  const isSelected = currentDN.toLowerCase() === match.dn.toLowerCase();
                  return (
                    <button
                      key={`${match.kind}-${match.dn}`}
                      id={`directory-search-option-${index}`}
                      type="button"
                      role="option"
                      aria-selected={isSelected}
                      aria-label={`${match.label} ${match.secondary}`}
                      onMouseEnter={() => setActiveMatchIndex(index)}
                      onClick={() => chooseMatch(match)}
                      className={cn(
                        'grid w-full grid-cols-[20px_minmax(0,1fr)_auto] items-center gap-2 rounded-md border border-transparent px-2.5 py-2 text-left transition-colors hover:bg-surface-sunken',
                        (isSelected || activeMatchIndex === index) && 'border-accent/30 bg-accent-soft',
                      )}
                    >
                      <MatchIcon className="h-4 w-4 text-fg-muted" aria-hidden />
                      <span className="min-w-0">
                        <strong className="block truncate text-xs font-medium text-fg">{match.label}</strong>
                        <small className="block truncate text-2xs text-fg-muted">
                          {match.secondary || match.dn}
                          {groupCount !== null ? ` · ${labels.directGroupCount(groupCount)}` : ''}
                        </small>
                        {match.kind === 'user' ? (
                          <small className="block truncate font-mono text-2xs text-fg-faint" title={match.dn}>
                            {match.dn}
                          </small>
                        ) : null}
                      </span>
                      <Badge tone={match.kind === 'user' ? 'accent' : 'neutral'}>
                        {match.kind === 'user' ? labels.users : labels.groups}
                      </Badge>
                    </button>
                  );
                })}
              </div>
            ) : null}
          </CardContent>
        </section>

        <section className="border-t border-line lg:col-span-2 xl:col-span-1 xl:border-t-0" aria-label={labels.inspectorTitle}>
          <CardHeader
            title={labels.inspectorTitle}
            description={labels.inspectorDescription}
            actions={selected ? (
              <Button
                variant="ghost"
                size="icon"
                aria-label={labels.clearSelection}
                title={labels.clearSelection}
                onClick={() => setSelected(null)}
              >
                <X className="h-3.5 w-3.5" />
              </Button>
            ) : undefined}
          />
          <CardContent className="pt-0">
            {detailError ? (
              <p className="mb-3 flex gap-2 rounded-md border border-danger/30 bg-danger-soft p-3 text-xs text-danger">
                <AlertTriangle className="h-4 w-4" />
                {detailError}
              </p>
            ) : null}
            {detailLoading && selected?.kind !== 'group' ? (
              <div aria-label={labels.loadingDetails} className="space-y-3">
                <Skeleton className="h-16 w-full" />
                <Skeleton className="h-28 w-full" />
              </div>
            ) : selected?.kind === 'user' ? (
              <UserInspector labels={labels} user={selected.user} onSelectGroup={(group) => {
                setSelected({ kind: 'group', group });
                setIncludeNested(true);
              }} />
            ) : selected?.kind === 'group' ? (
              <GroupInspector
                labels={labels}
                group={selected.group}
                members={members}
                loading={detailLoading}
                includeNested={includeNested}
                exporting={exporting}
                onIncludeNestedChange={setIncludeNested}
                onExport={exportGroupMembers}
              />
            ) : selected?.kind === 'node' ? (
              <NodeInspector labels={labels} node={selected.node} />
            ) : (
              <EmptyState icon={UserRound} title={labels.inspectorEmpty} className="py-20" />
            )}
          </CardContent>
        </section>
      </div>
    </Card>
  );
}

function DetailField({ label, children, mono = false }: { label: string; children?: React.ReactNode; mono?: boolean }) {
  return (
    <div className="min-w-0">
      <dt className="text-2xs font-medium uppercase text-fg-muted">{label}</dt>
      <dd className={cn('mt-1 break-words text-xs text-fg-secondary', mono && 'font-mono text-2xs')}>
        {children || '—'}
      </dd>
    </div>
  );
}

function UserInspector({
  labels,
  user,
  onSelectGroup,
}: {
  labels: typeof copy.en | typeof copy['zh-CN'];
  user: DirectoryUser;
  onSelectGroup: (group: DirectoryGroup) => void;
}) {
  const groups = user.groups || [];
  return (
    <div className="space-y-5">
      <div className="flex items-start gap-3">
        <span className="grid h-10 w-10 shrink-0 place-items-center rounded-md border border-accent/30 bg-accent-soft text-accent-fg">
          <UserRound className="h-5 w-5" aria-hidden />
        </span>
        <div className="min-w-0 flex-1">
          <h3 className="truncate text-sm font-semibold text-fg">{user.display_name || user.username}</h3>
          <p className="mt-0.5 truncate font-mono text-2xs text-fg-muted">{user.username}</p>
        </div>
        {user.username ? (
          <Link href={`/access?principal=${encodeURIComponent(user.username)}`}>
            <Button variant="outline" size="sm">
              <GitFork className="h-3.5 w-3.5" />
              {labels.analyzeAccess}
            </Button>
          </Link>
        ) : null}
      </div>

      <dl className="grid grid-cols-1 gap-4 rounded-md border border-line bg-surface-base p-3 sm:grid-cols-2">
        <DetailField label={labels.account}>{user.username}</DetailField>
        <DetailField label={labels.email}>{user.email}</DetailField>
        <DetailField label={labels.department}>{user.department}</DetailField>
        <DetailField label={labels.division}>{user.division}</DetailField>
        <DetailField label={labels.domain}>{user.domain}</DetailField>
        <DetailField label={labels.type}>user</DetailField>
        <div className="sm:col-span-2">
          <DetailField label={labels.dn} mono>{user.dn}</DetailField>
        </div>
      </dl>

      <section>
        <div className="mb-2 flex items-center justify-between gap-2">
          <h4 className="text-xs font-semibold text-fg">{labels.directGroups}</h4>
          <Badge tone="neutral">{labels.directGroupCount(groups.length)}</Badge>
        </div>
        {groups.length === 0 ? (
          <p className="rounded-md border border-line bg-surface-base px-3 py-4 text-xs text-fg-muted">{labels.empty}</p>
        ) : (
          <div className="max-h-[230px] overflow-auto rounded-md border border-line bg-surface-base p-1">
            {groups.map((groupDN) => (
              <button
                key={groupDN}
                type="button"
                className="flex w-full items-center gap-2 rounded px-2 py-2 text-left hover:bg-surface-sunken"
                onClick={() => onSelectGroup({ dn: groupDN, name: groupNameFromDN(groupDN) })}
              >
                <Users className="h-3.5 w-3.5 shrink-0 text-fg-muted" aria-hidden />
                <span className="min-w-0">
                  <strong className="block truncate text-xs font-medium text-fg">{groupNameFromDN(groupDN)}</strong>
                  <small className="block truncate font-mono text-2xs text-fg-muted" title={groupDN}>{groupDN}</small>
                </span>
                <ChevronRight className="ml-auto h-3.5 w-3.5 shrink-0 text-fg-muted" aria-hidden />
              </button>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}

function GroupInspector({
  labels,
  group,
  members,
  loading,
  includeNested,
  exporting,
  onIncludeNestedChange,
  onExport,
}: {
  labels: typeof copy.en | typeof copy['zh-CN'];
  group: DirectoryGroup;
  members: Member[];
  loading: boolean;
  includeNested: boolean;
  exporting: GroupExportFormat | '';
  onIncludeNestedChange: (value: boolean) => void;
  onExport: (format: GroupExportFormat) => Promise<void>;
}) {
  return (
    <div className="space-y-5">
      <div className="flex items-start gap-3">
        <span className="grid h-10 w-10 shrink-0 place-items-center rounded-md border border-line bg-surface-sunken text-fg-secondary">
          <Users className="h-5 w-5" aria-hidden />
        </span>
        <div className="min-w-0 flex-1">
          <h3 className="truncate text-sm font-semibold text-fg">{directoryGroupName(group)}</h3>
          <p className="mt-0.5 truncate font-mono text-2xs text-fg-muted" title={group.dn}>{group.dn}</p>
        </div>
      </div>

      <dl className="rounded-md border border-line bg-surface-base p-3">
        <DetailField label={labels.dn} mono>{group.dn}</DetailField>
      </dl>

      <section>
        <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <h4 className="text-xs font-semibold text-fg">{labels.groupMembers}</h4>
            <Badge tone="neutral">{labels.memberCount(members.length)}</Badge>
          </div>
          <div className="flex flex-wrap items-center justify-end gap-2">
            <div className="inline-flex rounded-md bg-surface-sunken p-1">
              <button
                type="button"
                className={cn(
                  'rounded px-2.5 py-1 text-2xs text-fg-muted',
                  !includeNested && 'bg-surface-base text-fg shadow-token-sm',
                )}
                onClick={() => onIncludeNestedChange(false)}
              >
                {labels.direct}
              </button>
              <button
                type="button"
                className={cn(
                  'rounded px-2.5 py-1 text-2xs text-fg-muted',
                  includeNested && 'bg-surface-base text-fg shadow-token-sm',
                )}
                onClick={() => onIncludeNestedChange(true)}
              >
                {labels.nested}
              </button>
            </div>
            <div className="inline-flex">
              <Button
                type="button"
                size="sm"
                className="rounded-r-none"
                loading={exporting === 'excel'}
                disabled={loading || Boolean(exporting)}
                onClick={() => void onExport('excel')}
              >
                <Download className="h-3.5 w-3.5" aria-hidden />
                {labels.exportExcel}
              </Button>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button type="button" size="sm" className="rounded-l-none border-l border-accent-hover px-2" disabled={loading || Boolean(exporting)} aria-label={labels.exportFormats} title={labels.exportFormats}>
                    <ChevronDown className="h-3.5 w-3.5" aria-hidden />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem disabled={loading || Boolean(exporting)} onSelect={() => void onExport('csv')}>
                    <Download className="h-3.5 w-3.5" aria-hidden />
                    {labels.exportCSV}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>
        </div>
        {loading ? (
          <div className="space-y-2">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : members.length === 0 ? (
          <p className="rounded-md border border-line bg-surface-base px-3 py-4 text-xs text-fg-muted">{labels.empty}</p>
        ) : (
          <div className="max-h-[310px] overflow-auto rounded-md border border-line bg-surface-base">
            {members.map((member) => (
              <div
                key={member.dn}
                className="grid grid-cols-[20px_minmax(0,1fr)_auto] items-center gap-2 border-b border-line px-3 py-2.5 last:border-b-0"
              >
                {member.type === 'group'
                  ? <Users className="h-3.5 w-3.5 text-fg-muted" aria-hidden />
                  : <UserRound className="h-3.5 w-3.5 text-fg-muted" aria-hidden />}
                <div className="min-w-0">
                  <p className="truncate text-xs font-medium text-fg">
                    {member.name || member.sam_account_name || groupNameFromDN(member.dn)}
                  </p>
                  <p className="truncate font-mono text-2xs text-fg-muted" title={member.dn}>
                    {member.sam_account_name || member.email || member.dn}
                  </p>
                </div>
                {includeNested ? (
                  <Badge tone="neutral">{labels.depth} {member.depth}</Badge>
                ) : null}
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}

function NodeInspector({
  labels,
  node,
}: {
  labels: typeof copy.en | typeof copy['zh-CN'];
  node: TreeNode;
}) {
  const Icon = iconFor(node.node_type);
  return (
    <div className="space-y-5">
      <div className="flex items-start gap-3">
        <span className="grid h-10 w-10 shrink-0 place-items-center rounded-md border border-line bg-surface-sunken text-fg-secondary">
          <Icon className="h-5 w-5" aria-hidden />
        </span>
        <div className="min-w-0">
          <h3 className="truncate text-sm font-semibold text-fg">{node.name}</h3>
          <Badge tone="neutral" className="mt-1">{node.node_type}</Badge>
        </div>
      </div>
      <dl className="space-y-4 rounded-md border border-line bg-surface-base p-3">
        <DetailField label={labels.type}>{node.node_type}</DetailField>
        <DetailField label={labels.dn} mono>{node.dn}</DetailField>
      </dl>
      <p className="rounded-md border border-info/30 bg-info-soft px-3 py-2.5 text-xs text-info">
        {labels.structuralHint}
      </p>
    </div>
  );
}
