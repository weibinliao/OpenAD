import type { PermissionReportItem } from './reportPayload';

export type PermissionExposureSeverity = 'critical' | 'high' | 'medium' | 'low';
export type PermissionExposureCategory = 'overexposure' | 'privilege' | 'hygiene' | 'governance' | 'operational-friction' | 'sensitive-data';
export type PermissionExposureConfidence = 'high' | 'medium' | 'low';
export type PermissionExposureRemediationEffort = 'quick-win' | 'owner-review' | 'planned-change';

export interface PermissionExposureFinding {
  fingerprint: string;
  ruleID: string;
  category: PermissionExposureCategory;
  severity: PermissionExposureSeverity;
  priorityScore: number;
  confidence: PermissionExposureConfidence;
  remediationEffort: PermissionExposureRemediationEffort;
  title: string;
  description: string;
  impact: string;
  suggestedAction: string;
  businessQuestion: string;
  controlMapping: string[];
  evidence: string[];
  sensitiveLabels?: string[];
  path: string;
  trustee: string;
  trusteeSid: string;
  rights: string;
  type: string;
  inherited: boolean;
  source: string;
}

export interface PermissionExposureSummary {
  total: number;
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

export interface PermissionExposureAnalysis {
  findings: PermissionExposureFinding[];
  summary: PermissionExposureSummary;
}

const broadTrusteePatterns = ['everyone', 'domain users', 'authenticated users', 'builtin\\users', 'built-in\\users'];
const privilegedTrusteePatterns = ['administrators', 'domain admins', 'enterprise admins', 'backup operators', 'account operators', 'server operators', 'print operators', 'schema admins'];

const sensitivePathSignals: Array<{ label: string; tokens: string[]; patterns?: RegExp[] }> = [
  { label: 'Payroll', tokens: ['payroll', 'salary', 'salaries', 'compensation', 'bonus', 'bonuses'] },
  { label: 'HR', tokens: ['hr', 'people', 'employee', 'employees'], patterns: [/\bhuman[\s._-]*resources\b/, /\bemployee[\s._-]*records\b/] },
  { label: 'Finance', tokens: ['finance', 'financial', 'accounting', 'accounts', 'budget', 'budgets', 'invoice', 'invoices', 'tax', 'taxes', 'ap', 'ar'] },
  { label: 'Legal / Contracts', tokens: ['legal', 'contract', 'contracts', 'nda', 'ndas', 'litigation', 'compliance'] },
  { label: 'Executive', tokens: ['executive', 'board', 'leadership', 'strategy', 'mna'], patterns: [/\bc[\s._-]*suite\b/, /\bmergers?[\s._-]*and[\s._-]*acquisitions\b/] },
  { label: 'Backups', tokens: ['backup', 'backups', 'restore', 'snapshot', 'snapshots', 'dump', 'dumps', 'archive', 'archives'] },
  { label: 'Source Code', tokens: ['source', 'src', 'repo', 'repos', 'repository', 'repositories', 'git', 'github', 'gitlab', 'svn', 'codebase'], patterns: [/\bsource[\s._-]*code\b/] },
  { label: 'Secrets / Credentials', tokens: ['secret', 'secrets', 'credential', 'credentials', 'password', 'passwords', 'key', 'keys', 'cert', 'certs', 'certificate', 'certificates', 'token', 'tokens', 'vault'] },
  { label: 'Customer / PII', tokens: ['pii', 'gdpr', 'customer', 'customers', 'client', 'clients', 'personaldata'], patterns: [/\bpersonal[\s._-]*data\b/, /\bcustomer[\s._-]*data\b/] },
  { label: 'IT / Admin', tokens: ['it', 'admin', 'admins', 'sysadmin', 'infrastructure', 'servers', 'domain-admins'] },
  { label: 'Confidential', tokens: ['confidential', 'restricted', 'private', 'sensitive', 'classified'] },
];

function normalized(value: string | undefined | null) {
  return (value || '').trim();
}

function identityFor(item: PermissionReportItem) {
  return normalized(item.trustee) || normalized(item.account_name) || normalized(item.trustee_sid) || 'Unknown';
}

function includesAny(value: string, patterns: string[]) {
  const lower = value.toLowerCase();
  return patterns.some((pattern) => lower.includes(pattern));
}

function tokenizePath(path: string) {
  const lowerPath = normalized(path).replace(/[\\/]+/g, '/').toLowerCase();
  const tokens = new Set<string>();

  for (const part of lowerPath.split('/')) {
    const trimmed = part.trim();
    if (!trimmed) {
      continue;
    }
    tokens.add(trimmed);
    for (const token of trimmed.split(/[\s._-]+/)) {
      if (token) {
        tokens.add(token);
      }
    }
  }

  return { lowerPath, tokens };
}

function classifySensitivePath(path: string) {
  const { lowerPath, tokens } = tokenizePath(path);
  if (!lowerPath) {
    return [];
  }

  return sensitivePathSignals
    .filter((signal) => signal.tokens.some((token) => tokens.has(token)) || (signal.patterns || []).some((pattern) => pattern.test(lowerPath)))
    .map((signal) => signal.label);
}

function sensitiveEvidence(labels: string[]) {
  return labels.length > 0
    ? [`Sensitive area: ${labels.join(', ')}`, 'Detection: path-name heuristic; validate with the data owner before removal.']
    : [];
}

function isSID(value: string) {
  return /^S-\d-/i.test(value.trim());
}

function rightsText(item: PermissionReportItem) {
  return normalized(item.rights).replaceAll('FullControl', 'Full Control').replaceAll('ReadAndExecute', 'Read and Execute');
}

function inferRightsRisk(item: Pick<PermissionReportItem, 'rights' | 'risk_level'>) {
  const provided = normalized(item.risk_level).toLowerCase();
  if (provided === 'high' || provided === 'medium' || provided === 'low') {
    return provided;
  }

  const rights = normalized(item.rights).toLowerCase().replaceAll('fullcontrol', 'full control');
  if (['full control', 'take ownership', 'change permissions', 'write dac', 'write owner'].some((token) => rights.includes(token))) {
    return 'high';
  }
  if (['modify', 'write', 'delete', 'create files', 'append data'].some((token) => rights.includes(token))) {
    return 'high';
  }
  if (rights.includes('execute')) {
    return 'medium';
  }
  return 'low';
}

function pathDepth(path: string) {
  return path.split(/[\\/]+/).filter((part) => part && !part.endsWith(':')).length;
}

function severityBaseScore(severity: PermissionExposureSeverity) {
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

function clampScore(value: number) {
  return Math.max(1, Math.min(100, Math.round(value)));
}

function fingerprint(ruleID: string, item: PermissionReportItem) {
  return [ruleID, item.path, identityFor(item), item.rights, item.type, String(item.inherited), item.source]
    .map((value) => normalized(value).toLowerCase())
    .join('|');
}

function baseEvidence(item: PermissionReportItem) {
  return [
    `Path: ${normalized(item.path) || '-'}`,
    `Trustee: ${identityFor(item)}`,
    `Rights: ${rightsText(item) || '-'}`,
    `ACE: ${normalized(item.type) || 'Allow'} / ${item.inherited ? 'Inherited' : 'Explicit'}`,
    normalized(item.source) ? `Source: ${normalized(item.source)}` : '',
    normalized(item.originating_group) ? `Originating group: ${normalized(item.originating_group)}` : '',
  ].filter(Boolean);
}

function createFinding(
  item: PermissionReportItem,
  ruleID: string,
  details: Omit<PermissionExposureFinding, 'fingerprint' | 'ruleID' | 'path' | 'trustee' | 'trusteeSid' | 'rights' | 'type' | 'inherited' | 'source' | 'evidence' | 'priorityScore'> & {
    evidence?: string[];
    priorityScore?: number;
  }
): PermissionExposureFinding {
  const score = details.priorityScore ?? severityBaseScore(details.severity) + (item.inherited ? -3 : 5);
  const sensitiveLabels = Array.from(new Set((details.sensitiveLabels || []).map((label) => normalized(label)).filter(Boolean)));

  return {
    fingerprint: fingerprint(ruleID, item),
    ruleID,
    category: details.category,
    severity: details.severity,
    priorityScore: clampScore(score),
    confidence: details.confidence,
    remediationEffort: details.remediationEffort,
    title: details.title,
    description: details.description,
    impact: details.impact,
    suggestedAction: details.suggestedAction,
    businessQuestion: details.businessQuestion,
    controlMapping: details.controlMapping,
    evidence: [...baseEvidence(item), ...(details.evidence || [])],
    sensitiveLabels: sensitiveLabels.length > 0 ? sensitiveLabels : undefined,
    path: normalized(item.path),
    trustee: identityFor(item),
    trusteeSid: normalized(item.trustee_sid),
    rights: rightsText(item),
    type: normalized(item.type) || 'Allow',
    inherited: Boolean(item.inherited),
    source: normalized(item.source),
  };
}

function findingsForPermission(item: PermissionReportItem) {
  const results: PermissionExposureFinding[] = [];
  const trustee = identityFor(item);
  const trusteeLower = trustee.toLowerCase();
  const rights = rightsText(item);
  const rightsLower = rights.toLowerCase();
  const rightsRisk = inferRightsRisk(item);
  const broadTrustee = includesAny(trustee, broadTrusteePatterns);
  const privilegedTrustee = includesAny(trustee, privilegedTrusteePatterns);
  const unresolvedSID = isSID(normalized(item.trustee)) || isSID(normalized(item.trustee_sid)) || trusteeLower === 'unknown';
  const hasNestedGroupEvidence = Boolean(normalized(item.originating_group) || normalized(item.group_inheritance_hierarchy));
  const isDeny = normalized(item.type).toLowerCase() === 'deny';
  const hasFullControl = rightsLower.includes('full control') || rightsLower.includes('fullcontrol');
  const hasOwnershipRights = ['take ownership', 'change permissions', 'write owner', 'write dac'].some((token) => rightsLower.includes(token));
  const hasReadSurface = ['read', 'list folder', 'read and execute', 'execute'].some((token) => rightsLower.includes(token));
  const sensitiveLabels = classifySensitivePath(item.path);
  const isSensitivePath = sensitiveLabels.length > 0;
  const hasSensitiveAccessSurface = hasReadSurface || rightsRisk === 'high' || hasFullControl || hasOwnershipRights;

  if (!isDeny && isSensitivePath && broadTrustee && hasSensitiveAccessSurface) {
    const highImpactAccess = rightsRisk === 'high' || hasFullControl || hasOwnershipRights;
    results.push(createFinding(item, 'sensitive-path-broad-access', {
      category: 'sensitive-data',
      severity: highImpactAccess ? 'critical' : 'high',
      priorityScore: highImpactAccess ? 98 : 88,
      confidence: 'medium',
      remediationEffort: highImpactAccess ? 'planned-change' : 'owner-review',
      title: `Sensitive path exposure: ${trustee} can ${highImpactAccess ? 'change' : 'read'} ${sensitiveLabels[0]} data`,
      description: 'A broad principal has access to a path that looks like regulated, confidential, operationally critical, or business-sensitive data.',
      impact: 'A normal user, compromised account, or ransomware process may reach data that usually requires a named owner, approval trail, and tighter group boundary.',
      suggestedAction: 'Confirm the data owner, replace broad access with a purpose-built group, and document whether this sensitive area is intentionally shared.',
      businessQuestion: `Should ${sensitiveLabels.join(', ')} data be reachable by ${trustee}?`,
      controlMapping: ['Sensitive data governance', 'Least privilege', 'Data owner approval', 'Ransomware blast-radius reduction'],
      evidence: sensitiveEvidence(sensitiveLabels),
      sensitiveLabels,
    }));
  }

  if (!isDeny && isSensitivePath && !broadTrustee && (rightsRisk === 'high' || privilegedTrustee || hasFullControl || hasOwnershipRights)) {
    results.push(createFinding(item, 'sensitive-path-high-risk-access', {
      category: 'sensitive-data',
      severity: item.inherited && !hasOwnershipRights && !hasFullControl ? 'medium' : 'high',
      priorityScore: hasOwnershipRights || hasFullControl ? 90 : item.inherited ? 76 : 84,
      confidence: 'medium',
      remediationEffort: 'owner-review',
      title: `High-impact access on sensitive ${sensitiveLabels[0]} path`,
      description: 'This permission may be valid, but sensitive paths need stronger owner review than ordinary shared folders.',
      impact: 'The trustee can affect data that may carry privacy, financial, legal, backup, source-code, credential, or executive impact.',
      suggestedAction: 'Ask the data owner to confirm the trustee, rights level, inheritance boundary, and business justification before accepting the risk.',
      businessQuestion: 'Is this high-impact access approved by the owner of the sensitive data?',
      controlMapping: ['Sensitive data governance', 'Owner recertification', 'Privileged access review'],
      evidence: sensitiveEvidence(sensitiveLabels),
      sensitiveLabels,
    }));
  }

  if (broadTrustee && rightsRisk === 'high') {
    results.push(createFinding(item, 'blast-radius-broad-write', {
      category: 'overexposure',
      severity: 'critical',
      priorityScore: 97,
      confidence: 'high',
      remediationEffort: 'planned-change',
      title: `Blast radius: ${trustee} can change data`,
      description: 'A broad principal has write-capable permissions. This is the fastest path from a normal account or ransomware process to wide data impact.',
      impact: 'Large user populations may be able to modify, delete, encrypt, or replace content under this path.',
      suggestedAction: 'Replace the broad principal with a named business group, downgrade to read-only if possible, and rescan to verify removal.',
      businessQuestion: 'Should this many people really be able to change this data?',
      controlMapping: ['Least privilege', 'Ransomware blast-radius reduction', 'Data access governance'],
    }));
  }

  if (broadTrustee && rightsRisk !== 'high' && hasReadSurface) {
    results.push(createFinding(item, 'broad-read-surface', {
      category: 'overexposure',
      severity: item.inherited ? 'medium' : 'high',
      confidence: 'high',
      remediationEffort: 'owner-review',
      title: `Broad read surface for ${trustee}`,
      description: 'A broad principal can read this path. This may be acceptable for public shares, but it should be an explicit business decision.',
      impact: 'Sensitive files placed here later may become visible to a large population by default.',
      suggestedAction: 'Confirm the data owner accepts broad read access, or replace it with a least-privilege read group.',
      businessQuestion: 'Is this folder intended to be visible to nearly everyone?',
      controlMapping: ['Data owner review', 'Least privilege', 'Compliance evidence'],
    }));
  }

  if (!item.inherited && rightsRisk === 'high') {
    results.push(createFinding(item, 'explicit-dangerous-ace', {
      category: 'governance',
      severity: broadTrustee ? 'critical' : 'high',
      priorityScore: broadTrustee ? 94 : 82,
      confidence: 'high',
      remediationEffort: broadTrustee ? 'planned-change' : 'owner-review',
      title: `${trustee} has explicit high-risk permissions`,
      description: 'Explicit high-risk ACEs bypass parent inheritance governance and often become long-lived exceptions.',
      impact: 'This path may drift away from standard access policy and become harder to review during audits.',
      suggestedAction: 'Validate the business reason, move access into a managed group, and document or remove the exception.',
      businessQuestion: 'Is this direct exception still required?',
      controlMapping: ['Access recertification', 'Exception management', 'Audit readiness'],
    }));
  }

  if (hasOwnershipRights) {
    results.push(createFinding(item, 'ownership-grade-permission', {
      category: 'privilege',
      severity: broadTrustee ? 'critical' : 'high',
      priorityScore: broadTrustee ? 99 : 88,
      confidence: 'high',
      remediationEffort: 'planned-change',
      title: `${trustee} can change ownership or permissions`,
      description: 'Ownership-grade rights can change the security boundary itself, not just file contents.',
      impact: 'A compromised trustee could grant persistence, hide access, or block legitimate administrators from recovering cleanly.',
      suggestedAction: 'Remove ownership/change-permission rights unless this trustee is a controlled administrative role.',
      businessQuestion: 'Who is allowed to change the access model for this data?',
      controlMapping: ['Privilege management', 'Change control', 'Tiered administration'],
    }));
  }

  if (hasFullControl && !hasOwnershipRights) {
    results.push(createFinding(item, 'full-control-entitlement', {
      category: 'privilege',
      severity: broadTrustee ? 'critical' : item.inherited ? 'medium' : 'high',
      confidence: 'high',
      remediationEffort: item.inherited ? 'owner-review' : 'planned-change',
      title: `${trustee} has Full Control`,
      description: 'Full Control is usually broader than day-to-day business access and should be reserved for ownership or administration roles.',
      impact: 'The trustee may be able to modify, delete, and change access in ways that exceed normal least-privilege needs.',
      suggestedAction: 'Downgrade to Modify or Read where ownership is not required, and keep Full Control limited to controlled owner/admin groups.',
      businessQuestion: 'Does this trustee need ownership-level access or only operational access?',
      controlMapping: ['Least privilege', 'Owner access review', 'Privilege reduction'],
    }));
  }

  if (privilegedTrustee) {
    results.push(createFinding(item, 'privileged-group-on-data', {
      category: 'privilege',
      severity: rightsRisk === 'high' ? 'high' : 'medium',
      confidence: 'medium',
      remediationEffort: 'owner-review',
      title: `${trustee} appears directly on data permissions`,
      description: 'Administrative groups on business data blur operational administration and data ownership.',
      impact: 'A domain or server admin compromise may immediately expand into data access without a separate data-owner decision.',
      suggestedAction: 'Confirm this is intentional. Prefer dedicated data-owner or break-glass groups instead of broad admin groups.',
      businessQuestion: 'Is this an admin convenience permission or a true data ownership requirement?',
      controlMapping: ['Tiered administration', 'Separation of duties', 'Privileged access review'],
    }));
  }

  if (unresolvedSID) {
    results.push(createFinding(item, 'orphaned-identity-on-acl', {
      category: 'hygiene',
      severity: rightsRisk === 'high' ? 'high' : 'medium',
      confidence: 'high',
      remediationEffort: 'quick-win',
      title: 'Orphaned or unresolved identity remains on ACL',
      description: 'A SID or unknown identity cannot be tied to a real owner during review.',
      impact: 'Auditors and administrators cannot prove who owns the access, and stale identities make cleanup harder.',
      suggestedAction: 'Resolve the SID to an account or remove the ACE if the identity no longer exists.',
      businessQuestion: 'Can we prove who this access belongs to?',
      controlMapping: ['Identity hygiene', 'Audit evidence quality', 'Access cleanup'],
    }));
  }

  if (hasNestedGroupEvidence && rightsRisk === 'high') {
    results.push(createFinding(item, 'nested-group-high-risk-access', {
      category: 'governance',
      severity: 'high',
      confidence: 'medium',
      remediationEffort: 'owner-review',
      title: `${trustee} receives high-risk access through group expansion`,
      description: 'Nested group paths make it difficult for owners to understand why a user has access.',
      impact: 'Access reviews can miss hidden membership chains, especially when groups are reused across shares.',
      suggestedAction: 'Review the originating group chain, remove unnecessary nesting, and keep one owner group per data area where possible.',
      businessQuestion: 'Can the data owner explain this access path without IT reverse-engineering it?',
      controlMapping: ['Effective permissions', 'Group hygiene', 'Data owner review'],
      evidence: normalized(item.group_inheritance_hierarchy) ? [`Group chain: ${normalized(item.group_inheritance_hierarchy)}`] : [],
    }));
  }

  if (item.inherited && rightsRisk === 'high' && pathDepth(normalized(item.path)) >= 3) {
    results.push(createFinding(item, 'inherited-high-risk-spread', {
      category: 'overexposure',
      severity: broadTrustee ? 'high' : 'medium',
      confidence: 'medium',
      remediationEffort: 'owner-review',
      title: 'Inherited high-risk access reaches a child path',
      description: 'High-risk permissions are flowing down the tree. This is efficient for administration but risky when sensitive subfolders exist below.',
      impact: 'A sensitive child folder may inherit access that was only intended for a broader parent area.',
      suggestedAction: 'Review inheritance boundaries and break inheritance only where data sensitivity requires a different access model.',
      businessQuestion: 'Do all child folders under this parent need the same access?',
      controlMapping: ['Inheritance governance', 'Sensitive folder review', 'Least privilege'],
    }));
  }

  if (isDeny) {
    results.push(createFinding(item, 'explicit-deny-friction', {
      category: 'operational-friction',
      severity: rightsRisk === 'high' ? 'medium' : 'low',
      confidence: 'medium',
      remediationEffort: 'quick-win',
      title: `${trustee} has an explicit deny entry`,
      description: 'Explicit deny ACEs can override expected group access and create hard-to-debug support incidents.',
      impact: 'Users may be blocked despite valid group membership, causing operational noise and confusing evidence.',
      suggestedAction: 'Confirm the deny is intentional; prefer removing allow access from groups rather than relying on deny entries.',
      businessQuestion: 'Is this deny entry a documented control or an old workaround?',
      controlMapping: ['Access model simplification', 'Support risk reduction', 'Exception cleanup'],
    }));
  }

  return results;
}

export function summarizePermissionExposure(findings: PermissionExposureFinding[]): PermissionExposureSummary {
  const priorityScores = findings.map((finding) => finding.priorityScore).sort((a, b) => b - a).slice(0, 10);
  const exposureScore = priorityScores.length === 0
    ? 0
    : Math.round(priorityScores.reduce((sum, score) => sum + score, 0) / priorityScores.length);
  const sensitiveFindings = findings.filter((finding) => finding.category === 'sensitive-data' || Boolean(finding.sensitiveLabels?.length));

  return {
    total: findings.length,
    critical: findings.filter((finding) => finding.severity === 'critical').length,
    high: findings.filter((finding) => finding.severity === 'high').length,
    medium: findings.filter((finding) => finding.severity === 'medium').length,
    low: findings.filter((finding) => finding.severity === 'low').length,
    exposureScore,
    rulesTriggered: new Set(findings.map((finding) => finding.ruleID)).size,
    riskyPaths: new Set(findings.map((finding) => finding.path.toLowerCase())).size,
    riskyTrustees: new Set(findings.map((finding) => finding.trustee.toLowerCase())).size,
    sensitiveFindings: sensitiveFindings.length,
    sensitivePaths: new Set(sensitiveFindings.map((finding) => finding.path.toLowerCase())).size,
    quickWins: findings.filter((finding) => finding.remediationEffort === 'quick-win').length,
    ownerReviews: findings.filter((finding) => finding.remediationEffort === 'owner-review').length,
  };
}

export function analyzePermissionExposure(permissions: PermissionReportItem[]): PermissionExposureAnalysis {
  const findings = permissions
    .flatMap((item) => findingsForPermission(item))
    .sort((a, b) => b.priorityScore - a.priorityScore || a.path.localeCompare(b.path));

  return {
    findings,
    summary: summarizePermissionExposure(findings),
  };
}
