import {
  RISK_FINDINGS_KEY,
  loadRiskFindings,
  summarizeRiskFindings,
  upsertRiskFindingsFromScan,
  updateRiskFindingStatus,
  type RiskFinding,
} from './riskFindings';

const legacyFinding: RiskFinding = {
  id: 'legacy-1',
  fingerprint: 'broad-access|c:\\finance|everyone|modify',
  status: 'accepted',
  severity: 'critical',
  type: 'broad-access',
  title: 'Broad access to Finance',
  suggestedAction: 'Remove broad write access.',
  path: 'C:\\Finance',
  trustee: 'Everyone',
  trusteeSid: 'S-1-1-0',
  rights: 'Modify',
  inherited: false,
  source: 'Explicit',
  firstSeenAt: '2026-05-08T08:00:00.000Z',
  lastSeenAt: '2026-05-09T08:00:00.000Z',
  lastSessionID: 'session-1',
  seenCount: 2,
  note: 'Approved exception',
};

function response(body: unknown, ok = true) {
  return Promise.resolve({
    ok,
    status: ok ? 200 : 500,
    json: async () => body,
  } as Response);
}

describe('riskFindings server persistence', () => {
  beforeEach(() => {
    window.localStorage.clear();
    global.fetch = jest.fn();
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  test('persists scan findings through the API without writing localStorage', async () => {
    const setItem = jest.spyOn(Storage.prototype, 'setItem');
    (global.fetch as jest.Mock).mockImplementation(() => response({ count: 1 }));

    await upsertRiskFindingsFromScan({
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

    expect(setItem).not.toHaveBeenCalled();
    expect(global.fetch).toHaveBeenCalledTimes(1);
    const [url, options] = (global.fetch as jest.Mock).mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/api/risk-findings/upsert');
    const payload = JSON.parse(String(options.body));
    expect(payload.items[0]).toEqual(expect.objectContaining({
      fingerprint: expect.any(String),
      trustee_sid: 'S-1-1-0',
      last_session_id: 'session-1',
      priority_score: expect.any(Number),
      evidence: expect.any(Array),
    }));
  });

  test('migrates legacy localStorage findings once before loading the server copy', async () => {
    window.localStorage.setItem(RISK_FINDINGS_KEY, JSON.stringify([legacyFinding]));
    (global.fetch as jest.Mock)
      .mockImplementationOnce(() => response({ count: 1 }))
      .mockImplementationOnce(() => response({ items: [{
        id: 'server-1',
        fingerprint: legacyFinding.fingerprint,
        status: 'accepted',
        severity: 'critical',
        type: 'broad-access',
        title: legacyFinding.title,
        suggested_action: legacyFinding.suggestedAction,
        path: legacyFinding.path,
        trustee: legacyFinding.trustee,
        trustee_sid: legacyFinding.trusteeSid,
        rights: legacyFinding.rights,
        inherited: false,
        source: legacyFinding.source,
        first_seen_at: legacyFinding.firstSeenAt,
        last_seen_at: legacyFinding.lastSeenAt,
        last_session_id: legacyFinding.lastSessionID,
        seen_count: 2,
        note: legacyFinding.note,
      }] }))
      .mockImplementationOnce(() => response({ items: [] }));

    const findings = await loadRiskFindings();
    expect(findings).toEqual([expect.objectContaining({
      id: 'server-1',
      status: 'accepted',
      trusteeSid: 'S-1-1-0',
      seenCount: 2,
    })]);
    expect(window.localStorage.getItem(RISK_FINDINGS_KEY)).toBeNull();

    const [importURL, importOptions] = (global.fetch as jest.Mock).mock.calls[0] as [string, RequestInit];
    expect(importURL).toContain('/api/risk-findings/import');
    expect(JSON.parse(String(importOptions.body)).items[0]).toEqual(expect.objectContaining({
      status: 'accepted',
      seen_count: 2,
      note: 'Approved exception',
    }));

    await loadRiskFindings();
    expect(global.fetch).toHaveBeenCalledTimes(3);
  });

  test('updates review state through the API', async () => {
    (global.fetch as jest.Mock).mockImplementation(() => response({
      id: 'server-1',
      fingerprint: legacyFinding.fingerprint,
      status: 'resolved',
      severity: 'critical',
      type: legacyFinding.type,
      title: legacyFinding.title,
      suggested_action: legacyFinding.suggestedAction,
      path: legacyFinding.path,
      trustee: legacyFinding.trustee,
      trustee_sid: legacyFinding.trusteeSid,
      rights: legacyFinding.rights,
      inherited: false,
      source: legacyFinding.source,
      first_seen_at: legacyFinding.firstSeenAt,
      last_seen_at: legacyFinding.lastSeenAt,
      seen_count: 2,
    }));

    const updated = await updateRiskFindingStatus('server-1', 'resolved');

    expect(updated.status).toBe('resolved');
    const [url, options] = (global.fetch as jest.Mock).mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/api/risk-findings/server-1');
    expect(options.method).toBe('PUT');
    expect(JSON.parse(String(options.body))).toEqual({ status: 'resolved' });
  });

  test('summarizes server-loaded findings', () => {
    const summary = summarizeRiskFindings([legacyFinding]);
    expect(summary.total).toBe(1);
    expect(summary.accepted).toBe(1);
    expect(summary.exposureScore).toBe(0);
  });
});
