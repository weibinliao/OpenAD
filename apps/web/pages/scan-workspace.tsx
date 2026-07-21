import Head from 'next/head';
import { useRouter } from 'next/router';
import { useEffect, useMemo, useRef, useState } from 'react';
import ScanCompletionSummary from '../components/ScanCompletionSummary';
import ScanExplorerConsole from '../components/ScanExplorerConsole';
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

const legacyExclusionGroupPatterns = [
  'Everyone',
  'NT AUTHORITY\\*',
  'BUILTIN\\Administrators',
  'BUILTIN\\Users',
  'BUILTIN\\Guests',
  'BUILTIN\\Power Users',
  'BUILTIN\\Account Operators',
  'BUILTIN\\Server Operators',
  'BUILTIN\\Print Operators',
  'BUILTIN\\Backup Operators',
  'BUILTIN\\Replicators',
  'Authenticated Users',
  'Creator Owner',
  'Creator Group',
  'Owner Rights',
];

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
  const scanSocketRef = useRef<WebSocket | null>(null);

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

  const closeScanSocket = () => {
    if (scanSocketRef.current) {
      scanSocketRef.current.close();
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
    const activeScanID = (liveScan.scanID || sessionID).trim();
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

  const runScan = async (pathOverride?: string) => {
    const requestedPath = normalizePath(pathOverride ?? path);
    const requestedIsUNC = isUNCPath(requestedPath);

    if (!requestedPath) {
      setError(text(locale, 'Path is required.', '必须填写扫描路径。'));
      return;
    }

    if (requestedPath !== normalizedPath) {
      setPath(requestedPath);
    }

    const scanID = buildScanID();
    setLoading(true);
    setCancelPending(false);
    setError('');
    setMessage('');
    setResults([]);
    setSkippedItems([]);
    setSessionID('');
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

    const createScanBody = (withEffectivePermissions: boolean) => {
      const requestedDepth = Number.isFinite(scanDefaults.defaultDepth)
        ? scanDefaults.defaultDepth
        : defaultScanDefaults.defaultDepth;
      const nextBody: Record<string, unknown> = {
        scan_id: scanID,
        path: requestedPath,
        depth: requestedDepth,
        include_inherited: scanDefaults.includeInherited,
      };

      if (withEffectivePermissions) {
        nextBody.effective_permissions = activeProfile
          ? {
            enabled: true,
            connection_id: activeProfile.id,
            exclude_group_patterns: legacyExclusionGroupPatterns,
            exclude_user_patterns: [],
          }
          : {
            enabled: true,
            server: config.adServer,
            base_dn: config.baseDN,
            username: config.username,
            password: config.password,
            exclude_group_patterns: legacyExclusionGroupPatterns,
            exclude_user_patterns: [],
          };
      }

      return nextBody;
    };

    const executeScan = async (withEffectivePermissions: boolean) => {
      const response = await fetch(`${apiBase()}/api/scan`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(createScanBody(withEffectivePermissions)),
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

      const nextRiskCount = nextResults
        .filter((item) => inferRiskLevel(item.rights, item.risk_level) === 'high')
        .length;
      recordWatchedShareScan({
        id: activeWatchedShareID || undefined,
        path: requestedPath,
        sessionID: nextSessionID,
        itemsScanned: nextItemsScanned,
        permissionCount: Number(data.permission_count || nextResults.length),
        highRiskCount: nextRiskCount,
      });
      upsertRiskFindingsFromScan({
        permissions: nextResults,
        sessionID: nextSessionID,
      });

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
      let data = await executeScan(canUseEffectivePermissions);
      if (data.status === 'cancelled') {
        applyCancelledState();
        return;
      }

      let usedFallbackMode = false;
      if (
        canUseEffectivePermissions
        && Number(data.items_scanned || 0) > 0
        && (!Array.isArray(data.permissions) || data.permissions.length === 0)
      ) {
        data = await executeScan(false);
        usedFallbackMode = true;
        if (data.status === 'cancelled') {
          applyCancelledState();
          return;
        }
      }

      const { nextResults, nextSkipped } = applySuccessfulResult(data);
      setMessage(usedFallbackMode
        ? text(
          locale,
          'AD expansion returned no rows, so this run fell back to the directory ACL dataset.',
          'AD 展开没有返回记录，本次运行已回退到目录 ACL 数据集。',
        )
        : text(locale, 'Scan completed.', '扫描已完成。'));
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
    } catch (requestError) {
      const nextError = requestError instanceof Error ? requestError.message : 'Scan failed';
      const shouldRetryWithoutAD = canUseEffectivePermissions && (
        nextError.toLowerCase().includes('effective permission')
        || nextError.toLowerCase().includes('invalid credential')
        || nextError.toLowerCase().includes('ldap result code 49')
      );

      if (shouldRetryWithoutAD) {
        try {
          const fallbackData = await executeScan(false);
          if (fallbackData.status === 'cancelled') {
            applyCancelledState();
            return;
          }

          const { nextResults, nextSkipped } = applySuccessfulResult(fallbackData);
          setMessage(text(
            locale,
            'AD expansion failed, so the scan completed with the directory ACL dataset.',
            'AD 展开失败，扫描已使用目录 ACL 数据集完成。',
          ));
          setError('');
          appendOperationLog(
            {
              scope: 'scan',
              action: 'run-scan-fallback',
              message: text(
                locale,
                'AD expansion failed and the scan used raw ACL results.',
                'AD 展开失败，扫描已使用原始 ACL 结果。',
              ),
              context: {
                path: requestedPath,
                originalError: nextError,
                permissions: nextResults.length,
                skipped: nextSkipped.length,
              },
            },
            workspaceSettings.auditLogging,
          );
          return;
        } catch (fallbackError) {
          const fallbackMessage = fallbackError instanceof Error ? fallbackError.message : nextError;
          setError(fallbackMessage);
          setLiveScan((current) => ({
            ...current,
            status: 'failed',
            error: fallbackMessage,
            lastUpdatedAt: Date.now(),
            liveConnected: false,
          }));
          appendOperationLog(
            {
              scope: 'scan',
              action: 'run-scan-failed',
              message: fallbackMessage,
              context: { path: requestedPath, unc: requestedIsUNC, adReady },
            },
            workspaceSettings.auditLogging,
          );
          return;
        }
      }

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
    } finally {
      setLoading(false);
      setCancelPending(false);
      window.setTimeout(() => closeScanSocket(), 1200);
    }
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
            onCancelScan={() => void cancelCurrentScan()}
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
            onRescan={() => void runScan(normalizedPath)}
          />
        ) : null}
      </div>
    </>
  );
}
