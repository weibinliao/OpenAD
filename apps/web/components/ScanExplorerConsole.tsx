import { useRouter } from 'next/router';
import { useEffect, useMemo, useState } from 'react';
import {
  Activity,
  AlertTriangle,
  ChevronRight,
  Folder,
  FolderOpen,
  HardDrive,
  Loader2,
  Network,
  Play,
  RefreshCw,
  Server,
  Square,
} from 'lucide-react';
import { useADConnection } from '../contexts/ADConnectionContext';
import { apiBase } from '../lib/runtimeApi';
import { cn } from '../lib/cn';
import { Card, CardContent } from './ui/card';
import { Badge, type BadgeTone } from './ui/badge';
import { Button } from './ui/button';
import { Input, NativeSelect } from './ui/input';
import { EmptyState } from './ui/empty-state';

interface DirectoryItem {
  name: string;
  path: string;
}

interface DirectoryResponse {
  path?: string;
  parent?: string;
  items?: DirectoryItem[];
  error?: string;
}

type NodeKind = 'drive' | 'share' | 'folder';

interface TreeNode {
  path: string;
  name: string;
  parentPath: string;
  kind: NodeKind;
  loaded: boolean;
  loading: boolean;
  expanded: boolean;
  childPaths: string[];
}

interface ScanExplorerConsoleProps {
  locale: string;
  path: string;
  depth: number;
  includeInherited: boolean;
  loading: boolean;
  cancelPending?: boolean;
  adReady: boolean;
  itemsScanned: number;
  permissionCount: number;
  resultCount: number;
  skippedCount: number;
  sessionID: string;
  progressStatus: string;
  progressCurrentPath: string;
  progressStartedAt: number;
  progressLastUpdatedAt: number;
  progressLiveConnected: boolean;
  progressTrackedPath: string;
  message: string;
  error: string;
  onPathChange: (value: string) => void;
  onDepthChange: (value: number) => void;
  onIncludeInheritedChange: (value: boolean) => void;
  onScan: (pathOverride?: string) => void;
  onCancelScan?: () => void;
}

interface RuntimeIdentitySnapshot {
  account_name?: string;
  username?: string;
  domain?: string;
  host?: string;
  goos?: string;
  note?: string;
}

function text(locale: string, en: string, zh: string) {
  return locale === 'zh-CN' ? zh : en;
}

