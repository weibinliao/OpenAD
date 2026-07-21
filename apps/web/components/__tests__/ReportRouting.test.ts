import { readFileSync } from 'fs';
import { resolve } from 'path';

describe('report route ownership', () => {
  test('history opens saved sessions in the report center', () => {
    const source = readFileSync(resolve(process.cwd(), 'pages/history.tsx'), 'utf8');

    expect(source).toContain('/reports?session=');
    expect(source).not.toContain('/scan-workspace?session=');
  });

  test('risk findings open their evidence scope in the report center', () => {
    const source = readFileSync(resolve(process.cwd(), 'pages/findings.tsx'), 'utf8');

    expect(source).toContain('params.set(\'mode\', \'folder\')');
    expect(source).toContain('router.push(`/reports?${params.toString()}`)');
    expect(source).not.toContain('router.push(`/scan-workspace?${params.toString()}`)');
  });
});
