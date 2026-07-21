import type { OperationLogEntry } from './workspaceSettings';

export interface DashboardSessionSummary {
  id: string;
  root_path: string;
  status: string;
  max_depth: number;
  include_inherited: boolean;
  items_scanned: number;
  permission_count: number;
  started_at: string;
  finished_at?: string;
}

export interface DashboardTrendSummary {
  completedSessionCount: number;
  latestSession: DashboardSessionSummary | null;
  previousSession: DashboardSessionSummary | null;
  itemDelta: number | null;
  permissionDelta: number | null;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function isOperationLogEntry(value: unknown): value is OperationLogEntry {
  if (!isRecord(value)) {
    return false;
  }

  return (
    typeof value.id === 'string' &&
    typeof value.at === 'string' &&
    typeof value.scope === 'string' &&
    typeof value.action === 'string' &&
    typeof value.message === 'string'
  );
}

export function readStoredOperationLog(raw: string | null) {
  if (!raw) {
    return [];
  }

  try {
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) {
      return [];
    }
    return parsed.filter(isOperationLogEntry);
  } catch {
    return [];
  }
}

export function deriveDashboardTrendSummary(sessions: DashboardSessionSummary[]): DashboardTrendSummary {
  const completedSessions = sessions.filter((session) => session.status === 'completed');
  const latestSession = completedSessions[0] || null;
  const previousSession = completedSessions[1] || null;

  return {
    completedSessionCount: completedSessions.length,
    latestSession,
    previousSession,
    itemDelta: latestSession && previousSession ? latestSession.items_scanned - previousSession.items_scanned : null,
    permissionDelta: latestSession && previousSession ? latestSession.permission_count - previousSession.permission_count : null,
  };
}
