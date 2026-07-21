import { analyzePermissionExposure } from './permissionExposure';
import type { PermissionReportItem } from './reportPayload';

function permission(overrides: Partial<PermissionReportItem> = {}): PermissionReportItem {
  return {
    path: 'C:\\Data\\Finance',
    trustee: 'DOMAIN\\Finance Readers',
    trustee_sid: 'S-1-5-21-1000',
    rights: 'Read',
    type: 'Allow',
    inherited: false,
    source: 'Explicit',
    ...overrides,
  };
}

describe('permissionExposure', () => {
  test('flags broad write-capable access as blast-radius critical exposure', () => {
    const analysis = analyzePermissionExposure([
      permission({ trustee: 'Everyone', rights: 'Modify', trustee_sid: 'S-1-1-0' }),
    ]);

    expect(analysis.findings.some((finding) => finding.ruleID === 'blast-radius-broad-write')).toBe(true);
    expect(analysis.summary.critical).toBeGreaterThanOrEqual(1);
    expect(analysis.summary.exposureScore).toBeGreaterThanOrEqual(90);
  });

  test('separates broad read exposure from write-capable blast radius', () => {
    const analysis = analyzePermissionExposure([
      permission({ trustee: 'Authenticated Users', rights: 'Read and Execute', inherited: true }),
    ]);

    expect(analysis.findings).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ ruleID: 'broad-read-surface', category: 'overexposure' }),
      ])
    );
    expect(analysis.summary.medium).toBeGreaterThanOrEqual(1);
  });

  test('flags broad access to sensitive business paths', () => {
    const analysis = analyzePermissionExposure([
      permission({ path: '\\\\fs01\\Shares\\HR\\Payroll', trustee: 'Domain Users', trustee_sid: 'S-1-5-21-513', rights: 'Read' }),
    ]);

    expect(analysis.findings).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          ruleID: 'sensitive-path-broad-access',
          category: 'sensitive-data',
          sensitiveLabels: expect.arrayContaining(['HR', 'Payroll']),
        }),
      ])
    );
    expect(analysis.summary.sensitivePaths).toBe(1);
  });

  test('adds owner-review context for high-risk access on sensitive paths', () => {
    const analysis = analyzePermissionExposure([
      permission({ path: 'D:\\Repos\\PaymentApp\\Secrets', trustee: 'DOMAIN\\Build Operators', rights: 'Full Control' }),
    ]);

    expect(analysis.findings).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          ruleID: 'sensitive-path-high-risk-access',
          category: 'sensitive-data',
          remediationEffort: 'owner-review',
          sensitiveLabels: expect.arrayContaining(['Source Code', 'Secrets / Credentials']),
        }),
      ])
    );
  });

  test('does not treat deny entries as sensitive-data exposure', () => {
    const analysis = analyzePermissionExposure([
      permission({ path: 'C:\\Shares\\Legal\\Contracts', trustee: 'Everyone', rights: 'Read', type: 'Deny' }),
    ]);

    expect(analysis.findings.some((finding) => finding.ruleID === 'sensitive-path-broad-access')).toBe(false);
  });

  test('adds governance and hygiene context for nested groups and orphaned SIDs', () => {
    const analysis = analyzePermissionExposure([
      permission({
        trustee: 'S-1-5-21-404',
        trustee_sid: 'S-1-5-21-404',
        rights: 'Full Control',
        originating_group: 'DOMAIN\\LegacyAccess',
        group_inheritance_hierarchy: 'LegacyAccess > Finance Admins',
      }),
    ]);

    expect(analysis.findings).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ ruleID: 'orphaned-identity-on-acl', remediationEffort: 'quick-win' }),
        expect.objectContaining({ ruleID: 'nested-group-high-risk-access', category: 'governance' }),
      ])
    );
    expect(analysis.summary.quickWins).toBeGreaterThanOrEqual(1);
  });
});
