import Head from 'next/head';
import { useRouter } from 'next/router';
import { useEffect, useMemo, useRef, useState } from 'react';
import ScanCompletionSummary from '../components/ScanCompletionSummary';
import ScanExplorerConsole, { type ServerBatchProgress } from '../components/ScanExplorerConsole';
import { useADConnection } from '../contexts/ADConnectionContext';
import { useI18n } from '../contexts/I18nContext';
import { upsertRiskFindingsFromScan } from '../lib/riskFindings';
import { apiBase, websocketBase } from '../lib/runtimeApi';
import {
  appendOperationLog,
  buildWorkspacePageTitle,
  defaultScanDefaults,
  defaultWorkspaceSettings,
  readWorkspaceSettings,
  type ScanDefaults,
  type WorkspaceSettings,
} from '../lib/workspaceSettings';
import { findWatchedShare, recordWatchedShareScan } from '../lib/watchedShares';

interface ScanPermission {
  path: string;
  trustee: string;
  trustee_sid: string;
  rights: string;
  type: string;
  inherited: boolean;
  source?: string;
  applies_to?: string;
  account_type?: string;
  access_mask?: string;
  risk_level?: string;
  parent_delta?: string;
  account_name?: string;
  first_name?: string;
  last_name?: string;
  email?: string;
  department?: string;
  division?: string;
  domain?: string;
  originating_group?: string;
  group_inheritance_hierarchy?: string;
}

interface IdentityResolutionSummary {
  directory_sync_run_id?: string;
  mode: string;
  resolved_principal_count: number;
  unresolved_principal_count: number;
  warning?: string;
}

interface SkippedPath {
  path: string;
  error: string;
}

interface ScanResponse {
  session_id?: string;
  root_path?: string;
  items_scanned?: number;
  permission_count?: number;
  permissions?: ScanPermission[];
  skipped?: SkippedPath[];
  error?: string;
  status?: string;
  identity_resolution?: IdentityResolutionSummary;
}

interface ScanProgressEvent {
  scan_id?: string;
  session_id?: string;
  items_scanned?: number;
  permission_count?: number;
  current_path?: string;
  status?: string;
  error?: string;
}

interface LiveScanState {
  scanID: string;
  sessionID: string;
  status: string;
  currentPath: string;
  itemsScanned: number;
  permissionCount: number;
  startedAt: number;
  lastUpdatedAt: number;
  liveConnected: boolean;
  error: string;
}

