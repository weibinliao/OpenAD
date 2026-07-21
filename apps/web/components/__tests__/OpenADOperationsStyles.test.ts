import fs from 'fs';
import path from 'path';

describe('OpenAD operations workspace styles', () => {
  const css = fs.readFileSync(
    path.join(process.cwd(), 'styles', 'openad-operations.css'),
    'utf8',
  );

  it('defines stable navigation descriptions and operational layouts', () => {
    expect(css).toContain('.pp-rail-description');
    expect(css).toContain('.pp-readonly-badge');
    expect(css).toContain('.openad-overview-status');
    expect(css).toContain('.openad-operations-grid');
    expect(css).toContain('.openad-reports-grid');
  });

  it('collapses the overview to one column at narrow widths', () => {
    expect(css).toMatch(/@media \(max-width: 900px\)[\s\S]*\.openad-operations-grid/);
    expect(css).toMatch(/@media \(max-width: 900px\)[\s\S]*\.openad-evidence-grid/);
  });
});
