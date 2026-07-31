import { readFileSync } from 'fs';
import { resolve } from 'path';

describe('Scan workspace SID resolution', () => {
  test('delegates fallback and exclusion behavior to the backend in one scan request', () => {
    const source = readFileSync(resolve(process.cwd(), 'pages/scan-workspace.tsx'), 'utf8');

    expect(source).not.toContain('legacyExclusionGroupPatterns');
    expect(source).not.toContain('executeScan(false)');
    expect(source).not.toContain('fallbackData');
    expect(source).toContain('identity_resolution');
  });
});
