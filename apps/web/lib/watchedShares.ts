export interface WatchedShareProfile {
  id: string;
  name: string;
  path: string;
  defaultDepth: number;
  includeInherited: boolean;
  createdAt: string;
  updatedAt: string;
  lastScannedAt?: string;
  lastSessionID?: string;
  lastItemsScanned?: number;
  lastPermissionCount?: number;
  lastHighRiskCount?: number;
}

export interface UpsertWatchedShareInput {
  id?: string;
  name?: string;
  path: string;
  defaultDepth: number;
  includeInherited: boolean;
}

export interface WatchedShareScanStats {
  id?: string;
  path: string;
  sessionID?: string;
  itemsScanned: number;
  permissionCount: number;
  highRiskCount: number;
  scannedAt?: string;
}

export const WATCHED_SHARES_KEY = 'permissionProtector.watchedShares';
export const WATCHED_SHARES_UPDATED_EVENT = 'permission-protector:watched-shares-updated';

function browserStorage() {
  if (typeof window === 'undefined') {
    return null;
  }
  return window.localStorage;
}

function createID() {
  if (typeof window !== 'undefined' && window.crypto?.randomUUID) {
    return window.crypto.randomUUID();
  }
  return `watched-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

function normalizePath(value: string) {
  return value.trim().replace(/\//g, '\\');
}

function compactPathName(path: string) {
  const normalized = normalizePath(path);
  const parts = normalized.split('\\').filter(Boolean);
  if (parts.length === 0) {
    return 'Watched Share';
  }
  if (normalized.startsWith('\\\\') && parts.length >= 2) {
    return parts.slice(1, 3).join(' / ') || parts[parts.length - 1];
  }
  return parts[parts.length - 1] || normalized;
}

function sanitizeItem(value: Partial<WatchedShareProfile> | null | undefined): WatchedShareProfile | null {
  const path = normalizePath(value?.path || '');
  if (!path) {
    return null;
  }

  const now = new Date().toISOString();
  return {
    id: (value?.id || '').trim() || createID(),
    name: (value?.name || '').trim() || compactPathName(path),
    path,
    defaultDepth: Number.isFinite(value?.defaultDepth) ? Number(value?.defaultDepth) : -1,
    includeInherited: value?.includeInherited !== false,
    createdAt: value?.createdAt || now,
    updatedAt: value?.updatedAt || now,
    lastScannedAt: value?.lastScannedAt || undefined,
    lastSessionID: value?.lastSessionID || undefined,
    lastItemsScanned: Number.isFinite(value?.lastItemsScanned) ? Number(value?.lastItemsScanned) : undefined,
    lastPermissionCount: Number.isFinite(value?.lastPermissionCount) ? Number(value?.lastPermissionCount) : undefined,
    lastHighRiskCount: Number.isFinite(value?.lastHighRiskCount) ? Number(value?.lastHighRiskCount) : undefined,
  };
}

function persistWatchedShares(items: WatchedShareProfile[]) {
  const storage = browserStorage();
  if (!storage) {
    return;
  }
  storage.setItem(WATCHED_SHARES_KEY, JSON.stringify(items));
  window.dispatchEvent(new Event(WATCHED_SHARES_UPDATED_EVENT));
}

export function readWatchedShares(): WatchedShareProfile[] {
  const storage = browserStorage();
  if (!storage) {
    return [];
  }

  try {
    const raw = storage.getItem(WATCHED_SHARES_KEY);
    if (!raw) {
      return [];
    }
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) {
      return [];
    }
    return parsed
      .map((item) => sanitizeItem(item))
      .filter((item): item is WatchedShareProfile => Boolean(item))
      .sort((a, b) => (b.updatedAt || '').localeCompare(a.updatedAt || ''));
  } catch {
    return [];
  }
}

export function upsertWatchedShare(input: UpsertWatchedShareInput) {
  const path = normalizePath(input.path);
  if (!path) {
    return null;
  }

  const now = new Date().toISOString();
  const items = readWatchedShares();
  const existingIndex = items.findIndex((item) => item.id === input.id || item.path.toLowerCase() === path.toLowerCase());
  const existing = existingIndex >= 0 ? items[existingIndex] : null;
  const next: WatchedShareProfile = {
    id: existing?.id || input.id || createID(),
    name: (input.name || existing?.name || '').trim() || compactPathName(path),
    path,
    defaultDepth: Number.isFinite(input.defaultDepth) ? input.defaultDepth : existing?.defaultDepth ?? -1,
    includeInherited: input.includeInherited !== false,
    createdAt: existing?.createdAt || now,
    updatedAt: now,
    lastScannedAt: existing?.lastScannedAt,
    lastSessionID: existing?.lastSessionID,
    lastItemsScanned: existing?.lastItemsScanned,
    lastPermissionCount: existing?.lastPermissionCount,
    lastHighRiskCount: existing?.lastHighRiskCount,
  };

  const nextItems = existingIndex >= 0
    ? items.map((item, index) => (index === existingIndex ? next : item))
    : [next, ...items];

  persistWatchedShares(nextItems);
  return next;
}

export function removeWatchedShare(id: string) {
  const items = readWatchedShares().filter((item) => item.id !== id);
  persistWatchedShares(items);
}

export function recordWatchedShareScan(stats: WatchedShareScanStats) {
  const path = normalizePath(stats.path);
  const items = readWatchedShares();
  const index = items.findIndex((item) => item.id === stats.id || item.path.toLowerCase() === path.toLowerCase());
  if (index < 0) {
    return null;
  }

  const now = stats.scannedAt || new Date().toISOString();
  const next: WatchedShareProfile = {
    ...items[index],
    path: items[index].path || path,
    updatedAt: now,
    lastScannedAt: now,
    lastSessionID: stats.sessionID || items[index].lastSessionID,
    lastItemsScanned: stats.itemsScanned,
    lastPermissionCount: stats.permissionCount,
    lastHighRiskCount: stats.highRiskCount,
  };
  const nextItems = items.map((item, itemIndex) => (itemIndex === index ? next : item));
  persistWatchedShares(nextItems);
  return next;
}

export function findWatchedShare(id: string) {
  return readWatchedShares().find((item) => item.id === id) || null;
}
