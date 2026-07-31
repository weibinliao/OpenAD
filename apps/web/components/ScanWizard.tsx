import React, { useEffect, useState } from 'react';
import { ChevronRight, Folder, Server, Users, Settings } from 'lucide-react';
import { useI18n } from '../contexts/I18nContext';
import { apiBase } from '../lib/runtimeApi';

interface ScanWizardProps {
  onStartScan: (config: ScanConfig) => void | Promise<void>;
  preferredType?: ScanConfig['type'];
}

interface ScanConfig {
  type: 'folder' | 'share' | 'ad';
  path: string;
  depth: number;
  includeInherited: boolean;
  effectivePermissionsEnabled: boolean;
  excludeGroupPatterns: string;
  excludeUserPatterns: string;
  adServer: string;
  adBaseDN: string;
  adUsername: string;
  adPassword: string;
  adQuery: string;
}

interface DirectoryItem {
  name: string;
  path: string;
}

export default function ScanWizard({ onStartScan, preferredType }: ScanWizardProps) {
  const { t } = useI18n();
  const [step, setStep] = useState(1);
  const [config, setConfig] = useState<ScanConfig>({
    type: 'folder',
    path: '',
    depth: 3,
    includeInherited: true,
    effectivePermissionsEnabled: false,
    excludeGroupPatterns: '',
    excludeUserPatterns: '',
    adServer: '',
    adBaseDN: '',
    adUsername: '',
    adPassword: '',
    adQuery: ''
  });
  const [testingConnection, setTestingConnection] = useState(false);
  const [browseOpen, setBrowseOpen] = useState(false);
  const [browsePath, setBrowsePath] = useState('');
  const [browseParentPath, setBrowseParentPath] = useState('');
  const [browseItems, setBrowseItems] = useState<DirectoryItem[]>([]);
  const [browseLoading, setBrowseLoading] = useState(false);
  const [browseError, setBrowseError] = useState('');
  const [connectionStatus, setConnectionStatus] = useState<{ type: 'idle' | 'success' | 'error'; message: string }>({
    type: 'idle',
    message: ''
  });

  const steps = [
    { id: 1, title: t('stepReportType'), icon: Settings },
    { id: 2, title: t('stepDataSource'), icon: Folder },
    { id: 3, title: t('stepConfig'), icon: Users },
  ];

  const updateConfig = (updates: Partial<ScanConfig>) => {
    setConfig((current) => ({ ...current, ...updates }));

    if ('adServer' in updates || 'adBaseDN' in updates || 'adUsername' in updates || 'adPassword' in updates) {
      setConnectionStatus({ type: 'idle', message: '' });
    }
  };

  useEffect(() => {
    if (!preferredType) {
      return;
    }

    setConfig((current) => ({ ...current, type: preferredType }));
    setStep(2);
  }, [preferredType]);

  const isADConnectionConfigured = Boolean(
    config.adServer.trim() &&
    config.adBaseDN.trim() &&
    config.adUsername.trim() &&
    config.adPassword.trim()
  );

  const isStepDisabled =
    (step === 2 && (config.type === 'ad' ? !isADConnectionConfigured : !config.path.trim())) ||
    (step === 3 && config.type !== 'ad' && config.effectivePermissionsEnabled && !isADConnectionConfigured);

  const handleNext = () => {
    if (step < 3) setStep(step + 1);
    else void onStartScan(config);
  };

  const handleTestADConnection = async () => {
    setTestingConnection(true);
    setConnectionStatus({ type: 'idle', message: '' });

    try {
      const response = await fetch(`${apiBase()}/api/ad/test`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          server: config.adServer,
          base_dn: config.adBaseDN,
          username: config.adUsername,
          password: config.adPassword,
        })
      });

      const data = await response.json();
      if (!response.ok) {
        throw new Error(data.error || t('adConnectionFailed'));
      }

      setConnectionStatus({
        type: 'success',
        message: data.message || t('adConnectionSuccess')
      });
    } catch (error) {
      setConnectionStatus({
        type: 'error',
        message: error instanceof Error ? error.message : t('adConnectionFailed')
      });
    } finally {
      setTestingConnection(false);
    }
  };

  const loadDirectories = async (path: string) => {
    setBrowseLoading(true);
    setBrowseError('');
    try {
      const query = path ? `?path=${encodeURIComponent(path)}` : '';
      const response = await fetch(`${apiBase()}/api/fs/directories${query}`);
      const data = await response.json();
      if (!response.ok) {
        throw new Error(data.error || t('loadFailed'));
      }

      setBrowsePath(data.path || '');
      setBrowseParentPath(data.parent || '');
      setBrowseItems((data.items || []) as DirectoryItem[]);
    } catch (error) {
      setBrowseError(error instanceof Error ? error.message : t('loadFailed'));
    } finally {
      setBrowseLoading(false);
    }
  };

  const handleOpenBrowse = async () => {
    setBrowseOpen(true);
    await loadDirectories(config.path.trim());
  };

  const handleSelectBrowsePath = () => {
    updateConfig({ path: browsePath });
    setBrowseOpen(false);
  };

  return (
    <div className="bg-background border border-border rounded-lg p-6">
      {/* Progress Steps */}
      <div className="flex items-center justify-between mb-8">
        {steps.map((s, index) => (
          <div key={s.id} className="flex items-center">
            <div className={`flex items-center justify-center w-10 h-10 rounded-full border-2 ${
              step >= s.id ? 'bg-primary border-primary text-primary-foreground' : 'border-border'
            }`}>
              <s.icon className="w-5 h-5" />
            </div>
            <span className={`ml-2 text-sm ${step >= s.id ? 'text-foreground' : 'text-muted-foreground'}`}>
              {s.title}
            </span>
            {index < steps.length - 1 && (
              <ChevronRight className="w-4 h-4 mx-4 text-muted-foreground" />
            )}
          </div>
        ))}
      </div>

      {/* Step Content */}
      <div className="min-h-[200px]">
        {step === 1 && (
          <div className="space-y-4">
            <h3 className="text-lg font-semibold">{t('selectReportType')}</h3>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              {[
                { type: 'folder', title: t('folderPerm'), desc: t('folderPermDesc'), icon: Folder },
                { type: 'share', title: t('shareAnalysis'), desc: t('shareAnalysisDesc'), icon: Server },
                { type: 'ad', title: t('adUserReport'), desc: t('adUserReportDesc'), icon: Users }
              ].map((option) => (
                <button
                  key={option.type}
                  type="button"
                  onClick={() => updateConfig({ type: option.type as ScanConfig['type'] })}
                  className={`p-4 border rounded-lg text-left transition-colors ${
                    config.type === option.type
                      ? 'border-primary bg-primary/5'
                      : 'border-border hover:border-primary/50'
                  }`}
                >
                  <div className="flex items-center justify-between gap-3">
                    <div className="flex items-center gap-2">
                      <option.icon className="w-5 h-5 text-primary" />
                      <h4 className="font-medium">{option.title}</h4>
                    </div>
                    {option.type === 'ad' && (
                      <span className="text-xs font-medium px-2 py-1 rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200">
                        {t('available')}
                      </span>
                    )}
                  </div>
                  <p className="text-sm text-muted-foreground mt-1">{option.desc}</p>
                </button>
              ))}
            </div>
          </div>
        )}

        {step === 2 && (
          <div className="space-y-4">
            <h3 className="text-lg font-semibold">
              {config.type === 'ad' ? t('connectAD') : t('selectDataSource')}
            </h3>

            {config.type === 'ad' ? (
              <div className="space-y-4">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label htmlFor="ad-server" className="block text-sm font-medium mb-2">{t('adServer')}</label>
                    <input
                      id="ad-server"
                      type="text"
                      value={config.adServer}
                      onChange={(e) => updateConfig({ adServer: e.target.value })}
                      placeholder="dc01.example.com:389 or ldaps://dc01.example.com:636"
                      className="w-full px-3 py-2 border border-border rounded-lg bg-background"
                    />
                  </div>
                  <div>
                    <label htmlFor="ad-base-dn" className="block text-sm font-medium mb-2">{t('baseDN')}</label>
                    <input
                      id="ad-base-dn"
                      type="text"
                      value={config.adBaseDN}
                      onChange={(e) => updateConfig({ adBaseDN: e.target.value })}
                      placeholder="DC=example,DC=com"
                      className="w-full px-3 py-2 border border-border rounded-lg bg-background"
                    />
                  </div>
                  <div>
                    <label htmlFor="ad-username" className="block text-sm font-medium mb-2">{t('bindUsername')}</label>
                    <input
                      id="ad-username"
                      type="text"
                      value={config.adUsername}
                      onChange={(e) => updateConfig({ adUsername: e.target.value })}
                      placeholder="administrator@example.com"
                      className="w-full px-3 py-2 border border-border rounded-lg bg-background"
                    />
                  </div>
                  <div>
                    <label htmlFor="ad-password" className="block text-sm font-medium mb-2">{t('password')}</label>
                    <input
                      id="ad-password"
                      type="password"
                      value={config.adPassword}
                      onChange={(e) => updateConfig({ adPassword: e.target.value })}
                      placeholder={t('enterADPassword')}
                      className="w-full px-3 py-2 border border-border rounded-lg bg-background"
                    />
                  </div>
                </div>

                <div className="flex flex-col md:flex-row md:items-center gap-3">
                  <button
                    type="button"
                    onClick={handleTestADConnection}
                    disabled={!isADConnectionConfigured || testingConnection}
                    className="px-4 py-2 border border-border rounded-lg hover:bg-secondary disabled:opacity-50"
                  >
                    {testingConnection ? t('testingAD') : t('testAD')}
                  </button>

                  {connectionStatus.type !== 'idle' && (
                    <div
                      className={`text-sm px-3 py-2 rounded-lg ${
                        connectionStatus.type === 'success'
                          ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                          : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                      }`}
                    >
                      {connectionStatus.message}
                    </div>
                  )}
                </div>

                <p className="text-sm text-muted-foreground">
                  {t('fillADHint')}
                </p>
              </div>
            ) : (
              <div className="space-y-4">
                <div>
                  <label htmlFor="scan-path" className="block text-sm font-medium mb-2">{t('pathToScan')}</label>
                  <input
                    id="scan-path"
                    type="text"
                    value={config.path}
                    onChange={(e) => updateConfig({ path: e.target.value })}
                    placeholder={'C:\\temp or \\\\server\\share'}
                    className="w-full px-3 py-2 border border-border rounded-lg bg-background"
                  />
                </div>
                <button type="button" className="px-4 py-2 border border-border rounded-lg hover:bg-secondary" onClick={() => void handleOpenBrowse()}>
                  {t('browseFolders')}
                </button>
              </div>
            )}
          </div>
        )}

        {step === 3 && (
          <div className="space-y-4">
            <h3 className="text-lg font-semibold">
              {config.type === 'ad' ? t('adQueryConfig') : t('scanConfig')}
            </h3>

            {config.type === 'ad' ? (
              <div className="space-y-4">
                <div>
                  <label htmlFor="ad-query" className="block text-sm font-medium mb-2">{t('userSearchQuery')}</label>
                  <input
                    id="ad-query"
                    type="text"
                    value={config.adQuery}
                    onChange={(e) => updateConfig({ adQuery: e.target.value })}
                    placeholder={t('userSearchHint')}
                    className="w-full px-3 py-2 border border-border rounded-lg bg-background"
                  />
                </div>

                <div className="rounded-lg border border-border bg-secondary/20 p-4 text-sm text-muted-foreground">
                  {t('queryExamples')}: <span className="font-mono">jdoe</span>, <span className="font-mono">John</span>, <span className="font-mono">john@example.com</span>。
                  {t('queryExamplesDesc')}
                </div>
              </div>
            ) : (
              <div className="space-y-4">
                <div>
                  <label htmlFor="scan-depth" className="block text-sm font-medium mb-2">{t('scanDepth')}</label>
                  <select
                    id="scan-depth"
                    value={config.depth}
                    onChange={(e) => updateConfig({ depth: parseInt(e.target.value) })}
                    className="w-full px-3 py-2 border border-border rounded-lg bg-background"
                  >
                    <option value={1}>{t('level1')}</option>
                    <option value={3}>{t('level3')}</option>
                    <option value={5}>{t('level5')}</option>
                    <option value={-1}>{t('unlimited')}</option>
                  </select>
                </div>
                <div className="flex items-center space-x-2">
                  <input
                    type="checkbox"
                    id="inherited"
                    checked={config.includeInherited}
                    onChange={(e) => updateConfig({ includeInherited: e.target.checked })}
                    className="rounded"
                  />
                  <label htmlFor="inherited" className="text-sm">{t('includeInheritedPerm')}</label>
                </div>

                <div className="rounded-lg border border-border p-4">
                  <div className="flex items-center space-x-2">
                    <input
                      type="checkbox"
                      id="effective-permissions"
                      checked={config.effectivePermissionsEnabled}
                      onChange={(e) => updateConfig({ effectivePermissionsEnabled: e.target.checked })}
                      className="rounded"
                    />
                    <label htmlFor="effective-permissions" className="text-sm font-medium">
                      {t('resolveEffective')}
                    </label>
                  </div>

                  {config.effectivePermissionsEnabled && (
                    <div className="mt-4 space-y-4">
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div>
                          <label htmlFor="scan-ad-server" className="block text-sm font-medium mb-2">{t('adServer')}</label>
                          <input
                            id="scan-ad-server"
                            type="text"
                            value={config.adServer}
                            onChange={(e) => updateConfig({ adServer: e.target.value })}
                            placeholder="dc01.example.com:389 or ldaps://dc01.example.com:636"
                            className="w-full px-3 py-2 border border-border rounded-lg bg-background"
                          />
                        </div>
                        <div>
                          <label htmlFor="scan-ad-base-dn" className="block text-sm font-medium mb-2">{t('baseDN')}</label>
                          <input
                            id="scan-ad-base-dn"
                            type="text"
                            value={config.adBaseDN}
                            onChange={(e) => updateConfig({ adBaseDN: e.target.value })}
                            placeholder="DC=example,DC=com"
                            className="w-full px-3 py-2 border border-border rounded-lg bg-background"
                          />
                        </div>
                        <div>
                          <label htmlFor="scan-ad-username" className="block text-sm font-medium mb-2">{t('bindUsername')}</label>
                          <input
                            id="scan-ad-username"
                            type="text"
                            value={config.adUsername}
                            onChange={(e) => updateConfig({ adUsername: e.target.value })}
                            placeholder="administrator@example.com"
                            className="w-full px-3 py-2 border border-border rounded-lg bg-background"
                          />
                        </div>
                        <div>
                          <label htmlFor="scan-ad-password" className="block text-sm font-medium mb-2">{t('password')}</label>
                          <input
                            id="scan-ad-password"
                            type="password"
                            value={config.adPassword}
                            onChange={(e) => updateConfig({ adPassword: e.target.value })}
                            placeholder={t('enterADPassword')}
                            className="w-full px-3 py-2 border border-border rounded-lg bg-background"
                          />
                        </div>
                      </div>

                      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div>
                          <label htmlFor="exclude-groups" className="block text-sm font-medium mb-2">{t('excludeGroupPatterns')}</label>
                          <textarea
                            id="exclude-groups"
                            value={config.excludeGroupPatterns}
                            onChange={(e) => updateConfig({ excludeGroupPatterns: e.target.value })}
                            placeholder={`${t('onePatternPerLine')}&#10;BUILTIN\\*&#10;*Operators*`}
                            rows={4}
                            className="w-full px-3 py-2 border border-border rounded-lg bg-background font-mono text-sm"
                          />
                        </div>
                        <div>
                          <label htmlFor="exclude-users" className="block text-sm font-medium mb-2">{t('excludeUserPatterns')}</label>
                          <textarea
                            id="exclude-users"
                            value={config.excludeUserPatterns}
                            onChange={(e) => updateConfig({ excludeUserPatterns: e.target.value })}
                            placeholder={`${t('onePatternPerLine')}&#10;svc_*&#10;backup_*`}
                            rows={4}
                            className="w-full px-3 py-2 border border-border rounded-lg bg-background font-mono text-sm"
                          />
                        </div>
                      </div>

                      <div className="flex flex-col md:flex-row md:items-center gap-3">
                        <button
                          type="button"
                          onClick={handleTestADConnection}
                          disabled={!isADConnectionConfigured || testingConnection}
                          className="px-4 py-2 border border-border rounded-lg hover:bg-secondary disabled:opacity-50"
                        >
                          {testingConnection ? t('testingAD') : t('testAD')}
                        </button>

                        {connectionStatus.type !== 'idle' && (
                          <div
                            className={`text-sm px-3 py-2 rounded-lg ${
                              connectionStatus.type === 'success'
                                ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                                : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                            }`}
                          >
                            {connectionStatus.message}
                          </div>
                        )}
                      </div>
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {browseOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
          <div
            role="dialog"
            aria-modal="true"
            aria-labelledby="directory-browser-title"
            className="flex max-h-[calc(100dvh-2rem)] w-full max-w-2xl flex-col overflow-hidden rounded-lg border border-border bg-background shadow-xl"
          >
            <div className="flex shrink-0 items-center justify-between border-b border-border px-4 py-3">
              <h4 id="directory-browser-title" className="text-base font-semibold">{t('browseTitle')}</h4>
              <button type="button" className="rounded px-2 py-1 text-sm hover:bg-secondary" onClick={() => setBrowseOpen(false)}>
                {t('cancel')}
              </button>
            </div>
            <div
              data-scroll-region="dialog-body"
              className="min-h-0 flex-1 space-y-3 overflow-y-auto overscroll-contain p-4"
            >
              <div className="text-sm text-muted-foreground">
                {t('browseCurrentPath')}: <span className="font-mono text-foreground">{browsePath || '-'}</span>
              </div>
              <div className="flex gap-2">
                <button
                  type="button"
                  disabled={browseLoading || !browseParentPath || browseParentPath === browsePath}
                  onClick={() => void loadDirectories(browseParentPath)}
                  className="rounded border border-border px-3 py-2 text-sm hover:bg-secondary disabled:opacity-50"
                >
                  {t('goUp')}
                </button>
                <button
                  type="button"
                  disabled={browseLoading || !browsePath}
                  onClick={handleSelectBrowsePath}
                  className="rounded bg-primary px-3 py-2 text-sm text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
                >
                  {t('selectCurrentPath')}
                </button>
              </div>

              {browseError && <div className="text-sm text-red-600">{browseError}</div>}
              {browseLoading ? (
                <div className="text-sm text-muted-foreground">{t('loadingDetails')}</div>
              ) : (
                <div className="rounded border border-border">
                  {browseItems.length === 0 ? (
                    <div className="px-3 py-4 text-sm text-muted-foreground">{t('noSubdirectories')}</div>
                  ) : (
                    browseItems.map((item) => (
                      <div key={item.path} className="flex items-center justify-between border-b border-border/60 px-3 py-2 last:border-b-0">
                        <div className="truncate text-sm" title={item.path}>
                          {item.name}
                        </div>
                        <button
                          type="button"
                          onClick={() => void loadDirectories(item.path)}
                          className="rounded bg-secondary px-2 py-1 text-xs hover:bg-secondary/80"
                        >
                          {t('openFolder')}
                        </button>
                      </div>
                    ))
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Navigation */}
      <div className="flex justify-between mt-8">
        <button
          type="button"
          onClick={() => setStep(Math.max(1, step - 1))}
          disabled={step === 1}
          className="px-4 py-2 border border-border rounded-lg disabled:opacity-50"
        >
          {t('back')}
        </button>
        <button
          type="button"
          onClick={handleNext}
          disabled={isStepDisabled}
          className="px-6 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 disabled:opacity-50"
        >
          {step === 3
            ? config.type === 'ad'
              ? (config.adQuery.trim() ? t('runAdQuery') : t('loadRootTree'))
              : t('startScan')
            : t('next')}
        </button>
      </div>
    </div>
  );
}
