import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/router';
import {
  GitFork,
  UserRound,
  FolderTree,
  Users,
  AlertTriangle,
  Info,
  ShieldQuestion,
  ChevronDown,
  ChevronRight,
} from 'lucide-react';
import { apiBase } from '../lib/runtimeApi';
import { useDict } from '../lib/i18n';
import { useI18n } from '../contexts/I18nContext';
import { Card, CardContent, CardHeader } from '../components/ui/card';
import { Badge, riskLabel, riskTone } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { EmptyState } from '../components/ui/empty-state';
import { Skeleton } from '../components/ui/skeleton';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/tabs';
import {
  TableContainer,
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '../components/ui/table';

interface Why {
  kind: string;
  description: string;
  group_name?: string;
  group_sid?: string;
  via_chain?: string;
}

interface AccessEntry {
  session_id: string;
  root_path: string;
  path: string;
  rights: string;
  type: string;
  inherited: boolean;
  risk_level?: string;
  trustee_sid: string;
  trustee?: string;
  why: Why;
}

interface ByUserResult {
  user: {
    sid: string;
    sam_account_name: string;
    upn: string;
    display_name: string;
    email: string;
    department: string;
    enabled: boolean;
  };
  group_count: number;
  groups: Array<{ group_sid: string; group_name: string; direct: boolean; via_chain?: string }>;
  sessions: Array<{ id: string; root_path: string; status: string }>;
  entries: AccessEntry[];
  by_root_path: Array<{ root_path: string; session_id: string; entries: AccessEntry[] }>;
  counts: { total: number; direct: number; via_group: number; allow: number; deny: number };
  error?: string;
}

interface ResourcePrincipal {
  sid: string;
  name: string;
  source: string;
  rights: string[];
  types: string[];
  risk_level?: string;
  group_name?: string;
  group_sid?: string;
  via_chain?: string;
  paths: string[];
  enabled?: boolean;
  member_count?: number;
}

interface ByResourceResult {
  path_prefix: string;
  session: { id: string; root_path: string; status: string };
  principals: ResourcePrincipal[];
  aces: Array<{
    path: string;
    trustee?: string;
    trustee_sid: string;
    rights: string;
    type: string;
    inherited: boolean;
    risk_level?: string;
  }>;
  counts: { aces: number; principals: number; groups: number; users: number; via_groups: number; unresolved: number };
  error?: string;
}

export default function AccessAnalysisPage() {
  const d = useDict();
  const router = useRouter();
  const [tab, setTab] = useState('by-user');

  // Deep links: /access?principal=<sid|sam> or /access?path=<prefix>
  useEffect(() => {
    if (!router.isReady) return;
    if (typeof router.query.path === 'string' && router.query.path) setTab('by-resource');
  }, [router.isReady, router.query.path]);

  return (
    <div className="app-v2 mx-auto max-w-7xl space-y-5">
      <div>
        <h1 className="text-lg font-semibold text-fg">{d.access.title}</h1>
        <p className="mt-0.5 text-sm text-fg-muted">{d.access.subtitle}</p>
      </div>

      <div className="flex items-start gap-2 rounded-lg border border-info/30 bg-info-soft px-4 py-2.5 text-xs text-info">
        <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />
        <p>
          <span className="font-semibold">{d.access.hintTitle}: </span>
          {d.access.hint}
        </p>
      </div>

      <Tabs value={tab} onValueChange={setTab}>
        <TabsList>
          <TabsTrigger value="by-user">
            <UserRound className="h-3.5 w-3.5" /> {d.access.byUser}
          </TabsTrigger>
          <TabsTrigger value="by-resource">
            <FolderTree className="h-3.5 w-3.5" /> {d.access.byResource}
          </TabsTrigger>
        </TabsList>
        <TabsContent value="by-user">
          <ByUserTab />
        </TabsContent>
        <TabsContent value="by-resource">
          <ByResourceTab />
        </TabsContent>
      </Tabs>
    </div>
  );
}

function ErrorBanner({ message }: { message: string }) {
  return (
    <div className="flex items-center gap-2 rounded-lg border border-warning/40 bg-warning-soft px-4 py-2.5 text-sm text-warning">
      <AlertTriangle className="h-4 w-4 shrink-0" />
      <span className="min-w-0 break-all">{message}</span>
    </div>
  );
}

function resourceGroupKey(principal: ResourcePrincipal) {
  return (principal.group_sid || (principal.source === 'group' ? principal.sid : '') || principal.group_name || principal.name)
    .trim()
    .toLowerCase();
}

function accessLabel(template: string, values: Record<string, string | number>) {
  return Object.entries(values).reduce((label, [key, value]) => label.replace(`{${key}}`, String(value)), template);
}

function WhyCell({ why, d }: { why: Why; d: ReturnType<typeof useDict> }) {
  if (why.kind === 'direct') {
    return <Badge tone="accent">{d.access.directAce}</Badge>;
  }
  return (
    <span className="inline-flex max-w-[280px] items-center gap-1.5">
      <Badge tone="info" className="shrink-0">
        {d.access.viaGroup}
      </Badge>
      <span className="truncate text-2xs text-fg-muted" title={why.via_chain ? `${why.group_name} (${why.via_chain})` : why.group_name}>
        {why.group_name}
        {why.via_chain ? ` (${why.via_chain})` : ''}
      </span>
    </span>
  );
}

/* --------------------------------- By user --------------------------------- */

function ByUserTab() {
  const d = useDict();
  const { locale } = useI18n();
  const router = useRouter();
  const [principal, setPrincipal] = useState('');
  const [result, setResult] = useState<ByUserResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const analyze = useCallback(
    async (value?: string) => {
      const target = (value ?? principal).trim();
      if (!target) return;
      setLoading(true);
      setError('');
      try {
        const response = await fetch(`${apiBase()}/api/access/by-user`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ principal: target }),
        });
        const data = (await response.json().catch(() => ({}))) as ByUserResult;
        if (!response.ok) throw new Error(data.error || d.common.error);
        setResult(data);
      } catch (err) {
        setResult(null);
        setError(err instanceof Error ? err.message : d.common.error);
      } finally {
        setLoading(false);
      }
    },
    [d, principal],
  );

  // Deep link ?principal=
  useEffect(() => {
    if (!router.isReady) return;
    const fromQuery = typeof router.query.principal === 'string' ? router.query.principal : '';
    if (fromQuery) {
      setPrincipal(fromQuery);
      analyze(fromQuery);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [router.isReady]);

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <Input
          value={principal}
          onChange={(event) => setPrincipal(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') analyze();
          }}
          placeholder={d.access.principalPlaceholder}
          className="w-full sm:max-w-md"
          aria-label={d.access.principalPlaceholder}
        />
        <Button onClick={() => analyze()} loading={loading} disabled={!principal.trim()} className="w-full sm:w-auto">
          <GitFork className="h-3.5 w-3.5" /> {d.access.analyze}
        </Button>
      </div>

      {error ? <ErrorBanner message={error} /> : null}

      {loading ? (
        <div className="space-y-3">
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-48 w-full" />
        </div>
      ) : result ? (
        <>
          {/* Summary row */}
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
            <Card>
              <CardHeader title={d.access.userCard} />
              <CardContent className="space-y-1 text-xs text-fg-secondary">
                <p className="text-sm font-semibold text-fg">
                  {result.user.display_name || result.user.sam_account_name}
                  {!result.user.enabled ? (
                    <Badge tone="danger" className="ml-2">
                      {d.access.disabledUser}
                    </Badge>
                  ) : null}
                </p>
                <p className="font-mono text-2xs">{result.user.sam_account_name}</p>
                {result.user.email ? <p>{result.user.email}</p> : null}
                {result.user.department ? <p>{result.user.department}</p> : null}
                <p className="break-all font-mono text-2xs text-fg-faint">{result.user.sid}</p>
              </CardContent>
            </Card>
            <Card>
              <CardHeader title={`${d.access.groupsCard} (${result.group_count})`} />
              <CardContent className="max-h-40 space-y-1 overflow-auto">
                {result.groups.length === 0 ? (
                  <p className="text-xs text-fg-muted">{d.common.none}</p>
                ) : (
                  result.groups.map((group) => (
                    <div key={group.group_sid} className="flex items-center gap-2 text-xs">
                      <Users className="h-3 w-3 shrink-0 text-fg-muted" />
                      <span className="truncate text-fg-secondary" title={group.via_chain || group.group_name}>
                        {group.group_name}
                      </span>
                      <Badge tone={group.direct ? 'accent' : 'neutral'} className="ml-auto shrink-0">
                        {group.direct ? d.identity.directMembers : d.identity.nestedMembers}
                      </Badge>
                    </div>
                  ))
                )}
              </CardContent>
            </Card>
            <Card>
              <CardHeader title={d.access.sessionsCard} />
              <CardContent className="max-h-40 space-y-1 overflow-auto">
                {result.sessions.map((session) => (
                  <p key={session.id} className="truncate font-mono text-2xs text-fg-secondary" title={session.root_path}>
                    {session.root_path}
                  </p>
                ))}
                <div className="flex gap-3 pt-1 text-2xs text-fg-muted">
                  <span>
                    {d.access.total}: <b className="tabular-nums">{result.counts.total}</b>
                  </span>
                  <span>
                    {d.access.direct}: <b className="tabular-nums">{result.counts.direct}</b>
                  </span>
                  <span>
                    {d.access.viaGroup}: <b className="tabular-nums">{result.counts.via_group}</b>
                  </span>
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Entries grouped by root path */}
          {result.by_root_path.length === 0 ? (
            <Card>
              <EmptyState
                icon={ShieldQuestion}
                title={d.access.noUserEntries}
                description={d.access.noUserEntriesHint}
              />
            </Card>
          ) : (
            result.by_root_path.map((groupEntry) => (
              <Card key={groupEntry.root_path}>
                <CardHeader
                  title={<span className="font-mono text-xs">{groupEntry.root_path}</span>}
                  description={`${groupEntry.entries.length}`}
                />
                <CardContent>
                  <TableContainer className="border-0">
                    <Table className="min-w-[860px]">
                      <TableHeader>
                        <TableRow>
                          <TableHead>{d.access.colPath}</TableHead>
                          <TableHead>{d.access.colRights}</TableHead>
                          <TableHead>{d.access.colType}</TableHead>
                          <TableHead>{d.access.colRisk}</TableHead>
                          <TableHead>{d.access.colWhy}</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {groupEntry.entries.map((entry, index) => (
                          <TableRow key={`${entry.path}-${entry.trustee_sid}-${index}`}>
                            <TableCell className="max-w-[320px]">
                              <span className="block truncate font-mono text-2xs" title={entry.path}>
                                {entry.path}
                              </span>
                            </TableCell>
                            <TableCell className="max-w-[180px]">
                              <span className="block truncate text-2xs" title={entry.rights}>
                                {entry.rights}
                              </span>
                            </TableCell>
                            <TableCell>
                              <Badge tone={entry.type === 'deny' ? 'danger' : 'success'}>
                                {entry.type === 'deny' ? d.access.deny : d.access.allow}
                              </Badge>
                            </TableCell>
                            <TableCell>
                              {entry.risk_level ? <Badge tone={riskTone(entry.risk_level)}>{riskLabel(entry.risk_level, locale)}</Badge> : '—'}
                            </TableCell>
                            <TableCell>
                              <WhyCell why={entry.why} d={d} />
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </TableContainer>
                </CardContent>
              </Card>
            ))
          )}
        </>
      ) : (
        <Card>
          <EmptyState icon={UserRound} title={d.access.emptyByUser} />
        </Card>
      )}
    </div>
  );
}

/* -------------------------------- By resource ------------------------------ */

function ByResourceTab() {
  const d = useDict();
  const { locale } = useI18n();
  const router = useRouter();
  const [path, setPath] = useState('');
  const [result, setResult] = useState<ByResourceResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(() => new Set());

  const analyze = useCallback(
    async (value?: string) => {
      const target = (value ?? path).trim();
      if (!target) return;
      setLoading(true);
      setError('');
      try {
        const response = await fetch(`${apiBase()}/api/access/by-resource`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ path_prefix: target }),
        });
        const data = (await response.json().catch(() => ({}))) as ByResourceResult;
        if (!response.ok) throw new Error(data.error || d.common.error);
        setResult(data);
        setCollapsedGroups(new Set());
      } catch (err) {
        setResult(null);
        setError(err instanceof Error ? err.message : d.common.error);
      } finally {
        setLoading(false);
      }
    },
    [d, path],
  );

  useEffect(() => {
    if (!router.isReady) return;
    const fromQuery = typeof router.query.path === 'string' ? router.query.path : '';
    if (fromQuery) {
      setPath(fromQuery);
      analyze(fromQuery);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [router.isReady]);

  const visiblePrincipals = useMemo(() => {
    if (!result) return [];
    return result.principals.filter(
      (principal) => principal.source !== 'group-member' || !collapsedGroups.has(resourceGroupKey(principal)),
    );
  }, [collapsedGroups, result]);

  const toggleGroup = (principal: ResourcePrincipal) => {
    const key = resourceGroupKey(principal);
    setCollapsedGroups((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  const sourceBadge = (principal: ResourcePrincipal) => {
    switch (principal.source) {
      case 'group':
        return <Badge tone="info">{d.access.sourceGroup}</Badge>;
      case 'user':
        return <Badge tone="accent">{d.access.sourceUser}</Badge>;
      case 'group-member':
        return <Badge tone="neutral">{d.access.sourceGroupMember}</Badge>;
      default:
        return <Badge tone="warning">{d.access.sourceUnresolved}</Badge>;
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <Input
          value={path}
          onChange={(event) => setPath(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') analyze();
          }}
          placeholder={d.access.pathPlaceholder}
          className="w-full font-mono sm:max-w-xl"
          aria-label={d.access.pathPlaceholder}
        />
        <Button onClick={() => analyze()} loading={loading} disabled={!path.trim()} className="w-full sm:w-auto">
          <GitFork className="h-3.5 w-3.5" /> {d.access.analyze}
        </Button>
      </div>

      {error ? <ErrorBanner message={error} /> : null}

      {loading ? (
        <div className="space-y-3">
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-48 w-full" />
        </div>
      ) : result ? (
        <>
          <div className="flex flex-wrap items-center gap-3 text-xs text-fg-secondary">
            <span className="font-mono text-2xs">{result.session.root_path}</span>
            <span>
              {d.access.principalsTitle}: <b className="tabular-nums">{result.counts.principals}</b>
            </span>
            <span>
              {d.access.sourceGroup}: <b className="tabular-nums">{result.counts.groups || 0}</b>
            </span>
            <span>
              {d.access.sourceUser}: <b className="tabular-nums">{result.counts.users}</b>
            </span>
            <span>
              {d.access.sourceGroupMember}: <b className="tabular-nums">{result.counts.via_groups}</b>
            </span>
            <span>
              {d.access.sourceUnresolved}: <b className="tabular-nums">{result.counts.unresolved}</b>
            </span>
          </div>

          <Card>
            <CardHeader title={d.access.principalsTitle} description={result.path_prefix} />
            <CardContent>
              {result.principals.length === 0 ? (
                <EmptyState
                  icon={ShieldQuestion}
                  title={d.access.noResourceEntries}
                  description={d.access.noResourceEntriesHint}
                />
              ) : (
                <TableContainer className="border-0">
                  <Table className="min-w-[980px]">
                    <TableHeader>
                      <TableRow>
                        <TableHead>{d.access.colPrincipal}</TableHead>
                        <TableHead>{d.access.colSource}</TableHead>
                        <TableHead>{d.access.colWhy}</TableHead>
                        <TableHead>{d.access.colRights}</TableHead>
                        <TableHead>{d.access.colType}</TableHead>
                        <TableHead>{d.access.colRisk}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {visiblePrincipals.map((principal, index) => {
                        const isGroup = principal.source === 'group';
                        const isGroupMember = principal.source === 'group-member';
                        const isCollapsed = isGroup && collapsedGroups.has(resourceGroupKey(principal));
                        return (
                          <TableRow
                            key={`${principal.source}-${principal.sid}-${principal.group_name || ''}-${index}`}
                            className={isGroup ? 'bg-surface-sunken/70' : undefined}
                          >
                            <TableCell>
                              <div className={`flex min-w-0 items-center gap-2 ${isGroupMember ? 'pl-8' : ''}`}>
                                {isGroup ? (
                                  <Button
                                    type="button"
                                    variant="ghost"
                                    size="icon"
                                    className="!h-6 !w-6 shrink-0"
                                    aria-expanded={!isCollapsed}
                                    aria-label={accessLabel(isCollapsed ? d.access.expandGroup : d.access.collapseGroup, {
                                      group: principal.name,
                                    })}
                                    title={accessLabel(isCollapsed ? d.access.expandGroup : d.access.collapseGroup, {
                                      group: principal.name,
                                    })}
                                    onClick={() => toggleGroup(principal)}
                                  >
                                    {isCollapsed ? (
                                      <ChevronRight className="h-3.5 w-3.5" />
                                    ) : (
                                      <ChevronDown className="h-3.5 w-3.5" />
                                    )}
                                  </Button>
                                ) : isGroupMember ? (
                                  <UserRound className="h-3.5 w-3.5 shrink-0 text-fg-muted" />
                                ) : null}
                                {isGroup ? <Users className="h-3.5 w-3.5 shrink-0 text-info" /> : null}
                                <div className="min-w-0">
                                  <p className="truncate text-xs font-medium text-fg" title={principal.name}>
                                    {principal.name}
                                    {principal.enabled === false ? (
                                      <Badge tone="danger" className="ml-1.5">
                                        {d.access.disabledUser}
                                      </Badge>
                                    ) : null}
                                  </p>
                                  {principal.sid ? (
                                    <p className="truncate font-mono text-2xs text-fg-faint" title={principal.sid}>
                                      {principal.sid}
                                    </p>
                                  ) : null}
                                </div>
                              </div>
                            </TableCell>
                            <TableCell>{sourceBadge(principal)}</TableCell>
                            <TableCell className="max-w-[240px]">
                              {isGroup ? (
                                <span className="block truncate text-2xs text-fg-muted">
                                  {accessLabel(d.access.membersCount, { count: principal.member_count || 0 })}
                                </span>
                              ) : isGroupMember ? (
                                <span
                                  className="block truncate text-2xs text-fg-muted"
                                  title={principal.via_chain ? `${principal.group_name} (${principal.via_chain})` : principal.group_name}
                                >
                                  {principal.group_name}
                                  {principal.via_chain ? ` (${principal.via_chain})` : ''}
                                </span>
                              ) : principal.source === 'unresolved' ? (
                                <span className="block truncate text-2xs text-fg-muted">{d.access.unresolvedHint}</span>
                              ) : (
                                '—'
                              )}
                            </TableCell>
                            <TableCell className="max-w-[180px]">
                              <span className="block truncate text-2xs" title={principal.rights.join(', ')}>
                                {principal.rights.join(', ')}
                              </span>
                            </TableCell>
                            <TableCell>
                              <span className="flex gap-1">
                                {principal.types.map((type) => (
                                  <Badge key={type} tone={type === 'deny' ? 'danger' : 'success'}>
                                    {type === 'deny' ? d.access.deny : d.access.allow}
                                  </Badge>
                                ))}
                              </span>
                            </TableCell>
                            <TableCell>
                              {principal.risk_level ? (
                                <Badge tone={riskTone(principal.risk_level)}>{riskLabel(principal.risk_level, locale)}</Badge>
                              ) : (
                                '—'
                              )}
                            </TableCell>
                          </TableRow>
                        );
                      })}
                    </TableBody>
                  </Table>
                </TableContainer>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader title={d.access.acesTitle} description={`${result.counts.aces}`} />
            <CardContent>
              <TableContainer className="border-0">
                <Table className="min-w-[900px]">
                  <TableHeader>
                    <TableRow>
                      <TableHead>{d.access.colPath}</TableHead>
                      <TableHead>{d.access.colPrincipal}</TableHead>
                      <TableHead>{d.access.colRights}</TableHead>
                      <TableHead>{d.access.colType}</TableHead>
                      <TableHead>{d.access.colInherited}</TableHead>
                      <TableHead>{d.access.colRisk}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {result.aces.map((ace, index) => (
                      <TableRow key={`${ace.path}-${ace.trustee_sid}-${index}`}>
                        <TableCell className="max-w-[280px]">
                          <span className="block truncate font-mono text-2xs" title={ace.path}>
                            {ace.path}
                          </span>
                        </TableCell>
                        <TableCell className="max-w-[200px]">
                          <span className="block truncate text-2xs" title={ace.trustee || ace.trustee_sid}>
                            {ace.trustee || ace.trustee_sid}
                          </span>
                        </TableCell>
                        <TableCell className="max-w-[160px]">
                          <span className="block truncate text-2xs" title={ace.rights}>
                            {ace.rights}
                          </span>
                        </TableCell>
                        <TableCell>
                          <Badge tone={ace.type === 'deny' ? 'danger' : 'success'}>
                            {ace.type === 'deny' ? d.access.deny : d.access.allow}
                          </Badge>
                        </TableCell>
                        <TableCell>{ace.inherited ? d.common.yes : d.common.no}</TableCell>
                        <TableCell>
                          {ace.risk_level ? <Badge tone={riskTone(ace.risk_level)}>{riskLabel(ace.risk_level, locale)}</Badge> : '—'}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            </CardContent>
          </Card>
        </>
      ) : (
        <Card>
          <EmptyState icon={FolderTree} title={d.access.emptyByResource} />
        </Card>
      )}
    </div>
  );
}
