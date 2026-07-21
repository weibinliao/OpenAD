import {
  buildWorkspaceCommands,
  resolveWorkspaceSearch,
} from './workspaceSearch';

describe('workspace search intent', () => {
  it('builds module commands from the OpenAD navigation definition', () => {
    const commands = buildWorkspaceCommands('en');

    expect(commands).toEqual(expect.arrayContaining([
      expect.objectContaining({ command: '/scan', href: '/scan-workspace' }),
      expect.objectContaining({ command: '/reports', href: '/reports' }),
      expect.objectContaining({ command: '/identity', href: '/identity' }),
    ]));
  });

  it.each([
    ['/reports', '/reports'],
    ['/scan', '/scan-workspace'],
    ['/settings', '/settings'],
  ])('resolves %s as a module command', (query, href) => {
    expect(resolveWorkspaceSearch(query, 'en')).toMatchObject({ kind: 'module', href });
  });

  it('does not interpret an unknown slash command as a resource path', () => {
    expect(resolveWorkspaceSearch('/missing-module', 'en')).toEqual({
      kind: 'invalid-command',
      query: '/missing-module',
    });
  });

  it.each([
    '\\\\fs01\\Finance',
    'D:\\Shares\\Finance',
    'D:/Shares/Finance',
  ])('recognizes %s as a resource path', (query) => {
    expect(resolveWorkspaceSearch(query, 'en')).toEqual({ kind: 'path', path: query });
  });

  it('keeps ordinary text as an AD directory query', () => {
    expect(resolveWorkspaceSearch('alex', 'en')).toEqual({
      kind: 'directory',
      query: 'alex',
    });
  });
});
