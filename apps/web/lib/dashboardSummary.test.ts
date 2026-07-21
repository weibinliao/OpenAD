import { deriveDashboardTrendSummary, readStoredOperationLog, type DashboardSessionSummary } from './dashboardSummary';

function buildSession(overrides: Partial<DashboardSessionSummary> = {}): DashboardSessionSummary {
  return {
    id: 'session-1',
    root_path: 'C:\\Data',
    status: 'completed',
    max_depth: -1,
    include_inherited: true,
    items_scanned: 12,
    permission_count: 24,
    started_at: '2026-04-02T08:00:00.000Z',
    finished_at: '2026-04-02T08:05:00.000Z',
    ...overrides,
  };
}

describe('dashboardSummary', () => {
  test('reads stored operation log entries from browser storage JSON', () => {
    expect(
      readStoredOperationLog(
        JSON.stringify([
          {
            id: 'log-1',
            at: '2026-04-02T08:00:00.000Z',
            scope: 'scan',
            action: 'completed',
            message: 'Scan finished',
          },
          {
            ignored: true,
          },
        ])
      )
    ).toEqual([
      {
        id: 'log-1',
        at: '2026-04-02T08:00:00.000Z',
        scope: 'scan',
        action: 'completed',
        message: 'Scan finished',
      },
    ]);
  });

  test('returns an empty log list when stored JSON is invalid', () => {
    expect(readStoredOperationLog('{bad json')).toEqual([]);
  });

  test('derives latest session deltas from the two newest completed runs', () => {
    const summary = deriveDashboardTrendSummary([
      buildSession({ id: 'latest', root_path: 'C:\\Current', items_scanned: 140, permission_count: 64 }),
      buildSession({ id: 'previous', root_path: 'C:\\Previous', items_scanned: 125, permission_count: 70 }),
      buildSession({ id: 'failed', status: 'failed', items_scanned: 999, permission_count: 999 }),
    ]);

    expect(summary.completedSessionCount).toBe(2);
    expect(summary.latestSession?.id).toBe('latest');
    expect(summary.previousSession?.id).toBe('previous');
    expect(summary.itemDelta).toBe(15);
    expect(summary.permissionDelta).toBe(-6);
  });

  test('keeps deltas empty when fewer than two completed sessions are available', () => {
    const summary = deriveDashboardTrendSummary([buildSession({ id: 'only-one' })]);

    expect(summary.completedSessionCount).toBe(1);
    expect(summary.latestSession?.id).toBe('only-one');
    expect(summary.previousSession).toBeNull();
    expect(summary.itemDelta).toBeNull();
    expect(summary.permissionDelta).toBeNull();
  });
});
