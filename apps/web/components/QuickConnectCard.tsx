import React, { useState } from 'react';
import { PlugZap, CheckCircle2, XCircle, Loader2 } from 'lucide-react';
import { apiBase } from '../lib/runtimeApi';
import { cn } from '../lib/cn';
import { useDict } from '../lib/i18n';
import { useADConnection } from '../contexts/ADConnectionContext';
import { Card, CardContent, CardHeader } from './ui/card';
import { Button } from './ui/button';
import { Input, Label, FieldHint } from './ui/input';

/**
 * Inline "connect in three fields" card shown wherever an AD connection is
 * required but none is active (Identity, Access). Server + account + password;
 * the Base DN is auto-discovered server-side. On success the new profile is
 * created, set active, and onConnected() fires so the page can reveal itself.
 */
export function QuickConnectCard({ onConnected, className }: { onConnected?: () => void; className?: string }) {
  const d = useDict();
  const { refreshProfiles, setActiveProfileId } = useADConnection();
  const [server, setServer] = useState('');
  const [user, setUser] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [discovered, setDiscovered] = useState<string>('');

  const connect = async () => {
    const s = server.trim();
    const u = user.trim();
    if (!s || !u || !password || busy) return;
    setBusy(true);
    setError('');
    setDiscovered('');
    try {
      // Verify + discover Base DN first so failures are reported before we store anything.
      const discoverRes = await fetch(`${apiBase()}/api/ad/discover`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server: s, username: u, password }),
      });
      const discoverData = await discoverRes.json().catch(() => ({}));
      if (!discoverRes.ok) throw new Error(discoverData.error || d.settings.testFailedMsg);
      setDiscovered(discoverData.base_dn || '');

      const createRes = await fetch(`${apiBase()}/api/ad/connections`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: s,
          server: s,
          base_dn: discoverData.base_dn || '',
          bind_user: discoverData.bind_user || u,
          password,
          is_default: true,
        }),
      });
      const createData = await createRes.json().catch(() => ({}));
      if (!createRes.ok) throw new Error(createData.error || d.settings.saveFailed);

      if (createData.id) setActiveProfileId(createData.id);
      await refreshProfiles();
      onConnected?.();
    } catch (err) {
      setError(err instanceof Error ? err.message : d.common.error);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card className={cn('mx-auto max-w-lg', className)}>
      <CardHeader
        className="openad-quick-connect-header"
        title={
          <span className="inline-flex items-center gap-2">
            <PlugZap className="h-4 w-4 text-accent-fg" /> {d.quickConnect.title}
          </span>
        }
        description={d.quickConnect.subtitle}
      />
      <CardContent className="openad-quick-connect-content space-y-3">
        <div>
          <Label htmlFor="qc-server">{d.settings.fieldServer}</Label>
          <Input
            id="qc-server"
            value={server}
            onChange={(event) => setServer(event.target.value)}
            placeholder="192.0.2.10"
            autoFocus
          />
        </div>
        <div>
          <Label htmlFor="qc-user">{d.settings.fieldBindUser}</Label>
          <Input
            id="qc-user"
            value={user}
            onChange={(event) => setUser(event.target.value)}
            placeholder="EXAMPLE\\alice"
          />
          <FieldHint>{d.settings.fieldBindUserHint}</FieldHint>
        </div>
        <div>
          <Label htmlFor="qc-pass">{d.settings.fieldPassword}</Label>
          <Input
            id="qc-pass"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') connect();
            }}
            autoComplete="new-password"
          />
        </div>

        {error ? (
          <p className="flex items-start gap-1.5 rounded-md border border-danger/40 bg-danger-soft px-3 py-2 text-xs text-danger">
            <XCircle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            <span className="min-w-0 break-all">{error}</span>
          </p>
        ) : null}
        {discovered && !error ? (
          <p className="flex items-center gap-1.5 rounded-md border border-success/40 bg-success-soft px-3 py-2 text-xs text-success">
            <CheckCircle2 className="h-3.5 w-3.5 shrink-0" />
            {d.quickConnect.discovered}: <span className="font-mono">{discovered}</span>
          </p>
        ) : null}

        <Button className="w-full" onClick={connect} loading={busy} disabled={!server.trim() || !user.trim() || !password}>
          {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <PlugZap className="h-3.5 w-3.5" />}
          {d.quickConnect.connect}
        </Button>
        <p className="text-center text-2xs text-fg-muted">{d.quickConnect.hint}</p>
      </CardContent>
    </Card>
  );
}
