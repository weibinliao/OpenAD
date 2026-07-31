import { analyzePermissionExposure, type PermissionExposureFinding, type PermissionExposureRemediationEffort } from './permissionExposure';
import type { PermissionReportItem } from './reportPayload';
import { apiBase } from './runtimeApi';

export type RiskFindingStatus = 'open' | 'accepted' | 'resolved';
export type RiskFindingSeverity = 'critical' | 'high' | 'medium' | 'low';

export type RiskFindingPermission = PermissionReportItem;

export interface RiskFinding {
  id: string;
  fingerprint: string;
  status: RiskFindingStatus;
  severity: RiskFindingSeverity;
  type: string;
  title: string;
  suggestedAction: string;
  path: string;
  trustee: string;
  trusteeSid: string;
  rights: string;
  inherited: boolean;
  source: string;
  firstSeenAt: string;
  lastSeenAt: string;
  lastSessionID?: string;
  seenCount: number;
  note?: string;
  description?: string;
  impact?: string;
  category?: string;
  priorityScore?: number;
  confidence?: string;
  remediationEffort?: PermissionExposureRemediationEffort;
  businessQuestion?: string;
  controlMapping?: string[];
  evidence?: string[];
  sensitiveLabels?: string[];
}

export interface RiskFindingSummary {
  total: number;
  open: number;
  accepted: number;
  resolved: number;
  critical: number;
  high: number;
  medium: number;
  low: number;
  exposureScore: number;
  rulesTriggered: number;
  riskyPaths: number;
  riskyTrustees: number;
  sensitiveFindings: number;
  sensitivePaths: number;
  quickWins: number;
  ownerReviews: number;
}

export const RISK_FINDINGS_KEY = 'permissionProtector.riskFindings';
export const RISK_FINDINGS_UPDATED_EVENT = 'permission-protector:risk-findings-updated';

interface RiskFindingWire {
  id: string;
  fingerprint: string;
  status: RiskFindingStatus;
  severity: RiskFindingSeverity;
  type: string;
  title: string;
  suggested_action: string;
  path: string;
  trustee: string;
  trustee_sid: string;
  rights: string;
  inherited: boolean;
  source: string;
  first_seen_at: string;
  last_seen_at: string;
  last_session_id?: string;
  seen_count: number;
  note?: string;
  description?: string;
  impact?: string;
  category?: string;
  priority_score?: number;
  confidence?: string;
  remediation_effort?: PermissionExposureRemediationEffort;
  business_question?: string;
  control_mapping?: string[];
  evidence?: string[];
  sensitive_labels?: string[];
}

interface RiskFindingListResponse {
  items?: RiskFindingWire[];
}

let legacyMigrationPromise: Promise<void> | null = null;

function legacyBrowserStorage() {
  if (typeof window === 'undefined') {
    return null;
  }
  return window.localStorage;
}

