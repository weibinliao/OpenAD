import {
  RISK_FINDINGS_KEY,
  readRiskFindings,
  summarizeRiskFindings,
  upsertRiskFindingsFromScan,
  updateRiskFindingStatus,
} from './riskFindings';

describe('riskFindings exposure integration', () => {
  beforeEach(() => {
    window.localStorage.removeItem(RISK_FINDINGS_KEY);
  });

  test('persists exposure-engine metadata from a scan', () => {
    const summary = upsertRiskFindingsFromScan({
      sessionID: 'session-1',
      scannedAt: '2026-05-08T08:00:00.000Z',
      permissions: [
        {
          path: 'C:\\Data\\Finance',
          trustee: 'Everyone',
          trustee_sid: 'S-1-1-0',
          rights: 'Modify',
          type: 'Allow',
          inherited: false,
          source: 'Explicit',
        },
      ],
    });

    const findings = readRiskFindings();
    expect(summary.exposureScore).toBeGreaterThanOrEqual(90);
    expect(summary.sensitivePaths).toBe(1);
    expect(findings[0]).toEqual(
      expect.objectContaining({
        category: expect.any(String),
        priorityScore: expect.any(Number),
        evidence: expect.any(Array),
        controlMapping: expect.any(Array),
        sensitiveLabels: expect.arrayContaining(['Finance']),
      })
    );
  });

  test('keeps accepted findings while incrementing seen count', () => {
    upsertRiskFindingsFromScan({
      sessionID: 'session-1',
      scannedAt: '2026-05-08T08:00:00.000Z',
      permissions: [
        {
          path: 'C:\\Data',
          trustee: 'Everyone',
          trustee_sid: 'S-1-1-0',
          rights: 'Modify',
          type: 'Allow',
          inherited: false,
        },
      ],
    });

    const [first] = readRiskFindings();
    updateRiskFindingStatus(first.id, 'accepted');

    upsertRiskFindingsFromScan({
      sessionID: 'session-2',
      scannedAt: '2026-05-09T08:00:00.000Z',
      permissions: [
        {
          path: 'C:\\Data',
          trustee: 'Everyone',
          trustee_sid: 'S-1-1-0',
          rights: 'Modify',
          type: 'Allow',
          inherited: false,
        },
      ],
    });

    const acceptedFinding = readRiskFindings().find((finding) => finding.id === first.id);
    const summary = summarizeRiskFindings();
    expect(acceptedFinding?.status).toBe('accepted');
    expect(acceptedFinding?.seenCount).toBe(2);
    expect(summary.accepted).toBeGreaterThanOrEqual(1);
  });
});
