import {
  buildOpenADNavigation,
  type OpenADLocale,
  type OpenADNavItem,
} from './openadNavigation';

export interface WorkspaceCommand {
  key: string;
  command: string;
  aliases: string[];
  label: string;
  description: string;
  href: string;
  icon: OpenADNavItem['icon'];
}

export type WorkspaceSearchIntent =
  | { kind: 'empty' }
  | { kind: 'module'; href: string; command: WorkspaceCommand }
  | { kind: 'invalid-command'; query: string }
  | { kind: 'path'; path: string }
  | { kind: 'directory'; query: string };

const PRIMARY_COMMANDS: Record<string, string> = {
  home: '/home',
  identity: '/identity',
  resources: '/resources',
  scan: '/scan',
  access: '/access',
  history: '/history',
  findings: '/findings',
  'file-activity': '/file-activity',
  audit: '/audit',
  settings: '/settings',
  reports: '/reports',
};

const EXTRA_ALIASES: Record<string, string[]> = {
  home: ['/overview'],
  identity: ['/directory', '/ad'],
  scan: ['/scanner'],
  findings: ['/risks'],
  reports: ['/report'],
};

function normalizeCommand(value: string) {
  const normalized = value.trim().toLowerCase();
  return normalized.startsWith('/') ? normalized : `/${normalized}`;
}

export function buildWorkspaceCommands(locale: OpenADLocale): WorkspaceCommand[] {
  return buildOpenADNavigation(locale)
    .flatMap((group) => group.items)
    .map((item) => {
      const command = PRIMARY_COMMANDS[item.key] || normalizeCommand(item.key);
      const aliases = new Set<string>([
        command,
        ...(item.href === '/' ? [] : [item.href]),
        ...(item.aliases || []).filter((alias) => !alias.includes('[')),
        ...(EXTRA_ALIASES[item.key] || []),
      ].map(normalizeCommand));

      return {
        key: item.key,
        command,
        aliases: Array.from(aliases),
        label: item.label,
        description: item.description,
        href: item.href,
        icon: item.icon,
      };
    });
}

export function findWorkspaceCommandMatches(
  query: string,
  commands: WorkspaceCommand[],
  limit = 12,
) {
  const normalized = normalizeCommand(query || '/');
  const term = normalized.slice(1);

  return commands
    .filter((command) => (
      normalized === '/'
      || command.aliases.some((alias) => alias.startsWith(normalized))
      || command.label.toLowerCase().includes(term)
    ))
    .slice(0, limit);
}

export function resolveWorkspaceSearch(
  query: string,
  locale: OpenADLocale,
): WorkspaceSearchIntent {
  const value = query.trim();
  if (!value) return { kind: 'empty' };

  if (value.startsWith('/')) {
    const normalized = normalizeCommand(value);
    const command = buildWorkspaceCommands(locale).find((item) => item.aliases.includes(normalized));
    return command
      ? { kind: 'module', href: command.href, command }
      : { kind: 'invalid-command', query: value };
  }

  if (/^\\\\/.test(value) || /^[a-z]:[\\/]/i.test(value) || value.includes('\\') || value.includes('/')) {
    return { kind: 'path', path: value };
  }

  return { kind: 'directory', query: value };
}