function createID() {
  if (typeof window !== 'undefined' && window.crypto?.randomUUID) {
    return window.crypto.randomUUID();
  }
  return `finding-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

function normalized(value: string | undefined | null) {
  return (value || '').trim();
}

function exposureSeverityScore(severity: RiskFindingSeverity) {
  switch (severity) {
    case 'critical':
      return 90;
    case 'high':
      return 74;
    case 'medium':
      return 48;
    case 'low':
      return 24;
  }
}

function findingFromExposure(finding: PermissionExposureFinding, scanStartedAt: string, sessionID?: string): RiskFinding {
  return {
    id: createID(),
    fingerprint: finding.fingerprint,
    status: 'open',
    severity: finding.severity,
    type: finding.ruleID,
    title: finding.title,
    suggestedAction: finding.suggestedAction,
    path: normalized(finding.path),
    trustee: normalized(finding.trustee),
    trusteeSid: normalized(finding.trusteeSid),
    rights: normalized(finding.rights),
    inherited: Boolean(finding.inherited),
    source: normalized(finding.source),
    firstSeenAt: scanStartedAt,
    lastSeenAt: scanStartedAt,
    lastSessionID: sessionID || undefined,
    seenCount: 1,
    description: normalized(finding.description) || undefined,
    impact: normalized(finding.impact) || undefined,
    category: finding.category,
    priorityScore: finding.priorityScore,
    confidence: finding.confidence,
    remediationEffort: finding.remediationEffort,
    businessQuestion: normalized(finding.businessQuestion) || undefined,
    controlMapping: finding.controlMapping,
    evidence: finding.evidence,
    sensitiveLabels: finding.sensitiveLabels,
  };
}

function sanitizeStringArray(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const items = value.map((item) => normalized(String(item))).filter(Boolean);
  return items.length > 0 ? items : undefined;
}

function sanitizeFinding(value: Partial<RiskFinding> | null | undefined): RiskFinding | null {
  const fingerprintValue = normalized(value?.fingerprint);
  const path = normalized(value?.path);
  const trustee = normalized(value?.trustee);
  if (!fingerprintValue || !path || !trustee) {
    return null;
  }

  const status = value?.status === 'accepted' || value?.status === 'resolved' ? value.status : 'open';
  const severity = value?.severity === 'critical' || value?.severity === 'high' || value?.severity === 'medium' || value?.severity === 'low'
    ? value.severity
    : 'medium';
  const now = new Date().toISOString();
  const priorityScore = Number(value?.priorityScore);

  return {
    id: normalized(value?.id) || createID(),
    fingerprint: fingerprintValue,
    status,
    severity,
    type: normalized(value?.type) || 'risk-finding',
    title: normalized(value?.title) || 'Permission risk requires review',
    suggestedAction: normalized(value?.suggestedAction) || 'Review this permission and document the decision.',
    path,
    trustee,
    trusteeSid: normalized(value?.trusteeSid),
    rights: normalized(value?.rights),
    inherited: Boolean(value?.inherited),
    source: normalized(value?.source),
    firstSeenAt: value?.firstSeenAt || now,
    lastSeenAt: value?.lastSeenAt || now,
    lastSessionID: normalized(value?.lastSessionID) || undefined,
    seenCount: Number.isFinite(value?.seenCount) ? Math.max(1, Number(value?.seenCount)) : 1,
    note: normalized(value?.note) || undefined,
    description: normalized(value?.description) || undefined,
    impact: normalized(value?.impact) || undefined,
    category: normalized(value?.category) || undefined,
    priorityScore: Number.isFinite(priorityScore) ? Math.max(1, Math.min(100, priorityScore)) : undefined,
    confidence: normalized(value?.confidence) || undefined,
    remediationEffort: value?.remediationEffort === 'quick-win' || value?.remediationEffort === 'owner-review' || value?.remediationEffort === 'planned-change' ? value.remediationEffort : undefined,
    businessQuestion: normalized(value?.businessQuestion) || undefined,
    controlMapping: sanitizeStringArray(value?.controlMapping),
    evidence: sanitizeStringArray(value?.evidence),
    sensitiveLabels: sanitizeStringArray(value?.sensitiveLabels),
  };
}

function readLegacyRiskFindings() {
  const storage = legacyBrowserStorage();
  if (!storage) {
    return [];
  }

  try {
    const raw = storage.getItem(RISK_FINDINGS_KEY);
    if (!raw) {
      return [];
    }
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) {
      return [];
    }
    return parsed
      .map((item) => sanitizeFinding(item))
      .filter((item): item is RiskFinding => Boolean(item))
      .sort((a, b) => b.lastSeenAt.localeCompare(a.lastSeenAt));
  } catch {
    return [];
  }
}

function toWireFinding(finding: RiskFinding): Omit<RiskFindingWire, 'id'> {
  return {
    fingerprint: finding.fingerprint,
    status: finding.status,
    severity: finding.severity,
    type: finding.type,
    title: finding.title,
    suggested_action: finding.suggestedAction,
    path: finding.path,
    trustee: finding.trustee,
    trustee_sid: finding.trusteeSid,
    rights: finding.rights,
    inherited: finding.inherited,
    source: finding.source,
    first_seen_at: finding.firstSeenAt,
    last_seen_at: finding.lastSeenAt,
    last_session_id: finding.lastSessionID,
    seen_count: finding.seenCount,
    note: finding.note,
    description: finding.description,
    impact: finding.impact,
    category: finding.category,
    priority_score: finding.priorityScore,
    confidence: finding.confidence,
    remediation_effort: finding.remediationEffort,
    business_question: finding.businessQuestion,
    control_mapping: finding.controlMapping,
    evidence: finding.evidence,
    sensitive_labels: finding.sensitiveLabels,
  };
}

function fromWireFinding(finding: RiskFindingWire): RiskFinding | null {
  return sanitizeFinding({
    id: finding.id,
    fingerprint: finding.fingerprint,
    status: finding.status,
    severity: finding.severity,
    type: finding.type,
    title: finding.title,
    suggestedAction: finding.suggested_action,
    path: finding.path,
    trustee: finding.trustee,
    trusteeSid: finding.trustee_sid,
    rights: finding.rights,
    inherited: finding.inherited,
    source: finding.source,
    firstSeenAt: finding.first_seen_at,
    lastSeenAt: finding.last_seen_at,
    lastSessionID: finding.last_session_id,
    seenCount: finding.seen_count,
    note: finding.note,
    description: finding.description,
    impact: finding.impact,
    category: finding.category,
    priorityScore: finding.priority_score,
    confidence: finding.confidence,
    remediationEffort: finding.remediation_effort,
    businessQuestion: finding.business_question,
    controlMapping: finding.control_mapping,
    evidence: finding.evidence,
    sensitiveLabels: finding.sensitive_labels,
  });
}

async function requestJSON<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${apiBase()}${path}`, options);
  const data = (await response.json().catch(() => ({}))) as T & { error?: string };
  if (!response.ok) {
    throw new Error(data.error || `Risk finding request failed (${response.status})`);
  }
  return data;
}

