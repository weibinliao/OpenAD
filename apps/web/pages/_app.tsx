import '../styles/tokens.css';
import '../styles/base.css';
import '../styles/desktop-shell.css';
import '../styles/desktop-theme.css';
import '../styles/openad-operations.css';
import type { AppProps } from 'next/app';
import Head from 'next/head';
import { useEffect, useState } from 'react';
import { I18nProvider, useI18n } from '../contexts/I18nContext';
import { ADConnectionProvider } from '../contexts/ADConnectionContext';
import { ThemeProvider } from '../contexts/ThemeContext';
import AppShellV2 from '../components/shell/AppShellV2';
import {
  WORKSPACE_SETTINGS_UPDATED_EVENT,
  defaultWorkspaceSettings,
  readWorkspaceSettings,
  resolveWorkspaceBranding,
  type WorkspaceSettings,
} from '../lib/workspaceSettings';

type DesktopPageComponent = AppProps['Component'] & { desktopExperience?: boolean };

function AppFrame({ Component, pageProps }: Pick<AppProps, 'Component' | 'pageProps'>) {
  const { t } = useI18n();
  const [workspaceSettings, setWorkspaceSettings] = useState<WorkspaceSettings>(defaultWorkspaceSettings);

  useEffect(() => {
    if (typeof window === 'undefined') return;

    const syncWorkspaceSettings = () => {
      setWorkspaceSettings(readWorkspaceSettings());
    };

    syncWorkspaceSettings();
    window.addEventListener('storage', syncWorkspaceSettings);
    window.addEventListener(WORKSPACE_SETTINGS_UPDATED_EVENT, syncWorkspaceSettings as EventListener);

    return () => {
      window.removeEventListener('storage', syncWorkspaceSettings);
      window.removeEventListener(WORKSPACE_SETTINGS_UPDATED_EVENT, syncWorkspaceSettings as EventListener);
    };
  }, []);

  const branding = resolveWorkspaceBranding(workspaceSettings, t('appTitle'));
  const iconHref = branding.workspaceLogoSrc;
  const iconType = branding.workspaceLogoType;
  const productName = 'OpenAD';
  const PageComponent = Component as DesktopPageComponent;
  const page = <Component {...pageProps} />;

  return (
    <>
      <Head>
        <title>{productName}</title>
        <meta name="application-name" content={productName} />
        <meta name="description" content={branding.workspaceTagline} />
        <meta property="og:site_name" content={productName} />
        <meta property="og:title" content={productName} />
        <meta property="og:description" content={branding.workspaceTagline} />
        <meta name="twitter:card" content="summary" />
        <meta name="twitter:title" content={productName} />
        <meta name="twitter:description" content={branding.workspaceTagline} />
        <link rel="icon" href={iconHref} type={iconType} />
        <link rel="shortcut icon" href={iconHref} />
      </Head>

      {PageComponent.desktopExperience ? (
        page
      ) : (
        <AppShellV2 workspaceSettings={workspaceSettings}>{page}</AppShellV2>
      )}
    </>
  );
}

export default function App({ Component, pageProps }: AppProps) {
  return (
    <ThemeProvider>
      <I18nProvider>
        <ADConnectionProvider>
          <AppFrame Component={Component} pageProps={pageProps} />
        </ADConnectionProvider>
      </I18nProvider>
    </ThemeProvider>
  );
}
