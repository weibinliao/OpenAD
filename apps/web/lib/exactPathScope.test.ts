import { filterRowsForExactSelectedPath, hasExactPathMatch, normalizePathForExactMatch } from './exactPathScope';

describe('exactPathScope', () => {
  test('normalizes exact-match path keys case-insensitively and trims trailing slashes', () => {
    expect(normalizePathForExactMatch('\\\\Server\\Share\\Finance\\')).toBe('\\\\server\\share\\finance');
    expect(normalizePathForExactMatch('C:/Finance/')).toBe('c:\\finance');
    expect(normalizePathForExactMatch('C:\\')).toBe('c:\\');
  });

  test('matches only the exact selected path and excludes child paths', () => {
    const rows = [
      { path: '\\\\server\\share\\Finance', id: 'root-row' },
      { path: '\\\\server\\share\\Finance\\Quarterly', id: 'child-row' },
      { path: '\\\\server\\share\\Payroll', id: 'sibling-row' },
    ];

    expect(hasExactPathMatch('\\\\SERVER\\share\\Finance', '\\\\server\\share\\Finance\\')).toBe(true);
    expect(hasExactPathMatch('\\\\server\\share\\Finance\\Quarterly', '\\\\server\\share\\Finance')).toBe(false);

    expect(filterRowsForExactSelectedPath(rows, '\\\\server\\share\\Finance\\')).toEqual([{ path: '\\\\server\\share\\Finance', id: 'root-row' }]);
  });
});