interface ScanRunOutcome {
  ok: boolean;
  cancelled?: boolean;
  path: string;
  sessionID?: string;
  error?: string;
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

function inferRiskLevel(rights: string, provided?: string) {
  const normalized = (provided || '').trim().toLowerCase();
  if (normalized === 'high' || normalized === 'medium' || normalized === 'low') {
    return normalized;
  }

  const value = rights.trim().toLowerCase();
  if (['full control', 'write', 'delete', 'take ownership', 'change permissions', 'modify']
    .some((token) => value.includes(token))) {
    return 'high';
  }
  if (value.includes('execute')) {
    return 'medium';
  }
  return 'low';
}

function buildScanID() {
  if (typeof window !== 'undefined' && window.crypto?.randomUUID) {
    return window.crypto.randomUUID();
  }
  return `scan-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

// During a discovered-share batch, the ref is updated before the live UI state.
// Prefer it so cancel never targets the previous share during that handoff.
export function resolveCancelScanID(activeScanID: string, liveScanID: string, sessionID: string) {
  return [activeScanID, liveScanID, sessionID]
    .map((value) => value.trim())
    .find(Boolean) || '';
}

export default function ScanWorkspacePage() {
  const router = useRouter();
  const { locale, t } = useI18n();
  const { activeProfile, config, connection } = useADConnection();

  const [path, setPath] = useState('');
  const [scanDefaults, setScanDefaults] = useState<ScanDefaults>(() => ({ ...defaultScanDefaults }));
  const [workspaceSettings, setWorkspaceSettings] = useState<WorkspaceSettings>(defaultWorkspaceSettings);
  const [loading, setLoading] = useState(false);
  const [cancelPending, setCancelPending] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [results, setResults] = useState<ScanPermission[]>([]);
  const [skippedItems, setSkippedItems] = useState<SkippedPath[]>([]);
  const [identityResolution, setIdentityResolution] = useState<IdentityResolutionSummary | null>(null);
  const [sessionID, setSessionID] = useState('');
  const [progressTrackedPath, setProgressTrackedPath] = useState('');
  const [itemsScanned, setItemsScanned] = useState(0);
  const [activeWatchedShareID, setActiveWatchedShareID] = useState('');
  const [liveScan, setLiveScan] = useState<LiveScanState>({
    scanID: '',
    sessionID: '',
    status: 'idle',
    currentPath: '',
    itemsScanned: 0,
    permissionCount: 0,
    startedAt: 0,
    lastUpdatedAt: 0,
    liveConnected: false,
    error: '',
  });
  const [serverBatch, setServerBatch] = useState<ServerBatchProgress | null>(null);
  const scanSocketRef = useRef<WebSocket | null>(null);
  const serverBatchCancelledRef = useRef(false);
  const activeScanIDRef = useRef('');

  const normalizedPath = normalizePath(path);
  const inlineADReady = Boolean(
    connection.connected
    && config.adServer
    && config.baseDN
    && config.username
    && config.password,
  );
  const adReady = Boolean(activeProfile || inlineADReady);
  const riskCount = useMemo(
    () => results.filter((item) => inferRiskLevel(item.rights, item.risk_level) === 'high').length,
    [results],
  );

  useEffect(() => {
    const queryPath = typeof router.query.path === 'string' ? router.query.path : '';
    if (queryPath) {
      setPath(queryPath);
    }
  }, [router.query.path]);

  useEffect(() => {
    setWorkspaceSettings(readWorkspaceSettings());
  }, []);

  useEffect(() => {
    const queryWatch = typeof router.query.watch === 'string' ? router.query.watch.trim() : '';
    if (!queryWatch) {
      return;
    }

    const profile = findWatchedShare(queryWatch);
    if (!profile) {
      return;
    }

    setActiveWatchedShareID(profile.id);
    setPath(profile.path);
    setScanDefaults({
      defaultDepth: profile.defaultDepth,
      includeInherited: profile.includeInherited,
    });
    setMessage(text(
      locale,
      'Watched share settings loaded. Start the scan when ready.',
      '已载入受监控目录配置，准备好后即可开始扫描。',
    ));
  }, [locale, router.query.watch]);

  const closeScanSocket = (expectedScanID?: string) => {
    const socket = scanSocketRef.current;
    if (socket) {
      if (expectedScanID && !socket.url.includes(`scan_id=${encodeURIComponent(expectedScanID)}`)) {
        return;
      }
      socket.close();
      scanSocketRef.current = null;
    }
  };

  const openScanSocket = (scanID: string, targetPath: string) => {
    closeScanSocket();

    try {
      const socket = new WebSocket(`${websocketBase()}/api/scan/ws?scan_id=${encodeURIComponent(scanID)}`);
      scanSocketRef.current = socket;

      socket.onopen = () => {
        setLiveScan((current) => ({
          ...current,
          scanID,
          status: current.status === 'idle' ? 'connecting' : current.status,
          currentPath: current.currentPath || targetPath,
          liveConnected: true,
          lastUpdatedAt: Date.now(),
        }));
      };

      socket.onmessage = (event) => {
        try {
          const payload = JSON.parse(event.data) as ScanProgressEvent;
          setLiveScan((current) => ({
            ...current,
            scanID,
            sessionID: payload.session_id || current.sessionID,
            status: payload.status || current.status,
            currentPath: payload.current_path || current.currentPath || targetPath,
            itemsScanned: Number(payload.items_scanned ?? current.itemsScanned),
            permissionCount: Number(payload.permission_count ?? current.permissionCount),
            liveConnected: true,
            lastUpdatedAt: Date.now(),
            error: payload.error || current.error,
          }));
        } catch {
          // Ignore malformed progress messages.
        }
      };

      socket.onerror = () => {
        setLiveScan((current) => ({ ...current, liveConnected: false }));
      };

      socket.onclose = () => {
        if (scanSocketRef.current === socket) {
          scanSocketRef.current = null;
        }
      };
    } catch {
      setLiveScan((current) => ({ ...current, liveConnected: false }));
    }
  };

  useEffect(() => () => closeScanSocket(), []);

  useEffect(() => {
    if (loading) {
      return;
    }

    const tracked = normalizePath(progressTrackedPath);
    const current = normalizePath(path);
    if (!tracked || !current || current === tracked || current.startsWith(`${tracked}\\`)) {
      return;
    }

    setLiveScan((currentState) => ({
      ...currentState,
      status: 'idle',
      currentPath: '',
      itemsScanned: 0,
      permissionCount: 0,
      startedAt: 0,
      lastUpdatedAt: 0,
      liveConnected: false,
      error: '',
    }));
  }, [loading, path, progressTrackedPath]);

  const cancelCurrentScan = async () => {
    if (serverBatch?.active) {
      serverBatchCancelledRef.current = true;
      setMessage(text(
        locale,
        'Stop requested. The current share will stop, and no additional discovered shares will start.',
        '已请求停止：当前共享会安全结束，后续已发现共享不会再启动扫描。',
      ));
    }

    const activeScanID = resolveCancelScanID(activeScanIDRef.current, liveScan.scanID, sessionID);
    if (!activeScanID || cancelPending) {
      return;
    }

    setCancelPending(true);
    setError('');
    setMessage(text(
      locale,
      'Stop request sent. Waiting for the scanner to exit cleanly...',
      '已发送停止请求，正在等待扫描器安全退出...',
    ));

    try {
      const response = await fetch(`${apiBase()}/api/scan/${encodeURIComponent(activeScanID)}/cancel`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      });
      const data = (await response.json().catch(() => ({}))) as { error?: string; message?: string };
      if (!response.ok) {
        throw new Error(data.error || 'Failed to cancel scan');
      }

      setMessage(data.message || text(locale, 'Scan cancellation requested.', '已请求停止扫描。'));
    } catch (requestError) {
      const nextError = requestError instanceof Error
        ? requestError.message
        : text(locale, 'Failed to cancel scan.', '停止扫描失败。');
      setError(nextError);
      setCancelPending(false);
    }
  };

  const runScan = async (pathOverride?: string): Promise<ScanRunOutcome> => {
    const requestedPath = normalizePath(pathOverride ?? path);
    const requestedIsUNC = isUNCPath(requestedPath);

    if (!requestedPath) {
      const nextError = text(locale, 'Path is required.', '必须填写扫描路径。');
      setError(nextError);
      return { ok: false, path: requestedPath, error: nextError };
    }

    if (requestedPath !== normalizedPath) {
      setPath(requestedPath);
    }

    const scanID = buildScanID();
    activeScanIDRef.current = scanID;
    setLoading(true);
    setCancelPending(false);
    setError('');
    setMessage('');
    setResults([]);
    setSkippedItems([]);
    setSessionID('');
    setIdentityResolution(null);
    setItemsScanned(0);
    setProgressTrackedPath(requestedPath);
    setLiveScan({
      scanID,
      sessionID: '',
      status: 'connecting',
      currentPath: requestedPath,
      itemsScanned: 0,
      permissionCount: 0,
      startedAt: Date.now(),
      lastUpdatedAt: Date.now(),
      liveConnected: false,
      error: '',
    });
    openScanSocket(scanID, requestedPath);

    const createScanBody = () => {
      const requestedDepth = Number.isFinite(scanDefaults.defaultDepth)
        ? scanDefaults.defaultDepth
        : defaultScanDefaults.defaultDepth;
      const nextBody: Record<string, unknown> = {
        scan_id: scanID,
        path: requestedPath,
        depth: requestedDepth,
        include_inherited: scanDefaults.includeInherited,
      };

      if (requestedIsUNC && adReady) {
        nextBody.effective_permissions = activeProfile
          ? {
            enabled: true,
            connection_id: activeProfile.id,
          }
          : {
            enabled: true,
            server: config.adServer,
            base_dn: config.baseDN,
            username: config.username,
            password: config.password,
          };
      }

      return nextBody;
    };

    const executeScan = async () => {
      const response = await fetch(`${apiBase()}/api/scan`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(createScanBody()),
      });
      const data = (await response.json()) as ScanResponse;
      if (!response.ok) {
        throw new Error(data.error || 'Scan failed');
      }
      return data;
    };

    const applyCancelledState = () => {
      setMessage(text(locale, 'Scan cancelled.', '扫描已停止。'));
      setError('');
      setLiveScan((current) => ({
        ...current,
        status: 'cancelled',
        lastUpdatedAt: Date.now(),
        liveConnected: false,
        error: '',
      }));
    };

    const applySuccessfulResult = (data: ScanResponse) => {
      const nextResults = Array.isArray(data.permissions) ? data.permissions : [];
      const nextSkipped = Array.isArray(data.skipped) ? data.skipped : [];
      const nextSessionID = data.session_id || '';
      const nextItemsScanned = Number(data.items_scanned || 0);

      setResults(nextResults);
      setSkippedItems(nextSkipped);
      setSessionID(nextSessionID);
      setItemsScanned(nextItemsScanned);
      setIdentityResolution(data.identity_resolution || null);
      setLiveScan((current) => ({
        ...current,
        sessionID: nextSessionID || current.sessionID,
        status: 'completed',
        currentPath: data.root_path || requestedPath,
        itemsScanned: nextItemsScanned,
        permissionCount: Number(data.permission_count || nextResults.length),
        lastUpdatedAt: Date.now(),
        liveConnected: false,
        error: '',
      }));

      return { nextResults, nextSkipped };
    };

    const canUseEffectivePermissions = requestedIsUNC && adReady;
    if (canUseEffectivePermissions) {
      setMessage(text(
        locale,
        'UNC scan will resolve AD principals through the active connection.',
        'UNC 扫描将通过当前 AD 连接解析域主体。',
      ));
    } else if (requestedIsUNC) {
      setMessage(text(
        locale,
        'AD is not connected, so the UNC scan will keep SID-oriented trustees.',
        'AD 未连接，UNC 扫描将保留 SID 形式的主体。',
      ));
    }

    try {
      const data = await executeScan();
      if (activeScanIDRef.current === scanID) {
        activeScanIDRef.current = '';
      }
      if (data.status === 'cancelled') {
        applyCancelledState();
        return { ok: false, cancelled: true, path: requestedPath };
      }

      const usedFallbackMode = data.identity_resolution?.mode === 'raw-fallback';

      const { nextResults, nextSkipped } = applySuccessfulResult(data);
      setMessage(usedFallbackMode
        ? text(
          locale,
          'AD expansion returned no rows, so this run fell back to the directory ACL dataset.',
          'AD 展开没有返回记录，本次运行已回退到目录 ACL 数据集。',
        )
        : text(locale, 'Scan completed.', '扫描已完成。'));

      const nextRiskCount = nextResults
        .filter((item) => inferRiskLevel(item.rights, item.risk_level) === 'high')
        .length;
      try {
        recordWatchedShareScan({
          id: activeWatchedShareID || undefined,
          path: requestedPath,
          sessionID: data.session_id || '',
          itemsScanned: Number(data.items_scanned || 0),
          permissionCount: Number(data.permission_count || nextResults.length),
          highRiskCount: nextRiskCount,
        });
      } catch (metadataError) {
        appendOperationLog(
          {
            scope: 'scan',
            action: 'record-watched-share-failed',
            message: metadataError instanceof Error ? metadataError.message : 'Watched share metadata could not be saved.',
            context: { path: requestedPath, sessionID: data.session_id || '' },
          },
          workspaceSettings.auditLogging,
        );
      }

      appendOperationLog(
        {
          scope: 'scan',
          action: usedFallbackMode ? 'run-scan-fallback' : 'run-scan',
          message: usedFallbackMode
            ? text(locale, 'Scan completed with raw ACL fallback.', '扫描已通过原始 ACL 回退完成。')
            : text(locale, 'Scan completed.', '扫描已完成。'),
          context: {
            path: requestedPath,
            permissions: nextResults.length,
            skipped: nextSkipped.length,
            unc: requestedIsUNC,
            adReady,
          },
        },
        workspaceSettings.auditLogging,
      );

      try {
        await upsertRiskFindingsFromScan({
          permissions: nextResults,
          sessionID: data.session_id || '',
        });
      } catch (riskPersistenceError) {
        const detail = riskPersistenceError instanceof Error ? riskPersistenceError.message : 'Risk finding persistence failed.';
        setMessage(`${text(
          locale,
          'Scan completed, but risk findings could not be saved.',
          '扫描已完成，但风险发现未能保存。',
        )} ${detail}`);
        appendOperationLog(
          {
            scope: 'scan',
            action: 'run-scan-risk-sync-failed',
            message: detail,
            context: {
              path: requestedPath,
              sessionID: data.session_id || '',
              permissions: nextResults.length,
            },
          },
          workspaceSettings.auditLogging,
        );
      }
      return { ok: true, path: requestedPath, sessionID: data.session_id || '' };
    } catch (requestError) {
      const nextError = requestError instanceof Error ? requestError.message : 'Scan failed';
      setError(nextError);
      setLiveScan((current) => ({
        ...current,
        status: 'failed',
        error: nextError,
        lastUpdatedAt: Date.now(),
        liveConnected: false,
      }));
      appendOperationLog(
        {
          scope: 'scan',
          action: 'run-scan-failed',
          message: nextError,
          context: { path: requestedPath, unc: requestedIsUNC, adReady },
        },
        workspaceSettings.auditLogging,
      );
      return { ok: false, path: requestedPath, error: nextError };
    } finally {
      if (activeScanIDRef.current === scanID) {
        activeScanIDRef.current = '';
      }
      setLoading(false);
      setCancelPending(false);
      window.setTimeout(() => closeScanSocket(scanID), 1200);
    }
  };

  const runDiscoveredShares = async (serverPath: string, sharePaths: string[]) => {
    const normalizedServerPath = normalizePath(serverPath);
    const shares = Array.from(new Set(
      sharePaths
        .map((sharePath) => normalizePath(sharePath))
        .filter((sharePath) => isUNCPath(sharePath)),
    ));
    if (!normalizedServerPath || shares.length === 0 || loading || serverBatch?.active) {
      return;
    }

    serverBatchCancelledRef.current = false;
    let completed = 0;
    let failed = 0;
    let stopped = false;
    const failedPaths: string[] = [];
    setError('');
    setResults([]);
    setSkippedItems([]);
    setIdentityResolution(null);
    setServerBatch({
      serverPath: normalizedServerPath,
      total: shares.length,
      completed,
      failed,
      currentPath: '',
      active: true,
    });

    for (const sharePath of shares) {
      if (serverBatchCancelledRef.current) {
        stopped = true;
        break;
      }

      setServerBatch({
        serverPath: normalizedServerPath,
        total: shares.length,
        completed,
        failed,
        currentPath: sharePath,
        active: true,
      });
      const outcome = await runScan(sharePath);
      if (outcome.ok) {
        completed++;
      } else if (outcome.cancelled || serverBatchCancelledRef.current) {
        stopped = true;
        break;
      } else {
        failed++;
        failedPaths.push(sharePath);
      }
      setServerBatch({
        serverPath: normalizedServerPath,
        total: shares.length,
        completed,
        failed,
        currentPath: '',
        active: true,
      });
    }

    if (serverBatchCancelledRef.current) {
      stopped = true;
    }
    setPath(normalizedServerPath);
    setProgressTrackedPath(normalizedServerPath);
    setResults([]);
    setSkippedItems([]);
    setSessionID('');
    setItemsScanned(0);
    setIdentityResolution(null);
    setLiveScan({
      scanID: '',
      sessionID: '',
      status: stopped ? 'cancelled' : 'completed',
      currentPath: normalizedServerPath,
      itemsScanned: 0,
      permissionCount: 0,
      startedAt: 0,
      lastUpdatedAt: Date.now(),
      liveConnected: false,
      error: '',
    });
    setServerBatch({
      serverPath: normalizedServerPath,
      total: shares.length,
      completed,
      failed,
      currentPath: '',
      active: false,
    });

    const failureDetail = failedPaths.length > 0 ? ` ${failedPaths.join(', ')}` : '';
    setMessage(stopped
      ? text(
        locale,
        `Server batch stopped after ${completed} of ${shares.length} shares. Completed snapshots remain available for access analysis.`,
        `服务器批量扫描已停止，已完成 ${completed}/${shares.length} 个共享；已完成共享的快照仍可用于访问分析。`,
      )
      : text(
        locale,
        `Server batch completed: ${completed}/${shares.length} shares scanned, ${failed} failed. Analyze a user with scope ${normalizedServerPath}.${failureDetail}`,
        `服务器批量扫描完成：已扫描 ${completed}/${shares.length} 个共享，失败 ${failed} 个。可在“访问分析 > 按用户”中填写范围 ${normalizedServerPath}。${failureDetail}`,
      ));
    appendOperationLog(
      {
        scope: 'scan',
        action: stopped ? 'run-server-batch-cancelled' : 'run-server-batch',
        message: stopped ? 'Server batch scan stopped.' : 'Server batch scan completed.',
        context: { serverPath: normalizedServerPath, total: shares.length, completed, failed, stopped },
      },
      workspaceSettings.auditLogging,
    );
  };

  const scanStageNote = results.length > 0
    ? text(
      locale,
      'The scan is complete. Continue from the summary below.',
      '扫描已完成，请从下方摘要继续后续工作。',
    )
    : normalizedPath
      ? text(
        locale,
        'The target is ready. Start or retry the scan from the workbench.',
        '目标已准备好，可在工作台中开始或重试扫描。',
      )
      : text(
        locale,
        'Enter a path or browse the directory tree below.',
        '输入路径，或在下方目录树中浏览。',
      );

  return (
    <>
      <Head>
        <title>{buildWorkspacePageTitle(
          text(locale, 'Scan Center', '扫描中心'),
          workspaceSettings,
          t('appTitle'),
        )}</title>
      </Head>

      <div className="app-v2 mx-auto max-w-7xl space-y-5">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <h1 className="text-lg font-semibold text-fg">
              {text(locale, 'Scan Center', '扫描中心')}
            </h1>
            <p className="mt-0.5 text-sm text-fg-muted">{scanStageNote}</p>
          </div>
        </div>

        <section aria-label={text(locale, 'Scan launch controls', '扫描启动控制')}>
          <ScanExplorerConsole
            locale={locale}
            path={path}
            depth={scanDefaults.defaultDepth}
            includeInherited={scanDefaults.includeInherited}
            loading={loading}
            cancelPending={cancelPending}
            adReady={adReady}
            itemsScanned={itemsScanned}
            permissionCount={loading ? liveScan.permissionCount : results.length}
            resultCount={results.length}
            skippedCount={skippedItems.length}
            sessionID={sessionID || liveScan.sessionID}
            progressStatus={liveScan.status}
            progressCurrentPath={liveScan.currentPath}
            progressStartedAt={liveScan.startedAt}
            progressLastUpdatedAt={liveScan.lastUpdatedAt}
            progressLiveConnected={liveScan.liveConnected}
            progressTrackedPath={progressTrackedPath}
            message={message}
            error={error}
            onPathChange={setPath}
            onDepthChange={(value) => setScanDefaults((current) => ({
              ...current,
              defaultDepth: value,
            }))}
            onIncludeInheritedChange={(value) => setScanDefaults((current) => ({
              ...current,
              includeInherited: value,
            }))}
            onScan={(nextPath) => void runScan(nextPath)}
            onScanDiscoveredShares={(serverPath, sharePaths) => void runDiscoveredShares(serverPath, sharePaths)}
            onCancelScan={() => void cancelCurrentScan()}
            serverBatch={serverBatch}
          />
        </section>

        {results.length > 0 && !loading ? (
          <ScanCompletionSummary
            locale={locale}
            path={normalizedPath}
            sessionID={sessionID || liveScan.sessionID}
            itemsScanned={itemsScanned}
            permissionCount={results.length}
            skippedCount={skippedItems.length}
            riskCount={riskCount}
            identityResolution={identityResolution}
            onRescan={() => void runScan(normalizedPath)}
          />
        ) : null}
      </div>
    </>
  );
}
