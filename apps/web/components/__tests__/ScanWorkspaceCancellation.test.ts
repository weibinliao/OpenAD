import { resolveCancelScanID } from '../../pages/scan-workspace';

describe('resolveCancelScanID', () => {
  it('uses the current ref while a batch is switching shares', () => {
    expect(resolveCancelScanID(' current-share ', 'previous-share', 'previous-session'))
      .toBe('current-share');
  });

  it('falls back to live scan and session identifiers for normal cancellation', () => {
    expect(resolveCancelScanID('', 'live-share', 'session-id')).toBe('live-share');
    expect(resolveCancelScanID('', '', 'session-id')).toBe('session-id');
  });

  it('returns an empty identifier when no scan is active', () => {
    expect(resolveCancelScanID('  ', '\t', '')).toBe('');
  });
});
