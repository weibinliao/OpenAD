import Head from 'next/head';
import { type ChangeEvent, type ComponentType, type FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import {
  AlertTriangle,
  CheckCircle2,
  Languages,
  Monitor,
  Moon,
  Pencil,
  Plug,
  Plus,
  RotateCcw,
  Save,
  Star,
  Sun,
  Trash2,
  Upload,
  XCircle,
} from 'lucide-react';
import { cn } from '../lib/cn';
import { useDict } from '../lib/i18n';
import { useI18n } from '../contexts/I18nContext';
import { useThemeContext } from '../contexts/ThemeContext';
import { useADConnection, type ConnectionProfile } from '../contexts/ADConnectionContext';
import { apiBase } from '../lib/runtimeApi';
import {
  buildWorkspacePageTitle,
  defaultWorkspaceSettings,
  isWorkspaceLogoMimeType,
  readWorkspaceSettings,
  resolveWorkspaceBranding,
  WORKSPACE_LOGO_ACCEPT,
  WORKSPACE_LOGO_MAX_BYTES,
  type WorkspaceLogoMimeType,
  type WorkspaceSettings,
  writeWorkspaceSettings,
} from '../lib/workspaceSettings';
import type { ThemeMode } from '../hooks/useTheme';
import { Card, CardHeader, CardContent } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { Input, Label, FieldHint } from '../components/ui/input';
import { Dialog, DialogContent, DialogFooter } from '../components/ui/dialog';
import { EmptyState } from '../components/ui/empty-state';
import { Skeleton } from '../components/ui/skeleton';
import ADConnectionCommandTips from '../components/ADConnectionCommandTips';
import {
  TableContainer,
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '../components/ui/table';

interface ConnectionFormState {
  name: string;
  server: string;
  baseDn: string;
  bindUser: string;
  password: string;
  isDefault: boolean;
}

const emptyConnectionForm: ConnectionFormState = {
  name: '',
  server: '',
  baseDn: '',
  bindUser: '',
  password: '',
  isDefault: false,
};

type WorkspaceIdentityDraft = Pick<WorkspaceSettings, 'workspaceLabel' | 'workspaceTagline'>;

function createWorkspaceIdentityDraft(value: WorkspaceSettings): WorkspaceIdentityDraft {
  return {
    workspaceLabel: value.workspaceLabel,
    workspaceTagline: value.workspaceTagline,
  };
}

function detectWorkspaceLogoMimeType(file: File): WorkspaceLogoMimeType | '' {
  const normalizedType = file.type.trim().toLowerCase();
  if (isWorkspaceLogoMimeType(normalizedType)) {
    return normalizedType;
  }

  const lowerName = file.name.trim().toLowerCase();
  if (lowerName.endsWith('.png')) return 'image/png';
  if (lowerName.endsWith('.jpg') || lowerName.endsWith('.jpeg')) return 'image/jpeg';
  return '';
}

function ToggleRow({
  label,
  desc,
  checked,
  onChange,
}: {
  label: string;
  desc: string;
  checked: boolean;
  onChange: (value: boolean) => void;
}) {
  return (
    <label className="flex items-start justify-between gap-3 rounded-md border border-line bg-surface-base px-3.5 py-2.5">
      <span className="min-w-0">
        <span className="block text-sm font-medium text-fg">{label}</span>
        <span className="mt-0.5 block text-xs text-fg-muted">{desc}</span>
      </span>
      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        className="mt-1 h-4 w-4 shrink-0 accent-[var(--accent,#2563eb)]"
      />
    </label>
  );
}

export default function SettingsPage() {
  const d = useDict();
  const { locale, setLocale, t } = useI18n();
  const { theme, setTheme } = useThemeContext();
  const {
    profiles,
    profilesLoading,
    profilesOffline,
    refreshProfiles,
    activeProfileId,
    setActiveProfileId,
  } = useADConnection();

  // ---------------------------------------------------------------- AD connections
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingProfile, setEditingProfile] = useState<ConnectionProfile | null>(null);
  const [connectionForm, setConnectionForm] = useState<ConnectionFormState>(emptyConnectionForm);
  const [connectionFormError, setConnectionFormError] = useState('');
  const [connectionSaving, setConnectionSaving] = useState(false);
  const [testingProfileId, setTestingProfileId] = useState<string | null>(null);
  const [testResults, setTestResults] = useState<Record<string, { ok: boolean; message: string }>>({});
  const [connectionsActionError, setConnectionsActionError] = useState('');

  const openCreateDialog = () => {
    setEditingProfile(null);
    setConnectionForm(emptyConnectionForm);
    setConnectionFormError('');
    setDialogOpen(true);
  };

  const openEditDialog = (profile: ConnectionProfile) => {
    setEditingProfile(profile);
    setConnectionForm({
      name: profile.name,
      server: profile.server,
      baseDn: profile.base_dn,
      bindUser: profile.bind_user,
      password: '',
      isDefault: profile.is_default,
    });
    setConnectionFormError('');
    setDialogOpen(true);
  };

  const submitConnectionForm = async (event: FormEvent) => {
    event.preventDefault();

    const server = connectionForm.server.trim();
    const baseDn = connectionForm.baseDn.trim();
    const bindUser = connectionForm.bindUser.trim();
    // Name is optional — default it to the server so operators only fill the essentials.
    const name = connectionForm.name.trim() || server;

    if (!server || !bindUser) {
      setConnectionFormError(d.settings.requiredFields);
      return;
    }
    if (!editingProfile && !connectionForm.password) {
      setConnectionFormError(d.settings.passwordRequired);
      return;
    }

    setConnectionSaving(true);
    setConnectionFormError('');
    try {
      const url = editingProfile
        ? `${apiBase()}/api/ad/connections/${editingProfile.id}`
        : `${apiBase()}/api/ad/connections`;
      const response = await fetch(url, {
        method: editingProfile ? 'PUT' : 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name,
          server,
          base_dn: baseDn,
          bind_user: bindUser,
          password: connectionForm.password,
          is_default: connectionForm.isDefault,
        }),
      });
      const data = (await response.json().catch(() => ({}))) as { error?: string; id?: string };
      if (!response.ok) {
        throw new Error(data.error || d.settings.saveFailed);
      }
      // First connection created (no editing, none existed): make it active so
      // the operator can go straight to browsing without an extra "Use" click.
      if (!editingProfile && data.id && profiles.length === 0) {
        setActiveProfileId(data.id);
      }
      setDialogOpen(false);
      await refreshProfiles();
    } catch (error) {
      setConnectionFormError(error instanceof Error ? error.message : d.settings.saveFailed);
    } finally {
      setConnectionSaving(false);
    }
  };

  const testProfile = async (profile: ConnectionProfile) => {
    setTestingProfileId(profile.id);
    setConnectionsActionError('');
    try {
      const response = await fetch(`${apiBase()}/api/ad/connections/${profile.id}/test`, { method: 'POST' });
      const data = (await response.json().catch(() => ({}))) as { ok?: boolean; message?: string; error?: string };
      setTestResults((prev) => ({
        ...prev,
        [profile.id]: {
          ok: Boolean(data.ok),
          message: data.ok ? data.message || d.settings.testPassed : data.error || d.settings.testFailedMsg,
        },
      }));
      await refreshProfiles();
    } catch {
      setTestResults((prev) => ({
        ...prev,
        [profile.id]: { ok: false, message: d.common.backendOffline },
      }));
    } finally {
      setTestingProfileId(null);
    }
  };

  const setDefaultProfile = async (profile: ConnectionProfile) => {
    setConnectionsActionError('');
    try {
      const response = await fetch(`${apiBase()}/api/ad/connections/${profile.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: profile.name,
          server: profile.server,
          base_dn: profile.base_dn,
          bind_user: profile.bind_user,
          password: '',
          is_default: true,
        }),
      });
      const data = (await response.json().catch(() => ({}))) as { error?: string };
      if (!response.ok) {
        throw new Error(data.error || d.settings.saveFailed);
      }
      await refreshProfiles();
    } catch (error) {
      setConnectionsActionError(error instanceof Error ? error.message : d.settings.saveFailed);
    }
  };

  const deleteProfile = async (profile: ConnectionProfile) => {
    if (typeof window !== 'undefined' && !window.confirm(d.settings.deleteConfirm)) {
      return;
    }
    setConnectionsActionError('');
    try {
      const response = await fetch(`${apiBase()}/api/ad/connections/${profile.id}`, { method: 'DELETE' });
      const data = (await response.json().catch(() => ({}))) as { error?: string };
      if (!response.ok) {
        throw new Error(data.error || d.settings.deleteFailed);
      }
      if (activeProfileId === profile.id) {
        setActiveProfileId(null);
      }
      await refreshProfiles();
    } catch (error) {
      setConnectionsActionError(error instanceof Error ? error.message : d.settings.deleteFailed);
    }
  };

  // ---------------------------------------------------------------- global workspace preferences
  const logoInputRef = useRef<HTMLInputElement | null>(null);

  const [workspaceSettings, setWorkspaceSettings] = useState<WorkspaceSettings>(defaultWorkspaceSettings);
  const [workspaceIdentityDraft, setWorkspaceIdentityDraft] = useState<WorkspaceIdentityDraft>(() =>
    createWorkspaceIdentityDraft(defaultWorkspaceSettings)
  );
  const [workspaceSettingsHydrated, setWorkspaceSettingsHydrated] = useState(false);
  const [identityMessage, setIdentityMessage] = useState<{ tone: 'success' | 'info' | 'error'; message: string } | null>(null);

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    const storedWorkspaceSettings = readWorkspaceSettings();
    setWorkspaceSettings(storedWorkspaceSettings);
    setWorkspaceIdentityDraft(createWorkspaceIdentityDraft(storedWorkspaceSettings));
    setWorkspaceSettingsHydrated(true);
  }, []);

  useEffect(() => {
    if (typeof window === 'undefined' || !workspaceSettingsHydrated) {
      return;
    }
    writeWorkspaceSettings(workspaceSettings);
  }, [workspaceSettings, workspaceSettingsHydrated]);

  const savedWorkspaceIdentity = useMemo(() => createWorkspaceIdentityDraft(workspaceSettings), [workspaceSettings]);
  const workspaceIdentityDirty =
    workspaceIdentityDraft.workspaceLabel !== savedWorkspaceIdentity.workspaceLabel ||
    workspaceIdentityDraft.workspaceTagline !== savedWorkspaceIdentity.workspaceTagline;

  const workspaceBranding = useMemo(
    () => resolveWorkspaceBranding(workspaceSettings, t('appTitle')),
    [workspaceSettings, t]
  );

  const handleIdentityChange = <K extends keyof WorkspaceIdentityDraft>(key: K, value: WorkspaceIdentityDraft[K]) => {
    setWorkspaceIdentityDraft((prev) => ({ ...prev, [key]: value }));
    setIdentityMessage(null);
  };

  const handleIdentitySave = (event: FormEvent) => {
    event.preventDefault();
    if (!workspaceIdentityDirty) {
      return;
    }
    const nextIdentity = { ...workspaceIdentityDraft };
    setWorkspaceSettings((prev) => ({ ...prev, ...nextIdentity }));
    setWorkspaceIdentityDraft(nextIdentity);
    setIdentityMessage({ tone: 'success', message: d.settings.identitySaved });
  };

  const handleIdentityRevert = () => {
    if (!workspaceIdentityDirty) {
      return;
    }
    setWorkspaceIdentityDraft(savedWorkspaceIdentity);
    setIdentityMessage({ tone: 'info', message: d.settings.draftReverted });
  };

  const handleLogoUpload = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }

    const mimeType = detectWorkspaceLogoMimeType(file);
    if (!mimeType) {
      setIdentityMessage({ tone: 'error', message: d.settings.logoBadType });
      event.target.value = '';
      return;
    }
    if (file.size > WORKSPACE_LOGO_MAX_BYTES) {
      setIdentityMessage({ tone: 'error', message: d.settings.logoTooLarge });
      event.target.value = '';
      return;
    }

    const reader = new FileReader();
    reader.onload = () => {
      const result = typeof reader.result === 'string' ? reader.result : '';
      const base64Payload = result.split(',')[1] || '';
      if (!base64Payload) {
        setIdentityMessage({ tone: 'error', message: d.settings.logoBadType });
        return;
      }
      setWorkspaceSettings((prev) => ({
        ...prev,
        workspaceLogoDataUrl: `data:${mimeType};base64,${base64Payload}`,
        workspaceLogoMimeType: mimeType,
        workspaceLogoFileName: file.name,
      }));
      setIdentityMessage({ tone: 'success', message: d.settings.logoSaved });
      event.target.value = '';
    };
    reader.onerror = () => {
      setIdentityMessage({ tone: 'error', message: d.settings.logoBadType });
      event.target.value = '';
    };
    reader.readAsDataURL(file);
  };

  const clearLogo = () => {
    if (logoInputRef.current) {
      logoInputRef.current.value = '';
    }
    setWorkspaceSettings((prev) => ({
      ...prev,
      workspaceLogoDataUrl: '',
      workspaceLogoMimeType: '',
      workspaceLogoFileName: '',
    }));
    setIdentityMessage({ tone: 'success', message: d.settings.logoCleared });
  };

  // ---------------------------------------------------------------- render helpers
  const formatTestedAt = (value: string | null | undefined) => {
    if (!value) return '';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return '';
    return date.toLocaleString(locale === 'zh-CN' ? 'zh-CN' : 'en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const themeOptions: Array<{ id: ThemeMode; label: string; icon: ComponentType<{ className?: string }> }> = [
    { id: 'light', label: d.shell.themeLight, icon: Sun },
    { id: 'dark', label: d.shell.themeDark, icon: Moon },
    { id: 'system', label: d.shell.themeSystem, icon: Monitor },
  ];

  return (
    <>
      <Head>
        <title>{buildWorkspacePageTitle(d.settings.title, workspaceSettings, t('appTitle'))}</title>
      </Head>

      <div className="app-v2 mx-auto max-w-6xl space-y-5">
        <div>
          <h1 className="text-lg font-semibold text-fg">{d.settings.title}</h1>
          <p className="mt-0.5 text-sm text-fg-muted">{d.settings.subtitle}</p>
        </div>

        {/* ---------------------------------------------------------- AD connections */}
        <Card>
          <CardHeader
            title={d.settings.adConnections}
            description={d.settings.adConnectionsDesc}
            actions={
              <Button size="sm" onClick={openCreateDialog} disabled={profilesOffline}>
                <Plus className="h-3.5 w-3.5" />
                {d.settings.addConnection}
              </Button>
            }
          />
          <CardContent className="space-y-3">
            {profilesOffline ? (
              <div className="flex items-center gap-2 rounded-lg border border-warning/40 bg-warning-soft px-4 py-2.5 text-sm text-warning">
                <AlertTriangle className="h-4 w-4 shrink-0" />
                {d.settings.connectionsOffline}
              </div>
            ) : null}

            {connectionsActionError ? (
              <div className="flex items-center gap-2 rounded-lg border border-danger/40 bg-danger-soft px-4 py-2.5 text-sm text-danger">
                <XCircle className="h-4 w-4 shrink-0" />
                {connectionsActionError}
              </div>
            ) : null}

            {profilesLoading ? (
              <div className="space-y-2">
                <Skeleton className="h-8 w-full" />
                <Skeleton className="h-8 w-full" />
              </div>
            ) : profiles.length === 0 ? (
              !profilesOffline ? (
                <EmptyState
                  icon={Plug}
                  title={d.settings.noConnections}
                  description={d.settings.noConnectionsHint}
                  action={
                    <Button size="sm" onClick={openCreateDialog}>
                      {d.settings.addConnection}
                    </Button>
                  }
                />
              ) : null
            ) : (
              <TableContainer>
                <Table className="min-w-[920px]">
                  <TableHeader>
                    <TableRow>
                      <TableHead>{d.settings.colName}</TableHead>
                      <TableHead>{d.settings.colServer}</TableHead>
                      <TableHead>{d.settings.colBaseDn}</TableHead>
                      <TableHead>{d.settings.colBindUser}</TableHead>
                      <TableHead>{d.settings.colLastTest}</TableHead>
                      <TableHead className="text-right">{d.settings.colActions}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {profiles.map((profile) => {
                      const isActive = profile.id === activeProfileId;
                      const testResult = testResults[profile.id];
                      return (
                        <TableRow key={profile.id}>
                          <TableCell>
                            <div className="flex flex-wrap items-center gap-1.5">
                              <span className="text-sm font-medium text-fg">{profile.name}</span>
                              {profile.is_default ? <Badge tone="accent">{d.settings.defaultBadge}</Badge> : null}
                              {isActive ? (
                                <Badge tone="info" dot>
                                  {d.settings.activeBadge}
                                </Badge>
                              ) : null}
                            </div>
                            {testResult ? (
                              <p
                                className={cn(
                                  'mt-1 flex items-center gap-1 text-2xs',
                                  testResult.ok ? 'text-success' : 'text-danger'
                                )}
                              >
                                {testResult.ok ? (
                                  <CheckCircle2 className="h-3 w-3 shrink-0" />
                                ) : (
                                  <XCircle className="h-3 w-3 shrink-0" />
                                )}
                                {testResult.message}
                              </p>
                            ) : null}
                          </TableCell>
                          <TableCell>
                            <span className="font-mono text-2xs text-fg-secondary">{profile.server}</span>
                          </TableCell>
                          <TableCell className="max-w-[200px]">
                            <span className="block truncate font-mono text-2xs text-fg-secondary" title={profile.base_dn}>
                              {profile.base_dn}
                            </span>
                          </TableCell>
                          <TableCell>
                            <span className="font-mono text-2xs text-fg-secondary">{profile.bind_user}</span>
                          </TableCell>
                          <TableCell>
                            {profile.last_test_ok === true ? (
                              <Badge tone="success" title={formatTestedAt(profile.last_tested_at)}>
                                {d.settings.testOkBadge}
                              </Badge>
                            ) : profile.last_test_ok === false ? (
                              <Badge tone="danger" title={formatTestedAt(profile.last_tested_at)}>
                                {d.settings.testFailedBadge}
                              </Badge>
                            ) : (
                              <Badge tone="neutral">{d.settings.neverTested}</Badge>
                            )}
                          </TableCell>
                          <TableCell className="text-right">
                            <div className="flex items-center justify-end gap-1">
                              <Button
                                size="sm"
                                variant="outline"
                                loading={testingProfileId === profile.id}
                                onClick={() => void testProfile(profile)}
                              >
                                {d.settings.actionTest}
                              </Button>
                              <Button
                                size="sm"
                                variant={isActive ? 'secondary' : 'outline'}
                                disabled={isActive}
                                onClick={() => setActiveProfileId(profile.id)}
                              >
                                {isActive ? d.settings.activeBadge : d.settings.actionUse}
                              </Button>
                              {!profile.is_default ? (
                                <Button
                                  size="sm"
                                  variant="ghost"
                                  title={d.settings.actionSetDefault}
                                  onClick={() => void setDefaultProfile(profile)}
                                >
                                  <Star className="h-3.5 w-3.5" />
                                </Button>
                              ) : null}
                              <Button
                                size="sm"
                                variant="ghost"
                                title={d.common.edit}
                                onClick={() => openEditDialog(profile)}
                              >
                                <Pencil className="h-3.5 w-3.5" />
                              </Button>
                              <Button
                                size="sm"
                                variant="ghost"
                                title={d.common.delete}
                                className="text-danger hover:text-danger"
                                onClick={() => void deleteProfile(profile)}
                              >
                                <Trash2 className="h-3.5 w-3.5" />
                              </Button>
                            </div>
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

        {/* ---------------------------------------------------------- appearance + language */}
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <Card>
            <CardHeader title={d.settings.appearance} description={d.settings.appearanceDesc} />
            <CardContent>
              <div className="flex flex-wrap gap-2" role="group" aria-label={d.settings.appearance}>
                {themeOptions.map((option) => {
                  const Icon = option.icon;
                  const active = theme === option.id;
                  return (
                    <button
                      key={option.id}
                      type="button"
                      aria-pressed={active}
                      onClick={() => setTheme(option.id)}
                      className={cn(
                        'flex min-w-[96px] flex-col items-center gap-1.5 rounded-md border px-4 py-3 text-xs font-medium transition-colors',
                        active
                          ? 'border-accent bg-accent-soft text-accent-fg'
                          : 'border-line bg-surface-base text-fg-secondary hover:bg-surface-sunken'
                      )}
                    >
                      <Icon className="h-4 w-4" />
                      {option.label}
                    </button>
                  );
                })}
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader title={d.settings.language} description={d.settings.languageDesc} />
            <CardContent>
              <div className="flex flex-wrap gap-2" role="group" aria-label={d.settings.language}>
                {(
                  [
                    ['en', d.settings.langEn],
                    ['zh-CN', d.settings.langZh],
                  ] as Array<['en' | 'zh-CN', string]>
                ).map(([value, label]) => {
                  const active = locale === value;
                  return (
                    <button
                      key={value}
                      type="button"
                      aria-pressed={active}
                      onClick={() => setLocale(value)}
                      className={cn(
                        'flex min-w-[96px] flex-col items-center gap-1.5 rounded-md border px-4 py-3 text-xs font-medium transition-colors',
                        active
                          ? 'border-accent bg-accent-soft text-accent-fg'
                          : 'border-line bg-surface-base text-fg-secondary hover:bg-surface-sunken'
                      )}
                    >
                      <Languages className="h-4 w-4" />
                      {label}
                    </button>
                  );
                })}
              </div>
            </CardContent>
          </Card>
        </div>

        {/* ---------------------------------------------------------- global behavior */}
        <Card>
          <CardHeader title={d.settings.behavior} description={d.settings.behaviorDesc} />
          <CardContent className="grid grid-cols-1 gap-2 lg:grid-cols-2">
            <ToggleRow
              label={d.settings.behaviorAuditLogging}
              desc={d.settings.behaviorAuditLoggingDesc}
              checked={workspaceSettings.auditLogging}
              onChange={(value) => setWorkspaceSettings((prev) => ({ ...prev, auditLogging: value }))}
            />
            <ToggleRow
              label={d.settings.behaviorRememberFilters}
              desc={d.settings.behaviorRememberFiltersDesc}
              checked={workspaceSettings.rememberFilters}
              onChange={(value) => setWorkspaceSettings((prev) => ({ ...prev, rememberFilters: value }))}
            />
            <ToggleRow
              label={d.settings.behaviorVerboseAd}
              desc={d.settings.behaviorVerboseAdDesc}
              checked={workspaceSettings.verboseADLogs}
              onChange={(value) => setWorkspaceSettings((prev) => ({ ...prev, verboseADLogs: value }))}
            />
            <ToggleRow
              label={d.settings.behaviorAutoLoadTree}
              desc={d.settings.behaviorAutoLoadTreeDesc}
              checked={workspaceSettings.autoLoadRootTree}
              onChange={(value) => setWorkspaceSettings((prev) => ({ ...prev, autoLoadRootTree: value }))}
            />
            <ToggleRow
              label={d.settings.behaviorCompactTables}
              desc={d.settings.behaviorCompactTablesDesc}
              checked={workspaceSettings.compactTables}
              onChange={(value) => setWorkspaceSettings((prev) => ({ ...prev, compactTables: value }))}
            />
          </CardContent>
        </Card>

        {/* ---------------------------------------------------------- workspace identity */}
        <Card>
          <CardHeader
            title={d.settings.identity}
            description={d.settings.identityDesc}
            actions={
              <Badge tone={workspaceIdentityDirty ? 'warning' : 'success'}>
                {workspaceIdentityDirty ? d.settings.unsavedDraft : d.settings.identitySynced}
              </Badge>
            }
          />
          <CardContent>
            <form className="space-y-3" onSubmit={handleIdentitySave}>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                <div>
                  <Label htmlFor="settings-workspace-label">{d.settings.workspaceLabel}</Label>
                  <Input
                    id="settings-workspace-label"
                    value={workspaceIdentityDraft.workspaceLabel}
                    onChange={(event) => handleIdentityChange('workspaceLabel', event.target.value)}
                    placeholder={defaultWorkspaceSettings.workspaceLabel}
                  />
                </div>
                <div>
                  <Label htmlFor="settings-workspace-tagline">{d.settings.workspaceTagline}</Label>
                  <Input
                    id="settings-workspace-tagline"
                    value={workspaceIdentityDraft.workspaceTagline}
                    onChange={(event) => handleIdentityChange('workspaceTagline', event.target.value)}
                    placeholder={defaultWorkspaceSettings.workspaceTagline}
                  />
                </div>
              </div>

              <div className="flex flex-wrap items-center gap-2">
                <img
                  src={workspaceBranding.workspaceLogoSrc}
                  alt={d.settings.logoPreview}
                  className="h-8 w-8 rounded-md border border-line bg-surface-sunken object-contain p-1"
                />
                <input
                  ref={logoInputRef}
                  type="file"
                  accept={WORKSPACE_LOGO_ACCEPT}
                  onChange={handleLogoUpload}
                  className="sr-only"
                />
                <Button type="button" size="sm" variant="outline" onClick={() => logoInputRef.current?.click()}>
                  <Upload className="h-3.5 w-3.5" />
                  {d.settings.uploadLogo}
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={!workspaceBranding.usingUploadedLogo}
                  onClick={clearLogo}
                >
                  <RotateCcw className="h-3.5 w-3.5" />
                  {d.settings.clearLogo}
                </Button>
                {workspaceBranding.workspaceLogoFileName ? (
                  <span className="text-2xs text-fg-muted">{workspaceBranding.workspaceLogoFileName}</span>
                ) : null}
                <span className="flex-1" />
                <Button type="button" size="sm" variant="secondary" disabled={!workspaceIdentityDirty} onClick={handleIdentityRevert}>
                  <RotateCcw className="h-3.5 w-3.5" />
                  {d.settings.revert}
                </Button>
                <Button type="submit" size="sm" disabled={!workspaceIdentityDirty}>
                  <Save className="h-3.5 w-3.5" />
                  {d.settings.saveIdentity}
                </Button>
              </div>

              {identityMessage ? (
                <p
                  role="status"
                  aria-live="polite"
                  className={cn(
                    'flex items-center gap-1.5 rounded-md border px-3 py-2 text-xs',
                    identityMessage.tone === 'success' && 'border-success/40 bg-success-soft text-success',
                    identityMessage.tone === 'info' && 'border-info/40 bg-info-soft text-info',
                    identityMessage.tone === 'error' && 'border-danger/40 bg-danger-soft text-danger'
                  )}
                >
                  {identityMessage.tone === 'error' ? (
                    <AlertTriangle className="h-3.5 w-3.5 shrink-0" />
                  ) : (
                    <CheckCircle2 className="h-3.5 w-3.5 shrink-0" />
                  )}
                  {identityMessage.message}
                </p>
              ) : null}
            </form>
          </CardContent>
        </Card>
      </div>

      {/* ---------------------------------------------------------- connection dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        {dialogOpen ? (
          <DialogContent
            title={editingProfile ? d.settings.editConnection : d.settings.addConnection}
            description={d.settings.adConnectionsDesc}
          >
            <form onSubmit={(event) => void submitConnectionForm(event)} className="space-y-3">
              <div>
                <Label htmlFor="connection-server">{d.settings.fieldServer}</Label>
                <Input
                  id="connection-server"
                  value={connectionForm.server}
                  onChange={(event) => setConnectionForm((prev) => ({ ...prev, server: event.target.value }))}
                  placeholder="192.0.2.10"
                  autoFocus
                />
                <FieldHint>{d.settings.fieldServerHint}</FieldHint>
              </div>
              <div>
                <Label htmlFor="connection-bind-user">{d.settings.fieldBindUser}</Label>
                <Input
                  id="connection-bind-user"
                  value={connectionForm.bindUser}
                  onChange={(event) => setConnectionForm((prev) => ({ ...prev, bindUser: event.target.value }))}
                  placeholder="EXAMPLE\\alice"
                />
                <FieldHint>{d.settings.fieldBindUserHint}</FieldHint>
              </div>
              <div>
                <Label htmlFor="connection-password">{d.settings.fieldPassword}</Label>
                <Input
                  id="connection-password"
                  type="password"
                  value={connectionForm.password}
                  onChange={(event) => setConnectionForm((prev) => ({ ...prev, password: event.target.value }))}
                  autoComplete="new-password"
                />
                {editingProfile ? <FieldHint>{d.settings.passwordKeepHint}</FieldHint> : null}
              </div>

              <details className="rounded-md border border-line px-3 py-2">
                <summary className="cursor-pointer text-xs font-medium text-fg-secondary">{d.settings.advanced}</summary>
                <div className="mt-3 space-y-3">
                  <div>
                    <Label htmlFor="connection-name">{d.settings.fieldName}</Label>
                    <Input
                      id="connection-name"
                      value={connectionForm.name}
                      onChange={(event) => setConnectionForm((prev) => ({ ...prev, name: event.target.value }))}
                      placeholder={connectionForm.server || 'DC-01'}
                    />
                  </div>
                  <div>
                    <Label htmlFor="connection-base-dn">{d.settings.fieldBaseDn}</Label>
                    <Input
                      id="connection-base-dn"
                      value={connectionForm.baseDn}
                      onChange={(event) => setConnectionForm((prev) => ({ ...prev, baseDn: event.target.value }))}
                      placeholder="DC=example,DC=com"
                    />
                    <FieldHint>{d.settings.fieldBaseDnHint}</FieldHint>
                  </div>
                  <label className="flex items-center gap-2 text-sm text-fg">
                    <input
                      type="checkbox"
                      checked={connectionForm.isDefault}
                      onChange={(event) => setConnectionForm((prev) => ({ ...prev, isDefault: event.target.checked }))}
                      className="h-4 w-4"
                    />
                    {d.settings.fieldIsDefault}
                  </label>
                </div>
              </details>

              <ADConnectionCommandTips locale={locale} />

              {connectionFormError ? (
                <p className="flex items-center gap-1.5 rounded-md border border-danger/40 bg-danger-soft px-3 py-2 text-xs text-danger">
                  <XCircle className="h-3.5 w-3.5 shrink-0" />
                  {connectionFormError}
                </p>
              ) : null}

              <DialogFooter>
                <Button type="button" variant="secondary" onClick={() => setDialogOpen(false)}>
                  {d.common.cancel}
                </Button>
                <Button type="submit" loading={connectionSaving}>
                  {d.common.save}
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        ) : null}
      </Dialog>
    </>
  );
}