function normalizePath(input: string) {
  return input.trim().replace(/\//g, '\\');
}

function isUNCPath(path: string) {
  return normalizePath(path).startsWith('\\\\');
}

function formatNumber(value: number) {
  return new Intl.NumberFormat().format(value);
}

function formatDuration(ms: number) {
  if (!ms || ms < 0) {
    return '00:00';
  }

  const totalSeconds = Math.floor(ms / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (hours > 0) {
    return `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`;
  }

  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`;
}

function splitPathSegments(path: string) {
  const normalized = normalizePath(path);
  if (!normalized) {
    return [];
  }

  if (isUNCPath(normalized)) {
    const trimmed = normalized.replace(/^\\\\+/, '');
    const parts = trimmed.split('\\').filter(Boolean);
    if (parts.length < 2) {
      return [];
    }

    const segments = [`\\\\${parts[0]}\\${parts[1]}`];
    let current = segments[0];
    for (const part of parts.slice(2)) {
      current = `${current}\\${part}`;
      segments.push(current);
    }
    return segments;
  }

  const match = normalized.match(/^[A-Za-z]:\\/);
  if (!match) {
    return [];
  }

  const root = match[0];
  const remainder = normalized.slice(root.length).split('\\').filter(Boolean);
  const segments = [root];
  let current = root.endsWith('\\') ? root.slice(0, -1) : root;
  for (const part of remainder) {
    current = `${current}\\${part}`;
    segments.push(current);
  }
  return segments;
}

function displayNameForPath(path: string) {
  const normalized = normalizePath(path);
  if (!normalized) {
    return '';
  }

  if (isUNCPath(normalized)) {
    const segments = normalized.replace(/^\\\\+/, '').split('\\').filter(Boolean);
    if (segments.length <= 2) {
      return `\\\\${segments.join('\\')}`;
    }
    return segments[segments.length - 1];
  }

  const trimmed = normalized.endsWith('\\') ? normalized.slice(0, -1) : normalized;
  const parts = trimmed.split('\\').filter(Boolean);
  return parts[parts.length - 1] || normalized;
}

function depthLabel(locale: string, depth: number) {
  if (depth < 0) {
    return text(locale, 'All levels', '全部层级');
  }

  return locale === 'zh-CN' ? `${depth} 层` : `${depth} level${depth === 1 ? '' : 's'}`;
}

function isPathWithinScope(path: string, scope: string) {
  const normalizedPath = normalizePath(path);
  const normalizedScope = normalizePath(scope);

  if (!normalizedPath || !normalizedScope) {
    return false;
  }

  if (normalizedPath === normalizedScope) {
    return true;
  }

  const prefix = normalizedScope.endsWith('\\') ? normalizedScope : `${normalizedScope}\\`;
  return normalizedPath.startsWith(prefix);
}

function kindForPath(path: string, parentPath: string): NodeKind {
  const normalized = normalizePath(path);
  if (/^[A-Za-z]:\\$/.test(normalized)) {
    return 'drive';
  }
  if (isUNCPath(normalized) && !parentPath) {
    return 'share';
  }
  return 'folder';
}

function breadcrumbItems(path: string) {
  return splitPathSegments(path).map((segment) => ({
    path: segment,
    label: displayNameForPath(segment),
  }));
}

function nodeIcon(kind: NodeKind, expanded: boolean) {
  if (kind === 'drive') {
    return <HardDrive className="h-4 w-4 text-info" />;
  }
  if (kind === 'share') {
    return <Network className="h-4 w-4 text-accent-fg" />;
  }
  return expanded ? <FolderOpen className="h-4 w-4 text-warning" /> : <Folder className="h-4 w-4 text-warning" />;
}

function progressTone(status: string, loading: boolean): BadgeTone {
  if (status === 'failed') {
    return 'danger';
  }
  if (status === 'cancelled') {
    return 'warning';
  }
  if (status === 'completed') {
    return 'success';
  }
  if (status === 'expanding' || status === 'running' || status === 'connecting' || loading) {
    return 'info';
  }
  return 'neutral';
}

function isWindowsCredentialError(message: string) {
  const normalized = message.toLowerCase();
  return [
    'user name or password is incorrect',
    'specified network password is not correct',
    'logon failure',
    'unknown user name or bad password',
    'bad username or password',
    'the network password is not correct',
    '用户名或密码不正确',
    '网络密码不正确',
    '登录失败',
    '未知的用户名或错误的密码',
  ].some((needle) => normalized.includes(needle.toLowerCase()));
}

function friendlyDirectoryError(locale: string, targetPath: string, rawError: string) {
  const message = rawError.trim() || text(locale, 'Failed to load directories.', '加载目录失败。');

  if (isUNCPath(targetPath)) {
    if (isWindowsCredentialError(message)) {
      return text(
        locale,
        `Windows rejected the account used by the backend while opening ${targetPath}. UNC browsing and scanning use the Windows identity running OpenAD; AD is only used to resolve names and groups. Raw error: ${message}`,
        `后端在打开 ${targetPath} 时被 Windows 拒绝了账号。UNC 浏览和扫描使用的是运行 OpenAD 的 Windows 身份；AD 只用于解析名称和组。原始错误：${message}`
      );
    }

    return text(
      locale,
      `UNC browsing failed for ${targetPath}. UNC browsing and scanning use the backend Windows identity. Raw error: ${message}`,
      `UNC 浏览 ${targetPath} 失败。UNC 浏览和扫描使用后端的 Windows 身份。原始错误：${message}`
    );
  }

  if (isWindowsCredentialError(message)) {
    return text(
      locale,
      `Windows denied access while reading ${targetPath}. Check that the account running OpenAD can read this folder. Raw error: ${message}`,
      `读取 ${targetPath} 时 Windows 拒绝了访问。请确认运行 OpenAD 的账号有读取这个文件夹的权限。原始错误：${message}`
    );
  }

  return message;
}

const captionClass = 'text-2xs font-medium uppercase tracking-wide text-fg-muted';

export default function ScanExplorerConsole({
  locale,
  path,
  depth,
  includeInherited,
  loading,
  cancelPending = false,
  adReady,
  itemsScanned,
  permissionCount,
  resultCount,
  skippedCount,
  sessionID,
  progressStatus,
  progressCurrentPath,
  progressStartedAt,
  progressLastUpdatedAt,
  progressLiveConnected,
  progressTrackedPath,
  message,
  error,
  onPathChange,
  onDepthChange,
  onIncludeInheritedChange,
  onScan,
  onCancelScan,
}: ScanExplorerConsoleProps) {
  const router = useRouter();
  const { config, connection } = useADConnection();
  const [pathInput, setPathInput] = useState(path);
  const [nodes, setNodes] = useState<Record<string, TreeNode>>({});
  const [rootPaths, setRootPaths] = useState<string[]>([]);
  const [rootsLoaded, setRootsLoaded] = useState(false);
  const [treeLoading, setTreeLoading] = useState(false);
  const [treeError, setTreeError] = useState('');
  const [treeErrorTarget, setTreeErrorTarget] = useState('');
  const [elapsedLabel, setElapsedLabel] = useState('00:00');
  const [runtimeIdentity, setRuntimeIdentity] = useState<RuntimeIdentitySnapshot | null>(null);
  const [runtimeIdentityError, setRuntimeIdentityError] = useState('');

  useEffect(() => {
    setPathInput(path);
  }, [path]);

  const selectedPath = normalizePath(path);
  const selectedTrail = useMemo(() => breadcrumbItems(selectedPath), [selectedPath]);
  const showADPanel = isUNCPath(selectedPath);
  const stagedPath = normalizePath(pathInput || selectedPath);
  const scanScopeActive = isPathWithinScope(selectedPath, progressTrackedPath);
  const hasProgressSnapshot =
    scanScopeActive &&
    (loading ||
      progressStatus === 'expanding' ||
      progressStatus === 'cancelled' ||
      progressStatus === 'completed' ||
      progressStatus === 'failed' ||
      progressStartedAt > 0 ||
      progressLastUpdatedAt > 0 ||
      itemsScanned > 0 ||
      permissionCount > 0 ||
      skippedCount > 0 ||
      Boolean(sessionID));
  const livePhaseLabel =
    progressStatus === 'expanding'
      ? text(locale, 'Resolving Domain Principals', '正在展开域主体')
      : progressStatus === 'cancelled'
        ? text(locale, 'Scan Cancelled', '扫描已停止')
      : progressStatus === 'completed'
        ? text(locale, 'Scan Completed', '扫描完成')
        : progressStatus === 'failed'
          ? text(locale, 'Scan Failed', '扫描失败')
          : loading
            ? text(locale, 'Scanning Directory Structure', '正在扫描目录结构')
            : text(locale, 'Ready to Scan', '等待开始扫描');
  const canCancelScan = Boolean(
    onCancelScan && (loading || progressStatus === 'running' || progressStatus === 'expanding' || progressStatus === 'connecting')
  );
  const permissionRowTotal = permissionCount || resultCount;
  const progressFacts = [
    { label: text(locale, 'Directories Scanned', '已扫目录'), value: formatNumber(itemsScanned) },
    { label: text(locale, 'Permission Rows', '权限记录'), value: formatNumber(permissionRowTotal) },
    { label: text(locale, 'Skipped Paths', '跳过路径'), value: formatNumber(skippedCount) },
  ];
  const summaryHeadline = hasProgressSnapshot ? livePhaseLabel : text(locale, 'Runtime idle', '运行待命');

  useEffect(() => {
    if (!showADPanel) {
      setRuntimeIdentity(null);
      setRuntimeIdentityError('');
      return;
    }

    let active = true;
    const loadRuntimeIdentity = async () => {
      try {
        const response = await fetch(`${apiBase()}/api/system/runtime-identity`);
        const data = (await response.json()) as RuntimeIdentitySnapshot & { error?: string };
        if (!active) {
          return;
        }
        if (!response.ok) {
          throw new Error(data.error || text(locale, 'Failed to load backend runtime identity.', '加载后端运行身份失败。'));
        }
        setRuntimeIdentity(data);
        setRuntimeIdentityError('');
      } catch (error) {
        if (!active) {
          return;
        }
        setRuntimeIdentity(null);
        setRuntimeIdentityError(error instanceof Error ? error.message : text(locale, 'Failed to load backend runtime identity.', '加载后端运行身份失败。'));
      }
    };

    void loadRuntimeIdentity();
    return () => {
      active = false;
    };
  }, [locale, showADPanel]);

  useEffect(() => {
    if (!progressStartedAt || progressStatus === 'idle') {
      setElapsedLabel('00:00');
      return;
    }

    const updateElapsed = () => setElapsedLabel(formatDuration(Date.now() - progressStartedAt));
    updateElapsed();
    if (progressStatus === 'completed' || progressStatus === 'failed' || progressStatus === 'cancelled') {
      return;
    }
    const timer = window.setInterval(updateElapsed, 1000);
    return () => window.clearInterval(timer);
  }, [progressStartedAt, progressStatus]);

  const fetchDirectories = async (targetPath: string) => {
    const normalized = normalizePath(targetPath);
    const query = normalized ? `?path=${encodeURIComponent(normalized)}` : '';
    const response = await fetch(`${apiBase()}/api/fs/directories${query}`);
    const data = (await response.json()) as DirectoryResponse;
    if (!response.ok) {
      throw new Error(data.error || text(locale, 'Failed to load directories.', '加载目录失败。'));
    }
    return data;
  };

  const ensureVirtualNode = (targetPath: string) => {
    const normalized = normalizePath(targetPath);
    if (!normalized) {
      return;
    }

    setNodes((current) => {
      if (current[normalized]) {
        return current;
      }
      return {
        ...current,
        [normalized]: {
          path: normalized,
          name: displayNameForPath(normalized),
          parentPath: '',
          kind: kindForPath(normalized, ''),
          loaded: false,
          loading: false,
          expanded: true,
          childPaths: [],
        },
      };
    });

    setRootPaths((current) => (current.includes(normalized) ? current : [...current, normalized]));
  };

  const hydrateDirectory = (parentPath: string, items: DirectoryItem[]) => {
    const normalizedParent = normalizePath(parentPath);
    const childPaths = items.map((item) => normalizePath(item.path));

    setNodes((current) => {
      const next = { ...current };

      if (normalizedParent) {
        const existingParent = next[normalizedParent];
        next[normalizedParent] = {
          path: normalizedParent,
          name: existingParent?.name || displayNameForPath(normalizedParent),
          parentPath: existingParent?.parentPath || '',
          kind: existingParent?.kind || kindForPath(normalizedParent, ''),
          loaded: true,
          loading: false,
          expanded: true,
          childPaths,
        };
      }

      for (const item of items) {
        const normalized = normalizePath(item.path);
        const existing = next[normalized];
        next[normalized] = {
          path: normalized,
          name: item.name || existing?.name || displayNameForPath(normalized),
          parentPath: normalizedParent,
          kind: existing?.kind || kindForPath(normalized, normalizedParent),
          loaded: existing?.loaded || false,
          loading: false,
          expanded: existing?.expanded || false,
          childPaths: existing?.childPaths || [],
        };
      }

      return next;
    });

    if (!normalizedParent) {
      setRootPaths(childPaths);
      setRootsLoaded(true);
    }
  };

  const loadRoots = async () => {
    setTreeLoading(true);
    setTreeError('');
    setTreeErrorTarget('');
    try {
      const data = await fetchDirectories('');
      hydrateDirectory('', Array.isArray(data.items) ? data.items : []);
      setTreeError('');
      setTreeErrorTarget('');
    } catch (requestError) {
      const rawError = requestError instanceof Error ? requestError.message : text(locale, 'Failed to load directories.', '加载目录失败。');
      setTreeError(rawError);
      setTreeErrorTarget('');
    } finally {
      setTreeLoading(false);
    }
  };

  const loadChildren = async (targetPath: string, options?: { force?: boolean; expand?: boolean }) => {
    const normalized = normalizePath(targetPath);
    if (!normalized) {
      await loadRoots();
      return;
    }

    setTreeError('');
    setTreeErrorTarget('');
    setNodes((current) => ({
      ...current,
      [normalized]: {
        path: normalized,
        name: current[normalized]?.name || displayNameForPath(normalized),
        parentPath: current[normalized]?.parentPath || '',
        kind: current[normalized]?.kind || kindForPath(normalized, ''),
        loaded: current[normalized]?.loaded || false,
        loading: true,
        expanded: options?.expand ?? true,
        childPaths: current[normalized]?.childPaths || [],
      },
    }));

    try {
      const existing = nodes[normalized];
      if (existing?.loaded && !options?.force) {
        setNodes((current) => ({
          ...current,
          [normalized]: { ...current[normalized], loading: false, expanded: options?.expand ?? true },
        }));
        setTreeError('');
        setTreeErrorTarget('');
        return;
      }

      const data = await fetchDirectories(normalized);
      hydrateDirectory(normalized, Array.isArray(data.items) ? data.items : []);
      setTreeError('');
      setTreeErrorTarget('');
    } catch (requestError) {
      const rawError = requestError instanceof Error ? requestError.message : text(locale, 'Failed to load directories.', '加载目录失败。');
      setTreeErrorTarget(normalized);
      setTreeError(friendlyDirectoryError(locale, normalized, rawError));
      setNodes((current) => ({
        ...current,
        [normalized]: {
          path: normalized,
          name: current[normalized]?.name || displayNameForPath(normalized),
          parentPath: current[normalized]?.parentPath || '',
          kind: current[normalized]?.kind || kindForPath(normalized, ''),
          loaded: current[normalized]?.loaded || false,
          loading: false,
          expanded: current[normalized]?.expanded || false,
          childPaths: current[normalized]?.childPaths || [],
        },
      }));
    }
  };

  const revealPath = async (targetPath: string) => {
    const normalized = normalizePath(targetPath);
    onPathChange(normalized);
    setPathInput(normalized);

    if (!normalized) {
      if (!rootsLoaded) {
        await loadRoots();
      }
      return;
    }

    if (!rootsLoaded) {
      await loadRoots();
    }

    const segments = splitPathSegments(normalized);
    if (segments.length === 0) {
      return;
    }

    if (isUNCPath(normalized)) {
      ensureVirtualNode(segments[0]);
    }

    for (const segment of segments) {
      setNodes((current) => ({
        ...current,
        [segment]: {
          path: segment,
          name: current[segment]?.name || displayNameForPath(segment),
          parentPath: current[segment]?.parentPath || '',
          kind: current[segment]?.kind || kindForPath(segment, current[segment]?.parentPath || ''),
          loaded: current[segment]?.loaded || false,
          loading: current[segment]?.loading || false,
          expanded: true,
          childPaths: current[segment]?.childPaths || [],
        },
      }));
      await loadChildren(segment, { expand: true });
    }
  };

  useEffect(() => {
    void loadRoots();
  }, []);

  useEffect(() => {
    if (!selectedPath) {
      return;
    }
    if (!nodes[selectedPath] && !loading) {
      void revealPath(selectedPath);
    }
  }, [loading, nodes, selectedPath]);

  const handleToggleNode = async (nodePath: string) => {
    const normalized = normalizePath(nodePath);
    const node = nodes[normalized];
    if (!node) {
      return;
    }

    if (node.expanded) {
      setNodes((current) => ({
        ...current,
        [normalized]: { ...current[normalized], expanded: false },
      }));
      return;
    }

    setNodes((current) => ({
      ...current,
      [normalized]: { ...current[normalized], expanded: true },
    }));

    if (!node.loaded) {
      await loadChildren(normalized, { expand: true });
    }
  };

  const renderNode = (nodePath: string, depthLevel: number) => {
    const node = nodes[nodePath];
    if (!node) {
      return null;
    }

    const active = selectedPath === node.path;
    const expandable = node.childPaths.length > 0 || !node.loaded;

      return (
        <div key={node.path}>
          <div
            className={cn(
              'flex items-center gap-1 rounded-md py-1 pr-2 text-sm transition-colors',
              active ? 'bg-accent-soft text-accent-fg' : 'text-fg hover:bg-surface-sunken'
            )}
            style={{ paddingLeft: `${depthLevel * 16 + 8}px` }}
          >
            <button
              type="button"
              aria-label={text(locale, 'Toggle folder', '切换目录')}
              onClick={() => void handleToggleNode(node.path)}
              className="flex h-5 w-5 shrink-0 items-center justify-center rounded text-fg-muted transition-colors hover:text-fg disabled:pointer-events-none disabled:opacity-40"
              disabled={!expandable}
            >
              {node.loading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <ChevronRight className={`h-3.5 w-3.5 transition ${node.expanded ? 'rotate-90' : ''}`} />}
            </button>
          <button
            type="button"
            onClick={() => {
              onPathChange(node.path);
              setPathInput(node.path);
            }}
            className="flex min-w-0 flex-1 items-center gap-2 text-left"
          >
            {nodeIcon(node.kind, node.expanded)}
            <span className="truncate">{node.name}</span>
          </button>
        </div>
        {node.expanded && node.childPaths.map((childPath) => renderNode(childPath, depthLevel + 1))}
      </div>
    );
  };

  const handleStartScan = () => {
    const nextPath = normalizePath(pathInput || selectedPath);
    if (!nextPath) {
      return;
    }

    if (nextPath !== selectedPath) {
      void revealPath(nextPath);
    }

    onScan(nextPath);
  };

  return (
    <div className="space-y-4 text-fg">
      <Card>
        <CardContent className="space-y-3 pt-4">
          <div className="flex flex-wrap items-center gap-2">
            <div className="relative min-w-0 flex-1 basis-64">
              <Input
                value={pathInput}
                onChange={(event) => setPathInput(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault();
                    const nextPath = normalizePath(pathInput || selectedPath);
                    if (!nextPath) {
                      return;
                    }

                    if (isUNCPath(nextPath)) {
                      // Keep UNC browsing explicit: reveal the tree first so the failure is visible.
                      void revealPath(nextPath);
                      return;
                    }

                    handleStartScan();
                  }
                }}
                placeholder="C:\\Data or \\\\server\\share"
                className="font-mono"
              />
            </div>
            <label className="flex items-center gap-2 text-xs font-medium text-fg-secondary">
              <span className="whitespace-nowrap">{text(locale, 'Scan depth', '扫描层级')}</span>
              <NativeSelect
                value={depth}
                onChange={(event) => onDepthChange(Number(event.target.value))}
                disabled={loading}
                className="w-auto"
              >
                {[-1, 1, 2, 3, 4, 5, 8, 10].map((value) => (
                  <option key={value} value={value}>{depthLabel(locale, value)}</option>
                ))}
              </NativeSelect>
            </label>
            <label className="flex items-center gap-2 text-xs font-medium text-fg-secondary">
              <input
                type="checkbox"
                checked={includeInherited}
                onChange={(event) => onIncludeInheritedChange(event.target.checked)}
                disabled={loading}
                className="h-3.5 w-3.5 rounded border-line"
              />
              <span className="whitespace-nowrap">{text(locale, 'Include inherited', '包含继承')}</span>
            </label>
            <Button
              type="button"
              onClick={handleStartScan}
              disabled={loading || !pathInput.trim()}
              variant="primary"
            >
              {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}
              {loading ? text(locale, 'Scanning…', '扫描中…') : text(locale, 'Start Scan', '开始扫描')}
            </Button>
            {canCancelScan ? (
              <Button
                type="button"
                onClick={onCancelScan}
                disabled={cancelPending}
                variant="secondary"
              >
                {cancelPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Square className="h-4 w-4 text-danger" />}
                {cancelPending ? text(locale, 'Stopping…', '正在停止…') : text(locale, 'Stop', '停止')}
              </Button>
            ) : null}
            <Button
              type="button"
              onClick={() => void loadRoots()}
              variant="ghost"
              size="icon"
              aria-label={text(locale, 'Refresh roots', '刷新根目录')}
            >
              <RefreshCw className={`h-4 w-4 ${treeLoading ? 'animate-spin' : ''}`} />
            </Button>
          </div>

          <p className="truncate font-mono text-2xs text-fg-muted">
            {stagedPath || text(locale, 'Select a directory or enter a path.', '请选择目录或输入路径。')}
          </p>
        </CardContent>
      </Card>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-3">
        <Card className="xl:col-span-2">
          <div className="flex items-start justify-between gap-3 border-b border-line px-4 pt-4 pb-3">
            <div className="text-sm font-semibold text-fg">{text(locale, 'Directory Browser', '目录浏览')}</div>
            <Badge tone="neutral" className="shrink-0">{rootPaths.length} {text(locale, 'roots', '个根')}</Badge>
          </div>

          <div className="flex flex-wrap items-center gap-1.5 border-b border-line px-4 py-2.5">
            {selectedTrail.length === 0 ? (
              <span className="text-sm text-fg-muted">{text(locale, 'Select a directory or type a UNC share path to begin.', '请选择目录，或输入 UNC 共享路径开始。')}</span>
            ) : (
              selectedTrail.map((item, index) => (
                <button
                  key={item.path}
                  type="button"
                  aria-pressed={item.path === selectedPath}
                  onClick={() => void revealPath(item.path)}
                  className={cn(
                    'inline-flex items-center gap-1 rounded-md border border-line px-2 py-1 text-xs transition-colors hover:bg-surface-sunken',
                    item.path === selectedPath ? 'bg-accent-soft text-accent-fg border-accent/30' : 'text-fg-secondary'
                  )}
                >
                  {index > 0 ? <ChevronRight className="h-3 w-3 text-fg-faint" /> : null}
                  <span>{item.label}</span>
                </button>
              ))
            )}
          </div>

          <div className="h-[420px] overflow-auto px-2 py-2">
            {treeError ? (
              <div className="mx-2 mb-3 flex items-start gap-2 rounded-lg border border-danger/40 bg-danger-soft px-3 py-2.5 text-sm text-danger">
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                <div className="min-w-0">
                  <div className="font-medium">{isUNCPath(treeErrorTarget || selectedPath) ? text(locale, 'UNC share could not be opened', 'UNC 共享无法打开') : text(locale, 'Tree loading failed', '目录树加载失败')}</div>
                  <div className="mt-0.5 text-xs">{treeError}</div>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {treeErrorTarget ? (
                      <Button
                        type="button"
                        onClick={() => void loadChildren(treeErrorTarget, { force: true, expand: true })}
                        variant="secondary"
                        size="sm"
                      >
                        <RefreshCw className="h-3.5 w-3.5" />
                        {text(locale, 'Retry browse', '重试浏览')}
                      </Button>
                    ) : null}
                    {isUNCPath(treeErrorTarget || selectedPath) ? (
                      <Button
                        type="button"
                        onClick={handleStartScan}
                        disabled={loading || !pathInput.trim()}
                        variant="primary"
                        size="sm"
                      >
                        <Play className="h-3.5 w-3.5" />
                        {text(locale, 'Try scan anyway', '仍然扫描')}
                      </Button>
                    ) : null}
                  </div>
                </div>
              </div>
            ) : null}
            {treeLoading && rootPaths.length === 0 ? (
              <div className="flex h-full items-center justify-center px-2">
                <div className="flex w-full max-w-md flex-col items-center gap-2 text-center">
                  <Loader2 className="h-4 w-4 animate-spin text-fg-muted" />
                  <div className="text-sm font-medium text-fg">{text(locale, 'Loading roots…', '正在加载根目录…')}</div>
                  <div className="text-xs text-fg-muted">{text(locale, 'Preparing the first directory and share roots for browsing.', '正在准备可浏览的首批目录与共享根路径。')}</div>
                </div>
              </div>
            ) : rootPaths.length === 0 ? (
              <div className="flex h-full items-center justify-center px-2">
                <EmptyState
                  icon={Folder}
                  title={text(locale, 'No folders available yet.', '当前没有可展示的目录。')}
                  description={text(locale, 'Type a UNC root or refresh the available roots to stage the next scan target.', '输入 UNC 根路径，或刷新可用根路径来准备下一次扫描目标。')}
                  className="w-full max-w-md py-6"
                />
              </div>
            ) : (
              rootPaths.map((nodePath) => renderNode(nodePath, 0))
            )}
          </div>
        </Card>

        <aside className="space-y-4">
          <Card className="px-4 py-4">
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <div className={captionClass}>{text(locale, 'Runtime', '运行态')}</div>
                <div className="mt-0.5 text-sm font-semibold text-fg">{summaryHeadline}</div>
              </div>
              <Badge tone={progressTone(progressStatus, loading)} className="shrink-0">{hasProgressSnapshot ? livePhaseLabel : text(locale, 'Idle', '待命')}</Badge>
            </div>
            {hasProgressSnapshot ? (
              <div className="mt-3 h-2 overflow-hidden rounded-full bg-surface-sunken">
                <div
                  className={`h-full rounded-full ${
                    progressStatus === 'failed'
                      ? 'bg-danger'
                      : progressStatus === 'cancelled'
                        ? 'bg-warning'
                        : progressStatus === 'completed'
                          ? 'bg-success'
                          : progressStatus === 'expanding'
                            ? 'w-[78%] bg-accent'
                            : loading
                              ? 'w-[58%] bg-accent'
                              : 'w-[18%] bg-fg-faint'
                  } ${loading && progressStatus !== 'failed' && progressStatus !== 'completed' ? 'animate-pulse' : ''}`}
                />
              </div>
            ) : null}
            <div className="mt-3 grid grid-cols-3 gap-2">
              {progressFacts.map((fact) => (
                <div key={fact.label} className="rounded-md border border-line bg-surface-sunken px-2 py-1.5">
                  <div className="text-2xs text-fg-muted">{fact.label}</div>
                  <div className="text-sm font-semibold text-fg tabular-nums">{fact.value}</div>
                </div>
              ))}
            </div>
            <div className="mt-3 text-xs leading-5 text-fg-muted">
              <Activity className="mr-1 inline h-3.5 w-3.5" />
              {hasProgressSnapshot
                ? `${text(locale, 'Elapsed', '耗时')}: ${elapsedLabel}`
                : text(locale, 'No active run yet.', '当前没有活动运行。')}
            </div>
          </Card>

          <Card className="px-4 py-4">
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <div className={captionClass}>{text(locale, 'Context', '上下文')}</div>
                <div className="mt-0.5 text-sm font-semibold text-fg">{showADPanel ? text(locale, 'UNC and AD resolution', 'UNC 与 AD 解析') : text(locale, 'Local scan workflow', '本地扫描流程')}</div>
              </div>
              <Badge tone={showADPanel ? (adReady ? 'success' : 'warning') : 'neutral'} className="shrink-0">
                {showADPanel ? (adReady ? text(locale, 'AD ready', 'AD 就绪') : text(locale, 'AD optional', 'AD 可选')) : text(locale, 'Local', '本地')}
              </Badge>
            </div>

            <div className="mt-3 flex flex-col gap-3 rounded-md border border-line bg-surface-sunken px-3 py-3">
              <div className="flex min-w-0 items-start gap-3">
                <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-accent-soft text-accent-fg">
                  <Server className="h-4 w-4" />
                </span>
                <div className="min-w-0">
                  <div className="text-sm font-semibold text-fg">{text(locale, 'AD connection module', 'AD 连接模块')}</div>
                  <div className="mt-1 text-xs leading-5 text-fg-muted">
                    {connection.connected
                      ? text(locale, 'Connection is verified. Open the AD workspace to review domain context or retest credentials.', '连接已验证。可进入 AD 工作区查看域上下文或重新测试凭据。')
                      : text(locale, 'Connect AD before UNC identity expansion or domain-aware permission review.', '在进行 UNC 身份展开或带域上下文的权限复核前，请先连接 AD。')}
                  </div>
                </div>
              </div>
              <Button type="button" onClick={() => void router.push('/ad-workspace')} variant="secondary" size="sm" className="self-start">
                {connection.connected ? text(locale, 'Open AD Workspace', '打开 AD 工作区') : text(locale, 'Connect AD', '连接 AD')}
              </Button>
            </div>

            {showADPanel ? (
              <>
                <p className="mt-2 text-xs leading-5 text-fg-muted">
                  {adReady ? text(locale, 'Share access stays on the backend Windows identity; AD only enriches identity resolution.', '共享访问仍由后端 Windows 身份执行；AD 只增强身份解析。') : text(locale, 'The scan can still run without AD if the backend Windows identity can access the share.', '如果后端 Windows 身份能访问共享，即使不连 AD 也可以继续扫描。')}
                </p>
                <div className="mt-3 grid grid-cols-1 gap-2">
                  <div className="rounded-md border border-line bg-surface-sunken px-2 py-1.5">
                    <div className="text-2xs text-fg-muted">{text(locale, 'Runtime identity', '运行身份')}</div>
                    <div className="break-all text-sm font-semibold text-fg">{runtimeIdentity?.account_name || text(locale, 'Unavailable', '暂不可用')}</div>
                  </div>
                  <div className="rounded-md border border-line bg-surface-sunken px-2 py-1.5">
                    <div className="text-2xs text-fg-muted">{text(locale, 'Directory account', '目录账号')}</div>
                    <div className="break-all text-sm font-semibold text-fg">{config.username || text(locale, 'Not configured', '尚未配置')}</div>
                  </div>
                </div>

                {runtimeIdentityError ? (
                  <div className="mt-3 flex items-start gap-2 rounded-lg border border-info/40 bg-info-soft px-3 py-2.5 text-sm text-info">
                    <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                    <div className="min-w-0">
                      <div className="font-medium">{text(locale, 'Runtime identity note', '运行身份提示')}</div>
                      <div className="mt-0.5 text-xs">{runtimeIdentityError}</div>
                    </div>
                  </div>
                ) : null}

                <div className="mt-3 rounded-md border border-line bg-surface-sunken px-3 py-2 text-xs leading-5 text-fg-muted">
                  {adReady
                    ? text(locale, 'AD credential changes now live in AD Workspace, keeping this scan panel focused on path selection and runtime state.', 'AD 凭据修改现在统一放在 AD 工作区，这里只保留路径选择与运行状态。')
                    : text(locale, 'Open AD Workspace to configure or verify AD before identity expansion.', '请打开 AD 工作区配置或验证 AD，再进行身份展开。')}
                </div>
              </>
            ) : (
              <div className="mt-2 flex items-start gap-3 text-xs leading-6 text-fg-muted">
                <HardDrive className="mt-1 h-4 w-4 shrink-0 text-info" />
                <span>{text(locale, 'For local folders, directory resolution is not required. Select a folder in the tree, confirm the staged path, then run the scan directly.', '对于本地目录，不需要目录解析。先在树中选择文件夹，确认已准备路径，然后直接启动扫描。')}</span>
              </div>
            )}
          </Card>

          {(message || error) ? (
            <Card className="px-4 py-4">
              {message ? (
                <div className="flex items-start gap-2 rounded-lg border border-info/40 bg-info-soft px-3 py-2.5 text-sm text-info">
                  <Activity className="mt-0.5 h-4 w-4 shrink-0" />
                  <div className="min-w-0">
                    <div className="font-medium">{text(locale, 'Operator message', '操作提示')}</div>
                    <div className="mt-0.5 text-xs">{message}</div>
                  </div>
                </div>
              ) : null}
              {error ? (
                <div className="mt-3 flex items-start gap-2 rounded-lg border border-danger/40 bg-danger-soft px-3 py-2.5 text-sm text-danger first:mt-0">
                  <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                  <div className="min-w-0">
                    <div className="font-medium">{text(locale, 'Scan error', '扫描错误')}</div>
                    <div className="mt-0.5 text-xs">{error}</div>
                  </div>
                </div>
              ) : null}
            </Card>
          ) : null}
        </aside>
      </div>
    </div>
  );
}
