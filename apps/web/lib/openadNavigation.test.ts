import { buildOpenADNavigation, matchesOpenADRoute } from './openadNavigation';

describe('OpenAD navigation architecture', () => {
  const groups = buildOpenADNavigation('zh-CN');
  const items = groups.flatMap((group) => group.items);

  it('uses one primary overview and preserves business routes', () => {
    expect(items.filter((item) => item.href === '/')).toHaveLength(1);
    expect(items.map((item) => item.href)).toEqual([
      '/',
      '/identity',
      '/resources',
      '/scan-workspace',
      '/access',
      '/history',
      '/findings',
      '/file-activity',
      '/audit',
      '/settings',
      '/reports',
    ]);
  });

  it('orders permission work from inventory to historical evidence', () => {
    const permissionGroup = groups.find((group) => group.key === 'permissions');
    expect(permissionGroup?.items.map((item) => item.key)).toEqual([
      'resources',
      'scan',
      'access',
      'history',
    ]);
    expect(groups.at(-1)?.items.map((item) => item.key)).toEqual(['reports']);
  });

  it('provides bilingual labels and operational descriptions', () => {
    expect(items.find((item) => item.key === 'home')).toMatchObject({
      label: '总览',
    });
    expect(items.find((item) => item.key === 'identity')).toMatchObject({
      label: '目录浏览',
      description: '用户、组、OU 与目录同步',
    });

    const english = buildOpenADNavigation('en').flatMap((group) => group.items);
    expect(english.find((item) => item.key === 'home')).toMatchObject({
      label: 'Overview',
    });
    expect(english.find((item) => item.key === 'identity')).toMatchObject({
      label: 'Directory Explorer',
      description: 'Users, groups, OUs and sync',
    });
  });

  it('matches nested routes and legacy aliases', () => {
    const audit = items.find((item) => item.key === 'audit');
    const findings = items.find((item) => item.key === 'findings');
    const home = items.find((item) => item.key === 'home');

    expect(audit && matchesOpenADRoute(audit, '/audit/[id]')).toBe(true);
    expect(findings && matchesOpenADRoute(findings, '/risks')).toBe(true);
    expect(home && matchesOpenADRoute(home, '/dashboard')).toBe(true);
  });
});
