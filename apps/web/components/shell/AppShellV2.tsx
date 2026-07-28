import React, { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/router';
import {
  CornerDownLeft,
  Languages,
  Loader2,
  Monitor,
  Moon,
  PanelLeftClose,
  PanelLeftOpen,
  Search,
  Server,
  Sun,
  UserRound,
  Users,
} from 'lucide-react';
import { cn } from '../../lib/cn';
import { useDict } from '../../lib/i18n';
import { useI18n } from '../../contexts/I18nContext';
import { useThemeContext } from '../../contexts/ThemeContext';
import { useADConnection } from '../../contexts/ADConnectionContext';
import { useRuntimeHealth } from '../../hooks/useRuntimeHealth';
import { buildOpenADNavigation, matchesOpenADRoute } from '../../lib/openadNavigation';
import { Badge } from '../ui/badge';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '../ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '../ui/tooltip';
import {
  defaultWorkspaceSettings,
  resolveWorkspaceBranding,
  type WorkspaceSettings,
} from '../../lib/workspaceSettings';
import DesktopWindowFrame, { useDesktopOSStyle } from './DesktopWindowFrame';
import {
  buildWorkspaceCommands,
  findWorkspaceCommandMatches,
  resolveWorkspaceSearch,
  type WorkspaceCommand,
} from '../../lib/workspaceSearch';
import {
  directoryGroupName,
  searchDirectoryObjects,
  type DirectoryMatch,
} from '../../lib/directorySearch';

const NAV_PINNED_KEY = 'permission-protector.desktop-nav-pinned';

type GlobalSearchOption =
  | { kind: 'command'; command: WorkspaceCommand }
  | { kind: 'directory'; match: DirectoryMatch };

export default function AppShellV2({
  children,
  workspaceSettings,
}: {
  children: ReactNode;
  workspaceSettings?: WorkspaceSettings;
}) {
  const d = useDict();
  const { t, locale, setLocale } = useI18n();
  const { theme, resolvedTheme, setTheme } = useThemeContext();
  const { connection, activeProfile } = useADConnection();
  const router = useRouter();
  const searchRef = useRef<HTMLInputElement>(null);
  const workspaceContentRef = useRef<HTMLElement>(null);
  const [query, setQuery] = useState('');
  const [searchMatches, setSearchMatches] = useState<DirectoryMatch[]>([]);
  const [searchLoading, setSearchLoading] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchActiveIndex, setSearchActiveIndex] = useState(-1);
  const [navHovered, setNavHovered] = useState(false);
  const [navFocused, setNavFocused] = useState(false);
  const [navPinned, setNavPinned] = useState(false);
  const [desktopLogoDataUrl, setDesktopLogoDataUrl] = useState('');
  const [osStyle, setOSStyle] = useDesktopOSStyle('windows');
  const runtimeHealth = useRuntimeHealth();

  const isChinese = locale === 'zh-CN';
  const copy = (en: string, zh: string) => (isChinese ? zh : en);
  const effectiveWorkspaceSettings = workspaceSettings || defaultWorkspaceSettings;
  const branding = resolveWorkspaceBranding(effectiveWorkspaceSettings, t('appTitle'));
  const productName = 'OpenAD';
  const titleBrand = branding.workspaceLabel === defaultWorkspaceSettings.workspaceLabel
    || branding.workspaceLabel === productName
    ? productName
    : branding.workspaceLabel;
  const desktopTheme = resolvedTheme === 'light' ? 'light' : 'dark';
  const navExpanded = navPinned || navHovered || navFocused;
  const runtimeLabel = runtimeHealth === 'healthy'
    ? d.shell.runtimeHealthy
    : runtimeHealth === 'checking'
      ? d.shell.runtimeChecking
      : d.shell.runtimeOffline;

  const groups = useMemo(
    () => buildOpenADNavigation(isChinese ? 'zh-CN' : 'en'),
    [isChinese],
  );
  const allItems = useMemo(() => groups.flatMap((group) => group.items), [groups]);
  const workspaceCommands = useMemo(
    () => buildWorkspaceCommands(isChinese ? 'zh-CN' : 'en'),
    [isChinese],
  );
  const commandMatches = useMemo(
    () => query.trim().startsWith('/')
      ? findWorkspaceCommandMatches(query, workspaceCommands)
      : [],
    [query, workspaceCommands],
  );
  const searchOptions = useMemo<GlobalSearchOption[]>(() => (
    query.trim().startsWith('/')
      ? commandMatches.map((command) => ({ kind: 'command', command }))
      : searchMatches.map((match) => ({ kind: 'directory', match }))
  ), [commandMatches, query, searchMatches]);
  const activeItem = allItems.find((item) => matchesOpenADRoute(item, router.pathname)) || allItems[0];
  const ActiveIcon = activeItem.icon;

  useEffect(() => {
    try {
      setNavPinned(window.localStorage.getItem(NAV_PINNED_KEY) === 'true');
    } catch {
      setNavPinned(false);
    }
  }, []);

  useEffect(() => {
    if (workspaceContentRef.current) {
      workspaceContentRef.current.scrollTop = 0;
    }
  }, [router.pathname]);

  useEffect(() => {
    if (branding.usingUploadedLogo) {
      setDesktopLogoDataUrl(branding.workspaceLogoSrc);
      return;
    }
    if (typeof fetch !== 'function') {
      return;
    }

    let cancelled = false;
    fetch(branding.workspaceLogoSrc)
      .then((response) => {
        if (!response.ok) throw new Error('Unable to load the default OpenAD logo.');
        return response.blob();
      })
      .then((blob) => new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : '');
        reader.onerror = () => reject(reader.error);
        reader.readAsDataURL(blob);
      }))
      .then((dataUrl) => {
        if (!cancelled) setDesktopLogoDataUrl(dataUrl);
      })
      .catch(() => {});

    return () => {
      cancelled = true;
    };
  }, [branding.usingUploadedLogo, branding.workspaceLogoSrc]);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        searchRef.current?.focus();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  useEffect(() => {
    const value = query.trim();
    if (value.startsWith('/')) {
      setSearchMatches([]);
      setSearchLoading(false);
      setSearchOpen(true);
      setSearchActiveIndex(commandMatches.length > 0 ? 0 : -1);
      return;
    }

    const isDirectoryQuery = Boolean(
      activeProfile?.id
      && value.length >= 2
      && !value.includes('\\')
      && !value.includes('/'),
    );
    if (!isDirectoryQuery || !activeProfile?.id) {
      setSearchMatches([]);
      setSearchLoading(false);
      setSearchOpen(false);
      setSearchActiveIndex(-1);
      return;
    }

    const controller = new AbortController();
    setSearchLoading(true);
    const timer = window.setTimeout(() => {
      void searchDirectoryObjects({
        connectionId: activeProfile.id,
        query: value,
        limit: 6,
        signal: controller.signal,
      }).then(({ matches }) => {
        setSearchMatches(matches);
        setSearchOpen(true);
        setSearchActiveIndex(matches.length > 0 ? 0 : -1);
      }).catch((searchError) => {
        if (searchError instanceof DOMException && searchError.name === 'AbortError') return;
        setSearchMatches([]);
        setSearchOpen(true);
        setSearchActiveIndex(-1);
      }).finally(() => {
        if (!controller.signal.aborted) setSearchLoading(false);
      });
    }, 220);

    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [activeProfile?.id, commandMatches.length, query]);

  const toggleNavPinned = () => {
    setNavPinned((current) => {
      const next = !current;
      try {
        window.localStorage.setItem(NAV_PINNED_KEY, String(next));
      } catch {
        // Keep the session state when persistent storage is unavailable.
      }
      return next;
    });
  };

  const runQuery = () => {
    const intent = resolveWorkspaceSearch(query, isChinese ? 'zh-CN' : 'en');
    if (intent.kind === 'empty') return;
    if (intent.kind === 'invalid-command') {
      setSearchOpen(true);
      setSearchActiveIndex(-1);
      return;
    }
    if (intent.kind === 'module') {
      setSearchOpen(false);
      void router.push(intent.href);
      return;
    }
    if (intent.kind === 'path') {
      setSearchOpen(false);
      void router.push(`/access?path=${encodeURIComponent(intent.path)}`);
      return;
    }
    setSearchOpen(false);
    void router.push(`/identity?q=${encodeURIComponent(intent.query)}`);
  };

  const selectGlobalMatch = (match: DirectoryMatch) => {
    const value = match.kind === 'user'
      ? match.user.username || match.label
      : directoryGroupName(match.group);
    setQuery(value);
    setSearchOpen(false);
    setSearchActiveIndex(-1);
    void router.push(`/identity?q=${encodeURIComponent(value)}`);
  };

  const selectModuleCommand = (command: WorkspaceCommand) => {
    setQuery(command.command);
    setSearchOpen(false);
    setSearchActiveIndex(-1);
    void router.push(command.href);
  };

  const selectSearchOption = (option: GlobalSearchOption) => {
    if (option.kind === 'command') selectModuleCommand(option.command);
    else selectGlobalMatch(option.match);
  };

  const handleGlobalSearchKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'ArrowDown' && searchOptions.length > 0) {
      event.preventDefault();
      setSearchOpen(true);
      setSearchActiveIndex((current) => (current + 1) % searchOptions.length);
      return;
    }
    if (event.key === 'ArrowUp' && searchOptions.length > 0) {
      event.preventDefault();
      setSearchOpen(true);
      setSearchActiveIndex((current) => (current <= 0 ? searchOptions.length - 1 : current - 1));
      return;
    }
    if (event.key === 'Escape') {
      setSearchOpen(false);
      setSearchActiveIndex(-1);
      return;
    }
    if (event.key === 'Enter') {
      event.preventDefault();
      const activeOption = searchOpen && searchActiveIndex >= 0
        ? searchOptions[searchActiveIndex]
        : null;
      if (activeOption) selectSearchOption(activeOption);
      else runQuery();
    }
  };

  const themeOptions = [
    { key: 'light' as const, label: d.shell.themeLight, icon: Sun },
    { key: 'dark' as const, label: d.shell.themeDark, icon: Moon },
    { key: 'system' as const, label: d.shell.themeSystem, icon: Monitor },
  ];

  return (
    <TooltipProvider delayDuration={220}>
      <DesktopWindowFrame
        theme={desktopTheme}
        osStyle={osStyle}
        title={titleBrand}
        logoSrc={branding.workspaceLogoSrc}
        logoDataUrl={desktopLogoDataUrl}
        contentClassName="pp-desktop-frame-content"
      >
        <div className="pp-desktop-shell">
          <div
            className={cn('pp-nav-slot', navExpanded && 'is-expanded', navPinned && 'is-pinned')}
            onPointerEnter={() => setNavHovered(true)}
            onPointerLeave={() => setNavHovered(false)}
            onFocusCapture={() => setNavFocused(true)}
            onBlurCapture={(event) => {
              if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
                setNavFocused(false);
              }
            }}
          >
            <aside
              className={cn('pp-nav-rail', navExpanded && 'is-expanded')}
              aria-label={copy('OpenAD product navigation', 'OpenAD 产品导航')}
            >
              {navExpanded ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button
                      type="button"
                      className="pp-rail-pin"
                      aria-label={navPinned ? d.shell.unpinSidebar : d.shell.pinSidebar}
                      aria-pressed={navPinned}
                      onClick={toggleNavPinned}
                    >
                      {navPinned
                        ? <PanelLeftClose className="h-3.5 w-3.5" aria-hidden />
                        : <PanelLeftOpen className="h-3.5 w-3.5" aria-hidden />}
                    </button>
                  </TooltipTrigger>
                  <TooltipContent side="right">
                    {navPinned ? d.shell.unpinSidebar : d.shell.pinSidebar}
                  </TooltipContent>
                </Tooltip>
              ) : null}

              <nav className="pp-rail-nav">
                {groups.map((group, groupIndex) => (
                  <div className="pp-rail-group" key={group.key} data-group={group.key}>
                    {groupIndex > 0 ? <span className="pp-rail-separator" aria-hidden /> : null}
                    <span className="pp-rail-group-label">{group.label}</span>
                    {group.items.map((item) => {
                      const active = matchesOpenADRoute(item, router.pathname);
                      return (
                        <Tooltip key={item.key}>
                          <TooltipTrigger asChild>
                            <Link
                              href={item.href}
                              className={cn('pp-rail-link', active && 'active')}
                              aria-label={`${item.label}: ${item.description}`}
                              aria-current={active ? 'page' : undefined}
                            >
                              <item.icon className="h-[18px] w-[18px] shrink-0" aria-hidden />
                              <span className="pp-rail-route-copy">
                                <span className="pp-rail-label">{item.label}</span>
                                <span className="pp-rail-description">{item.description}</span>
                              </span>
                              <span className="pp-rail-active-mark" aria-hidden />
                            </Link>
                          </TooltipTrigger>
                          <TooltipContent side="right" className="pp-nav-tooltip">
                            <strong>{item.label}</strong>
                            <span>{item.description}</span>
                          </TooltipContent>
                        </Tooltip>
                      );
                    })}
                  </div>
                ))}
              </nav>

              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <button
                    type="button"
                    className={cn('pp-runtime-trigger', `is-${runtimeHealth}`)}
                    aria-label={runtimeLabel}
                  >
                    <span className="pp-runtime-icon">
                      <Server className="h-4 w-4" aria-hidden />
                      <span className="pp-runtime-dot" aria-hidden />
                    </span>
                    {navExpanded ? (
                      <span className="pp-runtime-copy">
                        <strong>{runtimeLabel}</strong>
                        <small>API 18080 · WEB 43110</small>
                      </span>
                    ) : null}
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent side="right" align="end" sideOffset={10} className="pp-runtime-menu">
                  <DropdownMenuLabel>{d.shell.runtimeTitle}</DropdownMenuLabel>
                  <div className="pp-runtime-menu-status">
                    <span className={cn('pp-runtime-dot', `is-${runtimeHealth}`)} aria-hidden />
                    <strong>{runtimeLabel}</strong>
                  </div>
                  <DropdownMenuSeparator />
                  <div className="pp-runtime-detail-row">
                    <span>{d.shell.runtimeApi}</span>
                    <code>http://127.0.0.1:18080</code>
                  </div>
                  <div className="pp-runtime-detail-row">
                    <span>{d.shell.runtimeWeb}</span>
                    <code>http://127.0.0.1:43110</code>
                  </div>
                  <DropdownMenuSeparator />
                  <p className="pp-runtime-note">{d.shell.runtimeAutomatic}</p>
                  <p className="pp-runtime-note">{d.shell.runtimeLocalOnly}</p>
                  {runtimeHealth === 'offline' ? <p className="pp-runtime-help">{d.shell.runtimeHelp}</p> : null}
                </DropdownMenuContent>
              </DropdownMenu>
            </aside>
          </div>

          <div className="pp-workspace-main">
            <header className="pp-command-island">
              <div className="pp-page-context">
                <span className="pp-page-icon"><ActiveIcon className="h-4 w-4" aria-hidden /></span>
                <span className="pp-page-copy">
                  <strong>{activeItem.label}</strong>
                  <small>{activeItem.description}</small>
                </span>
              </div>

              <div className="pp-shell-search">
                {searchLoading
                  ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
                  : <Search className="h-4 w-4" aria-hidden />}
                <input
                  ref={searchRef}
                  value={query}
                  role="combobox"
                  aria-expanded={searchOpen}
                  aria-controls="pp-global-search-options"
                  aria-autocomplete="list"
                  aria-activedescendant={searchActiveIndex >= 0 ? `pp-global-search-option-${searchActiveIndex}` : undefined}
                  onChange={(event) => {
                    setQuery(event.target.value);
                    setSearchOpen(false);
                    setSearchActiveIndex(-1);
                  }}
                  onFocus={() => searchOptions.length > 0 && setSearchOpen(true)}
                  onBlur={() => setSearchOpen(false)}
                  onKeyDown={handleGlobalSearchKeyDown}
                  placeholder={copy(
                    'Search users, groups or paths; type / for modules',
                    '搜索用户、组或路径，输入 / 打开模块',
                  )}
                  aria-label={copy('Global workspace search', '全局工作区搜索')}
                />
                <button
                  type="button"
                  className="pp-shell-search-submit"
                  aria-label={copy('Run global search', '执行全局搜索')}
                  title={copy('Run global search', '执行全局搜索')}
                  onClick={runQuery}
                  disabled={!query.trim()}
                >
                  <CornerDownLeft className="h-3.5 w-3.5" aria-hidden />
                </button>
                {searchOpen ? (
                  <div
                    id="pp-global-search-options"
                    className="pp-shell-search-options"
                    role="listbox"
                    aria-label={copy('Workspace search suggestions', '工作区搜索建议')}
                  >
                    {searchOptions.length === 0 && !searchLoading ? (
                      <p>{query.trim().startsWith('/')
                        ? copy('Unknown module command', '未知模块命令')
                        : copy('No matching users or groups', '没有匹配的用户或组')}</p>
                    ) : searchOptions.map((option, index) => {
                      const MatchIcon = option.kind === 'command'
                        ? option.command.icon
                        : option.match.kind === 'user' ? UserRound : Users;
                      const optionKey = option.kind === 'command'
                        ? `command-${option.command.key}`
                        : `${option.match.kind}-${option.match.dn}`;
                      const optionLabel = option.kind === 'command'
                        ? `${option.command.command} ${option.command.label}`
                        : `${option.match.label} ${option.match.secondary}`;
                      return (
                        <button
                          key={optionKey}
                          id={`pp-global-search-option-${index}`}
                          type="button"
                          role="option"
                          aria-selected={index === searchActiveIndex}
                          aria-label={optionLabel}
                          className={cn('pp-shell-search-option', index === searchActiveIndex && 'is-active')}
                          onMouseDown={(event) => event.preventDefault()}
                          onMouseEnter={() => setSearchActiveIndex(index)}
                          onClick={() => selectSearchOption(option)}
                        >
                          <MatchIcon className="h-4 w-4 shrink-0" aria-hidden />
                          <span>
                            <strong>{option.kind === 'command' ? option.command.label : option.match.label}</strong>
                            <small>{option.kind === 'command'
                              ? `${option.command.command} · ${option.command.description}`
                              : option.match.secondary}</small>
                          </span>
                          <Badge tone="neutral">{option.kind === 'command'
                            ? copy('Module', '模块')
                            : option.match.kind === 'user' ? copy('User', '用户') : copy('Group', '组')}</Badge>
                        </button>
                      );
                    })}
                  </div>
                ) : null}
              </div>

              <div className="pp-command-actions">
                <Badge tone={activeProfile || connection.connected ? 'success' : 'neutral'} dot className="pp-ad-status">
                  {activeProfile ? activeProfile.name : connection.connected ? d.shell.adConnected : d.shell.adDisconnected}
                </Badge>

                <div className="pp-theme-segment" role="group" aria-label={copy('Theme', '主题')}>
                  {themeOptions.map((option) => {
                    const Icon = option.icon;
                    return (
                      <button
                        key={option.key}
                        type="button"
                        className={cn('pp-command-icon', theme === option.key && 'active')}
                        aria-label={option.label}
                        title={option.label}
                        onClick={() => setTheme(option.key)}
                      >
                        <Icon className="h-3.5 w-3.5" aria-hidden />
                      </button>
                    );
                  })}
                </div>

                <button
                  type="button"
                  className="pp-command-icon"
                  aria-label={d.shell.language}
                  title={d.shell.language}
                  onClick={() => setLocale(isChinese ? 'en' : 'zh-CN')}
                >
                  <Languages className="h-3.5 w-3.5" aria-hidden />
                </button>

                <button
                  type="button"
                  className="pp-os-style"
                  onClick={() => setOSStyle(osStyle === 'windows' ? 'mac' : 'windows')}
                  title={copy('Switch window chrome', '切换窗口样式')}
                >
                  {osStyle === 'windows' ? 'WIN' : 'MAC'}
                </button>
              </div>
            </header>

            <main
              ref={workspaceContentRef}
              className={cn(
                'pp-workspace-content',
                effectiveWorkspaceSettings.compactTables && 'is-compact-tables',
              )}
            >
              {children}
            </main>
          </div>
        </div>
      </DesktopWindowFrame>
    </TooltipProvider>
  );
}