async function migrateLegacyRiskFindings() {
  const storage = legacyBrowserStorage();
  if (!storage || !storage.getItem(RISK_FINDINGS_KEY)) {
    return;
  }

  if (!legacyMigrationPromise) {
    legacyMigrationPromise = (async () => {
      const items = readLegacyRiskFindings();
      if (items.length > 0) {
        await requestJSON('/api/risk-findings/import', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ items: items.map(toWireFinding) }),
        });
      }
      storage.removeItem(RISK_FINDINGS_KEY);
    })().finally(() => {
      legacyMigrationPromise = null;
    });
  }

  await legacyMigrationPromise;
}

function dispatchRiskFindingsUpdated() {
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new Event(RISK_FINDINGS_UPDATED_EVENT));
  }
}

export async function loadRiskFindings() {
  await migrateLegacyRiskFindings();
  const data = await requestJSON<RiskFindingListResponse>('/api/risk-findings');
  return (Array.isArray(data.items) ? data.items : [])
    .map((item) => fromWireFinding(item))
    .filter((item): item is RiskFinding => Boolean(item))
    .sort((a, b) => b.lastSeenAt.localeCompare(a.lastSeenAt));
}

export function summarizeRiskFindings(items: RiskFinding[] = []): RiskFindingSummary {
  const openItems = items.filter((item) => item.status === 'open');
  const sensitiveOpenItems = openItems.filter((item) => item.category === 'sensitive-data' || Boolean(item.sensitiveLabels?.length));
  const priorityScores = openItems
    .map((item) => item.priorityScore || exposureSeverityScore(item.severity))
    .sort((a, b) => b - a)
    .slice(0, 10);
  const exposureScore = priorityScores.length === 0
    ? 0
    : Math.round(priorityScores.reduce((sum, score) => sum + score, 0) / priorityScores.length);

  return {
    total: items.length,
    open: openItems.length,
    accepted: items.filter((item) => item.status === 'accepted').length,
    resolved: items.filter((item) => item.status === 'resolved').length,
    critical: openItems.filter((item) => item.severity === 'critical').length,
    high: openItems.filter((item) => item.severity === 'high').length,
    medium: openItems.filter((item) => item.severity === 'medium').length,
    low: openItems.filter((item) => item.severity === 'low').length,
    exposureScore,
    rulesTriggered: new Set(openItems.map((item) => item.type)).size,
    riskyPaths: new Set(openItems.map((item) => item.path.toLowerCase())).size,
    riskyTrustees: new Set(openItems.map((item) => item.trustee.toLowerCase())).size,
    sensitiveFindings: sensitiveOpenItems.length,
    sensitivePaths: new Set(sensitiveOpenItems.map((item) => item.path.toLowerCase())).size,
    quickWins: openItems.filter((item) => item.remediationEffort === 'quick-win').length,
    ownerReviews: openItems.filter((item) => item.remediationEffort === 'owner-review').length,
  };
}

export function severityRank(severity: RiskFindingSeverity): number {
  switch (severity) {
    case 'critical':
      return 400;
    case 'high':
      return 300;
    case 'medium':
      return 200;
    case 'low':
      return 100;
  }
}

export function sortRiskFindingsByPriority<T extends Pick<RiskFinding, 'priorityScore' | 'severity'>>(items: readonly T[]): T[] {
  return [...items].sort(
    (a, b) => (b.priorityScore ?? severityRank(b.severity)) - (a.priorityScore ?? severityRank(a.severity)),
  );
}

export async function upsertRiskFindingsFromScan({
  permissions,
  sessionID,
  scannedAt = new Date().toISOString(),
}: {
  permissions: RiskFindingPermission[];
  sessionID?: string;
  scannedAt?: string;
}) {
  const generated = analyzePermissionExposure(permissions).findings.map((finding) => findingFromExposure(finding, scannedAt, sessionID));
  if (generated.length === 0) {
    return 0;
  }

  await migrateLegacyRiskFindings();
  const data = await requestJSON<{ count?: number }>('/api/risk-findings/upsert', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ items: generated.map(toWireFinding) }),
  });
  dispatchRiskFindingsUpdated();
  return Number(data.count || 0);
}

export async function updateRiskFindingStatus(id: string, status: RiskFindingStatus, note?: string) {
  await migrateLegacyRiskFindings();
  const payload: { status: RiskFindingStatus; note?: string } = { status };
  if (normalized(note)) {
    payload.note = normalized(note);
  }
  const data = await requestJSON<RiskFindingWire>(`/api/risk-findings/${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  const finding = fromWireFinding(data);
  if (!finding) {
    throw new Error('Risk finding response is invalid');
  }
  dispatchRiskFindingsUpdated();
  return finding;
}

export function riskSeverityChipClass(severity: RiskFindingSeverity) {
  switch (severity) {
    case 'critical':
      return 'status-chip-danger';
    case 'high':
      return 'status-chip-warning';
    case 'medium':
      return 'status-chip-info';
    case 'low':
      return 'status-chip-success';
    default:
      return '';
  }
}
