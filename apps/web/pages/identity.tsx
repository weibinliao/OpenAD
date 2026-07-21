import React, { useCallback, useState } from 'react';
import { useRouter } from 'next/router';
import { AlertTriangle } from 'lucide-react';
import { apiBase } from '../lib/runtimeApi';
import { useDict } from '../lib/i18n';
import { useADConnection } from '../contexts/ADConnectionContext';
import { Card, CardContent, CardHeader } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { QuickConnectCard } from '../components/QuickConnectCard';
import DirectoryExplorerWorkbench from '../components/DirectoryExplorerWorkbench';

function interpolate(template: string, params: Record<string, string | number>) {
  return template.replace(/\{(\w+)\}/g, (_, key) => String(params[key] ?? `{${key}}`));
}

export default function IdentityPage() {
  const d = useDict();
  const router = useRouter();
  const { activeProfile } = useADConnection();
  const connectionId = activeProfile?.id ?? null;
  const initialQuery = Array.isArray(router.query.q)
    ? router.query.q[0] || ''
    : typeof router.query.q === 'string'
      ? router.query.q
      : '';

  if (!connectionId) {
    return (
      <div className="app-v2 mx-auto max-w-[1500px] space-y-5">
        <PageHeader d={d} connectionName={null} />
        <QuickConnectCard />
      </div>
    );
  }

  return (
    <div className="app-v2 mx-auto max-w-[1500px] space-y-5">
      <PageHeader d={d} connectionName={activeProfile?.name ?? null} />
      <DirectoryExplorerWorkbench connectionId={connectionId} initialQuery={initialQuery} />
      <DirectorySyncCard connectionId={connectionId} />
    </div>
  );
}

function PageHeader({ d, connectionName }: { d: ReturnType<typeof useDict>; connectionName: string | null }) {
  return (
    <div className="flex items-start justify-between gap-3">
      <div>
        <h1 className="text-lg font-semibold text-fg">{d.identity.title}</h1>
        <p className="mt-0.5 text-sm text-fg-muted">{d.identity.subtitle}</p>
      </div>
      {connectionName ? (
        <Badge tone="accent" dot>
          {interpolate(d.identity.connectionInUse, { name: connectionName })}
        </Badge>
      ) : null}
    </div>
  );
}

function ErrorBanner({ message }: { message: string }) {
  return (
    <div className="flex items-center gap-2 rounded-lg border border-danger/40 bg-danger-soft px-4 py-2.5 text-sm text-danger">
      <AlertTriangle className="h-4 w-4 shrink-0" />
      <span className="min-w-0 break-all">{message}</span>
    </div>
  );
}

/* ------------------------------ Directory sync ----------------------------- */

interface SyncRun {
  id: string;
  status: string;
  user_count: number;
  group_count: number;
  membership_count: number;
  error_message?: string;
  started_at: string;
  finished_at?: string;
}

function DirectorySyncCard({ connectionId }: { connectionId: string }) {
  const d = useDict();
  const [lastRun, setLastRun] = useState<SyncRun | null>(null);
  const [starting, setStarting] = useState(false);
  const [error, setError] = useState('');

  const loadLastRun = useCallback(async (): Promise<SyncRun | null> => {
    try {
      const response = await fetch(`${apiBase()}/api/ad/sync/runs?page=1&page_size=1`);
      const data = await response.json().catch(() => ({}));
      if (!response.ok) return null;
      const run = Array.isArray(data.items) && data.items.length > 0 ? (data.items[0] as SyncRun) : null;
      setLastRun(run);
      return run;
    } catch {
      return null;
    }
  }, []);

  React.useEffect(() => {
    loadLastRun();
  }, [loadLastRun]);

  // Poll while a run is in flight so counts appear when it completes.
  React.useEffect(() => {
    if (lastRun?.status !== 'running') return;
    const timer = window.setInterval(loadLastRun, 3000);
    return () => window.clearInterval(timer);
  }, [lastRun?.status, loadLastRun]);

  const startSync = useCallback(async () => {
    setStarting(true);
    setError('');
    try {
      const response = await fetch(`${apiBase()}/api/ad/sync`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ connection_id: connectionId }),
      });
      const data = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(data.error || d.common.error);
      await loadLastRun();
    } catch (err) {
      setError(err instanceof Error ? err.message : d.common.error);
    } finally {
      setStarting(false);
    }
  }, [connectionId, d, loadLastRun]);

  const running = lastRun?.status === 'running';

  return (
    <Card>
      <CardHeader
        title={d.identity.syncTitle}
        description={d.identity.syncDesc}
        actions={
          <Button size="sm" onClick={startSync} loading={starting || running} disabled={running}>
            {running ? d.identity.syncRunning : d.identity.syncNow}
          </Button>
        }
      />
      <CardContent>
        {error ? <ErrorBanner message={error} /> : null}
        {!lastRun ? (
          <p className="text-xs text-fg-muted">{d.identity.syncNever}</p>
        ) : (
          <div className="flex flex-wrap items-center gap-x-5 gap-y-1.5 text-xs text-fg-secondary">
            <Badge
              tone={lastRun.status === 'completed' ? 'success' : lastRun.status === 'running' ? 'info' : 'danger'}
              dot
            >
              {lastRun.status}
            </Badge>
            <span>
              {d.identity.syncLast}:{' '}
              {new Date(lastRun.finished_at || lastRun.started_at).toLocaleString()}
            </span>
            <span className="tabular-nums">
              {d.identity.syncUsers} {lastRun.user_count.toLocaleString()}
            </span>
            <span className="tabular-nums">
              {d.identity.syncGroups} {lastRun.group_count.toLocaleString()}
            </span>
            <span className="tabular-nums">
              {d.identity.syncMemberships} {lastRun.membership_count.toLocaleString()}
            </span>
            {lastRun.status === 'failed' && lastRun.error_message ? (
              <span className="text-danger">
                {d.identity.syncFailed}: {lastRun.error_message}
              </span>
            ) : null}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
